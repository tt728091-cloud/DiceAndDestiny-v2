package lookahead_test

import (
	"math"
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/state"
)

// FreshTurn values a turn from public facts alone and agrees with Fresh on
// the real battle. With one roll its miss chance is exactly the share of the
// 6^5 rolls no ability qualifies for; more rolls only lower it.
func TestFreshTurnMissChance(t *testing.T) {
	d := firstRollDecision(t)
	b := d.Battle
	lib, err := engine.SettledLibrary(b)
	if err != nil {
		t.Fatal(err)
	}
	actor, rt := b.Actors[d.Seat], b.Settled.Actors[d.Seat]
	die := b.DiceDefinitions[actor.DiceLoadout[0].DiceID]
	turn := lookahead.Turn{Die: die, Dice: actor.DiceLoadout[0].Count, Rolls: rt.MaxRolls, Energy: actor.Resources.EnergyPoints,
		Abilities: rt.OffensiveAbilityIDs, Modifiers: rt.AbilityModifiers,
		Cards: lookahead.HandCards(lib, actor.Cards.Hand, rt.CardInstances, rt.CardUses, b.Segment.Round)}
	solver := lookahead.NewDiceSolver()
	fresh, err := solver.Fresh(b, d.Seat)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := lookahead.NewDiceSolver().FreshTurn(lib, turn, nil); math.Abs(v-fresh) > 1e-12 {
		t.Fatalf("FreshTurn %v, Fresh %v", v, fresh)
	}
	// One roll, no cards: count the misses by brute force.
	turn.Cards, turn.Rolls = nil, 1
	_, miss := lookahead.NewDiceSolver().FreshTurn(lib, turn, nil)
	misses, total := 0, 0
	dice := make([]state.RolledDie, turn.Dice)
	var visit func(i int)
	visit = func(i int) {
		if i == len(dice) {
			total++
			for _, id := range turn.Abilities {
				if _, ok := engine.QualifiedAbilityTier(lib.Abilities[id], dice); ok {
					return
				}
			}
			misses++
			return
		}
		for _, face := range die.Faces {
			dice[i] = state.RolledDie{Index: i, DieID: die.ID, Face: face.Face, Value: face.Value, Symbols: face.Symbols}
			visit(i + 1)
		}
	}
	visit(0)
	if want := float64(misses) / float64(total); math.Abs(miss-want) > 1e-12 {
		t.Fatalf("one-roll miss chance %v, want %d/%d = %v", miss, misses, total, want)
	}
	previous := miss
	for rolls := 2; rolls <= 3; rolls++ {
		turn.Rolls = rolls
		_, m := lookahead.NewDiceSolver().FreshTurn(lib, turn, nil)
		if m < 0 || m > previous+1e-12 {
			t.Fatalf("%d rolls miss %v after %v", rolls, m, previous)
		}
		previous = m
	}
}

// PlanTurn, built from a viewer-safe Turn and the dice showing, agrees with
// the plan of the real battle, and its Parts follow the same choices as its
// value and miss chance: the parts' miss is Miss, and without dice cards the
// parts at the solver's rates give the plan's value.
func TestPlanTurnParts(t *testing.T) {
	runner, foe := brineMaskRunner(t)
	var plans []*lookahead.Decision
	keep := func(d *lookahead.Decision) bool {
		rt := d.Battle.Settled.Actors[d.Seat]
		return d.Battle.Segment.Current == "offensive" && len(rt.FinalDice) > 0 && rt.MaxRolls > rt.RollsUsed
	}
	for seed := uint64(1); seed <= 3; seed++ {
		r := &recorder{inner: lookahead.NewGreedyPolicy(), keep: keep}
		if _, err := runner.Play(seed, r, foe); err != nil {
			t.Fatal(err)
		}
		plans = append(plans, r.got...)
	}
	if len(plans) < 5 {
		t.Fatalf("only %d rolling decisions", len(plans))
	}
	rates := lookahead.DefaultRates()
	for _, d := range plans {
		b := d.Battle
		lib, err := engine.SettledLibrary(b)
		if err != nil {
			t.Fatal(err)
		}
		actor, rt := b.Actors[d.Seat], b.Settled.Actors[d.Seat]
		turn := lookahead.Turn{Die: b.DiceDefinitions[actor.DiceLoadout[0].DiceID], Dice: actor.DiceLoadout[0].Count, Rolls: rt.MaxRolls, Energy: actor.Resources.EnergyPoints,
			Abilities: rt.OffensiveAbilityIDs, Modifiers: rt.AbilityModifiers,
			Cards: lookahead.HandCards(lib, actor.Cards.Hand, rt.CardInstances, rt.CardUses, b.Segment.Round)}
		want, err := lookahead.NewDiceSolver().Plan(b, d.Seat)
		if err != nil || want == nil {
			t.Fatalf("plan: %v", err)
		}
		got := lookahead.NewDiceSolver().PlanTurn(lib, turn, rt.FinalDice, rt.MaxRolls-rt.RollsUsed, nil)
		if got == nil || math.Abs(got.Best()-want.Best()) > 1e-12 || got.Stop != want.Stop {
			t.Fatalf("PlanTurn %v, Plan %v", got, want)
		}
		parts := got.Parts()
		if math.Abs(parts[0]-got.Miss()) > 1e-12 {
			t.Fatalf("parts miss %v, Miss %v", parts[0], got.Miss())
		}
		turn.Cards = nil
		bare := lookahead.NewDiceSolver().PlanTurn(lib, turn, rt.FinalDice, rt.MaxRolls-rt.RollsUsed, nil)
		p := bare.Parts()
		if v := p[1] + rates.Status*p[2] + rates.EnergyGain*p[4]; math.Abs(v-bare.Best()) > 1e-9 {
			t.Fatalf("parts %v give %v, the plan is worth %v", p, v, bare.Best())
		}
		var all []int
		for _, die := range rt.FinalDice {
			all = append(all, die.Index)
		}
		if r := bare.RerollParts(all); math.Abs(r[0]-bare.RerollMiss(all)) > 1e-12 {
			t.Fatalf("reroll parts miss %v, RerollMiss %v", r[0], bare.RerollMiss(all))
		}
	}
}
