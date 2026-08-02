extends SceneTree

func _init() -> void:
	var gateway := BattleGateway.new()
	var battle_id := "authority-smoke-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if started.get("accepted") != true:
		push_error("Authority did not start battle: %s" % JSON.stringify(started))
		quit(1)
		return
	var pending: Dictionary = started.get("pending_input", {}).get("blade", {})
	var rolled := gateway.submit(BattleCommandBuilder.planning_roll(battle_id, "blade", pending))
	if rolled.get("accepted") != true:
		push_error("Authority did not accept planning roll: %s" % JSON.stringify(rolled))
		quit(1)
		return
	var history: Array = rolled.get("snapshot", {}).get("actors", {}).get("blade", {}).get("roll_history", [])
	if history.is_empty() or history[-1].get("dice", []).size() != 5:
		push_error("Authority did not return five combat dice: %s" % JSON.stringify(rolled))
		quit(1)
		return
	if OS.get_environment("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT") == "1":
		var transcript_path := WorkspacePaths.persistent_file("debug/authority-transcript.jsonl")
		if transcript_path != OS.get_environment("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH").simplify_path() or not FileAccess.file_exists(transcript_path):
			push_error("Authority transcript was not written to the launcher-owned workspace path: %s" % transcript_path)
			quit(1)
			return
		var transcript_file := FileAccess.open(transcript_path, FileAccess.READ)
		var saw_started := false
		var saw_roll := false
		var last_sequence := 0
		while not transcript_file.eof_reached():
			var line := transcript_file.get_line()
			if line.is_empty(): continue
			var record = JSON.parse_string(line)
			if not record is Dictionary:
				push_error("Authority transcript contains invalid JSONL: %s" % line)
				quit(1)
				return
			if int(record.get("sequence", 0)) <= last_sequence:
				push_error("Authority transcript sequence is not monotonic: %s" % line)
				quit(1)
				return
			last_sequence = int(record.get("sequence", 0))
			if str(record.get("battle_id", "")) != battle_id: continue
			if record.get("kind") == "battle_started": saw_started = true
			if record.get("kind") == "die_rolled" and record.get("actor_id") == "blade" and record.get("visibility") == "PRIVATE_SELF":
				saw_roll = record.get("details", {}).get("dice", []).size() == 5
		if not saw_started or not saw_roll:
			push_error("Native authority transcript omitted lifecycle or actual 5D6 roll: started=%s roll=%s" % [saw_started, saw_roll])
			quit(1)
			return
	print("BATTLE AUTHORITY: start and 5D6 planning roll passed (%s)" % battle_id)
	quit(0)
