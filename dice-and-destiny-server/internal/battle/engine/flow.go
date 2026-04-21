package engine

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

type Context struct {
	Battle *state.Battle
}

type FlowDecision string

const (
	WaitForCommand FlowDecision = "wait_for_command"
	ReadyToAdvance FlowDecision = "ready_to_advance"
)

type FlowResult struct {
	Decision FlowDecision
}

type SegmentFlow interface {
	// ID ties a behavior object to the stable segment data owned by the segment package.
	ID() segment.Segment
	OnEnter(ctx *Context) (FlowResult, error)
	CanAdvance(ctx *Context) (FlowDecision, error)
	OnExit(ctx *Context) (FlowResult, error)
}

func readyResult() FlowResult {
	return FlowResult{Decision: ReadyToAdvance}
}
