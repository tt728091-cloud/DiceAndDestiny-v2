extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const GAIN = preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
var canvas: SubViewport
var active: Control
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.size = Vector2i(1920,1080); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	RenderingServer.frame_pre_draw.connect(_check_frame)
	for width in [1920,1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width)
	print("ATTACK SELECTION STABILITY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int) -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	gateway.unified_defense = true
	var result := {}; var strike := {}
	for seed in range(1,150):
		result = gateway.start_battle("selection-stability-%d-%d" % [width,seed], seed)
		var card := {}
		for action in result.get("legal_actions", []):
			var ids: Array = action.get("payload",{}).get("card_ids",[])
			# Strong Swing is a program card: its start can also choose the ability.
			var raw := str(action.payload.get("status_id", ""))
			var choice = JSON.parse_string(raw) if raw.begins_with("{") else null
			if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0],{}).get("definition_id") == "strong_swing" and choice is Dictionary and choice.get("ability") == "adventurer_strike": card = action; break
		if card.is_empty(): continue
		result = gateway.submit(JSON.stringify(card))
		var roll := {}
		for action in result.legal_actions:
			if action.type == "planning_roll": roll = action; break
		if roll.is_empty(): continue
		result = gateway.submit(JSON.stringify(roll))
		for action in result.legal_actions:
			if action.type == "planning_select_ability" and action.payload.get("ability_id") == "adventurer_strike" and action.payload.get("activation_tier_id", action.payload.get("tier_id")) == "4_swords": strike = action; break
		if not strike.is_empty(): break
	_expect(not strike.is_empty(), "native four-sword Strike after Strong Swing is available")
	if strike.is_empty(): return
	# Establish the already-played card baseline, then observe real selection,
	# opponent decisions, reaction passes, and entry to unified Defense.
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result
	screen._auto_pass_disabled = true; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("selection-stability.json"))
	canvas.add_child(screen); screen.set_process(false); active = screen
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	var target: Button
	for button in screen._ability_dock.find_children("*", "Button", true, false):
		if button.get_meta("tier_id", "") == "4_swords": target = button; break
	_expect(target != null and not target.disabled, "four-sword tier is pointer-selectable")
	if target == null: active = null; screen.queue_free(); await process_frame; return
	await _click(target)
	for frame in 6: await process_frame
	var enemy: Button = screen.find_child("OffensiveTarget_goblin", true, false)
	_expect(enemy != null, "ability waits for an explicit enemy target")
	if enemy != null: await _click(enemy)
	_check_frame()
	await _watch(0.8)
	for step in 35:
		if screen._view.segment == "defensive": break
		if screen._director.has_beats(): screen._advance_beat()
		elif screen._view.learned_policy.get("model_turn", false): screen._apply_model_result(gateway.advance_model())
		else:
			var action := {}
			for entry in screen._view.legal_actions:
				if entry.type in ["pass", "planning_pass"]: action = entry; break
			if action.is_empty(): break
			screen._flow_until = 0; screen._send(JSON.stringify(action))
		_check_frame(); await _watch(0.8)
	_expect(screen._view.segment == "defensive", "real sequence reaches Defense")
	await _watch(1.0)
	var found := false
	for panel in screen._attack_intents.values():
		if panel.attacker_id != "blade": continue
		found = true
		_expect(is_instance_valid(panel.target_heading) and panel.target_heading.is_visible_in_tree(), "outgoing damage appears beneath recipient")
		_expect(panel.damage.text == "7", "five base plus two preparation damage is shown as seven immediately")
		_expect("Strong Swing" in panel.target_heading.tooltip_text and "4 Sword" in panel.target_heading.tooltip_text, "recipient hover preserves chosen tier and bonus provenance")
		var grids: Array = screen._damage_grids.filter(func(grid): return grid.source_id == panel.data.source_id)
		_expect(grids.size() == 1 and grids[0].card_children().size() == 7, "recipient shows seven separate pending cards")
	_expect(found, "outgoing Strike has recipient preview")
	var directory := OS.get_environment("DICE_AND_DESTINY_SELECTION_SCREENSHOTS")
	if not directory.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; canvas.get_texture().get_image().save_png(directory.path_join("attack-selection-%d.png" % width))
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
		if panel.attacker_id == active.viewer_actor_id: _expect(not panel.intent.is_visible_in_tree(), "no floating outgoing attack badge")
	for child in active.get_children():
		if child.get_script() == GAIN: _expect(false, "preparation card must not replay at attack reveal")
	if is_instance_valid(active._flow_transition):
		for key in active._flow_transition.ghosts:
			if str(key).begins_with("ability:") and is_instance_valid(active._flow_transition.ghosts[key].node):
				_expect(not active._flow_transition.ghosts[key].node.is_visible_in_tree(), "ability row never flies out of rail")
func _click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion,true); await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches selected tier")
	for pressed in [true,false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.position = point; click.pressed = pressed; canvas.push_input(click,true)
func _expect(ok: bool, message: String) -> void:
	if not ok and not failed: failed = true; push_error("SELECTION STABILITY: " + message)
