package state

import "diceanddestiny/server/internal/battle/operation"

type DamageResolutionStage string

const (
	DamageStageCollect      DamageResolutionStage = "collect"
	DamageStageCalculate    DamageResolutionStage = "calculate"
	DamageStageSelectCards  DamageResolutionStage = "select_cards"
	DamageStageReveal       DamageResolutionStage = "reveal"
	DamageStageReaction     DamageResolutionStage = "reaction_chain"
	DamageStageConsequences DamageResolutionStage = "immediate_consequences"
	DamageStageComplete     DamageResolutionStage = "complete"
)

type ActorDefeatState string

const (
	ActorNotDefeated   ActorDefeatState = ""
	ActorPendingDefeat ActorDefeatState = "pending_defeat"
	ActorDefeated      ActorDefeatState = "defeated"
)

type DamageResolutionState struct {
	ID                       string
	Stage                    DamageResolutionStage
	Revision                 int
	SourceProposals          []DamageSourceProposal
	ModifierProposals        []DamageModifierProposal
	AccumulatedProposals     []AccumulatedDamageProposal
	CardProposals            []ProposedCardRemoval
	PendingOperations        []FinalizedOperationProposal
	AppliedReactionWindowIDs map[string]bool
	ReactionResolutionID     string
	Revealed                 bool
	Committed                bool
}

type DamageSourceProposal struct {
	ID                       string                     `json:"id"`
	SourcePlanningProposalID string                     `json:"source_planning_proposal_id,omitempty"`
	SourceActorID            string                     `json:"source_actor_id"`
	SourceContentType        string                     `json:"source_content_type,omitempty"`
	SourceContentID          string                     `json:"source_content_id,omitempty"`
	TargetActorID            string                     `json:"target_actor_id"`
	BaseAmount               int                        `json:"base_amount"`
	DefensivePrevention      int                        `json:"defensive_prevention,omitempty"`
	ReactionPrevention       int                        `json:"reaction_prevention,omitempty"`
	ReactionModification     int                        `json:"reaction_modification,omitempty"`
	FinalAmount              int                        `json:"final_amount"`
	OriginatingOperation     FinalizedOperationProposal `json:"originating_operation"`
}

type DamageModifierProposal struct {
	ID                       string                     `json:"id"`
	SourcePlanningProposalID string                     `json:"source_planning_proposal_id,omitempty"`
	SourceActorID            string                     `json:"source_actor_id"`
	SourceContentType        string                     `json:"source_content_type,omitempty"`
	SourceContentID          string                     `json:"source_content_id,omitempty"`
	TargetProposalIDs        []string                   `json:"target_proposal_ids"`
	Amount                   int                        `json:"amount"`
	OriginatingOperation     FinalizedOperationProposal `json:"originating_operation"`
}

type AccumulatedDamageProposal struct {
	ID                 string   `json:"id"`
	TargetActorID      string   `json:"target_actor_id"`
	SourceProposalIDs  []string `json:"source_proposal_ids"`
	BaseAmount         int      `json:"base_amount"`
	ReactionPrevention int      `json:"reaction_prevention,omitempty"`
	FinalAmount        int      `json:"final_amount"`
}

type ProposedCardRemoval struct {
	ID                       string             `json:"id"`
	TargetActorID            string             `json:"target_actor_id"`
	CardID                   string             `json:"card_id"`
	OriginalZone             operation.CardZone `json:"original_zone"`
	DamageProposalIDs        []string           `json:"damage_proposal_ids,omitempty"`
	SourceActorIDs           []string           `json:"source_actor_ids,omitempty"`
	Sequence                 int                `json:"sequence"`
	Revealed                 bool               `json:"revealed"`
	Accepted                 bool               `json:"accepted"`
	Released                 bool               `json:"released,omitempty"`
	ReplacementForProposalID string             `json:"replacement_for_proposal_id,omitempty"`
}

type DamageReactionType string

const (
	DamageReactionPreventAccumulated DamageReactionType = "prevent_accumulated_damage"
	DamageReactionPreventSource      DamageReactionType = "prevent_damage_source"
	DamageReactionModifySource       DamageReactionType = "modify_damage_source"
	DamageReactionReplaceCard        DamageReactionType = "replace_damage_card"
)

type DamageReaction struct {
	Type              DamageReactionType `json:"type"`
	ProposalID        string             `json:"proposal_id"`
	Amount            int                `json:"amount,omitempty"`
	ReplacementCardID string             `json:"replacement_card_id,omitempty"`
}

func (actor ActorState) CurrentHealth() int {
	return len(actor.Cards.Deck) + len(actor.Cards.Discard) + len(actor.Cards.Hand)
}

func cloneDamageResolution(value *DamageResolutionState) *DamageResolutionState {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.SourceProposals = append([]DamageSourceProposal(nil), value.SourceProposals...)
	for i := range cloned.SourceProposals {
		cloned.SourceProposals[i].OriginatingOperation = cloneFinalizedOperation(value.SourceProposals[i].OriginatingOperation)
	}
	cloned.ModifierProposals = append([]DamageModifierProposal(nil), value.ModifierProposals...)
	for i := range cloned.ModifierProposals {
		cloned.ModifierProposals[i].TargetProposalIDs = copyStrings(value.ModifierProposals[i].TargetProposalIDs)
		cloned.ModifierProposals[i].OriginatingOperation = cloneFinalizedOperation(value.ModifierProposals[i].OriginatingOperation)
	}
	cloned.AccumulatedProposals = append([]AccumulatedDamageProposal(nil), value.AccumulatedProposals...)
	for i := range cloned.AccumulatedProposals {
		cloned.AccumulatedProposals[i].SourceProposalIDs = copyStrings(value.AccumulatedProposals[i].SourceProposalIDs)
	}
	cloned.CardProposals = append([]ProposedCardRemoval(nil), value.CardProposals...)
	for i := range cloned.CardProposals {
		cloned.CardProposals[i].DamageProposalIDs = copyStrings(value.CardProposals[i].DamageProposalIDs)
		cloned.CardProposals[i].SourceActorIDs = copyStrings(value.CardProposals[i].SourceActorIDs)
	}
	cloned.PendingOperations = append([]FinalizedOperationProposal(nil), value.PendingOperations...)
	for i := range cloned.PendingOperations {
		cloned.PendingOperations[i] = cloneFinalizedOperation(value.PendingOperations[i])
	}
	cloned.AppliedReactionWindowIDs = make(map[string]bool, len(value.AppliedReactionWindowIDs))
	for id, applied := range value.AppliedReactionWindowIDs {
		cloned.AppliedReactionWindowIDs[id] = applied
	}
	return &cloned
}

func cloneFinalizedOperation(value FinalizedOperationProposal) FinalizedOperationProposal {
	value.Operation = operation.ClonePlans([]operation.Plan{value.Operation})[0]
	value.SelectedTargets = copyStrings(value.SelectedTargets)
	return value
}
