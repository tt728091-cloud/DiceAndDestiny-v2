extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var base := {}; var strike := {}
	for seed in range(1, 150):
		base = gateway.start_battle("target-selection-%d" % seed, seed)
		for action in base.legal_actions:
			if action.type == "planning_roll": base = gateway.submit(JSON.stringify(action)); break
		for action in base.legal_actions:
			if action.type == "planning_select_ability" and action.payload.ability_id == "guarded_strike": strike = action; break
		if not strike.is_empty(): break
	_expect(not strike.is_empty(), "native Guarded Strike available")
	if strike.is_empty(): quit(1); return
	base.events = []; base.learned_policy = {}
	for width in [1920, 1280, 1024]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for count in [1, 2, 4]:
			var fixture := base.duplicate(true)
			if count == 1:
				fixture.snapshot.actors.erase("goblin-2")
				fixture.legal_actions = fixture.legal_actions.filter(func(a): return "goblin-2" not in a.get("payload", {}).get("target_ids", []))
			if count == 4:
				for id in ["goblin-3", "goblin-4"]:
					fixture.snapshot.actors[id] = fixture.snapshot.actors.goblin.duplicate(true)
					var action := strike.duplicate(true); action.payload.target_ids = [id]; fixture.legal_actions.append(action)
			var fake := FakeBattleAuthority.new()
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
			screen._auto_pass_disabled = true; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("targeting.json"))
			canvas.add_child(screen); screen.set_process(false)
			await _settle(screen)
			await _click_ability(screen)
			_expect(fake.commands.is_empty(), "ability click never submits against focused enemy")
			var targets := screen.find_children("OffensiveTarget_*", "Button", true, false)
			_expect(targets.size() == count, "every eligible enemy has a target cue")
			if not targets.is_empty():
				var alpha: float = targets[0]._outline.border_color.a
				await create_timer(0.3).timeout
				_expect(absf(alpha - targets[0]._outline.border_color.a) > 0.03, "highlight visibly pulses")
				for target in targets:
					_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(target.get_global_rect()), "portrait target fits viewport")
			# Repeat the same ability to cancel, then enter target choice again.
			await _click_ability(screen)
			_expect(screen._pending_attack.is_empty() and fake.commands.is_empty(), "cancel spends nothing")
			await _click_ability(screen)
			var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; canvas.push_input(escape, true)
			await _settle(screen)
			_expect(screen._pending_attack.is_empty() and fake.commands.is_empty(), "Escape cancels target selection")
			await _click_ability(screen)
			if count == 2 and DisplayServer.get_name() != "headless":
				await RenderingServer.frame_post_draw
				canvas.get_texture().get_image().save_png("res://.godot/layout-review/offensive-targeting-%d.png" % width)
			var chosen := "goblin-2" if count == 2 else "goblin"
			fake.enqueue(fixture)
			var target: Button = screen._enemy_buttons[chosen] if count == 4 else screen.find_child("OffensiveTarget_" + chosen, true, false)
			if target != null: await _click(target)
			_expect(fake.commands.size() == 1, "enemy click submits exactly once")
			if fake.commands.size() == 1:
				var command: Dictionary = JSON.parse_string(fake.commands[0])
				_expect(command.payload.ability_id == "guarded_strike" and command.payload.target_ids == [chosen], "exact selected target and ability preserved")
			await _settle(screen)
			_expect(screen._pending_attack.is_empty(), "selection clears after command")
			# A stale UI target cannot bypass legal actions or a defeated actor.
			screen._view.apply_result(fixture); screen._render(true); await _settle(screen)
			await _click_ability(screen)
			screen._view.actors.goblin.defeat_state = "defeated"
			screen._choose_offensive_target("goblin")
			_expect(fake.commands.size() == 1, "defeated target is rejected")
			screen._view.legal_actions = []
			screen._render(true); await _settle(screen)
			_expect(screen._pending_attack.is_empty(), "stale target choice clears on refresh")
			screen.queue_free(); await process_frame
	print("OFFENSIVE TARGETING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _settle(screen: Control) -> void:
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
func _click_ability(screen: Control) -> void:
	for button in screen._ability_dock.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.ability.blade.guarded_strike":
			await _click(button); await _settle(screen); return
	_expect(false, "ability button available")
func _click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches " + str(button.name))
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true)
		await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("OFFENSIVE TARGETING: " + message)
