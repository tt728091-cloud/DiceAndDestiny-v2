package engine

import (
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestIncomeEnergyStopsAtCap(t *testing.T) {
	for _, tc := range []struct{ cap, before, want, gain int }{
		{75, 60, 65, 5},
		{75, 73, 75, 2},
		{75, 75, 75, 0},
		// Battles saved before the cap have no maximum and stay uncapped.
		{0, 75, 80, 5},
	} {
		b, lib := adventurerFixture(t)
		for id, a := range b.Actors {
			a.Resources.MaxEnergyPoints = tc.cap
			a.Resources.EnergyPoints = tc.before
			a.EnergyPoints = tc.before
			b.Actors[id] = a
			r := b.Settled.Actors[id]
			r.IncomeCards = 0
			r.IncomeEnergy = 5
			b.Settled.Actors[id] = r
		}
		b.Segment.Current = segment.Income
		closeSettledWindow(&b)
		events, err := NewEngine().progressSettledIncome(&b, lib)
		if err != nil {
			t.Fatal(err)
		}
		if got := b.Actors["player"].Resources.EnergyPoints; got != tc.want || b.Actors["player"].EnergyPoints != tc.want {
			t.Fatalf("cap %d: %d + income = %d, want %d", tc.cap, tc.before, got, tc.want)
		}
		found := false
		for _, ev := range events {
			if ev.Type == event.TypeEnergyPointsGained && ev.ActorID == "player" {
				found = true
				if ev.Data["energy_gain"] != tc.gain || ev.EnergyPoints != tc.want {
					t.Fatalf("cap %d: income feedback %+v", tc.cap, ev)
				}
			}
		}
		if !found {
			t.Fatalf("cap %d: missing income feedback", tc.cap)
		}
	}
}

func TestCardEnergyGainStopsAtCap(t *testing.T) {
	b, lib := adventurerFixture(t)
	a := b.Actors["player"]
	a.Resources.MaxEnergyPoints = 75
	a.Resources.EnergyPoints = 70
	a.Cards = state.CardZones{Hand: []string{"second_wind-0"}}
	b.Actors["player"] = a
	if err := playProgramCard(NewEngine(), &b, lib, "player", "second_wind-0", nil); err != nil {
		t.Fatal(err)
	}
	if got := b.Actors["player"].Resources.EnergyPoints; got != 75 {
		t.Fatalf("Second Wind at 70 energy = %d, want the 75 cap", got)
	}
}
