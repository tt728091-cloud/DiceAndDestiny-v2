package income_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/income"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestNewRewardsCopiesValidIncomeRewards(t *testing.T) {
	rewards := []income.Reward{
		{ActorID: "player", DrawCards: 1, EnergyPoints: 2},
	}

	got, err := income.NewRewards(rewards...)
	if err != nil {
		t.Fatalf("NewRewards() returned error: %v", err)
	}

	rewards[0] = income.Reward{ActorID: "mutated", DrawCards: 99}

	want := []income.Reward{
		{ActorID: "player", DrawCards: 1, EnergyPoints: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewRewards() = %#v, want %#v", got, want)
	}
}

func TestNewRewardsRejectsInvalidRewardConfig(t *testing.T) {
	tests := []struct {
		name   string
		reward income.Reward
		want   string
	}{
		{
			name:   "missing actor",
			reward: income.Reward{DrawCards: 1},
			want:   "actor id is required",
		},
		{
			name:   "negative draw",
			reward: income.Reward{ActorID: "player", DrawCards: -1},
			want:   "draw cards must be non-negative",
		},
		{
			name:   "negative energy",
			reward: income.Reward{ActorID: "player", EnergyPoints: -1},
			want:   "energy points must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := income.NewRewards(tt.reward)
			if err == nil {
				t.Fatal("NewRewards() succeeded with invalid reward")
			}

			if !errors.Is(err, income.ErrInvalidReward) {
				t.Fatalf("NewRewards() error = %v, want ErrInvalidReward", err)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewRewards() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestApplyRewardsDrawsCardsAndAddsEnergyPoints(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck: []string{"strike", "guard"},
	})

	got, err := income.ApplyRewards(&battle, []income.Reward{
		{ActorID: "player", DrawCards: 1, EnergyPoints: 2},
	})
	if err != nil {
		t.Fatalf("ApplyRewards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewCardsDrawn("player", []string{"strike"}, false),
		event.NewEnergyPointsGained("player", 2),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("ApplyRewards() events = %#v, want %#v", got, wantEvents)
	}

	wantActor := state.ActorState{
		Cards: state.CardZones{
			Deck: []string{"guard"},
			Hand: []string{"strike"},
		},
		EnergyPoints: 2,
	}
	if !reflect.DeepEqual(battle.Actors["player"], wantActor) {
		t.Fatalf("player state = %#v, want %#v", battle.Actors["player"], wantActor)
	}
}

func TestApplyRewardsAllowsDrawZeroWithoutCardDraw(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck: []string{"strike", "guard"},
	})

	got, err := income.ApplyRewards(&battle, []income.Reward{
		{ActorID: "player", DrawCards: 0},
	})
	if err != nil {
		t.Fatalf("ApplyRewards() returned error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("ApplyRewards() events = %#v, want none", got)
	}

	wantCards := state.CardZones{
		Deck: []string{"strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantCards) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantCards)
	}
}

func TestApplyRewardsAllowsEnergyOnlyReward(t *testing.T) {
	battle := battleWithPlayerCards(state.CardZones{
		Deck: []string{"strike", "guard"},
	})

	got, err := income.ApplyRewards(&battle, []income.Reward{
		{ActorID: "player", EnergyPoints: 2},
	})
	if err != nil {
		t.Fatalf("ApplyRewards() returned error: %v", err)
	}

	wantEvents := []event.Event{
		event.NewEnergyPointsGained("player", 2),
	}
	if !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("ApplyRewards() events = %#v, want %#v", got, wantEvents)
	}

	if battle.Actors["player"].EnergyPoints != 2 {
		t.Fatalf("energy points = %d, want 2", battle.Actors["player"].EnergyPoints)
	}

	wantCards := state.CardZones{
		Deck: []string{"strike", "guard"},
	}
	if !reflect.DeepEqual(battle.Actors["player"].Cards, wantCards) {
		t.Fatalf("player card zones = %#v, want %#v", battle.Actors["player"].Cards, wantCards)
	}
}

func battleWithPlayerCards(cards state.CardZones) state.Battle {
	return state.Battle{
		ID:      "battle-1",
		Segment: segment.NewManager().InitialState(),
		Actors: map[string]state.ActorState{
			"player": {
				Cards: cards,
			},
		},
	}
}
