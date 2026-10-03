package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestBraceDiscardsSavedCardsFromEveryZone(t *testing.T) {
	for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneHand, operation.ZoneDiscard} {
		t.Run(string(zone), func(t *testing.T) {
			b, lib := adventurerFixture(t)
			a := b.Actors["player"]
			a.Cards = state.CardZones{Hand: []string{"brace-0"}, Removed: []string{"old-loss"}}
			setZone(&a.Cards, zone, append(zoneCards(a.Cards, zone), "nudge-0", "nudge-1", "strong_swing-0", "strong_swing-1"))
			b.Actors["player"] = a
			b.Segment.Current = segment.DamageResolution
			b.Settled.Stage = stageDamageReact
			batch := &state.SettledDamageBatch{ID: "saved", Sources: []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 4, FinalAmount: 4}}}
			for i, id := range []string{"nudge-0", "nudge-1", "strong_swing-0", "strong_swing-1"} {
				batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: id, CardID: id, TargetActorID: "player", OriginalZone: zone, Accepted: true, Revealed: true, Sequence: i + 1})
			}
			b.Settled.PendingDamage = batch
			openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
			e := NewEngine()
			if err := e.playSettledCard(&b, lib, "player", "brace-0", []string{"hit"}, "", 0, ""); err != nil {
				t.Fatal(err)
			}
			if b.Actors["player"].CurrentHealth() != 5 || batch.Sources[0].FinalAmount != 1 {
				t.Fatal("prevention changed health or wrong damage")
			}
			for _, p := range batch.Removals[1:] {
				if !p.Released || p.Accepted || p.ReleasedDestination != operation.ZoneDiscard || !containsString(b.Actors["player"].Cards.Discard, p.CardID) {
					t.Fatalf("saved card not discarded: %+v", p)
				}
			}
			// Persisted released proposals must not duplicate cards on reconciliation.
			raw, err := json.Marshal(b)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &b); err != nil {
				t.Fatal(err)
			}
			before := b.Actors["player"].Cards
			reconcileSettledDamage(b.Settled.PendingDamage, &b)
			if !reflect.DeepEqual(before, b.Actors["player"].Cards) {
				t.Fatal("repeated reconciliation moved cards twice")
			}
			if _, err := e.finishDamageBatch(&b, lib); err != nil {
				t.Fatal(err)
			}
			a = b.Actors["player"]
			if a.CurrentHealth() != 4 || len(a.Cards.Discard) != 4 || len(a.Cards.Deck) != 0 || len(a.Cards.Hand) != 0 || !reflect.DeepEqual(a.Cards.Removed, []string{"old-loss", "nudge-0"}) {
				t.Fatalf("commit must remove only unsaved damage: %+v", a.Cards)
			}
		})
	}
}

func TestProtectDiscardsOnlyActualSavedCardsAfterOverage(t *testing.T) {
	b, lib := adventurerFixture(t)
	a := b.Actors["player"]
	a.Cards = state.CardZones{Deck: []string{"nudge-0", "nudge-1"}}
	b.Actors["player"] = a
	b.Segment.Current = segment.DamageResolution
	b.Settled.Stage = stageDamageReact
	applyStatus(&b, lib, "player", "protect", 2)
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "sword_cut", BaseAmount: 3}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if _, err := e.spendRoundPrevention(&b, lib, "player", command.InteractionCommitmentData{ChoiceID: "spend_round_prevention", ProposalIDs: []string{"hit"}}); err != nil {
		t.Fatal(err)
	}
	a = b.Actors["player"]
	if len(a.Cards.Discard) != 1 || len(a.Cards.Deck) != 1 || a.CurrentHealth() != 2 || stacks(&b, "player", "protect") != 0 {
		t.Fatalf("one overage plus one saved card: %+v", a)
	}
	// If damage rises again, the same card is removed from its current discard zone.
	batch.Sources[0].ReactionPrevention = 0
	reconcileSettledDamage(batch, &b)
	if _, err := e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Actors["player"].CurrentHealth() != 0 || len(b.Actors["player"].Cards.Removed) != 2 {
		t.Fatal("reaccepted card did not follow live zone")
	}
}

func TestBraceCanSaveItselfWithoutDuplicatingOrRedrawing(t *testing.T) {
	b, lib := adventurerFixture(t)
	a := b.Actors["player"]
	a.Cards = state.CardZones{Hand: []string{"brace-0", "nudge-0"}}
	b.Actors["player"] = a
	b.Segment.Current = segment.DamageResolution
	b.Settled.Stage = stageDamageReact
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 2}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(&b, "damage", stageDamageReact, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if err := e.playSettledCard(&b, lib, "player", "brace-0", []string{"hit"}, "", 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	a = b.Actors["player"]
	if a.CurrentHealth() != 2 || (len(a.Cards.Discard) != 2 || !containsString(a.Cards.Discard, "brace-0") || !containsString(a.Cards.Discard, "nudge-0")) || len(a.Cards.Hand) != 0 || len(a.Cards.Removed) != 0 {
		t.Fatalf("fully prevented cards must survive exactly once in discard: %+v", a.Cards)
	}
	if drawn, err := e.drawSettledCard(&b, "player", "card_draw"); err != nil || drawn != "" {
		t.Fatalf("saved cards were redrawn: %q, %v", drawn, err)
	}
}
