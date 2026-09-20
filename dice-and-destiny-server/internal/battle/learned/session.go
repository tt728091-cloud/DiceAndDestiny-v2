package learned

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/mlsim"
)

const (
	HumanAlias = "blade"
	ModelAlias = "goblin"
)

type SessionConfig struct {
	OpponentDefinition string
	OpponentCount      int

	ContentRoot         string
	RunStateRoot        string
	ModelPath           string
	ModelSHA256         string
	ObservationManifest string
	DiagnosticsPath     string
	InferenceTimeout    time.Duration
}

type sessionPolicy interface {
	Metadata() PolicyMetadata
	Select(mlsim.Transition) (int, time.Duration, error)
}

type DecisionRecord struct {
	Sequence    int             `json:"sequence"`
	Controller  string          `json:"controller"`
	ActorSeat   string          `json:"actor_seat"`
	ActionIndex int             `json:"action_index"`
	Command     command.Command `json:"command"`
	InferenceMS float64         `json:"inference_ms,omitempty"`
}

type BattleTelemetry struct {
	Model              PolicyMetadata   `json:"model"`
	Seed               uint64           `json:"seed"`
	BattleID           string           `json:"battle_id"`
	HumanSeat          string           `json:"human_seat"`
	ModelSeat          string           `json:"model_seat"`
	Commands           []DecisionRecord `json:"commands"`
	HumanDecisions     int              `json:"human_decisions"`
	ModelDecisions     int              `json:"model_decisions"`
	InferenceLatencyMS []float64        `json:"inference_latency_ms"`
	Winner             string           `json:"winner,omitempty"`
	Result             string           `json:"result,omitempty"`
	AuthorityRejects   int              `json:"authority_rejects"`
	InvalidActions     int              `json:"invalid_actions"`
	StaleActions       int              `json:"stale_actions"`
	WrongSeatActions   int              `json:"wrong_seat_actions"`
	TruncationReason   string           `json:"truncation_reason,omitempty"`
	Errors             []string         `json:"errors"`
	Timeouts           int              `json:"timeouts"`
	FallbackCount      int              `json:"fallback_count"`
	StartedAtUTC       string           `json:"started_at_utc"`
	CompletedAtUTC     string           `json:"completed_at_utc,omitempty"`
}

type LifetimeTelemetry struct {
	ModelLoadCount int `json:"model_load_count"`
	BattlesStarted int `json:"battles_started"`
	Rematches      int `json:"rematches"`
	Errors         int `json:"errors"`
	Timeouts       int `json:"timeouts"`
	FallbackCount  int `json:"fallback_count"`
}

type Session struct {
	mu               sync.Mutex
	config           SessionConfig
	policy           sessionPolicy
	environment      *mlsim.Environment
	current          mlsim.Transition
	humanSeat        string
	modelSeat        string
	telemetry        BattleTelemetry
	lifetime         LifetimeTelemetry
	decisionSequence int
}

func NewSession(config SessionConfig) (*Session, error) {
	if config.OpponentCount < 0 || config.OpponentCount > 2 || (config.OpponentCount > 1 && config.OpponentDefinition == "") {
		return nil, fmt.Errorf("opponent count must be one or two scripted minions")
	}
	if config.ContentRoot == "" || (config.ModelPath == "" && config.OpponentDefinition == "") {
		return nil, fmt.Errorf("content root and a learned-policy path or single-ability opponent are required")
	}
	if config.InferenceTimeout <= 0 {
		config.InferenceTimeout = 2 * time.Second
	}
	if err := VerifyContentVersion(config.ContentRoot); err != nil {
		return nil, err
	}
	var policy sessionPolicy
	var err error
	if config.OpponentDefinition != "" {
		policy, err = loadSingleAbilityPolicy(config.ContentRoot, config.OpponentDefinition)
	} else {
		policy, err = loadSessionPolicy(config.ModelPath, config.ModelSHA256)
	}
	if err != nil {
		return nil, err
	}
	environment, err := mlsim.New(mlsim.Config{
		IncludeContentCatalog: config.OpponentDefinition != "",
		ContentRoot:           config.ContentRoot,
		RunStateRoot:          config.RunStateRoot,
		MaxActions:            mlsim.DefaultMaxActions,
		SessionID:             "phase3-player",
		ObservationSchema:     policy.Metadata().ObservationSchema,
		ObservationManifest:   config.ObservationManifest,
	})
	if err != nil {
		return nil, err
	}
	return &Session{
		config:      config,
		policy:      policy,
		environment: environment,
		lifetime:    LifetimeTelemetry{ModelLoadCount: modelLoadCount(policy)},
	}, nil
}

