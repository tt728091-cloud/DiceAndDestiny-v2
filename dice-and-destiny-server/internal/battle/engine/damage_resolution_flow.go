package engine

import (
	"fmt"
	"sort"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const damageResolutionStage = "damage_resolution"

type DamageResolutionFlow struct {
	RuntimeRegistry *operation.RuntimeRegistry
	RandomSource    damage.RandomSource
	EligibleActors  func(*state.Battle) []string
}

func (DamageResolutionFlow) ID() segment.Segment {
	return segment.DamageResolution
}

func (flow DamageResolutionFlow) OnEnter(ctx *Context) ([]event.Event, error) {
	initializeAutomaticActors(ctx.Battle)
	ctx.Battle.Flow.Stage = damageResolutionStage
	ctx.Battle.DamageResolution = &state.DamageResolutionState{
		ID:                       fmt.Sprintf("damage-round-%d", ctx.Battle.Segment.Round),
		Stage:                    state.DamageStageCollect,
		AppliedReactionWindowIDs: make(map[string]bool),
	}
	return nil, nil
}

func (flow DamageResolutionFlow) Progress(ctx *Context) (ProgressResult, error) {
	resolution := ctx.Battle.DamageResolution
	if resolution == nil {
		return ProgressResult{}, fmt.Errorf("damage resolution state is missing")
	}
	switch resolution.Stage {
	case state.DamageStageCollect:
		if err := damage.Collect(ctx.Battle, flow.runtimeRegistry(), resolution); err != nil {
			return ProgressResult{}, err
		}
		resolution.Stage = state.DamageStageCalculate
		events := make([]event.Event, 0, len(resolution.SourceProposals))
		for _, proposal := range resolution.SourceProposals {
			events = append(events, event.NewDamageProposed(proposal))
		}
		return progress(ProgressContinue, events...), nil
	case state.DamageStageCalculate:
		damage.Recalculate(resolution)
		resolution.Stage = state.DamageStageSelectCards
		return progress(ProgressContinue), nil
	case state.DamageStageSelectCards:
		if _, err := damage.ReconcileCards(ctx.Battle, resolution, flow.randomSource()); err != nil {
			return ProgressResult{}, err
		}
		resolution.Stage = state.DamageStageReveal
		return progress(ProgressContinue), nil
	case state.DamageStageReveal:
		revealed := damage.RevealCards(resolution)
		revealEvents := damageCardRevealEvents(revealed)
		if len(resolution.SourceProposals) == 0 && len(resolution.PendingOperations) == 0 {
			resolution.Stage = state.DamageStageConsequences
			return progress(ProgressContinue, revealEvents...), nil
		}
		resolution.ReactionResolutionID = resolution.ID + "-reaction"
		batch := damage.BuildProposalBatch(resolution)
		actors := flow.eligibleActors(ctx.Battle)
		if err := BeginResolution(ctx, ResolutionSpec{
			ID:    resolution.ReactionResolutionID,
			Batch: batch,
			InitialWindow: WindowSpec{
				ID:                resolution.ReactionResolutionID + "-window-1",
				Purpose:           state.InteractionPurposeReaction,
				Source:            state.SourceReference{Type: "damage_resolution", ID: resolution.ID},
				EligibleActors:    actors,
				RequiredActors:    actors,
				AllowedCommands:   []command.Type{command.TypeCommitInteraction, command.TypePass},
				HiddenCommitments: true,
				PassAllowed:       true,
				ReactionRound:     1,
				ChainDepth:        1,
			},
			ReactionPolicy: &state.ReactionWindowPolicy{
				Source:            state.SourceReference{Type: "damage_resolution", ID: resolution.ID},
				EligibleActors:    actors,
				RequiredActors:    actors,
				AllowedCommands:   []command.Type{command.TypeCommitInteraction, command.TypePass},
				HiddenCommitments: true,
				PassAllowed:       true,
			},
		}); err != nil {
			return ProgressResult{}, err
		}
		generic := ctx.Battle.Resolutions[resolution.ReactionResolutionID]
		generic.Stage = state.ResolutionReacting
		generic.Batch.Revealed = true
		generic.DamageResolutionID = resolution.ID
		ctx.Battle.Resolutions[generic.ID] = generic
		resolution.Stage = state.DamageStageReaction
		return progress(ProgressContinue, revealEvents...), nil
	case state.DamageStageReaction:
		return ProgressResult{}, fmt.Errorf("damage reaction state is not active")
	case state.DamageStageConsequences:
		// Status execution and status-triggered immediate work begin in Phase 7.
		resolution.Stage = state.DamageStageComplete
		return progress(ProgressContinue), nil
	case state.DamageStageComplete:
		resolveAutomaticActors(ctx.Battle)
		return progress(ProgressSegmentComplete), nil
	default:
		return ProgressResult{}, fmt.Errorf("unknown damage resolution stage %q", resolution.Stage)
	}
}

func (DamageResolutionFlow) HandleCommand(*Context, command.Command) ([]event.Event, error) {
	return nil, unsupportedCommand()
}

func (DamageResolutionFlow) OnExit(ctx *Context) ([]event.Event, error) {
	ctx.Battle.DamageResolution = nil
	return nil, nil
}

func (flow DamageResolutionFlow) runtimeRegistry() *operation.RuntimeRegistry {
	if flow.RuntimeRegistry != nil {
		return flow.RuntimeRegistry
	}
	return operation.DefaultRuntimeRegistry()
}

func (flow DamageResolutionFlow) randomSource() damage.RandomSource {
	if flow.RandomSource != nil {
		return flow.RandomSource
	}
	return dice.CryptoRandomSource{}
}

func (flow DamageResolutionFlow) eligibleActors(battle *state.Battle) []string {
	if flow.EligibleActors != nil {
		return append([]string(nil), flow.EligibleActors(battle)...)
	}
	actors := make([]string, 0, len(battle.Actors))
	for actorID := range battle.Actors {
		actors = append(actors, actorID)
	}
	sort.Strings(actors)
	return actors
}

func damageCardRevealEvents(cards []state.ProposedCardRemoval) []event.Event {
	grouped := make(map[string][]state.ProposedCardRemoval)
	for _, card := range cards {
		grouped[card.TargetActorID] = append(grouped[card.TargetActorID], card)
	}
	actorIDs := make([]string, 0, len(grouped))
	for actorID := range grouped {
		actorIDs = append(actorIDs, actorID)
	}
	sort.Strings(actorIDs)
	events := make([]event.Event, 0, len(actorIDs))
	for _, actorID := range actorIDs {
		events = append(events, event.NewDamageCardsRevealed(actorID, grouped[actorID]))
	}
	return events
}

func (e Engine) advanceDamageReactionWindow(
	ctx *Context,
	resolution state.ResolutionState,
	window state.InteractionWindow,
) (ProgressResult, error) {
	damageState := ctx.Battle.DamageResolution
	if damageState == nil || damageState.ID != resolution.DamageResolutionID {
		return ProgressResult{}, fmt.Errorf("active damage resolution %q is missing", resolution.DamageResolutionID)
	}

	var events []event.Event
	if !damageState.AppliedReactionWindowIDs[window.ID] {
		var reactions []state.DamageReaction
		actorIDs := make([]string, 0, len(window.Commitments))
		for actorID := range window.Commitments {
			actorIDs = append(actorIDs, actorID)
		}
		sort.Strings(actorIDs)
		for _, actorID := range actorIDs {
			reactions = append(reactions, window.Commitments[actorID].Data.DamageReactions...)
		}
		if err := damage.ApplyReactions(ctx.Battle, damageState, reactions); err != nil {
			return ProgressResult{}, err
		}
		for _, reaction := range reactions {
			targetActorID := ""
			for _, total := range damageState.AccumulatedProposals {
				if total.ID == reaction.ProposalID {
					targetActorID = total.TargetActorID
				}
			}
			for _, source := range damageState.SourceProposals {
				if source.ID == reaction.ProposalID {
					targetActorID = source.TargetActorID
				}
			}
			for _, card := range damageState.CardProposals {
				if card.ID == reaction.ProposalID {
					targetActorID = card.TargetActorID
				}
			}
			events = append(events, event.NewDamageModified(
				reaction.ProposalID,
				targetActorID,
				reaction.Amount,
			))
		}
		flow, err := e.damageFlow()
		if err != nil {
			return ProgressResult{}, err
		}
		if _, err := damage.ReconcileCards(ctx.Battle, damageState, flow.randomSource()); err != nil {
			return ProgressResult{}, err
		}
		events = append(events, damageCardRevealEvents(damage.RevealCards(damageState))...)
		damageState.AppliedReactionWindowIDs[window.ID] = true
		resolution.Batch = damage.BuildProposalBatch(damageState)
		resolution.Batch.ResolutionID = resolution.ID
		ctx.Battle.Resolutions[resolution.ID] = resolution
	}

	if windowHasNonPassCommitment(window) {
		if err := e.openReactionWindow(
			ctx.Battle,
			&resolution,
			window.ReactionRound+1,
			window.ChainDepth+1,
		); err != nil {
			return ProgressResult{}, err
		}
		return progress(ProgressContinue, events...), nil
	}

	commitResult, err := damage.Commit(ctx.Battle, damageState)
	if err != nil {
		return ProgressResult{}, err
	}
	for _, source := range commitResult.Sources {
		events = append(events, event.NewDamageCommitted(
			source.ID,
			source.SourceActorID,
			source.SourceContentID,
			source.TargetActorID,
			source.FinalAmount,
		))
	}
	for _, total := range commitResult.Totals {
		events = append(events, event.NewDamageCommitted(
			total.ID,
			"",
			"",
			total.TargetActorID,
			total.FinalAmount,
		))
	}
	for _, removed := range commitResult.Cards {
		events = append(events, event.NewCardPermanentlyRemoved(removed.Proposal))
	}

	damageState = ctx.Battle.DamageResolution
	damageState.SourceProposals = nil
	damageState.ModifierProposals = nil
	damageState.AccumulatedProposals = nil
	damageState.CardProposals = nil
	damageState.PendingOperations = nil
	damageState.Committed = true
	damageState.Stage = state.DamageStageConsequences

	resolution.Batch.Committed = true
	resolution.Stage = state.ResolutionComplete
	resolution.ActiveWindowID = ""
	ctx.Battle.Resolutions[resolution.ID] = resolution
	ctx.Battle.ActiveResolutionID = ""
	ctx.Battle.Flow.Stage = resolution.Origin.Stage
	ctx.Battle.Flow.Iteration = resolution.Origin.Iteration
	ctx.Battle.Flow.Actors = copyActorFlowStates(resolution.SuspendedActors)
	ctx.Battle.Flow.PendingInput = copyPendingInputs(resolution.SuspendedPendingInput)
	events = append(events, event.NewProposalBatchCommitted(resolution))
	events = append(events, event.NewResolutionCompleted(resolution))
	return progress(ProgressContinue, events...), nil
}

func (e Engine) damageFlow() (DamageResolutionFlow, error) {
	flow, err := e.FlowFor(segment.DamageResolution)
	if err != nil {
		return DamageResolutionFlow{}, err
	}
	switch typed := flow.(type) {
	case DamageResolutionFlow:
		return typed, nil
	case *DamageResolutionFlow:
		return *typed, nil
	default:
		return DamageResolutionFlow{}, fmt.Errorf("registered damage flow does not support damage reactions")
	}
}
