extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _action(actions: Array, actors: Dictionary, id: String) -> Dictionary:
	for candidate in actions:
		if candidate.get("type") != "planning_commit_cards": continue
		var ids: Array = candidate.payload.get("card_ids", [])
		if not ids.is_empty() and actors.blade.card_instances[ids[0]].definition_id == id: return candidate
	return {}
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary
	var mark := {}
	for seed_value in range(1, 101):
		base = gateway.start_battle("refusal-feedback-%d" % seed_value, seed_value)
		mark = _action(base.get("legal_actions", []), base.snapshot.actors, "mark_the_number")
		if not mark.is_empty() and not _action(base.legal_actions, base.snapshot.actors, "maledictions_refusal").is_empty(): break
	_expect(not mark.is_empty() and not _action(base.legal_actions, base.snapshot.actors, "maledictions_refusal").is_empty(), "native hand contains both screenshot cards")
	if failed: quit(1); return
	base.events = []; base.learned_policy = {}
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("refusal-feedback.json")); root.add_child(screen)
	await process_frame
	screen._send(JSON.stringify(mark))
	for child in screen.get_children():
		if child.get_script() == NOTICE: child.queue_free()
	await process_frame
	var before: Dictionary = screen._view.actor("goblin").duplicate(true)
	var play := _action(screen._view.legal_actions, screen._view.actors, "maledictions_refusal")
	_expect(not play.is_empty(), "Refusal is playable after Mark the Number")
	screen._send(JSON.stringify(play))
	var notices := screen.get_children().filter(func(child): return child.get_script() == NOTICE and not child.is_queued_for_deletion())
	_expect(notices.size() == 1, "real Refusal play produces one coordinated animation")
	if notices.is_empty(): screen.queue_free(); quit(1); return
	var notice = notices[0]; notice.set_process(false)
	_expect(notice.feedback.card_id == "maledictions_refusal", "correct card reveal")
	_expect(notice.feedback.changes.size() == 1, "Curse mark animates; status uses normal status application feedback")
	_expect(screen._view.actor("goblin").statuses.any(func(status): return status.definition_id == "maledictions_refusal" and int(status.stacks) == 1), "native play applies real Refusal status")
	_expect(not screen._curse_preparation_labels.has("goblin:maledictions_refusal"), "no Prepared label")
	for child in screen.get_children():
		if child.get_meta("feedback_notice", false): child.queue_free()
	await process_frame
	screen._render(); await process_frame
	var profile: ActorProfile = screen._actor_profiles.goblin
	_expect("Malediction" in profile.statuses.text, "normal status list contains Refusal")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for frame in 8: await process_frame
		for face in [1, 6, 0]:
			var expired: bool = face == 0
			var change := {"kind": "refusal_expired" if expired else "refusal_trigger", "actor_id": "goblin", "die": {"index": 0, "face": face, "die_id": "brine_d6"}, "blocked": face == 1, "count_before": 4, "count_after": 5 if face == 1 else 0}
			var feedback = NOTICE.new()
			feedback.configure(screen, {"card_id": "", "title": "Malediction’s Refusal", "changes": [change]})
			screen.add_child(feedback); feedback.set_process(false)
			feedback._elapsed = feedback.duration * 0.6; feedback.refresh()
			await process_frame
			_expect(root.get_visible_rect().encloses(feedback._label.get_global_rect()), "caption fits viewport")
			_expect(not feedback._roll.visible if expired else str(face) in feedback._roll.text, "expiry has no roll; trigger reveals authority face")
			_expect(("expired" if expired else "CLEANSE BLOCKED" if face == 1 else "CLEANSE SUCCEEDS") in feedback._label.text, "explicit outcome")
			await _capture("refusal-status-%d-%d" % [face, viewport.x])
			feedback.queue_free(); await process_frame
	var applied := {"accepted": true, "snapshot": screen._view.raw_snapshot.duplicate(true)}
	for width in [1920, 1280]:
		for target in ["blade", "goblin", "goblin-4"]: await _expiry_lifecycle(applied, width, target)
	var director := BattlePresentationDirector.new()
	var ev := {"type": "curse_resolved", "sequence": 100001, "actor_id": "goblin", "round": 1, "segment": "offensive", "data": {"kind": "refusal_trigger", "die": {"index": 0, "face": 1, "die_id": "brine_d6"}, "blocked": true, "count_before": 4, "count_after": 5}}
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [ev]})
	_expect(director.has_beats() and director.take_curse_updates().size() == 1, "trigger gates progression with one feedback beat")
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [ev]})
	_expect(director.take_curse_updates().is_empty(), "duplicate event does not replay trigger")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("MALEDICTION REFUSAL FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expiry_lifecycle(applied: Dictionary, width: int, target: String) -> void:
	root.size = Vector2i(width, width * 9 / 16)
	var f := applied.duplicate(true); f.events = []; f.learned_policy = {}; f.pending_input = {}; f.legal_actions = []
	if target == "goblin-4":
		for i in range(2, 5):
			var id := "goblin-" + str(i)
			f.snapshot.actors[id] = f.snapshot.actors.goblin.duplicate(true); f.snapshot.actors[id]["id"] = id
	for actor in f.snapshot.actors.values(): actor.statuses = []
	f.snapshot.actors[target].statuses = [{"definition_id": "curse_count", "stacks": 4}, {"definition_id": "maledictions_refusal", "stacks": 1}]
	f.snapshot.round = 2; f.snapshot.segment = "damage_resolution"; f.snapshot.stage = "damage_reaction"
	var before: Dictionary = f.snapshot.actors.duplicate(true)
	var effects_before := before.duplicate(true)
	for actor in effects_before.values(): actor["health"] = actor.current_health; actor["energy"] = actor.energy_points
	var effects_after := effects_before.duplicate(true)
	effects_after[target].statuses[0].stacks = 1
	var batch := {"id": "refusal-lifecycle", "sources": [{"id": "damage", "source_actor_id": "blade" if target != "blade" else "goblin", "target_actor_id": target, "source_content_id": "hexbrand", "final_amount": 0}], "removals": [], "status_applications": []}
	f.snapshot.settled_damage = batch; f.snapshot.damage_sources = batch.sources
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("refusal-lifecycle.json")); root.add_child(screen); screen.set_process(false)
	await process_frame
	var final := f.duplicate(true)
	final.snapshot.round = 3; final.snapshot.segment = "offensive"; final.snapshot.stage = "planning"
	final.snapshot.actors[target].statuses = [{"definition_id": "curse_count", "stacks": 1}]
	final.events = [
		{"sequence": 200000, "type": "damage_committed", "round": 2, "segment": "damage_resolution", "data": batch},
		{"sequence": 200001, "type": "effects_resolved", "round": 3, "segment": "ongoing_effects", "data": {"actors_before": effects_before, "actors_after": effects_after, "steps": [{"type": "curse_conversion", "actor_id": target, "count_before": 4, "count_after": 1, "damage": 1}]}},
		{"sequence": 200002, "type": "segment_entered", "round": 3, "segment": "income"},
		{"sequence": 200003, "type": "curse_resolved", "round": 3, "segment": "income", "actor_id": target, "data": {"kind": "refusal_expired"}},
		{"sequence": 200004, "type": "energy_points_gained", "round": 3, "segment": "income", "actor_id": target, "energy_points": 1}]
	screen._apply_model_result(final); screen._income_animation_generation += 1
	var notice = screen.get_children().filter(func(child): return child.get_script() == NOTICE)[0]
	notice.set_process(false)
	_expect(screen._director._queue.map(func(beat): return beat.type) == ["combat_damage", "effects_resolved", "curse_retry", "income_summary"], "expiry stays after Effects and before Income reward animation")
	for frame in 8: await process_frame
	notice.refresh()
	var slot: Control = screen._actor_profiles[target].statuses.ensure_slot("maledictions_refusal")
	_expect(notice._waiting() and slot.modulate.a == 1 and int(screen._actor_profiles[target].statuses.counts.get("maledictions_refusal", 0)) == 1, "queued expiry does not hide active Refusal during Damage")
	screen._show_damage_counts(1.0); notice.refresh()
	_expect(slot.modulate.a == 1, "damage counter updates preserve active Refusal")
	await _capture("refusal-damage-visible-%s-%d" % [target, width])
	screen._advance_beat(); screen._income_animation_generation += 1
	await process_frame; notice.refresh()
	var profile: ActorProfile = screen._actor_profiles[target]
	profile.show_effects_progress(effects_before[target], effects_after[target], "statuses", 1.0); notice.refresh()
	_expect(profile.statuses.cells.maledictions_refusal.modulate.a == 1 and notice._waiting(), "Refusal remains present through Curse Count conversion")
	screen._advance_beat(); screen._income_animation_generation += 1
	for frame in 8: await process_frame
	notice._elapsed = notice.duration * 0.25; notice.refresh(); await process_frame; notice.refresh()
	profile = screen._actor_profiles[target]; slot = profile.statuses.cells.maledictions_refusal
	_expect(not notice._waiting() and slot.modulate.a > 0 and slot.modulate.a < 1, "real HUD icon fades as expiry notice appears")
	var bounds: Rect2 = notice.get_global_transform_with_canvas().affine_inverse() * slot.get_global_rect()
	_expect(notice._defense_start.distance_to(bounds.get_center()) < 1, "green trail originates at exact status icon")
	_expect(notice._target.get_center().distance_to(bounds.get_center()) < 360, "expiry box stays local to the actor HUD")
	for other in screen._actor_profiles.values():
		_expect(not notice._label.get_global_rect().intersects(other.get_global_rect()), "expiry caption does not cover any actor HUD")
	_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "local expiry caption fits either screen edge")
	_expect(not notice._roll.visible and not notice._ability_label.visible, "expiry uses real icon without an invented roll or duplicate badge")
	await _capture("refusal-local-expiry-%s-%d" % [target, width])
	screen._render(); screen._income_animation_generation += 1
	await process_frame; notice.refresh()
	_expect(screen._actor_profiles[target].statuses.cells.maledictions_refusal.modulate.a < 1, "redraw preserves fade progress")
	notice._elapsed = notice.duration * 0.92; notice.refresh()
	_expect(notice.modulate.a < 1 and screen._actor_profiles[target].statuses.cells.maledictions_refusal.modulate.a == 0, "status disappears and caption fades out")
	notice._process(notice.duration); await process_frame; await process_frame
	_expect(int(screen._actor_profiles[target].statuses.counts.get("maledictions_refusal", 0)) == 0, "expired status stays gone in Income")
	screen.queue_free(); await process_frame

func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_CURSE_FEEDBACK_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await process_frame; RenderingServer.force_draw(); root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("MALEDICTION REFUSAL FEEDBACK: " + message)
