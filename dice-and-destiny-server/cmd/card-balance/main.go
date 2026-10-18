// Command card-balance measures what one copy of a card adds to a character's
// deck. For every card it adds one copy to the character's content deck and
// plays thousands of real authority battles with the greedy player: in the
// mirror against the unchanged deck (both seats) and against scripted
// minions, on the same seeds for every card. It compares each card with the
// unchanged deck and with a blank card (pure health in battle) and sets the
// result against the card's XP price.
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
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/content"
)

// defaultCards are the Starter-pool cards whose effects the greedy player
// plays properly (see notPlayable for the rest).
var defaultCards = []string{
	"steady_guard", "steady_guard_paths_variant_node_1", "steady_guard_paths_variant_node_2", "steady_guard_paths_variant_node_3",
	"steady_guard_paths_variant_node_4", "steady_guard_paths_variant_node_5", "steady_guard_paths_variant_node_6", "steady_guard_paths_variant_node_7",
	"strong_swing", "strong_swing_paths_variant_node_1", "strong_swing_paths_variant_node_2", "strong_swing_paths_variant_node_3",
	"strong_swing_paths_variant_node_4", "strong_swing_paths_variant_node_5", "strong_swing_paths_variant_node_7",
	"nudge", "nudge_paths_variant_node_1", "nudge_paths_variant_node_3", "nudge_paths_variant_node_4",
	"nudge_paths_variant_node_8", "nudge_paths_variant_node_9", "nudge_paths_variant_node_10",
	"try_again", "try_again_paths_variant_node_4",
	"take_stock", "take_stock_paths_variant_node_1", "take_stock_paths_variant_node_2", "take_stock_paths_variant_node_3", "take_stock_paths_variant_node_4",
	"second_wind", "second_wind_paths_variant_node_1", "second_wind_paths_variant_node_4",
}

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards root (read only)")
	hero := flag.String("hero", "starter", "character whose deck gets the card")
	cardList := flag.String("cards", strings.Join(defaultCards, ","), "comma-separated card IDs to measure")
	blank := flag.String("blank", "trailblaze", "a card with no battle effect (pure health)")
	matchups := flag.String("matchups", "mirror,drowned_regent,drowned_oracle_brine_mask,drowned_oracle_bell_diver,drowned_oracle_ribbon_eel", "mirror and/or scripted minion IDs")
	battles := flag.Int("battles", 2000, "seeds per matchup (the mirror plays each from both seats)")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel battles")
	energyValue := flag.Float64("energy-value", 0.05, "greedy player's energy value (calibrated)")
	drawValue := flag.Float64("draw-value", 0.75, "greedy player's value of a drawn card (calibrated)")
	cardValue := flag.Float64("card-value", 0.25, "greedy player's value of holding a card")
	out := flag.String("out", "", "optional JSON results path")
	htmlOut := flag.String("html", "", "optional HTML report path")
	from := flag.String("from", "", "render the report from a saved JSON results file instead of playing")
	flag.Parse()

	debug.SetGCPercent(400)
	if os.Getenv(content.AuthoredRootEnv) == "" {
		os.Setenv(content.AuthoredRootEnv, *authoredRoot)
	}
	catalogs, err := learned.CharacterCatalogs(*contentRoot, *authoredRoot)
	if err != nil {
		fail(err)
	}
	lib, ok := catalogs[*hero]
	if !ok {
		fail(fmt.Errorf("unknown character %q", *hero))
	}
	economy, err := loadout.LoadEconomy(*contentRoot, catalogs)
	if err != nil {
		fail(err)
	}
	var base []loadout.Entry
	baseCards := map[string]bool{}
	for _, e := range lib.Combatants[*hero].Decklist {
		base = append(base, loadout.Entry{CardID: e.CardID, Count: e.Count})
		baseCards[e.CardID] = true
	}
	if *from != "" {
		raw, err := os.ReadFile(*from)
		if err != nil {
			fail(err)
		}
		var rows []Row
		if err := json.Unmarshal(raw, &rows); err != nil {
			fail(err)
		}
		finish(rows, baseCards, *blank, *hero, *battles, "", *htmlOut, strings.Split(*matchups, ","))
		return
	}
	runState, err := os.MkdirTemp("", "card-balance-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)

	greedy := func() lookahead.Policy {
		g := lookahead.NewGreedyPolicy()
		g.EnergyValue, g.DrawValue, g.CardValue = *energyValue, *drawValue, *cardValue
		return g
	}
	variants := []string{"", *blank}
	for _, c := range strings.Split(*cardList, ",") {
		if c = strings.TrimSpace(c); c != "" {
			if _, ok := lib.Cards[c]; !ok {
				fail(fmt.Errorf("card %q is not in the %s library", c, *hero))
			}
			variants = append(variants, c)
		}
	}
	fmt.Printf("%s: base deck of %d cards, %d variants, matchups %s, %d seeds each\n", *hero, deckSize(base), len(variants), *matchups, *battles)

	results := map[string]map[string][]lookahead.BattleRecord{} // matchup -> variant -> records
	started := time.Now()
	for _, matchup := range strings.Split(*matchups, ",") {
		matchup = strings.TrimSpace(matchup)
		m := lookahead.Matchup{ContentRoot: *contentRoot, AuthoredRoot: *authoredRoot, RunStateRoot: runState, Hero: *hero, Opponent: matchup, Unified: true, HeroPlansFirst: true}
		seats := []bool{false}
		foe := func() (lookahead.Policy, error) {
			minion, err := learned.LoadScriptedMinion(*contentRoot, matchup)
			if err != nil {
				return nil, err
			}
			return lookahead.Scripted{Label: matchup, Policy: minion}, nil
		}
		if matchup == "mirror" {
			m.Opponent, m.FoeDeck = *hero, base
			seats = []bool{false, true}
			foe = func() (lookahead.Policy, error) { return greedy(), nil }
		}
		results[matchup] = map[string][]lookahead.BattleRecord{}
		for _, variant := range variants {
			m.HeroDeck = withCard(base, variant)
			for _, seatB := range seats {
				m.HeroInSeatB = seatB
				summary, err := lookahead.RunBattles(m, func() (lookahead.Policy, error) { return greedy(), nil }, foe, 1, *battles, *workers, nil)
				if err != nil {
					fail(fmt.Errorf("%s %s: %w", matchup, label(lib, variant), err))
				}
				results[matchup][variant] = append(results[matchup][variant], summary.Records...)
			}
			fmt.Fprintf(os.Stderr, "\r%-28s %-28s done (%s)", matchup, label(lib, variant), time.Since(started).Round(time.Second))
		}
		fmt.Fprintln(os.Stderr)
	}

	rows := report(lib, economy, *hero, variants, strings.Split(*matchups, ","), results)
	finish(rows, baseCards, *blank, *hero, *battles, *out, *htmlOut, strings.Split(*matchups, ","))
}

