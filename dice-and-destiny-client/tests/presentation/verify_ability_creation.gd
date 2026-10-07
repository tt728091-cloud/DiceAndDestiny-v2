extends "res://tests/presentation/verify_card_creation_guided.gd"
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
func _run() -> void:
	root.size = Vector2i(1440, 900)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 4; await _frames()
	_expect(characters.has_node("AbilityCreationWorkspace"), "Ability Creation tab opens")
	if not characters.has_node("AbilityCreationWorkspace"): quit(1); return
	var ui = characters.get_node("AbilityCreationWorkspace")
	for index in ui._template.item_count:
		ui._template.select(index); ui._load(false)
		for tab in [0,1,2]: ui._tabs.current_tab = tab; await process_frame
		_expect(ui._validate(), "ability template validates: " + str(ui.draft.id) + ": " + ui._status.text)
	for index in ui._template.item_count:
		if ui._template.get_item_metadata(index) == "adventurer_strike": ui._template.select(index); break
	ui._load(true); ui._tabs.current_tab = 0; await _frames()
	_node(ui, "id").text = "workshop_strike"; _node(ui, "id").text_changed.emit("workshop_strike")
	_node(ui, "name").text = "Workshop Strike"; _node(ui, "name").text_changed.emit("Workshop Strike")
	ui.draft.qualification.activation_tiers = [{"id": "base", "requirements": {"all": [{"type": "symbol_count", "symbol_id": "sword", "maximum": 5}]}, "operations": [{"type": "deal_damage", "target": "selected_targets", "amount": 3}, {"type": "draw_cards", "target": "self", "amount": 1}]}]
	ui.draft.cost.energy = 1
	ui.draft.hooks = [{"timing": "before_defense", "operations": [{"type": "apply_status", "target": "self", "status_id": "protect", "stack_count": 2}]}]
	ui._render(); _expect(ui._validate(), "shared draw and status ability validates")
	_expect(ui._preview.text.contains("draw 1 card") and ui._preview.text.contains("2 Protect"), "rules reflect actual configured effects")
	await _click(ui._publish); _expect(ui._status.text.begins_with("Published"), "ability published through button")
	ui._tabs.current_tab = 3; await _frames()
	ui._board.offensive = ["workshop_strike"]
	ui._scroll.ensure_control_visible(_node(ui,"assign")); await _frames()
	await _click_visible(ui, _node(ui,"assign")); _expect(ui._status.text.begins_with("Ability board saved"), "board assignment through pointer: " + ui._status.text)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer"); gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("ability-workshop-test", 47)
	_expect(result.get("ok", true), "battle starts with authored board")
	var played := false
	for step in 30:
		var actions: Array = result.get("legal_actions", [])
		var selected := {}
		for candidate in actions:
			if candidate.get("payload", {}).get("ability_id", "") == "workshop_strike": selected = candidate; played = true; break
		if selected.is_empty():
			for candidate in actions:
				if str(candidate.get("type", "")).contains("roll"): selected = candidate; break
		if selected.is_empty():  break
		var energy_before := int(result.get("snapshot", {}).get("actors", {}).get("blade", {}).get("energy_points", -1))
		result = gateway.submit(JSON.stringify(selected))
		if played: _expect(int(result.get("snapshot", {}).get("actors", {}).get("blade", {}).get("energy_points", -1)) == energy_before - 1, "configured offensive cost paid through authority command")
		_expect(result.get("ok", true), "authored battle action accepted")
		if played: break
	_expect(played, "new ability qualifies and can be selected in real battle")
	ui._tabs.current_tab = 4; await _frames(); ui._json.text = "{"; _expect(not ui._validate() and ui._publish.disabled, "invalid JSON blocks publication")
	ui._tabs.current_tab = 0; _expect(ui._tabs.current_tab == 0 and ui._json_pending, "invalid JSON is preserved without trapping navigation")
	ui._tabs.current_tab = 4; await _frames(); _expect(ui._json.text == "{", "returning to JSON restores invalid text")
	ui._json.text = JSON.stringify(ui.draft); ui._tabs.current_tab = 0; await _frames(); ui._validate()
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/ability-creation.png")
	characters.queue_free(); await process_frame
	print("ABILITY CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
