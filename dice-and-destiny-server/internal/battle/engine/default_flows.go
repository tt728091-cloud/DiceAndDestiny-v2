package engine

import "diceanddestiny/server/internal/battle/segment"

type OngoingEffectsFlow struct{}

func (OngoingEffectsFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (OngoingEffectsFlow) OnEnter(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (OngoingEffectsFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (OngoingEffectsFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

type IncomeFlow struct{}

func (IncomeFlow) ID() segment.Segment {
	return segment.Income
}

func (IncomeFlow) OnEnter(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (IncomeFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (IncomeFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

type OffensiveFlow struct{}

func (OffensiveFlow) ID() segment.Segment {
	return segment.Offensive
}

func (OffensiveFlow) OnEnter(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (OffensiveFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (OffensiveFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

type DefensiveFlow struct{}

func (DefensiveFlow) ID() segment.Segment {
	return segment.Defensive
}

func (DefensiveFlow) OnEnter(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (DefensiveFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (DefensiveFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

type DamageResolutionFlow struct{}

func (DamageResolutionFlow) ID() segment.Segment {
	return segment.DamageResolution
}

func (DamageResolutionFlow) OnEnter(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (DamageResolutionFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (DamageResolutionFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func DefaultFlows() []SegmentFlow {
	return []SegmentFlow{
		OngoingEffectsFlow{},
		IncomeFlow{},
		OffensiveFlow{},
		DefensiveFlow{},
		DamageResolutionFlow{},
	}
}
