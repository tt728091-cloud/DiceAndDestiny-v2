extends SceneTree

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
var failed := false
var captured := {}

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var menu = BOOTSTRAP.instantiate()
	root.add_child(menu)
	await process_frame
	_expect(menu._character_choice.item_count == 2, "two playable characters")
	_expect(menu._model_choice.item_count == 6, "six preserved opponents")
	menu._character_choice.select(1)
	_expect(str(menu._character_choice.get_selected_metadata()) == "venom", "Venom selection")
	await _capture("venom-menu")
	menu._start_selected()
	await process_frame
	await process_frame
	var screens := get_nodes_in_group("inspectable_battle_screen")
	if screens.is_empty():
		_expect(false, "startup did not open battle")
		quit(1)
		return
	var screen = screens[-1]
	_expect(str(screen._view.actor("blade").get("definition_id", "")) == "venom", "selected human character")
	_expect(str(screen._view.actor("goblin").get("definition_id", "")) == "blade_warden", "learned opponent character")
	_expect(screen.gateway.character == "venom", "rematch retains character")
	var exercised_choice := false
	for step in range(1500):
		await process_frame
		if screen._model_thinking:
			await create_timer(0.01).timeout
			continue
		_expect(not screen._model_error, "model inference: " + str(screen._error_message))
		if screen._model_error: break
		if screen._director.peek().get("type") == "effects_resolved":
			await create_timer(7.4).timeout
			continue
		if screen._director.peek().get("type") in ["card_cleanse", "poison_conversion"]:
			await create_timer(2.9).timeout
			continue
		screen._director.clear()
		screen._render()
		await process_frame
		if screen._view.is_complete(): break
		if not screen._error_message.is_empty():
			_expect(false, str(screen._error_message))
			break
		var stage := str(screen._view.stage)
		if stage == "defense_roll" and not screen._defense_roll_action().is_empty():
			await create_timer(1.0).timeout
			continue
		if stage in ["planning", "venom_status_reaction", "status_roll_reaction", "defense_reaction"]:
			await _capture("venom-" + stage)
		var actions: Array = screen._view.legal_actions
		if actions.is_empty(): continue
		var action := _preferred(actions)
		if action.is_empty():
			_expect(false, "no progressing action at " + stage)
			break
		var kind := str(action.get("type", ""))
		if kind == "planning_select_ability" and action.get("payload", {}).get("ability_id") == "needlefang":
			var tier_id := str(action.payload.tier_id)
			var clicked := false
			for button in screen.find_children("*", "Button", true, false):
				if str(button.get_meta("inspection_id", "")) == "battle.ability.blade.needlefang." + tier_id and not button.disabled:
					button.pressed.emit()
					clicked = true
					break
			_expect(clicked, "Needlefang tier selected directly on the board")
			_expect(screen.find_children("*", "AcceptDialog", false, false).is_empty(), "Needlefang selection opens no dialog")
		elif kind in ["planning_select_ability", "planning_commit_cards"] or (kind == "commit_interaction" and stage != "discard_to_hand_limit"):
			screen._show_venom_choices([action], "Choose action")
			await process_frame
			var dialogs := screen.find_children("*", "AcceptDialog", false, false)
			_expect(not dialogs.is_empty(), "choice dialog opens")
			if dialogs.is_empty(): break
			var dialog = dialogs[-1]
			var clicked := false
			for node in dialog.find_children("*", "Button", true, false):
				if node.text != "Cancel" and not node.text.is_empty():
					await _capture("venom-choice")
					node.pressed.emit()
					clicked = true
					exercised_choice = true
					break
			_expect(clicked, "choice has a submit button")
		else:
			screen._send(JSON.stringify(action))
	_expect(screen._view.is_complete(), "full graphical game completed")
	_expect(not screen._last_defense_auto_roll.is_empty(), "native match used automatic defense rolls")
	_expect(exercised_choice, "used graphical choice controls")
	_expect(not screen._last_auto_pass_input.is_empty(), "automatically acknowledged a forced decision in the native matchup")
	_expect(screen._error_message.is_empty(), "no command errors")
	var telemetry: Dictionary = screen.gateway.telemetry().get("result", {}).get("battle", {})
	_expect(int(telemetry.get("authority_rejects", 0)) == 0, "zero authority rejects")
	_expect(int(telemetry.get("fallback_count", 0)) == 0, "zero model fallbacks")
	screen.queue_free()
	await process_frame
	print("VENOM GODOT: selection, native matchup, choice controls, and full game " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _preferred(actions: Array) -> Dictionary:
	for kind in ["planning_commit_cards", "commit_interaction", "planning_roll", "planning_select_ability", "roll_dice", "pass", "planning_pass"]:
		for action in actions:
			if str(action.get("type", "")) == kind: return action
	return {}

func _capture(label: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_VENOM_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless" or captured.has(label): return
	captured[label] = true
	await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(label + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("VENOM GODOT: " + message)
