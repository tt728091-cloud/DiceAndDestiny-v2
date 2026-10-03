extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for width in [1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width)
	print("STRONG SWING PREPARATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int) -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result := {}; var swing_id := ""
	for seed_value in range(1, 45):
		result = gateway.start_battle("strong-swing-%d-%d" % [width, seed_value], seed_value)
		for action in result.get("legal_actions", []):
			var ids: Array = action.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == "strong_swing": swing_id = ids[0]; break
		if not swing_id.is_empty(): break
	_expect(not swing_id.is_empty(), "native Strong Swing offered before rolling")
	if swing_id.is_empty(): return
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("strong-swing.json")); canvas.add_child(screen); screen.set_process(false)
	for frame in 6: await process_frame
	screen._director.clear()
	var card := BattleCard.new(); card.instance_id = swing_id; card.definition_id = "strong_swing"
	screen._on_card_pressed(card); card.free()
	for frame in 6: await process_frame
	var target = _tile(screen, "adventurer_strike")
	_expect(target != null and not target.disabled, "unrolled Strike is a selectable card target")
	if target == null: screen.queue_free(); return
	await _click(target)
	await create_timer(2.0).timeout
	for frame in 6: await process_frame
	_expect(screen._error_message.is_empty(), "pointer play accepted: " + screen._error_message)
	screen._director.clear(); screen._render()
	for frame in 6: await process_frame
	_check_marker(screen)
	await _capture("strong-swing-before-roll-%d" % width)
	var roll := {}
	for action in screen._view.legal_actions:
		if action.get("type") == "planning_roll": roll = action; break
	_expect(not roll.is_empty(), "normal first roll still available")
	if not roll.is_empty(): result = gateway.submit(JSON.stringify(roll))
	screen._view.apply_result(result); screen._selected_card.clear(); screen._render()
	for frame in 6: await process_frame
	_check_marker(screen)
	for action in result.get("legal_actions", []):
		for id in action.get("payload", {}).get("card_ids", []):
			_expect(result.snapshot.actors.blade.card_instances.get(id, {}).get("definition_id") != "strong_swing", "no Strong Swing actions after first roll")
	await _capture("strong-swing-after-roll-%d" % width)
	# Complete the real offensive segment, including AI turns and reaction passes.
	for step in 40:
		if result.snapshot.segment != "offensive": break
		if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
		var chosen := {}
		for kind in ["planning_select_ability", "planning_pass", "pass"]:
			for action in result.get("legal_actions", []):
				if action.get("type") == kind: chosen = action; break
			if not chosen.is_empty(): break
		if chosen.is_empty(): break
		result = gateway.submit(JSON.stringify(chosen))
	_expect(result.snapshot.segment != "offensive", "native offensive segment completes")
	screen._view.apply_result(result); screen._render(true)
	await create_timer(2.0).timeout
	for frame in 4: await process_frame
	_expect(int(screen._actor_profiles.blade.statuses.counts.get("strong_swing_ready", 0)) == 0 and (not screen._actor_profiles.blade.statuses.cells.has("strong_swing_ready") or screen._actor_profiles.blade.statuses.cells.strong_swing_ready.modulate.a == 0.0), "status expires on offensive exit")
	_expect(screen.find_children("TemporaryDamageBonus", "Label", true, false).is_empty(), "temporary marker expires with status")
	canvas.notify_mouse_exited(); screen.queue_free(); await process_frame
func _tile(screen, id: String):
	for tile in screen.find_children("*", "Button", true, false):
		if tile is BattleAbilityTile and tile.ability_id == id: return tile
	return null
func _check_marker(screen) -> void:
	var tile = _tile(screen, "adventurer_strike")
	_expect(tile != null, "target ability stays on board")
	if tile == null: return
	var badge = tile.get_node_or_null("TemporaryDamageBonus")
	_expect(badge != null and "+2 DMG" in badge.text and "THIS OFFENSE" in badge.text, "persistent temporary +2 label")
	if badge != null: _expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(badge.get_global_rect()), "bonus marker fits viewport")
	_expect("even if unused" in tile.tooltip_text, "ability hover explains expiry")
	var strip = screen._actor_profiles.blade.statuses
	_expect(strip.cells.has("strong_swing_ready") and strip.counts.get("strong_swing_ready") == 1, "positive status appears on character board")
	if strip.cells.has("strong_swing_ready"): _expect("end of the offensive segment" in strip.cells.strong_swing_ready.tooltip_text, "status hover explains exact expiry")
func _click(button: Button) -> void:
	canvas.notify_mouse_entered()
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center(); canvas.push_input(motion, true); await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches the unqualified ability target")
	var click := InputEventMouseButton.new(); click.position = motion.position; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true; canvas.push_input(click, true)
	click = click.duplicate(); click.pressed = false; canvas.push_input(click, true); await process_frame
func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_SWING_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(); canvas.get_texture().get_image().save_png(directory.path_join(name + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("STRONG SWING: " + message)
