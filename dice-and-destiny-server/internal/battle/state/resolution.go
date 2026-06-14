package state

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
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
	Planning              *PlanningState
}

type PlanningState struct {
	Segment                  segment.Segment               `json:"segment"`
	Cycle                    int                           `json:"cycle"`
	DefaultMaxRolls          int                           `json:"default_max_rolls"`
	Actors                   map[string]PlanningActorState `json:"actors"`
	ChangedActorIDs          []string                      `json:"changed_actor_ids"`
	AppliedReactionWindowIDs map[string]bool               `json:"applied_reaction_window_ids,omitempty"`
	Finalized                bool                          `json:"finalized"`
}

type PlanningActorState struct {
	ActorID            string                  `json:"actor_id"`
	Participation      ActorProgressStatus     `json:"participation"`
	ReasonCode         string                  `json:"reason_code,omitempty"`
	RollRequestID      string                  `json:"roll_request_id,omitempty"`
	FinalDice          []RolledDie             `json:"final_dice,omitempty"`
	KeptIndices        []int                   `json:"kept_indices,omitempty"`
	RollsUsed          int                     `json:"rolls_used"`
	MaxRolls           int                     `json:"max_rolls"`
	SelectedAbility    string                  `json:"selected_ability,omitempty"`
	CommittedCards     []string                `json:"committed_cards,omitempty"`
	SelectedTargets    []string                `json:"selected_targets,omitempty"`
	EligibleTargetIDs  []string                `json:"eligible_target_ids,omitempty"`
	Passed             bool                    `json:"passed"`
	LockedIn           bool                    `json:"locked_in"`
	ActionSequence     int                     `json:"action_sequence"`
	Revision           int                     `json:"revision"`
	RevealedRevision   int                     `json:"revealed_revision"`
	RevealedCommitment *PlanningCommitmentData `json:"revealed_commitment,omitempty"`
	PaidCostIDs        []string                `json:"paid_cost_ids,omitempty"`
	ResolvedCardIDs    []string                `json:"resolved_card_ids,omitempty"`
}

type PlanningCommitmentData struct {
	Segment         segment.Segment `json:"segment"`
	Cycle           int             `json:"cycle"`
	FinalDice       []RolledDie     `json:"final_dice,omitempty"`
	KeptIndices     []int           `json:"kept_indices,omitempty"`
	RollsUsed       int             `json:"rolls_used"`
	MaxRolls        int             `json:"max_rolls"`
	SelectedAbility string          `json:"selected_ability,omitempty"`
	CommittedCards  []string        `json:"committed_cards,omitempty"`
	SelectedTargets []string        `json:"selected_targets,omitempty"`
	Passed          bool            `json:"passed"`
	LockedIn        bool            `json:"locked_in"`
}

type PlanningProposal struct {
	ID         string                       `json:"id"`
	ActorID    string                       `json:"actor_id"`
	Segment    segment.Segment              `json:"segment"`
	Commitment PlanningCommitmentData       `json:"commitment"`
	Defensible bool                         `json:"defensible,omitempty"`
	Operations []FinalizedOperationProposal `json:"operations,omitempty"`
}

type FinalizedOperationProposal struct {
	ID              string         `json:"id"`
	ContentType     string         `json:"content_type"`
	ContentID       string         `json:"content_id"`
	Operation       operation.Plan `json:"operation"`
	SourceActorID   string         `json:"source_actor_id"`
	SelectedTargets []string       `json:"selected_targets,omitempty"`
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
	ProposalOperationPlanning     ProposalOperation = "planning_proposal"
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
	Amount    *AmountData             `json:"amount,omitempty"`
	Selection *SelectionData          `json:"selection,omitempty"`
	Roll      *RollData               `json:"roll,omitempty"`
	Planning  *PlanningCommitmentData `json:"planning,omitempty"`
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
	ProposalIDs         []string                `json:"proposal_ids,omitempty"`
	CardIDs             []string                `json:"card_ids,omitempty"`
	TargetIDs           []string                `json:"target_ids,omitempty"`
	ChoiceID            string                  `json:"choice_id,omitempty"`
	Value               *int                    `json:"value,omitempty"`
	Planning            *PlanningCommitmentData `json:"planning,omitempty"`
	PlanningAdjustments []PlanningAdjustment    `json:"planning_adjustments,omitempty"`
}

