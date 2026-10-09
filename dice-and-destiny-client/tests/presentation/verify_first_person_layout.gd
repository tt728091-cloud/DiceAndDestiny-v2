extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var base: Dictionary = gateway.start_battle("first-person-layout", 43)
	_expect(base.get("accepted", false), "native Adventurer fixture")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1600, 1000), Vector2i(2560, 1080)]:
		root.size = viewport
		for count in [1, 2, 3, 4]:
			var fixture := base.duplicate(true)
			fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []
			fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
			fixture.pending_input = {"blade": {"id": "layout", "segment": "offensive", "stage": "planning", "allowed_commands": ["planning_reroll", "planning_pass"]}}
			for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
			if count == 3: fixture.snapshot.actors.goblin.definition_id = "drowned_oracle"
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("first-person.json"))
			screen._keep_hand_visible = true
			root.add_child(screen); screen.set_process(false); screen._keep_hand_visible = true
			await _settle()
			screen._hand_dock.keep_visible = true; screen._hand_dock.reveal = 1.0
			var player_zone := Rect2(0, screen.PLAYER_ZONE_TOP, 1920, 1080 - screen.PLAYER_ZONE_TOP)
			var enemy_zone := Rect2(0, 0, 1920, screen.PLAYER_ZONE_TOP)
			var inv: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
			for control in [screen._actor_profiles.blade, screen._roll_dock, screen._action_footer, screen._player_dice_dock, screen._ability_dock.get_parent(), screen._hand_dock]:
				_expect(player_zone.grow(1).encloses(inv * control.get_global_rect()), "player control contained in lower 36%%: %s / %s" % [control.name, inv * control.get_global_rect()])
			var header: Control = screen._root.get_node("PhaseRail")
			var banner: Control = header.get_node("RoundBanner")
			var utilities: Control = screen._root.get_node("BattleUtilities")
			_expect(header.get_child_count() == 1, "round banner is the only phase header")
			_expect(is_equal_approx((inv * banner.get_global_rect()).get_center().x, 960), "round banner stays centered")
			_expect((inv * utilities.get_global_rect()).end.y <= screen.TOP_HUD_BOTTOM, "utility buttons fit compact top header")
			_expect(not utilities.get_global_rect().intersects(banner.get_global_rect()), "utilities do not overlap round banner")
			_expect(absf(utilities.size.y - banner.size.y) <= 12, "utility buttons and banner have similar height")
			_expect(is_equal_approx((inv * screen._actor_profiles.blade.get_global_rect()).end.y, 1060), "player stats use freed bottom space")
			_expect(screen._ability_dock.find_children("*", "BattleAbilityTile", true, false).size() == 6, "all six Adventurer abilities remain")
			for tile in screen._ability_dock.find_children("*", "BattleAbilityTile", true, false):
				_expect(screen._ability_dock.get_parent().get_global_rect().grow(1).encloses(tile.get_global_rect()), "all six base abilities visible without scrolling")
			var fighters := 0
			for fighter in screen._root.get_node("BattleScenery").get_children():
				if not fighter.has_meta("actor_id"): continue
				fighters += 1
				_expect(fighter.get_meta("actor_id") != "blade", "no player portrait")
				_expect(enemy_zone.encloses(inv * fighter.get_global_rect()), "enemy artwork stays in top 64%")
			_expect(fighters == count, "exactly one portrait per enemy")
			if count == 3:
				var boss: Control = screen._actor_profiles.goblin.get_parent().fighter
				var minion: Control = screen._actor_profiles["goblin-2"].get_parent().fighter
				_expect(boss.size.y > minion.size.y and is_equal_approx((inv * boss.get_global_rect()).get_center().x, 960), "large authored enemy occupies the center at larger scale")
			for id in screen._enemy_ids():
				var profile: Control = screen._actor_profiles[id]
				var dock: Control = screen.dice_dock(id)
				_expect(not dock.visible, "enemy dice start hidden")
				var name_bounds := profile.get_global_rect()
				await _click(screen._enemy_buttons[id]); await _settle()
				profile = screen._actor_profiles[id]; dock = screen.dice_dock(id)
				_expect(dock.visible, "name pointer click opens this enemy's dice")
				_expect(profile.get_global_rect().is_equal_approx(name_bounds), "opening dice does not move stats")
				_expect(dock.get_global_rect().end.y <= profile.get_global_rect().position.y, "dice appear above name")
				_expect(enemy_zone.encloses(inv * dock.get_global_rect()), "expanded dice stay in enemy zone")
				screen._render(); await _settle()
				_expect(screen.dice_dock(id).visible, "visibility survives authority redraw")
				await _click(screen._enemy_buttons[id]); await _settle()
				_expect(not screen.dice_dock(id).visible, "second name click hides dice: %s / %d enemies / %s" % [id, count, viewport])
			await _capture("offense-%d-%d" % [count, viewport.x])
			# Two independent attacks from the same enemy, plus one per other foe.
			fixture.snapshot.unified_defense = true
			fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
			fixture.pending_input = {"blade": {"id": "layout", "segment": "defensive", "stage": "defense_selection", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
			var sources := []; var removals := []
			for i in count + 1:
				var attacker := "goblin" if i < 2 else "goblin-" + str(i)
				var id := "incoming-" + str(i)
				sources.append({"id": id, "source_actor_id": attacker, "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 4, "final_amount": 4, "status_applications": [{"target_actor_id": "blade", "status_id": "poison", "stacks": 2}]})
				fixture.legal_actions.append({"type": "planning_select_ability", "actor_id": "blade", "payload": {"pending_input_id": "layout", "ability_id": "guard", "target_ids": [id]}})
				for j in 4: removals.append({"card_id": id + "-" + str(j), "card_definition_id": ["steady_guard", "nudge", "try_again", "take_stock"][j], "target_actor_id": "blade", "original_zone": "deck", "accepted": true, "damage_proposal_ids": [id]})
			fixture.snapshot.damage_sources = sources
			fixture.snapshot.settled_damage = {"id": "layout-cards", "sources": sources, "removals": removals}
			screen._selected_source = ""; screen._view.apply_result(fixture); screen._render(); await _settle()
			_expect(screen._ability_dock.find_children("*", "BattleAbilityTile", true, false).is_empty(), "defense has no offensive ability list before choosing attack")
			_expect(not screen._player_dice_dock.visible, "offensive dice hidden during defense")
			for attack in screen._attack_intents.values():
				_expect(enemy_zone.encloses(inv * attack.intent.get_global_rect()), "each enemy attack stays above player zone")
				_expect(attack.intent_row.get_children().any(func(c): return c.get_meta("intent_effect", "") == "poison"), "attack retains applied status icon")
				for other in screen._attack_intents.values():
					if other != attack: _expect(not attack.intent.get_global_rect().intersects(other.intent.get_global_rect()), "multiple attacks have separate unobstructed boxes")
			for id in ["incoming-0", "incoming-1"]:
				await _click(screen._attack_intents[id].intent); await _settle()
				_expect(screen._selected_source == id, "pointer selects exact attack from same enemy")
				_expect(screen._ability_dock.get_meta("defense_source", "") == id, "defense choices belong to clicked attack")
			for grid in screen._damage_grids:
				grid.started_ms = Time.get_ticks_msec() - 5000; grid.refresh_playback()
				_expect(grid.get_child_count() == 4, "each source keeps its four threatened cards")
			await _capture("defense-%d-%d" % [count, viewport.x])
			# Expanded and collapsed dice work during defense too.
			await _click(screen._enemy_buttons.goblin); await _settle()
			_expect(screen.dice_dock("goblin").visible, "defense name click opens dice: %d enemies / %s" % [count, viewport])
			await _capture("dice-%d-%d" % [count, viewport.x])
			for actor in screen._actor_profiles:
				screen._actor_profiles[actor].statuses.set_counts({"poison": 3, "volatile_poison": 2, "bleed": 1, "blind": 1, "curse_count": 5, "grave_interest": 1, "second_knell": 1, "curse_bloom": 1})
			await _settle()
			for actor in screen._actor_profiles:
				var zone: Rect2 = player_zone if actor == "blade" else enemy_zone
				_expect(zone.encloses(inv * screen._actor_profiles[actor].get_global_rect()), "actor stats remain in their region after phase change")
			screen.queue_free(); await process_frame
	print("FIRST PERSON LAYOUT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _settle() -> void:
	await create_timer(0.75).timeout
	for frame in 8: await process_frame
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true)
	await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true)
		await process_frame
func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_LAYOUT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.1).timeout; RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("FIRST PERSON: " + message)
