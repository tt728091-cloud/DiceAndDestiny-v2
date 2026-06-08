package setup

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

var ErrInvalidCharacterCombatSetup = errors.New("invalid character combat setup")

func BattleSetupFromCharacterCombatSheet(
	sheet content.CharacterCombatSheet,
	library content.ContentLibrary,
) (state.BattleSetup, error) {
	if sheet.ActorID == "" {
		return state.BattleSetup{}, fmt.Errorf("%w: actor id is required", ErrInvalidCharacterCombatSetup)
	}

	deck, err := expandDecklist(sheet.Decklist, library.Cards)
	if err != nil {
		return state.BattleSetup{}, err
	}

	diceLoadout, diceDefinitions, err := buildDiceSetup(sheet.DiceLoadout, library.Dice)
	if err != nil {
		return state.BattleSetup{}, err
	}

	return state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:          sheet.ActorID,
				Deck:        deck,
				DiceLoadout: diceLoadout,
			},
		},
		DiceDefinitions: diceDefinitions,
	}, nil
}

func expandDecklist(
	decklist []content.DecklistEntry,
	cards map[string]content.CardContent,
) ([]string, error) {
	var deck []string
	for _, entry := range decklist {
		if entry.Count <= 0 {
			return nil, fmt.Errorf("%w: card count for %q must be positive", ErrInvalidCharacterCombatSetup, entry.CardID)
		}
		if _, ok := cards[entry.CardID]; !ok {
			return nil, fmt.Errorf("%w: card %q was not found", ErrInvalidCharacterCombatSetup, entry.CardID)
		}
		for i := 0; i < entry.Count; i++ {
			deck = append(deck, entry.CardID)
		}
	}
	if len(deck) == 0 {
		return nil, fmt.Errorf("%w: decklist is required", ErrInvalidCharacterCombatSetup)
	}
	return deck, nil
}

func buildDiceSetup(
	loadout []content.DiceLoadoutEntry,
	diceContent map[string]content.DiceContent,
) ([]state.DiceLoadoutEntry, []state.DiceDefinition, error) {
	diceLoadout := make([]state.DiceLoadoutEntry, 0, len(loadout))
	diceDefinitions := make([]state.DiceDefinition, 0, len(loadout))

	for _, entry := range loadout {
		if entry.Count <= 0 {
			return nil, nil, fmt.Errorf("%w: dice count for %q must be positive", ErrInvalidCharacterCombatSetup, entry.DiceID)
		}

		definition, ok := diceContent[entry.DiceID]
		if !ok {
			return nil, nil, fmt.Errorf("%w: dice %q was not found", ErrInvalidCharacterCombatSetup, entry.DiceID)
		}

		diceLoadout = append(diceLoadout, state.DiceLoadoutEntry{
			DiceID: entry.DiceID,
			Count:  entry.Count,
		})
		diceDefinitions = append(diceDefinitions, state.DiceDefinition{
			ID:        definition.ID,
			Name:      definition.Name,
			DieType:   definition.DieType,
			SideCount: definition.SideCount,
			Faces:     convertDiceFaces(definition.Faces),
		})
	}

	if len(diceLoadout) == 0 {
		return nil, nil, fmt.Errorf("%w: dice loadout is required", ErrInvalidCharacterCombatSetup)
	}
	return diceLoadout, diceDefinitions, nil
}

func convertDiceFaces(faces []content.DiceFace) []state.DiceFace {
	converted := make([]state.DiceFace, len(faces))
	for i, face := range faces {
		converted[i] = state.DiceFace{
			Face:    face.Face,
			Value:   face.Value,
			Symbols: copyStringsPreservingEmpty(face.Symbols),
		}
	}
	return converted
}

func copyStringsPreservingEmpty(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
