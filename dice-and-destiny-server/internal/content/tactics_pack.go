package content

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
)

// TacticsPackManifest is content/tactics_v1/pack.yaml: the Tactics Sandbox's
// ability copies and the heroes and enemies its setup screen offers.
type TacticsPackManifest struct {
	SchemaVersion int `yaml:"schema_version" json:"schema_version"`
	// AbilityCopies are shipped abilities the pack redefines for tactics
	// battles (range, area, on the move). Each replaces the definition with
	// the same ID in a tactics battle's catalog only.
	AbilityCopies []string `yaml:"ability_copies" json:"ability_copies"`
	Heroes        []string `yaml:"heroes" json:"heroes"`
	Enemies       []string `yaml:"enemies" json:"enemies"`
}

// ApplyTacticsPack adds the tactics pack at root to a battle library: its
// statuses, abilities and combatants. Abilities listed as copies replace the
// shipped definition of the same ID; any other ID must be new. Loading is
// strict and the result is validated as a whole. Only tactics battles call it.
func ApplyTacticsPack(lib BattleLibrary, root string) (BattleLibrary, TacticsPackManifest, error) {
	var manifest TacticsPackManifest
	if err := loadYAMLFile(filepath.Join(root, "pack.yaml"), &manifest); err != nil {
		return lib, manifest, err
	}
	if manifest.SchemaVersion != 1 || len(manifest.Heroes) == 0 || len(manifest.Enemies) == 0 {
		return lib, manifest, fmt.Errorf("%w: tactics pack needs schema_version 1, heroes and enemies", ErrInvalidContent)
	}
	out := BattleLibrary{CardTrees: lib.CardTrees, Symbols: maps.Clone(lib.Symbols), Dice: maps.Clone(lib.Dice), Cards: maps.Clone(lib.Cards), Abilities: maps.Clone(lib.Abilities), Statuses: maps.Clone(lib.Statuses), Combatants: maps.Clone(lib.Combatants)}
	statuses := map[string]BattleStatusDefinition{}
	abilities := map[string]BattleAbilityDefinition{}
	combatants := map[string]CombatantDefinition{}
	if err := loadBattleItems(filepath.Join(root, "statuses"), &statuses); err != nil {
		return lib, manifest, err
	}
	if err := loadBattleItems(filepath.Join(root, "abilities"), &abilities); err != nil {
		return lib, manifest, err
	}
	if err := loadBattleItems(filepath.Join(root, "combatants"), &combatants); err != nil {
		return lib, manifest, err
	}
	for id, status := range statuses {
		if _, exists := out.Statuses[id]; exists {
			return lib, manifest, fmt.Errorf("%w: tactics status %q already exists", ErrInvalidContent, id)
		}
		out.Statuses[id] = status
	}
	for _, id := range manifest.AbilityCopies {
		if _, ok := abilities[id]; !ok {
			return lib, manifest, fmt.Errorf("%w: tactics ability copy %q has no file", ErrInvalidContent, id)
		}
		if _, ok := out.Abilities[id]; !ok {
			return lib, manifest, fmt.Errorf("%w: tactics ability copy %q copies no shipped ability", ErrInvalidContent, id)
		}
	}
	for id, ability := range abilities {
		_, exists := out.Abilities[id]
		if exists != slices.Contains(manifest.AbilityCopies, id) {
			return lib, manifest, fmt.Errorf("%w: tactics ability %q must be new or listed in ability_copies", ErrInvalidContent, id)
		}
		if exists && out.Abilities[id].Type != ability.Type {
			return lib, manifest, fmt.Errorf("%w: tactics copy %q changes the ability type", ErrInvalidContent, id)
		}
		out.Abilities[id] = ability
	}
	for id, combatant := range combatants {
		if _, exists := out.Combatants[id]; exists {
			return lib, manifest, fmt.Errorf("%w: tactics combatant %q already exists", ErrInvalidContent, id)
		}
		out.Combatants[id] = combatant
	}
	for _, id := range manifest.Heroes {
		c, ok := combatants[id]
		if !ok || c.Tactics != nil || c.SingleAbilityPolicy != nil {
			return lib, manifest, fmt.Errorf("%w: tactics hero %q must be a pack combatant without a scripted policy", ErrInvalidContent, id)
		}
	}
	for _, id := range manifest.Enemies {
		c, ok := combatants[id]
		if !ok || c.Tactics == nil || c.SingleAbilityPolicy == nil {
			return lib, manifest, fmt.Errorf("%w: tactics enemy %q needs a tactics block and a scripted policy", ErrInvalidContent, id)
		}
	}
	if err := validateBattleLibrary(out); err != nil {
		return lib, manifest, err
	}
	return out, manifest, nil
}
