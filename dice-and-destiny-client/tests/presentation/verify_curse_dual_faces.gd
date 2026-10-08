extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var checked := {}
	var curse_fixture: Dictionary
	for character in ["venom", "curse"]:
		var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", character)
		var fixture: Dictionary = native.start_battle("dual-faces-" + character, 1788900535520209)
		_expect(fixture.get("accepted", false), "native character catalog loads")
		if character == "curse": curse_fixture = fixture
		BattlePresentationCatalog.configure(fixture.snapshot.content_catalog)
		var tray := BattleDiceTray.new(); root.add_child(tray)
		for die_id in fixture.snapshot.content_catalog.dice:
			checked[die_id] = true
			for face in range(1, 7):
				for cursed in [false, true]:
					tray.display([{"die_id": die_id, "face": face}], [0], true)
					tray.display_owned([{"index": 0, "cursed_faces": [float(face)] if cursed else [float(face % 6 + 1)]}])
					_expect(tray._curse_symbols[0].visible == cursed, "negative symbol matches current face, not just the die")
					_expect(tray._buttons[0].modulate == Color.WHITE, "main die retains normal colors")
					_expect(tray._numbers[0].visible and tray._numbers[0].text == str(face), "number survives double-symbol layout")
					_expect(tray._normal_symbols[0].visible, "original positive symbol stays visible")
					_expect(not StoneDie.glyph(BattlePresentationCatalog.symbol_id_for_die_face(die_id, face)).is_empty(), "shipped symbols have a carved stone glyph")
					var glyph := BattlePresentationCatalog.symbol_for_die_face(die_id, face)
					_expect(tray._buttons[0].text.begins_with(glyph) and tray._buttons[0].text.ends_with("\n%d" % face), "accessible die text preserves normal symbol and number")
					_expect(tray._buttons[0].button_pressed, "kept state survives mark refresh")
					if cursed:
						var normal: Control = tray._normal_symbols[0]
						_expect(not normal.get_rect().intersects(tray._curse_symbols[0].get_rect()), "positive and negative symbols do not overlap")
						_expect("Curse symbol" in tray._buttons[0].tooltip_text, "tooltip explains the negative symbol")
					# A different animated face must not retain a stale Curse symbol.
					tray._show_face(0, face % 6 + 1)
					_expect(tray._curse_symbols[0].visible == not cursed, "transient roll faces update the Curse symbol")
					tray.animate_roll([0], Time.get_ticks_msec() - 600, 450)
					_expect(tray._curse_symbols[0].visible == cursed, "animation settles to authoritative face and mark")
		tray.display([], [], false)
		tray.display_owned([{"cursed_faces": [1, 2, 3, 4, 5, 6]}])
		_expect(tray._buttons[0].text == "—" and not tray._curse_symbols[0].visible and not tray._normal_symbols[0].visible, "hidden roll reveals no face symbols")
		tray.display([{"die_id": "standard_d6", "face": 1}])
		tray.display_owned([])
		_expect(not tray._curse_symbols[0].visible, "clearing marks removes negative symbol")
		tray.queue_free(); await process_frame
	for id in ["standard_d6", "venom_d6", "curse_d6", "brine_d6"]: _expect(checked.has(id), "tested die family " + id)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		var f := curse_fixture.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
		f.snapshot.stage = "planning"; f.snapshot.segment = "offensive"
		var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("dual-faces.json")); root.add_child(screen)
		var owned: Array = []; var dice: Array = []
		for i in 5:
			owned.append({"index": i, "cursed_faces": [1.0]})
			dice.append({"index": i, "die_id": "brine_d6", "face": [4, 3, 6, 1, 3][i]})
		var enemy: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
		enemy.display(dice, [], false, "OPPONENT DICE · Rolls 3"); enemy.display_owned(owned)
		for frame in 10: await process_frame
		for i in 5:
			_expect(enemy._curse_symbols[i].visible == (i == 3), "only screenshot's face-1 result shows both symbols")
			if i == 3: _expect(enemy._buttons[i].get_global_rect().encloses(enemy._curse_symbols[i].get_global_rect()), "Curse icon fits die at both viewport sizes")
		var dir := OS.get_environment("DICE_AND_DESTINY_CURSE_DUAL_SCREENSHOTS")
		if not dir.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join("curse-dual-faces-%d.png" % viewport.x))
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("CURSE DUAL FACES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSE DUAL FACES: " + message)
