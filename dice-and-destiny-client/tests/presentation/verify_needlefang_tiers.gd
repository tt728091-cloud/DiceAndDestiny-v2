extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	# Use the native character catalog; control the offered actions to exercise
	# every qualification boundary without relying on a lucky random roll.
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("inline-tier-fixture", 1788900535520209)
	_expect(base.get("accepted") == true, "native fixture starts")
	for size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = size
		for count in [0, 2, 3, 4, 5]:
			await _check(count, size, 0)
		await _check(4, size, 1)
	print("NEEDLEFANG INLINE TIERS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _fixture(count: int, bonus: int) -> Dictionary:
	var result := base.duplicate(true)
	result.events = []
	result.learned_policy = {}
	result.pending_input = {"blade": {"id": "tier-input", "window_id": "tier-window", "stage": "planning", "segment": "offensive", "iteration": 1, "planning_cycle": 1, "allowed_commands": ["planning_select_ability", "planning_pass"]}}
	result.snapshot.stage = "planning"
	result.snapshot.segment = "offensive"
	var actor: Dictionary = result.snapshot.actors.blade
	actor.qualified_abilities = ["needlefang"] if count >= 3 else []
	actor.needlefang_damage_bonus = bonus
	actor.selected_ability = ""
	actor.selected_tier = ""
	actor.dice = []
	for index in 5:
		actor.dice.append({"index": index, "die_id": "venom_d6", "face": 1 if index < count else 4, "symbols": ["fang" if index < count else "gland"]})
	actor.roll_history = [{"dice": actor.dice}] if count > 0 else []
	result.legal_actions = []
	for tier in range(3, count + 1):
		result.legal_actions.append({"battle_id": result.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "tier-input", "ability_id": "needlefang", "tier_id": "fang_%d" % tier, "target_ids": ["goblin"]}})
	return result

func _check(count: int, size: Vector2i, bonus: int) -> void:
	var fixture := _fixture(count, bonus)
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate()
	screen.initial_result = fixture
	screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("inline-tiers.json"))
	root.add_child(screen)
	await process_frame
	await process_frame
	for n in range(3, 6):
		var button := _tier(screen, n)
		_expect(button != null, "all tiers remain visible")
		if button == null: continue
		_expect(button.disabled == (n > count), "%d Fangs: tier %d availability" % [count, n])
		var outcome := button.get_node("TierOutcome") as Label
		_expect(outcome.is_visible_in_tree() and outcome.text == "%d DMG +%d%s" % [7 - n + bonus, n - 2, BattlePresentationCatalog.status("poison").glyph], "visible tier summary includes damage upgrade and Poison")
		_expect(outcome.get_minimum_size().x <= button.size.x, "compact summary fits within its tier button")
		_expect(button.tooltip_text.contains("%d damage + %d Poison" % [7 - n + bonus, n - 2]), "tier preview includes Lens")
		_expect(root.get_visible_rect().encloses(button.get_global_rect()), "tier lies inside %s viewport" % size)
	await _capture("fang-%d-bonus-%d-%d" % [count, bonus, size.x])
	# Four Fangs must allow the lower tier as well as the four-Fang result.
	for chosen in range(3, count + 1):
		screen._view.apply_result(fixture)
		screen._render()
		await process_frame
		var next := fixture.duplicate(true)
		next.snapshot.actors.blade.selected_ability = "needlefang"
		next.snapshot.actors.blade.selected_tier = "fang_%d" % chosen
		fake.enqueue(next)
		var before: int = fake.commands.size()
		var click := InputEventMouseButton.new()
		click.position = _tier(screen, chosen).get_global_rect().get_center()
		click.button_index = MOUSE_BUTTON_LEFT
		click.pressed = true
		root.push_input(click, true)
		var release := click.duplicate() as InputEventMouseButton
		release.pressed = false
		root.push_input(release, true)
		await process_frame
		_expect(fake.commands.size() == before + 1, "one direct command per click")
		if fake.commands.size() > before:
			var command: Dictionary = JSON.parse_string(fake.commands[-1])
			_expect(command.payload.tier_id == "fang_%d" % chosen and command.payload.target_ids == ["goblin"], "exact offered tier and target submitted")
		_expect(screen.find_children("*", "AcceptDialog", false, false).is_empty(), "no tier popup")
		_expect(screen._view.actor("blade").get("selected_tier") == "fang_%d" % chosen, "selected tier retained while outcome replaces choices")
		while not screen._selection_morph.is_empty() or Time.get_ticks_msec() <= screen._flow_until: await process_frame
	# Old callbacks and review mode must never send gameplay commands.
	var before: int = fake.commands.size()
	screen._view.legal_actions = []
	screen._select_needlefang_tier("fang_3")
	_expect(fake.commands.size() == before, "stale tier ignored")
	screen._view.apply_result(fixture)
	screen._history_review = true
	screen._render()
	await process_frame
	for n in range(3, 6): _expect(_tier(screen, n).disabled, "history review disables all tiers")
	screen._select_needlefang_tier("fang_3")
	_expect(fake.commands.size() == before, "history review cannot select a tier")
	screen.active_store.clear()
	screen.queue_free()
	await process_frame

func _tier(screen, n: int) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if str(button.get_meta("inspection_id", "")) == "battle.ability.blade.needlefang.fang_%d" % n: return button
	return null

func _capture(label: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_TIER_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless" or not label.begins_with("fang-4-bonus-0"): return
	await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(label + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("NEEDLEFANG INLINE TIERS: " + message)