// finish prices the cards and writes the table, JSON and HTML report.
func finish(rows []Row, baseCards map[string]bool, blank, hero string, seeds int, out, htmlOut string, matchups []string) {
	prices := price(rows, baseCards, blank)
	for i := range rows {
		p := prices[rows[i].Card]
		rows[i].Pricing = &p
	}
	printTable(rows, matchups)
	if out != "" {
		raw, _ := json.MarshalIndent(rows, "", "  ")
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			fail(err)
		}
	}
	if htmlOut != "" {
		if err := os.WriteFile(htmlOut, []byte(renderHTML(hero, rows, prices, baseCards, seeds)), 0o644); err != nil {
			fail(err)
		}
		fmt.Println(htmlOut)
	}
}

func deckSize(deck []loadout.Entry) int {
	n := 0
	for _, e := range deck {
		n += e.Count
	}
	return n
}

// withCard is the deck with one more copy of card ("" leaves it unchanged).
func withCard(deck []loadout.Entry, card string) []loadout.Entry {
	out := append([]loadout.Entry(nil), deck...)
	if card == "" {
		return out
	}
	for i := range out {
		if out[i].CardID == card {
			out[i].Count++
			return out
		}
	}
	return append(out, loadout.Entry{CardID: card, Count: 1})
}

func label(lib content.BattleLibrary, id string) string {
	if id == "" {
		return "(unchanged deck)"
	}
	if c, ok := lib.Cards[id]; ok && c.Name != "" {
		return c.Name
	}
	return id
}

