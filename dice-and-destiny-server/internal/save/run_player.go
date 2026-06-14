package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
)

var ErrInvalidRunPlayerSave = errors.New("invalid run player save")

type savedRunPlayerStateFile struct {
	SchemaVersion   int                     `json:"schema_version"`
	ActorID         string                  `json:"actor_id"`
	Character       savedCharacterMetadata  `json:"character"`
	Resources       savedResources          `json:"resources"`
	Health          savedHealth             `json:"health"`
	Decklist        []savedDecklistEntry    `json:"decklist"`
	Cards           *savedCardZonesFile     `json:"cards"`
	DiceLoadout     []savedDiceLoadoutEntry `json:"dice_loadout"`
	AbilityIDs      []string                `json:"abilities"`
	Statuses        []savedStatus           `json:"statuses"`
	Tokens          []savedToken            `json:"tokens"`
	RollPreferences savedRollPreferences    `json:"roll_preferences"`
}

type savedCharacterMetadata struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type savedResources struct {
	StartingHandSize     int `json:"starting_hand_size"`
	MaxHandSize          int `json:"max_hand_size"`
	StartingEnergyPoints int `json:"starting_energy_points"`
	MaxEnergyPoints      int `json:"max_energy_points"`
	EnergyPoints         int `json:"energy_points"`
}

type savedHealth struct {
	Model     string `json:"model"`
	MaxHealth int    `json:"max_health"`
}

type savedDecklistEntry struct {
	CardID string `json:"card_id"`
	Count  int    `json:"count"`
}

type savedDiceLoadoutEntry struct {
	DiceID string `json:"dice_id"`
	Count  int    `json:"count"`
}

type savedStatus struct {
	InstanceID   string `json:"instance_id"`
	DefinitionID string `json:"definition_id"`
	Stacks       int    `json:"stacks"`
}

type savedToken struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

type savedRollPreferences struct {
	StatusEffects string `json:"status_effects"`
	Offensive     string `json:"offensive"`
	Defensive     string `json:"defensive"`
}

// SavedRunPlayerState and SavedCardZones retain the original exported save
// shape for callers that still construct the narrow legacy format.
type SavedRunPlayerState struct {
	ActorID string         `json:"actor_id"`
	Cards   SavedCardZones `json:"cards"`
}

