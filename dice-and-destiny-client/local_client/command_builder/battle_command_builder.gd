class_name BattleCommandBuilder
extends RefCounted

static func start_battle(battle_id: String, actor_id: String = "blade") -> String:
	return _json(battle_id, actor_id, "start_battle", {
		"player": {"instance_id": "blade", "definition_id": "blade_warden"},
		"enemies": [{"instance_id": "goblin", "definition_id": "venom_goblin"}],
	})

static func open_battle(battle_id: String, actor_id: String = "blade") -> String:
	return _json(battle_id, actor_id, "open_battle", {})

static func list_dev_snapshots(battle_id: String, actor_id: String = "blade") -> String:
	return _json(battle_id, actor_id, "list_dev_snapshots", {})

static func save_dev_snapshot(battle_id: String, actor_id: String, snapshot_name: String, overwrite: bool = false) -> String:
	return _json(battle_id, actor_id, "save_dev_snapshot", {"name": snapshot_name, "overwrite": overwrite})

static func load_dev_snapshot(battle_id: String, actor_id: String, snapshot_name: String) -> String:
	return _json(battle_id, actor_id, "load_dev_snapshot", {"name": snapshot_name})

static func list_dev_history(battle_id: String, actor_id: String = "blade") -> String:
	return _json(battle_id, actor_id, "list_dev_history", {})

static func mark_dev_history(battle_id: String, actor_id: String, label: String, kind: String, presented_sequence: int, client_state: Dictionary = {}, action: Dictionary = {}) -> String:
	return _json(battle_id, actor_id, "mark_dev_history", {"label": label, "kind": kind, "presented_sequence": presented_sequence, "client_state": client_state, "action": action})

static func jump_dev_history(battle_id: String, actor_id: String, point_id: String) -> String:
	return _json(battle_id, actor_id, "jump_dev_history", {"point_id": point_id})

static func commit_dev_history(battle_id: String, actor_id: String, mode: String) -> String:
	return _json(battle_id, actor_id, "commit_dev_history", {"mode": mode})

static func return_dev_history_latest(battle_id: String, actor_id: String) -> String:
	return _json(battle_id, actor_id, "return_dev_history_latest", {})

static func replay_dev_history_action(battle_id: String, actor_id: String, action: Dictionary) -> String:
	return _json(battle_id, actor_id, "replay_dev_history_action", {"action": action})

static func replace_dev_history_future(battle_id: String, actor_id: String) -> String:
	return _json(battle_id, actor_id, "replace_dev_history_future", {})

static func planning_roll(battle_id: String, actor_id: String, pending: Dictionary) -> String:
	return _planning(battle_id, actor_id, "planning_roll", pending)

static func planning_keep(battle_id: String, actor_id: String, pending: Dictionary, indices: Array) -> String:
	return _planning(battle_id, actor_id, "planning_keep", pending, {"kept_indices": _integer_indices(indices)})

static func planning_reroll(battle_id: String, actor_id: String, pending: Dictionary, indices: Array) -> String:
	return _planning(battle_id, actor_id, "planning_reroll", pending, {"reroll_indices": _integer_indices(indices)})

static func planning_commit_cards(
	battle_id: String, actor_id: String, pending: Dictionary, card_ids: Array,
	target_ids: Array = [], ability_id: String = "", status_id: String = "",
	die_index: int = -1
) -> String:
	var extra := {"card_ids": card_ids}
	if not target_ids.is_empty(): extra["target_ids"] = target_ids
	if not ability_id.is_empty(): extra["ability_id"] = ability_id
	if not status_id.is_empty(): extra["status_id"] = status_id
	if die_index >= 0: extra["die_index"] = die_index
	return _planning(battle_id, actor_id, "planning_commit_cards", pending, extra)

static func planning_select_ability(battle_id: String, actor_id: String, pending: Dictionary, ability_id: String, target_ids: Array) -> String:
	return _planning(battle_id, actor_id, "planning_select_ability", pending, {"ability_id": ability_id, "target_ids": target_ids})

