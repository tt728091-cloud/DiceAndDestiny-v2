package content

import (
	"fmt"
	"strings"
)

// Players see three timing choices per segment for their own turn: before,
// after, or any time. Each choice maps to engine windows.
//   - Offense before: planning until the actor's first offensive roll.
//   - Offense after: planning after that roll.
//   - Defense before: the Defense screen until the actor's first defense roll.
//   - Defense after: the Defense screen after that roll, until Pass. Passing
//     without rolling never reaches "after".
//
// The in-between moments are a separate, opt-in reaction setting so ordinary
// cards never pause play there: the offensive reaction to revealed attack dice,
// and the defense roll review before a roll applies (CardReactionWindows).
//
// damage_reaction is the legacy damage phase. It is never shown to players and
// rides along with "after" so old saves keep working.
var CardTimingChoices = map[string]map[string][]string{
	"offense": {
		"before": {"offensive_before_roll"},
		"after":  {"offensive_after_roll"},
		"any":    {"offensive_planning"},
	},
	"defense": {
		"before": {"defense_before_roll"},
		"after":  {"defense_after_roll", "damage_reaction"},
		"any":    {"defense_selection", "damage_reaction"},
	},
}

// CardReactionWindows is each segment's opt-in reaction moment.
var CardReactionWindows = map[string]string{
	"offense": "offensive_reaction",
	"defense": "defense_reaction",
}

// CardWindowMoments lists the segment moments ("offense_before", ...) each
// window covers. Windows outside the model (ongoing_damage) cover none.
var CardWindowMoments = map[string][]string{
	"offensive_before_roll": {"offense_before"},
	"offensive_planning":    {"offense_before", "offense_after"},
	"offensive_after_roll":  {"offense_after"},
	"offensive_reaction":    {"offense_reaction"},
	"defense_before_roll":   {"defense_before"},
	"defense_selection":     {"defense_before", "defense_after"},
	"defense_after_roll":    {"defense_after"},
	"defense_reaction":      {"defense_reaction"},
	"damage_reaction":       {"defense_after"},
}

// CardTiming classifies a segment's turn timing as "before", "after", "any" or
// "" (not playable during the turn). A legacy roll requirement narrows
// offensive planning; effects that need a prior roll can never play before it.
func CardTiming(segment string, windows []string, rollRequirement string, needsRoll bool) string {
	before, after := false, false
	for _, w := range windows {
		for _, m := range CardWindowMoments[w] {
			if w == "offensive_planning" && (m == "offense_before" && (rollRequirement == "after_first" || needsRoll) || m == "offense_after" && rollRequirement == "before_first") {
				continue
			}
			before = before || m == segment+"_before"
			after = after || m == segment+"_after"
		}
	}
	switch {
	case before && after:
		return "any"
	case before:
		return "before"
	case after:
		return "after"
	}
	return ""
}

// CardReaction reports whether the card opts into the segment's reaction moment.
func CardReaction(segment string, windows []string) bool {
	return ProgramContains(windows, CardReactionWindows[segment])
}

// CardTimingRules is the player-facing timing line shown in a card's rules.
func CardTimingRules(windows []string, rollRequirement string, needsRoll bool) string {
	words := map[string]map[string]string{
		"offense": {"before": "before your first roll", "after": "after your first roll", "any": "any time"},
		"defense": {"before": "before your first defense roll", "after": "after your first defense roll", "any": "any time"},
	}
	reaction := map[string]string{"offense": "as a reaction to revealed attack dice", "defense": "while a defense roll's result is showing"}
	names := map[string]string{"offense": "Offense", "defense": "Defense"}
	var parts []string
	for _, segment := range []string{"offense", "defense"} {
		turn := CardTiming(segment, windows, rollRequirement, needsRoll)
		react := CardReaction(segment, windows)
		switch {
		case turn != "" && react:
			parts = append(parts, names[segment]+", "+words[segment][turn]+", or "+reaction[segment])
		case turn != "":
			parts = append(parts, names[segment]+", "+words[segment][turn])
		case react:
			parts = append(parts, names[segment]+", only "+reaction[segment])
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Play: " + strings.Join(parts, "; ") + "."
}

// CardTimingTag is one segment of a card's timing ribbon. When is "before",
// "after", "any", or "" when the card plays only at the reaction moment.
type CardTimingTag struct {
	Segment  string `yaml:"segment" json:"segment"`
	When     string `yaml:"when,omitempty" json:"when,omitempty"`
	Reaction bool   `yaml:"reaction,omitempty" json:"reaction,omitempty"`
}

// CardTimingTags is the structured form of CardTimingRules for card faces.
func CardTimingTags(windows []string, rollRequirement string, needsRoll bool) []CardTimingTag {
	var tags []CardTimingTag
	for _, segment := range []string{"offense", "defense"} {
		tag := CardTimingTag{Segment: segment, When: CardTiming(segment, windows, rollRequirement, needsRoll), Reaction: CardReaction(segment, windows)}
		if tag.When != "" || tag.Reaction {
			tags = append(tags, tag)
		}
	}
	return tags
}

// CardPlayLimit is the short play-limit badge ("1/round"), or "".
func CardPlayLimit(perRound, perBattle int) string {
	var parts []string
	if perRound > 0 {
		parts = append(parts, fmt.Sprintf("%d/round", perRound))
	}
	if perBattle > 0 {
		parts = append(parts, fmt.Sprintf("%d/battle", perBattle))
	}
	return strings.Join(parts, " · ")
}

// CardStatusTarget is the ribbon's target badge for cards that remove
// statuses, whose faces ("Remove 1 debuff stack") don't say from whom:
// "Self", "Enemy", "Anyone", or several joined by "/". "" when no step,
// including inside options, removes statuses.
func CardStatusTarget(steps []CardStep) string {
	seen := map[string]bool{}
	var walk func([]CardStep)
	walk = func(steps []CardStep) {
		for _, s := range steps {
			if s.Effect == "remove_status" {
				seen[s.Target.Owner] = true
			}
			for _, o := range s.Choices {
				walk(o.Steps)
			}
		}
	}
	walk(steps)
	var words []string
	for _, owner := range []struct{ id, word string }{{"self", "Self"}, {"enemy", "Enemy"}, {"any", "Anyone"}} {
		if seen[owner.id] {
			words = append(words, owner.word)
		}
	}
	return strings.Join(words, "/")
}

// CardPlayLimitRules is the play limit as a rules sentence, or "".
func CardPlayLimitRules(perRound, perBattle int) string {
	times := func(n int) string {
		switch n {
		case 1:
			return "Once"
		case 2:
			return "Twice"
		}
		return fmt.Sprintf("Up to %d times", n)
	}
	var lines []string
	if perRound > 0 {
		lines = append(lines, times(perRound)+" per round.")
	}
	if perBattle > 0 {
		lines = append(lines, times(perBattle)+" per battle.")
	}
	return strings.Join(lines, "\n")
}

// defaultCardWindows drops before/after-only windows from a capability list,
// leaving the any-time windows that cover them.
func defaultCardWindows(windows []string) []string {
	var out []string
	for _, w := range windows {
		if !strings.HasSuffix(w, "_before_roll") && !strings.HasSuffix(w, "_after_roll") {
			out = append(out, w)
		}
	}
	return out
}
