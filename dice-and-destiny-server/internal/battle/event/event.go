package event

import (
	"sort"

	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Type is the stable event name clients can key presentation or logs from.
type Type string

const SchemaVersion = 1

const (
	TypeSegmentAdvanced         Type = "segment_advanced"
	TypeSegmentEntered          Type = "segment_entered"
	TypeCardsDrawn              Type = "cards_drawn"
	TypeDiscardReshuffled       Type = "discard_reshuffled"
	TypeEnergyPointsGained      Type = "energy_points_gained"
	TypeRollRequested           Type = "roll_requested"
	TypeDiceRolled              Type = "dice_rolled"
	TypeInteractionCommitted    Type = "interaction_committed"
	TypeInteractionRevealed     Type = "interaction_revealed"
	TypeInteractionWindowOpened Type = "interaction_window_opened"
	TypeProposalBatchRevealed   Type = "proposal_batch_revealed"
	TypeProposalBatchCommitted  Type = "proposal_batch_committed"
	TypeResolutionCompleted     Type = "resolution_completed"
	TypeDamageProposed          Type = "damage_proposed"
	TypeDamageCardsRevealed     Type = "damage_cards_revealed"
	TypeDamageModified          Type = "damage_prevented_or_modified"
	TypeDamageCommitted         Type = "damage_committed"
	TypeCardsPermanentlyRemoved Type = "cards_permanently_removed"
	TypeBattleCompleted         Type = "battle_completed"
	TypeCardPlayed              Type = "card_played"
	TypeAbilitySelected         Type = "ability_selected"
	TypeStatusChanged           Type = "status_changed"
	TypeEnemyPlanned            Type = "enemy_planned"
	TypeDefenseSelected         Type = "defense_selected"
)

// Event describes an authority-approved battle fact that already happened.
// Keep UI intent and transport details out of this package.
type Event struct {
	ID             string                        `json:"event_id,omitempty"`
	BattleID       string                        `json:"battle_id,omitempty"`
	SchemaVersion  int                           `json:"schema_version,omitempty"`
	Sequence       uint64                        `json:"sequence,omitempty"`
	Type           Type                          `json:"type"`
	ActorID        string                        `json:"actor_id,omitempty"`
	From           segment.Segment               `json:"from,omitempty"`
	To             segment.Segment               `json:"to,omitempty"`
	Segment        segment.Segment               `json:"segment,omitempty"`
	Round          int                           `json:"round,omitempty"`
	CompletedTurn  bool                          `json:"completed_turn,omitempty"`
	Cards          []string                      `json:"cards,omitempty"`
	DeckEmpty      bool                          `json:"deck_empty,omitempty"`
	Count          int                           `json:"count,omitempty"`
	EnergyPoints   int                           `json:"energy_points,omitempty"`
	RequestID      string                        `json:"request_id,omitempty"`
	PendingInputID string                        `json:"pending_input_id,omitempty"`
	Pool           state.RollPool                `json:"pool,omitempty"`
	SourceType     state.RollSourceType          `json:"source_type,omitempty"`
	SourceID       string                        `json:"source_id,omitempty"`
	Dice           []state.RolledDie             `json:"dice,omitempty"`
	RolledIndices  []int                         `json:"rolled_indices,omitempty"`
	RollsUsed      int                           `json:"rolls_used,omitempty"`
	MaxRolls       int                           `json:"max_rolls,omitempty"`
	RollsRemaining *int                          `json:"rolls_remaining,omitempty"`
	Combinations   []string                      `json:"combinations,omitempty"`
	SymbolCounts   map[string]int                `json:"symbol_counts,omitempty"`
	ResolutionID   string                        `json:"resolution_id,omitempty"`
	WindowID       string                        `json:"window_id,omitempty"`
	Purpose        state.InteractionPurpose      `json:"purpose,omitempty"`
	ReactionRound  int                           `json:"reaction_round,omitempty"`
	ChainDepth     int                           `json:"chain_depth,omitempty"`
	Commitment     *state.InteractionCommitment  `json:"commitment,omitempty"`
	Commitments    []state.InteractionCommitment `json:"commitments,omitempty"`
	ProposalBatch  *state.ProposalBatch          `json:"proposal_batch,omitempty"`
	ProposalID     string                        `json:"proposal_id,omitempty"`
	TargetActorID  string                        `json:"target_actor_id,omitempty"`
	Amount         int                           `json:"amount,omitempty"`
	OriginalZone   string                        `json:"original_zone,omitempty"`
	DamageCards    []state.ProposedCardRemoval   `json:"damage_cards,omitempty"`
	BattleResult   state.BattleStatus            `json:"battle_result,omitempty"`
	PrivateActorID string                        `json:"private_actor_id,omitempty"`
	Data           map[string]any                `json:"data,omitempty"`
}

func NewBattleCompleted(result state.BattleStatus) Event {
	return Event{
		Type:         TypeBattleCompleted,
		BattleResult: result,
	}
}

func NewDamageProposed(proposal state.DamageSourceProposal) Event {
	return Event{
		Type:          TypeDamageProposed,
		ActorID:       proposal.SourceActorID,
		SourceType:    state.RollSourceType(proposal.SourceContentType),
		SourceID:      proposal.SourceContentID,
		ProposalID:    proposal.ID,
		TargetActorID: proposal.TargetActorID,
		Amount:        proposal.BaseAmount,
	}
}

func NewDamageCardsRevealed(targetActorID string, cards []state.ProposedCardRemoval) Event {
	return Event{
		Type:          TypeDamageCardsRevealed,
		TargetActorID: targetActorID,
		DamageCards:   copyDamageCards(cards),
	}
}

func NewDamageModified(proposalID, targetActorID string, amount int) Event {
	return Event{
		Type:          TypeDamageModified,
		ProposalID:    proposalID,
		TargetActorID: targetActorID,
		Amount:        amount,
	}
}

func NewDamageCommitted(
	proposalID string,
	sourceActorID string,
	sourceID string,
	targetActorID string,
	amount int,
) Event {
	return Event{
		Type:          TypeDamageCommitted,
		ActorID:       sourceActorID,
		SourceID:      sourceID,
		ProposalID:    proposalID,
		TargetActorID: targetActorID,
		Amount:        amount,
	}
}

func NewCardPermanentlyRemoved(proposal state.ProposedCardRemoval) Event {
	return Event{
		Type:          TypeCardsPermanentlyRemoved,
		ProposalID:    proposal.ID,
		TargetActorID: proposal.TargetActorID,
		Cards:         []string{proposal.CardID},
		OriginalZone:  string(proposal.OriginalZone),
		DamageCards:   []state.ProposedCardRemoval{copyDamageCard(proposal)},
	}
}

func NewInteractionCommitted(
	resolutionID string,
	window state.InteractionWindow,
	commitment state.InteractionCommitment,
	privateActorID string,
) Event {
	copied := copyInteractionCommitment(commitment)
	return Event{
		Type:           TypeInteractionCommitted,
		ActorID:        commitment.ActorID,
		ResolutionID:   resolutionID,
		WindowID:       window.ID,
		Purpose:        window.Purpose,
		ReactionRound:  window.ReactionRound,
		ChainDepth:     window.ChainDepth,
		Commitment:     &copied,
		PrivateActorID: privateActorID,
	}
}

func NewInteractionRevealed(resolutionID string, window state.InteractionWindow) Event {
	commitments := make([]state.InteractionCommitment, 0, len(window.Commitments))
	actorIDs := make([]string, 0, len(window.Commitments))
	for actorID := range window.Commitments {
		actorIDs = append(actorIDs, actorID)
	}
	sort.Strings(actorIDs)
	for _, actorID := range actorIDs {
		commitments = append(commitments, copyInteractionCommitment(window.Commitments[actorID]))
	}
	return Event{
		Type:          TypeInteractionRevealed,
		ResolutionID:  resolutionID,
		WindowID:      window.ID,
		Purpose:       window.Purpose,
		ReactionRound: window.ReactionRound,
		ChainDepth:    window.ChainDepth,
		Commitments:   commitments,
	}
}

func NewInteractionWindowOpened(resolutionID string, window state.InteractionWindow) Event {
	return Event{
		Type:          TypeInteractionWindowOpened,
		ResolutionID:  resolutionID,
		WindowID:      window.ID,
		Purpose:       window.Purpose,
		ReactionRound: window.ReactionRound,
		ChainDepth:    window.ChainDepth,
	}
}

func NewProposalBatchCommitted(resolution state.ResolutionState) Event {
	batch := copyProposalBatch(resolution.Batch)
	return Event{
		Type:          TypeProposalBatchCommitted,
		ResolutionID:  resolution.ID,
		ProposalBatch: &batch,
	}
}

func NewProposalBatchRevealed(resolution state.ResolutionState) Event {
	batch := copyProposalBatch(resolution.Batch)
	return Event{
		Type:          TypeProposalBatchRevealed,
		ResolutionID:  resolution.ID,
		ProposalBatch: &batch,
	}
}

func NewResolutionCompleted(resolution state.ResolutionState) Event {
	return Event{
		Type:         TypeResolutionCompleted,
		ResolutionID: resolution.ID,
	}
}

// NewSegmentAdvanced converts segment progression data into the public event
// shape without making authority understand segment semantics.
func NewSegmentAdvanced(advance segment.Advance) Event {
	return Event{
		Type:          TypeSegmentAdvanced,
		From:          advance.From,
		To:            advance.To,
		Round:         advance.Round,
		CompletedTurn: advance.CompletedTurn,
	}
}

// NewSegmentEntered describes the current segment after state has changed.
func NewSegmentEntered(state segment.State) Event {
	return Event{
		Type:    TypeSegmentEntered,
		Segment: state.Current,
		Round:   state.Round,
	}
}

func NewCardsDrawn(actorID string, cards []string, deckEmpty bool) Event {
	return Event{
		Type:      TypeCardsDrawn,
		ActorID:   actorID,
		Cards:     append([]string(nil), cards...),
		DeckEmpty: deckEmpty,
	}
}

func NewDiscardReshuffled(actorID string, count int) Event {
	return Event{
		Type:    TypeDiscardReshuffled,
		ActorID: actorID,
		Count:   count,
	}
}

func NewEnergyPointsGained(actorID string, points int) Event {
	return Event{
		Type:         TypeEnergyPointsGained,
		ActorID:      actorID,
		EnergyPoints: points,
	}
}

func NewRollRequested(actorID string, segmentID segment.Segment, requestID string, pendingInputID string) Event {
	return Event{
		Type:           TypeRollRequested,
		ActorID:        actorID,
		Segment:        segmentID,
		RequestID:      requestID,
		PendingInputID: pendingInputID,
	}
}

func NewDiceRolled(
	actorID string,
	segmentID segment.Segment,
	requestID string,
	pool state.RollPool,
	sourceType state.RollSourceType,
	sourceID string,
	dice []state.RolledDie,
	rolledIndices []int,
	rollsUsed int,
	maxRolls int,
	combinations []string,
	symbolCounts map[string]int,
) Event {
	return Event{
		Type:           TypeDiceRolled,
		ActorID:        actorID,
		Segment:        segmentID,
		RequestID:      requestID,
		Pool:           pool,
		SourceType:     sourceType,
		SourceID:       sourceID,
		Dice:           copyRolledDice(dice),
		RolledIndices:  append([]int(nil), rolledIndices...),
		RollsUsed:      rollsUsed,
		MaxRolls:       maxRolls,
		RollsRemaining: intPtr(maxRolls - rollsUsed),
		Combinations:   append([]string(nil), combinations...),
		SymbolCounts:   copySymbolCounts(symbolCounts),
	}
}

func intPtr(value int) *int {
	return &value
}

// ForViewer returns a viewer-safe copy of battle events.
// Raw events remain authoritative facts; this helper hides card IDs that are
// not visible to the requested viewer.
func ForViewer(events []Event, viewerActorID string) []Event {
	filtered := make([]Event, len(events))
	for i, event := range events {
		filtered[i] = eventForViewer(event, viewerActorID)
	}
	return filtered
}

func eventForViewer(source Event, viewerActorID string) Event {
	filtered := source
	filtered.Cards = append([]string(nil), source.Cards...)
	filtered.Dice = copyRolledDice(source.Dice)
	filtered.RolledIndices = append([]int(nil), source.RolledIndices...)
	filtered.Combinations = append([]string(nil), source.Combinations...)
	filtered.SymbolCounts = copySymbolCounts(source.SymbolCounts)
	filtered.Commitment = copyInteractionCommitmentPointer(source.Commitment)
	filtered.Commitments = copyInteractionCommitments(source.Commitments)
	filtered.ProposalBatch = copyProposalBatchPointer(source.ProposalBatch)
	filtered.DamageCards = copyDamageCards(source.DamageCards)

	if source.Type == TypeCardsDrawn && source.ActorID != viewerActorID {
		filtered.Count = len(source.Cards)
		filtered.Cards = nil
	}
	if source.Type == TypeRollRequested && source.ActorID != viewerActorID {
		filtered.RequestID = ""
		filtered.PendingInputID = ""
	}
	if source.Type == TypeDiceRolled && source.ActorID != viewerActorID {
		filtered.RequestID = ""
		filtered.Pool = ""
		filtered.SourceType = ""
		filtered.SourceID = ""
		filtered.Dice = nil
		filtered.RolledIndices = nil
		filtered.RollsUsed = 0
		filtered.MaxRolls = 0
		filtered.RollsRemaining = nil
		filtered.Combinations = nil
		filtered.SymbolCounts = nil
	}
	if source.PrivateActorID != "" && source.PrivateActorID != viewerActorID {
		filtered.Commitment = nil
	}
	if source.Type == TypeInteractionRevealed {
		for i := range filtered.Commitments {
			if filtered.Commitments[i].ActorID != viewerActorID &&
				filtered.Commitments[i].Data.Planning != nil {
				filtered.Commitments[i].Data.Planning.KeptIndices = nil
			}
		}
	}
	if filtered.ProposalBatch != nil {
		for i := range filtered.ProposalBatch.Proposals {
			planning := filtered.ProposalBatch.Proposals[i].Data.Planning
			if planning != nil &&
				filtered.ProposalBatch.Proposals[i].Source.ActorID != viewerActorID {
				planning.KeptIndices = nil
			}
		}
	}
	filtered.PrivateActorID = ""

	return filtered
}

func copyInteractionCommitmentPointer(
	value *state.InteractionCommitment,
) *state.InteractionCommitment {
	if value == nil {
		return nil
	}
	copied := copyInteractionCommitment(*value)
	return &copied
}

func copyInteractionCommitments(
	values []state.InteractionCommitment,
) []state.InteractionCommitment {
	if values == nil {
		return nil
	}
	copied := make([]state.InteractionCommitment, len(values))
	for i, value := range values {
		copied[i] = copyInteractionCommitment(value)
	}
	return copied
}

func copyInteractionCommitment(
	value state.InteractionCommitment,
) state.InteractionCommitment {
	copied := value
	copied.Data.ProposalIDs = copyStrings(value.Data.ProposalIDs)
	copied.Data.CardIDs = copyStrings(value.Data.CardIDs)
	copied.Data.TargetIDs = copyStrings(value.Data.TargetIDs)
	if value.Data.Value != nil {
		amount := *value.Data.Value
		copied.Data.Value = &amount
	}
	if value.Data.Planning != nil {
		planning := copyPlanningCommitment(*value.Data.Planning)
		copied.Data.Planning = &planning
	}
	copied.Data.PlanningAdjustments = append(
		[]state.PlanningAdjustment(nil),
		value.Data.PlanningAdjustments...,
	)
	copied.Data.DamageReactions = append(
		[]state.DamageReaction(nil),
		value.Data.DamageReactions...,
	)
	return copied
}

func copyProposalBatchPointer(value *state.ProposalBatch) *state.ProposalBatch {
	if value == nil {
		return nil
	}
	copied := copyProposalBatch(*value)
	return &copied
}

func copyProposalBatch(value state.ProposalBatch) state.ProposalBatch {
	copied := value
	copied.Proposals = make([]state.Proposal, len(value.Proposals))
	for i, proposal := range value.Proposals {
		copied.Proposals[i] = proposal
		if proposal.Data.Amount != nil {
			amount := *proposal.Data.Amount
			copied.Proposals[i].Data.Amount = &amount
		}
		if proposal.Data.Selection != nil {
			selection := *proposal.Data.Selection
			selection.OptionIDs = copyStrings(proposal.Data.Selection.OptionIDs)
			copied.Proposals[i].Data.Selection = &selection
		}
		if proposal.Data.Roll != nil {
			roll := *proposal.Data.Roll
			roll.Dice = copyRolledDice(proposal.Data.Roll.Dice)
			copied.Proposals[i].Data.Roll = &roll
		}
		if proposal.Data.Planning != nil {
			planning := copyPlanningCommitment(*proposal.Data.Planning)
			copied.Proposals[i].Data.Planning = &planning
		}
	}
	return copied
}

func copyPlanningCommitment(value state.PlanningCommitmentData) state.PlanningCommitmentData {
	value.FinalDice = copyRolledDice(value.FinalDice)
	value.KeptIndices = append([]int(nil), value.KeptIndices...)
	value.CommittedCards = copyStrings(value.CommittedCards)
	value.SelectedTargets = copyStrings(value.SelectedTargets)
	return value
}

func copyRolledDice(values []state.RolledDie) []state.RolledDie {
	if values == nil {
		return nil
	}
	copied := make([]state.RolledDie, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Symbols = copyStrings(value.Symbols)
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func copyDamageCards(values []state.ProposedCardRemoval) []state.ProposedCardRemoval {
	if values == nil {
		return nil
	}
	copied := make([]state.ProposedCardRemoval, len(values))
	for i, value := range values {
		copied[i] = copyDamageCard(value)
	}
	return copied
}

func copyDamageCard(value state.ProposedCardRemoval) state.ProposedCardRemoval {
	value.DamageProposalIDs = copyStrings(value.DamageProposalIDs)
	value.SourceActorIDs = copyStrings(value.SourceActorIDs)
	return value
}

func copySymbolCounts(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
