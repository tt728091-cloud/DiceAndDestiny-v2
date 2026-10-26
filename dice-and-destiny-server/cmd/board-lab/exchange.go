package main

// Exchange-rate research (-exchange-dump, -exchange-check): what one more
// point of average damage, a stronger defense or more blank cards is worth
// against each other. -exchange-dump writes every default board's damage per
// turn at targets across the staircase, and each duel defense's prevention
// on each die, for the race-model analysis in ml/scripts/power_exchange
// (docs/game-design/power-exchange-rates.md).
// -exchange-check plays listed points as real blank-deck battles: a
// challenger board at one target, health and defense against a rival at
// another, to check the model's exchange rates.
//
//	go run ./cmd/board-lab -exchange-dump /tmp/exchange.json
//	go run ./cmd/board-lab -exchange-check ml/scripts/power_exchange/checks-plan.json -battles 2000 -out /tmp/checks.json

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/content"
)

type exchangeBoard struct {
	Label    string      `json:"label"`
	Template string      `json:"template"`
	Die      string      `json:"die"`
	Targets  []float64   `json:"targets"`
	Averages []float64   `json:"averages"`
	Odds     [][]float64 `json:"odds"` // [target][damage]
}

type exchangeDump struct {
	Boards   []exchangeBoard                 `json:"boards"`
	Defenses map[string]map[string][]float64 `json:"defenses"` // die → defense → prevent odds
}

func runExchangeDump(l lab, step float64, out string) {
	rr := roundRobin{lab: l, defenses: duelDefenses, target: l.builder.Menus.Pricing.Target}
	opts := l.builder.Menus.Options()
	dump := exchangeDump{Defenses: map[string]map[string][]float64{}}
	for _, b := range rr.defaultBoards() {
		eb := exchangeBoard{Label: b.Label, Template: b.Choice.Template, Die: b.Choice.Die}
		for t := opts.From; t <= opts.To+1e-9; t += step {
			t = math.Round(t*1000) / 1000
			if err := b.price(t, l.builder); err != nil {
				fail(fmt.Errorf("%s at %.2f: %w", b.Label, t, err))
			}
			eb.Targets = append(eb.Targets, t)
			eb.Averages = append(eb.Averages, b.Average)
			eb.Odds = append(eb.Odds, b.odds)
		}
		dump.Boards = append(dump.Boards, eb)
		if dump.Defenses[b.die.ID] == nil {
			dump.Defenses[b.die.ID] = map[string][]float64{}
			for _, d := range duelDefenses {
				dump.Defenses[b.die.ID][d] = defenseOdds(d, b.die)
			}
		}
		fmt.Fprintf(os.Stderr, "%s\n", b.Label)
	}
	if err := writeJSON(out, dump); err != nil {
		fail(err)
	}
}

// exchangePoint is one real-battle check: the hero (challenger) against the
// rival, each with its own target, health and defense.
type exchangePoint struct {
	Name         string  `json:"name"`
	Template     string  `json:"template"`
	Die          string  `json:"die"`
	HeroTarget   float64 `json:"hero_target"`
	HeroHealth   int     `json:"hero_health"`
	HeroDefense  string  `json:"hero_defense"`
	RivalTarget  float64 `json:"rival_target"`
	RivalHealth  int     `json:"rival_health"`
	RivalDefense string  `json:"rival_defense"`
	Model        float64 `json:"model"`
	Score        float64 `json:"score"`
	Wins         int     `json:"wins"`
	Draws        int     `json:"draws"`
	Losses       int     `json:"losses"`
	Battles      int     `json:"battles"`
	MeanRounds   float64 `json:"mean_rounds"`
	HeroAverage  float64 `json:"hero_average"`
	RivalAverage float64 `json:"rival_average"`
}

