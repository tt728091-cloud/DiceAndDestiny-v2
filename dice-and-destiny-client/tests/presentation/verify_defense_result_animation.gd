extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _fixture(catalyst: int = 0) -> Dictionary:
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
	fixture.snapshot.actors.blade.statuses = [] if catalyst == 0 else [{"definition_id": "catalyst", "stacks": catalyst}]
	fixture.snapshot.actors.blade.offensive_outcome = {"base_damage": 3, "status_applications": [{"target_actor_id": "goblin", "status_id": "poison", "stacks": 3}]}
	fixture.snapshot.damage_sources = [{"id": "enemy-attack", "source_actor_id": "goblin", "source_content_id": "venom_strike", "target_actor_id": "blade", "base_amount": 3}, {"id": "player-attack", "source_actor_id": "blade", "source_content_id": "needlefang", "target_actor_id": "goblin", "base_amount": 3}]
	fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "shedskin", "source_id": "enemy-attack", "rolled_face": 1, "rolled_faces": [1, 4]}, "goblin": {"actor_id": "goblin", "ability_id": "basic_defense", "source_id": "player-attack", "rolled_face": 6, "rolled_faces": [6]}}
	fixture.pending_input = {"blade": {"id": "defense-results", "segment": "defensive", "stage": "defense_reaction", "allowed_commands": ["pass"]}}
	fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "defense-results"}}]
	return fixture

func _screen(fixture: Dictionary):
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-animation.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	return screen

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("defense-animation", 1788900535520209)
	_expect(base.get("accepted") == true, "native content loads")
	await _verify_roll_handoff()
	await _verify_unified_sequence()
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_roll_seconds", 0.2)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_effects_seconds", 1.8)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_hold_seconds", 1.0)
	for size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = size
		var screen = _screen(_fixture(1))
		for frame in 4: await process_frame
		_expect(screen._defense_result_panels.size() == 2, "both defenders have compact results")
		var player = screen._defense_result_panels[0]; var enemy = screen._defense_result_panels[1]
		_expect(player.data.before == 3 and player.data.after == 2 and player.data.prevented == 1, "Fang prevents one")
		_expect(enemy.data.before == 3 and enemy.data.after == 0 and enemy.data.prevented == 6, "large block displays six but damage stops at zero")
		_expect("Poison ×3 pending" in enemy.data.attack_statuses, "blocking damage preserves attack Poison")
		_expect(player.damage.text == "3", "damage starts at incoming value")
		for label in screen._center.find_children("*", "Label", true, false):
			_expect(not label.text.contains("Roll 2 Venom dice") and not label.text.contains("Select 1 source"), "rules paragraphs removed from results")
		await create_timer(1.1).timeout
		var flights: Array = screen._root.find_children("*", "Control", false, false)
		var saw_flight := false
		for flight in flights:
			if str(flight.get_meta("inspection_id", "")).begins_with("battle.defense_status_flight."):
				saw_flight = true
				# Sample a known midpoint: window resize/rendering can otherwise
				# consume the entire short flight on a busy graphical test run.
				flight.started_ms = Time.get_ticks_msec() - 1250
				flight.arrived = false
				flight._process(0)
				_expect(not flight.arrived and flight.badge.modulate.a > 0, "Catalyst travels before arriving")
				var endpoint: Vector2 = flight.get_global_transform_with_canvas() * flight._local_center(flight.profile.statuses)
				_expect(endpoint.distance_to(flight.profile.statuses.get_global_rect().get_center()) < 0.1, "scaled trail lands on the actual status counter")
		_expect(saw_flight, "visible status flight exists")
		await _capture("flight-%d" % size.x)
		await create_timer(1.0).timeout
		_expect(player.damage.text == "2" and enemy.damage.text == "0", "damage animation finishes correctly")
		_expect("Catalyst ×2 · pending" in screen._actor_profiles.blade.statuses.text, "Catalyst count increases on arrival")
		_expect(screen._view.actor("blade").statuses[0].stacks == 1, "preview never mutates authoritative statuses")
		var start: int = player.started_ms
		screen._render()
		for frame in 3: await process_frame
		_expect(screen._defense_result_panels[0].started_ms == start and screen._defense_result_panels[0].damage.text == "2", "refresh does not restart animation")
		_expect("Catalyst ×2 · pending" in screen._actor_profiles.blade.statuses.text, "profile preview survives refresh without double gain")
		_expect(root.get_visible_rect().encloses(screen._auto_pass_toggle.get_global_rect()), "debug control remains accessible")
		await _capture("settled-%d" % size.x)
		await _close(screen)
	# Edge cases use the same native definitions as actual play.
	for scenario in ["new", "cap", "paid", "response", "coil", "incubation_exists", "barbed", "overflow", "other_source"]:
		var fixture := _fixture(3 if scenario == "cap" else 0)
		match scenario:
			"paid": fixture.snapshot.defense_selections.blade.catalyst_paid = true; fixture.snapshot.damage_sources[0].prevention = 2
			"response": fixture.snapshot.damage_sources[0].reaction_prevention = 1
			"coil", "incubation_exists":
				fixture.snapshot.defense_selections.blade.rolled_faces = [6, 6]
				if scenario == "incubation_exists": fixture.snapshot.actors.goblin.statuses = [{"definition_id": "incubation", "stacks": 1}]
			"barbed", "overflow":
				fixture.snapshot.defense_selections.blade.ability_id = "barbed_mantle"; fixture.snapshot.defense_selections.blade.rolled_faces = [1]
				if scenario == "overflow": fixture.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 3}]
			"other_source": fixture.snapshot.damage_sources.push_front({"id": "other", "source_actor_id": "goblin", "source_content_id": "sword_cut", "target_actor_id": "blade", "base_amount": 8})
		var screen = _screen(fixture)
		await process_frame
		var data: Dictionary
		for panel in screen._defense_result_panels:
			if panel.data.source_id == "enemy-attack": data = panel.data
		match scenario:
			"new": _expect(data.gains.size() == 1 and data.gains[0].after == 1, "first Catalyst creates one stack")
			"cap": _expect(data.gains.is_empty(), "full Catalyst has no false gain")
			"paid": _expect(data.before == 1 and data.after == 0, "paid Catalyst prevention counted once")
			"response": _expect(data.before == 2 and data.after == 1, "response-card prevention included")
			"coil": _expect(data.gains.size() == 1 and data.gains[0].status_id == "incubation", "two Coils preview one Incubation")
			"incubation_exists": _expect(data.gains.is_empty(), "existing Incubation not duplicated")
			"barbed": _expect(data.after == 1 and data.gains[0].target == "goblin" and data.gains[0].status_id == "poison", "Barbed Mantle previews enemy Poison and block")
			"overflow": _expect(data.gains.size() == 1 and data.gains[0].status_id == "incubation", "Poison overflow previews Incubation")
			"other_source": _expect(data.before == 3 and data.after == 2 and screen._defense_result_panels.size() == 3, "defense stays on chosen source and other attacks remain visible")
		await _close(screen)
	print("DEFENSE RESULT ANIMATION: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _verify_roll_handoff() -> void:
	var selection := _fixture()
	selection.snapshot.stage = "defense_selection"
	selection.snapshot.defense_selections = {}
	selection.legal_actions = []; selection.pending_input = {}
	var screen = _screen(selection)
	screen.set_process(false)
	await process_frame
	var board = screen._root
	var rolling := selection.duplicate(true)
	rolling.snapshot.stage = "defense_roll"
	rolling.snapshot.defense_selections = {"blade": {"ability_id": "shedskin", "source_id": "enemy-attack"}}
	rolling.pending_input = {"blade": {"id": "unified-roll", "segment": "defensive", "stage": "defense_roll", "allowed_commands": ["roll_dice"]}}
	rolling.legal_actions = [{"battle_id": rolling.snapshot.battle_id, "actor_id": "blade", "type": "roll_dice", "payload": {"pending_input_id": "unified-roll"}}]
	screen._view.apply_result(rolling)
	screen._render()
	_expect(screen._root == board, "authority roll handoff does not insert another screen")
	var fake := FakeBattleAuthority.new(); fake.enqueue(_fixture())
	screen.gateway = BattleGateway.new(fake)
	_expect(screen._auto_roll_defense_if_only_action(), "sole roll dispatches immediately after selection")
	_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]) == rolling.legal_actions[0], "exact legal roll submitted once")
	_expect(screen._defense_result_panels.size() == 2, "roll handoff opens both result panels together")
	_expect(not screen._auto_roll_defense_if_only_action() and fake.commands.size() == 1, "no duplicate roll on the next frame")
	await _close(screen)

