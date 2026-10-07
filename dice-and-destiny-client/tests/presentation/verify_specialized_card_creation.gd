extends "res://tests/presentation/verify_card_creation_guided.gd"
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters.select_character("venom"); characters._tabs.current_tab = 3; await _frames()
	var ui = characters.get_node("CardCreationWorkspace")
	_pick_template(ui, "culture_flask"); ui._load_template(); ui._clone_template()
	_line_edit(ui, "id", "workshop_culture"); _line_edit(ui, "name", "Workshop Culture")
	_node(ui, "energy").value = 0
	ui._tabs.current_tab = 1; await _frames()
	_node(ui, "mechanic.param.stacks").value = 1
	ui._validate(); _expect(not ui._publish_button.disabled, "specialized guided values validate: " + ui._error.text)
	_expect(ui._preview.text.contains("1 catalyst"), "preview generated from changed parameter")
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/card-creation-specialized.png")
	await _click(ui._publish_button); _expect(ui._error.text.begins_with("Published"), "specialized card published")
	await _click(ui._deck_button); await _frames()
	_expect(characters.selected_id == "workshop_culture", "specialized card opens in correct family deck")
	for id in characters._deck_counts(characters._drafts.venom).keys(): characters._set_card_count(id, 0)
	characters.inspect_entry("cards", "workshop_culture"); characters._quantity.value = 12
	await _click(characters._apply); _expect(not characters._dirty("venom"), "specialized deck saved")
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "venom"); gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("specialized-authoring-pointer", 37)
	var action := {}
	for candidate in result.get("legal_actions", []):
		var ids: Array = candidate.get("payload", {}).get("card_ids", [])
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances[ids[0]].definition_id == "workshop_culture": action = candidate; break
	_expect(not action.is_empty(), "new card is playable in a real battle")
	if not action.is_empty():
		result = gateway.submit(JSON.stringify(action))
		_expect(result.get("ok", true), "native play succeeds")
		var catalyst := 0
		for status in result.snapshot.actors.blade.statuses:
			if status.definition_id == "catalyst": catalyst = int(status.stacks)
		_expect(catalyst == 1, "configured amount takes effect in battle")
	characters._tabs.current_tab = 3; await _frames(); ui = characters.get_node("CardCreationWorkspace")
	_pick_template(ui, "black_dividend"); ui._load_template(); ui._clone_template(); ui._tabs.current_tab = 1; await _frames()
	_node(ui, "mechanic.param.energy").value = 3; _node(ui, "mechanic.param.rewards").value = 1
	ui._validate(); _expect(not ui._publish_button.disabled and ui._preview.text.contains("3 energy"), "delayed Curse rewards configurable")
	_pick_template(ui, "alchemists_gamble"); ui._load_template(); ui._tabs.current_tab = 1; await _frames()
	_node(ui, "roll.outcome.2.effect.0.amount").value = 7
	ui._validate(); _expect(not ui._publish_button.disabled and ui._preview.text.contains("7 damage"), "dice outcome table configurable")
	_line_edit(ui, "roll.outcome.0.faces", "1, 2, 3, 4, 6")
	ui._validate(); _expect(ui._publish_button.disabled, "overlapping outcomes cannot publish")
	characters.queue_free(); await process_frame
	print("SPECIALIZED CARD CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
