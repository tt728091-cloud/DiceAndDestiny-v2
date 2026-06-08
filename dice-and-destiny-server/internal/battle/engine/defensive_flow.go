package engine

import (
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/segment"
)

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

func (DefensiveFlow) HandleCommand(ctx *Context, cmd command.Command) (FlowResult, error) {
	if cmd.Type != command.TypeRollDice {
		return FlowResult{}, fmt.Errorf("unsupported command type")
	}

	var payload command.RollDicePayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return FlowResult{}, fmt.Errorf("invalid roll_dice payload")
	}

	events, err := dice.Roll(ctx.Battle, payload.RequestID, cmd.ActorID, payload.RerollIndices)
	if err != nil {
		return FlowResult{}, err
	}

	return FlowResult{
		Events:   events,
		Decision: WaitForCommand,
	}, nil
}
