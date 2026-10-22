// Command value-probe measures what riders are worth to the value network.
// It replays the held-out battles of a recorded dataset (the same seeds and
// greedy play, so the same positions) and, at the start of every round's
// offensive planning, changes one thing at a time on a copy of the battle,
// encodes the acting seat's view and reads the network's change in win
// chance. Each change divided by the change for one more card removed from
// the opponent (the damage unit) is its damage-equivalent.
//
// It never changes rates or content; the results are for people to weigh.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/battle/valuenet"
	"diceanddestiny/server/internal/content"
)

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards root (read only)")
	modelPath := flag.String("value-model", "", "value network file (value-v1)")
	data := flag.String("data", filepath.Join(serverRoot, "ml", "runs", "value-v1", "greedy-*"), "recorded datasets (glob); the probe replays those involving the model's held-out hero or opponent")
	first := flag.Uint64("first-seed", 100001, "first seed the datasets were recorded with")
	battles := flag.Int("battles", 350, "seeds per dataset (each played from both seats)")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel battles")
	threshold := flag.Float64("min-unit", 0.005, "per-position ratios use only positions where the damage unit moves the win chance at least this much")
	heldOut := flag.Bool("held-out", true, "replay the datasets involving the model's held-out hero or opponent; false replays the training datasets instead")
	only := flag.String("only", "", "replay only datasets whose directory name contains this text")
	out := flag.String("out", "", "optional JSON path for every probed position")
	paired := flag.Bool("paired", false, "measure riders without a network: identically seeded copies with and without each rider, played to the end by the greedy player")
	matchups := flag.String("matchups", "starter:starter:greedy,starter:drowned_oracle_brine_mask:script", "-paired: hero:opponent:foe (greedy or script), comma-separated")
	var pc pairedConfig
	flag.IntVar(&pc.futures, "futures", 8, "-paired: futures per position and change")
	flag.IntVar(&pc.every, "every", 1, "-paired: probe every n-th round start of the hero")
	flag.Parse()
	if *paired {
		runState, err := os.MkdirTemp("", "value-probe-run-state-")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(runState)
		runPaired(*contentRoot, *authoredRoot, runState, *matchups, *first, *battles, *workers, pc, *out)
		return
	}
	if *modelPath == "" {
		fail(fmt.Errorf("-value-model is required"))
	}
	model, err := valuenet.LoadFile(*modelPath)
	if err != nil {
		fail(err)
	}
	var meta struct {
		HeldOut struct{ Hero, Opponent string } `json:"held_out"`
	}
	raw, err := os.ReadFile(*modelPath)
	if err != nil {
		fail(err)
	}
	if err := json.Unmarshal(raw, &meta); err != nil || meta.HeldOut.Hero == "" {
		fail(fmt.Errorf("%s names no held-out hero and opponent", *modelPath))
	}
	held := map[string]bool{meta.HeldOut.Hero: true, meta.HeldOut.Opponent: true}
	dirs, _ := filepath.Glob(*data)
	runState, err := os.MkdirTemp("", "value-probe-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	collect := &collector{}
	fmt.Fprintf(os.Stderr, "model %s (sha256 %s), held out: %s and %s\n", *modelPath, model.SHA256, meta.HeldOut.Hero, meta.HeldOut.Opponent)
	for _, dir := range dirs {
		var header valuenet.DatasetHeader
		raw, err := os.ReadFile(filepath.Join(dir, "header.json"))
		if err != nil || json.Unmarshal(raw, &header) != nil || (held[header.Hero] || held[header.Opponent]) != *heldOut || !strings.Contains(filepath.Base(dir), *only) {
			continue
		}
		foe := func() (lookahead.Policy, error) {
			if header.Foe != "script" {
				return lookahead.NewGreedyPolicy(), nil
			}
			minion, err := learned.LoadScriptedMinion(*contentRoot, header.Opponent)
			if err != nil {
				return nil, err
			}
			return lookahead.Scripted{Label: header.Opponent, Policy: minion}, nil
		}
		m := lookahead.Matchup{ContentRoot: *contentRoot, AuthoredRoot: *authoredRoot, RunStateRoot: runState, Hero: header.Hero, Opponent: header.Opponent, Unified: true, HeroPlansFirst: true,
			Observe: func() lookahead.Observer { return newProber(model, collect, header.Hero+" vs "+header.Opponent) }}
		greedy := func() (lookahead.Policy, error) { return lookahead.NewGreedyPolicy(), nil }
		if _, err := lookahead.RunInterleaved(m, []lookahead.Entrant{{Name: "greedy", Hero: greedy}}, foe, []bool{false, true}, *first, *battles, *workers, nil); err != nil {
			fail(fmt.Errorf("%s: %w", dir, err))
		}
		fmt.Fprintf(os.Stderr, "%s vs %s: %d positions so far\n", header.Hero, header.Opponent, len(collect.samples))
	}
	if len(collect.samples) == 0 {
		fail(fmt.Errorf("no matching datasets under %s", *data))
	}
	fmt.Print(report(collect.samples, *threshold))
	if *out != "" {
		raw, err := json.Marshal(collect.samples)
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(*out, raw, 0o644); err != nil {
			fail(err)
		}
	}
}

