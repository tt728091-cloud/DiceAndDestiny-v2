package content

import (
	"path/filepath"
	"testing"
)

func TestBlankCardsOnlyCountAsHealth(t *testing.T) {
	lib, err := LoadBattleLibrary(filepath.Join("..", "..", "content", "battle_v1"))
	if err != nil {
		t.Fatal(err)
	}
	lib, err = LoadBattleExtension(lib, filepath.Join("..", "..", "content", "minions_v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ballast_stone", "shed_ribbon"} {
		if !IsBlankCard(lib.Cards[id]) {
			t.Fatalf("%s should be blank", id)
		}
	}
	if IsBlankCard(lib.Cards["brine_surge"]) {
		t.Fatal("Brine Surge has an effect")
	}
	// Only a card without effects may omit its play timing.
	surge := lib.Cards["brine_surge"]
	surge.Play.PlayableDuring = nil
	lib.Cards["brine_surge"] = surge
	if err := validateBattleLibrary(lib); err == nil {
		t.Fatal("an effect card without play timing must be rejected")
	}
}
