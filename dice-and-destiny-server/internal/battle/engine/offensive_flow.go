package engine

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const planningStage = "planning"

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

type SharedPlanningFlow struct {
	segmentID       segment.Segment
	offensiveAI     OffensiveAIController
	defaultMaxRolls int
	automaticAI     bool
}

func NewSharedPlanningFlow(
	segmentID segment.Segment,
	offensiveAI OffensiveAIController,
) (SharedPlanningFlow, error) {
	if segmentID != segment.Offensive && segmentID != segment.Defensive {
		return SharedPlanningFlow{}, fmt.Errorf("invalid planning segment %q", segmentID)
	}
	if segmentID == segment.Offensive && offensiveAI == nil {
		return SharedPlanningFlow{}, errors.New("offensive AI controller is required")
	}
	automaticAI := false
	switch offensiveAI.(type) {
	case DefaultOffensiveAIController, *DefaultOffensiveAIController:
		automaticAI = true
	}
	return SharedPlanningFlow{
		segmentID:       segmentID,
		offensiveAI:     offensiveAI,
		defaultMaxRolls: DefaultPlanningMaxRolls,
		automaticAI:     automaticAI || segmentID == segment.Defensive,
	}, nil
}

func (flow SharedPlanningFlow) ID() segment.Segment {
	return flow.segmentID
}

func (flow SharedPlanningFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	battle := ctx.Battle
	battle.Flow.Stage = planningStage
	battle.Flow.Iteration = 1

	specs := flow.actorSpecs(battle)
	eligible := 0
	for _, spec := range specs {
		if spec.Participation == state.ActorNotParticipating {
			battle.Flow.Actors[spec.ActorID] = state.ActorFlowState{
				Status:     state.ActorNotParticipating,
				ReasonCode: spec.ReasonCode,
			}
			continue
		}
		eligible++
		battle.Flow.Actors[spec.ActorID] = state.ActorFlowState{
			Status:     state.ActorResolvingAutomatic,
			ReasonCode: string(flow.segmentID) + "_planning",
		}
	}
	if eligible == 0 {
		return nil, nil
	}

	resolutionID := flow.resolutionID(battle)
	if err := BeginPlanningResolution(ctx, PlanningResolutionSpec{
		ID:              resolutionID,
		Segment:         flow.segmentID,
		DefaultMaxRolls: flow.defaultMaxRolls,
		Actors:          specs,
	}); err != nil {
		return nil, err
	}
	if flow.segmentID == segment.Offensive && !flow.automaticAI {
		if err := flow.applyLegacyOffensiveAI(ctx, resolutionID); err != nil {
			return nil, err
		}
	}
	resolution := battle.Resolutions[resolutionID]
	window := resolution.Windows[resolution.ActiveWindowID]
	var events []event.Event
	for _, actorID := range window.RequiredActors {
		if battle.Actors[actorID].Controller != state.ControllerHuman {
			continue
		}
		plan := resolution.Planning.Actors[actorID]
		events = append(events, event.NewRollRequested(
			actorID,
			flow.segmentID,
			plan.RollRequestID,
			fmt.Sprintf("input-%s-%s-%d", window.ID, actorID, plan.ActionSequence),
		))
	}
	return events, nil
}

func (flow SharedPlanningFlow) Progress(ctx *Context) (ProgressResult, error) {
	resolution, ok := ctx.Battle.Resolutions[flow.resolutionID(ctx.Battle)]
	if ok && resolution.Planning != nil && resolution.Planning.Finalized {
		for actorID, plan := range resolution.Planning.Actors {
			if plan.Participation == state.ActorNotParticipating {
				ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{
					Status:     state.ActorNotParticipating,
					ReasonCode: plan.ReasonCode,
				}
			} else {
				ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{
					Status:     state.ActorResolved,
					ReasonCode: string(flow.segmentID) + "_planning_finalized",
				}
			}
		}
		return progress(ProgressSegmentComplete), nil
	}
	if flow.segmentID == segment.Defensive && !ok {
		return progress(ProgressSegmentComplete), nil
	}
	return ProgressResult{}, fmt.Errorf("%s planning resolution is incomplete", flow.segmentID)
}

func (SharedPlanningFlow) HandleCommand(*Context, command.Command) ([]event.Event, error) {
	return nil, unsupportedCommand()
}

func (SharedPlanningFlow) OnExit(*Context) ([]event.Event, error) {
	return nil, nil
}