// sample is one probed position: the base win chance, the damage unit's
// change and each other change's.
type sample struct {
	Matchup string             `json:"matchup"`
	Round   int                `json:"round"`
	Health  float64            `json:"own_health_share"`
	Base    float64            `json:"base_win"`
	Unit    float64            `json:"unit_delta"`
	Deltas  map[string]float64 `json:"deltas"`
}

type collector struct {
	mu      sync.Mutex
	samples []sample
}

// prober probes positions for one runner.
type prober struct {
	eval    *valuenet.Evaluator
	enc     *valuenet.Encoder
	engine  engine.Engine
	collect *collector
	matchup string
	round   int
	input   []float32
}

func newProber(model *valuenet.Model, collect *collector, matchup string) lookahead.Observer {
	eng, err := engine.NewEngineWithConfig(engine.Config{OmitSnapshotContentCatalog: true}, engine.DefaultFlows()...)
	if err != nil {
		fail(err)
	}
	return &prober{eval: model.NewEvaluator(), enc: valuenet.NewEncoder(), engine: eng, collect: collect, matchup: matchup, input: make([]float32, valuenet.Width())}
}

func (p *prober) Finish(*state.Battle, lookahead.BattleRecord, lookahead.Policy, lookahead.Policy) {
	p.round = 0
}

func (p *prober) Decision(d *lookahead.Decision) {
	b := d.Battle
	if b.Segment.Round == p.round || !lookahead.PlanningStart(b) {
		return
	}
	p.round = b.Segment.Round
	seat := d.Seat
	foes := lookahead.Opponents(b, seat)
	if len(foes) == 0 {
		return
	}
	foe := foes[0]
	lib, err := engine.SettledLibrary(b)
	if err != nil {
		fail(err)
	}
	// Every change is made on a fork, which reseeds the randomness the
	// copy could otherwise read (a drawn card is a fresh draw).
	seed := b.Random.Seed ^ uint64(b.Segment.Round)*0x9e3779b97f4a7c15 ^ uint64(len(seat))
	win := func(change func(*state.Battle) bool) (float64, bool) {
		c := lookahead.Fork(b, seat, seed)
		if change != nil && !change(&c) {
			return 0, false
		}
		pos, err := valuenet.NewPosition(&c, seat, seat)
		if err != nil {
			fail(err)
		}
		if err := p.enc.Encode(pos, p.input); err != nil {
			fail(err)
		}
		w, _ := p.eval.Predict(p.input)
		return w, true
	}
	base, _ := win(nil)
	own := b.Actors[seat]
	s := sample{Matchup: p.matchup, Round: b.Segment.Round, Base: base, Deltas: map[string]float64{},
		Health: float64(own.CurrentHealth()) / float64(max(lookahead.MaxHealth(own), 1))}
	unit, ok := win(func(c *state.Battle) bool { return removeOne(c, foe, rand.New(rand.NewSource(int64(seed)))) })
	if !ok {
		return
	}
	s.Unit = unit - base
	try := func(name string, change func(*state.Battle) bool) {
		if v, ok := win(change); ok {
			s.Deltas[name] = v - base
		}
	}
	try("+5 energy", func(c *state.Battle) bool { engine.GainEnergy(c, seat, 5); return true })
	try("+1 card in hand (from the draw pile)", func(c *state.Battle) bool {
		drawn, err := p.engine.DrawCard(c, seat)
		return err == nil && drawn != ""
	})
	markers := preparationMarkers(lib)
	ids := make([]string, 0, len(lib.Statuses))
	for id := range lib.Statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		def := lib.Statuses[id]
		if def.Modifiers.MapOnly() || markers[id] {
			continue
		}
		target, label := foe, "enemy +1 "+def.Name
		if def.Polarity == "positive" {
			target, label = seat, "self +1 "+def.Name
		}
		try(label+" ("+id+")", func(c *state.Battle) bool { return engine.ApplyStatus(c, target, id, 1) == nil })
	}
	p.collect.mu.Lock()
	p.collect.samples = append(p.collect.samples, s)
	p.collect.mu.Unlock()
}