func loadSessionPolicy(path, expectedSHA256 string) (sessionPolicy, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read learned policy metadata: %w", err)
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(payload, &header); err != nil {
		return nil, fmt.Errorf("decode learned policy metadata: %w", err)
	}
	switch header.Format {
	case PolicyFormat:
		if expectedSHA256 != "" && expectedSHA256 != AcceptedPolicyFileSHA256 {
			return nil, fmt.Errorf(
				"accepted v1 learned policy SHA-256 mismatch: got pin %s, want %s",
				expectedSHA256,
				AcceptedPolicyFileSHA256,
			)
		}
		return LoadAcceptedPolicy(path)
	case PolicyFormatV2:
		return LoadCandidatePolicyV2(path, expectedSHA256)
	case PolicyFormatV3:
		return LoadCandidatePolicyV3(path, expectedSHA256)
	default:
		return nil, fmt.Errorf("unsupported learned policy format %q", header.Format)
	}
}

func (s *Session) Reset(battleID string, seed uint64, humanSeat string, rematch bool) (map[string]any, error) {
	return s.ResetCharacter(battleID, seed, humanSeat, rematch, "blade_warden")
}

func (s *Session) ResetCharacter(battleID string, seed uint64, humanSeat string, rematch bool, character string) (map[string]any, error) {
	if character == "" {
		character = "blade_warden"
	}
	if character != "blade_warden" && character != "venom" {
		return nil, fmt.Errorf("unknown playable character %q", character)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if humanSeat != "seat-a" && humanSeat != "seat-b" {
		return nil, fmt.Errorf("human seat must be seat-a or seat-b")
	}
	if s.config.OpponentCount > 1 && s.config.OpponentDefinition == "" {
		return nil, fmt.Errorf("multiple opponents require a scripted minion")
	}
	s.humanSeat = humanSeat
	s.modelSeat = otherSeat(humanSeat)
	opponent := "blade_warden"
	if s.config.OpponentDefinition != "" {
		opponent = s.config.OpponentDefinition
	}
	definitions := map[string]string{s.humanSeat: character, s.modelSeat: opponent}
	models := map[string]string{s.humanSeat: "human-godot-ui", s.modelSeat: s.policy.Metadata().ModelID}
	var teams map[string]string
	if s.config.OpponentCount == 2 {
		definitions["seat-c"] = opponent
		models["seat-c"] = s.policy.Metadata().ModelID
		teams = map[string]string{s.humanSeat: "heroes", s.modelSeat: "minions", "seat-c": "minions"}
	}
	transition, err := s.environment.Reset(mlsim.ResetRequest{
		Seed:            seed,
		BattleID:        battleID,
		SeatDefinitions: definitions, SeatModels: models, SeatTeams: teams,
	})
	if err != nil {
		s.lifetime.Errors++
		return nil, err
	}
	s.current = transition
	s.decisionSequence = 0
	s.lifetime.BattlesStarted++
	if rematch {
		s.lifetime.Rematches++
	}
	s.telemetry = BattleTelemetry{
		Model:              s.policy.Metadata(),
		Seed:               seed,
		BattleID:           transition.Metrics.BattleID,
		HumanSeat:          s.humanSeat,
		ModelSeat:          s.modelSeat,
		Commands:           []DecisionRecord{},
		InferenceLatencyMS: []float64{},
		Errors:             []string{},
		StartedAtUTC:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := s.recordDiagnostic("battle_started", s.telemetry); err != nil {
		return nil, err
	}
	view, err := s.environment.Observe(s.humanSeat)
	if err != nil {
		return nil, err
	}
	return s.present(view), nil
}

func (s *Session) SubmitHuman(commandJSON string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.ActorID != s.humanSeat {
		s.telemetry.WrongSeatActions++
		battle.RecordAuthorityTranscriptDiagnostic(s.telemetry.BattleID, s.humanSeat, "command_rejected_wrong_seat", map[string]any{"controller": "human", "summary": "Human command rejected because the learned opponent owns the decision", "current_actor_id": s.current.ActorID})
		return nil, fmt.Errorf("human command rejected while %s owns the current decision", s.current.ActorID)
	}
	actualJSON, err := s.unaliasCommand(commandJSON)
	if err != nil {
		s.telemetry.InvalidActions++
		return nil, err
	}
	action, err := command.ParseJSON(actualJSON)
	if err != nil {
		s.telemetry.InvalidActions++
		return nil, err
	}
	if action.ActorID != s.humanSeat {
		s.telemetry.WrongSeatActions++
		battle.RecordAuthorityTranscriptDiagnostic(s.telemetry.BattleID, action.ActorID, "command_rejected_wrong_seat", map[string]any{"controller": "human", "summary": "Human command actor did not match the configured human seat", "human_seat": s.humanSeat})
		return nil, fmt.Errorf("human command actor %q does not match human seat %q", action.ActorID, s.humanSeat)
	}
	index := currentActionIndex(s.current.Result.LegalActions, action)
	if index < 0 {
		s.telemetry.StaleActions++
		battle.RecordAuthorityTranscriptDiagnostic(s.telemetry.BattleID, action.ActorID, "command_rejected_stale", map[string]any{"controller": "human", "summary": "Human command was stale, fabricated, or no longer legal", "command": action})
		return nil, fmt.Errorf("human command is stale, fabricated, or no longer legal")
	}
	transition, result, err := s.environment.StepCommandForViewerWithTranscript(action, s.humanSeat, battle.TranscriptCommandContext{
		Controller:     "human",
		ActionIndex:    index,
		CandidateCount: len(s.current.Result.LegalActions),
		Metadata:       map[string]any{"human_seat": s.humanSeat, "model_seat": s.modelSeat},
	})
	if err != nil {
		s.telemetry.AuthorityRejects++
		s.telemetry.Errors = append(s.telemetry.Errors, err.Error())
		s.lifetime.Errors++
		return nil, err
	}
	s.current = transition
	s.telemetry.HumanDecisions++
	s.appendDecision("human", index, action, 0)
	s.updateTerminalTelemetry()
	if err := s.recordDiagnostic("human_decision", s.telemetry.Commands[len(s.telemetry.Commands)-1]); err != nil {
		return nil, err
	}
	return s.present(result), nil
}

func (s *Session) AdvanceModel() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.Terminal || s.current.TruncationReason != "" {
		return nil, fmt.Errorf("battle is complete")
	}
	if !s.isModelSeat(s.current.ActorID) {
		return nil, fmt.Errorf("learned policy does not own the current decision")
	}
	selected, latency, err := s.policy.Select(s.current)
	if err != nil {
		s.telemetry.Errors = append(s.telemetry.Errors, err.Error())
		s.lifetime.Errors++
		s.recordDiagnostic("model_error", map[string]any{"error": err.Error()})
		battle.RecordAuthorityTranscriptDiagnostic(s.telemetry.BattleID, s.modelSeat, "model_error", map[string]any{"controller": "learned_policy", "summary": "Learned-policy inference failed", "error": err.Error(), "model": s.policy.Metadata()})
		return nil, err
	}
	if latency > s.config.InferenceTimeout {
		s.telemetry.Timeouts++
		s.lifetime.Timeouts++
		err = fmt.Errorf("learned-policy inference exceeded %s (measured %s)", s.config.InferenceTimeout, latency)
		s.telemetry.Errors = append(s.telemetry.Errors, err.Error())
		s.recordDiagnostic("model_timeout", map[string]any{"error": err.Error(), "latency_ms": milliseconds(latency)})
		battle.RecordAuthorityTranscriptDiagnostic(s.telemetry.BattleID, s.modelSeat, "model_timeout", map[string]any{"controller": "learned_policy", "summary": "Learned-policy inference timed out without submitting a fallback", "error": err.Error(), "latency_ms": milliseconds(latency), "model": s.policy.Metadata()})
		return nil, err
	}
	if selected < 0 || selected >= len(s.current.Result.LegalActions) {
		s.telemetry.InvalidActions++
		return nil, fmt.Errorf("learned policy selected invalid action index %d", selected)
	}
	action := s.current.Result.LegalActions[selected]
	metadata := s.policy.Metadata()
	transition, result, err := s.environment.StepForViewerWithTranscript(selected, s.humanSeat, battle.TranscriptCommandContext{
		Controller:     "learned_policy",
		ActionIndex:    selected,
		CandidateCount: len(s.current.Result.LegalActions),
		InferenceMS:    milliseconds(latency),
		Metadata: map[string]any{
			"model_id":             metadata.ModelID,
			"checkpoint_sha256":    metadata.SourceCheckpointSHA256,
			"policy_export_sha256": metadata.PolicyExportSHA256,
			"human_seat":           s.humanSeat,
			"model_seat":           s.modelSeat,
		},
	})
	if err != nil {
		s.telemetry.AuthorityRejects++
		s.telemetry.Errors = append(s.telemetry.Errors, err.Error())
		s.lifetime.Errors++
		return nil, err
	}
	s.current = transition
	s.telemetry.ModelDecisions++
	s.telemetry.InferenceLatencyMS = append(s.telemetry.InferenceLatencyMS, milliseconds(latency))
	s.appendDecision("learned_policy", selected, action, milliseconds(latency))
	s.updateTerminalTelemetry()
	if err := s.recordDiagnostic("model_decision", s.telemetry.Commands[len(s.telemetry.Commands)-1]); err != nil {
		return nil, err
	}
	return s.present(result), nil
}

func (s *Session) Telemetry() (BattleTelemetry, LifetimeTelemetry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneTelemetry(s.telemetry), s.lifetime
}

func (s *Session) CurrentActor() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current.ActorID
}

