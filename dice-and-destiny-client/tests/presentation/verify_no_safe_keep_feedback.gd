extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var before := {}; var after := {}
	for seed_value in range(1, 161):
		var result: Dictionary = gateway.start_battle("no-safe-keep-%d" % seed_value, seed_value)
		var marked := false
		for step in 80:
			var action := _card_action(result, "no_safe_keep")
			if not action.is_empty():
				before = result.duplicate(true); after = gateway.submit(JSON.stringify(action)); print("NO SAFE KEEP native seed: ", seed_value); break
			if int(result.get("snapshot", {}).get("round", 1)) > 1: break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			if not marked:
				action = _card_action(result, "mark_the_number")
				if not action.is_empty():
					marked = true
			if action.is_empty():
				for candidate in result.get("legal_actions", []):
					if candidate.get("type") in ["planning_pass", "pass"]: action = candidate; break
			if action.is_empty():
				break
			result = gateway.submit(JSON.stringify(action))
		if not after.is_empty(): break
	_expect(after.get("accepted", false), "real No Safe Keep was legally played")
	if after.is_empty(): quit(1); return
	for width in [1920, 1280]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		await _check(before, after)
	print("NO SAFE KEEP FEEDBACK: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _card_action(result: Dictionary, id: String) -> Dictionary:
	for action in result.get("legal_actions", []):
		var payload: Dictionary = action.get("payload", {})
		var ids: Array = payload.get("card_ids", payload.get("commitment", {}).get("card_ids", []))
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == id: return action
	return {}

func _check(before: Dictionary, after: Dictionary) -> void:
	var initial := before.duplicate(true); initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	var result := after.duplicate(true); result.learned_policy = {}; result.legal_actions = []; result.pending_input = {}
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("no-safe-keep-feedback.json"))
	root.add_child(screen); await create_timer(0.7).timeout
	screen._apply_model_result(result)
	var notices: Array = screen.get_children().filter(func(child): return child.get_script() == NOTICE)
	_expect(notices.size() == 1, "one card-to-die sequence")
	if notices.is_empty(): screen.queue_free(); return
	var notice = notices[0]; notice.set_process(false)
	var change: Dictionary = notice.feedback.changes[0]
	_expect(change.get("kind") == "offensive_reroll" and notice.feedback.card_id == "no_safe_keep", "public roll identifies actual card and roll type")
	var index := int(change.index)
	var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
	_expect(tray._numbers[index].text == str(int(change.face_before)), "new face is hidden before flight")
	var saved: Array = tray._dice.duplicate(true)
	await create_timer(0.7).timeout
	_expect(not notice._waiting(), "feedback is ready before opponent reselection")
	notice._elapsed = notice.duration * 0.45; notice.refresh()
	_expect("Forces this kept die" in notice._label.text and not notice._roll.visible, "card explains forced reroll with no duplicate preview die")
	_expect(notice._target.size.x > 0, "trail targets actual die")
	await _capture("flight-%d" % root.size.x)
	notice._elapsed = notice.duration * 0.61; notice.refresh(); await process_frame
	# This notice is manually clocked: reapply after the newly visible tray
	# finishes its Container layout, just as its live process does each frame.
	notice.refresh()
	var first := tray._numbers[index].text
	_expect("Rolling" in notice._label.text and absf(tray._buttons[index].rotation) > 0.001, "physical die tumbles")
	await _capture("rolling-%d" % root.size.x)
	notice._elapsed += 0.09; notice.refresh()
	_expect(tray._numbers[index].text != first, "faces visibly cycle")
	for i in 5:
		if i != index: _expect(tray._numbers[i].text == str(int(saved[i].face)), "other dice stay fixed")
	screen._render(); await process_frame
	tray = screen._enemy_dice_dock.get_child(0)
	notice.refresh()
	_expect(tray._buttons[index].rotation != 0.0, "redraw resumes same roll")
	var elapsed: float = notice._elapsed
	screen._snapshot_panel_open = true; notice._process(1.0)
	_expect(notice._elapsed == elapsed, "inspection pauses animation")
	screen._snapshot_panel_open = false
	notice._elapsed = notice.duration * 0.86; notice.refresh(); await process_frame
	_expect(tray._numbers[index].text == str(int(change.face)) and tray._buttons[index].rotation == 0.0, "die settles on authoritative face")
	_expect(tray._dice == saved, "presentation does not mutate saved dice")
	_expect("%d → %d" % [int(change.face_before), int(change.face)] in notice._label.text, "caption explains before and after")
	_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()) and not notice._label.get_global_rect().intersects(screen._enemy_dice_dock.get_global_rect()), "caption fits without covering dice")
	await _capture("settled-%d" % root.size.x)
	screen._director.queue_result(result, 0, before.snapshot.actors)
	_expect(screen._director.take_curse_updates().is_empty(), "repeat result cannot replay card")
	notice._process(notice.duration); await process_frame
	_expect(not screen._card_gain_active(), "animation releases automatic play")
	# Equal before/after faces still need a real roll animation, and an early
	# dismissal must restore the final face and upright orientation.
	var same := change.duplicate(true); same.face_before = same.face
	var repeated = NOTICE.new()
	repeated.configure(screen, {"changes": [same], "card_id": "no_safe_keep", "title": "No Safe Keep"})
	screen.add_child(repeated); repeated.set_process(false)
	repeated._elapsed = repeated.duration * 0.61; repeated.refresh()
	_expect(absf(tray._buttons[index].rotation) > 0.001, "unchanged final face still animates")
	repeated.queue_free(); await process_frame
	_expect(tray._buttons[index].rotation == 0.0 and tray._numbers[index].text == str(int(change.face)), "early cleanup restores authoritative face")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _capture(label: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_KEEP_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(label + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("NO SAFE KEEP: " + message)
