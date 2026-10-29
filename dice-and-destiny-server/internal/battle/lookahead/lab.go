package lookahead

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// counter is implemented by players that report search statistics.
type counter interface {
	Counts() (searched, overrides int)
}

// futureCounter is implemented by players that report the futures they
// played.
type futureCounter interface {
	FutureCount() int
}

// PolicyFactory builds a fresh player for one worker (players cache state
// and are not shared between goroutines).
type PolicyFactory func() (Policy, error)

// Summary aggregates one player's battles. Settings describes how the player
// was configured, for reproducing it.
type Summary struct {
	Player   string `json:"player"`
	Settings string `json:"settings,omitempty"`
	Battles  int    `json:"battles"`
	// Errors counts battles the authority broke off by refusing a command it
	// had listed (left out of every score).
	Errors        int            `json:"errors,omitempty"`
	Wins          int            `json:"wins"`
	Draws         int            `json:"draws"`
	Losses        int            `json:"losses"`
	WinRate       float64        `json:"win_rate"`
	WinRateLow    float64        `json:"win_rate_ci95_low"`
	WinRateHigh   float64        `json:"win_rate_ci95_high"`
	Score         float64        `json:"score"`
	MeanRounds    float64        `json:"mean_rounds"`
	MeanHealth    float64        `json:"mean_hero_health_left"`
	MeanFoeHealth float64        `json:"mean_foe_health_left"`
	Decisions     float64        `json:"mean_hero_decisions"`
	ThinkMS       float64        `json:"mean_ms_per_decision"`
	Searched      int            `json:"searched_decisions,omitempty"`
	Overrides     int            `json:"overrides,omitempty"`
	Futures       int            `json:"futures,omitempty"`
	Advised       int            `json:"advised,omitempty"`
	CardsPlayed   map[string]int `json:"cards_played"`
	Abilities     map[string]int `json:"abilities_selected"`
	Records       []BattleRecord `json:"records"`
}

// RunBattles plays seeds [first, first+count) for the hero policy against the
// foe policy on `workers` goroutines, each with its own runner.
func RunBattles(m Matchup, hero, foe PolicyFactory, first uint64, count, workers int, progress func(done int)) (Summary, error) {
	var report func(done, total int)
	if progress != nil {
		report = func(done, _ int) { progress(done) }
	}
	results, err := RunInterleaved(m, []Entrant{{Hero: hero}}, foe, []bool{m.HeroInSeatB}, first, count, workers, report)
	if err != nil {
		return Summary{}, err
	}
	return Summarize(results[0]), nil
}

// Entrant is one hero player in an interleaved run.
type Entrant struct {
	Name string
	Hero PolicyFactory
}

