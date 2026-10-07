extends "res://tests/presentation/verify_character_deck_editing.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	runtime.selected_loadout_mode = "progression"
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect("100 XP available + 130 XP in deck = 230 XP" in _text(screen._summary), "starter value is included in card budget")
	screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	await _click(_control(screen, "sell.brace"))
	_expect("XP: 100 → 110" in _text(screen._purchase_details) and "Health: 12 → 11" in _text(screen._purchase_details), "sale previews credit and health loss")
	_expect(screen._purchase_confirm.text == "Receive 10 XP", "sale confirmation states received XP")
	await _capture(screen, "sell-preview")
	await _click(screen._purchase_cancel)
	_expect(screen._card_count("brace") == 2 and int(screen.catalogs.adventurer.progression.xp) == 100, "cancelling sale preserves cards and XP")
	await _click(_control(screen, "sell.brace")); await _click(screen._purchase_confirm)
	_expect(screen._card_count("brace") == 1 and screen._health() == 11, "sale removes exactly one copy")
	_expect("110 XP available + 120 XP in deck = 230 XP" in _text(screen._summary), "sale moves value from deck to available XP")
	await _click(_control(screen, "buy.brace")); await _click(screen._purchase_confirm)
	_expect(screen._health() == 12 and int(screen.catalogs.adventurer.progression.xp) == 100, "equal-price buyback restores budget")
	# Sell every starter copy using the actual UI, including the last card.
	var deck: Array = screen.character.decklist.duplicate(true)
	for entry in deck:
		screen.inspect_entry("cards", str(entry.card_id))
		for copy in int(entry.count):
			for frame in 3: await process_frame
			await _click(_control(screen, "sell." + str(entry.card_id)))
			if screen._health() == 1:
				_expect("This empties your deck" in _text(screen._purchase_details), "last-card sale explains battle requirement")
			await _click(screen._purchase_confirm)
	_expect(screen._health() == 0 and screen._deck_buttons.is_empty(), "all starter cards can be sold")
	_expect(int(screen.catalogs.adventurer.progression.xp) == 230, "full starter sale returns 130 XP on top of 100 allowance")
	_expect(_control(screen, "sell.second_wind").disabled, "cannot sell unowned card")
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		_expect(screen._details.get_global_rect().end.x <= root.get_visible_rect().size.x - 20, "sale inspector fits viewport")
		await _capture(screen, "sold-deck-%d" % width)
	screen.queue_free(); await process_frame
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect(screen._health() == 0 and int(screen.catalogs.adventurer.progression.xp) == 230, "empty deck and refunded XP persist")
	screen.queue_free(); await process_frame
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 8: await process_frame
	_expect(menu._menu_actions[0].disabled and menu._loadout_hint.visible, "empty progression deck cannot launch from menu")
	menu._character_choice.select(0); menu._character_choice.item_selected.emit(0)
	_expect(not menu._menu_actions[0].disabled, "other characters remain playable")
	menu._character_choice.select(3); menu._character_choice.item_selected.emit(3)
	_expect(menu._menu_actions[0].disabled, "returning to empty character disables Start")
	menu._loadout_choice.select(0); menu._loadout_choice.item_selected.emit(0)
	_expect(menu._menu_actions[0].disabled, "Sandbox shares the empty deck and blocks Start")
	menu._loadout_choice.select(1); menu._loadout_choice.item_selected.emit(1)
	await _click(menu._menu_actions[1])
	for frame in 6: await process_frame
	screen = root.get_node("CharacterCreation")
	screen.inspect_entry("cards", "tip_it")
	for frame in 4: await process_frame
	await _click(_control(screen, "buy.tip_it")); await _click(screen._purchase_confirm)
	_expect(screen._health() == 1 and int(screen.catalogs.adventurer.progression.xp) == 220, "empty deck can be rebuilt from library")
	await _click(_control(screen, "back"))
	_expect(not menu._menu_actions[0].disabled and not menu._loadout_hint.visible, "buyback re-enables battle start")
	await _click(menu._menu_actions[0])
	for frame in 8: await process_frame
	var battle_screen: Control
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == "res://app/screens/battle/battle_screen.gd": battle_screen = child
	_expect(battle_screen != null, "rebuilt deck launches through menu")
	if battle_screen != null:
		_expect(int(battle_screen._view.actor("blade").max_health) == 1, "battle uses rebuilt deck")
		battle_screen.queue_free(); await process_frame
	print("XP SALES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
