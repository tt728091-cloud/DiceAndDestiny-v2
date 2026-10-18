package engine

import (
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
