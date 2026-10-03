extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TARGET := preload("res://presentation/battle/die_face_targeting.gd")
const ADJACENT := preload("res://presentation/battle/adjacent_face_targeting.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
var captured := {}
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var covered := {}
	var saw_cursed := false
	for seed_value in range(1, 301):
		var result: Dictionary = gateway.start_battle("widen-flow-%d" % seed_value, seed_value)
		var has_card := false
		for instance in result.snapshot.actors.blade.hand:
			if result.snapshot.actors.blade.card_instances[instance].definition_id == "widen_the_crack": has_card = true
		if not has_card: continue
		var setup := _action(result, "mark_the_number")
		if setup.is_empty(): setup = _action(result, "black_fingerprint")
		if setup.is_empty(): setup = _action(result, "shared_misfortune")
		if setup.is_empty(): setup = _action(result, "rotten_numeral")
		if setup.is_empty(): continue
		result = gateway.submit(JSON.stringify(setup))
		var action := _action(result, "widen_the_crack")
		if action.is_empty(): continue
		var presented := 0
		for event in result.events: presented = maxi(presented, int(event.get("sequence", 0)))
		result.events = []; result.learned_policy = {}
		var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway; screen.last_presented_sequence = presented; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("widen-flow.json")); root.add_child(screen); await process_frame; screen.set_process(false)
		var before: Array = screen._view.actor("goblin").owned_dice.duplicate(true)
		var offense: Array = screen._view.rolled_dice("goblin").duplicate(true)
		var card := BattleCard.new(); card.definition_id = "widen_the_crack"; card.instance_id = action.payload.card_ids[0]
		screen._on_card_pressed(card); card.free(); await process_frame
		var selector = _selector(screen, TARGET)
		_expect(selector != null, "native play highlights eligible dice")
		if selector == null: screen.queue_free(); await process_frame; break
		var index := int(action.payload.status_id)
		selector.targets["goblin:%d" % index].pressed.emit(); await process_frame
		_expect(screen._error_message.is_empty(), "authority accepts chosen die")
		var notices: Array = _notices(screen)
		_expect(notices.size() == 1, "chosen die creates one roll notice")
		if notices.is_empty(): screen.queue_free(); await process_frame; break
		var notice = notices[0]; notice.set_process(false)
		saw_cursed = saw_cursed or notice.feedback.changes.any(func(change): return change.get("cursed", false))
		while screen._director.has_beats():
			screen._advance_beat(); await process_frame
		var choice = _selector(screen, ADJACENT)
		if choice != null: choice.set_process(false); choice._process(0)
		_expect(choice == null or not choice.visible, "face input hidden during roll")
		notice._elapsed = notice.duration * 0.16; notice.refresh(); await process_frame; notice.refresh()
		_expect(notice._batch_rolls.size() == 1 and notice._batch_rolls[0].visible, "check rolls over actual tray slot")
		_expect(notice._batch_targets.is_empty() and not notice._roll.visible, "roll has no face-placement line or detached preview")
		var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
		_expect(tray._buttons[index].get_global_rect().has_point(notice._batch_rolls[0].get_global_rect().get_center()), "roll belongs to chosen die")
		await _capture("roll", screen)
		notice._elapsed = notice.duration * 0.6; notice.refresh()
		_expect(notice._batch_targets.is_empty(), "settled check still applies no mark")
		_expect(screen._view.rolled_dice("goblin") == offense, "offensive dice unchanged by check")
		for i in 5: _expect(screen._view.actor("goblin").owned_dice[i].cursed_faces == before[i].cursed_faces, "check does not place marks")
		var options: Dictionary = screen._number_face_actions()
		var total := options.size()
		_expect(not notice._waiting(), "Count gain belongs to the roll, without duplicate feedback")
		if total == 0: _expect("No adjacent face available" in notice._label.text, "no-option outcome explained")
		notice._process(notice.duration); await process_frame
		_expect(_notices(screen).is_empty(), "roll completes before choosing adjacent mark")
		if choice != null:
			choice._process(0.2)
			_expect(choice.visible and choice.faces.size() == total, "only eligible chips highlight after roll")
			for face in choice.faces:
				var chip: Control = tray._mark_faces[index][int(face) - 1]
				_expect(choice.faces[face].get_global_rect().is_equal_approx(chip.get_global_rect()), "click target covers exact adjacent chip")
			await _capture("choices-%d" % total, screen)
			var face := int(options.keys()[0])
			if total == 1: choice._process(0.3)
			else:
				choice._process(1.0)
				_expect(screen._view.stage == "curse_choice", "two choices wait for user")
				choice.faces[face].pressed.emit()
			await process_frame
			_expect(screen._error_message.is_empty(), "authority accepts adjacent mark")
			_expect(screen._view.actor("goblin").owned_dice[index].cursed_faces.any(func(value): return int(value) == face), "selected adjacent face is actually marked")
			notices = _notices(screen)
			_expect(notices.size() == 1, "mark application has its own feedback")
			if not notices.is_empty():
				var mark_notice = notices[0]; mark_notice.set_process(false); mark_notice._elapsed = mark_notice.duration * 0.6; mark_notice.refresh(); await process_frame; mark_notice.refresh()
				_expect(mark_notice._batch_targets.size() == 1 and mark_notice._batch_rolls.is_empty(), "purple placement trail appears only for actual mark, with no extra roll")
				await _capture("mark", screen)
		else:
			_expect(screen._view.stage != "curse_choice", "no valid adjacent face continues without a picker")
		covered[total] = true
		screen.active_store.clear(); screen.queue_free(); await process_frame
		if covered.has(1) and covered.has(2) and saw_cursed: break
	_expect(covered.has(1) and covered.has(2), "native cases cover one and two eligible faces")
	_expect(saw_cursed, "native cursed check has no duplicate Count feedback")
	print("WIDEN THE CRACK FLOW: " + ("FAILED" if failed else "PASSED") + " · native options " + str(covered.keys())); quit(1 if failed else 0)
func _action(result: Dictionary, id: String) -> Dictionary:
	for action in result.get("legal_actions", []):
		var ids: Array = action.get("payload", {}).get("card_ids", [])
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances[ids[0]].definition_id == id: return action
	return {}
func _selector(screen, script):
	for child in screen._root.get_children():
		if child.get_script() == script: return child
	return null
func _notices(screen) -> Array:
	return screen.get_children().filter(func(child): return child.get_script() == NOTICE and not child.is_queued_for_deletion())
func _capture(label: String, screen) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_WIDEN_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless" or captured.has(label): return
	captured[label] = true
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		for frame in 8: await process_frame
		for notice in _notices(screen): notice.refresh()
		var choice = _selector(screen, ADJACENT)
		if choice != null and choice.visible: choice.refresh(screen._number_face_actions())
		RenderingServer.force_draw(false); root.get_texture().get_image().save_png(directory.path_join("%s-%d.png" % [label, width]))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("WIDEN FLOW: " + message)
