extends "res://tests/presentation/verify_character_deck_editing.gd"
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var characters = SCREEN.new(); root.add_child(characters)
	for frame in 6: await process_frame
	_expect(characters._tabs.get_tab_title(3) == "Card Creation", "Card Creation tab available")
	characters._tabs.current_tab = 3
	for frame in 6: await process_frame
	var ui = characters.get_node("CardCreationWorkspace")
	# Every original General template renders and validates in every guided pane.
	for index in ui._template.item_count:
		ui._template.select(index); ui._load_template()
		for tab in [0, 1, 2]:
			ui._tabs.current_tab = tab
			await process_frame
		ui._validate()
		_expect(not ui._publish_button.disabled, "template form remains valid: " + str(ui.draft.id) + ": " + ui._error.text)
	_pick_template(ui, "brace"); ui._load_template()
	var base: Dictionary = ui.draft.duplicate(true)
	for effect in ui.catalog.effects:
		ui.draft = base.duplicate(true); ui.draft.id = "effect_preview_" + effect; ui.draft.name = "Effect Preview " + effect
		ui.draft.program.steps = [ui._new_step(effect)]; ui.draft.program.windows = ui._compatible_windows(ui.draft.program.steps)
		ui._tabs.current_tab = 1; ui._render_fields(); ui._validate(); await process_frame
		_expect(not ui._publish_button.disabled, "all advertised effects have valid guided defaults: " + effect + ": " + ui._error.text)
	_pick_template(ui, "brace"); ui._load_template(); ui._clone_template()
	_line_edit(ui, "id", "guided_guard"); _line_edit(ui, "name", "Guided Guard")
	_node(ui, "energy").value = 2
	ui._tabs.current_tab = 1; await _frames()
	_node(ui, "steps.0.param.amount").value = 5
	_choose(_node(ui, "steps.0.param.destination"), "removed")
	await _frames()
	ui._validate()
	_expect(ui._preview.text.contains("5 damage") and ui._preview.text.contains("permanently removed"), "preview reflects guided effect edits")
	var add = _node(ui, "steps.effect")
	_expect(_option_disabled(add, "reroll") and not _option_disabled(add, "draw"), "incompatible offensive effect disabled, compatible draw available")
	_choose(add, "draw"); await _click_visible(ui, _node(ui, "steps.add")); await _frames()
	_expect(ui.draft.program.steps.size() == 2, "add ordered effect with pointer")
	ui._tabs.current_tab = 0; await _frames()
	_expect(["before", "after", "any"].all(func(c): return _option_disabled(_node(ui, "timing.offense"), c)), "offensive timing disabled by prevention")
	_node(ui, "buy").value = 8; await _frames()
	_expect(int(_node(ui, "sell").max_value) == 8 and int(ui.draft.economy.sell) == 8, "sale bound follows buy price")
	ui._tabs.current_tab = 2; await _frames()
	await _click_visible(ui, _node(ui, "upgrade.add")); await _frames()
	_expect(ui.draft.economy.upgrades.size() == 1, "guided upgrade branch added")
	ui._tabs.current_tab = 1; await _frames()
	# Revalidation must not detach live controls from the draft.
	ui._validate(); _node(ui, "steps.0.param.amount").value = 4; ui._validate()
	_expect(ui._preview.text.contains("4 damage"), "controls remain bound after native preview validation")
	# Blank/invalid numeric lists and advanced definitions cannot publish.
	ui._tabs.current_tab = 3; await _frames()
	ui._json.text = "{"; ui._json_dirty = true; ui._validate()
	_expect(ui._publish_button.disabled, "invalid advanced input blocks publishing")
	ui._sync_json(); ui._tabs.current_tab = 0; ui._validate()
	await _click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "publish from guided form: " + ui._error.text)
	_expect(not ui._deck_button.disabled, "deck handoff available after publication")
	await _click(ui._deck_button); await _frames()
	_expect(characters.selected_id == "guided_guard", "handoff selects published card in character deck")
	await _click(_control(characters, "add.guided_guard"))
	await _click(characters._apply)
	_expect(characters._card_count("guided_guard") == 1 and not characters._dirty("adventurer"), "authored card added and saved through actual deck controls")
	characters._tabs.current_tab = 3; await _frames(); ui = characters.get_node("CardCreationWorkspace")
	# Nested options, conditions and sacrifice must be editable without JSON.
	_pick_template(ui, "second_wind"); ui._load_template(); ui._clone_template(); ui._tabs.current_tab = 1; await _frames()
	await _click_visible(ui, _node(ui, "steps.0.remove")); await _frames()
	_choose(_node(ui, "steps.effect"), "sacrifice"); await _click_visible(ui, _node(ui, "steps.add")); await _frames()
	_expect(_node(ui, "steps.0.drawn_this_play").disabled, "drawn-card filter disabled without a preceding draw")
	_expect(_node(ui, "steps.0.owner").disabled and _node(ui, "steps.0.mode").disabled and _node(ui, "steps.0.down").disabled, "sacrifice target and ordering restrictions enforced")
	_expect(_option_disabled(_node(ui, "steps.effect"), "sacrifice"), "second sacrifice cost unavailable")
	_choose(_node(ui, "steps.effect"), "choice"); await _click_visible(ui, _node(ui, "steps.add")); await _frames()
	await _click_visible(ui, _node(ui, "steps.1.option.add")); await _frames()
	_line_edit(ui, "steps.1.option.1.name", "Recover strength")
	_node(ui, "steps.1.option.1.energy").value = 2
	await _click_visible(ui, _node(ui, "steps.1.option.0.steps.0.condition.add")); await _frames()
	_expect(ui.draft.program.steps[1].choices.size() == 2 and ui.draft.program.steps[1].choices[0].steps[0].condition.all.size() == 1, "nested options and conditions authored with controls")
	ui._validate(); _expect(not ui._publish_button.disabled, "composed sacrifice card validates: " + ui._error.text)
	ui._tabs.current_tab = 0; ui._scroll.scroll_vertical = 0; await _frames()
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/card-creation-guided.png")
	ui._tabs.current_tab = 1; ui._scroll.scroll_vertical = 0; await _frames()
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png("res://.godot/layout-review/card-creation-effects.png")
	ui._guard(func(): ui.queue_free())
	_expect(ui._pending_bar.visible, "unpublished changes protected on exit")
	characters.queue_free(); await process_frame
	print("GUIDED CARD CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _node(ui: Node, key: String) -> Control:
	for node in ui.find_children("*", "Control", true, false):
		if node.get_meta("editor_key", "") == key: return node
	push_error("missing editor control: " + key); failed = true; return null
func _line_edit(ui: Node, key: String, text: String) -> void:
	var node = _node(ui, key); node.text = text; node.text_changed.emit(text)
func _pick_template(ui: Node, id: String) -> void:
	for i in ui._template.item_count:
		if ui._template.get_item_metadata(i) == id: ui._template.select(i); return
	_expect(false, "template not found " + id)
func _choose(pick: OptionButton, value: String) -> void:
	for i in pick.item_count:
		if pick.get_item_metadata(i) == value:
			_expect(not pick.is_item_disabled(i), "option should be enabled: " + value)
			pick.select(i); pick.item_selected.emit(i); return
	_expect(false, "option missing: " + value)
func _option_disabled(pick: OptionButton, value: String) -> bool:
	for i in pick.item_count:
		if pick.get_item_metadata(i) == value: return pick.is_item_disabled(i)
	return true
func _frames() -> void:
	for frame in 5: await process_frame
func _click_visible(ui: Control, button: Button) -> void:
	ui._scroll.ensure_control_visible(button); await _frames(); await _click(button)
