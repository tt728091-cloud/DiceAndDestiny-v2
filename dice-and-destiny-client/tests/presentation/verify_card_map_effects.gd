extends SceneTree
## The card editor edits map effects: a map-only card (Trailblaze, as the tree
## workshop opens it) renders every tab without touching a battle program,
## its Move changes from 2 to 3 and validates through the authority; a battle
## card gains a map effect; the tree forge steps the map value and its diff
## reads "Map: Move 2 → Move 3".
var failed := false
const EDITOR := "res://app/screens/character/card_authoring.gd"
const FORGE := preload("res://app/screens/character/card_tree_forge.gd")
const DIFF := preload("res://app/screens/character/card_tree_diff.gd")

func _initialize() -> void: call_deferred("_run")
func _check(ok: bool, text: String) -> void:
	if not ok: failed = true; push_error(text)

func _control(node: Node, key: String, meta: String = "editor_key") -> Node:
	for child in node.find_children("*", "Control", true, false):
		if str(child.get_meta(meta, "")) == key and not child.is_queued_for_deletion(): return child
	return null

func _trailblaze() -> Dictionary:
	return {"schema_version": 1, "id": "trailblaze", "name": "Trailblaze", "type": "map", "access_type": "general",
		"map_effect": {"kind": "move", "amount": 2, "text": "Blaze a trail."},
		"economy": {"buy": 10, "sell": 10, "copy_limit": 20, "upgrades": []},
		"presentation": {"illustration_path": "res://assets/battle/cards/battle_focus.png", "effect_summary": "Map only", "rules_text": "Map card.", "art_key": ""},
		"cost": {"energy": 0}, "play": {"source_zones": ["hand"], "destination": "discard", "playable_during": []},
		"targeting": {"selector": "self", "minimum": 0, "maximum": 1}, "reaction_window": {"opens": false, "pass_required": false}, "operations": []}

func _run() -> void:
	root.size = Vector2i(1600, 1000)
	var runtime := root.get_node("LearnedBattleRuntime")
	# 1. A map-only card opened as the tree workshop opens it.
	var editor = load(EDITOR).new(); editor.embedded_draft = _trailblaze(); root.add_child(editor)
	for frame in 4: await process_frame
	var amount: SpinBox = _control(editor, "map.amount")
	_check(amount != null and int(amount.value) == 2, "the Card tab shows the map Move (2)")
	_check(_control(editor, "map.kind") != null and _control(editor, "map.energy_cost") != null and _control(editor, "map.ignore_terrain") != null and _control(editor, "map.text") != null, "the map section offers kind, extra energy, terrain options and text")
	_check(_control(editor, "timing.offense") == null and _control(editor, "uses_per_round") == null, "a map card shows no battle play timing")
	for tab in [1, 2, 3, 0]:
		editor._tabs.current_tab = tab; editor._change_tab(tab)
		for frame in 2: await process_frame
	amount = _control(editor, "map.amount")
	amount.value = 3
	editor._validate()
	_check(str(editor._error.text).begins_with("Definition valid"), "Trailblaze with Move 3 validates: %s" % editor._error.text)
	_check("Gain 3 Move" in str(editor._preview.text), "the rules preview reads the new Move: %s" % editor._preview.text)
	var accepted := {}
	editor.draft_accepted.connect(func(card): accepted.merge(card))
	editor._publish()
	for frame in 2: await process_frame
	_check(int(accepted.get("map_effect", {}).get("amount", 0)) == 3, "Use card settings hands back Move 3: %s" % [accepted.get("map_effect")])
	if is_instance_valid(editor): editor.queue_free()
	await process_frame
	# 2. A battle card gains a map effect.
	var catalog: Dictionary = runtime.card_authoring().result
	_check(Array(catalog.get("map_effect_kinds", [])) == ["move", "draw", "energy", "sight", "scout", "look", "influence", "sneak", "hide", "fall_back", "high_ground", "mark", "extend_time", "lure", "haggle", "field_medic", "spare_bandage"], "the editor gets the map effect kinds")
	editor = load(EDITOR).new(); editor.embedded_draft = catalog.templates.nudge.duplicate(true); root.add_child(editor)
	for frame in 4: await process_frame
	var enabled: CheckBox = _control(editor, "map.enabled")
	_check(enabled != null and not enabled.button_pressed and _control(editor, "map.amount") == null, "a battle card starts without a map effect")
	enabled.button_pressed = true
	for frame in 3: await process_frame
	_check(_control(editor, "map.amount") != null and editor.draft.get("map_effect") is Dictionary, "ticking the box adds a map effect")
	editor._validate()
	_check(str(editor._error.text).begins_with("Definition valid") and "Map: Gain 2 Move" in str(editor._preview.text), "Nudge with a map Move validates: %s / %s" % [editor._error.text, editor._preview.text])
	editor.queue_free(); await process_frame
	# 3. The forge steps the map value and the diff says so.
	var forge = FORGE.new(); forge.catalog = catalog; forge.runtime = runtime
	forge.compare_card = _trailblaze(); forge.compare_name = "Trailblaze"
	forge.session = {"card": _trailblaze(), "name_touched": true}
	root.add_child(forge); forge.start("edit", 10, false)
	for frame in 2: await process_frame
	var inc: Button = _control(forge, "forge.map.amount.inc", "tree_action")
	_check(inc != null, "the forge offers the map Move stepper")
	if inc != null: inc.pressed.emit()
	for frame in 2: await process_frame
	var changes: Array = DIFF.changes(_trailblaze(), forge.card, catalog).map(func(c): return str(c.text))
	_check("Map: Move 2 → Move 3" in changes, "the tree diff reads the map change: %s" % [changes])
	forge.queue_free(); await process_frame
	print("CARD MAP EFFECTS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
