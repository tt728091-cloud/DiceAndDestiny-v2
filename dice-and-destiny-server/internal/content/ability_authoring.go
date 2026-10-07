package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

type AuthoredAbilities struct {
	BoardRevisions map[string]int                     `json:"board_revisions,omitempty"`
	Version        int                                `json:"version"`
	Revision       int                                `json:"revision"`
	Abilities      map[string]BattleAbilityDefinition `json:"abilities"`
	Boards         map[string]AbilityBoard            `json:"boards"`
}

func ReadAuthoredAbilities(root string) (AuthoredAbilities, error) {
	out := AuthoredAbilities{Version: 1, Abilities: map[string]BattleAbilityDefinition{}, Boards: map[string]AbilityBoard{}}
	if root == "" {
		return out, nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "authored_abilities.json"))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&out); err != nil {
		return out, err
	}
	if out.Version != 1 || out.Abilities == nil || out.Boards == nil {
		return out, fmt.Errorf("invalid ability store")
	}
	return out, nil
}
func applyAuthoredAbilities(lib BattleLibrary, saved AuthoredAbilities) (BattleLibrary, error) {
	lib.Abilities = maps.Clone(lib.Abilities)
	lib.Combatants = maps.Clone(lib.Combatants)
	for id, a := range saved.Abilities {
		if id != a.ID || a.ConfigurationVersion != 1 {
			return lib, fmt.Errorf("invalid authored ability %s", id)
		}
		lib.Abilities[id] = a
	}
	for id, board := range saved.Boards {
		if c, ok := lib.Combatants[id]; ok {
			c.AbilityBoard = board
			lib.Combatants[id] = c
		}
	}
	return lib, nil
}
func ValidateAuthoredAbility(a BattleAbilityDefinition, lib BattleLibrary) error {
	if a.ConfigurationVersion != 1 {
		return fmt.Errorf("configuration_version must be 1")
	}
	if existing, ok := lib.Abilities[a.ID]; ok && existing.Type != a.Type {
		return fmt.Errorf("an existing ability cannot change type; create a new ID")
	}
	if a.ID == "" {
		return fmt.Errorf("ability ID required")
	}
	lib.Abilities = maps.Clone(lib.Abilities)
	lib.Abilities[a.ID] = a
	return validateBattleLibrary(lib)
}
func SaveAuthoredAbility(root string, lib BattleLibrary, a *BattleAbilityDefinition, character string, board *AbilityBoard, revision int) (AuthoredAbilities, error) {
	authoredMu.Lock()
	defer authoredMu.Unlock()
	saved, err := ReadAuthoredAbilities(root)
	if err != nil {
		return saved, err
	}
	if root == "" || revision != saved.Revision {
		return saved, fmt.Errorf("ability catalog changed; reload before publishing")
	}
	if a != nil {
		if err = ValidateAuthoredAbility(*a, lib); err != nil {
			return saved, err
		}
		saved.Abilities[a.ID] = *a
	}
	if board != nil {
		switch character {
		case "adventurer", "venom", "curse", "blade_warden":
		default:
			return saved, fmt.Errorf("unknown playable character")
		}
		if err = ValidateAbilityBoard(*board, lib); err != nil {
			return saved, err
		}
		saved.Boards[character] = *board
		if saved.BoardRevisions == nil {
			saved.BoardRevisions = map[string]int{}
		}
		saved.BoardRevisions[character] = saved.Revision + 1
	}
	candidate, err := applyAuthoredAbilities(lib, saved)
	if err != nil {
		return saved, err
	}
	if err = validateBattleLibrary(candidate); err != nil {
		return saved, err
	}
	saved.Revision++
	if err = os.MkdirAll(root, 0755); err != nil {
		return saved, err
	}
	raw, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return saved, err
	}
	f, err := os.CreateTemp(root, ".abilities-*.json")
	if err != nil {
		return saved, err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err == nil {
		err = os.Rename(f.Name(), filepath.Join(root, "authored_abilities.json"))
	}
	return saved, err
}
func ValidateAbilityBoard(board AbilityBoard, lib BattleLibrary) error {
	seen := map[string]bool{}
	for kind, ids := range map[string][]string{"offensive": board.Offensive, "defensive": board.Defensive} {
		if len(ids) < 1 || len(ids) > 20 {
			return fmt.Errorf("board needs 1–20 %s abilities", kind)
		}
		for _, id := range ids {
			if seen[id] || lib.Abilities[id].Type != kind {
				return fmt.Errorf("invalid %s ability %s", kind, id)
			}
			seen[id] = true
		}
	}
	return nil
}
