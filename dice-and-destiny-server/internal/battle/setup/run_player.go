package setup

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/state"
)

var ErrInvalidRunPlayerState = errors.New("invalid run player state")

type RunPlayerState struct {
	ActorID string
	Cards   RunCardZones
}

type RunCardZones struct {
	Deck    []string
	Hand    []string
	Discard []string
	Removed []string
}

func BattleSetupFromRunPlayer(player RunPlayerState) (state.BattleSetup, error) {
	switch {
	case player.ActorID == "":
		return state.BattleSetup{}, fmt.Errorf("%w: actor id is required", ErrInvalidRunPlayerState)
	case len(player.Cards.Deck) == 0:
		return state.BattleSetup{}, fmt.Errorf("%w: deck is required", ErrInvalidRunPlayerState)
	}

	return state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      player.ActorID,
				Deck:    copyStrings(player.Cards.Deck),
				Hand:    copyStrings(player.Cards.Hand),
				Discard: copyStrings(player.Cards.Discard),
				Removed: copyStrings(player.Cards.Removed),
			},
		},
	}, nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}
