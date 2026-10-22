package main

// The pure duel (-pure) measures the boards alone. Both sides are the custom
// adventurer's resources with a deck of blank cards (health only, nothing to
// play), the same defense and no riders; the opponent is a reference board
// (by default the Standard template on the standard die). Only the board and
// its die differ, so a board priced level with the reference should score
// 0.5. The battles run on a scratch copy of the content that adds the blank
// card and the reference character; the real content is never written.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/learned"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/content"
)

const (
	pureBlankCard = "zz_blank"
	// pureBlankCopies is the most copies of one card a deck may hold; the
	// blank deck spreads its health over as many blank cards as it needs.
	pureBlankCopies = 20
	pureReference   = "zz_reference"
	pureRefPrefix   = "zzref_"
	// pureNoDefense is Holdfast without its prevention (the engine needs a
	// defense on every board): -defense none.
	pureNoDefense = "zz_no_defense"
)

// PureBoard is one board's pure-duel result.
type PureBoard struct {
	Choice  boardprice.Choice `json:"choice"`
	Family  string            `json:"family"`
	Average float64           `json:"whole_average"`
	NoHit   float64           `json:"whole_no_hit"`
	Spread  float64           `json:"damage_sd"`
	Net     float64           `json:"net_average"` // after the reference's defense roll
	// Race is the score the race model predicts: both sides attack every
	// round with the priced odds, through the other's defense roll.
	Race      float64 `json:"race"`
	Score     float64 `json:"score"`
	Draws     float64 `json:"draws"`
	Battles   int     `json:"battles"`
	Rounds    float64 `json:"mean_rounds"`
	HealthEnd float64 `json:"mean_health_left"`
	FoeEnd    float64 `json:"mean_foe_health_left"`
}

type pureLab struct {
	lab
	health    int
	defenses  []string
	reference boardprice.Choice
	refDie    boardprice.Die
	refReport boardprice.Report
	prevent   []float64 // the reference's defense: chance of preventing k
	refOdds   []float64 // the reference's damage per turn
}

