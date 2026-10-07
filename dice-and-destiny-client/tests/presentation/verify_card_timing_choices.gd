extends "res://tests/presentation/verify_card_creation_guided.gd"

# The card editor offers Before / After / Any time per segment, disables
# choices the effects cannot use, writes the mapped windows, and publishes the
# timing line in the card's rules.
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 3; await _frames()
	var ui = characters.get_node("CardCreationWorkspace")
	_pick_template(ui, "nudge"); ui._load_template(); ui._clone_template()
	ui._tabs.current_tab = 0; await _frames()
	var offense: OptionButton = _node(ui, "timing.offense")
	var defense: OptionButton = _node(ui, "timing.defense")
	_expect(offense.get_item_text(offense.selected) == "After", "die change after the first roll reads as After: " + offense.get_item_text(offense.selected))
	_expect(_option_disabled(offense, "before") and _option_disabled(offense, "any") and not _option_disabled(offense, "after"), "die effects cannot play before rolling")
	_expect(["before", "after", "any"].all(func(c): return _option_disabled(defense, c)), "offensive die effects have no Defense timing")
	_pick_template(ui, "brace"); ui._load_template(); ui._clone_template()
	_line_edit(ui, "id", "early_guard"); _line_edit(ui, "name", "Early Guard")
	ui._tabs.current_tab = 0; await _frames()
	offense = _node(ui, "timing.offense"); defense = _node(ui, "timing.defense")
	_expect(defense.get_item_text(defense.selected) == "Any time" and offense.get_item_text(offense.selected) == "Not playable", "prevention reads as Defense any time")
	_expect(defense.tooltip_text.contains("until you Pass"), "timing hover explains the choice: " + defense.tooltip_text)
	_choose(defense, "after"); await _frames()
	_expect(ui.draft.program.windows == ["defense_after_roll", "defense_reaction", "damage_reaction"], "After maps to the after-roll windows: " + str(ui.draft.program.windows))
	ui._validate(); await _frames()
	_expect(ui._preview.text.contains("Play: Defense, after your first defense roll."), "preview states After timing: " + ui._preview.text)
	_choose(_node(ui, "timing.defense"), "before"); await _frames()
	_expect(ui.draft.program.windows == ["defense_before_roll"], "Before maps to the before-roll window: " + str(ui.draft.program.windows))
	ui._validate(); await _frames()
	_expect(ui._preview.text.contains("Play: Defense, before your first defense roll.") and ui._status.text.contains("Defense before"), "preview and status state Before timing: " + ui._status.text)
	await _click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "publish before-roll card: " + ui._error.text)
	characters.queue_free(); await process_frame
	print("CARD TIMING CHOICES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
