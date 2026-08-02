package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"diceanddestiny/server/internal/battle/learned"
)

type episodeRecord struct {
	Index            int                      `json:"index"`
	Seed             uint64                   `json:"seed"`
	BattleID         string                   `json:"battle_id"`
	HumanSeat        string                   `json:"human_seat"`
	ModelSeat        string                   `json:"model_seat"`
	Result           string                   `json:"result"`
	Winner           string                   `json:"winner"`
	CommandCount     int                      `json:"command_count"`
	Commands         []learned.DecisionRecord `json:"commands"`
	HumanDecisions   int                      `json:"human_decisions"`
	ModelDecisions   int                      `json:"model_decisions"`
	DurationMS       float64                  `json:"duration_ms"`
	MeanInferenceMS  float64                  `json:"mean_inference_ms"`
	P95InferenceMS   float64                  `json:"p95_inference_ms"`
	AuthorityRejects int                      `json:"authority_rejects"`
	InvalidActions   int                      `json:"invalid_actions"`
	StaleActions     int                      `json:"stale_actions"`
	WrongSeatActions int                      `json:"wrong_seat_actions"`
	Timeouts         int                      `json:"timeouts"`
	Errors           int                      `json:"errors"`
	FallbackCount    int                      `json:"fallback_count"`
	TruncationReason string                   `json:"truncation_reason,omitempty"`
	HiddenLeaks      int                      `json:"hidden_information_leaks"`
}

