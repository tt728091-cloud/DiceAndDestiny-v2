package battle

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/participant"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// This is deliberately a whole-authority test. It starts a real settled
// battle, plays a card, resolves an offensive ability, commits damage, crosses
// a round boundary, and resolves a status. The three definitions are YAML-only
// proof content and all share roll_dice/outcomes without source-specific code.
func TestYAMLOnlyCardAbilityAndStatusComposeThroughAuthority(t *testing.T) {
	script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
		{Stream: "card_draw", Bound: 21, Value: 0},
		{Stream: "ai_d100", Bound: 100, Value: 0},
		{Stream: "effect_dice", Bound: 6, Value: 0}, // card applies Volatile Poison
		{Stream: "combat_dice", Bound: 6, Value: 0}, {Stream: "combat_dice", Bound: 6, Value: 0},
		{Stream: "combat_dice", Bound: 6, Value: 2}, {Stream: "combat_dice", Bound: 6, Value: 3}, {Stream: "combat_dice", Bound: 6, Value: 5},
		{Stream: "effect_dice", Bound: 6, Value: 5}, // ability deals its critical 5
		{Stream: "damage_selection", Bound: 12, Value: 0}, {Stream: "damage_selection", Bound: 11, Value: 0},
		{Stream: "damage_selection", Bound: 10, Value: 0}, {Stream: "damage_selection", Bound: 9, Value: 0}, {Stream: "damage_selection", Bound: 8, Value: 0},
		{Stream: "ai_damage_response", Bound: 2, Value: 0},
		{Stream: "status_effect_dice", Bound: 6, Value: 0}, // status deals 2 next round
		{Stream: "damage_selection", Bound: 7, Value: 0}, {Stream: "damage_selection", Bound: 6, Value: 0},
		{Stream: "ai_d100", Bound: 100, Value: 0},
	}}

	root := participantTestServerRoot(t)
	base := NewFileParticipantAssembler(filepath.Join(root, "content"), filepath.Join(root, "save", "run_players"))
	assembler := ParticipantAssemblerFunc(func(participants []participant.Participant) (state.BattleSetup, error) {
		setup, err := base.AssembleParticipants(participants)
		if err != nil {
			return state.BattleSetup{}, err
		}
		var library content.BattleLibrary
		if err := json.Unmarshal(setup.SettledCatalog, &library); err != nil {
			return state.BattleSetup{}, err
		}
		for key, chart := range library.Combatants["venom_goblin"].AI.OffensivePlanning.Charts {
			chart.Abilities = nil
			chart.NoAbilityRanges = []content.D100Range{{Start: 1, End: 100}}
			library.Combatants["venom_goblin"].AI.OffensivePlanning.Charts[key] = chart
		}
		setup.SettledCatalog, err = json.Marshal(library)
		if err != nil {
			return state.BattleSetup{}, err
		}
		for index := range setup.Actors {
			actor := &setup.Actors[index]
			runtime := setup.SettledActors[actor.ID]
			runtime.IncomeCards = 0
			if actor.ID == "blade" {
				actor.Resources.StartingHandSize = 1
				actor.Resources.StartingEnergyPoints = 10
				actor.Resources.EnergyPoints = 10
				proofID := "blade-card-yaml-proof"
				actor.Deck = append([]string{proofID}, actor.Deck...)
				runtime.CardInstances[proofID] = state.CardInstance{InstanceID: proofID, DefinitionID: "alchemists_gamble"}
				runtime.OffensiveAbilityIDs = append(runtime.OffensiveAbilityIDs, "fateful_strike")
				actor.AbilityIDs = append(actor.AbilityIDs, "fateful_strike")
			} else {
				actor.Resources.StartingHandSize = 0
				runtime.DefensiveAbilityIDs = nil
			}
			setup.SettledActors[actor.ID] = runtime
		}
		return setup, nil
	})
	battleEngine, err := engine.NewEngineWithConfig(engine.Config{NamedRandom: script}, engine.DefaultFlows()...)
	if err != nil {
		t.Fatal(err)
	}
	authority := NewAuthority(battleEngine, repository.NewInMemory(), assembler)

	result := fullBattleSend(t, authority, map[string]any{"battle_id": "blade-v-goblin", "actor_id": "blade", "type": command.TypeStartBattle, "payload": map[string]any{"player": map[string]string{"instance_id": "blade", "definition_id": "blade_warden"}, "enemies": []map[string]string{{"instance_id": "goblin", "definition_id": "venom_goblin"}}}})
	assertFullBattleWait(t, result, "offensive", 1, "planning")
	if result.Snapshot.ContentCatalog == nil || result.Snapshot.ContentCatalog.Cards["alchemists_gamble"].Name == "" || result.Snapshot.ContentCatalog.Abilities["fateful_strike"].Name == "" || result.Snapshot.ContentCatalog.Statuses["volatile_poison"].Name == "" {
		t.Fatalf("public pinned content catalog is missing YAML-only definitions: %#v", result.Snapshot.ContentCatalog)
	}

	pending := pendingFull(t, result)
	result = fullBattleSend(t, authority, envelopeFull(result, command.TypePlanningCards, command.PlanningCardsPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpointFull(pending), CardIDs: []string{"blade-card-yaml-proof"}, TargetIDs: []string{"goblin"}}))
	if !actorHasStatus(result.Snapshot.Actors["goblin"], "volatile_poison", 1) {
		t.Fatalf("Alchemist's Gamble did not apply YAML-only status: %#v", result.Snapshot.Actors["goblin"].Statuses)
	}

	result = sendPlanningRollFull(t, authority, result)
	result = sendAbilityFull(t, authority, result, "fateful_strike", []string{"goblin"})
	assertFullBattleWait(t, result, "offensive", 1, "offensive_reaction")
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "defensive", 1, "defense_reaction")
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "damage_resolution", 1, "damage_reaction")
	if len(result.Snapshot.SettledDamage.Sources) != 1 || result.Snapshot.SettledDamage.Sources[0].SourceContentID != "fateful_strike" || result.Snapshot.SettledDamage.Sources[0].FinalAmount != 5 {
		t.Fatalf("Fateful Strike did not resolve its YAML roll outcome: %#v", result.Snapshot.SettledDamage.Sources)
	}

	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 2, "status_roll_reaction")
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "ongoing_effects", 2, "status_damage_reaction")
	if len(result.Snapshot.SettledDamage.Sources) != 1 || result.Snapshot.SettledDamage.Sources[0].SourceContentID != "volatile_poison" || result.Snapshot.SettledDamage.Sources[0].FinalAmount != 2 {
		t.Fatalf("Volatile Poison did not resolve through the shared roll interpreter: %#v", result.Snapshot.SettledDamage.Sources)
	}
	result = sendPassFull(t, authority, result)
	assertFullBattleWait(t, result, "offensive", 2, "planning")
	if !actorHasStatus(result.Snapshot.Actors["goblin"], "volatile_poison", 1) {
		t.Fatalf("damage outcome unexpectedly removed persistent Volatile Poison: %#v", result.Snapshot.Actors["goblin"].Statuses)
	}
	if err := script.AssertExhausted(); err != nil {
		t.Fatal(err)
	}
}

func actorHasStatus(actor snapshot.Actor, statusID string, stacks int) bool {
	for _, status := range actor.Statuses {
		if status.DefinitionID == statusID && status.Stacks == stacks {
			return true
		}
	}
	return false
}
