extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	base = gateway.start_battle("spined-feedback", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loaded")
	for scenario in ["single", "multiple", "multiple_first", "rejected"]: await _check(scenario)
	print("SPINED REBUTTAL FEEDBACK: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check(scenario: String) -> void:
	var fixture := base.duplicate(true)
	fixture.learned_policy = {}; fixture.events = []
	fixture.snapshot.stage = "defense_reaction"; fixture.snapshot.segment = "defensive"
	fixture.snapshot.actors.blade.hand = ["spined-card"]
	fixture.snapshot.actors.blade.card_instances = {"spined-card": {"instance_id": "spined-card", "definition_id": "spined_rebuttal"}}
	fixture.snapshot.damage_sources = []
	fixture.pending_input = {"blade": {"id": "defense-input", "stage": "defense_reaction", "segment": "defensive", "allowed_commands": ["commit_interaction", "pass"]}}
	fixture.legal_actions = []
	for index in (2 if scenario.begins_with("multiple") else 1):
		fixture.snapshot.damage_sources.append({"id": "source-%d" % index, "source_actor_id": "goblin" if index == 0 else "goblin-2", "source_content_id": "brine_lash", "target_actor_id": "blade", "base_amount": 5, "prevention": 2, "reaction_prevention": 0, "final_amount": 3})
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "defense-input", "commitment": {"card_ids": ["spined-card"], "proposal_ids": ["source-%d" % index], "choice_id": "prevent"}}})
	var chosen: int = 0 if scenario == "multiple_first" else fixture.legal_actions.size() - 1
	if scenario.begins_with("multiple"):
		fixture.snapshot.defense_history = {"source-0": {"actor_id": "blade", "source_id": "source-0", "ability_id": "shedskin", "rolled_face": 1, "rolled_faces": [1, 1], "finalized": true}}
		fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "source_id": "source-1", "ability_id": "shedskin", "rolled_face": 4, "rolled_faces": [4, 4]}}
	fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "source_content_id": "needlefang", "target_actor_id": "goblin", "base_amount": 3})
	var after := fixture.duplicate(true)
	after.snapshot.damage_sources[chosen].reaction_prevention = 1
	after.snapshot.actors.blade.hand = []
	after.snapshot.actors.blade.energy_points = 2
	after.events = [{"sequence": 100, "type": "card_played", "actor_id": "blade", "data": {"card_definition_id": "spined_rebuttal"}}]
	after.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "defense-input"}}]
	var fake := FakeBattleAuthority.new()
	fake.enqueue({"accepted": false, "error": "Test rejection"} if scenario == "rejected" else after)
	var next := after.duplicate(true); next.events = []; next.legal_actions = []; next.pending_input = {}
	fake.enqueue(next)
	var screen = SCREEN.instantiate(); screen.gateway = BattleGateway.new(fake); screen.initial_result = fixture
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("spined-feedback-test.json"))
	root.add_child(screen)
	await process_frame
	var card: BattleCard
	var deadline := Time.get_ticks_msec() + 10000
	while Time.get_ticks_msec() < deadline:
		for node in screen.find_children("*", "Button", true, false):
			if node is BattleCard and node.definition_id == "spined_rebuttal": card = node
		if card != null and not card.disabled: break
		await process_frame
	_expect(card != null and not card.disabled, "Spined Rebuttal is playable: " + scenario)
	if card != null: card.pressed.emit()
	await process_frame
	var dialogs := screen.find_children("*", "AcceptDialog", false, false)
	_expect(dialogs.is_empty() and fake.commands.is_empty(), "card selects highlighted targets without a popup or immediate spend")
	_expect(screen._defense_result_panels[-1].get_node_or_null("CardTarget") == null, "outgoing damage cannot be targeted")
	for index in fixture.legal_actions.size():
		_expect(screen._defense_result_panels[index].get_node_or_null("CardTarget") != null, "every legal incoming attack is highlighted")
	if scenario.begins_with("multiple"):
		_expect(screen._venom_choice_label(fixture.legal_actions[0]) != screen._venom_choice_label(fixture.legal_actions[1]), "fallback labels distinguish identical abilities by attacker")
	# Cancel leaves the hand and authority untouched, and targeting can restart.
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.card_target.cancel": button.pressed.emit(); break
	await process_frame
	_expect(screen._selected_card.is_empty() and fake.commands.is_empty(), "cancel spends nothing")
	for node in screen.find_children("*", "Button", true, false):
		if node is BattleCard and node.definition_id == "spined_rebuttal": node.pressed.emit(); break
	await process_frame
	var target: Button = screen._defense_result_panels[chosen].get_node("CardTarget")
	await process_frame
	var point: Vector2 = screen._defense_result_panels[chosen].damage.get_global_rect().get_center()
	_expect(target.get_global_rect().has_point(point), "damage number is inside the clickable target")
	var target_path := OS.get_environment("DICE_AND_DESTINY_SPINED_TARGET_SCREENSHOT")
	if scenario == "multiple" and not target_path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(target_path)
	var legal: Array = screen._view.legal_actions
	screen._view.legal_actions = []
	target.pressed.emit()
	_expect(fake.commands.is_empty(), "a stale target cannot submit a no-longer-legal action")
	screen._view.legal_actions = legal
	var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true
	root.push_input(click, true)
	click = click.duplicate(); click.pressed = false; root.push_input(click, true)
	_expect(fake.commands.size() == 1, "clicking the displayed damage number plays the card")
	target.pressed.emit()
	await process_frame
	_expect(fake.commands.size() == 1, "one command per card play")
	if fake.commands.size() == 1: _expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[chosen]), "exact chosen source is submitted")
	if scenario == "rejected":
		_expect(not screen._reaction_feedback_active(), "rejected card shows no success feedback")
	else:
		_expect(screen._reaction_feedback_active(), "feedback visible after success")
		var text := str(screen._reaction_card_feedback.get("text", ""))
		_expect("+1 prevention" in text and ("1 Poison queued for Brine Mask %d" % (chosen + 1)) in text, "explains prevention and queued Poison honestly")
		_expect(screen._reaction_card_feedback.source_id == "source-%d" % chosen, "glow belongs to selected source")
		_expect(not screen._director.has_beats(), "feedback is on the board, not a presentation popup")
		await create_timer(0.4).timeout
		var path := OS.get_environment("DICE_AND_DESTINY_SPINED_SCREENSHOT")
		if scenario == "single" and not path.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(path)
		_expect(fake.commands.size() == 1, "automatic pass waits for visible feedback")
		await create_timer(6.0).timeout
		_expect(fake.commands.size() == 2 and not screen._reaction_feedback_active(), "feedback fades and automatic pass resumes")
	screen.active_store.clear(); screen.queue_free()
	await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("SPINED REBUTTAL: " + message)
