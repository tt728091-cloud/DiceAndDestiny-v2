package engine

import (
	"diceanddestiny/server/internal/battle/income"
	"diceanddestiny/server/internal/battle/segment"
)

type IncomeFlow struct {
	rewards []income.Reward
}

func NewIncomeFlow(rewards ...income.Reward) (IncomeFlow, error) {
	copied, err := income.NewRewards(rewards...)
	if err != nil {
		return IncomeFlow{}, err
	}

	return IncomeFlow{rewards: copied}, nil
}

func (IncomeFlow) ID() segment.Segment {
	return segment.Income
}

func (flow IncomeFlow) OnEnter(ctx *Context) (FlowResult, error) {
	events, err := income.ApplyRewards(ctx.Battle, flow.rewards)
	if err != nil {
		return FlowResult{}, err
	}

	return FlowResult{
		Events:   events,
		Decision: ReadyToAdvance,
	}, nil
}

func (IncomeFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (IncomeFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}
