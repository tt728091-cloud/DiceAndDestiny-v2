extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("count-transition", 9)
	var events: Array = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/curse_grave_interest_transition.json"))
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
		f.snapshot.stage = "damage_reaction"; f.snapshot.segment = "damage_resolution"; f.snapshot.round = 3
		f.snapshot.actors.goblin.merge({"current_health": 9, "deck_count": 5, "hand_count": 1, "discard_count": 3, "removed_count": 7, "statuses": [{"definition_id": "curse_count", "stacks": 3}]}, true)
		f.snapshot.actors.blade.selected_ability = "hexbrand"; f.snapshot.actors.blade.selected_tier = "skull_4"
		f.snapshot.settled_damage = events[0].data.duplicate(true)
		for i in 5:
			f.snapshot.actors.goblin.owned_dice[i].id = "goblin/die/%d" % (i + 1)
			f.snapshot.actors.goblin.owned_dice[i].cursed_faces = [1]
		var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("count-transition.json")); root.add_child(screen); screen.set_process(false)
		await create_timer(0.85).timeout
		for frame in 5: await process_frame
		var result := f.duplicate(true); result.events = events.duplicate(true)
		result.snapshot.stage = "planning"; result.snapshot.segment = "offensive"; result.snapshot.round = 4
		result.snapshot.actors.goblin.merge({"current_health": 6, "deck_count": 2, "removed_count": 10, "statuses": [{"definition_id": "curse_count", "stacks": 1}]}, true)
		result.snapshot.actors.goblin.owned_dice[4].cursed_faces = [1, 4]
		result.snapshot.actors.goblin.owned_dice[2].cursed_faces = [1, 5, 6]
		screen._apply_model_result(result)
		var notices := screen.get_children().filter(func(node): return node.get_script() == NOTICE)
		_expect(notices.size() == 1, "legacy rolls form one post-damage sequence")
		if notices.is_empty(): screen.queue_free(); continue
		var notice = notices[0]; notice.set_process(false)
		_expect(screen._director._queue.map(func(beat): return beat.type) == ["combat_damage", "attack_curse", "effects_resolved"], "recorded order is damage, curse roll, then Effects")
		_expect(notice.feedback.changes.size() == 3, "legacy rolled marks do not duplicate their physical roll")
		_expect("×3" in screen._actor_profiles.goblin.statuses.text, "Count stays three during damage")
		screen._show_damage_counts(1.0)
		_expect(screen._actor_profiles.goblin.health.value == 6, "three attack damage lowers nine health to six")
		screen._director.advance(); screen._render(true)
		await create_timer(0.85).timeout
		for frame in 5: await process_frame
		_expect(screen._director.peek().event.round == 3, "post-damage curse remains in the originating round")
		notice._step = 2; notice._elapsed = notice.duration * 0.2; notice.refresh()
		_expect("×3" in screen._actor_profiles.goblin.statuses.text, "Count stays three while D5 rolls")
		notice._elapsed = notice.duration * 0.6; notice.refresh()
		_expect("already cursed" in notice._label.text and "3 → 4" in notice._label.text, "D5 face four explains the extra Count")
		_expect("×4" in screen._actor_profiles.goblin.statuses.text and notice._count_target.size.x > 0, "Count changes when the result arrives and has a visual trail")
		await _capture("count-%d" % width)
		screen._render(true); notice.refresh()
		_expect("×4" in screen._actor_profiles.goblin.statuses.text, "redraw keeps revealed Count")
		notice.queue_free(); screen._director.advance(); screen._render(true)
		await create_timer(0.85).timeout
		for frame in 5: await process_frame
		var panel = screen._effects_panel; panel.set_process(false); panel.resume_at(1.5); panel.present_progress()
		_expect(panel._grave_labels.size() == 1 and panel.entries.is_empty(), "legacy conversion has a trigger cue without inventing damage")
		var cue: Label = panel._grave_labels[0].label
		_expect("instead of damage" in cue.text and "4 → 1 Count · 0 damage" in cue.text, "Grave Interest explicitly accounts for all Count")
		_expect(not "Grave Debt" in cue.text, "legacy battle does not invent a status absent from its rules")
		_expect(root.get_visible_rect().encloses(cue.get_global_rect()), "trigger fits viewport")
		await _capture("conversion-%d" % width)
		panel.resume_at(panel.duration); panel.present_progress()
		_expect(screen._actor_profiles.goblin.health.value == 6 and "×1" in screen._actor_profiles.goblin.statuses.text, "Effects retains six health and one Count")
		var log := preload("res://local_client/view_state/combat_log.gd").new()
		log.receive({"snapshot": result.snapshot, "events": [{"type": "curse_resolved", "sequence": 999, "actor_id": "goblin", "data": events[-1].data.steps[-1].data}]})
		_expect("Grave Interest spends 3 Count" in log.text(), "combat log names the reason damage was replaced")
		screen.queue_free(); await process_frame
	print("CURSE COUNT TRANSITION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_COUNT_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
