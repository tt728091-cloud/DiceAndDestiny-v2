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
				ID: sheet.ActorID,
				Character: state.CharacterMetadata{
					ID:    sheet.Character.ID,
					Name:  sheet.Character.Name,
					Class: sheet.Character.Class,
				},
				Resources: state.ResourceState{
					StartingHandSize:     sheet.Resources.StartingHandSize,
					MaxHandSize:          sheet.Resources.MaxHandSize,
					StartingEnergyPoints: sheet.Resources.StartingEnergyPoints,
					MaxEnergyPoints:      sheet.Resources.MaxEnergyPoints,
					EnergyPoints:         sheet.Resources.StartingEnergyPoints,
				},
				Health: state.HealthMetadata{
					Model:     sheet.Health.Model,
					MaxHealth: sheet.Health.MaxHealth,
				},
				Decklist:        convertDecklist(sheet.Decklist),
				Deck:            deck,
				DiceLoadout:     diceLoadout,
				AbilityIDs:      copyStringsPreservingEmpty(sheet.AbilityIDs),
				Statuses:        convertStatuses(sheet.Statuses),
				Tokens:          convertTokens(sheet.Tokens),
				RollPreferences: convertRollPreferences(sheet.RollPreferences),
			},
		},
		DiceDefinitions: diceDefinitions,
	}, nil
}

func convertDecklist(values []content.DecklistEntry) []state.DecklistEntry {
	converted := make([]state.DecklistEntry, len(values))
	for i, value := range values {
		converted[i] = state.DecklistEntry{CardID: value.CardID, Count: value.Count}
	}
	return converted
}

func convertStatuses(values []content.StartingStatus) []state.StatusState {
	converted := make([]state.StatusState, len(values))
	for i, value := range values {
		converted[i] = state.StatusState{
			InstanceID:   value.InstanceID,
			DefinitionID: value.DefinitionID,
			Stacks:       value.Stacks,
		}
	}
	return converted
}

func convertTokens(values []content.StartingToken) []state.TokenState {
	converted := make([]state.TokenState, len(values))
	for i, value := range values {
		converted[i] = state.TokenState{ID: value.ID, Value: value.Value}
	}
	return converted
}

func convertRollPreferences(value content.RollPreferences) state.RollPreferences {
	return state.RollPreferences{
		StatusEffects: state.RollMode(value.StatusEffects),
		Offensive:     state.RollMode(value.Offensive),
		Defensive:     state.RollMode(value.Defensive),
	}
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
