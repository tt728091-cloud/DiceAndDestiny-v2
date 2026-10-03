extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const EFFECTS := preload("res://presentation/battle/automatic_effects.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("curse-conversion-counter", 9)
	var before := {"health": 11, "energy": 0, "deck_count": 8, "hand_count": 1, "discard_count": 2, "removed_count": 5, "statuses": [{"definition_id": "curse_count", "stacks": 7}]}
	var after := before.duplicate(true); after.health = 9; after.deck_count = 6; after.removed_count = 7; after.statuses[0].stacks = 1
	var removals := []
	for i in 2: removals.append({"card_id": "lost-%d" % i, "card_definition_id": "brine_surge", "original_zone": "deck", "target_actor_id": "goblin", "accepted": true, "damage_proposal_ids": ["curse-loss"]})
	var summary := {"actors_before": {"goblin": before}, "actors_after": {"goblin": after}, "steps": [
		{"type": "curse_resolved", "actor_id": "goblin", "data": {"kind": "conversion", "count_before": 7, "damage": 2, "count_after": 1}},
		{"type": "damage_committed", "data": {"sources": [{"id": "curse-loss", "target_actor_id": "goblin", "source_content_id": "curse_count", "final_amount": 2}], "removals": removals}}
	]}
	for width in [1920, 1280]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		var fixture := base.duplicate(true)
		fixture.events = [{"sequence": 1000, "type": "effects_resolved", "segment": "ongoing_effects", "round": 3, "data": summary}]
		fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
		fixture.snapshot.segment = "income"; fixture.snapshot.round = 3
		for key in ["deck_count", "hand_count", "discard_count", "removed_count", "statuses"]: fixture.snapshot.actors.goblin[key] = after[key]
		fixture.snapshot.actors.goblin.current_health = after.health
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("curse-conversion-counter.json")); root.add_child(screen)
		await process_frame
		var panel = screen._effects_panel; panel.set_process(false)
		_expect(panel.entries.size() == 1 and panel.entries[0].cards.size() == 2, "seven Count removes two cards")
		_expect(is_equal_approx(panel.duration, 5.2), "Curse-only sequence ends with card removal")
		# Sample the whole old closeout interval too: the damage box must never
		# turn into 7, 6, 5...1, including replay/inspection at a saved late time.
		for tick in range(0, 73):
			panel.resume_at(tick / 10.0); panel.present_progress()
			_expect(panel.entries[0].die.text == "2" and panel.entries[0].die.rotation == 0.0, "damage stays fixed at every timestamp")
		panel.resume_at(3.8); panel.present_progress()
		for card in panel.entries[0].cards_ui: _expect(card.modulate.a > 0.99, "both removed cards remain visible before dissolve")
		for frame in 8: await process_frame
		var directory := OS.get_environment("DICE_AND_DESTINY_CURSE_COUNTER_SCREENSHOTS")
		if not directory.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(directory.path_join("damage-%d.png" % width))
		panel.resume_at(5.2); panel.present_progress()
		_expect("Curse Count ×1" in screen._actor_profiles.goblin.statuses.text, "retained Count settles directly to one")
		for card in panel.entries[0].cards_ui: _expect(card.modulate.a == 0.0, "cards finish dissolving before advance")
		panel._process(0); await process_frame
		_expect(screen._director.peek().get("type") != "effects_resolved", "automatically moves on without extra counter phase")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	# Other real effect rolls retain their full timeline while Curse's box stays fixed.
	var mixed := summary.duplicate(true)
	mixed.steps.append({"type": "proposal_batch_committed", "data": {"rolls": [{"actor_id": "blade", "source_content_id": "poison", "die": {"face": 6}}]}})
	var effects = EFFECTS.new(); root.add_child(effects); effects.configure(mixed, {"blade": "Curse", "goblin": "Brine Mask"}); effects.set_process(false)
	_expect(is_equal_approx(effects.duration, 7.2), "mixed effects preserve real dice timeline")
	effects.resume_at(0.4); effects.present_progress()
	for entry in effects.entries:
		if entry.face > 0: _expect(entry.die.rotation != 0.0, "real Poison roll still animates")
	effects.resume_at(5.5); effects.present_progress()
	for entry in effects.entries:
		if entry.status_id == "curse_count": _expect(entry.die.text == "2", "Curse damage stays fixed beside other effects")
	effects.queue_free(); await process_frame
	print("CURSE CONVERSION COUNTER: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("CURSE CONVERSION COUNTER: " + message)
