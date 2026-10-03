extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("hud-tooltips", 43)
	for width in [1920, 1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for count in [1, 4]:
			for stage in ["planning", "defense_selection"]:
				var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
				fixture.snapshot.segment = "offensive" if stage == "planning" else "defensive"; fixture.snapshot.stage = stage
				for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				fixture.snapshot.damage_sources = []
				for id in fixture.snapshot.actors:
					fixture.snapshot.actors[id].statuses = [{"definition_id": "curse_count", "stacks": 4}]
					if stage == "defense_selection" and id != "blade":
						fixture.snapshot.damage_sources.append({"id": id + "-attack", "source_actor_id": id, "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 4})
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hud-tooltips.json")); canvas.add_child(screen); screen.set_process(false)
				for frame in 8: await process_frame
				canvas.notify_mouse_entered()
				for id in fixture.snapshot.actors:
					var profile: ActorProfile = screen._actor_profiles[id]
					var cells := {"status": profile.statuses.cells.curse_count}
					for key in ["deck", "hand", "discard", "removed"]: cells[key] = profile._stat_cells[key]
					for key in cells:
						var cell: Control = cells[key]
						for point in [cell.get_global_rect().position + Vector2(6, 8), cell.get_global_rect().end - Vector2(6, 8)]:
							await _move(point)
							_expect(canvas.gui_get_hovered_control() == cell, "%s %s icon and count receive hover: %s/%d/%d" % [id, key, stage, count, width])
						if key == "status": _expect(BattlePresentationCatalog.status("curse_count").text in cell.tooltip_text, "status contains pinned rules")
						else: _expect(key.capitalize() in cell.tooltip_text and str(int(profile._display_values[key])) in cell.tooltip_text, "pile identifies name and displayed count")
						if width == 1280 and count == 1 and stage == "planning": await _popup(cell, id + "-" + key)
				# A rebuilt board must retain the live hit targets.
				screen._render(); await process_frame; await process_frame
				await _move(screen._actor_profiles.goblin.statuses.cells.curse_count.get_global_rect().get_center())
				_expect(canvas.gui_get_hovered_control() == screen._actor_profiles.goblin.statuses.cells.curse_count, "redraw preserves status hover")
				canvas.notify_mouse_exited(); screen.queue_free(); await process_frame
	print("ACTOR HUD TOOLTIPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _move(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
func _popup(cell: Control, suffix: String) -> void:
	await _move(Vector2(10, 10)); await _move(cell.get_global_rect().get_center())
	await create_timer(0.8).timeout
	var label := _find_label(canvas, cell.tooltip_text)
	_expect(label != null, "real tooltip opens for " + suffix)
	if label != null:
		var window := label.get_window()
		_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(window.position), Vector2(window.size))), "tooltip stays on screen " + suffix)
	var dir := OS.get_environment("DICE_AND_DESTINY_HUD_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(); canvas.get_texture().get_image().save_png(dir.path_join("hud-hover-" + suffix + ".png"))
	await _move(Vector2(10, 10)); await process_frame
func _find_label(node: Node, value: String) -> Label:
	if node is Label and node.text == value and node.is_visible_in_tree(): return node
	for child in node.get_children(true):
		var found := _find_label(child, value)
		if found != null: return found
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("HUD HOVER: " + message)
