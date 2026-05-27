package snapshot

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Battle is the read-only view returned after events have been applied.
// It is safe for presentation or future network clients to render from.
type Battle struct {
	BattleID string           `json:"battle_id"`
	Segment  segment.Segment  `json:"segment"`
	Round    int              `json:"round"`
	Actors   map[string]Actor `json:"actors,omitempty"`
}

type Actor struct {
	EnergyPoints int `json:"energy_points"`
}

// FromBattle copies authoritative battle state into the public snapshot shape.
// Do not expose mutable state structs directly across the authority boundary.
func FromBattle(battle state.Battle) Battle {
	actors := make(map[string]Actor, len(battle.Actors))
	for id, actor := range battle.Actors {
		actors[id] = Actor{
			EnergyPoints: actor.EnergyPoints,
		}
	}
	if len(actors) == 0 {
		actors = nil
	}

	return Battle{
		BattleID: battle.ID,
		Segment:  battle.Segment.Current,
		Round:    battle.Segment.Round,
		Actors:   actors,
	}
}
