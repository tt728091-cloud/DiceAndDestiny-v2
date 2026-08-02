//go:build transcript_tools

package learned

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"diceanddestiny/server/internal/battle/transcript"
)

func TestLearnedAuthorityTranscriptLabelsControllersRematchesAndErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	t.Setenv("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT", "1")
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PROJECT_ENABLED", "1")
	t.Setenv("DICE_AND_DESTINY_RUNTIME_ROOT", root)
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH", path)
	transcript.ResetRegistryForTests()

	TestSessionRoutesHumanAndModelThroughAuthorityAndReusesModel(t)
	TestSessionRejectsWrongSeatStaleCommandAndTimeoutWithoutFallback(t)
	modelErrorSession := testSession(t, 2*time.Second)
	if _, err := modelErrorSession.Reset("model-error", 78, "seat-b", false); err != nil {
		t.Fatal(err)
	}
	modelErrorSession.current.Result.LegalActions = nil
	if _, err := modelErrorSession.AdvanceModel(); err == nil {
		t.Fatal("invalid model decision fixture did not produce an inference error")
	}
	records := readLearnedTranscript(t, path)

	battles := map[string]bool{}
	humanByBattle := map[string]bool{}
	modelByBattle := map[string]bool{}
	completed := map[string]bool{}
	modelMetadata := false
	timeout := false
	wrongSeat := false
	modelError := false
	for _, record := range records {
		if record.Kind == "battle_started" {
			battles[record.BattleID] = true
		}
		if record.Kind == "battle_completed" {
			completed[record.BattleID] = true
		}
		if record.Kind == "command_submitted" && record.Controller == "human" {
			humanByBattle[record.BattleID] = record.Visibility == transcript.VisibilityPrivateSelf && record.ActorID == record.HumanActorID
		}
		if record.Kind == "command_submitted" && record.Controller == "learned_policy" {
			modelByBattle[record.BattleID] = record.Visibility == transcript.VisibilityPrivateOpponent && record.ActorID == record.ModelActorID
			if record.Details["model_id"] != nil && record.Details["candidate_count"] != nil && record.Details["action_index"] != nil {
				modelMetadata = true
			}
		}
		if record.Kind == "model_timeout" {
			timeout = record.Visibility == transcript.VisibilityDebugSystem
		}
		if record.Kind == "command_rejected_wrong_seat" {
			wrongSeat = record.Visibility == transcript.VisibilityDebugSystem
		}
		if record.Kind == "model_error" {
			modelError = record.Visibility == transcript.VisibilityDebugSystem
		}
	}
	for _, battleID := range []string{"session-battle-seat-a", "session-battle-seat-b"} {
		if !battles[battleID] || !humanByBattle[battleID] || !modelByBattle[battleID] || !completed[battleID] {
			t.Fatalf("learned transcript coverage for %s is incomplete: started=%v human=%v model=%v completed=%v", battleID, battles[battleID], humanByBattle[battleID], modelByBattle[battleID], completed[battleID])
		}
	}
	if !modelMetadata || !timeout || !wrongSeat || !modelError {
		t.Fatalf("learned diagnostics incomplete: model_metadata=%v timeout=%v wrong_seat=%v model_error=%v", modelMetadata, timeout, wrongSeat, modelError)
	}
}

func readLearnedTranscript(t *testing.T, path string) []transcript.Record {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var result []transcript.Record
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var record transcript.Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid learned transcript JSONL: %v", err)
		}
		result = append(result, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
