extends "res://tests/presentation/verify_character_deck_editing.gd"

func _set_price(spin: SpinBox, amount: int) -> void:
	var edit := spin.get_line_edit()
	edit.grab_focus(); edit.select_all(); edit.text = str(amount)
	var enter := InputEventKey.new(); enter.keycode = KEY_ENTER; enter.pressed = true; root.push_input(enter, true)
	enter = InputEventKey.new(); enter.keycode = KEY_ENTER; enter.pressed = false; root.push_input(enter, true)
	edit.release_focus()
	for frame in 4: await process_frame

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	await _click(screen._admin_button)
	await _capture(screen, "admin-initial")
	_expect(screen._admin_overlay.visible and int(screen._admin_budgets.adventurer.value) == 220, "admin opens with total budget")
	await _set_price(screen._admin_prices.brace, 12)
	await _set_price(screen._admin_budgets.adventurer, 260)
	_expect("126 in deck + 0 in upgrades · 134 XP available" in screen._admin_preview.text, "preview reconciles price and budget")
	await _set_price(screen._admin_budgets.adventurer, 1)
	_expect(screen._admin_save.disabled and screen._admin_error.visible, "over-budget update is blocked")
	await _set_price(screen._admin_budgets.adventurer, 260)
	await _click(screen._admin_save)
	_expect(not screen._admin_overlay.visible, "admin save succeeds")
	_expect(int(screen.catalogs.adventurer.progression.xp) == 134 and int(screen.catalogs.adventurer.progression.total_budget) == 260, "saved balance reconciles")
	screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	_expect("12 XP" in _control(screen, "buy.brace").text and "+12 XP" in _control(screen, "sell.brace").text, "buy and sell use override")
	await _click(_control(screen, "sell.brace")); await _click(screen._purchase_confirm)
	_expect(int(screen.catalogs.adventurer.progression.xp) == 146, "sale refunds new price")
	await _click(_control(screen, "buy.brace")); await _click(screen._purchase_confirm)
	_expect(int(screen.catalogs.adventurer.progression.xp) == 134, "buyback uses new price")
	await _click(screen._admin_button)
	await _set_price(screen._admin_prices.brace, 8)
	await _click(_control(screen, "admin.close"))
	_expect(screen._card_price("brace") == 12, "closing discards edits")
	await _click(screen._admin_button)
	await _set_price(screen._admin_prices.brace, 8)
	for width in [1024, 1280, 1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		_expect(screen._admin_save.get_global_rect().end.y <= root.get_visible_rect().size.y, "save fits viewport")
		_expect(screen._admin_content.get_global_rect().end.x <= root.get_visible_rect().size.x, "admin panel fits viewport")
		await _capture(screen, "economy-admin-%d" % width)
	await _click(screen._admin_save)
	_expect(int(screen.catalogs.adventurer.progression.xp) == 146 and int(screen.catalogs.adventurer.progression.deck_value) == 114, "lower price releases XP")
	screen.queue_free(); await process_frame
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect(int(screen.catalogs.adventurer.progression.xp) == 146 and screen._card_price("brace") == 8, "prices and reconciled budget persist")
	for id in screen.ROSTER:
		screen.select_character(id)
		_expect(screen._card_price("brace") == 8, "global override visible for " + id)
		var p: Dictionary = screen.catalogs[id].progression
		_expect(int(p.xp) + int(p.deck_value) + int(p.upgrade_spent) == int(p.total_budget), "budget equation holds for " + id)
	screen.queue_free(); await process_frame
	print("ECONOMY ADMIN: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
