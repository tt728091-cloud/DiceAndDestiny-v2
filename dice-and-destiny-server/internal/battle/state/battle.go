package state

import (
	"errors"

	"diceanddestiny/server/internal/battle/segment"
)

type Battle struct {
	ID              string
	Segment         segment.State
	Actors          map[string]ActorState
	DiceDefinitions map[string]DiceDefinition
	RollRequests    map[string]RollRequest
}

type ActorState struct {
	Cards        CardZones
	DiceLoadout  []DiceLoadoutEntry
	Dice         DiceState
	EnergyPoints int
}

type CardZones struct {
	Deck    []string
	Hand    []string
	Discard []string
	Removed []string
}

type BattleSetup struct {
	Actors          []ActorSetup
	DiceDefinitions []DiceDefinition
}

type ActorSetup struct {
	ID          string
	Deck        []string
	Hand        []string
	Discard     []string
	Removed     []string
	DiceLoadout []DiceLoadoutEntry
}

type DiceDefinition struct {
	ID        string
	Name      string
	DieType   string
	SideCount int
	Faces     []DiceFace
}

type DiceFace struct {
	Face    int
	Value   int
	Symbols []string
}

type DiceLoadoutEntry struct {
	DiceID string
	Count  int
}

type DiceState struct {
	CurrentRoll *RollState
}

type RollPool string

const (
	RollPoolOffensive RollPool = "offensive"
	RollPoolDefensive RollPool = "defensive"
	RollPoolEffect    RollPool = "effect"
	RollPoolCard      RollPool = "card"
)

type RollSourceType string

const (
	RollSourceSegment RollSourceType = "segment"
	RollSourceAbility RollSourceType = "ability"
	RollSourceCard    RollSourceType = "card"
	RollSourceStatus  RollSourceType = "status"
	RollSourceSystem  RollSourceType = "system"
)

type RollRequest struct {
	ID          string
	ActorID     string
	Segment     segment.Segment
	Pool        RollPool
	SourceType  RollSourceType
	SourceID    string
	DiceLoadout []DiceLoadoutEntry
	MaxRolls    int
	Required    bool
	Complete    bool
}

type RollState struct {
	RequestID    string
	ActorID      string
	Segment      segment.Segment
	Pool         RollPool
	SourceType   RollSourceType
	SourceID     string
	Dice         []RolledDie
	KeptIndices  []int
	RollsUsed    int
	MaxRolls     int
	Combinations []string
	SymbolCounts map[string]int
	Complete     bool
}

type RolledDie struct {
	Index   int      `json:"index"`
	DieID   string   `json:"die_id"`
	Face    int      `json:"face"`
	Value   int      `json:"value"`
	Symbols []string `json:"symbols"`
}

func NewBattle(id string) (Battle, error) {
	return NewBattleFromSetup(id, BattleSetup{})
}

func NewBattleFromSetup(id string, setup BattleSetup) (Battle, error) {
	if id == "" {
		return Battle{}, errors.New("battle id is required")
	}

	actors := make(map[string]ActorState, len(setup.Actors))
	for _, actor := range setup.Actors {
		if actor.ID == "" {
			return Battle{}, errors.New("actor id is required")
		}

		actors[actor.ID] = ActorState{
			DiceLoadout: copyDiceLoadout(actor.DiceLoadout),
			Cards: CardZones{
				Deck:    append([]string(nil), actor.Deck...),
				Hand:    append([]string(nil), actor.Hand...),
				Discard: append([]string(nil), actor.Discard...),
				Removed: append([]string(nil), actor.Removed...),
			},
		}
	}

	return Battle{
		ID:              id,
		Segment:         segment.NewManager().InitialState(),
		Actors:          actors,
		DiceDefinitions: copyDiceDefinitions(setup.DiceDefinitions),
		RollRequests:    make(map[string]RollRequest),
	}, nil
}

func copyDiceLoadout(values []DiceLoadoutEntry) []DiceLoadoutEntry {
	return append([]DiceLoadoutEntry(nil), values...)
}

func copyDiceDefinitions(values []DiceDefinition) map[string]DiceDefinition {
	if len(values) == 0 {
		return nil
	}

	copied := make(map[string]DiceDefinition, len(values))
	for _, definition := range values {
		faces := make([]DiceFace, len(definition.Faces))
		for i, face := range definition.Faces {
			faces[i] = DiceFace{
				Face:    face.Face,
				Value:   face.Value,
				Symbols: copyStrings(face.Symbols),
			}
		}
		definition.Faces = faces
		copied[definition.ID] = definition
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
