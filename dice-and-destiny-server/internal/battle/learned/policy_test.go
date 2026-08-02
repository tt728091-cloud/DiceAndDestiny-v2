package learned

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/mlsim"
)

func TestAcceptedCheckpointExportLoadsAndSelectsGoldenDecision(t *testing.T) {
	policy, err := LoadAcceptedPolicy(testModelPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if metadata := policy.Metadata(); metadata.ModelID != AcceptedModelID || metadata.SourceParameterSHA256 != AcceptedParameterSHA256 {
		t.Fatalf("unexpected accepted metadata: %#v", metadata)
	}
	environment := testEnvironment(t)
	transition, err := environment.Reset(mlsim.ResetRequest{Seed: 20260802, BattleID: "golden-policy"})
	if err != nil {
		t.Fatal(err)
	}
	// These are the first decisions from an original Phase 2 Python inference
	// replay for this seed. Seat A is the accepted checkpoint; Seat B is the
	// documented heuristic. Checking several changing windows catches schema,
	// masking, candidate-order, and float inference drift.
	golden := []struct {
		actor string
		index int
		kind  command.Type
	}{
		{"seat-a", 6, command.TypePlanningRoll},
		{"seat-a", 8, command.TypePlanningCards},
		{"seat-a", 55, command.TypePlanningReroll},
		{"seat-a", 70, command.TypePlanningAbility},
		{"seat-b", 3, command.TypePlanningRoll},
		{"seat-b", 77, command.TypePlanningAbility},
		{"seat-a", 0, command.TypePass},
		{"seat-b", 3, command.TypeCommitInteraction},
		{"seat-a", 0, command.TypePass},
		{"seat-b", 0, command.TypePass},
		{"seat-a", 1, command.TypePlanningAbility},
		{"seat-b", 1, command.TypePlanningAbility},
	}
	for step, expected := range golden {
		if transition.ActorID != expected.actor {
			t.Fatalf("golden step %d actor=%s, want %s", step, transition.ActorID, expected.actor)
		}
		if expected.actor == "seat-a" {
			selected, _, err := policy.Select(transition)
			if err != nil {
				t.Fatal(err)
			}
			if selected != expected.index {
				t.Fatalf("golden step %d Go policy selected %d, want Python checkpoint index %d", step, selected, expected.index)
			}
		}
		if transition.Result.LegalActions[expected.index].Type != expected.kind {
			t.Fatalf("golden step %d candidate %d=%s, want %s", step, expected.index, transition.Result.LegalActions[expected.index].Type, expected.kind)
		}
		transition, err = environment.Step(expected.index)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAcceptedCheckpointCompatibilityRejectsMetadataDrift(t *testing.T) {
	payload, err := os.ReadFile(testModelPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file policyFile
	if err := json.Unmarshal(payload, &file); err != nil {
		t.Fatal(err)
	}
	file.ObservationSchema = "unreviewed-observation"
	if err := validatePolicyFile(file); err == nil {
		t.Fatal("incompatible observation schema was accepted")
	}
}

func TestAcceptedCheckpointCompatibilityRejectsWeightDrift(t *testing.T) {
	payload, err := os.ReadFile(testModelPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(payload, &file); err != nil {
		t.Fatal(err)
	}
	tensors := file["tensors"].(map[string]any)
	context := tensors["action_net.context.0.weight"].(map[string]any)
	values := context["values"].([]any)
	values[0] = values[0].(float64) + 0.001
	mutated, _ := json.Marshal(file)
	path := filepath.Join(t.TempDir(), "modified-weights.json")
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAcceptedPolicy(path); err == nil {
		t.Fatal("modified actor weights were accepted with unchanged checkpoint metadata")
	}
}

func TestViewerSafeEncodingUsesOnlyActingResultAndCurrentMask(t *testing.T) {
	environment := testEnvironment(t)
	transition, err := environment.Reset(mlsim.ResetRequest{Seed: 81, BattleID: "viewer-safe"})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := EncodeDecision(transition)
	if err != nil {
		t.Fatal(err)
	}
	if len(decision.Observation) != ObservationSize || len(decision.ActionMask) != MaximumActions {
		t.Fatalf("unexpected decision dimensions: %d %d", len(decision.Observation), len(decision.ActionMask))
	}
	valid := 0
	for _, value := range decision.ActionMask {
		if value {
			valid++
		}
	}
	if valid != len(transition.Result.LegalActions) {
		t.Fatalf("mask has %d valid slots for %d candidates", valid, len(transition.Result.LegalActions))
	}
	viewer := transition.ActorID
	opponent := otherSeat(viewer)
	opponentView := transition.Result.Snapshot.Actors[opponent]
	if len(opponentView.Hand) != 0 || len(opponentView.CardInstances) != 0 || len(opponentView.RollHistory) != 0 {
		t.Fatalf("model transition leaked opponent private state: %#v", opponentView)
	}
}

func TestSessionRoutesHumanAndModelThroughAuthorityAndReusesModel(t *testing.T) {
	session := testSession(t, 2*time.Second)
	for battle := 0; battle < 2; battle++ {
		humanSeat := "seat-a"
		if battle == 1 {
			humanSeat = "seat-b"
		}
		if _, err := session.Reset("session-battle-"+humanSeat, uint64(9100+battle), humanSeat, battle > 0); err != nil {
			t.Fatal(err)
		}
		for steps := 0; steps < mlsim.DefaultMaxActions && !session.current.Terminal; steps++ {
			if session.current.ActorID == session.modelSeat {
				if _, err := session.AdvanceModel(); err != nil {
					t.Fatal(err)
				}
				continue
			}
			action := preferredTestAction(session.current.Result.LegalActions)
			alias := aliasValue(commandMap(t, action), map[string]string{session.humanSeat: HumanAlias, session.modelSeat: ModelAlias})
			encoded, _ := json.Marshal(alias)
			if _, err := session.SubmitHuman(string(encoded)); err != nil {
				t.Fatal(err)
			}
		}
		if !session.current.Terminal || session.current.TruncationReason != "" {
			t.Fatalf("battle did not complete cleanly: %#v", session.current.Metrics)
		}
		telemetry, _ := session.Telemetry()
		if telemetry.ModelDecisions == 0 || telemetry.HumanDecisions == 0 || telemetry.FallbackCount != 0 || telemetry.Timeouts != 0 || len(telemetry.Errors) != 0 {
			t.Fatalf("invalid battle telemetry: %#v", telemetry)
		}
		if telemetry.AuthorityRejects != 0 || telemetry.InvalidActions != 0 || telemetry.StaleActions != 0 || telemetry.WrongSeatActions != 0 {
			t.Fatalf("authority safety failure: %#v", telemetry)
		}
	}
	_, lifetime := session.Telemetry()
	if lifetime.ModelLoadCount != 1 || lifetime.BattlesStarted != 2 || lifetime.Rematches != 1 || lifetime.FallbackCount != 0 {
		t.Fatalf("model was not persistent across rematch: %#v", lifetime)
	}
}

func TestSessionAcceptsKeepingNoDiceThenRerollingAll(t *testing.T) {
	session := testSession(t, 2*time.Second)
	if _, err := session.Reset("keep-none-reroll-all", 20260802, "seat-a", false); err != nil {
		t.Fatal(err)
	}

	submitAliased := func(action command.Command) map[string]any {
		t.Helper()
		alias := aliasValue(commandMap(t, action), map[string]string{session.humanSeat: HumanAlias, session.modelSeat: ModelAlias})
		encoded, err := json.Marshal(alias)
		if err != nil {
			t.Fatal(err)
		}
		view, err := session.SubmitHuman(string(encoded))
		if err != nil {
			t.Fatal(err)
		}
		return view
	}

	roll := actionOfType(t, session.current.Result.LegalActions, command.TypePlanningRoll)
	submitAliased(roll)
	var keep command.Command
	for _, action := range session.current.Result.LegalActions {
		if action.Type != command.TypePlanningKeep {
			continue
		}
		var payload command.PlanningKeepPayload
		if json.Unmarshal(action.Payload, &payload) == nil && len(payload.KeptIndices) == 0 {
			keep = action
			break
		}
	}
	if keep.Type == "" {
		t.Fatal("keep-none candidate was not offered")
	}
	var keepPayload map[string]any
	if err := json.Unmarshal(keep.Payload, &keepPayload); err != nil {
		t.Fatal(err)
	}
	if _, ok := keepPayload["kept_indices"].([]any); !ok {
		t.Fatalf("keep-none candidate did not encode kept_indices as an array: %s", keep.Payload)
	}
	submitAliased(keep)

	var rerollAll command.Command
	for _, action := range session.current.Result.LegalActions {
		if action.Type != command.TypePlanningReroll {
			continue
		}
		var payload command.PlanningRerollPayload
		if json.Unmarshal(action.Payload, &payload) == nil && len(payload.RerollIndices) == 5 {
			rerollAll = action
			break
		}
	}
	if rerollAll.Type == "" {
		t.Fatal("reroll-all candidate was not offered after keeping no dice")
	}
	view := submitAliased(rerollAll)
	if view["accepted"] != true {
		t.Fatalf("reroll-all response was not accepted: %#v", view)
	}
	telemetry, _ := session.Telemetry()
	if telemetry.StaleActions != 0 || telemetry.AuthorityRejects != 0 {
		t.Fatalf("keep-none/reroll-all caused authority rejection: %#v", telemetry)
	}
}

func TestSessionRejectsWrongSeatStaleCommandAndTimeoutWithoutFallback(t *testing.T) {
	session := testSession(t, time.Nanosecond)
	view, err := session.Reset("timeout", 77, "seat-b", false)
	if err != nil {
		t.Fatal(err)
	}
	if !object(view["learned_policy"])["model_turn"].(bool) {
		t.Fatal("seat-b human reset did not expose the opening learned-policy turn")
	}
	if _, err := session.SubmitHuman(`{"battle_id":"timeout","actor_id":"blade","type":"pass","payload":{}}`); err == nil {
		t.Fatal("wrong-seat human command was accepted")
	}
	if _, err := session.AdvanceModel(); err == nil {
		t.Fatal("measured timeout was not surfaced")
	}
	telemetry, lifetime := session.Telemetry()
	if telemetry.Timeouts != 1 || telemetry.FallbackCount != 0 || telemetry.ModelDecisions != 0 || lifetime.FallbackCount != 0 {
		t.Fatalf("timeout used or concealed a fallback: battle=%#v lifetime=%#v", telemetry, lifetime)
	}
}

func preferredTestAction(actions []command.Command) command.Command {
	for _, kind := range []command.Type{
		command.TypePlanningRoll,
		command.TypePlanningAbility,
		command.TypePlanningReroll,
		command.TypeRollDice,
		command.TypePlanningPass,
		command.TypePass,
		command.TypeCommitInteraction,
	} {
		for index := len(actions) - 1; index >= 0; index-- {
			if actions[index].Type == kind {
				return actions[index]
			}
		}
	}
	return actions[0]
}

func actionOfType(t *testing.T, actions []command.Command, kind command.Type) command.Command {
	t.Helper()
	for _, action := range actions {
		if action.Type == kind {
			return action
		}
	}
	t.Fatalf("missing %s action in %#v", kind, actions)
	return command.Command{}
}

func commandMap(t *testing.T, action command.Command) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func testSession(t *testing.T, timeout time.Duration) *Session {
	t.Helper()
	serverRoot := testServerRoot(t)
	session, err := NewSession(SessionConfig{
		ContentRoot:      filepath.Join(serverRoot, "content"),
		RunStateRoot:     filepath.Join(serverRoot, "save", "run_players"),
		ModelPath:        testModelPath(t),
		DiagnosticsPath:  filepath.Join(t.TempDir(), "phase3.jsonl"),
		InferenceTimeout: timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func testEnvironment(t *testing.T) *mlsim.Environment {
	t.Helper()
	serverRoot := testServerRoot(t)
	environment, err := mlsim.New(mlsim.Config{
		ContentRoot:  filepath.Join(serverRoot, "content"),
		RunStateRoot: filepath.Join(serverRoot, "save", "run_players"),
		MaxActions:   mlsim.DefaultMaxActions,
		SessionID:    "phase3-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func testModelPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(testServerRoot(t), "..", "dice-and-destiny-client", "models", "learned", "blade-warden-seed-11-final-v1.json")
}

func testServerRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate learned package")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
}
