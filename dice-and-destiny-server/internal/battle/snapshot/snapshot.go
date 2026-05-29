package snapshot

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Battle is the read-only view returned after events have been applied.
// It is safe for presentation or future network clients to render from.
type Battle struct {
	BattleID      string           `json:"battle_id"`
	Segment       segment.Segment  `json:"segment"`
	Round         int              `json:"round"`
	ViewerActorID string           `json:"viewer_actor_id,omitempty"`
	Actors        map[string]Actor `json:"actors,omitempty"`
}

type Actor struct {
	EnergyPoints int      `json:"energy_points"`
	Hand         []string `json:"hand,omitempty"`
	HandCount    int      `json:"hand_count"`
	DeckCount    int      `json:"deck_count"`
	DiscardCount int      `json:"discard_count"`
	RemovedCount int      `json:"removed_count"`
}

// FromBattle copies authoritative battle state into the public snapshot shape.
// Do not expose mutable state structs directly across the authority boundary.
func FromBattle(battle state.Battle) Battle {
	return FromBattleForViewer(battle, "")
}

// FromBattleForViewer copies authoritative battle state into a viewer-safe
// snapshot. Only the viewer actor receives hidden hand card IDs; all other
// actors expose counts only.
func FromBattleForViewer(battle state.Battle, viewerActorID string) Battle {
	actors := make(map[string]Actor, len(battle.Actors))
	for id, actor := range battle.Actors {
		cards := actor.Cards
		snapshotActor := Actor{
			EnergyPoints: actor.EnergyPoints,
			HandCount:    len(cards.Hand),
			DeckCount:    len(cards.Deck),
			DiscardCount: len(cards.Discard),
			RemovedCount: len(cards.Removed),
		}
		if id == viewerActorID {
			snapshotActor.Hand = append([]string(nil), cards.Hand...)
		}
		actors[id] = snapshotActor
	}
	if len(actors) == 0 {
		actors = nil
	}

	return Battle{
		BattleID:      battle.ID,
		Segment:       battle.Segment.Current,
		Round:         battle.Segment.Round,
		ViewerActorID: viewerActorID,
		Actors:        actors,
	}
}