type SavedCardZones struct {
	Deck    []string `json:"deck"`
	Hand    []string `json:"hand,omitempty"`
	Discard []string `json:"discard,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type savedCardZonesFile struct {
	Deck    *[]string `json:"deck"`
	Hand    []string  `json:"hand,omitempty"`
	Discard []string  `json:"discard,omitempty"`
	Removed []string  `json:"removed,omitempty"`
}

func LoadRunPlayerState(path string) (setup.RunPlayerState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return setup.RunPlayerState{}, fmt.Errorf("load run player state: read %q: %w", path, err)
	}

	var saved savedRunPlayerStateFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return setup.RunPlayerState{}, fmt.Errorf("load run player state: parse JSON %q: %w", path, err)
	}
	if err := validateSavedRunPlayerState(saved); err != nil {
		return setup.RunPlayerState{}, err
	}

	var decklist []state.DecklistEntry
	if saved.Decklist != nil {
		decklist = make([]state.DecklistEntry, len(saved.Decklist))
	}
	for i, entry := range saved.Decklist {
		decklist[i] = state.DecklistEntry{CardID: entry.CardID, Count: entry.Count}
	}
	var diceLoadout []state.DiceLoadoutEntry
	if saved.DiceLoadout != nil {
		diceLoadout = make([]state.DiceLoadoutEntry, len(saved.DiceLoadout))
	}
	for i, entry := range saved.DiceLoadout {
		diceLoadout[i] = state.DiceLoadoutEntry{DiceID: entry.DiceID, Count: entry.Count}
	}
	var statuses []state.StatusState
	if saved.Statuses != nil {
		statuses = make([]state.StatusState, len(saved.Statuses))
	}
	for i, status := range saved.Statuses {
		statuses[i] = state.StatusState{
			InstanceID:   status.InstanceID,
			DefinitionID: status.DefinitionID,
			Stacks:       status.Stacks,
		}
	}
	var tokens []state.TokenState
	if saved.Tokens != nil {
		tokens = make([]state.TokenState, len(saved.Tokens))
	}
	for i, token := range saved.Tokens {
		tokens[i] = state.TokenState{ID: token.ID, Value: token.Value}
	}

	return setup.RunPlayerState{
		ActorID: saved.ActorID,
		Character: state.CharacterMetadata{
			ID:    saved.Character.ID,
			Name:  saved.Character.Name,
			Class: saved.Character.Class,
		},
		Resources: state.ResourceState{
			StartingHandSize:     saved.Resources.StartingHandSize,
			MaxHandSize:          saved.Resources.MaxHandSize,
			StartingEnergyPoints: saved.Resources.StartingEnergyPoints,
			MaxEnergyPoints:      saved.Resources.MaxEnergyPoints,
			EnergyPoints:         saved.Resources.EnergyPoints,
		},
		Health: state.HealthMetadata{
			Model:     saved.Health.Model,
			MaxHealth: saved.Health.MaxHealth,
		},
		Decklist: decklist,
		Cards: setup.RunCardZones{
			Deck:    copyStrings(*saved.Cards.Deck),
			Hand:    copyStrings(saved.Cards.Hand),
			Discard: copyStrings(saved.Cards.Discard),
			Removed: copyStrings(saved.Cards.Removed),
		},
		DiceLoadout: diceLoadout,
		AbilityIDs:  copyStrings(saved.AbilityIDs),
		Statuses:    statuses,
		Tokens:      tokens,
		RollPreferences: state.RollPreferences{
			StatusEffects: state.RollMode(defaultString(saved.RollPreferences.StatusEffects, string(state.RollModeAutomatic))),
			Offensive:     state.RollMode(defaultString(saved.RollPreferences.Offensive, string(state.RollModeManual))),
			Defensive:     state.RollMode(defaultString(saved.RollPreferences.Defensive, string(state.RollModeManual))),
		},
	}, nil
}

func validateSavedRunPlayerState(saved savedRunPlayerStateFile) error {
	switch {
	case saved.SchemaVersion != 0 && saved.SchemaVersion != 1:
		return fmt.Errorf("%w: schema_version must be 1", ErrInvalidRunPlayerSave)
	case saved.ActorID == "":
		return fmt.Errorf("%w: actor_id is required", ErrInvalidRunPlayerSave)
	case saved.Cards == nil:
		return fmt.Errorf("%w: cards is required", ErrInvalidRunPlayerSave)
	case saved.Cards.Deck == nil:
		return fmt.Errorf("%w: cards.deck is required", ErrInvalidRunPlayerSave)
	case len(*saved.Cards.Deck)+len(saved.Cards.Hand)+len(saved.Cards.Discard) == 0:
		return fmt.Errorf("%w: at least one health card is required in deck, hand, or discard", ErrInvalidRunPlayerSave)
	case saved.Resources.StartingHandSize < 0 || saved.Resources.MaxHandSize < 0:
		return fmt.Errorf("%w: hand size resources must be non-negative", ErrInvalidRunPlayerSave)
	case saved.Resources.StartingEnergyPoints < 0 || saved.Resources.MaxEnergyPoints < 0 || saved.Resources.EnergyPoints < 0:
		return fmt.Errorf("%w: energy resources must be non-negative", ErrInvalidRunPlayerSave)
	}

	for _, entry := range saved.Decklist {
		if entry.CardID == "" || entry.Count <= 0 {
			return fmt.Errorf("%w: decklist entries require card_id and a positive count", ErrInvalidRunPlayerSave)
		}
	}
	for _, entry := range saved.DiceLoadout {
		if entry.DiceID == "" || entry.Count <= 0 {
			return fmt.Errorf("%w: dice_loadout entries require dice_id and a positive count", ErrInvalidRunPlayerSave)
		}
	}
	for _, status := range saved.Statuses {
		if status.InstanceID == "" || status.DefinitionID == "" || status.Stacks <= 0 {
			return fmt.Errorf("%w: statuses require instance_id, definition_id, and positive stacks", ErrInvalidRunPlayerSave)
		}
	}
	for _, token := range saved.Tokens {
		if token.ID == "" {
			return fmt.Errorf("%w: tokens require id", ErrInvalidRunPlayerSave)
		}
	}
	for name, mode := range map[string]string{
		"status_effects": defaultString(saved.RollPreferences.StatusEffects, string(state.RollModeAutomatic)),
		"offensive":      defaultString(saved.RollPreferences.Offensive, string(state.RollModeManual)),
		"defensive":      defaultString(saved.RollPreferences.Defensive, string(state.RollModeManual)),
	} {
		if mode != string(state.RollModeAutomatic) && mode != string(state.RollModeManual) {
			return fmt.Errorf("%w: roll_preferences.%s must be automatic or manual", ErrInvalidRunPlayerSave, name)
		}
	}
	if saved.SchemaVersion == 1 {
		if err := validateCompleteRunPlayerState(saved); err != nil {
			return err
		}
	}
	return nil
}

func validateCompleteRunPlayerState(saved savedRunPlayerStateFile) error {
	switch {
	case saved.Character.ID == "" || saved.Character.Name == "" || saved.Character.Class == "":
		return fmt.Errorf("%w: schema version 1 requires complete character metadata", ErrInvalidRunPlayerSave)
	case saved.Resources.MaxHandSize < saved.Resources.StartingHandSize:
		return fmt.Errorf("%w: max_hand_size must be at least starting_hand_size", ErrInvalidRunPlayerSave)
	case saved.Resources.MaxEnergyPoints < saved.Resources.StartingEnergyPoints ||
		saved.Resources.MaxEnergyPoints < saved.Resources.EnergyPoints:
		return fmt.Errorf("%w: max_energy_points must cover starting and current energy", ErrInvalidRunPlayerSave)
	case saved.Health.Model != "card_zones":
		return fmt.Errorf("%w: health.model must be card_zones", ErrInvalidRunPlayerSave)
	case len(saved.Decklist) == 0:
		return fmt.Errorf("%w: schema version 1 requires decklist", ErrInvalidRunPlayerSave)
	case len(saved.DiceLoadout) == 0:
		return fmt.Errorf("%w: schema version 1 requires dice_loadout", ErrInvalidRunPlayerSave)
	case len(saved.AbilityIDs) == 0:
		return fmt.Errorf("%w: schema version 1 requires abilities", ErrInvalidRunPlayerSave)
	}

	decklistTotal := 0
	for _, entry := range saved.Decklist {
		decklistTotal += entry.Count
	}
	zoneTotal := len(*saved.Cards.Deck) + len(saved.Cards.Hand) +
		len(saved.Cards.Discard) + len(saved.Cards.Removed)
	if decklistTotal != zoneTotal {
		return fmt.Errorf(
			"%w: decklist total %d must match all card zones total %d",
			ErrInvalidRunPlayerSave,
			decklistTotal,
			zoneTotal,
		)
	}
	if saved.Health.MaxHealth != decklistTotal {
		return fmt.Errorf(
			"%w: health.max_health %d must match decklist total %d",
			ErrInvalidRunPlayerSave,
			saved.Health.MaxHealth,
			decklistTotal,
		)
	}
	return nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}
