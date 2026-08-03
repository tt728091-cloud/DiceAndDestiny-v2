package battle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

type commandHandler interface {
	HandleCommand(cmd command.Command) engine.Result
}

type Participant = participant.Participant
type ParticipantAssembler = participant.Assembler
type ParticipantAssemblerFunc = participant.AssemblerFunc

type Authority struct {
	engine     engine.Engine
	repo       repository.Repository
	assembler  ParticipantAssembler
	transcript any
}

// TranscriptBattleContext is diagnostic-only metadata for an authority
// transcript. It is never consulted by gameplay, legal-action, RNG, or viewer
// filtering code.
type TranscriptBattleContext struct {
	Mode         string
	HumanActorID string
	ModelActorID string
	Controllers  map[string]string
	Metadata     map[string]any
}

// TranscriptCommandContext identifies the controller that selected one
// accepted authority command. Learned-policy callers add candidate and
// inference metadata here without changing the submitted command envelope.
type TranscriptCommandContext struct {
	Controller     string
	ActionIndex    int
	CandidateCount int
	InferenceMS    float64
	Metadata       map[string]any
}

var defaultAuthority = newDefaultAuthority()

func newDefaultAuthority() *Authority {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return NewAuthority(
			engine.NewEngine(),
			repository.NewDisk(filepath.Join(os.TempDir(), "dice-and-destiny", "battles")),
			ParticipantAssemblerFunc(func([]Participant) (state.BattleSetup, error) {
				return state.BattleSetup{}, errors.New("could not locate server content")
			}),
		)
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	contentRoot := os.Getenv("DICE_AND_DESTINY_CONTENT_ROOT")
	if contentRoot == "" {
		contentRoot = filepath.Join(serverRoot, "content")
	}
	runStateRoot := os.Getenv("DICE_AND_DESTINY_RUN_STATE_ROOT")
	if runStateRoot == "" {
		runStateRoot = filepath.Join(serverRoot, "save", "run_players")
	}
	battleStateRoot := os.Getenv("DICE_AND_DESTINY_BATTLE_STATE_ROOT")
	if battleStateRoot == "" {
		battleStateRoot = filepath.Join(serverRoot, "save", "battles")
	}
	scenarioStateRoot := os.Getenv("DICE_AND_DESTINY_SCENARIO_STATE_ROOT")
	if scenarioStateRoot == "" {
		scenarioStateRoot = filepath.Join(serverRoot, "save", "scenarios")
	}
	return NewAuthority(
		engine.NewEngine(),
		repository.Router{
			Normal:   repository.NewDisk(battleStateRoot),
			Scenario: repository.NewDisk(scenarioStateRoot),
		},
		NewFileParticipantAssembler(contentRoot, runStateRoot),
	)
}

func NewAuthority(
	battleEngine engine.Engine,
	repo repository.Repository,
	assembler ParticipantAssembler,
) *Authority {
	return &Authority{
		engine:     battleEngine,
		repo:       repo,
		assembler:  assembler,
		transcript: newAuthorityTranscript(),
	}
}

// HandleCommand is the portable battle authority JSON boundary.
func HandleCommand(commandJSON string) string {
	scenarioAuthority := newDefaultScenarioAuthority(defaultAuthority)
	snapshotAuthority := newDefaultSnapshotAuthority(scenarioAuthority, defaultAuthority)
	return newDefaultHistoryAuthority(snapshotAuthority, defaultAuthority).HandleCommandJSON(commandJSON)
}

func (authority *Authority) HandleCommandJSON(commandJSON string) string {
	return handleCommand(commandJSON, authority)
}

func handleCommand(commandJSON string, handler commandHandler) string {
	// Authority owns transport concerns: parse JSON, delegate the typed command,
	// then serialize the engine result. Gameplay meaning stays below this layer.
	cmd, err := command.ParseJSON(commandJSON)
	if err != nil {
		recordAuthorityTranscriptTransportRejection(commandJSON, err)
		return marshalResult(parseErrorResult(err))
	}

	return marshalResult(handler.HandleCommand(cmd))
}

func (authority *Authority) HandleCommand(cmd command.Command) engine.Result {
	return authority.handleCommandForViewer(cmd, cmd.ActorID, TranscriptCommandContext{})
}

