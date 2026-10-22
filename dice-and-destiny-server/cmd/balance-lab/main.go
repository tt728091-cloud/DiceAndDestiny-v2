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
	"diceanddestiny/server/internal/battle/valuenet"
)

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards root (read only)")
	hero := flag.String("hero", "starter", "hero combatant definition")
	opponent := flag.String("opponent", "drowned_oracle_brine_mask", "opponent combatant definition")
	foePlayer := flag.String("foe", "script", "opponent player: script (a minion's own script), greedy, greedy-v1, proxy or random")
	players := flag.String("players", "proxy,greedy,lookahead", "comma-separated players: random, proxy, greedy, greedy-v1, lookahead, lookahead-v1; a player may override player flags, e.g. lookahead@rounds=1@think-ms=50")
	seats := flag.String("seats", "a", "hero seat: a, b, or both (each seed played from each seat)")
	battles := flag.Int("battles", 200, "battles per player")
	first := flag.Uint64("first-seed", 1, "first battle seed")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel battles")
	out := flag.String("out", "", "optional JSON results path")
	record := flag.String("record", "", "record value-v1 training positions of every battle into this new directory (e.g. ml/runs/value-v1/<dataset>)")
	rates := lookahead.DefaultRates()
	base := playerConfig{budget: 96, searchWorkers: 1, energyValue: -1, energyGain: rates.EnergyGain, energySpend: rates.EnergySpend, statusValue: rates.Status, protectValue: -1, cardValue: 0.25, drawValue: 0.25}
	base.register(flag.CommandLine)
	tally := flag.String("tally", "", "instead of simulating, score your games against the look-ahead mirror from this battle log (\"default\" reads this workspace's log)")
	compare := flag.String("compare", "", "instead of simulating, pair two results files battle by battle: a.json[:player],b.json[:player]")
	flag.Parse()
	if *compare != "" {
		if err := printComparison(*compare); err != nil {
			fail(err)
		}
		return
	}
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
	var dataset *valuenet.Dataset
	if *record != "" {
		if dataset, err = valuenet.NewDataset(*record); err != nil {
			fail(err)
		}
		matchup.Observe = dataset.Observer
	}
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
	var entrants []lookahead.Entrant
	var configs []playerConfig
	for _, spec := range strings.Split(*players, ",") {
		spec = strings.TrimSpace(spec)
		name, config, err := parsePlayer(spec, base)
		if err != nil {
			fail(err)
		}
		factory, err := playerFactory(name, foe, heroFoe, config)
		if err != nil {
			fail(err)
		}
		entrants = append(entrants, lookahead.Entrant{Name: spec, Hero: factory})
		configs = append(configs, config)
	}
	// Players are interleaved seed by seed, so time-limited players share the
	// machine's load evenly.
	started := time.Now()
	results, err := lookahead.RunInterleaved(matchup, entrants, foe, seatList, *first, *battles, *workers, func(done, total int) {
		if done%25 == 0 || done == total {
			fmt.Fprintf(os.Stderr, "\r%d/%d battles", done, total)
		}
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "finished in %s\n", time.Since(started).Round(time.Second))
	if dataset != nil {
		var settings []string
		for e, entrant := range entrants {
			settings = append(settings, entrant.Name+": "+configs[e].describe(entrant.Name))
		}
		header, err := dataset.Close(valuenet.DatasetHeader{Hero: *hero, Opponent: *opponent, Foe: *foePlayer, Settings: settings, Command: strings.Join(os.Args, " ")})
		if err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "recorded %d positions from %d battles in %s\n", header.Rows, header.Battles, *record)
	}
	var summaries []lookahead.Summary
	for e, entrant := range entrants {
		settings := configs[e].describe(entrant.Name)
		if len(seatList) > 1 {
			for k, seatB := range seatList {
				summary := lookahead.Summarize(results[e][k**battles : (k+1)**battles])
				summary.Player = fmt.Sprintf("%s (seat %s)", entrant.Name, map[bool]string{false: "A", true: "B"}[seatB])
				summary.Settings = settings
				summaries = append(summaries, summary)
			}
		}
		summary := lookahead.Summarize(results[e])
		summary.Player, summary.Settings = entrant.Name, settings
		summaries = append(summaries, summary)
	}
	fmt.Println(lookahead.Table(summaries))
	for _, s := range summaries {
		fmt.Printf("%-10s cards per battle: %s\n", s.Player, lookahead.CardsLine(s))
		if s.Searched > 0 {
			fmt.Printf("%-10s searched %d decisions, overrode the greedy choice %d times (%.1f%%), %.0f futures per searched decision\n", s.Player, s.Searched, s.Overrides, 100*float64(s.Overrides)/float64(s.Searched), float64(s.Futures)/float64(s.Searched))
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

// playerConfig holds the flags that shape one player. The command line
// sets them for every player; a player listed as name@flag=value@... changes
// its own copy.
type playerConfig struct {
	budget, thinkMS, searchWorkers, rounds int
	trace                                  bool
	cardValue, drawValue                   float64
	// energyValue, when not negative, sets both energy rates; protectValue,
	// when not negative, overrides the status rate for Protect.
	energyValue, energyGain, energySpend float64
	statusValue, protectValue            float64
	bias                                 string
	// valueModel is a value network file that judges the look-ahead's
	// horizon (with rounds > 0).
	valueModel string
}

func (c *playerConfig) register(fs *flag.FlagSet) {
	fs.IntVar(&c.budget, "budget", c.budget, "look-ahead simulated futures per decision")
	fs.IntVar(&c.thinkMS, "think-ms", c.thinkMS, "look-ahead hard time limit per decision in ms, instead of -budget (0 uses -budget)")
	fs.IntVar(&c.searchWorkers, "search-workers", c.searchWorkers, "goroutines each look-ahead decision plays its futures on")
	fs.IntVar(&c.rounds, "rounds", c.rounds, "look-ahead horizon in rounds (0 plays each future to the end)")
	fs.BoolVar(&c.trace, "trace", c.trace, "print every look-ahead override")
	fs.StringVar(&c.bias, "bias", c.bias, "experiment: favour abilities in the greedy player's choices, e.g. measured_strike=2")
	fs.StringVar(&c.valueModel, "value-model", c.valueModel, "value network file (value-v1) that judges the look-ahead's horizon; needs -rounds > 0")
	fs.Float64Var(&c.energyValue, "energy-value", c.energyValue, "shortcut: sets -energy-gain-value and -energy-spend-value (negative leaves them)")
	fs.Float64Var(&c.energyGain, "energy-gain-value", c.energyGain, "damage one energy point gained is worth to the greedy player (and the look-ahead's simulated play)")
	fs.Float64Var(&c.energySpend, "energy-spend-value", c.energySpend, "damage one energy point spent on a card costs the greedy player")
	fs.Float64Var(&c.statusValue, "status-value", c.statusValue, "damage one stack of a status an ability gives its user is worth to the greedy player")
	fs.Float64Var(&c.protectValue, "protect-value", c.protectValue, "damage one stack of Protect is worth to the greedy player (negative: the -status-value)")
	fs.Float64Var(&c.cardValue, "card-value", c.cardValue, "damage holding a card for later is worth to the current greedy player")
	fs.Float64Var(&c.drawValue, "draw-value", c.drawValue, "damage one drawn card is worth to the greedy player")
}

// parsePlayer reads "name@flag=value@flag=value" into the player's name and
// its copy of the player flags.
func parsePlayer(spec string, base playerConfig) (string, playerConfig, error) {
	parts := strings.Split(spec, "@")
	c := base
	fs := flag.NewFlagSet(parts[0], flag.ContinueOnError)
	c.register(fs)
	var args []string
	for _, part := range parts[1:] {
		args = append(args, "-"+part)
	}
	if err := fs.Parse(args); err != nil {
		return "", c, fmt.Errorf("player %q: %w", spec, err)
	}
	return parts[0], c, nil
}

// rates are the player's exchange rates.
func (c playerConfig) rates() lookahead.Rates {
	r := lookahead.DefaultRates()
	r.Status, r.EnergyGain, r.EnergySpend = c.statusValue, c.energyGain, c.energySpend
	if c.energyValue >= 0 {
		r.EnergyGain, r.EnergySpend = c.energyValue, c.energyValue
	}
	if c.protectValue >= 0 {
		r = r.WithStatus("protect", c.protectValue)
	}
	return r
}

// describe records a player's settings in the results file.
func (c playerConfig) describe(name string) string {
	text := fmt.Sprintf("rates(%s) card-value=%g draw-value=%g", c.rates(), c.cardValue, c.drawValue)
	if c.bias != "" {
		text += " bias=" + c.bias
	}
	if strings.HasPrefix(name, "lookahead") {
		search := fmt.Sprintf("budget=%d", c.budget)
		if c.thinkMS > 0 {
			search = fmt.Sprintf("think-ms=%d", c.thinkMS)
		}
		text = fmt.Sprintf("%s rounds=%d search-workers=%d %s", search, c.rounds, max(1, c.searchWorkers), text)
		if c.valueModel != "" {
			text += " value-model=" + c.valueModel
		}
	}
	return text
}

func playerFactory(name string, foe lookahead.PolicyFactory, heroFoe bool, c playerConfig) (lookahead.PolicyFactory, error) {
	bias := parseBias(c.bias)
	greedy := func(v1 bool) *lookahead.GreedyPolicy {
		g := lookahead.NewGreedyPolicy()
		if v1 {
			g = lookahead.NewGreedyPolicyV1()
		} else {
			g.CardValue = c.cardValue
		}
		g.Rates, g.DrawValue, g.Bias = c.rates(), c.drawValue, bias
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
		var network *valuenet.Model
		if c.valueModel != "" {
			if c.rounds <= 0 {
				return nil, fmt.Errorf("%s: -value-model needs -rounds > 0", name)
			}
			var err error
			if network, err = valuenet.LoadFile(c.valueModel); err != nil {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "%s judges its horizon with %s (sha256 %s)\n", name, c.valueModel, network.SHA256)
		}
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
			search := lookahead.NewSearchPolicy(model, c.budget)
			if v1 {
				search = lookahead.NewSearchPolicyV1(model, c.budget)
			}
			search.Rollout = greedy(v1)
			search.Rounds = c.rounds
			search.Think = time.Duration(c.thinkMS) * time.Millisecond
			search.Workers = max(1, c.searchWorkers)
			if network != nil {
				search.Evaluator = network.Evaluators()
			}
			if c.trace {
				search.Trace = func(line string) { fmt.Fprintln(os.Stderr, line) }
			}
			return search, nil
		}, nil
	}
	return nil, fmt.Errorf("unknown player %q", name)
}

// printComparison pairs two players' battles by seed and hero seat. Each
// side is a results file written by -out, optionally followed by
// ":player" when the file holds several players.
func printComparison(spec string) error {
	parts := strings.Split(spec, ",")
	if len(parts) != 2 {
		return fmt.Errorf("-compare needs two results files, got %q", spec)
	}
	var sides [2]lookahead.Summary
	for i, part := range parts {
		s, err := readSummary(strings.TrimSpace(part))
		if err != nil {
			return err
		}
		sides[i] = s
	}
	c, err := lookahead.Compare(sides[0], sides[1])
	if err != nil {
		return err
	}
	for i, s := range sides {
		fmt.Printf("%c: %s · %s\n", 'A'+i, parts[i], s.Settings)
	}
	fmt.Print(c.String())
	return nil
}

// readSummary loads one player's combined summary from a results file.
func readSummary(spec string) (lookahead.Summary, error) {
	path, player := spec, ""
	if i := strings.LastIndex(spec, ":"); i > 0 && !strings.ContainsRune(spec[i:], '/') {
		path, player = spec[:i], spec[i+1:]
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return lookahead.Summary{}, err
	}
	var summaries []lookahead.Summary
	if err := json.Unmarshal(raw, &summaries); err != nil {
		return lookahead.Summary{}, fmt.Errorf("%s: %w", path, err)
	}
	var combined []lookahead.Summary
	var names []string
	for _, s := range summaries {
		if strings.Contains(s.Player, " (seat ") {
			continue
		}
		if player == "" || s.Player == player {
			combined = append(combined, s)
		}
		names = append(names, s.Player)
	}
	if len(combined) != 1 {
		return lookahead.Summary{}, fmt.Errorf("%s holds players %v; name one as %s:player", path, names, path)
	}
	return combined[0], nil
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
