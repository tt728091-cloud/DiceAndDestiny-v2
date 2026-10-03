extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var screen: Control
var fixture: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.size = Vector2i(1280, 720)
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	fixture = gateway.start_battle("hand-visibility", 43)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
	await _open()
	canvas.notify_mouse_entered()
	await _move(Vector2(20, 20)); await create_timer(0.4).timeout
	_expect(not screen._keep_hand_visible and not screen._hand_visibility_toggle.button_pressed, "auto-hide remains default")
	_expect(screen._hand_dock.reveal == 0, "hand hides when cursor is away")
	await _click(screen._root.get_node("BattleUtilities").get_children().filter(func(button): return button.get_meta("utility") == "settings")[0])
	await _click(screen._hand_visibility_toggle)
	await create_timer(0.4).timeout
	_expect(screen._keep_hand_visible and screen._hand_dock.reveal == 1, "real checkbox keeps hand raised immediately")
	_expect(not screen._auto_pass_disabled and not screen._auto_pass_toggle.button_pressed, "hand setting does not change auto-pass")
	var saved := ConfigFile.new(); saved.load(WorkspacePaths.persistent_file("battle_preferences.cfg"))
	_expect(saved.get_value("presentation", "keep_hand_visible", false), "preference written to disk")
	for viewport in [Vector2i(1024, 768), Vector2i(1920, 1080)]:
		canvas.size = viewport
		for frame in 6: await process_frame
		_expect(Rect2(Vector2.ZERO, Vector2(viewport)).encloses(screen._hand_visibility_toggle.get_global_rect()), "checkbox fits settings at " + str(viewport))
		var path := OS.get_environment("DICE_AND_DESTINY_FAN_SCREENSHOTS")
		if not path.is_empty() and DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png(path.path_join("hand-setting-%d.png" % viewport.x))
	# Battle redraws reconstruct the hand and settings controls.
	screen._render(); await create_timer(0.4).timeout
	_expect(screen._hand_dock.keep_visible and screen._hand_dock.reveal == 1 and screen._hand_visibility_toggle.button_pressed, "preference survives battle redraw")
	screen.queue_free(); await process_frame
	await _open()
	await _move(Vector2(20, 20)); await create_timer(0.4).timeout
	_expect(screen._hand_visibility_toggle.button_pressed and screen._hand_dock.reveal == 1, "new battle screen reloads preference")
	await _click(screen._root.get_node("BattleUtilities").get_children().filter(func(button): return button.get_meta("utility") == "settings")[0])
	await _click(screen._hand_visibility_toggle); await create_timer(0.4).timeout
	_expect(not screen._keep_hand_visible and screen._hand_dock.reveal == 0, "unchecking restores auto-hide immediately")
	# Gameplay-required visibility remains independent of the preference.
	screen._hand_dock.held_open = true; await create_timer(0.4).timeout
	_expect(screen._hand_dock.reveal == 1, "targeting/discard steps can still hold the hand open")
	screen._hand_dock.held_open = false; await create_timer(0.4).timeout
	_expect(screen._hand_dock.reveal == 0, "hand hides after required visibility ends")
	screen.queue_free(); await process_frame
	await _open()
	_expect(not screen._hand_visibility_toggle.button_pressed, "disabled preference also persists")
	var point: Vector2 = screen._hand_dock.get_global_transform_with_canvas() * Vector2(480, 15)
	await _move(point); await create_timer(0.4).timeout
	_expect(screen._hand_dock.reveal == 1, "hover raises hand in automatic mode")
	await _move(Vector2(20, 20)); await create_timer(0.4).timeout
	_expect(screen._hand_dock.reveal == 0, "leaving hides hand in automatic mode")
	screen.queue_free(); await process_frame
	print("HAND VISIBILITY SETTING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _open() -> void:
	screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hand-setting-test.json"))
	canvas.add_child(screen); screen.set_process(false)
	for frame in 8: await process_frame
func _move(point: Vector2) -> void:
	var event := InputEventMouseMotion.new(); event.position = point; canvas.push_input(event, true); await process_frame
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center(); await _move(point)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("HAND SETTING: " + message)
