extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for size in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		canvas.size = size
		for protect in [false, true]: await _scenario(protect)
	print("ADVENTURER DAMAGE PROGRESSION: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
# The recording played a since-retired prevent-3 card. Steady Guard prevents
# 1, so replay it as three Steady Guard plays against the same attack; that
# spends the starting energy and reproduces the reported threatened state.
const RECORDED_PREVENTION := 3
const STEADY_GUARD_PREVENTION := 1
func _replay_with_steady_guards(gateway, result: Dictionary, recorded: Dictionary) -> Dictionary:
	var sources: Array = recorded.get("payload", {}).get("commitment", {}).get("proposal_ids", [])
	if sources.size() != 1: return {"accepted": false, "error": "recorded card play has one source"}
	var source = _semantic(str(sources[0]))
	for play in RECORDED_PREVENTION / STEADY_GUARD_PREVENTION:
		# Leave the final enemy response to the recorded learned-policy step.
		while play > 0 and result.get("accepted", false) and result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model()
		if not result.get("accepted", false): return result
		var plays: Array = result.get("legal_actions", []).filter(func(action): return _steady_guard_play(result, action, source))
		if plays.is_empty(): return {"accepted": false, "error": "Steady Guard %d is playable against the recorded attack" % (play + 1)}
		var released := _released(result, source)
		result = gateway.submit(JSON.stringify(plays[0]))
		if not result.get("accepted", false): return result
		_expect(_released(result, source) == released + STEADY_GUARD_PREVENTION, "Steady Guard saves exactly one threatened card to discard")
	return result
func _steady_guard_play(result: Dictionary, action: Dictionary, source) -> bool:
	var commitment: Dictionary = action.get("payload", {}).get("commitment", {})
	var cards: Array = commitment.get("card_ids", [])
	if action.get("type") != "commit_interaction" or cards.size() != 1: return false
	if result.snapshot.actors.blade.card_instances.get(cards[0], {}).get("definition_id") != "steady_guard": return false
	var raw := str(commitment.get("choice_id", ""))
	var choice = JSON.parse_string(raw) if raw.begins_with("{") else null
	if not choice is Dictionary or choice.get("verb") != "start": return false
	return str(choice.get("source", "")).is_empty() or _semantic(str(choice.source)) == source
func _released(result: Dictionary, source) -> int:
	var count := 0
	for removal in result.get("snapshot", {}).get("settled_damage", {}).get("removals", []):
		if removal.get("released", false) and removal.get("released_destination") == "discard" and _semantic(removal.get("damage_proposal_ids", [])) == [source]: count += 1
	return count
func _scenario(protect: bool) -> void:
	# Replay the reported battle: Guarded Strike, Guard, prevent 3 (now three
	# Steady Guards), then enemy pass.
	# The old full-battle tests submitted commands directly and missed GUI occlusion.
	var fixture: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/adventurer_damage_progression.json"))
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result: Dictionary = gateway.start_battle(fixture.battle_id, int(fixture.seed))
	if not result.get("accepted", false): _expect(false, "native battle starts: " + str(result.get("error"))); return
	for step in fixture.trace:
		if step.controller == "human":
			var recorded: Dictionary = JSON.parse_string(JSON.stringify(step.command).replace('"seat-a"', '"blade"').replace('"seat-b"', '"goblin"'))
			var choices: Array = result.legal_actions.filter(func(action): return _semantic(action) == _semantic(recorded))
			if choices.is_empty() and not recorded.get("payload", {}).get("commitment", {}).get("card_ids", []).is_empty():
				result = _replay_with_steady_guards(gateway, result, recorded)
			else:
				if choices.size() != 1: _expect(false, "replay has one current legal match for " + str(recorded)); return
				result = gateway.submit(JSON.stringify(choices[0]))
		else:
			# Any-time starter cards can now answer the offensive reaction, a
			# decision the recording predates; decline it and let the model act.
			while not bool(result.get("learned_policy", {}).get("model_turn", true)) and result.get("accepted", false):
				var passes: Array = result.get("legal_actions", []).filter(func(action): return action.get("type") in ["pass", "planning_pass"])
				if passes.is_empty(): break
				result = gateway.submit(JSON.stringify(passes[0]))
			result = gateway.advance_model()
		if result.get("snapshot", {}).get("segment") == "defensive":
			_expect(_protect_count(result.snapshot.actors.blade) == 2, "Protect is public before defense")
		if not result.get("accepted", false): _expect(false, "replay command accepted: " + str(result.get("error"))); return
	# Protect is automatic now: the reported state offers only Pass, and Protect
	# saves the remaining damage on its own when it commits.
	_expect(result.snapshot.stage == "damage_reaction" and result.legal_actions.size() == 1 and result.legal_actions[0].type == "pass", "reported state offers only Pass; Protect is not played by hand")
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("adventurer-progress.json"))
	canvas.add_child(screen)
	await create_timer(3).timeout
	var strip = screen._actor_profiles.blade.statuses
	_expect(_protect_count(screen._view.actor("blade")) == 2 and strip.counts.get("protect") == 2, "authority and visible status count agree")
	_expect(strip.cells.has("protect") and strip.cells.protect.modulate.a == 1.0, "Protect icon is visible on character board")
	_expect("Positive status" in strip.cells.protect.tooltip_text and "next round" in strip.cells.protect.tooltip_text, "status hover explains polarity and expiration")
	_expect(BattlePresentationCatalog.definition("statuses", "protect").polarity == "positive", "catalog declares positive status")
	var hover := InputEventMouseMotion.new(); hover.position = strip.cells.protect.get_global_rect().get_center()
	canvas.push_input(hover, true); await process_frame
	_expect(canvas.gui_get_hovered_control() == strip.cells.protect, "pointer reaches Protect status hover")
	_expect(screen._action_footer.get_children().all(func(button): return not button is Button or "Protect" not in button.text), "no standalone protection button")
	var incoming: Array = screen._as_array(screen._view.settled_damage.get("sources", [])).filter(func(source): return str(source.get("target_actor_id", "")) == "blade")
	var source_id := str(incoming[0].get("id", "")) if not incoming.is_empty() else ""
	_expect(screen._attack_intents.has(source_id), "the incoming attack is on the board")
	if screen._attack_intents.has(source_id):
		await _click(screen._attack_intents[source_id].intent)
		for frame in 6: await process_frame
	_expect(screen._ability_dock.get_node_or_null("ProtectChoice") == null, "the attack offers no Protect choice")
	# Manual Pass, or automatic progression with only Pass left: either way
	# Protect acts as the damage lands.
	if protect: screen._auto_pass_disabled = false
	else:
		var pass_button: Button = screen._auto_pass_button
		var wait := Time.get_ticks_msec() + 8000
		while Time.get_ticks_msec() < wait and (not is_instance_valid(pass_button) or pass_button.disabled):
			await process_frame; pass_button = screen._auto_pass_button
		_expect(is_instance_valid(pass_button) and Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(pass_button.get_global_rect()), "Pass is enabled and fits viewport")
		if is_instance_valid(pass_button): await _click(pass_button)
	var deadline := Time.get_ticks_msec() + 15000
	while Time.get_ticks_msec() < deadline and (screen._view.round_number == 1 or screen._director.has_beats() or screen._held_defense_view != null or screen._card_gain_active()): await process_frame
	_expect(screen._view.round_number == 2 and screen._error_message.is_empty(), "Defense finishes and the next round starts")
	_expect(_protect_count(screen._view.actor("blade")) == 0 and int(screen._actor_profiles.blade.statuses.counts.get("protect", 0)) == 0, "spent Protect is removed from board")
	_expect(int(screen._view.actor("blade").current_health) == 12, "Protect saved the remaining damage on its own")
	print("PROGRESSION ", canvas.size, " ", "automatic Pass" if protect else "manual Pass", ": round ", screen._view.round_number)
	screen.queue_free(); await process_frame
func _click(button: Button) -> void:
	var viewport := button.get_viewport()
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center()
	viewport.push_input(motion, true); await process_frame
	_expect(viewport.gui_get_hovered_control() == button, "actual pointer hits " + button.text)
	var click := InputEventMouseButton.new(); click.position = motion.position; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true
	viewport.push_input(click, true)
	click = click.duplicate(); click.pressed = false; viewport.push_input(click, true)
	await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ADVENTURER PROGRESSION: " + message)

func _protect_count(actor: Dictionary) -> int:
	var count := 0
	for status in actor.get("statuses", []):
		if status.definition_id == "protect": count += int(status.stacks)
	return count
# Status acquisition adds authority sequence entries; match decisions by their
# meaning and submit the current legal command, preserving checkpoint safety.
func _semantic(value):
	if value is Dictionary:
		var result := {}
		for key in value:
			if key not in ["checkpoint", "pending_input_id"]: result[key] = _semantic(value[key])
		return result
	if value is Array: return value.map(_semantic)
	if value is String and value.begins_with("source-r"): return value.substr(0, value.rfind("-"))
	return value
