extends SceneTree
# Card feedback never holds input: a second card, or the offensive roll, can be
# submitted while an earlier card still animates, and both animations play
# together. Only an offensive roll holds the next roll.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const GAIN := preload("res://presentation/battle/card_gain_notice.gd")
const FEEDBACK_CARDS := ["take_stock", "second_wind", "battle_focus"]
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var pair_tested := false
	var roll_tested := false
	for seed_value in range(1, 80):
		if pair_tested and roll_tested: break
		var result: Dictionary = gateway.start_battle("concurrent-card-feedback-%d" % seed_value, seed_value)
		if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model()
		if result.get("accepted") != true: continue
		var plays := _feedback_plays(result)
		if plays.size() < 1: continue
		var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("concurrent-card-feedback.json"))
		root.add_child(screen)
		for frame in 4: await process_frame
		var first := str(plays[0].payload.card_ids[0])
		screen._send(JSON.stringify(plays[0]))
		_expect(screen._error_message.is_empty() and not _in_hand(screen, first), "first feedback card plays")
		_expect(screen._card_gain_active(), "first card starts its animation")
		await process_frame
		var second_play := _feedback_plays({"legal_actions": screen._view.legal_actions, "snapshot": screen._view.raw_snapshot})
		if not pair_tested and not second_play.is_empty():
			var second := str(second_play[0].payload.card_ids[0])
			var first_notice = _notices(screen)[0]
			screen._send(JSON.stringify(second_play[0]))
			_expect(screen._error_message.is_empty() and not _in_hand(screen, second), "second card plays while the first animates")
			var notices := _notices(screen)
			_expect(notices.size() == 2 and notices.has(first_notice), "first animation continues after the second play")
			await process_frame; await process_frame
			_expect(notices.all(func(notice): return is_instance_valid(notice) and notice._started and not notice._waiting()), "both card animations run together")
			pair_tested = true
		if not roll_tested and screen._view.allowed("planning_roll") and screen._card_gain_active():
			screen._send(BattleCommandBuilder.planning_roll(screen._view.battle_id, "blade", screen._pending()))
			_expect(screen._error_message.is_empty() and screen._view.rolls_used("blade") == 1 and screen._player_roll_active(), "roll is accepted while card feedback animates")
			var during_roll := _feedback_plays({"legal_actions": screen._view.legal_actions, "snapshot": screen._view.raw_snapshot})
			for card in screen._hand_dock.cards:
				if during_roll.any(func(action): return card.instance_id in action.payload.card_ids):
					_expect(not card.disabled, "playable hand cards stay enabled during the roll")
			if not during_roll.is_empty():
				var third := str(during_roll[0].payload.card_ids[0])
				screen._send(JSON.stringify(during_roll[0]))
				_expect(screen._error_message.is_empty() and not _in_hand(screen, third), "card plays while the dice roll")
				_expect(screen._player_roll_active(), "card play does not cut the roll animation short")
			if screen._view.allowed("planning_reroll") and screen._player_roll_active():
				var used: int = screen._view.rolls_used("blade")
				screen._send(BattleCommandBuilder.planning_reroll(screen._view.battle_id, "blade", screen._pending(), [0]))
				screen._reroll_unkept()
				_expect(screen._view.rolls_used("blade") == used, "a roll still holds the next roll")
			roll_tested = true
		screen.active_store.clear(); screen.queue_free(); await process_frame
	_expect(pair_tested, "two feedback cards exercised through the native gateway")
	_expect(roll_tested, "roll during card feedback exercised through the native gateway")
	print("CONCURRENT CARD FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

# Complete single-card plays of cards that animate a resource/card gain.
func _feedback_plays(result: Dictionary) -> Array:
	var plays: Array = []
	var instances: Dictionary = result.get("snapshot", {}).get("actors", {}).get("blade", {}).get("card_instances", {})
	for action in result.get("legal_actions", []):
		if action.get("actor_id") != "blade" or action.get("type") != "planning_commit_cards": continue
		var ids: Array = action.get("payload", {}).get("card_ids", [])
		if ids.size() != 1 or str(instances.get(ids[0], {}).get("definition_id", "")) not in FEEDBACK_CARDS: continue
		var choice = JSON.parse_string(str(action.payload.get("status_id", "")))
		if choice is Dictionary and str(choice.get("verb", "")) == "start" and str(choice.get("then", "")).is_empty() and _has_targeted_start(result, ids[0]): continue
		plays.append(action)
	return plays

func _has_targeted_start(result: Dictionary, card_id: String) -> bool:
	for action in result.get("legal_actions", []):
		if card_id not in action.get("payload", {}).get("card_ids", []): continue
		var choice = JSON.parse_string(str(action.payload.get("status_id", "")))
		if choice is Dictionary and not str(choice.get("then", "")).is_empty(): return true
	return false

func _in_hand(screen, card_id: String) -> bool:
	return screen._view.hand_cards().any(func(entry): return str(entry.instance_id) == card_id)

func _notices(screen) -> Array:
	return screen.get_children().filter(func(node): return node.get_script() == GAIN and not node.is_queued_for_deletion())

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("CONCURRENT CARD FEEDBACK: " + message)
