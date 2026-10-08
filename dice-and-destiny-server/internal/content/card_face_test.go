package content

import (
	"reflect"
	"strings"
	"testing"
)

func shippedCards(t *testing.T, packs ...string) BattleLibrary {
	t.Helper()
	lib, err := LoadBattleLibrary("../../content/battle_v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range packs {
		if lib, err = LoadBattleExtension(lib, "../../content/"+pack); err != nil {
			t.Fatal(err)
		}
	}
	return lib
}

// Card faces are a keyword and a number; defaults and timing stay off the face.
func TestProgramCardFacesAreShort(t *testing.T) {
	lib := shippedCards(t, "adventurer_v1", "general_v1")
	want := map[string]string{
		"brace":        "Prevent 3\nSaved cards → discard",
		"brace_plus":   "Prevent 3",
		"nudge":        "Change 1 die by ±1",
		"second_wind":  "Gain 2 Energy",
		"strong_swing": "+2 damage to 1 ability this Offense",
		"take_stock":   "Draw 2 cards",
		"try_again":    "Reroll 1 die",
		"dispel":       "Remove 1 enemy buff stack",
		"disrupt":      "Reroll 1 enemy die",
		"reinforce":    "Prevent 2\nor +1 Energy: Prevent 4",
		"second_guard": "Reroll any defense dice",
		"triage":       "Save 1 threatened card",
		"turn_the_die": "Flip 1 die\n(1↔6 · 2↔5 · 3↔4)",
	}
	for id, face := range want {
		c := lib.Cards[id]
		if c.Presentation.EffectSummary != face {
			t.Errorf("%s face = %q, want %q", id, c.Presentation.EffectSummary, face)
		}
		if strings.Contains(c.Presentation.EffectSummary, "Play:") || !strings.Contains(c.Presentation.RulesText, "Play:") {
			t.Errorf("%s: timing belongs in the rules, not the face", id)
		}
	}
}

// The full rules keep the precise wording without leaking raw values.
func TestProgramCardRulesReadNaturally(t *testing.T) {
	lib := shippedCards(t, "adventurer_v1", "general_v1")
	for id, c := range lib.Cards {
		if c.Program == nil {
			continue
		}
		for _, leak := range []string{"[", "1 stacks", "polarity", "up to 100", "100 times", "+0 energy"} {
			if strings.Contains(c.Presentation.RulesText, leak) {
				t.Errorf("%s rules contain %q: %q", id, leak, c.Presentation.RulesText)
			}
		}
	}
	if rules := lib.Cards["nudge"].Presentation.RulesText; !strings.Contains(rules, "by ±1. The result must stay within 1–5.") {
		t.Errorf("nudge rules lost its range: %q", rules)
	}
}

func TestCardTimingTagsMatchRules(t *testing.T) {
	lib := shippedCards(t, "adventurer_v1", "general_v1", "curse_v1")
	cases := map[string][]CardTimingTag{
		"strong_swing":  {{Segment: "offense", When: "before"}},
		"try_again":     {{Segment: "offense", When: "after"}},
		"brace":         {{Segment: "defense", When: "any"}},
		"dispel":        {{Segment: "offense", When: "any"}, {Segment: "defense", When: "any"}},
		"disrupt":       {{Segment: "offense", Reaction: true}},
		"second_guard":  {{Segment: "defense", Reaction: true}},
		"call_the_mark": {{Segment: "offense", Reaction: true}},
	}
	for id, want := range cases {
		if got := lib.Cards[id].Presentation.Timing; !reflect.DeepEqual(got, want) {
			t.Errorf("%s timing = %+v, want %+v", id, got, want)
		}
	}
	mark := lib.Cards["mark_the_number"].Presentation
	if mark.PlayLimit != "1/round" || !strings.Contains(mark.RulesText, "Once per round.") {
		t.Errorf("curse play limit = %q, rules %q", mark.PlayLimit, mark.RulesText)
	}
}

func TestFaceWordHelpers(t *testing.T) {
	if got := deltaWords([]int{-1, 1}); got != "±1" {
		t.Errorf("deltaWords(-1,1) = %q", got)
	}
	if got := deltaWords([]int{1, 2, -2}); got != "+1 or ±2" {
		t.Errorf("deltaWords(1,2,-2) = %q", got)
	}
	if got := orList([]int{1, 2, 3}); got != "1, 2 or 3" {
		t.Errorf("orList = %q", got)
	}
	if got := mechanicParamWords([]any{float64(-1), float64(1)}); got != "±1" {
		t.Errorf("mechanicParamWords = %q", got)
	}
	reroll := CardStep{Effect: "reroll", Target: CardTarget{Owner: "self", Mode: "up_to", Count: 2}, Params: map[string]any{"result": "higher", "consume_roll": true}}
	if got := faceStep(reroll, nil); got != "Reroll up to 2 dice, keep the higher (uses a roll)" {
		t.Errorf("non-default reroll face = %q", got)
	}
}
