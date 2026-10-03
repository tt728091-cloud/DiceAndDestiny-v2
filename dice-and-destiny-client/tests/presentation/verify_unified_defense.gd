extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for mode in ["brine-mask", "brine-mask-pair"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", mode, "adventurer")
		gateway.unified_defense = true
		var result: Dictionary = gateway.start_battle("unified-" + mode, 43)
		for step in 100:
			if not result.get("accepted", false): _expect(false, "native command: " + str(result.get("error"))); break
			if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
			if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
			else:
				var action := _choose(result.get("legal_actions", []))
				if action.is_empty(): _expect(false, "progression action missing"); break
				result = gateway.submit(JSON.stringify(action))
		_expect(result.snapshot.get("unified_defense", false), "new rule is public and pinned")
		_expect(result.snapshot.stage == "defense_selection" and not result.snapshot.get("settled_damage", {}).is_empty(), "cards revealed before first defense")
		for width in [1280,1920]:
			canvas.size = Vector2i(width,width*9/16)
			await _inspect_hub(result, mode)
		# Play Brace directly from the combined hub, then defend another source.
		var choices: Array = result.get("legal_actions", []).filter(func(a): return not a.get("payload", {}).get("commitment", {}).get("card_ids", []).is_empty())
		if not choices.is_empty():
			result = gateway.submit(JSON.stringify(choices[0]))
			_expect(result.get("accepted", false) and result.snapshot.stage == "defense_selection", "card can be played before defensive roll without leaving hub")
		var defense: Array = result.get("legal_actions", []).filter(func(a): return a.type == "planning_select_ability")
		if not defense.is_empty():
			result = gateway.submit(JSON.stringify(defense[0]))
			if result.snapshot.stage == "defense_roll": result = gateway.submit(JSON.stringify(_choose(result.legal_actions)))
			_expect(result.get("accepted", false), "native defense roll accepted")
			if result.snapshot.stage == "defense_reaction":
				var apply: Array = result.get("legal_actions", []).filter(func(a): return a.type == "planning_pass")
				_expect(apply.size() == 1, "one apply roll action")
				if not apply.is_empty(): result = gateway.submit(JSON.stringify(apply[0]))
			_expect(result.get("accepted", false) and result.snapshot.stage == "defense_selection", "defense returns to same combined hub: " + JSON.stringify(result) if not result.get("accepted", false) else "defense returns to same combined hub")
		if not result.get("accepted", false): quit(1); return
		var pass_actions: Array = result.get("legal_actions", []).filter(func(a): return a.type == "planning_pass")
		if not pass_actions.is_empty(): result = gateway.submit(JSON.stringify(pass_actions[0]))
		for step in 80:
			_expect(result.snapshot.segment != "damage_resolution", "no separate damage segment")
			if int(result.snapshot.round) > 1 or result.snapshot.get("status", "") in ["victory","defeat","completed"]: break
			if not result.learned_policy.get("model_turn", false): break
			result = gateway.advance_model()
			if not result.get("accepted", false): _expect(false, "model progresses after Pass: " + str(result.get("error"))); break
		_expect(int(result.snapshot.round) > 1 or result.snapshot.get("status", "") in ["victory","defeat","completed"], "Pass finishes all remaining defense choices")
		result = gateway.start_battle("unified-live-" + mode, 43)
		for step in 100:
			if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
			if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
			else: result = gateway.submit(JSON.stringify(_choose(result.get("legal_actions", []))))
		await _live_pass(gateway, result)
	print("UNIFIED DEFENSE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _choose(actions: Array) -> Dictionary:
	for kind in ["planning_roll", "planning_reroll", "roll_dice", "planning_select_ability", "planning_select_targets", "planning_pass", "pass"]:
		for action in actions:
			if action.type == kind: return action
	return {}
func _inspect_hub(result: Dictionary, mode: String) -> void:
	var fixture := result.duplicate(true); fixture.events = []; fixture.learned_policy = {}
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("unified-ui.json")); screen._auto_pass_disabled = true
	canvas.add_child(screen); screen.set_process(false)
	for frame in 12: await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	for grid in screen._damage_grids: grid.started_ms = Time.get_ticks_msec()-10000
	screen._process(0)
	for frame in 12: await process_frame
	_expect(screen._unified_defense(), "screen recognizes unified mode")
	_expect(screen._damage_grids.size() == fixture.snapshot.settled_damage.sources.size(), "one card list per incoming source on both sides")
	_expect(screen._attack_intents.size() == fixture.snapshot.settled_damage.sources.size(), "no duplicate attack intents")
	var seen := {}
	for grid in screen._damage_grids:
		for card in grid.get_children():
			_expect(not seen.has(card.instance_id), "no duplicated threatened card")
			seen[card.instance_id] = true
			_expect(not str(card.get_meta("removal_origin_zone", "")).is_empty(), "origin icon retained")
			_expect(Rect2(Vector2.ZERO,Vector2(canvas.size)).intersects(grid.get_global_rect()), "threat list stays visible")
	for source in fixture.snapshot.settled_damage.sources:
		var intent: Control = screen._attack_intents.get(str(source.id))
		_expect(intent != null, "attack stays attached to source")
		if intent != null: _expect(not intent.intent.tooltip_text.is_empty(), "attack rules hover retained")
	var cards: Array = fixture.legal_actions.filter(func(a): return not a.get("payload", {}).get("commitment", {}).get("card_ids", []).is_empty())
	if not cards.is_empty():
		var id: String = str(cards[0].payload.commitment.card_ids[0])
		var entry: Dictionary = fixture.snapshot.actors.blade.card_instances[id]
		_expect(screen._card_legal(str(entry.definition_id)), "prevention playable in Defense hub")
	var defenses: Array = fixture.legal_actions.filter(func(a): return a.type == "planning_select_ability")
	for ability_id in ["adventurer_guard", "adventurer_guard_plus"]:
		_expect(defenses.any(func(a): return a.payload.ability_id == ability_id), "both Guard versions available")
		var definition: Dictionary = screen._view.content_definition("abilities", ability_id)
		_expect(definition.get("saved_card_destination") == ("original" if ability_id.ends_with("_plus") else "discard"), "native pinned ability carries destination")
	if not defenses.is_empty():
		screen._selected_source = str(defenses[0].payload.target_ids[0])
		screen._render()
		for frame in 12: await process_frame
		for ability_id in ["adventurer_guard", "adventurer_guard_plus"]:
			var found := false
			for button in screen._root.find_children("*", "Button", true, false):
				var inspection := str(button.get_meta("inspection_id", ""))
				if ability_id + "." in inspection or inspection.ends_with("." + ability_id):
					found = true
					_expect(button.is_visible_in_tree(), "defense choice visible")
					_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(button.get_global_rect()), "defense choice inside viewport")
					_expect("Saved cards" in button.tooltip_text, "defense hover explains destination")
			_expect(found, ability_id + " has a rendered choice")
	var all_actions: Array = screen._view.legal_actions
	var passes: Array = all_actions.filter(func(action): return action.type == "planning_pass")
	screen._auto_pass_disabled = false
	if passes.size() == 1:
		screen._view.legal_actions = passes
		_expect(screen._sole_pass_action() == passes[0], "only Pass remaining enables automatic completion")
		screen._view.legal_actions = all_actions
	screen._auto_pass_disabled = true
	var directory := OS.get_environment("DICE_AND_DESTINY_UNIFIED_SCREENSHOTS")
	if not directory.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(false)
		canvas.get_texture().get_image().save_png(directory.path_join("%s-%d.png" % [mode,canvas.size.x]))
	screen.queue_free(); await process_frame
