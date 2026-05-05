package state

import (
	"errors"

	"diceanddestiny/server/internal/battle/segment"
)

type Battle struct {
	ID      string
	Segment segment.State
	Cards   map[string]CardZones
}

type CardZones struct {
	Deck []string
	Hand []string
}

func NewBattle(id string) (Battle, error) {
	if id == "" {
		return Battle{}, errors.New("battle id is required")
	}

	return Battle{
		ID:      id,
		Segment: segment.NewManager().InitialState(),
		Cards: map[string]CardZones{
			"player": {
				Deck: []string{"card-1", "card-2", "card-3"},
			},
		},
	}, nil
}
