package lookahead

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

// SearchPolicy is the look-ahead player (level 1, flat Monte Carlo). For each
// real decision it narrows the options with the dice solver, then plays each
// remaining option forward many times on reseeded copies of the battle (its
// own later moves by the greedy player, the opponent's by its model) and
// compares how the futures end.
//
// The greedy player's choice is the anchor. Every option is played on the
// same random futures as the anchor, weaker options are dropped by successive
// halving, and the survivor replaces the anchor only if it beats it by a
// clear margin (a paired t-statistic above MinT). Most options in a dice game
// differ by a few points of win chance, far below the noise of a few hundred
// futures, so without the anchor the search would mostly pick at random.
//
// Nothing is learned. It is strongest when the opponent model is exact, as
// it is for content-scripted minions; against another hero the model is the
// greedy player.
type SearchPolicy struct {
	Label   string
	Rollout *GreedyPolicy
	// NewOpponent builds the model of every other seat for one worker.
	NewOpponent func() Policy
	// Budget is the number of simulated futures per decision.
	Budget int
	// Rounds limits each future to this many rounds and scores the position
	// there; zero plays every future to the end of the battle.
	Rounds int
	// Evaluator, when set, builds each worker's judge of the horizon (a value
	// network): futures then stop at the first offensive-planning start
	// after Rounds and the judge's expected margin replaces the health
	// margin there. Nil scores the health race (version 2).
	Evaluator func() Evaluator
	// MarginWeight adds the health difference (as fractions of each side's
	// maximum) to the win/draw/loss score, to separate options that win
	// equally often.
	MarginWeight float64
	// TopRerolls keeps only the solver's best rerolls (and die choices) as
	// options, and TopAbilities the best ability choices (2 when zero).
	TopRerolls, TopAbilities int
	// MinT is the paired t-statistic an option needs to override the anchor.
	MinT float64
	// MaxBudget caps the extra futures spent confirming a promising option
	// (paired t between 1 and MinT).
	MaxBudget int
	// Workers plays futures on this many goroutines (1 when the caller
	// already runs many battles in parallel).
	Workers int
	// MaxThink stops confirming a promising option after this long; zero
	// means no limit.
	MaxThink time.Duration
	// Think, when set, is a hard limit on each decision's thinking time and
	// replaces Budget and MaxBudget: options are compared on as many futures
	// as fit. Successive halving gets the first half of the time, a
	// promising survivor's extra futures run to three quarters, and the
	// fresh-futures check to the limit. A future still running at the limit
	// is abandoned, and the options keep only the futures all of them
	// finished.
	Think time.Duration
	// Confirm re-tests a would-be override on fresh futures before using it.
	Confirm bool
	// Searched, Overrides and Rejected count decisions that were simulated,
	// those where the search overrode the greedy player, and would-be
	// overrides that failed the fresh-futures check; Futures counts the
	// futures played.
	Searched, Overrides, Rejected, Futures int
	// Trace, when set, receives a line for every override.
	Trace func(line string)

	// Advisor, when set, is a policy network consulted at each searched
	// decision: its pick replaces the greedy player's as the anchor when it
	// scores more than AdvisorMargin above it, and its AdvisorTop best
	// options (with both picks) are the candidates. Advised counts the
	// decisions where its pick became the anchor.
	Advisor       Advisor
	AdvisorMargin float64
	AdvisorTop    int
	Advised       int
	// NewRollout, when set, builds each worker's player for the searching
	// seat's own simulated moves instead of a copy of Rollout.
	NewRollout func() Policy

	pool []*searchWorker
}

// Advisor scores a decision's options (indices into d.Actions; greedy is the
// greedy player's pick) for the search.
type Advisor interface {
	Scores(d *Decision, options []int, greedy int) ([]float64, error)
}

// searchWorker owns everything one goroutine needs to play futures; players
// cache solver tables and are not shared.
type searchWorker struct {
	sim      *Simulator
	hero     Policy
	opponent Policy
	judge    Evaluator
}

// Counts reports how many decisions were searched and overridden.
func (s *SearchPolicy) Counts() (searched, overrides int) { return s.Searched, s.Overrides }

// AdvisedCount reports the decisions where the Advisor's pick became the
// anchor.
func (s *SearchPolicy) AdvisedCount() int { return s.Advised }

