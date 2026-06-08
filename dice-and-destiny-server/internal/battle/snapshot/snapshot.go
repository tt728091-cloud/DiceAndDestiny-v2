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
	EnergyPoints int            `json:"energy_points"`
	Hand         []string       `json:"hand,omitempty"`
	HandCount    int            `json:"hand_count"`
	DeckCount    int            `json:"deck_count"`
	DiscardCount int            `json:"discard_count"`
	RemovedCount int            `json:"removed_count"`
	Dice         *DiceRollState `json:"dice,omitempty"`
}

type DiceRollState struct {
	RequestID      string               `json:"request_id"`
	Segment        segment.Segment      `json:"segment"`
	Pool           state.RollPool       `json:"pool"`
	SourceType     state.RollSourceType `json:"source_type"`
	SourceID       string               `json:"source_id"`
	Dice           []state.RolledDie    `json:"dice"`
	KeptIndices    []int                `json:"kept_indices,omitempty"`
	RollsUsed      int                  `json:"rolls_used"`
	MaxRolls       int                  `json:"max_rolls"`
	RollsRemaining int                  `json:"rolls_remaining"`
	Combinations   []string             `json:"combinations,omitempty"`
	SymbolCounts   map[string]int       `json:"symbol_counts,omitempty"`
	Complete       bool                 `json:"complete,omitempty"`
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
		if actor.Dice.CurrentRoll != nil {
			snapshotActor.Dice = diceRollStateSnapshot(actor.Dice.CurrentRoll)
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

func diceRollStateSnapshot(roll *state.RollState) *DiceRollState {
	if roll == nil {
		return nil
	}

	return &DiceRollState{
		RequestID:      roll.RequestID,
		Segment:        roll.Segment,
		Pool:           roll.Pool,
		SourceType:     roll.SourceType,
		SourceID:       roll.SourceID,
		Dice:           copyRolledDice(roll.Dice),
		KeptIndices:    append([]int(nil), roll.KeptIndices...),
		RollsUsed:      roll.RollsUsed,
		MaxRolls:       roll.MaxRolls,
		RollsRemaining: roll.MaxRolls - roll.RollsUsed,
		Combinations:   append([]string(nil), roll.Combinations...),
		SymbolCounts:   copySymbolCounts(roll.SymbolCounts),
		Complete:       roll.Complete,
	}
}

func copyRolledDice(values []state.RolledDie) []state.RolledDie {
	copied := make([]state.RolledDie, len(values))
	for i, value := range values {
		copied[i] = value
		copied[i].Symbols = copyStrings(value.Symbols)
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func copySymbolCounts(values map[string]int) map[string]int {
	if values == nil {
		return nil
	}
	copied := make(map[string]int, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
