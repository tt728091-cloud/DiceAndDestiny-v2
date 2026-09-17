extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("spined-feedback", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loaded")
	for scenario in ["single", "multiple", "rejected"]: await _check(scenario)
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
	for index in (2 if scenario == "multiple" else 1):
		fixture.snapshot.damage_sources.append({"id": "source-%d" % index, "source_actor_id": "goblin", "source_content_id": "sword_cut" if index == 0 else "shield_bash", "target_actor_id": "blade", "base_amount": 5, "prevention": 2, "reaction_prevention": 0, "final_amount": 3})
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "defense-input", "commitment": {"card_ids": ["spined-card"], "proposal_ids": ["source-%d" % index], "choice_id": "prevent"}}})
	var chosen: int = fixture.legal_actions.size() - 1
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
	for node in screen.find_children("*", "Button", true, false):
		if node is BattleCard and node.definition_id == "spined_rebuttal": card = node
	_expect(card != null and not card.disabled, "Spined Rebuttal is playable")
	if card != null: card.pressed.emit()
	await process_frame
	var dialogs := screen.find_children("*", "AcceptDialog", false, false)
	if scenario == "multiple":
		_expect(fake.commands.is_empty() and dialogs.size() == 1, "multiple incoming attacks still require a choice")
		if dialogs.size() == 1:
			for button in dialogs[0].find_children("*", "Button", true, false):
				if "Shield Bash" in button.text: button.pressed.emit(); break
	else: _expect(dialogs.is_empty(), "single option opens no popup")
	await process_frame
	_expect(fake.commands.size() == 1, "one command per card play")
	if fake.commands.size() == 1: _expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[chosen]), "exact chosen source is submitted")
	if scenario == "rejected":
		_expect(not screen._reaction_feedback_active(), "rejected card shows no success feedback")
	else:
		_expect(screen._reaction_feedback_active(), "feedback visible after success")
		var text := str(screen._reaction_card_feedback.get("text", ""))
		_expect("+1 prevention" in text and "1 Poison queued for Blade Warden" in text, "explains prevention and queued Poison honestly")
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
