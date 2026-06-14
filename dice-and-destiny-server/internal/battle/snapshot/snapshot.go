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
	Flow          *SegmentFlow     `json:"flow,omitempty"`
	Actors        map[string]Actor `json:"actors,omitempty"`
}

type Actor struct {
	DefinitionID    string                   `json:"definition_id,omitempty"`
	Controller      state.ControllerType     `json:"controller,omitempty"`
	Character       *CharacterMetadata       `json:"character,omitempty"`
	EnergyPoints    int                      `json:"energy_points"`
	MaxEnergyPoints int                      `json:"max_energy_points,omitempty"`
	MaxHandSize     int                      `json:"max_hand_size,omitempty"`
	MaxHealth       int                      `json:"max_health,omitempty"`
	CurrentHealth   int                      `json:"current_health,omitempty"`
	Decklist        []state.DecklistEntry    `json:"decklist,omitempty"`
	Hand            []string                 `json:"hand,omitempty"`
	HandCount       int                      `json:"hand_count"`
	DeckCount       int                      `json:"deck_count"`
	DiscardCount    int                      `json:"discard_count"`
	RemovedCount    int                      `json:"removed_count"`
	DiceLoadout     []state.DiceLoadoutEntry `json:"dice_loadout,omitempty"`
	DiceCount       int                      `json:"dice_count,omitempty"`
	AbilityIDs      []string                 `json:"abilities,omitempty"`
	AbilityCount    int                      `json:"ability_count,omitempty"`
	Statuses        []state.StatusState      `json:"statuses,omitempty"`
	Tokens          []state.TokenState       `json:"tokens,omitempty"`
	RollPreferences *state.RollPreferences   `json:"roll_preferences,omitempty"`
	Dice            *DiceRollState           `json:"dice,omitempty"`
}

type CharacterMetadata struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type SegmentFlow struct {
	Segment      segment.Segment          `json:"segment"`
	Round        int                      `json:"round"`
	Entered      bool                     `json:"entered"`
	Stage        string                   `json:"stage,omitempty"`
	Iteration    int                      `json:"iteration"`
	Actors       map[string]ActorProgress `json:"actors,omitempty"`
	PendingInput map[string]PendingInput  `json:"pending_input,omitempty"`
}

type ActorProgress struct {
	Status     state.ActorProgressStatus `json:"status"`
	ReasonCode string                    `json:"reason_code,omitempty"`
}

type PendingInput struct {
	ID              string          `json:"id"`
	ActorID         string          `json:"actor_id"`
	Segment         segment.Segment `json:"segment"`
	Stage           string          `json:"stage"`
	Iteration       int             `json:"iteration"`
	InputType       string          `json:"input_type"`
	SourceType      string          `json:"source_type,omitempty"`
	SourceID        string          `json:"source_id,omitempty"`
	AllowedCommands []string        `json:"allowed_commands"`
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
		energyPoints := actor.Resources.EnergyPoints
		if energyPoints == 0 && actor.EnergyPoints != 0 {
			energyPoints = actor.EnergyPoints
		}
		snapshotActor := Actor{
			DefinitionID:    actor.DefinitionID,
			Controller:      actor.Controller,
			Character:       characterSnapshot(actor.Character),
			EnergyPoints:    energyPoints,
			MaxEnergyPoints: actor.Resources.MaxEnergyPoints,
			MaxHandSize:     actor.Resources.MaxHandSize,
			MaxHealth:       actor.Health.MaxHealth,
			HandCount:       len(cards.Hand),
			DeckCount:       len(cards.Deck),
			DiscardCount:    len(cards.Discard),
			RemovedCount:    len(cards.Removed),
			DiceCount:       diceCount(actor.DiceLoadout),
			AbilityCount:    len(actor.AbilityIDs),
			Statuses:        copyStatuses(actor.Statuses),
			Tokens:          copyTokens(actor.Tokens),
		}
		if actor.Health.Model != "" || actor.Health.MaxHealth != 0 {
			snapshotActor.CurrentHealth = len(cards.Deck) + len(cards.Hand) + len(cards.Discard)
		}
		if actor.Dice.CurrentRoll != nil && diceVisibleToViewer(battle, id, viewerActorID) {
			snapshotActor.Dice = diceRollStateSnapshot(actor.Dice.CurrentRoll)
		}
		if id == viewerActorID {
			snapshotActor.Hand = append([]string(nil), cards.Hand...)
			snapshotActor.Decklist = copyDecklist(actor.Decklist)
			snapshotActor.DiceLoadout = copyDiceLoadout(actor.DiceLoadout)
			snapshotActor.AbilityIDs = copyStrings(actor.AbilityIDs)
			if actor.RollPreferences != (state.RollPreferences{}) {
				preferences := actor.RollPreferences
				snapshotActor.RollPreferences = &preferences
			}
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
		Flow:          flowSnapshot(battle, viewerActorID),
		Actors:        actors,
	}
}

