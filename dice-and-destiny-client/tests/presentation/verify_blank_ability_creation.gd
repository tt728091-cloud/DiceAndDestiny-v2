extends "res://tests/presentation/verify_ability_creation.gd"

func _run() -> void:
	root.size = Vector2i(1440, 900)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 4; await _frames()
	var ui = characters.get_node("AbilityCreationWorkspace")
	_expect(ui.draft.id == "" and ui.draft.name == "" and ui._template.selected == -1, "creator starts blank without a template")
	_expect(ui._publish.disabled, "empty draft cannot publish")
	_expect(_node(ui, "ability_type").get_global_rect().position.y > _node(ui, "name").get_global_rect().end.y, "type selection is below name")
	_line_edit(ui, "id", "blank_attack"); _line_edit(ui, "name", "Blank Attack")
	ui._tabs.current_tab = 1; await _frames()
	await _click_visible(ui, _node(ui, "add.activation_tiers")); await _frames()
	_expect(ui._validate(), "offensive ability built without template: " + ui._status.text)
	var offensive_rules: String = ui._preview.text
	# Use the front-page selector, retain both sets of draft settings, and ensure
	# the inactive type's structures are never sent to native validation.
	ui._tabs.current_tab = 0; await _frames()
	_choose(_node(ui, "ability_type"), "defensive"); await _frames()
	_expect(not ui.draft.has("qualification") and not ui.draft.has("targeting") and ui.draft.resolution.operations.is_empty(), "defense starts with its own empty effects")
	_expect(not ui._validate() and ui._publish.disabled, "empty defense cannot publish")
	ui._tabs.current_tab = 1; await _frames()
	var effects: OptionButton = _node(ui, "add_kind.operations")
	_expect(_option_disabled(effects, "deal_damage") and not _option_disabled(effects, "prevent_damage"), "defense offers appropriate effects")
	_choose(effects, "prevent_damage"); await _click_visible(ui, _node(ui, "add.operations")); await _frames()
	_node(ui, "operations.amount").value = 3
	_expect(ui._validate(), "fixed defense built without template: " + ui._status.text)
	ui._tabs.current_tab = 0; await _frames()
	_choose(_node(ui, "ability_type"), "offensive"); await _frames()
	_expect(ui._validate() and ui._preview.text == offensive_rules, "switch restores offensive draft")
	_expect(not ui.draft.has("selection") and not ui.draft.has("resolution"), "offense excludes defensive structures")
	await _click(ui._publish); _expect(ui._status.text.begins_with("Published"), "blank offense publishes")
	_expect(_node(ui, "ability_type").disabled, "existing published ID keeps native type protection")
	# A new identity can switch type without a template, restoring its earlier
	# defensive configuration and preserving the entered identity/cost.
	_line_edit(ui, "id", "blank_defense"); _line_edit(ui, "name", "Blank Defense")
	_expect(not _node(ui, "ability_type").disabled, "new ID enables type choice")
	_choose(_node(ui, "ability_type"), "defensive"); await _frames()
	_expect(ui.draft.name == "Blank Defense" and int(ui.draft.resolution.operations[0].amount) == 3, "type change preserves identity and defensive work")
	_expect(ui._validate(), "restored defense validates")
	await _click(ui._publish); _expect(ui._status.text.begins_with("Published"), "blank defense publishes")
	ui._tabs.current_tab = 3; await _frames()
	# Both newly published definitions appear as compatible board entries.
	_expect(ui.catalog.compatible.adventurer.has("blank_attack") and ui.catalog.compatible.adventurer.has("blank_defense"), "both types compatible with Adventurer dice")
	ui._board.offensive = ["blank_attack"]; ui._board.defensive = ["blank_defense"]
	await _click_visible(ui, _node(ui, "assign")); _expect(ui._status.text.begins_with("Ability board saved"), "both blank abilities assigned")
	var runtime = root.get_node("LearnedBattleRuntime")
	for mode in ["sandbox", "progression"]:
		var loaded: Dictionary = runtime.character_catalogs(mode)
		var board: Dictionary = loaded.result.adventurer.combatants.adventurer.ability_board
		_expect(board.offensive == ["blank_attack"] and board.defensive == ["blank_defense"], "assigned definitions persist in " + mode)
	ui._tabs.current_tab = 0; await _frames()
	await _click(_node(ui, "new")); await _frames()
	_expect(ui.draft.id == "" and ui.draft.qualification.activation_tiers.is_empty(), "New blank clears prior definitions")
	_choose(_node(ui, "ability_type"), "defensive"); await _frames()
	_expect(ui.draft.resolution.operations.is_empty(), "New blank clears both type caches")
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/blank-ability-creation.png")
	characters.queue_free(); await process_frame
	print("BLANK ABILITY CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
