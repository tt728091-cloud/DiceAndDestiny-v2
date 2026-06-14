package content_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/content"
)

func TestRepositoryMockGoblinDefinitionLoadsCompleteSetupData(t *testing.T) {
	root := serverRoot(t)
	library, err := content.LoadContentLibrary(filepath.Join(root, "content"))
	if err != nil {
		t.Fatalf("LoadContentLibrary() returned error: %v", err)
	}

	got, err := content.LoadEnemyDefinition(
		filepath.Join(root, "content", "enemies", "mock_goblin.yaml"),
		library,
	)
	if err != nil {
		t.Fatalf("LoadEnemyDefinition() returned error: %v", err)
	}

	if got.ID != "mock_goblin" || got.Name != "Mock Goblin" || got.Class != "goblin" {
		t.Fatalf("enemy metadata = %#v, want mock goblin", got)
	}
	if got.Health.MaxHealth != 6 || len(got.Decklist) != 3 {
		t.Fatalf("enemy health/decklist = %#v / %#v, want six-card health", got.Health, got.Decklist)
	}
	if !reflect.DeepEqual(got.DiceLoadout, []content.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 2}}) {
		t.Fatalf("dice loadout = %#v, want Standard D6 x2", got.DiceLoadout)
	}
	if !reflect.DeepEqual(got.AbilityIDs, []string{"Mock Smite"}) {
		t.Fatalf("abilities = %#v, want Mock Smite", got.AbilityIDs)
	}
	if len(got.Statuses) != 1 || len(got.Tokens) != 1 {
		t.Fatalf("statuses/tokens = %#v / %#v, want authored starting state", got.Statuses, got.Tokens)
	}
}