// removeOne removes one of the actor's cards the way ordinary damage picks
// them: the discard pile first, then the draw pile, then the hand, at random
// within the pile. During private planning others see the counts the
// planning started with, so those move too.
func removeOne(b *state.Battle, actorID string, rng *rand.Rand) bool {
	actor := b.Actors[actorID]
	public, private := b.Settled.PlanningPublic[actorID]
	counts := []*int{&public.DiscardCount, &public.DeckCount, &public.HandCount}
	for k, pile := range []*[]string{&actor.Cards.Discard, &actor.Cards.Deck, &actor.Cards.Hand} {
		if len(*pile) == 0 {
			continue
		}
		i := rng.Intn(len(*pile))
		card := (*pile)[i]
		*pile = append(append([]string(nil), (*pile)[:i]...), (*pile)[i+1:]...)
		actor.Cards.Removed = append(append([]string(nil), actor.Cards.Removed...), card)
		b.Actors[actorID] = actor
		if private {
			*counts[k] = max(0, *counts[k]-1)
			public.RemovedCount++
			b.Settled.PlanningPublic[actorID] = public
		}
		return true
	}
	return false
}

// preparationMarkers are the statuses cards use to mark a prepared effect
// (an ability bonus's preparation, a configured mechanic's work). A stack
// without its card's effect cannot occur, so the probe leaves them out.
func preparationMarkers(lib content.BattleLibrary) map[string]bool {
	out := map[string]bool{}
	var visit func(card string, steps []content.CardStep)
	visit = func(card string, steps []content.CardStep) {
		for _, step := range steps {
			if step.Effect == "ability_bonus" {
				out[content.ProgramPreparationStatus(card, step)] = true
			}
			for _, option := range step.Choices {
				visit(card, option.Steps)
			}
		}
	}
	for id, card := range lib.Cards {
		if card.Program != nil {
			visit(id, card.Program.Steps)
		}
		if card.Mechanic != nil {
			out[content.MechanicStatusDefinitionID(card)] = true
		}
	}
	return out
}

// pricing are the board pricing's exchange rates agreed 2026-10-15, in
// damage, for the changes they cover.
var pricing = map[string]string{"+5 energy": "1.0 (0.2 per point)", "self +1 Protect Status (protect)": "1.0"}

type stats struct {
	n                       int
	mean, median, q1, q3    float64
	ratioOfMeans, meanDelta float64
}

