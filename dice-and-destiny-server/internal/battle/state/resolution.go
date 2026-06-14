package state

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
)

type FlowPhase string

const (
	FlowPhaseOnEnter    FlowPhase = "on_enter"
	FlowPhaseInProgress FlowPhase = "in_progress"
	FlowPhaseOnExit     FlowPhase = "on_exit"
)

type ResolutionStage string

const (
	ResolutionCollecting ResolutionStage = "collecting"
	ResolutionRevealing  ResolutionStage = "revealing"
	ResolutionReacting   ResolutionStage = "reacting"
	ResolutionCommitting ResolutionStage = "committing"
	ResolutionComplete   ResolutionStage = "complete"
)

type ResolutionCheckpoint struct {
	Segment   segment.Segment `json:"segment"`
	Phase     FlowPhase       `json:"phase"`
	Stage     string          `json:"stage"`
	Iteration int             `json:"iteration"`
}

type ResolutionState struct {
	ID                    string
	Origin                ResolutionCheckpoint
	Stage                 ResolutionStage
	Batch                 ProposalBatch
	Windows               map[string]InteractionWindow
	ActiveWindowID        string
	WindowSequence        int
	ReactionPolicy        *ReactionWindowPolicy
	SuspendedActors       map[string]ActorFlowState
	SuspendedPendingInput map[string]PendingInput
}

type ProposalBatch struct {
	ID           string     `json:"id"`
	ResolutionID string     `json:"resolution_id"`
	Proposals    []Proposal `json:"proposals"`
	Revealed     bool       `json:"revealed"`
	Committed    bool       `json:"committed"`
}

type ProposalOperation string

const (
	ProposalOperationAdjustValue  ProposalOperation = "adjust_value"
	ProposalOperationRecordChoice ProposalOperation = "record_choice"
)

type Proposal struct {
	ID        string            `json:"id"`
	Source    SourceReference   `json:"source"`
	Target    TargetReference   `json:"target"`
	Operation ProposalOperation `json:"operation"`
	Data      ProposalData      `json:"data"`
}

type SourceReference struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	ActorID      string `json:"actor_id,omitempty"`
	DefinitionID string `json:"definition_id,omitempty"`
}

type TargetReference struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	ActorID string `json:"actor_id,omitempty"`
}

type ProposalData struct {
	Amount    *AmountData    `json:"amount,omitempty"`
	Selection *SelectionData `json:"selection,omitempty"`
	Roll      *RollData      `json:"roll,omitempty"`
}

type AmountData struct {
	Value int `json:"value"`
}

type SelectionData struct {
	OptionIDs []string `json:"option_ids"`
}

type RollData struct {
	Dice []RolledDie `json:"dice"`
}

type InteractionPurpose string

const (
	InteractionPurposeRequiredRoll InteractionPurpose = "required_roll"
	InteractionPurposePlanning     InteractionPurpose = "planning"
	InteractionPurposeReaction     InteractionPurpose = "reaction"
	InteractionPurposeChooseCard   InteractionPurpose = "choose_card"
	InteractionPurposeSelectTarget InteractionPurpose = "select_target"
)

type RevealStatus string

const (
	RevealStatusCollecting RevealStatus = "collecting"
	RevealStatusRevealed   RevealStatus = "revealed"
)

type InteractionActorStatus string

const (
	InteractionActorEligible  InteractionActorStatus = "eligible"
	InteractionActorAwaiting  InteractionActorStatus = "awaiting"
	InteractionActorCommitted InteractionActorStatus = "committed"
	InteractionActorPassed    InteractionActorStatus = "passed"
)

type InteractionWindow struct {
	ID                  string
	Opened              bool
	Purpose             InteractionPurpose
	Source              SourceReference
	EligibleActors      []string
	RequiredActors      []string
	ActorProgress       map[string]InteractionActorStatus
	AllowedCommands     []command.Type
	HiddenCommitments   bool
	RevealStatus        RevealStatus
	PassAllowed         bool
	Commitments         map[string]InteractionCommitment
	ReactionRound       int
	ChainDepth          int
	SuspendedCheckpoint ResolutionCheckpoint
}

type InteractionCommitment struct {
	ID       string
	ActorID  string
	Command  command.Type
	Passed   bool
	Data     InteractionCommitmentData
	Revealed bool
}

type InteractionCommitmentData struct {
	ProposalIDs []string `json:"proposal_ids,omitempty"`
	CardIDs     []string `json:"card_ids,omitempty"`
	TargetIDs   []string `json:"target_ids,omitempty"`
	ChoiceID    string   `json:"choice_id,omitempty"`
	Value       *int     `json:"value,omitempty"`
}

