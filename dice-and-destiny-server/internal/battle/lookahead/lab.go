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

// PolicyFactory builds a fresh player for one worker (players cache state
// and are not shared between goroutines).
type PolicyFactory func() (Policy, error)

// Summary aggregates one player's battles.
type Summary struct {
	Player        string         `json:"player"`
	Battles       int            `json:"battles"`
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
	CardsPlayed   map[string]int `json:"cards_played"`
	Abilities     map[string]int `json:"abilities_selected"`
	Records       []BattleRecord `json:"records"`
}

// RunBattles plays seeds [first, first+count) for the hero policy against the
// foe policy on `workers` goroutines, each with its own runner.
func RunBattles(m Matchup, hero, foe PolicyFactory, first uint64, count, workers int, progress func(done int)) (Summary, error) {
	if workers < 1 {
		workers = 1
	}
	records := make([]BattleRecord, count)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	done, searched, overrides := 0, 0, 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner, err := NewRunner(m)
			var heroPolicy, foePolicy Policy
			if err == nil {
				heroPolicy, err = hero()
			}
			if err == nil {
				foePolicy, err = foe()
			}
			for i := range jobs {
				if err != nil {
					continue
				}
				var record BattleRecord
				record, err = runner.Play(first+uint64(i), heroPolicy, foePolicy)
				if err != nil {
					err = fmt.Errorf("seed %d: %w", first+uint64(i), err)
					continue
				}
				records[i] = record
				mu.Lock()
				done++
				if progress != nil {
					progress(done)
				}
				mu.Unlock()
			}
			mu.Lock()
			if c, ok := heroPolicy.(counter); ok {
				a, b := c.Counts()
				searched += a
				overrides += b
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}()
	}
	for i := 0; i < count; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return Summary{}, firstErr
	}
	summary := Summarize(records)
	summary.Searched, summary.Overrides = searched, overrides
	return summary, nil
}

// Summarize totals records; the win-rate interval is Wilson's 95% interval.
func Summarize(records []BattleRecord) Summary {
	s := Summary{Battles: len(records), CardsPlayed: map[string]int{}, Abilities: map[string]int{}, Records: records}
	decisions, think := 0, 0.0
	for _, r := range records {
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
		for k, v := range r.CardsPlayed {
			s.CardsPlayed[k] += v
		}
		for k, v := range r.Abilities {
			s.Abilities[k] += v
		}
	}
	n := float64(len(records))
	if n == 0 {
		return s
	}
	s.WinRate = float64(s.Wins) / n
	s.WinRateLow, s.WinRateHigh = wilson(s.Wins, len(records))
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

// Table renders summaries as a plain-text table.
func Table(summaries []Summary) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-12s %7s %5s %5s %5s %8s %15s %6s %7s %7s %8s %9s\n", "player", "battles", "win", "draw", "loss", "win %", "95% CI", "score", "rounds", "hp left", "foe hp", "ms/dec")
	for _, s := range summaries {
		fmt.Fprintf(&sb, "%-12s %7d %5d %5d %5d %7.1f%% %6.1f–%5.1f%% %6.3f %7.2f %7.2f %8.2f %9.2f\n",
			s.Player, s.Battles, s.Wins, s.Draws, s.Losses, 100*s.WinRate, 100*s.WinRateLow, 100*s.WinRateHigh, s.Score, s.MeanRounds, s.MeanHealth, s.MeanFoeHealth, s.ThinkMS)
	}
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
