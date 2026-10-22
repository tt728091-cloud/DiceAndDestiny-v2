package main

// The round robin (-roundrobin) scores every default board against every
// other with the cards taken out (duel.go), under each duel defense (none,
// Light Guard, Holdfast), by the race model, each cell averaged over a range
// of starting health. A seeded sample of the pairings is also played as real
// battles to check the model. Then it tunes the field: every board's damage
// target moves, all at once and for a few rounds, until each board's overall
// score (the mean over defenses, opponents and health) is 0.5 against the
// rest, and the tuned field is scored again. The tuned targets show how far
// each board is from level; what stays lopsided after tuning is the board's
// shape, which damage alone cannot fix.

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle/boardprice"
)

// A matchup the other side wins this often or more is lopsided.
const lopsided = 0.40

// rrBoard is one board of the field at its current target.
type rrBoard struct {
	Label   string            `json:"label"`
	Choice  boardprice.Choice `json:"choice"`
	Family  string            `json:"family"`
	Target  float64           `json:"target"`
	Average float64           `json:"whole_average"`
	NoHit   float64           `json:"whole_no_hit"`
	Damage  map[string]int    `json:"damage"` // tier → whole damage
	die     boardprice.Die
	board   boardprice.Board
	report  boardprice.Report
	odds    []float64
}

// rrSummary is one board's record against the field.
type rrSummary struct {
	Label     string             `json:"label"`
	Target    float64            `json:"target"`
	Average   float64            `json:"whole_average"`
	ByDefense map[string]float64 `json:"by_defense"` // mean score against the others
	Overall   float64            `json:"overall"`
	Worst     float64            `json:"worst"`
	WorstVs   string             `json:"worst_vs"`
	Best      float64            `json:"best"`
	BestVs    string             `json:"best_vs"`
	Lopsided  int                `json:"lopsided_losses"` // cells below `lopsided`
	Cells     int                `json:"cells"`
}

// rrField is a field of boards scored against each other.
type rrField struct {
	Name      string                 `json:"name"`
	Boards    []rrBoard              `json:"boards"`
	Matrices  map[string][][]float64 `json:"matrices"` // defense → [i][j], i's score against j
	Summaries []rrSummary            `json:"summaries"`
}

// rrCheck is one sampled pairing played as real battles.
type rrCheck struct {
	Board    string  `json:"board"`
	Opponent string  `json:"opponent"`
	Defense  string  `json:"defense"`
	Health   int     `json:"health"`
	Model    float64 `json:"model"`
	Score    float64 `json:"score"`
	Battles  int     `json:"battles"`
}

type roundRobin struct {
	lab
	healths  []int
	defenses []string
}

func runRoundRobin(l lab, healths string, rounds, sample, battles int, seed int64, out, markdown string) {
	started := time.Now()
	rr := roundRobin{lab: l, defenses: duelDefenses}
	var err error
	if rr.healths, err = parseRange(healths); err != nil {
		fail(err)
	}
	now := rr.field("Priced now", rr.defaultBoards())
	fmt.Fprintf(os.Stderr, "field scored (%s)\n", time.Since(started).Round(time.Second))
	ruled := rr.field("Priced on damage through the defenses", rr.ruled(now.Boards))
	tuned := rr.tune(now.Boards, rounds, started)
	tunedField := rr.field("Tuned", tuned)
	var checks []rrCheck
	if sample > 0 && battles > 0 {
		checks = rr.check(now.Boards, sample, battles, seed, started)
	}
	text := rr.summarize(now, ruled, tunedField, checks, rounds, battles)
	fmt.Println(text)
	if markdown != "" {
		if err := os.WriteFile(markdown, []byte(text), 0o644); err != nil {
			fail(err)
		}
	}
	if out != "" {
		if err := writeJSON(out, map[string]any{"healths": rr.healths, "defenses": rr.defenses, "fields": []rrField{now, ruled, tunedField}, "checks": checks}); err != nil {
			fail(err)
		}
	}
}

