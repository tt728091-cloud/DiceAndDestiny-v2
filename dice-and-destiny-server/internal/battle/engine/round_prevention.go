package engine

import (
	"errors"
	"strconv"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

func adjacentDiceCard(card content.BattleCardDefinition) bool {
	for _, op := range card.Operations {
		if op.Type == "modify_die" && op.Modification == "adjacent_non_six" {
			return true
		}
	}
	return false
}

func validAdjacentFace(before int, choice string) bool {
	face, err := strconv.Atoi(choice)
	return err == nil && face >= 1 && face <= 5 && (face == before-1 || face == before+1)
}

func roundPreventionEligible(b *state.Battle, lib content.BattleLibrary, actor, sourceID string) bool {
	// Protect authored as automatic prevention is never spent by hand.
	if m := lib.Statuses["protect"].Modifiers; m != nil && m.AutoPrevention > 0 {
		return false
	}
	source := batchSourceByID(b.Settled.PendingDamage, sourceID)
	return (b.Settled.Stage == stageDamageReact || (unifiedDefense(b) && b.Settled.Stage == stageDefenseSelect)) && stacks(b, actor, "protect") > 0 && !protectQueued(b, actor) &&
		source != nil && source.TargetActorID == actor && lib.Abilities[source.SourceContentID].Type == "offensive" && settledSourceAmount(*source) > 0
}

func roundPreventionActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	if (b.Settled.Stage != stageDamageReact && !(unifiedDefense(b) && b.Settled.Stage == stageDefenseSelect)) || b.Settled.PendingDamage == nil {
		return nil
	}
	var actions []command.Command
	for _, source := range b.Settled.PendingDamage.Sources {
		if roundPreventionEligible(b, lib, actor, source.ID) {
			actions = append(actions, legalCommand(b.ID, actor, command.TypeCommitInteraction, command.CommitInteractionPayload{
				PendingInputID: pending.ID, Checkpoint: interactionCheckpoint(pending),
				Commitment: command.InteractionCommitmentData{ChoiceID: "spend_round_prevention", ProposalIDs: []string{source.ID}},
			}))
		}
	}
	return actions
}

func (e Engine) spendRoundPrevention(b *state.Battle, lib content.BattleLibrary, actor string, choice command.InteractionCommitmentData) ([]event.Event, error) {
	if len(choice.CardIDs) != 0 || len(choice.ProposalIDs) != 1 || !roundPreventionEligible(b, lib, actor, first(choice.ProposalIDs)) {
		return nil, errors.New("round protection requires an unspent grant and one incoming attack this round")
	}
	source := batchSourceByID(b.Settled.PendingDamage, choice.ProposalIDs[0])
	before := settledSourceAmount(*source)
	amount := stacks(b, actor, "protect")
	source.ReactionPrevention += amount
	setUnifiedSourceAmount(b, source, max(0, before-amount))
	removeStatus(b, actor, "protect", 0)
	if unifiedDefense(b) {
		if err := e.reconcileUnifiedDamage(b, false); err != nil {
			return nil, err
		}
	} else {
		reconcileSettledDamageDestination(b.Settled.PendingDamage, b, true)
	}
	if !unifiedDefense(b) {
		advanceSettledReactionPriority(b, actor, true)
	}
	return []event.Event{settledEvent(event.TypeDamageModified, b, actor, map[string]any{
		"source_id": source.ID, "status_id": "protect", "status_before": amount, "status_after": 0, "prevention": source.ReactionPrevention,
		"damage_before": before, "damage_after": settledSourceAmount(*source), "target_actor_id": actor,
	})}, nil
}

// applyAutoPrevention lets statuses authored with auto_prevention (Protect)
// prevent damage on their own as incoming attacks are committed. Each point is
// drawn at random from the holder's remaining attack damage, so two attacks of
// 1 can both be stopped while one larger attack loses up to the full amount.
// The stacks used are spent; the saved cards go to the authored destination.
func (e Engine) applyAutoPrevention(b *state.Battle, lib content.BattleLibrary) ([]event.Event, error) {
	batch := b.Settled.PendingDamage
	if batch == nil {
		return nil, nil
	}
	var events []event.Event
	for _, actor := range sortedSettledActorIDs(b) {
		for _, status := range append([]state.StatusState(nil), b.Actors[actor].Statuses...) {
			m := lib.Statuses[status.DefinitionID].Modifiers
			if m == nil || m.AutoPrevention <= 0 || status.Stacks <= 0 {
				continue
			}
			var points []string
			for _, s := range batch.Sources {
				if s.TargetActorID == actor && lib.Abilities[s.SourceContentID].Type == "offensive" {
					for n := settledSourceAmount(s); n > 0; n-- {
						points = append(points, s.ID)
					}
				}
			}
			prevented, used := map[string]int{}, 0
			for budget := status.Stacks * m.AutoPrevention; used < budget && len(points) > 0; used++ {
				i, err := e.namedIntn(b, "auto_prevention", len(points))
				if err != nil {
					return nil, err
				}
				prevented[points[i]]++
				points = append(points[:i], points[i+1:]...)
			}
			if used == 0 {
				continue
			}
			for i := range batch.Sources {
				source := &batch.Sources[i]
				n := prevented[source.ID]
				if n == 0 {
					continue
				}
				before := settledSourceAmount(*source)
				source.ReactionPrevention += n
				setUnifiedSourceAmount(b, source, max(0, before-n))
				events = append(events, settledEvent(event.TypeDamageModified, b, actor, map[string]any{
					"source_id": source.ID, "status_id": status.DefinitionID, "automatic": true, "prevention": source.ReactionPrevention,
					"damage_before": before, "damage_after": settledSourceAmount(*source), "target_actor_id": actor,
				}))
			}
			removeStatus(b, actor, status.DefinitionID, (used+m.AutoPrevention-1)/m.AutoPrevention)
			original := m.AutoPreventionDestination == "original"
			if unifiedDefense(b) {
				if err := e.reconcileUnifiedDamage(b, original); err != nil {
					return nil, err
				}
			} else {
				reconcileSettledDamageDestination(batch, b, !original)
			}
		}
	}
	return events, nil
}
