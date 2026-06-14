package repository

import (
	"errors"
	"sync"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

var (
	ErrBattleNotFound = errors.New("battle not found")
	ErrBattleExists   = errors.New("battle already exists")
)

type Checkpoint struct {
	Battle state.Battle
	Events []event.Event
}

type Repository interface {
	Create(checkpoint Checkpoint) error
	Load(battleID string) (Checkpoint, error)
	Save(checkpoint Checkpoint) error
}

type InMemory struct {
	mu          sync.RWMutex
	checkpoints map[string]Checkpoint
}

func NewInMemory() *InMemory {
	return &InMemory{
		checkpoints: make(map[string]Checkpoint),
	}
}

func (repo *InMemory) Create(checkpoint Checkpoint) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, exists := repo.checkpoints[checkpoint.Battle.ID]; exists {
		return ErrBattleExists
	}
	repo.checkpoints[checkpoint.Battle.ID] = cloneCheckpoint(checkpoint)
	return nil
}

func (repo *InMemory) Load(battleID string) (Checkpoint, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	checkpoint, ok := repo.checkpoints[battleID]
	if !ok {
		return Checkpoint{}, ErrBattleNotFound
	}
	return cloneCheckpoint(checkpoint), nil
}

func (repo *InMemory) Save(checkpoint Checkpoint) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, exists := repo.checkpoints[checkpoint.Battle.ID]; !exists {
		return ErrBattleNotFound
	}
	repo.checkpoints[checkpoint.Battle.ID] = cloneCheckpoint(checkpoint)
	return nil
}

func cloneCheckpoint(checkpoint Checkpoint) Checkpoint {
	return Checkpoint{
		Battle: checkpoint.Battle.Clone(),
		Events: cloneEvents(checkpoint.Events),
	}
}

func cloneEvents(events []event.Event) []event.Event {
	if events == nil {
		return nil
	}
	cloned := make([]event.Event, len(events))
	for i, source := range events {
		cloned[i] = source
		cloned[i].Cards = append([]string(nil), source.Cards...)
		cloned[i].Dice = cloneRolledDice(source.Dice)
		cloned[i].RolledIndices = append([]int(nil), source.RolledIndices...)
		cloned[i].RollsRemaining = cloneInt(source.RollsRemaining)
		cloned[i].Combinations = append([]string(nil), source.Combinations...)
		cloned[i].SymbolCounts = cloneIntMap(source.SymbolCounts)
	}
	return cloned
}

func cloneRolledDice(values []state.RolledDie) []state.RolledDie {
	if values == nil {
		return nil
	}
	cloned := make([]state.RolledDie, len(values))
	for i, die := range values {
		cloned[i] = die
		cloned[i].Symbols = append([]string(nil), die.Symbols...)
	}
	return cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntMap(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	cloned := make(map[string]int, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
