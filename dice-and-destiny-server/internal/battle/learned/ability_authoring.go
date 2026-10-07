package learned

import (
	"bytes"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
)

func handleAbilityAuthoring(r runtimeRequest) string {
	catalogs, err := CharacterCatalogs(r.ContentRoot, r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	lib := catalogs["adventurer"]
	saved, err := content.ReadAuthoredAbilities(r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	if r.Op == "ability_authoring" {
		templates := map[string]content.BattleAbilityDefinition{}
		for id, a := range lib.Abilities {
			a.ConfigurationVersion = 1
			templates[id] = a
		}
		compatible := map[string][]string{}
		economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		access, err := loadout.ReadAccess(r.LoadoutRoot, economy)
		if err != nil {
			return runtimeError(err)
		}
		for character, c := range catalogs {
			for id, a := range c.Abilities {
				if access.Allows(character, "abilities", id) && engine.AbilityFitsDice(a, c.Combatants[character].DiceLoadout, c) {
					compatible[character] = append(compatible[character], id)
				}
			}
		}
		boards := map[string]content.AbilityBoard{}
		for id, c := range catalogs {
			boards[id] = c.Combatants[id].AbilityBoard
			progress, err := loadout.ReadProgress(r.LoadoutRoot, id, economy, c)
			if err != nil {
				return runtimeError(err)
			}
			if progress.AuthoredAbilityRevision > 0 {
				boards[id] = progress.Abilities
			}
		}
		return runtimeSuccess(map[string]any{"revision": saved.Revision, "compatible": compatible, "templates": templates, "boards": boards, "symbols": lib.Symbols, "dice": lib.Dice, "statuses": lib.Statuses, "effects": []string{"deal_damage", "draw_cards", "gain_resource", "apply_status", "remove_status_stack", "prevent_damage", "scale_damage", "roll_dice", "provoke", "apply_incubation", "incubation_or_poison", "special_effect", "noop"}, "special_effects": []string{"curse", "roll_cursed", "entomb_choice", "curse_face_choice", "curse_die_choice", "status_threshold", "conditional_status"}, "hook_timings": []string{"before_defense", "after_provoke", "after_damage", "after_defense"}, "patterns": []string{"three_of_a_kind", "exact_pair", "pair_or_better", "small_straight", "large_straight"}, "destinations": []string{"original", "discard"}})
	}
	if r.Op == "assign_abilities" {
		economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		access, err := loadout.ReadAccess(r.LoadoutRoot, economy)
		if err != nil {
			return runtimeError(err)
		}
		if err = access.ValidateBoard(r.Character, r.AbilityBoard); err != nil {
			return runtimeError(err)
		}
		for _, id := range r.AbilityBoard.Offensive {
			if !engine.AbilityFitsDice(lib.Abilities[id], lib.Combatants[r.Character].DiceLoadout, lib) {
				return runtimeError(fmt.Errorf("%s cannot qualify with %s dice", id, r.Character))
			}
		}
		out, err := content.SaveAuthoredAbility(r.LoadoutRoot, lib, nil, r.Character, &r.AbilityBoard, r.CatalogRevision)
		if err != nil {
			return runtimeError(err)
		}
		return runtimeSuccess(map[string]any{"revision": out.Revision, "board": r.AbilityBoard})
	}
	var incoming any
	if err = json.Unmarshal(r.Ability, &incoming); err != nil {
		return runtimeError(err)
	}
	raw, _ := json.Marshal(incoming)
	var a content.BattleAbilityDefinition
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&a); err != nil {
		return runtimeError(err)
	}
	a.Presentation.RulesText = "Generated from ability configuration."
	if err = content.ValidateAuthoredAbility(a, lib); err != nil {
		return runtimeError(err)
	}
	a.Presentation.RulesText = content.AbilityRules(a, lib)
	// Editing an assigned definition must not silently invalidate a character's dice.
	for id, c := range lib.Combatants {
		for _, assigned := range c.AbilityBoard.Offensive {
			if assigned == a.ID && !engine.AbilityFitsDice(a, c.DiceLoadout, lib) {
				return runtimeError(fmt.Errorf("%s cannot qualify with assigned character %s dice", a.ID, id))
			}
		}
	}
	if r.Op == "publish_ability" {
		out, err := content.SaveAuthoredAbility(r.LoadoutRoot, lib, &a, "", nil, r.CatalogRevision)
		if err != nil {
			return runtimeError(err)
		}
		return runtimeSuccess(map[string]any{"revision": out.Revision, "ability": out.Abilities[a.ID]})
	}
	return runtimeSuccess(map[string]any{"ability": a, "rules": a.Presentation.RulesText})
}
