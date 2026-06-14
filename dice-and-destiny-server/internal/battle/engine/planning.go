package engine

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const DefaultPlanningMaxRolls = 3

type PlanningActorSpec struct {
	ActorID           string
	Participation     state.ActorProgressStatus
	ReasonCode        string
	EligibleTargetIDs []string
}

type PlanningResolutionSpec struct {
	ID              string
	Segment         segment.Segment
	DefaultMaxRolls int
	Actors          []PlanningActorSpec
}

func BeginPlanningResolution(ctx *Context, spec PlanningResolutionSpec) error {
	if ctx == nil || ctx.Battle == nil {
		return errors.New("battle is required")
	}
	if spec.ID == "" {
		return errors.New("planning resolution id is required")
	}
	if spec.Segment != segment.Offensive && spec.Segment != segment.Defensive {
		return fmt.Errorf("invalid planning segment %q", spec.Segment)
	}
	if ctx.Battle.Segment.Current != spec.Segment {
		return fmt.Errorf("planning segment %q does not match current segment %q", spec.Segment, ctx.Battle.Segment.Current)
	}
	if ctx.Battle.ActiveResolutionID != "" {
		return fmt.Errorf("resolution %q is already active", ctx.Battle.ActiveResolutionID)
	}
	if _, exists := ctx.Battle.Resolutions[spec.ID]; exists {
		return fmt.Errorf("resolution %q already exists", spec.ID)
	}
	maxRolls := spec.DefaultMaxRolls
	if maxRolls == 0 {
		maxRolls = DefaultPlanningMaxRolls
	}
	if maxRolls < 1 {
		return errors.New("planning max rolls must be positive")
	}

	checkpoint := state.ResolutionCheckpoint{
		Segment:   spec.Segment,
		Phase:     ctx.Phase,
		Stage:     ctx.Battle.Flow.Stage,
		Iteration: ctx.Battle.Flow.Iteration,
	}
	planning := &state.PlanningState{
		Segment:                  spec.Segment,
		Cycle:                    1,
		DefaultMaxRolls:          maxRolls,
		Actors:                   make(map[string]state.PlanningActorState, len(spec.Actors)),
		AppliedReactionWindowIDs: make(map[string]bool),
	}
	var eligible []string
	for _, actorSpec := range spec.Actors {
		actor, ok := ctx.Battle.Actors[actorSpec.ActorID]
		if !ok {
			return fmt.Errorf("planning actor %q is not in battle", actorSpec.ActorID)
		}
		participation := actorSpec.Participation
		if participation == "" {
			participation = state.ActorNeedsInput
		}
		planningActor := state.PlanningActorState{
			ActorID:           actorSpec.ActorID,
			Participation:     participation,
			ReasonCode:        actorSpec.ReasonCode,
			MaxRolls:          maxRolls,
			EligibleTargetIDs: append([]string(nil), actorSpec.EligibleTargetIDs...),
		}
		if participation != state.ActorNotParticipating {
			eligible = append(eligible, actorSpec.ActorID)
			requestID := fmt.Sprintf("roll-%s-%s-%d", actorSpec.ActorID, spec.Segment, ctx.Battle.Segment.Round)
			planningActor.RollRequestID = requestID
			ctx.Battle.RollRequests[requestID] = state.RollRequest{
				ID:          requestID,
				ActorID:     actorSpec.ActorID,
				Segment:     spec.Segment,
				Pool:        planningRollPool(spec.Segment),
				SourceType:  state.RollSourceSegment,
				SourceID:    string(spec.Segment),
				DiceLoadout: append([]state.DiceLoadoutEntry(nil), actor.DiceLoadout...),
				MaxRolls:    maxRolls,
			}
		}
		planning.Actors[actorSpec.ActorID] = planningActor
	}
	sort.Strings(eligible)
	if len(eligible) == 0 {
		return errors.New("planning resolution requires an eligible actor")
	}
	planning.ChangedActorIDs = append([]string(nil), eligible...)
	window, err := newInteractionWindow(WindowSpec{
		ID:                planningWindowID(spec.ID, planning.Cycle),
		Purpose:           state.InteractionPurposePlanning,
		Source:            planningSource(spec.Segment, spec.ID),
		EligibleActors:    eligible,
		RequiredActors:    eligible,
		AllowedCommands:   planningCommandTypes(),
		HiddenCommitments: true,
	}, checkpoint)
	if err != nil {
		return err
	}
	resolution := state.ResolutionState{
		ID:                    spec.ID,
		Origin:                checkpoint,
		Stage:                 state.ResolutionCollecting,
		Windows:               map[string]state.InteractionWindow{window.ID: window},
		ActiveWindowID:        window.ID,
		WindowSequence:        1,
		ReactionPolicy:        planningReactionPolicy(ctx.Battle, spec.Segment, spec.ID),
		SuspendedActors:       copyActorFlowStates(ctx.Battle.Flow.Actors),
		SuspendedPendingInput: copyPendingInputs(ctx.Battle.Flow.PendingInput),
		Planning:              planning,
	}
	ctx.Battle.Resolutions[spec.ID] = resolution
	ctx.Battle.ActiveResolutionID = spec.ID
	ctx.Battle.Flow.Actors = make(map[string]state.ActorFlowState)
	ctx.Battle.Flow.PendingInput = make(map[string]state.PendingInput)
	for actorID, actor := range planning.Actors {
		if actor.Participation == state.ActorNotParticipating {
			ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:     state.ActorNotParticipating,
				ReasonCode: actor.ReasonCode,
			}
		}
	}
	return nil
}

