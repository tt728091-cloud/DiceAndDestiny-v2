package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestDiskCreateLoadSaveAndDuplicateProtection(t *testing.T) {
	root := t.TempDir()
	repo := NewDisk(root)
	checkpoint := diskTestCheckpoint(t, "disk-battle")
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if err := repo.Create(checkpoint); !errors.Is(err, ErrBattleExists) {
		t.Fatalf("duplicate Create() error = %v, want ErrBattleExists", err)
	}

	loaded, err := repo.Load("disk-battle")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded, checkpoint) {
		t.Fatalf("loaded checkpoint changed\n got: %#v\nwant: %#v", loaded, checkpoint)
	}
	if loaded.Events[0].ID != eventID("disk-battle", 1) ||
		loaded.Events[0].Sequence != 1 {
		t.Fatalf("stable event metadata = %#v", loaded.Events[0])
	}
	tampered := loaded
	tampered.Events = append([]event.Event(nil), loaded.Events...)
	tampered.Events[0].ActorID = "tampered"
	if err := repo.Save(tampered); !errors.Is(err, ErrCorruptCheckpoint) {
		t.Fatalf("tampered history Save() error = %v, want ErrCorruptCheckpoint", err)
	}

	actor := loaded.Battle.Actors["player"]
	actor.Tokens = []state.TokenState{{ID: "saved", Value: 7}}
	loaded.Battle.Actors["player"] = actor
	if err := repo.Save(loaded); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	reloaded, err := NewDisk(root).Load("disk-battle")
	if err != nil {
		t.Fatalf("restarted Load() returned error: %v", err)
	}
	if reloaded.Battle.Actors["player"].Tokens[0].Value != 7 {
		t.Fatalf("saved token = %#v", reloaded.Battle.Actors["player"].Tokens)
	}
	if _, err := repo.Load("../escape"); !errors.Is(err, ErrInvalidBattleID) {
		t.Fatalf("unsafe battle ID error = %v, want ErrInvalidBattleID", err)
	}
}

func TestDiskRejectsMissingCorruptAndUnsupportedCheckpoints(t *testing.T) {
	root := t.TempDir()
	repo := NewDisk(root)
	if _, err := repo.Load("missing"); !errors.Is(err, ErrBattleNotFound) {
		t.Fatalf("missing Load() error = %v, want ErrBattleNotFound", err)
	}

	if err := os.WriteFile(filepath.Join(root, "corrupt.json"), []byte(`{"schema_version":1`), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupt) returned error: %v", err)
	}
	if _, err := repo.Load("corrupt"); !errors.Is(err, ErrCorruptCheckpoint) {
		t.Fatalf("corrupt Load() error = %v, want ErrCorruptCheckpoint", err)
	}

	unsupported := diskTestCheckpoint(t, "unsupported")
	payload, err := json.Marshal(unsupported)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}
	raw["schema_version"] = 99
	payload, err = json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal(unsupported) returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "unsupported.json"), payload, 0o600); err != nil {
		t.Fatalf("WriteFile(unsupported) returned error: %v", err)
	}
	if _, err := repo.Load("unsupported"); !errors.Is(err, ErrUnsupportedCheckpoint) {
		t.Fatalf("unsupported Load() error = %v, want ErrUnsupportedCheckpoint", err)
	}

	unsupportedContent := diskTestCheckpoint(t, "unsupported-content")
	unsupportedContent.ContentPin.SchemaVersion = 99
	payload, err = json.Marshal(unsupportedContent)
	if err != nil {
		t.Fatalf("Marshal(unsupported content) returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "unsupported-content.json"), payload, 0o600); err != nil {
		t.Fatalf("WriteFile(unsupported content) returned error: %v", err)
	}
	if _, err := repo.Load("unsupported-content"); !errors.Is(err, ErrUnsupportedCheckpoint) {
		t.Fatalf("unsupported content Load() error = %v, want ErrUnsupportedCheckpoint", err)
	}
}

