package state

import (
	"errors"

	"diceanddestiny/server/internal/battle/segment"
)

type Battle struct {
	ID      string
	Segment segment.State
}

func NewBattle(id string) (Battle, error) {
	if id == "" {
		return Battle{}, errors.New("battle id is required")
	}

	return Battle{
		ID:      id,
		Segment: segment.NewManager().InitialState(),
	}, nil
}
