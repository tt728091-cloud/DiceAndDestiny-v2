// Command board-lab plays the custom adventurer's priced boards in real
// authority battles and reports how each scores against the Starter and the
// minions. It uses only the look-ahead package's exported runner and players
// (greedy hero; the Starter as a greedy mirror, minions by their own scripts),
// on paired seeds: every seed is played once from each seat.
//
// The matrix is every menu die × template at default picks, a set of seeded
// random boards (boardprice.RandomBoard) and a rider sweep: each default
// board's first flex slot with every rider it may take. Each board is written
// to the custom adventurer's sheet in a scratch loadout root through the
// authority's own save, so battles pin it exactly as the game does. Published
// cards are read from the authored root and never written.
//
// -pure plays the boards alone instead (pure.go): blank decks, one defense,
// no riders, every default board against a reference board, beside the race
// model's prediction. -roundrobin scores every default board against every
// other that way under three defenses and tunes the field (roundrobin.go).
//
//	go run ./cmd/board-lab -battles 200 -random 300 -random-battles 40 -markdown /tmp/board-lab.md
//	go run ./cmd/board-lab -pure -battles 2000 -health 15 -markdown /tmp/pure.md
//	go run ./cmd/board-lab -roundrobin -battles 1000 -sample 16 -markdown /tmp/round-robin.md
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
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/content"
)

// Board is one evaluated board.
type Board struct {
	Kind     string             `json:"kind"` // default, random or rider
	Choice   boardprice.Choice  `json:"choice"`
	Family   string             `json:"family"`
	Average  float64            `json:"whole_average"`
	NoHit    float64            `json:"whole_no_hit"`
	Scores   map[string]float64 `json:"scores"`
	Battles  map[string]int     `json:"battles"`
	Rider    string             `json:"rider,omitempty"` // rider sweep: the swept rider
	Slot     string             `json:"slot,omitempty"`
	ScoreLow map[string]float64 `json:"score_ci95_low"`
}

type lab struct {
	contentRoot string
	builder     *boardprice.Builder
	workers     int
	first       uint64
}

