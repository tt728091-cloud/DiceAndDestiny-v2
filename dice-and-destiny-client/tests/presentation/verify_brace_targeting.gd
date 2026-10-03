extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	base = gateway.start_battle("brace-targeting", 43)
	for width in [1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for unified in [false, true]:
			for scenario in ["single", "same_enemy", "multiple_enemies", "one_viable", "none"]: await _scenario(scenario, unified)
	await _native()
	print("BRACE TARGETING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(scenario: String, unified: bool = false) -> void:
	var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}
	fixture.snapshot.unified_defense = unified
	fixture.snapshot.segment = "defensive" if unified else "damage_resolution"; fixture.snapshot.stage = "defense_selection" if unified else "damage_reaction"
	fixture.snapshot.actors.blade.hand = ["brace-card"]; fixture.snapshot.actors.blade.hand_count = 1
	fixture.snapshot.actors.blade.card_instances = {"brace-card": {"instance_id": "brace-card", "definition_id": "brace"}}
	fixture.snapshot.actors.blade.energy_points = 3
	fixture.pending_input = {"blade": {"id": "damage-input", "segment": fixture.snapshot.segment, "stage": fixture.snapshot.stage, "allowed_commands": ["commit_interaction", "pass"]}}
	fixture.legal_actions = []; fixture.snapshot.damage_sources = []
	for i in (1 if scenario == "single" or scenario == "none" else 2):
		var amount := 0 if scenario == "none" or (scenario == "one_viable" and i == 0) else 3
		fixture.snapshot.damage_sources.append({"id": "incoming-%d" % i, "source_actor_id": "goblin-2" if scenario == "multiple_enemies" and i == 1 else "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 3, "final_amount": amount})
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "damage-input", "commitment": {"card_ids": ["brace-card"], "proposal_ids": ["incoming-%d" % i]}}})
	fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "adventurer_strike", "base_amount": 2, "final_amount": 2})
	fixture.snapshot.settled_damage = {"id": "batch", "sources": fixture.snapshot.damage_sources.duplicate(true), "removals": []}
	for source in fixture.snapshot.damage_sources:
		for i in int(source.final_amount):
			fixture.snapshot.settled_damage.removals.append({"id": source.id + str(i), "card_id": source.id + str(i), "card_definition_id": "take_stock", "target_actor_id": source.target_actor_id, "original_zone": "discard", "accepted": true, "released": false, "damage_proposal_ids": [source.id]})
	var after := fixture.duplicate(true); after.legal_actions = []; after.pending_input = {}; after.snapshot.actors.blade.hand = []
	var fake := FakeBattleAuthority.new(); fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("brace-targeting.json")); canvas.add_child(screen); screen.set_process(false)
	await _ready_hand(screen)
	# An earlier clicked source must never override the card-first choice.
	screen._selected_source = "outgoing"
	await _click_card(screen)
	for frame in 12: await process_frame
	if scenario == "single" or scenario == "one_viable":
		_expect(fake.commands.size() == 1, "one viable source plays immediately: " + scenario)
		if fake.commands.size() == 1: _expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[-1]), "auto-target sends exact current legal command")
	elif scenario == "none":
		_expect(fake.commands.is_empty() and screen._selected_card.is_empty(), "fully prevented attacks cannot waste Brace")
	else:
		_expect(fake.commands.is_empty() and screen._selected_card.get("source_targeting", false), "card first waits for source without spending")
		_expect(screen._sole_pass_action().is_empty(), "target choice blocks automatic pass")
		for id in ["incoming-0", "incoming-1"]:
			_expect(screen._attack_intents[id].intent.get_meta("inspection_id") == "battle.card_target." + id, "all incoming sources highlighted, including same-enemy attacks")
			_expect("Save with Brace" in _heading(screen, id).text, "damage stack identifies save action")
		_expect(screen._attack_intents.outgoing.intent.disabled and _heading(screen, "outgoing").disabled, "outgoing damage cannot be chosen")
		await _capture("brace-targets-%s-%d" % [scenario, canvas.size.x])
		# Click the selected card a second time to cancel without an authority call.
		await _ready_hand(screen); await _click_card(screen)
		_expect(screen._selected_card.is_empty() and fake.commands.is_empty(), "second click cancels without spending")
		await _ready_hand(screen); await _click_card(screen)
		for frame in 8: await process_frame
		var legal: Array = screen._view.legal_actions; screen._view.legal_actions = []
		await _click(_heading(screen, "incoming-1").get_global_rect().get_center())
		_expect(fake.commands.is_empty(), "stale target cannot submit")
		screen._view.legal_actions = legal
		var target: Control = _heading(screen, "incoming-1") if scenario == "same_enemy" else screen._attack_intents["incoming-1"].intent
		await _click(target.get_global_rect().get_center())
		_expect(fake.commands.size() == 1, "clicking highlighted stack or attack submits exactly once")
		if fake.commands.size() == 1: _expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[1]), "chosen source is preserved")
	_expect(screen._error_message.is_empty(), "no source-first instruction or command error")
	screen.queue_free(); await process_frame
