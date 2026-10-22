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
