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

func TestDrawCardsNeverRecyclesDiscard(t *testing.T) {
	for _, tc := range []struct {
		name      string
		deck      []string
		count     int
		wantDrawn []string
		shortage  bool
	}{
		{"empty deck", nil, 2, nil, true},
		{"short deck", []string{"last"}, 2, []string{"last"}, true},
		{"exact deck", []string{"last"}, 1, []string{"last"}, false},
		{"zero draw", nil, 0, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := battleWithPlayerCards(state.CardZones{Deck: tc.deck, Hand: []string{"held"}, Discard: []string{"spent-1", "spent-2"}, Removed: []string{"lost"}})
			health := b.Actors["player"].CurrentHealth()
			events, err := card.DrawCards(&b, "player", tc.count)
			if err != nil {
				t.Fatal(err)
			}
			wantEvents := []event.Event{event.NewCardsDrawn("player", tc.wantDrawn, tc.shortage)}
			if !reflect.DeepEqual(events, wantEvents) {
				t.Fatalf("events = %#v, want %#v", events, wantEvents)
			}
			want := state.CardZones{Hand: append([]string{"held"}, tc.wantDrawn...), Discard: []string{"spent-1", "spent-2"}, Removed: []string{"lost"}}
			if !reflect.DeepEqual(b.Actors["player"].Cards, want) || b.Actors["player"].CurrentHealth() != health {
				t.Fatalf("draw changed discard, removed cards, or health: %+v", b.Actors["player"])
			}
			// Later draws must also leave discard unavailable.
			for i := 0; i < 3; i++ {
				if _, err := card.DrawCards(&b, "player", 1); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(b.Actors["player"].Cards, want) {
					t.Fatal("later draw recycled discard")
				}
			}
		})
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
