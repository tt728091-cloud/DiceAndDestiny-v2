package content

import (
	"path/filepath"
	"testing"
)

func TestGeneralExtensionValidation(t *testing.T) {
	lib, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	lib, err = LoadBattleExtension(lib, filepath.Join("..", "..", "content", "general_v1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Cards) != 15 {
		t.Fatal("extension must add exactly eight cards")
	}
	original := lib.Cards["reinforce"]
	for _, mutation := range []func(*BattleCardDefinition){
		func(c *BattleCardDefinition) { c.Targeting.Selector = "self" },
		func(c *BattleCardDefinition) { c.Operations = nil },
		func(c *BattleCardDefinition) { c.Operations[0].Modification = "unknown" },
		func(c *BattleCardDefinition) { c.Operations[0].ExtraEnergy = -1 },
		func(c *BattleCardDefinition) { c.Operations[0].BonusAmount = 0 },
		func(c *BattleCardDefinition) { c.Operations[0].Modification = "flip_die" },
	} {
		card := original
		card.Operations = append([]BattleOperation(nil), original.Operations...)
		mutation(&card)
		lib.Cards[card.ID] = card
		if validateBattleLibrary(lib) == nil {
			t.Fatalf("invalid general configuration accepted: %+v", card)
		}
	}
}
