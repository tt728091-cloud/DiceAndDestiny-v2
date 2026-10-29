extends SceneTree

## The menu's look-ahead opponent plays a mirror of the human's character
## through the native runtime: it plays the human's character, finishes a
## battle from either seat, answers well inside its time allowance and never
## reveals its private planning to the human view.

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const LEARNED_RUNTIME := preload("res://local_client/learned_battle/learned_battle_runtime.gd")

var _failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var menu = BOOTSTRAP.instantiate()
	root.add_child(menu)
	await process_frame
	var keys: Array[String] = []
	for index in range(menu._model_choice.item_count):
		keys.append(str(menu._model_choice.get_item_metadata(index)))
	_expect(LEARNED_RUNTIME.MODEL_LOOKAHEAD in keys, "look-ahead mirror missing from the opponent menu")
	menu._model_choice.select(keys.find(LEARNED_RUNTIME.MODEL_LOOKAHEAD))
	menu._update_opponent_description()
	_expect("look-ahead" in menu._message.text, "look-ahead description missing")
	menu.queue_free()
	await process_frame

	var runtime = root.get_node_or_null("LearnedBattleRuntime")
	if runtime == null:
		_fail("LearnedBattleRuntime autoload is unavailable")
		quit(1)
		return
	for seat in ["seat-a", "seat-b"]:
		var gateway = LEARNED_GATEWAY.new(runtime, seat, LEARNED_RUNTIME.MODEL_LOOKAHEAD, "starter")
		gateway.unified_defense = true
		var view: Dictionary = gateway.start_battle("godot-lookahead-%s" % seat, 20261015)
		_expect(view.get("accepted") == true, "%s: look-ahead battle did not start: %s" % [seat, JSON.stringify(view)])
		if view.get("accepted") != true: continue
		_expect(str(view.get("snapshot", {}).get("actors", {}).get("goblin", {}).get("definition_id", "")) == "starter", "%s: opponent is not a Starter mirror" % seat)
		_expect(str(view.get("learned_policy", {}).get("controller_kind", "")) == "lookahead", "%s: controller is not the look-ahead player" % seat)
		var slowest := 0
		var guard := 0
		while view.get("battle_result", "") not in ["victory", "defeat", "draw"] and guard < 1500:
			guard += 1
			if view.get("learned_policy", {}).get("model_turn") == true:
				var started := Time.get_ticks_msec()
				view = gateway.advance_model()
				slowest = maxi(slowest, Time.get_ticks_msec() - started)
			else:
				var command := _preferred_command(view)
				if command.is_empty():
					_fail("%s: no human command at %s" % [seat, JSON.stringify(view.get("snapshot", {}).get("stage", ""))])
					break
				view = gateway.submit(JSON.stringify(command))
			if view.get("accepted") != true:
				_fail("%s: command failed: %s" % [seat, JSON.stringify(view).left(600)])
				break
			_expect(_safe_human_view(view), "%s: human view leaked the opponent's private state" % seat)
		_expect(view.get("battle_result", "") in ["victory", "defeat", "draw"], "%s: battle did not finish" % seat)
		_expect(slowest < LEARNED_RUNTIME.LOOKAHEAD_TIMEOUT_MS, "%s: look-ahead decision took %d ms" % [seat, slowest])
		var telemetry: Dictionary = gateway.telemetry().get("result", {}).get("battle", {})
		_expect(int(telemetry.get("model_decisions", 0)) > 0 and int(telemetry.get("timeouts", -1)) == 0, "%s: model decisions or timeouts wrong" % seat)
		print("LOOK-AHEAD MIRROR %s: %s after %d steps, slowest decision %d ms" % [seat, view.get("battle_result", ""), guard, slowest])
		if seat == "seat-a" and view.get("accepted") == true:
			var screen = BATTLE_SCREEN.instantiate()
			screen.initial_result = view
			screen.gateway = gateway
			screen.learned_battle_mode = true
			screen.learned_human_seat = seat
			screen.learned_seed = 20261015
			root.add_child(screen)
			await process_frame
			_expect(screen._is_lookahead_opponent(), "battle screen does not recognize the look-ahead opponent")
			_expect(screen._actor_display_name("goblin").begins_with("Look-ahead"), "opponent is not labelled Look-ahead")
			_expect(screen._model_timeout_ms() == LEARNED_RUNTIME.LOOKAHEAD_TIMEOUT_MS, "look-ahead thinking warns at the frozen-policy limit")
			screen.queue_free()
			await process_frame
	if not _failed:
		print("LOOK-AHEAD MIRROR GODOT: menu entry, both seats, view safety and labels passed")
	quit(1 if _failed else 0)

func _preferred_command(view: Dictionary) -> Dictionary:
	var actions: Array = view.get("legal_actions", [])
	for kind in ["planning_roll", "planning_select_ability", "roll_dice", "planning_pass", "pass", "commit_interaction"]:
		for action in actions:
			if str(action.get("type", "")) == kind:
				return action
	return actions[0] if not actions.is_empty() else {}

func _safe_human_view(view: Dictionary) -> bool:
	var snapshot: Dictionary = view.get("snapshot", {})
	var opponent: Dictionary = snapshot.get("actors", {}).get("goblin", {})
	if not opponent.get("hand", []).is_empty() or not opponent.get("card_instances", {}).is_empty():
		return false
	if str(snapshot.get("stage", "")) == "planning" and not opponent.get("selected_ability", "").is_empty():
		return false
	return true

func _expect(condition: bool, message: String) -> void:
	if not condition:
		_fail(message)

func _fail(message: String) -> void:
	_failed = true
	push_error(message)
	print("FAIL: " + message)