static func planning_select_targets(battle_id: String, actor_id: String, pending: Dictionary, target_ids: Array) -> String:
	return _planning(battle_id, actor_id, "planning_select_targets", pending, {"target_ids": target_ids})

static func planning_pass(battle_id: String, actor_id: String, pending: Dictionary) -> String:
	return _planning(battle_id, actor_id, "planning_pass", pending)

static func roll_dice(battle_id: String, actor_id: String, pending: Dictionary, reroll_indices: Array = []) -> String:
	var payload := {"pending_input_id": str(pending.get("id", "")), "request_id": str(pending.get("source_id", ""))}
	if not reroll_indices.is_empty(): payload["reroll_indices"] = _integer_indices(reroll_indices)
	return _json(battle_id, actor_id, "roll_dice", payload)

static func commit_interaction(
	battle_id: String, actor_id: String, pending: Dictionary, card_ids: Array = [],
	proposal_ids: Array = [], target_ids: Array = [], choice_id: String = "",
	planning_adjustments: Array = [], damage_reactions: Array = [], value = null
) -> String:
	var commitment := {}
	if not card_ids.is_empty(): commitment["card_ids"] = card_ids
	if not proposal_ids.is_empty(): commitment["proposal_ids"] = proposal_ids
	if not target_ids.is_empty(): commitment["target_ids"] = target_ids
	if not choice_id.is_empty(): commitment["choice_id"] = choice_id
	if not planning_adjustments.is_empty(): commitment["planning_adjustments"] = planning_adjustments
	if not damage_reactions.is_empty(): commitment["damage_reactions"] = damage_reactions
	if value != null: commitment["value"] = value
	return _json(battle_id, actor_id, "commit_interaction", {
		"pending_input_id": str(pending.get("id", "")),
		"checkpoint": _interaction_checkpoint(pending),
		"commitment": commitment,
	})

static func pass_command(battle_id: String, actor_id: String, pending: Dictionary) -> String:
	return _json(battle_id, actor_id, "pass", {
		"pending_input_id": str(pending.get("id", "")),
		"checkpoint": _interaction_checkpoint(pending),
	})

static func from_pending(battle_id: String, actor_id: String, command_type: String, pending: Dictionary) -> String:
	match command_type:
		"planning_roll": return planning_roll(battle_id, actor_id, pending)
		"planning_keep": return planning_keep(battle_id, actor_id, pending, [])
		"planning_reroll": return planning_reroll(battle_id, actor_id, pending, [])
		"planning_pass": return planning_pass(battle_id, actor_id, pending)
		"roll_dice": return roll_dice(battle_id, actor_id, pending)
		"pass": return pass_command(battle_id, actor_id, pending)
	return ""

static func _planning(battle_id: String, actor_id: String, kind: String, pending: Dictionary, extra: Dictionary = {}) -> String:
	var payload := {"pending_input_id": str(pending.get("id", "")), "checkpoint": _planning_checkpoint(pending)}
	for key in extra: payload[key] = extra[key]
	return _json(battle_id, actor_id, kind, payload)

static func _planning_checkpoint(pending: Dictionary) -> Dictionary:
	return {"window_id": str(pending.get("window_id", "")), "segment": str(pending.get("segment", "")), "stage": str(pending.get("stage", "")), "iteration": int(pending.get("iteration", 0)), "planning_cycle": int(pending.get("planning_cycle", 0))}

static func _interaction_checkpoint(pending: Dictionary) -> Dictionary:
	var checkpoint := {"window_id": str(pending.get("window_id", "")), "stage": str(pending.get("stage", "")), "iteration": int(pending.get("iteration", 0)), "reaction_round": int(pending.get("reaction_round", 0))}
	if int(pending.get("planning_cycle", 0)) != 0: checkpoint["planning_cycle"] = int(pending.get("planning_cycle", 0))
	return checkpoint

static func _integer_indices(values: Array) -> Array[int]:
	var result: Array[int] = []
	for value in values: result.append(int(value))
	return result

static func _json(battle_id: String, actor_id: String, kind: String, payload: Dictionary) -> String:
	return JSON.stringify({"battle_id": battle_id, "actor_id": actor_id, "type": kind, "payload": payload})
