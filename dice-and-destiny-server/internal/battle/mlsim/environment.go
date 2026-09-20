// Package mlsim exposes the real battle authority as a small, versioned,
// long-running environment for local machine-learning experiments.
package mlsim

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"diceanddestiny/server/internal/battle"
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

const (
	EnvironmentSchemaVersion = "dice-and-destiny-ml-env-v1"
	ObservationSchemaVersion = "dice-and-destiny-observation-v1"
	ActionSchemaVersion      = "dice-and-destiny-action-candidates-v1"
	EnvironmentSchemaV2      = "dice-and-destiny-ml-env-v2"
	ObservationSchemaV2      = "dice-and-destiny-observation-v2"
	ActionSchemaV2           = "dice-and-destiny-action-candidates-v2"
	EnvironmentSchemaV3      = "dice-and-destiny-ml-env-v3"
	ObservationSchemaV3      = "dice-and-destiny-observation-v3"
	ActionSchemaV3           = "dice-and-destiny-action-candidates-v3"
	DefaultMaxActions        = 1200
	AuthorityModeNormal      = "normal"
	AuthorityModeEphemeral   = "ephemeral"
	TelemetryModeFull        = "full"
	TelemetryModeTraining    = "training"
)

var SeatIDs = []string{"seat-a", "seat-b"}

type Config struct {
	IncludeContentCatalog bool

	ContentRoot         string
	RunStateRoot        string
	MaxActions          int
	SessionID           string
	AuthorityMode       string
	TelemetryMode       string
	TransportMode       string
	ObservationSchema   string
	ObservationManifest string
}

type ResetRequest struct {
	SeatTeams       map[string]string `json:"seat_teams,omitempty"`
	Seed            uint64            `json:"seed"`
	BattleID        string            `json:"battle_id,omitempty"`
	SeatModels      map[string]string `json:"seat_models,omitempty"`
	SeatDefinitions map[string]string `json:"seat_definitions,omitempty"`
}

type ActionRecord struct {
	Index   int             `json:"action_index"`
	ActorID string          `json:"actor_id"`
	Command command.Command `json:"command"`
}

type ReplayRecord struct {
	SeatTeams         map[string]string  `json:"seat_teams,omitempty"`
	EnvironmentSchema string             `json:"environment_schema"`
	ObservationSchema string             `json:"observation_schema"`
	ActionSchema      string             `json:"action_schema"`
	BattleID          string             `json:"battle_id"`
	Seed              uint64             `json:"seed"`
	SeatModels        map[string]string  `json:"seat_models"`
	SeatDefinitions   map[string]string  `json:"seat_definitions"`
	Actions           []ActionRecord     `json:"actions"`
	Winner            string             `json:"winner,omitempty"`
	Status            state.BattleStatus `json:"status,omitempty"`
	TruncationReason  string             `json:"truncation_reason,omitempty"`
}

type EpisodeMetrics struct {
	BattleID         string              `json:"battle_id"`
	Seed             uint64              `json:"seed"`
	Actions          int                 `json:"actions"`
	Rounds           int                 `json:"rounds"`
	Winner           string              `json:"winner,omitempty"`
	Status           state.BattleStatus  `json:"status,omitempty"`
	TruncationReason string              `json:"truncation_reason,omitempty"`
	TruncationActor  string              `json:"truncation_actor,omitempty"`
	ActionFrequency  map[string]int      `json:"action_frequency"`
	RemainingHealth  map[string]int      `json:"remaining_health,omitempty"`
	AuthorityRejects int                 `json:"authority_rejects"`
	InvalidActionIDs int                 `json:"invalid_action_indices"`
	StaleActions     int                 `json:"stale_action_submissions"`
	WrongSeatActions int                 `json:"wrong_seat_submissions"`
	DurationMillis   float64             `json:"duration_ms"`
	RandomCursor     uint64              `json:"random_cursor"`
	DamageBySeat     DamageBySeatMetrics `json:"damage_by_seat"`
}

type DamageMetrics struct {
	RawAttack     int `json:"raw_attack"`
	RawBleed      int `json:"raw_bleed"`
	RawPoison     int `json:"raw_poison"`
	RawTotal      int `json:"raw_total"`
	ResolvedTotal int `json:"resolved_total"`
	ActualTotal   int `json:"actual_total"`
}

type DamageBySeatMetrics struct {
	SeatA DamageMetrics `json:"seat-a"`
	SeatB DamageMetrics `json:"seat-b"`
}

