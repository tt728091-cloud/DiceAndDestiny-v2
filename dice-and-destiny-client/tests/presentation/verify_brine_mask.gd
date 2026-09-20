extends SceneTree

const MENU := preload("res://app/boot/battle_bootstrap.gd")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	for key in ["combat_transition_seconds", "damage_reveal_seconds", "damage_hold_seconds", "damage_removal_seconds", "effects_gather_seconds", "defense_roll_seconds", "defense_effects_seconds", "defense_hold_seconds"]:
		ProjectSettings.set_setting("dice_and_destiny/presentation/" + key, 0.1)
	var menu := MENU.new(); root.add_child(menu)
	await process_frame
	_expect(menu._character_choice.get_selected_metadata() == "venom", "Venom is ready to fight")
	_expect(menu._model_choice.get_selected_metadata() == "brine-mask", "minion is selectable in opponent dropdown")
	menu._model_choice.select(1); menu._update_opponent_description()
	_expect("trained Blade Warden" in menu._message.text, "description follows opponent selection")
	menu._model_choice.select(0); menu._update_opponent_description()
	menu._start_selected()
	await process_frame; await process_frame
	var screen: Control
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == "res://app/screens/battle/battle_screen.gd": screen = child
	_expect(screen != null, "Start Battle opens the actual battle screen" + (": " + menu._message.text if is_instance_valid(menu) else ""))
	if screen == null: quit(1); return
	await process_frame
	_expect(screen._view.actor("goblin").get("definition_id") == "drowned_oracle_brine_mask", "authority selected actual minion")
	_expect(screen._view.actor("goblin").get("max_health") == 16, "sixteen-card health deck")
	_expect(screen._actor_display_name("goblin") == "Brine Mask", "correct enemy name")
	var fighter := screen._root.get_node("BattleScenery/Fighter_enemy") as TextureRect
	_expect(fighter.texture.resource_path == "res://assets/battle/minions/drowned_oracle/brine_mask.png", "requested creature art is in battle")
	_expect(fighter.size.y == 320.0, "creature uses minion scale")
	_expect(BattlePresentationCatalog.symbol_for_die_face("brine_d6", 3) == "≈", "new dice symbols are published")
	_expect(not screen._view.content_definition("abilities", "salt_veil").is_empty(), "new rules are published to the UI")
	await _capture("brine-mask-start.png")
	var deadline := Time.get_ticks_msec() + 150000
	var saw_defense := false
	var saw_damage := false
	while Time.get_ticks_msec() < deadline and not screen._view.is_complete():
		if screen._model_error or not screen._error_message.is_empty(): break
		for panel in screen._defense_result_panels:
			if not saw_defense and panel.data.get("actor_id") == "goblin" and panel.data.get("ability_name") == "Salt Veil" and not panel.data.get("awaiting_roll", false) and not panel.data.dice.is_empty():
				_expect(panel.data.dice.size() == 1 and panel.dice_controls.size() == 1, "Salt Veil displays one defense die")
				var face := int(panel.data.dice[0].face)
				_expect(face >= 1 and face <= 6, "defense die has a valid result")
				_expect(int(panel.data.prevented) == ceili(face / 2.0), "defense displays half the roll rounded up: " + JSON.stringify(panel.data))
				if not saw_defense:
					saw_defense = true
					screen._auto_pass_disabled = true
					await create_timer(0.6).timeout
					await _capture("brine-mask-defense.png")
					screen._auto_pass_disabled = false
		saw_damage = saw_damage or screen._view.stage == "damage_reaction"
		if not screen._submitting and not screen._model_thinking and not screen._director.has_beats() and not screen._player_roll_active() and screen._selection_morph.is_empty() and Time.get_ticks_msec() >= screen._interaction_deadline(true) and not bool(screen._view.learned_policy.get("model_turn", false)):
			var action := _choose(screen._view.legal_actions)
			if not action.is_empty(): screen._send(JSON.stringify(action))
		await process_frame
	_expect(screen._error_message.is_empty() and not screen._model_error, "full screen battle has no authority or controller errors: " + screen._error_message)
	_expect(screen._view.is_complete(), "visible battle reaches victory, defeat, or draw")
	_expect(saw_defense and saw_damage, "visible defense and damage flow exercised")
	await _capture("brine-mask-complete.png")
	var telemetry: Dictionary = screen.gateway.telemetry().get("result", {})
	_expect(int(telemetry.get("lifetime", {}).get("model_load_count", -1)) == 0, "minion needs no trained model")
	screen._play_again()
	await process_frame; await process_frame
	_expect(not screen._view.is_complete() and screen._view.actor("goblin").get("definition_id") == "drowned_oracle_brine_mask", "rematch preserves creature")
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
	print("BRINE MASK UI FULL BATTLE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _choose(actions: Array) -> Dictionary:
	for kind in ["planning_commit_cards", "commit_interaction", "planning_roll", "planning_select_ability", "roll_dice", "pass", "planning_pass"]:
		for action in actions:
			if action.get("type") == kind: return action
	return {}

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_MINION_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("BRINE MASK: " + message)
