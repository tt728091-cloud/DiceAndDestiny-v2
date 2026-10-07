extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	base = gateway.start_battle("defense-entry", 43)
	for scenario in ["direct", "priority", "canceled", "unaffordable", "catalyst", "multiple", "card_first"]:
		await _check(scenario)
	print("DEFENSE ENTRY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _offense() -> Dictionary:
	var f := base.duplicate(true)
	f.events = []; f.learned_policy = {}
	f.snapshot.stage = "offensive_reaction"; f.snapshot.segment = "offensive"; f.snapshot.damage_sources = []
	f.pending_input = {"blade": {"id": "reaction", "stage": "offensive_reaction", "segment": "offensive", "allowed_commands": ["pass", "commit_interaction"]}}
	f.snapshot.actors.blade.defensive_abilities = ["shedskin"]
	f.snapshot.actors.blade.selected_ability = "needlefang"
	f.snapshot.actors.blade.selected_targets = ["goblin"]
	f.snapshot.actors.blade.offensive_outcome = {"base_damage": 4}
	f.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 1}]
	f.snapshot.actors.blade.hand = ["tip-card"]
	f.snapshot.actors.blade.card_instances = {"tip-card": {"definition_id": "tip_it", "instance_id": "tip-card"}}
	for id in ["goblin", "goblin-2"]:
		f.snapshot.actors[id].selected_ability = "brine_lash"
		f.snapshot.actors[id].selected_targets = ["blade"]
		f.snapshot.actors[id].offensive_outcome = {"base_damage": 5}
	f.legal_actions = [{"battle_id": f.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "reaction"}}, {"battle_id": f.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "reaction", "commitment": {"card_ids": ["tip-card"]}}}]
	f.defense_previews = []
	for id in ["goblin", "goblin-2"]:
		for paid in [false, true]:
			f.defense_previews.append({"source_actor_id": id, "source_content_id": "brine_lash", "target_actor_id": "blade", "ability_id": "shedskin", "spend_catalyst": paid})
	return f
func _defense(f: Dictionary, paid: bool) -> Dictionary:
	var d := f.duplicate(true)
	d.defense_previews = []; d.snapshot.segment = "defensive"; d.snapshot.stage = "defense_selection"
	d.pending_input = {"blade": {"id": "defense", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
	d.snapshot.damage_sources = []
	d.legal_actions = []
	for id in ["goblin", "goblin-2"]:
		d.snapshot.damage_sources.append({"id": "final-" + id, "source_actor_id": id, "source_content_id": "brine_lash", "target_actor_id": "blade", "base_amount": 5})
		d.legal_actions.append({"battle_id": f.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "defense", "ability_id": "shedskin", "target_ids": ["final-" + id], "spend_catalyst": paid}})
	return d
func _check(scenario: String) -> void:
	var f := _offense()
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(fake)
	screen._auto_pass_disabled = true; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-entry.json"))
	root.add_child(screen); screen.set_process(false)
	await create_timer(0.75).timeout
	for frame in 8: await process_frame
	var source := "preview:goblin-2" if scenario == "multiple" else "preview:goblin"
	_expect(screen._attack_intents.has(source), "revealed preview rendered")
	_expect(not screen._attack_intents[source].intent.disabled, "reaction intent can be inspected")
	await _click(screen._attack_intents[source].intent)
	_expect(fake.commands.is_empty() and screen._view.stage == "offensive_reaction", "inspection never passes")
	_expect(screen._card_legal("tip_it"), "offensive card stays legal while inspecting defense")
	_expect(screen._ability_actions("shedskin").size() == 2, "normal and Catalyst choices offered before passing: " + scenario)
	if scenario == "card_first":
		var after := f.duplicate(true); after.snapshot.actors.blade.hand = []; after.pending_input.blade.id = "after-card"
		fake.enqueue(after)
		screen._send(JSON.stringify(f.legal_actions[1]))
		_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).type == "commit_interaction", "card can be played without committing defense")
		_expect(screen._queued_defense.is_empty(), "inspection does not queue a defense across a card")
	else:
		var d := _defense(f, scenario == "catalyst")
		if scenario == "canceled":
			d.legal_actions = [d.legal_actions[1]]; d.snapshot.damage_sources = [d.snapshot.damage_sources[1]]
		if scenario == "unaffordable": d.legal_actions = []
		if scenario == "priority":
			var response := f.duplicate(true)
			response.pending_input.blade.id = "returned-priority"
			response.legal_actions[0].payload.pending_input_id = "returned-priority"
			fake.enqueue(response)
		fake.enqueue(d)
		var after := d.duplicate(true)
		after.snapshot.defense_plans = {"final-goblin-2" if scenario == "multiple" else "final-goblin": {"actor_id": "blade", "ability_id": "shedskin"}}
		after.legal_actions = [d.legal_actions[0]] if scenario == "multiple" else []
		fake.enqueue(after)
		if scenario == "direct": await _capture("reaction-defense-options")
		var tile: Control
		var rail: Control = screen._ability_dock.get_parent()
		_expect(rail.position.y < screen.PLAYER_ZONE_TOP and not rail.get_global_rect().intersects(screen._attack_intents[screen._selected_source].intent.get_global_rect()), "offensive reaction offers adjacent defenses without covering attack before passing")
		for button in screen._ability_dock.find_children("*", "Button", true, false):
			if button.get_meta("inspection_id", "") == "battle.ability.blade.shedskin": tile = button
		_expect(tile != null, "defense rail visible")
		if tile == null: screen.queue_free(); await process_frame; return
		await _click(tile.get_node("AbilityChoices").get_child(1 if scenario == "catalyst" else 0))
		if scenario == "priority":
			_expect(not screen._card_legal("tip_it"), "commitment closes offensive cards during returned priority")
			screen._continue_queued_defense()
		screen._continue_queued_defense()
		var expected := 1 if scenario in ["canceled", "unaffordable"] else 3 if scenario == "priority" else 2
		_expect(fake.commands.size() == expected, "pass and defense submitted once: " + scenario)
		if scenario not in ["canceled", "unaffordable"]:
			var submitted: Dictionary = JSON.parse_string(fake.commands[-1])
			_expect(submitted.type == "planning_select_ability" and submitted.payload.pending_input_id == "defense", "fresh authoritative command used")
			_expect(submitted.payload.target_ids == ["final-goblin-2" if scenario == "multiple" else "final-goblin"], "correct source reserved")
			_expect(bool(submitted.payload.get("spend_catalyst", false)) == (scenario == "catalyst"), "Catalyst choice preserved")
			_expect(not screen._card_legal("tip_it"), "offensive-only card unavailable in defense")
		else: _expect(not screen._error_message.is_empty(), "changed choice requests new defense")
		_expect(screen._queued_defense.is_empty(), "handoff consumed or canceled")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _click(button: Control) -> void:
	var position := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = position; root.push_input(motion, true)
	await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = position; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true)
	for frame in 4: await process_frame
func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_INTENT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.1).timeout; RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join(name + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DEFENSE ENTRY: " + message)
