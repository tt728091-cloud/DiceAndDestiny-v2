package devsnapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestStoreSaveListLoadOverwriteAndValidation(t *testing.T) {
	root := t.TempDir()
	when := time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)
	store := Store{Root: root, Now: func() time.Time { return when }}
	checkpoint := testCheckpoint(t, "battle-source")

	metadata, err := store.Save("round-2-effects", "blade", checkpoint, false)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if metadata.Name != "round-2-effects" || metadata.Round != 2 || metadata.ActorID != "blade" || !metadata.CreatedAt.Equal(when) {
		t.Fatalf("saved metadata = %#v", metadata)
	}
	if _, err := store.Save("round-2-effects", "blade", checkpoint, false); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate Save() error = %v", err)
	}
	if _, err := store.Save("../escape", "blade", checkpoint, false); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("unsafe Save() error = %v", err)
	}

	listed, err := store.List()
	if err != nil || len(listed) != 1 || listed[0].Name != metadata.Name {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	loaded, err := store.Load("round-2-effects")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded.Checkpoint, checkpoint) {
		t.Fatal("loaded checkpoint differs from saved authority checkpoint")
	}
	legacyMetadata := metadata
	legacyMetadata.Name = "legacy-checkpoint"
	legacyData, err := json.Marshal(Record{SchemaVersion: legacySchemaVersion, Metadata: legacyMetadata, Checkpoint: checkpoint})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "legacy-checkpoint.json"), legacyData, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.Load("legacy-checkpoint")
	if err != nil || legacy.History != nil || legacy.Metadata.HistoryIncluded {
		t.Fatalf("legacy snapshot compatibility = %#v, %v", legacy, err)
	}

	checkpoint.Battle.Random.Cursor = 9
	if _, err := store.Save("round-2-effects", "blade", checkpoint, true); err != nil {
		t.Fatalf("overwrite Save() returned error: %v", err)
	}
	replaced, err := store.Load("round-2-effects")
	if err != nil || replaced.Checkpoint.Battle.Random.Cursor != 9 {
		t.Fatalf("overwritten snapshot = %#v, %v", replaced, err)
	}

	if err := os.WriteFile(filepath.Join(root, "corrupt.json"), []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("List() corrupt error = %v", err)
	}
}

func testCheckpoint(t *testing.T, battleID string) repository.Checkpoint {
	t.Helper()
	battle, err := state.NewBattleFromSetup(battleID, state.BattleSetup{Actors: []state.ActorSetup{
		{ID: "blade", ControllerType: state.ControllerHuman, Deck: []string{"blade-card"}},
		{ID: "goblin", ControllerType: state.ControllerAI, Deck: []string{"goblin-card"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	battle.Segment.Round = 2
	battle.Random.Cursor = 4
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AppendEvents(&checkpoint, []event.Event{{Type: event.TypeSegmentEntered}}); err != nil {
		t.Fatal(err)
	}
	return checkpoint
}
