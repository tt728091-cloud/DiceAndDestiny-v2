extends "res://tests/presentation/verify_character_creation.gd"

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	var templates := JSON.stringify(screen.catalogs)
	_expect(screen._tabs.current_tab == 1, "editor opens with deck and library together")
	_expect(screen._library_pane.get_global_rect().end.x < screen._deck_pane.get_global_rect().position.x, "library starts left of deck")
	_expect(screen._deck_buttons.size() == 7, "starter deck is visible beside library")
	# Independent scrolling, filters, and swapping must not change the draft.
	var library_scroll: ScrollContainer = screen._library_list.get_parent()
	var deck_scroll: ScrollContainer = screen._deck_list.get_parent()
	library_scroll.scroll_vertical = 250
	for frame in 3: await process_frame
	var position := library_scroll.scroll_vertical
	_expect(position > 0 and deck_scroll.scroll_vertical == 0, "library scroll is independent")
	await _click(screen._swap)
	_expect(screen._deck_pane.get_global_rect().end.x < screen._library_pane.get_global_rect().position.x, "Swap puts deck on the left")
	_expect(library_scroll.scroll_vertical == position, "Swap preserves library scroll")
	await _click(screen._swap)
	screen._quantity.value = 3
	for frame in 4: await process_frame
	_expect(library_scroll.scroll_vertical == position, "editing does not jump the library back to the top")
	await _click(screen._revert)
	var old_offset: int = screen._card_split.split_offset
	var first: Rect2 = screen._library_pane.get_global_rect()
	var second: Rect2 = screen._deck_pane.get_global_rect()
	var handle := Vector2((first.end.x + second.position.x) / 2.0, first.get_center().y)
	await _drag(handle, handle + Vector2(80, 0))
	_expect(screen._card_split.split_offset > old_offset + 20, "divider resizes with pointer drag")
	screen._card_split.split_offset = old_offset
	await _capture(screen, "split-library-and-deck")
	await _click(screen._swap)
	await _capture(screen, "split-deck-and-library")
	await _click(screen._swap)
	library_scroll.scroll_vertical = 0
	screen._deck_search.text = "Brace"; screen._deck_search.text_changed.emit("Brace")
	for frame in 4: await process_frame
	_expect(screen._deck_buttons.size() == 2, "deck filter finds both Brace versions")
	screen._tabs.current_tab = 1
	for frame in 4: await process_frame
	_expect(screen._library_buttons.size() == screen._eligible_card_ids().size(), "library includes all cards allowed for the character type")
	screen._library_search.text = "Tip It"; screen._library_search.text_changed.emit("Tip It")
	for frame in 4: await process_frame
	_expect(screen._library_buttons.size() == 1, "library search finds a card not in the starter deck")
	_expect(screen._deck_buttons.size() == 2 and screen._deck_search.text == "Brace", "library filter preserves deck filter")
	await _click(screen._deck_buttons[0])
	_expect(screen.selected_id == "brace", "deck rows open shared inspector")
	await _click(screen._library_buttons[0])
	_expect(screen.selected_id == "tip_it" and screen._card_count("tip_it") == 0, "new card inspection")
	await _click(_control(screen, "add.tip_it"))
	_expect(screen._card_count("tip_it") == 1 and screen._health() == 13, "Add updates deck and health")
	_expect("13  HEALTH" in _text(screen._summary), "health summary updates immediately")
	_expect(screen._deck_search.text == "Brace", "editing preserves deck filter")
	screen._deck_search.text = "Tip It"; screen._deck_search.text_changed.emit("Tip It")
	for frame in 4: await process_frame
	_expect(screen._deck_buttons.size() == 1 and "×1" in _text(screen._deck_buttons[0]), "added card immediately appears in deck")
	_expect("×1" in _text(screen._library_buttons[0]), "library quantity updates too")
	await _click(screen._swap)
	_expect(screen._deck_search.text == "Tip It" and screen._library_search.text == "Tip It", "Swap preserves both searches")
	_expect(screen.selected_id == "tip_it" and screen._card_count("tip_it") == 1, "Swap preserves inspection and draft")
	await _click(screen._swap)
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
		screen._tabs.current_tab = 1
		screen._library_search.text = "Tip It"; screen._library_search.text_changed.emit("Tip It")
		screen.inspect_entry("cards", "tip_it")
		for frame in 6: await process_frame
		_expect(screen._apply.get_global_rect().end.x <= root.get_visible_rect().size.x, "Apply fits viewport")
		_expect(screen._apply.get_global_rect().end.y <= root.get_visible_rect().size.y, "Apply is reachable")
		_expect(screen._library_pane.get_global_rect().end.x < screen._deck_pane.get_global_rect().position.x, "both lists fit side by side")
		_expect(screen._deck_pane.get_global_rect().end.x < screen._details.get_global_rect().position.x, "deck does not overlap inspector")
		await _click(screen._swap)
		_expect(screen._deck_pane.get_global_rect().end.x < screen._library_pane.get_global_rect().position.x, "swapped panes fit")
		await _click(screen._swap)
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

func _drag(from: Vector2, to: Vector2) -> void:
	var move := InputEventMouseMotion.new(); move.position = from; root.push_input(move, true); await process_frame
	var press := InputEventMouseButton.new(); press.button_index = MOUSE_BUTTON_LEFT; press.position = from; press.pressed = true
	root.push_input(press, true); await process_frame
	move = InputEventMouseMotion.new(); move.position = to; move.relative = to - from; move.button_mask = MOUSE_BUTTON_MASK_LEFT
	root.push_input(move, true); await process_frame
	press = InputEventMouseButton.new(); press.button_index = MOUSE_BUTTON_LEFT; press.position = to; press.pressed = false
	root.push_input(press, true)
	for frame in 4: await process_frame
