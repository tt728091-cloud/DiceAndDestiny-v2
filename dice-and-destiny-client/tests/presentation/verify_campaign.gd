extends "res://tests/presentation/verify_character_deck_editing.gd"
## The campaign loop through pointer input: menu → campaign → battle → reward →
## spend XP in the deck editors → the next encounter fights with the new deck.

const CAMPAIGN_SCRIPT := "res://app/screens/campaign/campaign_screen.gd"
const BATTLE_SCRIPT := "res://app/screens/battle/battle_screen.gd"
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	# A legacy card equipped outside the campaign (Progression mode) blocks it.
	var catalogs: Dictionary = runtime.character_catalogs("progression").result
	var tip_price := int(catalogs.starter.economy.card_prices.get("tip_it", catalogs.starter.economy.default_card_price))
	var bought: Dictionary = runtime.purchase_progression("starter", "buy_card", "tip_it", int(catalogs.starter.progression.revision), tip_price)
	_expect(bought.get("ok") == true, "Progression mode can still equip a legacy card")
	var refused: Dictionary = runtime.purchase_progression("starter", "buy_card", "tip_it", int(bought.result.revision), tip_price, "", "", true)
	_expect(refused.get("ok") != true and "card-tree" in str(refused.get("error", "")), "campaign editing refuses buying a legacy card")
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 6: await process_frame
	var campaign_button: Button = menu._menu_actions[2]
	_expect(campaign_button.text == "Campaign" and campaign_button.get_index() == 0, "menu leads with the Campaign button")
	await _click(campaign_button)
	for frame in 6: await process_frame
	var campaign: Control = _find(CAMPAIGN_SCRIPT)
	_expect(campaign != null, "Campaign opens from the menu")
	if campaign == null: _finish(); return
	_expect(campaign._path.find_children("Encounter*", "PanelContainer", false, false).size() == 3, "three encounters are on the path")
	_expect("%d XP to spend" % (100 - tip_price) in campaign._summary.text, "campaign shows the starting XP less the legacy purchase")
	_expect("ENCOUNTER 1  ·  NEXT" in _text(campaign._path) and "Brine Mask" in _text(campaign._path), "first encounter is next and names its opponent")
	_expect(campaign._fight.text == "Fight · Tide Pool" and not campaign._banner.visible, "Fight offers the first encounter")
	_expect(campaign.character_id == "starter" and campaign._summary.text.begins_with("Starter"), "the campaign defaults to the Starter")
	_expect(campaign._character_choice.visible and campaign._character_choice.item_count == 2, "the Adventurer can be chosen too")
	campaign._character_choice.select(1); campaign._character_choice.item_selected.emit(1)
	for frame in 3: await process_frame
	_expect(campaign.character_id == "adventurer" and campaign._summary.text.begins_with("Adventurer"), "choosing the Adventurer shows its own progress")
	campaign._character_choice.select(0); campaign._character_choice.item_selected.emit(0)
	for frame in 3: await process_frame
	_expect(campaign._fight.disabled and "Tip It" in campaign._message.text, "an equipped legacy card blocks the campaign and is named")
	await _click(campaign._deck)
	for frame in 8: await process_frame
	var cleanup = _find("res://app/screens/character/character_creation.gd")
	_expect(cleanup != null and cleanup.campaign_mode, "the campaign opens its editors in campaign mode")
	if cleanup == null: _finish(); return
	_expect("tip_it" not in cleanup._eligible_card_ids() and "take_stock" in cleanup._eligible_card_ids(), "the campaign library offers only card-tree cards")
	_expect(cleanup._has_type_conflicts() and "only card-tree cards" in cleanup._save_status.text, "the equipped legacy card is flagged")
	cleanup.inspect_entry("cards", "tip_it")
	for frame in 4: await process_frame
	_expect("The campaign uses only card-tree cards" in _text(cleanup._details), "the legacy card explains why it is blocked")
	_expect(_control(cleanup, "buy.tip_it").disabled and not _control(cleanup, "sell.tip_it").disabled, "a legacy card can be sold but not bought")
	await _click(_control(cleanup, "sell.tip_it"))
	await _click(cleanup._purchase_confirm)
	_expect(cleanup._card_count("tip_it") == 0 and not cleanup._has_type_conflicts(), "selling the legacy card clears the conflict")
	await _click(_control(cleanup, "back"))
	for frame in 6: await process_frame
	_expect(not campaign._fight.disabled and "100 XP to spend" in campaign._summary.text, "the sale refunds its XP and unblocks the campaign")
	await _capture(campaign, "campaign-start")

	# Fight until a victory; a defeat on the way must earn nothing and repeat.
	var xp := 100
	var saw_defeat := false
	var won := false
	for attempt in 30:
		await _click(campaign._fight)
		for frame in 8: await process_frame
		var battle: Control = _find(BATTLE_SCRIPT)
		_expect(battle != null and battle._view.campaign.encounter.id == "tide_pool", "Fight starts the next encounter")
		if battle == null: _finish(); return
		var gateway: RefCounted = battle.gateway
		var result := _play_out(gateway, battle.initial_result)
		battle.queue_free(); await process_frame
		battle = BATTLE_SCREEN.instantiate()
		battle.initial_result = result; battle.gateway = gateway; battle.learned_battle_mode = true
		root.add_child(battle); await process_frame
		battle._director.clear(); battle._render()
		for frame in 4: await process_frame
		var reward: Label = battle.find_child("CampaignReward", true, false)
		_expect(reward != null, "the result screen shows the campaign reward")
		_expect(_find_button(battle, "Rematch · Same Seats") == null and _find_button(battle, "Continue · Spend XP & Next Battle") != null, "a campaign result continues the campaign instead of a rematch")
		won = result.battle_result == "victory"
		print("campaign attempt %d: %s" % [attempt + 1, result.battle_result])
		if won:
			_expect(reward != null and "+20 XP · 120 XP to spend" in reward.text, "victory awards the encounter's XP")
		else:
			saw_defeat = saw_defeat or result.battle_result == "defeat"
			_expect(reward != null and "No XP earned" in reward.text, "a loss earns nothing")
		await _capture(battle, "campaign-result-%s" % result.battle_result)
		await _click(_find_button(battle, "Continue · Spend XP & Next Battle"))
		for frame in 6: await process_frame
		campaign = _find(CAMPAIGN_SCRIPT)
		_expect(campaign != null and _find(BATTLE_SCRIPT) == null, "Continue returns to the campaign")
		if campaign == null: _finish(); return
		if won: break
		_expect(campaign._fight.text == "Fight · Tide Pool" and "%d XP to spend" % xp in campaign._summary.text, "after a loss the same encounter waits")
	_expect(won, "a campaign victory within thirty battles")
	if not won: _finish(); return
	xp += 20
	_expect(campaign._banner.visible and "+20 XP" in campaign._banner.text, "campaign banner reports the reward")
	_expect("%d XP to spend" % xp in campaign._summary.text and campaign._fight.text == "Fight · Sunken Belfry", "victory advances to the second encounter")
	_expect("ENCOUNTER 1  ·  CLEARED" in _text(campaign._path), "the first encounter shows cleared")
	await _capture(campaign, "campaign-after-victory")

	# Spend the reward in the progression editors, locked to XP spending.
	await _click(campaign._deck)
	for frame in 8: await process_frame
	var editor = _find("res://app/screens/character/character_creation.gd")
	_expect(editor != null and not campaign.visible, "Deck & Card Trees opens the editors")
	if editor != null: _expect(editor.character_id == "starter", "the editors open on the campaign character")
	if editor == null: _finish(); return
	_expect(editor.loadout_mode == "progression" and editor._mode_choice.disabled, "the campaign opens the editors locked to Progression")
	_expect(_control(editor, "back").text == "Back to campaign", "the editors return to the campaign")
	var health: int = editor._health()
	var price := int(editor.catalogs.starter.economy.card_prices.get("take_stock", 10))
	editor.inspect_entry("cards", "take_stock")
	for frame in 4: await process_frame
	await _click(_control(editor, "buy.take_stock"))
	await _click(editor._purchase_confirm)
	_expect(editor._health() == health + 1 and int(editor.catalogs.starter.progression.xp) == xp - price, "earned XP buys a card")
	xp -= price
	await _click(_control(editor, "back"))
	for frame in 6: await process_frame
	_expect(campaign.visible and "%d health · %d XP to spend" % [health + 1, xp] in campaign._summary.text, "campaign reflects the new deck and XP")

	# The next encounter fights with the changed deck.
	await _click(campaign._fight)
	for frame in 8: await process_frame
	var next_battle: Control = _find(BATTLE_SCRIPT)
	_expect(next_battle != null and next_battle._view.campaign.encounter.id == "sunken_belfry", "the second encounter starts")
	if next_battle != null:
		_expect(int(next_battle._view.actor("blade").max_health) == health + 1, "the next battle uses the purchased card")
		_expect("CAMPAIGN 2/3 · SUNKEN BELFRY" in _text(next_battle), "the battle names the campaign encounter")
		_expect(str(next_battle._view.actor("goblin").get("definition_id", "")) == "drowned_oracle_bell_diver", "the second encounter fights the Bell Diver")
		next_battle.queue_free(); await process_frame
	var status: Dictionary = runtime.campaign_status()
	_expect(int(status.result.characters.starter.xp) == xp and int(status.result.characters.starter.campaign.victories) == 1, "authority ledger agrees")
	if not saw_defeat: print("campaign: no defeat occurred; the loss path was not exercised this run")
	_finish()

