extends "res://tests/presentation/verify_economy_admin.gd"

func _hover(control: Control) -> void:
	var move := InputEventMouseMotion.new(); move.position = control.get_global_rect().get_center(); root.push_input(move, true)
	for frame in 6: await process_frame

func _search(screen, query: String) -> void:
	screen._admin_search.text = query; screen._admin_search.text_changed.emit(query)
	for frame in 6: await process_frame

func _run() -> void:
	root.gui_embed_subwindows = true
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	for width in [1280, 1920, 1024]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 8: await process_frame
		await _click(screen._admin_button)
		var before := JSON.stringify(screen._admin_draft)
		await _search(screen, "Brace")
		var group: Control = screen._admin_prices.brace.get_parent().get_parent()
		await _hover(group.get_child(0))
		_expect(screen._admin_item_id == "brace" and screen._admin_item_kind == "cards", "hovering the name selects the card preview")
		var cards: Array = screen._admin_item_details.find_children("*", "BattleCard", true, false)
		_expect(cards.size() == 1, "admin shows one full card")
		if cards.size() == 1:
			_expect(cards[0].definition_id == "brace" and cards[0].size == BattleCard.STANDARD_SIZE, "correct artwork and standard card dimensions")
			_expect(not cards[0].get_global_rect().intersects(group.get_global_rect()), "card preview never covers editing controls")
		_expect(screen.catalogs.adventurer.cards.brace.presentation.rules_text in _text(screen._admin_item_details), "same full card rules as character inspector")
		_expect(JSON.stringify(screen._admin_draft) == before, "hover is read-only")
		var bounds := root.get_visible_rect()
		_expect(bounds.encloses(screen._admin_item_details.get_parent().get_global_rect()), "preview stays on-screen at %d" % width)
		_expect(bounds.encloses(screen._admin_save.get_global_rect()), "save remains visible at %d" % width)
		await _capture(screen, "admin-card-preview-%d" % width)
		await _set_price(screen._admin_prices.brace, 12)
		_expect("12 XP" in screen._admin_item_context.text, "preview follows unsaved XP edits")
		var type_choice: OptionButton = screen._admin_types.cards.brace
		type_choice.select(1); type_choice.item_selected.emit(1)
		_expect(screen._type_name(str(type_choice.get_item_metadata(1))) in screen._admin_item_context.text, "preview follows unsaved type edits")
		await _search(screen, "Nudge")
		_expect(screen._admin_item_id == "", "search does not retain a hidden item's preview")
		screen._admin_prices.nudge.get_line_edit().grab_focus()
		for frame in 5: await process_frame
		_expect(screen._admin_item_id == "nudge", "keyboard focus previews an item")
		screen._admin_tabs.current_tab = 1
		for frame in 5: await process_frame
		_expect(screen._admin_item_id == "", "switching tabs clears stale card preview")
		await _search(screen, "Guard+")
		await _hover(screen._admin_types.abilities.adventurer_guard_plus.get_parent().get_parent().get_child(0))
		_expect(screen._admin_item_kind == "abilities" and screen._admin_item_id == "adventurer_guard_plus", "ability name hover updates preview")
		_expect(screen.catalogs.adventurer.abilities.adventurer_guard_plus.presentation.rules_text in _text(screen._admin_item_details), "ability preview includes full rules")
		_expect("Return to their piles" in _text(screen._admin_item_details), "upgraded defense destination is visible")
		_expect(screen._admin_item_details.find_children("*", "BattleCard", true, false).is_empty(), "ability does not reuse stale card artwork")
		await _capture(screen, "admin-ability-preview-%d" % width)
		await _click(_control(screen, "admin.close"))
		await _click(screen._admin_button)
		_expect(int(screen._admin_prices.brace.value) == 10, "preview and discarded edits do not change persisted prices")
		await _click(screen._admin_save)
		await _click(screen._admin_button)
		await _search(screen, "Brace+")
		await _hover(screen._admin_prices.brace_plus.get_parent().get_parent().get_child(0))
		_expect(screen._admin_item_id == "brace_plus", "hover works after save and reopen")
		await _click(_control(screen, "admin.close"))
	for id in screen.ROSTER:
		screen.select_character(id)
		await _click(screen._admin_button)
		screen._admin_tabs.current_tab = 1
		var ability: String = screen.character.ability_board.offensive[0]
		await _search(screen, str(screen.catalogs[id].abilities[ability].name))
		await _hover(screen._admin_types.abilities[ability].get_parent().get_parent().get_child(0))
		_expect(str(screen.character.name) in screen._admin_item_context.text, "preview names the selected character")
		_expect(screen.catalogs[id].abilities[ability].presentation.rules_text in _text(screen._admin_item_details), "preview resolves " + id + " ability rules")
		for tier in BattlePresentationCatalog.offensive_tier_summaries(ability):
			_expect(str(tier.recipe) in _text(screen._admin_item_details), "offensive dice qualification visible")
		await _click(_control(screen, "admin.close"))
	screen.queue_free(); await process_frame
	print("ADMIN ITEM PREVIEW: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