// FutureCount reports how many futures the searched decisions played.
func (s *SearchPolicy) FutureCount() int { return s.Futures }

// thinkCap bounds the futures per option in a time-limited decision, for
// positions so close to the end that futures cost almost nothing.
const thinkCap = 4096

// NewSearchPolicy builds a look-ahead player; newOpponent models the other
// seats (a scripted minion's own script, or NewGreedyPolicy for a hero).
func NewSearchPolicy(newOpponent func() Policy, budget int) *SearchPolicy {
	return &SearchPolicy{Label: "lookahead", Rollout: NewGreedyPolicy(), NewOpponent: newOpponent, Budget: budget, MaxBudget: 4 * budget, MarginWeight: 0.2, TopRerolls: 3, MinT: 2.5, Workers: 1, Confirm: true}
}

// NewSearchPolicyV1 is the first version: its own simulated play uses the
// first greedy player and overrides are not re-tested on fresh futures.
func NewSearchPolicyV1(newOpponent func() Policy, budget int) *SearchPolicy {
	s := NewSearchPolicy(newOpponent, budget)
	s.Label, s.Rollout, s.Confirm = "lookahead-v1", NewGreedyPolicyV1(), false
	return s
}

func (s *SearchPolicy) Name() string { return s.Label }

func (s *SearchPolicy) workers() ([]*searchWorker, error) {
	for len(s.pool) < max(1, s.Workers) {
		sim, err := NewSimulator()
		if err != nil {
			return nil, err
		}
		var hero Policy = &GreedyPolicy{Solver: NewDiceSolver(), Rates: s.Rollout.Rates, CardValue: s.Rollout.CardValue, DrawValue: s.Rollout.DrawValue, DiceCards: s.Rollout.DiceCards, Bias: s.Rollout.Bias}
		if s.NewRollout != nil {
			hero = s.NewRollout()
		}
		w := &searchWorker{sim: sim, hero: hero, opponent: s.NewOpponent()}
		if s.Evaluator != nil {
			w.judge = s.Evaluator()
		}
		s.pool = append(s.pool, w)
	}
	return s.pool[:max(1, s.Workers)], nil
}