func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error(message)

func _live_pass(gateway, result: Dictionary) -> void:
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("unified-live.json"))
	canvas.add_child(screen)
	var deadline := Time.get_ticks_msec() + 10000
	while Time.get_ticks_msec() < deadline and (not is_instance_valid(screen._auto_pass_button) or screen._auto_pass_button.disabled): await process_frame
	var button: Button = screen._auto_pass_button
	_expect(is_instance_valid(button) and not button.disabled, "real Pass button is reachable")
	if is_instance_valid(button) and not button.disabled:
		var point := button.get_global_rect().get_center()
		var move := InputEventMouseMotion.new(); move.position = point; canvas.push_input(move, true); await process_frame
		for pressed in [true,false]:
			var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; canvas.push_input(click,true); await process_frame
		deadline = Time.get_ticks_msec() + 20000
		while Time.get_ticks_msec() < deadline and not screen._view.is_complete() and (screen._view.round_number == 1 or screen._director.has_beats()):
			_expect(screen._view.segment != "damage_resolution", "live screen never enters separate Damage")
			await process_frame
		_expect(screen._view.round_number > 1 or screen._view.is_complete(), "pointer Pass progresses while unused defenses remain")
		_expect(screen._error_message.is_empty() and not screen._model_error, "live handoff has no authority or AI error")
	screen.queue_free(); await process_frame
