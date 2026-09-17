extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
# Seed from the reported Venom battle's authority transcript.
const INCIDENT_SEED := 1788877881195201
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	for character in ["venom", "blade_warden"]:
		for seat in ["seat-a", "seat-b"]:
			await _check_rolls(character, seat)
			await _check_skip(character, seat)
	print("OFFENSIVE ROLL CONTROLS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _start(character: String, seat: String):
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), seat, "global-champion", character)
	var result: Dictionary = gateway.start_battle("roll-controls-" + character + seat, INCIDENT_SEED)
	_expect(result.get("accepted") == true, "start " + character + seat)
	var screen = SCREEN.instantiate()
	screen.gateway = gateway
	screen.initial_result = result
	screen.learned_battle_mode = true
	screen.learned_human_seat = seat
	root.add_child(screen)
	screen.set_process(false)
	await _settle(screen)
	return screen

func _settle(screen) -> void:
	# Let real native model decisions finish, then dismiss presentation only.
	for step in range(2000):
		# Hold forced acknowledgements while inspecting the old reaction layout.
		# Automatic progression has its own verify_automatic_pass coverage.
		screen._last_auto_pass_input = screen._view.battle_id + ":" + str(screen._pending().get("id", ""))
		await process_frame
		# Pump native inference explicitly, without racing the forced-pass scheduler.
		if screen._model_thread != null and screen._model_thread.is_started(): screen._process(0.0)
		screen._director.clear()
		if not screen._model_thinking and not bool(screen._view.learned_policy.get("model_turn", false)):
			screen._last_auto_pass_input = screen._view.battle_id + ":" + str(screen._pending().get("id", ""))
			screen._render()
			await process_frame
			return
		if screen._model_error: break
		await create_timer(0.01).timeout
	_expect(false, "model failed to return control: " + str(screen._error_message))

func _check_rolls(character: String, seat: String) -> void:
	var screen = await _start(character, seat)
	var label := character + " " + seat
	_expect(_button(screen, "Roll 5 Dice") != null, label + " initial roll visible")
	var skip := _button(screen, "Skip Offensive Ability")
	_expect(skip != null, label + " explicit skip label")
	if skip != null:
		skip.pressed.emit()
		await process_frame
		var dialog := screen.get_node_or_null("SkipUnrolledOffense") as ConfirmationDialog
		_expect(dialog != null, label + " unrolled skip requires confirmation")
		_expect(screen._view.stage == "planning", label + " opening confirmation does not end planning")
		if dialog != null:
			dialog.canceled.emit()
			await process_frame
		_expect(screen._view.allowed("planning_roll"), label + " cancel preserves roll permission")
	if character == "venom" and seat == "seat-a":
		var culture = null
		for card in screen.find_children("*", "", true, false):
			if card is BattleCard and card.definition_id == "culture_flask": culture = card
		_expect(culture != null, "incident seed starts with Culture Flask")
		if culture != null:
			screen._on_card_pressed(culture)
			await process_frame
			var dialogs := screen.find_children("*", "AcceptDialog", false, false)
			_expect(dialogs.is_empty(), "single-option Culture Flask plays without a popup")
			await _settle(screen)
			_expect(screen._view.stage == "planning", "Culture Flask preserves planning")
			_expect(_button(screen, "Roll 5 Dice") != null, "Culture Flask preserves initial roll control")
	await _capture(character + "-" + seat + "-planning")
	var roll := _button(screen, "Roll 5 Dice")
	if roll != null:
		_expect(not roll.disabled, label + " roll enabled")
		_expect(root.get_visible_rect().encloses(roll.get_global_rect()), label + " roll button inside viewport")
		roll.pressed.emit()
		await _settle(screen)
		_expect(screen._view.rolls_used("blade") == 1, label + " first roll applied")
		_expect(screen._view.rolled_dice("blade").size() == 5, label + " rolled five dice")
		var first_face := int(screen._view.rolled_dice("blade")[0].face)
		screen._selected_indices = [0]
		var reroll := _button(screen, "Reroll Unkept")
		_expect(reroll != null and not reroll.disabled, label + " reroll available")
		if reroll != null:
			reroll.pressed.emit()
			await _settle(screen)
			_expect(screen._view.rolls_used("blade") == 2, label + " reroll applied")
			_expect(int(screen._view.rolled_dice("blade")[0].face) == first_face, label + " kept die unchanged")
	await _finish_screen(screen, label)

func _check_skip(character: String, seat: String) -> void:
	var screen = await _start(character, seat)
	var label := character + " " + seat
	var skip := _button(screen, "Skip Offensive Ability")
	if skip != null:
		skip.pressed.emit()
		await process_frame
		var dialog := screen.get_node_or_null("SkipUnrolledOffense") as ConfirmationDialog
		if dialog != null: dialog.confirmed.emit()
		await _settle(screen)
		_expect(screen._view.stage == "offensive_reaction", label + " confirmed skip reaches reaction")
		var roll := _button(screen, "Roll 5 Dice")
		_expect(roll != null and roll.disabled, label + " completed planning keeps a disabled roll control visible")
		var explained := false
		for node in screen.find_children("*", "Label", true, false):
			if "You can roll again next round." in node.text: explained = true
		_expect(explained, label + " skipped roll has visible explanation")
		await _capture(character + "-" + seat + "-skipped")
	await _finish_screen(screen, label)

func _finish_screen(screen, label: String) -> void:
	_expect(screen._error_message.is_empty(), label + " no command error: " + screen._error_message)
	var telemetry: Dictionary = screen.gateway.telemetry().get("result", {}).get("battle", {})
	_expect(int(telemetry.get("authority_rejects", 0)) == 0, label + " zero authority rejects")
	screen.queue_free()
	await process_frame

func _button(screen: Node, text: String) -> Button:
	for node in screen.find_children("*", "Button", true, false):
		if node.text == text and node.is_visible_in_tree(): return node
	return null

func _expect(value: bool, message: String) -> void:
	if not value:
		failed = true
		push_error(message)

func _capture(label: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_ROLL_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(label + ".png"))
