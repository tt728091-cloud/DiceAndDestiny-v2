class_name BattleCommandBuilder
extends RefCounted

static func from_pending(
	battle_id: String,
	actor_id: String,
	command_type: String,
	pending: Dictionary
) -> String:
	var payload: Dictionary = {"pending_input_id": pending.get("id", "")}
	if command_type != "roll_dice":
		payload["checkpoint"] = {
			"window_id": pending.get("window_id", ""),
			"segment": pending.get("segment", ""),
			"stage": pending.get("stage", ""),
			"iteration": pending.get("iteration", 0),
			"reaction_round": pending.get("reaction_round", 0),
			"planning_cycle": pending.get("planning_cycle", 0),
		}
	return JSON.stringify({
		"battle_id": battle_id,
		"actor_id": actor_id,
		"type": command_type,
		"payload": payload,
	})
