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
	engine    engine.Engine
	repo      repository.Repository
	assembler ParticipantAssembler
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
		engine:    battleEngine,
		repo:      repo,
		assembler: assembler,
	}
}

// HandleCommand is the portable battle authority JSON boundary.
func HandleCommand(commandJSON string) string {
	return newDefaultScenarioAuthority(defaultAuthority).HandleCommandJSON(commandJSON)
}

func (authority *Authority) HandleCommandJSON(commandJSON string) string {
	return handleCommand(commandJSON, authority)
}

func handleCommand(commandJSON string, handler commandHandler) string {
	// Authority owns transport concerns: parse JSON, delegate the typed command,
	// then serialize the engine result. Gameplay meaning stays below this layer.
	cmd, err := command.ParseJSON(commandJSON)
	if err != nil {
		return marshalResult(parseErrorResult(err))
	}

	return marshalResult(handler.HandleCommand(cmd))
}

func (authority *Authority) HandleCommand(cmd command.Command) engine.Result {
	if authority == nil || authority.repo == nil || authority.assembler == nil {
		return authorityRejected("battle authority is not configured")
	}
	if cmd.Type == command.TypeStartBattle {
		return authority.startBattle(cmd)
	}

	checkpoint, err := authority.repo.Load(cmd.BattleID)
	if err != nil {
		if errors.Is(err, repository.ErrBattleNotFound) {
			return authorityRejected("battle not found")
		}
		return authorityRejected(fmt.Sprintf("load battle: %v", err))
	}
	if cmd.Type == command.TypeOpenBattle {
		return authority.engine.OpenResult(&checkpoint.Battle, cmd.ActorID)
	}

	progressed, err := authority.engine.ApplyBattleCommand(&checkpoint.Battle, cmd)
	if err != nil {
		return authorityRejected(err.Error())
	}
	assigned, err := repository.AppendEvents(&checkpoint, progressed.Events)
	if err != nil {
		return authorityRejected(fmt.Sprintf("sequence battle events: %v", err))
	}
	progressed.Events = assigned
	if err := authority.repo.Save(checkpoint); err != nil {
		return authorityRejected(fmt.Sprintf("save battle: %v", err))
	}
	return authority.engine.ResultForViewer(&checkpoint.Battle, cmd.ActorID, progressed)
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
