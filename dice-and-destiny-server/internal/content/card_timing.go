package content

import "strings"

// Players see three timing choices per segment: before, after, or any time.
// Each choice maps to engine windows. A window's moments say which part of a
// segment it covers; offensive_planning and defense_selection cover both.
//   - Offense before: planning until the actor's first offensive roll.
//   - Offense after: planning after that roll, plus the offensive reaction.
//   - Defense before: the Defense screen until the actor's first defense roll.
//   - Defense after: that roll's review and the Defense screen afterwards,
//     until Pass. Passing without rolling never reaches "after".
//
// damage_reaction is the legacy damage phase. It is never shown to players and
// rides along with "after" so old saves keep working.
var CardTimingChoices = map[string]map[string][]string{
	"offense": {
		"before": {"offensive_before_roll"},
		"after":  {"offensive_after_roll", "offensive_reaction"},
		"any":    {"offensive_planning", "offensive_reaction"},
	},
	"defense": {
		"before": {"defense_before_roll"},
		"after":  {"defense_after_roll", "defense_reaction", "damage_reaction"},
		"any":    {"defense_selection", "defense_reaction", "damage_reaction"},
	},
}

// CardWindowMoments lists the segment moments ("offense_before", ...) each
// window covers. Windows outside the model (ongoing_damage) cover none.
var CardWindowMoments = map[string][]string{
	"offensive_before_roll": {"offense_before"},
	"offensive_planning":    {"offense_before", "offense_after"},
	"offensive_after_roll":  {"offense_after"},
	"offensive_reaction":    {"offense_after"},
	"defense_before_roll":   {"defense_before"},
	"defense_selection":     {"defense_before", "defense_after"},
	"defense_after_roll":    {"defense_after"},
	"defense_reaction":      {"defense_after"},
	"damage_reaction":       {"defense_after"},
}

// CardTiming classifies windows as "before", "after", "any" or "" (not
// playable) for one segment. A legacy roll requirement narrows offensive
// planning; effects that need a prior roll can never play before it.
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

// CardTimingRules is the player-facing timing line shown in a card's rules.
func CardTimingRules(windows []string, rollRequirement string, needsRoll bool) string {
	words := map[string]map[string]string{
		"offense": {"before": "Offense, before your first roll", "after": "Offense, after your first roll", "any": "Offense, any time"},
		"defense": {"before": "Defense, before your first defense roll", "after": "Defense, after your first defense roll", "any": "Defense, any time"},
	}
	var parts []string
	for _, segment := range []string{"offense", "defense"} {
		if timing := CardTiming(segment, windows, rollRequirement, needsRoll); timing != "" {
			parts = append(parts, words[segment][timing])
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Play: " + strings.Join(parts, "; ") + "."
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
