extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	for character in ["venom", "blade_warden"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", character)
		var fixture: Dictionary = gateway.start_battle("defense-review-" + character, 1788900535520209)
		_expect(fixture.get("accepted") == true, "native character catalog loads")
		fixture.events = []; fixture.learned_policy = {}
		fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
		var ability := "shedskin" if character == "venom" else "basic_defense"
		fixture.snapshot.damage_sources = [{"id": "incoming", "source_content_id": "sword_cut", "target_actor_id": "blade", "base_amount": 6}]
		fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": ability, "source_id": "incoming", "rolled_face": 3, "rolled_faces": [3, 2] if character == "venom" else [3]}}
		fixture.pending_input = {"blade": {"id": "review", "segment": "defensive", "stage": "defense_reaction", "allowed_commands": ["pass"]}}
		fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "review"}}]
		var next := fixture.duplicate(true)
		next.snapshot.segment = "damage_resolution"; next.snapshot.stage = "damage_reaction"
		next.pending_input.blade.id = "damage"; next.pending_input.blade.segment = "damage_resolution"; next.pending_input.blade.stage = "damage_reaction"
		next.legal_actions = []
		var fake := FakeBattleAuthority.new(); fake.enqueue(next)
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-review.json"))
		root.add_child(screen)
		for frame in 4: await process_frame
		var dice := 0
		for button in screen.find_children("*", "Button", true, false):
			if str(button.get_meta("inspection_id", "")).begins_with("battle.defense_die.blade"): dice += 1
		_expect(dice == (2 if character == "venom" else 1), "defensive results show all " + character + " dice")
		for button in screen.find_children("*", "Button", true, false):
			if button.get_meta("inspection_id", "") == "battle.utility.settings": _click(button)
		await process_frame
		var toggle: CheckBox = screen._auto_pass_toggle
		_expect(root.get_visible_rect().encloses(toggle.get_global_rect()), "debug toggle stays in viewport")
		_click(toggle)
		await process_frame
		_expect(screen._auto_pass_disabled, "pointer click disables auto-pass")
		await create_timer(3.4).timeout
		_expect(fake.commands.is_empty() and screen._view.stage == "defense_reaction", "debug mode holds defensive results indefinitely")
		var capture := OS.get_environment("DICE_AND_DESTINY_DEFENSE_REVIEW_SCREENSHOT")
		if character == "venom" and not capture.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(capture)
		_click(toggle)
		await process_frame
		await create_timer(2.8).timeout
		_expect(fake.commands.is_empty() and screen._view.stage == "defense_reaction", "results stay visible for at least three seconds after resuming")
		await create_timer(0.6).timeout
		_expect(fake.commands.size() == 1 and screen._view.stage == "damage_reaction", "automatically advances to damage after review")
		screen.active_store.clear(); screen.queue_free()
		await process_frame
	print("DEFENSE REVIEW: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _click(button: BaseButton) -> void:
	var press := InputEventMouseButton.new(); press.position = button.get_global_rect().get_center(); press.button_index = MOUSE_BUTTON_LEFT; press.pressed = true; root.push_input(press, true)
	var release := press.duplicate() as InputEventMouseButton; release.pressed = false; root.push_input(release, true)

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("DEFENSE REVIEW: " + message)
