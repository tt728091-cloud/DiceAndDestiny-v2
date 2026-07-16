class_name BattleViewState
extends RefCounted

var battle_id := ""
var status := ""
var battle_result := ""
var segment := ""
var stage := ""
var round_number := 0
var viewer_actor_id := "blade"
var actors: Dictionary = {}
var pending_input: Dictionary = {}
var origin: Dictionary = {}
var damage_sources: Array = []
var settled_damage: Dictionary = {}
var events: Array = []
var defense_rolls: Dictionary = {}
var defense_selections: Dictionary = {}
var effect_rolls: Array = []
var offensive_reveals: Dictionary = {}
var raw_snapshot: Dictionary = {}
var max_rolls_by_actor := {"blade": 3, "goblin": 3}

func apply_result(result: Dictionary) -> bool:
	if result.get("accepted") != true: return false
	var snapshot = result.get("snapshot")
	if not snapshot is Dictionary: return false
	var incoming_actors = snapshot.get("actors", {})
	if not incoming_actors is Dictionary: return false
	# The viewer-safe contract must never include an enemy hand or enemy card map.
	for actor_id in incoming_actors:
		if str(actor_id) == "blade": continue
		var actor = incoming_actors[actor_id]
		if actor is Dictionary and (not actor.get("hand", []).is_empty() or not actor.get("card_instances", {}).is_empty()):
			return false
	var incoming_round := int(snapshot.get("round", 0))
	var incoming_segment := str(snapshot.get("segment", ""))
	if incoming_round != round_number or incoming_segment in ["ongoing_effects", "income", "offensive"]:
		defense_rolls.clear(); defense_selections.clear()
	if incoming_round != round_number: offensive_reveals.clear()
	raw_snapshot = snapshot.duplicate(true)
	battle_id = str(snapshot.get("battle_id", ""))
	status = str(snapshot.get("status", ""))
	battle_result = str(result.get("battle_result", status if status in ["victory", "defeat", "draw", "escaped"] else ""))
	segment = str(snapshot.get("segment", ""))
	stage = str(snapshot.get("stage", snapshot.get("flow", {}).get("stage", "")))
	round_number = int(snapshot.get("round", 0))
	viewer_actor_id = str(snapshot.get("viewer_actor_id", "blade"))
	actors = incoming_actors.duplicate(true)
	pending_input = result.get("pending_input", {}).duplicate(true)
	origin = snapshot.get("origin", {}).duplicate(true)
	damage_sources = snapshot.get("damage_sources", []).duplicate(true)
	settled_damage = snapshot.get("settled_damage", {}).duplicate(true)
	effect_rolls = snapshot.get("effect_rolls", []).duplicate(true)
	events = result.get("events", []).duplicate(true)
	var incoming_defenses = snapshot.get("defense_selections", {})
	if incoming_defenses is Dictionary and not incoming_defenses.is_empty():
		defense_selections = incoming_defenses.duplicate(true)
		for actor_id in defense_selections:
			var selection: Dictionary = defense_selections[actor_id]
			var face := int(selection.get("rolled_face", 0))
			if face > 0:
				defense_rolls[str(actor_id)] = {"actor_id": str(actor_id), "ability_id": str(selection.get("ability_id", "basic_defense")), "source_id": str(selection.get("source_id", "")), "face": face}
	for battle_event in events:
		if battle_event.get("type") == "dice_rolled" and int(battle_event.get("max_rolls", 0)) > 0:
			max_rolls_by_actor[str(battle_event.get("actor_id", "blade"))] = int(battle_event.max_rolls)
		if battle_event.get("type") == "dice_rolled" and battle_event.get("segment") == "defensive" and battle_event.get("pool") == "defensive":
			var dice_value = battle_event.get("dice", [])
			var dice: Array = dice_value if dice_value is Array else []
			if not dice.is_empty():
				defense_rolls[str(battle_event.get("actor_id", ""))] = {"actor_id": str(battle_event.get("actor_id", "")), "ability_id": str(battle_event.get("source_id", "basic_defense")), "face": int(dice[0].get("face", 0)), "die": dice[0].duplicate(true)}
		if battle_event.get("type") == "defense_selected":
			var data_value = battle_event.get("data", {})
			var data: Dictionary = data_value if data_value is Dictionary else {}
			var actor_id := str(battle_event.get("actor_id", ""))
			var existing: Dictionary = defense_rolls.get(actor_id, {"actor_id": actor_id})
			existing["ability_id"] = str(data.get("ability_id", existing.get("ability_id", "")))
			existing["face"] = int(data.get("rolled_face", existing.get("face", 0)))
			existing["source_id"] = str(data.get("source_id", existing.get("source_id", "")))
			defense_rolls[actor_id] = existing
			defense_selections[actor_id] = existing.duplicate(true)
		if battle_event.get("type") == "interaction_revealed" and battle_event.get("segment") == "offensive":
			var reveal_data_value = battle_event.get("data", {})
			var reveal_data: Dictionary = reveal_data_value if reveal_data_value is Dictionary else {}
			var commitments_value = reveal_data.get("commitments", {})
			var commitments: Dictionary = commitments_value if commitments_value is Dictionary else {}
			for revealed_actor_id in commitments:
				var commitment_value = commitments[revealed_actor_id]
				if commitment_value is Dictionary: offensive_reveals[str(revealed_actor_id)] = commitment_value.duplicate(true)
	return not battle_id.is_empty() and viewer_actor_id == "blade"

