package content

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"diceanddestiny/server/internal/battle/segment"
	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidContent              = errors.New("invalid content")
	ErrInvalidCharacterCombatSheet = errors.New("invalid character combat sheet")
)

type ContentLibrary struct {
	Cards     map[string]CardContent
	Abilities map[string]AbilityContent
	Dice      map[string]DiceContent
}

type CardContent struct {
	SchemaVersion     int             `yaml:"schema_version"`
	ID                string          `yaml:"id"`
	Name              string          `yaml:"name"`
	Type              string          `yaml:"type"`
	Cost              ResourceCost    `yaml:"cost"`
	PhaseRestrictions []string        `yaml:"phase_restrictions"`
	Effects           []ContentEffect `yaml:"effects"`
}

type AbilityContent struct {
	SchemaVersion     int             `yaml:"schema_version"`
	ID                string          `yaml:"id"`
	Name              string          `yaml:"name"`
	Type              string          `yaml:"type"`
	PhaseRestrictions []string        `yaml:"phase_restrictions"`
	DiceRequirement   DiceRequirement `yaml:"dice_requirement"`
	Cost              ResourceCost    `yaml:"cost"`
	RequiresTarget    bool            `yaml:"requires_target"`
	Effects           []ContentEffect `yaml:"effects"`
}

type DiceContent struct {
	SchemaVersion int        `yaml:"schema_version"`
	ID            string     `yaml:"id"`
	Name          string     `yaml:"name"`
	DieType       string     `yaml:"die_type"`
	SideCount     int        `yaml:"side_count"`
	Faces         []DiceFace `yaml:"faces"`
}

type ResourceCost struct {
	EnergyPoints int `yaml:"energy_points"`
}

type ContentEffect struct {
	Type string `yaml:"type"`
}

type DiceRequirement struct {
	Kind string `yaml:"kind"`
}

type DiceFace struct {
	Face    int      `yaml:"face"`
	Value   int      `yaml:"value"`
	Symbols []string `yaml:"symbols"`
}

type CharacterCombatSheet struct {
	SchemaVersion   int                `yaml:"schema_version"`
	ActorID         string             `yaml:"actor_id"`
	Character       CharacterMetadata  `yaml:"character"`
	Resources       StartingResources  `yaml:"resources"`
	Health          CharacterHealth    `yaml:"health"`
	Decklist        []DecklistEntry    `yaml:"decklist"`
	DiceLoadout     []DiceLoadoutEntry `yaml:"dice_loadout"`
	AbilityIDs      []string           `yaml:"abilities"`
	Statuses        []StartingStatus   `yaml:"statuses"`
	Tokens          []StartingToken    `yaml:"tokens"`
	RollPreferences RollPreferences    `yaml:"roll_preferences"`
}

type CharacterMetadata struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Class string `yaml:"class"`
}

type StartingResources struct {
	StartingHandSize     int `yaml:"starting_hand_size"`
	MaxHandSize          int `yaml:"max_hand_size"`
	StartingEnergyPoints int `yaml:"starting_energy_points"`
	MaxEnergyPoints      int `yaml:"max_energy_points"`
}

type CharacterHealth struct {
	Model     string `yaml:"model"`
	MaxHealth int    `yaml:"max_health"`
}

type DecklistEntry struct {
	CardID string `yaml:"card_id"`
	Count  int    `yaml:"count"`
}

type DiceLoadoutEntry struct {
	DiceID string `yaml:"dice_id"`
	Count  int    `yaml:"count"`
}

type StartingStatus struct {
	InstanceID   string `yaml:"instance_id" json:"instance_id"`
	DefinitionID string `yaml:"definition_id" json:"definition_id"`
	Stacks       int    `yaml:"stacks" json:"stacks"`
}

type StartingToken struct {
	ID    string `yaml:"id" json:"id"`
	Value int    `yaml:"value" json:"value"`
}

type RollPreferences struct {
	StatusEffects string `yaml:"status_effects" json:"status_effects"`
	Offensive     string `yaml:"offensive" json:"offensive"`
	Defensive     string `yaml:"defensive" json:"defensive"`
}

type characterCombatSheetFile struct {
	SchemaVersion   int                  `yaml:"schema_version"`
	ActorID         string               `yaml:"actor_id"`
	Character       CharacterMetadata    `yaml:"character"`
	Resources       StartingResources    `yaml:"resources"`
	Health          *characterHealthFile `yaml:"health"`
	Decklist        []DecklistEntry      `yaml:"decklist"`
	DiceLoadout     []DiceLoadoutEntry   `yaml:"dice_loadout"`
	AbilityIDs      []string             `yaml:"abilities"`
	Statuses        []StartingStatus     `yaml:"statuses"`
	Tokens          []StartingToken      `yaml:"tokens"`
	RollPreferences RollPreferences      `yaml:"roll_preferences"`
}

