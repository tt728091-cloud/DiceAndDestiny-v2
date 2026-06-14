package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type Type string

const (
	TypeStartBattle    Type = "start_battle"
	TypeAdvanceSegment Type = "advance_segment"
	TypeRollDice       Type = "roll_dice"
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
