package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

type Context struct {
	Battle *state.Battle
	Phase  state.FlowPhase
}

type ProgressStatus string

const (
	ProgressContinue        ProgressStatus = "continue"
	ProgressWaitingForInput ProgressStatus = "waiting_for_input"
	ProgressSegmentComplete ProgressStatus = "segment_complete"
	ProgressBattleComplete  ProgressStatus = "battle_complete"
)

type ProgressResult struct {
	Status ProgressStatus
	Events []event.Event
}

type SegmentFlow interface {
	ID() segment.Segment
	OnEnter(ctx *Context) ([]event.Event, error)
	Progress(ctx *Context) (ProgressResult, error)
	HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error)
	OnExit(ctx *Context) ([]event.Event, error)
}

func progress(status ProgressStatus, events ...event.Event) ProgressResult {
	return ProgressResult{Status: status, Events: events}
}
