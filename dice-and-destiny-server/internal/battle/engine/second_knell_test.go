package engine

import (
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/segment"
	"testing"
)

func TestSecondKnellStatusLifecycle(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	e := NewEngine()
	cursePlay(t, e, &b, lib, "second_knell", "apply", "enemy")
	if stacks(&b, "enemy", "second_knell") != 1 || len(curseRuntime(&b).Preparations) != 0 {
		t.Fatal("must apply a real status")
	}
	applyStatus(&b, lib, "enemy", "second_knell", 1)
	if stacks(&b, "enemy", "second_knell") != 1 {
		t.Fatal("must cap at one")
	}
	b = b.Clone()
	b.Segment.Round++
	if len(curseCardChoices(&b, lib, "player", lib.Cards["second_knell"])) != 0 {
		t.Fatal("cannot stack another pending Second Knell")
	}
	b.Segment.Current = segment.OngoingEffects
	expireCursePreparations(&b)
	if stacks(&b, "enemy", "second_knell") != 0 {
		t.Fatal("did not expire after Effects")
	}
	found := false
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "second_knell_expired" {
			found = true
		}
	}
	if !found {
		t.Fatal("expiry feedback missing")
	}
}

func TestSecondKnellRetryFeedback(t *testing.T) {
	for _, stream := range []string{"curse_dice", "combat_dice", "status_dice"} {
		for _, retryFace := range []int{1, 6} {
			b, lib := curseFixture(t)
			cursePlan(&b)
			e := NewEngine()
			markCurse(&b, "enemy", 2, 1)
			applyStatus(&b, lib, "enemy", "second_knell", 1)
			flushCurseEvents(&b)
			e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: stream, Bound: 6, Value: 0}, {Stream: stream, Bound: 6, Value: retryFace - 1}}}
			result, err := e.ownedRoll(&b, lib, "enemy", 2, stream, true)
			if err != nil {
				t.Fatal(err)
			}
			count := 1
			if retryFace == 1 {
				count = 2
			}
			if result.Face != retryFace || !result.EffectRetried || stacks(&b, "enemy", "curse_count") != count || stacks(&b, "enemy", "second_knell") != 0 {
				t.Fatal("wrong retry/count/consumption")
			}
			events := flushCurseEvents(&b)
			if stream == "combat_dice" {
				for _, ev := range events {
					if ev.Data["kind"] == "second_knell_trigger" || ev.Data["kind"] == "owned_roll" {
						t.Fatal("private faces leaked")
					}
				}
				if len(b.Settled.Curse.PendingKnell) != 1 {
					t.Fatal("private retry not retained")
				}
				b = b.Clone()
				b.Settled.Stage = stageOffensiveReact
				events = flushCurseEvents(&b)
			}
			found := false
			for _, ev := range events {
				if ev.Data["kind"] != "second_knell_trigger" {
					continue
				}
				found = true
				if ev.Data["count_before"] != 0 && ev.Data["count_before"] != float64(0) {
					t.Fatal("wrong baseline")
				}
				if ev.Data["retry_cursed"] != (retryFace == 1) {
					t.Fatal("wrong retry outcome")
				}
			}
			if !found {
				t.Fatal("trigger feedback missing")
			}
			if len(flushCurseEvents(&b)) != 0 {
				t.Fatal("trigger repeated")
			}
		}
	}
}
func TestSecondKnellCleanseAndCleanRoll(t *testing.T) {
	for _, cleanse := range []bool{false, true} {
		b, lib := curseFixture(t)
		cursePlan(&b)
		e := NewEngine()
		markCurse(&b, "enemy", 0, 1)
		applyStatus(&b, lib, "enemy", "second_knell", 1)
		face := 6
		if cleanse {
			removeStatus(&b, "enemy", "second_knell", 0)
			face = 1
		}
		e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: face - 1}}}
		result, err := e.ownedRoll(&b, lib, "enemy", 0, "curse_dice", true)
		if err != nil {
			t.Fatal(err)
		}
		if result.EffectRetried {
			t.Fatal("clean/cleansed roll retried")
		}
		want := 1
		if cleanse {
			want = 0
		}
		if stacks(&b, "enemy", "second_knell") != want {
			t.Fatal("wrong retained status")
		}
	}
}

// A consumed status must not grant another effect retry (Catalyst shares the limit).
func TestSecondKnellStatusRetryLimit(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	markCurse(&b, "enemy", 0, 1)
	applyStatus(&b, lib, "enemy", "second_knell", 1)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 0}}}
	result, err := e.ownedRoll(&b, lib, "enemy", 0, "curse_dice", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.EffectRetried || stacks(&b, "enemy", "second_knell") != 1 {
		t.Fatal("must wait when retry already used")
	}
}

func TestSecondKnellUnquietHandsCause(t *testing.T) {
	b, lib := curseFixture(t)
	cursePlan(&b)
	markCurse(&b, "enemy", 0, 1)
	applyStatus(&b, lib, "enemy", "second_knell", 1)
	flushCurseEvents(&b)
	e := NewEngine()
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 0}, {Stream: "curse_dice", Bound: 6, Value: 5}}}
	cursePlay(t, e, &b, lib, "unquiet_hands", "0", "enemy")
	found := false
	for _, ev := range flushCurseEvents(&b) {
		if ev.Data["kind"] == "second_knell_trigger" {
			found = true
			if ev.Data["source_card_id"] != "unquiet_hands" || ev.Data["source_actor_id"] != "player" {
				t.Fatal("retry lost initiating card cue")
			}
		}
	}
	if !found {
		t.Fatal("missing card-triggered retry")
	}
}
