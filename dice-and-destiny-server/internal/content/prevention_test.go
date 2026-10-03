package content

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedCardDestinationValidation(t *testing.T) {
	for _, kind := range []string{"card", "ability"} {
		for _, value := range []string{"", "original", "discard", "deck", "typo"} {
			lib, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
			if err != nil {
				t.Fatal(err)
			}
			if kind == "card" {
				c := lib.Cards["emergency_ward"]
				c.SavedCardDestination = value
				lib.Cards[c.ID] = c
			} else {
				a := lib.Abilities["basic_defense"]
				a.SavedCardDestination = value
				lib.Abilities[a.ID] = a
			}
			err = validateBattleLibrary(lib)
			valid := value == "" || value == "original" || value == "discard"
			if valid && err != nil {
				t.Fatalf("%s %q: %v", kind, value, err)
			}
			if !valid && (err == nil || !strings.Contains(err.Error(), "saved_card_destination")) {
				t.Fatalf("%s %q must fail clearly: %v", kind, value, err)
			}
		}
	}
}