// defaultBoards is every die × template at default picks, priced now.
func (rr roundRobin) defaultBoards() []rrBoard {
	var boards []rrBoard
	for _, tpl := range rr.builder.Menus.Templates {
		for _, entry := range rr.builder.Menus.Dice {
			die, err := rr.builder.Die(entry.ID)
			if err != nil {
				fail(err)
			}
			if die.Layout.Family != tpl.Family {
				continue
			}
			choice, die, board, err := rr.builder.Resolve(rr.builder.Default(tpl.ID, die.ID))
			if err != nil {
				fail(err)
			}
			b := rrBoard{Label: tpl.Name + " · " + strings.TrimSuffix(die.Name, " Die"), Choice: choice, Family: die.Layout.Family, die: die, board: board}
			if err := b.price(rr.builder.Menus.Options().Target, rr.builder); err != nil {
				fail(fmt.Errorf("%s: %w", b.Label, err))
			}
			boards = append(boards, b)
		}
	}
	return boards
}

// price reprices a board at a damage target.
func (b *rrBoard) price(target float64, builder *boardprice.Builder) error {
	opts := builder.Menus.Options()
	opts.Target = target
	report, err := boardprice.Price(b.die, b.board, opts)
	if err != nil {
		return err
	}
	b.Target, b.report, b.odds = target, report, damageOdds(report)
	b.Average, b.NoHit = report.WholeAverage, report.WholeNoHit
	b.Damage = map[string]int{}
	for _, t := range report.Tiers {
		b.Damage[t.Tier] = t.WholeDamage
	}
	return nil
}

// score is a's expected score against b under a defense, at one health.
func score(a, b *rrBoard, defense string, health int) float64 {
	return race(defend(a.odds, defenseOdds(defense, b.die)), defend(b.odds, defenseOdds(defense, a.die)), health)
}

// cell is a's score against b under a defense, averaged over the healths.
func (rr roundRobin) cell(a, b *rrBoard, defense string) float64 {
	ours, theirs := defend(a.odds, defenseOdds(defense, b.die)), defend(b.odds, defenseOdds(defense, a.die))
	total := 0.0
	for _, h := range rr.healths {
		total += race(ours, theirs, h)
	}
	return total / float64(len(rr.healths))
}

// overall is a board's mean score against the others over every defense.
func (rr roundRobin) overall(a *rrBoard, field []rrBoard, self int) float64 {
	total, n := 0.0, 0
	for _, d := range rr.defenses {
		for j := range field {
			if j != self {
				total += rr.cell(a, &field[j], d)
				n++
			}
		}
	}
	return total / float64(n)
}

// field scores every pairing under every defense and summarizes each board.
func (rr roundRobin) field(name string, boards []rrBoard) rrField {
	f := rrField{Name: name, Boards: boards, Matrices: map[string][][]float64{}}
	n := len(boards)
	for _, d := range rr.defenses {
		m := make([][]float64, n)
		for i := range m {
			m[i] = make([]float64, n)
		}
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				m[i][i] = 0.5
				for j := i + 1; j < n; j++ {
					m[i][j] = rr.cell(&boards[i], &boards[j], d)
				}
			}(i)
		}
		wg.Wait()
		for i := 0; i < n; i++ {
			for j := 0; j < i; j++ {
				m[i][j] = 1 - m[j][i]
			}
		}
		f.Matrices[d] = m
	}
	for i, b := range boards {
		s := rrSummary{Label: b.Label, Target: b.Target, Average: b.Average, ByDefense: map[string]float64{}, Worst: 2, Best: -1}
		for _, d := range rr.defenses {
			total := 0.0
			for j := range boards {
				if j == i {
					continue
				}
				v := f.Matrices[d][i][j]
				total += v
				s.Cells++
				if v < lopsided {
					s.Lopsided++
				}
				if v < s.Worst {
					s.Worst, s.WorstVs = v, boards[j].Label+" ("+defenseName(d)+")"
				}
				if v > s.Best {
					s.Best, s.BestVs = v, boards[j].Label+" ("+defenseName(d)+")"
				}
			}
			s.ByDefense[d] = total / float64(n-1)
			s.Overall += s.ByDefense[d] / float64(len(rr.defenses))
		}
		f.Summaries = append(f.Summaries, s)
	}
	return f
}

