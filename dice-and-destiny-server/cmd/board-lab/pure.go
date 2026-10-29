package main

// The pure duel (-pure) plays every default board against one reference
// board (by default the Standard template on the standard die) with the cards
// taken out (duel.go), beside the race model's prediction. A board priced
// level with the reference should score 0.5.

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/boardprice"
)

const pureReference = "zz_reference"

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
	defense   string
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
	p := pureLab{lab: l, health: health, defense: defense}
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
	ref := duelist{ID: pureReference, Prefix: "zzref_", Label: "Reference", Choice: p.reference, Report: p.refReport}
	if err := l.writeScratch(scratch, health, defense, []duelist{ref}); err != nil {
		fail(err)
	}
	p.contentRoot = scratch
	p.prevent = defenseOdds(defense, p.refDie)
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
	text := p.summarize(control, boards, battles)
	fmt.Println(text)
	if markdown != "" {
		if err := os.WriteFile(markdown, []byte(text), 0o644); err != nil {
			fail(err)
		}
	}
	if out != "" {
		if err := writeJSON(out, map[string]any{"control": control, "boards": boards}); err != nil {
			fail(err)
		}
	}
}

// evaluate prices a board, predicts its score with the race model and,
// with battles, plays it against the reference.
func (p pureLab) evaluate(choice boardprice.Choice, battles int) PureBoard {
	resolved, report, err := p.builder.Price(choice)
	if err != nil {
		fail(fmt.Errorf("%+v: %w", choice, err))
	}
	die, _ := p.builder.Die(resolved.Die)
	odds := damageOdds(report)
	b := PureBoard{
		Choice: resolved, Family: die.Layout.Family, Average: report.WholeAverage, NoHit: report.WholeNoHit,
		Spread: spread(odds), Net: through(odds, p.prevent), Race: race(defend(odds, p.prevent), defend(p.refOdds, defenseOdds(p.defense, die)), p.health),
	}
	if battles == 0 {
		return b
	}
	s := p.playDuel(p.contentRoot, p.health, p.defense, resolved, report, pureReference, battles)
	b.Score, b.Battles, b.Draws = s.Score, s.Battles, float64(s.Draws)/float64(s.Battles)
	b.Rounds, b.HealthEnd, b.FoeEnd = s.MeanRounds, s.MeanHealth, s.MeanFoeHealth
	return b
}

func (p pureLab) summarize(control PureBoard, boards []PureBoard, battles int) string {
	var sb strings.Builder
	refTpl, _ := p.builder.Menus.Template(p.reference.Template)
	fmt.Fprintf(&sb, "### Pure board duel\n\n")
	fmt.Fprintf(&sb, "`go run ./cmd/board-lab -pure` · both sides: %d blank cards (health only), the custom adventurer's resources, defense %s, no riders · opponent: the %s template on the %s (%.2f per turn) · greedy players · paired seeds, %d battles per board (95%% interval about ±%.3f).\n\n",
		p.health, p.defense, refTpl.Name, p.refDie.Name, p.refReport.WholeAverage, battles, 1.96*math.Sqrt(0.25/float64(max(1, battles))))
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
