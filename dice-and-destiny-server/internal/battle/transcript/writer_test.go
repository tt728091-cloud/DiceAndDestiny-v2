package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorderRequiresWorkspaceLocalCanonicalPath(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "authority-transcript.jsonl")
	configureTestEnvironment(t, root, outside)
	ResetRegistryForTests()
	if recorder := FromEnvironment(); recorder != nil {
		t.Fatalf("recorder accepted a path outside the workspace runtime: %s", recorder.Path())
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("rejected external path was created: %v", err)
	}
}

func TestRecorderAppendsMonotonicVersionedJSONL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	configureTestEnvironment(t, root, path)
	ResetRegistryForTests()
	recorder := FromEnvironment()
	if recorder == nil {
		t.Fatal("valid workspace recorder was not constructed")
	}
	for _, kind := range []string{"first", "second"} {
		recorder.Diagnostic(Diagnostic{BattleID: "append", Kind: kind, Summary: kind, Controller: "authority"})
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sequence := uint64(0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid JSONL: %v", err)
		}
		sequence++
		if record.Sequence != sequence || record.SchemaVersion != SchemaVersion || record.RecorderVersion != RecorderVersion {
			t.Fatalf("invalid versioned sequence: %#v", record)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if sequence != 2 {
		t.Fatalf("got %d records, want 2", sequence)
	}
}

func TestRecorderRejectsIncompatibleExistingSchema(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":99,"sequence":1}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configureTestEnvironment(t, root, path)
	ResetRegistryForTests()
	if recorder := FromEnvironment(); recorder != nil {
		t.Fatal("recorder accepted an incompatible existing schema")
	}
}

func configureTestEnvironment(t *testing.T, root, path string) {
	t.Helper()
	t.Setenv(enableEnv, "1")
	t.Setenv(projectGateEnv, "1")
	t.Setenv(runtimeRootEnv, root)
	t.Setenv(pathEnv, path)
}
