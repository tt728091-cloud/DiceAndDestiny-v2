extends "res://tests/presentation/verify_economy_admin.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen._tabs.current_tab = 0; screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 4: await process_frame
	await _click(_control(screen, "upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	screen._tabs.current_tab = 1; screen.inspect_entry("cards", "brace")
	for frame in 4: await process_frame
	for width in [3456, 1280, 1920, 1024]:
		root.size = Vector2i(width, 2048 if width == 3456 else 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		for cycle in 6:
			await _click(screen._admin_button)
			for frame in 30: await process_frame
			var panel: Control = screen._admin_content.get_parent()
			var viewport := root.get_visible_rect()
			_expect(screen._admin_overlay.visible, "admin opens on cycle %d" % cycle)
			_expect(viewport.encloses(panel.get_global_rect()), "entire admin panel is on-screen at %d cycle %d: %s" % [width, cycle, panel.get_global_rect()])
			_expect(viewport.encloses(screen._admin_save.get_global_rect()), "save remains reachable")
			if cycle == 1:
				# Exercise the content-height growth/shrink that previously left
				# the centering container oversized even after the error cleared.
				screen._admin_error.text = "A character is over budget.\n".repeat(100)
				screen._admin_error.show()
				for frame in 8: await process_frame
				_expect(viewport.encloses(screen._admin_search.get_global_rect()), "large validation content keeps the top of the dialog visible")
				screen._refresh_admin_preview()
				for frame in 8: await process_frame
				_expect(viewport.encloses(panel.get_global_rect()), "clearing validation content restores the dialog inside the viewport")
			await _set_price(screen._admin_prices.brace, 11 if cycle % 2 == 0 else 10)
			await _click(screen._admin_save)
			_expect(not screen._admin_overlay.visible, "save closes admin on cycle %d" % cycle)
			_expect(int(screen.catalogs.adventurer.progression.xp) == (72 if cycle % 2 == 0 else 75), "repeated saves preserve upgrade investment and revalue cards")
		await _click(screen._admin_button)
		await _capture(screen, "admin-reopened-%d" % width)
		await _click(_control(screen, "admin.close"))
		await _click(screen._admin_button)
		screen._admin_save.grab_focus()
		var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; root.push_input(escape, true)
		escape = InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = false; root.push_input(escape, true)
		for frame in 4: await process_frame
		_expect(not screen._admin_overlay.visible, "Escape closes the reopened dialog")
	screen.queue_free(); await process_frame
	print("ADMIN REOPENING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
