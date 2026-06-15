package damage_test

import (
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/state"
)

func TestCollectAccumulatesSourcesAppliesDefenseAndKeepsCounterDamageIndependent(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"player": {Deck: []string{"p1", "p2", "p3", "p4"}},
		"enemy":  {Deck: []string{"e1", "e2", "e3", "e4"}},
	})
	battle.OffensiveProposals = []state.PlanningProposal{
		planningDamage("attack-1", "player", "enemy", "strike", 2),
		planningDamage("attack-2", "player", "enemy", "blast", 2),
	}
	battle.DefensiveProposals = []state.PlanningProposal{
		planningPrevention("guard-1", "enemy", "attack-1", 1),
		planningDamage("counter-1", "enemy", "player", "counter", 1),
	}
	resolution := &state.DamageResolutionState{}

	if err := damage.Collect(
		&battle,
		operation.DefaultRuntimeRegistry(),
		resolution,
	); err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}

	if len(resolution.SourceProposals) != 3 {
		t.Fatalf("source proposals = %#v, want three", resolution.SourceProposals)
	}
	if got := totalFor(t, resolution, "enemy").FinalAmount; got != 3 {
		t.Fatalf("enemy preview damage = %d, want 3", got)
	}
	if got := totalFor(t, resolution, "player").FinalAmount; got != 1 {
		t.Fatalf("counter preview damage = %d, want 1", got)
	}
	if resolution.SourceProposals[0].SourceActorID != "player" ||
		resolution.SourceProposals[0].SourceContentID != "strike" ||
		resolution.SourceProposals[0].OriginatingOperation.Operation.Type != operation.TypeDealDamage {
		t.Fatalf("source attribution was not preserved: %#v", resolution.SourceProposals[0])
	}
}

func TestCardSelectionUsesCombinedDeckDiscardThenHandWithoutMutation(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"target": {
			Deck:    []string{"deck-a", "deck-b"},
			Discard: []string{"discard-a", "discard-b"},
			Hand:    []string{"hand-a", "hand-b"},
		},
	})
	before := battle.Clone()
	resolution := &state.DamageResolutionState{
		SourceProposals: []state.DamageSourceProposal{
			{ID: "source", SourceActorID: "attacker", TargetActorID: "target", BaseAmount: 5},
		},
	}
	damage.Recalculate(resolution)

	random := &recordingRandom{values: []int{2, 0, 1, 0, 1}}
	selected, err := damage.ReconcileCards(&battle, resolution, random)
	if err != nil {
		t.Fatalf("ReconcileCards() returned error: %v", err)
	}
	if !reflect.DeepEqual(battle.Actors["target"].Cards, before.Actors["target"].Cards) {
		t.Fatal("damage selection mutated card zones")
	}
	if len(selected) != 5 {
		t.Fatalf("selected cards = %#v, want five", selected)
	}
	primary := 0
	hand := 0
	for _, proposal := range selected {
		switch proposal.OriginalZone {
		case operation.ZoneDeck, operation.ZoneDiscard:
			primary++
		case operation.ZoneHand:
			hand++
		}
	}
	if primary != 4 || hand != 1 {
		t.Fatalf("selected zones = %#v, want all four draw/discard before one hand", selected)
	}
	if selected[0].CardID != "discard-a" {
		t.Fatalf("first combined-population selection = %#v, want discard-a from index 2", selected[0])
	}
	if len(random.maximums) == 0 || random.maximums[0] != 4 {
		t.Fatalf("first random population size = %#v, want combined draw+discard size 4", random.maximums)
	}
	if !reflect.DeepEqual(selected[0].DamageProposalIDs, []string{"source"}) ||
		!reflect.DeepEqual(selected[0].SourceActorIDs, []string{"attacker"}) {
		t.Fatalf("damage-card source attribution = %#v", selected[0])
	}
}

