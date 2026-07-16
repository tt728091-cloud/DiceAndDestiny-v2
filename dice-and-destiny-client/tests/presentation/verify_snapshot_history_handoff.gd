extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	OS.set_environment("DICE_AND_DESTINY_ENABLE_SNAPSHOTS", "1")
	OS.set_environment("DICE_AND_DESTINY_ENABLE_HISTORY", "1")
	root.size = Vector2i(1280, 720)
	var source_point := {"id": "history-0000000000000011", "label": "Roll 5 Dice", "kind": "decision", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 0, "presented_sequence": 0, "action_type": "command"}
	var source_endpoint := {"id": "history-0000000000000012", "parent_point_id": source_point.id, "label": "Offensive Dice 1/3", "kind": "decision", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 1, "presented_sequence": 1}
	var source_branch := {"battle_id": "snapshot-history-source", "root_battle_id": "snapshot-history-source", "head_point_id": source_endpoint.id, "status": "active"}
	var source_timeline := {"branch": source_branch, "points": [source_point, source_endpoint]}
	var restored_point := source_point.duplicate(true); restored_point.id = "history-0000000000000021"
	var restored_endpoint := source_endpoint.duplicate(true); restored_endpoint.id = "history-0000000000000022"; restored_endpoint.parent_point_id = restored_point.id
	var restored_branch := {"battle_id": "battle-snapshot-restored-review", "root_battle_id": "battle-snapshot-restored-review-latest", "head_point_id": restored_endpoint.id, "cursor_point_id": restored_point.id, "parent_battle_id": "battle-snapshot-restored-review-latest", "latest_battle_id": "battle-snapshot-restored-review-latest", "base_point_id": restored_point.id, "status": "review"}
	var restored_timeline := {"branch": restored_branch, "points": [restored_point, restored_endpoint]}

	var fake := FakeBattleAuthority.new()
	fake.enqueue({"accepted": true, "data": {"timeline": source_timeline}})
	fake.enqueue({"accepted": true, "data": {"point": source_endpoint, "timeline": source_timeline}})
	var loaded := _fixture("battle-snapshot-restored-review")
	loaded.data = {"loaded_snapshot": {"name": "review-history", "event_count": 1, "history_included": true, "history_point_count": 2}, "history": {"restored": true, "review": true, "replay": false, "branch": restored_branch, "timeline": restored_timeline, "point": restored_point, "presented_sequence": 0, "client_state": {"selected_indices": [1, 3]}, "origin_battle_id": "battle-snapshot-restored-review-latest"}}
	fake.enqueue(loaded)
	fake.enqueue({"accepted": true, "data": {"timeline": restored_timeline}})

	var store := ActiveBattleStore.new("user://verify_snapshot_history_handoff_active.json"); store.clear()
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = _fixture("snapshot-history-source"); screen.gateway = BattleGateway.new(fake); screen.active_store = store
	root.add_child(screen); await process_frame; await process_frame
	screen.call("_load_dev_snapshot", "review-history"); await process_frame; await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var restored: Node = screens[-1]
	var state: Dictionary = restored.inspection_state()
	if not state.get("history_review", false) or state.get("history_point_id") != restored_point.id or state.get("history_points", []).size() != 2 or state.get("loaded_snapshot_name") != "review-history":
		_fail("snapshot load did not restore review history context: %s" % state); return
	var selected := _history_button(restored, restored_point.id)
	if selected == null or not selected.button_pressed or _button(restored, "Resume Here · Keep Existing Future") == null:
		_fail("restored snapshot history was not visibly selected and navigable"); return
	var active := store.load_active()
	if active.get("battle_id") != "battle-snapshot-restored-review" or active.get("history_context", {}).get("review") != true or int(active.get("last_sequence", -1)) != 0:
		_fail("restored snapshot history context was not persisted: %s" % active); return
	var command_types: Array = []
	for command_json in fake.commands: command_types.append(JSON.parse_string(command_json).get("type"))
	if command_types != ["list_dev_history", "mark_dev_history", "load_dev_snapshot", "list_dev_history"]:
		_fail("snapshot history handoff used unexpected authority commands: %s" % command_types); return
	store.clear(); restored.queue_free(); await process_frame
	print("SNAPSHOT HISTORY HANDOFF: restored timeline, selected review cursor, client state, and provenance")
	quit(0)

func _fixture(battle_id: String) -> Dictionary:
	return {"accepted": true, "events": [], "pending_input": {"blade": {"id": "pending", "window_id": "window", "segment": "offensive", "stage": "planning", "iteration": 1, "planning_cycle": 1, "allowed_commands": ["planning_roll", "planning_pass"]}}, "snapshot": {"battle_id": battle_id, "status": "active", "viewer_actor_id": "blade", "round": 1, "segment": "offensive", "stage": "planning", "actors": {"blade": {"definition_id": "blade_warden", "hand": [], "card_instances": {}, "current_health": 20, "max_health": 20, "deck_count": 15, "hand_count": 5, "roll_history": [], "offensive_abilities": ["sword_cut"], "defensive_abilities": ["basic_defense"]}, "goblin": {"definition_id": "venom_goblin", "current_health": 12, "max_health": 12, "deck_count": 9, "hand_count": 3, "offensive_abilities": ["jagged_slash"], "defensive_abilities": ["basic_defense"]}}}, "data": {}}

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _history_button(node: Node, point_id: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.has_meta("inspection_id") and str(child.get_meta("inspection_id")) == "battle.history.point.%s" % point_id: return child
	return null

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
