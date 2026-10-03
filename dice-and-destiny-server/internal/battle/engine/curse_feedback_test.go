package engine

import (
	battlerandom "diceanddestiny/server/internal/battle/random"
	"diceanddestiny/server/internal/battle/state"
	"fmt"
	"testing"
)

func TestCurseFeedbackDistinguishesDirectPlacementFromExpansionRoll(t *testing.T) {
	b, lib := curseFixture(t)
	e := Engine{}
	cursePlan(&b)
	cursePlay(t, e, &b, lib, "mark_the_number", "apply", "enemy")
	events := flushCurseEvents(&b)
	marked := 0
	for _, ev := range events {
		if ev.Data["kind"] == "owned_roll" {
			t.Fatal("initial Mark the Number must not roll")
		}
		if ev.Data["kind"] == "face_marked" {
			marked++
			if ev.Data["placement"] != "direct" || ev.Data["face"] != 1 {
				t.Fatalf("wrong direct placement feedback: %+v", ev.Data)
			}
		}
	}
	if marked != 1 {
		t.Fatalf("expected one new face, got %d", marked)
	}
	markCurse(&b, "enemy", 0, 1)
	flushCurseEvents(&b)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{{Stream: "curse_dice", Bound: 6, Value: 2}}}
	if err := e.lesserCurse(&b, lib, "enemy", "feedback", 0); err != nil {
		t.Fatal(err)
	}
	events = flushCurseEvents(&b)
	if len(events) != 2 || events[0].Data["kind"] != "owned_roll" || events[1].Data["placement"] != "rolled" || events[1].Data["face"] != 3 {
		t.Fatalf("expansion must publish the real roll followed by its mark: %+v", events)
	}
}

func TestDefenseCurseFeedbackNamesItsSource(t *testing.T) {
	for _, ability := range []string{"hexward_rebuttal", "misfortune_repaid"} {
		for _, expanded := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/expanded=%v", ability, expanded), func(t *testing.T) {
				b, lib := curseFixture(t)
				e := NewEngine()
				if expanded {
					for i := 0; i < 5; i++ {
						markCurse(&b, "enemy", i, 1)
					}
				}
				flushCurseEvents(&b)
				source := newSettledDamageSource(&b, "enemy", "player", "hexbrand", 5)
				source.Prevention = 2
				b.Settled.OffensiveSources = []state.SettledDamageSource{source}
				defense := state.SettledDefense{ActorID: "player", SourceID: source.ID, AbilityID: ability, RolledFace: 4, RolledFaces: []int{4}}
				if err := e.curseDefenseCompleted(&b, lib, defense); err != nil {
					t.Fatal(err)
				}
				if ability == "misfortune_repaid" {
					if _, err := e.startCurseWork(&b, lib, false); err != nil {
						t.Fatal(err)
					}
					if err := e.resolveCurseChoice(&b, lib, "2"); err != nil {
						t.Fatal(err)
					}
				}
				events := flushCurseEvents(&b)
				outcomes := 0
				for _, ev := range events {
					if ev.Data["kind"] != "face_marked" && ev.Data["kind"] != "owned_roll" {
						continue
					}
					outcomes++
					if ev.ActorID != "enemy" || ev.Data["source_actor_id"] != "player" || ev.Data["source_ability_id"] != ability || ev.Data["defense_source_id"] != source.ID {
						t.Fatalf("lost defense provenance: %+v", ev)
					}
				}
				if outcomes == 0 {
					t.Fatal("missing defense Curse events")
				}
			})
		}
	}
}

func TestHexbrandFeedbackExplainsSequentialOrdinaryCurse(t *testing.T) {
	b, lib := curseFixture(t)
	e := NewEngine()
	for i := 2; i < 5; i++ {
		markCurse(&b, "enemy", i, 1)
	}
	flushCurseEvents(&b)
	actor := b.Settled.Actors["player"]
	actor.SelectedTierID = "skull_5"
	b.Settled.Actors["player"] = actor
	source := newSettledDamageSource(&b, "player", "enemy", "hexbrand", 5)
	e.namedRandom = &battlerandom.Scripted{Values: []battlerandom.ScriptedValue{
		{Stream: "curse_target", Bound: 2, Value: 0}, {Stream: "curse_target", Bound: 1, Value: 0},
		{Stream: "owned_die_selection", Bound: 5, Value: 1}, {Stream: "curse_dice", Bound: 6, Value: 4},
	}}
	if err := e.curseDamageCompleted(&b, lib, &state.SettledDamageBatch{Sources: []state.SettledDamageSource{source}}); err != nil {
		t.Fatal(err)
	}
	events := flushCurseEvents(&b)
	if len(events) != 4 {
		t.Fatalf("expected two direct marks, one roll, one expansion mark: %+v", events)
	}
	for i, ev := range events {
		if ev.Data["attack_source_id"] != source.ID || ev.Data["source_ability_id"] != "hexbrand" || ev.Data["application_count"] != 3 {
			t.Fatalf("missing attack context: %+v", ev)
		}
		if i < 2 && (ev.Data["placement"] != "direct" || ev.Data["face"] != 1 || ev.Data["application_index"] != i+1) {
			t.Fatalf("wrong seed mark: %+v", ev)
		}
	}
	if events[2].Data["kind"] != "owned_roll" || events[3].Data["placement"] != "rolled" || events[3].Data["face"] != 5 || events[3].Data["application_index"] != 3 {
		t.Fatalf("third application must show rolled face 5: %+v", events)
	}
	if !containsInt(curseRuntime(&b).Dice["enemy"][1].CursedFaces, 5) {
		t.Fatal("expected face 5 on second owned die")
	}
}
