extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1024, 720)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new("user://verify_history_autoscroll_active.json"); store.clear()
	var battle_id := "history-autoscroll-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start history autoscroll battle"): return
	for index in 24:
		var marked := gateway.mark_dev_history(battle_id, "blade", "%02d · A deliberately wide recorded history action" % (index + 1), "decision", 0, {}, {"type": "autoscroll_test", "index": index})
		if not _accepted(marked, "record history action %d" % (index + 1)): return

	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = started; screen.gateway = gateway; screen.active_store = store
	root.add_child(screen); await process_frame; await process_frame; await process_frame
	var scroll := _history_scroll(screen)
	if scroll == null or _latest(scroll) <= 0 or scroll.scroll_horizontal < _latest(scroll) - 4:
		_fail("history did not initially follow its latest point: value=%d latest=%d" % [-1 if scroll == null else scroll.scroll_horizontal, -1 if scroll == null else _latest(scroll)])
		return

	# Manually inspecting older history pauses follow mode and survives render.
	scroll.scroll_horizontal = 0; await process_frame
	var refresh := _button(screen, "↻")
	if refresh == null: _fail("history refresh control was unavailable"); return
	refresh.pressed.emit(); await process_frame; await process_frame; await process_frame
	scroll = _history_scroll(screen)
	if scroll == null or scroll.scroll_horizontal > 4:
		_fail("manual history scroll position was not preserved")
		return

	# A successful gameplay action deliberately resumes following the newest point.
	var continue_action := _button(screen, "Continue Presentation")
	if continue_action == null: _fail("player action was unavailable for follow-latest validation"); return
	continue_action.pressed.emit(); await process_frame; await process_frame; await process_frame
	scroll = _history_scroll(screen)
	if scroll == null or scroll.scroll_horizontal < _latest(scroll) - 4:
		_fail("a successful player action did not move history to the newest point")
		return

	# Returning to the right edge re-enables follow mode for newly added points.
	scroll.scroll_horizontal = 1_000_000; await process_frame
	var added := gateway.mark_dev_history(battle_id, "blade", "25 · Newest recorded history action", "decision", 0, {}, {"type": "autoscroll_test", "index": 24})
	if not _accepted(added, "append newest history action"): return
	refresh = _button(screen, "↻"); refresh.pressed.emit(); await process_frame; await process_frame; await process_frame
	scroll = _history_scroll(screen)
	if scroll == null or scroll.scroll_horizontal < _latest(scroll) - 4:
		_fail("history did not resume following the newest point")
		return

	# Opening an older checkpoint carries the current viewport into the new review
	# screen and leaves the clicked checkpoint selected instead of jumping to the tail.
	scroll.scroll_horizontal = 0; await process_frame
	var history_points: Array = screen.inspection_state().get("history_points", [])
	if history_points.is_empty(): _fail("history points disappeared before review viewport validation"); return
	var selected_id := str(history_points[0].get("id", ""))
	var selected := _history_button(screen, selected_id)
	if selected == null: _fail("older history point was unavailable for viewport validation"); return
	selected.pressed.emit(); await process_frame; await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var review: Node = screens[-1]
	var review_scroll := _history_scroll(review)
	var review_selected := _history_button(review, selected_id)
	if review_scroll == null or review_scroll.scroll_horizontal > 4 or review_selected == null or not review_selected.button_pressed:
		_fail("opening an older checkpoint lost its viewport or selected highlight")
		return

	store.clear(); review.queue_free(); await process_frame
	print("HISTORY AUTOSCROLL: actions followed latest while review jumps preserved their viewport and selection")
	quit(0)

func _history_scroll(node: Node) -> ScrollContainer:
	for child in node.find_children("*", "ScrollContainer", true, false):
		if child.has_meta("inspection_id") and child.get_meta("inspection_id") == "battle.history.scroll": return child
	return null

func _latest(scroll: ScrollContainer) -> int:
	var bar := scroll.get_h_scroll_bar()
	return maxi(0, int(bar.max_value - bar.page))

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _history_button(node: Node, point_id: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.has_meta("inspection_id") and str(child.get_meta("inspection_id")) == "battle.history.point.%s" % point_id: return child
	return null

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)])
	return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