type PlanningAdjustmentType string

const (
	PlanningAdjustmentSetDieFace       PlanningAdjustmentType = "set_die_face"
	PlanningAdjustmentIncreaseMaxRolls PlanningAdjustmentType = "increase_max_rolls"
	PlanningAdjustmentClearAbility     PlanningAdjustmentType = "clear_ability"
	PlanningAdjustmentRemoveTarget     PlanningAdjustmentType = "remove_target"
	PlanningAdjustmentReopenActor      PlanningAdjustmentType = "reopen_actor"
)

type PlanningAdjustment struct {
	Type     PlanningAdjustmentType `json:"type"`
	ActorID  string                 `json:"actor_id"`
	DieIndex int                    `json:"die_index,omitempty"`
	Face     int                    `json:"face,omitempty"`
	Amount   int                    `json:"amount,omitempty"`
	TargetID string                 `json:"target_id,omitempty"`
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
		resolution.Planning = clonePlanningState(resolution.Planning)
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
	if data.Planning != nil {
		value := clonePlanningCommitmentData(*data.Planning)
		cloned.Planning = &value
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
	if data.Planning != nil {
		value := clonePlanningCommitmentData(*data.Planning)
		cloned.Planning = &value
	}
	cloned.PlanningAdjustments = append([]PlanningAdjustment(nil), data.PlanningAdjustments...)
	return cloned
}

func clonePlanningState(value *PlanningState) *PlanningState {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.ChangedActorIDs = copyStrings(value.ChangedActorIDs)
	cloned.Actors = make(map[string]PlanningActorState, len(value.Actors))
	for actorID, actor := range value.Actors {
		actor.FinalDice = copyRolledDice(actor.FinalDice)
		actor.KeptIndices = append([]int(nil), actor.KeptIndices...)
		actor.CommittedCards = copyStrings(actor.CommittedCards)
		actor.SelectedTargets = copyStrings(actor.SelectedTargets)
		actor.EligibleTargetIDs = copyStrings(actor.EligibleTargetIDs)
		actor.PaidCostIDs = copyStrings(actor.PaidCostIDs)
		actor.ResolvedCardIDs = copyStrings(actor.ResolvedCardIDs)
		if actor.RevealedCommitment != nil {
			revealed := clonePlanningCommitmentData(*actor.RevealedCommitment)
			actor.RevealedCommitment = &revealed
		}
		cloned.Actors[actorID] = actor
	}
	cloned.AppliedReactionWindowIDs = make(map[string]bool, len(value.AppliedReactionWindowIDs))
	for windowID, applied := range value.AppliedReactionWindowIDs {
		cloned.AppliedReactionWindowIDs[windowID] = applied
	}
	return &cloned
}

func clonePlanningCommitmentData(value PlanningCommitmentData) PlanningCommitmentData {
	value.FinalDice = copyRolledDice(value.FinalDice)
	value.KeptIndices = append([]int(nil), value.KeptIndices...)
	value.CommittedCards = copyStrings(value.CommittedCards)
	value.SelectedTargets = copyStrings(value.SelectedTargets)
	return value
}

func clonePlanningProposals(values []PlanningProposal) []PlanningProposal {
	if values == nil {
		return nil
	}
	cloned := make([]PlanningProposal, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].Commitment = clonePlanningCommitmentData(value.Commitment)
		if value.Operations != nil {
			cloned[i].Operations = append([]FinalizedOperationProposal(nil), value.Operations...)
			for j := range value.Operations {
				cloned[i].Operations[j].Operation = operation.ClonePlans([]operation.Plan{value.Operations[j].Operation})[0]
				cloned[i].Operations[j].SelectedTargets = copyStrings(value.Operations[j].SelectedTargets)
			}
		}
	}
	return cloned
}
