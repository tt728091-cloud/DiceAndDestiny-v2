package engine

import (
	"diceanddestiny/server/internal/battle/command"
	"diceanddestiny/server/internal/battle/segment"
	"diceanddestiny/server/internal/battle/state"
	"encoding/json"
	"testing"
)

func TestStrongSwingBeforeRollActionsAndRejection(t *testing.T) {
	b, lib := adventurerFixture(t)
	targets := map[string]bool{}
	for _, action := range planningCardActions(&b, lib, "player", state.PendingInput{}) {
		var p command.PlanningCardsPayload
		if err := json.Unmarshal(action.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if len(p.CardIDs) == 1 && p.CardIDs[0] == "strong_swing-0" {
			targets[p.AbilityID] = true
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
	adventurerRoll(&b, lib, []int{1, 1, 1, 1, 1})
	for rolls := 1; rolls <= 3; rolls++ {
		r := b.Settled.Actors["player"]
		r.RollsUsed = rolls
		b.Settled.Actors["player"] = r
		for _, action := range planningCardActions(&b, lib, "player", state.PendingInput{}) {
			var p command.PlanningCardsPayload
			json.Unmarshal(action.Payload, &p)
			for _, id := range p.CardIDs {
				if id == "strong_swing-0" || id == "strong_swing-1" {
					t.Fatal("Strong Swing still advertised after rolling")
				}
			}
		}
		before, _ := json.Marshal(b)
		if err := NewEngine().playSettledCard(&b, lib, "player", "strong_swing-0", nil, "adventurer_strike", 0, ""); err == nil {
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
			if err := e.playSettledCard(&b, lib, "player", "strong_swing-0", nil, "adventurer_strike", 0, ""); err != nil {
				t.Fatal(err)
			}
			if stacks(&b, "player", "strong_swing_ready") != 1 {
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
			if stacks(&b, "player", "strong_swing_ready") != 0 || len(b.Settled.Actors["player"].AbilityModifiers) != 0 {
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
	if err := e.playSettledCard(&b, lib, "player", "strong_swing-0", nil, "adventurer_strike", 0, ""); err != nil {
		t.Fatal(err)
	}
	removeStatus(&b, "player", "strong_swing_ready", 0)
	if len(b.Settled.Actors["player"].AbilityModifiers) != 0 {
		t.Fatal("removed preparation left stored modifiers")
	}
	if err := e.playSettledCard(&b, lib, "player", "strong_swing-1", nil, "adventurer_strike", 0, ""); err != nil {
		t.Fatal(err)
	}
	adventurerRoll(&b, lib, []int{1, 2, 3, 4, 5})
	r := b.Settled.Actors["player"]
	r.SelectedAbilityID = "adventurer_strike"
	b.Settled.Actors["player"] = r
	removeStatus(&b, "player", "strong_swing_ready", 0)
	ops, _ := resolvedOffensiveOperations(&b, lib, "player")
	if summarizeOffensiveOutcome(ops, nil)["base_damage"] != 4 {
		t.Fatal("removed status left a hidden bonus")
	}
}