type characterHealthFile struct {
	Model     string `yaml:"model"`
	MaxHealth *int   `yaml:"max_health"`
}

func LoadContentLibrary(root string) (ContentLibrary, error) {
	cards, err := loadCards(filepath.Join(root, "cards"))
	if err != nil {
		return ContentLibrary{}, err
	}

	abilities, err := loadAbilities(filepath.Join(root, "abilities"))
	if err != nil {
		return ContentLibrary{}, err
	}

	dice, err := loadDice(filepath.Join(root, "dice"))
	if err != nil {
		return ContentLibrary{}, err
	}

	return ContentLibrary{
		Cards:     cards,
		Abilities: abilities,
		Dice:      dice,
	}, nil
}

func LoadCharacterCombatSheet(path, contentRoot string) (CharacterCombatSheet, error) {
	library, err := LoadContentLibrary(contentRoot)
	if err != nil {
		return CharacterCombatSheet{}, err
	}

	return LoadCharacterCombatSheetWithLibrary(path, library)
}

func LoadCharacterCombatSheetWithLibrary(path string, library ContentLibrary) (CharacterCombatSheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CharacterCombatSheet{}, fmt.Errorf("load character combat sheet: read %q: %w", path, err)
	}

	var file characterCombatSheetFile
	if err := decodeKnownYAML(data, &file); err != nil {
		return CharacterCombatSheet{}, fmt.Errorf("load character combat sheet: parse YAML %q: %w", path, err)
	}

	return buildCharacterCombatSheet(file, library)
}

func buildCharacterCombatSheet(file characterCombatSheetFile, library ContentLibrary) (CharacterCombatSheet, error) {
	if err := validateCharacterCombatSheetFile(file, library); err != nil {
		return CharacterCombatSheet{}, err
	}

	maxHealth := decklistTotal(file.Decklist)
	return CharacterCombatSheet{
		SchemaVersion: file.SchemaVersion,
		ActorID:       file.ActorID,
		Character:     file.Character,
		Resources:     file.Resources,
		Health: CharacterHealth{
			Model:     file.Health.Model,
			MaxHealth: maxHealth,
		},
		Decklist:        copyDecklist(file.Decklist),
		DiceLoadout:     copyDiceLoadout(file.DiceLoadout),
		AbilityIDs:      copyStrings(file.AbilityIDs),
		Statuses:        copyStatuses(file.Statuses),
		Tokens:          copyTokens(file.Tokens),
		RollPreferences: normalizedRollPreferences(file.RollPreferences),
	}, nil
}

func loadCards(dir string) (map[string]CardContent, error) {
	paths, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}

	items := make(map[string]CardContent, len(paths))
	for _, path := range paths {
		var card CardContent
		if err := loadYAMLFile(path, &card); err != nil {
			return nil, err
		}
		if err := validateNamedContent("card", card.SchemaVersion, card.ID, card.Name, card.PhaseRestrictions); err != nil {
			return nil, err
		}
		if _, exists := items[card.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate card id %q", ErrInvalidContent, card.ID)
		}
		items[card.ID] = card
	}

	return items, nil
}

func loadAbilities(dir string) (map[string]AbilityContent, error) {
	paths, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}

	items := make(map[string]AbilityContent, len(paths))
	for _, path := range paths {
		var ability AbilityContent
		if err := loadYAMLFile(path, &ability); err != nil {
			return nil, err
		}
		if err := validateNamedContent("ability", ability.SchemaVersion, ability.ID, ability.Name, ability.PhaseRestrictions); err != nil {
			return nil, err
		}
		if _, exists := items[ability.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate ability id %q", ErrInvalidContent, ability.ID)
		}
		items[ability.ID] = ability
	}

	return items, nil
}

func loadDice(dir string) (map[string]DiceContent, error) {
	paths, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}

	items := make(map[string]DiceContent, len(paths))
	for _, path := range paths {
		var die DiceContent
		if err := loadYAMLFile(path, &die); err != nil {
			return nil, err
		}
		if err := validateDie(die); err != nil {
			return nil, err
		}
		if _, exists := items[die.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate dice id %q", ErrInvalidContent, die.ID)
		}
		items[die.ID] = die
	}

	return items, nil
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("load content: read %q: %w", dir, err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func loadYAMLFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("load content: read %q: %w", path, err)
	}
	if err := decodeKnownYAML(data, v); err != nil {
		return fmt.Errorf("load content: parse YAML %q: %w", path, err)
	}
	return nil
}

func decodeKnownYAML(data []byte, v any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(v)
}

