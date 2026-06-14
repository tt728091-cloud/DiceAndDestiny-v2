package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const automaticStage = "automatic"

type OngoingEffectsFlow struct{}

func (OngoingEffectsFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (OngoingEffectsFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	initializeAutomaticActors(ctx.Battle)
	return nil, nil
}

func (OngoingEffectsFlow) Progress(ctx *Context) (ProgressResult, error) {
	resolveAutomaticActors(ctx.Battle)
	return progress(ProgressSegmentComplete), nil
}

func (OngoingEffectsFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	return nil, unsupportedCommand()
}

func (OngoingEffectsFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return nil, nil
}

type DamageResolutionFlow struct{}

func (DamageResolutionFlow) ID() segment.Segment {
	return segment.DamageResolution
}

func (DamageResolutionFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	initializeAutomaticActors(ctx.Battle)
	return nil, nil
}

func (DamageResolutionFlow) Progress(ctx *Context) (ProgressResult, error) {
	resolveAutomaticActors(ctx.Battle)
	return progress(ProgressSegmentComplete), nil
}

func (DamageResolutionFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	return nil, unsupportedCommand()
}

func (DamageResolutionFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return nil, nil
}

func initializeAutomaticActors(battle *state.Battle) {
	battle.Flow.Stage = automaticStage
	battle.Flow.Iteration = 1
	for actorID := range battle.Actors {
		battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolvingAutomatic}
	}
}

func resolveAutomaticActors(battle *state.Battle) {
	for actorID, actor := range battle.Flow.Actors {
		actor.Status = state.ActorResolved
		battle.Flow.Actors[actorID] = actor
	}
}