func summarize(samples []sample, name string, threshold float64) stats {
	var ratios []float64
	var sumDelta, sumUnit float64
	n := 0
	for _, s := range samples {
		delta, ok := s.Deltas[name]
		if name == "unit" {
			delta, ok = s.Unit, true
		}
		if !ok {
			continue
		}
		n++
		sumDelta += delta
		sumUnit += s.Unit
		if s.Unit >= threshold {
			ratios = append(ratios, delta/s.Unit)
		}
	}
	st := stats{n: n}
	if n == 0 {
		return st
	}
	st.meanDelta = sumDelta / float64(n)
	if sumUnit != 0 {
		st.ratioOfMeans = sumDelta / sumUnit
	}
	if len(ratios) > 0 {
		sort.Float64s(ratios)
		for _, r := range ratios {
			st.mean += r
		}
		st.mean /= float64(len(ratios))
		st.median, st.q1, st.q3 = quantile(ratios, 0.5), quantile(ratios, 0.25), quantile(ratios, 0.75)
	}
	return st
}

func quantile(sorted []float64, q float64) float64 {
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := min(lo+1, len(sorted)-1)
	return sorted[lo] + (pos-float64(lo))*(sorted[hi]-sorted[lo])
}

func report(samples []sample, threshold float64) string {
	names := map[string]bool{}
	for _, s := range samples {
		for k := range s.Deltas {
			names[k] = true
		}
	}
	ordered := []string{"+5 energy", "+1 card in hand (from the draw pile)"}
	var statuses []string
	for k := range names {
		if k != ordered[0] && k != ordered[1] {
			statuses = append(statuses, k)
		}
	}
	sort.Strings(statuses)
	ordered = append(ordered, statuses...)
	var sb strings.Builder
	unit := summarize(samples, "unit", threshold)
	fmt.Fprintf(&sb, "%d positions; one more card removed from the opponent moves the win chance by %+.4f on average.\n", len(samples), unit.meanDelta)
	fmt.Fprintf(&sb, "Per-position ratios use the positions where that unit is at least %.3f.\n\n", threshold)
	fmt.Fprintf(&sb, "| Change | Positions | Mean | Median | IQR | Ratio of means | 2026-10-15 rate |\n|---|---|---|---|---|---|---|\n")
	for _, name := range ordered {
		st := summarize(samples, name, threshold)
		if st.n == 0 {
			continue
		}
		rate := pricing[name]
		if rate == "" {
			rate = "—"
		}
		fmt.Fprintf(&sb, "| %s | %d | %.2f | %.2f | %.2f … %.2f | %.2f | %s |\n", name, st.n, st.mean, st.median, st.q1, st.q3, st.ratioOfMeans, rate)
	}
	type split struct {
		label string
		keep  func(sample) bool
	}
	splits := []split{
		{"rounds 1–2", func(s sample) bool { return s.Round <= 2 }},
		{"rounds 3–5", func(s sample) bool { return s.Round >= 3 && s.Round <= 5 }},
		{"rounds 6+", func(s sample) bool { return s.Round >= 6 }},
		{"health ≥ half", func(s sample) bool { return s.Health >= 0.5 }},
		{"health < half", func(s sample) bool { return s.Health < 0.5 }},
	}
	fmt.Fprintf(&sb, "\nMedian damage-equivalent (positions) by round and own health:\n\n| Change |")
	for _, sp := range splits {
		fmt.Fprintf(&sb, " %s |", sp.label)
	}
	fmt.Fprintf(&sb, "\n|---|%s\n", strings.Repeat("---|", len(splits)))
	parts := make([][]sample, len(splits))
	for i, sp := range splits {
		for _, s := range samples {
			if sp.keep(s) {
				parts[i] = append(parts[i], s)
			}
		}
	}
	for _, name := range append([]string{"unit"}, ordered...) {
		label := name
		if name == "unit" {
			label = "unit: win-chance change per card removed (mean)"
		}
		fmt.Fprintf(&sb, "| %s |", label)
		for i := range splits {
			st := summarize(parts[i], name, threshold)
			switch {
			case st.n == 0:
				fmt.Fprintf(&sb, " — |")
			case name == "unit":
				fmt.Fprintf(&sb, " %+.4f (%d) |", st.meanDelta, st.n)
			default:
				fmt.Fprintf(&sb, " %.2f (%d) |", st.median, st.n)
			}
		}
		fmt.Fprintln(&sb)
	}
	return sb.String()
}

func defaultServerRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
