package engine

import (
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"testing"
)

func TestRefusalStatusLifecycle(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	cursePlay(t, NewEngine(), &b, lib, "maledictions_refusal", "apply", "enemy")
	if stacks(&b, "enemy", "maledictions_refusal") != 1 || len(curseRuntime(&b).Preparations) != 0 {
		t.Fatal("Refusal must be a status, not a preparation")
	}
	applyStatus(&b, lib, "enemy", "maledictions_refusal", 1)
	if stacks(&b, "enemy", "maledictions_refusal") != 1 {
		t.Fatal("status must cap at one")
	}
	b = b.Clone()
	b.Segment.Round++
	if len(curseCardChoices(&b, lib, "player", lib.Cards["maledictions_refusal"])) != 0 {
		t.Fatal("duplicate guard offered")
	}
	b.Segment.Current = segment.OngoingEffects
	expireCursePreparations(&b)
	if stacks(&b, "enemy", "maledictions_refusal") != 1 {
		t.Fatal("must last until Income")
	}
	b.Segment.Current = segment.Income
	expireCursePreparations(&b)
	if stacks(&b, "enemy", "maledictions_refusal") != 0 {
		t.Fatal("must expire at Income")
	}
	found := false
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "refusal_expired" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing expiry event")
	}
}

func TestRefusalStatusCleanseCheck(t *testing.T) {
	for _, face := range []int{1, 6} {
		for _, cleansed := range []bool{false, true} {
			b, lib := curseFixture(t)
			markCurse(&b, "enemy", 0, 1)
			applyStatus(&b, lib, "enemy", "curse_count", 4)
			applyStatus(&b, lib, "enemy", "maledictions_refusal", 1)
			if cleansed {
				removeStatus(&b, "enemy", "maledictions_refusal", 0)
			}
			flushCurseEvents(&b)
			e := NewEngine()
			e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 5, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: face - 1}}}
			if err := e.cleanseCurseCount(&b, lib, "enemy", 0); err != nil {
				t.Fatal(err)
			}
			want := 0
			if !cleansed && face == 1 {
				want = 5
			}
			if stacks(&b, "enemy", "curse_count") != want || stacks(&b, "enemy", "maledictions_refusal") != 0 {
				t.Fatal("incorrect cleanse or consumption")
			}
			found := false
			for _, ev := range flushCurseEvents(&b) {
				if ev.Data["kind"] == "refusal_trigger" {
					found = true
					if ev.Data["blocked"] != (face == 1) || ev.Data["count_before"] != 4 || ev.Data["count_after"] != want {
						t.Fatal("feedback disagrees with authority")
					}
				}
			}
			if found == cleansed {
				t.Fatal("removed status must not trigger; active status must publish outcome")
			}
		}
	}
}

func TestRefusalUsesFinalKnellFace(t *testing.T) {
	for _, face := range []int{1, 6} {
		b, lib := curseFixture(t)
		markCurse(&b, "enemy", 0, 1)
		applyStatus(&b, lib, "enemy", "curse_count", 4)
		applyStatus(&b, lib, "enemy", "maledictions_refusal", 1)
		applyStatus(&b, lib, "enemy", "second_knell", 1)
		e := NewEngine()
		e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "owned_die_selection", Bound: 5, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: face - 1}}}
		if err := e.cleanseCurseCount(&b, lib, "enemy", 2); err != nil {
			t.Fatal(err)
		}
		want := 3 // original cursed roll adds one, then cleanse removes two
		if face == 1 {
			want = 6
		}
		if stacks(&b, "enemy", "curse_count") != want {
			t.Fatal("must use final retry face for cleanse")
		}
		found := false
		for _, ev := range flushCurseEvents(&b) {
			if ev.Data["kind"] == "refusal_trigger" {
				found = true
				if ev.Data["already_rolled"] != true || ev.Data["blocked"] != (face == 1) {
					t.Fatal("retry presentation contract wrong")
				}
			}
		}
		if !found {
			t.Fatal("missing final cleanse outcome")
		}
	}
}
