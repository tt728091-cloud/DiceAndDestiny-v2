extends SceneTree

const DevToolingGuard := preload("res://tests/support/dev_tooling_guard.gd")

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	if DevToolingGuard.skip_unless_enabled(self, "verify_real_history_presentation", [DevToolingGuard.HISTORY]): return
	root.size = Vector2i(1920, 1080)
	ProjectSettings.set_setting("dice_and_destiny/presentation/income_animation_seconds", 0.2)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_real_history_presentation_active.json")); store.clear()
	var battle_id := "history-presentation-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start presentation battle"): return
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = started; screen.gateway = gateway; screen.active_store = store; screen.last_presented_sequence = 0
	root.add_child(screen); await process_frame; await process_frame
	# A new battle opens on the automatic Income animation, which offers no manual action.
	if str(screen.inspection_state().get("presentation_type", "")) != "income_summary": _fail("new battle did not open on the automatic Income presentation: %s" % screen.inspection_state()); return
	if _button(screen, "Continue Presentation") != null: _fail("automated Income presentation exposed a manual history action"); return
	await create_timer(1.5).timeout; await process_frame; await process_frame

	var points: Array = screen.inspection_state().get("history_points", [])
	var income_index := -1
	for index in points.size():
		if str(points[index].get("action_type", "")) == "presentation_continue": income_index = index; break
	if income_index < 0 or income_index + 1 >= points.size(): _fail("automatic Income was not recorded as a replayable point followed by its result: %s" % [points]); return
	var recorded_ids := _point_ids(points)
	var income_id := str(points[income_index].get("id", "")); var next_id := str(points[income_index + 1].get("id", ""))
	var income_button := _history_button(screen, income_id)
	if income_button == null: _fail("Income presentation point was not jumpable"); return
	income_button.pressed.emit(); await process_frame; await process_frame

	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen"); var review: Node = screens[-1]
	var review_state: Dictionary = review.inspection_state()
	if not review_state.get("history_review", false) or review_state.get("history_point_id") != income_id or str(review_state.get("presentation_type", "")) != "income_summary": _fail("Income review did not restore the recorded presentation beat: %s" % review_state); return
	var resume := _button(review, "Resume Here · Keep Existing Future")
	if resume == null or _button(review, "Continue Presentation") != null: _fail("Income review lacked its resume control or exposed a manual presentation action"); return
	resume.pressed.emit(); await process_frame; await process_frame

	screens = get_nodes_in_group("inspectable_battle_screen"); var replay: Node = screens[-1]
	var replay_state: Dictionary = replay.inspection_state()
	if not replay_state.get("history_replay", false) or replay_state.get("history_point_id") != income_id or _point_ids(replay_state.get("history_points", [])) != recorded_ids: _fail("presentation resume dropped the forward history: %s" % replay_state); return
	# The automatic Income action replays itself and must match the recorded action.
	await create_timer(1.5).timeout; await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen"); var advanced: Node = screens[-1]
	var advanced_state: Dictionary = advanced.inspection_state()
	var advanced_ids := _point_ids(advanced_state.get("history_points", []))
	if not advanced_state.get("history_divergence_pending", {}).is_empty() or advanced_ids.slice(0, recorded_ids.size()) != recorded_ids: _fail("replayed automatic Income diverged from or dropped the recorded history: %s" % advanced_state); return
	if advanced_state.get("history_replay", false) and advanced_state.get("history_point_id") != next_id: _fail("replayed automatic Income did not move to its recorded next point: %s" % advanced_state); return
	if str(advanced_state.get("presentation_type", "")) == "income_summary" or _button(advanced, "Roll 3/3") == null: _fail("replayed Income presentation did not advance to the offensive roll: %s" % advanced_state); return

	store.clear(); advanced.queue_free(); await process_frame
	print("REAL HISTORY PRESENTATION: automatic Income action recorded, reviewed, resumed, and replayed without timeline loss")
	quit(0)

func _point_ids(points: Array) -> Array:
	var ids := []
	for point in points: ids.append(str(point.get("id", "")))
	return ids

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _history_button(node: Node, point_id: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.has_meta("inspection_id") and str(child.get_meta("inspection_id")) == "battle.history.point.%s" % point_id: return child
	return null

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
