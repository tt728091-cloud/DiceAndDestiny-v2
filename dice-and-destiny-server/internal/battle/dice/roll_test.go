package dice_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/dice"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestRollUsesDeterministicSourceAndMapsFacesSymbolsAndCombinations(t *testing.T) {
	battle := battleWithRollRequest(5, 3)

	got, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(0, 1, 2, 3, 4)),
	)
	if err != nil {
		t.Fatalf("Roll() returned error: %v", err)
	}

	wantDice := []state.RolledDie{
		{Index: 0, DieID: "Symbol D6", Face: 1, Value: 1, Symbols: []string{"sword"}},
		{Index: 1, DieID: "Symbol D6", Face: 2, Value: 2, Symbols: []string{"sword", "shield"}},
		{Index: 2, DieID: "Symbol D6", Face: 3, Value: 3, Symbols: []string{}},
		{Index: 3, DieID: "Symbol D6", Face: 4, Value: 4, Symbols: []string{"focus"}},
		{Index: 4, DieID: "Symbol D6", Face: 5, Value: 5, Symbols: []string{"focus"}},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Dice.CurrentRoll.Dice, wantDice) {
		t.Fatalf("rolled dice = %#v, want %#v", battle.Actors["player"].Dice.CurrentRoll.Dice, wantDice)
	}

	wantCounts := map[string]int{"focus": 2, "shield": 1, "sword": 2}
	if !reflect.DeepEqual(battle.Actors["player"].Dice.CurrentRoll.SymbolCounts, wantCounts) {
		t.Fatalf("symbol counts = %#v, want %#v", battle.Actors["player"].Dice.CurrentRoll.SymbolCounts, wantCounts)
	}

	wantCombinations := []string{"large_straight", "small_straight"}
	if !reflect.DeepEqual(battle.Actors["player"].Dice.CurrentRoll.Combinations, wantCombinations) {
		t.Fatalf("combinations = %#v, want %#v", battle.Actors["player"].Dice.CurrentRoll.Combinations, wantCombinations)
	}

	wantEvents := []event.Event{
		event.NewDiceRolled(
			"player",
			segment.Offensive,
			"roll-player-test",
			state.RollPoolOffensive,
			state.RollSourceSegment,
			"offensive",
			wantDice,
			[]int{0, 1, 2, 3, 4},
			1,
			3,
			wantCombinations,
			wantCounts,
		),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("events = %#v, want %#v", got, wantEvents)
	}
}

func TestRollValuesStayWithinDefinedFaces(t *testing.T) {
	battle := battleWithRollRequest(3, 3)

	if _, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(99, 100, 101)),
	); err != nil {
		t.Fatalf("Roll() returned error: %v", err)
	}

	for _, rolled := range battle.Actors["player"].Dice.CurrentRoll.Dice {
		if rolled.Face < 1 || rolled.Face > 6 {
			t.Fatalf("rolled face = %d, want within 1..6", rolled.Face)
		}
		if rolled.Value < 1 || rolled.Value > 6 {
			t.Fatalf("rolled value = %d, want within 1..6", rolled.Value)
		}
	}
}

func TestRollRerollsAllNonKeptDiceByDefaultAfterFirstRoll(t *testing.T) {
	battle := battleWithRollRequest(3, 3)

	if _, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(0, 1, 2)),
	); err != nil {
		t.Fatalf("first Roll() returned error: %v", err)
	}

	actor := battle.Actors["player"]
	actor.Dice.CurrentRoll.KeptIndices = []int{1}
	battle.Actors["player"] = actor

	got, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(5, 4)),
	)
	if err != nil {
		t.Fatalf("second Roll() returned error: %v", err)
	}

	wantValues := []int{6, 2, 5}
	for i, want := range wantValues {
		if battle.Actors["player"].Dice.CurrentRoll.Dice[i].Value != want {
			t.Fatalf("die %d value = %d, want %d", i, battle.Actors["player"].Dice.CurrentRoll.Dice[i].Value, want)
		}
	}
	if !reflect.DeepEqual(got[0].RolledIndices, []int{0, 2}) {
		t.Fatalf("rolled indices = %#v, want [0 2]", got[0].RolledIndices)
	}
	if battle.Actors["player"].Dice.CurrentRoll.RollsUsed != 2 {
		t.Fatalf("rolls used = %d, want 2", battle.Actors["player"].Dice.CurrentRoll.RollsUsed)
	}
}

func TestRollEnforcesMaxRolls(t *testing.T) {
	battle := battleWithRollRequest(1, 1)

	if _, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(0)),
	); err != nil {
		t.Fatalf("first Roll() returned error: %v", err)
	}

	_, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(1)),
	)
	if err == nil {
		t.Fatal("second Roll() succeeded after max rolls exhausted")
	}
	if !errors.Is(err, dice.ErrInvalidRoll) {
		t.Fatalf("second Roll() error = %v, want ErrInvalidRoll", err)
	}
}

func TestRollRejectsRequestForDifferentCurrentSegment(t *testing.T) {
	battle := battleWithRollRequest(1, 1)
	battle.Segment = segment.State{Current: segment.Defensive, Round: 1}

	_, err := dice.Roll(
		&battle,
		"roll-player-test",
		"player",
		nil,
		dice.WithRandomSource(dice.NewSequenceRandomSource(0)),
	)
	if err == nil {
		t.Fatal("Roll() succeeded with request for a different current segment")
	}
	if !errors.Is(err, dice.ErrInvalidRoll) {
		t.Fatalf("Roll() error = %v, want ErrInvalidRoll", err)
	}
}

func battleWithRollRequest(count int, maxRolls int) state.Battle {
	return state.Battle{
		ID:      "battle-1",
		Segment: segment.State{Current: segment.Offensive, Round: 1},
		Actors: map[string]state.ActorState{
			"player": {
				DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "Symbol D6", Count: count}},
			},
		},
		DiceDefinitions: map[string]state.DiceDefinition{
			"Symbol D6": {
				ID:        "Symbol D6",
				Name:      "Symbol D6",
				DieType:   "d6",
				SideCount: 6,
				Faces: []state.DiceFace{
					{Face: 1, Value: 1, Symbols: []string{"sword"}},
					{Face: 2, Value: 2, Symbols: []string{"sword", "shield"}},
					{Face: 3, Value: 3, Symbols: []string{}},
					{Face: 4, Value: 4, Symbols: []string{"focus"}},
					{Face: 5, Value: 5, Symbols: []string{"focus"}},
					{Face: 6, Value: 6, Symbols: []string{"star"}},
				},
			},
		},
		RollRequests: map[string]state.RollRequest{
			"roll-player-test": {
				ID:          "roll-player-test",
				ActorID:     "player",
				Segment:     segment.Offensive,
				Pool:        state.RollPoolOffensive,
				SourceType:  state.RollSourceSegment,
				SourceID:    "offensive",
				DiceLoadout: []state.DiceLoadoutEntry{{DiceID: "Symbol D6", Count: count}},
				MaxRolls:    maxRolls,
			},
		},
	}
}
