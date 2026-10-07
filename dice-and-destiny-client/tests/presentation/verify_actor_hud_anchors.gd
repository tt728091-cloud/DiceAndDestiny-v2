extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const GAIN := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "curse")
	var base: Dictionary = gateway.start_battle("actor-hud-anchors", 3)
	_expect(base.get("accepted", false), "native fixture")
	for id in base.snapshot.content_catalog.statuses:
		_expect(ICONS.SHAPES.has(id), "catalog status has a distinct symbol: " + str(id))
	base.events = []; base.learned_policy = {}; base.legal_actions = []; base.pending_input = {}
	base.snapshot.segment = "offensive"; base.snapshot.stage = "planning"
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1600, 1000), Vector2i(2560, 1080)]:
		root.size = viewport
		for enemies in [1, 2, 3, 4, 5]:
			var fixture := base.duplicate(true)
			fixture.snapshot.actors.erase("goblin-2")
			for i in range(2, enemies + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
			for id in fixture.snapshot.actors:
				fixture.snapshot.actors[id].statuses = [{"definition_id": "curse_count", "stacks": 2}, {"definition_id": "grave_interest", "stacks": 3}]
				fixture.snapshot.actors[id].owned_dice = []
				for die in 5: fixture.snapshot.actors[id].owned_dice.append({"index": die, "cursed_faces": [1, 3], "entombed": die == 2})
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hud-test.json"))
			root.add_child(screen); screen.set_process(false)
			for frame in 6: await process_frame
			_expect(screen._actor_profiles.size() == enemies + 1, "every actor has an independent panel")
			for id in screen._actor_profiles:
				var profile: ActorProfile = screen._actor_profiles[id]
				var bounds := profile.get_global_rect()
				if id != "blade":
					var dock: Control = screen.dice_dock(id)
					dock.expanded = true
					for frame in 3: await process_frame
					_expect(dock.get_parent() == screen._root, "dice have an independent fold-out dock")
					_expect(dock.get_global_rect().end.y <= bounds.position.y, "dice sit above the enemy name")
					_expect(root.get_visible_rect().encloses(dock.get_global_rect()), "enemy dice fit viewport")
					_expect(not dock.get_global_rect().intersects(screen._hand_dock.get_global_rect()), "enemy dice leave room for expanded hand")
					var tray: BattleDiceTray = dock.get_child(0)
					_expect(tray._buttons[0].size == BattleDiceTray.HUD_DIE_SIZE and tray._mark_faces[0][0].get_theme_font_size("font_size") >= 12, "shared larger dice retain readable curse numerals")
					var matching_player_tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
					_expect(tray._buttons[0].get_global_rect().size.is_equal_approx(matching_player_tray._buttons[0].get_global_rect().size), "enemy and player dice have identical rendered sizes")
					for other in screen._enemy_dice_docks:
						if other != id: _expect(not dock.get_global_rect().intersects(screen.dice_dock(other).get_global_rect()), "enemies keep distinct dice footprints")
				_expect(root.get_visible_rect().encloses(bounds), "HUD fits " + str(viewport))
				_expect(not bounds.intersects(screen._hand_dock.get_global_rect()), "HUD does not cover hand")
				_expect((screen._root.get_global_transform_with_canvas().affine_inverse() * bounds).position.y >= screen.PLAYER_ZONE_TOP if id == "blade" else (screen._root.get_global_transform_with_canvas().affine_inverse() * bounds).end.y <= screen.PLAYER_ZONE_TOP, "HUD stays in its owner zone")
				for other in screen._actor_profiles:
					if id != other: _expect(not bounds.intersects(screen._actor_profiles[other].get_global_rect()), "actor HUDs do not overlap: %s %s / %s %s" % [id, bounds, other, screen._actor_profiles[other].get_global_rect()])
				for kind in ["health", "energy", "deck", "hand", "discard", "removed"]:
					_expect(bounds.encloses(screen.actor_anchor_rect(id, kind)), "live " + kind + " anchor inside owner")
				for status in ["curse_count", "grave_interest"]:
					var anchor: Rect2 = screen.actor_anchor_rect(id, "status", status)
					_expect(bounds.encloses(anchor), "exact status icon inside owner")
					_expect(profile.statuses.cells[status].get_node("Count").text.is_valid_int(), "visible status has only its count")
					_expect(not profile.statuses.cells[status].tooltip_text.is_empty(), "status rules on hover")
				_expect(screen.actor_anchor_rect(id, "status", "curse_count") != screen.actor_anchor_rect(id, "status", "grave_interest"), "distinct status endpoints")
			var player_profile: ActorProfile = screen._actor_profiles.blade
			var player_bounds := player_profile.get_global_rect()
			for controls in [screen._roll_dock, screen._action_footer, screen._player_dice_dock, screen._ability_dock]:
				_expect(not player_bounds.intersects(controls.get_global_rect()), "player HUD clears expanded cursed dice and controls")
			var player_tray: BattleDiceTray = screen._player_dice_dock.get_child(0)
			var player_ground: Vector2 = player_profile.get_parent().global_position
			player_tray.display_owned([])
			for frame in 6: await process_frame
			_expect(player_profile.get_global_rect().is_equal_approx(player_bounds), "clearing curse grids does not move the player HUD")
			player_tray.display_owned(fixture.snapshot.actors.blade.owned_dice)
			for frame in 6: await process_frame
			_expect(player_profile.get_parent().global_position.is_equal_approx(player_ground), "expanding curse grids does not move the player")
			if enemies <= 4: await _capture("hud-%d-%dx%d" % [enemies, viewport.x, viewport.y])
			# Movement, scaling, and a fresh root must all resolve the same actor ID.
			var profile: ActorProfile = screen._actor_profiles.goblin
			var fighter: Control = profile.get_parent().fighter
			var before := profile.status_anchor("curse_count")
			var dice_before: Vector2 = screen.dice_dock("goblin").global_position
			fighter.position += Vector2(-35, -25)
			for frame in 2: await process_frame
			var expected: Vector2 = Vector2(-35, -25) * screen._root.scale
			_expect(profile.status_anchor("curse_count").is_equal_approx(before + expected), "status follows fighter movement")
			_expect(screen.dice_dock("goblin").global_position.is_equal_approx(dice_before + expected), "dice and face endpoints follow fighter movement")
			var dock = profile.get_parent(); before = profile.status_anchor("curse_count"); dock.attachment_offset += Vector2(8, 6)
			for frame in 2: await process_frame
			_expect(profile.status_anchor("curse_count").is_equal_approx(before + Vector2(8, 6) * screen._root.scale), "status follows independent HUD offset")
			# Off-focus cards must still have a destination, and survive a redraw.
			var target := "goblin-2" if enemies > 1 else "goblin"
			var update := {"kind": "application", "data": {"target_actor_id": target, "status_id": "curse_count", "before": 1, "after": 2}, "card_feedback": {"key": "hud", "cards": [{"definition_id": "maledictions_refusal", "instance_id": "test", "actor_id": "blade"}]}}
			var notice = GAIN.new(); var updates: Array[Dictionary] = [update]; notice.configure(screen, updates); screen.add_child(notice); notice.set_process(false)
			notice._started = true; notice._elapsed = 0.9; notice.refresh()
			_expect(notice._destinations.size() == 1, "card finds off-focus actor")
			_expect((notice.get_global_transform_with_canvas() * notice._destinations[0]).is_equal_approx(screen._actor_profiles[target].status_anchor("curse_count")), "card trail hits precise status")
			screen._render(); for frame in 5: await process_frame
			notice.refresh()
			_expect((notice.get_global_transform_with_canvas() * notice._destinations[0]).is_equal_approx(screen._actor_profiles[target].status_anchor("curse_count")), "card reacquires anchor after rebuild")
			# Energy and all four pile endpoints share the same live resolver.
			var recipient: ActorProfile = screen._actor_profiles[target]
			notice.updates.clear()
			for stat in ["energy", "deck", "hand", "discard", "removed"]:
				var amount := int(recipient._display_values[stat])
				notice.updates.append({"kind": "resource", "data": {"target_actor_id": target, "stat": stat, "before": amount - 1, "after": amount, "amount": 1}})
			recipient.get_parent().attachment_offset += Vector2(-10, -8)
			for frame in 2: await process_frame
			notice.refresh()
			for i in notice.updates.size():
				var stat: String = notice.updates[i].data.stat
				_expect((notice.get_global_transform_with_canvas() * notice._destinations[i]).is_equal_approx(recipient.anchor_rect(stat).get_center()), "moving " + stat + " receives its own trail")
			if enemies == 3 and viewport == Vector2i(1920, 1080):
				root.size = Vector2i(1280, 720)
				for frame in 4: await process_frame
				notice.refresh()
				for i in notice.updates.size():
					_expect((notice.get_global_transform_with_canvas() * notice._destinations[i]).is_equal_approx(recipient.anchor_rect(str(notice.updates[i].data.stat)).get_center()), "in-flight resize preserves endpoint")
				root.size = viewport
				for frame in 4: await process_frame
			notice.queue_free()
			var curse = preload("res://presentation/battle/curse_notice.gd").new()
			curse.configure(screen, {"card_id": "", "title": "Curse", "changes": [
				{"actor_id": target, "index": 0, "face": 1, "rolled": false, "marked": true},
				{"actor_id": target, "index": 1, "face": 3, "rolled": false, "marked": true}]})
			screen.add_child(curse); curse.set_process(false); curse._elapsed = curse.duration * 0.6
			curse.refresh(); await process_frame; curse.refresh()
			_expect(curse._batch_targets.size() == 2, "off-focus enemy receives both curse trails")
			var target_tray: BattleDiceTray = screen.dice_dock(target).get_child(0)
			for i in 2:
				var endpoint: Vector2 = curse.get_global_transform_with_canvas() * curse._batch_targets[i].get_center()
				_expect(endpoint.is_equal_approx(target_tray._mark_faces[i][0 if i == 0 else 2].get_global_rect().get_center()), "curse trail hits its owner's exact compact face")
			if enemies == 3 and viewport == Vector2i(1920, 1080): await _capture("off-focus-curse-trails")
			curse.queue_free()
			# Pending gains and heavy status combinations must remain compact.
			profile = screen._actor_profiles.blade
			profile.statuses.set_counts({"poison": 3, "volatile_poison": 2, "bleed": 1, "blind": 1, "curse_count": 5, "grave_interest": 1, "second_knell": 1, "curse_bloom": 1})
			profile.show_pending_applications({"poison": 3})
			for frame in 3: await process_frame
			_expect(profile.pending_statuses.cells.poison.get_node("Count").text == "+3", "pending addition remains separate from actual stacks")
			_expect(not profile.get_global_rect().intersects(screen._hand_dock.get_global_rect()), "pending status fits above hand")
			screen.queue_free(); await process_frame
	for id in ICONS.SHAPES:
		_expect(ICONS.texture(id) != null and ICONS.texture(id).get_width() == 48, "vector symbol renders: " + str(id))
	print("ACTOR HUD ANCHORS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _capture(id: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_HUD_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.4).timeout
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ACTOR HUD: " + message)
