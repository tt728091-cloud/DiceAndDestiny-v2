package learned

import (
	"bytes"
	"diceanddestiny/server/internal/battle/loadout"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"fmt"
)

func handleCardAuthoring(r runtimeRequest) string {
	catalogs, err := CharacterCatalogs(r.ContentRoot, r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	lib := catalogs["adventurer"]
	saved, err := content.ReadAuthoredCards(r.LoadoutRoot)
	if err != nil {
		return runtimeError(err)
	}
	if r.Op == "card_authoring" {
		economy, err := loadout.LoadEconomy(r.ContentRoot, catalogs)
		if err != nil {
			return runtimeError(err)
		}
		templates := map[string]content.BattleCardDefinition{}
		for id, c := range lib.Cards {
			editable, err := content.EditableCard(c)
			if err == nil {
				if editable.AccessType == "" {
					editable.AccessType = economy.Access.Cards[id]
					if editable.AccessType == "" {
						editable.AccessType = "general"
					}
				}
				if editable.Economy == nil {
					character := editable.AccessType
					if character == "general" {
						character = "adventurer"
					}
					price := economy.Price(character, id)
					editable.Economy = &content.CardEconomy{Buy: price, Sell: price, CopyLimit: loadout.MaxCopies}
					if u, ok := economy.CardUpgrades(character)[id]; ok {
						editable.Economy.Upgrades = []content.CardUpgrade{{To: u.To, XP: u.XP}}
					}
				}
				templates[id] = editable
			}
		}
		names := map[string]string{}
		for id, card := range lib.Cards {
			names[id] = card.Name
		}
		targets := map[string]content.CardTargetSpec{}
		for effect := range content.CardCapabilities() {
			targets[effect] = content.CardTargetCapabilities(effect)
		}
		return runtimeSuccess(map[string]any{"dice": lib.Dice, "mechanics": content.CardMechanics(), "abilities": lib.Abilities, "access_types": economy.Access.Types, "card_names": names, "target_rules": targets, "condition_types": []string{"symbol_count", "number_pattern", "exact_faces"}, "patterns": []string{"three_of_a_kind", "exact_pair", "pair_or_better", "small_straight", "large_straight"}, "limits": map[string]int{"steps": 32, "options": 16, "depth": 4, "conditions": 16, "count": 100}, "revision": saved.Revision, "effects": content.CardCapabilities(), "windows": content.CardWindows, "templates": templates, "statuses": lib.Statuses, "symbols": lib.Symbols, "target_owners": []string{"self", "enemy", "any"}, "target_modes": []string{"one", "exact", "up_to", "all"}, "target_selections": []string{"choose", "random"}, "source_zones": []string{"hand", "discard", "deck"}, "destinations": []string{"hand", "discard", "deck", "removed"}, "roll_requirements": []string{"any", "before_first", "after_first"}})
	}
	// Godot round-trips JSON integers as integral floats. Canonicalize numeric
	// spelling while retaining strict typed and unknown-field validation.
	var incoming any
	if err = json.Unmarshal(r.Card, &incoming); err != nil {
		return runtimeError(err)
	}
	r.Card, err = json.Marshal(incoming)
	if err != nil {
		return runtimeError(err)
	}
	var c content.BattleCardDefinition
	d := json.NewDecoder(bytes.NewReader(r.Card))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return runtimeError(err)
	}
	if c.Program == nil && c.Mechanic == nil {
		return runtimeError(fmt.Errorf("authored card requires a program or specialized mechanic"))
	}
	c = content.PrepareProgramCard(c, &lib)
	c = content.PrepareMechanicCard(c, &lib)
	lib.Cards[c.ID] = c
	if err = content.ValidateAuthoredCard(c, lib); err != nil {
		return runtimeError(err)
	}
	if r.Op == "publish_card" {
		for id, candidate := range catalogs {
			candidate.Cards[c.ID] = c
			catalogs[id] = candidate
		}
		economy, econErr := loadout.LoadEconomy(r.ContentRoot, catalogs)
		if econErr != nil {
			return runtimeError(econErr)
		}
		if econErr = loadout.ValidateCatalogUpdate(r.LoadoutRoot, economy, catalogs); econErr != nil {
			return runtimeError(econErr)
		}
		out, err := content.SaveAuthoredCard(r.LoadoutRoot, lib, c, r.CatalogRevision)
		if err != nil {
			return runtimeError(err)
		}
		return runtimeSuccess(map[string]any{"revision": out.Revision, "card": out.Cards[c.ID]})
	}
	return runtimeSuccess(map[string]any{"card": c, "rules": c.Presentation.RulesText})
}
