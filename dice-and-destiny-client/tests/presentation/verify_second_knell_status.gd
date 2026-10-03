extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
const GAIN := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary
	var action := {}
	for seed_value in range(1, 101):
		base = gateway.start_battle("knell-status-%d" % seed_value, seed_value)
		for candidate in base.get("legal_actions", []):
			var ids: Array = candidate.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "second_knell": action = candidate; break
		if not action.is_empty(): break
	_expect(not action.is_empty(), "native Second Knell card is playable")
	if action.is_empty(): quit(1); return
	base.events = []; base.learned_policy = {}
	var screen = _screen(base, gateway); await process_frame
	screen._send(JSON.stringify(action)); await process_frame
	_expect(not screen._curse_preparation_labels.has("goblin:second_knell"), "no floating Prepared label")
	var gains: Array = screen.get_children().filter(func(node): return node.get_script() == GAIN and not node.is_queued_for_deletion())
	_expect(gains.size() == 1, "status application gets normal card-to-status animation")
	if not gains.is_empty():
		gains[0].set_process(false); gains[0]._started = true; gains[0]._elapsed = gains[0].duration * 0.8; gains[0].refresh()
		_expect("Second Knell ×1" in screen._actor_profiles.goblin.statuses.text, "normal enemy status list includes Second Knell")
		_expect("+1 Second Knell" in gains[0]._label.text, "application names status")
		await _capture("application")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	for width in [1280, 1920]:
		for retry_hit in [false, true]:
			await _retry(base, width, retry_hit)
	var expiry := base.duplicate(true); expiry.events = [{"sequence": 1000, "type": "curse_resolved", "round": 2, "segment": "ongoing_effects", "actor_id": "goblin", "data": {"kind": "second_knell_expired"}}]
	screen = _screen(expiry, BattleGateway.new(FakeBattleAuthority.new())); await process_frame
	var notice = _notice(screen); _expect(notice != null, "expiry gets feedback")
	if notice != null:
		notice.set_process(false); notice._elapsed = 0.6; notice.refresh()
		_expect(not notice._roll.visible and "expired" in notice._label.text, "expiry never invents a roll")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("SECOND KNELL STATUS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _retry(base: Dictionary, width: int, retry_hit: bool) -> void:
	root.size = Vector2i(width, int(width * 9 / 16.0))
	var fixture := base.duplicate(true); fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.actors.goblin.statuses = [{"definition_id": "curse_count", "stacks": 3 if retry_hit else 2}]
	var die := {"index": 0, "owned_id": "goblin/die/1", "die_id": "brine_d6", "face": 1}
	var retry := die.duplicate(true); retry.face = 1 if retry_hit else 6
	fixture.events = [{"sequence": 1000, "type": "curse_resolved", "round": 1, "segment": "offensive", "actor_id": "goblin", "data": {"kind": "second_knell_trigger", "die": die, "retry_die": retry, "retry_cursed": retry_hit, "count_before": 1, "count_after": 3 if retry_hit else 2, "roll_context": "curse_dice"}}]
	if retry_hit:
		fixture.events[0].data["source_card_id"] = "unquiet_hands"
		fixture.events[0].data["source_actor_id"] = "blade"
	fixture.events.push_front({"sequence": 999, "type": "curse_resolved", "round": 1, "segment": "offensive", "actor_id": "goblin", "data": {"kind": "owned_roll", "die": retry, "cursed": retry_hit, "roll_context": "curse_dice", "second_knell_part": true}})
	fixture.events.push_front({"sequence": 998, "type": "curse_resolved", "round": 1, "segment": "offensive", "actor_id": "goblin", "data": {"kind": "owned_roll", "die": die, "cursed": true, "roll_context": "curse_dice", "second_knell_part": true}})
	# A response may include automatic Effects and Income after the retry.
	var bundled := fixture.duplicate(true)
	bundled.events.append({"sequence": 1001, "type": "effects_resolved", "round": 2, "segment": "ongoing_effects", "data": {}})
	bundled.events.append({"sequence": 1002, "type": "segment_entered", "round": 2, "segment": "income"})
	bundled.events.append({"sequence": 1003, "type": "energy_points_gained", "round": 2, "segment": "income", "actor_id": "goblin", "energy_points": 1})
	var director := BattlePresentationDirector.new(); director.queue_result(bundled)
	_expect(director.peek().get("type") == "curse_retry", "retry plays before next Effects and Income")
	_expect(director.take_curse_updates().size() == 1, "paired owned rolls do not create duplicate feedback")
	var screen = _screen(fixture, BattleGateway.new(FakeBattleAuthority.new())); await process_frame
	screen.set_process(false)
	var notice = _notice(screen); _expect(notice != null, "trigger creates feedback")
	if notice == null: screen.queue_free(); await process_frame; return
	notice.set_process(false)
	_expect(is_instance_valid(notice._card) == retry_hit, "initiating card remains visible when it caused the check")
	_expect(screen._director.peek().type == "curse_retry" and not notice._waiting(), "retry owns an uninterrupted presentation beat")
	var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
	var saved: Array = tray._dice.duplicate(true)
	var seen := {}
	for progress in [0.06, 0.10, 0.15]:
		notice._elapsed = notice.duration * progress; notice.refresh(); seen[notice._roll.text] = true
	_expect(seen.size() > 1, "initial die rolls visibly")
	_expect("Curse Count ×1" in screen._actor_profiles.goblin.statuses.text, "Count stays at baseline before hit arrives")
	notice._elapsed = notice.duration * 0.32; notice.refresh(); await process_frame
	_expect("Curse Count ×2" in screen._actor_profiles.goblin.statuses.text, "first hit arrives independently")
	_expect(notice._ability_label.modulate.a == 1 and "Curse hit" in notice._label.text, "status waits for actual hit")
	await _capture("first-hit-%s-%d" % [retry_hit, width])
	notice._elapsed = notice.duration * 0.42; notice.refresh(); await process_frame
	_expect(notice._ability_label.modulate.a < 1 and "consumed" in notice._label.text, "status fades with named consumption")
	_expect(notice._knell_origin.size.x > 0 and notice._die_rect.size.x > 0, "trail has status source and physical die target")
	await _capture("consume-%s-%d" % [retry_hit, width])
	seen.clear()
	for progress in [0.54, 0.60, 0.66]:
		notice._elapsed = notice.duration * progress; notice.refresh(); seen[notice._roll.text] = true
	_expect(seen.size() > 1, "same die retry visibly cycles faces")
	_expect(tray._dice == saved, "separate check preserves offensive dice")
	notice._elapsed = notice.duration * 0.8; notice.refresh()
	_expect("Curse Count ×2" in screen._actor_profiles.goblin.statuses.text, "second Count waits for its own arrival")
	notice._elapsed = notice.duration * 0.9; notice.refresh(); await process_frame
	_expect(("+1 more Count" if retry_hit else "no extra Count") in notice._label.text, "final outcome explains hit or miss")
	_expect("Curse Count ×%d" % (3 if retry_hit else 2) in screen._actor_profiles.goblin.statuses.text, "correct final Count")
	_expect(notice._ability_label.modulate.a == 0 and "offense unchanged" in notice._label.text, "status consumed, extra check clearly separate")
	_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "caption fits viewport")
	await _capture("final-%s-%d" % [retry_hit, width])
	var elapsed: float = notice._elapsed
	screen._snapshot_panel_open = true; notice._process(0.5)
	_expect(notice._elapsed == elapsed, "inspection pauses retry")
	screen._snapshot_panel_open = false; screen._render(); await process_frame
	_expect(is_instance_valid(notice) and notice._elapsed == elapsed, "redraw retains retry progress")
	screen._director.queue_result(fixture, screen._director.last_sequence())
	_expect(screen._director.take_curse_updates().is_empty(), "duplicate events do not repeat")
	notice._process(notice.duration); await process_frame; await process_frame
	_expect(not is_instance_valid(notice) and not screen._director.has_beats(), "retry completes and advances automatically")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _screen(result: Dictionary, gateway):
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("second-knell-status.json")); root.add_child(screen); return screen
func _notice(screen):
	for child in screen.get_children():
		if child.get_script() == NOTICE and not child.is_queued_for_deletion(): return child
	return null
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_KNELL_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		for frame in 6: await process_frame
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("SECOND KNELL STATUS: " + message)
