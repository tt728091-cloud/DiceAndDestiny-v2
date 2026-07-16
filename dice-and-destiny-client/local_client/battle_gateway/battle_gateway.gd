class_name BattleGateway
extends RefCounted

var _authority: BattleAuthority

func _init(authority: BattleAuthority = null) -> void:
	_authority = authority if authority != null else GoGDExtensionBattleAuthority.new()

func list_scenarios(actor_id: String = "player") -> Dictionary:
	return _submit(ScenarioCommandBuilder.list_scenarios(actor_id))

func validate_named_scenario(scenario_id: String, actor_id: String = "player") -> Dictionary:
	return _submit(ScenarioCommandBuilder.validate_named(scenario_id, actor_id))

func start_named_scenario(scenario_id: String, actor_id: String = "player") -> Dictionary:
	return _submit(ScenarioCommandBuilder.start_named(scenario_id, actor_id))

func start_battle(battle_id: String, actor_id: String = "blade") -> Dictionary:
	return _submit(BattleCommandBuilder.start_battle(battle_id, actor_id))

func open_battle(battle_id: String, actor_id: String = "blade") -> Dictionary:
	return _submit(BattleCommandBuilder.open_battle(battle_id, actor_id))

func list_dev_snapshots(battle_id: String, actor_id: String = "blade") -> Dictionary:
	return _submit(BattleCommandBuilder.list_dev_snapshots(battle_id, actor_id))

func save_dev_snapshot(battle_id: String, actor_id: String, snapshot_name: String, overwrite: bool = false) -> Dictionary:
	return _submit(BattleCommandBuilder.save_dev_snapshot(battle_id, actor_id, snapshot_name, overwrite))

func load_dev_snapshot(battle_id: String, actor_id: String, snapshot_name: String) -> Dictionary:
	return _submit(BattleCommandBuilder.load_dev_snapshot(battle_id, actor_id, snapshot_name))

func list_dev_history(battle_id: String, actor_id: String = "blade") -> Dictionary:
	return _submit(BattleCommandBuilder.list_dev_history(battle_id, actor_id))

func mark_dev_history(battle_id: String, actor_id: String, label: String, kind: String, presented_sequence: int, client_state: Dictionary = {}, action: Dictionary = {}) -> Dictionary:
	return _submit(BattleCommandBuilder.mark_dev_history(battle_id, actor_id, label, kind, presented_sequence, client_state, action))

func jump_dev_history(battle_id: String, actor_id: String, point_id: String) -> Dictionary:
	return _submit(BattleCommandBuilder.jump_dev_history(battle_id, actor_id, point_id))

func commit_dev_history(battle_id: String, actor_id: String, mode: String) -> Dictionary:
	return _submit(BattleCommandBuilder.commit_dev_history(battle_id, actor_id, mode))

func return_dev_history_latest(battle_id: String, actor_id: String) -> Dictionary:
	return _submit(BattleCommandBuilder.return_dev_history_latest(battle_id, actor_id))

func replay_dev_history_action(battle_id: String, actor_id: String, action: Dictionary) -> Dictionary:
	return _submit(BattleCommandBuilder.replay_dev_history_action(battle_id, actor_id, action))

func replace_dev_history_future(battle_id: String, actor_id: String) -> Dictionary:
	return _submit(BattleCommandBuilder.replace_dev_history_future(battle_id, actor_id))

func submit(command_json: String) -> Dictionary:
	return _submit(command_json)

func submit_pending_command(
	battle_id: String,
	actor_id: String,
	command_type: String,
	pending: Dictionary
) -> Dictionary:
	var command_json := BattleCommandBuilder.from_pending(
		battle_id,
		actor_id,
		command_type,
		pending
	)
	if command_json.is_empty():
		return {"accepted": false, "error": "No builder for command %s" % command_type}
	return _submit(command_json)

func _submit(command_json: String) -> Dictionary:
	var result_json := _authority.submit_command(command_json)
	var parsed = JSON.parse_string(result_json)
	if parsed is Dictionary:
		return parsed
	return {
		"accepted": false,
		"error": "Authority returned invalid JSON",
		"raw_result": result_json,
	}
