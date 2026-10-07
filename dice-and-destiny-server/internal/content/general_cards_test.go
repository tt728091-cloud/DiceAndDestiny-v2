package content

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// legacyReinforce is the pre-program general_choice definition. Pinned battles
// may still carry it, so its validation stays covered.
const legacyReinforce = `{"schema_version":1,"id":"reinforce","name":"Reinforce","type":"action","presentation":{"rules_text":"Prevent 2 damage from one incoming source. You may pay 1 extra energy to prevent 4 instead. Saved cards remain in their current piles. Played cards stay played.","effect_summary":"Prevent 2 damage. Pay +1 energy to prevent 4 instead.","illustration_path":"res://assets/battle/cards/emergency_ward.png"},"cost":{"energy":1},"play":{"source_zones":["hand"],"destination":"discard","playable_during":[{"segment":"defensive","phase":"main","window_purpose":"reaction"},{"segment":"damage_resolution","phase":"main","window_purpose":"reaction"}]},"targeting":{"selector":"general_choice","minimum":1,"maximum":1},"reaction_window":{"opens":false},"operations":[{"type":"general_card","modification":"boost_prevention","amount":2,"extra_energy":1,"bonus_amount":2}],"saved_card_destination":"original"}`

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
	for _, id := range []string{"dispel", "disrupt", "matchmaker", "reclaim", "reinforce", "second_guard", "triage", "turn_the_die"} {
		if lib.Cards[id].Program == nil {
			t.Fatalf("%s is not a program card", id)
		}
	}
	var original BattleCardDefinition
	if err = json.Unmarshal([]byte(legacyReinforce), &original); err != nil {
		t.Fatal(err)
	}
	lib.Cards[original.ID] = original
	if err = validateBattleLibrary(lib); err != nil {
		t.Fatalf("legacy general card rejected: %v", err)
	}
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
