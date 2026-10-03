extends "res://tests/presentation/verify_character_deck_editing.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen._tabs.current_tab = 0
	screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect(int(screen.catalogs.adventurer.progression.xp) == 75, "upgrade spends 25 XP")
	var key := "downgrade.adventurer_guard_plus.adventurer_guard"
	_expect(_control(screen, key) != null and not _control(screen, key).disabled, "upgraded ability offers downgrade")
	await _capture(screen, "guard-plus-downgrade-option")
	await _click(_control(screen, key))
	_expect("XP: 75 → 100" in _text(screen._purchase_details) and "Health: 12 → 12" in _text(screen._purchase_details), "downgrade previews refund without health loss")
	_expect("Saved cards return to their piles" in _text(screen._purchase_details) and "Saved cards go to discard" in _text(screen._purchase_details), "before and after rules show changed defense")
	_expect(screen._purchase_confirm.text == "Receive 25 XP" and not screen._skip_prompt.visible, "downgrade confirmation shows refund")
	await _click(screen._purchase_cancel)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard_plus"] and int(screen.catalogs.adventurer.progression.xp) == 75, "cancel leaves upgraded ability and XP intact")
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 5: await process_frame
		await _click(_control(screen, key))
		_expect(screen._purchase_confirm.get_global_rect().end.y <= root.get_visible_rect().size.y, "refund button stays in viewport")
		await _capture(screen, "ability-downgrade-%d" % width)
		await _click(screen._purchase_cancel)
	await _click(_control(screen, key)); await _click(screen._purchase_confirm)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard"], "downgrade restores base tier")
	_expect(screen.selected_kind == "abilities" and screen.selected_id == "adventurer_guard", "inspector selects restored ability")
	var p: Dictionary = screen.catalogs.adventurer.progression
	_expect(int(p.xp) == 100 and int(p.upgrade_spent) == 0 and int(p.total_budget) == 220, "refund reconciles total budget")
	_expect(screen._health() == 12 and _control(screen, key) == null, "health unchanged and duplicate refund unavailable")
	screen.queue_free(); await process_frame
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect(screen.character.ability_board.defensive == ["adventurer_guard"] and int(screen.catalogs.adventurer.progression.xp) == 100, "base tier and refund persist")
	screen._tabs.current_tab = 0; screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard_plus"] and int(screen.catalogs.adventurer.progression.xp) == 75, "upgrade can be repurchased")
	screen.queue_free(); await process_frame
	print("ABILITY DOWNGRADES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