func (s *Session) appendDecision(controller string, index int, action command.Command, inferenceMS float64) {
	s.decisionSequence++
	s.telemetry.Commands = append(s.telemetry.Commands, DecisionRecord{
		Sequence:    s.decisionSequence,
		Controller:  controller,
		ActorSeat:   action.ActorID,
		ActionIndex: index,
		Command:     action,
		InferenceMS: inferenceMS,
	})
}

func (s *Session) updateTerminalTelemetry() {
	metrics := s.environment.Metrics()
	s.telemetry.AuthorityRejects = metrics.AuthorityRejects
	s.telemetry.InvalidActions += metrics.InvalidActionIDs
	s.telemetry.StaleActions += metrics.StaleActions
	s.telemetry.WrongSeatActions += metrics.WrongSeatActions
	s.telemetry.TruncationReason = metrics.TruncationReason
	if !s.current.Terminal && s.current.TruncationReason == "" {
		return
	}
	s.telemetry.Winner = metrics.Winner
	s.telemetry.Result = humanResult(metrics.Winner, metrics.Status, s.humanSeat)
	s.telemetry.CompletedAtUTC = time.Now().UTC().Format(time.RFC3339Nano)
	s.recordDiagnostic("battle_completed", s.telemetry)
}

func (s *Session) present(result engine.Result) map[string]any {
	encoded, _ := json.Marshal(result)
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var view map[string]any
	_ = decoder.Decode(&view)
	view = aliasMap(view, s.aliases(false))
	modelTurn := !s.current.Terminal && s.current.TruncationReason == "" && s.isModelSeat(s.current.ActorID)
	if modelTurn {
		delete(view, "pending_input")
		delete(view, "legal_actions")
	}
	if s.current.Terminal {
		view["battle_result"] = humanResult(s.current.Winner, s.current.Metrics.Status, s.humanSeat)
	}
	view["learned_policy"] = map[string]any{
		"enabled":                  true,
		"controller_kind":          s.policy.Metadata().Algorithm,
		"model_turn":               modelTurn,
		"acting_actor_id":          s.aliases(false)[s.current.ActorID],
		"opponent_count":           max(1, s.config.OpponentCount),
		"model_id":                 s.policy.Metadata().ModelID,
		"policy_export_sha256":     s.policy.Metadata().PolicyExportSHA256,
		"checkpoint_sha256":        s.policy.Metadata().SourceCheckpointSHA256,
		"parameter_sha256":         s.policy.Metadata().SourceParameterSHA256,
		"phase2_revision":          s.policy.Metadata().SourceRevision,
		"training_engine_revision": s.policy.Metadata().TrainingEngineRevision,
		"content_version":          s.policy.Metadata().ContentVersion,
		"runtime_rules":            "automatic-effects-v1",
		"architecture":             s.policy.Metadata().Architecture,
		"environment_schema":       s.policy.Metadata().EnvironmentSchema,
		"observation_schema":       s.policy.Metadata().ObservationSchema,
		"action_schema":            s.policy.Metadata().ActionSchema,
		"human_seat":               s.humanSeat,
		"model_seat":               s.modelSeat,
		"seed":                     s.telemetry.Seed,
		"model_decisions":          s.telemetry.ModelDecisions,
		"human_decisions":          s.telemetry.HumanDecisions,
		"timeouts":                 s.telemetry.Timeouts,
		"errors":                   len(s.telemetry.Errors),
		"fallback_count":           0,
		"model_load_count":         s.lifetime.ModelLoadCount,
	}
	return view
}

