extends "res://tests/presentation/verify_campaign.gd"
## Expedition battle statuses by pointer. An expedition save is driven to an
## ambushed challenge through the authority's map ops; the real battle screen
## then shows the Ambush status (icon, count, rules on hover), rolls eight
## dice, lets the player click exactly five to keep, returns to five, never
## grants it again, keeps it through a relaunch of the pending battle, and
## expires it unused at the end of the first Offensive segment.

const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")

func _run() -> void:
	root.gui_embed_subwindows = true
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	for size in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		root.size = size
		for frame in 3: await process_frame
		var fixture := _ambush_fixture(runtime)
		_expect(not fixture.is_empty(), "an ambushed challenge was reachable (%dx%d)" % [size.x, size.y])
		if fixture.is_empty(): continue
		await _check_ambush_battle(runtime, fixture, "%dx%d" % [size.x, size.y])
	_finish_statuses()

func _finish_statuses() -> void:
	print("EXPEDITION STATUSES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

## New expedition saves until the east tile's group can be challenged with an
## Ambush (cover or distance plus habit), as a player would do from the map.
func _ambush_fixture(runtime) -> Dictionary:
	for seed in range(1, 80):
		var created: Dictionary = runtime.campaign_new("starter", "Ambush %d" % seed, "expedition", "", seed)
		if created.get("ok") != true: _expect(false, "campaign_new failed: %s" % str(created.get("error", ""))); return {}
		var id: String = str(created.result.id)
		var view: Dictionary = _status(runtime, id)
		var hand: Array = view.hand
		if not _act(runtime, id, {"kind": "move", "target": {"q": 1, "r": 0}, "cards": [str(hand[0].id)]}): return {}
		view = _status(runtime, id); hand = view.hand
		if not _act(runtime, id, {"kind": "explore", "slot": "east", "cards": [str(hand[0].id), str(hand[1].id)]}): return {}
		if not _act(runtime, id, {"kind": "end_turn"}): return {}
		view = _status(runtime, id); hand = view.hand
		var cost := 0
		for move in view.moves:
			if int(move.to.q) == 2 and int(move.to.r) == 0 and str(move.get("blocked", "")).is_empty(): cost = int(move.cost)
		if cost < 1 or cost > hand.size(): continue
		var payment: Array = []
		for index in cost: payment.append(str(hand[index].id))
		if not _act(runtime, id, {"kind": "move", "target": {"q": 2, "r": 0}, "cards": payment}): return {}
		view = _status(runtime, id)
		var group := ""
		for hex in view.hexes:
			if int(hex.q) == 2 and int(hex.r) == 1 and hex.get("group") != null: group = str(hex.group.id)
		if group.is_empty(): continue
		if not _act(runtime, id, {"kind": "challenge", "group": group}): return {}
		var status: Dictionary = runtime.expedition_status(id).result
		var pending = status.get("pending_battle")
		if pending is Dictionary and pending.get("ambush") == true:
			return {"save_id": id, "opponent": str(pending.opponent), "count": int(pending.opponent_count), "battle_id": str(pending.id)}
	return {}

func _status(runtime, id: String) -> Dictionary:
	var response: Dictionary = runtime.expedition_status(id)
	_expect(response.get("ok") == true, "expedition_status: %s" % str(response.get("error", "")))
	return response.get("result", {}).get("view", {})

func _act(runtime, id: String, action: Dictionary) -> bool:
	var response: Dictionary = runtime.expedition_act(id, action, 0)
	_expect(response.get("ok") == true, "expedition_act %s: %s" % [str(action.kind), str(response.get("error", ""))])
	return response.get("ok") == true

func _start(runtime, fixture: Dictionary) -> Array:
	var gateway: RefCounted = LEARNED_GATEWAY.new(runtime, "seat-a", "minion:%s:%d" % [fixture.opponent, fixture.count], "starter")
	gateway.unified_defense = true
	gateway.loadout_mode = "expedition"
	gateway.campaign_save = fixture.save_id
	gateway.expedition = true
	var result: Dictionary = gateway.start_battle("ignored", 0)
	_expect(result.get("accepted") == true, "the pending expedition battle starts: %s" % str(result.get("error", "")))
	if result.get("accepted") != true: return []
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result; screen.viewer_actor_id = "blade"; screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(); screen.learned_battle_mode = true; screen.learned_human_seat = "seat-a"; screen.learned_seed = 0
	screen._auto_pass_disabled = true
	root.add_child(screen)
	for frame in 8: await process_frame
	return [gateway, screen, result]

func _inspection(screen: Node, id: String) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == id and not button.is_queued_for_deletion() and button.is_visible_in_tree(): return button
	return null

## Waits for the authority's dice and for the tray's roll animation to settle.
func _wait_dice(screen, count: int) -> void:
	var started := Time.get_ticks_msec()
	while Time.get_ticks_msec() - started < 6000:
		await process_frame
		if screen._submitting or screen._player_roll_active() or screen._view.rolled_dice("blade").size() != count: continue
		var settled := true
		for child in screen._player_dice_dock.get_children():
			if child is BattleDiceTray and not child._rolling_indices.is_empty(): settled = false
		if settled:
			for frame in 2: await process_frame
			return

func _check_ambush_battle(runtime, fixture: Dictionary, label: String) -> void:
	var started := await _start(runtime, fixture)
	if started.is_empty(): return
	var gateway: RefCounted = started[0]; var screen = started[1]; var result: Dictionary = started[2]
	_expect(str(result.snapshot.battle_id) == fixture.battle_id, "the battle uses the pending battle's id (%s)" % label)
	_expect(str(result.campaign.expedition.name) != "" and result.campaign.expedition.ambush == true, "the battle view carries the expedition context (%s)" % label)
	# The status bar: icon, count and full rules on hover.
	var profile: ActorProfile = screen._actor_profiles.blade
	_expect(profile.statuses.cells.has("ambush"), "Ambush is on the player's status bar (%s)" % label)
	if not profile.statuses.cells.has("ambush"): screen.queue_free(); return
	var cell: Control = profile.statuses.cells.ambush
	_expect(cell.get_node("Count").text == "3" and cell.modulate.a == 1.0, "Ambush shows 3 stacks (%s)" % label)
	var rules: String = BattlePresentationCatalog.status("ambush").text
	_expect("Ambush" in cell.tooltip_text and rules in cell.tooltip_text and "expires" in rules, "Ambush hover shows the full rules and expiry (%s)" % label)
	var motion := InputEventMouseMotion.new(); motion.position = cell.get_global_rect().get_center(); root.push_input(motion, true)
	for frame in 3: await process_frame
	_expect(root.gui_get_hovered_control() == cell, "the pointer hovers the Ambush icon (%s)" % label)
	var icon_rect := cell.get_global_rect()
	var viewport_rect := root.get_visible_rect()
	_expect(viewport_rect.encloses(icon_rect), "the status icon is inside the viewport (%s): %s in %s" % [label, icon_rect, viewport_rect])
	await _capture(screen, "expedition-ambush-status-" + label)
	# Roll: eight dice, then choose exactly five with the mouse.
	var roll := _inspection(screen, "battle.command.planning_roll")
	_expect(roll != null and not roll.disabled, "the first roll is offered (%s)" % label)
	if roll == null: screen.queue_free(); return
	await _click(roll)
	await _wait_dice(screen, 8)
	_expect(screen._view.rolled_dice("blade").size() == 8 and screen._view.choose_count("blade") == 5, "an Ambush first roll rolls 8 dice and asks for 5 (%s)" % label)
	var strip = screen._actor_profiles.blade.statuses
	_expect(not strip.cells.has("ambush") or strip.cells.ambush.modulate.a == 0.0, "Ambush is used up by the roll (%s)" % label)
	var tray: BattleDiceTray = null
	for child in screen._player_dice_dock.get_children():
		if child is BattleDiceTray: tray = child
	_expect(tray != null and tray._buttons.size() >= 8, "the dice tray grew to eight dice (%s)" % label)
	if tray != null:
		var shown := 0
		for index in tray._buttons.size():
			if index < 8 and tray._buttons[index].is_visible_in_tree() and not tray._buttons[index].disabled: shown += 1
		_expect(shown == 8, "all eight dice are shown and selectable (%s): %d" % [label, shown])
		_expect(root.get_visible_rect().encloses(tray.get_global_rect()), "the eight-dice tray stays inside the viewport (%s): %s in %s" % [label, tray.get_global_rect(), root.get_visible_rect()])
	var keep := _inspection(screen, "battle.command.planning_keep")
	_expect(keep != null and keep.disabled and keep.text == "Keep 5/8", "Keep 5/8 waits for five chosen dice (%s)" % label)
	_expect(_inspection(screen, "battle.command.planning_reroll") == null, "no reroll while choosing (%s)" % label)
	await _capture(screen, "expedition-ambush-eight-dice-" + label)
	var faces: Array = []
	for index in 5:
		var die := _inspection(screen, "battle.die.blade.%d" % index)
		_expect(die != null, "die %d is clickable (%s)" % [index, label])
		if die == null: continue
		faces.append(int(screen._view.rolled_dice("blade")[index].get("face", 0)))
		await _click(die)
	keep = _inspection(screen, "battle.command.planning_keep")
	_expect(keep != null and not keep.disabled, "Keep 5 enables with exactly five dice chosen (%s)" % label)
	if keep == null: screen.queue_free(); return
	await _click(keep)
	await _wait_dice(screen, 5)
	var kept_faces: Array = []
	for die in screen._view.rolled_dice("blade"): kept_faces.append(int(die.get("face", 0)))
	faces.sort(); kept_faces.sort()
	_expect(screen._view.rolled_dice("blade").size() == 5 and kept_faces == faces and screen._view.choose_count("blade") == 0, "keeping collapses to the five chosen faces (%s): %s vs %s" % [label, kept_faces, faces])
	_expect(screen._view.rolls_used("blade") == 1, "choosing uses no roll (%s)" % label)
	await _capture(screen, "expedition-ambush-kept-" + label)
	# The normal flow resumes: keep one, reroll the rest, still five dice.
	var first := _inspection(screen, "battle.die.blade.0")
	if first != null: await _click(first)
	var reroll := _inspection(screen, "battle.command.planning_reroll")
	_expect(reroll != null and not reroll.disabled and reroll.text == "Roll 2/3", "rerolls return to the usual 3-roll budget (%s)" % label)
	if reroll != null:
		await _click(reroll)
		await _wait_dice(screen, 5)
		_expect(screen._view.rolled_dice("blade").size() == 5 and screen._view.rolls_used("blade") == 2, "a reroll brings back five dice (%s)" % label)
	# Relaunch: the pending battle restarts identically with Ambush again.
	screen.queue_free(); await process_frame
	var again := await _start(runtime, fixture)
	if again.is_empty(): return
	var screen2 = again[1]; var result2: Dictionary = again[2]
	_expect(str(result2.snapshot.battle_id) == fixture.battle_id, "relaunching restarts the same pending battle (%s)" % label)
	var stacks := 0
	for status in result2.snapshot.actors.blade.get("statuses", []):
		if str(status.get("definition_id", "")) == "ambush": stacks += int(status.get("stacks", 0))
	_expect(stacks == 3, "Ambush persists through the relaunch (%s)" % label)
	# Expiry unused: skip the offense without rolling, then play to round 2.
	var skip := _inspection(screen2, "battle.command.planning_pass")
	_expect(skip != null and skip.text == "Skip Offensive Ability", "the skip control is offered (%s)" % label)
	if skip != null:
		await _click(skip)
		for frame in 4: await process_frame
		var dialog: ConfirmationDialog = null
		for candidate in root.find_children("*", "ConfirmationDialog", true, false):
			if candidate.visible: dialog = candidate
		_expect(dialog != null, "skipping an unrolled offense asks first (%s)" % label)
		if dialog != null: await _click(dialog.get_ok_button())
		for frame in 6: await process_frame
	var gateway2: RefCounted = again[0]
	screen2.queue_free(); await process_frame
	var view: Dictionary = _drive_to_round(gateway2, 2)
	stacks = 0
	for status in view.get("snapshot", {}).get("actors", {}).get("blade", {}).get("statuses", []):
		if str(status.get("definition_id", "")) == "ambush": stacks += int(status.get("stacks", 0))
	_expect(int(view.get("snapshot", {}).get("round", 0)) >= 2 or view.get("battle_result", "") != "", "the battle reached round 2 (%s)" % label)
	_expect(stacks == 0, "an unused Ambush expires at the end of the first Offensive segment (%s)" % label)

## Drives the authority with the campaign proxy until planning in `round`.
func _drive_to_round(gateway: RefCounted, round_number: int) -> Dictionary:
	var view: Dictionary = gateway.observe()
	_expect(view.get("accepted") == true, "observe reads the current battle: %s" % str(view.get("error", "")))
	var guard := 0
	while guard < 600:
		guard += 1
		if view.get("battle_result", "") in ["victory", "defeat", "draw"]: break
		var snapshot: Dictionary = view.get("snapshot", {})
		if int(snapshot.get("round", 0)) >= round_number and str(snapshot.get("stage", "")) == "planning" and view.get("learned_policy", {}).get("model_turn") != true: break
		if view.get("learned_policy", {}).get("model_turn") == true:
			view = gateway.advance_model()
		else:
			var actions: Array = view.get("legal_actions", [])
			if actions.is_empty(): view = gateway.advance_model(); continue
			var chosen: Dictionary = actions[0]
			for kind in ["planning_roll", "planning_select_ability", "roll_dice", "planning_pass", "pass", "commit_interaction"]:
				var found := false
				for index in range(actions.size() - 1, -1, -1):
					if str(actions[index].get("type", "")) == kind: chosen = actions[index]; found = true; break
				if found: break
			view = gateway.submit(JSON.stringify(chosen))
		if view.get("accepted") != true:
			_expect(false, "driving the battle failed: %s" % str(view.get("error", "")))
			break
	return view
