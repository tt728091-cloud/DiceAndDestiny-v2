package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/snapshot"
	"diceanddestiny/server/internal/battle/state"
)

func TestCommittedWoundsExcludeDefenseAndCardPrevention(t *testing.T) {
	b, lib := adventurerFixture(t)
	b.Settled.UnifiedDefense = true
	b.Segment.Current = segment.Defensive
	b.Settled.Stage = stageDefenseSelect
	a := b.Actors["player"]
	a.Cards = state.CardZones{Hand: []string{"guard_bulwark-0"}, Discard: []string{"nudge-0", "nudge-1", "strong_swing-0", "strong_swing-1", "take_stock-0", "take_stock-1", "second_wind-0"}}
	b.Actors["player"] = a
	e := NewEngine()
	// Seven incoming, two already blocked by defense, then three by Bulwark.
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", SourceActorID: "enemy", SourceContentID: "brine_surge", TargetActorID: "player", BaseAmount: 7, Prevention: 2}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	openSettledWindow(&b, "damage", stageDefenseSelect, "damage_response", []command.Type{command.TypeCommitInteraction, command.TypePass})
	if err := playProgramCard(e, &b, lib, "player", "guard_bulwark-0", func(c programChoice) bool { return c.Source == "hit" }); err != nil {
		t.Fatal(err)
	}
	if len(b.Wounds) != 0 {
		t.Fatal("preview/prevention recorded a wound before commit")
	}
	if _, err := e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if len(b.Wounds) != 1 || len(b.Wounds[0].Cards) != 2 {
		t.Fatalf("expected one two-card wound: %+v", b.Wounds)
	}
	w := b.Wounds[0]
	if w.SourceID != "hit" || w.SourceActorID != "enemy" || w.SourceContentID != "brine_surge" || w.TargetActorID != "player" {
		t.Fatalf("lost source identity: %+v", w)
	}
	for _, c := range w.Cards {
		if !containsString(b.Actors["player"].Cards.Removed, c.CardID) || c.CardID == "guard_bulwark-0" || c.CardDefinitionID == "" {
			t.Fatalf("incorrect lost card: %+v", c)
		}
	}
	// Reconciliation/review/reload cannot duplicate wounds or mutate the live ledger.
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var restored state.Battle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Wounds, restored.Wounds) {
		t.Fatal("wounds did not survive persistence")
	}
	restored.RecordDamageWounds(batch.ID, batch.Sources, batch.Removals)
	if len(restored.Wounds) != 1 {
		t.Fatal("duplicate batch recorded")
	}
	clone := restored.Clone()
	clone.Wounds[0].Cards[0].CardID = "changed"
	if reflect.DeepEqual(clone.Wounds, restored.Wounds) {
		t.Fatal("clone shares wound cards")
	}
	restored.Status = state.BattleActive
	if !reflect.DeepEqual(snapshot.FromBattleForViewer(restored, "player").Wounds, restored.Wounds) {
		t.Fatal("committed wounds missing during ongoing battle")
	}
	restored.Status = state.BattleStatus("draw")
	view := snapshot.FromBattleForViewer(restored, "player")
	if !reflect.DeepEqual(view.Wounds, restored.Wounds) {
		t.Fatal("terminal snapshot omitted wounds")
	}
	view.Wounds[0].Cards[0].CardID = "changed"
	if restored.Wounds[0].Cards[0].CardID == "changed" {
		t.Fatal("snapshot shares ledger")
	}
}

func TestWoundsSeparateSourcesRoundsAndOverkill(t *testing.T) {
	for _, segmentID := range []segment.Segment{segment.Defensive, segment.OngoingEffects} {
		t.Run(string(segmentID), func(t *testing.T) {
			b, lib := adventurerFixture(t)
			b.Settled.UnifiedDefense = true
			b.Segment.Current = segmentID
			a := b.Actors["player"]
			a.Cards = state.CardZones{Discard: []string{"nudge-0", "nudge-1"}, Deck: []string{"strong_swing-0", "strong_swing-1", "take_stock-0", "take_stock-1"}, Hand: []string{"guard_bulwark-0"}}
			b.Actors["player"] = a
			e := NewEngine()
			first, err := e.buildDamageBatch(&b, []state.SettledDamageSource{
				{ID: "one", SourceActorID: "enemy", SourceContentID: "poison", TargetActorID: "player", BaseAmount: 2},
				{ID: "blocked", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 3, Prevention: 3},
				{ID: "two", SourceActorID: "enemy", SourceContentID: "volatile_poison", TargetActorID: "player", BaseAmount: 2},
			})
			if err != nil {
				t.Fatal(err)
			}
			b.Settled.PendingDamage = first
			if _, err = e.finishDamageBatch(&b, lib); err != nil {
				t.Fatal(err)
			}
			if len(b.Wounds) != 2 || len(b.Wounds[0].Cards) != 2 || len(b.Wounds[1].Cards) != 2 || b.Wounds[0].SourceID != "one" || b.Wounds[1].SourceID != "two" {
				t.Fatalf("hits pooled or fully prevented hit recorded: %+v", b.Wounds)
			}
			b.Segment.Current = segmentID
			b.Segment.Round++
			second, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "one", SourceActorID: "enemy", TargetActorID: "player", BaseAmount: 99}})
			if err != nil {
				t.Fatal(err)
			}
			b.Settled.PendingDamage = second
			if _, err = e.finishDamageBatch(&b, lib); err != nil {
				t.Fatal(err)
			}
			if len(b.Wounds) != 3 || len(b.Wounds[2].Cards) != 3 || b.Wounds[2].Round <= b.Wounds[0].Round {
				t.Fatalf("overkill or repeat-hit grouping wrong: %+v", b.Wounds)
			}
			seen := map[string]bool{}
			for _, w := range b.Wounds {
				for _, c := range w.Cards {
					if seen[c.CardID] {
						t.Fatal("card counted twice")
					}
					seen[c.CardID] = true
				}
			}
			if len(seen) != len(b.Actors["player"].Cards.Removed) {
				t.Fatal("lost-card totals differ")
			}
		})
	}
}

func TestWoundCommitFollowsPlayedCardZoneAndRejectsInvalidBatch(t *testing.T) {
	b, lib := adventurerFixture(t)
	b.Segment.Current = segment.Defensive
	b.Settled.UnifiedDefense = true
	a := b.Actors["player"]
	a.Cards = state.CardZones{Hand: []string{"nudge-0"}, Deck: []string{"nudge-1"}}
	b.Actors["player"] = a
	e := NewEngine()
	batch, err := e.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "hit", TargetActorID: "player", BaseAmount: 1}})
	if err != nil {
		t.Fatal(err)
	}
	b.Settled.PendingDamage = batch
	id := batch.Removals[0].CardID
	a = b.Actors["player"]
	moveCard(&a.Cards, id, batch.Removals[0].OriginalZone, operation.ZoneDiscard)
	b.Actors["player"] = a
	if _, err = e.finishDamageBatch(&b, lib); err != nil {
		t.Fatal(err)
	}
	if b.Wounds[0].Cards[0].OriginalZone != operation.ZoneDiscard {
		t.Fatal("recorded stale zone")
	}
	b.Settled.PendingDamage = &state.SettledDamageBatch{ID: "invalid", Removals: []state.ProposedCardRemoval{{Accepted: true, CardID: "missing", TargetActorID: "player"}}}
	if _, err = e.finishDamageBatch(&b, lib); err == nil {
		t.Fatal("missing-card commit accepted")
	}
	if len(b.Wounds) != 1 {
		t.Fatal("failed commit recorded a wound")
	}
}