func (e Engine) progressPlanningResolution(
	ctx *Context,
	resolution state.ResolutionState,
	window state.InteractionWindow,
) (ProgressResult, error) {
	if resolution.Planning == nil {
		return ProgressResult{}, errors.New("planning state is missing")
	}
	if window.Purpose == state.InteractionPurposePlanning {
		switch window.RevealStatus {
		case state.RevealStatusCollecting:
			return e.collectPlanningWindow(ctx, resolution, window)
		case state.RevealStatusRevealed:
			resolution.Batch.Revealed = true
			ctx.Battle.Resolutions[resolution.ID] = resolution
			if err := e.openReactionWindow(ctx.Battle, &resolution, 1, 1); err != nil {
				return ProgressResult{}, err
			}
			return progress(ProgressContinue, event.NewProposalBatchRevealed(resolution)), nil
		default:
			return ProgressResult{}, fmt.Errorf("invalid planning reveal status %q", window.RevealStatus)
		}
	}
	if window.Purpose != state.InteractionPurposeReaction {
		return ProgressResult{}, fmt.Errorf("planning resolution has unsupported window purpose %q", window.Purpose)
	}
	switch window.RevealStatus {
	case state.RevealStatusCollecting:
		return e.collectWindow(ctx, resolution, window)
	case state.RevealStatusRevealed:
		if windowHasNonPassCommitment(window) {
			if err := e.openReactionWindow(ctx.Battle, &resolution, window.ReactionRound+1, window.ChainDepth+1); err != nil {
				return ProgressResult{}, err
			}
			return progress(ProgressContinue), nil
		}
		return e.revalidatePlanning(ctx, resolution)
	default:
		return ProgressResult{}, fmt.Errorf("invalid reaction reveal status %q", window.RevealStatus)
	}
}

func (e Engine) collectPlanningWindow(
	ctx *Context,
	resolution state.ResolutionState,
	window state.InteractionWindow,
) (ProgressResult, error) {
	if !window.Opened {
		window.Opened = true
		resolution.Windows[window.ID] = window
		ctx.Battle.Resolutions[resolution.ID] = resolution
		return progress(ProgressContinue, event.NewInteractionWindowOpened(resolution.ID, window)), nil
	}
	planning := resolution.Planning
	for _, actorID := range sortedRequiredActors(window) {
		if window.ActorProgress[actorID] != state.InteractionActorAwaiting {
			continue
		}
		plan := planning.Actors[actorID]
		if plan.LockedIn {
			commitment := planningInteractionCommitment(window, plan, planning.Segment, planning.Cycle)
			if err := recordWindowCommitment(&window, actorID, commitment); err != nil {
				return ProgressResult{}, err
			}
			ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:       state.ActorLockedIn,
				ReasonCode:   string(planning.Segment) + "_planning_locked",
				CommitmentID: commitment.ID,
			}
			resolution.Windows[window.ID] = window
			ctx.Battle.Resolutions[resolution.ID] = resolution
			return progress(
				ProgressContinue,
				event.NewInteractionCommitted(resolution.ID, window, commitment, actorID),
			), nil
		}
		actor := ctx.Battle.Actors[actorID]
		switch actor.Controller {
		case state.ControllerHuman:
			ensurePlanningPendingInput(ctx.Battle, resolution, window, plan)
		case state.ControllerAI, state.ControllerSystem:
			events, err := autoPlanActor(ctx.Battle, planning, actorID)
			if err != nil {
				return ProgressResult{}, err
			}
			resolution.Planning = planning
			ctx.Battle.Resolutions[resolution.ID] = resolution
			return progress(ProgressContinue, events...), nil
		default:
			return ProgressResult{}, fmt.Errorf("invalid controller type %q for actor %q", actor.Controller, actorID)
		}
	}
	for _, actorID := range sortedRequiredActors(window) {
		if window.ActorProgress[actorID] == state.InteractionActorAwaiting {
			ctx.Battle.Resolutions[resolution.ID] = resolution
			return progress(ProgressWaitingForInput), nil
		}
	}

	window.RevealStatus = state.RevealStatusRevealed
	for actorID, commitment := range window.Commitments {
		commitment.Revealed = true
		window.Commitments[actorID] = commitment
		plan := planning.Actors[actorID]
		revealed := planningCommitmentData(plan, planning.Cycle)
		revealed.Segment = planning.Segment
		plan.RevealedCommitment = &revealed
		plan.RevealedRevision = plan.Revision
		planning.Actors[actorID] = plan
	}
	resolution.Stage = state.ResolutionRevealing
	resolution.Batch = planningProposalBatch(resolution.ID, planning, window.RequiredActors)
	resolution.Windows[window.ID] = window
	resolution.Planning = planning
	ctx.Battle.Resolutions[resolution.ID] = resolution
	return progress(ProgressContinue, event.NewInteractionRevealed(resolution.ID, window)), nil
}

func autoPlanActor(
	battle *state.Battle,
	planning *state.PlanningState,
	actorID string,
) ([]event.Event, error) {
	plan := planning.Actors[actorID]
	var events []event.Event
	actor := battle.Actors[actorID]
	if len(actor.DiceLoadout) > 0 && plan.RollsUsed < plan.MaxRolls {
		rolled, err := dice.Roll(battle, plan.RollRequestID, actorID, nil)
		if err != nil {
			return nil, err
		}
		events = append(events, rolled...)
		plan = syncPlanningDice(battle, plan)
	}
	for _, abilityID := range actor.AbilityIDs {
		definition := battle.Content.Abilities[abilityID]
		if abilityAllowedForPlanning(battle, planning.Segment, abilityID) &&
			diceRequirementMet(definition.DiceRequirement, plan.FinalDice) &&
			definition.EnergyCost <= actor.Resources.EnergyPoints {
			plan.SelectedAbility = abilityID
			break
		}
	}
	if plan.SelectedAbility != "" && len(plan.EligibleTargetIDs) > 0 {
		plan.SelectedTargets = []string{plan.EligibleTargetIDs[0]}
	}
	if plan.SelectedAbility == "" {
		plan.Passed = true
	}
	plan.LockedIn = true
	plan.ActionSequence++
	plan.Revision++
	planning.Actors[actorID] = plan
	return events, nil
}

