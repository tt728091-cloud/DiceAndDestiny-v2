class_name ScenarioCommandBuilder
extends RefCounted

const CONTROL_BATTLE_ID := "scenario-control"

static func list_scenarios(actor_id: String = "player") -> String:
	return JSON.stringify(_envelope(CONTROL_BATTLE_ID, actor_id, "list_scenarios", {}))

static func validate_named(scenario_id: String, actor_id: String = "player") -> String:
	return JSON.stringify(_envelope(
		CONTROL_BATTLE_ID,
		actor_id,
		"validate_scenario",
		{"scenario_id": scenario_id}
	))

static func start_named(scenario_id: String, actor_id: String = "player") -> String:
	return JSON.stringify(_envelope(
		CONTROL_BATTLE_ID,
		actor_id,
		"start_scenario",
		{"scenario_id": scenario_id}
	))

static func open_battle(battle_id: String, actor_id: String = "player") -> String:
	return JSON.stringify(_envelope(battle_id, actor_id, "open_battle", {}))

static func _envelope(
	battle_id: String,
	actor_id: String,
	command_type: String,
	payload: Dictionary
) -> Dictionary:
	return {
		"battle_id": battle_id,
		"actor_id": actor_id,
		"type": command_type,
		"payload": payload,
	}
