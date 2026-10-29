extends SceneTree
## Protect by pointer against two Brine Masks (real native authority, the
## simultaneous flow of new battles): Guarded Strike grants 2 visible Protect;
## Defense offers no Protect control; when the attacks land Protect prevents 2
## of the incoming damage on its own and is spent.
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	await _scenario()
	print("PROTECT AUTOMATIC: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _human_turn(gateway, result: Dictionary) -> Dictionary:
	for step in 200:
		if result.get("accepted") != true or result.get("battle_result", "") != "": return result
		if not result.get("learned_policy", {}).get("model_turn", false): return result
		result = gateway.advance_model()
	return result

func _incoming(snapshot: Dictionary) -> int:
	var total := 0
	for source in snapshot.get("damage_sources", []):
		if str(source.target_actor_id) == "blade": total += int(source.final_amount)
	return total

## Plays Guarded Strike, then reaches Defense with damage incoming.
func _defense_with_protect(runtime) -> Array:
	for seed_value in range(1, 120):
		var gateway = GATEWAY.new(runtime, "seat-a", "brine-mask-pair", "adventurer")
		gateway.unified_defense = true; gateway.simultaneous_play = true
		var result: Dictionary = _human_turn(gateway, gateway.start_battle("protect-automatic-%d" % seed_value, seed_value))
		var rolls: Array = result.legal_actions.filter(func(action): return action.type == "planning_roll")
		if rolls.is_empty(): continue
		result = _human_turn(gateway, gateway.submit(JSON.stringify(rolls[0])))
		var strike: Array = result.get("legal_actions", []).filter(func(action): return action.type == "planning_select_ability" and action.payload.get("ability_id") == "guarded_strike")
		if strike.is_empty(): continue
		result = _human_turn(gateway, gateway.submit(JSON.stringify(strike[0])))
		for step in 20:
			if result.get("accepted") != true or result.snapshot.get("simultaneous_step", "") == "defense_before": break
			var passes: Array = result.get("legal_actions", []).filter(func(action): return action.type == "planning_pass")
			if passes.is_empty(): break
			result = _human_turn(gateway, gateway.submit(JSON.stringify(passes[0])))
		if result.get("accepted") == true and result.snapshot.get("simultaneous_step", "") == "defense_before" and _incoming(result.snapshot) > 0: return [gateway, result, seed_value]
	return []

func _scenario() -> void:
	var found := _defense_with_protect(root.get_node("LearnedBattleRuntime"))
	_expect(not found.is_empty(), "a seed plays Guarded Strike and reaches Defense with damage incoming")
	if found.is_empty(): return
	var gateway = found[0]; var result: Dictionary = found[1]
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.viewer_actor_id = "blade"; screen.gateway = gateway
	screen.learned_battle_mode = true; screen.learned_human_seat = "seat-a"; screen.learned_seed = int(found[2])
	screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("protect-automatic.json"))
	root.add_child(screen)
	for frame in 12: await process_frame
	_expect(int(screen._actor_profiles.blade.statuses.counts.get("protect", 0)) == 2, "Guarded Strike's 2 Protect are on the status bar")
	_expect(not screen._view.legal_actions.any(func(a): return str(a.get("payload", {}).get("commitment", {}).get("choice_id", "")) == "spend_round_prevention"), "Protect is never played by hand")
	var sources: Array = screen._view.damage_sources.filter(func(source): return str(source.target_actor_id) == "blade" and int(source.final_amount) > 0)
	screen._select_attack_intent(str(sources[0].id))
	for frame in 4: await process_frame
	_expect(screen._ability_dock.get_node_or_null("ProtectChoice") == null, "an incoming attack offers no Protect choice")
	var incoming := _incoming(screen._view.raw_snapshot)
	var removed_before := int(screen._view.actor("blade").get("removed_count", 0))
	# Take the attacks undefended: Pass now and through After-defense.
	screen._auto_pass_disabled = false
	var deadline := Time.get_ticks_msec() + 30000
	while Time.get_ticks_msec() < deadline and screen._view.round_number == 1 and not screen._view.is_complete():
		if screen._sole_pass_action().is_empty() and not screen._submitting and not screen._model_thinking and screen._view.legal_actions.any(func(a): return a.type == "planning_pass" and str(a.get("payload", {}).get("source_id", "")).is_empty()):
			for action in screen._view.legal_actions:
				if action.type == "planning_pass" and str(action.get("payload", {}).get("source_id", "")).is_empty():
					screen._send(JSON.stringify(action)); break
		await process_frame
	_expect(screen._view.round_number > 1 or screen._view.is_complete(), "Defense finishes")
	var lost := int(screen._view.actor("blade").get("removed_count", 0)) - removed_before
	_expect(lost == maxi(0, incoming - 2), "Protect prevented 2 of %d incoming damage on its own: %d cards lost" % [incoming, lost])
	_expect(int(screen._actor_profiles.blade.statuses.counts.get("protect", 0)) == 0, "Protect is spent")
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or opponent error")
	screen.queue_free(); await process_frame

func _expect(ok: bool, message: String) -> void:
	if not ok:
		push_error("PROTECT AUTOMATIC: " + message)
		failed = true