func (e Engine) handlePlanningCommand(
	battle *state.Battle,
	cmd command.Command,
) ([]event.Event, error) {
	resolution, window, err := activeWindow(battle)
	if err != nil {
		return nil, err
	}
	if resolution.Planning == nil || window.Purpose != state.InteractionPurposePlanning {
		return nil, fmt.Errorf("command %q is not supported by the active interaction window", cmd.Type)
	}
	if cmd.BattleID != battle.ID {
		return nil, fmt.Errorf("command battle %q does not match battle %q", cmd.BattleID, battle.ID)
	}
	actor, ok := battle.Actors[cmd.ActorID]
	if !ok {
		return nil, fmt.Errorf("actor %q is not in battle", cmd.ActorID)
	}
	if actor.Controller != state.ControllerHuman {
		return nil, fmt.Errorf("actor %q is not human-controlled", cmd.ActorID)
	}
	plan, ok := resolution.Planning.Actors[cmd.ActorID]
	if !ok || plan.Participation == state.ActorNotParticipating {
		return nil, fmt.Errorf("actor %q is not participating in %s planning", cmd.ActorID, resolution.Planning.Segment)
	}
	if plan.LockedIn || window.ActorProgress[cmd.ActorID] != state.InteractionActorAwaiting {
		return nil, fmt.Errorf("actor %q is locked in and has not been reopened", cmd.ActorID)
	}
	if progress, ok := battle.Flow.Actors[cmd.ActorID]; !ok || progress.Status != state.ActorNeedsInput {
		return nil, fmt.Errorf("actor %q is not waiting for input", cmd.ActorID)
	}
	pending, ok := battle.Flow.PendingInput[cmd.ActorID]
	if !ok {
		return nil, fmt.Errorf("actor %q has no pending input", cmd.ActorID)
	}

	pendingID, checkpoint, err := decodePlanningCommand(cmd)
	if err != nil {
		return nil, err
	}
	if pendingID == "" {
		return nil, errors.New("pending_input_id is required")
	}
	if pending.ID != pendingID {
		return nil, fmt.Errorf("pending input %q is stale; current input is %q", pendingID, pending.ID)
	}
	if checkpoint.WindowID != window.ID ||
		checkpoint.Segment != string(resolution.Planning.Segment) ||
		checkpoint.Stage != resolution.Origin.Stage ||
		checkpoint.Iteration != resolution.Origin.Iteration ||
		checkpoint.PlanningCycle != resolution.Planning.Cycle {
		if cmd.Type != command.TypeRollDice {
			return nil, errors.New("planning command has a stale window checkpoint")
		}
		checkpoint = command.PlanningCheckpoint{
			WindowID:      window.ID,
			Segment:       string(resolution.Planning.Segment),
			Stage:         resolution.Origin.Stage,
			Iteration:     resolution.Origin.Iteration,
			PlanningCycle: resolution.Planning.Cycle,
		}
	}
	if pending.WindowID != window.ID ||
		pending.Segment != resolution.Origin.Segment ||
		pending.Stage != checkpoint.Stage ||
		pending.Iteration != checkpoint.Iteration ||
		pending.PlanningCycle != checkpoint.PlanningCycle {
		return nil, fmt.Errorf("pending input %q does not match planning checkpoint", pending.ID)
	}
	if !commandAllowed(pending.AllowedCommands, cmd.Type) || !commandAllowed(window.AllowedCommands, cmd.Type) {
		return nil, fmt.Errorf("command %q is not allowed for pending input %q", cmd.Type, pending.ID)
	}

	var events []event.Event
	switch cmd.Type {
	case command.TypeRollDice:
		var payload command.RollDicePayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid roll_dice payload")
		}
		events, err = dice.Roll(battle, plan.RollRequestID, cmd.ActorID, payload.RerollIndices)
		if err != nil {
			return nil, err
		}
		plan = syncPlanningDice(battle, plan)
	case command.TypePlanningRoll:
		if len(actor.DiceLoadout) == 0 {
			return nil, errors.New("actor has no planning dice")
		}
		events, err = dice.Roll(battle, plan.RollRequestID, cmd.ActorID, nil)
		if err != nil {
			return nil, err
		}
		plan = syncPlanningDice(battle, plan)
	case command.TypePlanningKeep:
		var payload command.PlanningKeepPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid planning_keep payload")
		}
		if err := validateKeptIndices(plan.FinalDice, payload.KeptIndices); err != nil {
			return nil, err
		}
		plan.KeptIndices = append([]int(nil), payload.KeptIndices...)
		updateActorKeptIndices(battle, cmd.ActorID, plan.KeptIndices)
	case command.TypePlanningReroll:
		var payload command.PlanningRerollPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid planning_reroll payload")
		}
		events, err = dice.Roll(battle, plan.RollRequestID, cmd.ActorID, payload.RerollIndices)
		if err != nil {
			return nil, err
		}
		plan = syncPlanningDice(battle, plan)
	case command.TypePlanningCards:
		var payload command.PlanningCardsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid planning_commit_cards payload")
		}
		if len(payload.CardIDs) == 0 {
			return nil, errors.New("at least one card id is required")
		}
		for _, cardID := range payload.CardIDs {
			definition, ok := battle.Content.Cards[cardID]
			if !ok {
				return nil, fmt.Errorf("card %q has no loaded content definition", cardID)
			}
			if !segmentAllowed(definition.Segments, resolution.Planning.Segment) {
				return nil, fmt.Errorf("card %q is not allowed for %s planning", cardID, resolution.Planning.Segment)
			}
		}
		combined := append(append([]string(nil), plan.CommittedCards...), payload.CardIDs...)
		if err := validateCommittedCards(actor.Cards.Hand, combined); err != nil {
			return nil, err
		}
		plan.CommittedCards = combined
	case command.TypePlanningAbility:
		var payload command.PlanningAbilityPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid planning_select_ability payload")
		}
		if !containsString(actor.AbilityIDs, payload.AbilityID) ||
			!abilityAllowedForPlanning(battle, resolution.Planning.Segment, payload.AbilityID) {
			return nil, fmt.Errorf("ability %q is not allowed for %s planning", payload.AbilityID, resolution.Planning.Segment)
		}
		plan.SelectedAbility = payload.AbilityID
		plan.Passed = false
	case command.TypePlanningTargets:
		var payload command.PlanningTargetsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid planning_select_targets payload")
		}
		if len(payload.TargetIDs) == 0 {
			return nil, errors.New("at least one target id is required")
		}
		if err := validatePlanningTargets(plan.EligibleTargetIDs, payload.TargetIDs); err != nil {
			return nil, err
		}
		plan.SelectedTargets = append([]string(nil), payload.TargetIDs...)
	case command.TypePlanningPass:
		plan.Passed = true
		plan.SelectedAbility = ""
		plan.SelectedTargets = nil
	case command.TypePlanningLockIn:
		if !plan.Passed && plan.SelectedAbility == "" && len(plan.CommittedCards) == 0 {
			return nil, errors.New("planning lock-in requires an ability, committed card, or pass")
		}
		if err := validateFinalPlanningSelection(battle, actor, plan, resolution.Planning.Segment); err != nil {
			return nil, err
		}
		plan.LockedIn = true
	default:
		return nil, fmt.Errorf("unsupported planning command %q", cmd.Type)
	}

	if cmd.Type != command.TypeRollDice {
		plan.ActionSequence++
	}
	plan.Revision++
	resolution.Planning.Actors[cmd.ActorID] = plan
	delete(battle.Flow.PendingInput, cmd.ActorID)
	if plan.LockedIn {
		battle.Flow.Actors[cmd.ActorID] = state.ActorFlowState{
			Status:       state.ActorLockedIn,
			ReasonCode:   string(resolution.Planning.Segment) + "_planning_locked",
			CommitmentID: fmt.Sprintf("commit-%s-%s", window.ID, cmd.ActorID),
		}
	} else {
		ensurePlanningPendingInput(battle, resolution, window, plan)
	}
	battle.Resolutions[resolution.ID] = resolution
	return events, nil
}