func validateNamedContent(kind string, schemaVersion int, id, name string, phaseRestrictions []string) error {
	switch {
	case schemaVersion != 1:
		return fmt.Errorf("%w: %s %q schema_version must be 1", ErrInvalidContent, kind, id)
	case id == "":
		return fmt.Errorf("%w: %s id is required", ErrInvalidContent, kind)
	case name == "":
		return fmt.Errorf("%w: %s %q name is required", ErrInvalidContent, kind, id)
	case id != name:
		return fmt.Errorf("%w: %s id %q must match unique display name %q", ErrInvalidContent, kind, id, name)
	}

	for _, value := range phaseRestrictions {
		if !segment.IsValid(segment.Segment(value)) {
			return fmt.Errorf("%w: %s %q uses unknown segment %q", ErrInvalidContent, kind, id, value)
		}
	}

	return nil
}

func validateDie(die DiceContent) error {
	if err := validateNamedContent("dice", die.SchemaVersion, die.ID, die.Name, nil); err != nil {
		return err
	}
	switch {
	case die.DieType == "":
		return fmt.Errorf("%w: dice %q die_type is required", ErrInvalidContent, die.ID)
	case die.SideCount <= 0:
		return fmt.Errorf("%w: dice %q side_count must be positive", ErrInvalidContent, die.ID)
	case len(die.Faces) != die.SideCount:
		return fmt.Errorf("%w: dice %q face count must match side_count", ErrInvalidContent, die.ID)
	}
	for _, face := range die.Faces {
		if face.Face <= 0 {
			return fmt.Errorf("%w: dice %q face numbers must be positive", ErrInvalidContent, die.ID)
		}
	}
	return nil
}

func validateCharacterCombatSheetFile(file characterCombatSheetFile, library ContentLibrary) error {
	switch {
	case file.SchemaVersion != 1:
		return fmt.Errorf("%w: schema_version must be 1", ErrInvalidCharacterCombatSheet)
	case file.ActorID == "":
		return fmt.Errorf("%w: actor_id is required", ErrInvalidCharacterCombatSheet)
	case file.Character.ID == "":
		return fmt.Errorf("%w: character.id is required", ErrInvalidCharacterCombatSheet)
	case file.Character.Name == "":
		return fmt.Errorf("%w: character.name is required", ErrInvalidCharacterCombatSheet)
	case file.Character.Class == "":
		return fmt.Errorf("%w: character.class is required", ErrInvalidCharacterCombatSheet)
	case file.Resources.StartingHandSize < 0:
		return fmt.Errorf("%w: resources.starting_hand_size must be non-negative", ErrInvalidCharacterCombatSheet)
	case file.Resources.MaxHandSize < file.Resources.StartingHandSize:
		return fmt.Errorf("%w: resources.max_hand_size must be at least starting_hand_size", ErrInvalidCharacterCombatSheet)
	case file.Resources.StartingEnergyPoints < 0:
		return fmt.Errorf("%w: resources.starting_energy_points must be non-negative", ErrInvalidCharacterCombatSheet)
	case file.Resources.MaxEnergyPoints < file.Resources.StartingEnergyPoints:
		return fmt.Errorf("%w: resources.max_energy_points must be at least starting_energy_points", ErrInvalidCharacterCombatSheet)
	case file.Health == nil:
		return fmt.Errorf("%w: health is required", ErrInvalidCharacterCombatSheet)
	case file.Health.Model != "card_zones":
		return fmt.Errorf("%w: health.model must be card_zones", ErrInvalidCharacterCombatSheet)
	}

	if err := validateDecklist(file.Decklist, file.Health, library.Cards); err != nil {
		return err
	}
	if err := validateDiceLoadout(file.DiceLoadout, library.Dice); err != nil {
		return err
	}
	if err := validateAbilityIDs(file.AbilityIDs, library.Abilities); err != nil {
		return err
	}
	if err := validateStartingState(file.Statuses, file.Tokens, file.RollPreferences); err != nil {
		return err
	}

	return nil
}

func validateDecklist(decklist []DecklistEntry, health *characterHealthFile, cards map[string]CardContent) error {
	if len(decklist) == 0 {
		return fmt.Errorf("%w: decklist is required", ErrInvalidCharacterCombatSheet)
	}

	seen := map[string]bool{}
	for _, entry := range decklist {
		switch {
		case entry.CardID == "":
			return fmt.Errorf("%w: decklist.card_id is required", ErrInvalidCharacterCombatSheet)
		case entry.Count <= 0:
			return fmt.Errorf("%w: decklist count for %q must be positive", ErrInvalidCharacterCombatSheet, entry.CardID)
		case seen[entry.CardID]:
			return fmt.Errorf("%w: duplicate decklist card_id %q", ErrInvalidCharacterCombatSheet, entry.CardID)
		}
		if _, ok := cards[entry.CardID]; !ok {
			return fmt.Errorf("%w: referenced card %q was not found", ErrInvalidCharacterCombatSheet, entry.CardID)
		}
		seen[entry.CardID] = true
	}

	maxHealth := decklistTotal(decklist)
	if health.MaxHealth != nil && *health.MaxHealth != maxHealth {
		return fmt.Errorf("%w: health.max_health %d must match decklist total %d", ErrInvalidCharacterCombatSheet, *health.MaxHealth, maxHealth)
	}

	return nil
}

