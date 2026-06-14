package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type Type string

const (
	TypeStartBattle       Type = "start_battle"
	TypeAdvanceSegment    Type = "advance_segment"
	TypeRollDice          Type = "roll_dice"
	TypeCommitInteraction Type = "commit_interaction"
	TypePass              Type = "pass"
	TypePlanningRoll      Type = "planning_roll"
	TypePlanningKeep      Type = "planning_keep"
	TypePlanningReroll    Type = "planning_reroll"
	TypePlanningCards     Type = "planning_commit_cards"
	TypePlanningAbility   Type = "planning_select_ability"
	TypePlanningTargets   Type = "planning_select_targets"
	TypePlanningPass      Type = "planning_pass"
	TypePlanningLockIn    Type = "planning_lock_in"
)

var (
	ErrInvalidJSON     = errors.New("invalid command JSON")
	ErrInvalidEnvelope = errors.New("invalid command envelope")
)

type Command struct {
	BattleID string
	ActorID  string
	Type     Type
	Payload  json.RawMessage
}

type envelope struct {
	BattleID       string          `json:"battle_id"`
	ActorID        string          `json:"actor_id"`
	Type           Type            `json:"type"`
	Action         Type            `json:"action"`
	Payload        json.RawMessage `json:"payload"`
	RequestID      string          `json:"request_id,omitempty"`
	PendingInputID string          `json:"pending_input_id,omitempty"`
	RerollIndices  []int           `json:"reroll_indices,omitempty"`
}

type AdvanceSegmentPayload struct{}

type ParticipantDescriptor struct {
	InstanceID   string `json:"instance_id"`
	DefinitionID string `json:"definition_id"`
}

type StartBattlePayload struct {
	Player  ParticipantDescriptor   `json:"player"`
	Enemies []ParticipantDescriptor `json:"enemies"`
}

type RollDicePayload struct {
	RequestID      string `json:"request_id,omitempty"`
	PendingInputID string `json:"pending_input_id,omitempty"`
	RerollIndices  []int  `json:"reroll_indices,omitempty"`
}

type InteractionCheckpoint struct {
	WindowID      string `json:"window_id"`
	Stage         string `json:"stage"`
	Iteration     int    `json:"iteration"`
	ReactionRound int    `json:"reaction_round"`
	PlanningCycle int    `json:"planning_cycle,omitempty"`
}

type PlanningCheckpoint struct {
	WindowID      string `json:"window_id"`
	Segment       string `json:"segment"`
	Stage         string `json:"stage"`
	Iteration     int    `json:"iteration"`
	PlanningCycle int    `json:"planning_cycle"`
}

type PlanningRollPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
}

type PlanningKeepPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
	KeptIndices    []int              `json:"kept_indices"`
}

type PlanningRerollPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
	RerollIndices  []int              `json:"reroll_indices"`
}

type PlanningCardsPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
	CardIDs        []string           `json:"card_ids"`
}

type PlanningAbilityPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
	AbilityID      string             `json:"ability_id"`
}

type PlanningTargetsPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
	TargetIDs      []string           `json:"target_ids"`
}

type PlanningPassPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
}

type PlanningLockInPayload struct {
	PendingInputID string             `json:"pending_input_id"`
	Checkpoint     PlanningCheckpoint `json:"checkpoint"`
}

type InteractionCommitmentData struct {
	ProposalIDs         []string             `json:"proposal_ids,omitempty"`
	CardIDs             []string             `json:"card_ids,omitempty"`
	TargetIDs           []string             `json:"target_ids,omitempty"`
	ChoiceID            string               `json:"choice_id,omitempty"`
	Value               *int                 `json:"value,omitempty"`
	PlanningAdjustments []PlanningAdjustment `json:"planning_adjustments,omitempty"`
}

type PlanningAdjustment struct {
	Type     string `json:"type"`
	ActorID  string `json:"actor_id"`
	DieIndex int    `json:"die_index,omitempty"`
	Face     int    `json:"face,omitempty"`
	Amount   int    `json:"amount,omitempty"`
	TargetID string `json:"target_id,omitempty"`
}

type CommitInteractionPayload struct {
	PendingInputID string                    `json:"pending_input_id"`
	Checkpoint     InteractionCheckpoint     `json:"checkpoint"`
	Commitment     InteractionCommitmentData `json:"commitment"`
}

type PassPayload struct {
	PendingInputID string                `json:"pending_input_id"`
	Checkpoint     InteractionCheckpoint `json:"checkpoint"`
}

func ParseJSON(commandJSON string) (Command, error) {
	var env envelope
	if err := json.Unmarshal([]byte(commandJSON), &env); err != nil {
		return Command{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	cmd := Command{
		BattleID: env.BattleID,
		ActorID:  env.ActorID,
		Type:     env.Type,
		Payload:  env.Payload,
	}
	if cmd.Type == "" {
		cmd.Type = env.Action
	}
	if len(cmd.Payload) == 0 && cmd.Type == TypeRollDice {
		payload, err := json.Marshal(RollDicePayload{
			RequestID:      env.RequestID,
			PendingInputID: env.PendingInputID,
			RerollIndices:  env.RerollIndices,
		})
		if err != nil {
			return Command{}, fmt.Errorf("%w: roll_dice payload could not be built", ErrInvalidEnvelope)
		}
		cmd.Payload = payload
	}

	if err := cmd.ValidateEnvelope(); err != nil {
		return Command{}, err
	}

	return cmd, nil
}

func (c Command) ValidateEnvelope() error {
	switch {
	case c.BattleID == "":
		return fmt.Errorf("%w: battle_id is required", ErrInvalidEnvelope)
	case c.ActorID == "":
		return fmt.Errorf("%w: actor_id is required", ErrInvalidEnvelope)
	case c.Type == "":
		return fmt.Errorf("%w: type is required", ErrInvalidEnvelope)
	case len(c.Payload) == 0:
		return fmt.Errorf("%w: payload is required", ErrInvalidEnvelope)
	}

	trimmed := bytes.TrimSpace(c.Payload)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("%w: payload must be a JSON object", ErrInvalidEnvelope)
	}

	return nil
}

func DecodePayload(cmd Command, target any) error {
	if err := json.Unmarshal(cmd.Payload, target); err != nil {
		return fmt.Errorf("decode %q payload: %w", cmd.Type, err)
	}
	return nil
}
