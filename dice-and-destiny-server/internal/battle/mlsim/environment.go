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
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

const (
	EnvironmentSchemaVersion = "dice-and-destiny-ml-env-v1"
	ObservationSchemaVersion = "dice-and-destiny-observation-v1"
	ActionSchemaVersion      = "dice-and-destiny-action-candidates-v1"
	DefaultMaxActions        = 1200
)

var SeatIDs = []string{"seat-a", "seat-b"}

type Config struct {
	ContentRoot  string
	RunStateRoot string
	MaxActions   int
	SessionID    string
}

type ResetRequest struct {
	Seed       uint64            `json:"seed"`
	BattleID   string            `json:"battle_id,omitempty"`
	SeatModels map[string]string `json:"seat_models,omitempty"`
}

type ActionRecord struct {
	Index   int             `json:"action_index"`
	ActorID string          `json:"actor_id"`
	Command command.Command `json:"command"`
}

type ReplayRecord struct {
	EnvironmentSchema string             `json:"environment_schema"`
	ObservationSchema string             `json:"observation_schema"`
	ActionSchema      string             `json:"action_schema"`
	BattleID          string             `json:"battle_id"`
	Seed              uint64             `json:"seed"`
	SeatModels        map[string]string  `json:"seat_models"`
	Actions           []ActionRecord     `json:"actions"`
	Winner            string             `json:"winner,omitempty"`
	Status            state.BattleStatus `json:"status,omitempty"`
	TruncationReason  string             `json:"truncation_reason,omitempty"`
}

type EpisodeMetrics struct {
	BattleID         string             `json:"battle_id"`
	Seed             uint64             `json:"seed"`
	Actions          int                `json:"actions"`
	Rounds           int                `json:"rounds"`
	Winner           string             `json:"winner,omitempty"`
	Status           state.BattleStatus `json:"status,omitempty"`
	TruncationReason string             `json:"truncation_reason,omitempty"`
	TruncationActor  string             `json:"truncation_actor,omitempty"`
	ActionFrequency  map[string]int     `json:"action_frequency"`
	RemainingHealth  map[string]int     `json:"remaining_health,omitempty"`
	AuthorityRejects int                `json:"authority_rejects"`
	InvalidActionIDs int                `json:"invalid_action_indices"`
	StaleActions     int                `json:"stale_action_submissions"`
	WrongSeatActions int                `json:"wrong_seat_submissions"`
	DurationMillis   float64            `json:"duration_ms"`
}

type Transition struct {
	ActorID          string         `json:"actor_id,omitempty"`
	Result           engine.Result  `json:"result"`
	Terminal         bool           `json:"terminal"`
	Winner           string         `json:"winner,omitempty"`
	TruncationReason string         `json:"truncation_reason,omitempty"`
	Metrics          EpisodeMetrics `json:"metrics"`
	Replay           *ReplayRecord  `json:"replay,omitempty"`
}

type Environment struct {
	config       Config
	authority    *battle.Authority
	current      Transition
	legalActions []command.Command
	replay       ReplayRecord
	metrics      EpisodeMetrics
	episode      uint64
	startedAt    time.Time
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
	return &Environment{config: config}, nil
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
	e.authority = battle.NewAuthority(
		engine.NewEngine(),
		repository.NewInMemory(),
		battle.NewFileParticipantAssembler(e.config.ContentRoot, e.config.RunStateRoot),
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
	e.replay = ReplayRecord{
		EnvironmentSchema: EnvironmentSchemaVersion,
		ObservationSchema: ObservationSchemaVersion,
		ActionSchema:      ActionSchemaVersion,
		BattleID:          battleID,
		Seed:              request.Seed,
		SeatModels:        cloneStringsMap(request.SeatModels),
	}
	e.metrics = EpisodeMetrics{
		BattleID:        battleID,
		Seed:            request.Seed,
		ActionFrequency: map[string]int{},
	}
	e.startedAt = time.Now()
	payload, err := json.Marshal(command.StartBattlePayload{
		Seats: []command.ParticipantDescriptor{
			{InstanceID: SeatIDs[0], DefinitionID: "blade_warden"},
			{InstanceID: SeatIDs[1], DefinitionID: "blade_warden"},
		},
		Seed: &request.Seed,
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
	return e.selectNext(started, SeatIDs[0])
}

func (e *Environment) Observe(seatID string) (engine.Result, error) {
	if e.authority == nil || e.replay.BattleID == "" {
		return engine.Result{}, errors.New("environment has not been reset")
	}
	if !validSeat(seatID) {
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
	transition, _, err := e.StepForViewer(actionIndex, actorID)
	return transition, err
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
	result := e.authority.HandleCommandForViewerWithTranscript(action, viewerActorID, transcriptContext)
	if !result.Accepted {
		e.metrics.AuthorityRejects++
		return Transition{}, result, fmt.Errorf("implementation failure: authority rejected enumerated action %s: %s", action.Type, result.Error)
	}
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
	transition, err := e.Reset(ResetRequest{Seed: record.Seed, BattleID: record.BattleID, SeatModels: record.SeatModels})
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
		e.current = Transition{ActorID: seatID, Result: compactResult(view), Metrics: e.metrics}
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
		for _, seatID := range SeatIDs {
			e.metrics.RemainingHealth[seatID] = result.Snapshot.Actors[seatID].CurrentHealth
		}
		e.replay.Winner = result.Snapshot.WinnerActorID
		e.replay.Status = result.Snapshot.Status
	}
	e.metrics.DurationMillis = float64(time.Since(e.startedAt).Microseconds()) / 1000
	replay := e.replay
	e.current = Transition{
		Result:           compactResult(result),
		Terminal:         truncation == "",
		Winner:           e.metrics.Winner,
		TruncationReason: truncation,
		Metrics:          e.metrics,
		Replay:           &replay,
	}
	return e.current
}

func canonicalActions(actions []command.Command) []command.Command {
	result := append([]command.Command(nil), actions...)
	sort.SliceStable(result, func(i, j int) bool {
		left, _ := json.Marshal(result[i])
		right, _ := json.Marshal(result[j])
		return string(left) < string(right)
	})
	return result
}

func nextSeatOrder(snap *snapshot.Battle, preferred string) []string {
	if snap != nil && validSeat(snap.PriorityActorID) {
		return []string{snap.PriorityActorID, otherSeat(snap.PriorityActorID)}
	}
	if validSeat(preferred) {
		return []string{preferred, otherSeat(preferred)}
	}
	return append([]string(nil), SeatIDs...)
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

func compactResult(result engine.Result) engine.Result {
	result.Events = nil
	if result.Snapshot != nil {
		result.Snapshot.ContentCatalog = nil
	}
	return result
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
