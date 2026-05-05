package state

import (
	"errors"

	"diceanddestiny/server/internal/battle/segment"
)

type Battle struct {
	ID      string
	Segment segment.State
	Actors  map[string]ActorState
}

type ActorState struct {
	Cards CardZones
}

type CardZones struct {
	Deck    []string
	Hand    []string
	Discard []string
	Removed []string
}

type BattleSetup struct {
	Actors []ActorSetup
}

type ActorSetup struct {
	ID   string
	Deck []string
}

func NewBattle(id string) (Battle, error) {
	return NewBattleFromSetup(id, BattleSetup{
		Actors: []ActorSetup{
			{
				ID:   "player",
				Deck: []string{"card-1", "card-2", "card-3"},
			},
		},
	})
}

func NewBattleFromSetup(id string, setup BattleSetup) (Battle, error) {
	if id == "" {
		return Battle{}, errors.New("battle id is required")
	}

	actors := make(map[string]ActorState, len(setup.Actors))
	for _, actor := range setup.Actors {
		if actor.ID == "" {
			return Battle{}, errors.New("actor id is required")
		}

		actors[actor.ID] = ActorState{
			Cards: CardZones{
				Deck:    append([]string(nil), actor.Deck...),
				Hand:    nil,
				Discard: nil,
				Removed: nil,
			},
		}
	}

	return Battle{
		ID:      id,
		Segment: segment.NewManager().InitialState(),
		Actors:  actors,
	}, nil
}
