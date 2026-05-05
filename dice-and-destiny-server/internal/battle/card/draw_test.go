package card_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/card"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestDrawCardsMovesCardsFromDeckToHand(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				Cards: state.CardZones{
					Deck: []string{"strike", "guard", "focus"},
					Hand: []string{"starter"},
				},
			},
		},
	}

	got, err := card.DrawCards(&battle, "player", 2)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"strike", "guard"}, false),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck: []string{"focus"},
		Hand: []string{"starter", "strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsUsesDeterministicDeckOrder(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				Cards: state.CardZones{
					Deck: []string{"first", "second", "third"},
				},
			},
		},
	}

	if _, err := card.DrawCards(&battle, "player", 1); err != nil {
		t.Fatalf("first DrawCards() returned error: %v", err)
	}
	if _, err := card.DrawCards(&battle, "player", 1); err != nil {
		t.Fatalf("second DrawCards() returned error: %v", err)
	}

	wantHand := []string{"first", "second"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Hand, wantHand) {
		t.Fatalf("hand = %#v, want %#v", battle.Actors["player"].Cards.Hand, wantHand)
	}

	wantDeck := []string{"third"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Deck, wantDeck) {
		t.Fatalf("deck = %#v, want %#v", battle.Actors["player"].Cards.Deck, wantDeck)
	}
}

func TestDrawCardsEmptyDeckReturnsExplicitDeckEmptyEvent(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {},
		},
	}

	got, err := card.DrawCards(&battle, "player", 1)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	want := []event.Event{
		event.NewCardsDrawn("player", nil, true),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, want)
	}

	if len(battle.Actors["player"].Cards.Deck) != 0 || len(battle.Actors["player"].Cards.Hand) != 0 {
		t.Fatalf("empty deck draw changed zones: %#v", battle.Actors["player"].Cards)
	}
}

func TestDrawCardsRejectsMissingActorCardState(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors:  map[string]state.ActorState{},
	}

	_, err := card.DrawCards(&battle, "player", 1)
	if err == nil {
		t.Fatal("DrawCards() succeeded with missing actor card state")
	}

	if !errors.Is(err, card.ErrMissingCardState) {
		t.Fatalf("DrawCards() error = %v, want ErrMissingCardState", err)
	}
}
