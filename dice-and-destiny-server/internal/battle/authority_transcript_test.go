//go:build transcript_tools

package battle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/transcript"
)

func TestAuthorityTranscriptDisabledIsBehaviorallyIdenticalAndWritesNothing(t *testing.T) {
	disabledRoot := t.TempDir()
	disabledPath := filepath.Join(disabledRoot, "user", "debug", "authority-transcript.jsonl")
	t.Setenv("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT", "0")
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PROJECT_ENABLED", "1")
	t.Setenv("DICE_AND_DESTINY_RUNTIME_ROOT", disabledRoot)
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH", disabledPath)
	transcript.ResetRegistryForTests()
	disabled := newTwoSeatTestAuthority(t)

	enabledRoot, enabledPath := enableTranscriptForTest(t)
	_ = enabledRoot
	enabled := newTwoSeatTestAuthority(t)

	disabledResult := startTwoSeatBattle(t, disabled, "transcript-equivalence", 20260802)
	enabledResult := startTwoSeatBattle(t, enabled, "transcript-equivalence", 20260802)
	assertResultsEquivalent(t, disabledResult, enabledResult)
	for step := 0; step < 30 && disabledResult.Status != engine.ProgressBattleComplete; step++ {
		checkpoint, err := disabled.repo.Load("transcript-equivalence")
		if err != nil {
			t.Fatal(err)
		}
		actorID := ""
		for _, candidate := range []string{"seat-a", "seat-b"} {
			if _, pending := checkpoint.Battle.Flow.PendingInput[candidate]; pending {
				actorID = candidate
				break
			}
		}
		if actorID == "" {
			t.Fatalf("no pending actor at equivalence step %d", step)
		}
		disabledResult = openTwoSeatBattle(t, disabled, "transcript-equivalence", actorID)
		enabledResult = openTwoSeatBattle(t, enabled, "transcript-equivalence", actorID)
		assertResultsEquivalent(t, disabledResult, enabledResult)
		action := chooseMirrorAction(disabledResult.LegalActions)
		disabledResult = sendTwoSeatCommand(t, disabled, action)
		enabledResult = sendTwoSeatCommand(t, enabled, action)
		assertResultsEquivalent(t, disabledResult, enabledResult)
	}
	if _, err := os.Stat(disabledPath); !os.IsNotExist(err) {
		t.Fatalf("disabled transcript path exists or returned unexpected error: %v", err)
	}
	if records := readTranscriptRecords(t, enabledPath); len(records) == 0 {
		t.Fatal("enabled equivalence authority wrote no transcript records")
	}
}