func viewer_pending() -> Dictionary:
	var value = pending_input.get("blade", {})
	return value if value is Dictionary else {}

func allowed(command_type: String) -> bool:
	return command_type in viewer_pending().get("allowed_commands", [])

func actor(actor_id: String) -> Dictionary:
	var value = actors.get(actor_id, {})
	return value if value is Dictionary else {}

func hand_cards() -> Array:
	var result: Array = []
	var blade := actor("blade")
	var instances: Dictionary = blade.get("card_instances", {})
	for instance_id in blade.get("hand", []):
		var instance: Dictionary = instances.get(instance_id, {})
		result.append({"instance_id": str(instance_id), "definition_id": str(instance.get("definition_id", "unknown"))})
	return result

func rolled_dice(actor_id: String) -> Array:
	var history: Array = actor(actor_id).get("roll_history", [])
	if history.is_empty():
		var reveal: Dictionary = offensive_reveal(actor_id)
		var revealed_dice = reveal.get("dice", [])
		return revealed_dice if revealed_dice is Array else []
	var dice = history[-1].get("dice", [])
	return dice if dice is Array else []

func offensive_reveal(actor_id: String) -> Dictionary:
	var value = offensive_reveals.get(actor_id, {})
	if value is Dictionary and not value.is_empty(): return value
	var actor_state := actor(actor_id)
	var outcome = actor_state.get("offensive_outcome", {})
	if not outcome is Dictionary or outcome.is_empty(): return {}
	return {
		"ability_id": str(actor_state.get("selected_ability", "")),
		"tier_id": str(actor_state.get("selected_tier", "")),
		"targets": _array(actor_state.get("selected_targets", [])),
		"dice": rolled_dice_from_history(actor_state),
		"outcome": outcome.duplicate(true),
	}

func rolled_dice_from_history(actor_state: Dictionary) -> Array:
	var history: Array = _array(actor_state.get("roll_history", []))
	if history.is_empty(): return []
	return _array(history[-1].get("dice", []))

func _array(value) -> Array:
	return value if value is Array else []

func rolls_used(actor_id: String) -> int:
	return actor(actor_id).get("roll_history", []).size()

func max_rolls(actor_id: String) -> int:
	var used := rolls_used(actor_id)
	if actor_id == "blade" and used > 0 and not allowed("planning_reroll"): return used
	return int(max_rolls_by_actor.get(actor_id, 3))

func incoming_sources(target_id: String = "blade") -> Array:
	var result: Array = []
	for source in damage_sources:
		if source is Dictionary and str(source.get("target_actor_id", "")) == target_id:
			result.append(source)
	return result

func is_complete() -> bool:
	return not battle_result.is_empty() or status in ["victory", "defeat", "draw", "escaped", "complete"]

func is_scenario() -> bool:
	return origin.get("kind") == "scenario"
