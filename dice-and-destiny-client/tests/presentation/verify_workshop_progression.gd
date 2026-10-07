extends "res://tests/presentation/verify_character_deck_editing.gd"
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var data: Dictionary = runtime.card_authoring().result
	var card: Dictionary = data.templates.brace.duplicate(true)
	card.id = "branching_ward"; card.name = "Branching Ward"
	card.economy = {"buy": 20, "sell": 7, "copy_limit": 1, "upgrades": [{"to": "brace_plus", "xp": 5}, {"to": "emergency_ward", "xp": 8}]}
	_expect(runtime.card_authoring("publish_card", card, int(data.revision)).get("ok", false), "publish configurable economy")
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	screen.inspect_entry("cards", card.id)
	for frame in 5: await process_frame
	await _click(_control(screen, "buy." + card.id)); await _click(screen._purchase_confirm)
	_expect(screen._card_count(card.id) == 1 and int(screen.catalogs.adventurer.progression.xp) == 80, "authored buy price charged")
	_expect(_control(screen, "buy." + card.id).disabled, "copy cap disables purchase")
	_expect(_control(screen, "sell." + card.id).text.contains("7 XP"), "independent sale price displayed")
	await _click(_control(screen, "sell." + card.id)); await _click(screen._purchase_confirm)
	_expect(screen._card_count(card.id) == 0 and int(screen.catalogs.adventurer.progression.xp) == 87, "authored sale price credited")
	await _click(_control(screen, "buy." + card.id)); await _click(screen._purchase_confirm)
	var first = _control(screen, "upgrade." + card.id + ".brace_plus")
	var second = _control(screen, "upgrade." + card.id + ".emergency_ward")
	_expect(first != null and second != null, "both authored upgrade branches available")
	await _click(second)
	_expect(screen._purchase_details.get_child_count() > 0 and screen._pending_purchase.target_id == "emergency_ward", "review quotes selected branch")
	await _click(screen._purchase_confirm)
	_expect(screen._card_count(card.id) == 0 and screen._card_count("emergency_ward") == 1, "selected branch replaces only owned card")
	_expect(int(screen.catalogs.adventurer.progression.xp) == 59, "selected branch charges 8 XP")
	screen.queue_free(); await process_frame
	var sandbox = SCREEN.new(); sandbox.loadout_mode = "sandbox"; root.add_child(sandbox)
	for frame in 5: await process_frame
	sandbox.inspect_entry("cards", card.id)
	sandbox._set_card_count(card.id, 2)
	_expect(sandbox._card_count(card.id) == 1 and sandbox._add_copy.disabled, "Sandbox edit handler enforces authored copy cap")
	sandbox.queue_free(); await process_frame
	print("WORKSHOP PROGRESSION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
