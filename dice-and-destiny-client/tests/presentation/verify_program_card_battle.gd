extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE = preload("res://presentation/battle/curse_notice.gd")
const SELECTOR = preload("res://presentation/battle/die_face_targeting.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
func _run() -> void:
	var runtime = root.get_node("LearnedBattleRuntime")
	var card: Dictionary = await _create_and_equip(runtime)

	canvas = SubViewport.new(); canvas.size = Vector2i(1920, 1080); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(runtime, "seat-a", "brine-mask-pair", "adventurer"); gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("authored-pointer-battle", 19)
	for action in result.legal_actions:
		if action.type == "planning_roll": result = gateway.submit(JSON.stringify(action)); break
	var id := ""
	for instance in result.snapshot.actors.blade.hand:
		if result.snapshot.actors.blade.card_instances[instance].definition_id == card.id: id = instance; break
	_expect(not id.is_empty(), "authored card in opening hand")
	if id.is_empty(): quit(1); return
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("program-card-battle.json")); canvas.add_child(screen); screen.set_process(false)
	await _settle(screen)
	var hand = screen._hand_dock; hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == id: index = i
	_expect(index >= 0, "card is visible")
	if index < 0: quit(1); return
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	await _click_point(point); await _settle(screen)
	_expect(screen._selected_card.get("instance_id") == id, "pointer click keeps authored card selected in hand")
	_expect(not screen._roll_dock.visible and not screen._action_footer.get_parent().visible, "other actions hidden during card choice")
	# Program die effects target the dice on the board, card-first.
	var selectors: Array = screen._root.get_children().filter(func(node): return node.get_script() == SELECTOR)
	_expect(selectors.size() == 1 and selectors[0].targets.size() == 5, "five actual dice offered on the board")
	if selectors.is_empty() or selectors[0].targets.is_empty(): quit(1); return
	var rolls_before: int = screen._view.rolls_used("blade")
	var target: Control = selectors[0].targets.values()[0]
	await _click_point(target.get_global_rect().get_center())
	for frame in 6: await process_frame
	_expect(id not in screen._view.actor("blade").hand, "only selected card resolves and leaves hand")
	_expect(screen._view.rolls_used("blade") == rolls_before, "custom reroll preserves normal attempts")
	var notices: Array = screen.get_children().filter(func(node): return node.get_script() == NOTICE)
	_expect(notices.size() == 1, "renamed card produces reroll animation")
	if not notices.is_empty():
		var notice = notices[0]; notice.set_process(false); notice._elapsed = notice.duration * 0.45; notice.refresh()
		_expect(notice.feedback.card_instance_id == id, "animation knows exact selected copy")
		_expect(notice._card != null and notice._card.position.y > 700, "animation retains card in hand")
		_expect(not notice._label.visible, "no floating card summary")
		if DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
			canvas.get_texture().get_image().save_png("res://.godot/layout-review/program-card-battle.png")
	screen.queue_free(); await process_frame
	print("PROGRAM CARD BATTLE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _settle(screen: Control) -> void:
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	for frame in 6: await process_frame
func _click_point(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; canvas.push_input(click, true); await process_frame

func _editor_node(ui: Node, key: String) -> Control:
	for node in ui.find_children("*", "Control", true, false):
		if node.get_meta("editor_key", "") == key: return node
	return null
func _root_click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; root.push_input(click, true); await process_frame
	for frame in 5: await process_frame
func _create_and_equip(_runtime: Node) -> Dictionary:
	root.size = Vector2i(1280, 720)
	var characters = load("res://app/screens/character/character_creation.gd").new(); root.add_child(characters)
	for frame in 5: await process_frame
	characters._tabs.current_tab = 3
	for frame in 5: await process_frame
	var ui = characters.get_node("CardCreationWorkspace")
	for i in ui._template.item_count:
		if ui._template.get_item_metadata(i) == "try_again": ui._template.select(i)
	await _root_click(_editor_node(ui, "clone"))
	for edit in [["id", "workshop_reroll"], ["name", "Workshop Reroll"]]:
		var line = _editor_node(ui, edit[0]); line.text = edit[1]; line.text_changed.emit(edit[1])
	ui._validate(); _expect(not ui._publish_button.disabled, "guided reroll definition valid")
	await _root_click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "publish reroll through Card Creation")
	var card: Dictionary = ui.draft.duplicate(true)
	await _root_click(ui._deck_button)
	_expect(characters.selected_id == card.id, "new card selected in deck editor")
	for id in characters._deck_counts(characters._drafts.adventurer).keys(): characters._set_card_count(id, 0)
	characters.inspect_entry("cards", card.id); characters._quantity.value = 6
	characters.inspect_entry("cards", "steady_guard"); characters._quantity.value = 6
	await _root_click(characters._apply)
	_expect(not characters._dirty("adventurer") and characters._card_count(card.id) == 6, "save authored battle deck through character editor")
	characters.queue_free(); await process_frame
	return card
