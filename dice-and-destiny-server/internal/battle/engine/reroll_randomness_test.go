package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

// Reproduce the reported round-three rolls using the actual planning commands.
// Identical faces are legal: the important distinction is consuming NEW draws.
func TestRepeatedThreeDiceRerollConsumesFreshDraws(t *testing.T) {
	b, lib := venomFixture(t)
	b.Segment.Current = segment.Offensive
	b.Random = state.RandomState{Mode: state.RandomModeReproducible, Algorithm: state.RandomAlgorithmSHA256, Seed: 1789498827225662, Cursor: 90}
	r := b.Settled.Actors["player"]
	r.FinalDice = rolledFaces(lib, "venom_d6", []int{6, 3, 4, 2, 4})
	r.RollsUsed, r.MaxRolls = 1, 3
	b.Settled.Actors["player"] = r
	openSettledWindow(&b, "planning", stageOffensivePlan, "planning", []command.Type{command.TypePlanningKeep, command.TypePlanningReroll})
	e := NewEngine()
	keep, _ := json.Marshal(command.PlanningKeepPayload{KeptIndices: []int{1, 3}})
	reroll, _ := json.Marshal(command.PlanningRerollPayload{RerollIndices: []int{0, 2, 4}})
	for roll := 0; roll < 2; roll++ {
		before := b.Random.Cursor
		if _, err := e.handleOffensivePlanningCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningKeep, Payload: keep}); err != nil {
			t.Fatal(err)
		}
		if b.Random.Cursor != before {
			t.Fatal("keeping dice changed random state")
		}
		// The authority clones/persists state between submissions. Neither may
		// reset the cursor, even when the two commands arrive back-to-back.
		b = b.Clone()
		encoded, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		var restored state.Battle
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		b = restored
		events, err := e.handleOffensivePlanningCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningReroll, Payload: reroll})
		if err != nil {
			t.Fatal(err)
		}
		if b.Random.Cursor != before+3 {
			t.Fatalf("cursor = %d, want %d", b.Random.Cursor, before+3)
		}
		var faces []int
		for _, die := range b.Settled.Actors["player"].FinalDice {
			faces = append(faces, die.Face)
		}
		if !reflect.DeepEqual(faces, []int{5, 3, 5, 2, 6}) {
			t.Fatalf("reported roll changed: %v", faces)
		}
		if len(events) != 1 || !reflect.DeepEqual(events[0].RolledIndices, []int{0, 2, 4}) {
			t.Fatal("reroll event lost selected indices")
		}
	}
	if _, err := e.handleOffensivePlanningCommand(&b, lib, command.Command{ActorID: "player", Type: command.TypePlanningReroll, Payload: reroll}); err == nil {
		t.Fatal("exhausted reroll was accepted")
	}
	if b.Random.Cursor != 96 {
		t.Fatal("rejected reroll consumed randomness")
	}
}
