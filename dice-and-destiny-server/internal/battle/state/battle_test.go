package state_test

import (
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestNewBattleInitializesMinimalState(t *testing.T) {
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}

	if battle.ID != "battle-1" {
		t.Fatalf("battle ID = %q, want %q", battle.ID, "battle-1")
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

	wantPlayerCards := state.CardZones{
		Deck: []string{"card-1", "card-2", "card-3"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantPlayerCards) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, wantPlayerCards)
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
			{ID: "player", Deck: []string{"strike", "guard"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	want := state.CardZones{
		Deck: []string{"strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, want) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, want)
	}
}

func TestNewBattleFromSetupCopiesDeckInput(t *testing.T) {
	deck := []string{"strike", "guard"}
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", Deck: deck},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	deck[0] = "mutated"

	wantDeck := []string{"strike", "guard"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Deck, wantDeck) {
		t.Fatalf("player deck = %#v, want %#v", battle.Actors["player"].Cards.Deck, wantDeck)
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
