package loadout

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"

	"diceanddestiny/server/internal/battle/boardprice"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/content"
)

// CustomAdventurer is the character whose sheet chooses its die and holds its
// own priced ability board (the Board Builder). Other characters keep their
// authored dice and boards.
const CustomAdventurer = "custom_adventurer"

// SheetBoard is what a battle pins from a sheet: the die, the board and the
// generated abilities the board uses.
type SheetBoard struct {
	Dice      string                                     `json:"dice,omitempty"`
	Abilities content.AbilityBoard                       `json:"ability_board"`
	Generated map[string]content.BattleAbilityDefinition `json:"generated_abilities,omitempty"`
	// Choice is the Board Builder choice behind the board, when it has one.
	Choice *boardprice.Choice `json:"board_choice,omitempty"`
}

// ReadSheetBoard reads the die and board from a character's saved sheet
// without validating, migrating or writing it (battle assembly). ok is false
// when the character has no sheet yet.
func ReadSheetBoard(root, character string) (SheetBoard, bool, error) {
	filename, err := progressPath(root, character)
	if err != nil {
		return SheetBoard{}, false, err
	}
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return SheetBoard{}, false, nil
	}
	if err != nil {
		return SheetBoard{}, false, err
	}
	var sheet SheetBoard
	if err := json.Unmarshal(data, &sheet); err != nil {
		return SheetBoard{}, false, err
	}
	return sheet, true, nil
}

// OverlayCustomBoard applies the custom adventurer's saved die, board and
// generated abilities to a catalog, so character views and battles see the
// sheet. Libraries without the character, and roots without a sheet, are
// returned unchanged.
func OverlayCustomBoard(lib content.BattleLibrary, root string) (content.BattleLibrary, error) {
	if root == "" {
		return lib, nil
	}
	if _, ok := lib.Combatants[CustomAdventurer]; !ok {
		return lib, nil
	}
	sheet, ok, err := ReadSheetBoard(root, CustomAdventurer)
	if err != nil || !ok {
		return lib, err
	}
	return ApplySheetBoard(lib, CustomAdventurer, sheet)
}

// ApplySheetBoard adds a sheet's generated abilities to a copy of the catalog
// and gives the character the sheet's die (five of it) and board. The result
// is validated: the die has roles and every offensive ability can qualify on
// it.
func ApplySheetBoard(lib content.BattleLibrary, character string, sheet SheetBoard) (content.BattleLibrary, error) {
	combatant, ok := lib.Combatants[character]
	if !ok {
		return lib, fmt.Errorf("unknown character %q", character)
	}
	if character != CustomAdventurer && (sheet.Dice != "" || len(sheet.Generated) > 0) {
		return lib, fmt.Errorf("%s does not choose its die or board", character)
	}
	out := lib
	out.Abilities = maps.Clone(lib.Abilities)
	out.Combatants = maps.Clone(lib.Combatants)
	for id, ability := range sheet.Generated {
		if id != ability.ID || len(id) <= len(boardprice.GeneratedPrefix) || id[:len(boardprice.GeneratedPrefix)] != boardprice.GeneratedPrefix {
			return lib, fmt.Errorf("generated ability %q must use the %s prefix", id, boardprice.GeneratedPrefix)
		}
		if ability.Type != "offensive" || ability.ConfigurationVersion != 1 {
			return lib, fmt.Errorf("generated ability %q must be a configured offensive ability", id)
		}
		out.Abilities[id] = ability
	}
	if sheet.Dice != "" {
		combatant.DiceLoadout = []content.DiceLoadoutEntry{{DiceID: sheet.Dice, Count: content.LoadoutDiceCount}}
	}
	if len(sheet.Abilities.Offensive) > 0 || len(sheet.Abilities.Defensive) > 0 {
		combatant.AbilityBoard = content.AbilityBoard{Offensive: slices.Clone(sheet.Abilities.Offensive), Defensive: slices.Clone(sheet.Abilities.Defensive)}
	}
	out.Combatants[character] = combatant
	if len(sheet.Generated) > 0 {
		if err := content.ValidateLibrary(out); err != nil {
			return lib, err
		}
	}
	if err := ValidateAbilities(combatant.AbilityBoard, out); err != nil {
		return lib, err
	}
	if err := validateSheetDice(out, character, sheet.Dice, combatant.AbilityBoard); err != nil {
		return lib, err
	}
	return out, nil
}

// validateSheetDice checks a sheet's die choice: only the custom adventurer
// chooses one, it must be a catalog die with roles, and every offensive
// ability on the board must be able to qualify on five of it.
func validateSheetDice(lib content.BattleLibrary, character, dice string, board content.AbilityBoard) error {
	if dice == "" {
		return nil
	}
	if character != CustomAdventurer {
		return fmt.Errorf("%s does not choose its die", character)
	}
	die, ok := lib.Dice[dice]
	if !ok {
		return fmt.Errorf("unknown die %q", dice)
	}
	if _, err := content.DieRoles(die); err != nil {
		return err
	}
	loadout := []content.DiceLoadoutEntry{{DiceID: dice, Count: content.LoadoutDiceCount}}
	for _, id := range board.Offensive {
		if !engine.AbilityFitsDice(lib.Abilities[id], loadout, lib) {
			return fmt.Errorf("%s can never qualify on %s", lib.Abilities[id].Name, die.Name)
		}
	}
	return nil
}

// SaveSheetBoard writes a die, a board and the generated abilities it uses to
// a sheet, after validating them against the catalog: the die must have
// roles, every ability must exist (or be generated) and qualify on the die,
// and the board must pass the character's access rules. Abilities the new
// board no longer uses are dropped from the sheet.
func SaveSheetBoard(root, character string, e Economy, lib content.BattleLibrary, sheet SheetBoard, revision int) (Progress, error) {
	progressMu.Lock()
	defer progressMu.Unlock()
	e, _, err := effectiveEconomy(root, e)
	if err != nil {
		return Progress{}, err
	}
	p, err := readProgress(root, character, e, lib)
	if err != nil {
		return p, err
	}
	if revision != p.Revision {
		return p, fmt.Errorf("sheet changed; refresh before saving the board")
	}
	used := map[string]content.BattleAbilityDefinition{}
	for _, id := range sheet.Abilities.Offensive {
		if ability, ok := sheet.Generated[id]; ok {
			used[id] = ability
		}
	}
	sheet.Generated = used
	if _, err := ApplySheetBoard(lib, character, sheet); err != nil {
		return p, err
	}
	if err := e.Access.ValidateBoard(character, sheet.Abilities); err != nil {
		return p, err
	}
	next := cloneProgress(p)
	next.Dice = sheet.Dice
	next.Abilities = content.AbilityBoard{Offensive: slices.Clone(sheet.Abilities.Offensive), Defensive: slices.Clone(sheet.Abilities.Defensive)}
	next.GeneratedAbilities = nil
	if len(used) > 0 {
		next.GeneratedAbilities = used
	}
	next.BoardChoice = sheet.Choice
	next.Revision++
	filename, _ := progressPath(root, character)
	if err := writeProgress(filename, next); err != nil {
		return p, err
	}
	return next, nil
}
