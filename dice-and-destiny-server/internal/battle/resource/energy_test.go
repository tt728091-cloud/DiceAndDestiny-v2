package resource_test

import (
	"errors"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/resource"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestAddEnergyPointsUpdatesActorStateAndReturnsEvent(t *testing.T) {
	battle := battleWithPlayerEnergy(2)

	got, err := resource.AddEnergyPoints(&battle, "player", 3)
	if err != nil {
		t.Fatalf("AddEnergyPoints() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewEnergyPointsGained("player", 3),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("AddEnergyPoints() events = %#v, want %#v", got, wantEvents)
	}

	if battle.Actors["player"].EnergyPoints != 5 {
		t.Fatalf("energy points = %d, want 5", battle.Actors["player"].EnergyPoints)
	}
}

func TestAddEnergyPointsAllowsZeroWithoutEvent(t *testing.T) {
	battle := battleWithPlayerEnergy(2)

	got, err := resource.AddEnergyPoints(&battle, "player", 0)
	if err != nil {
		t.Fatalf("AddEnergyPoints() returned error: %v", err)
	}

	if got != nil {
		t.Fatalf("AddEnergyPoints() events = %#v, want nil", got)
	}

	if battle.Actors["player"].EnergyPoints != 2 {
		t.Fatalf("energy points = %d, want 2", battle.Actors["player"].EnergyPoints)
	}
}

func TestAddEnergyPointsRejectsNegativePoints(t *testing.T) {
	battle := battleWithPlayerEnergy(0)

	_, err := resource.AddEnergyPoints(&battle, "player", -1)
	if err == nil {
		t.Fatal("AddEnergyPoints() succeeded with negative points")
	}

	if !errors.Is(err, resource.ErrInvalidEnergyPoints) {
		t.Fatalf("AddEnergyPoints() error = %v, want ErrInvalidEnergyPoints", err)
	}
}

func TestAddEnergyPointsRejectsMissingActorState(t *testing.T) {
	battle := state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors:  map[string]state.ActorState{},
	}

	_, err := resource.AddEnergyPoints(&battle, "player", 1)
	if err == nil {
		t.Fatal("AddEnergyPoints() succeeded with missing actor state")
	}

	if !errors.Is(err, resource.ErrMissingActorState) {
		t.Fatalf("AddEnergyPoints() error = %v, want ErrMissingActorState", err)
	}
}

func battleWithPlayerEnergy(points int) state.Battle {
	return state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				EnergyPoints: points,
			},
		},
	}
}
