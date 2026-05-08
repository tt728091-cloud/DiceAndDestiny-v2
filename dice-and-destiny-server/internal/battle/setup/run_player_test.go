package setup_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/card"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
)

func TestBattleSetupFromRunPlayerPreservesActorAndCardZones(t *testing.T) {
	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck:    []string{"strike", "guard"},
			Hand:    []string{"opener"},
			Discard: []string{"spent"},
			Removed: []string{"lost"},
		},
	})
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	want := state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      "player",
				Deck:    []string{"strike", "guard"},
				Hand:    []string{"opener"},
				Discard: []string{"spent"},
				Removed: []string{"lost"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BattleSetupFromRunPlayer() = %#v, want %#v", got, want)
	}
}

func TestBattleSetupFromRunPlayerCopiesInputZoneSlices(t *testing.T) {
	deck := []string{"strike", "guard"}
	hand := []string{"opener"}
	discard := []string{"spent"}
	removed := []string{"lost"}

	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck:    deck,
			Hand:    hand,
			Discard: discard,
			Removed: removed,
		},
	})
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	deck[0] = "mutated"
	hand[0] = "mutated"
	discard[0] = "mutated"
	removed[0] = "mutated"

	gotActor := got.Actors[0]
	wantActor := state.ActorSetup{
		ID:      "player",
		Deck:    []string{"strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(gotActor, wantActor) {
		t.Fatalf("actor setup = %#v, want %#v", gotActor, wantActor)
	}
}

func TestBattleSetupFromRunPlayerShufflesDeckWhenRequested(t *testing.T) {
	source := &fakeShuffleSource{indexes: []int{1, 0, 1}}

	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck:    []string{"strike", "guard", "focus", "bash"},
			Hand:    []string{"opener"},
			Discard: []string{"spent"},
			Removed: []string{"lost"},
		},
	}, setup.WithDeckShuffleSource(source))
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	want := state.ActorSetup{
		ID:      "player",
		Deck:    []string{"focus", "bash", "strike", "guard"},
		Hand:    []string{"opener"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(got.Actors[0], want) {
		t.Fatalf("actor setup = %#v, want %#v", got.Actors[0], want)
	}
}

func TestBattleSetupFromRunPlayerShuffledDeckFeedsIncomeDrawOrder(t *testing.T) {
	battleSetup, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck: []string{"strike", "guard", "focus", "bash"},
		},
	}, setup.WithDeckShuffleSource(&fakeShuffleSource{indexes: []int{1, 0, 1}}))
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	battle, err := state.NewBattleFromSetup("battle-1", battleSetup)
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	eng := engine.NewEngine()
	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"focus"}, false),
	}
	if !reflect.DeepEqual(got.Enter.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Enter.Events, wantEvents)
	}

	wantZones := state.CardZones{
		Deck: []string{"bash", "strike", "guard"},
		Hand: []string{"focus"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestBattleSetupFromRunPlayerRejectsRequestedShuffleWithoutSource(t *testing.T) {
	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck: []string{"strike", "guard"},
		},
	}, setup.WithDeckShuffleSource(nil))
	if err == nil {
		t.Fatalf("BattleSetupFromRunPlayer() succeeded with setup %#v", got)
	}

	if !errors.Is(err, card.ErrInvalidShuffle) {
		t.Fatalf("BattleSetupFromRunPlayer() error = %v, want ErrInvalidShuffle", err)
	}
}

func TestBattleSetupFromRunPlayerRejectsMissingActorID(t *testing.T) {
	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		Cards: setup.RunCardZones{
			Deck: []string{"strike"},
		},
	})
	if err == nil {
		t.Fatalf("BattleSetupFromRunPlayer() succeeded with setup %#v", got)
	}

	if !errors.Is(err, setup.ErrInvalidRunPlayerState) {
		t.Fatalf("BattleSetupFromRunPlayer() error = %v, want ErrInvalidRunPlayerState", err)
	}
}

func TestBattleSetupFromRunPlayerRejectsEmptyDeck(t *testing.T) {
	got, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
	})
	if err == nil {
		t.Fatalf("BattleSetupFromRunPlayer() succeeded with setup %#v", got)
	}

	if !errors.Is(err, setup.ErrInvalidRunPlayerState) {
		t.Fatalf("BattleSetupFromRunPlayer() error = %v, want ErrInvalidRunPlayerState", err)
	}

	if !strings.Contains(err.Error(), "deck is required") {
		t.Fatalf("BattleSetupFromRunPlayer() error = %q, want explicit deck requirement", err.Error())
	}
}

func TestBattleSetupFromRunPlayerCanCreateBattle(t *testing.T) {
	battleSetup, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck:    []string{"strike", "guard"},
			Hand:    []string{"opener"},
			Discard: []string{"spent"},
			Removed: []string{"lost"},
		},
	})
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	battle, err := state.NewBattleFromSetup("battle-1", battleSetup)
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

func TestBattleSetupFromRunPlayerSupportsIncomeDraw(t *testing.T) {
	battleSetup, err := setup.BattleSetupFromRunPlayer(setup.RunPlayerState{
		ActorID: "player",
		Cards: setup.RunCardZones{
			Deck:    []string{"strike", "guard"},
			Hand:    []string{"opener"},
			Discard: []string{"spent"},
			Removed: []string{"lost"},
		},
	})
	if err != nil {
		t.Fatalf("BattleSetupFromRunPlayer() returned error: %v", err)
	}

	battle, err := state.NewBattleFromSetup("battle-1", battleSetup)
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}

	eng := engine.NewEngine()
	got, err := eng.AdvanceSegment(&battle)
	if err != nil {
		t.Fatalf("AdvanceSegment() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"strike"}, false),
	}
	if !reflect.DeepEqual(got.Enter.Events, wantEvents) {
		t.Fatalf("enter events = %#v, want %#v", got.Enter.Events, wantEvents)
	}

	wantSegment := segment.State{Current: segment.Income, Round: 1}
	if battle.Segment != wantSegment {
		t.Fatalf("battle segment = %#v, want %#v", battle.Segment, wantSegment)
	}

	wantZones := state.CardZones{
		Deck:    []string{"guard"},
		Hand:    []string{"opener", "strike"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("player cards = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

type fakeShuffleSource struct {
	indexes []int
}

func (s *fakeShuffleSource) Intn(n int) int {
	next := s.indexes[0]
	s.indexes = s.indexes[1:]
	return next
}
