extends SceneTree

const MENU := preload("res://app/boot/battle_bootstrap.gd")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	for key in ["combat_transition_seconds", "damage_reveal_seconds", "damage_hold_seconds", "damage_removal_seconds", "effects_gather_seconds", "defense_roll_seconds", "defense_effects_seconds", "defense_hold_seconds"]:
		ProjectSettings.set_setting("dice_and_destiny/presentation/" + key, 0.1)
	var menu := MENU.new(); root.add_child(menu)
	await process_frame
	var found := -1
	for i in menu._model_choice.item_count:
		if menu._model_choice.get_item_metadata(i) == "brine-mask-pair": found = i
	_expect(found >= 0, "pair is selectable in menu")
	menu._model_choice.select(found)
	menu._start_selected()
	await process_frame; await process_frame
	var screen: Control
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == "res://app/screens/battle/battle_screen.gd": screen = child
	_expect(screen != null, "Start Battle opens the actual battle screen" + (": " + menu._message.text if is_instance_valid(menu) else ""))
	if screen == null: quit(1); return
	screen.queue_free(); await process_frame
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	var initial: Dictionary = gateway.start_battle("pair-original-screen", 3, true)
	screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = gateway
	screen.learned_battle_mode = true; screen.learned_seed = 3
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("pair-ui.json"))
	root.add_child(screen); await process_frame; await process_frame
	_expect(screen._view.actors.size() == 3, "three independent combatants")
	_expect(screen._enemy_buttons.size() == 2 and screen._actor_profiles.size() == 2, "two portraits share original enemy profile")
	var scenery: Control = screen._root.get_node("BattleScenery")
	var first := scenery.get_node("Fighter_enemy") as TextureRect
	var second := scenery.get_node("Fighter_enemy_2") as TextureRect
	_expect(first.texture == second.texture and first.texture.resource_path.ends_with("drowned_oracle/brine_mask.png"), "both requested mask sprites")
	_expect(absf(first.position.x - second.position.x) < first.size.x and first.position != second.position, "enemies overlap in original battlefield")
	screen._enemy_buttons["goblin-2"].pressed.emit(); await process_frame
	_expect(screen._focused_enemy == "goblin-2", "portrait selects second enemy")
	await _capture("brine-pair-start.png")
	var deadline := Time.get_ticks_msec() + 180000
	var saw_defense := false
	var saw_damage := false
	var defended := {}
	var saw_pair := false
	var saw_two_defenses := false
	var saw_simultaneous_dice := false
	while Time.get_ticks_msec() < deadline and not screen._view.is_complete():
		if screen._model_error or not screen._error_message.is_empty(): break
		if screen._view.stage == "defense_reaction":
			for plan in screen._view.raw_snapshot.get("defense_plans", {}).values():
				if plan.get("actor_id") == "blade" and int(plan.get("rolled_face", 0)) > 0:
					saw_simultaneous_dice = true
					_expect(screen._defense_result_panels.any(func(panel): return panel.data.source_id == plan.source_id and panel.dice_controls.size() == 2), "queued native defense dice are visible alongside the first roll")
		for panel in screen._defense_result_panels:
			if not saw_defense and panel.data.get("actor_id") in ["goblin", "goblin-2"] and panel.data.get("ability_name") == "Salt Veil" and not panel.data.get("awaiting_roll", false) and not panel.data.dice.is_empty():
				_expect(panel.data.dice.size() == 1 and panel.dice_controls.size() == 1, "Salt Veil displays one defense die")
				var face := int(panel.data.dice[0].face)
				_expect(face >= 1 and face <= 6, "defense die has a valid result")
				_expect(int(panel.data.prevented) == ceili(face / 2.0), "defense displays half the roll rounded up: " + JSON.stringify(panel.data))
				if not saw_defense:
					saw_defense = true
					screen._auto_pass_disabled = true
					await create_timer(0.6).timeout
					await _capture("brine-pair-defense.png")
					screen._auto_pass_disabled = false
		saw_damage = saw_damage or screen._view.stage == "damage_reaction"
		if not screen._submitting and not screen._model_thinking and not screen._director.has_beats() and not screen._player_roll_active() and screen._selection_morph.is_empty() and Time.get_ticks_msec() >= screen._interaction_deadline(true) and not bool(screen._view.learned_policy.get("model_turn", false)):
			var action := _choose(screen._view.legal_actions, screen)
			if screen._view.stage == "defense_selection":
				var incoming: Array = screen._view.damage_sources.filter(func(source): return source.get("target_actor_id") == "blade")
				if incoming.size() == 2: saw_pair = true
				if action.get("type") == "planning_select_ability":
					var round_id := str(screen._view.round_number)
					if not defended.has(round_id): defended[round_id] = []
					var source_id := str(action.payload.target_ids[0])
					_expect(source_id not in defended[round_id], "one defense per incoming attack")
					_expect(screen._combat_columns.blade.get_children().any(func(panel): return panel.data.source_id == source_id), "defense targets one of the simultaneously visible attacks")
					defended[round_id].append(source_id)
					if defended[round_id].size() == 2: saw_two_defenses = true
					await _capture("brine-pair-incoming.png")
			if not action.is_empty(): screen._send(JSON.stringify(action))
		await process_frame
	_expect(screen._error_message.is_empty() and not screen._model_error, "full screen battle has no authority or controller errors: " + screen._error_message)
	_expect(screen._view.is_complete(), "visible battle reaches victory, defeat, or draw")
	_expect(saw_defense and saw_damage and saw_pair and saw_two_defenses, "two attacks, two separate defenses, visible dice and damage flow exercised: %s" % [defended])
	_expect(saw_simultaneous_dice, "full encounter exercises simultaneously rolled player defenses")
	await _capture("brine-pair-complete.png")
	var telemetry: Dictionary = screen.gateway.telemetry().get("result", {})
	_expect(int(telemetry.get("lifetime", {}).get("model_load_count", -1)) == 0, "minion needs no trained model")
	screen._play_again()
	await process_frame; await process_frame
	_expect(not screen._view.is_complete() and screen._view.actor("goblin").get("definition_id") == "drowned_oracle_brine_mask", "rematch preserves creature")
	_expect(screen._view.actors.size() == 3 and screen._view.actor("goblin-2").get("current_health") == 16, "rematch resets both enemies")
	_expect(screen._view.actor("goblin").get("current_health") == 16, "rematch restores all health cards")
	screen.queue_free(); await process_frame
	# Switching back to a trained enemy and then to a minion must replace the
	# runtime session rather than retaining the previous opponent or policy.
	var trained = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-b", "accepted-v1", "venom")
	var switched: Dictionary = trained.start_battle("brine-switch-trained", 33)
	_expect(switched.get("accepted") == true and switched.get("snapshot", {}).get("actors", {}).get("goblin", {}).get("definition_id") == "blade_warden", "trained opponent remains available")
	var minion = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-b", "brine-mask", "venom")
	switched = minion.start_battle("brine-switch-back", 33)
	_expect(switched.get("accepted") == true and switched.get("snapshot", {}).get("actors", {}).get("goblin", {}).get("definition_id") == "drowned_oracle_brine_mask", "minion works in Seat B after model switch")
	print("BRINE PAIR ORIGINAL SCREEN UI FULL BATTLE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _choose(actions: Array, screen: Control) -> Dictionary:
	# Exercise real automatic defense handoffs, including the final wave.
	if screen._view.stage == "defense_reaction" and actions.size() == 1 and actions[0].get("type") == "pass": return {}
	for kind in ["planning_commit_cards", "commit_interaction", "planning_roll", "planning_select_ability", "roll_dice", "pass", "planning_pass"]:
		for action in actions:
			if action.get("type") == kind and screen._action_in_focus(action): return action
	return {}

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_MINION_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("BRINE PAIR ORIGINAL SCREEN: " + message)
