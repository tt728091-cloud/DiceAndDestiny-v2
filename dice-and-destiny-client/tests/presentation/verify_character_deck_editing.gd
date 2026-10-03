extends "res://tests/presentation/verify_character_creation.gd"

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	var templates := JSON.stringify(screen.catalogs)
	screen._tabs.current_tab = 3
	for frame in 4: await process_frame
	_expect(screen._entry_buttons.size() == screen.catalogs.adventurer.cards.size(), "library includes all supported cards")
	screen._search.text = "Tip It"; screen._search.text_changed.emit("Tip It")
	for frame in 4: await process_frame
	_expect(screen._entry_buttons.size() == 1, "library search finds a card not in the starter deck")
	await _click(screen._entry_buttons[0])
	_expect(screen.selected_id == "tip_it" and screen._card_count("tip_it") == 0, "new card inspection")
	await _click(_control(screen, "add.tip_it"))
	_expect(screen._card_count("tip_it") == 1 and screen._health() == 13, "Add updates deck and health")
	_expect("13  HEALTH" in _text(screen._summary), "health summary updates immediately")
	_expect(not screen._apply.disabled and screen._dirty("adventurer"), "draft is unsaved")
	_expect(JSON.stringify(screen.catalogs) == templates, "edits preserve shared template")
	await _click(screen._roster_buttons.venom)
	await _click(screen._roster_buttons.adventurer)
	_expect(screen._card_count("tip_it") == 1, "switching characters preserves drafts")
	screen.inspect_entry("cards", "tip_it")
	for frame in 4: await process_frame
	var edit: LineEdit = screen._quantity.get_line_edit()
	edit.grab_focus(); edit.select_all(); edit.text = "3"
	var enter := InputEventKey.new(); enter.keycode = KEY_ENTER; enter.pressed = true; root.push_input(enter, true)
	for frame in 4: await process_frame
	_expect(screen._card_count("tip_it") == 3 and screen._health() == 15, "typed quantity commits")
	await _click(screen._apply)
	_expect(not screen._dirty("adventurer") and screen._apply.disabled, "Apply saves and clears dirty state")
	var response: Dictionary = runtime.character_catalogs()
	_expect(response.result.adventurer.get("owned_decklist", []).size() == 8, "native persisted newly added card")
	await _click(_control(screen, "add.tip_it"))
	await _click(screen._revert)
	_expect(screen._card_count("tip_it") == 3, "Revert restores saved deck")
	await _click(screen._reset)
	_expect(screen._health() == 12 and screen._dirty("adventurer"), "Reset changes only draft")
	response = runtime.character_catalogs()
	_expect(response.result.adventurer.get("owned_decklist", []).size() == 8, "Reset does not silently save")
	await _click(screen._revert)
	for width in [1024, 1280, 1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		screen._tabs.current_tab = 3
		screen._search.text = "Tip It"; screen._search.text_changed.emit("Tip It")
		screen.inspect_entry("cards", "tip_it")
		for frame in 6: await process_frame
		_expect(screen._apply.get_global_rect().end.x <= root.get_visible_rect().size.x, "Apply fits viewport")
		_expect(screen._apply.get_global_rect().end.y <= root.get_visible_rect().size.y, "Apply is reachable")
		await _capture(screen, "deck-editor-%d" % width)
	# Empty draft stays editable, but cannot be applied.
	for entry in screen._drafts.adventurer.duplicate(true): screen._set_card_count(str(entry.card_id), 0)
	_expect(screen._health() == 0 and screen._apply.disabled, "empty deck is blocked")
	await _click(_control(screen, "back"))
	_expect(screen._confirm.visible, "Back protects unsaved work")
	await _click(screen._cancel_changes)
	_expect(is_instance_valid(screen) and not screen._confirm.visible, "Cancel keeps draft open")
	await _click(_control(screen, "back"))
	await _click(screen._discard_changes)
	await process_frame
	_expect(not is_instance_valid(screen), "Discard exits without saving")
	screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	_expect(screen._health() == 15 and screen._card_count("tip_it") == 3, "reopened screen loads saved deck")
	screen.queue_free(); await process_frame
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 8: await process_frame
	_expect("15 health" in menu._character_choice.get_item_text(3), "menu reports saved health")
	menu.queue_free(); await process_frame
	runtime.select_model("brine-mask")
	var battle: Dictionary = runtime.start_battle("custom-deck-ui", "seat-a", 731, false, "adventurer", true)
	_expect(battle.get("accepted", false), "saved deck starts a real battle")
	if battle.get("accepted", false):
		_expect(int(battle.snapshot.actors.blade.max_health) == 15, "battle uses edited health")
		var count := 0
		for instance in battle.snapshot.actors.blade.card_instances.values():
			if instance.definition_id == "tip_it": count += 1
		_expect(count == 3, "battle instantiates newly added card copies")
		_expect(int(battle.snapshot.actors.goblin.max_health) == 16, "enemy deck is unchanged")
	print("CHARACTER DECK EDITING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _control(screen: Node, key: String) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("character_control", "") == key: return button
	return null
