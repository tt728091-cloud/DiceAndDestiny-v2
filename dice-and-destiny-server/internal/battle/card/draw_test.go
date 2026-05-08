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
	battle := battleWithPlayerCards(state.CardZones{
		Deck:    []string{"strike", "guard", "focus"},
		Hand:    []string{"starter"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	})

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
		Deck:    []string{"focus"},
		Hand:    []string{"starter", "strike", "guard"},
		Discard: []string{"spent"},
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsUsesDeterministicDeckOrder(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck: []string{"first", "second", "third"},
	})

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

func TestDrawCardsDeckHasEnoughCardsDoesNotTouchDiscard(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck:    []string{"deck-card-1", "deck-card-2", "deck-card-3"},
		Hand:    []string{"starter"},
		Discard: []string{"discard-card-1", "discard-card-2"},
		Removed: []string{"removed-card"},
	})

	got, err := card.DrawCards(
		&battle,
		"player",
		2,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{0}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"deck-card-1", "deck-card-2"}, false),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    []string{"deck-card-3"},
		Hand:    []string{"starter", "deck-card-1", "deck-card-2"},
		Discard: []string{"discard-card-1", "discard-card-2"},
		Removed: []string{"removed-card"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsShortDeckDrawsDeckBeforeDiscardReshuffle(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck:    []string{"deck-card-1"},
		Hand:    []string{"starter"},
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3"},
		Removed: []string{"removed-card"},
	})

	got, err := card.DrawCards(
		&battle,
		"player",
		2,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{0, 0}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewDiscardReshuffled("player", 3),
		event.NewCardsDrawn("player", []string{"deck-card-1", "discard-card-2"}, false),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    []string{"discard-card-3", "discard-card-1"},
		Hand:    []string{"starter", "deck-card-1", "discard-card-2"},
		Discard: nil,
		Removed: []string{"removed-card"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsDoesNotMergeDiscardIntoNonEmptyDeck(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck:    []string{"deck-card-1", "deck-card-2"},
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3"},
	})

	got, err := card.DrawCards(
		&battle,
		"player",
		1,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{0, 0}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"deck-card-1"}, false),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    []string{"deck-card-2"},
		Hand:    []string{"deck-card-1"},
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsEmptyDeckReshufflesDiscardAndDrawsRequestedCards(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck:    nil,
		Hand:    []string{"starter"},
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3", "discard-card-4"},
	})

	got, err := card.DrawCards(
		&battle,
		"player",
		2,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{1, 0, 1}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewDiscardReshuffled("player", 4),
		event.NewCardsDrawn("player", []string{"discard-card-3", "discard-card-4"}, false),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    []string{"discard-card-1", "discard-card-2"},
		Hand:    []string{"starter", "discard-card-3", "discard-card-4"},
		Discard: nil,
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsEmptyDeckAndShortDiscardDrawsAllPossibleCards(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3", "discard-card-4"},
	})

	got, err := card.DrawCards(
		&battle,
		"player",
		5,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{1, 0, 1}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewDiscardReshuffled("player", 4),
		event.NewCardsDrawn("player", []string{"discard-card-3", "discard-card-4", "discard-card-1", "discard-card-2"}, true),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    nil,
		Hand:    []string{"discard-card-3", "discard-card-4", "discard-card-1", "discard-card-2"},
		Discard: nil,
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsEmptyDeckAndEmptyDiscardReturnsExplicitShortResult(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Hand:    []string{"starter"},
		Removed: []string{"lost"},
	})

	got, err := card.DrawCards(&battle, "player", 1)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", nil, true),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    nil,
		Hand:    []string{"starter"},
		Discard: nil,
		Removed: []string{"lost"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsDoesNotDrawOrReshuffleRemovedCards(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Removed: []string{"removed-card-1", "removed-card-2"},
	})

	got, err := card.DrawCards(&battle, "player", 2)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", nil, true),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("DrawCards() events = %#v, want %#v", got, wantEvents)
	}

	wantZones := state.CardZones{
		Deck:    nil,
		Hand:    nil,
		Discard: nil,
		Removed: []string{"removed-card-1", "removed-card-2"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantZones) {
		t.Fatalf("card zones = %#v, want %#v", battle.Actors["player"].Cards, wantZones)
	}
}

func TestDrawCardsShufflesDiscardBeforeDrawingFromIt(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3"},
	})

	_, err := card.DrawCards(
		&battle,
		"player",
		1,
		card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{0, 0}}),
	)
	if err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantHand := []string{"discard-card-2"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Hand, wantHand) {
		t.Fatalf("hand = %#v, want %#v", battle.Actors["player"].Cards.Hand, wantHand)
	}

	if reflect.DeepEqual(battle.Actors["player"].Cards.Hand, []string{"discard-card-1"}) {
		t.Fatalf("discard was drawn in original discard order")
	}
}

func TestDrawCardsDiscardReshuffleOrderIsDeterministicWithSameSeed(t *testing.T) {
	first := battleWithPlayerCards(state.CardZones{
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3", "discard-card-4"},
	})
	second := battleWithPlayerCards(state.CardZones{
		Discard: []string{"discard-card-1", "discard-card-2", "discard-card-3", "discard-card-4"},
	})

	if _, err := card.DrawCards(&first, "player", 2, card.WithDiscardShuffleSource(card.NewSeededShuffleSource(42))); err != nil {
		t.Fatalf("first DrawCards() returned error: %v", err)
	}
	if _, err := card.DrawCards(&second, "player", 2, card.WithDiscardShuffleSource(card.NewSeededShuffleSource(42))); err != nil {
		t.Fatalf("second DrawCards() returned error: %v", err)
	}

	if !reflect.DeepEqual(first.Actors["player"].Cards, second.Actors["player"].Cards) {
		t.Fatalf("same seed produced different card zones: %#v and %#v", first.Actors["player"].Cards, second.Actors["player"].Cards)
	}
}

func TestDrawCardsLeavesRemovedUnchangedWhenReshufflingDiscard(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Discard: []string{"discard-card-1", "discard-card-2"},
		Removed: []string{"removed-card"},
	})

	if _, err := card.DrawCards(&battle, "player", 1, card.WithDiscardShuffleSource(&fakeShuffleSource{indexes: []int{0}})); err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantRemoved := []string{"removed-card"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Removed, wantRemoved) {
		t.Fatalf("removed = %#v, want %#v", battle.Actors["player"].Cards.Removed, wantRemoved)
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

func battleWithPlayerCards(cards state.CardZones) state.Battle {
	return state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				Cards: cards,
			},
		},
	}
}
