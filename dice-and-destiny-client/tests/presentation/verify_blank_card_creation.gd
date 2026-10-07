extends "res://tests/presentation/verify_card_creation_guided.gd"
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")

func _run() -> void:
	root.size = Vector2i(1440, 900)
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 3; await _frames()
	var ui = characters.get_node("CardCreationWorkspace")
	_expect(ui._template.selected == -1 and ui.draft.id == "" and ui.draft.name == "", "creator opens without a template")
	_expect(ui.draft.program.steps.is_empty() and ui.draft.economy.upgrades.is_empty() and ui._art.texture == null, "blank card has no inherited effects, upgrades or art")
	_expect(ui._publish_button.disabled and ui._deck_button.disabled, "incomplete blank cannot publish or open a deck")
	await _click(_node(ui, "clone")); await _click(_node(ui, "load"))
	_expect(ui.draft.id == "" and ui._template.selected == -1, "optional template actions require a selection")
	for tab in [1, 2, 3, 0]:
		ui._tabs.current_tab = tab; await _frames()
		_expect(ui._tabs.current_tab == tab, "blank can visit and leave every tab")
	await _click(_node(ui, "back")); await _frames()
	_expect(not characters.has_node("CardCreationWorkspace"), "untouched blank exits without a dirty prompt")
	characters._tabs.current_tab = 3; await _frames(); ui = characters.get_node("CardCreationWorkspace")
	# New blank must also clear all fields of a loaded specialized/template card.
	_pick_template(ui, "culture_flask"); ui._load_template()
	await _click(_node(ui, "new")); await _frames()
	_expect(not ui.draft.has("mechanic") and ui.draft.program.steps.is_empty(), "blank clears specialized mechanic")
	_line_edit(ui, "name", "Keep this draft")
	await _click(_node(ui, "new")); await _frames()
	_expect(ui._pending_bar.visible, "starting blank protects unpublished edits")
	await _click(_node(ui, "keep_editing"))
	_expect(ui.draft.name == "Keep this draft", "Keep editing retains draft")
	ui._tabs.current_tab = 3; await _frames(); ui._json.text = "{ broken"; await _frames()
	await _click(_node(ui, "new")); await _frames(); await _click(_node(ui, "discard_changes")); await _frames()
	_expect(ui._tabs.current_tab == 0 and not ui._json_dirty and ui.draft.name == "", "blank safely discards malformed JSON after confirmation")
	_line_edit(ui, "id", "blank_energy_card"); _line_edit(ui, "name", "Fresh Resolve")
	ui._validate(); _expect(ui._publish_button.disabled, "named card without effects cannot publish")
	ui._tabs.current_tab = 1; await _frames()
	_choose(_node(ui, "steps.effect"), "energy")
	await _click_visible(ui, _node(ui, "steps.add")); await _frames()
	_node(ui, "steps.0.param.amount").value = 3
	ui._validate()
	_expect(not ui._publish_button.disabled and ui._preview.text.contains("3 energy"), "effect authored from blank validates and previews")
	ui._tabs.current_tab = 0; ui._scroll.scroll_vertical = 0; await _frames()
	if DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(); DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		root.get_texture().get_image().save_png("res://.godot/layout-review/blank-card-creation.png")
	await _click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "blank-authored card publishes through native validation: " + ui._error.text)
	await _click(ui._deck_button); await _frames()
	_expect(characters.selected_id == "blank_energy_card", "published blank card opens in deck")
	for id in characters._deck_counts(characters._drafts.adventurer).keys(): characters._set_card_count(id, 0)
	characters.inspect_entry("cards", "blank_energy_card"); characters._quantity.value = 12
	await _click(characters._apply)
	_expect(not characters._dirty("adventurer") and characters._card_count("blank_energy_card") == 12, "blank-authored card equips and saves")
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer"); gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("blank-card-authoring", 37)
	var action := {}
	for candidate in result.get("legal_actions", []):
		var ids: Array = candidate.get("payload", {}).get("card_ids", [])
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances[ids[0]].definition_id == "blank_energy_card": action = candidate; break
	_expect(not action.is_empty(), "blank-authored card is playable in real battle")
	if not action.is_empty():
		var before: int = result.snapshot.actors.blade.energy_points
		result = gateway.submit(JSON.stringify(action))
		_expect(result.get("ok", true) and result.snapshot.actors.blade.energy_points == before + 3, "authored effect grants exactly three energy")
	characters.queue_free(); await process_frame
	print("BLANK CARD CREATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
