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
	print("BATTLE AUTHORITY: start and 5D6 planning roll passed (%s)" % battle_id)
	quit(0)
