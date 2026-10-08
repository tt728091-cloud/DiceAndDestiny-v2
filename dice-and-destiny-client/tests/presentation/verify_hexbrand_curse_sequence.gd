extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("hexbrand-sequence", 9)
	_expect(base.get("accepted", false), "native catalog loads")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		await _check(base, viewport)
	await _multiple_followups(base)
	print("HEXBRAND CURSE SEQUENCE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check(base: Dictionary, viewport: Vector2i) -> void:
	var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
	f.snapshot.stage = "damage_reaction"; f.snapshot.segment = "damage_resolution"
	f.snapshot.actors.blade.selected_ability = "hexbrand"; f.snapshot.actors.blade.selected_tier = "skull_5"
	for i in 5: f.snapshot.actors.goblin.owned_dice[i].cursed_faces = [1] if i >= 2 else []
	var source := {"id": "hexbrand-test", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "hexbrand", "base_amount": 5, "prevention": 2, "final_amount": 3}
	var batch := {"id": "hexbrand-batch", "sources": [source], "removals": [], "status_applications": [], "actors_before": f.snapshot.actors.duplicate(true)}
	for i in 3: batch.removals.append({"card_id": "lost-" + str(i), "card_definition_id": "brine_surge", "accepted": true, "target_actor_id": "goblin", "original_zone": "deck", "damage_proposal_ids": [source.id]})
	f.snapshot.damage_sources = [source]; f.snapshot.settled_damage = batch
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen._damage_cards_open["actor:goblin"] = true # Lists start folded; watch the tear.
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hexbrand-sequence.json")); root.add_child(screen)
	for frame in 8: await process_frame
	var cue: Label = screen._curse_attack_origins[source.id]
	_expect(cue.is_visible_in_tree() and cue.text == "Hexbrand · 3 Curse", "pending stack announces exact Curse quantity")
	_expect(cue.get_global_rect().position.y >= screen._damage_grids[0].get_global_rect().end.y, "follow-up is directly beneath the damage stack")
	_expect("mark face 1" in BattlePresentationCatalog.ability("hexbrand").text and "already cursed" in BattlePresentationCatalog.ability("hexbrand").text, "ability rules explain clean seeds, expansion, and failed new marks")
	await create_timer(0.9).timeout
	await _capture("hexbrand-preview-%d" % viewport.x)
	var result := f.duplicate(true)
	result.snapshot.actors.goblin.current_health = 13; result.snapshot.actors.goblin.removed_count = 3; result.snapshot.actors.goblin.deck_count = 11
	result.snapshot.actors.goblin.owned_dice[0].cursed_faces = [1]
	result.snapshot.actors.goblin.owned_dice[1].cursed_faces = [1, 5]
	result.events = [{"sequence": 100000, "type": "damage_committed", "segment": "damage_resolution", "data": batch}]
	for step in 3:
		var index := 0 if step == 0 else 1
		var face := 5 if step == 2 else 1
		var data := {"kind": "face_marked", "die_id": result.snapshot.actors.goblin.owned_dice[index].id, "face": face, "placement": "rolled" if step == 2 else "direct", "source_actor_id": "blade", "source_ability_id": "hexbrand", "attack_source_id": source.id, "ordinary_curse": true, "application_index": step + 1, "application_count": 3}
		if step == 2:
			var roll := data.duplicate(true); roll.kind = "owned_roll"; roll.roll_context = "curse_dice"; roll.cursed = false
			roll.die = {"owned_id": data.die_id, "index": index, "face": face, "die_id": "brine_d6"}
			result.events.append({"sequence": 100003, "type": "curse_resolved", "actor_id": "goblin", "segment": "damage_resolution", "data": roll})
		result.events.append({"sequence": 100001 + step + (1 if step == 2 else 0), "type": "curse_resolved", "actor_id": "goblin", "segment": "damage_resolution", "data": data})
	var director := BattlePresentationDirector.new()
	var all_events: Array = result.events.duplicate(true)
	all_events.append({"sequence": 100005, "type": "effects_resolved", "round": 2, "segment": "ongoing_effects", "data": {}})
	director.queue_result({"snapshot": result.snapshot, "events": all_events}, 0, f.snapshot.actors)
	_expect(director._queue.map(func(beat): return beat.type) == ["combat_damage", "effects_resolved"], "Curse belongs to damage beat before next Effects")
	_expect(director._queue[0].watermark == 100004 and director._queue[0].curse_followups.size() == 1, "one presentation cursor includes damage and all three Curse outcomes")
	screen._apply_model_result(result)
	var notices := screen.get_children().filter(func(child): return child.get_script() == NOTICE)
	_expect(notices.size() == 1, "one coordinated Hexbrand sequence")
	if notices.is_empty(): screen.queue_free(); return
	var notice = notices[0]; notice.set_process(false)
	_expect(notice.feedback.changes.size() == 3 and notice._waiting(), "three applications await damage completion")
	var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
	_expect(not tray._mark_faces[0][0].get_meta("cursed") and not tray._mark_faces[1][0].get_meta("cursed") and not tray._mark_faces[1][4].get_meta("cursed"), "all incoming marks stay hidden during damage")
	var board_id: int = screen._root.get_instance_id()
	var stack_id: int = screen._damage_grids[0].get_instance_id()
	var deadline := Time.get_ticks_msec() + 7000
	var saw_tear := false
	while notice._waiting() and Time.get_ticks_msec() < deadline:
		var grid: Control = screen._damage_grids[0]
		if is_instance_valid(grid._tear) and grid._tear.progress > 0 and grid._tear.progress < 1:
			saw_tear = true
			_expect(notice._waiting(), "Curse waits while damage cards tear")
		await process_frame
	_expect(saw_tear, "cards visibly tear before Curse launches")
	_expect(screen._root.get_instance_id() == board_id and screen._damage_grids[0].get_instance_id() == stack_id, "same board and stack persist from tear to Curse")
	_expect(screen._director.peek().type == "combat_damage" and not notice._waiting(), "Curse starts automatically inside damage sequence")
	_expect(int(screen._actor_profiles.goblin._stat_labels.removed.text) == 3 and screen._actor_profiles.goblin.health.value == 13, "Removed and health finish before Curse")
	cue = screen._curse_attack_origins[source.id]
	notice._elapsed = 0.3; notice.refresh()
	var origin: Vector2 = notice.get_global_transform_with_canvas().affine_inverse() * cue.get_global_rect().get_center()
	_expect(notice._defense_start.distance_to(origin) < 1, "green launch originates at the stack follow-up label")
	_expect(not notice._label.visible and not notice._ability_label.visible, "no detached purple pop-ups")
	await _capture("hexbrand-stack-launch-%d" % viewport.x)
	tray = screen._enemy_dice_dock.get_child(0)
	notice._elapsed = notice._launch_end() + (notice.duration - notice._launch_end()) * 0.2; notice.refresh()
	_expect(notice._face_batch_end() == 3 and notice.duration >= 2.8, "three applications share one longer beat")
	_expect(notice._batch_rolls.size() == 1 and notice._batch_rolls[0].visible, "expansion check has a concurrent roll preview")
	_expect(not tray._mark_faces[0][0].get_meta("cursed") and not tray._mark_faces[1][4].get_meta("cursed"), "all marks wait for shared arrival")
	notice._elapsed = notice._launch_end() + (notice.duration - notice._launch_end()) * 0.57; notice.refresh(); await process_frame; notice.refresh()
	_expect(notice._batch_targets.size() == 3, "three simultaneous exact-face trails, including two faces of the same die")
	_expect("Clean dice: face 1" in notice._label.text and "roll to expand" in notice._label.text, "mixed direct and rolled marks explained")
	_expect(root.get_visible_rect().encloses(cue.get_global_rect()), "stack cue fits viewport")
	_expect(tray._mark_faces[0][0].get_meta("cursed") and tray._mark_faces[1][0].get_meta("cursed") and tray._mark_faces[1][4].get_meta("cursed"), "all three marks arrive together")
	await _capture("hexbrand-curse-batch-%d" % viewport.x)
	screen._render(); await process_frame; notice.refresh(); await process_frame; notice.refresh()
	_expect(notice._batch_targets.size() == 3, "all trails reanchor after redraw")
	# Exercise three simultaneous rolled results, including repeat checks of D2.
	tray = screen._enemy_dice_dock.get_child(0)
	tray.display([{"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 4}, {"die_id": "brine_d6", "face": 2}, {"die_id": "brine_d6", "face": 5}, {"die_id": "brine_d6", "face": 6}])
	var saved_dice: Array = tray._dice.duplicate(true)
	var saved_numbers := tray._numbers.map(func(number): return number.text)
	for change in notice.feedback.changes: change.rolled = true
	notice._elapsed = notice._launch_end() + (notice.duration - notice._launch_end()) * 0.2; notice.refresh(); await process_frame; notice.refresh()
	_expect(not notice._roll.visible and notice._batch_rolls.size() == 3, "no detached roll: all three checks overlay their slots")
	for i in 3:
		var preview: Label = notice._batch_rolls[i]
		var index := int(notice.feedback.changes[i].index)
		_expect(tray._buttons[index].get_global_rect().grow(5).has_point(preview.get_global_rect().get_center()), "each roll is anchored to the correct physical die")
	_expect(notice._batch_rolls[1].position.x != notice._batch_rolls[2].position.x, "two results from one die remain separately visible in its slot")
	await _capture("hexbrand-tray-rolling-%d" % viewport.x)
	notice._elapsed = notice._launch_end() + (notice.duration - notice._launch_end()) * 0.57; notice.refresh(); await process_frame; notice.refresh()
	for i in 3:
		_expect(notice._batch_rolls[i].text.ends_with(str(int(notice.feedback.changes[i].face))), "all rolls settle to authority faces")
		var start: Vector2 = notice._batch_starts[i]
		_expect(start.y < notice._batch_targets[i].position.y, "lines run from the die down to its face map")
	_expect(tray._dice == saved_dice and tray._numbers.map(func(number): return number.text) == saved_numbers, "offensive faces remain unchanged beneath temporary checks")
	await _capture("hexbrand-tray-results-%d" % viewport.x)
	notice._process(notice.duration); await process_frame; await process_frame
	_expect(not is_instance_valid(notice) and not screen._director.has_beats(), "all three applications finish automatically")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _multiple_followups(base: Dictionary) -> void:
	var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
	f.snapshot.stage = "damage_reaction"; f.snapshot.segment = "damage_resolution"
	var batch := {"id": "two-followups", "sources": [], "removals": [], "actors_before": f.snapshot.actors.duplicate(true)}
	for target in ["blade", "goblin"]:
		batch.sources.append({"id": "source-" + target, "source_actor_id": "goblin" if target == "blade" else "blade", "target_actor_id": target, "source_content_id": "hexbrand", "base_amount": 0, "final_amount": 0})
		f.snapshot.actors[target].owned_dice[0].cursed_faces = []
	f.snapshot.damage_sources = batch.sources; f.snapshot.settled_damage = batch
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("two-curse-followups.json")); root.add_child(screen)
	await process_frame
	var result := f.duplicate(true)
	result.events = [{"sequence": 200000, "type": "damage_committed", "segment": "damage_resolution", "data": batch}]
	for i in 2:
		var target: String = ["blade", "goblin"][i]
		result.snapshot.actors[target].owned_dice[0].cursed_faces = [1]
		result.events.append({"sequence": 200001 + i, "type": "curse_resolved", "actor_id": target, "segment": "damage_resolution", "data": {"kind": "face_marked", "die_id": result.snapshot.actors[target].owned_dice[0].id, "face": 1, "placement": "direct", "source_ability_id": "hexbrand", "attack_source_id": "source-" + target}})
	screen._apply_model_result(result)
	var notices := screen.get_children().filter(func(child): return child.get_script() == NOTICE)
	_expect(notices.size() == 2 and screen._director._queue.size() == 1, "two attacks retain separate follow-ups within the same damage beat")
	for notice in notices: notice.set_process(false)
	var deadline := Time.get_ticks_msec() + 7000
	while notices.any(func(notice): return notice._waiting()) and Time.get_ticks_msec() < deadline: await process_frame
	_expect(notices.all(func(notice): return not notice._waiting()), "both attack follow-ups start together, including zero-card damage")
	for notice in notices:
		notice._elapsed = 0.3; notice.refresh()
		var cue: Control = screen._curse_attack_origins[notice.feedback.attack_source_id]
		var origin: Vector2 = notice.get_global_transform_with_canvas().affine_inverse() * cue.get_global_rect().get_center()
		_expect(notice._defense_start.distance_to(origin) < 1, "each attack launches from its own stack cue")
		_expect(notice._batch_dice.size() == 1, "each attack retains its own receiving die")
	for notice in notices: notice._process(notice.duration)
	await process_frame; await process_frame
	_expect(not screen._director.has_beats(), "all follow-ups complete without an extra click")
	screen._director.queue_result(result, 0, {})
	_expect(not screen._director.has_beats() and screen._director.take_curse_updates().is_empty(), "neither attack replays on repeated authority snapshots")
	screen.queue_free(); await process_frame

func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_HEXBRAND_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await process_frame; RenderingServer.force_draw(); root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("HEXBRAND CURSE SEQUENCE: " + message)
