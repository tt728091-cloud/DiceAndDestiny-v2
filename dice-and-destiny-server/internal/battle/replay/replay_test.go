package replay_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/replay"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestReplayReturnsRecordedRandomResultsAndViewerSafeEvents(t *testing.T) {
	repo := repository.NewDisk(t.TempDir())
	battle, err := state.NewBattleFromSetup("replay-battle", state.BattleSetup{
		Actors: []state.ActorSetup{
			{ID: "player", ControllerType: state.ControllerHuman, Deck: []string{"p"}},
			{ID: "enemy", ControllerType: state.ControllerAI, Deck: []string{"e"}},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatalf("NewCheckpoint() returned error: %v", err)
	}
	window := state.InteractionWindow{
		ID: "hidden-window", Purpose: state.InteractionPurposePlanning,
	}
	hidden := event.NewInteractionCommitted(
		"resolution-1",
		window,
		state.InteractionCommitment{
			ID: "enemy-commit", ActorID: "enemy", Command: command.TypePlanningLockIn,
			Data: state.InteractionCommitmentData{
				ChoiceID: "secret-choice",
			},
		},
		"enemy",
	)
	rolled := event.NewDiceRolled(
		"player",
		segment.Offensive,
		"roll-1",
		state.RollPoolOffensive,
		state.RollSourceAbility,
		"ability-1",
		[]state.RolledDie{{Index: 0, DieID: "d6", Face: 5, Value: 5, Symbols: []string{"sun", "blade"}}},
		[]int{0},
		1,
		3,
		nil,
		map[string]int{"sun": 1, "blade": 1},
	)
	if _, err := repository.AppendEvents(&checkpoint, []event.Event{hidden, rolled}); err != nil {
		t.Fatalf("AppendEvents() returned error: %v", err)
	}
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	reader := replay.NewReader(repo)
	authoritative, err := reader.Read(replay.Request{BattleID: battle.ID})
	if err != nil {
		t.Fatalf("Read(authoritative) returned error: %v", err)
	}
	if authoritative.Events[0].Commitment == nil ||
		authoritative.Events[0].Commitment.Data.ChoiceID != "secret-choice" {
		t.Fatalf("authoritative replay lost hidden commitment: %#v", authoritative.Events[0])
	}
	if !reflect.DeepEqual(authoritative.Events[1].Dice, rolled.Dice) ||
		!reflect.DeepEqual(authoritative.Events[1].SymbolCounts, rolled.SymbolCounts) {
		t.Fatalf("recorded random result changed: %#v", authoritative.Events[1])
	}

	viewer, err := reader.Read(replay.Request{BattleID: battle.ID, ViewerActorID: "player"})
	if err != nil {
		t.Fatalf("Read(viewer) returned error: %v", err)
	}
	if viewer.Events[0].Commitment != nil || viewer.Events[0].PrivateActorID != "" {
		t.Fatalf("viewer replay exposed hidden commitment: %#v", viewer.Events[0])
	}
	if !reflect.DeepEqual(authoritative.Events[1].Dice, viewer.Events[1].Dice) {
		t.Fatal("viewer filtering changed the viewer's recorded dice result")
	}
}

func TestReplayDetectsChangedExpectedContent(t *testing.T) {
	repo := repository.NewInMemory()
	battle, _ := state.NewBattle("replay-content")
	checkpoint, err := repository.NewCheckpoint(battle)
	if err != nil {
		t.Fatalf("NewCheckpoint() returned error: %v", err)
	}
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	_, err = replay.NewReader(repo).Read(replay.Request{
		BattleID:                   battle.ID,
		ExpectedContentFingerprint: "different",
	})
	if !errors.Is(err, replay.ErrContentChanged) {
		t.Fatalf("Read() error = %v, want ErrContentChanged", err)
	}
}