func (e Engine) revalidatePlanning(
	ctx *Context,
	resolution state.ResolutionState,
) (ProgressResult, error) {
	planning := resolution.Planning
	affected, err := applyPlanningReactionAdjustments(ctx.Battle, &resolution)
	if err != nil {
		return ProgressResult{}, err
	}
	if len(affected) > 0 {
		planning = resolution.Planning
		planning.Cycle++
		planning.ChangedActorIDs = append([]string(nil), affected...)
		for _, actorID := range affected {
			plan := planning.Actors[actorID]
			plan.LockedIn = false
			plan.ActionSequence++
			planning.Actors[actorID] = plan
		}
		window, err := newInteractionWindow(WindowSpec{
			ID:                planningWindowID(resolution.ID, planning.Cycle),
			Purpose:           state.InteractionPurposePlanning,
			Source:            planningSource(planning.Segment, resolution.ID),
			EligibleActors:    affected,
			RequiredActors:    affected,
			AllowedCommands:   planningCommandTypes(),
			HiddenCommitments: true,
		}, resolution.Origin)
		if err != nil {
			return ProgressResult{}, err
		}
		resolution.WindowSequence++
		resolution.Stage = state.ResolutionCollecting
		resolution.Batch = state.ProposalBatch{}
		resolution.ActiveWindowID = window.ID
		resolution.Windows[window.ID] = window
		resolution.Planning = planning
		ctx.Battle.Flow.Actors = make(map[string]state.ActorFlowState)
		ctx.Battle.Flow.PendingInput = make(map[string]state.PendingInput)
		for actorID, plan := range planning.Actors {
			status := state.ActorLockedIn
			if plan.Participation == state.ActorNotParticipating {
				status = state.ActorNotParticipating
			}
			if containsString(affected, actorID) {
				status = state.ActorResolvingAutomatic
			}
			ctx.Battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:     status,
				ReasonCode: string(planning.Segment) + "_planning_revalidation",
			}
		}
		ctx.Battle.Resolutions[resolution.ID] = resolution
		return progress(ProgressContinue), nil
	}

	planning.Finalized = true
	for actorID, plan := range planning.Actors {
		if plan.Participation != state.ActorNotParticipating {
			if err := validateFinalPlanningSelection(
				ctx.Battle,
				ctx.Battle.Actors[actorID],
				plan,
				planning.Segment,
			); err != nil {
				return ProgressResult{}, fmt.Errorf("validate %s planning for actor %q: %w", planning.Segment, actorID, err)
			}
		}
		if plan.RollRequestID == "" {
			continue
		}
		request := ctx.Battle.RollRequests[plan.RollRequestID]
		request.Complete = true
		ctx.Battle.RollRequests[plan.RollRequestID] = request
		actor := ctx.Battle.Actors[actorID]
		if actor.Dice.CurrentRoll != nil && actor.Dice.CurrentRoll.RequestID == plan.RollRequestID {
			actor.Dice.CurrentRoll.Complete = true
			ctx.Battle.Actors[actorID] = actor
		}
	}
	finalized, err := finalizedPlanningProposals(ctx.Battle, planning)
	if err != nil {
		return ProgressResult{}, err
	}
	if planning.Segment == segment.Offensive {
		ctx.Battle.OffensiveProposals = finalized
	} else {
		ctx.Battle.DefensiveProposals = finalized
	}
	resolution.Planning = planning
	resolution.Stage = state.ResolutionComplete
	resolution.Batch.Committed = true
	resolution.Batch.Revealed = true
	resolution.ActiveWindowID = ""
	ctx.Battle.Resolutions[resolution.ID] = resolution
	ctx.Battle.ActiveResolutionID = ""
	ctx.Battle.Flow.Stage = resolution.Origin.Stage
	ctx.Battle.Flow.Iteration = resolution.Origin.Iteration
	ctx.Battle.Flow.Actors = copyActorFlowStates(resolution.SuspendedActors)
	ctx.Battle.Flow.PendingInput = copyPendingInputs(resolution.SuspendedPendingInput)
	return progress(
		ProgressContinue,
		event.NewProposalBatchCommitted(resolution),
		event.NewResolutionCompleted(resolution),
	), nil
}

