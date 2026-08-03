extends SceneTree

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const EXPECTED_V1_MODEL_ID := "blade-warden-maskable-ppo-seed-11-final-v1"
const EXPECTED_V2_MODEL_ID := "blade-warden-decision-quality-seed-22-v2"
const EXPECTED_POLICY_SHA256 := "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"

var _failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var runtime = root.get_node_or_null("LearnedBattleRuntime")
	if runtime == null:
		_fail("LearnedBattleRuntime autoload is unavailable")
		_finish()
		return

	# Start from the normal default to prove that the menu can replace an
	# already-loaded old policy without requiring an environment flag or restart.
	_expect(runtime.select_model("accepted-v1").get("ok") == true, "old v1 selection failed")
	var old_initialized: Dictionary = runtime.ensure_initialized()
	_expect(old_initialized.get("ok") == true, "old v1 policy failed to initialize")
	_expect(
		str(old_initialized.get("result", {}).get("model", {}).get("model_id", "")) == EXPECTED_V1_MODEL_ID,
		"normal runtime did not initialize old v1 first"
	)

	var menu = BOOTSTRAP.instantiate()
	root.add_child(menu)
	await process_frame
	var menu_text := _button_text(menu)
	for label in [
		"Human Seat A · Old v1",
		"Human Seat B · Old v1",
		"Human Seat A · New v2",
		"Human Seat B · New v2",
	]:
		_expect(label in menu_text, "opponent menu is missing %s" % label)
	var new_v2 := _find_button_containing(menu, "Human Seat B · New v2")
	_expect(new_v2 != null and new_v2.visible and not new_v2.disabled, "new v2 Seat B option is not actionable")
	if new_v2 != null:
		new_v2.pressed.emit()
	await process_frame
	await process_frame

	var screen = _current_battle_screen()
	_expect(screen != null, "new v2 menu selection did not open the battle screen")
	if screen == null:
		_finish()
		return
	var view: Dictionary = screen.initial_result
	_expect(screen.learned_human_seat == "seat-b", "new v2 menu lost the selected human seat")
	_expect(view.get("accepted") == true, "v2 graphical battle did not start: %s" % JSON.stringify(view))
	_expect(view.get("learned_policy", {}).get("model_turn") == true, "v2 policy did not own the opening turn")
	_expect(str(view.get("learned_policy", {}).get("model_id", "")) == EXPECTED_V2_MODEL_ID, "menu loaded the wrong opponent")
	_expect(str(view.get("learned_policy", {}).get("policy_export_sha256", "")) == EXPECTED_POLICY_SHA256, "menu loaded the wrong v2 export hash")
	_expect(str(view.get("learned_policy", {}).get("observation_schema", "")) == "dice-and-destiny-observation-v2", "menu loaded the wrong observation schema")

	var gateway: RefCounted = screen.gateway
	var guard := 0
	while view.get("learned_policy", {}).get("model_turn") == true and guard < 100:
		guard += 1
		view = gateway.advance_model()
		if view.get("accepted") != true:
			break
	_expect(guard > 0 and guard < 100, "v2 model did not yield a clean graphical turn")
	_expect(view.get("accepted") == true, "v2 model advance failed: %s" % JSON.stringify(view))
	var telemetry: Dictionary = gateway.telemetry().get("result", {}).get("battle", {})
	_expect(int(telemetry.get("model_decisions", 0)) > 0, "v2 runtime recorded no model decisions")
	_expect(int(telemetry.get("authority_rejects", 0)) == 0, "v2 runtime recorded authority rejects")
	_expect(int(telemetry.get("invalid_actions", 0)) == 0, "v2 runtime recorded invalid actions")
	_expect(int(telemetry.get("stale_actions", 0)) == 0, "v2 runtime recorded stale actions")
	_expect(int(telemetry.get("wrong_seat_actions", 0)) == 0, "v2 runtime recorded wrong-seat actions")
	_finish()

func _button_text(node: Node) -> String:
	var values: Array[String] = []
	for child in node.find_children("*", "Button", true, false):
		values.append(str(child.text))
	return "\n".join(values)

func _find_button_containing(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if text in str(child.text):
			return child
	return null

func _current_battle_screen():
	var screens := get_nodes_in_group("inspectable_battle_screen")
	return screens[-1] if not screens.is_empty() else null

func _expect(condition: bool, message: String) -> void:
	if not condition:
		_fail(message)

func _fail(message: String) -> void:
	_failed = true
	push_error("PHASE 2 V2 GODOT: %s" % message)

func _finish() -> void:
	if _failed:
		quit(1)
	else:
		print("PHASE 2 V2 GODOT: menu lists old/new opponents and switches to pinned v2")
		quit(0)
