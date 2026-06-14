package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/income"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const incomeStageRewards = "rewards"

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

func (flow IncomeFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = incomeStageRewards
	ctx.Battle.Flow.Iteration = 1
	for actorID := range ctx.Battle.Actors {
		ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolved}
	}
	return income.ApplyRewards(ctx.Battle, flow.rewards)
}

func (IncomeFlow) Progress(ctx *Context) (ProgressResult, error) {
	return progress(ProgressSegmentComplete), nil
}

func (IncomeFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	return nil, unsupportedCommand()
}

func (IncomeFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return nil, nil
}
