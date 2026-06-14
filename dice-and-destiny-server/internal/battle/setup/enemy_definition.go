package setup

import (
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

var ErrInvalidEnemySetup = errors.New("invalid enemy setup")

func BattleSetupFromEnemyDefinition(
	instanceID string,
	definition content.EnemyDefinition,
	library content.ContentLibrary,
) (state.BattleSetup, error) {
	if instanceID == "" {
		return state.BattleSetup{}, fmt.Errorf("%w: instance id is required", ErrInvalidEnemySetup)
	}

	deck, err := expandDecklist(definition.Decklist, library.Cards)
	if err != nil {
		return state.BattleSetup{}, fmt.Errorf("%w: %v", ErrInvalidEnemySetup, err)
	}
	diceLoadout, diceDefinitions, err := buildDiceSetup(definition.DiceLoadout, library.Dice)
	if err != nil {
		return state.BattleSetup{}, fmt.Errorf("%w: %v", ErrInvalidEnemySetup, err)
	}

	return state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID: instanceID,
				Character: state.CharacterMetadata{
					ID:    definition.ID,
					Name:  definition.Name,
					Class: definition.Class,
				},
				Resources: state.ResourceState{
					StartingHandSize:     definition.Resources.StartingHandSize,
					MaxHandSize:          definition.Resources.MaxHandSize,
					StartingEnergyPoints: definition.Resources.StartingEnergyPoints,
					MaxEnergyPoints:      definition.Resources.MaxEnergyPoints,
					EnergyPoints:         definition.Resources.StartingEnergyPoints,
				},
				Health: state.HealthMetadata{
					Model:     definition.Health.Model,
					MaxHealth: definition.Health.MaxHealth,
				},
				Decklist:        convertDecklist(definition.Decklist),
				Deck:            deck,
				DiceLoadout:     diceLoadout,
				AbilityIDs:      copyStringsPreservingEmpty(definition.AbilityIDs),
				Statuses:        enemyStatuses(instanceID, definition.Statuses),
				Tokens:          convertTokens(definition.Tokens),
				RollPreferences: convertRollPreferences(definition.RollPreferences),
			},
		},
		DiceDefinitions: diceDefinitions,
	}, nil
}

func enemyStatuses(instanceID string, values []content.StartingStatus) []state.StatusState {
	converted := convertStatuses(values)
	for i := range converted {
		converted[i].InstanceID = instanceID + "-" + converted[i].InstanceID
	}
	return converted
}