func main() {
	serverRoot := defaultServerRoot()
	modelPath := flag.String("model", filepath.Join(serverRoot, "..", "dice-and-destiny-client", "models", "learned", "blade-warden-seed-11-final-v1.json"), "accepted exported learned policy")
	battles := flag.Int("battles", 100, "consecutive battles")
	seedStart := flag.Uint64("seed-start", 30_000_000, "first deterministic acceptance seed")
	output := flag.String("output", filepath.Join(serverRoot, "ml", "runs", "phase3-acceptance"), "acceptance output directory")
	flag.Parse()
	if *battles < 100 {
		fatalf("Phase 3 acceptance requires at least 100 battles")
	}
	session, err := learned.NewSession(learned.SessionConfig{
		ContentRoot:      filepath.Join(serverRoot, "content"),
		RunStateRoot:     filepath.Join(serverRoot, "save", "run_players"),
		ModelPath:        *modelPath,
		InferenceTimeout: 2 * time.Second,
	})
	if err != nil {
		fatalf("load accepted policy: %v", err)
	}

	started := time.Now()
	records := make([]episodeRecord, 0, *battles)
	failures := []string{}
	seenBattles := map[string]bool{}
	allInferenceSamples := []float64{}
	for index := 0; index < *battles; index++ {
		humanSeat := "seat-a"
		if index%2 == 1 {
			humanSeat = "seat-b"
		}
		seed := *seedStart + uint64(index)
		battleID := fmt.Sprintf("phase3-acceptance-%03d-%d", index, seed)
		episodeStarted := time.Now()
		view, resetErr := session.Reset(battleID, seed, humanSeat, index > 0)
		if resetErr != nil {
			failures = append(failures, fmt.Sprintf("battle %d reset: %v", index, resetErr))
			break
		}
		hiddenLeaks := checkHumanView(view)
		for actions := 0; actions < 1200 && !isComplete(view); actions++ {
			metadata := mapValue(view["learned_policy"])
			if boolValue(metadata["model_turn"]) {
				view, err = session.AdvanceModel()
			} else {
				commandJSON, commandErr := preferredProxyCommand(view)
				if commandErr != nil {
					err = commandErr
				} else {
					view, err = session.SubmitHuman(commandJSON)
				}
			}
			if err != nil {
				failures = append(failures, fmt.Sprintf("battle %d action: %v", index, err))
				break
			}
			hiddenLeaks += checkHumanView(view)
		}
		telemetry, _ := session.Telemetry()
		allInferenceSamples = append(allInferenceSamples, telemetry.InferenceLatencyMS...)
		record := episodeRecord{
			Index: index, Seed: seed, BattleID: telemetry.BattleID,
			HumanSeat: telemetry.HumanSeat, ModelSeat: telemetry.ModelSeat,
			Result: telemetry.Result, Winner: telemetry.Winner,
			CommandCount: len(telemetry.Commands), Commands: telemetry.Commands, HumanDecisions: telemetry.HumanDecisions,
			ModelDecisions:   telemetry.ModelDecisions,
			DurationMS:       float64(time.Since(episodeStarted).Microseconds()) / 1000,
			MeanInferenceMS:  mean(telemetry.InferenceLatencyMS),
			P95InferenceMS:   percentile(telemetry.InferenceLatencyMS, 0.95),
			AuthorityRejects: telemetry.AuthorityRejects, InvalidActions: telemetry.InvalidActions,
			StaleActions: telemetry.StaleActions, WrongSeatActions: telemetry.WrongSeatActions,
			Timeouts: telemetry.Timeouts, Errors: len(telemetry.Errors),
			FallbackCount: telemetry.FallbackCount, TruncationReason: telemetry.TruncationReason,
			HiddenLeaks: hiddenLeaks,
		}
		records = append(records, record)
		if seenBattles[record.BattleID] || record.BattleID != battleID || telemetry.Seed != seed {
			failures = append(failures, fmt.Sprintf("battle %d leaked reset identity/state", index))
		}
		seenBattles[record.BattleID] = true
		if !isComplete(view) || record.TruncationReason != "" || record.ModelDecisions == 0 {
			failures = append(failures, fmt.Sprintf("battle %d did not complete through the accepted policy", index))
		}
		if record.AuthorityRejects+record.InvalidActions+record.StaleActions+record.WrongSeatActions+record.Timeouts+record.Errors+record.FallbackCount+record.HiddenLeaks != 0 {
			failures = append(failures, fmt.Sprintf("battle %d recorded a prohibited failure", index))
		}
		for decisionIndex, decision := range telemetry.Commands {
			if decision.Sequence != decisionIndex+1 ||
				decision.Command.BattleID != telemetry.BattleID {
				failures = append(failures, fmt.Sprintf("battle %d retained an invalid or cross-battle command record", index))
			}
			if decision.Controller == "learned_policy" && decision.ActorSeat != telemetry.ModelSeat {
				failures = append(failures, fmt.Sprintf("battle %d learned command used wrong seat", index))
			}
			if decision.Controller == "human" && decision.ActorSeat != telemetry.HumanSeat {
				failures = append(failures, fmt.Sprintf("battle %d human command used wrong seat", index))
			}
		}
		if len(telemetry.Commands) != telemetry.HumanDecisions+telemetry.ModelDecisions {
			failures = append(failures, fmt.Sprintf("battle %d decision trace/count mismatch", index))
		}
		if err != nil {
			break
		}
	}

	_, lifetime := session.Telemetry()
	durations := []float64{}
	results := map[string]int{}
	totals := episodeRecord{}
	seatCounts := map[string]int{}
	modelDecisions := 0
	actionFrequency := map[string]map[string]int{"human": {}, "learned_policy": {}}
	for _, record := range records {
		durations = append(durations, record.DurationMS)
		results[record.Result]++
		seatCounts[record.HumanSeat]++
		modelDecisions += record.ModelDecisions
		for _, decision := range record.Commands {
			actionFrequency[decision.Controller][string(decision.Command.Type)]++
		}
		totals.AuthorityRejects += record.AuthorityRejects
		totals.InvalidActions += record.InvalidActions
		totals.StaleActions += record.StaleActions
		totals.WrongSeatActions += record.WrongSeatActions
		totals.Timeouts += record.Timeouts
		totals.Errors += record.Errors
		totals.FallbackCount += record.FallbackCount
		totals.HiddenLeaks += record.HiddenLeaks
	}
	if len(records) != *battles {
		failures = append(failures, fmt.Sprintf("recorded %d of %d requested battles", len(records), *battles))
	}
	if seatCounts["seat-a"] == 0 || seatCounts["seat-b"] == 0 {
		failures = append(failures, "both human seat assignments were not exercised")
	}
	if lifetime.ModelLoadCount != 1 || lifetime.BattlesStarted != len(records) || lifetime.Rematches != max(0, len(records)-1) {
		failures = append(failures, fmt.Sprintf("persistent lifetime mismatch: %#v", lifetime))
	}
	summary := map[string]any{
		"acceptance_passed":         len(failures) == 0,
		"acceptance_failures":       failures,
		"battles":                   len(records),
		"human_seat_counts":         seatCounts,
		"results":                   results,
		"model_id":                  learned.AcceptedModelID,
		"policy_export_sha256":      learned.AcceptedPolicyFileSHA256,
		"checkpoint_sha256":         learned.AcceptedCheckpointSHA256,
		"parameter_sha256":          learned.AcceptedParameterSHA256,
		"model_decisions":           modelDecisions,
		"action_frequency":          actionFrequency,
		"model_load_count":          lifetime.ModelLoadCount,
		"process_restarts":          0,
		"mean_battle_duration_ms":   mean(durations),
		"p95_battle_duration_ms":    percentile(durations, 0.95),
		"mean_inference_latency_ms": mean(allInferenceSamples),
		"p95_inference_latency_ms":  percentile(allInferenceSamples, 0.95),
		"authority_rejects":         totals.AuthorityRejects,
		"invalid_actions":           totals.InvalidActions,
		"stale_actions":             totals.StaleActions,
		"wrong_seat_actions":        totals.WrongSeatActions,
		"timeouts":                  totals.Timeouts,
		"errors":                    totals.Errors,
		"fallback_count":            totals.FallbackCount,
		"hidden_information_leaks":  totals.HiddenLeaks,
		"unexplained_truncations":   countTruncations(records),
		"elapsed_seconds":           time.Since(started).Seconds(),
	}
	if err := writeResults(*output, records, summary); err != nil {
		fatalf("write acceptance results: %v", err)
	}
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(encoded))
	if len(failures) != 0 {
		os.Exit(1)
	}
}

