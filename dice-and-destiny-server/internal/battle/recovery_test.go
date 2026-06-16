package battle

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestAuthorityRecoversPlanningAndReactionAcrossProcessRestarts(t *testing.T) {
	serverRoot := participantTestServerRoot(t)
	stateRoot := t.TempDir()
	assembler := NewFileParticipantAssembler(
		filepath.Join(serverRoot, "content"),
		filepath.Join(serverRoot, "save", "run_players"),
	)
	firstRepo := repository.NewDisk(stateRoot)
	firstAuthority := NewAuthority(engine.NewEngine(), firstRepo, assembler)
	battleID := "restart-planning-reaction"

	started := sendRecoveryCommand(t, firstAuthority, map[string]any{
		"battle_id": battleID,
		"actor_id":  "player",
		"type":      command.TypeStartBattle,
		"payload": map[string]any{
			"player": map[string]string{
				"instance_id":   "player",
				"definition_id": "current_run_player",
			},
			"enemies": []map[string]string{{
				"instance_id": "goblin-1", "definition_id": "mock_goblin",
			}},
		},
	})
	if started.Status != engine.ProgressWaitingForInput {
		t.Fatalf("start status = %q, want planning wait", started.Status)
	}
	planningCheckpoint, err := firstRepo.Load(battleID)
	if err != nil {
		t.Fatalf("Load(planning) returned error: %v", err)
	}

	secondRepo := repository.NewDisk(stateRoot)
	reloadedPlanning, err := secondRepo.Load(battleID)
	if err != nil {
		t.Fatalf("restarted Load(planning) returned error: %v", err)
	}
	if !reflect.DeepEqual(reloadedPlanning, planningCheckpoint) {
		t.Fatal("planning checkpoint changed across repository reconstruction")
	}
	secondAuthority := NewAuthority(engine.NewEngine(), secondRepo, failingAssembler{})
	pending := reloadedPlanning.Battle.Flow.PendingInput["player"]
	passed := sendRecoveryCommand(t, secondAuthority, map[string]any{
		"battle_id": battleID,
		"actor_id":  "player",
		"type":      command.TypePlanningPass,
		"payload": command.PlanningPassPayload{
			PendingInputID: pending.ID,
			Checkpoint: command.PlanningCheckpoint{
				WindowID:      pending.WindowID,
				Segment:       string(pending.Segment),
				Stage:         pending.Stage,
				Iteration:     pending.Iteration,
				PlanningCycle: pending.PlanningCycle,
			},
		},
	})
	if passed.Status != engine.ProgressWaitingForInput {
		t.Fatalf("planning pass status = %q", passed.Status)
	}

	afterPass, err := secondRepo.Load(battleID)
	if err != nil {
		t.Fatalf("Load(after pass) returned error: %v", err)
	}
	pending = afterPass.Battle.Flow.PendingInput["player"]
	locked := sendRecoveryCommand(t, secondAuthority, map[string]any{
		"battle_id": battleID,
		"actor_id":  "player",
		"type":      command.TypePlanningLockIn,
		"payload": command.PlanningLockInPayload{
			PendingInputID: pending.ID,
			Checkpoint: command.PlanningCheckpoint{
				WindowID:      pending.WindowID,
				Segment:       string(pending.Segment),
				Stage:         pending.Stage,
				Iteration:     pending.Iteration,
				PlanningCycle: pending.PlanningCycle,
			},
		},
	})
	if locked.Status != engine.ProgressWaitingForInput {
		t.Fatalf("planning lock status = %q, want reaction wait", locked.Status)
	}
	reactionCheckpoint, err := secondRepo.Load(battleID)
	if err != nil {
		t.Fatalf("Load(reaction) returned error: %v", err)
	}
	if reactionCheckpoint.Battle.ActiveResolutionID == "" ||
		reactionCheckpoint.Battle.Flow.PendingInput["player"].ReactionRound != 1 {
		t.Fatalf("reaction checkpoint is incomplete: %#v", reactionCheckpoint.Battle)
	}

	thirdRepo := repository.NewDisk(stateRoot)
	reloadedReaction, err := thirdRepo.Load(battleID)
	if err != nil {
		t.Fatalf("restarted Load(reaction) returned error: %v", err)
	}
	if !reflect.DeepEqual(reloadedReaction, reactionCheckpoint) {
		t.Fatal("reaction checkpoint changed across repository reconstruction")
	}
	thirdAuthority := NewAuthority(engine.NewEngine(), thirdRepo, failingAssembler{})
	pending = reloadedReaction.Battle.Flow.PendingInput["player"]
	continued := sendRecoveryCommand(t, thirdAuthority, map[string]any{
		"battle_id": battleID,
		"actor_id":  "player",
		"type":      command.TypePass,
		"payload": command.PassPayload{
			PendingInputID: pending.ID,
			Checkpoint: command.InteractionCheckpoint{
				WindowID:      pending.WindowID,
				Stage:         pending.Stage,
				Iteration:     pending.Iteration,
				ReactionRound: pending.ReactionRound,
			},
		},
	})
	if continued.Status != engine.ProgressWaitingForInput {
		t.Fatalf("continued status = %q, want next planning wait", continued.Status)
	}

	finalCheckpoint, err := thirdRepo.Load(battleID)
	if err != nil {
		t.Fatalf("Load(final) returned error: %v", err)
	}
	assertContiguousEvents(t, finalCheckpoint)
	if finalCheckpoint.ContentPin != planningCheckpoint.ContentPin {
		t.Fatal("content pin changed across restart")
	}
}

