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
	source := batchSourceByID(b.Settled.PendingDamage, sourceID)
	return b.Settled.Stage == stageDamageReact && stacks(b, actor, "protect") > 0 &&
		source != nil && source.TargetActorID == actor && lib.Abilities[source.SourceContentID].Type == "offensive" && settledSourceAmount(*source) > 0
}

func roundPreventionActions(b *state.Battle, lib content.BattleLibrary, actor string, pending state.PendingInput) []command.Command {
	if b.Settled.Stage != stageDamageReact || b.Settled.PendingDamage == nil {
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
	removeStatus(b, actor, "protect", 0)
	reconcileSettledDamage(b.Settled.PendingDamage, b)
	advanceSettledReactionPriority(b, actor, true)
	return []event.Event{settledEvent(event.TypeDamageModified, b, actor, map[string]any{
		"source_id": source.ID, "ability_id": "guarded_strike", "status_id": "protect", "status_before": amount, "status_after": 0, "prevention": source.ReactionPrevention,
		"damage_before": before, "damage_after": settledSourceAmount(*source), "target_actor_id": actor,
	})}, nil
}
