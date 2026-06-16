package scenario

import (
	"encoding/json"
	"errors"
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

type Builder struct {
	Assembler participant.Assembler
}

func (builder Builder) Build(spec Spec) (state.Battle, error) {
	if builder.Assembler == nil {
		return state.Battle{}, errors.New("scenario participant assembler is required")
	}
	if err := ValidateSpec(spec); err != nil {
		return state.Battle{}, err
	}
	setup, err := builder.Assembler.AssembleParticipants(spec.Participants())
	if err != nil {
		return state.Battle{}, fmt.Errorf("assemble scenario participants: %w", err)
	}
	if err := applyDescriptors(&setup, spec.Participants()); err != nil {
		return state.Battle{}, err
	}
	battleID := spec.BattleID
	if battleID == "" {
		battleID = "scenario-build"
	}
	battle, err := state.NewBattleFromSetup(battleID, setup)
	if err != nil {
		return state.Battle{}, err
	}
	for actorID, override := range spec.Actors {
		actor := battle.Actors[actorID]
		if override.CardZones != nil {
			actor.Cards = cloneCardZones(*override.CardZones)
		}
		if override.Energy != nil {
			actor.Resources.EnergyPoints = *override.Energy
			actor.EnergyPoints = *override.Energy
		}
		if override.Statuses != nil {
			actor.Statuses = append([]state.StatusState(nil), (*override.Statuses)...)
		}
		if override.Tokens != nil {
			actor.Tokens = append([]state.TokenState(nil), (*override.Tokens)...)
		}
		if override.DefeatState != nil {
			actor.DefeatState = *override.DefeatState
		}
		battle.Actors[actorID] = actor
	}
	battle.OffensiveProposals = cloneProposals(spec.Prerequisites.OffensiveProposals)
	battle.DefensiveProposals = cloneProposals(spec.Prerequisites.DefensiveProposals)
	battle.Segment = segment.State{Current: spec.Entry.Segment, Round: spec.Entry.Round}
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	switch spec.Random.Mode {
	case state.RandomModeReproducible:
		battle.Random = state.RandomState{
			Mode:      state.RandomModeReproducible,
			Algorithm: state.RandomAlgorithmSHA256,
			Seed:      spec.Random.Seed,
		}
	default:
		battle.Random = state.RandomState{
			Mode:      state.RandomModeNormal,
			Algorithm: state.RandomAlgorithmCrypto,
		}
	}
	fingerprint, err := Fingerprint(spec)
	if err != nil {
		return state.Battle{}, err
	}
	battle.Origin = state.BattleOrigin{
		Kind:                state.BattleOriginScenario,
		ScenarioID:          spec.Metadata.ID,
		ScenarioSchema:      spec.SchemaVersion,
		ScenarioFingerprint: fingerprint,
		CreatedBy:           "local_developer",
	}
	if err := ValidateBattle(battle, spec); err != nil {
		return state.Battle{}, err
	}
	return battle, nil
}

func (builder Builder) BuildAndProgress(
	spec Spec,
	battleEngine engine.Engine,
) (state.Battle, engine.ProgressionResult, error) {
	battle, err := builder.Build(spec)
	if err != nil {
		return state.Battle{}, engine.ProgressionResult{}, err
	}
	progressed, err := battleEngine.ProgressUntilInput(&battle)
	if err != nil {
		return state.Battle{}, engine.ProgressionResult{}, err
	}
	if len(spec.SetupScript) == 0 {
		return battle, progressed, nil
	}
	for i, step := range spec.SetupScript {
		payload, err := materializeScriptPayload(battle, step)
		if err != nil {
			return state.Battle{}, engine.ProgressionResult{}, fmt.Errorf("setup_script[%d]: %w", i, err)
		}
		cmd := command.Command{
			BattleID: battle.ID,
			ActorID:  step.ActorID,
			Type:     step.Type,
			Payload:  payload,
		}
		next, err := battleEngine.ApplyBattleCommand(&battle, cmd)
		if err != nil {
			return state.Battle{}, engine.ProgressionResult{}, fmt.Errorf("setup_script[%d]: %w", i, err)
		}
		progressed.Events = append(progressed.Events, next.Events...)
		progressed.Status = next.Status
		if err := assertWait(battle, step.Expect); err != nil {
			return state.Battle{}, engine.ProgressionResult{}, fmt.Errorf("setup_script[%d]: %w", i, err)
		}
	}
	return battle, progressed, nil
}

func materializeScriptPayload(battle state.Battle, step ScriptStep) (json.RawMessage, error) {
	pending, ok := battle.Flow.PendingInput[step.ActorID]
	if !ok {
		return nil, fmt.Errorf("actor %q has no pending input", step.ActorID)
	}
	var value any
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, errors.New("payload must be valid JSON")
	}
	replacements := map[string]any{
		"$pending_input_id": pending.ID,
		"$window_id":        pending.WindowID,
		"$segment":          string(pending.Segment),
		"$stage":            pending.Stage,
		"$iteration":        pending.Iteration,
		"$planning_cycle":   pending.PlanningCycle,
		"$reaction_round":   pending.ReactionRound,
	}
	value = replaceScriptValues(value, replacements)
	return json.Marshal(value)
}

