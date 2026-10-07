package engine

import (
	"fmt"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestTakeStockNeverRedrawsDiscardIncludingItself(t *testing.T) {
	for remaining := 0; remaining <= 2; remaining++ {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			b, lib := adventurerFixture(t)
			a := b.Actors["player"]
			a.Cards = state.CardZones{Hand: []string{"take_stock-0"}, Discard: []string{"second_wind-0"}, Removed: []string{"nudge-0"}}
			for i := 0; i < remaining; i++ {
				a.Cards.Deck = append(a.Cards.Deck, fmt.Sprintf("brace-%d", i))
			}
			b.Actors["player"] = a
			health := a.CurrentHealth()
			e := NewEngine()
			if err := playProgramCard(e, &b, lib, "player", "take_stock-0", nil); err != nil {
				t.Fatal(err)
			}
			a = b.Actors["player"]
			if len(a.Cards.Hand) != remaining || len(a.Cards.Deck) != 0 || !reflect.DeepEqual(a.Cards.Discard, []string{"second_wind-0", "take_stock-0"}) || !reflect.DeepEqual(a.Cards.Removed, []string{"nudge-0"}) || a.CurrentHealth() != health || a.Resources.EnergyPoints != 9 {
				t.Fatalf("Take Stock must draw only available cards, discard itself, and preserve health: %+v", a)
			}
			// Exhausted draws survive checkpoint cloning and never request randomness.
			b = b.Clone()
			e.namedRandom = &ownedSelectionScript{}
			for i := 0; i < 3; i++ {
				drawn, err := e.drawSettledCard(&b, "player", "card_draw")
				if err != nil || drawn != "" {
					t.Fatalf("empty draw = %q, %v", drawn, err)
				}
			}
			if !reflect.DeepEqual(b.Actors["player"], a) {
				t.Fatal("empty draws mutated actor")
			}
		})
	}
}

func TestSettledIncomeWithDiscardOnlyPreservesHealthAndAdvances(t *testing.T) {
	b, lib := adventurerFixture(t)
	for id, a := range b.Actors {
		a.Cards = state.CardZones{Discard: []string{id + "-spent"}}
		a.Resources.EnergyPoints = 0
		a.EnergyPoints = 0
		b.Actors[id] = a
		r := b.Settled.Actors[id]
		r.IncomeCards = 1
		r.IncomeEnergy = 1
		b.Settled.Actors[id] = r
	}
	b.Segment.Current = segment.Income
	closeSettledWindow(&b)
	e := NewEngine()
	e.namedRandom = &ownedSelectionScript{}
	events, err := e.progressSettledIncome(&b, lib)
	if err != nil {
		t.Fatal(err)
	}
	if b.Segment.Current != segment.Offensive {
		t.Fatalf("income stalled at %s", b.Segment.Current)
	}
	for id, a := range b.Actors {
		if len(a.Cards.Deck) != 0 || len(a.Cards.Hand) != 0 || !reflect.DeepEqual(a.Cards.Discard, []string{id + "-spent"}) || a.CurrentHealth() != 1 || a.Resources.EnergyPoints != 1 {
			t.Fatalf("%s income: %+v", id, a)
		}
		found := false
		for _, ev := range events {
			if ev.Type == event.TypeCardsDrawn && ev.ActorID == id {
				if !reflect.DeepEqual(ev, event.NewCardsDrawn(id, nil, true)) {
					t.Fatalf("empty draw event: %+v", ev)
				}
				found = true
			}
			if ev.Type == event.TypeDiscardReshuffled {
				t.Fatal("ordinary income reshuffled")
			}
		}
		if !found {
			t.Fatalf("no shortage event for %s", id)
		}
	}
	// A discard-only actor is alive and its discarded card can still absorb damage.
	e = NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 1, FinalAmount: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Removals) != 1 || batch.Removals[0].OriginalZone != operation.ZoneDiscard || batch.Removals[0].CardID != "player-spent" {
		t.Fatalf("damage must still select discard: %+v", batch.Removals)
	}
}