func TestDrawAndDiscardSelectionIsUniformPerCardNotPerZone(t *testing.T) {
	zones := state.CardZones{
		Deck:    []string{"deck-only"},
		Discard: []string{"discard-one", "discard-two", "discard-three"},
	}
	wantByIndex := []struct {
		cardID string
		zone   operation.CardZone
	}{
		{cardID: "deck-only", zone: operation.ZoneDeck},
		{cardID: "discard-one", zone: operation.ZoneDiscard},
		{cardID: "discard-two", zone: operation.ZoneDiscard},
		{cardID: "discard-three", zone: operation.ZoneDiscard},
	}

	for index, want := range wantByIndex {
		t.Run(want.cardID, func(t *testing.T) {
			battle := testBattle(t, map[string]state.CardZones{"target": zones})
			resolution := &state.DamageResolutionState{
				SourceProposals: []state.DamageSourceProposal{
					{ID: "source", SourceActorID: "attacker", TargetActorID: "target", BaseAmount: 1},
				},
			}
			damage.Recalculate(resolution)
			random := &recordingRandom{values: []int{index}}

			selected, err := damage.ReconcileCards(&battle, resolution, random)
			if err != nil {
				t.Fatalf("ReconcileCards() returned error: %v", err)
			}
			if !reflect.DeepEqual(random.maximums, []int{4}) {
				t.Fatalf("random population sizes = %#v, want one four-card population", random.maximums)
			}
			if len(selected) != 1 ||
				selected[0].CardID != want.cardID ||
				selected[0].OriginalZone != want.zone {
				t.Fatalf("selection at combined index %d = %#v, want %s from %s", index, selected, want.cardID, want.zone)
			}
		})
	}
}

func TestReactionsPreventSourcesReplaceCardsReleaseExcessAndSelectIncreases(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"target": {
			Deck:    []string{"a", "b", "c", "d"},
			Discard: []string{"e"},
			Hand:    []string{"f"},
		},
	})
	resolution := &state.DamageResolutionState{
		SourceProposals: []state.DamageSourceProposal{
			{ID: "source-1", SourceActorID: "one", TargetActorID: "target", BaseAmount: 2},
			{ID: "source-2", SourceActorID: "two", TargetActorID: "target", BaseAmount: 1},
		},
	}
	damage.Recalculate(resolution)
	if _, err := damage.ReconcileCards(&battle, resolution, dice.NewSequenceRandomSource(0)); err != nil {
		t.Fatalf("initial ReconcileCards() returned error: %v", err)
	}
	damage.RevealCards(resolution)
	firstCardID := resolution.CardProposals[0].ID

	if err := damage.ApplyReactions(&battle, resolution, []state.DamageReaction{
		{Type: state.DamageReactionPreventSource, ProposalID: "source-1", Amount: 1},
		{Type: state.DamageReactionReplaceCard, ProposalID: firstCardID, ReplacementCardID: "f"},
	}); err != nil {
		t.Fatalf("ApplyReactions() returned error: %v", err)
	}
	if _, err := damage.ReconcileCards(&battle, resolution, dice.NewSequenceRandomSource(0)); err != nil {
		t.Fatalf("reduced ReconcileCards() returned error: %v", err)
	}
	if got := activeCardCount(resolution); got != 2 {
		t.Fatalf("active cards after reduction = %d, want 2", got)
	}
	if !resolution.CardProposals[0].Released {
		t.Fatal("replaced card proposal was not released")
	}

	if err := damage.ApplyReactions(&battle, resolution, []state.DamageReaction{
		{Type: state.DamageReactionModifySource, ProposalID: "source-2", Amount: 2},
	}); err != nil {
		t.Fatalf("increase ApplyReactions() returned error: %v", err)
	}
	added, err := damage.ReconcileCards(&battle, resolution, dice.NewSequenceRandomSource(0))
	if err != nil {
		t.Fatalf("increased ReconcileCards() returned error: %v", err)
	}
	revealed := damage.RevealCards(resolution)
	if len(added) != 2 || len(revealed) < 2 || activeCardCount(resolution) != 4 {
		t.Fatalf("increase selected/revealed = %d/%d active=%d, want 2/at least2/4", len(added), len(revealed), activeCardCount(resolution))
	}
}

