package participant

import (
	"diceanddestiny/server/internal/battle/state"
)

type SourceKind string

const (
	SourceRunPlayer           SourceKind = "run_player"
	SourceCharacterDefinition SourceKind = "character_definition"
	SourceEnemyDefinition     SourceKind = "enemy_definition"
)

type Participant struct {
	InstanceID   string
	DefinitionID string
	Controller   state.ControllerType
	Source       SourceKind
}

type Assembler interface {
	AssembleParticipants(participants []Participant) (state.BattleSetup, error)
}

type AssemblerFunc func(participants []Participant) (state.BattleSetup, error)

func (fn AssemblerFunc) AssembleParticipants(
	participants []Participant,
) (state.BattleSetup, error) {
	return fn(participants)
}
