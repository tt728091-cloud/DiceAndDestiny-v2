extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var result: Dictionary = gateway.start_battle("native-flow-selection", 1789679833957118)
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("native-flow-selection.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	var inspect_frame := _check_combat_width.bind(screen)
	RenderingServer.frame_pre_draw.connect(inspect_frame)
	await process_frame
	var chosen := {}
	for attempt in 4:
		for action in screen._view.legal_actions:
			if action.get("type") == "planning_select_ability": chosen = action; break
		if not chosen.is_empty(): break
		var roll := {}
		for action in screen._view.legal_actions:
			if action.get("type") in ["planning_roll", "planning_reroll"]: roll = action; break
		if roll.is_empty(): break
		screen._send(JSON.stringify(roll))
		while screen._player_roll_active(): await process_frame
		await process_frame
	_expect(not chosen.is_empty(), "native rolls qualify an attack")
	if not chosen.is_empty():
		for frame in 5: await process_frame
		var selected_key := "ability:blade:" + str(chosen.payload.ability_id)
		var original_rect := Rect2()
		for tile in screen._ability_dock.find_children("*", "Control", true, false):
			if tile.get_meta("flow_key", "") == selected_key: original_rect = tile.get_global_rect()
		screen._send(JSON.stringify(chosen))
		_expect(screen._flow_transition.ghosts.has(selected_key), "selected tile retained during outcome change")
		if screen._flow_transition.ghosts.has(selected_key):
			_expect(screen._flow_transition.ghosts[selected_key].rect == original_rect, "transition captures the settled tile position, not an unlaid-out rebuild")
		_expect(screen._error_message.is_empty(), "authority accepts attack")
		_expect(not screen._selection_morph.is_empty() and screen._selection_morph.get("text", "") not in ["Selected", "No offensive effect pending", ""], "native selected outcome is visible")
		var sampled := 0
		var captured := false
		var sampling_started := Time.get_ticks_msec()
		while not screen._selection_morph.is_empty() or Time.get_ticks_msec() < screen._flow_until:
			_check_steady(screen); sampled += 1
			if not captured and Time.get_ticks_msec() - sampling_started > 220:
				captured = true
				var screenshot := OS.get_environment("DICE_AND_DESTINY_SELECTION_SCREENSHOT")
				if not screenshot.is_empty() and DisplayServer.get_name() != "headless":
					await RenderingServer.frame_post_draw
					root.get_texture().get_image().save_png(screenshot)
			await process_frame
		_expect(sampled > 10, "selection and its follow-up handoff sampled frame by frame")
		screen.learned_battle_mode = true
		screen._director.configure_learned_battle(true)
		screen._auto_pass_disabled = false
		var deadline := Time.get_ticks_msec() + 60000
		var saw_defense := false; var saw_damage := false; var saw_commit := false; var saw_effects := false
		while Time.get_ticks_msec() < deadline:
			_check_steady(screen)
			var stage: String = screen._view.stage
			var beat: String = str(screen._director.peek().get("type", ""))
			saw_defense = saw_defense or not screen._defense_result_panels.is_empty()
			saw_damage = saw_damage or stage == "damage_reaction"
			saw_commit = saw_commit or beat == "combat_damage"
			saw_effects = saw_effects or beat == "effects_resolved"
			if screen._view.round_number > 1 and stage == "planning" and not screen._director.has_beats(): break
			if screen._model_error or not screen._error_message.is_empty(): break
			if not screen._submitting and not screen._model_thinking and not screen._director.has_beats() and screen._selection_morph.is_empty() and Time.get_ticks_msec() >= screen._interaction_deadline(true):
				var action := {}
				if stage == "defense_selection":
					for legal in screen._view.legal_actions:
						if legal.get("type") == "planning_select_ability": action = legal; break
					if not action.is_empty():
						var target: String = action.payload.target_ids[0]
						for panel in screen._combat_columns.blade.get_children():
							if panel.data.source_id == target: panel.source_selected.emit(target); break
				elif stage == "planning":
					for legal in screen._view.legal_actions:
						if legal.get("type") == "planning_select_ability": action = legal; break
				if action.is_empty():
					for legal in screen._view.legal_actions:
						if legal.get("type") in ["pass", "planning_pass"]: action = legal; break
				if not action.is_empty(): screen._send(JSON.stringify(action))
			await process_frame
		_expect(screen._error_message.is_empty() and not screen._model_error, "native model and authority complete legal reactions")
		_expect(saw_defense and saw_damage and saw_commit and saw_effects, "real round presents defense, damage, removal and Effects")
		_expect(screen._view.round_number > 1 and screen._view.stage == "planning" and not screen._director.has_beats(), "native battle reaches next planning round")
	RenderingServer.frame_pre_draw.disconnect(inspect_frame)
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("NATIVE COMBAT SELECTION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)

func _check_steady(screen: Control) -> void:
	for node in screen._root.find_children("*", "Control", true, false):
		var key := str(node.get_meta("flow_key", ""))
		if key in ["HandDock", "PlayerDice", "EnemyDice", "RollControls", "BattleActionFooter"]:
			_expect(node.modulate.a == 1.0, "%s stays opaque during every transition frame" % key)
		if key.begins_with("ability:") and screen._flow_transition.ghosts.has(key):
			var previous: Control = screen._flow_transition.ghosts[key].node
			var old_alpha := previous.modulate.a if is_instance_valid(previous) and previous.is_visible_in_tree() else 0.0
			_expect(node.modulate.a + old_alpha >= 0.999, "chosen ability has no transparent gap")

func _check_combat_width(screen: Control) -> void:
	if not is_instance_valid(screen._root): return
	for column in screen._combat_columns.values():
		for panel in column.get_children():
			if panel.is_visible_in_tree() and panel.modulate.a > 0.9:
				_expect(panel.get_global_rect().size.x > 300.0, "visible combat panel retains lane width before draw: stage=%s size=%s" % [screen._view.stage, panel.get_global_rect().size])
