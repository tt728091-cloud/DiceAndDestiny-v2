extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new("user://verify_real_history_scene_active.json"); store.clear()
	var battle_id := "history-scene-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start battle"): return
	var roll_command := BattleCommandBuilder.planning_roll(battle_id, "blade", started.get("pending_input", {}).get("blade", {}))
	var first := gateway.mark_dev_history(battle_id, "blade", "Roll 5 Dice", "decision", started.get("events", []).size(), {}, _command_action(roll_command))
	if not _accepted(first, "record first UI action"): return
	var first_id := str(first.get("data", {}).get("point", {}).get("id", ""))
	var rolled := gateway.submit(roll_command)
	if not _accepted(rolled, "roll before later history point"): return
	var second := gateway.mark_dev_history(battle_id, "blade", "Recorded Later Choice", "decision", rolled.get("events", []).size(), {}, {"type": "recorded_later_choice"})
	if not _accepted(second, "record later UI action"): return
	var second_id := str(second.get("data", {}).get("point", {}).get("id", ""))
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = rolled; screen.gateway = gateway; screen.active_store = store; screen.last_presented_sequence = rolled.get("events", []).size()
	root.add_child(screen); await process_frame; await process_frame
	var state: Dictionary = screen.inspection_state()
	var points: Array = state.get("history_points", [])
	if points.size() != 3 or points[0].get("id") != first_id or points[1].get("id") != second_id or not str(points[2].get("action_key", "")).is_empty(): _fail("real screen did not load its complete history including the current endpoint: %s" % state); return
	var point_button: Button = null
	var history_bar: PanelContainer = null
	for control in screen.find_children("*", "Button", true, false):
		if control.has_meta("inspection_id") and str(control.get_meta("inspection_id")) == "battle.history.point.%s" % first_id: point_button = control
	for control in screen.find_children("*", "PanelContainer", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.history.bar": history_bar = control
	if point_button == null or history_bar == null or history_bar.size.x < 1000 or history_bar.size.y <= 0: _fail("real history point was not jumpable from a laid-out timeline bar"); return
	point_button.pressed.emit(); await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen"); var review: Node = screens[-1]
	var review_state: Dictionary = review.inspection_state()
	if not review_state.get("history_review", false): _fail("real history jump did not enter review mode: %s" % review_state); return
	var review_roll := _button(review, "Roll 5 Dice")
	if review_roll == null or not review_roll.disabled or _button(review, "Return to Latest") == null: _fail("real review screen was not read-only or lacked exit controls"); return
	var review_panel: PanelContainer = null
	for control in review.find_children("*", "PanelContainer", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.history.review": review_panel = control
	if review_panel == null or review_panel.size.x <= 0 or review_panel.size.y <= 0: _fail("real review controls did not complete layout"); return
	_button(review, "Resume Here · Keep Existing Future").pressed.emit(); await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen"); var replay: Node = screens[-1]
	var replay_state: Dictionary = replay.inspection_state()
	if not replay_state.get("history_replay", false) or replay_state.get("history_points", []).size() != 3 or replay_state.get("history_point_id") != first_id: _fail("keeping history did not retain the forward timeline: %s" % replay_state); return
	var replay_roll := _button(replay, "Roll 5 Dice")
	if replay_roll == null or replay_roll.disabled: _fail("recorded replay action was not available"); return
	replay_roll.pressed.emit(); await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen"); var advanced: Node = screens[-1]
	var advanced_state: Dictionary = advanced.inspection_state()
	if not advanced_state.get("history_replay", false) or advanced_state.get("history_point_id") != second_id or advanced_state.get("history_points", []).size() != 3: _fail("matching action did not move forward through retained history: %s" % advanced_state); return
	var different := _button(advanced, "Pass Planning")
	if different == null: different = _button(advanced, "Reroll Unkept")
	if different == null: _fail("no alternate action was available for divergence validation"); return
	different.pressed.emit(); await process_frame; await process_frame
	if advanced.inspection_state().get("history_divergence_pending", {}).is_empty() or _button(advanced, "Cancel · Keep Existing Future") == null: _fail("different action did not prompt before dropping history"); return
	_button(advanced, "Cancel · Keep Existing Future").pressed.emit(); await process_frame
	if advanced.inspection_state().get("history_points", []).size() != 3: _fail("canceling the changed action dropped retained history"); return
	store.clear(); advanced.queue_free(); await process_frame
	print("REAL HISTORY SCENE: retained forward bar, exact-action replay, read-only review, and divergence confirmation passed")
	quit(0)

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false

func _command_action(command_json: String) -> Dictionary:
	var command: Dictionary = JSON.parse_string(command_json)
	return {"type": "command", "actor_id": str(command.get("actor_id", "blade")), "command_type": str(command.get("type", "")), "payload": command.get("payload", {}).duplicate(true)}

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
