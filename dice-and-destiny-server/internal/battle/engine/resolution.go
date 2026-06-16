package engine

import (
	"errors"
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

type ProposalRule interface {
	Operation() state.ProposalOperation
	Validate(ctx *Context, proposal state.Proposal) error
	Apply(ctx *Context, proposal state.Proposal) ([]event.Event, error)
}

type InteractionAI interface {
	Commit(ctx *Context, window state.InteractionWindow, actorID string) (state.InteractionCommitment, error)
}

type DefaultInteractionAI struct{}

func (DefaultInteractionAI) Commit(
	_ *Context,
	window state.InteractionWindow,
	actorID string,
) (state.InteractionCommitment, error) {
	if !window.PassAllowed {
		return state.InteractionCommitment{}, fmt.Errorf(
			"automatic actor %q requires an interaction commitment for window %q",
			actorID,
			window.ID,
		)
	}
	return state.InteractionCommitment{
		ID:      fmt.Sprintf("commit-%s-%s", window.ID, actorID),
		ActorID: actorID,
		Command: command.TypePass,
		Passed:  true,
	}, nil
}

type ResolutionSpec struct {
	ID             string
	Batch          state.ProposalBatch
	InitialWindow  WindowSpec
	ReactionPolicy *state.ReactionWindowPolicy
}

type WindowSpec struct {
	ID                string
	Purpose           state.InteractionPurpose
	Source            state.SourceReference
	EligibleActors    []string
	RequiredActors    []string
	AllowedCommands   []command.Type
	HiddenCommitments bool
	PassAllowed       bool
	ReactionRound     int
	ChainDepth        int
}

func BeginResolution(ctx *Context, spec ResolutionSpec) error {
	if ctx == nil || ctx.Battle == nil {
		return errors.New("battle is required")
	}
	battle := ctx.Battle
	if battle.ActiveResolutionID != "" {
		return fmt.Errorf("resolution %q is already active", battle.ActiveResolutionID)
	}
	if spec.ID == "" {
		return errors.New("resolution id is required")
	}
	if !isValidFlowPhase(ctx.Phase) {
		return fmt.Errorf("invalid resolution phase %q", ctx.Phase)
	}
	if _, exists := battle.Resolutions[spec.ID]; exists {
		return fmt.Errorf("resolution %q already exists", spec.ID)
	}
	if battle.Resolutions == nil {
		battle.Resolutions = make(map[string]state.ResolutionState)
	}
	if spec.Batch.ID == "" {
		return errors.New("proposal batch id is required")
	}
	if spec.Batch.ResolutionID == "" {
		spec.Batch.ResolutionID = spec.ID
	}
	if spec.Batch.ResolutionID != spec.ID {
		return errors.New("proposal batch resolution id does not match resolution")
	}
	if err := validateProposalBatch(spec.Batch); err != nil {
		return err
	}

	checkpoint := state.ResolutionCheckpoint{
		Segment:   battle.Segment.Current,
		Phase:     ctx.Phase,
		Stage:     battle.Flow.Stage,
		Iteration: battle.Flow.Iteration,
	}
	window, err := newInteractionWindow(spec.InitialWindow, checkpoint)
	if err != nil {
		return err
	}
	if err := validateWindowActors(battle, window.EligibleActors); err != nil {
		return err
	}
	if spec.ReactionPolicy != nil {
		reactionWindow, err := newInteractionWindow(WindowSpec{
			ID:                spec.ID + "-reaction-policy",
			Purpose:           state.InteractionPurposeReaction,
			Source:            spec.ReactionPolicy.Source,
			EligibleActors:    spec.ReactionPolicy.EligibleActors,
			RequiredActors:    spec.ReactionPolicy.RequiredActors,
			AllowedCommands:   spec.ReactionPolicy.AllowedCommands,
			HiddenCommitments: spec.ReactionPolicy.HiddenCommitments,
			PassAllowed:       spec.ReactionPolicy.PassAllowed,
			ReactionRound:     1,
			ChainDepth:        1,
		}, checkpoint)
		if err != nil {
			return fmt.Errorf("invalid reaction policy: %w", err)
		}
		if err := validateWindowActors(battle, reactionWindow.EligibleActors); err != nil {
			return err
		}
	}
	resolution := state.ResolutionState{
		ID:                    spec.ID,
		Origin:                checkpoint,
		Stage:                 state.ResolutionCollecting,
		Batch:                 spec.Batch,
		Windows:               map[string]state.InteractionWindow{window.ID: window},
		ActiveWindowID:        window.ID,
		WindowSequence:        1,
		ReactionPolicy:        copyReactionPolicy(spec.ReactionPolicy),
		SuspendedActors:       copyActorFlowStates(battle.Flow.Actors),
		SuspendedPendingInput: copyPendingInputs(battle.Flow.PendingInput),
	}
	battle.Resolutions[resolution.ID] = resolution
	battle.ActiveResolutionID = resolution.ID
	battle.Flow.Actors = make(map[string]state.ActorFlowState)
	battle.Flow.PendingInput = make(map[string]state.PendingInput)
	return nil
}

func (e Engine) progressResolution(battle *state.Battle) (ProgressResult, error) {
	working := battle.Clone()
	resolution, window, err := activeWindow(&working)
	if err != nil {
		return ProgressResult{}, err
	}
	if window.Purpose == state.InteractionPurposeReaction {
		if window.ReactionRound > e.maxReactionRounds {
			return ProgressResult{}, fmt.Errorf(
				"%w: limit %d for resolution %q",
				ErrReactionRoundLimit,
				e.maxReactionRounds,
				resolution.ID,
			)
		}
		if window.ChainDepth > e.maxReactionChainDepth {
			return ProgressResult{}, fmt.Errorf(
				"%w: limit %d for resolution %q",
				ErrReactionChainDepthLimit,
				e.maxReactionChainDepth,
				resolution.ID,
			)
		}
	}
	ctx := e.context(&working, resolution.Origin.Phase)
	if resolution.Planning != nil {
		result, err := e.progressPlanningResolution(ctx, resolution, window)
		if err != nil {
			return ProgressResult{}, err
		}
		*battle = working
		return result, nil
	}

	switch window.RevealStatus {
	case state.RevealStatusCollecting:
		result, err := e.collectWindow(ctx, resolution, window)
		if err != nil {
			return ProgressResult{}, err
		}
		*battle = working
		return result, nil
	case state.RevealStatusRevealed:
		result, err := e.advanceRevealedWindow(ctx, resolution, window)
		if err != nil {
			return ProgressResult{}, err
		}
		*battle = working
		return result, nil
	default:
		return ProgressResult{}, fmt.Errorf("invalid reveal status %q", window.RevealStatus)
	}
}

func (e Engine) collectWindow(
	ctx *Context,
	resolution state.ResolutionState,
	window state.InteractionWindow,
) (ProgressResult, error) {
	if !window.Opened {
		window.Opened = true
		resolution.Windows[window.ID] = window
		ctx.Battle.Resolutions[resolution.ID] = resolution
		return progress(
			ProgressContinue,
			event.NewInteractionWindowOpened(resolution.ID, window),
		), nil
	}
	for _, actorID := range sortedRequiredActors(window) {
		if window.ActorProgress[actorID] != state.InteractionActorAwaiting {
			continue
		}
		actor := ctx.Battle.Actors[actorID]
		switch actor.Controller {
		case state.ControllerHuman:
			ensureWindowPendingInput(ctx.Battle, resolution, window, actorID)
		case state.ControllerAI, state.ControllerSystem:
			commitment, err := e.interactionAI.Commit(ctx, window, actorID)
			if err != nil {
				return ProgressResult{}, err
			}
			if err := recordWindowCommitment(&window, actorID, commitment); err != nil {
				return ProgressResult{}, err
			}
			resolution.Windows[window.ID] = window
			ctx.Battle.Resolutions[resolution.ID] = resolution
			privateActorID := ""
			if window.HiddenCommitments {
				privateActorID = actorID
			}
			return progress(
				ProgressContinue,
				event.NewInteractionCommitted(resolution.ID, window, commitment, privateActorID),
			), nil
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
	}
	resolution.Stage = state.ResolutionRevealing
	resolution.Windows[window.ID] = window
	ctx.Battle.Resolutions[resolution.ID] = resolution
	return progress(
		ProgressContinue,
		event.NewInteractionRevealed(resolution.ID, window),
	), nil
}

func (e Engine) advanceRevealedWindow(
	ctx *Context,
	resolution state.ResolutionState,
	window state.InteractionWindow,
) (ProgressResult, error) {
	if window.Purpose != state.InteractionPurposeReaction && resolution.ReactionPolicy != nil {
		resolution.Batch.Revealed = true
		ctx.Battle.Resolutions[resolution.ID] = resolution
		if err := e.openReactionWindow(ctx.Battle, &resolution, 1, 1); err != nil {
			return ProgressResult{}, err
		}
		return progress(ProgressContinue, event.NewProposalBatchRevealed(resolution)), nil
	}
	if window.Purpose == state.InteractionPurposeReaction && windowHasNonPassCommitment(window) {
		if resolution.DamageResolutionID != "" {
			return e.advanceDamageReactionWindow(ctx, resolution, window)
		}
		if err := e.openReactionWindow(ctx.Battle, &resolution, window.ReactionRound+1, window.ChainDepth+1); err != nil {
			return ProgressResult{}, err
		}
		return progress(ProgressContinue), nil
	}
	if window.Purpose == state.InteractionPurposeReaction && resolution.DamageResolutionID != "" {
		return e.advanceDamageReactionWindow(ctx, resolution, window)
	}

	resolution.Stage = state.ResolutionCommitting
	events, err := e.commitProposalBatch(ctx, &resolution)
	if err != nil {
		return ProgressResult{}, err
	}
	resolution.Stage = state.ResolutionComplete
	resolution.ActiveWindowID = ""
	ctx.Battle.Resolutions[resolution.ID] = resolution
	ctx.Battle.ActiveResolutionID = ""
	ctx.Battle.Flow.Stage = resolution.Origin.Stage
	ctx.Battle.Flow.Iteration = resolution.Origin.Iteration
	ctx.Battle.Flow.Actors = copyActorFlowStates(resolution.SuspendedActors)
	ctx.Battle.Flow.PendingInput = copyPendingInputs(resolution.SuspendedPendingInput)
	events = append(events, event.NewResolutionCompleted(resolution))
	return progress(ProgressContinue, events...), nil
}

func (e Engine) openReactionWindow(
	battle *state.Battle,
	resolution *state.ResolutionState,
	reactionRound int,
	chainDepth int,
) error {
	if reactionRound > e.maxReactionRounds {
		return fmt.Errorf(
			"%w: limit %d for resolution %q",
			ErrReactionRoundLimit,
			e.maxReactionRounds,
			resolution.ID,
		)
	}
	if chainDepth > e.maxReactionChainDepth {
		return fmt.Errorf(
			"%w: limit %d for resolution %q",
			ErrReactionChainDepthLimit,
			e.maxReactionChainDepth,
			resolution.ID,
		)
	}
	policy := resolution.ReactionPolicy
	if policy == nil {
		return errors.New("reaction policy is required")
	}
	var windowID string
	for {
		resolution.WindowSequence++
		windowID = fmt.Sprintf("%s-window-%d", resolution.ID, resolution.WindowSequence)
		if _, exists := resolution.Windows[windowID]; !exists {
			break
		}
	}
	window, err := newInteractionWindow(WindowSpec{
		ID:                windowID,
		Purpose:           state.InteractionPurposeReaction,
		Source:            policy.Source,
		EligibleActors:    policy.EligibleActors,
		RequiredActors:    policy.RequiredActors,
		AllowedCommands:   policy.AllowedCommands,
		HiddenCommitments: policy.HiddenCommitments,
		PassAllowed:       policy.PassAllowed,
		ReactionRound:     reactionRound,
		ChainDepth:        chainDepth,
	}, resolution.Origin)
	if err != nil {
		return err
	}
	resolution.Stage = state.ResolutionReacting
	resolution.ActiveWindowID = window.ID
	resolution.Windows[window.ID] = window
	battle.Flow.Actors = make(map[string]state.ActorFlowState)
	battle.Flow.PendingInput = make(map[string]state.PendingInput)
	if resolution.Planning != nil {
		for actorID, plan := range resolution.Planning.Actors {
			status := state.ActorLockedIn
			if plan.Participation == state.ActorNotParticipating {
				status = state.ActorNotParticipating
			}
			battle.Flow.Actors[actorID] = state.ActorFlowState{
				Status:     status,
				ReasonCode: string(resolution.Planning.Segment) + "_planning_reaction",
			}
		}
	}
	battle.Resolutions[resolution.ID] = *resolution
	return nil
}

func (e Engine) commitProposalBatch(
	ctx *Context,
	resolution *state.ResolutionState,
) ([]event.Event, error) {
	for _, proposal := range resolution.Batch.Proposals {
		rule, ok := e.proposalRules[proposal.Operation]
		if !ok {
			return nil, fmt.Errorf("no proposal rule registered for operation %q", proposal.Operation)
		}
		if err := rule.Validate(ctx, proposal); err != nil {
			return nil, fmt.Errorf("validate proposal %q: %w", proposal.ID, err)
		}
	}

	var events []event.Event
	for _, proposal := range resolution.Batch.Proposals {
		proposalEvents, err := e.proposalRules[proposal.Operation].Apply(ctx, proposal)
		if err != nil {
			return nil, fmt.Errorf("apply proposal %q: %w", proposal.ID, err)
		}
		events = append(events, proposalEvents...)
	}
	resolution.Batch.Committed = true
	resolution.Batch.Revealed = true
	ctx.Battle.Resolutions[resolution.ID] = *resolution
	events = append(events, event.NewProposalBatchCommitted(*resolution))
	return events, nil
}

func newInteractionWindow(
	spec WindowSpec,
	checkpoint state.ResolutionCheckpoint,
) (state.InteractionWindow, error) {
	if spec.ID == "" {
		return state.InteractionWindow{}, errors.New("interaction window id is required")
	}
	if !isValidInteractionPurpose(spec.Purpose) {
		return state.InteractionWindow{}, fmt.Errorf("invalid interaction purpose %q", spec.Purpose)
	}
	if len(spec.EligibleActors) == 0 {
		return state.InteractionWindow{}, errors.New("interaction window eligible actors are required")
	}
	if len(spec.RequiredActors) == 0 {
		return state.InteractionWindow{}, errors.New("interaction window required actors are required")
	}
	if len(spec.AllowedCommands) == 0 {
		return state.InteractionWindow{}, errors.New("interaction window allowed commands are required")
	}
	if spec.Source.Type == "" || spec.Source.ID == "" {
		return state.InteractionWindow{}, errors.New("interaction window source is required")
	}
	if spec.Purpose == state.InteractionPurposeReaction &&
		(spec.ReactionRound < 1 || spec.ChainDepth < 1) {
		return state.InteractionWindow{}, errors.New("reaction window round and chain depth must be positive")
	}
	eligible := make(map[string]struct{}, len(spec.EligibleActors))
	progress := make(map[string]state.InteractionActorStatus, len(spec.EligibleActors))
	for _, actorID := range spec.EligibleActors {
		if actorID == "" {
			return state.InteractionWindow{}, errors.New("interaction window actor id is required")
		}
		if _, exists := eligible[actorID]; exists {
			return state.InteractionWindow{}, fmt.Errorf("duplicate eligible actor %q", actorID)
		}
		eligible[actorID] = struct{}{}
		progress[actorID] = state.InteractionActorEligible
	}
	for _, actorID := range spec.RequiredActors {
		if _, ok := eligible[actorID]; !ok {
			return state.InteractionWindow{}, fmt.Errorf("required actor %q is not eligible", actorID)
		}
		progress[actorID] = state.InteractionActorAwaiting
	}
	if spec.PassAllowed && !commandAllowed(spec.AllowedCommands, command.TypePass) {
		return state.InteractionWindow{}, errors.New("pass-enabled window must allow the pass command")
	}
	return state.InteractionWindow{
		ID:                  spec.ID,
		Purpose:             spec.Purpose,
		Source:              spec.Source,
		EligibleActors:      append([]string(nil), spec.EligibleActors...),
		RequiredActors:      append([]string(nil), spec.RequiredActors...),
		ActorProgress:       progress,
		AllowedCommands:     append([]command.Type(nil), spec.AllowedCommands...),
		HiddenCommitments:   spec.HiddenCommitments,
		RevealStatus:        state.RevealStatusCollecting,
		PassAllowed:         spec.PassAllowed,
		Commitments:         make(map[string]state.InteractionCommitment),
		ReactionRound:       spec.ReactionRound,
		ChainDepth:          spec.ChainDepth,
		SuspendedCheckpoint: checkpoint,
	}, nil
}

func validateActiveResolution(battle *state.Battle) error {
	if battle.ActiveResolutionID == "" {
		return nil
	}
	resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
	if !ok {
		return fmt.Errorf("active resolution %q is missing", battle.ActiveResolutionID)
	}
	if resolution.Origin.Segment != battle.Segment.Current ||
		resolution.Origin.Stage != battle.Flow.Stage ||
		resolution.Origin.Iteration != battle.Flow.Iteration {
		return fmt.Errorf("active resolution %q does not match current flow checkpoint", resolution.ID)
	}
	if !isValidFlowPhase(resolution.Origin.Phase) {
		return fmt.Errorf("active resolution %q has invalid phase %q", resolution.ID, resolution.Origin.Phase)
	}
	if resolution.ActiveWindowID == "" {
		return fmt.Errorf("active resolution %q has no active window", resolution.ID)
	}
	window, ok := resolution.Windows[resolution.ActiveWindowID]
	if !ok {
		return fmt.Errorf("active interaction window %q is missing", resolution.ActiveWindowID)
	}
	if window.SuspendedCheckpoint != resolution.Origin {
		return fmt.Errorf("interaction window %q has a stale suspended checkpoint", window.ID)
	}
	return nil
}

func activeWindow(battle *state.Battle) (state.ResolutionState, state.InteractionWindow, error) {
	if battle.ActiveResolutionID == "" {
		return state.ResolutionState{}, state.InteractionWindow{}, errors.New("no active resolution")
	}
	resolution, ok := battle.Resolutions[battle.ActiveResolutionID]
	if !ok {
		return state.ResolutionState{}, state.InteractionWindow{}, errors.New("active resolution is missing")
	}
	window, ok := resolution.Windows[resolution.ActiveWindowID]
	if !ok {
		return state.ResolutionState{}, state.InteractionWindow{}, errors.New("active interaction window is missing")
	}
	return resolution, window, nil
}

func ensureWindowPendingInput(
	battle *state.Battle,
	resolution state.ResolutionState,
	window state.InteractionWindow,
	actorID string,
) {
	if _, exists := battle.Flow.PendingInput[actorID]; exists {
		return
	}
	battle.Flow.Actors[actorID] = state.ActorFlowState{
		Status:     state.ActorNeedsInput,
		ReasonCode: string(window.Purpose),
	}
	battle.Flow.PendingInput[actorID] = state.PendingInput{
		ID:              fmt.Sprintf("input-%s-%s", window.ID, actorID),
		ActorID:         actorID,
		Segment:         resolution.Origin.Segment,
		Phase:           resolution.Origin.Phase,
		Stage:           resolution.Origin.Stage,
		Iteration:       resolution.Origin.Iteration,
		WindowID:        window.ID,
		ReactionRound:   window.ReactionRound,
		InputType:       string(window.Purpose),
		SourceType:      window.Source.Type,
		SourceID:        window.Source.ID,
		AllowedCommands: append([]command.Type(nil), window.AllowedCommands...),
	}
}

func validateWindowActors(battle *state.Battle, actorIDs []string) error {
	for _, actorID := range actorIDs {
		if _, ok := battle.Actors[actorID]; !ok {
			return fmt.Errorf("interaction window actor %q is not in battle", actorID)
		}
	}
	return nil
}

func recordWindowCommitment(
	window *state.InteractionWindow,
	actorID string,
	commitment state.InteractionCommitment,
) error {
	if window.ActorProgress[actorID] != state.InteractionActorAwaiting {
		return fmt.Errorf("actor %q is not awaiting interaction input", actorID)
	}
	if commitment.ID == "" {
		commitment.ID = fmt.Sprintf("commit-%s-%s", window.ID, actorID)
	}
	if commitment.ActorID == "" {
		commitment.ActorID = actorID
	}
	if commitment.ActorID != actorID {
		return fmt.Errorf("commitment actor %q does not match actor %q", commitment.ActorID, actorID)
	}
	if commitment.Passed && !window.PassAllowed {
		return fmt.Errorf("pass is not allowed for window %q", window.ID)
	}
	if commitment.Passed {
		if !commandAllowed(window.AllowedCommands, command.TypePass) {
			return fmt.Errorf("pass is not an allowed command for window %q", window.ID)
		}
		commitment.Command = command.TypePass
		window.ActorProgress[actorID] = state.InteractionActorPassed
	} else {
		if !commandAllowed(window.AllowedCommands, commitment.Command) {
			return fmt.Errorf("command %q is not allowed for window %q", commitment.Command, window.ID)
		}
		window.ActorProgress[actorID] = state.InteractionActorCommitted
	}
	window.Commitments[actorID] = commitment
	return nil
}

func (e Engine) handleInteractionCommand(
	battle *state.Battle,
	cmd command.Command,
) ([]event.Event, error) {
	resolution, window, err := activeWindow(battle)
	if err != nil {
		return nil, err
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
	pending, ok := battle.Flow.PendingInput[cmd.ActorID]
	if !ok {
		return nil, fmt.Errorf("actor %q has no pending input", cmd.ActorID)
	}

	var pendingInputID string
	var checkpoint command.InteractionCheckpoint
	commitment := state.InteractionCommitment{
		ID:      fmt.Sprintf("commit-%s-%s", window.ID, cmd.ActorID),
		ActorID: cmd.ActorID,
		Command: cmd.Type,
	}
	switch cmd.Type {
	case command.TypeCommitInteraction:
		var payload command.CommitInteractionPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid commit_interaction payload")
		}
		pendingInputID = payload.PendingInputID
		checkpoint = payload.Checkpoint
		commitment.Data = state.InteractionCommitmentData{
			ProposalIDs:         append([]string(nil), payload.Commitment.ProposalIDs...),
			CardIDs:             append([]string(nil), payload.Commitment.CardIDs...),
			TargetIDs:           append([]string(nil), payload.Commitment.TargetIDs...),
			ChoiceID:            payload.Commitment.ChoiceID,
			Value:               copyIntPointer(payload.Commitment.Value),
			PlanningAdjustments: copyPlanningAdjustments(payload.Commitment.PlanningAdjustments),
			DamageReactions:     copyDamageReactions(payload.Commitment.DamageReactions),
		}
	case command.TypePass:
		var payload command.PassPayload
		if err := command.DecodePayload(cmd, &payload); err != nil {
			return nil, errors.New("invalid pass payload")
		}
		pendingInputID = payload.PendingInputID
		checkpoint = payload.Checkpoint
		commitment.Passed = true
	default:
		return nil, fmt.Errorf("command %q is not supported by interaction windows", cmd.Type)
	}
	if pendingInputID == "" {
		return nil, errors.New("pending_input_id is required")
	}
	if pending.ID != pendingInputID {
		return nil, fmt.Errorf("pending input %q is stale; current input is %q", pendingInputID, pending.ID)
	}
	if checkpoint.WindowID != window.ID ||
		checkpoint.Stage != resolution.Origin.Stage ||
		checkpoint.Iteration != resolution.Origin.Iteration ||
		checkpoint.ReactionRound != window.ReactionRound {
		return nil, fmt.Errorf("interaction command has a stale window checkpoint")
	}
	if pending.WindowID != window.ID ||
		pending.Stage != checkpoint.Stage ||
		pending.Iteration != checkpoint.Iteration ||
		pending.ReactionRound != checkpoint.ReactionRound {
		return nil, fmt.Errorf("pending input %q does not match interaction checkpoint", pending.ID)
	}
	if !commandAllowed(window.AllowedCommands, cmd.Type) {
		return nil, fmt.Errorf("command %q is not allowed for window %q", cmd.Type, window.ID)
	}
	if err := recordWindowCommitment(&window, cmd.ActorID, commitment); err != nil {
		return nil, err
	}
	delete(battle.Flow.PendingInput, cmd.ActorID)
	battle.Flow.Actors[cmd.ActorID] = state.ActorFlowState{
		Status:       state.ActorLockedIn,
		ReasonCode:   string(window.Purpose),
		CommitmentID: commitment.ID,
	}
	resolution.Windows[window.ID] = window
	battle.Resolutions[resolution.ID] = resolution
	privateActorID := ""
	if window.HiddenCommitments {
		privateActorID = cmd.ActorID
	}
	return []event.Event{
		event.NewInteractionCommitted(resolution.ID, window, commitment, privateActorID),
	}, nil
}

func validateProposalBatch(batch state.ProposalBatch) error {
	if len(batch.Proposals) == 0 {
		return errors.New("proposal batch requires at least one proposal")
	}
	seen := make(map[string]struct{}, len(batch.Proposals))
	for _, proposal := range batch.Proposals {
		if proposal.ID == "" {
			return errors.New("proposal id is required")
		}
		if _, exists := seen[proposal.ID]; exists {
			return fmt.Errorf("duplicate proposal id %q", proposal.ID)
		}
		seen[proposal.ID] = struct{}{}
		if proposal.Source.Type == "" || proposal.Source.ID == "" {
			return fmt.Errorf("proposal %q source is required", proposal.ID)
		}
		if proposal.Target.Type == "" || proposal.Target.ID == "" {
			return fmt.Errorf("proposal %q target is required", proposal.ID)
		}
		if proposal.Operation == "" {
			return fmt.Errorf("proposal %q operation is required", proposal.ID)
		}
		payloads := 0
		if proposal.Data.Amount != nil {
			payloads++
		}
		if proposal.Data.Selection != nil {
			payloads++
		}
		if proposal.Data.Roll != nil {
			payloads++
		}
		if proposal.Data.Planning != nil {
			payloads++
		}
		if payloads != 1 {
			return fmt.Errorf("proposal %q requires exactly one typed payload", proposal.ID)
		}
	}
	return nil
}

func windowHasNonPassCommitment(window state.InteractionWindow) bool {
	for _, commitment := range window.Commitments {
		if !commitment.Passed {
			return true
		}
	}
	return false
}

func commandAllowed(allowed []command.Type, commandType command.Type) bool {
	for _, candidate := range allowed {
		if candidate == commandType {
			return true
		}
	}
	return false
}

func isValidInteractionPurpose(purpose state.InteractionPurpose) bool {
	switch purpose {
	case state.InteractionPurposeRequiredRoll,
		state.InteractionPurposePlanning,
		state.InteractionPurposeReaction,
		state.InteractionPurposeChooseCard,
		state.InteractionPurposeSelectTarget:
		return true
	default:
		return false
	}
}

func isValidFlowPhase(phase state.FlowPhase) bool {
	switch phase {
	case state.FlowPhaseOnEnter, state.FlowPhaseInProgress, state.FlowPhaseOnExit:
		return true
	default:
		return false
	}
}

func copyActorFlowStates(values map[string]state.ActorFlowState) map[string]state.ActorFlowState {
	cloned := make(map[string]state.ActorFlowState, len(values))
	for actorID, progress := range values {
		cloned[actorID] = progress
	}
	return cloned
}

func copyPendingInputs(values map[string]state.PendingInput) map[string]state.PendingInput {
	cloned := make(map[string]state.PendingInput, len(values))
	for actorID, pending := range values {
		pending.AllowedCommands = append([]command.Type(nil), pending.AllowedCommands...)
		cloned[actorID] = pending
	}
	return cloned
}

func copyReactionPolicy(policy *state.ReactionWindowPolicy) *state.ReactionWindowPolicy {
	if policy == nil {
		return nil
	}
	cloned := *policy
	cloned.EligibleActors = append([]string(nil), policy.EligibleActors...)
	cloned.RequiredActors = append([]string(nil), policy.RequiredActors...)
	cloned.AllowedCommands = append([]command.Type(nil), policy.AllowedCommands...)
	return &cloned
}

func copyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func copyPlanningAdjustments(values []command.PlanningAdjustment) []state.PlanningAdjustment {
	if values == nil {
		return nil
	}
	copied := make([]state.PlanningAdjustment, len(values))
	for i, value := range values {
		copied[i] = state.PlanningAdjustment{
			Type:     state.PlanningAdjustmentType(value.Type),
			ActorID:  value.ActorID,
			DieIndex: value.DieIndex,
			Face:     value.Face,
			Amount:   value.Amount,
			TargetID: value.TargetID,
		}
	}
	return copied
}

func copyDamageReactions(values []command.DamageReaction) []state.DamageReaction {
	if values == nil {
		return nil
	}
	copied := make([]state.DamageReaction, len(values))
	for i, value := range values {
		copied[i] = state.DamageReaction{
			Type:              state.DamageReactionType(value.Type),
			ProposalID:        value.ProposalID,
			Amount:            value.Amount,
			ReplacementCardID: value.ReplacementCardID,
		}
	}
	return copied
}

func sortedRequiredActors(window state.InteractionWindow) []string {
	actors := append([]string(nil), window.RequiredActors...)
	sort.Strings(actors)
	return actors
}
