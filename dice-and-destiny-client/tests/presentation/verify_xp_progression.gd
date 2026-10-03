extends "res://tests/presentation/verify_character_deck_editing.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	var sandbox_deck: Array = screen.character.decklist.duplicate(true)
	screen._mode_choice.select(1); screen._mode_choice.item_selected.emit(1)
	for frame in 8: await process_frame
	_expect(screen.loadout_mode == "progression", "progression mode switches on")
	_expect(not screen._apply.visible and not screen._reset.visible, "progression cannot apply free sandbox edits")
	_expect(screen._card_count("brace") == 3 and screen._card_count("brace_plus") == 0, "progression starts with base Brace")
	_expect(screen.character.ability_board.defensive == ["adventurer_guard"], "progression starts with base Guard")
	var initial_xp := int(screen.catalogs.adventurer.progression.xp)
	_expect(initial_xp == 100, "configured starter XP displayed")
	screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.brace"))
	_expect(screen._purchase_overlay.visible and "BEFORE" in _text(screen._purchase_details) and "AFTER" in _text(screen._purchase_details), "upgrade preview shows before and after")
	_expect(str(screen.catalogs.adventurer.cards.brace_plus.presentation.rules_text) in _text(screen._purchase_details), "upgrade explains saved-card destination")
	await _click(screen._purchase_cancel)
	_expect(int(screen.catalogs.adventurer.progression.xp) == initial_xp, "cancel spends nothing")
	await _click(_control(screen, "upgrade.brace"))
	await _capture(screen, "xp-brace-upgrade-preview")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count("brace") == 2 and screen._card_count("brace_plus") == 1, "one Brace upgraded")
	_expect(screen._health() == 12 and int(screen.catalogs.adventurer.progression.xp) == initial_xp - 10, "card upgrade preserves health and spends configured XP")
	screen._tabs.current_tab = 0
	screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.adventurer_guard"))
	await _capture(screen, "xp-guard-upgrade-preview")
	await _click(screen._purchase_confirm)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard_plus"], "Guard is replaced in its ability slot")
	_expect(int(screen.catalogs.adventurer.progression.xp) == initial_xp - 35, "ability purchase charges XP")
	screen._tabs.current_tab = 1
	screen._library_search.text = "Tip It"; screen._library_search.text_changed.emit("Tip It")
	for frame in 4: await process_frame
	await _click(screen._library_buttons[0])
	await _click(_control(screen, "buy.tip_it"))
	_expect("Health: 12 → 13" in _text(screen._purchase_details), "card purchase previews health increase")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count("tip_it") == 1 and screen._health() == 13, "new card bought and equipped")
	_expect(int(screen.catalogs.adventurer.progression.xp) == initial_xp - 45, "card purchase charged once")
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		_expect(screen._details.get_global_rect().end.x <= root.get_visible_rect().size.x - 20, "XP inspector fits viewport")
		await _capture(screen, "xp-editor-%d" % width)
	var response: Dictionary = runtime.character_catalogs("progression")
	_expect(int(response.result.adventurer.progression.xp) == initial_xp - 45, "reloading does not grant more XP")
	screen._mode_choice.select(0); screen._mode_choice.item_selected.emit(0)
	for frame in 6: await process_frame
	_expect(screen.character.decklist == sandbox_deck, "sandbox deck remains unchanged")
	screen._mode_choice.select(1); screen._mode_choice.item_selected.emit(1)
	for frame in 6: await process_frame
	_expect(screen._card_count("tip_it") == 1 and int(screen.catalogs.adventurer.progression.xp) == initial_xp - 45, "mode switching preserves purchases")
	screen.queue_free(); await process_frame
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 6: await process_frame
	_expect(menu._loadout_choice.get_selected_metadata() == "progression", "menu retains selected progression mode")
	_expect("13 health" in menu._character_choice.get_item_text(3), "menu shows progression health")
	menu._seat_choice.select(1)
	await _click(menu._menu_actions[0])
	for frame in 8: await process_frame
	var battle_screen: Control
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == "res://app/screens/battle/battle_screen.gd": battle_screen = child
	_expect(battle_screen != null, "progression battle starts through menu")
	if battle_screen != null:
		_expect(battle_screen.gateway.loadout_mode == "progression", "menu pins progression mode for rematches")
		_expect(int(battle_screen._view.actor("blade").max_health) == 13, "battle uses purchased card")
		_expect(battle_screen._view.actor("blade").defensive_abilities == ["adventurer_guard_plus"], "battle uses purchased ability upgrade")
		_expect(int(battle_screen._view.actor("goblin").max_health) == 16, "enemy unchanged")
		battle_screen.queue_free(); await process_frame
	# Spend down remaining XP and verify the UI prevents overspending.
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen.inspect_entry("cards", "tip_it")
	for attempt in 5:
		for frame in 3: await process_frame
		await _click(_control(screen, "buy.tip_it")); await _click(screen._purchase_confirm)
	_expect(int(screen.catalogs.adventurer.progression.xp) == 5 and _control(screen, "buy.tip_it").disabled, "insufficient XP disables purchase")
	screen.queue_free(); await process_frame
	print("XP PROGRESSION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
