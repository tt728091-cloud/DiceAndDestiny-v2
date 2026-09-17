package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestAutomaticSegmentsKeepChronologicalTranscriptContext(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	configureTestEnvironment(t, root, path)
	ResetRegistryForTests()
	r := FromEnvironment()
	if r == nil {
		t.Fatal("missing recorder")
	}
	before := state.Battle{ID: "segment-context", Segment: segment.State{Round: 1, Current: segment.DamageResolution}}
	before.Flow.Stage = "damage_reaction"
	after := before.Clone()
	after.Segment = segment.State{Round: 2, Current: segment.Offensive}
	after.Flow.Stage = "planning"
	cmd := command.Command{BattleID: before.ID, ActorID: "seat-a", Type: command.TypePass}
	ctx := BattleContext{HumanActorID: "seat-a"}
	events := []event.Event{
		{Type: event.TypeCardsPermanentlyRemoved, Cards: []string{"old-damage"}},
		{Type: event.TypeSegmentAdvanced, From: segment.DamageResolution, To: segment.OngoingEffects, Round: 2, CompletedTurn: true},
		{Type: event.TypeSegmentEntered, Segment: segment.OngoingEffects, Round: 2},
		{Type: event.TypeInteractionWindowOpened, Segment: segment.OngoingEffects, Round: 2, Data: map[string]any{"batch_id": "poison-batch"}},
		{Type: event.TypeDamageCardsRevealed, Segment: segment.OngoingEffects, Round: 2},
		{Type: event.TypeCardsPermanentlyRemoved, Cards: []string{"poison-1"}},
		{Type: event.TypeCardsPermanentlyRemoved, Cards: []string{"poison-2"}},
		{Type: event.TypeDamageCommitted, Segment: segment.OngoingEffects, Round: 2},
		{Type: event.TypeSegmentAdvanced, From: segment.OngoingEffects, To: segment.Income, Round: 2},
		{Type: event.TypeSegmentEntered, Segment: segment.Income, Round: 2},
		{Type: event.TypeCardsDrawn, ActorID: "seat-a", Count: 1},
		{Type: event.TypeEnergyPointsGained, ActorID: "seat-a", EnergyPoints: 3},
		{Type: event.TypeSegmentAdvanced, From: segment.Income, To: segment.Offensive, Round: 2},
		{Type: event.TypeSegmentEntered, Segment: segment.Offensive, Round: 2},
	}
	r.Begin(cmd, CommandContext{}, ctx, &before)
	r.Accepted(Transition{Command: cmd, Battle: ctx, Before: &before, After: &after, Events: events})
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var headers []string
	var removed []string
	var accepted, opened, incomeDraw bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		header := record.Segment
		if len(headers) == 0 || headers[len(headers)-1] != header {
			headers = append(headers, header)
		}
		if record.Segment == "ongoing_effects" || record.Segment == "income" {
			if record.Round != 2 || record.Stage == "planning" || record.Stage == "damage_reaction" {
				t.Fatalf("inherited unrelated checkpoint: %+v", record)
			}
		}
		switch record.Kind {
		case "command_accepted":
			accepted = record.Round == 1 && record.Segment == "damage_resolution" && record.Stage == "damage_reaction"
		case "cards_permanently_removed":
			removed = append(removed, record.Segment)
		case "status_trigger_batch_opened":
			opened = record.SourceID == "poison-batch" && !strings.Contains(record.Summary, "Unknown")
		case "cards_drawn":
			incomeDraw = record.Segment == "income"
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headers, []string{"damage_resolution", "ongoing_effects", "income", "offensive"}) {
		t.Fatalf("headers jump: %v", headers)
	}
	if !reflect.DeepEqual(removed, []string{"damage_resolution", "ongoing_effects", "ongoing_effects"}) {
		t.Fatalf("card loss misattributed: %v", removed)
	}
	if !accepted || !opened || !incomeDraw {
		t.Fatalf("accepted=%v opened=%v incomeDraw=%v", accepted, opened, incomeDraw)
	}
}

func TestEventContextHonorsStampedSegmentsAndClearsOldWindows(t *testing.T) {
	old := Record{Round: 3, Segment: "offensive", Stage: "offensive_reaction", WindowID: "old", PendingInputID: "old-input"}
	next := advanceEventContext(old, event.Event{Type: event.TypeSegmentAdvanced, From: segment.Offensive, To: segment.Defensive, Round: 3})
	if next.Segment != "defensive" || next.Stage != "" || next.WindowID != "" || next.PendingInputID != "" {
		t.Fatalf("old window leaked: %+v", next)
	}
	stamped := advanceEventContext(next, event.Event{Segment: segment.DamageResolution, Round: 3, WindowID: "damage", PendingInputID: "damage-input"})
	if stamped.Segment != "damage_resolution" || stamped.WindowID != "damage" || stamped.PendingInputID != "damage-input" {
		t.Fatalf("explicit event context lost: %+v", stamped)
	}
}
