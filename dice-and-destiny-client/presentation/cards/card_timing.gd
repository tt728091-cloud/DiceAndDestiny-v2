extends RefCounted

# Mirrors content.CardWindowMoments: the part of each segment a window covers.
# Reaction moments are opt-in and shown separately from turn timing.
const WINDOW_MOMENTS := {
	"offensive_before_roll": ["offense_before"],
	"offensive_planning": ["offense_before", "offense_after"],
	"offensive_after_roll": ["offense_after"],
	"offensive_reaction": ["offense_reaction"],
	"defense_before_roll": ["defense_before"],
	"defense_selection": ["defense_before", "defense_after"],
	"defense_after_roll": ["defense_after"],
	"defense_reaction": ["defense_reaction"],
	"damage_reaction": ["defense_after"],
}
const LABELS := {"before": "Before", "after": "After", "any": "Any time"}
const REACTIONS := {"offense": "Reaction", "defense": "Roll review"}

# Player-facing "Offense · Before" style chips for a program or mechanic card.
static func chips(windows: Array, roll_requirement: String = "any") -> Array[String]:
	var result: Array[String] = []
	for segment in ["offense", "defense"]:
		var before := false; var after := false; var reaction := false
		for w in windows:
			for moment in WINDOW_MOMENTS.get(str(w), []):
				if w == "offensive_planning" and (moment == "offense_before" and roll_requirement == "after_first" or moment == "offense_after" and roll_requirement == "before_first"): continue
				before = before or moment == segment + "_before"; after = after or moment == segment + "_after"
				reaction = reaction or moment == segment + "_reaction"
		var timing := "any" if before and after else "before" if before else "after" if after else ""
		var words: String = LABELS[timing] if not timing.is_empty() else ""
		if reaction: words += (" + " if not words.is_empty() else "") + REACTIONS[segment]
		if not words.is_empty(): result.append("%s · %s" % [segment.capitalize(), words])
	return result