func main() {
	serverRoot := defaultServerRoot()
	contentRoot := flag.String("content-root", filepath.Join(serverRoot, "content"), "battle content root")
	authoredRoot := flag.String("authored-root", filepath.Join(serverRoot, "content", "authored"), "published cards root (read only)")
	battles := flag.Int("battles", 200, "battles per default board and opponent (half from each seat)")
	opponents := flag.String("opponents", "starter,adventurer,drowned_oracle_brine_mask,drowned_oracle_bell_diver,drowned_oracle_ribbon_eel", "opponents for the default boards")
	random := flag.Int("random", 300, "random boards")
	randomBattles := flag.Int("random-battles", 40, "battles per random board and opponent")
	randomOpponents := flag.String("random-opponents", "starter,adventurer", "opponents for the random boards")
	riderBattles := flag.Int("rider-battles", 200, "battles per rider-sweep board and opponent (0 skips the sweep)")
	riderOpponents := flag.String("rider-opponents", "starter,adventurer", "opponents for the rider sweep")
	controls := flag.Int("control-battles", 200, "battles for the control matchups against the Starter (0 skips them)")
	seed := flag.Int64("seed", 7, "seed of the random boards")
	first := flag.Uint64("first-seed", 1, "first battle seed")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-2), "parallel battles")
	out := flag.String("out", "", "optional JSON results path")
	markdown := flag.String("markdown", "", "optional Markdown report path")
	pure := flag.Bool("pure", false, "pure duel: blank decks, one defense, no riders, every default board against a reference board")
	health := flag.Int("health", 15, "pure duel: blank cards in each deck (the health)")
	reference := flag.String("reference", "standard:standard_d6", "pure duel: the reference board, template:die at default picks")
	defense := flag.String("defense", "holdfast", "pure duel: both sides' one defense, or none")
	templates := flag.String("templates", "", "pure duel: only these templates (comma separated)")
	roundrobin := flag.Bool("roundrobin", false, "round robin: every default board against every other with blank decks, under each duel defense, by the race model; then tune every board's target to level")
	healths := flag.String("healths", "12-30", "round robin: the starting healths each cell is averaged over")
	tuneRounds := flag.Int("tune-rounds", 3, "round robin: tuning rounds")
	sample := flag.Int("sample", 16, "round robin: pairings per defense played as real battles (-battles each) to check the model")
	flag.Parse()

	debug.SetGCPercent(400)
	// Battles read published cards from the authored root; each board's sheet
	// lives in its own scratch loadout root.
	if err := os.Setenv(content.AuthoredRootEnv, *authoredRoot); err != nil {
		fail(err)
	}
	catalogs, err := learned.CharacterCatalogs(*contentRoot)
	if err != nil {
		fail(err)
	}
	builder, err := boardprice.NewBuilder(*contentRoot, catalogs[loadout.CustomAdventurer])
	if err != nil {
		fail(err)
	}
	l := lab{contentRoot: *contentRoot, builder: builder, workers: *workers, first: *first}
	if *pure {
		runPure(l, *health, *reference, *defense, *battles, *templates, *out, *markdown)
		return
	}
	if *roundrobin {
		runRoundRobin(l, *healths, *tuneRounds, *sample, *battles, *seed, *out, *markdown)
		return
	}
	started := time.Now()
	var controlLines []string
	if *controls > 0 {
		// Controls: the Starter's own mirror, and the custom adventurer
		// carrying the Starter's authored board on the standard die. Both
		// should score about 0.5 if the harness is fair.
		mirror := l.play(*authoredRoot, "starter", "starter", *controls)
		starterBoard := content.AbilityBoard{Offensive: catalogs["starter"].Combatants["starter"].AbilityBoard.Offensive, Defensive: catalogs["starter"].Combatants["starter"].AbilityBoard.Defensive}
		root, err := os.MkdirTemp("", "board-lab-sheet-")
		if err != nil {
			fail(err)
		}
		if err := l.writeBoard(root, loadout.SheetBoard{Dice: "standard_d6", Abilities: starterBoard}); err != nil {
			fail(err)
		}
		same := l.play(root, loadout.CustomAdventurer, "starter", *controls)
		sameAdventurer := l.play(root, loadout.CustomAdventurer, "adventurer", *controls)
		adventurer := l.play(*authoredRoot, "adventurer", "starter", *controls)
		os.RemoveAll(root)
		controlLines = []string{
			fmt.Sprintf("| Starter vs the Starter (greedy mirror) | %.3f |", mirror),
			fmt.Sprintf("| Custom Adventurer with the Starter's own board, standard die, vs the Starter | %.3f |", same),
			fmt.Sprintf("| The Adventurer (the Starter's board plus Guard+) vs the Starter | %.3f |", adventurer),
			fmt.Sprintf("| Custom Adventurer with the Starter's own board, standard die, vs the Adventurer | %.3f |", sameAdventurer),
		}
		fmt.Fprintf(os.Stderr, "controls: starter mirror %.3f, custom with the Starter board vs starter %.3f, adventurer vs starter %.3f, custom with the Starter board vs adventurer %.3f (%s)\n", mirror, same, adventurer, sameAdventurer, time.Since(started).Round(time.Second))
	}
	var boards []Board
	// Every die × template at default picks.
	for _, tpl := range builder.Menus.Templates {
		for _, entry := range builder.Menus.Dice {
			die, err := builder.Die(entry.ID)
			if err != nil {
				fail(err)
			}
			if die.Layout.Family != tpl.Family {
				continue
			}
			boards = append(boards, l.evaluate("default", builder.Default(tpl.ID, die.ID), split(*opponents), *battles))
			report(boards[len(boards)-1], started)
		}
	}
	// The rider sweep: each template's default board (first die of its
	// family) with every rider its first flex slot may take.
	if *riderBattles > 0 {
		for _, tpl := range builder.Menus.Templates {
			dieID := ""
			for _, entry := range builder.Menus.Dice {
				if die, _ := builder.Die(entry.ID); die.Layout.Family == tpl.Family && dieID == "" {
					dieID = die.ID
				}
			}
			var slot boardprice.TemplateSlot
			for _, s := range tpl.Slots {
				if s.Flex() {
					slot = s
					break
				}
			}
			if slot.ID == "" {
				continue
			}
			for _, rider := range builder.Menus.Riders {
				if !rider.RiderFits(slot.Job) {
					continue
				}
				choice := boardprice.Choice{Die: dieID, Template: tpl.ID, Slots: []boardprice.SlotPick{{Slot: slot.ID, Rider: rider.ID}}}
				b := l.evaluate("rider", choice, split(*riderOpponents), *riderBattles)
				b.Rider, b.Slot = rider.ID, slot.ID
				boards = append(boards, b)
				report(b, started)
			}
		}
	}
	// Seeded random boards.
	rng := rand.New(rand.NewSource(*seed))
	for i := 0; i < *random; i++ {
		choice, err := builder.RandomBoard(rng, "")
		if err != nil {
			fail(err)
		}
		boards = append(boards, l.evaluate("random", choice, split(*randomOpponents), *randomBattles))
		if (i+1)%25 == 0 {
			fmt.Fprintf(os.Stderr, "random boards: %d/%d (%s)\n", i+1, *random, time.Since(started).Round(time.Second))
		}
	}
	text := summarize(boards, builder, *battles, *randomBattles, *riderBattles, *seed, controlLines, *controls)
	fmt.Println(text)
	if *markdown != "" {
		if err := os.WriteFile(*markdown, []byte(text), 0o644); err != nil {
			fail(err)
		}
	}
	if *out != "" {
		raw, err := json.MarshalIndent(boards, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(*out, raw, 0o644); err != nil {
			fail(err)
		}
	}
}

