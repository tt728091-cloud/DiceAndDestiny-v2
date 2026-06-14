package engine

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const offensiveStagePlanning = "planning"

type OffensiveAIController interface {
	Plan(ctx *Context, actorID string) (state.OffensiveCommitment, error)
}

type DefaultOffensiveAIController struct{}

func (DefaultOffensiveAIController) Plan(ctx *Context, actorID string) (state.OffensiveCommitment, error) {
	return state.OffensiveCommitment{
		ID:      fmt.Sprintf("commit-%s-offensive-%d-%d", actorID, ctx.Battle.Segment.Round, ctx.Battle.Flow.Iteration),
		ActorID: actorID,
	}, nil
}

type OffensiveFlow struct {
	controller OffensiveAIController
}

func NewOffensiveFlow(controller OffensiveAIController) (OffensiveFlow, error) {
	if controller == nil {
		return OffensiveFlow{}, errors.New("offensive AI controller is required")
	}
	return OffensiveFlow{controller: controller}, nil
}

func defaultOffensiveFlow() OffensiveFlow {
	flow, err := NewOffensiveFlow(DefaultOffensiveAIController{})
	if err != nil {
		panic(err)
	}
	return flow
}

func (OffensiveFlow) ID() segment.Segment {
	return segment.Offensive
}

func (flow OffensiveFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	battle := ctx.Battle
	battle.Flow.Stage = offensiveStagePlanning
	battle.Flow.Iteration = 1
	if battle.RollRequests == nil {
		battle.RollRequests = make(map[string]state.RollRequest)
	}
	if battle.Commitments == nil {
		battle.Commitments = make(map[string]state.OffensiveCommitment)
	}

	var events []event.Event
	for _, actorID := range sortedActorIDs(battle.Actors) {
		actor := battle.Actors[actorID]
		if len(actor.DiceLoadout) == 0 {
			battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:     state.ActorNotParticipating,
				ReasonCode: "no_offensive_dice",
			}
			continue
		}

		switch actor.Controller {
		case state.ControllerHuman:
			requestID := fmt.Sprintf("roll-%s-offensive-%d-%d", actorID, battle.Segment.Round, battle.Flow.Iteration)
			inputID := fmt.Sprintf("input-%s-offensive-%d-%d", actorID, battle.Segment.Round, battle.Flow.Iteration)
			battle.RollRequests[requestID] = state.RollRequest{
				ID:          requestID,
				ActorID:     actorID,
				Segment:     segment.Offensive,
				Pool:        state.RollPoolOffensive,
				SourceType:  state.RollSourceSegment,
				SourceID:    string(segment.Offensive),
				DiceLoadout: append([]state.DiceLoadoutEntry(nil), actor.DiceLoadout...),
				MaxRolls:    3,
			}
			battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:     state.ActorNeedsInput,
				ReasonCode: "offensive_roll_available",
			}
			battle.Flow.PendingInput[actorID] = state.PendingInput{
				ID:              inputID,
				ActorID:         actorID,
				Segment:         segment.Offensive,
				Stage:           offensiveStagePlanning,
				Iteration:       battle.Flow.Iteration,
				InputType:       string(command.TypeRollDice),
				SourceType:      string(state.RollSourceSegment),
				SourceID:        requestID,
				AllowedCommands: []command.Type{command.TypeRollDice},
			}
			events = append(events, event.NewRollRequested(actorID, segment.Offensive, requestID, inputID))
		case state.ControllerAI:
			battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolvingAutomatic}
		case state.ControllerSystem:
			battle.Flow.Actors[actorID] = state.ActorFlowState{Status: state.ActorResolved}
		default:
			return nil, fmt.Errorf("invalid controller type %q for actor %q", actor.Controller, actorID)
		}
	}
	return events, nil
}

func (flow OffensiveFlow) Progress(ctx *Context) (ProgressResult, error) {
	battle := ctx.Battle
	for _, actorID := range sortedActorIDs(battle.Actors) {
		actorProgress := battle.Flow.Actors[actorID]
		if actorProgress.Status != state.ActorResolvingAutomatic {
			continue
		}

		commitment, err := flow.controller.Plan(ctx, actorID)
		if err != nil {
			return ProgressResult{}, err
		}
		if commitment.ID == "" || commitment.ActorID != actorID {
			return ProgressResult{}, fmt.Errorf("invalid AI commitment for actor %q", actorID)
		}
		battle.Commitments[commitment.ID] = commitment
		actorProgress.Status = state.ActorLockedIn
		actorProgress.ReasonCode = "ai_offensive_planned"
		actorProgress.CommitmentID = commitment.ID
		battle.Flow.Actors[actorID] = actorProgress
		return progress(ProgressContinue), nil
	}

	for _, actorID := range sortedActorIDs(battle.Actors) {
		switch battle.Flow.Actors[actorID].Status {
		case state.ActorNeedsInput:
			return progress(ProgressWaitingForInput), nil
		case state.ActorLockedIn, state.ActorResolved, state.ActorNotParticipating:
			continue
		default:
			return ProgressResult{}, fmt.Errorf(
				"actor %q has unresolved offensive status %q",
				actorID,
				battle.Flow.Actors[actorID].Status,
			)
		}
	}
	return progress(ProgressSegmentComplete), nil
}

func (OffensiveFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return nil, nil
}

func (OffensiveFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	if cmd.Type != command.TypeRollDice {
		return nil, unsupportedCommand()
	}

	var payload command.RollDicePayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, fmt.Errorf("invalid roll_dice payload")
	}
	if err := validatePendingCommand(ctx.Battle, cmd, payload.PendingInputID); err != nil {
		return nil, err
	}
	return dice.Roll(ctx.Battle, payload.RequestID, cmd.ActorID, payload.RerollIndices)
}

func validatePendingCommand(battle *state.Battle, cmd command.Command, pendingInputID string) error {
	if battle.ID != cmd.BattleID {
		return fmt.Errorf("command battle %q does not match battle %q", cmd.BattleID, battle.ID)
	}
	actor, ok := battle.Actors[cmd.ActorID]
	if !ok {
		return fmt.Errorf("actor %q is not in battle", cmd.ActorID)
	}
	if actor.Controller != state.ControllerHuman {
		return fmt.Errorf("actor %q is not human-controlled", cmd.ActorID)
	}
	progress, ok := battle.Flow.Actors[cmd.ActorID]
	if !ok || progress.Status != state.ActorNeedsInput {
		return fmt.Errorf("actor %q is not waiting for input", cmd.ActorID)
	}
	pending, ok := battle.Flow.PendingInput[cmd.ActorID]
	if !ok {
		return fmt.Errorf("actor %q has no pending input", cmd.ActorID)
	}
	if pendingInputID == "" {
		return errors.New("pending_input_id is required")
	}
	if pending.ID != pendingInputID {
		return fmt.Errorf("pending input %q is stale; current input is %q", pendingInputID, pending.ID)
	}
	if pending.Segment != battle.Segment.Current ||
		pending.Stage != battle.Flow.Stage ||
		pending.Iteration != battle.Flow.Iteration {
		return fmt.Errorf("pending input %q does not match current flow checkpoint", pending.ID)
	}
	for _, allowed := range pending.AllowedCommands {
		if allowed == cmd.Type {
			return nil
		}
	}
	return fmt.Errorf("command %q is not allowed for pending input %q", cmd.Type, pending.ID)
}

func sortedActorIDs(actors map[string]state.ActorState) []string {
	ids := make([]string, 0, len(actors))
	for actorID := range actors {
		ids = append(ids, actorID)
	}
	sort.Strings(ids)
	return ids
}