func applyPlanningReactionAdjustments(
	battle *state.Battle,
	resolution *state.ResolutionState,
) ([]string, error) {
	planning := resolution.Planning
	affectedSet := make(map[string]bool)
	windowIDs := make([]string, 0, len(resolution.Windows))
	for windowID := range resolution.Windows {
		windowIDs = append(windowIDs, windowID)
	}
	sort.Strings(windowIDs)
	for _, windowID := range windowIDs {
		window := resolution.Windows[windowID]
		if window.Purpose != state.InteractionPurposeReaction ||
			window.RevealStatus != state.RevealStatusRevealed ||
			planning.AppliedReactionWindowIDs[windowID] {
			continue
		}
		for _, commitment := range window.Commitments {
			for _, adjustment := range commitment.Data.PlanningAdjustments {
				changed, err := applyPlanningAdjustment(battle, planning, adjustment)
				if err != nil {
					return nil, err
				}
				if changed {
					affectedSet[adjustment.ActorID] = true
				}
			}
		}
		planning.AppliedReactionWindowIDs[windowID] = true
	}
	affected := make([]string, 0, len(affectedSet))
	for actorID := range affectedSet {
		affected = append(affected, actorID)
	}
	sort.Strings(affected)
	resolution.Planning = planning
	return affected, nil
}

func applyPlanningAdjustment(
	battle *state.Battle,
	planning *state.PlanningState,
	adjustment state.PlanningAdjustment,
) (bool, error) {
	plan, ok := planning.Actors[adjustment.ActorID]
	if !ok || plan.Participation == state.ActorNotParticipating {
		return false, fmt.Errorf("planning adjustment actor %q is not participating", adjustment.ActorID)
	}
	switch adjustment.Type {
	case state.PlanningAdjustmentSetDieFace:
		if adjustment.DieIndex < 0 || adjustment.DieIndex >= len(plan.FinalDice) {
			return false, fmt.Errorf("planning die index %d is out of range", adjustment.DieIndex)
		}
		die := plan.FinalDice[adjustment.DieIndex]
		definition, ok := battle.DiceDefinitions[die.DieID]
		if !ok {
			return false, fmt.Errorf("dice definition %q is missing", die.DieID)
		}
		for _, face := range definition.Faces {
			if face.Face != adjustment.Face {
				continue
			}
			if die.Face == face.Face && die.Value == face.Value && equalStrings(die.Symbols, face.Symbols) {
				return false, nil
			}
			die.Face = face.Face
			die.Value = face.Value
			die.Symbols = append([]string(nil), face.Symbols...)
			plan.FinalDice[adjustment.DieIndex] = die
			updateActorPlanningDice(battle, adjustment.ActorID, plan)
			plan.Revision++
			planning.Actors[adjustment.ActorID] = plan
			return true, nil
		}
		return false, fmt.Errorf("face %d is not defined for die %q", adjustment.Face, die.DieID)
	case state.PlanningAdjustmentIncreaseMaxRolls:
		if adjustment.Amount <= 0 {
			return false, errors.New("max roll increase must be positive")
		}
		plan.MaxRolls += adjustment.Amount
		request := battle.RollRequests[plan.RollRequestID]
		request.MaxRolls = plan.MaxRolls
		battle.RollRequests[plan.RollRequestID] = request
		updateActorPlanningDice(battle, adjustment.ActorID, plan)
		plan.Revision++
		planning.Actors[adjustment.ActorID] = plan
		return true, nil
	case state.PlanningAdjustmentClearAbility:
		if plan.SelectedAbility == "" {
			return false, nil
		}
		plan.SelectedAbility = ""
		plan.Revision++
		planning.Actors[adjustment.ActorID] = plan
		return true, nil
	case state.PlanningAdjustmentRemoveTarget:
		updated := removeString(plan.SelectedTargets, adjustment.TargetID)
		if len(updated) == len(plan.SelectedTargets) {
			return false, nil
		}
		plan.SelectedTargets = updated
		plan.Revision++
		planning.Actors[adjustment.ActorID] = plan
		return true, nil
	case state.PlanningAdjustmentReopenActor:
		return true, nil
	default:
		return false, fmt.Errorf("unsupported planning adjustment %q", adjustment.Type)
	}
}

