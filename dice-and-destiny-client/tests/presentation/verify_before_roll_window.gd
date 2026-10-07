extends "res://tests/presentation/verify_card_creation_guided.gd"

# The card editor offers the before-any-defense-roll window with a clear label
# and hover rules, sets it with pointer input, and publishes the timing rule.
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 3; await _frames()
	var ui = characters.get_node("CardCreationWorkspace")
	_pick_template(ui, "brace"); ui._load_template(); ui._clone_template()
	_line_edit(ui, "id", "early_guard"); _line_edit(ui, "name", "Early Guard")
	ui._tabs.current_tab = 0; await _frames()
	var early: CheckBox = _node(ui, "window.defense_before_roll")
	_expect(early.text == "Defense · before any roll" and early.tooltip_text.contains("first defense"), "before-roll window labelled with hover rules: " + early.text)
	_expect(not early.disabled and not early.button_pressed, "prevention can opt into the before-roll window")
	_expect(_node(ui, "window.defense_selection").text == "Defense · any time", "any-time defense window labelled")
	await _click_visible(ui, early); await _frames()
	for window in ["defense_selection", "damage_reaction"]:
		var check: CheckBox = _node(ui, "window." + window)
		if check.button_pressed: await _click_visible(ui, check); await _frames()
	_expect(ui.draft.program.windows == ["defense_before_roll"], "pointer input leaves only the before-roll window: " + str(ui.draft.program.windows))
	ui._validate(); await _frames()
	_expect(ui._preview.text.contains("Play only before you roll any defense this round."), "preview states the before-roll timing: " + ui._preview.text)
	await _click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "publish before-roll card: " + ui._error.text)
	characters.queue_free(); await process_frame
	print("BEFORE ROLL WINDOW: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
