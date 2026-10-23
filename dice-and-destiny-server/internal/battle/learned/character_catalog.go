package learned

import (
	"fmt"
	"path/filepath"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
)

// PlayableCharacters are the characters a player can pick, in roster order.
var PlayableCharacters = []string{"adventurer", "venom", "curse", "blade_warden", "starter", loadout.CustomAdventurer}

// CharacterCatalogs reads definitions without creating/resetting a battle,
// initializing a policy, drawing cards, or writing any player state.
func CharacterCatalogs(root string, authoredRoots ...string) (map[string]content.BattleLibrary, error) {
	if root == "" {
		return nil, fmt.Errorf("content_root is required")
	}
	result := map[string]content.BattleLibrary{}
	for _, id := range PlayableCharacters {
		lib, err := characterCatalog(root, id, authoredRoots...)
		if err != nil {
			return nil, err
		}
		result[id] = lib
	}
	return result, nil
}

// characterCatalog is one playable character's catalog: the base rules, every
// character pack and, with a loadout root, its authored content and sheet.
func characterCatalog(root, id string, authoredRoots ...string) (content.BattleLibrary, error) {
	lib, err := content.LoadBattleLibrary(filepath.Join(root, "battle_v1"))
	if err != nil {
		return lib, err
	}
	packs := []string{"venom_v1", "curse_v1", "adventurer_v1", "general_v1", "minions_v1"}
	if id == loadout.CustomAdventurer {
		packs = append(packs, battle.CustomAdventurerPack)
	}
	for _, pack := range packs {
		lib, err = content.LoadBattleExtension(lib, filepath.Join(root, pack))
		if err != nil {
			return lib, err
		}
	}
	if len(authoredRoots) > 0 {
		lib, err = content.OverlayAuthoredCards(lib, authoredRoots[0])
		if err != nil {
			return lib, err
		}
		if id == loadout.CustomAdventurer {
			// The sheet's chosen die and priced board.
			if lib, err = loadout.OverlayCustomBoard(lib, authoredRoots[0]); err != nil {
				return lib, err
			}
		}
	}
	if _, ok := lib.Combatants[id]; !ok {
		return lib, fmt.Errorf("character %q not found", id)
	}
	return lib, nil
}

// Use an explicit presentation envelope instead of changing legacy saved JSON
// tags on DecklistEntry/DiceLoadoutEntry (which remain backward compatible).
func characterCatalogView(catalogs map[string]content.BattleLibrary) map[string]any {
	result := map[string]any{}
	for id, lib := range catalogs {
		c := lib.Combatants[id]
		deck := []map[string]any{}
		for _, entry := range c.Decklist {
			deck = append(deck, map[string]any{"card_id": entry.CardID, "count": entry.Count})
		}
		dice := []map[string]any{}
		for _, entry := range c.DiceLoadout {
			dice = append(dice, map[string]any{"dice_id": entry.DiceID, "count": entry.Count})
		}
		character := map[string]any{"id": c.ID, "name": c.Name, "class": c.Class, "resources": c.Resources, "income": c.Income, "decklist": deck, "dice_loadout": dice, "ability_board": c.AbilityBoard, "starting_statuses": c.StartingStatuses}
		result[id] = map[string]any{"symbols": lib.Symbols, "cards": lib.Cards, "abilities": lib.Abilities, "dice": lib.Dice, "statuses": lib.Statuses, "combatants": map[string]any{id: character}}
	}
	return result
}
