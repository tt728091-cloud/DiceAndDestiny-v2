package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/state"
)

type Result struct {
	Accepted bool      `json:"accepted"`
	Events   []Event   `json:"events,omitempty"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type Event struct {
	Type          string `json:"type"`
	From          string `json:"from,omitempty"`
	To            string `json:"to,omitempty"`
	Round         int    `json:"round,omitempty"`
	CompletedTurn bool   `json:"completed_turn,omitempty"`
}

type Snapshot struct {
	BattleID string `json:"battle_id"`
	Segment  string `json:"segment"`
	Round    int    `json:"round"`
}

func (e Engine) HandleCommand(cmd command.Command) Result {
	switch cmd.Type {
	case command.TypeAdvanceSegment:
		return e.handleAdvanceSegment(cmd)
	default:
		return rejected("unsupported command type")
	}
}

func (e Engine) handleAdvanceSegment(cmd command.Command) Result {
	var payload command.AdvanceSegmentPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return rejected("invalid advance_segment payload")
	}

	battle, err := state.NewBattle(cmd.BattleID)
	if err != nil {
		return rejected(err.Error())
	}

	advanced, err := e.AdvanceSegment(&battle)
	if err != nil {
		return rejected(err.Error())
	}

	return Result{
		Accepted: true,
		Events: []Event{
			{
				Type:          "segment_advanced",
				From:          advanced.Advance.From.String(),
				To:            advanced.Advance.To.String(),
				Round:         advanced.Advance.Round,
				CompletedTurn: advanced.Advance.CompletedTurn,
			},
		},
		Snapshot: &Snapshot{
			BattleID: battle.ID,
			Segment:  battle.Segment.Current.String(),
			Round:    battle.Segment.Round,
		},
	}
}

func rejected(message string) Result {
	return Result{
		Accepted: false,
		Error:    message,
	}
}
