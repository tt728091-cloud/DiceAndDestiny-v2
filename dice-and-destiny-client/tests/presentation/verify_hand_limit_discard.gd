extends SceneTree
## A real Adventurer battle (the menu's learned-opponent route) plays without
## spending cards until its hand exceeds the limit. The player then selects the
## required discards and presses the Discard button with the pointer. Shipped
## cards are program cards, and a single discard once collided with a card play.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var tested := {}
	for seed in range(1, 40):
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
		gateway.unified_defense = true
		var result: Dictionary = gateway.start_battle("hand-limit-discard-%d" % seed, seed)
		_expect(result.get("accepted") == true, "battle starts: %s" % result.get("error", ""))
		for step in 600:
			if result.get("accepted") != true or not str(result.get("battle_result", "")).is_empty(): break
			if result.snapshot.stage == "discard_to_hand_limit" and not result.learned_policy.get("model_turn", false): break
			if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
			else: result = gateway.submit(JSON.stringify(_next_action(result)))
		if result.get("accepted") != true or result.snapshot.stage != "discard_to_hand_limit": continue
		var blade: Dictionary = result.snapshot.actors.blade
		var need := int(blade.hand_count) - int(blade.max_hand_size)
		if tested.has(need): continue
		tested[need] = true
		await _discard_with_pointer(gateway, result, need, seed)
		if tested.has(1): break
	_expect(tested.has(1), "a real battle reached a one-card hand-limit discard: %s" % [tested.keys()])
	print("HAND LIMIT DISCARD: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _discard_with_pointer(gateway, result: Dictionary, need: int, seed: int) -> void:
	var label := "seed %d, discard %d" % [seed, need]
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hand-limit-discard.json"))
	root.add_child(screen)
	for frame in 12: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	var cards: Array = screen._hand_dock.find_children("*", "BattleCard", true, false)
	var hand_before: Array = result.snapshot.actors.blade.hand.duplicate()
	var chosen: Array[String] = []
	for index in need:
		await _click(cards[index])
		cards = screen._hand_dock.find_children("*", "BattleCard", true, false)
		chosen.append(str(cards[index].instance_id))
	var commit: Button = _button_containing(screen, "Discard selected cards")
	_expect(commit != null and not commit.disabled, "%s: Discard enables after selecting %d card(s): %s" % [label, need, commit.text if commit != null else "missing"])
	if commit != null: await _click(commit)
	for frame in 6: await process_frame
	_expect(screen._error_message.is_empty(), "%s: discard is accepted without an error: %s" % [label, screen._error_message])
	var blade: Dictionary = screen._view.actor("blade")
	_expect(screen._view.stage != "discard_to_hand_limit" or int(blade.get("hand_count", 0)) <= int(blade.get("max_hand_size", 0)), "%s: the hand-limit window closes" % label)
	# Snapshots publish discard as counts; the next round's Income draws only from the deck.
	var discard_before := int(result.snapshot.actors.blade.discard_count)
	for id in chosen:
		_expect(id in hand_before and not id in blade.get("hand", []), "%s: %s left the hand" % [label, id])
	_expect(int(blade.get("discard_count", 0)) >= discard_before + need and int(blade.get("removed_count", 0)) == int(result.snapshot.actors.blade.removed_count), "%s: the discards reached the discard pile without removal" % label)
	screen.active_store.clear(); screen.queue_free(); await process_frame

## Plays only Take Stock (draw 2), so the hand grows past the limit.
func _next_action(result: Dictionary) -> Dictionary:
	var actions: Array = result.legal_actions
	var instances: Dictionary = result.snapshot.actors.blade.get("card_instances", {})
	for action in actions:
		if action.type != "planning_commit_cards" or action.payload.get("card_ids", []).size() != 1: continue
		var choice = JSON.parse_string(str(action.payload.get("status_id", "{}")))
		if choice is Dictionary and choice.get("verb", "") == "cancel": continue
		if instances.get(str(action.payload.card_ids[0]), {}).get("definition_id", "") == "take_stock": return action
	for kind in ["planning_roll", "roll_dice", "planning_select_ability", "planning_select_targets", "planning_pass", "pass"]:
		for action in actions:
			if action.type == kind: return action
	return {}

func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; motion.global_position = point
	root.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed
		click.position = point; click.global_position = point
		root.push_input(click, true); await process_frame

func _button_containing(node: Node, text_part: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if text_part in child.text and child.is_visible_in_tree(): return child
	return null

func _expect(ok: bool, message: String) -> void:
	if not ok:
		push_error("HAND LIMIT DISCARD: " + message)
		failed = true
