extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("fixed-footer", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loaded")
	for size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = size
		for phase in [["ongoing_effects", "status_roll_reaction"], ["offensive", "planning"], ["defensive", "defense_reaction"], ["damage_resolution", "damage_reaction"], ["income", "income"], ["ongoing_effects", "presentation"]]:
			await _check(size, phase[0], phase[1])
	print("FIXED ACTION FOOTER: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check(size: Vector2i, segment: String, stage: String) -> void:
	var fixture := base.duplicate(true)
	fixture.learned_policy = {}; fixture.events = []
	fixture.snapshot.segment = segment; fixture.snapshot.stage = stage
	fixture.snapshot.actors.blade.statuses = [{"definition_id": "poison", "stacks": 2}, {"definition_id": "catalyst", "stacks": 1}]
	fixture.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 3}, {"definition_id": "volatile_poison", "stacks": 3}]
	fixture.snapshot.effect_rolls = []
	for index in 8:
		fixture.snapshot.effect_rolls.append({"actor_id": "blade" if index < 2 else "goblin", "status_instance_id": "toxin-%d" % index, "source_content_id": "poison" if index < 5 else "volatile_poison", "resolved": true, "die": {"index": index, "die_id": "standard_d6", "face": 6 if index < 7 else 5}})
	var command := "planning_pass" if stage == "planning" else "pass"
	fixture.pending_input = {"blade": {"id": "footer-input", "window_id": "footer-window", "stage": stage, "segment": segment, "allowed_commands": [command]}}
	fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": command, "payload": {"pending_input_id": "footer-input"}}]
	if stage == "presentation": fixture.events = [{"sequence": 1, "type": "status_changed", "data": {"description": "Many status changes"}}]
	var fake := FakeBattleAuthority.new(); var after := fixture.duplicate(true); after.events = []; fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.gateway = BattleGateway.new(fake); screen.initial_result = fixture
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("footer-test.json"))
	root.add_child(screen); screen.set_process(false)
	for frame in 4: await process_frame
	# Stress future content growth as well as the eight-die reported screen.
	for index in 30:
		var detail := Label.new(); detail.text = "Additional effect %d: Poison rolls, damage results and status explanations." % index; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; screen._center.add_child(detail)
	for frame in 4: await process_frame
	var button := screen._action_footer.get_child(0) as Button
	_expect(button != null and not button.disabled, "footer action available in " + stage)
	if button != null:
		var position := button.get_global_rect()
		_expect(root.get_visible_rect().encloses(position), "%s footer inside %s viewport" % [stage, size])
		_expect(position.end.y > root.get_visible_rect().end.y - 90, "action stays at bottom")
		var scroll: ScrollContainer = screen._center_scroll
		_expect(scroll.get_v_scroll_bar().max_value > scroll.get_v_scroll_bar().page, "overflow content scrolls")
		var path := OS.get_environment("DICE_AND_DESTINY_FOOTER_SCREENSHOTS")
		if not path.is_empty() and stage == "status_roll_reaction" and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(path.path_join("footer-%d.png" % size.x))
		scroll.scroll_vertical = int(scroll.get_v_scroll_bar().max_value)
		for frame in 3: await process_frame
		_expect(button.get_global_rect() == position, "scrolling does not move footer")
		# Confirm real pointer input reaches the fixed footer even at full scroll.
		# Planning's unrolled-skip confirmation is a separate intentional choice.
		if stage != "planning":
			var press := InputEventMouseButton.new(); press.position = position.get_center(); press.button_index = MOUSE_BUTTON_LEFT; press.pressed = true; root.push_input(press, true)
			var release := press.duplicate() as InputEventMouseButton; release.pressed = false; root.push_input(release, true)
			await process_frame
			if stage == "presentation": _expect(not screen._director.has_beats(), "Continue Presentation is clickable")
			else: _expect(fake.commands.size() == 1, "pass is clickable in " + stage)
	screen.active_store.clear(); screen.queue_free()
	await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("FIXED ACTION FOOTER: " + message)
