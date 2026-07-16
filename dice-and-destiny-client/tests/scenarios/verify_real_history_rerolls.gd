extends SceneTree

func _init() -> void:
	var gateway := BattleGateway.new()
	var source_id := "history-rerolls-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(source_id, "blade")
	if not _accepted(started, "start reroll history battle"): return

	# First create an active replacement branch. This is the condition that used
	# to leave latest_battle_id pointing at the original pre-roll battle.
	var bootstrap := gateway.mark_dev_history(source_id, "blade", "Create replacement branch", "decision", 0, {}, {"type": "bootstrap"})
	if not _accepted(bootstrap, "mark replacement origin"): return
	var bootstrap_id := str(bootstrap.get("data", {}).get("point", {}).get("id", ""))
	var review := gateway.jump_dev_history(source_id, "blade", bootstrap_id)
	if not _accepted(review, "review replacement origin"): return
	var branch_id := str(review.get("snapshot", {}).get("battle_id", ""))
	var replaced := gateway.commit_dev_history(branch_id, "blade", "replace")
	if not _accepted(replaced, "activate replacement branch"): return

	var pending: Dictionary = replaced.get("pending_input", {}).get("blade", {})
	var roll_command := BattleCommandBuilder.planning_roll(branch_id, "blade", pending)
	var roll_action := _command_action(roll_command)
	var roll_point := gateway.mark_dev_history(branch_id, "blade", "Roll 5 Dice", "decision", 0, {}, roll_action)
	if not _accepted(roll_point, "mark first roll"): return
	var roll_point_id := str(roll_point.get("data", {}).get("point", {}).get("id", ""))
	var rolled := gateway.submit(roll_command)
	if not _accepted(rolled, "perform first roll"): return

	var reroll_one_action := {"type": "reroll_unkept", "kept_indices": [0], "reroll_indices": [1, 2, 3, 4]}
	var reroll_one_point := gateway.mark_dev_history(branch_id, "blade", "Reroll Unkept Dice", "decision", 0, {"selected_indices": [0]}, reroll_one_action)
	if not _accepted(reroll_one_point, "mark second roll"): return
	pending = rolled.get("pending_input", {}).get("blade", {})
	var kept_once := gateway.submit(BattleCommandBuilder.planning_keep(branch_id, "blade", pending, [0]))
	if not _accepted(kept_once, "keep before second roll"): return
	pending = kept_once.get("pending_input", {}).get("blade", {})
	var rerolled_once := gateway.submit(BattleCommandBuilder.planning_reroll(branch_id, "blade", pending, [1, 2, 3, 4]))
	if not _accepted(rerolled_once, "perform second roll"): return

	var reroll_two_action := {"type": "reroll_unkept", "kept_indices": [0, 1], "reroll_indices": [2, 3, 4]}
	var reroll_two_point := gateway.mark_dev_history(branch_id, "blade", "Reroll Unkept Dice", "decision", 0, {"selected_indices": [0, 1]}, reroll_two_action)
	if not _accepted(reroll_two_point, "mark final roll"): return
	pending = rerolled_once.get("pending_input", {}).get("blade", {})
	var kept_twice := gateway.submit(BattleCommandBuilder.planning_keep(branch_id, "blade", pending, [0, 1]))
	if not _accepted(kept_twice, "keep before final roll"): return
	pending = kept_twice.get("pending_input", {}).get("blade", {})
	var final_roll := gateway.submit(BattleCommandBuilder.planning_reroll(branch_id, "blade", pending, [2, 3, 4]))
	if not _accepted(final_roll, "perform final roll"): return

	var rewind := gateway.jump_dev_history(branch_id, "blade", roll_point_id)
	if not _accepted(rewind, "rewind to first roll"): return
	var replay_id := str(rewind.get("snapshot", {}).get("battle_id", ""))
	var preserved := gateway.commit_dev_history(replay_id, "blade", "preserve")
	if not _accepted(preserved, "retain reroll future"): return
	var replay_roll := gateway.replay_dev_history_action(replay_id, "blade", roll_action)
	if not _accepted(replay_roll, "replay first roll"): return
	var replay_second := gateway.replay_dev_history_action(replay_id, "blade", reroll_one_action)
	if not _accepted(replay_second, "replay second roll"): return
	var replay_final := gateway.replay_dev_history_action(replay_id, "blade", reroll_two_action)
	if not _accepted(replay_final, "replay final roll"): return
	if not _same_state(final_roll.get("snapshot", {}), replay_final.get("snapshot", {})):
		_fail("final reroll replay did not restore the active replacement future")
		return
	var timeline := gateway.list_dev_history(replay_id, "blade")
	if not _accepted(timeline, "list completed reroll replay"): return
	if timeline.get("data", {}).get("timeline", {}).get("points", []).size() != 3 or timeline.get("data", {}).get("timeline", {}).get("branch", {}).get("status") != "active":
		_fail("completed reroll replay dropped history or remained stuck: %s" % JSON.stringify(timeline))
		return

	print("REAL HISTORY REROLLS: replacement branch rewound and replayed all three rolls to its true latest checkpoint")
	quit(0)

func _command_action(command_json: String) -> Dictionary:
	var command: Dictionary = JSON.parse_string(command_json)
	return {"type": "command", "actor_id": str(command.get("actor_id", "blade")), "command_type": str(command.get("type", "")), "payload": command.get("payload", {}).duplicate(true)}

func _same_state(left_value, right_value) -> bool:
	if not left_value is Dictionary or not right_value is Dictionary: return false
	var left: Dictionary = left_value.duplicate(true)
	var right: Dictionary = right_value.duplicate(true)
	left.erase("battle_id"); right.erase("battle_id")
	return left == right

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)])
	return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
