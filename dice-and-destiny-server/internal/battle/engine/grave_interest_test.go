package engine

import (
	"diceanddestiny/server/internal/battle/segment"
	"fmt"
	"testing"
)

func TestGraveInterestStatusLifecycle(t *testing.T) {
	for _, count := range []int{0, 2, 3, 4, 7} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			cursePlay(t, e, &b, lib, "grave_interest", "apply", "enemy")
			if stacks(&b, "enemy", "grave_interest") != 1 || len(b.Settled.Curse.Preparations) != 0 {
				t.Fatal("card must apply a real status, not a preparation")
			}
			applyStatus(&b, lib, "enemy", "grave_interest", 1)
			if stacks(&b, "enemy", "grave_interest") != 1 {
				t.Fatal("status must cap at one")
			}
			b = b.Clone()
			if stacks(&b, "enemy", "grave_interest") != 1 {
				t.Fatal("pending status lost on clone")
			}
			applyStatus(&b, lib, "enemy", "curse_count", count)
			b.Segment.Current = segment.OngoingEffects
			b.Segment.Round++
			closeSettledWindow(&b)
			if _, err := e.convertCurse(&b, lib); err != nil {
				t.Fatal(err)
			}
			debt := 0
			if count >= 3 {
				debt = 1
			}
			if stacks(&b, "enemy", "grave_interest") != 0 || stacks(&b, "enemy", "grave_debt") != debt || stacks(&b, "enemy", "curse_count") != count%3 {
				t.Fatal("wrong status conversion or expiration")
			}
			damage := max(0, count/3-1)
			if damage == 0 && b.Settled.PendingDamage != nil {
				t.Fatal("diverted group dealt damage")
			}
			if damage > 0 && (b.Settled.PendingDamage == nil || b.Settled.PendingDamage.Sources[0].BaseAmount != damage) {
				t.Fatal("remaining groups did not convert normally")
			}
			found := false
			for _, ev := range flushCurseEvents(&b) {
				if ev.Data["kind"] == "grave_interest_trigger" {
					found = true
					if ev.Data["triggered"] != (count >= 3) || ev.Data["count_spent"] != debt*3 {
						t.Fatalf("incorrect public trigger feedback: %+v", ev)
					}
				}
			}
			if !found {
				t.Fatal("missing public trigger/expiry feedback")
			}
		})
	}
	t.Run("cleanse", func(t *testing.T) {
		b, lib := curseFixture(t)
		cursePlan(&b)
		e := NewEngine()
		cursePlay(t, e, &b, lib, "grave_interest", "apply", "enemy")
		removeStatus(&b, "enemy", "grave_interest", 0)
		applyStatus(&b, lib, "enemy", "curse_count", 3)
		b.Segment.Current = segment.OngoingEffects
		closeSettledWindow(&b)
		if _, err := e.convertCurse(&b, lib); err != nil {
			t.Fatal(err)
		}
		if stacks(&b, "enemy", "grave_debt") != 0 || b.Settled.PendingDamage == nil || b.Settled.PendingDamage.Sources[0].BaseAmount != 1 {
			t.Fatal("cleansed status left a hidden effect")
		}
	})
}

func TestGraveDebtIncome(t *testing.T) {
	for _, income := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(income), func(t *testing.T) {
			b, lib := curseFixture(t)
			applyStatus(&b, lib, "enemy", "grave_debt", 1)
			b = b.Clone()
			actor := b.Actors["enemy"]
			actor.Resources.EnergyPoints = 0
			b.Actors["enemy"] = actor
			runtime := b.Settled.Actors["enemy"]
			runtime.IncomeEnergy = income
			runtime.IncomeCards = 0
			b.Settled.Actors["enemy"] = runtime
			b.Segment.Current = segment.Income
			closeSettledWindow(&b)
			events, err := NewEngine().progressSettledIncome(&b, lib)
			if err != nil {
				t.Fatal(err)
			}
			gain := max(0, income-1)
			if b.Actors["enemy"].Resources.EnergyPoints != gain || stacks(&b, "enemy", "grave_debt") != 0 {
				t.Fatal("debt must reduce one income, floor at zero, and expire")
			}
			found := false
			for _, ev := range events {
				if ev.ActorID == "enemy" && ev.Data["status_id"] == "grave_debt" {
					found = true
					if ev.Data["energy_gain"] != gain || ev.Data["energy_prevented"] != min(1, income) {
						t.Fatalf("wrong income feedback: %+v", ev)
					}
				}
			}
			if !found {
				t.Fatal("missing debt consumption feedback")
			}
		})
	}
}