// Delta is a paired difference against the unchanged deck on the same seeds.
type Delta struct {
	Mean float64 `json:"mean"`
	SE   float64 `json:"se"`
}

type Row struct {
	Card         string             `json:"card"`
	Name         string             `json:"name"`
	Price        int                `json:"price_xp"`
	Energy       int                `json:"energy"`
	Effect       string             `json:"effect"`
	PlayedMirror float64            `json:"played_per_battle_mirror"`
	Score        map[string]Delta   `json:"score_change"`
	Health       map[string]Delta   `json:"health_left_change"`
	Base         map[string]float64 `json:"unchanged_deck_score"`
	Pricing      *Pricing           `json:"pricing,omitempty"`
}

func report(lib content.BattleLibrary, economy loadout.Economy, hero string, variants, matchups []string, results map[string]map[string][]lookahead.BattleRecord) []Row {
	var rows []Row
	for _, v := range variants {
		if v == "" {
			continue
		}
		card := lib.Cards[v]
		row := Row{Card: v, Name: label(lib, v), Price: economy.Price(hero, v), Energy: card.Cost.Energy,
			Effect: strings.ReplaceAll(card.Presentation.EffectSummary, "\n", " / "),
			Score:  map[string]Delta{}, Health: map[string]Delta{}, Base: map[string]float64{}}
		for _, matchup := range matchups {
			matchup = strings.TrimSpace(matchup)
			baseRecords := index(results[matchup][""])
			var ds, dh []float64
			played, n := 0, 0
			for _, r := range results[matchup][v] {
				b, ok := baseRecords[key(r)]
				if !ok {
					continue
				}
				ds = append(ds, score(r)-score(b))
				dh = append(dh, float64(r.HeroHealth-b.HeroHealth))
				played += r.CardsPlayed[v]
				n++
			}
			row.Score[matchup], row.Health[matchup] = meanSE(ds), meanSE(dh)
			total := 0.0
			for _, b := range results[matchup][""] {
				total += score(b)
			}
			row.Base[matchup] = total / float64(max(1, len(results[matchup][""])))
			if matchup == "mirror" && n > 0 {
				row.PlayedMirror = float64(played) / float64(n)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func index(records []lookahead.BattleRecord) map[string]lookahead.BattleRecord {
	out := map[string]lookahead.BattleRecord{}
	for _, r := range records {
		out[key(r)] = r
	}
	return out
}

func key(r lookahead.BattleRecord) string { return fmt.Sprintf("%d/%s", r.Seed, r.Seat) }

func score(r lookahead.BattleRecord) float64 {
	switch r.Result {
	case "win":
		return 1
	case "draw":
		return 0.5
	}
	return 0
}

func meanSE(xs []float64) Delta {
	if len(xs) < 2 {
		return Delta{}
	}
	m := 0.0
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	return Delta{Mean: m, SE: math.Sqrt(v / float64(len(xs)-1) / float64(len(xs)))}
}

func printTable(rows []Row, matchups []string) {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Score["mirror"].Mean > rows[j].Score["mirror"].Mean })
	fmt.Printf("\n%-20s %4s %3s %6s", "card", "XP", "E", "played")
	for _, m := range matchups {
		fmt.Printf(" %18s", short(m))
	}
	fmt.Printf(" %16s\n", "mirror per 10 XP")
	for _, r := range rows {
		fmt.Printf("%-20s %4d %3d %6.2f", r.Name, r.Price, r.Energy, r.PlayedMirror)
		for _, m := range matchups {
			m = strings.TrimSpace(m)
			d := r.Score[m]
			if m != "mirror" && m != "drowned_regent" {
				d = r.Health[m]
				fmt.Printf("   hp %+5.2f ±%4.2f", d.Mean, 1.96*d.SE)
				continue
			}
			fmt.Printf("   %+6.3f ±%5.3f", d.Mean, 1.96*d.SE)
		}
		fmt.Printf(" %16.4f\n", r.Score["mirror"].Mean*10/float64(max(r.Price, 1)))
	}
}

func short(m string) string {
	return strings.TrimPrefix(strings.TrimPrefix(m, "drowned_oracle_"), "drowned_")
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