// HandleCommandForViewer applies cmd through the ordinary authority boundary
// while filtering the returned events and snapshot for resultViewerActorID.
// The command actor still owns validation; choosing a different result viewer
// cannot grant that viewer permission to act.
func (authority *Authority) HandleCommandForViewer(cmd command.Command, resultViewerActorID string) engine.Result {
	return authority.handleCommandForViewer(cmd, resultViewerActorID, TranscriptCommandContext{})
}

// HandleCommandForViewerWithTranscript applies the same command path as
// HandleCommandForViewer while attaching diagnostics that are consumed only
// when the compile-time and runtime transcript gates are enabled.
func (authority *Authority) HandleCommandForViewerWithTranscript(
	cmd command.Command,
	resultViewerActorID string,
	context TranscriptCommandContext,
) engine.Result {
	return authority.handleCommandForViewer(cmd, resultViewerActorID, context)
}

// HandleSimulationCommand applies the identical authority command path while
// avoiding construction of a result that a training transport immediately
// discards. Accepted events are still sequenced and persisted; terminal
// commands still return the complete viewer-safe result.
func (authority *Authority) HandleSimulationCommand(cmd command.Command) engine.Result {
	return authority.HandleSimulationCommandWithTranscript(cmd, TranscriptCommandContext{})
}

// HandleSimulationCommandWithTranscript preserves the optional diagnostic
// context while using the compact nonterminal result path.
func (authority *Authority) HandleSimulationCommandWithTranscript(
	cmd command.Command,
	context TranscriptCommandContext,
) engine.Result {
	return authority.handleCommandForViewerMode(cmd, cmd.ActorID, context, true)
}

// ConfigureTranscriptBattle attaches diagnostic seat/controller labels to
// this authority. The disabled release implementation is a no-op.
func (authority *Authority) ConfigureTranscriptBattle(context TranscriptBattleContext) {
	configureAuthorityTranscript(authority, context)
}

// InspectBattleState returns an isolated diagnostic copy of the current
// authority-owned state. Gameplay callers should consume viewer-filtered
// results instead; throughput parity tests use this hook to compare hidden
// state and deterministic RNG cursors without granting mutation access.
func (authority *Authority) InspectBattleState(battleID string) (state.Battle, error) {
	if authority == nil || authority.repo == nil {
		return state.Battle{}, errors.New("battle authority is not configured")
	}
	checkpoint, err := authority.repo.Load(battleID)
	if err != nil {
		return state.Battle{}, err
	}
	return checkpoint.Battle.Clone(), nil
}

// InspectBattleRandomCursor reads the terminal deterministic cursor without
// cloning the full battle. It is diagnostic metadata, never gameplay input.
func (authority *Authority) InspectBattleRandomCursor(battleID string) (uint64, error) {
	if authority == nil || authority.repo == nil {
		return 0, errors.New("battle authority is not configured")
	}
	checkpoint, err := authority.repo.Load(battleID)
	if err != nil {
		return 0, err
	}
	return checkpoint.Battle.Random.Cursor, nil
}

