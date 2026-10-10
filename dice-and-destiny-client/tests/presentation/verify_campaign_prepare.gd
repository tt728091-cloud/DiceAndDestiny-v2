extends "res://tests/presentation/verify_campaign.gd"
## The campaign's Prepare screen through pointer input: select a deck card, see
## its tree, upgrade and trade a copy down in place, preview a tree card, buy a
## base, sell, store and re-add a copy, and upgrade an ability.

const PREPARE_SCRIPT := "res://app/screens/campaign/campaign_prepare.gd"

func _run() -> void:
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	var campaign = preload("res://app/screens/campaign/campaign_screen.gd").new(); root.add_child(campaign)
	for frame in 6: await process_frame
	_expect(campaign._deck.text == "Prepare · Deck & Abilities", "the campaign offers Prepare")
	await _click(campaign._deck)
	for frame in 8: await process_frame
	var prepare: Control = _find(PREPARE_SCRIPT)
	_expect(prepare != null and not campaign.visible, "Prepare opens from the campaign")
	if prepare == null: _finish(); return
	_expect(prepare.character_id == "starter" and prepare._title.text == "Prepare · Starter", "Prepare is about the campaign character")
	_expect(prepare._health.text == "12" and prepare._xp.text == "100", "Prepare shows health and XP")
	_expect(_prepare_control(prepare, "prepare.card.steady_guard") != null and _prepare_control(prepare, "prepare.ability.adventurer_guard") != null, "the deck and abilities are listed")
	_expect(_prepare_control(prepare, "prepare.card.tip_it") == null, "only the character's own cards are listed")

	# Select Steady Guard: its tree appears with upgrades and a trade-down.
	await _tap(_prepare_control(prepare, "prepare.card.steady_guard"))
	for frame in 6: await process_frame
	_expect(prepare._canvas.visible and prepare._center_title.text == "Steady Guard paths", "selecting a card shows its tree")
	_expect(prepare._canvas.selected == "base" and prepare._canvas.node_states.get("base") == "owned", "the selected card is highlighted as owned")
	var up: Dictionary = prepare._trades_from("steady_guard", "").filter(func(t): return int(t.cost) > 0 and t.available)[0]
	var down: Dictionary = prepare._trades_from("steady_guard", "").filter(func(t): return int(t.cost) < 0 and t.available)[0]
	_expect(prepare._canvas.node_states.get(str(up.to_node)) == "available", "reachable upgrades glow in the tree")
	await _capture(prepare, "prepare-steady-guard")

	# Preview a tree card from the canvas, then upgrade a copy from the preview.
	prepare._canvas.node_selected.emit(str(up.to_node))
	for frame in 4: await process_frame
	var target := str(up.request.target_id)
	_expect(prepare.focus_node == str(up.to_node) and _prepare_control(prepare, "prepare.trade.steady_guard." + target) != null, "a tree card previews with its upgrade")
	await _capture(prepare, "prepare-preview")
	await _tap(_prepare_control(prepare, "prepare.trade.steady_guard." + target))
	for frame in 4: await process_frame
	_expect(prepare._health.text == "12" and prepare._xp.text == str(100 - int(up.cost)), "an upgrade keeps health and spends the XP difference")
	_expect(prepare.selection.id == target and prepare._deck_count("steady_guard") == 2 and prepare._deck_count(target) == 1, "the upgraded copy stays in the deck and is selected")
	_expect(prepare._message.text.contains("Steady Guard →"), "the trade is confirmed in words")
	await _capture(prepare, "prepare-after-upgrade")

	# Trade a Steady Guard copy down for a refund.
	await _tap(_prepare_control(prepare, "prepare.card.steady_guard"))
	await _tap(_prepare_control(prepare, "prepare.trade.steady_guard." + str(down.request.target_id)))
	for frame in 4: await process_frame
	var xp := 100 - int(up.cost) - int(down.cost)
	_expect(prepare._xp.text == str(xp) and prepare._health.text == "12", "trading down refunds XP and keeps health")

	# Add a new card from the bases, then sell one copy and store another.
	await _tap(prepare._add_button)
	for frame in 4: await process_frame
	_expect(prepare._center_content.find_child("Bases", true, false) != null and prepare._center_content.find_child("Bases", true, false).get_child_count() == prepare.data.bases.size(), "Add a new card lists every tree base")
	await _capture(prepare, "prepare-add")
	await _tap(_prepare_control(prepare, "prepare.buy.dispel"))
	for frame in 4: await process_frame
	xp -= 10
	_expect(prepare._health.text == "13" and prepare._xp.text == str(xp) and prepare.selection.id == "dispel", "buying a base adds it to the deck")
	await _tap(_prepare_control(prepare, "prepare.card.nudge"))
	await _tap(_prepare_control(prepare, "prepare.sell.nudge"))
	for frame in 4: await process_frame
	xp += 10
	_expect(prepare._health.text == "12" and prepare._xp.text == str(xp) and prepare._deck_count("nudge") == 1, "selling refunds the card's XP")
	await _tap(_prepare_control(prepare, "prepare.store.nudge"))
	for frame in 4: await process_frame
	_expect(prepare._health.text == "11" and _prepare_control(prepare, "prepare.stored.nudge") != null, "storing moves a copy out of the deck")
	await _tap(_prepare_control(prepare, "prepare.stored.nudge"))
	await _tap(_prepare_control(prepare, "prepare.equip.nudge"))
	for frame in 4: await process_frame
	_expect(prepare._health.text == "12" and _prepare_control(prepare, "prepare.stored.nudge") == null, "a stored copy can go back into the deck")

	# Abilities: upgrade Guard, then sell the tier back.
	await _tap(_prepare_control(prepare, "prepare.ability.adventurer_guard"))
	for frame in 4: await process_frame
	_expect(not prepare._canvas.visible and "NEXT TIER" in _text(prepare._center_content), "an ability shows its rules and next tier")
	await _tap(_prepare_control(prepare, "prepare.upgrade_ability.adventurer_guard"))
	for frame in 4: await process_frame
	xp -= 25
	_expect(prepare._xp.text == str(xp) and prepare.selection.id == "adventurer_guard_plus", "upgrading an ability spends XP and selects the new tier")
	await _capture(prepare, "prepare-ability")
	await _tap(_prepare_control(prepare, "prepare.downgrade_ability.adventurer_guard_plus"))
	for frame in 4: await process_frame
	xp += 25
	_expect(prepare._xp.text == str(xp) and prepare.selection.id == "adventurer_guard", "selling an ability tier back refunds it")

	# Unaffordable trades are disabled with a reason.
	for width in [1024, 1280, 1920]:
		root.size = Vector2i(width, 768 if width == 1024 else width * 9 / 16)
		for frame in 6: await process_frame
		_expect(prepare._details.get_global_rect().end.x <= root.get_visible_rect().size.x, "Prepare fits a %d-wide window" % width)
		await _capture(prepare, "prepare-%d" % width)
	root.size = Vector2i(1280, 720)

	# The battle uses the changed deck; back to the campaign shows it too.
	await _tap(_prepare_control(prepare, "prepare.back"))
	for frame in 6: await process_frame
	_expect(campaign.visible and "%d XP to spend" % xp in campaign._summary.text and "12 health" in campaign._summary.text, "the campaign reflects Prepare's trades")
	var status: Dictionary = runtime.campaign_status()
	_expect(int(status.result.characters.starter.xp) == xp, "the authority ledger agrees")
	_finish()

func _finish() -> void:
	print("CAMPAIGN PREPARE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
