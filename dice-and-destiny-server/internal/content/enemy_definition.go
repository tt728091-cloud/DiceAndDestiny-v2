package content

import (
	"errors"
	"fmt"
	"os"
)

var ErrInvalidEnemyDefinition = errors.New("invalid enemy definition")

type EnemyDefinition struct {
	SchemaVersion   int
	ID              string
	Name            string
	Class           string
	Resources       StartingResources
	Health          CharacterHealth
	Decklist        []DecklistEntry
	DiceLoadout     []DiceLoadoutEntry
	AbilityIDs      []string
	Statuses        []StartingStatus
	Tokens          []StartingToken
	RollPreferences RollPreferences
}

type enemyDefinitionFile struct {
	SchemaVersion   int                  `yaml:"schema_version"`
	ID              string               `yaml:"id"`
	Name            string               `yaml:"name"`
	Class           string               `yaml:"class"`
	Resources       StartingResources    `yaml:"resources"`
	Health          *characterHealthFile `yaml:"health"`
	Decklist        []DecklistEntry      `yaml:"decklist"`
	DiceLoadout     []DiceLoadoutEntry   `yaml:"dice_loadout"`
	AbilityIDs      []string             `yaml:"abilities"`
	Statuses        []StartingStatus     `yaml:"statuses"`
	Tokens          []StartingToken      `yaml:"tokens"`
	RollPreferences RollPreferences      `yaml:"roll_preferences"`
}

func LoadEnemyDefinition(path string, library ContentLibrary) (EnemyDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EnemyDefinition{}, fmt.Errorf("load enemy definition: read %q: %w", path, err)
	}

	var file enemyDefinitionFile
	if err := decodeKnownYAML(data, &file); err != nil {
		return EnemyDefinition{}, fmt.Errorf("load enemy definition: parse YAML %q: %w", path, err)
	}
	if err := validateEnemyDefinition(file, library); err != nil {
		return EnemyDefinition{}, err
	}

	return EnemyDefinition{
		SchemaVersion: file.SchemaVersion,
		ID:            file.ID,
		Name:          file.Name,
		Class:         file.Class,
		Resources:     file.Resources,
		Health: CharacterHealth{
			Model:     file.Health.Model,
			MaxHealth: decklistTotal(file.Decklist),
		},
		Decklist:        copyDecklist(file.Decklist),
		DiceLoadout:     copyDiceLoadout(file.DiceLoadout),
		AbilityIDs:      copyStrings(file.AbilityIDs),
		Statuses:        copyStatuses(file.Statuses),
		Tokens:          copyTokens(file.Tokens),
		RollPreferences: normalizedRollPreferences(file.RollPreferences),
	}, nil
}

func validateEnemyDefinition(file enemyDefinitionFile, library ContentLibrary) error {
	switch {
	case file.SchemaVersion != 1:
		return fmt.Errorf("%w: schema_version must be 1", ErrInvalidEnemyDefinition)
	case file.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidEnemyDefinition)
	case file.Name == "":
		return fmt.Errorf("%w: name is required", ErrInvalidEnemyDefinition)
	case file.Class == "":
		return fmt.Errorf("%w: class is required", ErrInvalidEnemyDefinition)
	case file.Resources.StartingHandSize < 0:
		return fmt.Errorf("%w: resources.starting_hand_size must be non-negative", ErrInvalidEnemyDefinition)
	case file.Resources.MaxHandSize < file.Resources.StartingHandSize:
		return fmt.Errorf("%w: resources.max_hand_size must be at least starting_hand_size", ErrInvalidEnemyDefinition)
	case file.Resources.StartingEnergyPoints < 0:
		return fmt.Errorf("%w: resources.starting_energy_points must be non-negative", ErrInvalidEnemyDefinition)
	case file.Resources.MaxEnergyPoints < file.Resources.StartingEnergyPoints:
		return fmt.Errorf("%w: resources.max_energy_points must be at least starting_energy_points", ErrInvalidEnemyDefinition)
	case file.Health == nil:
		return fmt.Errorf("%w: health is required", ErrInvalidEnemyDefinition)
	case file.Health.Model != "card_zones":
		return fmt.Errorf("%w: health.model must be card_zones", ErrInvalidEnemyDefinition)
	}

	if err := validateDecklist(file.Decklist, file.Health, library.Cards); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEnemyDefinition, err)
	}
	if err := validateDiceLoadout(file.DiceLoadout, library.Dice); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEnemyDefinition, err)
	}
	if err := validateAbilityIDs(file.AbilityIDs, library.Abilities); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEnemyDefinition, err)
	}
	if err := validateStartingState(file.Statuses, file.Tokens, file.RollPreferences); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEnemyDefinition, err)
	}
	return nil
}
