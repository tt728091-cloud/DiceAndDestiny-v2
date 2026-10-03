extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary
	var play := {}
	for seed_value in range(1, 101):
		base = gateway.start_battle("dividend-status-%d" % seed_value, seed_value)
		for candidate in base.get("legal_actions", []):
			var ids: Array = candidate.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "black_dividend": play = candidate; break
		if not play.is_empty(): break
	_expect(not play.is_empty(), "native card found")
	if failed: quit(1); return
	base.events = []; base.learned_policy = {}
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("dividend-feedback.json")); root.add_child(screen)
	await process_frame; screen.set_process(false)
	screen._send(JSON.stringify(play))
	_expect(screen._error_message.is_empty(), "authority accepts play")
	_expect(screen._view.actor("goblin").statuses.any(func(status): return status.definition_id == "black_dividend" and int(status.stacks) == 1), "enemy gains real status")
	_expect(not screen._curse_preparation_labels.has("goblin:black_dividend"), "no Prepared label")
	for child in screen.get_children():
		if child.get_meta("feedback_notice", false): child.queue_free()
	await process_frame; screen._render(); await process_frame
	_expect("Black Dividend" in screen._actor_profiles.goblin.statuses.text, "status visible in enemy list")
	for viewport in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		root.size = viewport
		for frame in 8: await process_frame
		await _capture("status-%d" % viewport.x)
		for outcome in ["first", "second", "expired"]:
			var change := {"kind": "dividend_expired" if outcome == "expired" else "dividend_trigger", "actor_id": "goblin", "source_actor_id": "blade", "die": {"index": 2, "face": 6}, "energy_before": 1, "energy_after": 2, "rewards": 2 if outcome == "second" else 1, "consumed": outcome == "second"}
			var notice = NOTICE.new(); notice.configure(screen, {"card_id": "", "title": "Black Dividend", "changes": [change]}); screen.add_child(notice); notice.set_process(false)
			notice._elapsed = notice.duration * 0.75; notice.refresh(); await process_frame
			_expect(not notice._roll.visible and notice._batch_rolls.is_empty(), "status proc invents no roll")
			_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "caption fits viewport")
			_expect(("expired" if outcome == "expired" else "consumed" if outcome == "second" else "waits for next round") in notice._label.text, "clear lifecycle outcome")
			if outcome != "expired": _expect(notice._count_target.size != Vector2.ZERO and notice._die_rect.size != Vector2.ZERO, "die and Energy endpoints are present")
			await _capture("%s-%d" % [outcome, viewport.x])
			notice.queue_free(); await process_frame
	var director := BattlePresentationDirector.new()
	var roll := {"type": "curse_resolved", "sequence": 100000, "actor_id": "goblin", "round": 1, "segment": "offensive", "data": {"kind": "owned_roll", "roll_context": "curse_dice", "cursed": true, "die": {"owned_id": "enemy/die/3", "index": 2, "face": 6, "die_id": "brine_d6"}}}
	var reward := {"type": "curse_resolved", "sequence": 100001, "actor_id": "goblin", "round": 1, "segment": "offensive", "data": {"kind": "dividend_trigger", "source_actor_id": "blade", "die": {"index": 2, "face": 6}, "energy_before": 1, "energy_after": 2, "rewards": 1, "consumed": false}}
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [roll, reward]})
	var feedback: Array = director.take_curse_updates()
	_expect(feedback.size() == 1 and feedback[0].changes.size() == 2 and feedback[0].changes[1].kind == "dividend_trigger", "reward follows its triggering roll")
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [reward]})
	_expect(director.take_curse_updates().is_empty(), "duplicate reward event suppressed")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("BLACK DIVIDEND STATUS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_DIVIDEND_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(false); root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("BLACK DIVIDEND: " + message)
