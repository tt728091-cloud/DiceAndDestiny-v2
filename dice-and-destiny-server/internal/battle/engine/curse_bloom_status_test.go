package engine

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"testing"
)

func TestCurseBloomStatusLifecycle(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	cursePlay(t, e, &b, lib, "curse_bloom", "apply", "enemy")
	if stacks(&b, "enemy", "curse_bloom") != 1 || len(curseRuntime(&b).Preparations) != 0 {
		t.Fatal("must be a status, not preparation")
	}
	b = b.Clone()
	b.Segment.Round++
	if len(curseCardChoices(&b, lib, "player", lib.Cards["curse_bloom"])) != 0 {
		t.Fatal("duplicate active status allowed")
	}
	b.Segment.Current = segment.OngoingEffects
	expireCursePreparations(&b)
	if stacks(&b, "enemy", "curse_bloom") != 0 {
		t.Fatal("status did not expire")
	}
	found := false
	for _, ev := range flushCurseEvents(&b) {
		found = found || ev.Data["kind"] == "bloom_expired"
	}
	if !found {
		t.Fatal("expiry feedback missing")
	}
}

func TestCurseBloomStatusTrigger(t *testing.T) {
	for _, mode := range []string{"surge", "blocked", "released", "no_dice", "removed", "attack"} {
		t.Run(mode, func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, "curse_bloom", "apply", "enemy")
			if mode != "no_dice" {
				for i := 0; i < 5; i++ {
					for f := 1; f <= 6; f++ {
						markCurse(&b, "enemy", i, f)
					}
				}
			}
			if mode == "removed" {
				removeStatus(&b, "enemy", "curse_bloom", 0)
			}
			flushCurseEvents(&b)
			b = b.Clone()
			b.Segment.Round++
			b.Segment.Current = segment.OngoingEffects
			curseRuntime(&b).ConversionRound = b.Segment.Round
			kind := "curse_count"
			if mode == "attack" {
				kind = "brine_lash"
			}
			source := newSettledDamageSource(&b, "player", "enemy", kind, 1)
			batch := &state.SettledDamageBatch{Sources: []state.SettledDamageSource{source}, Removals: []state.ProposedCardRemoval{{Accepted: mode != "blocked", Released: mode == "released", DamageProposalIDs: []string{source.ID}}}}
			if err := e.curseDamageCompleted(&b, lib, batch); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "surge" {
				want = 3
			}
			if stacks(&b, "enemy", "curse_count") != want {
				t.Fatalf("wrong extra Count: want %d", want)
			}
			events := flushCurseEvents(&b)
			triggers := 0
			rolls := 0
			for _, ev := range events {
				if ev.Data["kind"] == "bloom_trigger" {
					triggers++
				}
				if ev.Data["kind"] == "owned_roll" {
					rolls++
					if ev.Data["source_card_id"] != "curse_bloom" || ev.Data["attack_source_id"] != source.ID {
						t.Fatal("roll lost status cause")
					}
				}
			}
			if mode == "removed" || mode == "attack" {
				if triggers != 0 {
					t.Fatal("unexpected trigger")
				}
			} else if triggers != 1 || stacks(&b, "enemy", "curse_bloom") != 0 {
				t.Fatal("must consume exactly once")
			}
			if rolls != want {
				t.Fatal("wrong expansion count")
			}
			if err := e.curseDamageCompleted(&b, lib, batch); err != nil {
				t.Fatal(err)
			}
			if stacks(&b, "enemy", "curse_count") != want {
				t.Fatal("status triggered twice")
			}
			if curseRuntime(&b).ConversionRound != b.Segment.Round {
				t.Fatal("new Count converted in same Effects")
			}
		})
	}
}

func TestCurseBloomOldCatalog(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	delete(lib.Statuses, "curse_bloom")
	cursePlay(t, NewEngine(), &b, lib, "curse_bloom", "apply", "enemy")
	if stacks(&b, "enemy", "curse_bloom") != 0 || len(curseRuntime(&b).Preparations) != 1 {
		t.Fatal("legacy catalog compatibility")
	}
}
