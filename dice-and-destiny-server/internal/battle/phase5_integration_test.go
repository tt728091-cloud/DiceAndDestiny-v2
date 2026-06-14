package battle

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestLoadedContentDrivesPlanningProposalsAndSurvivesRepository(t *testing.T) {
	root := participantTestServerRoot(t)
	assembler := NewFileParticipantAssembler(
		filepath.Join(root, "content"),
		filepath.Join(root, "save", "run_players"),
	)
	setup, err := assembler.AssembleParticipants([]Participant{
		{InstanceID: "player", DefinitionID: "current_run_player", Controller: state.ControllerHuman},
	})
	if err != nil {
		t.Fatalf("AssembleParticipants() returned error: %v", err)
	}
	if len(setup.Content.Cards["Mock Strike"].Operations) != 1 ||
		len(setup.Content.Abilities["Mock Smite"].Operations) != 1 ||
		len(setup.Content.Statuses["poison"].Triggers) != 1 {
		t.Fatalf("runtime content catalog is incomplete: %#v", setup.Content)
	}

	battle, err := state.NewBattleFromSetup("phase-5-planning", setup)
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	battle.Segment = segment.State{Current: segment.Offensive, Round: 1}
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	eng := engine.NewEngine()
	if _, err := eng.ProgressUntilInput(&battle); err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}

	rejected := battle.Clone()
	err = applyPlanningPayloadError(eng, &rejected, command.TypePlanningAbility, command.PlanningAbilityPayload{
		PendingInputID: rejected.Flow.PendingInput["player"].ID,
		Checkpoint:     planningCheckpoint(rejected),
		AbilityID:      "Mock Guarding Light",
	})
	if err == nil {
		t.Fatal("defensive loaded ability was accepted during offensive planning")
	}

	applyPlanningPayload(t, eng, &battle, command.TypePlanningCards, command.PlanningCardsPayload{
		PendingInputID: battle.Flow.PendingInput["player"].ID,
		Checkpoint:     planningCheckpoint(battle),
		CardIDs:        []string{"Mock Strike"},
	})
	applyPlanningPayload(t, eng, &battle, command.TypePlanningLockIn, command.PlanningLockInPayload{
		PendingInputID: battle.Flow.PendingInput["player"].ID,
		Checkpoint:     planningCheckpoint(battle),
	})

	pending := battle.Flow.PendingInput["player"]
	applyPlanningPayload(t, eng, &battle, command.TypePass, command.PassPayload{
		PendingInputID: pending.ID,
		Checkpoint: command.InteractionCheckpoint{
			WindowID:      pending.WindowID,
			Stage:         pending.Stage,
			Iteration:     pending.Iteration,
			ReactionRound: pending.ReactionRound,
		},
	})

	if len(battle.OffensiveProposals) != 1 {
		t.Fatalf("offensive proposals = %#v, want one", battle.OffensiveProposals)
	}
	proposal := battle.OffensiveProposals[0]
	if len(proposal.Operations) != 1 ||
		proposal.Operations[0].ContentType != "card" ||
		proposal.Operations[0].ContentID != "Mock Strike" ||
		proposal.Operations[0].Operation.Type != operation.TypeNoop {
		t.Fatalf("typed finalized proposal = %#v", proposal)
	}
	if len(battle.Actors["player"].Cards.Removed) != 0 {
		t.Fatal("planning applied a card effect or permanently removed a card")
	}

	repo := repository.NewInMemory()
	if err := repo.Create(repository.Checkpoint{Battle: battle}); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	loaded, err := repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded.Battle.Content, battle.Content) ||
		!reflect.DeepEqual(loaded.Battle.OffensiveProposals, battle.OffensiveProposals) {
		t.Fatal("content catalog or finalized operation references changed across repository save/load")
	}
}

func applyPlanningPayloadError(
	eng engine.Engine,
	battle *state.Battle,
	commandType command.Type,
	payload any,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = eng.ApplyBattleCommand(battle, command.Command{
		BattleID: battle.ID,
		ActorID:  "player",
		Type:     commandType,
		Payload:  encoded,
	})
	return err
}

func planningCheckpoint(battle state.Battle) command.PlanningCheckpoint {
	pending := battle.Flow.PendingInput["player"]
	return command.PlanningCheckpoint{
		WindowID:      pending.WindowID,
		Segment:       string(pending.Segment),
		Stage:         pending.Stage,
		Iteration:     pending.Iteration,
		PlanningCycle: pending.PlanningCycle,
	}
}

func applyPlanningPayload(
	t *testing.T,
	eng engine.Engine,
	battle *state.Battle,
	commandType command.Type,
	payload any,
) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	if _, err := eng.ApplyBattleCommand(battle, command.Command{
		BattleID: battle.ID,
		ActorID:  "player",
		Type:     commandType,
		Payload:  encoded,
	}); err != nil {
		t.Fatalf("%s returned error: %v", commandType, err)
	}
}
