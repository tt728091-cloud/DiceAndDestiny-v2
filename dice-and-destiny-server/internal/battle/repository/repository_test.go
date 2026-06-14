package repository_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/state"
)

func TestInMemoryCreateLoadAndSaveUseIndependentCheckpoints(t *testing.T) {
	repo := repository.NewInMemory()
	battle, err := state.NewBattleFromSetup("battle-1", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:           "player",
				DefinitionID: "paladin",
				Deck:         []string{"strike"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	checkpoint := repository.Checkpoint{
		Battle: battle,
		Events: []event.Event{
			event.NewCardsDrawn("player", []string{"strike"}, false),
		},
	}
	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	checkpoint.Battle.Actors["player"].Cards.Deck[0] = "mutated-input"
	checkpoint.Events[0].Cards[0] = "mutated-input"

	loaded, err := repo.Load("battle-1")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if loaded.Battle.Actors["player"].Cards.Deck[0] != "strike" ||
		loaded.Events[0].Cards[0] != "strike" {
		t.Fatalf("stored checkpoint aliased create input: %#v", loaded)
	}

	loaded.Battle.Actors["player"].Cards.Deck[0] = "saved"
	loaded.Events[0].Cards[0] = "saved"
	if err := repo.Save(loaded); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	loaded.Battle.Actors["player"].Cards.Deck[0] = "mutated-after-save"
	loaded.Events[0].Cards[0] = "mutated-after-save"

	reloaded, err := repo.Load("battle-1")
	if err != nil {
		t.Fatalf("second Load() returned error: %v", err)
	}
	if reloaded.Battle.Actors["player"].Cards.Deck[0] != "saved" ||
		reloaded.Events[0].Cards[0] != "saved" {
		t.Fatalf("stored checkpoint aliased save input: %#v", reloaded)
	}
}

func TestInMemoryRejectsDuplicateCreateAndMissingLoadOrSave(t *testing.T) {
	repo := repository.NewInMemory()
	battle, err := state.NewBattle("battle-1")
	if err != nil {
		t.Fatalf("NewBattle() returned error: %v", err)
	}
	checkpoint := repository.Checkpoint{Battle: battle}

	if err := repo.Create(checkpoint); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if err := repo.Create(checkpoint); !errors.Is(err, repository.ErrBattleExists) {
		t.Fatalf("duplicate Create() error = %v, want ErrBattleExists", err)
	}
	if _, err := repo.Load("missing"); !errors.Is(err, repository.ErrBattleNotFound) {
		t.Fatalf("missing Load() error = %v, want ErrBattleNotFound", err)
	}

	missing := checkpoint
	missing.Battle.ID = "missing"
	if err := repo.Save(missing); !errors.Is(err, repository.ErrBattleNotFound) {
		t.Fatalf("missing Save() error = %v, want ErrBattleNotFound", err)
	}

	reloaded, err := repo.Load("battle-1")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(reloaded, repository.Checkpoint{Battle: battle}) {
		t.Fatalf("checkpoint changed after rejected operations: %#v", reloaded)
	}
}