// RunInterleaved plays seeds [first, first+count) from each listed hero seat
// (false: seat A) for every entrant against the foe, on `workers`
// goroutines. Battles are dealt seed by seed, seat by seat and entrant by
// entrant, so time-limited players share the machine's load evenly. It
// returns each entrant's records, seat by seat in seed order.
func RunInterleaved(m Matchup, entrants []Entrant, foe PolicyFactory, seatsB []bool, first uint64, count, workers int, progress func(done, total int)) ([][]BattleRecord, error) {
	if workers < 1 {
		workers = 1
	}
	type job struct{ entrant, slot int }
	results := make([][]BattleRecord, len(entrants))
	for e := range results {
		results[e] = make([]BattleRecord, count*len(seatsB))
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	done, total := 0, count*len(seatsB)*len(entrants)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner, err := NewRunner(m)
			var foePolicy Policy
			if err == nil {
				foePolicy, err = foe()
			}
			heroes := make([]Policy, len(entrants))
			for jb := range jobs {
				if err != nil {
					continue
				}
				hero := heroes[jb.entrant]
				if hero == nil {
					if hero, err = entrants[jb.entrant].Hero(); err != nil {
						continue
					}
					heroes[jb.entrant] = hero
				}
				seed := first + uint64(jb.slot%count)
				runner.matchup.HeroInSeatB = seatsB[jb.slot/count]
				before := searchCounts(hero)
				var record BattleRecord
				record, err = runner.Play(seed, hero, foePolicy)
				if err != nil && strings.Contains(err.Error(), "rejected enumerated action") {
					// The authority refused a command it listed as legal: an
					// engine fault, not the player's. The battle is kept as an
					// error, left out of every score and comparison, and
					// reported.
					record = BattleRecord{Seed: seed, Seat: runner.heroSeat(), Result: "error", Error: err.Error()}
					err = nil
				}
				if err != nil {
					err = fmt.Errorf("%s seed %d: %w", entrants[jb.entrant].Name, seed, err)
					continue
				}
				after := searchCounts(hero)
				record.Searched, record.Overrides, record.Futures, record.Advised = after[0]-before[0], after[1]-before[1], after[2]-before[2], after[3]-before[3]
				results[jb.entrant][jb.slot] = record
				mu.Lock()
				done++
				if progress != nil {
					progress(done, total)
				}
				mu.Unlock()
			}
			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}()
	}
	for i := 0; i < count; i++ {
		for seat := range seatsB {
			for e := range entrants {
				jobs <- job{entrant: e, slot: seat*count + i}
			}
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// searchCounts reads a player's running search statistics: searched
// decisions, overrides and futures.
func searchCounts(p Policy) [4]int {
	var out [4]int
	if c, ok := p.(counter); ok {
		out[0], out[1] = c.Counts()
	}
	if c, ok := p.(futureCounter); ok {
		out[2] = c.FutureCount()
	}
	if c, ok := p.(advisedCounter); ok {
		out[3] = c.AdvisedCount()
	}
	return out
}

// advisedCounter is implemented by players whose network can change their
// default move.
type advisedCounter interface {
	AdvisedCount() int
}

// Summarize totals records; the win-rate interval is Wilson's 95% interval.
func Summarize(records []BattleRecord) Summary {
	s := Summary{CardsPlayed: map[string]int{}, Abilities: map[string]int{}, Records: records}
	decisions, think := 0, 0.0
	var valid []BattleRecord
	for _, r := range records {
		if r.Result == "error" {
			s.Errors++
			continue
		}
		valid = append(valid, r)
	}
	s.Battles = len(valid)
	for _, r := range valid {
		switch r.Result {
		case "win":
			s.Wins++
		case "draw":
			s.Draws++
		default:
			s.Losses++
		}
		s.MeanRounds += float64(r.Rounds)
		s.MeanHealth += float64(r.HeroHealth)
		s.MeanFoeHealth += float64(r.FoeHealth)
		decisions += r.HeroDecisions
		think += r.HeroThinkMS
		s.Searched += r.Searched
		s.Overrides += r.Overrides
		s.Futures += r.Futures
		s.Advised += r.Advised
		for k, v := range r.CardsPlayed {
			s.CardsPlayed[k] += v
		}
		for k, v := range r.Abilities {
			s.Abilities[k] += v
		}
	}
	n := float64(len(valid))
	if n == 0 {
		return s
	}
	s.WinRate = float64(s.Wins) / n
	s.WinRateLow, s.WinRateHigh = wilson(s.Wins, len(valid))
	s.Score = (float64(s.Wins) + 0.5*float64(s.Draws)) / n
	s.MeanRounds /= n
	s.MeanHealth /= n
	s.MeanFoeHealth /= n
	s.Decisions = float64(decisions) / n
	if decisions > 0 {
		s.ThinkMS = think / float64(decisions)
	}
	return s
}

func wilson(successes, n int) (float64, float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.96
	p := float64(successes) / float64(n)
	nf := float64(n)
	center := (p + z*z/(2*nf)) / (1 + z*z/nf)
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / (1 + z*z/nf)
	return center - half, center + half
}

// Table renders summaries as a plain-text table. "fut/srch" is the mean
// number of futures a searched decision played.
func Table(summaries []Summary) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-12s %7s %5s %5s %5s %8s %15s %6s %7s %7s %8s %9s %8s\n", "player", "battles", "win", "draw", "loss", "win %", "95% CI", "score", "rounds", "hp left", "foe hp", "ms/dec", "fut/srch")
	for _, s := range summaries {
		futures := ""
		if s.Searched > 0 {
			futures = fmt.Sprintf("%.0f", float64(s.Futures)/float64(s.Searched))
		}
		fmt.Fprintf(&sb, "%-12s %7d %5d %5d %5d %7.1f%% %6.1f–%5.1f%% %6.3f %7.2f %7.2f %8.2f %9.2f %8s\n",
			s.Player, s.Battles, s.Wins, s.Draws, s.Losses, 100*s.WinRate, 100*s.WinRateLow, 100*s.WinRateHigh, s.Score, s.MeanRounds, s.MeanHealth, s.MeanFoeHealth, s.ThinkMS, futures)
	}
	return sb.String()
}

// RecordScore scores one battle for the hero: win 1, draw 0.5, loss 0. A
// truncated battle counts as a draw.
func RecordScore(r BattleRecord) float64 {
	switch r.Result {
	case "win":
		return 1
	case "loss":
		return 0
	}
	return 0.5
}

// Comparison pairs two players' battles on the same seed and hero seat.
// Diff is A's score minus B's, per battle.
type Comparison struct {
	A, B           string
	Pairs          int
	Unpaired       int
	Errors         int // pairs left out because either battle was an error
	ScoreA, ScoreB float64
	A3, B3         [3]int // wins, draws, losses
	Better, Worse  int    // battles where A scored more / less than B
	Diff, Low      float64
	High, Z        float64
}

// Compare pairs a's and b's records by seed and hero seat and reports the
// mean score difference with its 95% interval and z.
func Compare(a, b Summary) (Comparison, error) {
	type key struct {
		seed uint64
		seat string
	}
	other := map[key]BattleRecord{}
	for _, r := range b.Records {
		other[key{r.Seed, r.Seat}] = r
	}
	c := Comparison{A: a.Player, B: b.Player}
	var kept []BattleRecord
	for _, ra := range a.Records {
		rb, ok := other[key{ra.Seed, ra.Seat}]
		if ra.Result == "error" || (ok && rb.Result == "error") {
			c.Errors++
			delete(other, key{ra.Seed, ra.Seat})
			continue
		}
		kept = append(kept, ra)
	}
	a.Records = kept
	var diffs []float64
	tally := func(t *[3]int, r BattleRecord) {
		switch RecordScore(r) {
		case 1:
			t[0]++
		case 0:
			t[2]++
		default:
			t[1]++
		}
	}
	for _, ra := range a.Records {
		rb, ok := other[key{ra.Seed, ra.Seat}]
		if !ok {
			c.Unpaired++
			continue
		}
		delete(other, key{ra.Seed, ra.Seat})
		sa, sb := RecordScore(ra), RecordScore(rb)
		c.ScoreA += sa
		c.ScoreB += sb
		tally(&c.A3, ra)
		tally(&c.B3, rb)
		switch {
		case sa > sb:
			c.Better++
		case sa < sb:
			c.Worse++
		}
		diffs = append(diffs, sa-sb)
	}
	c.Unpaired += len(other)
	c.Pairs = len(diffs)
	if c.Pairs < 2 {
		return c, fmt.Errorf("only %d paired battles between %s and %s", c.Pairs, a.Player, b.Player)
	}
	n := float64(c.Pairs)
	c.ScoreA /= n
	c.ScoreB /= n
	var sq float64
	for _, d := range diffs {
		c.Diff += d
	}
	c.Diff /= n
	for _, d := range diffs {
		sq += (d - c.Diff) * (d - c.Diff)
	}
	se := math.Sqrt(sq / (n - 1) / n)
	c.Low, c.High = c.Diff-1.96*se, c.Diff+1.96*se
	if se > 0 {
		c.Z = c.Diff / se
	}
	return c, nil
}

// String renders a comparison for people.
func (c Comparison) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "A %s: score %.3f (win/draw/loss %d/%d/%d)\n", c.A, c.ScoreA, c.A3[0], c.A3[1], c.A3[2])
	fmt.Fprintf(&sb, "B %s: score %.3f (win/draw/loss %d/%d/%d)\n", c.B, c.ScoreB, c.B3[0], c.B3[1], c.B3[2])
	fmt.Fprintf(&sb, "%d paired battles (%d unpaired", c.Pairs, c.Unpaired)
	if c.Errors > 0 {
		fmt.Fprintf(&sb, ", %d left out: authority errors", c.Errors)
	}
	fmt.Fprintf(&sb, "); A better on %d, worse on %d\n", c.Better, c.Worse)
	fmt.Fprintf(&sb, "A − B: %+.4f (95%% CI %+.4f … %+.4f), z = %.2f\n", c.Diff, c.Low, c.High, c.Z)
	return sb.String()
}

// CardsLine lists cards played per battle, most played first.
func CardsLine(s Summary) string {
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range s.CardsPlayed {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v || list[i].v == list[j].v && list[i].k < list[j].k })
	parts := []string{}
	for _, e := range list {
		parts = append(parts, fmt.Sprintf("%s %.2f", e.k, float64(e.v)/float64(max(s.Battles, 1))))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
