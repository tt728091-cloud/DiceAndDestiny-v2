extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var active: Control
var pass_rect := Rect2()
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	RenderingServer.frame_pre_draw.connect(_check_controls)
	for width in [1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for after_roll in [false, true]: await _scenario(width, after_roll)
	print("PROTECT CHOICES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int, after_roll: bool) -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result := {}
	var found := false
	for seed_value in range(1, 80):
		result = gateway.start_battle("protect-choices-%d-%d" % [width, seed_value], seed_value)
		var rolls: Array = result.legal_actions.filter(func(action): return action.type == "planning_roll")
		if rolls.is_empty(): continue
		result = gateway.submit(JSON.stringify(rolls[0]))
		var choices: Array = result.legal_actions.filter(func(action): return action.type == "planning_select_ability" and action.payload.get("ability_id") == "guarded_strike")
		if choices.is_empty(): continue
		result = gateway.submit(JSON.stringify(choices[0])); found = true; break
	_expect(found, "native Guarded Strike grants Protect")
	if not found: return
	for step in 100:
		if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
		if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
		else:
			var actions: Array = result.legal_actions.filter(func(action): return action.type == "pass")
			if actions.is_empty(): _expect(false, "fixture reaches defense: " + str(result)); return
			result = gateway.submit(JSON.stringify(actions[0]))
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("protect-choices.json"))
	canvas.add_child(screen)
	await create_timer(8).timeout
	await _settle()
	active = screen; pass_rect = Rect2(); _check_controls()
	_expect(screen._actor_profiles.blade.statuses.counts.get("protect") == 2, "earned Protect remains visible")
	var sources: Array = screen._view.damage_sources.filter(func(source): return source.target_actor_id == "blade")
	_expect(sources.size() == 2, "two incoming attacks available")
	var target := str(sources[-1].id)
	await _click(screen._incoming_attack_rows[target]); await _settle()
	var protect: Button = screen._ability_dock.get_node_or_null("ProtectChoice")
	_expect(protect != null, "Protect shares the dice defense popup")
	var guard: Button
	for button in screen._ability_dock.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.ability.blade.adventurer_guard": guard = button
	_expect(guard != null and not guard.disabled, "Guard remains available alongside Protect")
	if guard == null: active = null; screen.queue_free(); await process_frame; return
	var deadline := 0
	if after_roll:
		await _click(guard)
		deadline = Time.get_ticks_msec() + 22000
		while Time.get_ticks_msec() < deadline and not screen._view.raw_snapshot.get("defense_history", {}).has(target):
			await process_frame; _check_controls()
		_expect(screen._view.stage == "defense_selection", "roll finishes automatically without Apply")
		# Let saved-card feedback settle before selecting the still-protected source.
		await create_timer(3).timeout
		_expect(not screen._source_protection_action(target).is_empty(), "Protect remains legal after the rolled defense")
		await _click(screen._attack_intents[target].intent); await _settle()
	protect = screen._ability_dock.get_node_or_null("ProtectChoice")
	deadline = Time.get_ticks_msec() + 10000
	while is_instance_valid(protect) and protect.disabled and Time.get_ticks_msec() < deadline: await process_frame
	_expect(protect != null and not protect.disabled, "handled attack can still spend Protect")
	if protect != null:
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(false)
			canvas.get_texture().get_image().save_png("res://.godot/layout-review/protect-choices-%d.png" % width)
		var before := {}
		for source in screen._view.damage_sources: before[source.id] = int(source.get("final_amount", source.base_amount))
		await _click(protect)
		_expect(screen.get_children().all(func(child): return not child is AcceptDialog), "Protect starts immediately without another target dialog")
		_expect(screen._damage_feedback.get("source_id") == target and not screen._damage_feedback.get("saved", []).is_empty(), "selected source immediately starts prevention and saved-card feedback")
		await _settle()
		var feedback: Control
		for node in screen._root.get_children():
			if node.get_script() == preload("res://presentation/battle/damage_response_feedback.gd"): feedback = node
		_expect(feedback != null, "prevention animation is mounted directly after Protect click")
		_expect(screen._source_protection_action(target).is_empty(), "consumption removes repeat-use action")
		_expect(int(screen._actor_profiles.blade.statuses.counts.get("protect", 0)) == 0, "consumed status disappears from player HUD")
		for source in screen._view.damage_sources:
			_expect(int(source.get("final_amount", source.base_amount)) == (maxi(0, before[source.id] - 2) if source.id == target else before[source.id]), "only chosen source receives protection")
		screen._spend_source_protection(target)
		_expect(screen._error_message.is_empty(), "stale choice cannot submit a second spend")
	active = null; screen.active_store.clear(); screen.queue_free(); await process_frame
func _check_controls() -> void:
	if not is_instance_valid(active): return
	_expect(active.get_children().all(func(child): return not child is AcceptDialog), "no target modal anywhere in Protect sequence")
	for button in active._action_footer.get_children():
		if not button is Button: continue
		_expect("Apply" not in button.text and "Protect" not in button.text and "protection" not in button.text, "footer has no Apply or Protect")
		var rect: Rect2 = active._root.get_global_transform_with_canvas().affine_inverse() * button.get_global_rect()
		# Ignore unsorted controls before the first render; check every pre-draw.
		if rect.size.x <= 0: continue
		_expect(rect.position.x >= 370 and rect.end.x <= 434 and absf(rect.position.y - 716) < 2, "Pass stays in upper-right: " + str(rect))
		if pass_rect == Rect2(): pass_rect = rect
		else: _expect(rect.position.distance_to(pass_rect.position) < 2, "Pass never shifts during defense")
func _settle() -> void:
	for frame in 8: await process_frame
func _click(button: Button) -> void:
	if not is_instance_valid(button): _expect(false, "pointer target exists"); return
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center(); canvas.push_input(motion,true)
	await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches " + button.name)
	for pressed in [true,false]:
		var event := InputEventMouseButton.new(); event.button_index = MOUSE_BUTTON_LEFT; event.position = motion.position; event.pressed = pressed; canvas.push_input(event,true)
	await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok:
		push_error("PROTECT CHOICES: " + message)
		failed = true
