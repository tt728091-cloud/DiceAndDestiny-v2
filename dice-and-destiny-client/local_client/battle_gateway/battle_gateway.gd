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

func open_battle(battle_id: String, actor_id: String = "player") -> Dictionary:
	return _submit(ScenarioCommandBuilder.open_battle(battle_id, actor_id))

func submit_pending_command(
	battle_id: String,
	actor_id: String,
	command_type: String,
	pending: Dictionary
) -> Dictionary:
	return _submit(BattleCommandBuilder.from_pending(
		battle_id,
		actor_id,
		command_type,
		pending
	))

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