type ReactionWindowPolicy struct {
	Source            SourceReference
	EligibleActors    []string
	RequiredActors    []string
	AllowedCommands   []command.Type
	HiddenCommitments bool
	PassAllowed       bool
}

func cloneResolutions(values map[string]ResolutionState) map[string]ResolutionState {
	if values == nil {
		return nil
	}
	cloned := make(map[string]ResolutionState, len(values))
	for id, resolution := range values {
		resolution.Batch = cloneProposalBatch(resolution.Batch)
		resolution.Windows = cloneInteractionWindows(resolution.Windows)
		resolution.ReactionPolicy = cloneReactionPolicy(resolution.ReactionPolicy)
		resolution.SuspendedActors = cloneActorFlowStates(resolution.SuspendedActors)
		resolution.SuspendedPendingInput = clonePendingInputs(resolution.SuspendedPendingInput)
		cloned[id] = resolution
	}
	return cloned
}

func cloneProposalBatch(batch ProposalBatch) ProposalBatch {
	cloned := batch
	cloned.Proposals = make([]Proposal, len(batch.Proposals))
	for i, proposal := range batch.Proposals {
		cloned.Proposals[i] = proposal
		cloned.Proposals[i].Data = cloneProposalData(proposal.Data)
	}
	return cloned
}

func cloneProposalData(data ProposalData) ProposalData {
	cloned := data
	if data.Amount != nil {
		value := *data.Amount
		cloned.Amount = &value
	}
	if data.Selection != nil {
		value := *data.Selection
		value.OptionIDs = copyStrings(data.Selection.OptionIDs)
		cloned.Selection = &value
	}
	if data.Roll != nil {
		value := *data.Roll
		value.Dice = copyRolledDice(data.Roll.Dice)
		cloned.Roll = &value
	}
	return cloned
}

func cloneInteractionWindows(values map[string]InteractionWindow) map[string]InteractionWindow {
	if values == nil {
		return nil
	}
	cloned := make(map[string]InteractionWindow, len(values))
	for id, window := range values {
		window.EligibleActors = copyStrings(window.EligibleActors)
		window.RequiredActors = copyStrings(window.RequiredActors)
		window.AllowedCommands = append([]command.Type(nil), window.AllowedCommands...)
		if window.ActorProgress != nil {
			window.ActorProgress = make(map[string]InteractionActorStatus, len(window.ActorProgress))
			for actorID, progress := range values[id].ActorProgress {
				window.ActorProgress[actorID] = progress
			}
		}
		if window.Commitments != nil {
			window.Commitments = make(map[string]InteractionCommitment, len(window.Commitments))
			for actorID, commitment := range values[id].Commitments {
				commitment.Data = cloneCommitmentData(commitment.Data)
				window.Commitments[actorID] = commitment
			}
		}
		cloned[id] = window
	}
	return cloned
}

func cloneReactionPolicy(policy *ReactionWindowPolicy) *ReactionWindowPolicy {
	if policy == nil {
		return nil
	}
	cloned := *policy
	cloned.EligibleActors = copyStrings(policy.EligibleActors)
	cloned.RequiredActors = copyStrings(policy.RequiredActors)
	cloned.AllowedCommands = append([]command.Type(nil), policy.AllowedCommands...)
	return &cloned
}

func cloneActorFlowStates(values map[string]ActorFlowState) map[string]ActorFlowState {
	if values == nil {
		return nil
	}
	cloned := make(map[string]ActorFlowState, len(values))
	for actorID, progress := range values {
		cloned[actorID] = progress
	}
	return cloned
}

func clonePendingInputs(values map[string]PendingInput) map[string]PendingInput {
	if values == nil {
		return nil
	}
	cloned := make(map[string]PendingInput, len(values))
	for actorID, pending := range values {
		pending.AllowedCommands = append([]command.Type(nil), pending.AllowedCommands...)
		cloned[actorID] = pending
	}
	return cloned
}

func cloneCommitmentData(data InteractionCommitmentData) InteractionCommitmentData {
	cloned := data
	cloned.ProposalIDs = copyStrings(data.ProposalIDs)
	cloned.CardIDs = copyStrings(data.CardIDs)
	cloned.TargetIDs = copyStrings(data.TargetIDs)
	if data.Value != nil {
		value := *data.Value
		cloned.Value = &value
	}
	return cloned
}
