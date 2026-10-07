package engine

import (
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/content"
	"encoding/json"
	"testing"
)

// strongSwingStatus is the visible preparation status the Strong Swing program
// card applies; its ID is derived from the card's ability-bonus effect.
func strongSwingStatus(lib content.BattleLibrary) string {
	return content.ProgramStatusID("strong_swing", lib.Cards["strong_swing"].Program.Steps[0])
}
func strikeChoice(c programChoice) bool { return c.Ability == "adventurer_strike" }

func TestStrongSwingBeforeRollActionsAndRejection(t *testing.T) {
	b, lib := adventurerFixture(t)
	e := NewEngine()
	forged := programAction(t, &b, lib, "start")
	if id, _ := programPayload(forged); id != "strong_swing-0" {
		for _, a := range programActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
			if id, _ := programPayload(a); id == "strong_swing-0" {
				forged = a
			}
		}
	}
	if _, err := e.handleProgramCommand(&b, lib, forged); err != nil {
		t.Fatal(err)
	}
	targets := map[string]bool{}
	for _, a := range programActions(&b, lib, "player", b.Flow.PendingInput["player"]) {
		_, key := programPayload(a)
		var c programChoice
		if json.Unmarshal([]byte(key), &c) == nil && c.Verb == "target" {
			targets[c.Ability] = true
		}
	}
	if len(targets) != 6 {
		t.Fatalf("pre-roll targets=%v, want all six abilities", targets)
	}
	for _, id := range b.Settled.Actors["player"].OffensiveAbilityIDs {
		if !targets[id] {
			t.Fatalf("missing target %s", id)
		}
	}
	if _, err := e.handleProgramCommand(&b, lib, programAction(t, &b, lib, "cancel")); err != nil {
		t.Fatal(err)
	}
	adventurerRoll(&b, lib, []int{1, 1, 1, 1, 1})
	for rolls := 1; rolls <= 3; rolls++ {
		r := b.Settled.Actors["player"]
		r.RollsUsed = rolls
		b.Settled.Actors["player"] = r
		if programOffers(&b, lib, "strong_swing-0") || programOffers(&b, lib, "strong_swing-1") {
			t.Fatal("Strong Swing still advertised after rolling")
		}
		before, _ := json.Marshal(b)
		if _, err := e.ApplyBattleCommand(&b, forged); err == nil {
			t.Fatal("forged late play accepted")
		}
		after, _ := json.Marshal(b)
		if string(before) != string(after) {
			t.Fatal("rejected play mutated state")
		}
	}
}

func TestStrongSwingExpiresAtOffensiveExitUsedOrUnused(t *testing.T) {
	for _, selection := range []string{"adventurer_strike", "adventurer_large_straight", ""} {
		t.Run(selection, func(t *testing.T) {
			b, lib := adventurerFixture(t)
			enemy := b.Actors["enemy"]
			enemy.Cards.Deck = []string{"enemy-health"}
			b.Actors["enemy"] = enemy
			e := NewEngine()
			if err := playProgramCard(e, &b, lib, "player", "strong_swing-0", strikeChoice); err != nil {
				t.Fatal(err)
			}
			if stacks(&b, "player", strongSwingStatus(lib)) != 1 {
				t.Fatal("preparation missing visible status")
			}
			// Resume a persisted preparation before rolling.
			raw, _ := json.Marshal(b)
			if err := json.Unmarshal(raw, &b); err != nil {
				t.Fatal(err)
			}
			adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
			r := b.Settled.Actors["player"]
			r.SelectedAbilityID = selection
			r.SelectedTierID = "3_swords"
			r.SelectedTargetIDs = []string{"enemy"}
			b.Settled.Actors["player"] = r
			if _, err := e.finalizeOffensiveSources(&b, lib); err != nil {
				t.Fatal(err)
			}
			if b.Segment.Current != segment.Defensive {
				t.Fatalf("exit did not advance: %s", b.Segment.Current)
			}
			if stacks(&b, "player", strongSwingStatus(lib)) != 0 || len(b.Settled.Actors["player"].AbilityModifiers) != 0 {
				t.Fatal("temporary preparation survived Offensive Exit")
			}
			damage := 0
			for _, source := range b.Settled.OffensiveSources {
				if source.SourceActorID == "player" {
					damage += source.BaseAmount
				}
			}
			want := map[string]int{"adventurer_strike": 6, "adventurer_large_straight": 8, "": 0}[selection]
			if damage != want {
				t.Fatalf("committed damage=%d want %d", damage, want)
			}
		})
	}
}

func TestStrongSwingStatusControlsBonus(t *testing.T) {
	b, lib := adventurerFixture(t)
	e := NewEngine()
	if err := playProgramCard(e, &b, lib, "player", "strong_swing-0", strikeChoice); err != nil {
		t.Fatal(err)
	}
	removeStatus(&b, "player", strongSwingStatus(lib), 0)
	if len(b.Settled.Actors["player"].AbilityModifiers) != 0 {
		t.Fatal("removed preparation left stored modifiers")
	}
	if err := playProgramCard(e, &b, lib, "player", "strong_swing-1", strikeChoice); err != nil {
		t.Fatal(err)
	}
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
	r := b.Settled.Actors["player"]
	r.SelectedAbilityID = "adventurer_strike"
	b.Settled.Actors["player"] = r
	removeStatus(&b, "player", strongSwingStatus(lib), 0)
	ops, _ := resolvedOffensiveOperations(&b, lib, "player")
	if summarizeOffensiveOutcome(ops, nil)["base_damage"] != 4 {
		t.Fatal("removed status left a hidden bonus")
	}
}