// InspectBattleEventsJSON returns the exact persisted authority event stream
// for parity diagnostics without exposing mutable repository storage.
func (authority *Authority) InspectBattleEventsJSON(battleID string) ([]byte, error) {
	if authority == nil || authority.repo == nil {
		return nil, errors.New("battle authority is not configured")
	}
	checkpoint, err := authority.repo.Load(battleID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(checkpoint.Events)
}

func (authority *Authority) handleCommandForViewer(
	cmd command.Command,
	resultViewerActorID string,
	context TranscriptCommandContext,
) engine.Result {
	return authority.handleCommandForViewerMode(cmd, resultViewerActorID, context, false)
}

func (authority *Authority) handleCommandForViewerMode(
	cmd command.Command,
	resultViewerActorID string,
	context TranscriptCommandContext,
	simulationResult bool,
) engine.Result {
	if authority == nil || authority.repo == nil || authority.assembler == nil {
		result := authorityRejected("battle authority is not configured")
		recordAuthorityTranscriptRejected(authority, cmd, context, nil, result.Error)
		return result
	}
	if cmd.Type == command.TypeStartBattle {
		beginAuthorityTranscriptCommand(authority, cmd, context, nil)
		result := authority.startBattle(cmd)
		if !result.Accepted {
			recordAuthorityTranscriptRejected(authority, cmd, context, nil, result.Error)
			return result
		}
		checkpoint, err := authority.repo.Load(cmd.BattleID)
		if err != nil {
			recordAuthorityTranscriptRejected(authority, cmd, context, nil, fmt.Sprintf("reload started battle: %v", err))
			return result
		}
		recordAuthorityTranscriptAccepted(authority, cmd, context, nil, &checkpoint.Battle, checkpoint.Events)
		return result
	}

	checkpoint, err := authority.repo.Load(cmd.BattleID)
	if err != nil {
		if errors.Is(err, repository.ErrBattleNotFound) {
			result := authorityRejected("battle not found")
			recordAuthorityTranscriptRejected(authority, cmd, context, nil, result.Error)
			return result
		}
		result := authorityRejected(fmt.Sprintf("load battle: %v", err))
		recordAuthorityTranscriptRejected(authority, cmd, context, nil, result.Error)
		return result
	}
	if cmd.Type == command.TypeOpenBattle {
		return authority.engine.OpenResult(&checkpoint.Battle, cmd.ActorID)
	}
	before := captureAuthorityTranscriptBefore(authority, &checkpoint.Battle)
	beginAuthorityTranscriptCommand(authority, cmd, context, &checkpoint.Battle)

	progressed, err := authority.engine.ApplyBattleCommand(&checkpoint.Battle, cmd)
	if err != nil {
		result := authorityRejected(err.Error())
		recordAuthorityTranscriptRejected(authority, cmd, context, before, result.Error)
		return result
	}
	assigned, err := repository.AppendEvents(&checkpoint, progressed.Events)
	if err != nil {
		result := authorityRejected(fmt.Sprintf("sequence battle events: %v", err))
		recordAuthorityTranscriptRejected(authority, cmd, context, before, result.Error)
		return result
	}
	progressed.Events = assigned
	if err := authority.repo.Save(checkpoint); err != nil {
		result := authorityRejected(fmt.Sprintf("save battle: %v", err))
		recordAuthorityTranscriptRejected(authority, cmd, context, before, result.Error)
		return result
	}
	recordAuthorityTranscriptAccepted(authority, cmd, context, before, &checkpoint.Battle, assigned)
	if simulationResult && progressed.Status != engine.ProgressBattleComplete && !state.IsTerminalBattleStatus(checkpoint.Battle.Status) {
		return authority.engine.SimulationResult(&checkpoint.Battle, progressed)
	}
	return authority.engine.ResultForViewer(&checkpoint.Battle, resultViewerActorID, progressed)
}

func (authority *Authority) startBattle(cmd command.Command) engine.Result {
	var payload command.StartBattlePayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return authorityRejected("invalid start_battle payload")
	}

	participants, err := validateStartBattle(cmd, payload)
	if err != nil {
		return authorityRejected(err.Error())
	}
	battleSetup, err := authority.assembler.AssembleParticipants(participants)
	if err != nil {
		return authorityRejected(fmt.Sprintf("assemble battle participants: %v", err))
	}
	if err := applyParticipantDescriptors(&battleSetup, participants); err != nil {
		return authorityRejected(err.Error())
	}

	battleState, err := state.NewBattleFromSetup(cmd.BattleID, battleSetup)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if payload.Seed != nil {
		battleState.Random = state.RandomState{
			Mode:      state.RandomModeReproducible,
			Algorithm: state.RandomAlgorithmSHA256,
			Seed:      *payload.Seed,
		}
	}
	progressed, err := authority.engine.ProgressUntilInput(&battleState)
	if err != nil {
		return authorityRejected(err.Error())
	}

	checkpoint, err := repository.NewCheckpoint(battleState)
	if err != nil {
		return authorityRejected(fmt.Sprintf("create battle checkpoint: %v", err))
	}
	assigned, err := repository.AppendEvents(&checkpoint, progressed.Events)
	if err != nil {
		return authorityRejected(fmt.Sprintf("sequence battle events: %v", err))
	}
	progressed.Events = assigned
	if err := authority.repo.Create(checkpoint); err != nil {
		if errors.Is(err, repository.ErrBattleExists) {
			return authorityRejected("battle already exists")
		}
		return authorityRejected(fmt.Sprintf("create battle: %v", err))
	}
	return authority.engine.ResultForViewer(&battleState, cmd.ActorID, progressed)
}

