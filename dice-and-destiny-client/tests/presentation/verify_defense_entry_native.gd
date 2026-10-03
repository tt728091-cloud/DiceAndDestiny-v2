extends SceneTree
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	for character in ["curse", "venom", "blade_warden"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", character)
		var result: Dictionary = gateway.start_battle("native-defense-entry-" + character, 43)
		var reached := false
		for step in 100:
			if result.get("accepted") != true: _expect(false, str(result.get("error"))); break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			if result.snapshot.get("stage") == "offensive_reaction" and not result.get("defense_previews", []).is_empty(): reached = true; break
			var action := _choose(result.get("legal_actions", []))
			if action.is_empty(): break
			result = gateway.submit(JSON.stringify(action))
		_expect(reached, character + " reaches revealed incoming attacks")
		if not reached: continue
		var option: Dictionary = result.defense_previews[0]
		_expect(option.target_actor_id == "blade" and str(option.source_actor_id).begins_with("goblin"), "preview aliases are viewer safe")
		var committed := false
		for step in 100:
			if result.get("accepted") != true: _expect(false, str(result.get("error"))); break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			if result.snapshot.get("stage") == "defense_selection":
				for action in result.get("legal_actions", []):
					if action.type != "planning_select_ability" or action.payload.ability_id != option.ability_id or bool(action.payload.get("spend_catalyst", false)) != bool(option.get("spend_catalyst", false)): continue
					for source in result.snapshot.get("damage_sources", []):
						if source.id in action.payload.target_ids and source.source_actor_id == option.source_actor_id and source.source_content_id == option.source_content_id:
							result = gateway.submit(JSON.stringify(action)); committed = result.get("accepted") == true; break
					if committed: break
				break
			var action := _choose(result.get("legal_actions", []), true)
			if action.is_empty(): break
			result = gateway.submit(JSON.stringify(action))
		_expect(committed, character + " preview resolves to accepted fresh defense against exact attacker")
	print("NATIVE DEFENSE ENTRY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _choose(actions: Array, passing: bool = false) -> Dictionary:
	for type in (["pass", "planning_pass"] if passing else ["planning_roll", "planning_select_ability", "planning_pass", "pass", "roll_dice"]):
		for action in actions:
			if action.type == type: return action
	return {}
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("NATIVE DEFENSE ENTRY: " + message)