// runPure plays every die × template at default picks against the
// reference board.
func runPure(l lab, health int, reference, defense string, battles int, only string, out, markdown string) {
	started := time.Now()
	refTemplate, refDie, ok := strings.Cut(reference, ":")
	if !ok {
		fail(fmt.Errorf("-reference is template:die, got %q", reference))
	}
	p := pureLab{lab: l, health: health, defenses: []string{defense}}
	if defense == "none" {
		p.defenses = []string{pureNoDefense}
	}
	var err error
	if p.reference, p.refReport, err = l.builder.Price(l.builder.Default(refTemplate, refDie)); err != nil {
		fail(err)
	}
	if p.refDie, err = l.builder.Die(p.reference.Die); err != nil {
		fail(err)
	}
	scratch, err := os.MkdirTemp("", "board-lab-pure-content-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(scratch)
	if err := p.writeContent(scratch); err != nil {
		fail(err)
	}
	p.contentRoot = scratch
	p.prevent = p.defenseOdds(p.refDie)
	p.refOdds = damageOdds(p.refReport)

	var boards []PureBoard
	// The control: the reference board against itself.
	control := p.evaluate(p.reference, battles)
	fmt.Fprintf(os.Stderr, "control: reference vs itself %.3f (%s)\n", control.Score, time.Since(started).Round(time.Second))
	for _, tpl := range l.builder.Menus.Templates {
		for _, entry := range l.builder.Menus.Dice {
			die, err := l.builder.Die(entry.ID)
			if err != nil {
				fail(err)
			}
			if die.Layout.Family != tpl.Family || (only != "" && !strings.Contains(","+only+",", ","+tpl.ID+",")) {
				continue
			}
			b := p.evaluate(l.builder.Default(tpl.ID, die.ID), battles)
			boards = append(boards, b)
			fmt.Fprintf(os.Stderr, "%-44s avg %.2f net %.2f  score %.3f  rounds %.1f  (%s)\n", b.Choice.Template+" · "+b.Choice.Die, b.Average, b.Net, b.Score, b.Rounds, time.Since(started).Round(time.Second))
		}
	}
	text := p.summarize(control, boards, battles, defense)
	fmt.Println(text)
	if markdown != "" {
		if err := os.WriteFile(markdown, []byte(text), 0o644); err != nil {
			fail(err)
		}
	}
	if out != "" {
		raw, err := json.MarshalIndent(map[string]any{"control": control, "boards": boards}, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(out, raw, 0o644); err != nil {
			fail(err)
		}
	}
}

// writeContent copies the content to a scratch root and adds the blank card
// and the reference character (the custom adventurer's resources, the
// reference board's generated abilities under their own IDs, the defense and
// a blank deck) to the custom adventurer's pack there.
func (p pureLab) writeContent(scratch string) error {
	if err := copyTree(p.contentRoot, scratch); err != nil {
		return err
	}
	pack := filepath.Join(scratch, battle.CustomAdventurerPack)
	for i, entry := range p.deck() {
		blank := map[string]any{
			"schema_version":  1,
			"id":              entry.CardID,
			"name":            fmt.Sprintf("Blank %d", i+1),
			"type":            "blank",
			"presentation":    map[string]any{"rules_text": "Blank. This card has no effect and cannot be played; it only counts as health."},
			"cost":            map[string]any{"energy": 0},
			"play":            map[string]any{"source_zones": []string{"hand"}, "destination": "discard", "playable_during": []any{}},
			"targeting":       map[string]any{"selector": "self", "minimum": 0, "maximum": 1},
			"reaction_window": map[string]any{"opens": false},
			"operations":      []any{},
		}
		if err := writeJSON(filepath.Join(pack, "cards", entry.CardID+".yaml"), blank); err != nil {
			return err
		}
	}
	catalogs, err := learned.CharacterCatalogs(p.contentRoot)
	if err != nil {
		return err
	}
	lib := catalogs[loadout.CustomAdventurer]
	if p.defenses[0] == pureNoDefense {
		if err := writeNoDefense(filepath.Join(pack, "abilities", pureNoDefense+".yaml"), lib.Abilities["holdfast"]); err != nil {
			return err
		}
	}
	abilities, board, err := p.builder.Generate(p.reference, p.refReport)
	if err != nil {
		return err
	}
	rename := func(id string) string { return pureRefPrefix + strings.TrimPrefix(id, boardprice.GeneratedPrefix) }
	for _, a := range abilities {
		a.ID, a.Name = rename(a.ID), "Reference "+a.Name
		if err := writeJSON(filepath.Join(pack, "abilities", a.ID+".yaml"), a); err != nil {
			return err
		}
	}
	ref := lib.Combatants[loadout.CustomAdventurer]
	ref.ID, ref.Name = pureReference, "Zz Reference"
	ref.DiceLoadout = []content.DiceLoadoutEntry{{DiceID: p.reference.Die, Count: content.LoadoutDiceCount}}
	ref.AbilityBoard = content.AbilityBoard{Defensive: p.defenses}
	for _, id := range board.Offensive {
		ref.AbilityBoard.Offensive = append(ref.AbilityBoard.Offensive, rename(id))
	}
	ref.Decklist = nil
	for _, entry := range p.deck() {
		ref.Decklist = append(ref.Decklist, content.DecklistEntry{CardID: entry.CardID, Count: entry.Count})
	}
	raw, err := yaml.Marshal(ref)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pack, "combatants", pureReference+".yaml"), raw, 0o644)
}

// writeNoDefense writes Holdfast with every prevention replaced by 1 energy,
// which blank decks cannot spend: it rolls and prevents nothing.
func writeNoDefense(path string, holdfast content.BattleAbilityDefinition) error {
	raw, err := json.Marshal(holdfast)
	if err != nil {
		return err
	}
	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		return err
	}
	def["id"], def["name"] = pureNoDefense, "No Defense"
	def["presentation"] = map[string]any{"rules_text": "Rolls 3 dice and prevents nothing; each symbol gains energy."}
	resolution, _ := def["resolution"].(map[string]any)
	operations, _ := resolution["operations"].([]any)
	if len(operations) != 1 {
		return fmt.Errorf("holdfast: expected one roll, got %d operations", len(operations))
	}
	roll, _ := operations[0].(map[string]any)
	outcomes, _ := roll["outcomes"].([]any)
	for _, o := range outcomes {
		outcome, _ := o.(map[string]any)
		ops, _ := outcome["operations"].([]any)
		for i, op := range ops {
			if m, _ := op.(map[string]any); m["type"] == "prevent_damage" {
				ops[i] = map[string]any{"type": "gain_resource", "target": "self", "amount": 1, "resource": "energy"}
			}
		}
	}
	return writeJSON(path, def)
}

// deck is the blank deck: health cards over as few blank cards as the
// per-card limit allows.
func (p pureLab) deck() []loadout.Entry {
	var out []loadout.Entry
	for left, i := p.health, 1; left > 0; i++ {
		n := min(left, pureBlankCopies)
		out = append(out, loadout.Entry{CardID: fmt.Sprintf("%s_%d", pureBlankCard, i), Count: n})
		left -= n
	}
	return out
}

