package engine

import (
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// outcomeMatches reports whether a roll outcome covers a rolled face: by face
// number, or by the role its symbol has on the die that rolled it.
func outcomeMatches(library content.BattleLibrary, dieID string, outcome content.BattleOutcome, face int) bool {
	if containsInt(outcome.Faces, face) {
		return true
	}
	if len(outcome.Roles) == 0 {
		return false
	}
	role := faceRole(library, dieID, face)
	return role != "" && containsString(outcome.Roles, role)
}

// faceRole is the role of a face's symbol on a die: the catalog's published
// roles, or the role function for catalogs pinned before roles existed.
func faceRole(library content.BattleLibrary, dieID string, face int) string {
	die, ok := library.Dice[dieID]
	if !ok {
		return ""
	}
	roles := die.Roles
	if roles == nil {
		layout, err := content.DieRoles(die)
		if err != nil {
			return ""
		}
		roles = layout.Roles
	}
	for _, f := range die.Faces {
		if f.Number == face {
			return roles[f.Symbol]
		}
	}
	return ""
}

// ownerDieID is the actor's own combat die, the die "dice: owner" rolls.
func ownerDieID(battle *state.Battle, actorID string) string {
	if actor, ok := battle.Actors[actorID]; ok && len(actor.DiceLoadout) > 0 {
		return actor.DiceLoadout[0].DiceID
	}
	return ""
}

// defenseDieIDs pairs each rolled defense face with the die that rolled it.
// Defenses roll the defender's own dice.
func defenseDieIDs(battle *state.Battle, s state.SettledDefense) []string {
	faces := defenseFaces(s)
	out := make([]string, len(faces))
	for i := range faces {
		if i < len(s.RolledDice) && s.RolledDice[i].DieID != "" {
			out[i] = s.RolledDice[i].DieID
		} else {
			out[i] = ownerDieID(battle, s.ActorID)
		}
	}
	return out
}
