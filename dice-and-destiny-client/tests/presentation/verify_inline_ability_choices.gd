extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("inline-ability-choices", 1788900535520209)
	for viewport_size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport_size
		for ability in ["shedskin", "fever_spike", "terminal_bite", "venom_gland", "sword_cut"]:
			await _check(ability, viewport_size)
	print("INLINE ABILITY CHOICES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _check(ability: String, viewport_size: Vector2i) -> void:
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []
	var defense := ability == "shedskin"
	var stage := "defense_selection" if defense else "planning"
	var segment := "defensive" if defense else "offensive"
	fixture.snapshot.stage = stage; fixture.snapshot.segment = segment
	fixture.pending_input = {"blade": {"id": "inline-input", "stage": stage, "segment": segment, "allowed_commands": ["planning_select_ability"]}}
	fixture.snapshot.actors.blade.selected_ability = ""
	fixture.snapshot.actors.blade.qualified_abilities = [ability]
	fixture.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 3}]
	fixture.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 3}, {"definition_id": "volatile_poison", "stacks": 3}]
	fixture.snapshot.damage_sources = [{"id": "source-1", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 6}]
	var variants := [{}, {"spend_catalyst": true}] if defense else [{"toxin_choices": ["volatile_poison"]}, {"toxin_choices": ["volatile_poison", "volatile_poison"]}, {"toxin_choices": ["poison"]}, {"toxin_choices": ["poison", "volatile_poison"]}, {"toxin_choices": ["poison", "poison"]}] if ability in ["fever_spike", "terminal_bite"] else [{"tier_id": "base"}, {"tier_id": "greater"}, {"tier_id": "strongest"}] if ability == "sword_cut" else [{}]
	if ability == "sword_cut": fixture.snapshot.actors.blade.offensive_abilities = [ability]
	for variant in variants:
		var payload := {"pending_input_id": "inline-input", "ability_id": ability, "target_ids": ["source-1" if defense else "goblin"]}
		payload.merge(variant)
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": payload})
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("inline-choice-test.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	if defense: screen._selected_source = "source-1"; screen._render()
	await _frames()
	for selected in variants.size():
		if selected > 0:
			screen._view.apply_result(fixture)
			if defense: screen._selected_source = "source-1"
			screen._render(); await _frames()
		var tile: BattleAbilityTile
		for button in screen._ability_dock.find_children("*", "Button", true, false):
			if button is BattleAbilityTile and button.ability_id == ability: tile = button
		_expect(tile != null, "ability tile exists")
		if tile == null: break
		var choices := tile.get_node_or_null("AbilityChoices")
		var target: Button = tile
		if variants.size() > 1:
			_expect(choices != null and choices.get_child_count() == variants.size(), "every legal alternative is inline, including more than three choices")
			if choices == null: break
			for option in choices.get_children():
				_expect(not option.disabled, "legal option enabled")
				_expect(tile.get_global_rect().encloses(option.get_global_rect()), "option remains inside ability box")
				_expect(screen._ability_dock.get_parent().get_global_rect().encloses(option.get_global_rect()), "all choices fit visible ability rail")
				_expect(option.size.y >= 48, "readable clickable option height")
			target = choices.get_child(selected)
		if selected == 0: await _capture("%s-%d" % [ability, viewport_size.x])
		var next := fixture.duplicate(true); next.legal_actions = []
		fake.enqueue(next)
		var click := InputEventMouseButton.new(); click.position = target.get_global_rect().get_center(); click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true
		root.push_input(click, true); var release := click.duplicate(); release.pressed = false; root.push_input(release, true)
		await process_frame
		_expect(fake.commands.size() == selected + 1, "one command per real inline click")
		if fake.commands.size() > selected: _expect(fake.commands[selected] == JSON.stringify(fixture.legal_actions[selected]), "exact tier, toxin, source and cost preserved")
		_expect(screen.find_children("*", "AcceptDialog", false, false).is_empty(), "no ability choice popup")
		while not screen._selection_morph.is_empty() or Time.get_ticks_msec() <= screen._flow_until: await process_frame
	# Old controls must not submit a source or option that is no longer offered.
	var count: int = fake.commands.size()
	screen._view.legal_actions = []; screen._select_ability_action(fixture.legal_actions[0])
	_expect(fake.commands.size() == count, "stale action rejected")
	screen._view.apply_result(fixture); screen._history_review = true
	screen._select_ability_action(fixture.legal_actions[0])
	_expect(fake.commands.size() == count, "review mode rejects actions")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _frames() -> void:
	for frame in 6: await process_frame

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_INLINE_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("INLINE ABILITY CHOICES: " + message)
