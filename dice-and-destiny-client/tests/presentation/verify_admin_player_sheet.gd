extends "res://tests/presentation/verify_card_trees.gd"

func _sheet(screen):
	return screen.get_node("AdminPlayerSheet")
func _search_sheet(sheet, value: String) -> void:
	sheet._search.text = value; sheet._search.text_changed.emit(value)
func _select_sheet_row(sheet, id: String) -> void:
	var item: TreeItem = sheet._rows[id]
	sheet._table.scroll_to_item(item); await _frames()
	var point: Vector2 = sheet._table.global_position + sheet._table.get_item_area_rect(item).get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	await _frames()
func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen); await _frames()
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		await create_timer(0.25).timeout
	await _frames(); await _click(screen._admin_button); await _frames()
	if _control(screen, "admin.player_sheet") == null:
		_expect(false, "Admin sheet entry is available after opening settings: " + screen._error.text); quit(1); return
	await _click(_control(screen, "admin.player_sheet")); await _frames()
	var sheet = _sheet(screen)
	_expect(sheet._rows.size() == screen.catalogs.adventurer.cards.size(), "admin sheet includes every published card")
	_expect(not sheet._rows.has("steady_guard_2_paths_variant_node_1"), "unpublished variants are not catalog entries")
	var specialized := ""
	for id in sheet._cards:
		if sheet._card_type(id) == "venom": specialized = id; break
	_expect(not specialized.is_empty() and sheet._rows.has(specialized), "other character cards remain visible")
	_search_sheet(sheet, specialized); await _frames(); await _select_sheet_row(sheet, specialized)
	_expect(sheet._selected == specialized and _text(sheet._details).contains("Outside this character's type"), "pointer inspection explains incompatible card")
	_search_sheet(sheet, "zz-unmatched"); await _frames()
	_expect(sheet._rows.is_empty() and _text(sheet._details).contains("No cards match"), "empty search clears stale preview")
	_search_sheet(sheet, ""); _choose(sheet._type, "venom"); await _frames()
	for id in sheet._rows: _expect(sheet._card_type(id) == "venom", "type filter")
	_choose(sheet._type, ""); sheet._scope.select(3); sheet._scope.item_selected.emit(3); await _frames()
	_expect(sheet._rows.size() == screen.catalogs.adventurer.progression.decklist.size(), "owned filter excludes unowned cards")
	await _click(sheet._back); await _frames()
	_expect(screen._admin_overlay.visible and not screen.has_node("AdminPlayerSheet"), "Back returns to existing admin settings")
	if screen.has_node("AdminPlayerSheet"):
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(); root.get_texture().get_image().save_png("res://.godot/layout-review/admin-sheet-failure.png")
		quit(1); return
	await _click(_control(screen, "admin.trees")); await _frames()
	var tree = screen.get_node("CardTreeWorkspace")
	await _click(_tree_control(tree, "example")); await _frames()
	await _click(_tree_control(tree, "validate")); await _click(tree._publish); await _frames()
	_expect(tree._message.text.begins_with("Published"), "publish example for catalog test")
	await _click(_tree_control(tree, "back")); await _frames()
	var p: Dictionary = runtime.card_trees().result.progression
	var bought: Dictionary = runtime.purchase_progression("adventurer", "buy_collection_card", "steady_guard_2", int(p.revision), 10)
	_expect(bought.get("ok", false), "collect the example base card")
	var upgraded: Dictionary = runtime.purchase_progression("adventurer", "tree_card", "steady_guard_2", int(runtime.card_trees().result.progression.revision), 3, "steady_guard_2_paths_variant_node_1")
	_expect(upgraded.get("ok", false), "create owned stored variant")
	await _click(screen._admin_button); await _frames()
	var before := JSON.stringify(runtime.character_catalogs("progression").result)
	await _click(_control(screen, "admin.player_sheet")); await _frames(); sheet = _sheet(screen)
	_expect(sheet._rows.has("steady_guard_2_paths_variant_node_1") and sheet._rows.has("steady_guard_2_paths_variant_node_2"), "owned and unowned published variants both appear")
	_expect(sheet._rows["steady_guard_2_paths_variant_node_1"].get_text(6) == "1" and sheet._rows["steady_guard_2_paths_variant_node_1"].get_text(5) == "0", "stored variant is distinct from equipped cards")
	_search_sheet(sheet, "steady_guard_2_paths_variant_node_1"); await _frames(); await _select_sheet_row(sheet, "steady_guard_2_paths_variant_node_1")
	_expect(_text(sheet._details).contains("Mighty Ward") and _text(sheet._details).contains("direct purchasing is disabled"), "variant rules and acquisition explained")
	_search_sheet(sheet, ""); sheet._scope.select(2); sheet._scope.item_selected.emit(2); await _frames()
	_expect(sheet._rows.size() == 4, "variant filter includes every tree variant")
	_choose(sheet._character, "venom"); await _frames()
	_expect(sheet._rows.size() == 4 and sheet._rows["steady_guard_2_paths_variant_node_1"].get_text(6) == "0", "character switch changes ownership, not catalog completeness")
	_choose(sheet._character, "adventurer"); sheet._scope.select(0); sheet._scope.item_selected.emit(0)
	_search_sheet(sheet, "Steady"); await _frames(); await _select_sheet_row(sheet, "steady_guard_2_paths_variant_node_1")
	for width in [1920, 1280]:
		root.size = Vector2i(width, int(width * 9.0 / 16.0)); await _frames()
		_expect(root.get_visible_rect().encloses(sheet._back.get_global_rect()), "back fits viewport")
		_expect(sheet._table.get_global_rect().end.x < root.get_visible_rect().end.x, "table remains within screen")
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(); DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
			root.get_texture().get_image().save_png("res://.godot/layout-review/admin-player-sheet-%d.png" % width)
	var after := JSON.stringify(runtime.character_catalogs("progression").result)
	_expect(before == after, "admin sheet browsing does not change definitions, XP, ownership or loadouts")
	await _click(sheet._back); await _frames(); screen.queue_free(); await process_frame
	print("ADMIN PLAYER SHEET: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
