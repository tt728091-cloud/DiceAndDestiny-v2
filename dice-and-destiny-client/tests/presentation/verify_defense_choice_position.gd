extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas)
	for character in ["curse", "venom", "blade_warden"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", character)
		var base: Dictionary = gateway.start_battle("defense-position-" + character, 43)
		for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
			canvas.size = viewport
			for count in [1, 2, 3, 4]:
				var fixture := base.duplicate(true)
				fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []
				fixture.snapshot.stage = "defense_selection"; fixture.snapshot.segment = "defensive"
				fixture.pending_input = {"blade": {"id": "choose", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
				fixture.snapshot.damage_sources = []
				for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				for actor in fixture.snapshot.actors:
					if actor == "blade": continue
					fixture.snapshot.damage_sources.append({"id": "attack-" + actor, "source_actor_id": actor, "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 6})
					for ability in fixture.snapshot.actors.blade.defensive_abilities:
						fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "choose", "ability_id": ability, "target_ids": ["attack-" + actor]}})
				var fake := FakeBattleAuthority.new()
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-position.json"))
				canvas.add_child(screen); screen.set_process(false)
				for frame in 6: await process_frame
				var target := "attack-goblin" if count == 1 else "attack-goblin-" + str(count)
				await _click(screen._attack_intents[target].intent)
				for frame in 6: await process_frame
				var rail: Control = screen._ability_dock.get_parent()
				var bounds := rail.get_global_rect()
				var intent: Rect2 = screen._attack_intents[target].intent.get_global_rect()
				_expect(screen._selected_source == target, "intent click selects exact attacker")
				_expect(bounds.end.x < intent.position.x and intent.position.x - bounds.end.x < 20, "choices are immediately left of selected intent")
				_expect(Rect2(Vector2.ZERO, Vector2(viewport)).encloses(bounds), "choices fit the viewport")
				var tiles := []
				for button in screen._ability_dock.find_children("*", "Button", true, false):
					if button is BattleAbilityTile: tiles.append(button)
				_expect(tiles.size() == fixture.snapshot.actors.blade.defensive_abilities.size(), "every defensive ability appears for " + character)
				for tile in tiles: _expect(bounds.grow(1).encloses(tile.get_global_rect()), "full defense tile visible for " + character)
				# Follow the selected actor without rebuilding the choices.
				var panel = screen._attack_intents[target]
				var fighter: Control = screen._actor_profiles[panel.attacker_id].get_parent().fighter
				var old_x := rail.global_position.x
				fighter.position.x -= 24; panel._update()
				_expect(is_equal_approx(rail.global_position.x, old_x - 24 * screen._root.scale.x), "defenses follow live actor movement")
				fighter.position.x += 24; panel._update()
				await _capture(character, count, viewport.x)
				var next := fixture.duplicate(true); next.snapshot.stage = "defense_reaction"; next.pending_input = {}; next.legal_actions = []
				fake.enqueue(next)
				if not tiles.is_empty():
					var choice: Button = tiles[0]
					if choice.disabled:
						for button in choice.find_children("*", "Button", true, false):
							if button.has_meta("ability_action") and not button.disabled: choice = button; break
					for frame in 3: await process_frame
					await _click(choice)
					_expect(fake.commands.size() == 1, "nearby defense tile accepts the click for %s / %d enemies / %d pixels (commands: %s)" % [character, count, viewport.x, str(fake.commands)])
					if fake.commands.size() == 1: _expect(JSON.parse_string(fake.commands[0]).payload.target_ids == [target], "defense keeps the selected attack identity")
				screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DEFENSE CHOICE POSITION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true)
		await process_frame
func _capture(character: String, count: int, width: int) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_DEFENSE_POSITION_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless" or character != "curse": return
	RenderingServer.force_draw(false)
	canvas.get_texture().get_image().save_png(directory.path_join("defense-choices-%d-%d.png" % [count, width]))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DEFENSE CHOICE POSITION: " + message)
