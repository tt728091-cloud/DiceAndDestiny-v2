package setup_test

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"diceanddestiny/server/internal/battle/setup"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func TestBattleSetupFromCharacterCombatSheetUsesRepositoryYAMLContent(t *testing.T) {
	contentRoot := filepath.Join(serverRoot(t), "content")
	library, err := content.LoadContentLibrary(contentRoot)
	if err != nil {
		t.Fatalf("LoadContentLibrary() returned error: %v", err)
	}
	sheet, err := content.LoadCharacterCombatSheetWithLibrary(
		filepath.Join(contentRoot, "characters", "mock_paladin.yaml"),
		library,
	)
	if err != nil {
		t.Fatalf("LoadCharacterCombatSheetWithLibrary() returned error: %v", err)
	}

	got, err := setup.BattleSetupFromCharacterCombatSheet(sheet, library)
	if err != nil {
		t.Fatalf("BattleSetupFromCharacterCombatSheet() returned error: %v", err)
	}

	if len(got.Actors) != 1 {
		t.Fatalf("actor count = %d, want 1", len(got.Actors))
	}
	actor := got.Actors[0]
	if actor.ID != "player" {
		t.Fatalf("actor id = %q, want player", actor.ID)
	}
	if len(actor.Deck) != 20 {
		t.Fatalf("deck size = %d, want 20", len(actor.Deck))
	}
	wantLoadout := []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 5}}
	if !reflect.DeepEqual(actor.DiceLoadout, wantLoadout) {
		t.Fatalf("dice loadout = %#v, want %#v", actor.DiceLoadout, wantLoadout)
	}
	if actor.Character.ID != "Mock Paladin" || actor.Character.Class != "paladin" {
		t.Fatalf("character metadata = %#v, want Mock Paladin", actor.Character)
	}
	if actor.Resources.EnergyPoints != 2 || actor.Resources.MaxEnergyPoints != 10 ||
		actor.Resources.StartingHandSize != 4 || actor.Resources.MaxHandSize != 7 {
		t.Fatalf("resources = %#v, want authored player resources", actor.Resources)
	}
	if actor.Health.Model != "card_zones" || actor.Health.MaxHealth != 20 {
		t.Fatalf("health metadata = %#v, want card_zones max 20", actor.Health)
	}
	if len(actor.Decklist) != 3 || len(actor.AbilityIDs) != 4 {
		t.Fatalf("decklist/abilities = %#v / %#v, want complete loadout", actor.Decklist, actor.AbilityIDs)
	}
	if actor.RollPreferences.StatusEffects != state.RollModeAutomatic ||
		actor.RollPreferences.Offensive != state.RollModeManual {
		t.Fatalf("roll preferences = %#v, want authored defaults", actor.RollPreferences)
	}

	if len(got.DiceDefinitions) != 1 {
		t.Fatalf("dice definition count = %d, want 1", len(got.DiceDefinitions))
	}
	die := got.DiceDefinitions[0]
	if die.ID != "Standard D6" || die.SideCount != 6 || len(die.Faces) != 6 {
		t.Fatalf("dice definition = %#v, want Standard D6 from YAML", die)
	}
	for i, face := range die.Faces {
		if face.Face != i+1 || face.Value != i+1 {
			t.Fatalf("face %d = %#v, want face/value %d", i, face, i+1)
		}
		if face.Symbols == nil {
			t.Fatalf("face %d symbols = nil, want authored empty array", i)
		}
	}
}

func serverRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
}
