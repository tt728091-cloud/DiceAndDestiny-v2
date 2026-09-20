extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = gateway.start_battle("player-roll-animation", 1789679833957118)
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("player-roll-animation.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	for frame in 5: await process_frame
	var roll := BattleCommandBuilder.planning_roll(screen._view.battle_id, "blade", screen._pending())
	screen._send(roll)
	_expect(screen._error_message.is_empty() and screen._player_roll_active(), "accepted initial roll starts animation")
	_expect(screen._player_roll_feedback.indices == [0, 1, 2, 3, 4], "initial roll animates all five dice")
	var used: int = screen._view.rolls_used("blade")
	screen._send(roll)
	_expect(screen._view.rolls_used("blade") == used, "repeated click during animation cannot submit another roll")
	await _watch_roll(screen, [])
	_expect(screen._view.rolls_used("blade") == 1, "animation does not spend additional rolls")
	screen._selected_indices = [0, 2]
	screen._reroll_unkept()
	_expect(screen._error_message.is_empty() and screen._player_roll_active(), "accepted reroll starts animation")
	_expect(screen._player_roll_feedback.indices == [1, 3, 4], "only unkept dice animate")
	await _watch_roll(screen, [0, 2])
	_expect(screen._view.rolls_used("blade") == 2, "keep plus reroll spends one roll")
	var tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
	_expect(tray.selected == [0, 2], "kept highlights survive animation")
	# Both standard and illustrated dice animate even when their result repeats.
	for die_id in ["standard_d6", "venom_d6"]:
		var standalone := BattleDiceTray.new(); root.add_child(standalone)
		standalone.display([{"die_id": die_id, "face": 4}], [], true)
		for repeat in 2:
			standalone.animate_roll([0], Time.get_ticks_msec(), 450)
			_expect(standalone._buttons[0].tooltip_text == "Rolling…", "same-result roll still gives feedback: " + die_id)
			standalone._roll_started_ms -= 500; standalone._update_roll()
			_expect(standalone._buttons[0].text.ends_with("\n4") and standalone._buttons[0].rotation == 0.0, "exact authoritative face restored: " + die_id)
		standalone.queue_free()
	# Failed commands must not create a cosmetic success roll.
	screen._send(BattleCommandBuilder.planning_reroll(screen._view.battle_id, "blade", screen._pending(), [99]))
	_expect(not screen._error_message.is_empty() and not screen._player_roll_active(), "rejected roll does not animate")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("PLAYER ROLL ANIMATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _watch_roll(screen: Control, kept: Array) -> void:
	var start: int = screen._player_roll_feedback.started_ms
	var initial_tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
	var held := {}
	for index in kept: held[index] = initial_tray._buttons[index].text
	var faces := {}; var redrawn := false
	while screen._player_roll_active():
		var tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
		for index in 5:
			var die := tray._buttons[index]
			if index in kept:
				_expect(die.text == held[index] and die.rotation == 0.0, "held die remains stationary")
				_expect(die.get_theme_stylebox("disabled") == die.get_theme_stylebox("pressed"), "held highlight remains visible while input is locked")
			else:
				faces[die.text] = true
				_expect(die.tooltip_text == "Rolling…", "unkept die visibly rolling")
		if not redrawn and Time.get_ticks_msec() - start > 150:
			screen._render(); redrawn = true
			_expect(screen._player_roll_feedback.started_ms == start, "redraw retains original clock")
		await process_frame
	_expect(faces.size() > 2, "multiple transient faces are shown")
	for frame in 3: await process_frame
	var final_tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
	var actual: Array = screen._view.rolled_dice("blade")
	for index in 5:
		_expect(final_tray._buttons[index].text.ends_with("\n%d" % int(actual[index].face)), "all dice finish at authority values")
		_expect(final_tray._buttons[index].rotation == 0.0, "all dice settle upright")
	_expect(not final_tray._buttons[0].disabled, "dice selection restored after settling")

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("PLAYER ROLL ANIMATION: " + message)