// evaluate prices a board, writes it to a scratch sheet and plays it.
func (l lab) evaluate(kind string, choice boardprice.Choice, opponents []string, battles int) Board {
	resolved, report, err := l.builder.Price(choice)
	if err != nil {
		fail(fmt.Errorf("%+v: %w", choice, err))
	}
	die, _ := l.builder.Die(resolved.Die)
	root, err := os.MkdirTemp("", "board-lab-sheet-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(root)
	if err := l.writeSheet(root, resolved, report); err != nil {
		fail(fmt.Errorf("%+v: %w", resolved, err))
	}
	runState, err := os.MkdirTemp("", "board-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	b := Board{Kind: kind, Choice: resolved, Family: die.Layout.Family, Average: report.WholeAverage, NoHit: report.WholeNoHit, Scores: map[string]float64{}, Battles: map[string]int{}, ScoreLow: map[string]float64{}}
	for _, opponent := range opponents {
		var records []lookahead.BattleRecord
		for _, seatB := range []bool{false, true} {
			m := lookahead.Matchup{ContentRoot: l.contentRoot, AuthoredRoot: root, RunStateRoot: runState, Hero: loadout.CustomAdventurer, Opponent: opponent, Unified: true, HeroPlansFirst: true, HeroInSeatB: seatB}
			summary, err := lookahead.RunBattles(m, greedy, l.foe(opponent), l.first, battles/2, l.workers, nil)
			if err != nil {
				fail(fmt.Errorf("%s vs %s: %w", resolved.Template, opponent, err))
			}
			records = append(records, summary.Records...)
		}
		s := lookahead.Summarize(records)
		b.Scores[opponent] = s.Score
		b.Battles[opponent] = s.Battles
		b.ScoreLow[opponent] = s.WinRateLow
	}
	return b
}

// play scores a hero against an opponent from both seats on paired seeds;
// root is the loadout (or authored) root the battles read.
func (l lab) play(root, hero, opponent string, battles int) float64 {
	runState, err := os.MkdirTemp("", "board-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	var records []lookahead.BattleRecord
	for _, seatB := range []bool{false, true} {
		m := lookahead.Matchup{ContentRoot: l.contentRoot, AuthoredRoot: root, RunStateRoot: runState, Hero: hero, Opponent: opponent, Unified: true, HeroPlansFirst: true, HeroInSeatB: seatB}
		summary, err := lookahead.RunBattles(m, greedy, l.foe(opponent), l.first, battles/2, l.workers, nil)
		if err != nil {
			fail(fmt.Errorf("%s vs %s: %w", hero, opponent, err))
		}
		records = append(records, summary.Records...)
	}
	return lookahead.Summarize(records).Score
}

// writeSheet confirms a board on the custom adventurer's sheet the way the
// Board Builder does: generated abilities, the board and the die.
func (l lab) writeSheet(root string, choice boardprice.Choice, report boardprice.Report) error {
	abilities, board, err := l.builder.Generate(choice, report)
	if err != nil {
		return err
	}
	generated := map[string]content.BattleAbilityDefinition{}
	for _, a := range abilities {
		generated[a.ID] = a
	}
	return l.writeBoard(root, loadout.SheetBoard{Dice: choice.Die, Abilities: board, Generated: generated, Choice: &choice})
}

// writeBoard saves a die and board to the custom adventurer's sheet through
// the authority's validated save.
func (l lab) writeBoard(root string, sheet loadout.SheetBoard) error {
	catalogs, err := learned.CharacterCatalogs(l.contentRoot, root)
	if err != nil {
		return err
	}
	economy, err := loadout.LoadEconomy(l.contentRoot, catalogs)
	if err != nil {
		return err
	}
	lib := catalogs[loadout.CustomAdventurer]
	progress, err := loadout.ReadProgress(root, loadout.CustomAdventurer, economy, lib)
	if err != nil {
		return err
	}
	_, err = loadout.SaveSheetBoard(root, loadout.CustomAdventurer, economy, lib, sheet, progress.Revision)
	return err
}

func greedy() (lookahead.Policy, error) { return lookahead.NewGreedyPolicy(), nil }

// foe plays a hero (the Starter mirror) greedily and a minion by its own
// script.
func (l lab) foe(opponent string) lookahead.PolicyFactory {
	if slices.Contains(learned.PlayableCharacters, opponent) {
		return greedy
	}
	return func() (lookahead.Policy, error) {
		minion, err := learned.LoadScriptedMinion(l.contentRoot, opponent)
		if err != nil {
			return nil, err
		}
		return lookahead.Scripted{Label: opponent, Policy: minion}, nil
	}
}

func report(b Board, started time.Time) {
	var parts []string
	for _, opponent := range sortedKeys(b.Scores) {
		parts = append(parts, fmt.Sprintf("%s %.3f", short(opponent), b.Scores[opponent]))
	}
	label := b.Choice.Template + " · " + b.Choice.Die
	if b.Rider != "" {
		label += " · " + b.Slot + "=" + b.Rider
	}
	fmt.Fprintf(os.Stderr, "%-7s %-44s avg %.2f  %s  (%s)\n", b.Kind, label, b.Average, strings.Join(parts, "  "), time.Since(started).Round(time.Second))
}

// The Starter mirror band: a board within it plays level with the Starter.
const bandLow, bandHigh = 0.45, 0.55

// references are the Starter-level opponents a board is judged against: the
// Starter itself and the Adventurer (the Starter's board plus Guard+), which
// plays an equal board level (see the controls).
var references = []string{"starter", "adventurer"}

func summarize(boards []Board, b *boardprice.Builder, battles, randomBattles, riderBattles int, seed int64, controls []string, controlBattles int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### Board lab results\n\n")
	fmt.Fprintf(&sb, "`go run ./cmd/board-lab` · greedy hero (look-ahead's `GreedyPolicy`) · the Starter and the Adventurer as greedy players, minions by their own scripts · unified defense · paired seeds (each seed from both seats).\n\n")
	fmt.Fprintf(&sb, "Score = (wins + ½ draws) / battles for the custom adventurer. Starter mirror band %.2f–%.2f.\n\n", bandLow, bandHigh)
	if len(controls) > 0 {
		fmt.Fprintf(&sb, "**Controls** (%d battles each):\n\n| Matchup | Score |\n|---|---|\n%s\n\n", controlBattles, strings.Join(controls, "\n"))
	}
	opponents := map[string]bool{}
	for _, board := range boards {
		if board.Kind == "default" {
			for o := range board.Scores {
				opponents[o] = true
			}
		}
	}
	cols := sortedKeys(opponents)
	rank := func(o string) int {
		for i, r := range references {
			if o == r {
				return i
			}
		}
		return len(references)
	}
	sort.SliceStable(cols, func(i, j int) bool { return rank(cols[i]) < rank(cols[j]) })
	fmt.Fprintf(&sb, "**Every die × template at default picks** (%d battles per cell; band = inside %.2f–%.2f):\n\n", battles, bandLow, bandHigh)
	fmt.Fprintf(&sb, "| Template | Die | Avg / turn | No hit |")
	for _, c := range cols {
		fmt.Fprintf(&sb, " %s |", title(c))
	}
	for _, r := range references {
		if opponents[r] {
			fmt.Fprintf(&sb, " %s band |", title(r))
		}
	}
	fmt.Fprintf(&sb, "\n|---|---|---|---|")
	for range cols {
		fmt.Fprintf(&sb, "---|")
	}
	for _, r := range references {
		if opponents[r] {
			fmt.Fprintf(&sb, "---|")
		}
	}
	fmt.Fprintf(&sb, "\n")
	outside := map[string][]string{}
	family := map[string]map[string][]float64{}
	for _, board := range boards {
		if board.Kind != "default" {
			continue
		}
		tpl, _ := b.Menus.Template(board.Choice.Template)
		die, _ := b.Die(board.Choice.Die)
		fmt.Fprintf(&sb, "| %s | %s %s | %.2f | %.1f%% |", tpl.Name, strings.TrimSuffix(die.Name, " Die"), die.Layout.Order, board.Average, board.NoHit*100)
		for _, c := range cols {
			fmt.Fprintf(&sb, " %.3f |", board.Scores[c])
		}
		for _, r := range references {
			if !opponents[r] {
				continue
			}
			score := board.Scores[r]
			if family[r] == nil {
				family[r] = map[string][]float64{}
			}
			family[r][board.Family] = append(family[r][board.Family], score)
			if score < bandLow || score > bandHigh {
				fmt.Fprintf(&sb, " **out** |")
				outside[r] = append(outside[r], fmt.Sprintf("%s on the %s (%.3f)", tpl.Name, die.Name, score))
			} else {
				fmt.Fprintf(&sb, " in |")
			}
		}
		fmt.Fprintf(&sb, "\n")
	}
	fmt.Fprintf(&sb, "\n")
	for _, r := range references {
		if !opponents[r] {
			continue
		}
		if len(outside[r]) == 0 {
			fmt.Fprintf(&sb, "- Against the %s every default board is inside the band.", title(r))
		} else {
			fmt.Fprintf(&sb, "- Against the %s, %d of the default boards are outside the band: %s.", title(r), len(outside[r]), strings.Join(outside[r], "; "))
		}
		fmt.Fprintf(&sb, " Mean by family:")
		for _, f := range sortedKeys(family[r]) {
			fmt.Fprintf(&sb, " {%s} %.3f;", f, mean(family[r][f]))
		}
		fmt.Fprintf(&sb, "\n")
	}
	// Random boards.
	var random []Board
	for _, board := range boards {
		if board.Kind == "random" {
			random = append(random, board)
		}
	}
	if len(random) > 0 {
		fmt.Fprintf(&sb, "\n**%d random boards** (`RandomBoard`, seed %d, %d battles each; a single board's 95%% interval is about ±%.2f, so the spread includes noise):\n\n| Opponent | Mean | Median | 10th–90th percentile | Inside the band |\n|---|---|---|---|---|\n", len(random), seed, randomBattles, 1.96*math.Sqrt(0.25/float64(randomBattles)))
		for _, r := range references {
			var scores []float64
			in := 0
			for _, board := range random {
				score, ok := board.Scores[r]
				if !ok {
					continue
				}
				scores = append(scores, score)
				if score >= bandLow && score <= bandHigh {
					in++
				}
			}
			if len(scores) == 0 {
				continue
			}
			sort.Float64s(scores)
			fmt.Fprintf(&sb, "| %s | %.3f | %.3f | %.3f–%.3f | %d of %d |\n", title(r), mean(scores), scores[len(scores)/2], scores[len(scores)/10], scores[len(scores)*9/10], in, len(scores))
		}
		for _, r := range references {
			effects := riderEffects(random, b, r)
			if effects == nil {
				continue
			}
			fmt.Fprintf(&sb, "\nRider effect on the score against the %s across the random boards (least squares on how many slots carry each rider; \"No rider\" is the baseline):\n\n| Rider | Slots carrying it | Effect per slot | ±95%% |\n|---|---|---|---|\n", title(r))
			for _, e := range effects {
				fmt.Fprintf(&sb, "| %s | %d | %+.3f | %.3f |\n", e.name, e.count, e.effect, e.ci)
			}
		}
	}
	// The rider sweep.
	sweep := map[string][]Board{}
	for _, board := range boards {
		if board.Kind == "rider" {
			sweep[board.Choice.Template] = append(sweep[board.Choice.Template], board)
		}
	}
	if len(sweep) > 0 {
		var riders []string
		for _, r := range b.Menus.Riders {
			if r.RiderFits("flex") {
				riders = append(riders, r.ID)
			}
		}
		for _, ref := range references {
			if _, ok := sweep[sortedKeys(sweep)[0]][0].Scores[ref]; !ok {
				continue
			}
			best, worst := map[string]int{}, map[string]int{}
			fmt.Fprintf(&sb, "\n**Rider sweep against the %s** (each template's first flex slot, on the first die of its family, with every rider; %d battles each):\n\n| Template | Slot |", title(ref), riderBattles)
			for _, r := range riders {
				rider, _ := b.Menus.Rider(r)
				fmt.Fprintf(&sb, " %s |", rider.Name)
			}
			fmt.Fprintf(&sb, " Best | Worst |\n|---|---|")
			for range riders {
				fmt.Fprintf(&sb, "---|")
			}
			fmt.Fprintf(&sb, "---|---|\n")
			for _, tplID := range sortedKeys(sweep) {
				rows := sweep[tplID]
				tpl, _ := b.Menus.Template(tplID)
				scores := map[string]float64{}
				for _, row := range rows {
					scores[row.Rider] = row.Scores[ref]
				}
				hi, lo := "", ""
				for _, r := range riders {
					if hi == "" || scores[r] > scores[hi] {
						hi = r
					}
					if lo == "" || scores[r] < scores[lo] {
						lo = r
					}
				}
				best[hi]++
				worst[lo]++
				fmt.Fprintf(&sb, "| %s | %s |", tpl.Name, rows[0].Slot)
				for _, r := range riders {
					fmt.Fprintf(&sb, " %.3f |", scores[r])
				}
				rh, _ := b.Menus.Rider(hi)
				rl, _ := b.Menus.Rider(lo)
				fmt.Fprintf(&sb, " %s | %s |\n", rh.Name, rl.Name)
			}
			var always []string
			for r, n := range best {
				if n == len(sweep) {
					rider, _ := b.Menus.Rider(r)
					always = append(always, rider.Name+" is always best")
				}
			}
			for r, n := range worst {
				if n == len(sweep) {
					rider, _ := b.Menus.Rider(r)
					always = append(always, rider.Name+" is always worst")
				}
			}
			sort.Strings(always)
			if len(always) == 0 {
				fmt.Fprintf(&sb, "\nNo rider is best or worst on every template.")
			} else {
				fmt.Fprintf(&sb, "\n%s.", strings.Join(always, "; "))
			}
			fmt.Fprintf(&sb, " Best on:")
			for _, r := range sortedKeys(best) {
				rider, _ := b.Menus.Rider(r)
				fmt.Fprintf(&sb, " %s %d;", rider.Name, best[r])
			}
			fmt.Fprintf(&sb, " worst on:")
			for _, r := range sortedKeys(worst) {
				rider, _ := b.Menus.Rider(r)
				fmt.Fprintf(&sb, " %s %d;", rider.Name, worst[r])
			}
			fmt.Fprintf(&sb, " (of %d templates). A score's 95%% interval is about ±%.2f.\n", len(sweep), 1.96*math.Sqrt(0.25/float64(riderBattles)))
		}
	}
	return sb.String()
}

type riderEffect struct {
	name       string
	count      int
	effect, ci float64
}

// riderEffects regresses the score against an opponent on how many slots
// carry each rider (intercept plus one column per rider other than none).
func riderEffects(boards []Board, b *boardprice.Builder, opponent string) []riderEffect {
	var riders []string
	for _, r := range b.Menus.Riders {
		if r.ID != boardprice.RiderNone {
			riders = append(riders, r.ID)
		}
	}
	k := len(riders) + 1
	xtx := make([][]float64, k)
	for i := range xtx {
		xtx[i] = make([]float64, k)
	}
	xty := make([]float64, k)
	counts := map[string]int{}
	rows := make([][]float64, len(boards))
	for n, board := range boards {
		row := make([]float64, k)
		row[0] = 1
		for _, s := range board.Choice.Slots {
			for i, r := range riders {
				if s.Rider == r {
					row[i+1]++
					counts[r]++
				}
			}
		}
		rows[n] = row
		y := board.Scores[opponent]
		for i := 0; i < k; i++ {
			xty[i] += row[i] * y
			for j := 0; j < k; j++ {
				xtx[i][j] += row[i] * row[j]
			}
		}
	}
	inv, ok := invert(xtx)
	if !ok {
		return nil
	}
	beta := make([]float64, k)
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			beta[i] += inv[i][j] * xty[j]
		}
	}
	rss := 0.0
	for n, board := range boards {
		fit := 0.0
		for i := 0; i < k; i++ {
			fit += rows[n][i] * beta[i]
		}
		rss += math.Pow(board.Scores[opponent]-fit, 2)
	}
	sigma2 := rss / math.Max(1, float64(len(boards)-k))
	var out []riderEffect
	for i, r := range riders {
		rider, _ := b.Menus.Rider(r)
		out = append(out, riderEffect{name: rider.Name, count: counts[r], effect: beta[i+1], ci: 1.96 * math.Sqrt(sigma2*inv[i+1][i+1])})
	}
	return out
}

// invert is Gauss–Jordan inversion of a small matrix.
func invert(m [][]float64) ([][]float64, bool) {
	n := len(m)
	a := make([][]float64, n)
	for i := range m {
		a[i] = make([]float64, 2*n)
		copy(a[i], m[i])
		a[i][n+i] = 1
	}
	for c := 0; c < n; c++ {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return nil, false
		}
		a[c], a[p] = a[p], a[c]
		pivot := a[c][c]
		for j := range a[c] {
			a[c][j] /= pivot
		}
		for r := 0; r < n; r++ {
			if r == c {
				continue
			}
			f := a[r][c]
			for j := range a[r] {
				a[r][j] -= f * a[c][j]
			}
		}
	}
	out := make([][]float64, n)
	for i := range a {
		out[i] = a[i][n:]
	}
	return out, true
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func split(list string) []string {
	var out []string
	for _, part := range strings.Split(list, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func short(opponent string) string { return strings.TrimPrefix(opponent, "drowned_oracle_") }

func title(opponent string) string {
	words := strings.Split(short(opponent), "_")
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func defaultServerRoot() string {
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "content", "battle_v1")); err == nil {
				return dir
			}
			if _, err := os.Stat(filepath.Join(dir, "dice-and-destiny-server", "content", "battle_v1")); err == nil {
				return filepath.Join(dir, "dice-and-destiny-server")
			}
		}
	}
	return "."
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "board-lab:", err)
	os.Exit(1)
}
