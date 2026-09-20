package engine

import (
	"testing"

	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/event"
	"diceanddestiny/server/internal/battle/operation"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
)

func TestDamageRemovesMarkedReactionCardFromItsCurrentZone(t *testing.T) {
	b, lib := venomFixture(t)
	eng := NewEngine()
	b.Segment.Current = segment.DamageResolution
	b.Settled.Stage = stageDamageReact
	actor := b.Actors["player"]
	actor.Cards = state.CardZones{Hand: []string{"molt", "shock", "reserve", "incubate"}}
	actor.Resources.EnergyPoints = 2
	b.Actors["player"] = actor
	runtime := b.Settled.Actors["player"]
	runtime.HandLimit = 5
	runtime.CardInstances["molt"] = state.CardInstance{InstanceID: "molt", DefinitionID: "emergency_molt"}
	b.Settled.Actors["player"] = runtime
	enemy := b.Actors["enemy"]
	enemy.Cards.Deck = []string{"enemy-card"}
	b.Actors["enemy"] = enemy
	source := state.SettledDamageSource{ID: "sword-cut", SourceActorID: "enemy", TargetActorID: "player", SourceContentID: "sword_cut", BaseAmount: 6, Prevention: 3, FinalAmount: 3}
	batch := &state.SettledDamageBatch{ID: "combat", Sources: []state.SettledDamageSource{source}, Overage: map[string]int{}}
	for i, id := range []string{"molt", "shock", "reserve"} {
		batch.Removals = append(batch.Removals, state.ProposedCardRemoval{ID: id, TargetActorID: "player", CardID: id, OriginalZone: operation.ZoneHand, Accepted: true, Sequence: i + 1})
	}
	b.Settled.PendingDamage = batch
	choices := venomCardChoices(&b, lib, "player", lib.Cards["emergency_molt"])
	if len(choices) == 0 {
		t.Fatal("Molt choice missing")
	}
	if err := eng.playSettledReactionCard(&b, lib, "player", command.InteractionCommitmentData{CardIDs: []string{"molt"}, ProposalIDs: []string{"sword-cut"}, ChoiceID: choices[0].Key}); err != nil {
		t.Fatal(err)
	}
	if b.Actors["player"].CurrentHealth() != 4 || !containsString(b.Actors["player"].Cards.Discard, "molt") {
		t.Fatal("playing the marked card should only move it to discard")
	}
	if batch.Sources[0].FinalAmount != 1 || !batch.Removals[1].Released || !batch.Removals[2].Released {
		t.Fatalf("Molt did not reduce damage from 3 to 1: %+v", batch)
	}
	events, err := eng.finishDamageBatch(&b, lib)
	if err != nil {
		t.Fatal(err)
	}
	player := b.Actors["player"]
	if player.CurrentHealth() != 3 || len(player.Cards.Removed) != 1 || player.Cards.Removed[0] != "molt" || len(player.Cards.Discard) != 0 || len(player.Cards.Hand) != 3 {
		t.Fatalf("one damage must remove the marked card exactly once from discard: %+v (health %d)", player.Cards, player.CurrentHealth())
	}
	if batch.Removals[0].OriginalZone != operation.ZoneDiscard {
		t.Fatal("committed removal must identify the actual zone for client counters")
	}
	removedEvents := 0
	for _, emitted := range events {
		if emitted.Type == event.TypeCardsPermanentlyRemoved {
			removedEvents++
		}
	}
	if removedEvents != 1 {
		t.Fatalf("removal events=%d, want 1", removedEvents)
	}
	// The next round's three Bleed cannot select Molt a second time.
	next, err := eng.buildDamageBatch(&b, []state.SettledDamageSource{{ID: "bleed", SourceActorID: "player", TargetActorID: "player", SourceContentID: "bleed", BaseAmount: 3, FinalAmount: 3}})
	if err != nil {
		t.Fatal(err)
	}
	for _, removal := range next.Removals {
		if removal.CardID == "molt" {
			t.Fatal("removed reaction card was selected for damage twice")
		}
	}
}
