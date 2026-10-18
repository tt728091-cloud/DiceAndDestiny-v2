// Command balance-lab plays real authority battles between a hero and an
// opponent with several players on the same seeds and prints their win
// rates. It never opens a network listener or writes outside -out.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/lookahead"
)

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards root (read only)")
	hero := flag.String("hero", "starter", "hero combatant definition")
	opponent := flag.String("opponent", "drowned_oracle_brine_mask", "opponent combatant definition")
	foePlayer := flag.String("foe", "script", "opponent player: script (a minion's own script), greedy, greedy-v1, proxy or random")
	players := flag.String("players", "proxy,greedy,lookahead", "comma-separated players: random, proxy, greedy, greedy-v1, lookahead, lookahead-v1")
	seats := flag.String("seats", "a", "hero seat: a, b, or both (each seed played from each seat)")
	battles := flag.Int("battles", 200, "battles per player")
	first := flag.Uint64("first-seed", 1, "first battle seed")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel battles")
	budget := flag.Int("budget", 96, "look-ahead simulated futures per decision")
	rounds := flag.Int("rounds", 0, "look-ahead horizon in rounds (0 plays each future to the end)")
	out := flag.String("out", "", "optional JSON results path")
	trace := flag.Bool("trace", false, "print every look-ahead override")
	bias := flag.String("bias", "", "experiment: favour abilities in the greedy player's choices, e.g. measured_strike=2")
	energyValue := flag.Float64("energy-value", 0.1, "damage one energy point is worth to the greedy player (and the look-ahead's simulated play)")
	cardValue := flag.Float64("card-value", 0.25, "damage holding a card for later is worth to the current greedy player")
	drawValue := flag.Float64("draw-value", 0.25, "damage one drawn card is worth to the greedy player")
	tally := flag.String("tally", "", "instead of simulating, score your games against the look-ahead mirror from this battle log (\"default\" reads this workspace's log)")
	flag.Parse()
	if *tally != "" {
		path := *tally
		if path == "default" {
			path = filepath.Join(serverRoot, "..", "dice-and-destiny-client", ".godot", "runtime", "workspace", "client", "user", "phase3", "learned-battles.jsonl")
		}
		if err := printTally(path); err != nil {
			fail(err)
		}
		return
	}

	debug.SetGCPercent(400)
	runState, err := os.MkdirTemp("", "balance-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	matchup := lookahead.Matchup{ContentRoot: *contentRoot, AuthoredRoot: *authoredRoot, RunStateRoot: runState, Hero: *hero, Opponent: *opponent, Unified: true, HeroPlansFirst: true}
	foe := func() (lookahead.Policy, error) {
		switch *foePlayer {
		case "greedy":
			return lookahead.NewGreedyPolicy(), nil
		case "greedy-v1":
			return lookahead.NewGreedyPolicyV1(), nil
		case "proxy":
			return lookahead.ProxyPolicy{}, nil
		case "random":
			return lookahead.RandomPolicy{}, nil
		case "script":
			minion, err := learned.LoadScriptedMinion(*contentRoot, *opponent)
			if err != nil {
				return nil, err
			}
			return lookahead.Scripted{Label: *opponent, Policy: minion}, nil
		}
		return nil, fmt.Errorf("unknown opponent player %q", *foePlayer)
	}
	seatList := map[string][]bool{"a": {false}, "b": {true}, "both": {false, true}}[*seats]
	if seatList == nil {
		fail(fmt.Errorf("unknown -seats %q", *seats))
	}
	fmt.Printf("%s vs %s (%s) · unified defense · seeds %d–%d · hero seat %s · %d workers\n\n", *hero, *opponent, *foePlayer, *first, *first+uint64(*battles)-1, *seats, *workers)
	// A hero opponent is modelled by the look-ahead the way the game does: by
	// the greedy player of the look-ahead's own version.
	heroFoe := *foePlayer != "script"
	var summaries []lookahead.Summary
	for _, name := range strings.Split(*players, ",") {
		name = strings.TrimSpace(name)
		factory, err := playerFactory(name, foe, heroFoe, *budget, *rounds, *trace, *energyValue, *cardValue, *drawValue, parseBias(*bias))
		if err != nil {
			fail(err)
		}
		started := time.Now()
		var records []lookahead.BattleRecord
		searched, overrides := 0, 0
		for _, seatB := range seatList {
			m := matchup
			m.HeroInSeatB = seatB
			summary, err := lookahead.RunBattles(m, factory, foe, *first, *battles, *workers, func(done int) {
				if done%25 == 0 || done == *battles {
					fmt.Fprintf(os.Stderr, "\r%-12s %d/%d battles", name, done, *battles)
				}
			})
			fmt.Fprintln(os.Stderr)
			if err != nil {
				fail(fmt.Errorf("%s: %w", name, err))
			}
			records = append(records, summary.Records...)
			searched, overrides = searched+summary.Searched, overrides+summary.Overrides
			if len(seatList) > 1 {
				summary.Player = fmt.Sprintf("%s (seat %s)", name, map[bool]string{false: "A", true: "B"}[seatB])
				summaries = append(summaries, summary)
			}
		}
		summary := lookahead.Summarize(records)
		summary.Player, summary.Searched, summary.Overrides = name, searched, overrides
		summaries = append(summaries, summary)
		fmt.Fprintf(os.Stderr, "%-12s finished in %s\n", name, time.Since(started).Round(time.Second))
	}
	fmt.Println(lookahead.Table(summaries))
	for _, s := range summaries {
		fmt.Printf("%-10s cards per battle: %s\n", s.Player, lookahead.CardsLine(s))
		if s.Searched > 0 {
			fmt.Printf("%-10s searched %d decisions, overrode the greedy choice %d times (%.1f%%)\n", s.Player, s.Searched, s.Overrides, 100*float64(s.Overrides)/float64(s.Searched))
		}
	}
	if *out != "" {
		raw, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(*out, raw, 0o644); err != nil {
			fail(err)
		}
	}
}