func TestCommitUsesPreviewPathMovesOnceAndFailsAtomically(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"target": {Deck: []string{"a", "b"}, Discard: []string{"c"}},
	})
	resolution := &state.DamageResolutionState{
		SourceProposals: []state.DamageSourceProposal{
			{ID: "source", SourceActorID: "attacker", TargetActorID: "target", BaseAmount: 2},
		},
	}
	damage.Recalculate(resolution)
	if _, err := damage.ReconcileCards(&battle, resolution, dice.NewSequenceRandomSource(0)); err != nil {
		t.Fatalf("ReconcileCards() returned error: %v", err)
	}
	preview := totalFor(t, resolution, "target").FinalAmount
	result, err := damage.Commit(&battle, resolution)
	if err != nil {
		t.Fatalf("Commit() returned error: %v", err)
	}
	if preview != 2 || result.Totals[0].FinalAmount != preview ||
		len(battle.Actors["target"].Cards.Removed) != preview {
		t.Fatalf("preview/commit mismatch: preview=%d result=%#v actor=%#v", preview, result, battle.Actors["target"])
	}
	afterFirstCommit := battle.Clone()
	if _, err := damage.Commit(&battle, resolution); err == nil {
		t.Fatal("second Commit() permanently removed the same proposed cards again")
	}
	if !reflect.DeepEqual(battle, afterFirstCommit) {
		t.Fatal("rejected duplicate commit mutated battle state")
	}

	invalidBattle := testBattle(t, map[string]state.CardZones{
		"target": {Deck: []string{"a", "b"}},
	})
	invalidResolution := &state.DamageResolutionState{
		CardProposals: []state.ProposedCardRemoval{
			{ID: "valid", TargetActorID: "target", CardID: "a", OriginalZone: operation.ZoneDeck, Accepted: true},
			{ID: "invalid", TargetActorID: "target", CardID: "missing", OriginalZone: operation.ZoneDeck, Accepted: true},
		},
	}
	before := invalidBattle.Clone()
	if _, err := damage.Commit(&invalidBattle, invalidResolution); err == nil {
		t.Fatal("Commit() accepted a missing proposed card")
	}
	if !reflect.DeepEqual(invalidBattle, before) {
		t.Fatal("failed commit partially mutated battle state")
	}
}

func TestCommitCanDefeatMultipleActorsWithoutEndingBattle(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"one": {Deck: []string{"one-card"}},
		"two": {Hand: []string{"two-card"}},
	})
	resolution := &state.DamageResolutionState{
		SourceProposals: []state.DamageSourceProposal{
			{ID: "to-one", SourceActorID: "two", TargetActorID: "one", BaseAmount: 1},
			{ID: "to-two", SourceActorID: "one", TargetActorID: "two", BaseAmount: 1},
		},
	}
	damage.Recalculate(resolution)
	if _, err := damage.ReconcileCards(&battle, resolution, dice.NewSequenceRandomSource(0)); err != nil {
		t.Fatalf("ReconcileCards() returned error: %v", err)
	}
	if _, err := damage.Commit(&battle, resolution); err != nil {
		t.Fatalf("Commit() returned error: %v", err)
	}
	for _, actorID := range []string{"one", "two"} {
		actor := battle.Actors[actorID]
		if actor.CurrentHealth() != 0 || actor.DefeatState != state.ActorPendingDefeat {
			t.Fatalf("actor %q state = %#v, want zero pending defeat", actorID, actor)
		}
	}
	if battle.Status != state.BattleActive {
		t.Fatalf("battle status = %q, want active", battle.Status)
	}
}

func TestDamageBeyondRemainingHealthSelectsEveryCardOnceAndQueuesStatusWork(t *testing.T) {
	battle := testBattle(t, map[string]state.CardZones{
		"target": {
			Deck:    []string{"same", "same"},
			Discard: []string{"discard"},
			Hand:    []string{"hand"},
		},
	})
	damageAmount := 10
	stackCount := 1
	battle.OffensiveProposals = []state.PlanningProposal{{
		ID:      "attack",
		ActorID: "attacker",
		Operations: []state.FinalizedOperationProposal{
			{
				ID:              "damage",
				ContentType:     "ability",
				ContentID:       "attack",
				SourceActorID:   "attacker",
				SelectedTargets: []string{"target"},
				Operation: operation.Plan{
					ID:     "damage-operation",
					Type:   operation.TypeDealDamage,
					Target: operation.TargetSelectedTargets,
					Amount: &damageAmount,
				},
			},
			{
				ID:              "status",
				ContentType:     "ability",
				ContentID:       "attack",
				SourceActorID:   "attacker",
				SelectedTargets: []string{"target"},
				Operation: operation.Plan{
					ID:         "status-operation",
					Type:       operation.TypeApplyStatus,
					Target:     operation.TargetSelectedTargets,
					StatusID:   "poison",
					StackCount: &stackCount,
				},
			},
		},
	}}
	resolution := &state.DamageResolutionState{}
	if err := damage.Collect(&battle, operation.DefaultRuntimeRegistry(), resolution); err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}
	selected, err := damage.ReconcileCards(
		&battle,
		resolution,
		dice.NewSequenceRandomSource(0),
	)
	if err != nil {
		t.Fatalf("ReconcileCards() returned error: %v", err)
	}
	if len(selected) != 4 || len(resolution.PendingOperations) != 1 {
		t.Fatalf("selected=%#v pending=%#v, want four cards and queued status", selected, resolution.PendingOperations)
	}
	result, err := damage.Commit(&battle, resolution)
	if err != nil {
		t.Fatalf("Commit() returned error: %v", err)
	}
	if len(result.Cards) != 4 || len(battle.PendingOperations) != 1 ||
		battle.PendingOperations[0].Operation.Type != operation.TypeApplyStatus {
		t.Fatalf("commit result=%#v pending=%#v", result, battle.PendingOperations)
	}
	if battle.Actors["target"].DefeatState != state.ActorPendingDefeat ||
		battle.Status != state.BattleActive {
		t.Fatalf("zero-health queued consequence state = actor %#v battle %q", battle.Actors["target"], battle.Status)
	}
}

