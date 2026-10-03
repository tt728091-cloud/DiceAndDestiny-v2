package engine

import (
	"bytes"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"testing"
)

func TestDefensePreviewsPreserveReactionAndPrivacy(t *testing.T) {
	lib := settledTestLibrary(t)
	b := settledStatusBattle(t, lib, "", 0)
	p := b.Settled.Actors["player"]
	p.DefensiveAbilityIDs = []string{"basic_defense"}
	b.Settled.Actors["player"] = p
	enemy := b.Settled.Actors["enemy"]
	enemy.SelectedAbilityID = "sword_cut"
	enemy.SelectedTargetIDs = []string{"player"}
	enemy.FinalDice = rolledFaces(lib, "standard_d6", []int{1, 1, 1, 1, 1})
	enemy.SelectedTierID = "five_swords"
	b.Settled.Actors["enemy"] = enemy
	e := NewEngine()
	if len(e.defensePreviews(&b, "player")) != 0 {
		t.Fatal("preview exposed before reveal")
	}
	b.Settled.Stage = stageOffensiveReact
	openSettledWindow(&b, "offense", stageOffensiveReact, "reaction", []command.Type{command.TypeCommitInteraction, command.TypePass})
	before, _ := json.Marshal(b)
	options := e.defensePreviews(&b, "player")
	if len(options) != 1 || options[0].SourceActorID != "enemy" || options[0].AbilityID != "basic_defense" {
		t.Fatalf("revealed defense options: %+v", options)
	}
	if len(e.defensePreviews(&b, "enemy")) != 0 {
		t.Fatal("other actor's turn leaked choices")
	}
	for _, action := range e.LegalActions(&b, "player") {
		if action.Type == command.TypePlanningAbility {
			t.Fatal("preview became a submit-ready reaction command")
		}
	}
	after, _ := json.Marshal(b)
	if !bytes.Equal(before, after) {
		t.Fatal("inspection changed battle state")
	}
	// Distinct already-generated attacks retain exact identities, even with the same attacker.
	b.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "a", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 2}, {ID: "b", SourceActorID: "enemy", SourceContentID: "sword_cut", TargetActorID: "player", BaseAmount: 3}}
	options = e.defensePreviews(&b, "player")
	if len(options) != 3 || options[0].SourceID != "a" || options[1].SourceID != "b" {
		t.Fatalf("source identities: %+v", options)
	}
	b.Settled.OffensiveSources = nil
	enemy.SelectedAbilityID = ""
	b.Settled.Actors["enemy"] = enemy
	if len(e.defensePreviews(&b, "player")) != 0 {
		t.Fatal("no incoming attack must not offer defense")
	}
}