// evaluate prices a board, writes it with the duel's defense to a scratch
// sheet and plays it against the reference from both seats.
func (p pureLab) evaluate(choice boardprice.Choice, battles int) PureBoard {
	resolved, report, err := p.builder.Price(choice)
	if err != nil {
		fail(fmt.Errorf("%+v: %w", choice, err))
	}
	die, _ := p.builder.Die(resolved.Die)
	root, err := os.MkdirTemp("", "board-lab-sheet-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(root)
	abilities, board, err := p.builder.Generate(resolved, report)
	if err != nil {
		fail(err)
	}
	generated := map[string]content.BattleAbilityDefinition{}
	for _, a := range abilities {
		generated[a.ID] = a
	}
	board.Defensive = p.defenses
	if err := p.writeBoard(root, loadout.SheetBoard{Dice: resolved.Die, Abilities: board, Generated: generated}); err != nil {
		fail(fmt.Errorf("%+v: %w", resolved, err))
	}
	odds := damageOdds(report)
	heroDefense := p.defenseOdds(die)
	b := PureBoard{
		Choice: resolved, Family: die.Layout.Family, Average: report.WholeAverage, NoHit: report.WholeNoHit,
		Spread: spread(odds), Net: through(odds, p.prevent), Race: race(defend(odds, p.prevent), defend(p.refOdds, heroDefense), p.health),
	}
	if battles == 0 {
		return b
	}
	runState, err := os.MkdirTemp("", "board-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	deck := p.deck()
	var records []lookahead.BattleRecord
	for _, seatB := range []bool{false, true} {
		m := lookahead.Matchup{ContentRoot: p.contentRoot, AuthoredRoot: root, RunStateRoot: runState, Hero: loadout.CustomAdventurer, Opponent: pureReference, Unified: true, HeroPlansFirst: true, HeroInSeatB: seatB, HeroDeck: deck, FoeDeck: deck}
		summary, err := lookahead.RunBattles(m, greedy, greedy, p.first, battles/2, p.workers, nil)
		if err != nil {
			fail(fmt.Errorf("%s · %s: %w", resolved.Template, resolved.Die, err))
		}
		records = append(records, summary.Records...)
	}
	s := lookahead.Summarize(records)
	b.Score, b.Battles, b.Draws = s.Score, s.Battles, float64(s.Draws)/float64(s.Battles)
	b.Rounds, b.HealthEnd, b.FoeEnd = s.MeanRounds, s.MeanHealth, s.MeanFoeHealth
	return b
}

// damageOdds is a board's damage per offensive turn under the pricing's
// play: odds[d] is the chance of dealing d (a miss deals 0).
func damageOdds(r boardprice.Report) []float64 {
	odds := []float64{1}
	for _, t := range r.Tiers {
		for len(odds) <= t.WholeDamage {
			odds = append(odds, 0)
		}
		odds[t.WholeDamage] += t.UseWhole
		odds[0] -= t.UseWhole
	}
	odds[0] = math.Max(0, odds[0])
	return odds
}

// defend is the damage that gets through a defense roll, prevent[k] being
// the chance it prevents k.
func defend(odds, prevent []float64) []float64 {
	out := make([]float64, len(odds))
	for d, pd := range odds {
		for k, pk := range prevent {
			out[max(0, d-k)] += pd * pk
		}
	}
	return out
}

// through is the average damage through a defense.
func through(odds, prevent []float64) float64 {
	total := 0.0
	for d, p := range defend(odds, prevent) {
		total += float64(d) * p
	}
	return total
}

// spread is the standard deviation of the damage per turn.
func spread(odds []float64) float64 {
	m, square := 0.0, 0.0
	for d, p := range odds {
		m += float64(d) * p
		square += float64(d*d) * p
	}
	return math.Sqrt(math.Max(0, square-m*m))
}

// killRounds is the chance that damage per round drawn from odds first
// reaches health on round t (index t-1), up to the rounds where almost
// nothing is left alive.
func killRounds(odds []float64, health int) []float64 {
	alive := make([]float64, health) // alive[h]: dealt h so far, h < health
	alive[0] = 1
	var out []float64
	for t := 0; t < 500; t++ {
		next := make([]float64, health)
		killed := 0.0
		for h, ph := range alive {
			if ph == 0 {
				continue
			}
			for d, pd := range odds {
				if h+d >= health {
					killed += ph * pd
				} else {
					next[h+d] += ph * pd
				}
			}
		}
		out = append(out, killed)
		alive = next
		left := 0.0
		for _, ph := range alive {
			left += ph
		}
		if left < 1e-12 {
			break
		}
	}
	return out
}

// race is the expected score of a side whose damage through the other's
// defense is `ours` against one dealing `theirs`, both attacking every round
// and starting at `health`: a win if it reaches the other's health first, a
// half if both fall in the same round.
func race(ours, theirs []float64, health int) float64 {
	a, b := killRounds(ours, health), killRounds(theirs, health)
	theirLeft := 1.0 // chance they have not killed us before round t
	score := 0.0
	for t := 0; t < len(a); t++ {
		bt := 0.0
		if t < len(b) {
			bt = b[t]
		}
		score += a[t] * (theirLeft - bt + 0.5*bt)
		theirLeft -= bt
	}
	return score
}

// defenseOdds is the distribution of what the duel's defense prevents when
// rolled on a die: three dice, main and odd faces prevent 1, side 2, the
// second odd (3-1-1-1) 3 and apex 0 — Holdfast's outcomes by role. Without
// a defense it always prevents 0.
func (p pureLab) defenseOdds(die boardprice.Die) []float64 {
	if p.defenses[0] == pureNoDefense {
		return []float64{1}
	}
	prevents := map[string]float64{"main": 1, "odd": 1, "side": 2, "odd_b": 3, "apex": 0}
	var face []float64
	for _, f := range die.Faces {
		face = append(face, prevents[die.Layout.Roles[f.Symbol]])
	}
	dist := []float64{1}
	for i := 0; i < 3; i++ {
		next := make([]float64, len(dist)+3)
		for k, pk := range dist {
			for _, v := range face {
				next[k+int(v)] += pk / float64(len(face))
			}
		}
		dist = next
	}
	return dist
}

func (p pureLab) summarize(control PureBoard, boards []PureBoard, battles int, defense string) string {
	var sb strings.Builder
	refTpl, _ := p.builder.Menus.Template(p.reference.Template)
	fmt.Fprintf(&sb, "### Pure board duel\n\n")
	fmt.Fprintf(&sb, "`go run ./cmd/board-lab -pure` · both sides: %d blank cards (health only), the custom adventurer's resources, defense %s, no riders · opponent: the %s template on the %s (%.2f per turn) · greedy players · paired seeds, %d battles per board (95%% interval about ±%.3f).\n\n",
		p.health, defense, refTpl.Name, p.refDie.Name, p.refReport.WholeAverage, battles, 1.96*math.Sqrt(0.25/float64(battles)))
	fmt.Fprintf(&sb, "Control: the reference board against itself scores %.3f (race model %.3f).\n\n", control.Score, control.Race)
	fmt.Fprintf(&sb, "Avg = damage per offensive turn at the priced play; Net = the same after the reference's defense roll; SD = spread of the damage per turn; Race = the score the race model predicts (both sides attack every round with the priced odds, through the other's defense roll); Rounds = mean battle length.\n\n")
	fmt.Fprintf(&sb, "| Template | Die | Avg | Net | No hit | SD | Race | Score | Score − Race | Rounds |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, b := range boards {
		tpl, _ := p.builder.Menus.Template(b.Choice.Template)
		die, _ := p.builder.Die(b.Choice.Die)
		fmt.Fprintf(&sb, "| %s | %s %s | %.2f | %.2f | %.1f%% | %.2f | %.3f | %.3f | %+.3f | %.1f |\n", tpl.Name, strings.TrimSuffix(die.Name, " Die"), die.Layout.Order, b.Average, b.Net, b.NoHit*100, b.Spread, b.Race, b.Score, b.Score-b.Race, b.Rounds)
	}
	var avg, net, sd, raced, score []float64
	for _, b := range boards {
		avg, net, sd, raced, score = append(avg, b.Average), append(net, b.Net), append(sd, b.Spread), append(raced, b.Race), append(score, b.Score)
	}
	fmt.Fprintf(&sb, "\nCorrelation of the score with Avg %.2f, Net %.2f, SD %.2f, Race %.2f.\n", correlation(avg, score), correlation(net, score), correlation(sd, score), correlation(raced, score))
	families := map[string][]float64{}
	for _, b := range boards {
		families[b.Family] = append(families[b.Family], b.Score)
	}
	keys := make([]string, 0, len(families))
	for k := range families {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(&sb, "Mean score by family:")
	for _, k := range keys {
		fmt.Fprintf(&sb, " {%s} %.3f;", k, mean(families[k]))
	}
	fmt.Fprintf(&sb, "\n")
	return sb.String()
}

func correlation(x, y []float64) float64 {
	mx, my := mean(x), mean(y)
	sxy, sxx, syy := 0.0, 0.0, 0.0
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0
	}
	return sxy / math.Sqrt(sxx*syy)
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}
