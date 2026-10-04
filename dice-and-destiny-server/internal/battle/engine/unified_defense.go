package engine

import (
	"encoding/json"
	"fmt"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/damage"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func unifiedDefense(b *state.Battle) bool {
	return b.Settled != nil && b.Settled.UnifiedDefense && b.Segment.Current == segment.Defensive && b.Settled.PendingDamage != nil && !b.Settled.PendingDamage.Committed && (b.Settled.Venom == nil || b.Settled.Venom.Active == nil) && (b.Settled.Curse == nil || b.Settled.Curse.Active == nil)
}

// The hub remains open between individual defenses. Passing is final for that
// participant; it never consumes another participant's choices.
func (e Engine) unifiedDefenseHub(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	closeSettledWindow(b)
	b.Settled.DefenseSelections = map[string]state.SettledDefense{}
	b.Settled.Stage = stageDefenseSelect
	b.Flow.Stage = stageDefenseSelect
	for _, actor := range externalSettledActorIDs(b) {
		if b.Settled.DefensePassed[actor] {
			continue
		}
		openSettledWindowForActors(b, "unified-defense", stageDefenseSelect, "reaction", []command.Type{command.TypePlanningAbility, command.TypePlanningPass, command.TypeCommitInteraction}, []string{actor}, false)
		return nil, nil
	}
	if err := e.selectAIDefenses(b, lib); err != nil {
		return nil, err
	}
	if len(b.Settled.DefenseSelections) > 0 {
		return e.resolveDefenseRollsAndOpenReaction(b, lib)
	}
	if err := e.autoAIDamageResponse(b, lib, b.Settled.PendingDamage); err != nil {
		return nil, err
	}
	return e.finishDamageBatch(b, lib)
}

func (e Engine) handleUnifiedCard(b *state.Battle, lib content.BattleLibrary, cmd command.Command) ([]event.Event, error) {
	var payload command.CommitInteractionPayload
	if err := command.DecodePayload(cmd, &payload); err != nil {
		return nil, err
	}
	if payload.Commitment.ChoiceID == "spend_round_prevention" {
		return e.spendRoundPrevention(b, lib, cmd.ActorID, payload.Commitment)
	}
	cardID := first(payload.Commitment.CardIDs)
	definition := settledCardDefinitionID(b, cmd.ActorID, cardID)
	sourceID := first(payload.Commitment.ProposalIDs)
	before := 0
	if source := batchSourceByID(b.Settled.PendingDamage, sourceID); source != nil {
		before = settledSourceAmount(*source)
	}
	if err := e.playSettledReactionCard(b, lib, cmd.ActorID, payload.Commitment); err != nil {
		return nil, err
	}
	data := map[string]any{"card_instance_id": cardID, "card_definition_id": definition, "choice_id": payload.Commitment.ChoiceID}
	kind := event.TypeCardPlayed
	if source := batchSourceByID(b.Settled.PendingDamage, sourceID); source != nil {
		kind = event.TypeDamageModified
		data["source_id"], data["damage_before"], data["damage_after"], data["target_actor_id"] = sourceID, before, settledSourceAmount(*source), source.TargetActorID
	}
	return []event.Event{settledEvent(kind, b, cmd.ActorID, data)}, nil
}

func (e Engine) passUnifiedDefense(b *state.Battle, lib content.BattleLibrary, actor string) ([]event.Event, error) {
	b.Settled.DefensePassed[actor] = true
	skipRemainingDefenses(b, actor)
	return e.unifiedDefenseHub(b, lib)
}

// Record ordered arithmetic while retaining authored base/prevention metadata.
func setUnifiedSourceAmount(b *state.Battle, source *state.SettledDamageSource, amount int) {
	if !unifiedDefense(b) {
		return
	}
	source.ResolutionAdjustment = 0
	source.ResolutionAdjustment = max(0, amount) - settledSourceAmount(*source)
}

func currentRemovalZone(b *state.Battle, r state.ProposedCardRemoval) operation.CardZone {
	for _, zone := range damage.DefaultSelectionOrder() {
		if containsString(zoneCards(b.Actors[r.TargetActorID].Cards, zone), r.CardID) {
			return zone
		}
	}
	return r.OriginalZone
}

