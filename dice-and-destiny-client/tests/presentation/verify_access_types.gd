extends "res://tests/presentation/verify_economy_admin.gd"

func _choose_type(screen, kind: String, id: String, type_id: String) -> void:
	if kind != "characters":
		screen._admin_tabs.current_tab = 0 if kind == "cards" else 1
		for frame in 4: await process_frame
		screen._admin_search.text = str(screen.catalogs[screen.character_id][kind][id].name)
		screen._admin_search.text_changed.emit(screen._admin_search.text)
		for frame in 4: await process_frame
	var choice: OptionButton = screen._admin_types[kind][id]
	for index in choice.item_count:
		if str(choice.get_item_metadata(index)) == type_id:
			choice.select(index); choice.item_selected.emit(index); break
	for frame in 4: await process_frame

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect("pinprick" not in screen._eligible_card_ids() and "black_fingerprint" not in screen._eligible_card_ids(), "Adventurer excludes themed pools")
	for id in screen.ROSTER:
		screen.select_character(id)
		_expect("brace" in screen._eligible_card_ids(), "General is available to " + id)
	screen.select_character("venom")
	_expect("pinprick" in screen._eligible_card_ids() and "black_fingerprint" not in screen._eligible_card_ids(), "Venom has matching pool")
	for frame in 4: await process_frame
	await _click(screen._admin_button)
	await _choose_type(screen, "cards", "pinprick", "curse")
	_expect("incompatible equipped items" in screen._admin_preview.text and not screen._admin_save.disabled, "admin warns without deleting owned cards")
	await _capture(screen, "types-card-reassignment")
	await _click(screen._admin_save)
	_expect("pinprick" not in screen._eligible_card_ids() and screen._card_count("pinprick") == 1, "reassignment removes eligibility but retains owned card")
	screen.inspect_entry("cards", "pinprick")
	for frame in 4: await process_frame
	_expect(_control(screen, "buy.pinprick").disabled and not _control(screen, "sell.pinprick").disabled, "incompatible item sellable but not purchasable")
	_expect("Type conflict" in screen._save_status.text, "battle conflict explained")
	screen.select_character("curse"); screen.inspect_entry("cards", "pinprick")
	for frame in 4: await process_frame
	_expect("pinprick" in screen._eligible_card_ids(), "Curse gains reassigned card")
	await _click(_control(screen, "buy.pinprick")); await _click(screen._purchase_confirm)
	_expect(screen._card_count("pinprick") == 1 and int(screen.catalogs.curse.progression.xp) == 90, "new pool owner can buy")
	await _click(screen._admin_button)
	await _choose_type(screen, "cards", "pinprick", "general")
	await _choose_type(screen, "abilities", "adventurer_guard_plus", "venom")
	await _capture(screen, "types-ability-reassignment")
	await _click(screen._admin_save)
	screen.select_character("adventurer"); screen._tabs.current_tab = 0; screen.inspect_entry("abilities", "adventurer_guard")
	for frame in 4: await process_frame
	_expect("pinprick" in screen._eligible_card_ids(), "General override opens card for Adventurer")
	_expect(_control(screen, "upgrade.adventurer_guard").disabled, "mismatched ability upgrade blocked")
	await _click(screen._admin_button)
	await _choose_type(screen, "characters", "adventurer", "venom")
	await _click(screen._admin_save)
	_expect(not _control(screen, "upgrade.adventurer_guard").disabled, "character type enables matching upgrade")
	await _click(_control(screen, "upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect(screen.character.ability_board.defensive == ["adventurer_guard_plus"], "matching ability upgrade works")
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		await _click(screen._admin_button)
		await _choose_type(screen, "characters", "adventurer", "general")
		_expect(root.get_visible_rect().encloses(screen._admin_save.get_global_rect()), "admin save fits")
		_expect(root.get_visible_rect().encloses(screen._admin_types.characters.adventurer.get_global_rect()), "character type selector fits")
		await _capture(screen, "types-admin-%d" % width)
		await _click(_control(screen, "admin.close"))
		_expect(screen._character_type() == "venom", "Cancel preserves saved type")
	screen.queue_free(); await process_frame
	screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect(screen._character_type() == "venom" and screen._type_allowed("abilities", "adventurer_guard_plus"), "types persist across reopen")
	screen.queue_free(); await process_frame
	print("ACCESS TYPES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