// tune moves every board's target, all at once, until each one's overall
// score against the current field is 0.5, for the given rounds.
func (rr roundRobin) tune(start []rrBoard, rounds int, started time.Time) []rrBoard {
	field := append([]rrBoard(nil), start...)
	for round := 1; round <= rounds; round++ {
		next := append([]rrBoard(nil), field...)
		var wg sync.WaitGroup
		for i := range field {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				next[i] = rr.level(field, i)
			}(i)
		}
		wg.Wait()
		field = next
		worst := 0.0
		for i := range field {
			worst = math.Max(worst, math.Abs(rr.overall(&field[i], field, i)-0.5))
		}
		fmt.Fprintf(os.Stderr, "tuning round %d: largest overall gap from 0.5 is %.3f (%s)\n", round, worst, time.Since(started).Round(time.Second))
	}
	return field
}

// level finds the target at which board i's overall score against the rest
// of the field is closest to 0.5 (bisection; whole-number rounding makes the
// score move in steps, so the closest target tried wins).
func (rr roundRobin) level(field []rrBoard, i int) rrBoard {
	best, bestGap := field[i], math.Inf(1)
	lo, hi := 3.0, 12.0
	for step := 0; step < 18; step++ {
		t := (lo + hi) / 2
		b := field[i]
		if err := b.price(t, rr.builder); err != nil {
			fail(fmt.Errorf("%s at target %.3f: %w", b.Label, t, err))
		}
		v := rr.overall(&b, field, i)
		if gap := math.Abs(v - 0.5); gap < bestGap {
			best, bestGap = b, gap
		}
		if v < 0.5 {
			lo = t
		} else {
			hi = t
		}
	}
	return best
}

// reference is the index of the Standard template on the standard die.
func (rr roundRobin) reference(field []rrBoard) int {
	for i, b := range field {
		if b.Choice.Template == "standard" && b.Choice.Die == "standard_d6" {
			return i
		}
	}
	fail(fmt.Errorf("the field has no Standard template on the standard die"))
	return -1
}

// throughDefenses is a board's average damage per turn through each duel
// defense rolled on a die, averaged over the defenses.
func (rr roundRobin) throughDefenses(b *rrBoard, die boardprice.Die) float64 {
	total := 0.0
	for _, d := range rr.defenses {
		total += through(b.odds, defenseOdds(d, die))
	}
	return total / float64(len(rr.defenses))
}

// ruled prices every board by one rule instead of tuning it against the
// field: its target is set so that its damage through the duel defenses
// (rolled on the standard die, averaged over the defenses) matches the
// reference board's priced now.
func (rr roundRobin) ruled(start []rrBoard) []rrBoard {
	ref := start[rr.reference(start)]
	want := rr.throughDefenses(&ref, ref.die)
	out := append([]rrBoard(nil), start...)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			best, bestGap := out[i], math.Inf(1)
			lo, hi := 3.0, 12.0
			for step := 0; step < 18; step++ {
				t := (lo + hi) / 2
				b := start[i]
				if err := b.price(t, rr.builder); err != nil {
					fail(fmt.Errorf("%s at target %.3f: %w", b.Label, t, err))
				}
				v := rr.throughDefenses(&b, ref.die)
				if gap := math.Abs(v - want); gap < bestGap {
					best, bestGap = b, gap
				}
				if v < want {
					lo = t
				} else {
					hi = t
				}
			}
			out[i] = best
		}(i)
	}
	wg.Wait()
	return out
}

