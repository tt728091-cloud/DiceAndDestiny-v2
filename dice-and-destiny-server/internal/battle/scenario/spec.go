package scenario

import (
	"encoding/json"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/participant"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

const SchemaVersion = 1

type Metadata struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type EntryPoint struct {
	Round   int             `json:"round" yaml:"round"`
	Segment segment.Segment `json:"segment" yaml:"segment"`
}

type RandomPolicy struct {
	Mode string `json:"mode" yaml:"mode"`
	Seed uint64 `json:"seed,omitempty" yaml:"seed,omitempty"`
}

type ParticipantSpec struct {
	InstanceID   string                 `json:"instance_id" yaml:"instance_id"`
	DefinitionID string                 `json:"definition_id" yaml:"definition_id"`
	Controller   state.ControllerType   `json:"controller" yaml:"controller"`
	Source       participant.SourceKind `json:"source" yaml:"source"`
}

type ActorOverride struct {
	CardZones   *state.CardZones        `json:"card_zones,omitempty" yaml:"card_zones,omitempty"`
	Energy      *int                    `json:"energy,omitempty" yaml:"energy,omitempty"`
	Statuses    *[]state.StatusState    `json:"statuses,omitempty" yaml:"statuses,omitempty"`
	Tokens      *[]state.TokenState     `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	DefeatState *state.ActorDefeatState `json:"defeat_state,omitempty" yaml:"defeat_state,omitempty"`
}

func (override ActorOverride) SetCardZones(zones state.CardZones) ActorOverride {
	override.CardZones = &zones
	return override
}

func (override ActorOverride) SetEnergy(energy int) ActorOverride {
	override.Energy = &energy
	return override
}

func (override ActorOverride) SetStatuses(statuses []state.StatusState) ActorOverride {
	override.Statuses = &statuses
	return override
}

func (override ActorOverride) AddStatus(status state.StatusState) ActorOverride {
	var statuses []state.StatusState
	if override.Statuses != nil {
		statuses = append(statuses, (*override.Statuses)...)
	}
	statuses = append(statuses, status)
	override.Statuses = &statuses
	return override
}

func (override ActorOverride) SetTokens(tokens []state.TokenState) ActorOverride {
	override.Tokens = &tokens
	return override
}

func (override ActorOverride) SetDefeatState(defeat state.ActorDefeatState) ActorOverride {
	override.DefeatState = &defeat
	return override
}

type SegmentPrerequisites struct {
	OffensiveProposals []state.PlanningProposal `json:"offensive_proposals,omitempty" yaml:"offensive_proposals,omitempty"`
	DefensiveProposals []state.PlanningProposal `json:"defensive_proposals,omitempty" yaml:"defensive_proposals,omitempty"`
}

type WaitExpectation struct {
	Segment       segment.Segment          `json:"segment,omitempty" yaml:"segment,omitempty"`
	InputType     string                   `json:"input_type,omitempty" yaml:"input_type,omitempty"`
	WindowPurpose state.InteractionPurpose `json:"window_purpose,omitempty" yaml:"window_purpose,omitempty"`
}

type ScriptStep struct {
	ActorID string          `json:"actor_id" yaml:"actor_id"`
	Type    command.Type    `json:"type" yaml:"type"`
	Payload json.RawMessage `json:"payload" yaml:"-"`
	Expect  WaitExpectation `json:"expect,omitempty" yaml:"expect,omitempty"`
}

func (step *ScriptStep) UnmarshalYAML(unmarshal func(any) error) error {
	var raw struct {
		ActorID string                 `yaml:"actor_id"`
		Type    command.Type           `yaml:"type"`
		Payload map[string]interface{} `yaml:"payload"`
		Expect  WaitExpectation        `yaml:"expect"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	payload, err := json.Marshal(raw.Payload)
	if err != nil {
		return err
	}
	step.ActorID = raw.ActorID
	step.Type = raw.Type
	step.Payload = payload
	step.Expect = raw.Expect
	return nil
}

type Spec struct {
	SchemaVersion int                      `json:"schema_version" yaml:"schema_version"`
	BattleID      string                   `json:"battle_id,omitempty" yaml:"battle_id,omitempty"`
	Metadata      Metadata                 `json:"metadata" yaml:"metadata"`
	Player        ParticipantSpec          `json:"player" yaml:"player"`
	Enemies       []ParticipantSpec        `json:"enemies" yaml:"enemies"`
	Entry         EntryPoint               `json:"entry" yaml:"entry"`
	Actors        map[string]ActorOverride `json:"actors,omitempty" yaml:"actors,omitempty"`
	Prerequisites SegmentPrerequisites     `json:"prerequisites,omitempty" yaml:"prerequisites,omitempty"`
	Random        RandomPolicy             `json:"random" yaml:"random"`
	SetupScript   []ScriptStep             `json:"setup_script,omitempty" yaml:"setup_script,omitempty"`
}

func (spec Spec) Participants() []participant.Participant {
	values := make([]participant.Participant, 0, 1+len(spec.Enemies))
	for _, value := range append([]ParticipantSpec{spec.Player}, spec.Enemies...) {
		values = append(values, participant.Participant{
			InstanceID:   value.InstanceID,
			DefinitionID: value.DefinitionID,
			Controller:   value.Controller,
			Source:       value.Source,
		})
	}
	return values
}
