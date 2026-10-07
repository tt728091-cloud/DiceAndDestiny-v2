extends "res://tests/presentation/verify_blank_ability_creation.gd"

func _tab(ui, index: int) -> void:
	await _frames()
	var point: Vector2 = ui._tabs.global_position + ui._tabs.get_tab_rect(index).get_center()
	await _pointer_click(point)

func _click(button: Button) -> void:
	await _frames()
	await _pointer_click(button.get_global_rect().get_center())

func _pointer_click(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed
		root.push_input(event, true); await process_frame
	await _frames()

func _run() -> void:
	root.content_scale_mode = Window.CONTENT_SCALE_MODE_DISABLED
	root.content_scale_size = Vector2i.ZERO
	for width in [1024, 1440, 1920]:
		print("Checking ability JSON navigation at %d px" % width)
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		await create_timer(0.2).timeout
		var characters = SCREEN.new(); root.add_child(characters); await _frames()
		characters._tabs.current_tab = 4; await _frames()
		var ui = characters.get_node("AbilityCreationWorkspace")
		for destination in [0, 1, 2, 3]:
			await _tab(ui, 4); await _tab(ui, destination)
			_expect(ui._tabs.current_tab == destination, "blank draft can leave JSON for tab %d at %d" % [destination, width])
		_expect(not ui._dirty, "opening JSON without editing does not mark draft dirty")
		await _tab(ui, 4)
		ui._json.text = "{ broken JSON"; _expect(not ui._validate(), "malformed JSON blocks publishing")
		await _tab(ui, 0)
		_expect(ui._tabs.current_tab == 0 and ui._json_pending and ui._publish.disabled, "malformed JSON never traps tab navigation")
		_line_edit(ui, "name", "Preserved form draft")
		await _tab(ui, 4)
		_expect(ui._json.text == "{ broken JSON", "malformed raw text survives guided tab edits")
		await _click(_node(ui, "back")); await _frames()
		_expect(ui._notice.visible, "Back protects edited JSON")
		_expect(ui._notice.size.y < 90, "discard confirmation stays a compact row")
		_expect(ui._notice.get_child(0).size.x > 200, "confirmation label has readable width")
		_expect(root.get_visible_rect().encloses(_node(ui, "discard_edits").get_global_rect()), "discard button stays visible")
		_expect(ui._scroll.size.y > 200, "confirmation does not collapse editor")
		if DisplayServer.get_name() != "headless":
			# A background window may skip drawing; explicitly render the settled
			# layout rather than waiting indefinitely for frame_post_draw.
			RenderingServer.force_draw()
			DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
			root.get_texture().get_image().save_png("res://.godot/layout-review/ability-json-confirm-%d.png" % width)
		await _click(_node(ui, "keep_editing"))
		_expect(not ui._notice.visible and ui._json.text == "{ broken JSON", "Keep editing retains raw input")
		# Syntactically valid JSON with an unrenderable field must also be kept
		# apart from the guided form, rather than crashing it on a tab change.
		var malformed: Dictionary = ui.draft.duplicate(true); malformed.cost = null
		ui._json.text = JSON.stringify(malformed); ui._validate()
		await _tab(ui, 1)
		_expect(ui._tabs.current_tab == 1 and ui.draft.cost is Dictionary, "invalid JSON field shapes cannot corrupt forms")
		await _click(_node(ui, "discard_json")); await _frames()
		_expect(not ui._json_pending and not ui._json_warning.visible, "explicit discard clears invalid JSON")
		_expect(ui.draft.name == "Preserved form draft", "discard JSON preserves guided edits")
		# A valid object with incomplete gameplay rules remains freely editable.
		await _tab(ui, 4)
		var incomplete: Dictionary = ui.draft.duplicate(true); incomplete.id = "incomplete_attack"
		ui._json.text = JSON.stringify(incomplete)
		await _tab(ui, 0)
		_expect(ui._tabs.current_tab == 0 and ui.draft.id == "incomplete_attack" and ui._publish.disabled, "incomplete configuration syncs to editable forms")
		await _click(_node(ui, "back")); await _frames()
		_expect(ui._notice.visible and root.get_visible_rect().encloses(_node(ui, "discard_edits").get_global_rect()), "final exit confirmation is visible and reachable")
		await _click(_node(ui, "discard_edits")); await _frames()
		_expect(not characters.has_node("AbilityCreationWorkspace"), "Discard edits returns to characters")
		characters._tabs.current_tab = 4; await _frames(); ui = characters.get_node("AbilityCreationWorkspace")
		await _tab(ui, 4); await _click(_node(ui, "back")); await _frames()
		_expect(not characters.has_node("AbilityCreationWorkspace"), "unmodified blank JSON exits without a discard prompt")
		characters.queue_free(); await process_frame
	print("ABILITY JSON NAVIGATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
