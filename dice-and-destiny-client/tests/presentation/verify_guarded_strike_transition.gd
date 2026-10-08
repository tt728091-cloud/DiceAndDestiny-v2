extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var active: Control
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.size = Vector2i(1920,1080); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	RenderingServer.frame_pre_draw.connect(_check_frame)
	RenderingServer.frame_pre_draw.connect(_check_badges)
	# Headless runs emit no draw signals; process_frame observes the prior frame's drawn layout.
	process_frame.connect(_check_enemy_dice)
	for width in [1920,1280,1024]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width)
	print("GUARDED STRIKE TRANSITION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int) -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result := {}; var strike := {}
	for seed in range(1,150):
		result = gateway.start_battle("guarded-strike-transition-%d-%d" % [width,seed], seed)
		var roll := {}
		for action in result.legal_actions:
			if action.type == "planning_roll": roll = action; break
		if roll.is_empty(): continue
		result = gateway.submit(JSON.stringify(roll))
		for action in result.legal_actions:
			if action.type == "planning_select_ability" and action.payload.get("ability_id") == "guarded_strike": strike = action; break
		if not strike.is_empty(): break
	_expect(not strike.is_empty(), "native Guarded Strike is available")
	if strike.is_empty(): return
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result
	screen.learned_battle_mode = true; screen._auto_pass_disabled = false; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("guarded-strike-transition.json"))
	canvas.add_child(screen); active = screen
	for frame in 6: await process_frame
	# Unfolded enemy dice must stay anchored through every threaded model rebuild.
	for id in screen._enemy_ids(): screen._enemy_dice_visible[id] = true
	screen._render(true)
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	var target: Button
	for button in screen._ability_dock.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.ability.blade.guarded_strike": target = button; break
	_expect(target != null and not target.disabled, "Guarded Strike is pointer-selectable")
	if target == null: active = null; screen.queue_free(); await process_frame; return
	await _click(target)
	for frame in 6: await process_frame
	var enemy: Button = screen.find_child("OffensiveTarget_goblin", true, false)
	_expect(enemy != null, "ability waits for an explicit enemy target")
	if enemy != null: await _click(enemy)
	_check_frame()
	await _watch(0.8)
	# Exercise the live threaded model handoff. Applying results directly before
	# node processing misses the late-frame rebuild that caused the flash.
	var deadline := Time.get_ticks_msec() + 45000
	while screen._view.segment != "defensive" and Time.get_ticks_msec() < deadline:
		await process_frame
		if screen._model_thinking or screen._submitting or screen._director.has_beats() or Time.get_ticks_msec() < screen._flow_until: continue
		if screen._view.learned_policy.get("model_turn", false): continue
		for action in screen._view.legal_actions:
			if action.type == "pass":
				screen._send(JSON.stringify(action)); break
	_expect(screen._view.segment == "defensive", "real sequence reaches Defense")
	await _watch(1.0)
	var found := false
	for panel in screen._attack_intents.values():
		if panel.attacker_id != "blade": continue
		found = true
		_expect(is_instance_valid(panel.target_heading) and panel.target_heading.is_visible_in_tree(), "outgoing damage appears beneath recipient")
		_expect(panel.damage.text == "5", "Guarded Strike shows five damage")
		_expect("Guarded Strike" in panel.target_heading.tooltip_text, "recipient hover preserves ability rules")
		var grids: Array = screen._damage_grids.filter(func(grid): return grid.source_id == panel.data.source_id)
		_expect(grids.size() == 1 and grids[0].card_children().size() == 5, "recipient shows five separate pending cards")
	_expect(found, "outgoing Guarded Strike has recipient preview")
	var incoming := 0
	for panel in screen._attack_intents.values():
		if panel.attacker_id == "blade": continue
		incoming += 1
		_expect(panel.intent.is_visible_in_tree(), "enemy attack badge remains visible at its anchor")
	_expect(incoming == 2, "both enemy attacks survive the handoff")
	var directory := OS.get_environment("DICE_AND_DESTINY_SELECTION_SCREENSHOTS")
	if not directory.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; canvas.get_texture().get_image().save_png(directory.path_join("guarded-strike-transition-%d.png" % width))
	active = null; screen.active_store.clear(); screen.queue_free(); await process_frame
func _watch(seconds: float) -> void:
	var end := Time.get_ticks_msec() + int(seconds * 1000)
	while Time.get_ticks_msec() < end:
		await process_frame; _check_frame()
func _check_frame() -> void:
	if not is_instance_valid(active): return
	var inverse: Transform2D = active._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * active._player_profile_dock.get_global_rect()
	_expect(rect.position.y >= active.PLAYER_ZONE_TOP and absf(rect.end.y - 1060) < 2, "player HUD stays at bottom on every frame")
	for panel in active._attack_intents.values():
		_expect(active._actor_profiles.has(panel.attacker_id), "attack source belongs to displayed actor: " + str(panel.source))
		if panel.attacker_id == active.viewer_actor_id: _expect(not panel.intent.is_visible_in_tree(), "no floating outgoing attack badge")
	if is_instance_valid(active._flow_transition):
		for key in active._flow_transition.ghosts:
			if str(key).begins_with("ability:") and is_instance_valid(active._flow_transition.ghosts[key].node):
				_expect(not active._flow_transition.ghosts[key].node.is_visible_in_tree(), "ability row never flies out of rail")
func _check_enemy_dice() -> void:
	if not is_instance_valid(active) or not is_instance_valid(active._root): return
	var inverse: Transform2D = active._root.get_global_transform_with_canvas().affine_inverse()
	for id in active._enemy_dice_docks:
		var dock: Control = active._enemy_dice_docks[id]
		if not is_instance_valid(dock) or not dock.is_visible_in_tree() or not is_instance_valid(dock.profile_dock): continue
		var rect: Rect2 = inverse * dock.get_global_rect()
		var profile: Rect2 = inverse * dock.profile_dock.get_global_rect()
		_expect(absf(rect.end.y + 8 - profile.position.y) < 2 and absf(rect.get_center().x - profile.get_center().x) < 2, "enemy dice stay above their profile on every frame: %s %s vs %s" % [id, rect, profile])
func _click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion,true); await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches Guarded Strike")
	for pressed in [true,false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.position = point; click.pressed = pressed; canvas.push_input(click,true)
func _expect(ok: bool, message: String) -> void:
	if not ok and not failed: failed = true; push_error("GUARDED STRIKE TRANSITION: " + message)

func _check_badges() -> void:
	if not is_instance_valid(active): return
	var inverse: Transform2D = active._root.get_global_transform_with_canvas().affine_inverse()
	for badge in active.find_children("AttackIntent_*", "Button", true, false):
		if not badge.is_visible_in_tree() or badge.modulate.a < 0.01: continue
		var badge_rect: Rect2 = inverse * badge.get_global_rect()
		_expect(badge_rect.position.y >= active.TOP_HUD_BOTTOM - 1, "no attack badge at top-left: " + badge.name + " " + str(badge_rect) + " stage " + active._view.stage + " beat " + str(active._director.peek()))
