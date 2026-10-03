extends SceneTree
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	for definition in ["brace", "brace_plus", "adventurer_guard", "adventurer_guard_plus"]:
		var tested := false
		for seed in range(1, 41):
			var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
			gateway.unified_defense = true
			var result: Dictionary = gateway.start_battle("prevention-%s-%d" % [definition, seed], seed)
			for step in 80:
				if not result.get("accepted", false): break
				if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
				if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
				else:
					var next := _next(result.get("legal_actions", []))
					if next.is_empty(): break
					result = gateway.submit(JSON.stringify(next))
			var action := _variant(result, definition)
			if action.is_empty(): continue
			_expect(not _variant(result, "adventurer_guard").is_empty() and not _variant(result, "adventurer_guard_plus").is_empty(), "both defenses selectable in the native hub")
			var before: Dictionary = result.snapshot.actors.blade.duplicate(true)
			result = gateway.submit(JSON.stringify(action))
			while result.get("accepted", false) and result.snapshot.stage in ["defense_roll", "defense_reaction"]:
				result = gateway.submit(JSON.stringify(_next(result.legal_actions)))
			_expect(result.get("accepted", false), definition + " accepted by native authority")
			var saved := 0
			for removal in result.snapshot.get("settled_damage", {}).get("removals", []):
				if removal.get("target_actor_id") != "blade" or not removal.get("released", false): continue
				saved += 1
				var expected: String = "discard" if not definition.ends_with("_plus") else str(removal.original_zone)
				if removal.get("card_id") in action.get("payload", {}).get("commitment", {}).get("card_ids", []): expected = "discard"
				_expect(removal.get("released_destination") == expected, definition + " publishes configured saved-card destination")
			if saved == 0: continue # Three Coins can prevent zero damage.
			_expect(result.snapshot.actors.blade.current_health == before.current_health, definition + " preserves health")
			tested = true
			break
		_expect(tested, definition + " exercised native prevention")
	print("PREVENTION VARIANTS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _variant(result: Dictionary, definition: String) -> Dictionary:
	for action in result.get("legal_actions", []):
		var payload: Dictionary = action.get("payload", {})
		if action.type == "planning_select_ability" and payload.get("ability_id") == definition: return action
		for id in payload.get("commitment", {}).get("card_ids", []):
			if result.snapshot.actors.blade.card_instances.get(id, {}).get("definition_id") == definition: return action
	return {}
func _next(actions: Array) -> Dictionary:
	for kind in ["roll_dice", "planning_pass", "pass", "planning_roll", "planning_select_ability"]:
		for action in actions:
			if action.type == kind: return action
	return {}
func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error(message)
