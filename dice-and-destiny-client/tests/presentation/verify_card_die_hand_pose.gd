extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
const SELECTOR := preload("res://presentation/battle/die_face_targeting.gd")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for width in [1024, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width)
	print("CARD DIE HAND POSE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int) -> void:
	canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result := {}; var ids: Array = []
	for seed_value in range(1, 120):
		result = gateway.start_battle("card-die-pose-%d-%d" % [width, seed_value], seed_value)
		for action in result.get("legal_actions", []):
			if action.type == "planning_roll": result = gateway.submit(JSON.stringify(action)); break
		ids.clear()
		for id in result.snapshot.actors.blade.hand:
			if result.snapshot.actors.blade.card_instances[id].definition_id == "try_again": ids.append(id)
		if ids.size() >= 2: break
	_expect(ids.size() >= 2, "two identical Try Again cards in native hand")
	if ids.size() < 2: return
	var id: String = ids[1]
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("card-die-pose.json")); canvas.add_child(screen); screen.set_process(false)
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for entry in screen._timed_buttons: entry.until = 0
	screen._process(0)
	var rolls_before: int = screen._view.rolls_used("blade")
	await _click_hand(screen, id)
	for frame in 6: await process_frame
	var selectors: Array = screen._root.get_children().filter(func(node): return node.get_script() == SELECTOR)
	_expect(selectors.size() == 1, "pointer card click exposes die targets")
	if selectors.is_empty(): screen.queue_free(); return
	var target: Control = selectors[0].targets["blade:0"]
	var motion := InputEventMouseMotion.new(); motion.position = target.get_global_rect().get_center(); canvas.push_input(motion, true)
	for frame in 6: await process_frame
	var selected: BattleCard
	for card in screen._hand_dock.cards:
		if card.instance_id == id: selected = card
	var pose: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse() * selected.get_global_transform_with_canvas()
	var card_size := selected.size
	await _click(target)
	var notices: Array = screen.get_children().filter(func(node): return node.get_script() == NOTICE)
	_expect(notices.size() == 1, "one native card-to-die notice")
	if notices.is_empty(): screen.queue_free(); return
	var notice = notices[0]; notice.set_process(false); screen._director.clear()
	_expect(notice.feedback.card_instance_id == id and notice._card.instance_id == id, "exact played copy owns captured pose")
	_expect(not id in screen._view.actor("blade").hand and ids[0] in screen._view.actor("blade").hand, "only selected copy is spent")
	_expect(screen._view.rolls_used("blade") == rolls_before, "reroll consumes no normal attempt")
	for frame in 4: await process_frame
	for frame in 55:
		notice._elapsed = notice.duration * frame / 56.0; notice.refresh()
		_expect(notice._card.position.distance_to(pose.origin) < 1 and notice._card.size == card_size, "card never leaves hand pose")
		_expect(is_equal_approx(notice._card.rotation, pose.get_rotation()) and notice._card.scale.is_equal_approx(pose.get_scale()), "rotation and scale retained")
		_expect(not notice._label.visible, "no detached summary")
		_expect(screen._hand_dock.gain_animation_active, "hand stays open through feedback")
		_expect(notice._target.size.x > 0, "actual die is anchored throughout reroll")
		var points: PackedVector2Array = notice._card_arc(notice._target.get_center(), 1.0)
		_expect(points[0].distance_to(pose * (card_size * 0.5)) < 1, "arc starts at selected hand card")
		_expect(points[-1].distance_to(notice._target.get_center()) < 1, "arc lands on selected die")
		if frame == 25:
			await _capture("try-again-hand-arc-%d" % width)
		if frame == 34:
			_expect(absf(notice._reroll_tray._buttons[0].rotation) > 0.001, "actual die tumbles after arc")
		if frame == 45:
			screen._render(); await process_frame; notice.refresh()
	_expect(notice._reroll_tray._numbers[0].text == str(notice.feedback.changes[0].face), "reroll settles on authority result")
	# Shared presenter: every card definition must use a supplied pose, and
	# missing private/replay poses must never resurrect the old floating card.
	for definition in BattlePresentationCatalog._catalog.cards:
		var data: Dictionary = notice.feedback.duplicate(true); data.card_id = definition
		var preview = NOTICE.new(); preview.configure(screen, data, {id: {"transform": pose, "size": card_size}}); screen.add_child(preview); preview.set_process(false)
		preview._elapsed = 0.5; preview.refresh()
		_expect(preview._card.position.distance_to(pose.origin) < 1, definition + " uses shared hand pose")
		preview.queue_free(); await process_frame
		preview = NOTICE.new(); preview.configure(screen, data); screen.add_child(preview); preview.set_process(false)
		_expect(preview._card == null, definition + " never falls back to floating card")
		preview.queue_free(); await process_frame
	notice.queue_free(); await process_frame
	_expect(not screen._card_gain_active(), "feedback releases input on completion")
	canvas.notify_mouse_exited(); screen.queue_free(); await process_frame
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true)
	await process_frame
func _capture(label: String) -> void:
	if DisplayServer.get_name() == "headless": return
	await process_frame; RenderingServer.force_draw()
	canvas.get_texture().get_image().save_png("res://.godot/layout-review/" + label + ".png")
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CARD DIE POSE: " + message)

func _click_hand(screen, id: String) -> void:
	var hand = screen._hand_dock
	hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == id: index = i
	_expect(index >= 0, "selected card exists in live hand")
	if index < 0: return
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; canvas.push_input(click, true)
	await process_frame
	_expect(screen._selected_card.get("instance_id") == id, "real hand click starts targeting")