func runExchangeCheck(l lab, plan string, battles int, out string) {
	started := time.Now()
	raw, err := os.ReadFile(plan)
	if err != nil {
		fail(err)
	}
	var points []exchangePoint
	if err := json.Unmarshal(raw, &points); err != nil {
		fail(err)
	}
	for i := range points {
		p := &points[i]
		choice := l.builder.Default(p.Template, p.Die)
		hero, heroReport, err := l.builder.PriceAt(choice, p.HeroTarget)
		if err != nil {
			fail(err)
		}
		rival, rivalReport, err := l.builder.PriceAt(choice, p.RivalTarget)
		if err != nil {
			fail(err)
		}
		die, _ := l.builder.Die(hero.Die)
		heroOdds, rivalOdds := damageOdds(heroReport), damageOdds(rivalReport)
		p.HeroAverage, p.RivalAverage = heroReport.WholeAverage, rivalReport.WholeAverage
		p.Model = raceRounds(killRounds(defend(heroOdds, defenseOdds(p.RivalDefense, die)), p.RivalHealth), killRounds(defend(rivalOdds, defenseOdds(p.HeroDefense, die)), p.HeroHealth))

		scratch, err := os.MkdirTemp("", "board-lab-exchange-content-")
		if err != nil {
			fail(err)
		}
		ref := duelist{ID: pureReference, Prefix: "zzref_", Label: "Rival", Choice: rival, Report: rivalReport}
		if err := l.writeScratch(scratch, max(p.HeroHealth, p.RivalHealth), p.RivalDefense, []duelist{ref}); err != nil {
			fail(err)
		}
		s := l.playExchange(scratch, hero, heroReport, p.HeroDefense, p.HeroHealth, p.RivalHealth, battles)
		os.RemoveAll(scratch)
		p.Score, p.Battles, p.MeanRounds = s.Score, s.Battles, s.MeanRounds
		p.Wins, p.Draws, p.Losses = s.Wins, s.Draws, s.Losses
		fmt.Fprintf(os.Stderr, "%-40s model %.3f battles %.3f (W/D/L %d/%d/%d) rounds %.2f (%s)\n", p.Name, p.Model, p.Score, p.Wins, p.Draws, p.Losses, p.MeanRounds, time.Since(started).Round(time.Second))
	}
	if out != "" {
		if err := writeJSON(out, points); err != nil {
			fail(err)
		}
	}
}

// playExchange is playDuel with the hero's own defense and the two sides'
// own healths.
func (l lab) playExchange(contentRoot string, choice boardprice.Choice, report boardprice.Report, defense string, heroHealth, rivalHealth, battles int) lookahead.Summary {
	root, err := os.MkdirTemp("", "board-lab-sheet-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(root)
	abilities, board, err := l.builder.Generate(choice, report)
	if err != nil {
		fail(err)
	}
	generated := map[string]content.BattleAbilityDefinition{}
	for _, a := range abilities {
		generated[a.ID] = a
	}
	board.Defensive = []string{defenseAbility(defense)}
	scratch := l
	scratch.contentRoot = contentRoot
	if err := scratch.writeBoard(root, loadout.SheetBoard{Dice: choice.Die, Abilities: board, Generated: generated}); err != nil {
		fail(fmt.Errorf("%+v: %w", choice, err))
	}
	runState, err := os.MkdirTemp("", "board-lab-run-state-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(runState)
	var records []lookahead.BattleRecord
	for _, seatB := range []bool{false, true} {
		m := lookahead.Matchup{ContentRoot: contentRoot, AuthoredRoot: root, RunStateRoot: runState, Hero: loadout.CustomAdventurer, Opponent: pureReference, Unified: true, HeroPlansFirst: true, HeroInSeatB: seatB, HeroDeck: blankDeck(heroHealth), FoeDeck: blankDeck(rivalHealth)}
		summary, err := lookahead.RunBattles(m, greedy, greedy, l.first, battles/2, l.workers, nil)
		if err != nil {
			fail(fmt.Errorf("%s: %w", strings.Join([]string{choice.Template, choice.Die}, " · "), err))
		}
		records = append(records, summary.Records...)
	}
	return lookahead.Summarize(records)
}