## Script tests get an empty authored root; campaign decks need the published
## card trees, so copy them in (read-only use of the tracked files).
func _seed_card_trees() -> void:
	var target := OS.get_environment("DICE_AND_DESTINY_AUTHORED_ROOT")
	var source := OS.get_environment("DICE_AND_DESTINY_CONTENT_ROOT").path_join("authored")
	_expect(not target.is_empty() and target.simplify_path() != source.simplify_path(), "the test has a disposable authored root")
	if target.is_empty() or target.simplify_path() == source.simplify_path(): return
	for name in ["authored_cards.json", "economy_admin.json"]:
		_expect(DirAccess.copy_absolute(source.path_join(name), target.path_join(name)) == OK, "copied %s into the disposable authored root" % name)

func _finish() -> void:
	print("CAMPAIGN: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

## Plays the battle through the authority with a simple human proxy.
func _play_out(gateway: RefCounted, view: Dictionary) -> Dictionary:
	var guard := 0
	while view.get("battle_result", "") not in ["victory", "defeat", "draw"] and guard < 1500:
		guard += 1
		if view.get("learned_policy", {}).get("model_turn") == true:
			view = gateway.advance_model()
		else:
			var actions: Array = view.get("legal_actions", [])
			var chosen: Dictionary = actions[0] if not actions.is_empty() else {}
			for kind in ["planning_roll", "planning_select_ability", "planning_reroll", "roll_dice", "planning_pass", "pass", "commit_interaction"]:
				var found := false
				for index in range(actions.size() - 1, -1, -1):
					if str(actions[index].get("type", "")) == kind: chosen = actions[index]; found = true; break
				if found: break
			view = gateway.submit(JSON.stringify(chosen))
		if view.get("accepted") != true:
			_expect(false, "campaign battle command failed: %s" % str(view.get("error", "")))
			break
	_expect(view.get("battle_result", "") in ["victory", "defeat", "draw"], "campaign battle reached a result")
	return view

func _find(script_path: String) -> Control:
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == script_path and not child.is_queued_for_deletion(): return child
	return null

func _find_button(node: Node, text: String) -> Button:
	for button in node.find_children("*", "Button", true, false):
		if button.text == text: return button
	return null