func replaceScriptValues(value any, replacements map[string]any) any {
	switch typed := value.(type) {
	case string:
		if replacement, ok := replacements[typed]; ok {
			return replacement
		}
		return typed
	case []any:
		for i := range typed {
			typed[i] = replaceScriptValues(typed[i], replacements)
		}
		return typed
	case map[string]any:
		for key := range typed {
			typed[key] = replaceScriptValues(typed[key], replacements)
		}
		return typed
	default:
		return value
	}
}

func (builder Builder) BuildCheckpoint(spec Spec) (repository.Checkpoint, error) {
	battle, err := builder.Build(spec)
	if err != nil {
		return repository.Checkpoint{}, err
	}
	return repository.NewCheckpoint(battle)
}

func applyDescriptors(setup *state.BattleSetup, participants []participant.Participant) error {
	if setup == nil {
		return errors.New("participant assembler returned nil setup")
	}
	byID := make(map[string]participant.Participant, len(participants))
	for _, value := range participants {
		byID[value.InstanceID] = value
	}
	if len(setup.Actors) != len(participants) {
		return errors.New("participant assembler returned the wrong actor count")
	}
	for i := range setup.Actors {
		value, ok := byID[setup.Actors[i].ID]
		if !ok {
			return fmt.Errorf("participant assembler returned unexpected actor %q", setup.Actors[i].ID)
		}
		setup.Actors[i].DefinitionID = value.DefinitionID
		setup.Actors[i].ControllerType = value.Controller
		delete(byID, setup.Actors[i].ID)
	}
	if len(byID) != 0 {
		return errors.New("participant assembler omitted a requested actor")
	}
	return nil
}

func cloneCardZones(value state.CardZones) state.CardZones {
	return state.CardZones{
		Deck:    append([]string(nil), value.Deck...),
		Hand:    append([]string(nil), value.Hand...),
		Discard: append([]string(nil), value.Discard...),
		Removed: append([]string(nil), value.Removed...),
	}
}

func cloneProposals(values []state.PlanningProposal) []state.PlanningProposal {
	if values == nil {
		return nil
	}
	encoded, _ := json.Marshal(values)
	var cloned []state.PlanningProposal
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func assertWait(battle state.Battle, expect WaitExpectation) error {
	if expect == (WaitExpectation{}) {
		return nil
	}
	if expect.Segment != "" && battle.Segment.Current != expect.Segment {
		return fmt.Errorf("reached segment %q, want %q", battle.Segment.Current, expect.Segment)
	}
	if len(battle.Flow.PendingInput) == 0 {
		return errors.New("expected a pending input")
	}
	for _, pending := range battle.Flow.PendingInput {
		if expect.InputType != "" && pending.InputType != expect.InputType {
			continue
		}
		if expect.WindowPurpose != "" {
			resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
			if !ok {
				continue
			}
			window, ok := resolution.Windows[resolution.ActiveWindowID]
			if !ok || window.Purpose != expect.WindowPurpose {
				continue
			}
		}
		return nil
	}
	return errors.New("pending input did not match expected wait")
}
