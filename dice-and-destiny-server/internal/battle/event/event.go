package event

import "diceanddestiny/server/internal/battle/segment"

// Type is the stable event name clients can key presentation or logs from.
type Type string

const (
	TypeSegmentAdvanced Type = "segment_advanced"
	TypeSegmentEntered  Type = "segment_entered"
)

// Event describes an authority-approved battle fact that already happened.
// Keep UI intent and transport details out of this package.
type Event struct {
	Type          Type            `json:"type"`
	From          segment.Segment `json:"from,omitempty"`
	To            segment.Segment `json:"to,omitempty"`
	Segment       segment.Segment `json:"segment,omitempty"`
	Round         int             `json:"round,omitempty"`
	CompletedTurn bool            `json:"completed_turn,omitempty"`
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
