extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _event(sequence: int, actor: String, status: String, before: int, after: int = 0) -> Dictionary:
	return {"sequence": sequence, "type": "card_played", "actor_id": actor, "segment": "defensive", "data": {"card_instance_id": "antidote-" + str(sequence), "card_definition_id": "antidote", "operation": "remove_status", "choice_id": status, "stacks_before": before, "stacks_after": after, "stacks_removed": before - after}}

func _fixture(actor: String) -> Dictionary:
	var result := base.duplicate(true)
	result.events = [_event(100, actor, "poison", 3)]
	result.learned_policy = {}
	result.snapshot.segment = "defensive"
	result.snapshot.stage = "defense_reaction"
	result.snapshot.pass_hands_off_priority = true
	result.snapshot.actors[actor].statuses = [{"definition_id": "bleed", "stacks": 1}]
	result.snapshot.effect_rolls = [{"actor_id": actor, "source_content_id": "poison", "die": {"face": 2}, "proposed_damage": 1}]
	result.pending_input = {"blade": {"id": "cleanse-response", "segment": "defensive", "stage": "defense_reaction", "allowed_commands": ["pass"]}}
	result.legal_actions = [{"battle_id": result.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "cleanse-response"}}]
	return result

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("cleanse-fixture", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loaded")
	# Each recorded card play remains distinct; replayed events are not repeated.
	var director := BattlePresentationDirector.new()
	var grouped := _fixture("goblin")
	grouped.events.append(_event(101, "goblin", "incubation", 1))
	director.queue_result(grouped)
	_expect(director.has_pending_card_cleanse() and director.peek().event.data.choice_id == "poison", "first cleanse queued")
	director.advance()
	_expect(director.peek().event.data.choice_id == "incubation", "second card keeps its own status cause")
	director.advance(); director.queue_result(grouped)
	_expect(not director.has_beats(), "presented events do not replay")
	grouped.events = [_event(102, "goblin", "poison", 0)]
	director.queue_result(grouped)
	_expect(not director.has_beats(), "no removal means no cleanse animation")
	for actor in ["goblin", "blade"]:
		await _check(actor)
	print("CARD CLEANSE PRESENTATION: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check(actor: String) -> void:
	var fixture := _fixture(actor)
	var fake := FakeBattleAuthority.new()
	var next := fixture.duplicate(true); next.events = []; next.legal_actions = []; next.pending_input = {}
	fake.enqueue(next)
	var screen = SCREEN.instantiate()
	screen.gateway = BattleGateway.new(fake); screen.initial_result = fixture
	screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("cleanse-test.json"))
	root.add_child(screen)
	await process_frame
	await process_frame
	var panel: Control
	for node in screen.find_children("*", "Control", true, false):
		if node.get_meta("inspection_id", "") == "battle.card_cleanse": panel = node
	_expect(panel != null, "played card is visible")
	if panel != null:
		_expect(panel.tooltip_text.contains("Antidote") and panel.tooltip_text.contains("Poison  3 → 0"), "card explains exact removal")
		_expect(root.get_visible_rect().encloses(panel.get_global_rect()), "presentation fits the viewport")
		_expect(panel._tokens.size() == 3, "three stack icons shown")
	var profile: ActorProfile = screen._actor_profiles[actor]
	var ghost := profile.find_child("CleansedStatus", true, false) as Label
	_expect(ghost != null and ghost.text.contains("Poison ×3"), "removed status visible beside its owner")
	_expect(profile.statuses.text.contains("Bleed ×1"), "other status stays visible")
	_expect(fake.commands.is_empty(), "automatic pass waits for card animation")
	# The scheduler must not cover an opponent card with its next thinking screen.
	screen.learned_battle_mode = true
	screen._schedule_model_if_needed({"learned_policy": {"model_turn": true}})
	_expect(not screen._model_thinking, "model waits for card presentation")
	screen.learned_battle_mode = false
	await create_timer(0.5).timeout
	var path := OS.get_environment("DICE_AND_DESTINY_CLEANSE_SCREENSHOT")
	if actor == "goblin" and not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(path)
	await create_timer(1.0).timeout
	_expect(is_instance_valid(ghost) and ghost.modulate.a < 0.8, "status visibly fades")
	_expect(fake.commands.is_empty(), "pass still waits during fade")
	_expect(screen._view.effect_rolls == fixture.snapshot.effect_rolls, "already rolled poison dice are unchanged")
	await create_timer(1.5).timeout
	_expect(not screen._director.has_beats(), "animation finishes without a confirmation popup")
	_expect(fake.commands.size() == 1, "no-choice opponent handoff resumes after animation even with debug pause checked")
	_expect(screen._actor_profiles[actor].statuses.text.contains("Bleed ×1") and not screen._actor_profiles[actor].statuses.text.contains("Poison"), "profile settles to authoritative statuses")
	screen.active_store.clear(); screen.queue_free()
	await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("CARD CLEANSE: " + message)
