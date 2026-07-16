extends SceneTree

func _init() -> void:
	var gateway := BattleGateway.new()
	var source_id := "snapshot-native-source-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var snapshot_name := "round-1-offensive-native-%d" % Time.get_ticks_usec()
	var started := gateway.start_battle(source_id, "blade")
	if not _accepted(started, "start source battle"): return
	var roll_command := BattleCommandBuilder.planning_roll(source_id, "blade", started.get("pending_input", {}).get("blade", {}))
	var marked := gateway.mark_dev_history(source_id, "blade", "Roll 5 Dice", "decision", started.get("events", []).size(), {"selected_indices": [1, 3]}, _command_action(roll_command))
	if not _accepted(marked, "record pre-snapshot history action"): return
	var source_first_id := str(marked.get("data", {}).get("point", {}).get("id", ""))
	var rolled := gateway.submit(roll_command)
	if not _accepted(rolled, "roll before snapshot capture"): return
	var endpoint := gateway.mark_dev_history(source_id, "blade", "Offensive Dice 1/3", "decision", rolled.get("events", []).size(), {"selected_indices": [3]}, {})
	if not _accepted(endpoint, "record pre-snapshot history endpoint"): return
	var source_endpoint_id := str(endpoint.get("data", {}).get("point", {}).get("id", ""))
	var captured_snapshot: Dictionary = rolled.get("snapshot", {}).duplicate(true)

	var saved := gateway.save_dev_snapshot(source_id, "blade", snapshot_name)
	if not _accepted(saved, "capture snapshot"): return
	var duplicate := gateway.save_dev_snapshot(source_id, "blade", snapshot_name)
	if duplicate.get("accepted") == true or "overwrite" not in str(duplicate.get("error", "")):
		_fail("duplicate capture was not safely rejected: %s" % JSON.stringify(duplicate)); return
	var overwritten := gateway.save_dev_snapshot(source_id, "blade", snapshot_name, true)
	if not _accepted(overwritten, "explicit overwrite"): return
	var listed := gateway.list_dev_snapshots(source_id, "blade")
	if not _accepted(listed, "list snapshots"): return
	var entries: Array = listed.get("data", {}).get("snapshots", [])
	var captured_entry: Dictionary = {}
	for entry_value in entries:
		if str(entry_value.get("name", "")) == snapshot_name: captured_entry = entry_value; break
	if captured_entry.is_empty() or int(captured_entry.get("event_count", 0)) < 1 or captured_entry.get("history_included") != true or int(captured_entry.get("history_point_count", 0)) != 2:
		_fail("snapshot metadata listing was incomplete: %s" % JSON.stringify(listed)); return

	var first := gateway.load_dev_snapshot(source_id, "blade", snapshot_name)
	if not _accepted(first, "load snapshot as new battle"): return
	var first_id := str(first.get("snapshot", {}).get("battle_id", ""))
	if first_id.is_empty() or first_id == source_id or first.get("events", []).size() != int(captured_entry.get("event_count", 0)):
		_fail("first load did not create a clean independent battle: %s" % JSON.stringify(first)); return
	if not _same_state(captured_snapshot, first.get("snapshot", {})):
		_fail("first loaded battle did not match captured viewer state"); return
	var first_history: Dictionary = first.get("data", {}).get("history", {})
	var first_points: Array = first_history.get("timeline", {}).get("points", [])
	if first_history.get("restored") != true or first_points.size() != 2 or first_points[0].get("id") in [source_first_id, source_endpoint_id] or first_points[1].get("id") in [source_first_id, source_endpoint_id]:
		_fail("first load did not restore independent history: %s" % JSON.stringify(first)); return
	var restored_review := gateway.jump_dev_history(first_id, "blade", str(first_points[0].get("id", "")))
	if not _accepted(restored_review, "jump through restored snapshot history"): return
	if not _same_state(started.get("snapshot", {}), restored_review.get("snapshot", {})):
		_fail("restored snapshot history did not reach its original pre-roll state"); return
	var restored_review_id := str(restored_review.get("snapshot", {}).get("battle_id", ""))
	var returned := gateway.return_dev_history_latest(restored_review_id, "blade")
	if not _accepted(returned, "return to restored snapshot latest state"): return
	if str(returned.get("snapshot", {}).get("battle_id", "")) != first_id or not _same_state(captured_snapshot, returned.get("snapshot", {})):
		_fail("restored history did not return to its independently loaded latest battle"); return
	var pending: Dictionary = first.get("pending_input", {}).get("blade", {})
	var advanced := gateway.submit(BattleCommandBuilder.planning_pass(first_id, "blade", pending))
	if not _accepted(advanced, "advance first loaded battle"): return
	if advanced.get("events", []).is_empty():
		_fail("loaded battle could not continue through normal authority commands"); return

	var second := gateway.load_dev_snapshot(first_id, "blade", snapshot_name)
	if not _accepted(second, "restart snapshot into another battle"): return
	var second_id := str(second.get("snapshot", {}).get("battle_id", ""))
	if second_id.is_empty() or second_id in [source_id, first_id]:
		_fail("restart did not create a second independent battle: %s" % JSON.stringify(second)); return
	if not _same_state(captured_snapshot, second.get("snapshot", {})):
		_fail("restarted battle did not return to the captured authority state"); return
	var second_points: Array = second.get("data", {}).get("history", {}).get("timeline", {}).get("points", [])
	if second_points.size() != 2 or second_points[0].get("id") == first_points[0].get("id") or second_points[1].get("id") == first_points[1].get("id"):
		_fail("restarting snapshot reused another loaded battle's history identities"); return
	var original := gateway.open_battle(source_id, "blade")
	if not _accepted(original, "reopen original"): return
	if not _same_state(captured_snapshot, original.get("snapshot", {})):
		_fail("loading or advancing clones mutated the original battle"); return

	print("REAL DEV SNAPSHOTS: history bundle, jump/return, independent restart, advance, and source isolation passed")
	quit(0)

func _same_state(left_value, right_value) -> bool:
	if not left_value is Dictionary or not right_value is Dictionary: return false
	var left: Dictionary = left_value.duplicate(true)
	var right: Dictionary = right_value.duplicate(true)
	left.erase("battle_id"); right.erase("battle_id")
	return left == right

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false

func _command_action(command_json: String) -> Dictionary:
	var command: Dictionary = JSON.parse_string(command_json)
	return {"type": "command", "actor_id": str(command.get("actor_id", "blade")), "command_type": str(command.get("type", "")), "payload": command.get("payload", {}).duplicate(true)}

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
