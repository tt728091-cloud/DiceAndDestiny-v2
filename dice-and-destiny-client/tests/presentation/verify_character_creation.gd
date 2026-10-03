extends SceneTree
const SCREEN := preload("res://app/screens/character/character_creation.gd")
const MENU := preload("res://app/boot/battle_bootstrap.tscn")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var runtime = root.get_node("LearnedBattleRuntime")
	var before_initialized: bool = runtime._initialized
	for width in [1280, 1920, 1024]:
		root.size = Vector2i(width, width * 9 / 16 if width != 1024 else 768)
		var screen = SCREEN.new(); root.add_child(screen)
		for frame in 8: await process_frame
		_expect(screen.catalogs.size() == 4, "native catalog has all four characters")
		for id in screen.ROSTER:
			await _click(screen._roster_buttons[id])
			_expect(screen.character_id == id, "pointer character selection")
			var original := JSON.stringify(screen.catalogs)
			var hp := 0
			for card in screen.character.decklist: hp += int(card.count)
			_expect(str(hp) + "  HEALTH" in _text(screen._summary), "health equals configured card count")
			for tab in range(3):
				screen._tabs.current_tab = tab
				for frame in 4: await process_frame
				var expected: int = screen.character.ability_board.offensive.size() + screen.character.ability_board.defensive.size() if tab == 0 else screen.character.decklist.size() if tab == 1 else screen.character.dice_loadout.size()
				_expect(screen._entry_buttons.size() == expected, "every configured entry appears")
				if not screen._entry_buttons.is_empty():
					screen.selected_id = ""
					await _click(screen._entry_buttons[0])
					_expect(screen.selected_id == screen._entry_buttons[0].get_meta("entry_id"), "pointer inspection")
				for entry in screen._entry_buttons:
					screen.inspect_entry(entry.get_meta("entry_kind"), entry.get_meta("entry_id"))
					_expect(not _text(screen._details).is_empty(), "details available for every entry")
					if tab != 2:
						var rules: String = screen.catalogs[id][screen.selected_kind][screen.selected_id].presentation.rules_text
						_expect(rules in _text(screen._details), "full authored rules are visible")
				for frame in 4: await process_frame
				_expect(screen.size.x <= root.get_visible_rect().size.x, "screen fits viewport")
				_expect(screen._details.get_global_rect().end.x <= root.get_visible_rect().size.x - 20, "inspection remains inside right edge")
				if tab == 1:
					screen._search.text = "zz-no-match"; screen._search.text_changed.emit(screen._search.text)
					_expect(screen._entry_buttons.is_empty(), "deck search empty state")
					screen._search.text = ""; screen._search.text_changed.emit("")
				_expect(JSON.stringify(screen.catalogs) == original, "viewer never modifies character definitions")
			screen._tabs.current_tab = 0
			screen.inspect_entry("abilities", str(screen.character.ability_board.offensive[0]))
			await _capture(screen, "%s-abilities-%d" % [id,width])
			if id == "adventurer":
				screen._tabs.current_tab = 1; screen.inspect_entry("cards", "brace_plus")
				await _capture(screen, "adventurer-deck-%d" % width)
		screen.queue_free(); await process_frame
	_expect(runtime._initialized == before_initialized, "viewer never initializes a battle or policy")
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 8: await process_frame
	await _click(menu._menu_actions[1])
	await process_frame
	_expect(not menu.visible, "menu hidden while exploring")
	var viewer = root.get_node("CharacterCreation")
	for button in viewer.find_children("*", "Button", true, false):
		if button.get_meta("character_control", "") == "back": await _click(button); break
	await process_frame
	_expect(menu.visible, "Back restores menu")
	menu.queue_free(); await process_frame
	print("CHARACTER CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true,false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event,true); await process_frame
	for frame in 3: await process_frame
func _text(node: Node) -> String:
	var result := str(node.text) if node is Label else ""
	for child in node.get_children(): result += "\n" + _text(child)
	return result
func _capture(_screen, filename: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_CHARACTER_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	for frame in 4: await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(filename + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
