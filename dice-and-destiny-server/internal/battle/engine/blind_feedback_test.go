package engine

import (
	"diceanddestiny/server/internal/battle/command"
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"testing"
)

func TestBlindPublicFeedback(t *testing.T) {
	library := settledTestLibrary(t)
	for face := 1; face <= 6; face++ {
		battle := settledStatusBattle(t, library, "blind", 1)
		runtime := battle.Settled.Actors["player"]
		runtime.SelectedAbilityID = "sword_cut"
		battle.Settled.Actors["player"] = runtime
		battle.Settled.OffensiveSources = []state.SettledDamageSource{{ID: "source", SourceActorID: "player"}}
		script := &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "effect_dice", Bound: 6, Value: face - 1}}}
		eng, err := NewEngineWithConfig(Config{NamedRandom: script}, DefaultFlows()...)
		if err != nil {
			t.Fatal(err)
		}
		events, opened, err := eng.resolveBlindCheckpoint(&battle, library)
		if err != nil || !opened || len(events) != 1 || events[0].Data["ability_id"] != "sword_cut" {
			t.Fatalf("missing roll cause: %v %v", events, err)
		}
		public := snapshot.FromBattleForViewer(battle, "enemy").BlindCheck
		if public["face"] != face || public["ability_id"] != "sword_cut" || public["actor_id"] != "player" {
			t.Fatal("reaction snapshot lost revealed Blind check")
		}
		events, err = eng.handleBlindReactionCommand(&battle, library, command.Command{Type: command.TypePass})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 || events[0].Type != "blind_resolved" {
			t.Fatal("result must precede defense events")
		}
		data := events[0].Data
		if data["face"] != face || data["cancelled"] != (face <= 2) || data["ability_id"] != "sword_cut" {
			t.Fatalf("wrong outcome metadata: %v", data)
		}
		if stacks(&battle, "player", "blind") != 0 {
			t.Fatal("Blind was not consumed")
		}
	}
}

func TestBlindFeedbackUsesReactionAdjustedFace(t *testing.T) {
	library := settledTestLibrary(t)
	battle := settledStatusBattle(t, library, "blind", 1)
	runtime := battle.Settled.Actors["player"]
	runtime.SelectedAbilityID = "sword_cut"
	battle.Settled.Actors["player"] = runtime
	eng, _ := NewEngineWithConfig(Config{NamedRandom: &ownedSelectionScript{Values: []battlerandom.ScriptedValue{{Stream: "effect_dice", Bound: 6, Value: 0}}}}, DefaultFlows()...)
	_, _, err := eng.resolveBlindCheckpoint(&battle, library)
	if err != nil {
		t.Fatal(err)
	}
	battle.Settled.PendingBlind.Face = 5
	events, err := eng.handleBlindReactionCommand(&battle, library, command.Command{Type: command.TypePass})
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Data["face"] != 5 || events[0].Data["cancelled"] != false {
		t.Fatal("feedback used original roll instead of final adjusted face")
	}
}
