package setup

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/card"
	"diceanddestiny/server/internal/battle/state"
)

var ErrInvalidRunPlayerState = errors.New("invalid run player state")

type RunPlayerState struct {
	ActorID         string
	Character       state.CharacterMetadata
	Resources       state.ResourceState
	Health          state.HealthMetadata
	Decklist        []state.DecklistEntry
	Cards           RunCardZones
	DiceLoadout     []state.DiceLoadoutEntry
	AbilityIDs      []string
	Statuses        []state.StatusState
	Tokens          []state.TokenState
	RollPreferences state.RollPreferences
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
	diceDefinitions   []state.DiceDefinition
}

func WithDeckShuffleSource(source card.ShuffleSource) BattleSetupOption {
	return func(options *battleSetupOptions) {
		options.shuffleDeck = true
		options.deckShuffleSource = source
	}
}

func WithDiceDefinitions(definitions []state.DiceDefinition) BattleSetupOption {
	return func(options *battleSetupOptions) {
		options.diceDefinitions = copyDiceDefinitions(definitions)
	}
}

func BattleSetupFromRunPlayer(player RunPlayerState, opts ...BattleSetupOption) (state.BattleSetup, error) {
	if player.ActorID == "" {
		return state.BattleSetup{}, fmt.Errorf("%w: actor id is required", ErrInvalidRunPlayerState)
	}
	if healthCardCount(player.Cards) == 0 {
		return state.BattleSetup{}, fmt.Errorf("%w: at least one health card is required in deck, hand, or discard", ErrInvalidRunPlayerState)
	}

	options := battleSetupOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	deck := copyStrings(player.Cards.Deck)
	if options.shuffleDeck {
		if err := card.ShuffleDeck(deck, options.deckShuffleSource); err != nil {
			return state.BattleSetup{}, err
		}
	}

	decklist := copyDecklist(player.Decklist)
	if len(decklist) == 0 {
		decklist = deriveDecklist(player.Cards)
	}

	return state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:              player.ActorID,
				Character:       player.Character,
				Resources:       player.Resources,
				Health:          player.Health,
				Decklist:        decklist,
				Deck:            deck,
				Hand:            copyStrings(player.Cards.Hand),
				Discard:         copyStrings(player.Cards.Discard),
				Removed:         copyStrings(player.Cards.Removed),
				DiceLoadout:     copyDiceLoadout(player.DiceLoadout),
				AbilityIDs:      copyStrings(player.AbilityIDs),
				Statuses:        copyStatuses(player.Statuses),
				Tokens:          copyTokens(player.Tokens),
				RollPreferences: player.RollPreferences,
			},
		},
		DiceDefinitions: copyDiceDefinitions(options.diceDefinitions),
	}, nil
}

func healthCardCount(cards RunCardZones) int {
	return len(cards.Deck) + len(cards.Hand) + len(cards.Discard)
}

func deriveDecklist(cards RunCardZones) []state.DecklistEntry {
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, zone := range [][]string{cards.Deck, cards.Hand, cards.Discard, cards.Removed} {
		for _, cardID := range zone {
			if counts[cardID] == 0 {
				order = append(order, cardID)
			}
			counts[cardID]++
		}
	}
	result := make([]state.DecklistEntry, 0, len(order))
	for _, cardID := range order {
		result = append(result, state.DecklistEntry{CardID: cardID, Count: counts[cardID]})
	}
	return result
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}

func copyDecklist(values []state.DecklistEntry) []state.DecklistEntry {
	return append([]state.DecklistEntry(nil), values...)
}

func copyDiceLoadout(values []state.DiceLoadoutEntry) []state.DiceLoadoutEntry {
	return append([]state.DiceLoadoutEntry(nil), values...)
}

func copyStatuses(values []state.StatusState) []state.StatusState {
	return append([]state.StatusState(nil), values...)
}

func copyTokens(values []state.TokenState) []state.TokenState {
	return append([]state.TokenState(nil), values...)
}

func copyDiceDefinitions(values []state.DiceDefinition) []state.DiceDefinition {
	if values == nil {
		return nil
	}
	copied := make([]state.DiceDefinition, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Faces = make([]state.DiceFace, len(value.Faces))
		for j, face := range value.Faces {
			copied[i].Faces[j] = face
			if face.Symbols != nil {
				copied[i].Faces[j].Symbols = append([]string{}, face.Symbols...)
			}
		}
	}
	return copied
}
