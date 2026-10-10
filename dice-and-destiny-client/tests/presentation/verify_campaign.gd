extends "res://tests/presentation/verify_character_deck_editing.gd"
## The campaign loop through pointer input: menu → campaign → battle → reward →
## spend XP in Prepare → the next encounter fights with the new deck.

const CAMPAIGN_SCRIPT := "res://app/screens/campaign/campaign_screen.gd"
const BATTLE_SCRIPT := "res://app/screens/battle/battle_screen.gd"
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 6: await process_frame
	var campaign_button: Button = menu._menu_actions[2]
	_expect(campaign_button.text == "Campaign" and campaign_button.get_index() == 0, "menu leads with the Campaign button")
	await _click(campaign_button)
	for frame in 6: await process_frame
	var campaign: Control = _find(CAMPAIGN_SCRIPT)
	_expect(campaign != null, "Campaign opens from the menu")
	if campaign == null: _finish(); return

	# No campaigns yet: start one from the Starter sheet with the default name.
	_expect(campaign._saves_view.visible and not campaign._play_view.visible and "No campaigns yet" in _text(campaign._save_list), "Campaign opens on the empty save list")
	_expect(campaign._new_sheet == "starter" and campaign._new_name.text == "Starter campaign" and "100 bonus XP" in campaign._new_note.text, "a new campaign defaults to the Starter sheet")
	await _capture(campaign, "campaign-saves-empty")
	await _click(_campaign_control(campaign, "campaign.begin"))
	for frame in 4: await process_frame
	_expect(campaign._play_view.visible and campaign._title.text == "Starter campaign" and not campaign.save_id.is_empty(), "Begin opens the new campaign")
	var first_save: String = campaign.save_id
	_expect(campaign._path.find_children("Encounter*", "PanelContainer", false, false).size() == 3, "three encounters are on the path")
	_expect("Starter · 12 health · 100 XP to spend" in campaign._summary.text, "the campaign starts from the Starter sheet with 100 bonus XP")
	_expect("ENCOUNTER 1  ·  NEXT" in _text(campaign._path) and "Brine Mask" in _text(campaign._path), "first encounter is next and names its opponent")
	_expect(campaign._fight.text == "Fight · Tide Pool" and not campaign._banner.visible and not campaign._fight.disabled, "Fight offers the first encounter")

	# A second campaign from the Adventurer sheet is its own save.
	await _click(campaign._all_saves)
	await _click(_campaign_control(campaign, "campaign.sheet.adventurer"))
	_expect(campaign._new_name.text == "Adventurer campaign", "the default name follows the chosen sheet")
	campaign._new_name.text = "Second run"; campaign._new_name.text_changed.emit("Second run")
	await _click(_campaign_control(campaign, "campaign.begin"))
	for frame in 4: await process_frame
	_expect(campaign._title.text == "Second run" and campaign._summary.text.begins_with("Adventurer") and campaign.save_id != first_save, "a second campaign starts from the Adventurer sheet")
	var second_save: String = campaign.save_id
	await _click(campaign._all_saves)
	_expect(campaign._save_list.get_child_count() == 2 and "Starter campaign" in _text(campaign._save_list) and "Second run" in _text(campaign._save_list), "both campaigns are listed")
	await _capture(campaign, "campaign-saves")

	# The editors' Progression mode never touches a campaign save.
	var catalogs: Dictionary = runtime.character_catalogs("progression").result
	var bought: Dictionary = runtime.purchase_progression("starter", "buy_card", "tip_it", int(catalogs.starter.progression.revision), int(catalogs.starter.economy.card_prices.get("tip_it", catalogs.starter.economy.default_card_price)))
	_expect(bought.get("ok") == true, "Progression mode buys into its own save")
	await _click(_campaign_control(campaign, "campaign.continue." + first_save))
	_expect(campaign._title.text == "Starter campaign" and "12 health · 100 XP to spend" in campaign._summary.text and not campaign._fight.disabled, "the campaign save ignores the editors' purchases")

	# Delete the second campaign after confirming.
	await _click(campaign._all_saves)
	await _click(_campaign_control(campaign, "campaign.delete." + second_save))
	_expect(campaign._confirm_delete.visible and "Second run" in campaign._confirm_delete.dialog_text, "deleting asks first")
	campaign._confirm_delete.get_cancel_button().pressed.emit()
	for frame in 2: await process_frame
	_expect(campaign._save_list.get_child_count() == 2, "cancelling keeps the campaign")
	await _click(_campaign_control(campaign, "campaign.delete." + second_save))
	campaign._confirm_delete.get_ok_button().pressed.emit()
	for frame in 4: await process_frame
	_expect(campaign._save_list.get_child_count() == 1 and "Second run" not in _text(campaign._save_list), "confirming deletes only that campaign")
	await _click(_campaign_control(campaign, "campaign.continue." + first_save))
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

	# Spend the reward in Prepare: buy a new base card.
	await _click(campaign._deck)
	for frame in 8: await process_frame
	var prepare: Control = _find("res://app/screens/campaign/campaign_prepare.gd")
	_expect(prepare != null and not campaign.visible and prepare.character_id == "starter", "Prepare opens on the campaign character")
	if prepare == null: _finish(); return
	var health := int(prepare.data.health)
	var price := int(prepare._base_offer("take_stock").cost)
	await _tap(prepare._add_button)
	await _tap(_prepare_control(prepare, "prepare.buy.take_stock"))
	for frame in 4: await process_frame
	_expect(int(prepare.data.health) == health + 1 and int(prepare.data.xp) == xp - price, "earned XP buys a card")
	xp -= price
	await _tap(_prepare_control(prepare, "prepare.back"))
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
	var saved: Dictionary = status.result.saves.filter(func(s): return s.id == first_save)[0]
	_expect(int(saved.xp) == xp and int(saved.campaign.victories) == 1, "the campaign save agrees")
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

## Scroll a control into view, as a player would, then click it.
func _tap(button: Button) -> void:
	_expect(button != null, "control present")
	if button == null: return
	var scroll := button.get_parent()
	while scroll != null and not scroll is ScrollContainer: scroll = scroll.get_parent()
	if scroll != null:
		scroll.ensure_control_visible(button)
		for frame in 2: await process_frame
	await _click(button)

func _campaign_control(screen: Node, key: String) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("campaign_control", "") == key and not button.is_queued_for_deletion(): return button
	return null

func _prepare_control(screen: Node, key: String) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("prepare_control", "") == key and not button.is_queued_for_deletion(): return button
	return null

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