func TestTerminalCheckpointReloadsAndRejectsCommandsWithoutMutation(t *testing.T) {
	root := t.TempDir()
	repo := repository.NewDisk(root)
	battle, err := state.NewBattleFromSetup("terminal-restart", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", ControllerType: state.ControllerHuman, Deck: []string{"health"}},
			{ID: "enemy", ControllerType: state.ControllerAI, Removed: []string{"health"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	battle.Status = state.BattleVictory
	enemy := battle.Actors["enemy"]
	enemy.DefeatState = state.ActorDefeated
	battle.Actors["enemy"] = enemy
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatalf("NewCheckpoint() returned error: %v", err)
	}
	if _, err := repository.AppendEvents(
		&checkpoint,
		[]event.Event{event.NewBattleCompleted(state.BattleVictory)},
	); err != nil {
		t.Fatalf("AppendEvents() returned error: %v", err)
	}
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	restartedRepo := repository.NewDisk(root)
	reloaded, err := restartedRepo.Load(battle.ID)
	if err != nil {
		t.Fatalf("restarted Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(reloaded, checkpoint) {
		t.Fatal("terminal checkpoint changed across restart")
	}
	authority := NewAuthority(engine.NewEngine(), restartedRepo, failingAssembler{})
	result := sendRecoveryCommandUnchecked(t, authority, map[string]any{
		"battle_id": battle.ID,
		"actor_id":  "player",
		"type":      command.TypePass,
		"payload":   map[string]any{},
	})
	if result.Accepted || result.Error != "battle is complete" {
		t.Fatalf("terminal command result = %#v", result)
	}
	after, err := restartedRepo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load(after rejection) returned error: %v", err)
	}
	if !reflect.DeepEqual(after, checkpoint) {
		t.Fatal("rejected terminal command mutated the checkpoint")
	}
}

func sendRecoveryCommand(t *testing.T, authority *Authority, payload any) engine.Result {
	t.Helper()
	result := sendRecoveryCommandUnchecked(t, authority, payload)
	if !result.Accepted {
		t.Fatalf("authority rejected command: %#v", result)
	}
	return result
}

func sendRecoveryCommandUnchecked(t *testing.T, authority *Authority, payload any) engine.Result {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(command) returned error: %v", err)
	}
	return decodeAuthorityResult(t, authority.HandleCommandJSON(string(encoded)))
}

func assertContiguousEvents(t *testing.T, checkpoint repository.Checkpoint) {
	t.Helper()
	for i, battleEvent := range checkpoint.Events {
		want := uint64(i + 1)
		if battleEvent.Sequence != want ||
			battleEvent.BattleID != checkpoint.BattleID ||
			battleEvent.SchemaVersion != event.SchemaVersion ||
			battleEvent.ID == "" {
			t.Fatalf("event %d metadata = %#v", i, battleEvent)
		}
	}
	if checkpoint.NextEventSequence != uint64(len(checkpoint.Events)+1) {
		t.Fatalf("next event sequence = %d, want %d",
			checkpoint.NextEventSequence, len(checkpoint.Events)+1)
	}
}

type failingAssembler struct{}

func (failingAssembler) AssembleParticipants([]Participant) (state.BattleSetup, error) {
	return state.BattleSetup{}, errors.New("live content should not be loaded during resume")
}
