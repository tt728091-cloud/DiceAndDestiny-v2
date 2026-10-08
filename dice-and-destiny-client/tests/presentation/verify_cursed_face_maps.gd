extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "curse")
	var base: Dictionary = native.start_battle("curse-face-maps", 1788900535520209)
	_expect(base.get("accepted", false), "native catalog starts")
	BattlePresentationCatalog.configure(base.snapshot.content_catalog)
	await _all_faces()
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []
		f.snapshot.stage = "planning"; f.snapshot.segment = "offensive"
		f.pending_input = {"blade": {"id": "map-input", "stage": "planning", "segment": "offensive", "allowed_commands": ["planning_keep", "planning_reroll", "planning_select_ability"]}}
		var sets := [[], [1], [1, 3], [2, 4, 6], [1, 2, 3, 4, 5, 6]]
		for owner in ["blade", "goblin"]:
			var actor: Dictionary = f.snapshot.actors[owner]; actor.owned_dice = []
			for i in 5: actor.owned_dice.append({"index": i, "cursed_faces": sets[i], "entombed": i == 3})
			actor.dice = []
			if owner == "blade":
				for i in 5: actor.dice.append({"index": i, "face": 1 if i < 3 else 4, "die_id": "curse_d6"})
				actor.roll_history = [{"dice": actor.dice}]
				actor.qualified_abilities = ["hexbrand"]
		# JSON round trips reproduce the authority's floating-point number format.
		f = JSON.parse_string(JSON.stringify(f))
		var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("curse-face-maps.json")); screen._auto_pass_disabled = true
		root.add_child(screen)
		screen._enemy_dice_visible["goblin"] = true
		screen._enemy_dice_dock.expanded = true
		for frame in 10: await process_frame
		for dock in [screen._player_dice_dock, screen._enemy_dice_dock]:
			var tray: BattleDiceTray = dock.get_child(0)
			_expect(tray._mark_legend.visible == not tray.compact_row, "compact row keeps legend in tooltips; full tray shows legend")
			for i in 5:
				_expect(tray._mark_labels[i].text == ("D%d %d/6" if tray.hud_compact else "D%d · %d/6") % [i + 1, sets[i].size()], "die identity and exact face count visible")
				_expect(root.get_visible_rect().encloses(tray._mark_panels[i].get_global_rect()), "face map fits viewport")
				for face in 6:
					var chip: Label = tray._mark_faces[i][face]
					_expect(chip.get_meta("cursed") == (face + 1 in sets[i]), "only actual cursed faces highlighted")
					_expect(tray._mark_panels[i].get_global_rect().encloses(chip.get_global_rect()), "all six face chips fit beneath their die")
					_expect(chip.text == str(face + 1), "integer face numbers without decimal noise")
				_expect(tray._buttons[i].disabled if dock == screen._enemy_dice_dock else true, "enemy face maps do not enable dice")
			if dock == screen._enemy_dice_dock:
				for button in tray._buttons: _expect(button.text == "—", "map reveals persistent marks without revealing hidden rolls")
		var dice_rect: Rect2 = screen._player_dice_dock.get_global_rect()
		var roll_rect: Rect2 = screen._roll_dock.get_global_rect()
		var rail_rect: Rect2 = screen._ability_dock.get_parent().get_global_rect()
		# battle-hud-anchors.md: Roll/Skip stack to the right of the player dice at the same top.
		_expect(roll_rect.position.x >= dice_rect.end.x and not roll_rect.intersects(dice_rect) and roll_rect.end.y <= rail_rect.position.y and dice_rect.end.y <= rail_rect.position.y, "maps, roll controls and abilities never overlap: dice %s roll %s rail %s" % [dice_rect, roll_rect, rail_rect])
		_expect(screen._enemy_dice_dock.get_global_rect().end.y <= screen._actor_profiles[screen._focused_enemy].get_global_rect().position.y, "enemy dice and maps unfold above their HUD")
		_expect(not screen._enemy_dice_dock.get_global_rect().intersects(screen._hand_dock.get_global_rect()), "enemy curse maps stay above the hand")
		var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
		_expect(tray._bindings[3].visible and tray._bound_labels[3].text == "BOUND", "loaded Entombment has persistent frame and named badge")
		_expect(tray._bindings[3].started_ms == -1, "loaded binding is settled")
		screen._view.actors.goblin.owned_dice[2].entombed = true
		screen._render(); await process_frame
		tray = screen._enemy_dice_dock.get_child(0)
		var started: int = tray._bindings[2].started_ms
		_expect(started >= 0 and tray._bindings[2].visible, "new binding animates on exact affected die")
		_expect(not tray._bindings[0].visible and not tray._bindings[1].visible and not tray._bindings[4].visible, "unaffected dice remain unbound")
		screen._render(); await process_frame
		tray = screen._enemy_dice_dock.get_child(0)
		_expect(tray._bindings[2].started_ms == started, "redraw preserves application time")
		for frame in 10: await process_frame
		await _capture(viewport.x)
		screen._view.actors.goblin.owned_dice[2].entombed = false
		screen._render(); await process_frame
		tray = screen._enemy_dice_dock.get_child(0)
		_expect(not tray._bindings[2].visible and not tray._bound_labels[2].visible and tray._bindings[3].visible, "release removes only that die's binding")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("CURSED FACE MAPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _all_faces() -> void:
	var tray := BattleDiceTray.new(); root.add_child(tray)
	tray.display([{"die_id": "curse_d6", "face": 3}], [], true)
	for mask in 64:
		var faces: Array = []
		for bit in 6:
			if mask & (1 << bit): faces.append(float(bit + 1))
		tray.display_owned([{"index": 0, "cursed_faces": faces}])
		_expect(tray._mark_labels[0].text == ("D1 · %d/6" % faces.size() if mask else "D1 · clean"), "all 64 face combinations counted")
		for bit in 6: _expect(tray._mark_faces[0][bit].get_meta("cursed") == (bool(mask & (1 << bit))), "all 64 face combinations mapped")
	tray.display_owned([{"cursed_faces": [3.0, 1.0, 3.0], "entombed": true}])
	_expect(tray._buttons[0].disabled and tray._bound_labels[0].visible, "Entombment is distinct from curse coverage")
	_expect(tray._bindings[0].visible and tray._bound_labels[0].text == "ENTOMBED", "binding frame and explicit badge visible")
	_expect("Cursed faces: 1, 3" in tray._buttons[0].tooltip_text and "2 of 6" in tray._buttons[0].tooltip_text, "sorted distinct integer faces in tooltip")
	var tooltip := tray._buttons[0].tooltip_text
	tray.display_owned(tray._owned)
	_expect(tray._buttons[0].tooltip_text == tooltip, "redraw does not duplicate tooltip text")
	tray.animate_roll([0], Time.get_ticks_msec(), 450)
	_expect(tray._mark_labels[0].text == "D1 · 2/6" and tray._mark_faces[0][2].get_meta("cursed"), "map remains stable throughout roll animation")
	tray._roll_started_ms -= 500; tray._update_roll()
	_expect(tray._buttons[0].disabled and tray._bound_labels[0].visible, "animation retains bound state")
	_expect(tray._bindings[0].visible and tray._numbers[0].text == "3", "binding persists through rolls without obscuring result")
	tray.display_owned([{"cursed_faces": [1, 3], "entombed": false}])
	_expect(not tray._buttons[0].disabled and not tray._bound_labels[0].visible, "released die regains normal selection")
	_expect(not tray._bindings[0].visible, "release clears frame")
	tray.display_owned([{"cursed_faces": null}])
	_expect(not tray._mark_legend.visible and tray._buttons[0].modulate == Color.WHITE, "cleanse resets map and tint")
	tray.display_owned([])
	for panel in tray._mark_panels: _expect(not panel.visible, "missing owned data clears stale marks")
	tray.queue_free(); await process_frame
func _capture(width: int) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_CURSE_MAP_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join("curse-face-maps-%d.png" % width))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSED FACE MAPS: " + message)
