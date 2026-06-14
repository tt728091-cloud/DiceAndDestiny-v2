package state_test

import (
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestNewBattleInitializesEmptyBattleState(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	if battle.ID != "battle-1" {
		t.Fatalf("battle ID = %q, want %q", battle.ID, "battle-1")
	}
	if battle.Status != state.BattleActive {
		t.Fatalf("battle status = %q, want active", battle.Status)
	}

	wantSegment := segment.NewManager().InitialState()
	if battle.Segment != wantSegment {
		t.Fatalf("battle segment = %#v, want %#v", battle.Segment, wantSegment)
	}

	if battle.Segment.Current != segment.OngoingEffects {
		t.Fatalf("battle segment current = %q, want %q", battle.Segment.Current, segment.OngoingEffects)
	}

	if battle.Segment.Round != 1 {
		t.Fatalf("battle segment round = %d, want 1", battle.Segment.Round)
	}

	if len(battle.Actors) != 0 {
		t.Fatalf("actors = %#v, want no hardcoded actors", battle.Actors)
	}
	if len(battle.DiceDefinitions) != 0 {
		t.Fatalf("dice definitions = %#v, want no hardcoded definitions", battle.DiceDefinitions)
	}
}

func TestNewBattleFromSetupPersistsDefinitionAndController(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "goblin-1",
				DefinitionID:   "goblin",
				ControllerType: state.ControllerAI,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	actor := battle.Actors["goblin-1"]
	if actor.DefinitionID != "goblin" || actor.Controller != state.ControllerAI {
		t.Fatalf("actor = %#v, want goblin AI", actor)
	}
}

func TestNewBattleFromSetupRejectsDuplicateActorID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "goblin-1"},
			{ID: "goblin-1"},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}

func TestNewBattleRejectsEmptyBattleID(t *testing.T) {
	battle, err := state.NewBattle("")
	if err == nil {
		t.Fatalf("NewBattle() succeeded with battle %#v", battle)
	}
}

func TestNewBattleFromSetupInitializesActorCardState(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      "player",
				Deck:    []string{"strike", "guard"},
				Hand:    []string{"opener"},
				Discard: []string{"spent"},
				Removed: []string{"lost"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	want := state.CardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, want) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, want)
	}
}

func TestNewBattleFromSetupCopiesCardZoneInputs(t *testing.T) {
	deck := []string{"strike", "guard"}
	hand := []string{"opener"}
	discard := []string{"spent"}
	removed := []string{"lost"}
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      "player",
				Deck:    deck,
				Hand:    hand,
				Discard: discard,
				Removed: removed,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	deck[0] = "mutated"
	hand[0] = "mutated"
	discard[0] = "mutated"
	removed[0] = "mutated"

	want := state.CardZones{
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, want) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, want)
	}
}

func TestNewBattleFromSetupRejectsMissingActorID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{Deck: []string{"strike"}},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}

func TestNewBattleFromSetupRejectsEmptyBattleID(t *testing.T) {
	battle, err := state.NewBattleFromSetup("", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: []string{"strike"}},
		},
	})
	if err == nil {
		t.Fatalf("NewBattleFromSetup() succeeded with battle %#v", battle)
	}
}