func testBattle(t *testing.T, zones map[string]state.CardZones) state.Battle {
	t.Helper()
	setups := make([]state.ActorSetup, 0, len(zones)+1)
	for actorID, cards := range zones {
		setups = append(setups, state.ActorSetup{
			ID:             actorID,
			ControllerType: state.ControllerHuman,
			Health:         state.HealthMetadata{Model: "card_as_health"},
			Deck:           cards.Deck,
			Hand:           cards.Hand,
			Discard:        cards.Discard,
			Removed:        cards.Removed,
		})
	}
	if _, exists := zones["attacker"]; !exists {
		setups = append(setups, state.ActorSetup{
			ID:             "attacker",
			ControllerType: state.ControllerAI,
			Deck:           []string{"health"},
		})
	}
	battle, err := state.NewBattleFromSetup("damage-test", state.BattleSetup{Actors: setups})
	if err != nil {
		t.Fatalf("NewBattleFromSetup() returned error: %v", err)
	}
	return battle
}

func planningDamage(id, sourceActorID, targetActorID, contentID string, amount int) state.PlanningProposal {
	return state.PlanningProposal{
		ID:      id,
		ActorID: sourceActorID,
		Operations: []state.FinalizedOperationProposal{{
			ID:              id + "-damage",
			ContentType:     "ability",
			ContentID:       contentID,
			SourceActorID:   sourceActorID,
			SelectedTargets: []string{targetActorID},
			Operation: operation.Plan{
				ID:     id + "-operation",
				Type:   operation.TypeDealDamage,
				Target: operation.TargetSelectedTargets,
				Amount: intPointer(amount),
			},
		}},
	}
}

func planningPrevention(id, sourceActorID, targetProposalID string, amount int) state.PlanningProposal {
	return state.PlanningProposal{
		ID:      id,
		ActorID: sourceActorID,
		Operations: []state.FinalizedOperationProposal{{
			ID:              id + "-prevent",
			ContentType:     "ability",
			ContentID:       "guard",
			SourceActorID:   sourceActorID,
			SelectedTargets: []string{targetProposalID},
			Operation: operation.Plan{
				ID:     id + "-operation",
				Type:   operation.TypePreventDamage,
				Target: operation.TargetSelectedProposal,
				Amount: intPointer(amount),
			},
		}},
	}
}

func totalFor(
	t *testing.T,
	resolution *state.DamageResolutionState,
	actorID string,
) state.AccumulatedDamageProposal {
	t.Helper()
	for _, total := range resolution.AccumulatedProposals {
		if total.TargetActorID == actorID {
			return total
		}
	}
	t.Fatalf("total for actor %q was not found", actorID)
	return state.AccumulatedDamageProposal{}
}

func activeCardCount(resolution *state.DamageResolutionState) int {
	count := 0
	for _, proposal := range resolution.CardProposals {
		if proposal.Accepted {
			count++
		}
	}
	return count
}

func intPointer(value int) *int {
	return &value
}

type recordingRandom struct {
	values   []int
	next     int
	maximums []int
}

func (random *recordingRandom) Intn(maxExclusive int) (int, error) {
	random.maximums = append(random.maximums, maxExclusive)
	value := random.values[random.next%len(random.values)]
	random.next++
	return value % maxExclusive, nil
}
