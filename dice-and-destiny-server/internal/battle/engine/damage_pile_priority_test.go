package engine

import (
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"fmt"
	"reflect"
	"testing"
)

func TestDamagePriorityForPlayersAndEnemies(t *testing.T) {
	for _, target := range []string{"player", "enemy"} {
		for amount := 0; amount <= 7; amount++ {
			t.Run(fmt.Sprintf("%s/%d", target, amount), func(t *testing.T) {
				b, lib := adventurerFixture(t)
				for id, a := range b.Actors {
					a.Cards = state.CardZones{Discard: []string{id + "-discard-1", id + "-discard-2"}, Deck: []string{id + "-deck-1", id + "-deck-2"}, Hand: []string{id + "-hand-1", id + "-hand-2"}, Removed: []string{id + "-old-loss"}}
					b.Actors[id] = a
				}
				b.Segment.Current = segment.DamageResolution
				b.Settled.Stage = stageDamageReact
				before := b.Clone()
				e := NewEngine()
				source := "enemy"
				if target == "enemy" {
					source = "player"
				}
				batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: source, TargetActorID: target, BaseAmount: amount}})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before.Actors, b.Actors) {
					t.Fatal("reveal moved cards before commit")
				}
				wantZones := []operation.CardZone{operation.ZoneDiscard, operation.ZoneDiscard, operation.ZoneDeck, operation.ZoneDeck, operation.ZoneHand, operation.ZoneHand}
				if len(batch.Removals) != min(amount, 6) || batch.Overage[target] != max(0, amount-6) {
					t.Fatalf("wrong damage count/overage: %+v", batch)
				}
				seen := map[string]bool{}
				for i, r := range batch.Removals {
					if r.OriginalZone != wantZones[i] || seen[r.CardID] || r.TargetActorID != target {
						t.Fatalf("priority or duplicate selection at %d: %+v", i, r)
					}
					seen[r.CardID] = true
				}
				b.Settled.PendingDamage = batch
				if _, err = e.finishDamageBatch(&b, lib); err != nil {
					t.Fatal(err)
				}
				a := b.Actors[target]
				if a.CurrentHealth() != max(0, 6-amount) || len(a.Cards.Discard) != max(0, 2-amount) || len(a.Cards.Deck) != 2-min(2, max(0, amount-2)) || len(a.Cards.Hand) != 2-min(2, max(0, amount-4)) || len(a.Cards.Removed) != 1+min(6, amount) {
					t.Fatalf("wrong committed piles: %+v", a.Cards)
				}
			})
		}
	}
}
