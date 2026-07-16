extends SceneTree

func _init() -> void:
	var gateway := BattleGateway.new()
	var source_id := "history-native-source-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(source_id, "blade")
	if not _accepted(started, "start source battle"): return
	var initial_snapshot: Dictionary = started.get("snapshot", {}).duplicate(true)
	var initial_pending: Dictionary = started.get("pending_input", {}).get("blade", {})
	var roll_command := BattleCommandBuilder.planning_roll(source_id, "blade", initial_pending)
	var roll_action := _command_action(roll_command)

	var first := gateway.mark_dev_history(source_id, "blade", "Roll 5 Dice", "decision", 0, {"selected_indices": [1, 3], "selected_card": {"definition_id": "loaded_die"}}, roll_action)
	if not _accepted(first, "mark first decision"): return
	var first_id := str(first.get("data", {}).get("point", {}).get("id", ""))
	if first_id.is_empty(): _fail("first history point had no ID"); return
	var rolled := gateway.submit(roll_command)
	if not _accepted(rolled, "advance source after first point"): return
	if rolled.get("snapshot", {}).get("actors", {}).get("blade", {}).get("roll_history", []).is_empty(): _fail("source battle did not advance after the first history point"); return
	var recorded_second_action := {"type": "recorded_second_action", "choice": "keep future"}
	var second := gateway.mark_dev_history(source_id, "blade", "Recorded Later Choice", "decision", 0, {"selected_indices": [0, 2]}, recorded_second_action)
	if not _accepted(second, "mark second decision"): return
	var listed := gateway.list_dev_history(source_id, "blade")
	if not _accepted(listed, "list history"): return
	var points: Array = listed.get("data", {}).get("timeline", {}).get("points", [])
	var second_id := str(second.get("data", {}).get("point", {}).get("id", ""))
	if points.size() != 2 or points[0].get("id") != first_id or points[1].get("id") != second_id: _fail("history timeline was incomplete or out of order: %s" % JSON.stringify(listed)); return

	var review := gateway.jump_dev_history(source_id, "blade", first_id)
	if not _accepted(review, "jump to first point"): return
	var review_id := str(review.get("snapshot", {}).get("battle_id", ""))
	var history_data: Dictionary = review.get("data", {}).get("history", {})
	if review_id.is_empty() or review_id == source_id or history_data.get("review") != true: _fail("jump did not create a read-only review battle: %s" % JSON.stringify(review)); return
	if not _same_state(initial_snapshot, review.get("snapshot", {})): _fail("review state did not match the point before the first roll"); return
	if _int_array(history_data.get("client_state", {}).get("selected_indices", [])) != [1, 3]: _fail("review did not restore client selection state: %s" % JSON.stringify(history_data)); return
	if review.get("snapshot", {}).get("actors", {}).get("goblin", {}).has("hand"): _fail("history review exposed hidden enemy hand state"); return
	var blocked := gateway.submit(BattleCommandBuilder.planning_roll(review_id, "blade", review.get("pending_input", {}).get("blade", {})))
	if blocked.get("accepted") == true or "read-only" not in str(blocked.get("error", "")): _fail("review battle accepted gameplay: %s" % JSON.stringify(blocked)); return
	var snapshot_name := "history-review-native-%d" % Time.get_ticks_usec()
	var captured := gateway.save_dev_snapshot(review_id, "blade", snapshot_name)
	if not _accepted(captured, "capture a shareable snapshot from review"): return
	var loaded_review := gateway.load_dev_snapshot(review_id, "blade", snapshot_name)
	if not _accepted(loaded_review, "load reviewed snapshot with history"): return
	var loaded_review_id := str(loaded_review.get("snapshot", {}).get("battle_id", ""))
	var loaded_history: Dictionary = loaded_review.get("data", {}).get("history", {})
	var loaded_points: Array = loaded_history.get("timeline", {}).get("points", [])
	if loaded_review_id.is_empty() or loaded_history.get("review") != true or loaded_points.size() != 2 or loaded_points[0].get("id") in [first_id, second_id] or loaded_points[1].get("id") in [first_id, second_id]: _fail("review snapshot did not restore an independent complete history: %s" % JSON.stringify(loaded_review)); return
	if not _same_state(initial_snapshot, loaded_review.get("snapshot", {})) or _int_array(loaded_history.get("client_state", {}).get("selected_indices", [])) != [1, 3]: _fail("review snapshot did not restore its cursor state and client selection"); return
	var loaded_latest := gateway.return_dev_history_latest(loaded_review_id, "blade")
	if not _accepted(loaded_latest, "return to latest within loaded review snapshot"): return
	if loaded_latest.get("snapshot", {}).get("battle_id") == source_id or not _same_state(rolled.get("snapshot", {}), loaded_latest.get("snapshot", {})): _fail("loaded review snapshot did not retain an independent latest future"); return
	var returned := gateway.return_dev_history_latest(review_id, "blade")
	if not _accepted(returned, "return to latest"): return
	if returned.get("snapshot", {}).get("battle_id") != source_id or not _same_state(rolled.get("snapshot", {}), returned.get("snapshot", {})): _fail("return latest did not restore the advanced source future"); return

	var preserve_review := gateway.jump_dev_history(source_id, "blade", first_id)
	if not _accepted(preserve_review, "rewind for preserved branch"): return
	var preserve_id := str(preserve_review.get("snapshot", {}).get("battle_id", ""))
	var preserved := gateway.commit_dev_history(preserve_id, "blade", "preserve")
	if not _accepted(preserved, "preserve old future and resume branch"): return
	var preserved_timeline := gateway.list_dev_history(preserve_id, "blade")
	if not _accepted(preserved_timeline, "list preserved future"): return
	if preserved_timeline.get("data", {}).get("timeline", {}).get("points", []).size() != 2 or preserved_timeline.get("data", {}).get("timeline", {}).get("branch", {}).get("status") != "replay": _fail("keeping the future dropped later points: %s" % JSON.stringify(preserved_timeline)); return
	var blocked_replay := gateway.submit(BattleCommandBuilder.planning_roll(preserve_id, "blade", preserved.get("pending_input", {}).get("blade", {})))
	if blocked_replay.get("accepted") == true or "preserved history" not in str(blocked_replay.get("error", "")): _fail("preserved replay bypassed action matching: %s" % JSON.stringify(blocked_replay)); return
	var replayed := gateway.replay_dev_history_action(preserve_id, "blade", roll_action)
	if not _accepted(replayed, "replay matching recorded roll"): return
	var replay_data: Dictionary = replayed.get("data", {}).get("history", {})
	if replay_data.get("replay") != true or replay_data.get("branch", {}).get("cursor_point_id") != second_id or replay_data.get("timeline", {}).get("points", []).size() != 2: _fail("matching action did not move forward while retaining history: %s" % JSON.stringify(replayed)); return
	if not _same_state(rolled.get("snapshot", {}), replayed.get("snapshot", {})): _fail("matching replay did not restore the exact recorded next checkpoint"); return
	var divergence := gateway.replay_dev_history_action(preserve_id, "blade", {"type": "different_action"})
	if divergence.get("accepted") == true or divergence.get("data", {}).get("history", {}).get("divergence") != true or int(divergence.get("data", {}).get("history", {}).get("future_point_count", 0)) != 1: _fail("different action did not require confirmation: %s" % JSON.stringify(divergence)); return
	preserved_timeline = gateway.list_dev_history(preserve_id, "blade")
	if preserved_timeline.get("data", {}).get("timeline", {}).get("points", []).size() != 2: _fail("unconfirmed divergence changed history"); return
	var truncated := gateway.replace_dev_history_future(preserve_id, "blade")
	if not _accepted(truncated, "confirm future replacement"): return
	if truncated.get("data", {}).get("history", {}).get("timeline", {}).get("points", []).size() != 1: _fail("confirmed divergence did not drop only the current and later points: %s" % JSON.stringify(truncated)); return
	var replacement := gateway.mark_dev_history(preserve_id, "blade", "Different Choice", "decision", 0, {}, {"type": "different_action"})
	if not _accepted(replacement, "write divergent replacement point"): return
	var replacement_points: Array = replacement.get("data", {}).get("timeline", {}).get("points", [])
	if replacement_points.size() != 2 or replacement_points[0].get("id") != first_id or replacement_points[1].get("id") == second_id: _fail("replacement history did not retain the past and replace the future: %s" % JSON.stringify(replacement)); return
	var original := gateway.open_battle(source_id, "blade")
	if not _accepted(original, "reopen original after branch play"): return
	if not _same_state(rolled.get("snapshot", {}), original.get("snapshot", {})): _fail("playing the divergent branch mutated the preserved future"); return

	var replace_review := gateway.jump_dev_history(source_id, "blade", first_id)
	if not _accepted(replace_review, "rewind for replacement branch"): return
	var replace_id := str(replace_review.get("snapshot", {}).get("battle_id", ""))
	var replaced := gateway.commit_dev_history(replace_id, "blade", "replace")
	if not _accepted(replaced, "replace old future"): return
	var old_timeline := gateway.list_dev_history(source_id, "blade")
	if not _accepted(old_timeline, "inspect archived future"): return
	if old_timeline.get("data", {}).get("timeline", {}).get("branch", {}).get("status") != "archived": _fail("replace did not archive the prior future metadata: %s" % JSON.stringify(old_timeline)); return
	var archived_play := gateway.submit(BattleCommandBuilder.planning_roll(source_id, "blade", original.get("pending_input", {}).get("blade", {})))
	if archived_play.get("accepted") == true or "archived" not in str(archived_play.get("error", "")): _fail("archived future remained playable after replacement: %s" % JSON.stringify(archived_play)); return
	var new_timeline := gateway.list_dev_history(replace_id, "blade")
	if not _accepted(new_timeline, "inspect replacement branch"): return
	if new_timeline.get("data", {}).get("timeline", {}).get("branch", {}).get("status") != "active": _fail("replacement branch did not become active: %s" % JSON.stringify(new_timeline)); return

	print("REAL DEV HISTORY: retained timeline, review snapshot restore, exact replay, divergence, return latest, and archived futures passed")
	quit(0)

func _same_state(left_value, right_value) -> bool:
	if not left_value is Dictionary or not right_value is Dictionary: return false
	var left: Dictionary = left_value.duplicate(true)
	var right: Dictionary = right_value.duplicate(true)
	left.erase("battle_id"); right.erase("battle_id")
	return left == right

func _int_array(values: Array) -> Array:
	var result: Array = []
	for value in values: result.append(int(value))
	return result

func _command_action(command_json: String) -> Dictionary:
	var command: Dictionary = JSON.parse_string(command_json)
	return {"type": "command", "actor_id": str(command.get("actor_id", "blade")), "command_type": str(command.get("type", "")), "payload": command.get("payload", {}).duplicate(true)}

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
