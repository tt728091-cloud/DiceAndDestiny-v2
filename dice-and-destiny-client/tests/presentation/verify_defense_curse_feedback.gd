extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("defense-curse-feedback", 9)
	_expect(base.get("accepted", false), "native catalog loads")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		for defender in ["blade", "goblin"]:
			for rolled in [false, true]:
				root.size = viewport
				await _check(base, defender, viewport, rolled)
	await _native_finalization()
	print("DEFENSE CURSE FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check(base: Dictionary, defender: String, viewport: Vector2i, rolled: bool) -> void:
	var attacker := "goblin" if defender == "blade" else "blade"
	var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.pending_input = {}; f.legal_actions = []
	f.snapshot.segment = "defensive"; f.snapshot.stage = "defense_reaction"
	var source := {"id": "incoming-curse-test", "source_actor_id": attacker, "target_actor_id": defender, "source_content_id": "brine_lash", "base_amount": 3, "prevention": 0, "final_amount": 3}
	f.snapshot.damage_sources = [source]
	f.snapshot.defense_selections = {defender: {"actor_id": defender, "source_id": source.id, "ability_id": "hexward_rebuttal", "rolled_face": 1, "rolled_faces": [1], "finalized": false}}
	for i in 5: f.snapshot.actors[attacker].owned_dice[i].cursed_faces = [1] if rolled or i >= 3 else []
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-curse-feedback.json")); root.add_child(screen)
	for frame in 8: await process_frame
	var panel = screen._defense_result_panels[0]
	panel.started_ms = Time.get_ticks_msec() - 9000; panel._update()
	_expect(panel.data.effects_pending and panel.damage.text == "3", "prevention waits for finalized Curse; legal defense response remains available")
	var snapshot := f.duplicate(true)
	snapshot.snapshot.stage = "damage_reaction"; snapshot.snapshot.segment = "damage_resolution"
	snapshot.snapshot.defense_selections[defender].finalized = true
	snapshot.snapshot.damage_sources[0].prevention = 2
	snapshot.snapshot.damage_sources[0].final_amount = 1
	var face := 4 if rolled else 1
	snapshot.snapshot.actors[attacker].owned_dice[2].cursed_faces = [1, 4] if rolled else [1]
	var cause := {"die_id": snapshot.snapshot.actors[attacker].owned_dice[2].id, "face": face, "source_actor_id": defender, "source_ability_id": "hexward_rebuttal", "defense_source_id": source.id}
	var marked := cause.duplicate(true); marked.merge({"kind": "face_marked", "placement": "rolled" if rolled else "direct"})
	snapshot.events = []
	if rolled:
		var check := cause.duplicate(true); check.merge({"kind": "owned_roll", "roll_context": "curse_dice", "die": {"owned_id": cause.die_id, "index": 2, "face": face, "die_id": "brine_d6"}, "cursed": false})
		snapshot.events.append({"sequence": 100000, "type": "curse_resolved", "actor_id": attacker, "segment": "defensive", "data": check})
	# A later attack in the same authority response must remain queued until
	# Damage; it must neither delay defense Curse nor reveal its mark early.
	if defender == "blade" and rolled:
		snapshot.snapshot.actors[attacker].owned_dice[0].cursed_faces = [1, 6]
		snapshot.events.append({"sequence": 100002, "type": "curse_resolved", "actor_id": attacker, "segment": "damage_resolution", "data": {"kind": "face_marked", "die_id": snapshot.snapshot.actors[attacker].owned_dice[0].id, "face": 6, "placement": "direct", "source_actor_id": defender, "source_ability_id": "hexbrand", "attack_source_id": "later-attack"}})
	snapshot.events.append({"sequence": 100001, "type": "curse_resolved", "actor_id": attacker, "segment": "defensive", "data": marked})
	if defender == "blade": screen._apply_model_result(snapshot)
	else:
		# Exercise the human command path as well as a model priority handoff.
		screen.gateway._authority.enqueue(snapshot)
		screen._send(JSON.stringify({"type": "pass", "actor_id": "blade", "payload": {}}))
	var notices := screen.get_children().filter(func(child): return child.get_script() == NOTICE)
	_expect(notices.size() == 1, "finalized defense queues one Curse animation")
	if notices.is_empty(): screen.queue_free(); return
	var notice = notices[0]; notice.set_process(false)
	for frame in 8: await process_frame
	_expect(screen._view.stage == "defense_reaction" and screen._held_defense_view.stage == "damage_reaction", "defense outcome plays before Damage is exposed")
	panel = screen._defense_result_panels[0]
	if defender == "blade" and rolled:
		_expect(screen._held_defense_director.has_beats() and not screen._director.has_beats(), "later offensive Curse waits behind the complete defense outcome")
		_expect(6 not in screen._view.actor(attacker).owned_dice[0].cursed_faces, "later offensive Curse mark is not exposed on defense board")
	_expect(not panel.data.effects_pending, "prevention and Curse are released together")
	# Seek both clocks to the concurrent green-trail portion.
	screen._defense_outcome_start = Time.get_ticks_msec() - ceili((TIMING.roll_seconds() + 0.85) * 1000)
	panel.started_ms = screen._defense_outcome_start
	panel.data.roll_started_ms = Time.get_ticks_msec() - 9000
	panel._update(); notice._elapsed = 0.85; notice.refresh()
	var origin: Vector2 = notice.get_global_transform_with_canvas().affine_inverse() * panel.dice_controls[0].get_global_rect().get_center()
	_expect(notice._defense_start.distance_to(origin) < 1, "Curse green trail starts at the live landed defense die")
	_expect(not notice._ability_label.visible and not notice._label.visible, "no detached purple defense pop-ups")
	_expect(notice._batch_dice.size() == 1 and notice._batch_targets.size() == 1, "Curse trail targets the exact die and numbered face")
	_expect(notice._batch_rolls.is_empty() or not notice._batch_rolls[0].visible, "Curse roll waits for green trail arrival")
	var tray: BattleDiceTray = screen.dice_dock(attacker).get_child(0)
	_expect(not tray._mark_faces[2][face - 1].get_meta("cursed"), "new mark stays hidden during launch")
	_expect(tray._mark_faces[3][0].get_meta("cursed"), "existing marks remain visible")
	await _capture("defense-together-%s-%d-%s" % [defender, viewport.x, str(rolled)])
	notice._elapsed = 1.2; notice.refresh()
	if rolled: _expect(notice._batch_rolls.size() == 1 and notice._batch_rolls[0].visible, "expansion rolls on the receiving die after launch")
	notice._elapsed = 1.0 + (notice.duration - 1.0) * 0.57; notice.refresh()
	panel.started_ms = Time.get_ticks_msec() - ceili((TIMING.roll_seconds() + TIMING.effects_seconds()) * 1000); panel._update()
	_expect(panel.damage.text == "1", "prevention settles from 3 to 1 during Defense")
	_expect(tray._mark_faces[2][face - 1].get_meta("cursed"), "Curse face lights during the same Defense outcome")
	if rolled: _expect(notice._batch_rolls.size() == 1 and notice._batch_rolls[0].text.ends_with("4"), "check settles to authority face")
	await _capture("defense-complete-%s-%d-%s" % [defender, viewport.x, str(rolled)])
	screen._render(); await process_frame; notice.refresh()
	panel = screen._defense_result_panels[0]; panel._update(); notice.refresh()
	origin = notice.get_global_transform_with_canvas().affine_inverse() * panel.dice_controls[0].get_global_rect().get_center()
	_expect(notice._defense_start.distance_to(origin) < 1, "redraw reanchors to the defense die")
	notice._elapsed = notice.duration - 0.01; notice._process(0.02); await process_frame
	screen._defense_outcome_until = 0; screen._process(0); await process_frame
	_expect(screen._held_defense_view == null and screen._view.stage == "damage_reaction", "Damage appears only after the complete defense outcome")
	if defender == "blade" and rolled:
		var later := screen.get_children().filter(func(child): return child.get_script() == NOTICE)
		_expect(later.size() == 1 and not later[0].feedback.get("defense_inline", false), "only the later offensive Curse remains after defense")
	screen._director.queue_result(snapshot, 0, {})
	_expect(screen._director.take_curse_updates().is_empty(), "Damage cannot replay the defense Curse")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _native_finalization() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "curse")
	var result: Dictionary = gateway.start_battle("defense-curse-native-timing", 43)
	var checked := 0
	for step in 160:
		var before := result.duplicate(true)
		if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model()
		else:
			var chosen := {}
			var actions: Array = result.get("legal_actions", [])
			if result.snapshot.get("stage") == "defense_selection":
				for action in actions:
					if action.type == "planning_select_ability" and action.payload.get("ability_id") == "hexward_rebuttal": chosen = action; break
			if chosen.is_empty():
				for type in ["planning_roll", "planning_select_ability", "roll_dice", "pass", "planning_pass"]:
					for action in actions:
						if action.type == type: chosen = action; break
					if not chosen.is_empty(): break
			if chosen.is_empty(): break
			result = gateway.submit(JSON.stringify(chosen))
		_expect(result.get("accepted", false), "native defense action accepted")
		if not result.get("accepted", false): break
		var has_curse: bool = result.get("events", []).any(func(event): return event.get("type") == "curse_resolved" and event.get("data", {}).get("source_ability_id") == "hexward_rebuttal")
		if before.snapshot.get("stage") != "defense_reaction" or not has_curse: continue
		before.learned_policy = {}; before.legal_actions = []; before.pending_input = {}; before.events = []
		var screen = SCREEN.instantiate(); screen.initial_result = before; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("native-defense-curse-review.json")); root.add_child(screen)
		await process_frame
		screen._apply_model_result(result)
		_expect(screen._held_defense_view != null and screen._view.stage == "defense_reaction", "native finalization holds defense board across phase/wave handoff")
		var notices := screen.get_children().filter(func(child): return child.get_script() == NOTICE)
		_expect(not notices.is_empty(), "real native Curse event joins defense outcome")
		for notice in notices:
			_expect(notice.feedback.get("defense_inline", false), "native defense event is not inferred as an after-damage attack")
		screen.queue_free(); await process_frame
		checked += 1
		if checked >= 2: break
	_expect(checked >= 2, "two incoming attacks finalize and present their defenses independently")

func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_DEFENSE_CURSE_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DEFENSE CURSE FEEDBACK: " + message)