func decodePlanningCommand(cmd command.Command) (string, command.PlanningCheckpoint, error) {
	switch cmd.Type {
	case command.TypeRollDice:
		var payload command.RollDicePayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid roll_dice payload")
		}
		return payload.PendingInputID, command.PlanningCheckpoint{}, nil
	case command.TypePlanningRoll:
		var payload command.PlanningRollPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_roll payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningKeep:
		var payload command.PlanningKeepPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_keep payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningReroll:
		var payload command.PlanningRerollPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_reroll payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningCards:
		var payload command.PlanningCardsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_commit_cards payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningAbility:
		var payload command.PlanningAbilityPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_select_ability payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningTargets:
		var payload command.PlanningTargetsPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_select_targets payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningPass:
		var payload command.PlanningPassPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_pass payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	case command.TypePlanningLockIn:
		var payload command.PlanningLockInPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return "", command.PlanningCheckpoint{}, errors.New("invalid planning_lock_in payload")
		}
		return payload.PendingInputID, payload.Checkpoint, nil
	default:
		return "", command.PlanningCheckpoint{}, fmt.Errorf("unsupported planning command %q", cmd.Type)
	}
}

func ensurePlanningPendingInput(
	battle *state.Battle,
	resolution state.ResolutionState,
	window state.InteractionWindow,
	plan state.PlanningActorState,
) {
	allowed := allowedPlanningCommands(battle, battle.Actors[plan.ActorID], plan, resolution.Planning.Segment)
	battle.Flow.Actors[plan.ActorID] = state.ActorFlowState{
		Status:     state.ActorNeedsInput,
		ReasonCode: string(resolution.Planning.Segment) + "_planning",
	}
	battle.Flow.PendingInput[plan.ActorID] = state.PendingInput{
		ID:              fmt.Sprintf("input-%s-%s-%d", window.ID, plan.ActorID, plan.ActionSequence),
		ActorID:         plan.ActorID,
		Segment:         resolution.Origin.Segment,
		Phase:           resolution.Origin.Phase,
		Stage:           resolution.Origin.Stage,
		Iteration:       resolution.Origin.Iteration,
		WindowID:        window.ID,
		PlanningCycle:   resolution.Planning.Cycle,
		InputType:       string(state.InteractionPurposePlanning),
		SourceType:      window.Source.Type,
		SourceID:        plan.RollRequestID,
		AllowedCommands: allowed,
	}
}

func allowedPlanningCommands(
	battle *state.Battle,
	actor state.ActorState,
	plan state.PlanningActorState,
	planningSegment segment.Segment,
) []command.Type {
	var allowed []command.Type
	if len(actor.DiceLoadout) > 0 && plan.RollsUsed < plan.MaxRolls {
		allowed = append(allowed, command.TypeRollDice)
		if plan.RollsUsed == 0 {
			allowed = append(allowed, command.TypePlanningRoll)
		} else {
			allowed = append(allowed, command.TypePlanningReroll)
		}
	}
	if len(plan.FinalDice) > 0 {
		allowed = append(allowed, command.TypePlanningKeep)
	}
	if hasAllowedPlanningCard(battle, actor, plan, planningSegment) {
		allowed = append(allowed, command.TypePlanningCards)
	}
	for _, abilityID := range actor.AbilityIDs {
		if abilityAllowedForPlanning(battle, planningSegment, abilityID) {
			allowed = append(allowed, command.TypePlanningAbility)
			break
		}
	}
	if len(plan.EligibleTargetIDs) > 0 {
		allowed = append(allowed, command.TypePlanningTargets)
	}
	allowed = append(allowed, command.TypePlanningPass)
	if plan.Passed || plan.SelectedAbility != "" || len(plan.CommittedCards) > 0 {
		allowed = append(allowed, command.TypePlanningLockIn)
	}
	return allowed
}

func planningCommandTypes() []command.Type {
	return []command.Type{
		command.TypeRollDice,
		command.TypePlanningRoll,
		command.TypePlanningKeep,
		command.TypePlanningReroll,
		command.TypePlanningCards,
		command.TypePlanningAbility,
		command.TypePlanningTargets,
		command.TypePlanningPass,
		command.TypePlanningLockIn,
	}
}

func planningInteractionCommitment(
	window state.InteractionWindow,
	plan state.PlanningActorState,
	planningSegment segment.Segment,
	cycle int,
) state.InteractionCommitment {
	data := planningCommitmentData(plan, cycle)
	data.Segment = planningSegment
	return state.InteractionCommitment{
		ID:      fmt.Sprintf("commit-%s-%s", window.ID, plan.ActorID),
		ActorID: plan.ActorID,
		Command: command.TypePlanningLockIn,
		Data: state.InteractionCommitmentData{
			CardIDs:   append([]string(nil), plan.CommittedCards...),
			TargetIDs: append([]string(nil), plan.SelectedTargets...),
			ChoiceID:  plan.SelectedAbility,
			Planning:  &data,
		},
	}
}

func planningCommitmentData(
	plan state.PlanningActorState,
	cycle int,
) state.PlanningCommitmentData {
	return state.PlanningCommitmentData{
		Cycle:           cycle,
		FinalDice:       cloneRolledDice(plan.FinalDice),
		KeptIndices:     append([]int(nil), plan.KeptIndices...),
		RollsUsed:       plan.RollsUsed,
		MaxRolls:        plan.MaxRolls,
		SelectedAbility: plan.SelectedAbility,
		CommittedCards:  append([]string(nil), plan.CommittedCards...),
		SelectedTargets: append([]string(nil), plan.SelectedTargets...),
		Passed:          plan.Passed,
		LockedIn:        plan.LockedIn,
	}
}

