package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

type Result struct {
	Accepted bool             `json:"accepted"`
	Events   []event.Event    `json:"events,omitempty"`
	Snapshot *snapshot.Battle `json:"snapshot,omitempty"`
	Error    string           `json:"error,omitempty"`
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

	events := make([]event.Event, 0, len(advanced.Exit.Events)+1+len(advanced.Enter.Events))
	events = append(events, advanced.Exit.Events...)
	events = append(events, event.NewSegmentAdvanced(advanced.Advance))
	events = append(events, advanced.Enter.Events...)

	return Result{
		Accepted: true,
		// Events describe what changed; snapshots describe state after the change.
		// The shared packages own those shapes so authority only serializes them.
		Events:   events,
		Snapshot: battleSnapshot(&battle),
	}
}

func battleSnapshot(battle *state.Battle) *snapshot.Battle {
	if battle == nil {
		return nil
	}

	// Copy mutable authoritative state into the read-only client/network shape.
	snap := snapshot.FromBattle(*battle)
	return &snap
}

func rejected(message string) Result {
	return Result{
		Accepted: false,
		Error:    message,
	}
}