func characterSnapshot(value state.CharacterMetadata) *CharacterMetadata {
	if value == (state.CharacterMetadata{}) {
		return nil
	}
	return &CharacterMetadata{ID: value.ID, Name: value.Name, Class: value.Class}
}

func diceCount(loadout []state.DiceLoadoutEntry) int {
	total := 0
	for _, entry := range loadout {
		total += entry.Count
	}
	return total
}

func copyDecklist(values []state.DecklistEntry) []state.DecklistEntry {
	return append([]state.DecklistEntry(nil), values...)
}

func copyDiceLoadout(values []state.DiceLoadoutEntry) []state.DiceLoadoutEntry {
	return append([]state.DiceLoadoutEntry(nil), values...)
}

func copyStatuses(values []state.StatusState) []state.StatusState {
	return append([]state.StatusState(nil), values...)
}

func copyTokens(values []state.TokenState) []state.TokenState {
	return append([]state.TokenState(nil), values...)
}

func PendingInputForViewer(battle state.Battle, viewerActorID string) map[string]PendingInput {
	pending, ok := battle.Flow.PendingInput[viewerActorID]
	if !ok {
		return nil
	}
	return map[string]PendingInput{
		viewerActorID: pendingInputSnapshot(pending),
	}
}

func flowSnapshot(battle state.Battle, viewerActorID string) *SegmentFlow {
	if battle.Flow.Segment == "" {
		return nil
	}
	actors := make(map[string]ActorProgress, len(battle.Flow.Actors))
	for actorID, actor := range battle.Flow.Actors {
		actors[actorID] = ActorProgress{
			Status:     actor.Status,
			ReasonCode: actor.ReasonCode,
		}
	}
	if len(actors) == 0 {
		actors = nil
	}
	return &SegmentFlow{
		Segment:      battle.Flow.Segment,
		Round:        battle.Flow.Round,
		Entered:      battle.Flow.Entered,
		Stage:        battle.Flow.Stage,
		Iteration:    battle.Flow.Iteration,
		Actors:       actors,
		PendingInput: PendingInputForViewer(battle, viewerActorID),
	}
}

func pendingInputSnapshot(pending state.PendingInput) PendingInput {
	allowed := make([]string, len(pending.AllowedCommands))
	for i, commandType := range pending.AllowedCommands {
		allowed[i] = string(commandType)
	}
	return PendingInput{
		ID:              pending.ID,
		ActorID:         pending.ActorID,
		Segment:         pending.Segment,
		Stage:           pending.Stage,
		Iteration:       pending.Iteration,
		InputType:       pending.InputType,
		SourceType:      pending.SourceType,
		SourceID:        pending.SourceID,
		AllowedCommands: allowed,
	}
}

func diceVisibleToViewer(battle state.Battle, actorID string, viewerActorID string) bool {
	if actorID == viewerActorID {
		return true
	}
	return battle.Flow.Segment != segment.Offensive || battle.Flow.Stage == "reveal"
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