func TestAuthorityTranscriptProvesD100PoisonAntidoteDamageAndSecretDefense(t *testing.T) {
	_, path := enableTranscriptForTest(t)
	TestAuthorityRunsBladeWardenVsVenomGoblinFullBattle(t)
	records := readTranscriptRecords(t, path)
	assertTranscriptEnvelope(t, records)

	poisonPrivate := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "die_rolled" && record.StatusID == "poison" && reflect.DeepEqual(intValues(record.Details["faces"]), []int{2, 6})
	})
	poisonReveal := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "die_revealed" && record.StatusID == "poison" && reflect.DeepEqual(intValues(record.Details["faces"]), []int{2, 6})
	})
	if poisonPrivate.Visibility != transcript.VisibilityPrivateSelf || poisonReveal.Visibility != transcript.VisibilityPublic || poisonReveal.RevealOfSequence != poisonPrivate.Sequence {
		t.Fatalf("Poison visibility transition is incomplete: private=%#v reveal=%#v", poisonPrivate, poisonReveal)
	}
	antidote := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "card_played" && record.CardDefinitionID == "antidote" && record.StatusID == "poison"
	})
	removed := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "status_removed" && record.StatusID == "poison"
	})
	if antidote.Visibility != transcript.VisibilityPublic || intValueAny(removed.Details["stacks_before"]) != 2 || intValueAny(removed.Details["stacks_after"]) != 0 || antidote.Sequence >= removed.Sequence {
		t.Fatalf("Antidote/full removal narrative is incomplete: antidote=%#v removed=%#v", antidote, removed)
	}
	redundant := findTranscriptRecord(records, func(record transcript.Record) bool {
		roll := mapAny(record.Details["roll"])
		die := mapAny(roll["die"])
		return record.Kind == "status_outcome_evaluated" && record.StatusID == "poison" && intValueAny(die["face"]) == 6 && record.Details["redundant"] == true
	})
	if redundant.Sequence <= removed.Sequence {
		t.Fatalf("redundant face-6 outcome was not evaluated after Antidote: %#v", redundant)
	}
	poisonDamage := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "damage_calculated" && record.SourceID == "poison" && record.Details["committed"] == true
	})
	if intValueAny(poisonDamage.Details["base_amount"]) != 1 || intValueAny(poisonDamage.Details["final_amount"]) != 1 || !recordDetailsHas(poisonDamage, "prevention") || !recordDetailsHas(poisonDamage, "reaction_prevention") {
		t.Fatalf("Poison damage calculation is incomplete: %#v", poisonDamage)
	}
	if findTranscriptRecord(records, func(record transcript.Record) bool { return record.Kind == "health_changed" }).Kind == "" ||
		findTranscriptRecord(records, func(record transcript.Record) bool { return record.Kind == "cards_permanently_removed" }).Kind == "" {
		t.Fatal("damage transcript omitted its public health or card-loss consequences")
	}

	d100 := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "d100_rolled" && record.Controller == "d100"
	})
	privateDefense := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "defense_selected_private" && record.ActorID == "goblin" && record.Visibility == transcript.VisibilityPrivateOpponent
	})
	privateDefenseDie := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "die_rolled" && record.ActorID == "goblin" && record.Segment == "defensive" && record.Visibility == transcript.VisibilityPrivateOpponent
	})
	publicDefense := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "defense_selected" && record.ActorID == "goblin" && record.Visibility == transcript.VisibilityPublic
	})
	if d100.Sequence == 0 || privateDefense.Sequence >= publicDefense.Sequence || privateDefenseDie.Sequence >= publicDefense.Sequence || publicDefense.RevealOfSequence == 0 {
		t.Fatalf("D100 secret defense transition is incomplete: d100=%#v choice=%#v die=%#v reveal=%#v", d100, privateDefense, privateDefenseDie, publicDefense)
	}

	tip := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "die_modified" && record.CardDefinitionID == "tip_it"
	})
	tipCard := findTranscriptRecord(records, func(record transcript.Record) bool {
		return record.Kind == "card_played" && record.CardDefinitionID == "tip_it"
	})
	if tipCard.Sequence == 0 || tipCard.Sequence >= tip.Sequence || intValueAny(tip.Details["face_before"]) != 6 || intValueAny(tip.Details["face_after"]) != 5 || !recordDetailsHas(tip, "ability_before") || !recordDetailsHas(tip, "ability_after") {
		t.Fatalf("Tip It record is incomplete: %#v", tip)
	}
}

func TestAuthorityTranscriptTwoExternalDefensesStayPrivateUntilJointReveal(t *testing.T) {
	_, path := enableTranscriptForTest(t)
	TestTwoExternalBasicDefenseRollsResolveSequentially(t)
	records := readTranscriptRecords(t, path)
	for _, actorID := range []string{"seat-a", "seat-b"} {
		choice := findTranscriptRecord(records, func(record transcript.Record) bool {
			return record.Kind == "defense_selected_private" && record.ActorID == actorID
		})
		die := findTranscriptRecord(records, func(record transcript.Record) bool {
			return record.Kind == "die_rolled" && record.ActorID == actorID && record.Segment == "defensive"
		})
		reveal := findTranscriptRecord(records, func(record transcript.Record) bool {
			return record.Kind == "defense_selected" && record.ActorID == actorID
		})
		wantVisibility := transcript.VisibilityPrivateSelf
		if actorID == "seat-b" {
			wantVisibility = transcript.VisibilityPrivateOpponent
		}
		if choice.Visibility != wantVisibility || die.Visibility != wantVisibility || reveal.Visibility != transcript.VisibilityPublic || choice.Sequence >= reveal.Sequence || die.Sequence >= reveal.Sequence {
			t.Fatalf("%s secret defense ordering mismatch: choice=%#v die=%#v reveal=%#v", actorID, choice, die, reveal)
		}
	}
}

func TestAuthorityTranscriptAppendsDistinctBattlesAndDiagnosticRejections(t *testing.T) {
	_, path := enableTranscriptForTest(t)
	authority := newTwoSeatTestAuthority(t)
	first := startTwoSeatBattle(t, authority, "transcript-rematch-a", 41)
	stale := chooseMirrorAction(first.LegalActions)
	first = sendTwoSeatCommand(t, authority, stale)
	if rejected := sendTwoSeatCommandAllowReject(t, authority, stale); rejected.Accepted {
		t.Fatal("stale command unexpectedly succeeded")
	}
	startTwoSeatBattle(t, authority, "transcript-rematch-b", 42)
	TestAuthorityStartsTwoExternalBladeWardenSeatsWithPrivatePlanning(t)
	records := readTranscriptRecords(t, path)
	battles := map[string]bool{}
	for _, record := range records {
		battles[record.BattleID] = true
	}
	if !battles["transcript-rematch-a"] || !battles["transcript-rematch-b"] {
		t.Fatalf("append-only transcript did not retain both battles: %#v", battles)
	}
	if findTranscriptRecord(records, func(record transcript.Record) bool { return strings.HasPrefix(record.Kind, "command_rejected") }).Kind == "" {
		t.Fatal("stale authority rejection was not diagnosed")
	}
	if findTranscriptRecord(records, func(record transcript.Record) bool { return record.Kind == "command_rejected_fabricated" }).Kind == "" {
		t.Fatal("fabricated authority command was not diagnosed")
	}
}

