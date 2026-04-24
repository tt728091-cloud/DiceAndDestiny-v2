package battle

import (
	"encoding/json"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
)

func TestHandleCommandRejectsInvalidJSON(t *testing.T) {
	got := decodeAuthorityResult(t, HandleCommand(`{"battle_id"`))

	want := engine.Result{
		Accepted: false,
		Error:    "invalid command JSON",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestHandleCommandRejectsInvalidEnvelopeBeforeEngine(t *testing.T) {
	handler := &recordingHandler{
		result: engine.Result{Accepted: true},
	}

	got := decodeAuthorityResult(t, handleCommand(`{
		"battle_id": "battle-1",
		"actor_id": "system",
		"type": "advance_segment",
		"payload": []
	}`, handler))

	want := engine.Result{
		Accepted: false,
		Error:    "invalid command envelope",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("handleCommand() mismatch\n got: %#v\nwant: %#v", got, want)
	}

	if len(handler.commands) != 0 {
		t.Fatalf("engine received %d commands, want 0", len(handler.commands))
	}
}

func TestHandleCommandPassesStructurallyValidEnvelopeToEngine(t *testing.T) {
	handler := &recordingHandler{
		result: engine.Result{
			Accepted: true,
			Snapshot: &engine.Snapshot{
				BattleID: "battle-1",
				Segment:  "income",
				Round:    1,
			},
		},
	}

	got := decodeAuthorityResult(t, handleCommand(`{
		"battle_id": "battle-1",
		"actor_id": "system",
		"type": "mystery_command",
		"payload": {}
	}`, handler))

	if !got.Accepted {
		t.Fatalf("handleCommand() rejected structurally valid command: %#v", got)
	}

	if len(handler.commands) != 1 {
		t.Fatalf("engine received %d commands, want 1", len(handler.commands))
	}

	want := command.Command{
		BattleID: "battle-1",
		ActorID:  "system",
		Type:     command.Type("mystery_command"),
		Payload:  json.RawMessage(`{}`),
	}
	if !reflect.DeepEqual(handler.commands[0], want) {
		t.Fatalf("engine command = %#v, want %#v", handler.commands[0], want)
	}
}

func TestHandleCommandRejectsUnsupportedCommandTypeFromEngine(t *testing.T) {
	got := decodeAuthorityResult(t, HandleCommand(`{
		"battle_id": "battle-1",
		"actor_id": "system",
		"type": "mystery_command",
		"payload": {}
	}`))

	want := engine.Result{
		Accepted: false,
		Error:    "unsupported command type",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestHandleCommandAdvanceSegmentReturnsEventAndSnapshot(t *testing.T) {
	got := decodeAuthorityResult(t, HandleCommand(`{
		"battle_id": "battle-1",
		"actor_id": "system",
		"type": "advance_segment",
		"payload": {}
	}`))

	want := engine.Result{
		Accepted: true,
		Events: []engine.Event{
			{
				Type:  "segment_advanced",
				From:  "ongoing_effects",
				To:    "income",
				Round: 1,
			},
		},
		Snapshot: &engine.Snapshot{
			BattleID: "battle-1",
			Segment:  "income",
			Round:    1,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func decodeAuthorityResult(t *testing.T, gotJSON string) engine.Result {
	t.Helper()

	var got engine.Result
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatalf("HandleCommand returned invalid JSON: %v\n%s", err, gotJSON)
	}
	return got
}

type recordingHandler struct {
	commands []command.Command
	result   engine.Result
}

func (h *recordingHandler) HandleCommand(cmd command.Command) engine.Result {
	h.commands = append(h.commands, cmd)
	return h.result
}
