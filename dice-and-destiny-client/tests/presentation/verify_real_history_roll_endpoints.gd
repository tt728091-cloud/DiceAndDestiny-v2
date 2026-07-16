extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_real_history_roll_endpoints_active.json")); store.clear()
	var battle_id := "history-roll-endpoints-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start roll endpoint battle"): return
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = started; screen.gateway = gateway; screen.active_store = store; screen.last_presented_sequence = 0
	root.add_child(screen); await process_frame; await process_frame

	for step in 2:
		var continue_button := _button(screen, "Continue Presentation")
		if continue_button == null: _fail("startup presentation step %d was unavailable" % (step + 1)); return
		continue_button.pressed.emit(); await process_frame; await process_frame
	var roll := _button(screen, "Roll 5 Dice")
	if roll == null: _fail("initial offensive roll was unavailable"); return
	roll.pressed.emit(); await process_frame; await process_frame
	for roll_number in [2, 3]:
		var reroll := _button(screen, "Reroll Unkept")
		if reroll == null: _fail("offensive reroll %d was unavailable" % roll_number); return
		reroll.pressed.emit(); await process_frame; await process_frame

	var state: Dictionary = screen.inspection_state()
	var history: Array = state.get("history_points", [])
	var rolls: Array = state.get("snapshot", {}).get("actors", {}).get("blade", {}).get("roll_history", [])
	if rolls.size() != 3 or history.size() < 6:
		_fail("three rolls did not produce a separately recorded terminal state: %s" % state)
		return
	var terminal: Dictionary = history[-1]
	if "Offensive Dice 3/3" not in str(terminal.get("label", "")) or not str(terminal.get("action_key", "")).is_empty():
		_fail("latest 3/3 state was not an actionless history endpoint: %s" % terminal)
		return
	var roll_origin_id := str(history[-4].get("id", ""))
	var terminal_id := str(terminal.get("id", ""))
	var terminal_label := str(terminal.get("label", ""))
	var pass_planning := _button(screen, "Pass Planning")
	if pass_planning == null: _fail("post-roll planning action was unavailable"); return
	pass_planning.pressed.emit(); await process_frame; await process_frame
	history = screen.inspection_state().get("history_points", [])
	if history.size() < 2 or str(history[-2].get("id", "")) != terminal_id or str(history[-2].get("label", "")) != terminal_label or not str(history[-1].get("label", "")).begins_with("Pass Planning"):
		_fail("the action after 3/3 replaced or moved the completed third-roll entry: %s" % history)
		return
	var origin_button := _history_button(screen, roll_origin_id)
	if origin_button == null: _fail("earlier roll state was not jumpable"); return
	origin_button.pressed.emit(); await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var earlier: Node = screens[-1]
	var forward_button := _history_button(earlier, terminal_id)
	if forward_button == null: _fail("3/3 endpoint disappeared after jumping backward"); return
	forward_button.pressed.emit(); await process_frame; await process_frame
	screens = get_nodes_in_group("inspectable_battle_screen")
	var restored: Node = screens[-1]
	var restored_state: Dictionary = restored.inspection_state()
	var restored_rolls: Array = restored_state.get("snapshot", {}).get("actors", {}).get("blade", {}).get("roll_history", [])
	if not restored_state.get("history_review", false) or restored_rolls.size() != 3 or restored_state.get("history_point_id") != terminal_id:
		_fail("forward jump did not restore the recorded 3/3 roll state: %s" % restored_state)
		return

	store.clear(); restored.queue_free(); await process_frame
	print("REAL HISTORY ROLL ENDPOINTS: 0/3, 1/3, 2/3, and 3/3 states remained independently jumpable")
	quit(0)

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _history_button(node: Node, point_id: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.has_meta("inspection_id") and child.get_meta("inspection_id") == "battle.history.point.%s" % point_id: return child
	return null

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)])
	return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
