extends "res://tests/presentation/verify_character_deck_editing.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	_expect(screen._confirm_buy.button_pressed and screen._confirm_sell.button_pressed, "both confirmations default on")
	await _click(_control(screen, "buy.brace"))
	await _click(screen._skip_prompt)
	await _click(screen._purchase_cancel)
	_expect(screen._confirm_buy.button_pressed and screen._card_count("brace") == 3, "cancel changes neither preference nor deck")
	await _click(_control(screen, "buy.brace"))
	_expect(not screen._skip_prompt.button_pressed, "cancelled checkbox is reset")
	await _click(screen._skip_prompt)
	await _capture(screen, "buy-do-not-show-again")
	await _click(screen._purchase_confirm)
	_expect(not screen._confirm_buy.button_pressed and screen._confirm_sell.button_pressed, "buy preference changes independently")
	_expect(screen._card_count("brace") == 4 and int(screen.catalogs.adventurer.progression.xp) == 90, "confirmed buy happens once")
	await _click(_control(screen, "buy.brace"))
	_expect(not screen._purchase_overlay.visible and screen._card_count("brace") == 5 and int(screen.catalogs.adventurer.progression.xp) == 80, "subsequent buy immediately executes once")
	await _click(_control(screen, "sell.brace"))
	_expect(screen._purchase_overlay.visible, "sale still asks independently")
	await _click(screen._skip_prompt); await _click(screen._purchase_confirm)
	await _click(_control(screen, "sell.brace"))
	_expect(not screen._purchase_overlay.visible and screen._card_count("brace") == 3 and int(screen.catalogs.adventurer.progression.xp) == 100, "subsequent sale immediately credits once")
	# A new screen reads preferences from disk, rather than shared static memory.
	screen.queue_free(); await process_frame
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect(not screen._confirm_buy.button_pressed and not screen._confirm_sell.button_pressed, "both preferences persist on reopen")
	screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.brace"))
	_expect(screen._purchase_overlay.visible and not screen._skip_prompt.visible, "upgrade retains its separate review")
	await _click(screen._purchase_cancel)
	for width in [1024, 1280, 1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		_expect(screen._confirm_sell.get_global_rect().end.x <= root.get_visible_rect().size.x, "confirmation toggles fit viewport")
		await _click(screen._confirm_buy)
		await _click(_control(screen, "buy.brace"))
		_expect(screen._purchase_overlay.visible, "toggle restores buy review")
		_expect(screen._skip_prompt.is_visible_in_tree() and screen._skip_prompt.get_global_rect().end.y <= root.get_visible_rect().size.y, "checkbox fits dialog")
		await _capture(screen, "transaction-preferences-%d" % width)
		await _click(screen._purchase_cancel)
		await _click(screen._confirm_buy)
	await _click(screen._confirm_sell)
	await _click(_control(screen, "sell.brace"))
	_expect(screen._purchase_overlay.visible, "toggle restores sale review")
	await _click(screen._purchase_cancel)
	await _click(screen._confirm_sell)
	# Direct sales cannot sell a missing copy. Direct buys cannot overspend.
	for copy in 3: await _click(_control(screen, "sell.brace"))
	_expect(_control(screen, "sell.brace").disabled and int(screen.catalogs.adventurer.progression.xp) == 130, "selling stops at zero copies")
	for copy in 13: await _click(_control(screen, "buy.brace"))
	_expect(_control(screen, "buy.brace").disabled and int(screen.catalogs.adventurer.progression.xp) == 0, "buying stops at zero XP")
	_expect(screen._card_count("brace") == 13, "direct repeated trades use fresh revisions")
	screen.queue_free(); await process_frame
	print("TRANSACTION PREFERENCES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