func preferredProxyCommand(view map[string]any) (string, error) {
	actions := listValue(view["legal_actions"])
	if len(actions) == 0 {
		return "", fmt.Errorf("human proxy has no visible legal action")
	}
	for _, kind := range []string{"planning_roll", "planning_select_ability", "planning_reroll", "roll_dice", "planning_pass", "pass", "commit_interaction"} {
		for index := len(actions) - 1; index >= 0; index-- {
			action := mapValue(actions[index])
			if stringValue(action["type"]) == kind {
				encoded, _ := json.Marshal(action)
				return string(encoded), nil
			}
		}
	}
	encoded, _ := json.Marshal(actions[0])
	return string(encoded), nil
}

func checkHumanView(view map[string]any) int {
	snapshot := mapValue(view["snapshot"])
	actors := mapValue(snapshot["actors"])
	opponent := mapValue(actors[learned.ModelAlias])
	if len(listValue(opponent["hand"])) != 0 || len(mapValue(opponent["card_instances"])) != 0 {
		return 1
	}
	if stringValue(snapshot["stage"]) == "planning" && len(listValue(opponent["roll_history"])) != 0 {
		return 1
	}
	return 0
}

func isComplete(view map[string]any) bool {
	result := stringValue(view["battle_result"])
	return result == "victory" || result == "defeat" || result == "draw"
}

func writeResults(output string, records []episodeRecord, summary map[string]any) error {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	var episodes []byte
	for _, record := range records {
		encoded, _ := json.Marshal(record)
		episodes = append(episodes, encoded...)
		episodes = append(episodes, '\n')
	}
	if err := os.WriteFile(filepath.Join(output, "episodes.jsonl"), episodes, 0o600); err != nil {
		return err
	}
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	return os.WriteFile(filepath.Join(output, "summary.json"), append(encoded, '\n'), 0o600)
}

func countTruncations(records []episodeRecord) int {
	count := 0
	for _, record := range records {
		if record.TruncationReason != "" {
			count++
		}
	}
	return count
}

func percentile(values []float64, quantile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	position := float64(len(ordered)-1) * quantile
	lower, upper := int(math.Floor(position)), int(math.Ceil(position))
	if lower == upper {
		return ordered[lower]
	}
	weight := position - float64(lower)
	return ordered[lower]*(1-weight) + ordered[upper]*weight
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func listValue(value any) []any {
	result, _ := value.([]any)
	return result
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func defaultServerRoot() string {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