func playerFactory(name string, foe lookahead.PolicyFactory, heroFoe bool, budget, rounds int, trace bool, energyValue, cardValue, drawValue float64, bias map[string]float64) (lookahead.PolicyFactory, error) {
	greedy := func(v1 bool) *lookahead.GreedyPolicy {
		g := lookahead.NewGreedyPolicy()
		if v1 {
			g = lookahead.NewGreedyPolicyV1()
		} else {
			g.CardValue = cardValue
		}
		g.EnergyValue, g.DrawValue, g.Bias = energyValue, drawValue, bias
		return g
	}
	switch name {
	case "random":
		return func() (lookahead.Policy, error) { return lookahead.RandomPolicy{}, nil }, nil
	case "proxy":
		return func() (lookahead.Policy, error) { return lookahead.ProxyPolicy{}, nil }, nil
	case "greedy", "greedy-v1":
		v1 := name == "greedy-v1"
		return func() (lookahead.Policy, error) {
			g := greedy(v1)
			if v1 {
				g.Label = "greedy-v1"
			}
			return g, nil
		}, nil
	case "lookahead", "lookahead-v1":
		v1 := name == "lookahead-v1"
		return func() (lookahead.Policy, error) {
			// The look-ahead models a minion with its own script and a hero
			// with the greedy player of its own version (a fresh instance per
			// worker: players cache state).
			model := func() lookahead.Policy {
				if heroFoe {
					return greedy(v1)
				}
				m, _ := foe()
				return m
			}
			if _, err := foe(); err != nil {
				return nil, err
			}
			search := lookahead.NewSearchPolicy(model, budget)
			if v1 {
				search = lookahead.NewSearchPolicyV1(model, budget)
			}
			search.Rollout = greedy(v1)
			search.Rounds = rounds
			if trace {
				search.Trace = func(line string) { fmt.Fprintln(os.Stderr, line) }
			}
			return search, nil
		}, nil
	}
	return nil, fmt.Errorf("unknown player %q", name)
}

// printTally scores the human's finished battles against the look-ahead
// mirror, per character and seat, from the game's battle log.
func printTally(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	type key struct{ version, character, seat string }
	type score struct{ win, draw, loss int }
	scores := map[key]*score{}
	var order []key
	for _, line := range strings.Split(string(raw), "\n") {
		var record struct {
			Kind    string `json:"kind"`
			Payload struct {
				Model struct {
					ModelID      string `json:"model_id"`
					Architecture string `json:"architecture"`
				} `json:"model"`
				HumanSeat string `json:"human_seat"`
				Result    string `json:"result"`
				Character string `json:"character"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.Kind != "battle_completed" || record.Payload.Model.ModelID != learned.LookaheadModelID {
			continue
		}
		version := "v1"
		if record.Payload.Model.Architecture == learned.LookaheadArchitecture {
			version = "v2"
		}
		k := key{version, record.Payload.Character, record.Payload.HumanSeat}
		if scores[k] == nil {
			scores[k] = &score{}
			order = append(order, k)
		}
		switch record.Payload.Result {
		case "victory":
			scores[k].win++
		case "draw":
			scores[k].draw++
		default:
			scores[k].loss++
		}
	}
	if len(order) == 0 {
		fmt.Println("No finished battles against the look-ahead mirror yet.")
		return nil
	}
	total := score{}
	fmt.Printf("%-4s %-14s %-7s %5s %5s %5s\n", "AI", "character", "seat", "you", "draw", "AI")
	for _, k := range order {
		s := scores[k]
		fmt.Printf("%-4s %-14s %-7s %5d %5d %5d\n", k.version, k.character, k.seat, s.win, s.draw, s.loss)
		total.win, total.draw, total.loss = total.win+s.win, total.draw+s.draw, total.loss+s.loss
	}
	n := total.win + total.draw + total.loss
	you := (float64(total.win) + 0.5*float64(total.draw)) / float64(n)
	fmt.Printf("\nTotal: you %d, draws %d, AI %d over %d games · your score %.0f%%", total.win, total.draw, total.loss, n, 100*you)
	// Rough 95% margin for a share of n games near 50%.
	fmt.Printf(" (±%.0f points at 95%%)\n", 100*1.96*0.5/math.Sqrt(float64(n)))
	return nil
}

// parseBias reads "ability=value,ability=value".
func parseBias(text string) map[string]float64 {
	out := map[string]float64{}
	for _, part := range strings.Split(text, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(value, "%g", &v); err == nil {
			out[name] = v
		}
	}
	return out
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