// check plays a seeded sample of pairings as real battles, per defense, at a
// random health from the range, beside the race model at that health.
func (rr roundRobin) check(boards []rrBoard, sample, battles int, seed int64, started time.Time) []rrCheck {
	rng := rand.New(rand.NewSource(seed))
	var checks []rrCheck
	maxHealth := rr.healths[len(rr.healths)-1]
	for _, d := range rr.defenses {
		var duelists []duelist
		for i, b := range boards {
			duelists = append(duelists, duelist{ID: fmt.Sprintf("zz_rr_%02d", i), Prefix: fmt.Sprintf("zzrr%02d_", i), Label: fmt.Sprintf("RR%02d", i), Choice: b.Choice, Report: b.report})
		}
		scratch, err := os.MkdirTemp("", "board-lab-rr-content-")
		if err != nil {
			fail(err)
		}
		if err := rr.writeScratch(scratch, maxHealth, d, duelists); err != nil {
			fail(err)
		}
		seen := map[[2]int]bool{}
		for len(seen) < sample {
			i, j := rng.Intn(len(boards)), rng.Intn(len(boards))
			if i == j || seen[[2]int{i, j}] {
				continue
			}
			seen[[2]int{i, j}] = true
			h := rr.healths[rng.Intn(len(rr.healths))]
			s := rr.playDuel(scratch, h, d, boards[i].Choice, boards[i].report, duelists[j].ID, battles)
			c := rrCheck{Board: boards[i].Label, Opponent: boards[j].Label, Defense: d, Health: h, Model: score(&boards[i], &boards[j], d, h), Score: s.Score, Battles: s.Battles}
			checks = append(checks, c)
			fmt.Fprintf(os.Stderr, "check %-9s %-24s vs %-24s health %2d  model %.3f  battles %.3f (%s)\n", d, c.Board, c.Opponent, h, c.Model, c.Score, time.Since(started).Round(time.Second))
		}
		os.RemoveAll(scratch)
	}
	return checks
}

func defenseName(d string) string {
	switch d {
	case "none":
		return "no defense"
	case "light":
		return "Light Guard"
	case "holdfast":
		return "Holdfast"
	}
	return d
}

func (rr roundRobin) summarize(now, ruled, tuned rrField, checks []rrCheck, rounds, battles int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### Board round robin\n\n")
	fmt.Fprintf(&sb, "`go run ./cmd/board-lab -roundrobin` · every default board against every other · blank decks (health only), no riders, the same defense on both sides · race model (both sides attack every round with the priced odds per tier, through the other's defense roll), each cell averaged over starting health %d–%d · score = wins + ½ draws.\n\n", rr.healths[0], rr.healths[len(rr.healths)-1])
	fmt.Fprintf(&sb, "Defenses: **no defense**; **Light Guard** (1 die: 1–2 prevent nothing, 3–4 prevent 1, 5–6 prevent 2; 1 on average); **Holdfast** (3 of your dice by role, 3.5 on average, up to 6, or 9 on a 3-1-1-1 die). A matchup below %.2f (the other side wins %.0f%% or more) counts as lopsided.\n\n", lopsided, (1-lopsided)*100)
	if len(checks) > 0 {
		var diffs []float64
		for _, c := range checks {
			diffs = append(diffs, c.Score-c.Model)
		}
		rms := 0.0
		for _, d := range diffs {
			rms += d * d
		}
		rms = math.Sqrt(rms / float64(len(diffs)))
		fmt.Fprintf(&sb, "**Model check:** %d sampled pairings played as real battles (%d each, so ±%.3f per cell): typical gap from the model %.3f, largest %.3f.\n\n", len(checks), battles, 1.96*math.Sqrt(0.25/float64(battles)), rms, maxAbs(diffs))
	}
	rr.writeSummary(&sb, now, nil)
	fmt.Fprintf(&sb, "\n**Priced on damage through the defenses** (one rule for any board, not tuned against the field): every board's target set so that its average damage per turn through the three defenses, rolled on the standard die and averaged over them, matches the Standard template's on the standard die priced now (%.2f).\n\n", rr.throughDefenses(&now.Boards[rr.reference(now.Boards)], now.Boards[rr.reference(now.Boards)].die))
	rr.writeSummary(&sb, ruled, &now)
	fmt.Fprintf(&sb, "\n**Tuned:** every board's pricing target moved, all at once over %d rounds, until its overall score is as close to 0.5 as whole damage allows. Change is the average damage per turn against the board priced now.\n\n", rounds)
	rr.writeSummary(&sb, tuned, &now)
	for _, f := range []rrField{now, ruled, tuned} {
		for _, d := range rr.defenses {
			rr.writeMatrix(&sb, f, d)
		}
	}
	if len(checks) > 0 {
		fmt.Fprintf(&sb, "\n**Model check, every sampled pairing:**\n\n| Board | Opponent | Defense | Health | Model | Battles | Gap |\n|---|---|---|---|---|---|---|\n")
		for _, c := range checks {
			fmt.Fprintf(&sb, "| %s | %s | %s | %d | %.3f | %.3f | %+.3f |\n", c.Board, c.Opponent, defenseName(c.Defense), c.Health, c.Model, c.Score, c.Score-c.Model)
		}
	}
	return sb.String()
}

