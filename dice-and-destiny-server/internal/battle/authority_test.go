package battle

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestHandleCommandRollDice(t *testing.T) {
	commandJSON := `{
		"battle_id": "battle-1",
		"actor_id": "player",
		"type": "roll_dice",
		"payload": {
			"pool": "offensive"
		}
	}`

	gotJSON := HandleCommand(commandJSON)

	var got result
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatalf("HandleCommand returned invalid JSON: %v\n%s", err, gotJSON)
	}

	want := result{
		Accepted: true,
		Events: []event{
			{
				Type:    "dice_rolled",
				ActorID: "player",
				Values:  []string{"sword", "shield", "focus"},
			},
		},
		Snapshot: &snapshot{
			BattleID: "battle-1",
			Segment:  "offensive",
			Round:    1,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HandleCommand() mismatch\n got: %#v\nwant: %#v\njson: %s", got, want, gotJSON)
	}
}
