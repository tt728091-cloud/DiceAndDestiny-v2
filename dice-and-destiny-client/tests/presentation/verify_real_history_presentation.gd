extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_real_history_presentation_active.json")); store.clear()
	var battle_id := "history-presentation-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start presentation battle"): return
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = started; screen.gateway = gateway; screen.active_store = store; screen.last_presented_sequence = 0
	root.add_child(screen); await process_frame; await process_frame
	for step in 2:
		var continue_button := _button(screen, "Continue Presentation")
		if continue_button == null: _fail("startup presentation did not provide history step %d" % (step + 1)); return
		continue_button.pressed.emit(); await process_frame; await process_frame
	var state: Dictionary = screen.inspection_state(); var points: Array = state.get("history_points", [])
	if points.size() < 2 or str(points[0].get("action_type", "")) != "presentation_continue" or str(points[1].get("action_type", "")) != "presentation_continue": _fail("presentation continues were not recorded as replayable points: %s" % state); return
	var first_id := str(points[0].get("id", "")); var second_id := str(points[1].get("id", "")); var first_button: Button = null
	for control in screen.find_children("*", "Button", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.history.point.%s" % first_id: first_button = control
	if first_button == null: _fail("first presentation point was not jumpable"); return
	first_button.pressed.emit(); await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen"); var review: Node = screens[-1]
	if not review.inspection_state().get("history_review", false) or _button(review, "Resume Here · Keep Existing Future") == null or _button(review, "Continue Presentation") == null: _fail("presentation review did not retain both its beat and resume controls"); return
	_button(review, "Resume Here · Keep Existing Future").pressed.emit(); await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen"); var replay: Node = screens[-1]
	var replay_state: Dictionary = replay.inspection_state()
	if not replay_state.get("history_replay", false) or replay_state.get("history_points", []).size() < 2 or replay_state.get("history_point_id") != first_id: _fail("presentation resume dropped the forward history: %s" % replay_state); return
	var replay_continue := _button(replay, "Continue Presentation")
	if replay_continue == null: _fail("recorded presentation action was unavailable during replay"); return
	replay_continue.pressed.emit(); await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen"); var advanced: Node = screens[-1]
	var advanced_state: Dictionary = advanced.inspection_state()
	if not advanced_state.get("history_replay", false) or advanced_state.get("history_point_id") != second_id or advanced_state.get("history_points", []).size() < 2: _fail("matching Continue Presentation did not move forward without dropping history: %s" % advanced_state); return
	store.clear(); advanced.queue_free(); await process_frame
	print("REAL HISTORY PRESENTATION: ongoing/income continues retained, resumed, and replayed forward without timeline loss")
	quit(0)

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