func TestDiskAtomicSaveFailurePreservesPreviousCheckpoint(t *testing.T) {
	root := t.TempDir()
	repo := NewDisk(root)
	checkpoint := diskTestCheckpoint(t, "atomic")
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	updated := checkpoint
	updated.Battle.Status = state.BattleVictory
	repo.rename = func(string, string) error {
		return errors.New("injected rename failure")
	}
	if err := repo.Save(updated); err == nil {
		t.Fatal("Save() succeeded despite injected rename failure")
	}
	reloaded, err := NewDisk(root).Load("atomic")
	if err != nil {
		t.Fatalf("Load() after failed save returned error: %v", err)
	}
	if !reflect.DeepEqual(reloaded, checkpoint) {
		t.Fatal("failed atomic save replaced the valid checkpoint")
	}
	matches, err := filepath.Glob(filepath.Join(root, ".atomic.json.tmp-*"))
	if err != nil {
		t.Fatalf("Glob() returned error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary checkpoint files remain: %#v", matches)
	}
}

func TestCloneCheckpointRekeysIdentityAndPreservesAuthorityState(t *testing.T) {
	source := diskTestCheckpoint(t, "snapshot-source")
	source.Battle.Random.Cursor = 17
	cloned, err := CloneCheckpoint(source, "snapshot-copy")
	if err != nil {
		t.Fatalf("CloneCheckpoint() returned error: %v", err)
	}
	if cloned.BattleID != "snapshot-copy" || cloned.Battle.ID != "snapshot-copy" {
		t.Fatalf("cloned battle identity = %q / %q", cloned.BattleID, cloned.Battle.ID)
	}
	if cloned.Events[0].BattleID != "snapshot-copy" ||
		cloned.Events[0].ID != eventID("snapshot-copy", 1) {
		t.Fatalf("cloned event identity = %#v", cloned.Events[0])
	}
	if cloned.Battle.Random != source.Battle.Random ||
		cloned.NextEventSequence != source.NextEventSequence ||
		cloned.ContentPin != source.ContentPin {
		t.Fatal("clone changed random state, sequencing, or content pin")
	}
	cloned.Battle.Actors["player"].Cards.Deck[0] = "mutated"
	cloned.Events[0].Dice[0].Face = 1
	if source.Battle.Actors["player"].Cards.Deck[0] == "mutated" || source.Events[0].Dice[0].Face == 1 {
		t.Fatal("clone aliases source checkpoint state")
	}
}

func diskTestCheckpoint(t *testing.T, battleID string) Checkpoint {
	t.Helper()
	battle, err := state.NewBattleFromSetup(battleID, state.BattleSetup{
		Actors: []state.ActorSetup{{
			ID:             "player",
			ControllerType: state.ControllerHuman,
			Deck:           []string{"health"},
		}},
		DiceDefinitions: []state.DiceDefinition{{
			ID:        "symbol-d6",
			Name:      "Symbol D6",
			DieType:   "d6",
			SideCount: 6,
			Faces: []state.DiceFace{{
				Face: 6, Value: 6, Symbols: []string{"crown"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	checkpoint, err := NewCheckpoint(battle)
	if err != nil {
		t.Fatalf("NewCheckpoint() returned error: %v", err)
	}
	_, err = AppendEvents(&checkpoint, []event.Event{
		event.NewDiceRolled(
			"player",
			segment.Offensive,
			"roll-1",
			state.RollPoolOffensive,
			state.RollSourceSegment,
			"offensive",
			[]state.RolledDie{{Index: 0, DieID: "symbol-d6", Face: 6, Value: 6, Symbols: []string{"crown"}}},
			[]int{0},
			1,
			3,
			nil,
			map[string]int{"crown": 1},
		),
	})
	if err != nil {
		t.Fatalf("AppendEvents() returned error: %v", err)
	}
	return checkpoint
}
