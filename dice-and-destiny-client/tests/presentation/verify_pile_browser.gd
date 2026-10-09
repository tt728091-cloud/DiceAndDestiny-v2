extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var base: Dictionary = gateway.start_battle("pile-browser", 43)
	_expect(base.get("accepted", false), "native starter battle loads")
	for width in [1920, 1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for enemies in [1, 4]:
			for stage in ["planning", "damage_reaction"]:
				var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
				fixture.snapshot.segment = "offensive" if stage == "planning" else "damage_resolution"; fixture.snapshot.stage = stage
				for i in range(2, enemies + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				var owner: Dictionary = fixture.snapshot.actors.blade
				owner.deck_composition = {"steady_guard": 2, "strong_swing": 1}; owner.deck_count = 3
				owner.discard_composition = {"nudge": 2}; owner.discard_count = 2
				owner.removed_composition = {"try_again": 1}; owner.removed_count = 1
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("pile-browser.json")); canvas.add_child(screen); screen.set_process(false)
				for frame in 12: await process_frame
				canvas.notify_mouse_entered()
				for actor_id in fixture.snapshot.actors:
					for zone in ["deck", "hand", "discard", "removed"]:
						var cell: Control = screen._actor_profiles[actor_id]._stat_cells[zone]
						_expect(zone.capitalize() in cell.tooltip_text, "all player/enemy piles keep explanatory hover text")
						await _click(cell.get_global_rect().get_center())
						if actor_id != "blade" or zone == "hand":
							_expect(not is_instance_valid(screen._pile_browser), "enemy piles and hand do not open a browser")
							continue
						_expect(is_instance_valid(screen._pile_browser), "player pile opens from real pointer click")
						if not is_instance_valid(screen._pile_browser): continue
						var browser = screen._pile_browser
						for frame in 4: await process_frame
						_expect(browser.rows.get_child_count() == int(owner[zone + "_count"]), "one row per card including duplicate copies")
						var expected: Dictionary = owner[zone + "_composition"].duplicate()
						for row in browser.rows.get_children():
							expected[row.get_meta("definition_id")] -= 1
							_expect(row.size.y <= 40, "inventory uses compact headers")
						_expect(expected.values().all(func(count): return count == 0), "list exactly matches its authority composition")
						var row: Control = browser.rows.get_child(0)
						await _move(row.get_global_rect().get_center())
						_expect(is_instance_valid(browser.preview) and browser.preview.definition_id == row.get_meta("definition_id"), "row hover shows matching full card")
						_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(browser.panel.get_global_rect()), "panel stays inside viewport")
						if is_instance_valid(browser.preview):
							_expect(browser.panel.get_global_rect().encloses(browser.preview.get_global_rect()), "full preview stays inside panel")
							_expect(browser.preview.get_global_rect().position.x > browser.scroll.get_global_rect().end.x, "pile preview stays to the right of every list row")
						if zone == "deck": await _capture("pile-browser-%d-%d-%s" % [width, enemies, stage])
						await _click(browser.close_button.get_global_rect().get_center())
						_expect(screen._open_pile.is_empty() and not is_instance_valid(screen._pile_browser), "Close returns to battle")
				# Large piles scroll, tabs switch categories, Escape and outside clicks dismiss.
				screen._view.actors.blade.discard_composition = {"steady_guard": 30}; screen._view.actors.blade.discard_count = 30
				await _click(screen._actor_profiles.blade._stat_cells.deck.get_global_rect().get_center())
				var browser = screen._pile_browser
				await _click(browser.tabs.discard.get_global_rect().get_center())
				_expect(screen._open_pile == "discard" and browser.rows.get_child_count() == 30, "tabs switch to current pile")
				browser.scroll.scroll_vertical = 99999
				for frame in 3: await process_frame
				var last: Control = browser.rows.get_child(-1)
				_expect(browser.scroll.get_global_rect().intersects(last.get_global_rect()), "scroll reaches final card")
				await _move(last.get_global_rect().get_center())
				_expect(is_instance_valid(browser.preview), "scrolled row supports full preview")
				# Board rebuilds use fresh compositions rather than stale opened lists.
				screen._view.actors.blade.discard_composition = {}; screen._view.actors.blade.discard_count = 0
				screen._render()
				for frame in 12: await process_frame
				browser = screen._pile_browser
				_expect(browser.zone == "discard" and browser.rows.get_child(0).text == "This pile is empty.", "refresh retains category and displays newly empty pile")
				var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; canvas.push_input(escape, true); await process_frame
				_expect(screen._open_pile.is_empty(), "Escape closes browser")
				await _click(screen._actor_profiles.blade._stat_cells.removed.get_global_rect().get_center())
				await _click(Vector2(10, 10))
				_expect(screen._open_pile.is_empty(), "outside click closes without playing a card")
				canvas.notify_mouse_exited(); screen.queue_free(); await process_frame
	print("PILE BROWSER: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _move(point: Vector2) -> void:
	var event := InputEventMouseMotion.new(); event.position = point; canvas.push_input(event, true); await process_frame
func _click(point: Vector2) -> void:
	await _move(point)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true); await process_frame
func _capture(filename: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_PILE_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(); canvas.get_texture().get_image().save_png(directory.path_join(filename + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("PILE BROWSER: " + message)
