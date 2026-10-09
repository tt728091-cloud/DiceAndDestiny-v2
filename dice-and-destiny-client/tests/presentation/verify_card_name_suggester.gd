extends SceneTree

## Generated tree-card names: adjective from the change, noun from the effect,
## never the base name, at most 14 characters, and unique against every taken
## card, ability and status name, with numerals only as a last resort.

const NAMES = preload("res://app/screens/character/card_name_suggester.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var guard := {"program": {"steps": [{"effect": "prevent", "target": {"owner": "self"}, "params": {"amount": 1}}]}}
	var cheaper := [{"text": "Energy 5 → 0", "tone": "gain"}]
	_expect(NAMES.suggest(guard, cheaper, {}) == "Swift Ward", "cheaper prevention is a Swift Ward")
	_expect(NAMES.suggest(guard, cheaper, {"swift ward": true}) == "Swift Block", "a taken name moves to the next noun")
	var stronger := [{"text": "Prevent damage 1 → 2", "tone": "gain"}]
	_expect(NAMES.suggest(guard, stronger, {}) == "Mighty Ward", "stronger prevention is a Mighty Ward")
	_expect(NAMES.suggest(guard, [{"text": "Energy 5 → 10", "tone": "loss"}], {}) == "Heavy Ward", "costlier is Heavy")
	var revive := {"program": {"steps": [{"effect": "sacrifice", "target": {"owner": "self"}}, {"effect": "move_cards", "target": {"owner": "self", "zones": ["removed"]}}]}}
	_expect(NAMES.suggest(revive, stronger, {}).ends_with("Revival"), "revive cards skip the sacrifice cost and use revival nouns")
	var dispel := {"program": {"steps": [{"effect": "remove_status", "target": {"owner": "enemy"}}]}}
	_expect(NAMES.suggest(dispel, stronger, {}) == "Mighty Rite", "enemy status removal uses dispel nouns")
	# Every pairing taken: fall back to numerals on the first pairing.
	var taken := {}
	for adjective in NAMES.ADJECTIVES.cheaper:
		for noun in NAMES.NOUNS.prevent: taken[("%s %s" % [adjective, noun]).to_lower()] = true
	_expect(NAMES.suggest(guard, cheaper, taken) == "Swift Ward II", "numeral only when all pairings are taken")
	for adjective in NAMES.ADJECTIVES.keys():
		for word in NAMES.ADJECTIVES[adjective]:
			for kind in NAMES.NOUNS.keys():
				var name := NAMES.suggest({"program": {"steps": [{"effect": kind, "target": {"owner": "self"}}]}}, [], {})
				_expect(name.length() <= NAMES.LIMIT and not name.contains("·"), "short plain name: " + name)
	_expect(NAMES.numbered("Reinforce", {"reinforce": true, "reinforce ii": true}) == "Reinforce III", "numbered skips taken numerals")
	print("CARD NAME SUGGESTER: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
