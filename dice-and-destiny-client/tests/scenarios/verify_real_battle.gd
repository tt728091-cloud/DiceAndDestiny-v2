extends SceneTree

func _init() -> void:
	var gateway := BattleGateway.new()
	var battle_id := "native-smoke-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start"): return
	var pending: Dictionary = started.get("pending_input", {}).get("blade", {})
	if started.get("snapshot", {}).get("segment") != "offensive" or "planning_roll" not in pending.get("allowed_commands", []): _fail("start did not reach Offensive planning: %s" % JSON.stringify(started)); return
	var rolled := gateway.submit(BattleCommandBuilder.planning_roll(battle_id, "blade", pending))
	if not _accepted(rolled, "roll"): return
	var dice: Array = rolled.get("snapshot", {}).get("actors", {}).get("blade", {}).get("roll_history", [])[-1].get("dice", [])
	if dice.size() != 5: _fail("real roll did not return five dice"); return
	for die in dice:
		if int(die.get("face", 0)) < 1 or die.get("symbols", []).is_empty(): _fail("real die lacks face/symbol: %s" % JSON.stringify(die)); return
	var reopened := gateway.open_battle(battle_id, "blade")
	if not _accepted(reopened, "reopen"): return
	if reopened.get("snapshot", {}).get("battle_id") != battle_id or not reopened.get("events", []).is_empty(): _fail("reopen changed structure: %s" % JSON.stringify(reopened)); return
	print("REAL NATIVE SMOKE: start, random 5D6 roll, persistence, reopen passed (%s)" % battle_id); quit(0)

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false
func _fail(message: String) -> void: push_error(message); quit(1)
