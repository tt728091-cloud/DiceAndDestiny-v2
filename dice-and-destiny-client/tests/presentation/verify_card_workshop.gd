extends SceneTree
var failed := false
func _initialize() -> void: call_deferred("_run")
func _check(ok: bool, text: String) -> void:
	if not ok: failed = true; push_error(text)
func _run() -> void:
	var runtime := root.get_node("LearnedBattleRuntime")
	var response: Dictionary = runtime.card_authoring()
	_check(response.get("ok", false), str(response.get("error", "No card capabilities")))
	if not response.get("ok", false): quit(1); return
	_check(response.result.templates.size() >= 21, "General templates missing")
	var screen = load("res://app/screens/character/card_authoring.gd").new(); root.add_child(screen)
	await process_frame; await process_frame
	# The workshop opens on a blank card; load Brace explicitly as the template.
	for i in screen._template.item_count:
		if screen._template.get_item_metadata(i) == "brace": screen._template.select(i)
	screen._load_template()
	_check(screen.draft.get("id", "") == "brace", "Workshop did not load Brace")
	screen.draft.id = "workshop_test_guard"; screen.draft.name = "Workshop Test Guard"
	screen.draft.cost.energy = 2; screen.draft.program.steps[0].params.amount = 5
	screen._validate(); _check(screen._error.text == "Definition valid.", screen._error.text)
	_check(screen._preview.text.contains("5 damage"), "Rules preview did not follow configured prevention")
	screen._publish(); _check(screen._error.text.begins_with("Published"), screen._error.text)
	var catalogs: Dictionary = runtime.character_catalogs()
	_check(catalogs.get("ok", false), "Catalog reload failed")
	_check(catalogs.get("result", {}).get("adventurer", {}).get("cards", {}).has("workshop_test_guard"), "Published card missing from shared library")
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/card-workshop.png")
	screen.queue_free(); await process_frame
	var characters = load("res://app/screens/character/character_creation.gd").new(); root.add_child(characters); await process_frame
	_check(characters.catalogs.get("adventurer", {}).get("cards", {}).has("workshop_test_guard"), "Character screen cannot use published card")
	characters.queue_free(); await process_frame
	print("CARD WORKSHOP: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
