package loadout

import (
	"fmt"
	"maps"

	"diceanddestiny/server/internal/content"
)

// Pool assignments are configuration, independent of card timing/ability kind.
// General is shared; a character also has access to its selected specialization.
type AccessRules struct {
	Types      map[string]string `yaml:"types" json:"types"`
	Characters map[string]string `yaml:"character_types" json:"character_types"`
	Cards      map[string]string `yaml:"card_types" json:"card_types"`
	Abilities  map[string]string `yaml:"ability_types" json:"ability_types"`
}

func pool(m map[string]string, id string) string {
	if value := m[id]; value != "" {
		return value
	}
	return "general"
}
func (a AccessRules) Allows(character, kind, id string) bool {
	assignments := a.Cards
	if kind == "abilities" {
		assignments = a.Abilities
	}
	required := pool(assignments, id)
	return required == "general" || required == pool(a.Characters, character)
}
func (a AccessRules) Check(character, kind, id string) error {
	if a.Allows(character, kind, id) {
		return nil
	}
	assignments := a.Cards
	if kind == "abilities" {
		assignments = a.Abilities
	}
	return fmt.Errorf("%s requires the %s type; %s has the %s type", id, pool(assignments, id), character, pool(a.Characters, character))
}
func (a AccessRules) ValidateDeck(character string, deck []Entry) error {
	for _, card := range deck {
		if err := a.Check(character, "cards", card.CardID); err != nil {
			return err
		}
	}
	return nil
}
func (a AccessRules) ValidateBoard(character string, board content.AbilityBoard) error {
	for _, ids := range [][]string{board.Offensive, board.Defensive} {
		for _, id := range ids {
			if err := a.Check(character, "abilities", id); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a AccessRules) Problems(character string, deck []Entry, board content.AbilityBoard) []string {
	problems := []string{}
	for _, card := range deck {
		if err := a.Check(character, "cards", card.CardID); err != nil {
			problems = append(problems, err.Error())
		}
	}
	for _, ids := range [][]string{board.Offensive, board.Defensive} {
		for _, id := range ids {
			if err := a.Check(character, "abilities", id); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}
	return problems
}
func (a AccessRules) withOverrides(s AdminSettings) AccessRules {
	a.Characters = maps.Clone(a.Characters)
	a.Cards = maps.Clone(a.Cards)
	a.Abilities = maps.Clone(a.Abilities)
	if a.Characters == nil {
		a.Characters = map[string]string{}
	}
	if a.Cards == nil {
		a.Cards = map[string]string{}
	}
	if a.Abilities == nil {
		a.Abilities = map[string]string{}
	}
	maps.Copy(a.Characters, s.CharacterTypes)
	maps.Copy(a.Cards, s.CardTypes)
	maps.Copy(a.Abilities, s.AbilityTypes)
	return a
}
func (a AccessRules) validate(catalogs map[string]content.BattleLibrary) error {
	if a.Types["general"] == "" {
		return fmt.Errorf("access types must define General")
	}
	for kind, assignments := range map[string]map[string]string{"characters": a.Characters, "cards": a.Cards, "abilities": a.Abilities} {
		for id, typeID := range assignments {
			if a.Types[typeID] == "" {
				return fmt.Errorf("unknown type %q for %s %q", typeID, kind, id)
			}
			found := false
			for character, lib := range catalogs {
				switch kind {
				case "characters":
					found = found || character == id
				case "cards":
					_, ok := lib.Cards[id]
					found = found || ok
				case "abilities":
					_, ok := lib.Abilities[id]
					found = found || ok
				}
			}
			if !found {
				return fmt.Errorf("unknown %s assignment %q", kind, id)
			}
		}
	}
	return nil
}

// Read effective access without creating progression saves (used by Sandbox).
func ReadAccess(root string, base Economy) (AccessRules, error) {
	if root == "" {
		return base.Access, nil
	}
	effective, _, err := effectiveEconomy(root, base)
	return effective.Access, err
}
