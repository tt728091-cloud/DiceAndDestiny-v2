package income

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/card"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/resource"
	"diceanddestiny/server/internal/battle/state"
)

var ErrInvalidReward = errors.New("invalid income reward")

type Reward struct {
	ActorID      string
	DrawCards    int
	EnergyPoints int
}

func DefaultRewards() []Reward {
	return []Reward{
		{ActorID: "player", DrawCards: 1},
	}
}

func NewRewards(rewards ...Reward) ([]Reward, error) {
	copied := make([]Reward, len(rewards))
	for i, reward := range rewards {
		if err := validateReward(reward); err != nil {
			return nil, err
		}
		copied[i] = reward
	}

	return copied, nil
}

func ApplyRewards(battle *state.Battle, rewards []Reward) ([]event.Event, error) {
	var events []event.Event
	for _, reward := range rewards {
		if reward.DrawCards > 0 {
			drawEvents, err := card.DrawCards(battle, reward.ActorID, reward.DrawCards)
			if err != nil {
				return nil, err
			}
			events = append(events, drawEvents...)
		}

		if reward.EnergyPoints > 0 {
			resourceEvents, err := resource.AddEnergyPoints(battle, reward.ActorID, reward.EnergyPoints)
			if err != nil {
				return nil, err
			}
			events = append(events, resourceEvents...)
		}
	}

	return events, nil
}

func validateReward(reward Reward) error {
	switch {
	case reward.ActorID == "":
		return fmt.Errorf("%w: actor id is required", ErrInvalidReward)
	case reward.DrawCards < 0:
		return fmt.Errorf("%w: draw cards must be non-negative", ErrInvalidReward)
	case reward.EnergyPoints < 0:
		return fmt.Errorf("%w: energy points must be non-negative", ErrInvalidReward)
	}

	return nil
}
