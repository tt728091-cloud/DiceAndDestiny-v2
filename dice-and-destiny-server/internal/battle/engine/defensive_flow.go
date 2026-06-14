package engine

import (
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
)

type DefensiveFlow struct{}

func (DefensiveFlow) ID() segment.Segment {
	return segment.Defensive
}

func (DefensiveFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	initializeAutomaticActors(ctx.Battle)
	return nil, nil
}

func (DefensiveFlow) Progress(ctx *Context) (ProgressResult, error) {
	resolveAutomaticActors(ctx.Battle)
	return progress(ProgressSegmentComplete), nil
}

func (DefensiveFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return nil, nil
}

func (DefensiveFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	if cmd.Type != command.TypeRollDice {
		return nil, unsupportedCommand()
	}

	var payload command.RollDicePayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, fmt.Errorf("invalid roll_dice payload")
	}
	return dice.Roll(ctx.Battle, payload.RequestID, cmd.ActorID, payload.RerollIndices)
}