// writeSummary writes a field's summary; against a base field it adds each
// board's change in average damage.
func (rr roundRobin) writeSummary(sb *strings.Builder, f rrField, base *rrField) {
	tuned := base != nil
	fmt.Fprintf(sb, "**%s** — each board's mean score against the other %d boards:\n\n| # | Board | Avg / turn |", f.Name, len(f.Boards)-1)
	if tuned {
		fmt.Fprintf(sb, " Change |")
	}
	for _, d := range rr.defenses {
		fmt.Fprintf(sb, " %s |", defenseName(d))
	}
	fmt.Fprintf(sb, " Overall | Worst matchup | Lopsided losses |\n|---|---|---|")
	if tuned {
		fmt.Fprintf(sb, "---|")
	}
	for range rr.defenses {
		fmt.Fprintf(sb, "---|")
	}
	fmt.Fprintf(sb, "---|---|---|\n")
	for i, s := range f.Summaries {
		fmt.Fprintf(sb, "| %d | %s | %.2f |", i+1, s.Label, s.Average)
		if tuned {
			fmt.Fprintf(sb, " %+.1f%% |", (s.Average/base.Summaries[i].Average-1)*100)
		}
		for _, d := range rr.defenses {
			fmt.Fprintf(sb, " %.3f |", s.ByDefense[d])
		}
		fmt.Fprintf(sb, " **%.3f** | %.3f vs %s | %d of %d |\n", s.Overall, s.Worst, s.WorstVs, s.Lopsided, s.Cells)
	}
	var overall []float64
	lop := 0
	for _, s := range f.Summaries {
		overall = append(overall, s.Overall)
		lop += s.Lopsided
	}
	sort.Float64s(overall)
	fmt.Fprintf(sb, "\nOverall scores span %.3f–%.3f; %d of the %d pairings (board pairs × defenses) are lopsided.\n", overall[0], overall[len(overall)-1], lop, len(f.Boards)*(len(f.Boards)-1)*len(rr.defenses)/2)
}

func (rr roundRobin) writeMatrix(sb *strings.Builder, f rrField, d string) {
	n := len(f.Boards)
	fmt.Fprintf(sb, "\n**%s, %s** — row board's score against the column board, in %%:\n\n| # | Board |", f.Name, defenseName(d))
	for j := 1; j <= n; j++ {
		fmt.Fprintf(sb, " %d |", j)
	}
	fmt.Fprintf(sb, " Mean |\n|---|---|")
	for j := 0; j <= n; j++ {
		fmt.Fprintf(sb, "---|")
	}
	fmt.Fprintf(sb, "\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(sb, "| %d | %s |", i+1, f.Boards[i].Label)
		for j := 0; j < n; j++ {
			if i == j {
				fmt.Fprintf(sb, " — |")
				continue
			}
			v := f.Matrices[d][i][j]
			if v < lopsided {
				fmt.Fprintf(sb, " **%.0f** |", v*100)
			} else {
				fmt.Fprintf(sb, " %.0f |", v*100)
			}
		}
		fmt.Fprintf(sb, " %.1f |\n", f.Summaries[i].ByDefense[d]*100)
	}
}

func maxAbs(values []float64) float64 {
	m := 0.0
	for _, v := range values {
		m = math.Max(m, math.Abs(v))
	}
	return m
}

// parseRange reads "12-30" or "15" as a list of healths.
func parseRange(s string) ([]int, error) {
	lo, hi, found := strings.Cut(s, "-")
	a, err := strconv.Atoi(strings.TrimSpace(lo))
	if err != nil {
		return nil, fmt.Errorf("health range %q: %w", s, err)
	}
	b := a
	if found {
		if b, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil {
			return nil, fmt.Errorf("health range %q: %w", s, err)
		}
	}
	if a < 1 || b < a {
		return nil, fmt.Errorf("health range %q is empty", s)
	}
	var out []int
	for h := a; h <= b; h++ {
		out = append(out, h)
	}
	return out, nil
}
