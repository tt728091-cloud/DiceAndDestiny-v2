package card_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/card"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestShuffleDeckReordersCardsDeterministicallyFromSource(t *testing.T) {
	deck := []string{"strike", "guard", "focus", "bash"}
	source := &fakeShuffleSource{indexes: []int{1, 0, 1}}

	if err := card.ShuffleDeck(deck, source); err != nil {
		t.Fatalf("ShuffleDeck() returned error: %v", err)
	}

	wantDeck := []string{"focus", "bash", "strike", "guard"}
	if !reflect.DeepEqual(deck, wantDeck) {
		t.Fatalf("deck = %#v, want %#v", deck, wantDeck)
	}

	wantCalls := []int{4, 3, 2}
	if !reflect.DeepEqual(source.calls, wantCalls) {
		t.Fatalf("source calls = %#v, want %#v", source.calls, wantCalls)
	}
}

func TestShuffleDeckPreservesCardIDs(t *testing.T) {
	deck := []string{"strike", "guard", "focus", "bash"}
	before := cardIDCounts(deck)

	if err := card.ShuffleDeck(deck, &fakeShuffleSource{indexes: []int{1, 0, 1}}); err != nil {
		t.Fatalf("ShuffleDeck() returned error: %v", err)
	}

	after := cardIDCounts(deck)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("shuffled card ID counts = %#v, want %#v", after, before)
	}
}

func TestDrawCardsConsumesShuffledDeckOrder(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				Cards: state.CardZones{
					Deck: []string{"strike", "guard", "focus", "bash"},
				},
			},
		},
	}

	actor := battle.Actors["player"]
	if err := card.ShuffleDeck(actor.Cards.Deck, &fakeShuffleSource{indexes: []int{1, 0, 1}}); err != nil {
		t.Fatalf("ShuffleDeck() returned error: %v", err)
	}
	battle.Actors["player"] = actor

	if _, err := card.DrawCards(&battle, "player", 2); err != nil {
		t.Fatalf("DrawCards() returned error: %v", err)
	}

	wantHand := []string{"focus", "bash"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Hand, wantHand) {
		t.Fatalf("hand = %#v, want %#v", battle.Actors["player"].Cards.Hand, wantHand)
	}

	wantDeck := []string{"strike", "guard"}
	if !reflect.DeepEqual(battle.Actors["player"].Cards.Deck, wantDeck) {
		t.Fatalf("deck = %#v, want %#v", battle.Actors["player"].Cards.Deck, wantDeck)
	}
}

func TestSeededShuffleSourceProducesStableOrder(t *testing.T) {
	first := []string{"strike", "guard", "focus", "bash", "ward"}
	second := append([]string(nil), first...)

	if err := card.ShuffleDeck(first, card.NewSeededShuffleSource(42)); err != nil {
		t.Fatalf("first ShuffleDeck() returned error: %v", err)
	}
	if err := card.ShuffleDeck(second, card.NewSeededShuffleSource(42)); err != nil {
		t.Fatalf("second ShuffleDeck() returned error: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different decks: %#v and %#v", first, second)
	}
}

func TestShuffleDeckRejectsMissingSource(t *testing.T) {
	err := card.ShuffleDeck([]string{"strike", "guard"}, nil)
	if err == nil {
		t.Fatal("ShuffleDeck() succeeded with nil source")
	}

	if !errors.Is(err, card.ErrInvalidShuffle) {
		t.Fatalf("ShuffleDeck() error = %v, want ErrInvalidShuffle", err)
	}
}

func TestShuffleDeckRejectsOutOfRangeSourceIndex(t *testing.T) {
	deck := []string{"strike", "guard", "focus"}

	err := card.ShuffleDeck(deck, &fakeShuffleSource{indexes: []int{3}})
	if err == nil {
		t.Fatal("ShuffleDeck() succeeded with out-of-range index")
	}

	if !errors.Is(err, card.ErrInvalidShuffle) {
		t.Fatalf("ShuffleDeck() error = %v, want ErrInvalidShuffle", err)
	}
}

type fakeShuffleSource struct {
	indexes []int
	calls   []int
}

func (s *fakeShuffleSource) Intn(n int) int {
	s.calls = append(s.calls, n)

	next := s.indexes[0]
	s.indexes = s.indexes[1:]
	return next
}

func cardIDCounts(ids []string) map[string]int {
	counts := make(map[string]int, len(ids))
	for _, id := range ids {
		counts[id]++
	}
	return counts
}
