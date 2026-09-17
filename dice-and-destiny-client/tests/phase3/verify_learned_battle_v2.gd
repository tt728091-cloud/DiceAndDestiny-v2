extends SceneTree

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const EXPECTED_V1_MODEL_ID := "blade-warden-maskable-ppo-seed-11-final-v1"
const EXPECTED_V2_MODEL_ID := "blade-warden-decision-quality-seed-22-v2"
const EXPECTED_V2_POLICY_SHA256 := "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
const EXPECTED_V3_MODEL_ID := "blade-warden-optimized-5m-seed-22-v3"
const EXPECTED_V3_POLICY_SHA256 := "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"
const EXPECTED_PRIOR_GLOBAL_CP38_MODEL_ID := "blade-warden-global-champion-cp38-winner-health-v2"
const EXPECTED_PRIOR_GLOBAL_CP38_POLICY_SHA256 := "6aeb05e0657c8c221298c79780895bcc6aa527f1780c6c5eb66833293daf6693"
const EXPECTED_GLOBAL_MODEL_ID := "blade-warden-global-champion-cp193-winner-health-v2"
const EXPECTED_GLOBAL_POLICY_SHA256 := "cd3d7451e91d071274c874b017e3c7e73358862090fd71954e282e5bf505b814"

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
	_expect(runtime.select_model("decision-v2").get("ok") == true, "new v2 selection failed")
	var v2_initialized: Dictionary = runtime.ensure_initialized()
	_expect(v2_initialized.get("ok") == true, "new v2 policy failed to initialize")
	_expect(
		str(v2_initialized.get("result", {}).get("model", {}).get("model_id", "")) == EXPECTED_V2_MODEL_ID,
		"normal runtime did not switch to new v2"
	)
	_expect(
		str(v2_initialized.get("result", {}).get("model", {}).get("policy_export_sha256", "")) == EXPECTED_V2_POLICY_SHA256,
		"normal runtime loaded the wrong v2 export hash"
	)
	_expect(runtime.select_model("optimized-v3").get("ok") == true, "strongest v3 selection failed")
	var v3_initialized: Dictionary = runtime.ensure_initialized()
	_expect(v3_initialized.get("ok") == true, "strongest v3 policy failed to initialize")
	_expect(
		str(v3_initialized.get("result", {}).get("model", {}).get("model_id", "")) == EXPECTED_V3_MODEL_ID,
		"normal runtime did not switch to strongest v3"
	)
	_expect(
		str(v3_initialized.get("result", {}).get("model", {}).get("policy_export_sha256", "")) == EXPECTED_V3_POLICY_SHA256,
		"normal runtime loaded the wrong strongest v3 export hash"
	)
	_expect(runtime.select_model("prior-global-cp38").get("ok") == true, "prior global CP38 selection failed")
	var cp38_initialized: Dictionary = runtime.ensure_initialized()
	_expect(cp38_initialized.get("ok") == true, "prior global CP38 policy failed to initialize")
	_expect(
		str(cp38_initialized.get("result", {}).get("model", {}).get("model_id", "")) == EXPECTED_PRIOR_GLOBAL_CP38_MODEL_ID,
		"normal runtime did not preserve prior global CP38"
	)
	_expect(
		str(cp38_initialized.get("result", {}).get("model", {}).get("policy_export_sha256", "")) == EXPECTED_PRIOR_GLOBAL_CP38_POLICY_SHA256,
		"normal runtime loaded the wrong prior global CP38 export hash"
	)

	var menu = BOOTSTRAP.instantiate()
	root.add_child(menu)
	await process_frame
	var mode_scroll := menu.find_child("ModeScroll", true, false) as ScrollContainer
	_expect(mode_scroll != null, "battle-mode list is not inside a scroll container")
	if mode_scroll != null:
		_expect(mode_scroll.vertical_scroll_mode == ScrollContainer.SCROLL_MODE_AUTO, "battle-mode list does not auto-scroll vertically")
		_expect(mode_scroll.horizontal_scroll_mode == ScrollContainer.SCROLL_MODE_DISABLED, "battle-mode list unexpectedly scrolls horizontally")
		_expect(mode_scroll.follow_focus, "battle-mode list does not follow keyboard focus")
	_expect(menu._character_choice.item_count == 2, "playable character selection missing")
	_expect(menu._model_choice.item_count == 6, "six preserved model choices missing")
	var keys: Array[String] = []
	for index in range(menu._model_choice.item_count):
		keys.append(str(menu._model_choice.get_item_metadata(index)))
	for key in ["accepted-v1", "decision-v2", "optimized-v3", "prior-global-cp480", "prior-global-cp38", "global-champion"]:
		_expect(key in keys, "opponent menu is missing " + key)
	menu._seat_choice.select(1)
	menu._model_choice.select(keys.find("global-champion"))
	var start_button := _find_button_containing(menu, "Start Battle")
	_expect(start_button != null and not start_button.disabled, "start button is not actionable")
	if start_button != null: start_button.pressed.emit()
	await process_frame
	await process_frame

	var screen = _current_battle_screen()
	_expect(screen != null, "global champion menu selection did not open the battle screen")
	if screen == null:
		_finish()
		return
	var view: Dictionary = screen.initial_result
	_expect(screen.learned_human_seat == "seat-b", "global champion menu lost the selected human seat")
	_expect(view.get("accepted") == true, "global champion graphical battle did not start: %s" % JSON.stringify(view))
	_expect(view.get("learned_policy", {}).get("model_turn") == true, "global champion policy did not own the opening turn")
	_expect(str(view.get("learned_policy", {}).get("model_id", "")) == EXPECTED_GLOBAL_MODEL_ID, "menu loaded the wrong global champion")
	_expect(str(view.get("learned_policy", {}).get("policy_export_sha256", "")) == EXPECTED_GLOBAL_POLICY_SHA256, "menu loaded the wrong global champion export hash")
	_expect(str(view.get("learned_policy", {}).get("observation_schema", "")) == "dice-and-destiny-observation-v2", "menu loaded the wrong observation schema")

	var gateway: RefCounted = screen.gateway
	var guard := 0
	while view.get("learned_policy", {}).get("model_turn") == true and guard < 100:
		guard += 1
		view = gateway.advance_model()
		if view.get("accepted") != true:
			break
	_expect(guard > 0 and guard < 100, "global champion did not yield a clean graphical turn")
	_expect(view.get("accepted") == true, "global champion advance failed: %s" % JSON.stringify(view))
	var telemetry: Dictionary = gateway.telemetry().get("result", {}).get("battle", {})
	_expect(int(telemetry.get("model_decisions", 0)) > 0, "global champion runtime recorded no model decisions")
	_expect(int(telemetry.get("authority_rejects", 0)) == 0, "global champion runtime recorded authority rejects")
	_expect(int(telemetry.get("invalid_actions", 0)) == 0, "global champion runtime recorded invalid actions")
	_expect(int(telemetry.get("stale_actions", 0)) == 0, "global champion runtime recorded stale actions")
	_expect(int(telemetry.get("wrong_seat_actions", 0)) == 0, "global champion runtime recorded wrong-seat actions")
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
	push_error("LEARNED V1/V2/V3/GLOBAL GODOT: %s" % message)

func _finish() -> void:
	if _failed:
		quit(1)
	else:
		print("LEARNED V1/V2/V3/GLOBAL GODOT: menu preserves all opponents and switches to global champion CP193")
		quit(0)
