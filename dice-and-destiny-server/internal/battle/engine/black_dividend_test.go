package engine

import (
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"testing"
)

func TestBlackDividendStatusRewards(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	cursePlay(t, e, &b, lib, "black_dividend", "apply", "enemy")
	if stacks(&b, "enemy", "black_dividend") != 1 || len(curseRuntime(&b).Preparations) != 0 {
		t.Fatal("must be an enemy status, not a preparation")
	}
	if len(curseCardChoices(&b, lib, "player", lib.Cards["black_dividend"])) != 0 {
		t.Fatal("cannot duplicate status")
	}
	markCurse(&b, "enemy", 0, 1)
	start := b.Actors["player"].Resources.EnergyPoints
	roll := func(face int) {
		t.Helper()
		e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: face - 1}}}
		if _, err := e.ownedRoll(&b, lib, "enemy", 0, "curse_dice", false); err != nil {
			t.Fatal(err)
		}
	}
	roll(6)
	if b.Actors["player"].Resources.EnergyPoints != start {
		t.Fatal("clean result rewarded")
	}
	roll(1)
	roll(1)
	if b.Actors["player"].Resources.EnergyPoints != start+1 || stacks(&b, "enemy", "black_dividend") != 1 {
		t.Fatal("first reward must persist; at most once per round")
	}
	b = b.Clone()
	b.Segment.Round++
	b.Segment.Current = segment.OngoingEffects
	roll(1)
	if b.Actors["player"].Resources.EnergyPoints != start+2 || stacks(&b, "enemy", "black_dividend") != 0 {
		t.Fatal("second reward must consume status")
	}
	roll(1)
	if b.Actors["player"].Resources.EnergyPoints != start+2 {
		t.Fatal("exceeded max rewards")
	}
	rewards := 0
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "dividend_trigger" {
			rewards++
			if ev.ActorID != "enemy" || ev.Data["source_actor_id"] != "player" {
				t.Fatal("wrong source/target")
			}
		}
	}
	if rewards != 2 {
		t.Fatalf("wanted two reward events, got %d", rewards)
	}
}

func TestBlackDividendExpiryAndRemoval(t *testing.T) {
	for _, cleanse := range []bool{false, true} {
		b, lib := curseFixture(t)
		cursePlan(&b)
		e := NewEngine()
		cursePlay(t, e, &b, lib, "black_dividend", "apply", "enemy")
		if cleanse {
			removeStatus(&b, "enemy", "black_dividend", 0)
		} else {
			b.Segment.Current = segment.OngoingEffects
			expireCursePreparations(&b)
			if stacks(&b, "enemy", "black_dividend") != 1 {
				t.Fatal("expired early")
			}
			b.Segment.Round++
			expireCursePreparations(&b)
		}
		if stacks(&b, "enemy", "black_dividend") != 0 {
			t.Fatal("status remains")
		}
		markCurse(&b, "enemy", 0, 1)
		before := b.Actors["player"].Resources.EnergyPoints
		e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 0}}}
		if _, err := e.ownedRoll(&b, lib, "enemy", 0, "curse_dice", false); err != nil {
			t.Fatal(err)
		}
		if b.Actors["player"].Resources.EnergyPoints != before {
			t.Fatal("removed status rewarded")
		}
		expired := false
		for _, ev := range flushCurseEvents(&b) {
			expired = expired || ev.Data["kind"] == "dividend_expired"
		}
		if expired == cleanse {
			t.Fatal("wrong expiry feedback")
		}
	}
}

func TestBlackDividendPrivateRollAndLegacy(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	cursePlay(t, e, &b, lib, "black_dividend", "apply", "enemy")
	markCurse(&b, "enemy", 0, 1)
	flushCurseEvents(&b)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "combat_dice", Bound: 6, Value: 0}}}
	if _, err := e.ownedRoll(&b, lib, "enemy", 0, "combat_dice", false); err != nil {
		t.Fatal(err)
	}
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "dividend_trigger" {
			t.Fatal("private face leaked")
		}
	}
	b = b.Clone()
	b.Settled.Stage = stageOffensiveReact
	rewards := 0
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "dividend_trigger" {
			rewards++
		}
	}
	if rewards != 1 || len(flushCurseEvents(&b)) != 0 {
		t.Fatal("reveal must publish exactly once")
	}
	b, lib = curseFixture(t)
	cursePlan(&b)
	delete(lib.Statuses, "black_dividend")
	cursePlay(t, e, &b, lib, "black_dividend", "apply", "enemy")
	if stacks(&b, "enemy", "black_dividend") != 0 || len(curseRuntime(&b).Preparations) != 1 {
		t.Fatal("legacy catalog compatibility")
	}
}

func TestBlackDividendRemovedInstanceCannotRearm(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	cursePlay(t, e, &b, lib, "black_dividend", "apply", "enemy")
	removeStatus(&b, "enemy", "black_dividend", 0)
	applyStatus(&b, lib, "enemy", "black_dividend", 1)
	before := b.Actors["player"].Resources.EnergyPoints
	rewardBlackDividend(&b, "enemy", state.RolledDie{Index: 0, Face: 1}, "curse_dice")
	if b.Actors["player"].Resources.EnergyPoints != before {
		t.Fatal("orphan source reused for unrelated status instance")
	}
}