func validateStartBattle(
	cmd command.Command,
	payload command.StartBattlePayload,
) ([]Participant, error) {
	if strings.HasPrefix(cmd.BattleID, "scenario-") {
		return nil, errors.New("start_battle cannot use the reserved scenario battle namespace")
	}
	if len(payload.Seats) > 0 {
		if payload.Player.InstanceID != "" || payload.Player.DefinitionID != "" || len(payload.Enemies) != 0 {
			return nil, errors.New("external seats cannot be combined with player or enemies")
		}
		if len(payload.Seats) != 2 {
			return nil, errors.New("external battle requires exactly two seats")
		}
		participants := make([]Participant, 0, len(payload.Seats))
		seen := make(map[string]struct{}, len(payload.Seats))
		for _, seat := range payload.Seats {
			if seat.InstanceID == "" {
				return nil, errors.New("seat instance_id is required")
			}
			if seat.DefinitionID == "" {
				return nil, errors.New("seat definition_id is required")
			}
			if _, exists := seen[seat.InstanceID]; exists {
				return nil, fmt.Errorf("duplicate participant instance_id %q", seat.InstanceID)
			}
			seen[seat.InstanceID] = struct{}{}
			participants = append(participants, Participant{
				InstanceID:   seat.InstanceID,
				DefinitionID: seat.DefinitionID,
				Controller:   state.ControllerExternal,
				Source:       participant.SourceCharacterDefinition,
			})
		}
		if _, ok := seen[cmd.ActorID]; !ok {
			return nil, errors.New("start_battle actor_id must match an external seat instance_id")
		}
		return participants, nil
	}
	if payload.Player.InstanceID == "" {
		return nil, errors.New("player instance_id is required")
	}
	if payload.Player.DefinitionID == "" {
		return nil, errors.New("player definition_id is required")
	}
	if cmd.ActorID != payload.Player.InstanceID {
		return nil, errors.New("start_battle actor_id must match player instance_id")
	}
	if len(payload.Enemies) == 0 {
		return nil, errors.New("at least one enemy is required")
	}

	participants := make([]Participant, 0, 1+len(payload.Enemies))
	participants = append(participants, Participant{
		InstanceID:   payload.Player.InstanceID,
		DefinitionID: payload.Player.DefinitionID,
		Controller:   state.ControllerHuman,
		Source:       participant.SourceRunPlayer,
	})
	seen := map[string]struct{}{payload.Player.InstanceID: {}}
	for _, enemy := range payload.Enemies {
		if enemy.InstanceID == "" {
			return nil, errors.New("enemy instance_id is required")
		}
		if enemy.DefinitionID == "" {
			return nil, errors.New("enemy definition_id is required")
		}
		if _, exists := seen[enemy.InstanceID]; exists {
			return nil, fmt.Errorf("duplicate participant instance_id %q", enemy.InstanceID)
		}
		seen[enemy.InstanceID] = struct{}{}
		participants = append(participants, Participant{
			InstanceID:   enemy.InstanceID,
			DefinitionID: enemy.DefinitionID,
			Controller:   state.ControllerAI,
			Source:       participant.SourceEnemyDefinition,
		})
	}
	return participants, nil
}

func applyParticipantDescriptors(
	battleSetup *state.BattleSetup,
	participants []Participant,
) error {
	if battleSetup == nil {
		return errors.New("participant assembler returned nil setup")
	}
	byID := make(map[string]Participant, len(participants))
	for _, participant := range participants {
		byID[participant.InstanceID] = participant
	}
	if len(battleSetup.Actors) != len(participants) {
		return errors.New("participant assembler returned the wrong actor count")
	}
	for i := range battleSetup.Actors {
		actor := &battleSetup.Actors[i]
		participant, ok := byID[actor.ID]
		if !ok {
			return fmt.Errorf("participant assembler returned unexpected actor %q", actor.ID)
		}
		actor.DefinitionID = participant.DefinitionID
		actor.ControllerType = participant.Controller
		delete(byID, actor.ID)
	}
	if len(byID) != 0 {
		return errors.New("participant assembler omitted a requested actor")
	}
	return nil
}

func parseErrorResult(err error) engine.Result {
	switch {
	case errors.Is(err, command.ErrInvalidJSON):
		return engine.Result{
			Accepted: false,
			Error:    "invalid command JSON",
		}
	default:
		return engine.Result{
			Accepted: false,
			Error:    "invalid command envelope",
		}
	}
}

func marshalResult(r engine.Result) string {
	payload, err := json.Marshal(r)
	if err != nil {
		return `{"accepted":false,"error":"result serialization failed"}`
	}
	return string(payload)
}

func authorityRejected(message string) engine.Result {
	return engine.Result{
		Accepted: false,
		Error:    message,
	}
}
