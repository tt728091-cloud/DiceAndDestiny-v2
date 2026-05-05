package event_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
)

func TestNewSegmentAdvancedIncludesProgressionFields(t *testing.T) {
	got := event.NewSegmentAdvanced(segment.Advance{
		From:          segment.DamageResolution,
		To:            segment.OngoingEffects,
		Round:         2,
		CompletedTurn: true,
	})

	want := event.Event{
		Type:          event.TypeSegmentAdvanced,
		From:          segment.DamageResolution,
		To:            segment.OngoingEffects,
		Round:         2,
		CompletedTurn: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewSegmentAdvanced() = %#v, want %#v", got, want)
	}
}

func TestSegmentAdvancedJSONShape(t *testing.T) {
	got, err := json.Marshal(event.NewSegmentAdvanced(segment.Advance{
		From:          segment.DamageResolution,
		To:            segment.OngoingEffects,
		Round:         2,
		CompletedTurn: true,
	}))
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	want := `{"type":"segment_advanced","from":"damage_resolution","to":"ongoing_effects","round":2,"completed_turn":true}`
	if string(got) != want {
		t.Fatalf("event JSON = %s, want %s", got, want)
	}
}

func TestNewSegmentEnteredIncludesCurrentSegmentFields(t *testing.T) {
	got := event.NewSegmentEntered(segment.State{
		Current: segment.Income,
		Round:   1,
	})

	want := event.Event{
		Type:    event.TypeSegmentEntered,
		Segment: segment.Income,
		Round:   1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewSegmentEntered() = %#v, want %#v", got, want)
	}
}

func TestNewCardsDrawnIncludesActorAndCards(t *testing.T) {
	cards := []string{"card-1", "card-2"}
	got := event.NewCardsDrawn("player", cards, false)
	cards[0] = "mutated"

	want := event.Event{
		Type:    event.TypeCardsDrawn,
		ActorID: "player",
		Cards:   []string{"card-1", "card-2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewCardsDrawn() = %#v, want %#v", got, want)
	}
}

func TestCardsDrawnJSONShape(t *testing.T) {
	got, err := json.Marshal(event.NewCardsDrawn("player", []string{"card-1"}, true))
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	want := `{"type":"cards_drawn","actor_id":"player","cards":["card-1"],"deck_empty":true}`
	if string(got) != want {
		t.Fatalf("event JSON = %s, want %s", got, want)
	}
}

func TestEventProductionCodeDoesNotImportPresentationPackages(t *testing.T) {
	assertProductionImportsAllowed(t, []string{
		"dice-and-destiny-client",
		"gdextension",
		"godot",
		"/ui",
		"ui/",
	})
}

func assertProductionImportsAllowed(t *testing.T, forbiddenImportFragments []string) {
	t.Helper()

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
					t.Fatalf("production file %s imports forbidden package %q", file, importPath)
				}
			}
		}
	}
}

func TestEventRoundTripsThroughJSON(t *testing.T) {
	want := event.Event{
		Type:          event.TypeSegmentAdvanced,
		From:          segment.Offensive,
		To:            segment.Defensive,
		Round:         1,
		CompletedTurn: false,
	}

	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}

	var got event.Event
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-tripped event = %#v, want %#v", got, want)
	}
}