// Proposals reserve instances without moving them. Each accepted instance belongs
// to exactly one source, so drawing or playing it cannot evade its pending damage.
func (e Engine) fillUnifiedReservations(b *state.Battle, batch *state.SettledDamageBatch) error {
	reserved := map[string]bool{}
	for _, r := range batch.Removals {
		if r.Accepted && !r.Released {
			reserved[r.CardID] = true
		}
	}
	batch.Overage = map[string]int{}
	for _, source := range batch.Sources {
		count := 0
		for _, r := range batch.Removals {
			if r.Accepted && !r.Released && containsString(r.DamageProposalIDs, source.ID) {
				count++
			}
		}
		need := source.FinalAmount - count
		for _, zone := range damage.DefaultSelectionOrder() {
			var available []string
			for _, id := range zoneCards(b.Actors[source.TargetActorID].Cards, zone) {
				protected := false
				for _, r := range batch.Removals {
					if r.ProtectedFromSource && r.CardID == id && containsString(r.DamageProposalIDs, source.ID) {
						protected = true
						break
					}
				}
				if !reserved[id] && !protected {
					available = append(available, id)
				}
			}
			for need > 0 && len(available) > 0 {
				index, err := e.namedIntn(b, "damage_selection", len(available))
				if err != nil {
					return err
				}
				id := available[index]
				available = append(available[:index], available[index+1:]...)
				reserved[id] = true
				batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: fmt.Sprintf("%s-removal-%d", batch.ID, len(batch.Removals)+1), TargetActorID: source.TargetActorID, CardID: id, CardDefinitionID: b.Settled.Actors[source.TargetActorID].CardInstances[id].DefinitionID, OriginalZone: zone, DamageProposalIDs: []string{source.ID}, SourceActorIDs: []string{source.SourceActorID}, Sequence: len(batch.Removals) + 1, Revealed: true, Accepted: true})
				need--
			}
		}
		batch.Overage[source.TargetActorID] += max(0, need)
	}
	return nil
}

func (e Engine) reconcileUnifiedDamage(b *state.Battle, restoreOriginal bool) error {
	batch := b.Settled.PendingDamage
	if batch == nil {
		return nil
	}
	for i := range batch.Sources {
		source := &batch.Sources[i]
		source.FinalAmount = settledSourceAmount(*source)
		count := 0
		for _, r := range batch.Removals {
			if r.Accepted && !r.Released && containsString(r.DamageProposalIDs, source.ID) {
				count++
			}
		}
		// Defense and card prevention protect live hand first, then draw, then
		// discard. Explicit status effects may send saved instances to discard.
		for _, zone := range []operation.CardZone{operation.ZoneHand, operation.ZoneDeck, operation.ZoneDiscard} {
			var candidates []int
			for j, r := range batch.Removals {
				if r.Accepted && !r.Released && containsString(r.DamageProposalIDs, source.ID) && currentRemovalZone(b, r) == zone {
					candidates = append(candidates, j)
				}
			}
			for count > source.FinalAmount && len(candidates) > 0 {
				index, err := e.namedIntn(b, "damage_prevention", len(candidates))
				if err != nil {
					return err
				}
				r := &batch.Removals[candidates[index]]
				candidates = append(candidates[:index], candidates[index+1:]...)
				if restoreOriginal {
					damage.RetainPreventedCard(b, r)
				} else {
					damage.DiscardPreventedCard(b, r)
				}
				r.Accepted, r.Released = false, true
				count--
			}
		}
		if original := sourceBySettledID(b, source.ID); original != nil {
			*original = *source
		}
	}
	// Only previously unfilled excess damage may claim newly available cards.
	// Existing reservations are never reselected or transferred between attacks.
	return e.fillUnifiedReservations(b, batch)
}

func unifiedContinueCommand(b *state.Battle, actor string, pending state.PendingInput) command.Command {
	payload, _ := json.Marshal(command.PlanningPassPayload{PendingInputID: pending.ID, Checkpoint: planningCheckpoint(pending)})
	return command.Command{BattleID: b.ID, ActorID: actor, Type: command.TypePlanningPass, Payload: payload}
}

// All card and defensive-ability definitions share this destination setting.
// Omitted values preserve the live pile; explicitly authored discard moves only
// newly saved cards, never a prior effect's already released reservations.
func (e Engine) reconcilePreventionDestination(b *state.Battle, destination string) error {
	if destination != "" && destination != content.SavedCardsOriginal && destination != content.SavedCardsDiscard {
		return fmt.Errorf("invalid saved_card_destination %q", destination)
	}
	if unifiedDefense(b) {
		return e.reconcileUnifiedDamage(b, destination != content.SavedCardsDiscard)
	}
	reconcileSettledDamageDestination(b.Settled.PendingDamage, b, destination == content.SavedCardsDiscard)
	return nil
}
