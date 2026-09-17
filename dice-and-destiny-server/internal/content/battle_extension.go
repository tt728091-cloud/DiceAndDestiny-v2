package content

import (
	"fmt"
	"path/filepath"
)

// LoadBattleExtension adds a separately versioned character pack to the frozen
// base rules. It never permits a pack to replace an existing definition.
func LoadBattleExtension(lib BattleLibrary, root string) (BattleLibrary, error) {
	var symbols symbolCatalogFile
	if err := loadYAMLFile(filepath.Join(root, "symbols.yaml"), &symbols); err != nil {
		return lib, err
	}
	if symbols.SchemaVersion != 1 {
		return lib, fmt.Errorf("invalid extension schema")
	}
	for _, symbol := range symbols.Symbols {
		if _, exists := lib.Symbols[symbol.ID]; exists {
			return lib, fmt.Errorf("duplicate extension symbol %q", symbol.ID)
		}
		lib.Symbols[symbol.ID] = symbol
	}
	if err := loadBattleItems(filepath.Join(root, "dice"), &lib.Dice); err != nil {
		return lib, err
	}
	if err := loadBattleItems(filepath.Join(root, "statuses"), &lib.Statuses); err != nil {
		return lib, err
	}
	if err := loadBattleItems(filepath.Join(root, "abilities"), &lib.Abilities); err != nil {
		return lib, err
	}
	if err := loadBattleItems(filepath.Join(root, "cards"), &lib.Cards); err != nil {
		return lib, err
	}
	if err := loadBattleItems(filepath.Join(root, "combatants"), &lib.Combatants); err != nil {
		return lib, err
	}
	return lib, validateBattleLibrary(lib)
}