func _verify_unified_sequence() -> void:
	var screen = _screen(_fixture(1))
	var fake := FakeBattleAuthority.new()
	var next := _fixture(1)
	next.snapshot.segment = "damage_resolution"; next.snapshot.stage = "damage_reaction"
	next.pending_input = {}; next.legal_actions = []
	fake.enqueue(next)
	screen.gateway = BattleGateway.new(fake)
	screen._auto_pass_disabled = false
	for frame in 3: await process_frame
	var board = screen._root
	var player = screen._defense_result_panels[0]
	var enemy = screen._defense_result_panels[1]
	var player_die = player.dice_controls[0]
	var enemy_die = enemy.dice_controls[0]
	_expect(is_equal_approx(screen.DEFENSE_TIMING.total_seconds(), 5.1), "default sequence has a readable 5.1-second budget")
	_expect(player.block.modulate.a == 0 and player.benefit_labels[0].modulate.a == 0 and player.gain_origins[0].modulate.a == 0, "benefits stay hidden until dice land")
	var player_face: String = player_die.text
	var enemy_face: String = enemy_die.text
	await create_timer(0.15).timeout
	_expect(player_die.text != player_face and enemy_die.text != enemy_face, "both sides animate their dice together")
	await _capture("unified-rolling")
	await create_timer(1.15).timeout
	_expect(screen._root == board and screen._defense_result_panels[0] == player and player.dice_controls[0] == player_die and enemy.dice_controls[0] == enemy_die, "dice land in the same controls without a screen transition")
	_expect(player_die.text.ends_with("\n1") and enemy_die.text.ends_with("\n6"), "both dice land on authoritative results")
	_expect(player.damage.text == "3" and enemy.damage.text == "3", "incoming damage stays intact while landing registers")
	await _capture("unified-landed")
	await create_timer(2.4).timeout
	_expect(player.damage.text == "2" and enemy.damage.text == "0", "prevention applies after landing")
	_expect(fake.commands.is_empty(), "auto-pass leaves time to read the settled result")
	_expect(screen._root == board, "effects animate without rebuilding the board")
	await _capture("unified-results")
	await create_timer(1.8).timeout
	_expect(fake.commands.size() == 1, "auto-pass advances once after rolling, effects and final hold")
	await _close(screen)

func _capture(name: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_DEFENSE_ANIMATION_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(name + ".png"))

func _close(screen) -> void:
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true; push_error("DEFENSE RESULT ANIMATION: " + message)
