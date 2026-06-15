package engine_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/repository"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestProductionDamageResolutionFlowPersistsReactsRecalculatesAndCommits(t *testing.T) {
	battle := damageFlowBattle(t)
	damageFlow := engine.DamageResolutionFlow{
		RandomSource: dice.NewSequenceRandomSource(0),
	}
	eng, err := engine.NewEngineWithFlows(damageFlow, &waitAfterDamageFlow{})
	if err != nil {
		t.Fatalf("NewEngineWithFlows() returned error: %v", err)
	}

	initialCards := battle.Actors["player"].Cards
	progressed, err := eng.ProgressUntilInput(&battle)
	if err != nil {
		t.Fatalf("ProgressUntilInput() returned error: %v", err)
	}
	if progressed.Status != engine.ProgressWaitingForInput {
		t.Fatalf("status = %q, want waiting_for_input", progressed.Status)
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, initialCards) {
		t.Fatal("damage proposal or reveal mutated health-card zones")
	}
	if countEvents(progressed.Events, event.TypeDamageProposed) != 1 ||
		countEvents(progressed.Events, event.TypeDamageCardsRevealed) != 1 {
		t.Fatalf("initial damage events = %#v", progressed.Events)
	}
	reveal := firstEvent(progressed.Events, event.TypeDamageCardsRevealed)
	if len(reveal.DamageCards) != 2 {
		t.Fatalf("revealed cards = %#v, want one combined two-card reveal", reveal.DamageCards)
	}
	playerView := snapshot.FromBattleForViewer(battle, "player")
	enemyView := snapshot.FromBattleForViewer(battle, "enemy")
	if playerView.Damage == nil ||
		len(playerView.Damage.PendingTotals) != 1 ||
		len(playerView.Damage.RevealedCards) != 2 ||
		playerView.Damage.ActiveInteractionID == "" ||
		!reflect.DeepEqual(playerView.Damage.RevealedCards, enemyView.Damage.RevealedCards) {
		t.Fatalf("active damage snapshots are incomplete: player=%#v enemy=%#v", playerView.Damage, enemyView.Damage)
	}

	repo := repository.NewInMemory()
	if err := repo.Create(repository.Checkpoint{Battle: battle, Events: progressed.Events}); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	loaded, err := repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !reflect.DeepEqual(loaded.Battle.DamageResolution, battle.DamageResolution) {
		t.Fatal("damage proposals changed across repository save/load")
	}
	loaded.Battle.DamageResolution.CardProposals[0].CardID = "tampered"
	reloaded, err := repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("second Load() returned error: %v", err)
	}
	if reloaded.Battle.DamageResolution.CardProposals[0].CardID == "tampered" {
		t.Fatal("repository damage proposal state aliases a loaded checkpoint")
	}
	battle = reloaded.Battle

	pending := battle.Flow.PendingInput["player"]
	reacted, err := applyInteraction(t, eng, &battle, command.TypeCommitInteraction, command.CommitInteractionPayload{
		PendingInputID: pending.ID,
		Checkpoint:     interactionCheckpoint(pending),
		Commitment: command.InteractionCommitmentData{
			DamageReactions: []command.DamageReaction{
				{
					Type:       string(state.DamageReactionPreventAccumulated),
					ProposalID: "damage-total-player",
					Amount:     1,
				},
				{
					Type:       string(state.DamageReactionModifySource),
					ProposalID: "attack-damage-target-player",
					Amount:     2,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("damage reaction returned error: %v", err)
	}
	if reacted.Status != engine.ProgressWaitingForInput {
		t.Fatalf("reaction status = %q, want next reaction round", reacted.Status)
	}
	if countEvents(reacted.Events, event.TypeDamageCardsRevealed) != 1 {
		t.Fatalf("increased damage did not reveal additional cards: %#v", reacted.Events)
	}
	if activeDamageCards(battle) != 3 {
		t.Fatalf("active proposed cards = %d, want 3 after recalculation", activeDamageCards(battle))
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, initialCards) {
		t.Fatal("recalculation mutated health-card zones before commit")
	}
	if err := repo.Save(repository.Checkpoint{Battle: battle, Events: reacted.Events}); err != nil {
		t.Fatalf("Save() at second reaction wait returned error: %v", err)
	}
	loaded, err = repo.Load(battle.ID)
	if err != nil {
		t.Fatalf("Load() at second reaction wait returned error: %v", err)
	}
	battle = loaded.Battle
	if activeDamageCards(battle) != 3 || battle.Flow.PendingInput["player"].ReactionRound != 2 {
		t.Fatalf("second reaction wait did not resume correctly: %#v", battle.DamageResolution)
	}

	pending = battle.Flow.PendingInput["player"]
	committed, err := applyInteraction(t, eng, &battle, command.TypePass, command.PassPayload{
		PendingInputID: pending.ID,
		Checkpoint:     interactionCheckpoint(pending),
	})
	if err != nil {
		t.Fatalf("reaction pass returned error: %v", err)
	}
	if committed.Status != engine.ProgressWaitingForInput ||
		battle.Segment.Current != segment.OngoingEffects {
		t.Fatalf("post-commit result = %#v segment=%q, want next segment wait", committed, battle.Segment.Current)
	}
	if len(battle.Actors["player"].Cards.Removed) != 3 ||
		battle.Actors["player"].CurrentHealth() != 1 {
		t.Fatalf("committed player cards = %#v", battle.Actors["player"].Cards)
	}
	if countEvents(committed.Events, event.TypeDamageCommitted) != 2 ||
		countEvents(committed.Events, event.TypeCardsPermanentlyRemoved) != 3 {
		t.Fatalf("commit events = %#v", committed.Events)
	}
}

func TestDamageVisibilityHidesCardsBeforeRevealAndPublishesAfterReveal(t *testing.T) {
	battle := damageFlowBattle(t)
	battle.DamageResolution = &state.DamageResolutionState{
		ID:    "damage",
		Stage: state.DamageStageSelectCards,
		AccumulatedProposals: []state.AccumulatedDamageProposal{{
			ID:            "damage-total-player",
			TargetActorID: "player",
			FinalAmount:   1,
		}},
		CardProposals: []state.ProposedCardRemoval{{
			ID:            "card-1",
			TargetActorID: "player",
			CardID:        "p1",
			OriginalZone:  operation.ZoneDeck,
			Accepted:      true,
		}},
	}
	for _, viewer := range []string{"player", "enemy"} {
		view := snapshot.FromBattleForViewer(battle, viewer)
		if view.Damage == nil || len(view.Damage.RevealedCards) != 0 {
			t.Fatalf("viewer %q pre-reveal damage snapshot = %#v", viewer, view.Damage)
		}
		if len(view.Damage.PendingTotals) != 1 {
			t.Fatalf("viewer %q cannot see public pending total", viewer)
		}
	}

	battle.DamageResolution.CardProposals[0].Revealed = true
	for _, viewer := range []string{"player", "enemy"} {
		view := snapshot.FromBattleForViewer(battle, viewer)
		if len(view.Damage.RevealedCards) != 1 ||
			view.Damage.RevealedCards[0].CardID != "p1" {
			t.Fatalf("viewer %q post-reveal damage snapshot = %#v", viewer, view.Damage)
		}
	}
	revealedEvent := event.NewDamageCardsRevealed(
		"player",
		battle.DamageResolution.CardProposals,
	)
	for _, viewer := range []string{"player", "enemy"} {
		filtered := event.ForViewer([]event.Event{revealedEvent}, viewer)
		if filtered[0].DamageCards[0].CardID != "p1" {
			t.Fatalf("viewer %q did not receive public revealed card", viewer)
		}
	}
}

func damageFlowBattle(t *testing.T) state.Battle {
	t.Helper()
	battle, err := state.NewBattleFromSetup("damage-flow", state.BattleSetup{
		Actors: []state.ActorSetup{
			{
				ID:             "player",
				ControllerType: state.ControllerHuman,
				Health:         state.HealthMetadata{Model: "card_as_health"},
				Deck:           []string{"p1", "p2", "p3", "p4"},
			},
			{
				ID:             "enemy",
				ControllerType: state.ControllerAI,
				Health:         state.HealthMetadata{Model: "card_as_health"},
				Deck:           []string{"e1", "e2"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	amount := 2
	battle.OffensiveProposals = []state.PlanningProposal{{
		ID:      "attack",
		ActorID: "enemy",
		Operations: []state.FinalizedOperationProposal{{
			ID:              "attack-damage",
			ContentType:     "ability",
			ContentID:       "generic_attack",
			SourceActorID:   "enemy",
			SelectedTargets: []string{"player"},
			Operation: operation.Plan{
				ID:     "deal-two",
				Type:   operation.TypeDealDamage,
				Target: operation.TargetSelectedTargets,
				Amount: &amount,
			},
		}},
	}}
	battle.Segment = segment.State{Current: segment.DamageResolution, Round: 1}
	battle.Flow = state.NewSegmentFlowState(battle.Segment)
	return battle
}

type waitAfterDamageFlow struct{}

func (*waitAfterDamageFlow) ID() segment.Segment {
	return segment.OngoingEffects
}

func (*waitAfterDamageFlow) OnEnter(ctx *engine.Context) ([]event.Event, error) {
	ctx.Battle.Flow.Stage = "post_damage_wait"
	ctx.Battle.Flow.Iteration = 1
	ctx.Battle.Flow.Actors["player"] = state.ActorFlowState{Status: state.ActorNeedsInput}
	ctx.Battle.Flow.Actors["enemy"] = state.ActorFlowState{Status: state.ActorResolved}
	ctx.Battle.Flow.PendingInput["player"] = state.PendingInput{
		ID:              "post-damage-input",
		ActorID:         "player",
		Segment:         segment.OngoingEffects,
		Phase:           state.FlowPhaseInProgress,
		Stage:           "post_damage_wait",
		Iteration:       1,
		InputType:       "test_wait",
		AllowedCommands: []command.Type{command.TypePass},
	}
	return nil, nil
}

func (*waitAfterDamageFlow) Progress(*engine.Context) (engine.ProgressResult, error) {
	return engine.ProgressResult{Status: engine.ProgressWaitingForInput}, nil
}

func (*waitAfterDamageFlow) HandleCommand(*engine.Context, command.Command) ([]event.Event, error) {
	return nil, nil
}

func (*waitAfterDamageFlow) OnExit(*engine.Context) ([]event.Event, error) {
	return nil, nil
}

func applyInteraction(
	t *testing.T,
	eng engine.Engine,
	battle *state.Battle,
	commandType command.Type,
	payload any,
) (engine.ProgressionResult, error) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	return eng.ApplyBattleCommand(battle, command.Command{
		BattleID: battle.ID,
		ActorID:  "player",
		Type:     commandType,
		Payload:  encoded,
	})
}

func interactionCheckpoint(pending state.PendingInput) command.InteractionCheckpoint {
	return command.InteractionCheckpoint{
		WindowID:      pending.WindowID,
		Stage:         pending.Stage,
		Iteration:     pending.Iteration,
		ReactionRound: pending.ReactionRound,
	}
}

func countEvents(events []event.Event, eventType event.Type) int {
	count := 0
	for _, candidate := range events {
		if candidate.Type == eventType {
			count++
		}
	}
	return count
}

func firstEvent(events []event.Event, eventType event.Type) event.Event {
	for _, candidate := range events {
		if candidate.Type == eventType {
			return candidate
		}
	}
	return event.Event{}
}

func activeDamageCards(battle state.Battle) int {
	count := 0
	for _, card := range battle.DamageResolution.CardProposals {
		if card.Accepted {
			count++
		}
	}
	return count
}
