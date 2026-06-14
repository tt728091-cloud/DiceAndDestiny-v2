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
		cloned[i].Commitment = cloneInteractionCommitment(source.Commitment)
		cloned[i].Commitments = cloneInteractionCommitments(source.Commitments)
		cloned[i].ProposalBatch = cloneProposalBatch(source.ProposalBatch)
	}
	return cloned
}

func cloneInteractionCommitment(
	value *state.InteractionCommitment,
) *state.InteractionCommitment {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Data.ProposalIDs = append([]string(nil), value.Data.ProposalIDs...)
	cloned.Data.CardIDs = append([]string(nil), value.Data.CardIDs...)
	cloned.Data.TargetIDs = append([]string(nil), value.Data.TargetIDs...)
	cloned.Data.Value = cloneInt(value.Data.Value)
	return &cloned
}

func cloneInteractionCommitments(
	values []state.InteractionCommitment,
) []state.InteractionCommitment {
	if values == nil {
		return nil
	}
	cloned := make([]state.InteractionCommitment, len(values))
	for i := range values {
		value := cloneInteractionCommitment(&values[i])
		cloned[i] = *value
	}
	return cloned
}

func cloneProposalBatch(value *state.ProposalBatch) *state.ProposalBatch {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Proposals = make([]state.Proposal, len(value.Proposals))
	for i, proposal := range value.Proposals {
		cloned.Proposals[i] = proposal
		if proposal.Data.Amount != nil {
			amount := *proposal.Data.Amount
			cloned.Proposals[i].Data.Amount = &amount
		}
		if proposal.Data.Selection != nil {
			selection := *proposal.Data.Selection
			selection.OptionIDs = append([]string(nil), proposal.Data.Selection.OptionIDs...)
			cloned.Proposals[i].Data.Selection = &selection
		}
		if proposal.Data.Roll != nil {
			roll := *proposal.Data.Roll
			roll.Dice = cloneRolledDice(proposal.Data.Roll.Dice)
			cloned.Proposals[i].Data.Roll = &roll
		}
	}
	return &cloned
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
