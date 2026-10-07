extends "res://tests/presentation/verify_character_deck_editing.gd"

func _run() -> void:
	root.size = Vector2i(1280,720)
	var screen = SCREEN.new(); screen.loadout_mode = "progression"; root.add_child(screen)
	for frame in 8: await process_frame
	_expect("10 XP × 2 = 20 XP total" in _text(_control(screen,"entry.deck.cards.brace")), "deck shows quantity total")
	_expect("10 XP × 2 = 20 XP total" in _text(_control(screen,"entry.library.cards.brace")), "library shows owned total")
	_expect("10 XP × 0 = 0 XP total" in _text(_control(screen,"entry.library.cards.tip_it")), "unowned cards show unit price and zero total")
	screen.inspect_entry("cards","brace")
	for frame in 5: await process_frame
	_expect("10 XP × 2 = 20 XP total" in _text(screen._details), "inspector includes total")
	await _click(_control(screen,"buy.brace")); await _click(screen._purchase_confirm)
	_expect("10 XP × 3 = 30 XP total" in _text(_control(screen,"entry.deck.cards.brace")), "purchase refreshes total")
	await _click(_control(screen,"sell.brace")); await _click(screen._purchase_confirm)
	_expect("10 XP × 2 = 20 XP total" in _text(_control(screen,"entry.deck.cards.brace")), "sale refreshes total")
	await _click(_control(screen,"upgrade.brace")); await _click(screen._purchase_confirm)
	_expect("20 XP × 2 = 40 XP total" in _text(_control(screen,"entry.deck.cards.brace_plus")), "upgraded card uses configured value")
	_expect("10 XP × 1 = 10 XP total" in _text(_control(screen,"entry.deck.cards.brace")), "upgrade reduces base-card total")
	# Change the configured unit price through the admin API and refresh the view.
	var settings: Dictionary = screen.catalogs.adventurer.admin_settings.duplicate(true)
	settings.revision = int(settings.revision); settings.card_prices = {"brace":12}; settings.budgets = {"adventurer":260}
	var response: Dictionary = root.get_node("LearnedBattleRuntime").save_economy_admin(settings)
	_expect(response.get("ok",false), "admin repricing accepted")
	screen.reload_catalogs()
	_expect("12 XP × 1 = 12 XP total" in _text(_control(screen,"entry.deck.cards.brace")), "admin price refreshes multiplication")
	screen._tabs.current_tab = 0
	_expect("0 XP total · included starter ability" in _text(_control(screen,"entry.abilities.adventurer_strike")), "base ability costs are explicit")
	screen.inspect_entry("abilities","adventurer_guard")
	for frame in 5: await process_frame
	await _click(_control(screen,"upgrade.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect("25 XP total" in _text(_control(screen,"entry.abilities.adventurer_guard_plus")), "upgraded ability shows tier cost")
	_expect("25 XP total" in _text(screen._details), "ability inspector shows total")
	for width in [1024,1280,1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for tab in [0,1]:
			screen._tabs.current_tab = tab
			for frame in 6: await process_frame
			for button in screen._entry_buttons:
				for label in button.find_children("*","Label",true,false):
					_expect(label.get_global_rect().end.y <= button.get_global_rect().end.y + 1, "row contains XP and rules text")
			await _capture(screen,"xp-totals-%d-%d" % [tab,width])
	screen._tabs.current_tab = 0; screen.inspect_entry("abilities","adventurer_guard_plus")
	for frame in 5: await process_frame
	await _click(_control(screen,"downgrade.adventurer_guard_plus.adventurer_guard")); await _click(screen._purchase_confirm)
	_expect("0 XP total" in _text(_control(screen,"entry.abilities.adventurer_guard")), "downgrade resets displayed investment")
	screen.queue_free(); await process_frame
	print("ENTRY XP TOTALS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
