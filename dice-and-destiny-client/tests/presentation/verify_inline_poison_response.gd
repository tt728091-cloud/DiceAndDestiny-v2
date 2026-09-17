extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var base: Dictionary = gateway.start_battle("inline-poison", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loads")
	for has_response in [false, true]:
		var fixture := base.duplicate(true)
		fixture.learned_policy = {}; fixture.events = []
		fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "venom_status_reaction"; fixture.snapshot.presentation_stage = "defense_reaction"
		fixture.snapshot.venom_work = {"Kind": "application", "StatusID": "poison", "Stacks": 1, "TargetActorID": "goblin"}
		fixture.snapshot.damage_sources = [{"id": "incoming", "source_content_id": "sword_cut", "source_actor_id": "goblin", "target_actor_id": "blade", "base_amount": 5, "prevention": 2, "reaction_prevention": 1}]
		fixture.snapshot.defense_selections = {"blade": {"ability_id": "shedskin", "source_id": "incoming", "rolled_faces": [1, 2], "rolled_face": 1}}
		fixture.snapshot.actors.goblin.statuses = []
		fixture.pending_input = {"blade": {"id": "poison-response", "segment": "defensive", "stage": "venom_status_reaction", "allowed_commands": ["pass", "commit_interaction"]}}
		fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "poison-response"}}]
		if has_response: fixture.legal_actions.append({"type": "commit_interaction", "actor_id": "blade", "payload": {}})
		var after := fixture.duplicate(true)
		after.snapshot.stage = "defense_reaction"; after.snapshot.erase("venom_work"); after.snapshot.erase("presentation_stage")
		after.snapshot.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 1}]
		after.events = [{"sequence": 100, "type": "proposal_batch_committed", "segment": "defensive", "data": {"status_application": {"target_actor_id": "goblin", "status_id": "poison", "before": 0, "after": 1}}}]
		after.legal_actions = []; after.pending_input = {}
		var fake := FakeBattleAuthority.new(); fake.enqueue(after)
		var screen = SCREEN.instantiate(); screen.set_process(false); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("inline-poison.json"))
		root.add_child(screen); screen.set_process(false)
		for i in 4: await process_frame
		_expect(screen._defense_result_panels.size() == 1, "defense results remain visible during child response")
		for label in screen.find_children("*", "Label", true, false):
			_expect(label.text != "STATUS APPLICATION" and label.text != "Venom Status Reaction", "no application interstitial")
		_expect(screen._sole_pass_action().is_empty() == has_response, "auto-ack only when no real response is legal")
		screen.set_process(true)
		await create_timer(0.8).timeout
		if has_response:
			_expect(fake.commands.is_empty(), "preserves real response choice")
		else:
			_expect(fake.commands.size() == 1, "background acknowledgement ignores debug auto-pass toggle")
			_expect("Poison ×1" in screen._actor_profiles.goblin.statuses.text, "committed Poison reaches enemy status panel")
			_expect(not screen._director.has_beats(), "application has no separate presentation beat")
			var path := OS.get_environment("DICE_AND_DESTINY_POISON_INLINE_SCREENSHOT")
			if not path.is_empty() and DisplayServer.get_name() != "headless":
				await RenderingServer.frame_post_draw
				root.get_texture().get_image().save_png(path)
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("INLINE POISON RESPONSE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("INLINE POISON: " + message)
