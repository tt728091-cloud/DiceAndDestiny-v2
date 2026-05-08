package setup

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/card"
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

type BattleSetupOption func(*battleSetupOptions)

type battleSetupOptions struct {
	shuffleDeck       bool
	deckShuffleSource card.ShuffleSource
}

func WithDeckShuffleSource(source card.ShuffleSource) BattleSetupOption {
	return func(options *battleSetupOptions) {
		options.shuffleDeck = true
		options.deckShuffleSource = source
	}
}

func BattleSetupFromRunPlayer(player RunPlayerState, opts ...BattleSetupOption) (state.BattleSetup, error) {
	switch {
	case player.ActorID == "":
		return state.BattleSetup{}, fmt.Errorf("%w: actor id is required", ErrInvalidRunPlayerState)
	case len(player.Cards.Deck) == 0:
		return state.BattleSetup{}, fmt.Errorf("%w: deck is required", ErrInvalidRunPlayerState)
	}

	options := battleSetupOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&options)
	}

	deck := copyStrings(player.Cards.Deck)
	if options.shuffleDeck {
		if err := card.ShuffleDeck(deck, options.deckShuffleSource); err != nil {
			return state.BattleSetup{}, err
		}
	}

	return state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:      player.ActorID,
				Deck:    deck,
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