func (s *SearchPolicy) Choose(d *Decision) (int, error) {
	started := time.Now()
	var deadline time.Time
	if s.Think > 0 {
		deadline = started.Add(s.Think)
	}
	// at is the moment a share of the thinking time has passed.
	at := func(share float64) time.Time { return started.Add(time.Duration(share * float64(s.Think))) }
	options := Useful(d.Battle, d.Seat, d.Actions)
	if len(options) == 1 {
		return options[0], nil
	}
	anchor, err := s.Rollout.Choose(d)
	if err != nil {
		return 0, err
	}
	var candidates []int
	if s.Advisor != nil {
		candidates, anchor, err = s.advise(d, options, anchor)
	} else {
		candidates, err = s.candidates(d, options, anchor)
	}
	if err != nil {
		return 0, err
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	pool, err := s.workers()
	if err != nil {
		return 0, err
	}
	s.Searched++
	base := uint64(d.Rand.Int63())
	type arm struct {
		index  int
		offset uint64 // futures j of this arm use seed slot offset+j
		values []float64
	}
	anchorArm := &arm{index: anchor}
	var alive []*arm
	for _, c := range candidates {
		if c != anchor {
			alive = append(alive, &arm{index: c})
		}
	}
	// extend plays the listed arms' futures up to n, in parallel. Future j
	// uses the same seed for every arm, so arms are compared on equal luck.
	// With `until` set no future starts after it, and every arm keeps only
	// the futures all of them finished (a partial last round is dropped).
	extend := func(arms []*arm, n int, until time.Time) {
		from := n
		for _, a := range arms {
			from = min(from, len(a.values))
		}
		if from >= n {
			return
		}
		finished := make([][]bool, len(arms))
		for k, a := range arms {
			for len(a.values) < n {
				a.values = append(a.values, 0)
			}
			finished[k] = make([]bool, n)
			for j := 0; j < from; j++ {
				finished[k][j] = true
			}
		}
		runJobs(pool, (n-from)*len(arms), until, func(w *searchWorker) func(t int) {
			future := s.future(w, d, deadline)
			return func(t int) {
				j, k := from+t/len(arms), t%len(arms)
				a := arms[k]
				v, err := future(a.index, base+(a.offset+uint64(j))*0x9e3779b97f4a7c15)
				if errors.Is(err, errDeadline) {
					return
				}
				a.values[j] = v
				finished[k][j] = true
			}
		})
		common := n
		for k := range arms {
			done := from
			for done < n && finished[k][done] {
				done++
			}
			common = min(common, done)
			for j := from; j < n; j++ {
				if finished[k][j] {
					s.Futures++
				}
			}
		}
		for _, a := range arms {
			a.values = a.values[:common]
		}
	}
	pairedDiff := func(a, anchor *arm) (mean, t float64) {
		n := len(a.values)
		if n < 2 {
			return 0, 0
		}
		var sum, sq float64
		for j := 0; j < n; j++ {
			x := a.values[j] - anchor.values[j]
			sum += x
			sq += x * x
		}
		mean = sum / float64(n)
		variance := sq/float64(n) - mean*mean
		if variance <= 1e-12 {
			if mean > 0 {
				return mean, math.Inf(1)
			}
			return mean, 0
		}
		return mean, mean / math.Sqrt(variance/float64(n))
	}
	diff := func(a *arm) (float64, float64) { return pairedDiff(a, anchorArm) }
	phases := max(1, int(math.Ceil(math.Log2(float64(len(alive)+1)))))
	step := max(4, s.Budget/((len(alive)+1)*phases))
	n, spent := 0, 0
	for phase := 1; ; phase++ {
		if s.Think > 0 {
			extend(append([]*arm{anchorArm}, alive...), thinkCap, at(0.5*float64(phase)/float64(phases)))
			n = len(anchorArm.values)
		} else {
			n += step
			extend(append([]*arm{anchorArm}, alive...), n, time.Time{})
			spent += step * (len(alive) + 1)
		}
		sort.SliceStable(alive, func(i, j int) bool {
			mi, _ := diff(alive[i])
			mj, _ := diff(alive[j])
			return mi > mj
		})
		if len(alive) == 1 {
			break
		}
		alive = alive[:(len(alive)+1)/2]
	}
	// A promising survivor gets more futures, up to MaxBudget (and MaxThink),
	// or to three quarters of the thinking time, before the verdict.
	for {
		mean, t := diff(alive[0])
		if mean <= 0 || t < 1 || t >= s.MinT {
			break
		}
		more := max(step, n/2)
		if s.Think > 0 {
			if !time.Now().Before(at(0.75)) {
				break
			}
			extend([]*arm{anchorArm, alive[0]}, min(n+more, thinkCap), at(0.75))
			if len(anchorArm.values) <= n {
				break
			}
			n = len(anchorArm.values)
			continue
		}
		if spent >= s.MaxBudget || (s.MaxThink > 0 && time.Since(started) > s.MaxThink) {
			break
		}
		n += more
		extend([]*arm{anchorArm, alive[0]}, n, time.Time{})
		spent += 2 * more
	}
	mean, t := diff(alive[0])
	if mean > 0 && t >= s.MinT && s.Confirm {
		// The survivor was picked for looking best on these futures, which
		// flatters it. It must win again on futures it has never seen.
		fresh := max(len(alive[0].values), 24)
		freshAnchor := &arm{index: anchor, offset: 1 << 32}
		freshChallenger := &arm{index: alive[0].index, offset: 1 << 32}
		extend([]*arm{freshAnchor, freshChallenger}, fresh, deadline)
		if m, ft := pairedDiff(freshChallenger, freshAnchor); m <= 0 || ft < s.MinT {
			s.Rejected++
			return anchor, nil
		}
	}
	if mean > 0 && t >= s.MinT {
		s.Overrides++
		if s.Trace != nil {
			s.Trace(fmt.Sprintf("round %d %s: greedy %s → look-ahead %s (+%.3f, t=%.1f, %d futures each; dice %v, energy %d)",
				d.Battle.Segment.Round, d.Battle.Settled.Window.Stage, Describe(d.Battle, d.Seat, d.Actions[anchor]), Describe(d.Battle, d.Seat, d.Actions[alive[0].index]),
				mean, t, len(alive[0].values), diceFaces(d.Battle, d.Seat), d.Battle.Actors[d.Seat].Resources.EnergyPoints))
		}
		return alive[0].index, nil
	}
	return anchor, nil
}

// runJobs runs jobs 0..total-1 in order on the pool's workers; each worker
// builds its job function once. With until set no job starts after it. A
// single worker runs the jobs inline: handing each future to a goroutine
// cost about a third of the search's time.
func runJobs(pool []*searchWorker, total int, until time.Time, build func(w *searchWorker) func(t int)) {
	var next atomic.Int64
	work := func(w *searchWorker) {
		job := build(w)
		for {
			if !until.IsZero() && !time.Now().Before(until) {
				return
			}
			t := int(next.Add(1) - 1)
			if t >= total {
				return
			}
			job(t)
		}
	}
	if len(pool) == 1 {
		work(pool[0])
		return
	}
	var wg sync.WaitGroup
	for _, w := range pool {
		wg.Add(1)
		go func(w *searchWorker) {
			defer wg.Done()
			work(w)
		}(w)
	}
	wg.Wait()
}

// future returns a function that plays one future of the decision: the
// listed action, then the playout policies, on a fork of the battle seeded
// with seed. It scores the end as the search does (the result or the judge's
// value, plus MarginWeight times the margin); a future that fails scores -2.
// Only errDeadline is returned as an error.
func (s *SearchPolicy) future(w *searchWorker, d *Decision, deadline time.Time) func(index int, seed uint64) (float64, error) {
	policies := map[string]Policy{}
	for id := range d.Battle.Actors {
		policies[id] = w.opponent
	}
	policies[d.Seat] = w.hero
	playout := Playout{Sim: w.sim, Hero: d.Seat, Policies: policies, Rounds: s.Rounds, Evaluate: s.evaluate, Judge: w.judge, Deadline: deadline}
	return func(index int, seed uint64) (float64, error) {
		b := Fork(d.Battle, d.Seat, seed)
		action := d.Actions[index]
		v, judged, expected, err := playout.Play(&b, &action, rand.New(rand.NewSource(int64(seed))))
		switch {
		case errors.Is(err, errDeadline):
			return 0, err
		case err != nil:
			return -2, nil
		case judged:
			return v + s.MarginWeight*expected, nil
		}
		return v + s.MarginWeight*margin(&b, d.Seat), nil
	}
}

// Describe summarizes a command for people.
func Describe(b *state.Battle, seat string, action command.Command) string {
	instances := b.Settled.Actors[seat].CardInstances
	switch action.Type {
	case command.TypePlanningReroll:
		var p command.PlanningRerollPayload
		_ = json.Unmarshal(action.Payload, &p)
		return fmt.Sprintf("reroll dice %v", p.RerollIndices)
	case command.TypePlanningAbility:
		var p command.PlanningAbilityPayload
		_ = json.Unmarshal(action.Payload, &p)
		if p.TierID != "" {
			return fmt.Sprintf("select %s (%s)", p.AbilityID, p.TierID)
		}
		return "select " + p.AbilityID
	case command.TypePlanningCards, command.TypeCommitInteraction:
		ids := CardIDs(action)
		if len(ids) == 1 {
			choice := CardChoice(action)
			name := instances[ids[0]].DefinitionID
			if choice.Label != "" {
				return fmt.Sprintf("%s: %s", name, choice.Label)
			}
			return name
		}
	}
	return string(action.Type)
}

func diceFaces(b *state.Battle, seat string) []int {
	var out []int
	for _, die := range b.Settled.Actors[seat].FinalDice {
		out = append(out, die.Face)
	}
	return out
}

// advise asks the Advisor about the decision's wide options (the solver's
// best 8 rerolls and die choices, best 4 ability choices, every other move):
// its pick becomes the anchor when it scores more than AdvisorMargin above
// the greedy pick, and the candidates are its AdvisorTop best options with
// both picks. If it fails, the search goes on without it.
func (s *SearchPolicy) advise(d *Decision, options []int, greedy int) ([]int, int, error) {
	wide, err := s.candidatesWith(d, options, greedy, 8, 4)
	if err != nil || len(wide) < 2 {
		return wide, greedy, err
	}
	scores, err := s.Advisor.Scores(d, wide, greedy)
	if err != nil || len(scores) != len(wide) {
		c, err := s.candidates(d, options, greedy)
		return c, greedy, err
	}
	order := make([]int, len(wide))
	at := 0
	for k := range order {
		order[k] = k
		if wide[k] == greedy {
			at = k
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	anchor := greedy
	if scores[order[0]]-scores[at] > s.AdvisorMargin {
		anchor = wide[order[0]]
		s.Advised++
	}
	keep := map[int]bool{anchor: true, greedy: true}
	for k := 0; k < len(order) && k < max(s.AdvisorTop, 1); k++ {
		keep[wide[order[k]]] = true
	}
	var out []int
	for i := range keep {
		out = append(out, i)
	}
	sort.Ints(out)
	return out, anchor, nil
}

// candidates keeps every option except the solver's weaker rerolls, die
// targets and ability choices, and always includes the greedy player's pick.
func (s *SearchPolicy) candidates(d *Decision, options []int, anchor int) ([]int, error) {
	topAbilities := s.TopAbilities
	if topAbilities <= 0 {
		topAbilities = 2
	}
	return s.candidatesWith(d, options, anchor, s.TopRerolls, topAbilities)
}

// candidatesWith is candidates with the solver's best topRerolls rerolls and
// die choices and best topAbilities ability choices.
func (s *SearchPolicy) candidatesWith(d *Decision, options []int, anchor int, topRerolls, topAbilities int) ([]int, error) {
	plan, err := s.Rollout.plan(d.Battle, d.Seat, nil)
	if err != nil {
		return nil, err
	}
	lib, err := library(d.Battle)
	if err != nil {
		return nil, err
	}
	rt := d.Battle.Settled.Actors[d.Seat]
	type scored struct {
		index int
		value float64
	}
	var rerolls, abilities, dice []scored
	var out []int
	for _, i := range options {
		action := d.Actions[i]
		switch {
		case plan != nil && action.Type == command.TypePlanningReroll:
			var payload command.PlanningRerollPayload
			if json.Unmarshal(action.Payload, &payload) == nil {
				rerolls = append(rerolls, scored{i, plan.Reroll(payload.RerollIndices)})
				continue
			}
		case action.Type == command.TypePlanningAbility && d.Battle.Segment.Current == "offensive":
			var payload command.PlanningAbilityPayload
			if json.Unmarshal(action.Payload, &payload) == nil {
				abilities = append(abilities, scored{i, abilityTierValue(lib, payload.AbilityID, payload.TierID, s.Rollout.Rates) + cardDamageBonus(rt.AbilityModifiers, payload.AbilityID) + s.Rollout.Bias[payload.AbilityID]})
				continue
			}
		case plan != nil && CardChoice(action).Kind == "offensive_die":
			choice := CardChoice(action)
			dice = append(dice, scored{i, dieChoiceValue(plan, choiceCard(d.Battle, d.Seat, action), choice)})
			continue
		}
		out = append(out, i)
	}
	top := func(list []scored, n int) {
		sort.SliceStable(list, func(a, b int) bool { return list[a].value > list[b].value })
		for k := 0; k < len(list) && k < n; k++ {
			out = append(out, list[k].index)
		}
	}
	top(rerolls, topRerolls)
	top(abilities, topAbilities)
	top(dice, topRerolls)
	found := false
	for _, i := range out {
		found = found || i == anchor
	}
	if !found {
		out = append(out, anchor)
	}
	sort.Ints(out)
	return out, nil
}

// evaluate scores a position at the playout horizon by the health race:
// each side's live cards as a fraction of its maximum.
func (s *SearchPolicy) evaluate(b *state.Battle, hero string) float64 {
	return math.Tanh(2 * margin(b, hero))
}

// margin is the hero's health fraction minus the strongest opponent's.
func margin(b *state.Battle, hero string) float64 {
	h := b.Actors[hero]
	heroFrac := float64(h.CurrentHealth()) / float64(max(MaxHealth(h), 1))
	foeFrac := 0.0
	for _, id := range Opponents(b, hero) {
		a := b.Actors[id]
		foeFrac = math.Max(foeFrac, float64(a.CurrentHealth())/float64(max(MaxHealth(a), 1)))
	}
	return heroFrac - foeFrac
}
