package snapshot

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Battle is the read-only view returned after events have been applied.
// It is safe for presentation or future network clients to render from.
type Battle struct {
	BattleID string          `json:"battle_id"`
	Segment  segment.Segment `json:"segment"`
	Round    int             `json:"round"`
}

// FromBattle copies authoritative battle state into the public snapshot shape.
// Do not expose mutable state structs directly across the authority boundary.
func FromBattle(battle state.Battle) Battle {
	return Battle{
		BattleID: battle.ID,
		Segment:  battle.Segment.Current,
		Round:    battle.Segment.Round,
	}
}
