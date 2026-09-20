extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var base: Dictionary = gateway.start_battle("card-gains", 1789679833957118)
	_expect(base.get("accepted") == true, "native battle starts")
	var action := {}
	for candidate in base.legal_actions:
		if candidate.get("type") != "planning_commit_cards": continue
		var ids: Array = candidate.payload.get("card_ids", [])
		if not ids.is_empty() and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "culture_flask": action = candidate; break
	_expect(not action.is_empty(), "Culture Flask is playable with recorded seed")
	if action.is_empty(): quit(1); return
	var before := base.duplicate(true)
	base.events = []; base.learned_policy = {}
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("card-gains.json"))
	root.add_child(screen); await process_frame
	screen._send(JSON.stringify(action))
	var notices := _notices(screen)
	_expect(notices.size() == 1, "real Culture Flask play produces one gain animation")
	if not notices.is_empty():
		var notice = notices[0]
		_expect(notice.updates[0].data.before == 0 and notice.updates[0].data.after == 2, "actual Catalyst gain is 2")
		_expect(notice._cards[0].definition_id == "culture_flask" and "+2 Catalyst" in notice._label.text, "card and benefit named beside recipient")
		_expect(not screen._director.has_beats(), "no reaction interstitial added")
		await create_timer(0.35).timeout
		_expect(notice._label.modulate.a > 0.9, "gain label visibly fades in")
		var path := OS.get_environment("DICE_AND_DESTINY_CARD_GAIN_SCREENSHOT")
		if not path.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(path)
		screen._render()
		_expect(_notices(screen).size() == 1 and _notices(screen)[0] == notice, "redraw preserves animation without replay")
		await create_timer(1.3).timeout
		_expect("Catalyst ×2" in screen._actor_profiles.blade.statuses.text, "Catalyst count settles correctly")
		_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "notice fits on screen")
		await create_timer(0.9).timeout
		_expect(_notices(screen).is_empty(), "animation cleans itself up")
	var result := {"snapshot": screen._view.raw_snapshot, "events": screen._view.events}
	# The same event cannot replay a gain, even if the caller supplies an old baseline.
	screen._director.queue_result(result, screen._director.last_sequence(), before.snapshot.actors)
	_expect(screen._director.take_status_updates().is_empty(), "replayed event does not repeat gain")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	# Both sides and capped gains use the same generic path, including AI results.
	for actor_id in ["blade", "goblin"]:
		for start in [0, 2, 3]:
			await _check_capped(before, actor_id, start)
	_check_resources(before)
	await _check_enemy_burst(before)
	print("CARD GAINS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check_capped(base: Dictionary, actor_id: String, start: int) -> void:
	var initial := base.duplicate(true); initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	initial.snapshot.actors[actor_id].statuses = [{"definition_id": "catalyst", "stacks": start}]
	var result := initial.duplicate(true)
	result.snapshot.actors[actor_id].statuses = [{"definition_id": "catalyst", "stacks": mini(3, start + 2)}]
	result.snapshot.actors[actor_id].energy_points -= 1; result.snapshot.actors[actor_id].hand_count -= 1
	result.events = [{"sequence": 100, "type": "card_played", "actor_id": actor_id, "segment": "offensive", "energy_cost": 1, "data": {"card_definition_id": "culture_flask", "card_instance_id": "flask"}}]
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("card-gains-cap.json"))
	root.add_child(screen)
	screen._apply_model_result(result)
	var notices := _notices(screen)
	_expect(notices.size() == (0 if start == 3 else 1), "no fake gains at cap on " + actor_id)
	if not notices.is_empty():
		var notice = notices[0]
		_expect(notice.updates[0].data.after - notice.updates[0].data.before == mini(2, 3 - start), "capped gain amount on " + actor_id)
		_expect(notice.updates[0].data.target_actor_id == actor_id, "gain targets correct profile")
		notice._started = true; notice._elapsed = 0.4; notice.refresh(); await process_frame
		_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "gain on either side fits viewport")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _check_resources(base: Dictionary) -> void:
	var initial: Dictionary = base.snapshot.actors.duplicate(true)
	var result := base.duplicate(true)
	result.snapshot.actors.blade.hand_count += 1 # Play one, draw two.
	result.snapshot.actors.blade.energy_points += 1
	result.events = [{"sequence": 100, "type": "card_played", "actor_id": "blade", "energy_cost": 0, "data": {"card_definition_id": "battle_focus", "card_instance_id": "focus"}}]
	var director := BattlePresentationDirector.new()
	director.queue_result(result, 0, initial)
	var updates := director.take_status_updates()
	_expect(updates.size() == 2, "energy and card draw gains have feedback")
	for update in updates:
		_expect(update.data.amount == (2 if update.data.stat == "hand" else 1), "draw amount accounts for card spent")
	# A queued status application must not receive a duplicate snapshot notice.
	result.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 1}]
	result.events = [{"sequence": 101, "type": "card_played", "actor_id": "blade", "data": {"card_definition_id": "pinprick", "card_instance_id": "pin"}}, {"sequence": 102, "type": "proposal_batch_committed", "data": {"status_application": {"target_actor_id": "goblin", "status_id": "poison", "before": 0, "after": 1}}}]
	director.queue_result(result, 0, initial)
	var poison := 0
	for update in director.take_status_updates():
		if update.data.get("status_id") == "poison": poison += 1
	_expect(poison == 1, "existing Poison animation not duplicated")