type Transition struct {
	ActorID          string           `json:"actor_id,omitempty"`
	Result           engine.Result    `json:"result"`
	Terminal         bool             `json:"terminal"`
	Winner           string           `json:"winner,omitempty"`
	TruncationReason string           `json:"truncation_reason,omitempty"`
	Metrics          EpisodeMetrics   `json:"metrics"`
	Replay           *ReplayRecord    `json:"replay,omitempty"`
	EncodedDecision  *EncodedDecision `json:"encoded_decision,omitempty"`
}

type Environment struct {
	seatIDs      []string
	config       Config
	assembler    battle.ParticipantAssembler
	authority    *battle.Authority
	current      Transition
	legalActions []command.Command
	replay       ReplayRecord
	metrics      EpisodeMetrics
	episode      uint64
	startedAt    time.Time
	manifestV3   *observationManifestV3
}

func New(config Config) (*Environment, error) {
	if config.ContentRoot == "" {
		return nil, errors.New("content root is required")
	}
	if config.MaxActions <= 0 {
		config.MaxActions = DefaultMaxActions
	}
	if config.SessionID == "" {
		config.SessionID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if config.AuthorityMode == "" {
		config.AuthorityMode = AuthorityModeNormal
	}
	if config.AuthorityMode != AuthorityModeNormal && config.AuthorityMode != AuthorityModeEphemeral {
		return nil, fmt.Errorf("unknown authority mode %q", config.AuthorityMode)
	}
	if config.TelemetryMode == "" {
		config.TelemetryMode = TelemetryModeFull
	}
	if config.TelemetryMode != TelemetryModeFull && config.TelemetryMode != TelemetryModeTraining {
		return nil, fmt.Errorf("unknown telemetry mode %q", config.TelemetryMode)
	}
	if config.TransportMode == "" {
		config.TransportMode = TransportModeFull
	}
	if config.TransportMode != TransportModeFull && config.TransportMode != TransportModeEncoded && config.TransportMode != TransportModeParity {
		return nil, fmt.Errorf("unknown transport mode %q", config.TransportMode)
	}
	if config.ObservationSchema == "" {
		config.ObservationSchema = ObservationSchemaVersion
	}
	if config.ObservationSchema != ObservationSchemaVersion && config.ObservationSchema != ObservationSchemaV2 && config.ObservationSchema != ObservationSchemaV3 {
		return nil, fmt.Errorf("unknown observation schema %q", config.ObservationSchema)
	}
	var manifestV3 *observationManifestV3
	if config.ObservationSchema == ObservationSchemaV3 {
		var err error
		manifestV3, err = loadObservationManifestV3(config.ObservationManifest)
		if err != nil {
			return nil, err
		}
	}
	return &Environment{
		config:     config,
		assembler:  battle.NewCachedFileParticipantAssembler(config.ContentRoot, config.RunStateRoot),
		manifestV3: manifestV3,
	}, nil
}

// Reset creates a completely new in-memory authority and a fresh mirror battle.
// The Go process stays alive, but no battle checkpoint can leak across episodes.
func (e *Environment) Reset(request ResetRequest) (Transition, error) {
	e.episode++
	battleID := request.BattleID
	if battleID == "" {
		battleID = fmt.Sprintf("ml-%s-%06d-%d", e.config.SessionID, e.episode, request.Seed)
	}
	if request.SeatModels == nil {
		request.SeatModels = map[string]string{}
	}
	if request.SeatDefinitions == nil {
		request.SeatDefinitions = map[string]string{SeatIDs[0]: "blade_warden", SeatIDs[1]: "blade_warden"}
	}
	e.seatIDs = nil
	for id := range request.SeatDefinitions {
		e.seatIDs = append(e.seatIDs, id)
	}
	sort.Strings(e.seatIDs)
	if len(e.seatIDs) != 2 && (e.config.ObservationSchema != ObservationSchemaVersion || e.config.TransportMode != TransportModeFull) {
		return Transition{}, errors.New("multi-combatant encounters require raw observations")
	}
	for _, seatID := range e.seatIDs {
		if request.SeatDefinitions[seatID] == "" {
			return Transition{}, fmt.Errorf("definition id is required for %s", seatID)
		}
	}
	var repo repository.Repository = repository.NewInMemory()
	if e.config.AuthorityMode == AuthorityModeEphemeral {
		repo = repository.NewEphemeral()
	}
	simulationEngine, err := engine.NewEngineWithConfig(engine.Config{
		OmitSnapshotContentCatalog: !e.config.IncludeContentCatalog && e.config.ObservationSchema != ObservationSchemaV2 && e.config.ObservationSchema != ObservationSchemaV3,
	}, engine.DefaultFlows()...)
	if err != nil {
		return Transition{}, err
	}
	e.authority = battle.NewAuthority(
		simulationEngine,
		repo,
		e.assembler,
	)
	humanSeat, modelSeat := transcriptSeats(request.SeatModels)
	e.authority.ConfigureTranscriptBattle(battle.TranscriptBattleContext{
		Mode:         "learned_mirror",
		HumanActorID: humanSeat,
		ModelActorID: modelSeat,
		Controllers: map[string]string{
			humanSeat: "human",
			modelSeat: "learned_policy",
		},
		Metadata: map[string]any{"seat_models": cloneStringsMap(request.SeatModels)},
	})
	environmentSchema, observationSchema, actionSchema := e.SchemaVersions()
	e.replay = ReplayRecord{
		EnvironmentSchema: environmentSchema,
		ObservationSchema: observationSchema,
		ActionSchema:      actionSchema,
		BattleID:          battleID,
		Seed:              request.Seed,
		SeatModels:        cloneStringsMap(request.SeatModels),
		SeatDefinitions:   cloneStringsMap(request.SeatDefinitions),
		SeatTeams:         cloneStringsMap(request.SeatTeams),
	}
	e.metrics = EpisodeMetrics{
		BattleID:        battleID,
		Seed:            request.Seed,
		ActionFrequency: map[string]int{},
	}
	e.startedAt = time.Now()
	var seats []command.ParticipantDescriptor
	for _, id := range e.seatIDs {
		seats = append(seats, command.ParticipantDescriptor{InstanceID: id, DefinitionID: request.SeatDefinitions[id], TeamID: request.SeatTeams[id]})
	}
	payload, err := json.Marshal(command.StartBattlePayload{
		Seats: seats,
		Seed:  &request.Seed,
	})
	if err != nil {
		return Transition{}, err
	}
	started := e.authority.HandleCommand(command.Command{
		BattleID: battleID,
		ActorID:  SeatIDs[0],
		Type:     command.TypeStartBattle,
		Payload:  payload,
	})
	if !started.Accepted {
		return Transition{}, fmt.Errorf("authority rejected reset: %s", started.Error)
	}
	transition, err := e.selectNext(started, SeatIDs[0])
	if err != nil {
		return Transition{}, err
	}
	return e.transportTransition(transition)
}

func (e *Environment) Observe(seatID string) (engine.Result, error) {
	if e.authority == nil || e.replay.BattleID == "" {
		return engine.Result{}, errors.New("environment has not been reset")
	}
	if e.replay.SeatDefinitions[seatID] == "" {
		return engine.Result{}, fmt.Errorf("unknown seat %q", seatID)
	}
	result := e.authority.HandleCommand(command.Command{
		BattleID: e.replay.BattleID,
		ActorID:  seatID,
		Type:     command.TypeOpenBattle,
		Payload:  json.RawMessage(`{}`),
	})
	if !result.Accepted {
		return engine.Result{}, fmt.Errorf("open %s: %s", seatID, result.Error)
	}
	result.LegalActions = canonicalActions(result.LegalActions)
	return result, nil
}

func (e *Environment) LegalActions(seatID string) ([]command.Command, error) {
	result, err := e.Observe(seatID)
	if err != nil {
		return nil, err
	}
	return append([]command.Command(nil), result.LegalActions...), nil
}

// Step selects one of the exact commands returned by the authority. The
// original command envelope is submitted unchanged through HandleCommand.
func (e *Environment) Step(actionIndex int) (Transition, error) {
	if actionIndex < 0 || actionIndex >= len(e.legalActions) {
		e.metrics.InvalidActionIDs++
		return Transition{}, fmt.Errorf("action index %d outside legal range [0,%d)", actionIndex, len(e.legalActions))
	}
	actorID := e.legalActions[actionIndex].ActorID
	transition, _, err := e.stepForViewerWithTranscript(
		actionIndex,
		actorID,
		battle.TranscriptCommandContext{},
		e.config.TelemetryMode == TelemetryModeTraining,
	)
	if err != nil {
		return Transition{}, err
	}
	return e.transportTransition(transition)
}

// StepForViewer submits the exact indexed authority command while returning
// the applied command's events and snapshot filtered for viewerActorID. The
// next transition remains the acting seat's viewer-safe model input.
func (e *Environment) StepForViewer(actionIndex int, viewerActorID string) (Transition, engine.Result, error) {
	return e.StepForViewerWithTranscript(actionIndex, viewerActorID, battle.TranscriptCommandContext{})
}

// StepForViewerWithTranscript follows the identical indexed authority path
// while carrying diagnostics that are ignored unless transcript tooling is
// compile-time and runtime enabled.
func (e *Environment) StepForViewerWithTranscript(
	actionIndex int,
	viewerActorID string,
	transcriptContext battle.TranscriptCommandContext,
) (Transition, engine.Result, error) {
	return e.stepForViewerWithTranscript(actionIndex, viewerActorID, transcriptContext, false)
}

func (e *Environment) stepForViewerWithTranscript(
	actionIndex int,
	viewerActorID string,
	transcriptContext battle.TranscriptCommandContext,
	simulationResult bool,
) (Transition, engine.Result, error) {
	if e.current.Terminal || e.current.TruncationReason != "" {
		return Transition{}, engine.Result{}, errors.New("episode is complete; reset before stepping")
	}
	if actionIndex < 0 || actionIndex >= len(e.legalActions) {
		e.metrics.InvalidActionIDs++
		return Transition{}, engine.Result{}, fmt.Errorf("action index %d outside legal range [0,%d)", actionIndex, len(e.legalActions))
	}
	action := e.legalActions[actionIndex]
	if transcriptContext.ActionIndex == 0 && actionIndex != 0 {
		transcriptContext.ActionIndex = actionIndex
	}
	if transcriptContext.CandidateCount == 0 {
		transcriptContext.CandidateCount = len(e.legalActions)
	}
	if transcriptContext.Controller == "" {
		transcriptContext.Controller = transcriptController(e.replay.SeatModels[action.ActorID])
	}
	var result engine.Result
	if simulationResult {
		result = e.authority.HandleSimulationCommandWithTranscript(action, transcriptContext)
	} else {
		result = e.authority.HandleCommandForViewerWithTranscript(action, viewerActorID, transcriptContext)
	}
	if !result.Accepted {
		e.metrics.AuthorityRejects++
		return Transition{}, result, fmt.Errorf("implementation failure: authority rejected enumerated action %s: %s", action.Type, result.Error)
	}
	e.recordCommittedDamage(result.Events)
	e.replay.Actions = append(e.replay.Actions, ActionRecord{Index: actionIndex, ActorID: action.ActorID, Command: action})
	e.metrics.Actions++
	e.metrics.ActionFrequency[string(action.Type)]++
	if e.metrics.Actions >= e.config.MaxActions && result.Status != engine.ProgressBattleComplete {
		e.metrics.TruncationActor = action.ActorID
		return e.finish(result, "action_limit"), result, nil
	}
	transition, err := e.selectNext(result, action.ActorID)
	return transition, result, err
}

func (e *Environment) recordCommittedDamage(events []event.Event) {
	for _, battleEvent := range events {
		if battleEvent.Type != event.TypeDamageCommitted || battleEvent.Data == nil {
			continue
		}
		sources, ok := battleEvent.Data["sources"].([]state.SettledDamageSource)
		if !ok {
			continue
		}
		for _, source := range sources {
			metrics := e.damageMetricsForSeat(source.TargetActorID)
			if metrics == nil {
				continue
			}
			switch source.SourceContentID {
			case "bleed":
				metrics.RawBleed += source.BaseAmount
			case "poison":
				metrics.RawPoison += source.BaseAmount
			default:
				metrics.RawAttack += source.BaseAmount
			}
			metrics.RawTotal += source.BaseAmount
			metrics.ResolvedTotal += source.FinalAmount
		}
		removals, ok := battleEvent.Data["removals"].([]state.ProposedCardRemoval)
		if !ok {
			continue
		}
		for _, removal := range removals {
			if !removal.Accepted || removal.Released {
				continue
			}
			metrics := e.damageMetricsForSeat(removal.TargetActorID)
			if metrics == nil {
				continue
			}
			metrics.ActualTotal++
		}
	}
}

func (e *Environment) damageMetricsForSeat(seatID string) *DamageMetrics {
	switch seatID {
	case "seat-a":
		return &e.metrics.DamageBySeat.SeatA
	case "seat-b":
		return &e.metrics.DamageBySeat.SeatB
	default:
		return nil
	}
}

// StepCommandForViewer accepts only a command currently enumerated by the
// authority and then submits that original candidate unchanged.
func (e *Environment) StepCommandForViewer(action command.Command, viewerActorID string) (Transition, engine.Result, error) {
	return e.StepCommandForViewerWithTranscript(action, viewerActorID, battle.TranscriptCommandContext{})
}

func (e *Environment) StepCommandForViewerWithTranscript(
	action command.Command,
	viewerActorID string,
	transcriptContext battle.TranscriptCommandContext,
) (Transition, engine.Result, error) {
	for index, candidate := range e.legalActions {
		if commandsEqual(candidate, action) {
			return e.StepForViewerWithTranscript(index, viewerActorID, transcriptContext)
		}
	}
	e.metrics.InvalidActionIDs++
	return Transition{}, engine.Result{}, errors.New("command is not a current legal candidate")
}

func transcriptSeats(models map[string]string) (string, string) {
	human, model := "", ""
	for _, seatID := range SeatIDs {
		if transcriptController(models[seatID]) == "human" {
			human = seatID
		} else if models[seatID] != "" {
			model = seatID
		}
	}
	if human == "" {
		human = SeatIDs[0]
	}
	if model == "" || model == human {
		if human == SeatIDs[0] {
			model = SeatIDs[1]
		} else {
			model = SeatIDs[0]
		}
	}
	return human, model
}

func transcriptController(model string) string {
	if strings.Contains(strings.ToLower(model), "human") {
		return "human"
	}
	if model != "" {
		return "learned_policy"
	}
	return "external"
}

func (e *Environment) Current() Transition {
	return e.current
}

func (e *Environment) Metrics() EpisodeMetrics {
	return e.metrics
}

func (e *Environment) Replay(record ReplayRecord) (Transition, error) {
	transition, err := e.Reset(ResetRequest{
		Seed: record.Seed, BattleID: record.BattleID,
		SeatModels: record.SeatModels, SeatDefinitions: record.SeatDefinitions, SeatTeams: record.SeatTeams,
	})
	if err != nil {
		return Transition{}, err
	}
	for index, expected := range record.Actions {
		if transition.Terminal || transition.TruncationReason != "" {
			return Transition{}, fmt.Errorf("replay ended before action %d", index)
		}
		if transition.ActorID != expected.ActorID {
			return Transition{}, fmt.Errorf("replay action %d actor %q, want %q", index, transition.ActorID, expected.ActorID)
		}
		matched := -1
		for candidateIndex, candidate := range e.legalActions {
			if commandsEqual(candidate, expected.Command) {
				matched = candidateIndex
				break
			}
		}
		if matched < 0 {
			return Transition{}, fmt.Errorf("replay action %d is not currently legal", index)
		}
		transition, err = e.Step(matched)
		if err != nil {
			return Transition{}, fmt.Errorf("replay action %d: %w", index, err)
		}
	}
	if record.TruncationReason == "" && !transition.Terminal {
		return Transition{}, errors.New("replay did not reach recorded terminal state")
	}
	if transition.Winner != record.Winner {
		return Transition{}, fmt.Errorf("replay winner %q, want %q", transition.Winner, record.Winner)
	}
	return transition, nil
}

func (e *Environment) selectNext(last engine.Result, preferredActor string) (Transition, error) {
	if last.Status == engine.ProgressBattleComplete || (last.Snapshot != nil && state.IsTerminalBattleStatus(last.Snapshot.Status)) {
		return e.finish(last, ""), nil
	}
	order := nextSeatOrder(last.Snapshot, preferredActor)
	for _, seatID := range order {
		view, err := e.Observe(seatID)
		if err != nil {
			return Transition{}, err
		}
		if len(view.LegalActions) == 0 {
			continue
		}
		e.legalActions = append([]command.Command(nil), view.LegalActions...)
		e.current = Transition{ActorID: seatID, Result: e.compactResult(view), Metrics: e.metrics}
		return e.current, nil
	}
	return Transition{}, errors.New("active battle has no legal action for either external seat")
}

func (e *Environment) finish(result engine.Result, truncation string) Transition {
	e.legalActions = nil
	e.metrics.TruncationReason = truncation
	e.replay.TruncationReason = truncation
	if result.Snapshot != nil {
		e.metrics.Rounds = result.Snapshot.CompletedRounds
		e.metrics.Winner = result.Snapshot.WinnerActorID
		e.metrics.Status = result.Snapshot.Status
		e.metrics.RemainingHealth = map[string]int{}
		for _, seatID := range e.seatIDs {
			e.metrics.RemainingHealth[seatID] = result.Snapshot.Actors[seatID].CurrentHealth
		}
		e.replay.Winner = result.Snapshot.WinnerActorID
		e.replay.Status = result.Snapshot.Status
	}
	e.metrics.DurationMillis = float64(time.Since(e.startedAt).Microseconds()) / 1000
	if cursor, err := e.authority.InspectBattleRandomCursor(e.replay.BattleID); err == nil {
		e.metrics.RandomCursor = cursor
	}
	replay := e.replay
	e.current = Transition{
		Result:           e.compactResult(result),
		Terminal:         truncation == "",
		Winner:           e.metrics.Winner,
		TruncationReason: truncation,
		Metrics:          e.metrics,
		Replay:           &replay,
	}
	return e.current
}

func canonicalActions(actions []command.Command) []command.Command {
	type keyedAction struct {
		command command.Command
		key     string
	}
	keyed := make([]keyedAction, len(actions))
	for index, action := range actions {
		encoded, _ := json.Marshal(action)
		keyed[index] = keyedAction{command: action, key: string(encoded)}
	}
	sort.SliceStable(keyed, func(i, j int) bool {
		return keyed[i].key < keyed[j].key
	})
	result := make([]command.Command, len(keyed))
	for index := range keyed {
		result[index] = keyed[index].command
	}
	return result
}

func nextSeatOrder(snap *snapshot.Battle, preferred string) []string {
	ids := append([]string(nil), SeatIDs...)
	if snap != nil && len(snap.Actors) > 0 {
		ids = nil
		for id := range snap.Actors {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}
	first := preferred
	if snap != nil && snap.PriorityActorID != "" {
		first = snap.PriorityActorID
	}
	for i, id := range ids {
		if id == first {
			return append([]string{id}, append(ids[:i], ids[i+1:]...)...)
		}
	}
	return ids
}

func otherSeat(seatID string) string {
	if seatID == SeatIDs[0] {
		return SeatIDs[1]
	}
	return SeatIDs[0]
}

func validSeat(seatID string) bool {
	return seatID == SeatIDs[0] || seatID == SeatIDs[1]
}

func (e *Environment) compactResult(result engine.Result) engine.Result {
	result.Events = nil
	if result.Snapshot != nil && !e.config.IncludeContentCatalog && e.config.ObservationSchema != ObservationSchemaV2 && e.config.ObservationSchema != ObservationSchemaV3 {
		result.Snapshot.ContentCatalog = nil
	}
	return result
}

func (e *Environment) transportTransition(transition Transition) (Transition, error) {
	if e.config.TransportMode == TransportModeFull || transition.Terminal || transition.TruncationReason != "" {
		return transition, nil
	}
	var encoded EncodedDecision
	var err error
	if e.config.ObservationSchema == ObservationSchemaV2 {
		encoded, err = encodeDecisionV2(transition)
	} else if e.config.ObservationSchema == ObservationSchemaV3 {
		encoded, err = encodeDecisionV3(transition, e.manifestV3)
	} else {
		encoded, err = encodeDecision(transition)
	}
	if err != nil {
		return Transition{}, err
	}
	if e.config.TransportMode == TransportModeEncoded {
		transition.Result = engine.Result{}
	}
	transition.EncodedDecision = &encoded
	return transition, nil
}

func (e *Environment) SchemaVersions() (string, string, string) {
	if e.config.ObservationSchema == ObservationSchemaV2 {
		return EnvironmentSchemaV2, ObservationSchemaV2, ActionSchemaV2
	}
	if e.config.ObservationSchema == ObservationSchemaV3 {
		return EnvironmentSchemaV3, ObservationSchemaV3, ActionSchemaV3
	}
	return EnvironmentSchemaVersion, ObservationSchemaVersion, ActionSchemaVersion
}

func commandsEqual(left, right command.Command) bool {
	if left.BattleID != right.BattleID || left.ActorID != right.ActorID || left.Type != right.Type {
		return false
	}
	var leftPayload, rightPayload any
	if json.Unmarshal(left.Payload, &leftPayload) != nil || json.Unmarshal(right.Payload, &rightPayload) != nil {
		return false
	}
	return reflect.DeepEqual(leftPayload, rightPayload)
}

func cloneStringsMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
