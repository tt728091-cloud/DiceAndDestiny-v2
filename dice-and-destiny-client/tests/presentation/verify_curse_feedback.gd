extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary
	var action := {}
	for seed_value in range(1, 21):
		base = gateway.start_battle("curse-feedback-%d" % seed_value, seed_value)
		for candidate in base.get("legal_actions", []):
			if candidate.get("type") != "planning_commit_cards": continue
			var ids: Array = candidate.payload.get("card_ids", [])
			if not ids.is_empty() and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "mark_the_number": action = candidate; break
		if not action.is_empty(): break
	_expect(not action.is_empty(), "real Mark the Number available")
	if action.is_empty(): quit(1); return
	base.events = []; base.learned_policy = {}
	root.size = Vector2i(1920, 1080)
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("curse-feedback.json")); root.add_child(screen)
	await process_frame
	screen._send(JSON.stringify(action))
	var notices := _notices(screen)
	_expect(notices.size() == 1, "actual card play creates one Curse animation")
	if notices.is_empty(): screen.queue_free(); quit(1); return
	var notice = notices[0]; notice.set_process(false)
	_expect(notice.feedback.card_id == "mark_the_number", "animation names actual card")
	var change: Dictionary = notice.feedback.changes[0]
	_expect(not change.rolled and change.marked and change.face == 1, "first Curse is direct, no fake roll")
	_expect(change.index >= 0 and change.index < 5 and change.actor_id == "goblin", "real event maps to exact owned enemy die")
	var saved_dice: Array = screen._view.actor("goblin").get("dice", []).duplicate(true)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for frame in 8: await process_frame
		notice._elapsed = notice.duration * 0.57; notice.refresh(); await process_frame
		_expect("Applied directly · no roll" in notice._label.text and "Face 1 · Curse applied" in notice._label.text, "caption explains placement")
		_expect(not notice._roll.visible, "direct placement shows no rolling die")
		_expect(notice._target.size.x > 0, "pulse targets exact face chip")
		_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "caption fits viewport")
		await _capture("direct-%d" % viewport.x)
	screen._render(); await process_frame
	_expect(_notices(screen).size() == 1 and _notices(screen)[0] == notice, "redraw preserves existing animation")
	var result := {"snapshot": screen._view.raw_snapshot, "events": screen._view.events}
	screen._director.queue_result(result, 0, base.snapshot.actors)
	_expect(screen._director.take_curse_updates().is_empty(), "duplicate events cannot repeat animation")
	var id := str(change.die_id)
	var face_event := {"type": "curse_resolved", "actor_id": "goblin", "sequence": 100001, "data": {"kind": "owned_roll", "roll_context": "curse_dice", "die": {"owned_id": id, "index": change.index, "face": 3, "die_id": "brine_d6"}, "cursed": false}}
	var mark_event := {"type": "curse_resolved", "actor_id": "goblin", "sequence": 100002, "data": {"kind": "face_marked", "die_id": id, "face": 3, "placement": "rolled"}}
	var director := BattlePresentationDirector.new()
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [face_event, mark_event]})
	var batches := director.take_curse_updates()
	_expect(batches.size() == 1 and batches[0].changes.size() == 1 and batches[0].changes[0].rolled and batches[0].changes[0].marked, "roll and mark coalesce into one accurate outcome")
	var rolled = NOTICE.new(); rolled.configure(screen, batches[0]); screen.add_child(rolled); rolled.set_process(false)
	_expect(rolled._waiting(), "successive Curse feedback is serialized")
	notice.queue_free(); await process_frame
	rolled._elapsed = 0.1; rolled.refresh()
	_expect(not rolled._roll.visible and rolled._batch_rolls.size() == 1 and rolled._batch_rolls[0].visible and "Rolling" in rolled._label.text, "real roll gets preview")
	rolled._elapsed = rolled.duration * 0.57; rolled.refresh()
	_expect("→ 3" in rolled._label.text and "offense unchanged" in rolled._label.text, "roll settles to authoritative result")
	# Fixture includes the final authoritative map; the preview is not state.
	var owned: Array = screen._view.actor("goblin").owned_dice.duplicate(true)
	owned[int(change.index)].cursed_faces.append(3)
	screen._enemy_dice_dock.get_child(0).display_owned(owned)
	await process_frame
	rolled.refresh()
	await _capture("rolled-1280")
	_expect(screen._view.actor("goblin").get("dice", []) == saved_dice, "feedback never overwrites offensive results")
	var elapsed: float = rolled._elapsed
	screen._snapshot_panel_open = true; rolled._process(0.5)
	_expect(rolled._elapsed == elapsed and rolled.modulate.a == 0, "inspection pauses and hides feedback")
	screen._snapshot_panel_open = false
	rolled._process(rolled.duration); await process_frame
	_expect(_notices(screen).is_empty(), "feedback ends automatically")
	# Already cursed rolls add Count without pretending to place another mark.
	face_event.data.cursed = true; face_event.sequence = 100003
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [face_event]})
	var repeat: Dictionary = director.take_curse_updates()[0].changes[0]
	_expect(repeat.rolled and not repeat.marked and repeat.cursed, "already-marked roll retains Count outcome")
	face_event.sequence = 100004; face_event.data.roll_context = "combat_dice"
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": [face_event]})
	_expect(director.take_curse_updates().is_empty(), "ordinary offense uses existing roll animation")
	var marks: Array = []
	var player_owned: Array = screen._view.actor("blade").owned_dice.duplicate(true)
	for i in 5:
		player_owned[i].cursed_faces = [2]
		marks.append({"sequence": 100010 + i, "type": "curse_resolved", "actor_id": "blade", "data": {"kind": "face_marked", "die_id": player_owned[i].id, "face": 2, "placement": "direct"}})
	marks.append({"sequence": 100015, "type": "curse_resolved", "actor_id": "goblin", "data": {"kind": "choice", "card_id": "eclipse_of_the_black_star"}})
	director.queue_result({"snapshot": screen._view.raw_snapshot, "events": marks})
	var many: Dictionary = director.take_curse_updates()[0]
	_expect(many.changes.size() == 5 and many.card_id.is_empty() and many.title == "Eclipse of the Black Star", "ability places multiple faces without inventing a played card")
	screen._player_dice_dock.get_child(0).display_owned(player_owned)
	var multi = NOTICE.new(); multi.configure(screen, many); screen.add_child(multi); multi.set_process(false)
	for frame in 5: await process_frame
	multi._elapsed = multi.duration * 0.57; multi.refresh(); await process_frame; multi.refresh()
	_expect(multi._batch_targets.size() == 5 and multi._face_batch_end() == 5, "all five player faces have simultaneous trails")
	_expect(not multi._label.get_global_rect().intersects(screen._player_dice_dock.get_global_rect()), "player notice remains clear of dice")
	await _capture("five-face-batch")
	multi._process(multi.duration * 0.44)
	await process_frame
	_expect(_notices(screen).is_empty(), "multi-face sequence finishes automatically")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("CURSE FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _notices(screen: Control) -> Array:
	return screen.get_children().filter(func(child): return child.get_script() == NOTICE and not child.is_queued_for_deletion())
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_CURSE_FEEDBACK_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSE FEEDBACK: " + message)
