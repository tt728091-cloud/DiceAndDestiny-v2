extends SceneTree

## Older published cards stored a sacrifice step's params as null. The tree
## workshop's change summaries and node tooltips must describe such steps
## without a script error.

const DIFF = preload("res://app/screens/character/card_tree_diff.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var catalog := {"effects": {"sacrifice": {"label": "Sacrifice cards as a cost", "parameters": {}}, "draw": {"label": "Draw cards", "parameters": {"amount": {"type": "integer", "default": 1}}}}}
	var sacrifice := {"effect": "sacrifice", "params": null, "target": {"owner": "self", "mode": "exact", "count": 1}}
	var before := {"cost": {"energy": 0}, "program": {"steps": [{"effect": "draw", "params": {"amount": 2}}]}}
	var after := {"cost": {"energy": 0}, "program": {"steps": [sacrifice, {"effect": "draw", "params": {"amount": 3}}]}}
	var entries: Array = DIFF.changes(before, after, catalog)
	_expect(not entries.is_empty(), "a sacrifice step with null params is described")
	_expect(DIFF.step_params(sacrifice) == {}, "null params read as an empty dictionary")
	_expect(not DIFF.summary(before, after, catalog).is_empty() and not DIFF.with_xp(before, after, catalog).is_empty(), "summary and tooltip changes build")
	print("CARD TREE DIFF PARAMS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
