package battle

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
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
			Snapshot: &snapshot.Battle{
				BattleID: "battle-1",
				Segment:  segment.Income,
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

func TestHandleCommandRejectsCommandForUnknownBattle(t *testing.T) {
	got := decodeAuthorityResult(t, HandleCommand(`{
		"battle_id": "missing-battle",
		"actor_id": "system",
		"type": "mystery_command",
		"payload": {}
	}`))

	want := engine.Result{
		Accepted: false,
		Error:    "battle not found",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAuthorityStartsPersistsAndReloadsParticipantBattle(t *testing.T) {
	repo := repository.NewInMemory()
	assembler := &recordingAssembler{}
	authority := NewAuthority(engine.NewEngine(), repo, assembler)

	started := decodeAuthorityResult(t, authority.HandleCommandJSON(`{
		"battle_id": "battle-lifecycle",
		"actor_id": "player",
		"type": "start_battle",
		"payload": {
			"player": {
				"instance_id": "player",
				"definition_id": "run-player"
			},
			"enemies": [
				{
					"instance_id": "goblin-1",
					"definition_id": "goblin"
				},
				{
					"instance_id": "goblin-2",
					"definition_id": "goblin"
				}
			]
		}
	}`))

	if !started.Accepted || started.Status != engine.ProgressWaitingForInput {
		t.Fatalf("start_battle result = %#v, want accepted wait", started)
	}
	if started.Snapshot == nil || len(started.Snapshot.Actors) != 3 {
		t.Fatalf("start snapshot = %#v, want three actors", started.Snapshot)
	}
	if started.Snapshot.Actors["player"].DefinitionID != "run-player" ||
		started.Snapshot.Actors["player"].Controller != state.ControllerHuman {
		t.Fatalf("player snapshot = %#v, want run-player human", started.Snapshot.Actors["player"])
	}
	if started.Snapshot.Actors["goblin-1"].DefinitionID != "goblin" ||
		started.Snapshot.Actors["goblin-1"].Controller != state.ControllerAI {
		t.Fatalf("enemy snapshot = %#v, want goblin AI", started.Snapshot.Actors["goblin-1"])
	}
	if len(assembler.participants) != 3 {
		t.Fatalf("assembler participants = %#v, want three", assembler.participants)
	}

	firstCheckpoint, err := repo.Load("battle-lifecycle")
	if err != nil {
		t.Fatalf("Load() after start returned error: %v", err)
	}
	initialEventCount := len(firstCheckpoint.Events)
	if initialEventCount == 0 {
		t.Fatal("start did not persist authoritative events")
	}
	pending := started.PendingInput["player"]
	if pending.ID == "" || pending.SourceID == "" {
		t.Fatalf("pending input = %#v, want offensive roll request", pending)
	}

	rolled := decodeAuthorityResult(t, authority.HandleCommandJSON(fmt.Sprintf(`{
		"battle_id": "battle-lifecycle",
		"actor_id": "player",
		"type": "roll_dice",
		"payload": {
			"request_id": %q,
			"pending_input_id": %q
		}
	}`, pending.SourceID, pending.ID)))
	if !rolled.Accepted || rolled.Status != engine.ProgressWaitingForInput {
		t.Fatalf("roll result = %#v, want accepted wait", rolled)
	}
	if len(rolled.Events) != 1 || rolled.Events[0].Type != event.TypeDiceRolled {
		t.Fatalf("roll events = %#v, want dice_rolled", rolled.Events)
	}

	secondCheckpoint, err := repo.Load("battle-lifecycle")
	if err != nil {
		t.Fatalf("Load() after roll returned error: %v", err)
	}
	if len(secondCheckpoint.Events) != initialEventCount+1 {
		t.Fatalf("stored event count = %d, want %d", len(secondCheckpoint.Events), initialEventCount+1)
	}
	currentRoll := secondCheckpoint.Battle.Actors["player"].Dice.CurrentRoll
	if currentRoll == nil || currentRoll.RollsUsed != 1 {
		t.Fatalf("stored player roll = %#v, want one persisted roll", currentRoll)
	}
}

func TestAuthorityRejectsDuplicateBattleIDWithoutReplacingCheckpoint(t *testing.T) {
	repo := repository.NewInMemory()
	authority := NewAuthority(engine.NewEngine(), repo, &recordingAssembler{})
	startJSON := `{
		"battle_id": "battle-duplicate",
		"actor_id": "player",
		"type": "start_battle",
		"payload": {
			"player": {"instance_id": "player", "definition_id": "run-player"},
			"enemies": [{"instance_id": "goblin-1", "definition_id": "goblin"}]
		}
	}`

	first := decodeAuthorityResult(t, authority.HandleCommandJSON(startJSON))
	if !first.Accepted {
		t.Fatalf("first start rejected: %#v", first)
	}
	before, err := repo.Load("battle-duplicate")
	if err != nil {
		t.Fatalf("Load() before duplicate returned error: %v", err)
	}

	second := decodeAuthorityResult(t, authority.HandleCommandJSON(startJSON))
	if second.Accepted || second.Error != "battle already exists" {
		t.Fatalf("duplicate start = %#v, want battle already exists", second)
	}
	after, err := repo.Load("battle-duplicate")
	if err != nil {
		t.Fatalf("Load() after duplicate returned error: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("duplicate start replaced the stored checkpoint")
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

type recordingAssembler struct {
	participants []Participant
}

func (assembler *recordingAssembler) AssembleParticipants(
	participants []Participant,
) (state.BattleSetup, error) {
	assembler.participants = append([]Participant(nil), participants...)
	actors := make([]state.ActorSetup, len(participants))
	for i, participant := range participants {
		actors[i] = state.ActorSetup{
			ID:          participant.InstanceID,
			Deck:        []string{"strike", "guard"},
			DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "Standard D6", Count: 1}},
		}
	}
	return state.BattleSetup{
		Actors: actors,
		DiceDefinitions: []state.DiceDefinition{
			{
				ID:        "Standard D6",
				Name:      "Standard D6",
				DieType:   "d6",
				SideCount: 6,
				Faces: []state.DiceFace{
					{Face: 1, Value: 1},
					{Face: 2, Value: 2},
					{Face: 3, Value: 3},
					{Face: 4, Value: 4},
					{Face: 5, Value: 5},
					{Face: 6, Value: 6},
				},
			},
		},
	}, nil
}
