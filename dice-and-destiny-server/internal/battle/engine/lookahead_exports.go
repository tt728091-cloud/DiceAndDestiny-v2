package engine

import (
	"fmt"

	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// QualifiedAbilityTier reports the first activation tier the dice meet, using
// the authority's own qualification rules. Look-ahead tooling that plans dice
// calls it instead of copying the rules.
func QualifiedAbilityTier(ability content.BattleAbilityDefinition, dice []state.RolledDie) (content.AbilityTier, bool) {
	return qualifiedTier(ability, dice)
}

// SettledLibrary decodes a battle's pinned content catalog through the
// engine's shared cache.
func SettledLibrary(battle *state.Battle) (content.BattleLibrary, error) {
	return settledLibrary(battle)
}

// ApplyStatus adds stacks of a status to an actor through the authority's
// own stacking rules. Look-ahead tooling (the value probe) uses it to try
// "what if" changes on copies of a battle, never on a live one.
func ApplyStatus(battle *state.Battle, actorID, statusID string, stacks int) error {
	library, err := settledLibrary(battle)
	if err != nil {
		return err
	}
	if _, ok := library.Statuses[statusID]; !ok {
		return fmt.Errorf("status %q is not in the battle's catalog", statusID)
	}
	applyStatus(battle, library, actorID, statusID, stacks)
	return nil
}

// GainEnergy adds energy within the actor's cap, as an energy gain does; for
// look-ahead tooling on copies of a battle.
func GainEnergy(battle *state.Battle, actorID string, amount int) {
	gainEnergy(battle, actorID, amount)
}

// DrawCard draws one card into the actor's hand as an ordinary card draw
// does (an empty draw pile draws nothing); for look-ahead tooling on copies
// of a battle.
func (e Engine) DrawCard(battle *state.Battle, actorID string) (string, error) {
	return e.drawSettledCard(battle, actorID, "card_draw")
}