func planningProposalBatch(
	resolutionID string,
	planning *state.PlanningState,
	actorIDs []string,
) state.ProposalBatch {
	proposals := make([]state.Proposal, 0, len(actorIDs))
	for _, actorID := range actorIDs {
		plan := planning.Actors[actorID]
		data := planningCommitmentData(plan, planning.Cycle)
		data.Segment = planning.Segment
		proposals = append(proposals, state.Proposal{
			ID: fmt.Sprintf("%s-%s-cycle-%d", planning.Segment, actorID, planning.Cycle),
			Source: state.SourceReference{
				Type:    "planning_actor",
				ID:      actorID,
				ActorID: actorID,
			},
			Target: state.TargetReference{
				Type:    "planning_commitment",
				ID:      actorID,
				ActorID: actorID,
			},
			Operation: state.ProposalOperationPlanning,
			Data: state.ProposalData{
				Planning: &data,
			},
		})
	}
	return state.ProposalBatch{
		ID:           fmt.Sprintf("%s-batch-cycle-%d", resolutionID, planning.Cycle),
		ResolutionID: resolutionID,
		Proposals:    proposals,
	}
}

func finalizedPlanningProposals(
	battle *state.Battle,
	planning *state.PlanningState,
) ([]state.PlanningProposal, error) {
	actorIDs := make([]string, 0, len(planning.Actors))
	for actorID, plan := range planning.Actors {
		if plan.Participation != state.ActorNotParticipating {
			actorIDs = append(actorIDs, actorID)
		}
	}
	sort.Strings(actorIDs)
	proposals := make([]state.PlanningProposal, 0, len(actorIDs))
	for _, actorID := range actorIDs {
		plan := planning.Actors[actorID]
		operations, err := finalizedContentOperations(battle, plan)
		if err != nil {
			return nil, err
		}
		commitment := planningCommitmentData(plan, planning.Cycle)
		commitment.Segment = planning.Segment
		proposals = append(proposals, state.PlanningProposal{
			ID:         fmt.Sprintf("final-%s-%s", planning.Segment, actorID),
			ActorID:    actorID,
			Segment:    planning.Segment,
			Commitment: commitment,
			Defensible: planning.Segment == segment.Offensive && !plan.Passed && len(plan.SelectedTargets) > 0,
			Operations: operations,
		})
	}
	return proposals, nil
}

func planningReactionPolicy(
	battle *state.Battle,
	planningSegment segment.Segment,
	resolutionID string,
) *state.ReactionWindowPolicy {
	actors := sortedActorIDs(battle.Actors)
	return &state.ReactionWindowPolicy{
		Source:            planningSource(planningSegment, resolutionID),
		EligibleActors:    actors,
		RequiredActors:    actors,
		AllowedCommands:   []command.Type{command.TypeCommitInteraction, command.TypePass},
		HiddenCommitments: true,
		PassAllowed:       true,
	}
}

func planningSource(planningSegment segment.Segment, resolutionID string) state.SourceReference {
	return state.SourceReference{
		Type: "planning_batch",
		ID:   resolutionID,
	}
}

func planningWindowID(resolutionID string, cycle int) string {
	return fmt.Sprintf("%s-planning-%d", resolutionID, cycle)
}

func planningRollPool(planningSegment segment.Segment) state.RollPool {
	if planningSegment == segment.Defensive {
		return state.RollPoolDefensive
	}
	return state.RollPoolOffensive
}

func syncPlanningDice(battle *state.Battle, plan state.PlanningActorState) state.PlanningActorState {
	actor := battle.Actors[plan.ActorID]
	if actor.Dice.CurrentRoll == nil {
		return plan
	}
	plan.FinalDice = cloneRolledDice(actor.Dice.CurrentRoll.Dice)
	plan.KeptIndices = append([]int(nil), actor.Dice.CurrentRoll.KeptIndices...)
	plan.RollsUsed = actor.Dice.CurrentRoll.RollsUsed
	plan.MaxRolls = actor.Dice.CurrentRoll.MaxRolls
	return plan
}

func updateActorKeptIndices(battle *state.Battle, actorID string, kept []int) {
	actor := battle.Actors[actorID]
	if actor.Dice.CurrentRoll != nil {
		actor.Dice.CurrentRoll.KeptIndices = append([]int(nil), kept...)
	}
	battle.Actors[actorID] = actor
}

func updateActorPlanningDice(
	battle *state.Battle,
	actorID string,
	plan state.PlanningActorState,
) {
	actor := battle.Actors[actorID]
	if actor.Dice.CurrentRoll != nil {
		actor.Dice.CurrentRoll.Dice = cloneRolledDice(plan.FinalDice)
		actor.Dice.CurrentRoll.KeptIndices = append([]int(nil), plan.KeptIndices...)
		actor.Dice.CurrentRoll.RollsUsed = plan.RollsUsed
		actor.Dice.CurrentRoll.MaxRolls = plan.MaxRolls
		actor.Dice.CurrentRoll.SymbolCounts = dice.SymbolCounts(plan.FinalDice)
		actor.Dice.CurrentRoll.Combinations = dice.Combinations(plan.FinalDice)
	}
	battle.Actors[actorID] = actor
}

func validateKeptIndices(diceValues []state.RolledDie, indices []int) error {
	seen := make(map[int]bool, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(diceValues) {
			return fmt.Errorf("kept die index %d is out of range", index)
		}
		if seen[index] {
			return fmt.Errorf("duplicate kept die index %d", index)
		}
		seen[index] = true
	}
	return nil
}

func validateCommittedCards(hand []string, committed []string) error {
	available := make(map[string]int)
	for _, cardID := range hand {
		available[cardID]++
	}
	for _, cardID := range committed {
		if available[cardID] == 0 {
			return fmt.Errorf("card %q is not available in the actor hand", cardID)
		}
		available[cardID]--
	}
	return nil
}