func (flow SharedPlanningFlow) actorSpecs(battle *state.Battle) []PlanningActorSpec {
	actorIDs := sortedActorIDs(battle.Actors)
	specs := make([]PlanningActorSpec, 0, len(actorIDs))
	if flow.segmentID == segment.Offensive {
		for _, actorID := range actorIDs {
			var targets []string
			for _, targetID := range actorIDs {
				if targetID != actorID {
					targets = append(targets, targetID)
				}
			}
			specs = append(specs, PlanningActorSpec{
				ActorID:           actorID,
				Participation:     state.ActorNeedsInput,
				EligibleTargetIDs: targets,
			})
		}
		return specs
	}

	incoming := make(map[string][]string)
	for _, proposal := range battle.OffensiveProposals {
		if !proposal.Defensible {
			continue
		}
		for _, targetID := range proposal.Commitment.SelectedTargets {
			if _, exists := battle.Actors[targetID]; exists {
				incoming[targetID] = append(incoming[targetID], proposal.ID)
			}
		}
	}
	for _, actorID := range actorIDs {
		targets := append([]string(nil), incoming[actorID]...)
		sort.Strings(targets)
		if len(targets) == 0 {
			specs = append(specs, PlanningActorSpec{
				ActorID:       actorID,
				Participation: state.ActorNotParticipating,
				ReasonCode:    "no_incoming_defensible_proposal",
			})
			continue
		}
		specs = append(specs, PlanningActorSpec{
			ActorID:           actorID,
			Participation:     state.ActorNeedsInput,
			EligibleTargetIDs: targets,
		})
	}
	return specs
}

func (flow SharedPlanningFlow) applyLegacyOffensiveAI(ctx *Context, resolutionID string) error {
	resolution := ctx.Battle.Resolutions[resolutionID]
	for _, actorID := range sortedActorIDs(ctx.Battle.Actors) {
		actor := ctx.Battle.Actors[actorID]
		if actor.Controller != state.ControllerAI {
			continue
		}
		commitment, err := flow.offensiveAI.Plan(ctx, actorID)
		if err != nil {
			return err
		}
		if commitment.ActorID == "" {
			commitment.ActorID = actorID
		}
		if commitment.ActorID != actorID {
			return fmt.Errorf("invalid AI commitment for actor %q", actorID)
		}
		plan := resolution.Planning.Actors[actorID]
		plan.FinalDice = cloneRolledDice(commitment.FinalDice)
		plan.RollsUsed = commitment.RollsUsed
		plan.SelectedAbility = commitment.SelectedAbility
		plan.CommittedCards = append([]string(nil), commitment.SelectedCards...)
		plan.SelectedTargets = append([]string(nil), commitment.SelectedTargets...)
		if current := ctx.Battle.Actors[actorID].Dice.CurrentRoll; current != nil {
			plan.FinalDice = cloneRolledDice(current.Dice)
			plan.KeptIndices = append([]int(nil), current.KeptIndices...)
			plan.RollsUsed = current.RollsUsed
			plan.MaxRolls = current.MaxRolls
		}
		if plan.SelectedAbility == "" && len(plan.CommittedCards) == 0 {
			plan.Passed = true
		}
		plan.LockedIn = true
		plan.ActionSequence = 1
		plan.Revision = 1
		resolution.Planning.Actors[actorID] = plan
	}
	ctx.Battle.Resolutions[resolutionID] = resolution
	return nil
}

func (flow SharedPlanningFlow) resolutionID(battle *state.Battle) string {
	return fmt.Sprintf("planning-%s-round-%d", flow.segmentID, battle.Segment.Round)
}

type OffensiveFlow struct {
	shared SharedPlanningFlow
}

func NewOffensiveFlow(controller OffensiveAIController) (OffensiveFlow, error) {
	shared, err := NewSharedPlanningFlow(segment.Offensive, controller)
	if err != nil {
		return OffensiveFlow{}, err
	}
	return OffensiveFlow{shared: shared}, nil
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
	return flow.shared.OnEnter(ctx)
}

func (flow OffensiveFlow) Progress(ctx *Context) (ProgressResult, error) {
	return flow.shared.Progress(ctx)
}

func (flow OffensiveFlow) HandleCommand(ctx *Context, cmd command.Command) ([]event.Event, error) {
	return flow.shared.HandleCommand(ctx, cmd)
}

func (flow OffensiveFlow) OnExit(ctx *Context) ([]event.Event, error) {
	return flow.shared.OnExit(ctx)
}
