extends "res://tests/presentation/verify_character_deck_editing.gd"

func _hover(button: Button) -> void:
	var event := InputEventMouseMotion.new(); event.position = button.get_global_rect().get_center(); root.push_input(event, true)
	for frame in 6: await process_frame

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	var original := JSON.stringify(screen.catalogs.adventurer.progression)
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		for pair in [["cards", "brace"], ["abilities", "adventurer_guard"]]:
			screen._tabs.current_tab = 1 if pair[0] == "cards" else 0
			screen.inspect_entry(pair[0], pair[1])
			for frame in 5: await process_frame
			var button: Button = _control(screen, "upgrade." + pair[1])
			await _hover(button)
			var comparison = screen._comparison
			_expect(comparison.visible, "pointer hover opens " + pair[0] + " comparison")
			# Brace+ (program) says saved cards "stay"; Guard+ says they "return".
			var kept_word := "stay" if "stay" in comparison.after_text else "return"
			_expect("Saved cards go to discard" in comparison.before_text and kept_word in comparison.after_text, "both full rules are shown")
			_expect(not comparison.changed_after.is_empty() and kept_word in comparison.changed_after, "changed words highlighted")
			_expect("energy" not in comparison.changed_after, "unchanged wording stays neutral")
			_expect("energy" in comparison.after_text, "cost metadata included")
			_expect(comparison.get_global_rect().position.x >= 0 and comparison.get_global_rect().end.x <= root.get_visible_rect().size.x and comparison.get_global_rect().end.y <= root.get_visible_rect().size.y, "comparison fits viewport")
			_expect(not comparison.get_global_rect().intersects(button.get_global_rect()), "comparison does not cover action button")
			await _capture(screen, "%s-upgrade-hover-%d" % [pair[0], width])
			_expect(JSON.stringify(screen.catalogs.adventurer.progression) == original, "hover cannot spend XP or modify loadout")
			# Dismissal and keyboard entry use the same preview.
			var move := InputEventMouseMotion.new(); move.position = Vector2(20,20); root.push_input(move,true)
			await create_timer(0.25).timeout
			_expect(not comparison.visible, "leaving hover dismisses comparison")
			button.grab_focus(); await process_frame
			_expect(comparison.visible, "keyboard focus opens comparison")
			button.release_focus(); comparison.dismiss()
	# Hover remains informative when the upgrade is unaffordable.
	screen._tabs.current_tab = 1; screen.inspect_entry("cards", "brace")
	for frame in 5: await process_frame
	var disabled_button := _control(screen, "upgrade.brace"); disabled_button.disabled = true
	var plus_before: int = screen._card_count("brace_plus")
	var xp_before := int(screen.catalogs.adventurer.progression.xp)
	await _hover(disabled_button)
	_expect(screen._comparison.visible, "disabled upgrade still reveals comparison")
	disabled_button.disabled = false
	await _click(disabled_button)
	_expect(screen._purchase_overlay.visible and not screen._comparison.visible, "click opens existing review without hover overlap")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count("brace_plus") == plus_before + 1 and int(screen.catalogs.adventurer.progression.xp) == xp_before - 10, "card upgrade still purchases once")
	screen._tabs.current_tab = 0; screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 5: await process_frame
	await _click(_control(screen, "upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard_plus"], "ability upgrade still works")
	var downgrade := _control(screen, "downgrade.adventurer_guard_plus.adventurer_guard")
	await _hover(downgrade)
	_expect(screen._comparison.visible and "Downgrade" in screen._comparison._title.text, "downgrade also compares tiers")
	screen.inspect_entry("cards", "nudge")
	_expect(not screen._comparison.visible, "changing inspected entry clears stale comparison")
	# Long authored rules remain readable using the independent scroll columns.
	screen.inspect_entry("cards", "brace")
	for frame in 5: await process_frame
	var button := _control(screen, "upgrade.brace")
	screen._comparison.present(button, "Current", "Upgrade", "Old rule. ".repeat(300), "New rule. ".repeat(300), 10)
	for frame in 6: await process_frame
	_expect(screen._comparison._after_rules.get_v_scroll_bar().max_value > screen._comparison._after_rules.size.y, "long rules scroll inside bounded comparison")
	var pointer := InputEventMouseMotion.new(); pointer.position = screen._comparison._after_rules.get_global_rect().get_center(); root.push_input(pointer, true)
	await create_timer(0.25).timeout
	_expect(screen._comparison.visible, "moving into the comparison keeps it open for reading")
	var wheel := InputEventMouseButton.new(); wheel.position = pointer.position; wheel.button_index = MOUSE_BUTTON_WHEEL_DOWN; wheel.pressed = true; root.push_input(wheel, true)
	for frame in 5: await process_frame
	_expect(screen._comparison._after_rules.get_v_scroll_bar().value > 0, "full rules can be scrolled with pointer")
	screen.queue_free(); await process_frame
	print("UPGRADE COMPARISONS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
