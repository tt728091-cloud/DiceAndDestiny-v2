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

func DrawCards(battle *state.Battle, actorID string, count int) ([]event.Event, error) {
	switch {
	case battle == nil:
		return nil, fmt.Errorf("%w: battle is nil", ErrInvalidDraw)
	case actorID == "":
		return nil, fmt.Errorf("%w: actor id is required", ErrInvalidDraw)
	case count < 0:
		return nil, fmt.Errorf("%w: count must be non-negative", ErrInvalidDraw)
	}

	zones, ok := battle.Cards[actorID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingCardState, actorID)
	}

	drawCount := count
	if drawCount > len(zones.Deck) {
		drawCount = len(zones.Deck)
	}

	drawn := append([]string(nil), zones.Deck[:drawCount]...)
	zones.Deck = append([]string(nil), zones.Deck[drawCount:]...)
	zones.Hand = append(zones.Hand, drawn...)
	battle.Cards[actorID] = zones

	return []event.Event{
		event.NewCardsDrawn(actorID, drawn, drawCount < count),
	}, nil
}
