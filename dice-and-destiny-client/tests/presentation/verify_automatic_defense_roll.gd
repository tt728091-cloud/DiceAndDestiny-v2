extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1280, 720)
	for choice in ["shedskin", "paid_shedskin", "basic_defense"]:
		var ability: String = "shedskin" if choice == "paid_shedskin" else choice
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "blade_warden" if ability == "basic_defense" else "venom")
		var fixture: Dictionary = gateway.start_battle("automatic-defense-" + choice, 1788900535520209)
		_expect(fixture.get("accepted") == true, "native catalog loads")
		fixture.events = []; fixture.learned_policy = {}
		fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_roll"
		fixture.snapshot.defense_selections = {"blade": {"ability_id": ability, "source_id": "incoming", "catalyst_paid": choice == "paid_shedskin", "rolled_face": 0}}
		fixture.snapshot.damage_sources = [{"id": "incoming", "source_content_id": "sword_cut", "source_actor_id": "goblin", "target_actor_id": "blade", "base_amount": 5}]
		fixture.pending_input = {"blade": {"id": "roll-input", "segment": "defensive", "stage": "defense_roll", "allowed_commands": ["roll_dice"]}}
		var action := {"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "roll_dice", "payload": {"pending_input_id": "roll-input"}}
		fixture.legal_actions = [action]
		var after := fixture.duplicate(true)
		after.snapshot.stage = "defense_reaction"
		after.snapshot.defense_selections.blade.rolled_face = 4
		after.snapshot.defense_selections.blade.rolled_faces = [4, 1] if ability == "shedskin" else [4]
		after.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "review"}}]
		after.pending_input.blade = {"id": "review", "segment": "defensive", "stage": "defense_reaction", "allowed_commands": ["pass"]}
		var next := after.duplicate(true); next.events = []; next.legal_actions = []; next.pending_input = {}; next.snapshot.stage = "damage_reaction"; next.snapshot.segment = "damage_resolution"
		var fake := FakeBattleAuthority.new(); fake.enqueue(after); fake.enqueue(next)
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("automatic-defense.json")); root.add_child(screen)
		for i in 4: await process_frame
		var panels := screen.find_children("*", "VBoxContainer", true, false).filter(func(n): return n.get_script() == preload("res://presentation/battle/defense_roll.gd"))
		_expect(panels.size() == 1 and panels[0].dice.size() == (2 if ability == "shedskin" else 1), "correct defense dice animate")
		_expect(fake.commands.is_empty(), "animation before roll result")
		var path := OS.get_environment("DICE_AND_DESTINY_DEFENSE_ROLL_SCREENSHOT")
		if choice == "shedskin" and not path.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(path)
		await create_timer(0.9).timeout
		_expect(fake.commands.size() == 1 and fake.commands[0] == JSON.stringify(action), "exactly one automatic authority roll despite Disable auto-pass")
		_expect(screen._view.stage == "defense_reaction" and screen._defense_result_panels.size() == 1, "revealed benefits remain visible")
		await create_timer(0.3).timeout
		_expect(fake.commands.size() == 1, "no duplicate roll submission")
		screen._set_auto_pass_disabled(false)
		await create_timer(2.8).timeout
		_expect(screen._view.stage == "defense_reaction", "results remain for at least three seconds")
		await create_timer(0.6).timeout
		_expect(screen._view.stage == "damage_reaction", "normal review then continues")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("AUTOMATIC DEFENSE ROLL: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("AUTOMATIC DEFENSE: " + message)