func _ready_hand(screen) -> void:
	for frame in 12: await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	screen._hand_dock.held_open = true; screen._hand_dock.reveal = 1; screen._hand_dock._layout()
	await process_frame
func _click_card(screen) -> void:
	var hand: Control = screen._hand_dock
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[0] * Vector2(90, 100))
	await _click(point)
func _click(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true); await process_frame
func _heading(screen, id: String) -> Button:
	for button in screen._root.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.damage_stack." + id: return button
	return null
func _capture(filename: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_BRACE_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(); canvas.get_texture().get_image().save_png(directory.path_join(filename + ".png"))
func _native() -> void:
	var trace: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/adventurer_damage_progression.json"))
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result: Dictionary = gateway.start_battle(trace.battle_id, int(trace.seed))
	for step in trace.trace:
		if step.controller == "human":
			var command: Dictionary = JSON.parse_string(JSON.stringify(step.command).replace('"seat-a"', '"blade"').replace('"seat-b"', '"goblin"'))
			if command.type == "commit_interaction" and command.payload.get("commitment", {}).has("card_ids"): break
			var choices: Array = result.legal_actions.filter(func(action): return _semantic(action) == _semantic(command))
			if choices.size() != 1: _expect(false, "native trace matches legal action"); return
			result = gateway.submit(JSON.stringify(choices[0]))
		else: result = gateway.advance_model()
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("brace-native.json")); canvas.add_child(screen); screen.set_process(false)
	await _ready_hand(screen)
	var card_index := -1
	for i in screen._hand_dock.cards.size():
		if screen._hand_dock.cards[i].definition_id == "brace": card_index = i; break
	_expect(card_index >= 0, "native trace reaches playable Brace")
	if card_index >= 0:
		var before: Dictionary = screen._view.actor("blade").duplicate(true)
		var card_id: String = screen._hand_dock.cards[card_index].instance_id
		var hand: Control = screen._hand_dock
		await _click(hand.get_global_transform_with_canvas() * (hand.base_transforms[card_index] * Vector2(70, 25)))
		var after: Dictionary = screen._view.actor("blade")
		_expect(screen._error_message.is_empty() and card_id not in after.hand, "real Brace click accepted without selecting incoming source")
		_expect(int(after.energy_points) == int(before.energy_points) - 1, "real play spends exactly one energy")
		_expect(int(after.current_health) == int(before.current_health), "prevention preserves health")
		_expect(not screen._damage_feedback.is_empty(), "real play starts saved-card feedback")
		if not screen._damage_feedback.is_empty(): _expect(screen._damage_feedback.saved.size() == 3, "Brace saves three revealed damage cards")
		_expect(int(after.discard_count) > int(before.discard_count), "played and saved cards reach discard")
	screen.queue_free(); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("BRACE TARGETING: " + message)

func _semantic(value):
	if value is Dictionary:
		var result := {}
		for key in value:
			if key not in ["checkpoint", "pending_input_id"]: result[key] = _semantic(value[key])
		return result
	if value is Array: return value.map(_semantic)
	if value is String and value.begins_with("source-r"): return value.substr(0, value.rfind("-"))
	return value
