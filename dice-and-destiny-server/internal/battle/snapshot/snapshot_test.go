package snapshot_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestFromBattleIncludesBattleIDCurrentSegmentAndRound(t *testing.T) {
	battle := state.Battle{
		ID: "battle-1",
		Segment: segment.State{
			Current: segment.Income,
			Round:   3,
		},
	}

	got := snapshot.FromBattle(battle)
	want := snapshot.Battle{
		BattleID: "battle-1",
		Segment:  segment.Income,
		Round:    3,
	}
	if got != want {
		t.Fatalf("FromBattle() = %#v, want %#v", got, want)
	}
}

func TestBattleSnapshotJSONShape(t *testing.T) {
	got, err := json.Marshal(snapshot.Battle{
		BattleID: "battle-1",
		Segment:  segment.Income,
		Round:    1,
	})
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	want := `{"battle_id":"battle-1","segment":"income","round":1}`
	if string(got) != want {
		t.Fatalf("snapshot JSON = %s, want %s", got, want)
	}
}

func TestBattleSnapshotRoundTripsThroughJSON(t *testing.T) {
	want := snapshot.Battle{
		BattleID: "battle-1",
		Segment:  segment.Defensive,
		Round:    2,
	}

	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var got snapshot.Battle
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-tripped snapshot = %#v, want %#v", got, want)
	}
}

func TestSnapshotProductionCodeDoesNotImportPresentationPackages(t *testing.T) {
	forbiddenImportFragments := []string{
		"dice-and-destiny-client",
		"gdextension",
		"godot",
		"/ui",
		"ui/",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("finding Go files: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing imports in %s: %v", file, err)
		}

		for _, imp := range parsed.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquoting import path %s in %s: %v", imp.Path.Value, file, err)
			}

			lowerImportPath := strings.ToLower(importPath)
			for _, forbidden := range forbiddenImportFragments {
				if strings.Contains(lowerImportPath, forbidden) {
					t.Fatalf("snapshot production file %s imports forbidden package %q", file, importPath)
				}
			}
		}
	}
}
