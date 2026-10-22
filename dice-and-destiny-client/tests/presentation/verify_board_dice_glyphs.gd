extends SceneTree
## Every die the authority describes by roles (the board-selection dice and the
## character dice) engraves a glyph on each face instead of a text fallback.
const STONE_DIE := preload("res://presentation/dice/stone_die.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var response: Dictionary = root.get_node("LearnedBattleRuntime").character_catalogs()
	_expect(response.get("ok", false), "character catalogs load: %s" % str(response.get("error", "")))
	var dice: Dictionary = response.get("result", {}).get("custom_adventurer", {}).get("dice", {})
	var checked := 0
	for die_id in ["standard_d6", "hearth_d6", "grove_d6", "forge_d6", "loom_d6"]:
		_expect(dice.has(die_id), "%s is in the catalog" % die_id)
	for die_id in dice:
		var roles: Dictionary = dice[die_id].get("roles", {})
		if roles.is_empty(): continue
		for face in dice[die_id].faces:
			_expect(roles.has(str(face.symbol)), "%s face %d has a role" % [die_id, int(face.number)])
			_expect(not STONE_DIE.glyph(str(face.symbol)).is_empty(), "%s (%s) is engraved, not a text fallback" % [face.symbol, die_id])
			checked += 1
	_expect(checked >= 30, "role dice faces checked (%d)" % checked)
	print("BOARD DICE GLYPHS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if ok: return
	failed = true; push_error(message); print("FAIL: " + message)