func validateDiceLoadout(loadout []DiceLoadoutEntry, dice map[string]DiceContent) error {
	if len(loadout) == 0 {
		return fmt.Errorf("%w: dice_loadout is required", ErrInvalidCharacterCombatSheet)
	}

	seen := map[string]bool{}
	for _, entry := range loadout {
		switch {
		case entry.DiceID == "":
			return fmt.Errorf("%w: dice_loadout.dice_id is required", ErrInvalidCharacterCombatSheet)
		case entry.Count <= 0:
			return fmt.Errorf("%w: dice_loadout count for %q must be positive", ErrInvalidCharacterCombatSheet, entry.DiceID)
		case seen[entry.DiceID]:
			return fmt.Errorf("%w: duplicate dice_loadout dice_id %q", ErrInvalidCharacterCombatSheet, entry.DiceID)
		}
		if _, ok := dice[entry.DiceID]; !ok {
			return fmt.Errorf("%w: referenced dice %q was not found", ErrInvalidCharacterCombatSheet, entry.DiceID)
		}
		seen[entry.DiceID] = true
	}

	return nil
}

func validateAbilityIDs(abilityIDs []string, abilities map[string]AbilityContent) error {
	if len(abilityIDs) == 0 {
		return fmt.Errorf("%w: abilities are required", ErrInvalidCharacterCombatSheet)
	}

	seen := map[string]bool{}
	for _, abilityID := range abilityIDs {
		switch {
		case abilityID == "":
			return fmt.Errorf("%w: ability id is required", ErrInvalidCharacterCombatSheet)
		case seen[abilityID]:
			return fmt.Errorf("%w: duplicate ability id %q", ErrInvalidCharacterCombatSheet, abilityID)
		}
		if _, ok := abilities[abilityID]; !ok {
			return fmt.Errorf("%w: referenced ability %q was not found", ErrInvalidCharacterCombatSheet, abilityID)
		}
		seen[abilityID] = true
	}

	return nil
}

func decklistTotal(decklist []DecklistEntry) int {
	total := 0
	for _, entry := range decklist {
		total += entry.Count
	}
	return total
}

func copyDecklist(values []DecklistEntry) []DecklistEntry {
	return append([]DecklistEntry(nil), values...)
}

func copyDiceLoadout(values []DiceLoadoutEntry) []DiceLoadoutEntry {
	return append([]DiceLoadoutEntry(nil), values...)
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}

func copyStatuses(values []StartingStatus) []StartingStatus {
	return append([]StartingStatus(nil), values...)
}

func copyTokens(values []StartingToken) []StartingToken {
	return append([]StartingToken(nil), values...)
}

func normalizedRollPreferences(value RollPreferences) RollPreferences {
	if value.StatusEffects == "" {
		value.StatusEffects = "automatic"
	}
	if value.Offensive == "" {
		value.Offensive = "manual"
	}
	if value.Defensive == "" {
		value.Defensive = "manual"
	}
	return value
}

func validateStartingState(statuses []StartingStatus, tokens []StartingToken, preferences RollPreferences) error {
	statusIDs := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		switch {
		case status.InstanceID == "":
			return fmt.Errorf("%w: status instance_id is required", ErrInvalidContent)
		case status.DefinitionID == "":
			return fmt.Errorf("%w: status definition_id is required", ErrInvalidContent)
		case status.Stacks <= 0:
			return fmt.Errorf("%w: status %q stacks must be positive", ErrInvalidContent, status.InstanceID)
		}
		if _, exists := statusIDs[status.InstanceID]; exists {
			return fmt.Errorf("%w: duplicate status instance_id %q", ErrInvalidContent, status.InstanceID)
		}
		statusIDs[status.InstanceID] = struct{}{}
	}

	tokenIDs := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token.ID == "" {
			return fmt.Errorf("%w: token id is required", ErrInvalidContent)
		}
		if _, exists := tokenIDs[token.ID]; exists {
			return fmt.Errorf("%w: duplicate token id %q", ErrInvalidContent, token.ID)
		}
		tokenIDs[token.ID] = struct{}{}
	}

	normalized := normalizedRollPreferences(preferences)
	for name, mode := range map[string]string{
		"status_effects": normalized.StatusEffects,
		"offensive":      normalized.Offensive,
		"defensive":      normalized.Defensive,
	} {
		if mode != "automatic" && mode != "manual" {
			return fmt.Errorf("%w: roll_preferences.%s must be automatic or manual", ErrInvalidContent, name)
		}
	}
	return nil
}
