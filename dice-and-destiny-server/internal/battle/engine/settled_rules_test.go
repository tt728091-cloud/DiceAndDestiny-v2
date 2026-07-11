package engine

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestSettledQualificationKeepsEveryValidHumanChoiceAndIndependentBonuses(t *testing.T) {
	library := settledTestLibrary(t)
	dice := rolledFaces(library, "standard_d6", []int{1, 1, 2, 3, 6})
	ids := qualifiedAbilities(library, []string{"sword_cut", "golden_edge", "shield_bash", "perfect_form"}, dice, nil)
	if len(ids) != 2 || ids[0] != "golden_edge" || ids[1] != "sword_cut" {
		t.Fatalf("qualified abilities=%v", ids)
	}
	ability := library.Abilities["sword_cut"]
	tier, ok := qualifiedTier(ability, dice)
	if !ok || tier.ID != "four_swords" {
		t.Fatalf("tier=%#v ok=%v", tier, ok)
	}
	if !requirementsMet(library.Cards["sharpen_blade"].Operations[0].Modifier.AddConditionalBonus.Requirements, dice) {
		t.Fatal("exact-pair modifier did not qualify")
	}
}

func TestEntangleIsContentDrivenConsumedAtOffensiveEntryAndReducesRolls(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "entangle", 1)
	eng := NewEngine()
	if err := eng.applyOffensiveEntryTriggers(&battle, library); err != nil {
		t.Fatal(err)
	}
	if got := battle.Settled.Actors["player"].MaxRolls; got != 2 {
		t.Fatalf("max rolls=%d want 2", got)
	}
	if len(battle.Actors["player"].Statuses) != 0 {
		t.Fatal("Entangle was not consumed")
	}
	if battle.Settled.Window != nil {
		t.Fatal("Entangle opened a reaction window")
	}
}

func TestBlindConsumesWithoutAbilityAndCompletesReactableRollWithAbility(t *testing.T) {
	library := settledTestLibrary(t)
	t.Run("no ability", func(t *testing.T) {
		battle := settledStatusBattle(t, library, "blind", 1)
		eng := NewEngine()
		events, opened, err := eng.resolveBlindCheckpoint(&battle, library)
		if err != nil || opened || len(events) != 0 {
			t.Fatalf("result events=%v opened=%v err=%v", events, opened, err)
		}
		if len(battle.Actors["player"].Statuses) != 0 {
			t.Fatal("Blind remained after its checkpoint")
		}
	})
	t.Run("selected ability", func(t *testing.T) {
		battle := settledStatusBattle(t, library, "blind", 1)
		runtime := battle.Settled.Actors["player"]
		runtime.SelectedAbilityID = "sword_cut"
		battle.Settled.Actors["player"] = runtime
		battle.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "source", SourceActorID: "player"}}
		script := &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "status_effect_dice", Bound: 6, Value: 0}}}
		eng, err := NewEngineWithConfig(Config{NamedRandom: script}, DefaultFlows()...)
		if err != nil {
			t.Fatal(err)
		}
		_, opened, err := eng.resolveBlindCheckpoint(&battle, library)
		if err != nil || !opened {
			t.Fatalf("opened=%v err=%v", opened, err)
		}
		if battle.Settled.Actors["player"].SelectedAbilityID == "" {
			t.Fatal("Blind committed before its reaction window closed")
		}
		if _, err := eng.handleBlindReactionCommand(&battle, library, command.Command{Type: command.TypePass}); err != nil {
			t.Fatal(err)
		}
		if battle.Settled.Actors["player"].SelectedAbilityID != "" || len(battle.Settled.OffensiveSources) != 0 {
			t.Fatal("Blind face 1 did not cancel the selected source")
		}
		if len(battle.Actors["player"].Statuses) != 0 {
			t.Fatal("Blind was not removed after its roll")
		}
		if err := script.AssertExhausted(); err != nil {
			t.Fatal(err)
		}
	})
}

func settledTestLibrary(t *testing.T) content.BattleLibrary {
	t.Helper()
	library, err := content.LoadBattleLibrary(filepath.Join("..", "..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	return library
}
func settledStatusBattle(t *testing.T, library content.BattleLibrary, statusID string, stacks int) state.Battle {
	t.Helper()
	catalog, err := json.Marshal(library)
	if err != nil {
		t.Fatal(err)
	}
	battle, err := state.NewBattleFromSetup("status-test", state.BattleSetup{Actors: []state.ActorSetup{{ID: "player", ControllerType: state.ControllerHuman, Statuses: []state.StatusState{{InstanceID: "status-1", DefinitionID: statusID, Stacks: stacks}}}, {ID: "enemy", ControllerType: state.ControllerAI}}, SettledCatalog: catalog, SettledActors: map[string]state.SettledActorRuntime{"player": {MaxRolls: 3}, "enemy": {MaxRolls: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	battle.Segment.Current = segment.Offensive
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	return battle
}
