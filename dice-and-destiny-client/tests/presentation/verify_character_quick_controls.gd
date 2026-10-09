extends "res://tests/presentation/verify_character_deck_editing.gd"
## Row-level +/- controls, inspector isolation, and Escape inside workspaces.

func _visible_click(key: String, screen: Node) -> void:
	var button := _control(screen, key)
	_expect(button != null, "control exists: " + key)
	if button == null: return
	var parent := button.get_parent()
	while parent != null:
		if parent is ScrollContainer: parent.ensure_control_visible(button)
		parent = parent.get_parent()
	for frame in 4: await process_frame
	await _click(button)

func _escape() -> void:
	for pressed in [true, false]:
		var event := InputEventKey.new(); event.keycode = KEY_ESCAPE; event.pressed = pressed
		root.push_input(event, true); await process_frame
	for frame in 4: await process_frame

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	# Sandbox: quick buttons change one copy without moving the inspector.
	screen.inspect_entry("cards", "steady_guard")
	for frame in 4: await process_frame
	var guard: int = screen._card_count("steady_guard"); var nudge: int = screen._card_count("nudge"); var health: int = screen._health()
	await _visible_click("quick_add.deck.nudge", screen)
	_expect(screen._card_count("nudge") == nudge + 1 and screen._health() == health + 1, "deck row + adds one copy")
	_expect(screen.selected_id == "steady_guard" and int(screen._quantity.value) == guard, "inspector keeps the inspected card's count")
	await _visible_click("quick_remove.library.nudge", screen)
	_expect(screen._card_count("nudge") == nudge and not screen._dirty("adventurer"), "library row − removes it again")
	_expect(_control(screen, "quick_remove.library.tip_it").disabled, "− is disabled at zero copies")
	await _visible_click("quick_add.library.tip_it", screen)
	_expect(screen._card_count("tip_it") == 1 and "×1" in _text(_control(screen, "entry.deck.cards.tip_it")), "library + puts a new card in the deck")
	await _click(screen._revert)
	# Escape closes the open workspace, never the character screen beneath it.
	for tab in [3, 4, 5]:
		var name: String = ["CardCreationWorkspace", "AbilityCreationWorkspace", "CardTreeWorkspace"][tab - 3]
		screen._tabs.current_tab = tab
		for frame in 6: await process_frame
		_expect(screen.has_node(name), name + " opens")
		await _escape()
		_expect(is_instance_valid(screen) and screen.is_inside_tree(), "Escape in %s keeps Character Creation open" % name)
		_expect(not screen.has_node(name), "Escape returns from untouched " + name)
	screen.queue_free(); await process_frame
	# Progression: + and − go through the usual buy and sell review.
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	var xp := int(screen.catalogs.adventurer.progression.xp); var copies: int = screen._card_count("steady_guard"); var price: int = screen._card_price("steady_guard")
	await _visible_click("quick_add.deck.steady_guard", screen)
	_expect(screen._purchase_overlay.visible, "+ opens the buy review")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count("steady_guard") == copies + 1 and int(screen.catalogs.adventurer.progression.xp) == xp - price, "+ buys exactly one copy")
	await _visible_click("quick_remove.deck.steady_guard", screen)
	_expect(screen._purchase_overlay.visible, "− opens the sale review")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count("steady_guard") == copies and int(screen.catalogs.adventurer.progression.xp) == xp, "− sells exactly one copy")
	screen.queue_free(); await process_frame
	print("CHARACTER QUICK CONTROLS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