func validatePlanningTargets(eligible []string, selected []string) error {
	seen := make(map[string]bool, len(selected))
	for _, targetID := range selected {
		if targetID == "" {
			return errors.New("target id is required")
		}
		if seen[targetID] {
			return fmt.Errorf("duplicate target %q", targetID)
		}
		if !containsString(eligible, targetID) {
			return fmt.Errorf("target %q is not eligible", targetID)
		}
		seen[targetID] = true
	}
	return nil
}

func abilityAllowedForPlanning(
	battle *state.Battle,
	planningSegment segment.Segment,
	abilityID string,
) bool {
	definition, ok := battle.Content.Abilities[abilityID]
	if !ok {
		return false
	}
	return segmentAllowed(definition.Segments, planningSegment)
}

func hasAllowedPlanningCard(
	battle *state.Battle,
	actor state.ActorState,
	plan state.PlanningActorState,
	planningSegment segment.Segment,
) bool {
	available := make(map[string]int)
	for _, cardID := range actor.Cards.Hand {
		available[cardID]++
	}
	for _, cardID := range plan.CommittedCards {
		available[cardID]--
	}
	for cardID, count := range available {
		definition, ok := battle.Content.Cards[cardID]
		if count > 0 && ok && segmentAllowed(definition.Segments, planningSegment) {
			return true
		}
	}
	return false
}

func segmentAllowed(restrictions []segment.Segment, current segment.Segment) bool {
	if len(restrictions) == 0 {
		return true
	}
	for _, restriction := range restrictions {
		if restriction == current {
			return true
		}
	}
	return false
}

func validateFinalPlanningSelection(
	battle *state.Battle,
	actor state.ActorState,
	plan state.PlanningActorState,
	planningSegment segment.Segment,
) error {
	if plan.Passed {
		return nil
	}
	totalCost := 0
	requiresTarget := false
	if plan.SelectedAbility != "" {
		definition, ok := battle.Content.Abilities[plan.SelectedAbility]
		if !ok {
			return fmt.Errorf("ability %q has no loaded content definition", plan.SelectedAbility)
		}
		if !segmentAllowed(definition.Segments, planningSegment) {
			return fmt.Errorf("ability %q is not allowed for %s planning", plan.SelectedAbility, planningSegment)
		}
		if !diceRequirementMet(definition.DiceRequirement, plan.FinalDice) {
			return fmt.Errorf("ability %q dice requirement %q is not met", plan.SelectedAbility, definition.DiceRequirement)
		}
		totalCost += definition.EnergyCost
		requiresTarget = requiresTarget || definition.RequiresTarget
	}
	for _, cardID := range plan.CommittedCards {
		definition, ok := battle.Content.Cards[cardID]
		if !ok {
			return fmt.Errorf("card %q has no loaded content definition", cardID)
		}
		if !segmentAllowed(definition.Segments, planningSegment) {
			return fmt.Errorf("card %q is not allowed for %s planning", cardID, planningSegment)
		}
		totalCost += definition.EnergyCost
		requiresTarget = requiresTarget || definition.RequiresTarget
	}
	if totalCost > actor.Resources.EnergyPoints {
		return fmt.Errorf("planning selection costs %d energy points; actor has %d", totalCost, actor.Resources.EnergyPoints)
	}
	if requiresTarget && len(plan.SelectedTargets) == 0 {
		return errors.New("planning selection requires a target")
	}
	return nil
}

func diceRequirementMet(requirement string, values []state.RolledDie) bool {
	switch requirement {
	case "", "none":
		return true
	case "small_straight", "large_straight":
		return containsString(dice.Combinations(values), requirement)
	case "five_sixes":
		count := 0
		for _, die := range values {
			if die.Face == 6 {
				count++
			}
		}
		return count >= 5
	default:
		return false
	}
}

func finalizedContentOperations(
	battle *state.Battle,
	plan state.PlanningActorState,
) ([]state.FinalizedOperationProposal, error) {
	var proposals []state.FinalizedOperationProposal
	appendContent := func(contentType, contentID string, occurrence int, definition state.RuntimeContentDefinition) {
		for operationIndex, operationPlan := range definition.Operations {
			proposals = append(proposals, state.FinalizedOperationProposal{
				ID: fmt.Sprintf(
					"final-%s-%s-%s-%d-%d",
					plan.ActorID,
					contentType,
					contentID,
					occurrence,
					operationIndex,
				),
				ContentType:     contentType,
				ContentID:       contentID,
				Operation:       operation.ClonePlans([]operation.Plan{operationPlan})[0],
				SourceActorID:   plan.ActorID,
				SelectedTargets: append([]string(nil), plan.SelectedTargets...),
			})
		}
	}
	if plan.SelectedAbility != "" {
		definition, ok := battle.Content.Abilities[plan.SelectedAbility]
		if !ok {
			return nil, fmt.Errorf("ability %q has no loaded content definition", plan.SelectedAbility)
		}
		appendContent("ability", plan.SelectedAbility, 0, definition)
	}
	occurrences := make(map[string]int)
	for _, cardID := range plan.CommittedCards {
		definition, ok := battle.Content.Cards[cardID]
		if !ok {
			return nil, fmt.Errorf("card %q has no loaded content definition", cardID)
		}
		appendContent("card", cardID, occurrences[cardID], definition)
		occurrences[cardID]++
	}
	return proposals, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func cloneRolledDice(values []state.RolledDie) []state.RolledDie {
	if values == nil {
		return nil
	}
	cloned := make([]state.RolledDie, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].Symbols = append([]string(nil), value.Symbols...)
	}
	return cloned
}

func sortedActorIDs(actors map[string]state.ActorState) []string {
	ids := make([]string, 0, len(actors))
	for actorID := range actors {
		ids = append(ids, actorID)
	}
	sort.Strings(ids)
	return ids
}