func (s *Session) unaliasCommand(commandJSON string) (string, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(commandJSON))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode human command: %w", err)
	}
	actual := aliasMap(value, s.aliases(true))
	encoded, err := json.Marshal(actual)
	if err != nil {
		return "", fmt.Errorf("encode human command: %w", err)
	}
	return string(encoded), nil
}

func (s *Session) recordDiagnostic(kind string, payload any) error {
	if s.config.DiagnosticsPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.config.DiagnosticsPath), 0o755); err != nil {
		return fmt.Errorf("create Phase 3 diagnostics directory: %w", err)
	}
	record := map[string]any{
		"kind":            kind,
		"recorded_at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		"battle_id":       s.telemetry.BattleID,
		"payload":         payload,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode Phase 3 diagnostic: %w", err)
	}
	file, err := os.OpenFile(s.config.DiagnosticsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open Phase 3 diagnostics: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write Phase 3 diagnostics: %w", err)
	}
	return nil
}

func aliasMap(value map[string]any, aliases map[string]string) map[string]any {
	return aliasValue(value, aliases).(map[string]any)
}

func aliasValue(value any, aliases map[string]string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			aliasKey := key
			if replacement, ok := aliases[key]; ok {
				aliasKey = replacement
			}
			result[aliasKey] = aliasValue(typed[key], aliases)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = aliasValue(typed[index], aliases)
		}
		return result
	case string:
		if replacement, ok := aliases[typed]; ok {
			return replacement
		}
		return typed
	default:
		return typed
	}
}

