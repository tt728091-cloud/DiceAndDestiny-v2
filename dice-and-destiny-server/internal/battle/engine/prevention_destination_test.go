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

func TestUpgradedBraceRetainsSavedCardsFromEveryZone(t *testing.T) {
	for _, zone := range []operation.CardZone{operation.ZoneDeck, operation.ZoneHand, operation.ZoneDiscard} {
		t.Run(string(zone), func(t *testing.T) {
			b, lib := adventurerFixture(t)
			a := b.Actors["player"]
			a.Cards = state.CardZones{Hand: []string{"brace_plus-0"}, Removed: []string{"old-loss"}}
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
			if err := playProgramCard(e, &b, lib, "player", "brace_plus-0", func(c programChoice) bool { return c.Source == "hit" }); err != nil {
				t.Fatal(err)
			}
			if b.Actors["player"].CurrentHealth() != 5 || batch.Sources[0].FinalAmount != 1 {
				t.Fatal("prevention changed health or wrong damage")
			}
			for _, p := range batch.Removals[1:] {
				if !p.Released || p.Accepted || p.ReleasedDestination != zone || !containsString(zoneCards(b.Actors["player"].Cards, zone), p.CardID) {
					t.Fatalf("saved card changed pile: %+v", p)
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
			savedPileCount := 3
			if zone == operation.ZoneDiscard {
				savedPileCount++ // Includes the played Brace.
			}
			if a.CurrentHealth() != 4 || !containsString(a.Cards.Discard, "brace_plus-0") || len(zoneCards(a.Cards, zone)) != savedPileCount || !reflect.DeepEqual(a.Cards.Removed, []string{"old-loss", "nudge-0"}) {
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

func TestUpgradedBraceCanSaveItselfWithoutDuplicatingOrRedrawing(t *testing.T) {
	b, lib := adventurerFixture(t)
	a := b.Actors["player"]
	a.Cards = state.CardZones{Hand: []string{"brace_plus-0", "nudge-0"}}
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
	if err := playProgramCard(e, &b, lib, "player", "brace_plus-0", func(c programChoice) bool { return c.Source == "hit" }); err != nil {
		t.Fatal(err)
	}
	for _, r := range batch.Removals {
		want := operation.ZoneHand
		if r.CardID == "brace_plus-0" {
			want = operation.ZoneDiscard
		}
		if !r.Released || r.ReleasedDestination != want {
			t.Fatalf("saved-card feedback must follow the play destination: %+v", r)
		}
	}
	if _, err := e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	a = b.Actors["player"]
	if a.CurrentHealth() != 2 || !reflect.DeepEqual(a.Cards.Discard, []string{"brace_plus-0"}) || !reflect.DeepEqual(a.Cards.Hand, []string{"nudge-0"}) || len(a.Cards.Removed) != 0 {
		t.Fatalf("played Brace must be discarded and saved Nudge must remain in hand: %+v", a.Cards)
	}
	if drawn, err := e.drawSettledCard(&b, "player", "card_draw"); err != nil || drawn != "" {
		t.Fatalf("saved cards were redrawn: %q, %v", drawn, err)
	}
}

func TestUnifiedUpgradedBraceRetainsPilesAndDoesNotUndoItsPlay(t *testing.T) {
	for _, separateSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "saves_itself", true: "protects_other_source"}[separateSource], func(t *testing.T) {
			b, lib := adventurerFixture(t)
			b.Settled.UnifiedDefense = true
			b.Segment.Current = segment.Defensive
			b.Settled.Stage = stageDefenseSelect
			a := b.Actors["player"]
			a.Cards = state.CardZones{Hand: []string{"brace_plus-0", "nudge-0"}, Deck: []string{"nudge-1"}, Discard: []string{"strong_swing-0"}}
			b.Actors["player"] = a
			batch := &state.SettledDamageBatch{ID: "mixed", Sources: []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 4}}}
			for _, zone := range []operation.CardZone{operation.ZoneDiscard, operation.ZoneDeck, operation.ZoneHand} {
				for _, id := range zoneCards(a.Cards, zone) {
					source := "hit"
					if separateSource && id == "brace_plus-0" {
						source = "other"
					}
					batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: id, CardID: id, TargetActorID: "player", OriginalZone: zone, Accepted: true, Revealed: true, DamageProposalIDs: []string{source}})
				}
			}
			if separateSource {
				batch.Sources[0].BaseAmount = 3
				batch.Sources = append(batch.Sources, state.SettledDamageSource{ID: "other", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 1})
			}
			b.Settled.PendingDamage = batch
			openSettledWindow(&b, "defense", stageDefenseSelect, "defense_selection", []command.Type{command.TypeCommitInteraction, command.TypePass})
			e := NewEngine()
			if err := playProgramCard(e, &b, lib, "player", "brace_plus-0", func(c programChoice) bool { return c.Source == "hit" }); err != nil {
				t.Fatal(err)
			}
			for _, r := range batch.Removals {
				if r.Released && r.ReleasedDestination != currentRemovalZone(&b, r) {
					t.Fatalf("incorrect saved-card destination: %+v", r)
				}
				if separateSource && r.CardID == "brace_plus-0" && (!r.Accepted || r.Released) {
					t.Fatal("protecting one source also protected Brace from another source")
				}
			}
			expected := state.CardZones{Hand: []string{"nudge-0"}, Deck: []string{"nudge-1"}, Discard: []string{"strong_swing-0", "brace_plus-0"}}
			if !reflect.DeepEqual(b.Actors["player"].Cards, expected) {
				t.Fatalf("prevention moved cards or undid Brace play: %+v", b.Actors["player"].Cards)
			}
			if err := e.reconcileUnifiedDamage(&b, true); err != nil {
				t.Fatal(err)
			}
			if _, err := e.finishDamageBatch(&b, lib); err != nil {
				t.Fatal(err)
			}
			a = b.Actors["player"]
			lost := "strong_swing-0"
			if separateSource {
				lost = "brace_plus-0"
			}
			if a.CurrentHealth() != 3 || !reflect.DeepEqual(a.Cards.Removed, []string{lost}) || !reflect.DeepEqual(a.Cards.Hand, []string{"nudge-0"}) || !reflect.DeepEqual(a.Cards.Deck, []string{"nudge-1"}) {
				t.Fatalf("only unprotected source cards may be removed: %+v", a.Cards)
			}
		})
	}
}
