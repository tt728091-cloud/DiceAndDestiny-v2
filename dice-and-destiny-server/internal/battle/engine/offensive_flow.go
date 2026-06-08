package engine

import (
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

type OffensiveFlow struct{}

func (OffensiveFlow) ID() segment.Segment {
	return segment.Offensive
}

func (OffensiveFlow) OnEnter(ctx *Context) (FlowResult, error) {
	if ctx == nil || ctx.Battle == nil {
		return FlowResult{}, nil
	}

	if ctx.Battle.RollRequests == nil {
		ctx.Battle.RollRequests = map[string]state.RollRequest{}
	}

	waiting := false
	for actorID, actor := range ctx.Battle.Actors {
		if len(actor.DiceLoadout) == 0 {
			continue
		}

		requestID := fmt.Sprintf("roll-%s-offensive-%d", actorID, ctx.Battle.Segment.Round)
		if _, exists := ctx.Battle.RollRequests[requestID]; exists {
			waiting = true
			continue
		}

		ctx.Battle.RollRequests[requestID] = state.RollRequest{
			ID:          requestID,
			ActorID:     actorID,
			Segment:     segment.Offensive,
			Pool:        state.RollPoolOffensive,
			SourceType:  state.RollSourceSegment,
			SourceID:    string(segment.Offensive),
			DiceLoadout: append([]state.DiceLoadoutEntry(nil), actor.DiceLoadout...),
			MaxRolls:    3,
			Required:    false,
		}
		waiting = true
	}

	if waiting {
		return FlowResult{Decision: WaitForCommand}, nil
	}
	return readyResult(), nil
}

func (OffensiveFlow) CanAdvance(ctx *Context) (FlowDecision, error) {
	return ReadyToAdvance, nil
}

func (OffensiveFlow) OnExit(ctx *Context) (FlowResult, error) {
	return readyResult(), nil
}

func (OffensiveFlow) HandleCommand(ctx *Context, cmd command.Command) (FlowResult, error) {
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
