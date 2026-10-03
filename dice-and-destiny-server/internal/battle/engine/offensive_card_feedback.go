package engine

import (
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Publish the actual contribution of each card at the joint reveal, when the
// opponent's planning is public. Re-evaluate against the current dice so misses,
// expired modifiers and later reaction changes cannot advertise a false bonus.
func offensiveCardDamageBonuses(b *state.Battle, lib content.BattleLibrary, actorID string) []map[string]any {
	runtime := b.Settled.Actors[actorID]
	if len(runtime.AbilityModifiers) == 0 {
		return nil
	}
	preview := b.Clone()
	r := preview.Settled.Actors[actorID]
	r.AbilityModifiers = nil
	preview.Settled.Actors[actorID] = r
	damage := func() int {
		ops, _ := resolvedOffensiveOperations(&preview, lib, actorID)
		return summarizeOffensiveOutcome(ops, r.SelectedTargetIDs)["base_damage"].(int)
	}
	before := damage()
	var bonuses []map[string]any
	for _, modifier := range runtime.AbilityModifiers {
		r.AbilityModifiers = append(r.AbilityModifiers, modifier)
		preview.Settled.Actors[actorID] = r
		after := damage()
		card := runtime.CardInstances[modifier.SourceCardInstanceID]
		if after > before && card.DefinitionID != "" {
			bonuses = append(bonuses, map[string]any{
				"card_instance_id": modifier.SourceCardInstanceID, "card_definition_id": card.DefinitionID,
				"ability_id": r.SelectedAbilityID, "before": before, "after": after, "amount": after - before,
			})
		}
		before = after
	}
	return bonuses
}
