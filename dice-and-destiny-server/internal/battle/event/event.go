package event

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Type is the stable event name clients can key presentation or logs from.
type Type string

const (
	TypeSegmentAdvanced    Type = "segment_advanced"
	TypeSegmentEntered     Type = "segment_entered"
	TypeCardsDrawn         Type = "cards_drawn"
	TypeDiscardReshuffled  Type = "discard_reshuffled"
	TypeEnergyPointsGained Type = "energy_points_gained"
	TypeRollRequested      Type = "roll_requested"
	TypeDiceRolled         Type = "dice_rolled"
)

// Event describes an authority-approved battle fact that already happened.
// Keep UI intent and transport details out of this package.
type Event struct {
	Type           Type                 `json:"type"`
	ActorID        string               `json:"actor_id,omitempty"`
	From           segment.Segment      `json:"from,omitempty"`
	To             segment.Segment      `json:"to,omitempty"`
	Segment        segment.Segment      `json:"segment,omitempty"`
	Round          int                  `json:"round,omitempty"`
	CompletedTurn  bool                 `json:"completed_turn,omitempty"`
	Cards          []string             `json:"cards,omitempty"`
	DeckEmpty      bool                 `json:"deck_empty,omitempty"`
	Count          int                  `json:"count,omitempty"`
	EnergyPoints   int                  `json:"energy_points,omitempty"`
	RequestID      string               `json:"request_id,omitempty"`
	PendingInputID string               `json:"pending_input_id,omitempty"`
	Pool           state.RollPool       `json:"pool,omitempty"`
	SourceType     state.RollSourceType `json:"source_type,omitempty"`
	SourceID       string               `json:"source_id,omitempty"`
	Dice           []state.RolledDie    `json:"dice,omitempty"`
	RolledIndices  []int                `json:"rolled_indices,omitempty"`
	RollsUsed      int                  `json:"rolls_used,omitempty"`
	MaxRolls       int                  `json:"max_rolls,omitempty"`
	RollsRemaining *int                 `json:"rolls_remaining,omitempty"`
	Combinations   []string             `json:"combinations,omitempty"`
	SymbolCounts   map[string]int       `json:"symbol_counts,omitempty"`
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

	return filtered
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
