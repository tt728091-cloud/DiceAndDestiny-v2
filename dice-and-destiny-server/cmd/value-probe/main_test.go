package main

import (
	"math/rand"
	"testing"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// The damage unit removes a card the way damage picks one, discard pile
// first, and moves the counts others see during private planning.
func TestRemoveOneFollowsDamageOrder(t *testing.T) {
	b := state.Battle{
		Actors: map[string]state.ActorState{"foe": {Cards: state.CardZones{Hand: []string{"h1"}, Deck: []string{"d1", "d2"}, Discard: []string{"x1"}}}},
		Settled: &state.SettledRuntime{PlanningPublic: map[string]state.SettledPlanningPublicState{
			"foe": {HandCount: 1, DeckCount: 2, DiscardCount: 1},
		}},
	}
	rng := rand.New(rand.NewSource(1))
	for _, want := range []string{"discard", "deck", "deck", "hand"} {
		before := b.Actors["foe"].Cards
		if !removeOne(&b, "foe", rng) {
			t.Fatalf("nothing removed for %s", want)
		}
		after := b.Actors["foe"].Cards
		got := map[bool]string{len(after.Discard) < len(before.Discard): "discard"}[true]
		if got == "" && len(after.Deck) < len(before.Deck) {
			got = "deck"
		}
		if got == "" && len(after.Hand) < len(before.Hand) {
			got = "hand"
		}
		if got != want {
			t.Fatalf("removed from %q, want %q", got, want)
		}
	}
	public := b.Settled.PlanningPublic["foe"]
	if public.RemovedCount != 4 || public.HandCount+public.DeckCount+public.DiscardCount != 0 || len(b.Actors["foe"].Cards.Removed) != 4 {
		t.Fatalf("public %+v, removed %v", public, b.Actors["foe"].Cards.Removed)
	}
	if removeOne(&b, "foe", rng) {
		t.Fatal("removed a card from empty piles")
	}
}

// Card-preparation markers are found from the cards, by their own rules.
func TestPreparationMarkers(t *testing.T) {
	step := content.CardStep{Effect: "ability_bonus", Params: map[string]any{"damage": 2}}
	lib := content.BattleLibrary{Cards: map[string]content.BattleCardDefinition{
		"swing": {ID: "swing", Program: &content.CardProgram{Steps: []content.CardStep{step}}},
		"lens":  {ID: "lens", Mechanic: &content.CardMechanic{Kind: "venom_lens"}},
	}}
	markers := preparationMarkers(lib)
	if !markers[content.ProgramPreparationStatus("swing", step)] || !markers[content.MechanicStatusDefinitionID(lib.Cards["lens"])] || len(markers) != 2 {
		t.Fatalf("markers %v", markers)
	}
}
