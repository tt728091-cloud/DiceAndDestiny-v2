package engine

import (
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// DefensePreview is a choice to carry across the offensive reaction boundary,
// not a legal command. The client must revalidate it against defense_selection.
type DefensePreview struct {
	SourceID        string `json:"source_id,omitempty"`
	SourceActorID   string `json:"source_actor_id"`
	SourceContentID string `json:"source_content_id"`
	TargetActorID   string `json:"target_actor_id"`
	AbilityID       string `json:"ability_id"`
	SpendCatalyst   bool   `json:"spend_catalyst,omitempty"`
}

func defenseAffordable(b *state.Battle, library content.BattleLibrary, actorID, abilityID, sourceContentID string) bool {
	ability := library.Abilities[abilityID]
	return b.Actors[actorID].Resources.EnergyPoints >= ability.Cost.Energy &&
		(!abilityOnlyDefense(ability) || library.Abilities[sourceContentID].Type == "offensive")
}

func (e Engine) defensePreviews(b *state.Battle, actorID string) []DefensePreview {
	if b == nil || b.Settled == nil || b.Settled.Stage != stageOffensiveReact || state.IsTerminalBattleStatus(b.Status) {
		return nil
	}
	if _, ok := b.Flow.PendingInput[actorID]; !ok {
		return nil
	}
	library, err := settledLibrary(b)
	if err != nil {
		return nil
	}
	var result []DefensePreview
	add := func(sourceID, attacker, contentID, target string) {
		if target != actorID {
			return
		}
		for _, abilityID := range b.Settled.Actors[actorID].DefensiveAbilityIDs {
			if !defenseAffordable(b, library, actorID, abilityID, contentID) {
				continue
			}
			option := DefensePreview{SourceID: sourceID, SourceActorID: attacker, SourceContentID: contentID, TargetActorID: target, AbilityID: abilityID}
			result = append(result, option)
			if canPayAbility(b, actorID, library.Abilities[abilityID]) {
				option.SpendCatalyst = true
				result = append(result, option)
			}
		}
	}
	for _, source := range b.Settled.OffensiveSources {
		add(source.ID, source.SourceActorID, source.SourceContentID, source.TargetActorID)
	}
	for _, attacker := range sortedSettledActorIDs(b) {
		if b.Actors[attacker].DefeatState == state.ActorDefeated {
			continue
		}
		runtime := b.Settled.Actors[attacker]
		ops, ok := resolvedOffensiveOperations(b, library, attacker)
		if !ok {
			continue
		}
		outcome := summarizeOffensiveOutcome(ops, runtime.SelectedTargetIDs)
		if outcome["base_damage"].(int) <= 0 {
			continue
		}
		for _, target := range runtime.SelectedTargetIDs {
			add("", attacker, runtime.SelectedAbilityID, target)
		}
	}
	return result
}
