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
