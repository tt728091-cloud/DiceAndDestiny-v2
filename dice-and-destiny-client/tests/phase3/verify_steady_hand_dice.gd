extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	for face in [1, 4]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
		var result: Dictionary = gateway.start_battle("steady-hand-display-%d" % face, 1789502400810547)
		_expect(result.get("accepted") == true, "native battle starts")
		var screen = SCREEN.instantiate()
		screen.gateway = gateway
		screen.initial_result = result
		screen.learned_battle_mode = true
		screen.learned_human_seat = "seat-a"
		root.add_child(screen)
		await _settle(screen)
		for action in screen._view.legal_actions:
			if action.get("type") == "planning_roll":
				screen._send(JSON.stringify(action))
				break
		await _settle(screen)
		var history: Array = screen._view.actor("blade").get("roll_history", []).duplicate(true)
		var selected: Dictionary = {}
		var index := -1
		for action in screen._view.legal_actions:
			var payload: Dictionary = action.get("payload", {})
			var key := str(payload.get("status_id", ""))
			if action.get("type") == "planning_commit_cards" and key.ends_with(":%d" % face):
				for card_id in payload.get("card_ids", []):
					if screen._view.actor("blade").get("card_instances", {}).get(card_id, {}).get("definition_id") == "steady_hand":
						selected = action
						index = int(key.split(":")[1])
		_expect(not selected.is_empty(), "Steady Hand has a legal set-to-%d choice" % face)
		if not selected.is_empty():
			var before := int(screen._view.rolled_dice("blade")[index].face)
			screen._send(JSON.stringify(selected))
			await _settle(screen)
			_expect(before != face, "test changes the die")
			_expect(screen._view.actor("blade").roll_history == history, "card preserves original roll history")
			_expect(int(screen._view.rolled_dice("blade")[index].face) == face, "tray data uses edited face")
			_expect(screen._view.rolled_dice("blade")[index].symbols == (["fang"] if face == 1 else ["gland"]), "edited symbol matches face")
			var tray: BattleDiceTray = screen.find_children("*", "BattleDiceTray", true, false)[0]
			var buttons := tray.find_children("*", "Button", true, false)
			_expect(buttons[index].text.ends_with("\n%d" % face), "visible die button displays edited face")
			# A cached reveal must not overwrite a newer authority pool; reload
			# must also work without relying on a dice_rolled event.
			screen._view.offensive_reveals.blade = {"dice": history[-1].dice}
			_expect(int(screen._view.rolled_dice("blade")[index].face) == face, "stale reveal cannot override live dice")
			var restored := BattleViewState.new()
			_expect(restored.apply_result({"accepted": true, "snapshot": screen._view.raw_snapshot}), "snapshot reload succeeds")
			_expect(int(restored.rolled_dice("blade")[index].face) == face, "reload preserves edited die")
		_expect(screen._error_message.is_empty(), "no command error")
		screen.active_store.clear()
		screen.queue_free()
		await process_frame
	# Do not substitute defense/status dice for the offensive tray.
	var fallback := BattleViewState.new()
	fallback.actors = {"blade": {"dice": {"pool": "defensive", "dice": [{"face": 6}]}, "roll_history": [{"dice": [{"face": 3}]}]}}
	_expect(fallback.rolled_dice("blade")[0].face == 3, "non-offensive pool uses offensive history fallback")
	fallback.actors.blade.dice = {"pool": "offensive", "dice": []}
	_expect(fallback.rolled_dice("blade").is_empty(), "empty current pool does not resurrect stale history")
	print("STEADY HAND DICE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _settle(screen) -> void:
	for step in range(2000):
		await process_frame
		screen._director.clear()
		if not screen._model_thinking and not bool(screen._view.learned_policy.get("model_turn", false)):
			screen._render()
			await process_frame
			return
		if screen._model_error: break
		await create_timer(0.01).timeout
	_expect(false, "model did not return control")

func _expect(ok: bool, message: String) -> void:
	if not ok:
		failed = true
		push_error("STEADY HAND DICE: " + message)
