package battle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/scenario"
)

type ScenarioAuthorityConfig struct {
	BuildEnabled   bool
	RuntimeEnabled bool
	Catalog        scenario.Catalog
	Builder        scenario.Builder
	Engine         engine.Engine
	Repository     repository.Repository
	Gameplay       *Authority
	IDGenerator    func(string) (string, error)
}

type ScenarioAuthority struct {
	config ScenarioAuthorityConfig
}

type scenarioRequestPayload struct {
	ScenarioID string         `json:"scenario_id,omitempty"`
	Spec       *scenario.Spec `json:"spec,omitempty"`
}

func NewScenarioAuthority(config ScenarioAuthorityConfig) *ScenarioAuthority {
	if config.IDGenerator == nil {
		config.IDGenerator = generateScenarioBattleID
	}
	return &ScenarioAuthority{config: config}
}

func newDefaultScenarioAuthority(gameplay *Authority) *ScenarioAuthority {
	_, sourceFile, _, ok := runtime.Caller(0)
	serverRoot := ""
	if ok {
		serverRoot = filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	}
	scenarioRoot := os.Getenv("DICE_AND_DESTINY_SCENARIO_ROOT")
	if scenarioRoot == "" {
		scenarioRoot = filepath.Join(serverRoot, "scenarios")
	}
	return NewScenarioAuthority(ScenarioAuthorityConfig{
		BuildEnabled:   scenarioToolsBuildEnabled,
		RuntimeEnabled: os.Getenv("DICE_AND_DESTINY_ENABLE_SCENARIOS") == "1",
		Catalog:        scenario.Catalog{Root: scenarioRoot},
		Builder:        scenario.Builder{Assembler: gameplay.assembler},
		Engine:         gameplay.engine,
		Repository:     gameplay.repo,
		Gameplay:       gameplay,
	})
}

func (authority *ScenarioAuthority) HandleCommandJSON(commandJSON string) string {
	return handleCommand(commandJSON, authority)
}

func (authority *ScenarioAuthority) HandleCommand(cmd command.Command) engine.Result {
	if authority == nil || authority.config.Gameplay == nil {
		return authorityRejected("scenario authority is not configured")
	}
	switch cmd.Type {
	case command.TypeListScenarios, command.TypeValidateScenario, command.TypeStartScenario:
		if !authority.config.BuildEnabled || !authority.config.RuntimeEnabled {
			return authorityRejected("scenario tooling is disabled")
		}
	}
	switch cmd.Type {
	case command.TypeListScenarios:
		return authority.listScenarios()
	case command.TypeValidateScenario:
		return authority.validateScenario(cmd)
	case command.TypeStartScenario:
		return authority.startScenario(cmd)
	default:
		return authority.config.Gameplay.HandleCommand(cmd)
	}
}

func (authority *ScenarioAuthority) listScenarios() engine.Result {
	entries, err := authority.config.Catalog.List()
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{Accepted: true, Data: map[string]any{"scenarios": entries}}
}

func (authority *ScenarioAuthority) validateScenario(cmd command.Command) engine.Result {
	spec, err := authority.requestSpec(cmd)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if _, err := authority.config.Builder.Build(spec); err != nil {
		return authorityRejected(err.Error())
	}
	fingerprint, err := scenario.Fingerprint(spec)
	if err != nil {
		return authorityRejected(err.Error())
	}
	return engine.Result{
		Accepted: true,
		Data: map[string]any{
			"valid":       true,
			"scenario_id": spec.Metadata.ID,
			"fingerprint": fingerprint,
		},
	}
}

func (authority *ScenarioAuthority) startScenario(cmd command.Command) engine.Result {
	if authority.config.Repository == nil {
		return authorityRejected("scenario repository is not configured")
	}
	spec, err := authority.requestSpec(cmd)
	if err != nil {
		return authorityRejected(err.Error())
	}
	if cmd.ActorID != spec.Player.InstanceID {
		return authorityRejected("start_scenario actor_id must match player instance_id")
	}
	battleID, err := authority.config.IDGenerator(spec.Metadata.ID)
	if err != nil {
		return authorityRejected(fmt.Sprintf("generate scenario battle id: %v", err))
	}
	if err := repository.ValidateBattleID(battleID); err != nil {
		return authorityRejected(fmt.Sprintf("generate scenario battle id: %v", err))
	}
	if len(battleID) < len("scenario-") || battleID[:len("scenario-")] != "scenario-" {
		return authorityRejected("generated scenario battle id must use scenario- prefix")
	}
	spec.BattleID = battleID
	battleState, progressed, err := authority.config.Builder.BuildAndProgress(spec, authority.config.Engine)
	if err != nil {
		return authorityRejected(err.Error())
	}
	checkpoint, err := repository.NewCheckpoint(battleState)
	if err != nil {
		return authorityRejected(fmt.Sprintf("create scenario checkpoint: %v", err))
	}
	assigned, err := repository.AppendEvents(&checkpoint, progressed.Events)
	if err != nil {
		return authorityRejected(fmt.Sprintf("sequence scenario events: %v", err))
	}
	progressed.Events = assigned
	if err := authority.config.Repository.Create(checkpoint); err != nil {
		if errors.Is(err, repository.ErrBattleExists) {
			return authorityRejected("battle already exists")
		}
		return authorityRejected(fmt.Sprintf("create scenario battle: %v", err))
	}
	return authority.config.Engine.ResultForViewer(&battleState, cmd.ActorID, progressed)
}

func (authority *ScenarioAuthority) requestSpec(cmd command.Command) (scenario.Spec, error) {
	var payload scenarioRequestPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return scenario.Spec{}, errors.New("invalid scenario payload")
	}
	if payload.ScenarioID != "" && payload.Spec != nil {
		return scenario.Spec{}, errors.New("provide scenario_id or spec, not both")
	}
	if payload.ScenarioID != "" {
		return authority.config.Catalog.Load(payload.ScenarioID)
	}
	if payload.Spec == nil {
		return scenario.Spec{}, errors.New("scenario_id or spec is required")
	}
	if err := scenario.ValidateSpec(*payload.Spec); err != nil {
		return scenario.Spec{}, err
	}
	return *payload.Spec, nil
}

func generateScenarioBattleID(scenarioID string) (string, error) {
	if scenarioID == "" {
		scenarioID = "inline"
	}
	if err := scenario.ValidateScenarioID(scenarioID); err != nil {
		return "", err
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "scenario-" + scenarioID + "-" + hex.EncodeToString(suffix[:]), nil
}
