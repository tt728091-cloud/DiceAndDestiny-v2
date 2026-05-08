package card

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

var (
	ErrInvalidDraw      = errors.New("invalid card draw")
	ErrMissingCardState = errors.New("missing actor card state")
)

type DrawOption func(*drawOptions)

type drawOptions struct {
	discardShuffleSource    ShuffleSource
	hasDiscardShuffleSource bool
}

func WithDiscardShuffleSource(source ShuffleSource) DrawOption {
	return func(options *drawOptions) {
		options.discardShuffleSource = source
		options.hasDiscardShuffleSource = true
	}
}

func DrawCards(battle *state.Battle, actorID string, count int, opts ...DrawOption) ([]event.Event, error) {
	switch {
	case battle == nil:
		return nil, fmt.Errorf("%w: battle is nil", ErrInvalidDraw)
	case actorID == "":
		return nil, fmt.Errorf("%w: actor id is required", ErrInvalidDraw)
	case count < 0:
		return nil, fmt.Errorf("%w: count must be non-negative", ErrInvalidDraw)
	}

	actor, ok := battle.Actors[actorID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingCardState, actorID)
	}

	options := drawOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&options)
	}

	zones := actor.Cards
	remaining := count
	var events []event.Event
	var drawn []string

	for remaining > 0 {
		if len(zones.Deck) > 0 {
			drawCount := remaining
			if drawCount > len(zones.Deck) {
				drawCount = len(zones.Deck)
			}

			drawn = append(drawn, zones.Deck[:drawCount]...)
			zones.Deck = append([]string(nil), zones.Deck[drawCount:]...)
			remaining -= drawCount
			continue
		}

		if len(zones.Discard) == 0 {
			break
		}

		zones.Deck = append([]string(nil), zones.Discard...)
		if err := ShuffleDeck(zones.Deck, discardShuffleSource(options)); err != nil {
			return nil, err
		}
		zones.Discard = nil
		events = append(events, event.NewDiscardReshuffled(actorID, len(zones.Deck)))
	}

	zones.Hand = append(zones.Hand, drawn...)
	actor.Cards = zones
	battle.Actors[actorID] = actor

	events = append(events, event.NewCardsDrawn(actorID, drawn, remaining > 0))
	return events, nil
}

func discardShuffleSource(options drawOptions) ShuffleSource {
	if options.hasDiscardShuffleSource {
		return options.discardShuffleSource
	}

	return NewSeededShuffleSource(1)
}
