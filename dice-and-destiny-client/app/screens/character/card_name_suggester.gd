extends RefCounted
## Suggests short, unique names for tree cards: an adjective for what changed
## from the predecessor plus a noun for what the card does ("Swift Ward",
## "Mighty Toss"). Names never repeat the base card's name. Candidates that are
## taken (by any card, ability or status) or longer than LIMIT are skipped; only
## when every pairing is taken does a numeral suffix appear. Authors can always
## rename the card.

const LIMIT := 14

const ADJECTIVES := {
	"cheaper": ["Swift", "Quick", "Nimble", "Brisk", "Light"],
	"costlier": ["Heavy", "Slow", "Weary", "Laden", "Sluggish"],
	"stronger": ["Mighty", "Deep", "Greater", "Firm", "Grand"],
	"weaker": ["Lesser", "Faint", "Thin", "Meager", "Dull"],
	"wider": ["Twin", "Broad", "Sweeping", "Wide", "Double"],
	"narrower": ["Narrow", "Lone", "Single", "Tight"],
	"kept": ["Steadfast", "Careful", "Kept"],
	"fleeting": ["Fleeting", "Final", "Last"],
	"added": ["Gifted", "Blessed", "Empowered"],
	"stripped": ["Bare", "Plain", "Stripped"],
	"other": ["Altered", "Odd", "Strange", "New"],
}

const NOUNS := {
	"prevent": ["Ward", "Block", "Stance", "Bastion", "Aegis"],
	"save_cards": ["Haven", "Shelter", "Refuge"],
	"reroll": ["Toss", "Throw", "Cast", "Roll"],
	"reroll_enemy": ["Jolt", "Scatter", "Upset"],
	"reroll_defense": ["Parry", "Deflect", "Feint"],
	"adjust_die": ["Touch", "Tilt", "Push", "Tap"],
	"adjust_enemy": ["Shove", "Bump", "Knock"],
	"set_die": ["Weight", "Fix", "Mark"],
	"flip_die": ["Turn", "Flip", "Twist"],
	"copy_die": ["Mirror", "Echo", "Copy"],
	"draw": ["Study", "Search", "Glance", "Insight"],
	"energy": ["Breath", "Spark", "Pulse", "Surge"],
	"cleanse": ["Remedy", "Cure", "Tonic", "Balm"],
	"dispel": ["Rite", "Unbinding", "Undoing"],
	"apply_status": ["Hex", "Brand", "Mark"],
	"ability_bonus": ["Blow", "Edge", "Swing", "Cut"],
	"recover": ["Return", "Retrieval", "Salvage"],
	"revive": ["Revival", "Return", "Rebirth"],
	"sacrifice": ["Offering", "Price", "Tithe"],
	"choice": ["Gambit", "Fork", "Choice"],
	"venom": ["Sting", "Toxin", "Venom"],
	"curse": ["Omen", "Malice", "Hex"],
	"default": ["Gambit", "Trick", "Ploy"],
}

## A unique name for `card`, given the change entries from its predecessor
## (card_tree_diff.gd) and the lowercase names already taken.
static func suggest(card: Dictionary, changes: Array, used: Dictionary) -> String:
	var adjectives: Array = []
	for change in changes:
		for word in ADJECTIVES[_change_kind(change)]:
			if word not in adjectives: adjectives.append(word)
	if adjectives.is_empty(): adjectives = ADJECTIVES.other.duplicate()
	var nouns: Array = NOUNS[_noun_kind(card)]
	for adjective in adjectives:
		for noun in nouns:
			var candidate := "%s %s" % [adjective, noun]
			if candidate.length() <= LIMIT and not used.has(candidate.to_lower()): return candidate
	return numbered("%s %s" % [adjectives[0], nouns[0]], used)

## Lowercase names taken by cards, keyed by card ID. A card never blocks its
## own name: a shared card appears once by ID however many trees lend it, so
## excluding its ID frees its name for editing from any of them.
static func taken(names_by_id: Dictionary, except_card: String = "") -> Dictionary:
	var used := {}
	for id in names_by_id:
		if str(id) != except_card: used[str(names_by_id[id]).strip_edges().to_lower()] = true
	used.erase("")
	return used

## `name` itself when free, otherwise `name II`, `name III`, ….
static func numbered(name: String, used: Dictionary) -> String:
	if not used.has(name.to_lower()): return name
	var numerals := ["II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X"]
	for numeral in numerals:
		var candidate := "%s %s" % [name, numeral]
		if not used.has(candidate.to_lower()): return candidate
	var n := numerals.size() + 2
	while used.has(("%s %d" % [name, n]).to_lower()): n += 1
	return "%s %d" % [name, n]

static func _change_kind(change: Dictionary) -> String:
	var text := str(change.get("text", "")); var tone := str(change.get("tone", "neutral"))
	if text.begins_with("Energy"): return "cheaper" if tone == "gain" else "costlier"
	if text.contains("Saved cards"): return "kept" if text.ends_with("original piles") or text.ends_with("original") else "other"
	if text.begins_with("After play") and text.ends_with("removed"): return "fleeting"
	if text.contains("+ "): return "added"
	if text.contains("− "): return "stripped"
	if text.contains(" targets"): return "wider" if tone == "gain" else "narrower"
	if tone == "gain": return "stronger"
	if tone == "loss": return "weaker"
	return "other"

static func _noun_kind(card: Dictionary) -> String:
	if card.get("mechanic") is Dictionary:
		var access := str(card.get("access_type", ""))
		return access if NOUNS.has(access) else "default"
	var steps: Array = card.get("program", {}).get("steps", []) if card.get("program") is Dictionary else []
	for step in steps:
		var effect := str(step.get("effect", ""))
		var target: Dictionary = step.get("target", {})
		var owner := str(target.get("owner", "self"))
		if effect == "sacrifice": continue
		if effect == "remove_status": return "dispel" if owner == "enemy" else "cleanse"
		if effect == "move_cards": return "revive" if "removed" in target.get("zones", []) else "recover"
		if effect == "reroll": return "reroll_enemy" if owner == "enemy" else "reroll"
		if effect == "adjust_die": return "adjust_enemy" if owner == "enemy" else "adjust_die"
		if effect in ["curse", "roll_cursed"]: return "curse"
		if NOUNS.has(effect): return effect
		return "default"
	return "sacrifice" if not steps.is_empty() else "default"
