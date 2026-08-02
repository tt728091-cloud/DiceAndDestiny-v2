package transcript

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/state"
)

const (
	SchemaVersion   = 1
	RecorderVersion = "authority-transcript-v1"

	VisibilityPublic          = "PUBLIC"
	VisibilityPrivateSelf     = "PRIVATE_SELF"
	VisibilityPrivateOpponent = "PRIVATE_OPPONENT"
	VisibilityDebugSystem     = "DEBUG_SYSTEM"
)

type Record struct {
	SchemaVersion    int            `json:"schema_version"`
	RecorderVersion  string         `json:"recorder_version"`
	RecordedAtUTC    string         `json:"recorded_at_utc"`
	Sequence         uint64         `json:"sequence"`
	CausedBySequence uint64         `json:"caused_by_sequence,omitempty"`
	RevealOfSequence uint64         `json:"reveal_of_sequence,omitempty"`
	BattleID         string         `json:"battle_id,omitempty"`
	Seed             uint64         `json:"seed,omitempty"`
	Mode             string         `json:"mode,omitempty"`
	HumanActorID     string         `json:"human_actor_id,omitempty"`
	ModelActorID     string         `json:"model_actor_id,omitempty"`
	Round            int            `json:"round,omitempty"`
	Segment          string         `json:"segment,omitempty"`
	Stage            string         `json:"stage,omitempty"`
	WindowID         string         `json:"window_id,omitempty"`
	PendingInputID   string         `json:"pending_input_id,omitempty"`
	Kind             string         `json:"kind"`
	ActorID          string         `json:"actor_id,omitempty"`
	TargetActorID    string         `json:"target_actor_id,omitempty"`
	Controller       string         `json:"controller,omitempty"`
	SourceType       string         `json:"source_type,omitempty"`
	SourceID         string         `json:"source_id,omitempty"`
	ProposalID       string         `json:"proposal_id,omitempty"`
	StatusID         string         `json:"status_id,omitempty"`
	CardInstanceID   string         `json:"card_instance_id,omitempty"`
	CardDefinitionID string         `json:"card_definition_id,omitempty"`
	AbilityID        string         `json:"ability_id,omitempty"`
	Visibility       string         `json:"visibility"`
	Summary          string         `json:"summary"`
	Details          map[string]any `json:"details,omitempty"`
}

type BattleContext struct {
	Mode         string
	HumanActorID string
	ModelActorID string
	Controllers  map[string]string
	Metadata     map[string]any
}

type CommandContext struct {
	Controller     string
	ActionIndex    int
	CandidateCount int
	InferenceMS    float64
	Metadata       map[string]any
}

type Transition struct {
	Command command.Command
	Context CommandContext
	Battle  BattleContext
	Before  *state.Battle
	After   *state.Battle
	Events  []event.Event
	Error   string
}

type Diagnostic struct {
	BattleID   string
	ActorID    string
	Kind       string
	Summary    string
	Details    map[string]any
	Context    BattleContext
	Battle     *state.Battle
	Controller string
}

type draft struct {
	Record
	privateKey string
	revealKeys []string
}
