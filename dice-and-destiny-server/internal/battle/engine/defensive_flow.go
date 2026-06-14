package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
)

type DefensiveFlow struct {
	shared SharedPlanningFlow
}

func newDefensiveFlow() DefensiveFlow {
	shared, err := NewSharedPlanningFlow(segment.Defensive, nil)
	if err != nil {
		panic(err)
	}
	return DefensiveFlow{shared: shared}
}

func (DefensiveFlow) ID() segment.Segment {
	return segment.Defensive
}

func (flow DefensiveFlow) configured() SharedPlanningFlow {
	if flow.shared.segmentID == "" {
		return newDefensiveFlow().shared
	}
	return flow.shared
}

func (flow DefensiveFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	return flow.configured().OnEnter(ctx)
}

func (flow DefensiveFlow) Progress(ctx *Context) (ProgressResult, error) {
	return flow.configured().Progress(ctx)
}

func (flow DefensiveFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	return flow.configured().HandleCommand(ctx, cmd)
}

func (flow DefensiveFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return flow.configured().OnExit(ctx)
}
