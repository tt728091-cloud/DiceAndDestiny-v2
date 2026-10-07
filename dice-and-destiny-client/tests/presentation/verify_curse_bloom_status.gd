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
		base = gateway.start_battle("bloom-status-%d" % seed_value, seed_value)
		for candidate in base.get("legal_actions", []):
			var ids: Array = candidate.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "curse_bloom": play = candidate; break
		if not play.is_empty(): break
	_expect(not play.is_empty(), "native card found")
	if failed: quit(1); return
	base.events = []; base.learned_policy = {}
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("bloom-feedback.json")); root.add_child(screen)
	await process_frame; screen.set_process(false)
	screen._send(JSON.stringify(play))
	_expect(screen._error_message.is_empty(), "authority accepts play")
	_expect(screen._view.actor("goblin").statuses.any(func(status): return status.definition_id == "curse_bloom" and int(status.stacks) == 1), "enemy gains real status")
	_expect(not screen._curse_preparation_labels.has("goblin:curse_bloom"), "no Prepared label")
	for child in screen.get_children():
		if child.get_meta("feedback_notice", false): child.queue_free()
	await process_frame; screen._render(); await process_frame
	_expect("Curse Bloom" in screen._actor_profiles.goblin.statuses.text, "status visible in enemy list")
	for viewport in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		root.size = viewport
		for frame in 8: await process_frame
		await _capture("status-%d" % viewport.x)
		for outcome in ["expand", "blocked", "no_dice", "expired"]:
			var change := {"kind": "bloom_expired" if outcome == "expired" else "bloom_trigger", "actor_id": "goblin", "damage": 0 if outcome == "blocked" else 1, "attempts": 3 if outcome == "expand" else 0}
			var notice = NOTICE.new(); notice.configure(screen, {"card_id": "curse_bloom", "title": "Curse Bloom", "changes": [change]}); screen.add_child(notice); notice.set_process(false)
			notice._elapsed = notice.duration * 0.6; notice.refresh(); await process_frame
			_expect(not notice._roll.visible and notice._batch_rolls.is_empty(), "status consumption invents no roll")
			_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "caption fits viewport")
			var text := "expired" if outcome == "expired" else "No expansion" if outcome == "blocked" else "No cursed dice" if outcome == "no_dice" else "3 expansion attempts"
			_expect(text in notice._label.text, "clear lifecycle outcome")
			await _capture("%s-%d" % [outcome, viewport.x])
			notice.queue_free(); await process_frame
	var director := BattlePresentationDirector.new()
	var changes := [{"type": "curse_resolved", "sequence": 100001, "actor_id": "goblin", "round": 2, "segment": "ongoing_effects", "data": {"kind": "bloom_trigger", "damage": 1, "attempts": 3}}]
	for index in 3:
		changes.append({"type": "curse_resolved", "sequence": 100002 + index * 2, "actor_id": "goblin", "round": 2, "segment": "ongoing_effects", "data": {"kind": "owned_roll", "roll_context": "curse_dice", "cursed": false, "source_card_id": "curse_bloom", "die": {"owned_id": "enemy/die/%d" % index, "index": index, "face": 6, "die_id": "brine_d6"}}})
		changes.append({"type": "curse_resolved", "sequence": 100003 + index * 2, "actor_id": "goblin", "round": 2, "segment": "ongoing_effects", "data": {"kind": "face_marked", "placement": "rolled", "source_card_id": "curse_bloom", "die_id": "enemy/die/%d" % index, "face": 6}})
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": changes})
	var feedback: Array = director.take_curse_updates()
	_expect(feedback.size() == 1 and feedback[0].changes.size() == 4 and feedback[0].changes[0].kind == "bloom_trigger", "consumption precedes three expansion rolls")
	if not feedback.is_empty():
		var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
		var owned: Array = screen._view.actor("goblin").owned_dice.duplicate(true)
		for index in 3: owned[index].cursed_faces = [1, 6]
		tray.display_owned(owned); await process_frame
		var notice = NOTICE.new(); notice.configure(screen, feedback[0]); screen.add_child(notice); notice.set_process(false)
		notice._step = 1; notice._elapsed = notice.duration * 0.6; notice.refresh()
		# Revealing an enemy tray schedules its Container layout for the next frame.
		for frame in 2: await process_frame
		notice.refresh()
		_expect(notice._face_batch_end() == 4 and notice._batch_rolls.size() == 3 and notice._batch_targets.size() == 3, "all three expansions animate together")
		notice.queue_free(); await process_frame
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": changes})
	_expect(director.take_curse_updates().is_empty(), "duplicate trigger suppressed")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("CURSE BLOOM STATUS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_BLOOM_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(false); root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSE BLOOM: " + message)
