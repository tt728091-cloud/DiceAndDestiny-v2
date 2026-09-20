extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var base: Dictionary = gateway.start_battle("pinprick-antidote-feedback", 1789759004756520)
	base.events = []; base.learned_policy = {}; base.pending_input = {}; base.legal_actions = []
	base.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 2}]
	var reaction := base.duplicate(true)
	reaction.snapshot.actors.blade.hand.pop_back()
	reaction.snapshot.actors.blade.hand_count -= 1
	reaction.snapshot.actors.blade.energy_points -= 1
	reaction.snapshot.actors.blade.discard_count += 1
	reaction.snapshot.stage = "venom_status_reaction"; reaction.snapshot.presentation_stage = "planning"
	reaction.snapshot.venom_work = {"Kind": "application", "StatusID": "poison", "Stacks": 1, "SourceActorID": "blade", "TargetActorID": "goblin"}
	reaction.events = [{"sequence": 100, "type": "card_played", "actor_id": "blade", "segment": "offensive", "data": {"card_definition_id": "pinprick", "card_instance_id": "pinprick"}}]
	var cleansed := reaction.duplicate(true)
	cleansed.snapshot.actors.goblin.hand_count -= 1
	cleansed.snapshot.actors.goblin.energy_points -= 1
	cleansed.snapshot.actors.goblin.discard_count += 1
	cleansed.snapshot.actors.goblin.statuses = []
	cleansed.events = [{"sequence": 101, "type": "card_played", "actor_id": "goblin", "segment": "offensive", "data": {"card_definition_id": "antidote", "card_instance_id": "antidote", "choice_id": "poison", "operation": "remove_status", "stacks_before": 2, "stacks_after": 0, "stacks_removed": 2}}]
	cleansed.pending_input = {"blade": {"id": "respond", "stage": "venom_status_reaction", "segment": "offensive", "allowed_commands": ["pass"]}}
	cleansed.legal_actions = [{"battle_id": base.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "respond"}}]
	var resolved := cleansed.duplicate(true)
	resolved.snapshot.stage = "planning"; resolved.snapshot.erase("venom_work")
	resolved.pending_input = {}; resolved.legal_actions = []
	resolved.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 1}]
	resolved.events = [{"sequence": 102, "type": "proposal_batch_committed", "actor_id": "blade", "segment": "offensive", "data": {"status_application": {"target_actor_id": "goblin", "status_id": "poison", "before": 0, "after": 1}}}]
	var fake := FakeBattleAuthority.new(); fake.enqueue(resolved)
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("pinprick-antidote-feedback.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	await process_frame
	screen._apply_model_result(reaction)
	screen._apply_model_result(cleansed)
	for frame in 4: await process_frame
	_expect(screen._director.peek().get("type") == "card_cleanse", "opponent response immediately presents Antidote without player input")
	var ghost := screen._actor_profiles.goblin.find_child("CleansedStatus", true, false) as Label
	_expect(ghost != null and "Poison ×2" in ghost.text, "original two Poison stay visible for cleanse animation")
	await create_timer(1.6).timeout
	_expect(fake.commands.is_empty(), "Pinprick waits until Antidote is shown")
	var capture := OS.get_environment("DICE_AND_DESTINY_PINPRICK_SCREENSHOT")
	if not capture.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture)
	var deadline := Time.get_ticks_msec() + 6000
	while fake.commands.is_empty() and Time.get_ticks_msec() < deadline: await process_frame
	_expect(fake.commands.size() == 1, "response continues automatically after cleanse")
	await create_timer(1.0).timeout
	_expect("Poison ×1" in screen._actor_profiles.goblin.statuses.text, "Pinprick then adds one Poison to the cleansed actor")
	var log: String = screen._view.combat_log.text()
	_expect(log.find("played Pinprick") < log.find("played Antidote"), "log preserves initiating card and reaction order")
	_expect(log.find("played Antidote") >= 0 and log.find("played Antidote") < log.find("Poison 2 → 0"), "log names Antidote before removal")
	_expect(log.count("Poison 2 → 0") == 1 and log.count("Poison 0 → 1") == 1, "each status change logged once, without duplicate snapshot delta")
	# Re-delivery cannot replay the cleanse or duplicate log entries.
	screen._view.apply_result(resolved); screen._director.queue_result(resolved)
	_expect(screen._view.combat_log.text() == log, "repeated result does not duplicate logs")
	_expect(not screen._director.has_beats(), "repeated result does not replay feedback: " + str(screen._director.peek()))
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("PINPRICK ANTIDOTE FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("PINPRICK ANTIDOTE FEEDBACK: " + message)
