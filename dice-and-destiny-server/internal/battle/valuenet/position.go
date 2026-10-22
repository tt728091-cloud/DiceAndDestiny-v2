// Package valuenet judges battle positions for the look-ahead player with a
// small learned network (value-v1). The encoder describes a position only by
// what it shows one seat and what its content does, never by card, ability,
// status or character ID, so the network can judge boards it has never
// seen. The network itself is trained offline (ml/dice_destiny_ml/
// value_train.py) and evaluated here in pure Go.
package valuenet

import (
	"sort"

	"diceanddestiny/server/internal/battle/engine"
	"diceanddestiny/server/internal/battle/lookahead"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
	"diceanddestiny/server/internal/content"
)

// Position is one seat's view of a battle: the viewer-safe snapshot, the
// battle's pinned content catalog (public, the same for every seat) and each
// side's dice. Dice are public table setup that the snapshot only lists for
// the viewer; every player sees which dice the others hold. Acting is the
// seat the authority is waiting on.
type Position struct {
	Seat    string
	Acting  string
	View    snapshot.Battle
	Library content.BattleLibrary
	Dice    map[string]DiceSetup
}

// DiceSetup is one side's offensive dice: a single die type and how many.
// A side with mixed dice has none (the solver only plans identical dice).
type DiceSetup struct {
	Die   state.DiceDefinition
	Count int
}

// NewPosition builds the seat's position. It reads the battle only through
// the simulator's viewer-safe snapshot, the pinned catalog and the dice
// loadouts.
func NewPosition(sim *lookahead.Simulator, b *state.Battle, seat, acting string) (Position, error) {
	lib, err := engine.SettledLibrary(b)
	if err != nil {
		return Position{}, err
	}
	dice := map[string]DiceSetup{}
	for id, actor := range b.Actors {
		if len(actor.DiceLoadout) == 1 {
			dice[id] = DiceSetup{Die: b.DiceDefinitions[actor.DiceLoadout[0].DiceID], Count: actor.DiceLoadout[0].Count}
		}
	}
	return Position{Seat: seat, Acting: acting, View: sim.Snapshot(b, seat), Library: lib, Dice: dice}, nil
}

// opponent is the other team's actor with the most health left (as a share
// of its maximum), the side the health race is run against.
func (p Position) opponent() string {
	own := p.View.Actors[p.Seat]
	var ids []string
	for id, actor := range p.View.Actors {
		if id != p.Seat && (own.TeamID == "" || actor.TeamID != own.TeamID) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	best, bestFrac := "", -1.0
	for _, id := range ids {
		a := p.View.Actors[id]
		if frac := float64(liveCards(a)) / float64(max(allCards(a), 1)); frac > bestFrac {
			best, bestFrac = id, frac
		}
	}
	return best
}

func liveCards(a snapshot.Actor) int { return a.HandCount + a.DeckCount + a.DiscardCount }

func allCards(a snapshot.Actor) int { return liveCards(a) + a.RemovedCount }
