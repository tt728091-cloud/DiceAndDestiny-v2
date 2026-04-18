package battle

import (
	"encoding/json"
)

type command struct {
	BattleID string         `json:"battle_id"`
	ActorID  string         `json:"actor_id"`
	Type     string         `json:"type"`
	Payload  commandPayload `json:"payload"`
}

type commandPayload struct {
	Pool string `json:"pool"`
}

type result struct {
	Accepted bool      `json:"accepted"`
	Events   []event   `json:"events,omitempty"`
	Snapshot *snapshot `json:"snapshot,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type event struct {
	Type    string   `json:"type"`
	ActorID string   `json:"actor_id"`
	Values  []string `json:"values"`
}

type snapshot struct {
	BattleID string `json:"battle_id"`
	Segment  string `json:"segment"`
	Round    int    `json:"round"`
}

// HandleCommand is the portable battle authority entry point for the spike.
func HandleCommand(commandJSON string) string {
	var cmd command
	if err := json.Unmarshal([]byte(commandJSON), &cmd); err != nil {
		return marshalResult(result{
			Accepted: false,
			Error:    "invalid command JSON",
		})
	}

	if cmd.Type != "roll_dice" {
		return marshalResult(result{
			Accepted: false,
			Error:    "unsupported command type",
		})
	}

	if cmd.Payload.Pool != "offensive" {
		return marshalResult(result{
			Accepted: false,
			Error:    "unsupported dice pool",
		})
	}

	return marshalResult(result{
		Accepted: true,
		Events: []event{
			{
				Type:    "dice_rolled",
				ActorID: cmd.ActorID,
				Values:  []string{"sword", "shield", "focus"},
			},
		},
		Snapshot: &snapshot{
			BattleID: cmd.BattleID,
			Segment:  cmd.Payload.Pool,
			Round:    1,
		},
	})
}

func marshalResult(r result) string {
	payload, err := json.Marshal(r)
	if err != nil {
		return `{"accepted":false,"error":"result serialization failed"}`
	}
	return string(payload)
}