func _notices(screen: Node) -> Array:
	var result := []
	for child in screen.get_children():
		if child.get_script() == NOTICE and not child.is_queued_for_deletion(): result.append(child)
	return result

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("CARD GAINS: " + message)

func _check_enemy_burst(base: Dictionary) -> void:
	var initial := base.duplicate(true); initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("card-gains-burst.json"))
	root.add_child(screen); await process_frame
	var result := initial.duplicate(true)
	for index in 3:
		result.snapshot.actors.goblin.energy_points += 1
		result.events.append({"sequence": 100 + index, "type": "card_played", "actor_id": "goblin", "energy_cost": 0, "data": {"card_definition_id": "battle_focus", "card_instance_id": "focus-%d" % index}})
		screen._apply_model_result(result.duplicate(true))
		await process_frame
	var notices := _notices(screen)
	_expect(notices.size() == 3, "three distinct card plays retained")
	for notice in notices: _expect(notice.updates.size() == 2, "each card combines draw and energy in one animation")
	await create_timer(0.95).timeout
	var visible := 0
	for notice in notices:
		if notice.modulate.a > 0.1: visible += 1
	_expect(visible == 1, "quick plays show only one card at a time")
	var notice = notices[0]
	_expect(notice._label.text == "+1 Energy · +1 card", "short combined benefit, correct singular card label")
	var card_bounds: Rect2 = notice._cards[0].get_global_rect()
	var profile_bounds: Rect2 = screen._actor_profiles.goblin.get_global_rect()
	_expect(card_bounds.end.x < profile_bounds.position.x and root.get_visible_rect().encloses(card_bounds), "card is beside enemy profile, not over its stats or dice")
	var dice_bounds: Rect2 = screen._enemy_dice_dock.get_global_rect()
	_expect(not notice._label.get_global_rect().intersects(dice_bounds), "benefit label does not overlap enemy dice")
	_expect(notice._destinations.size() == 2, "trails connect to both affected resource values")
	var path := OS.get_environment("DICE_AND_DESTINY_CARD_GAIN_SCREENSHOT")
	if not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(path.replace(".png", "-enemy.png"))
	screen._render(); await process_frame
	_expect(_notices(screen).size() == 3, "redraw does not duplicate queued plays")
	screen.learned_battle_mode = true
	screen._schedule_model_if_needed({"learned_policy": {"model_turn": true}})
	_expect(not screen._model_thinking, "opponent waits for its visible card feedback")
	screen.learned_battle_mode = false
	for active in notices:
		active._started = true; active._elapsed = active.duration + 0.1; active.refresh(); await process_frame
	_expect(_notices(screen).is_empty(), "queued feedback completes and leaves no labels")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	# The presentation watermark can lag behind received events. Those events
	# must still contribute feedback once only while the older beat is queued.
	var director := BattlePresentationDirector.new()
	result.events.push_front({"sequence": 99, "type": "status_changed", "actor_id": "goblin", "data": {}})
	director.queue_result(result, 0, initial.snapshot.actors)
	_expect(not director.take_status_updates().is_empty() and director.has_beats(), "gain feedback received while a beat is pending")
	director.queue_result(result, 0, initial.snapshot.actors)
	_expect(director.take_status_updates().is_empty(), "unpresented repeated events never duplicate card feedback")