func currentActionIndex(actions []command.Command, target command.Command) int {
	for index, action := range actions {
		if action.BattleID != target.BattleID || action.ActorID != target.ActorID || action.Type != target.Type {
			continue
		}
		var left, right any
		if json.Unmarshal(action.Payload, &left) == nil && json.Unmarshal(target.Payload, &right) == nil {
			leftJSON, _ := json.Marshal(left)
			rightJSON, _ := json.Marshal(right)
			if bytes.Equal(leftJSON, rightJSON) {
				return index
			}
		}
	}
	return -1
}

func otherSeat(seat string) string {
	if seat == "seat-a" {
		return "seat-b"
	}
	return "seat-a"
}

func humanResult(winner string, status any, humanSeat string) string {
	if fmt.Sprint(status) == "draw" || winner == "" {
		return "draw"
	}
	if winner == humanSeat {
		return "victory"
	}
	return "defeat"
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func cloneTelemetry(source BattleTelemetry) BattleTelemetry {
	encoded, _ := json.Marshal(source)
	var result BattleTelemetry
	_ = json.Unmarshal(encoded, &result)
	return result
}

func modelLoadCount(policy sessionPolicy) int {
	if _, ok := policy.(*singleAbilityPolicy); ok {
		return 0
	}
	return 1
}

func (s *Session) isModelSeat(id string) bool {
	return id == s.modelSeat || (s.config.OpponentCount == 2 && id == "seat-c")
}
func (s *Session) aliases(reverse bool) map[string]string {
	m := map[string]string{s.humanSeat: HumanAlias, s.modelSeat: ModelAlias}
	if s.config.OpponentCount == 2 {
		m["seat-c"] = "goblin-2"
	}
	if !reverse {
		return m
	}
	r := map[string]string{}
	for k, v := range m {
		r[v] = k
	}
	return r
}