func enableTranscriptForTest(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "user", "debug", "authority-transcript.jsonl")
	t.Setenv("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT", "1")
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PROJECT_ENABLED", "1")
	t.Setenv("DICE_AND_DESTINY_RUNTIME_ROOT", root)
	t.Setenv("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH", path)
	transcript.ResetRegistryForTests()
	return root, path
}

func readTranscriptRecords(t *testing.T, path string) []transcript.Record {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var records []transcript.Record
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var record transcript.Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("corrupt JSONL record %d: %v\n%s", len(records)+1, err, scanner.Text())
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func assertTranscriptEnvelope(t *testing.T, records []transcript.Record) {
	t.Helper()
	if len(records) == 0 {
		t.Fatal("transcript is empty")
	}
	validVisibility := map[string]bool{
		transcript.VisibilityPublic: true, transcript.VisibilityPrivateSelf: true,
		transcript.VisibilityPrivateOpponent: true, transcript.VisibilityDebugSystem: true,
	}
	for index, record := range records {
		if record.SchemaVersion != transcript.SchemaVersion || record.RecorderVersion != transcript.RecorderVersion || record.Sequence != uint64(index+1) || record.RecordedAtUTC == "" || !validVisibility[record.Visibility] || record.Kind == "" || record.Summary == "" {
			t.Fatalf("invalid transcript envelope at %d: %#v", index, record)
		}
	}
}

func assertResultsEquivalent(t *testing.T, left, right engine.Result) {
	t.Helper()
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	var leftValue, rightValue any
	_ = json.Unmarshal(leftJSON, &leftValue)
	_ = json.Unmarshal(rightJSON, &rightValue)
	leftValue = normalizedViewerResult(leftValue)
	rightValue = normalizedViewerResult(rightValue)
	if !reflect.DeepEqual(leftValue, rightValue) {
		t.Fatalf("transcript changed viewer-safe authority result at %s", firstJSONDifference(leftValue, rightValue, "$"))
	}
}

func normalizedViewerResult(value any) any {
	root, _ := value.(map[string]any)
	events, _ := root["events"].([]any)
	normalizedEvents := make([]string, 0, len(events))
	for _, value := range events {
		eventMap, _ := value.(map[string]any)
		delete(eventMap, "event_id")
		delete(eventMap, "sequence")
		encoded, _ := json.Marshal(eventMap)
		normalizedEvents = append(normalizedEvents, string(encoded))
	}
	sort.Strings(normalizedEvents)
	root["events"] = normalizedEvents
	return root
}

func firstJSONDifference(left, right any, path string) string {
	if reflect.DeepEqual(left, right) {
		return ""
	}
	leftMap, leftIsMap := left.(map[string]any)
	rightMap, rightIsMap := right.(map[string]any)
	if leftIsMap && rightIsMap {
		keys := unionAnyKeys(leftMap, rightMap)
		for _, key := range keys {
			if difference := firstJSONDifference(leftMap[key], rightMap[key], path+"."+key); difference != "" {
				return difference
			}
		}
	}
	leftSlice, leftIsSlice := left.([]any)
	rightSlice, rightIsSlice := right.([]any)
	if leftIsSlice && rightIsSlice {
		if len(leftSlice) != len(rightSlice) {
			return path + " length"
		}
		for index := range leftSlice {
			if difference := firstJSONDifference(leftSlice[index], rightSlice[index], fmt.Sprintf("%s[%d]", path, index)); difference != "" {
				return difference
			}
		}
	}
	return fmt.Sprintf("%s: left=%v right=%v", path, left, right)
}

func unionAnyKeys(left, right map[string]any) []string {
	seen := map[string]bool{}
	for key := range left {
		seen[key] = true
	}
	for key := range right {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func findTranscriptRecord(records []transcript.Record, predicate func(transcript.Record) bool) transcript.Record {
	for _, record := range records {
		if predicate(record) {
			return record
		}
	}
	return transcript.Record{}
}

func intValues(value any) []int {
	encoded, _ := json.Marshal(value)
	var numbers []int
	_ = json.Unmarshal(encoded, &numbers)
	return numbers
}

func intValueAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func mapAny(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func recordDetailsHas(record transcript.Record, key string) bool {
	_, ok := record.Details[key]
	return ok
}
