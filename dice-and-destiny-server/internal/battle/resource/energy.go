package resource

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

var (
	ErrInvalidEnergyPoints = errors.New("invalid energy points")
	ErrMissingActorState   = errors.New("missing actor state")
)

func AddEnergyPoints(battle *state.Battle, actorID string, points int) ([]event.Event, error) {
	switch {
	case battle == nil:
		return nil, fmt.Errorf("%w: battle is nil", ErrInvalidEnergyPoints)
	case actorID == "":
		return nil, fmt.Errorf("%w: actor id is required", ErrInvalidEnergyPoints)
	case points < 0:
		return nil, fmt.Errorf("%w: points must be non-negative", ErrInvalidEnergyPoints)
	}

	actor, ok := battle.Actors[actorID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrMissingActorState, actorID)
	}

	if points == 0 {
		return nil, nil
	}

	if actor.Resources.EnergyPoints == 0 && actor.EnergyPoints != 0 {
		actor.Resources.EnergyPoints = actor.EnergyPoints
	}
	actor.Resources.EnergyPoints += points
	actor.EnergyPoints = actor.Resources.EnergyPoints
	battle.Actors[actorID] = actor

	return []event.Event{event.NewEnergyPointsGained(actorID, points)}, nil
}
