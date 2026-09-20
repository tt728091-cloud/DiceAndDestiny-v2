extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const CARD_GRID := preload("res://presentation/cards/damage_card_grid.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = gateway.start_battle("effects-card-grid", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	var before := {}; var after := {}; var rolls := []; var sources := []; var removals := []
	var definitions := ["incubate", "culture_flask", "distill", "emergency_molt", "forked_tongue", "fever_cycle", "extract", "pinprick", "venom_reserve"]
	for actor in ["blade", "goblin"]:
		before[actor] = {"health": 20, "energy": 3, "deck_count": 20, "hand_count": 0, "discard_count": 0, "removed_count": 0, "statuses": [{"definition_id": "poison", "stacks": 3}, {"definition_id": "volatile_poison", "stacks": 3}]}
		after[actor] = before[actor].duplicate(true); after[actor].health = 11; after[actor].deck_count = 11; after[actor].removed_count = 9
		var card_index := 0
		for status in ["poison", "volatile_poison"]:
			for index in 3:
				var source := "%s-%s-%d" % [actor, status, index]
				var damage := 2 if status == "volatile_poison" else 1
				rolls.append({"actor_id": actor, "source_content_id": status, "die": {"face": index + 1}})
				sources.append({"id": source, "target_actor_id": actor, "source_content_id": status, "final_amount": damage})
				for lost in damage:
					removals.append({"card_id": "%s-card-%d" % [actor, card_index], "card_definition_id": definitions[card_index], "target_actor_id": actor, "original_zone": "deck", "accepted": true, "released": false, "damage_proposal_ids": [source]})
					card_index += 1
	var event := {"sequence": 1, "type": "effects_resolved", "segment": "ongoing_effects", "round": 2, "data": {"actors_before": before, "actors_after": after, "steps": [{"type": "proposal_batch_committed", "data": {"rolls": rolls}}, {"type": "damage_committed", "data": {"sources": sources, "removals": removals}}]}}
	fixture.events = [event]; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"; fixture.snapshot.round = 2
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("effects-card-grid.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	var panel = screen._effects_panel
	panel.set_paused(true)
	_expect(panel.entries.size() == 12 and panel.groups.size() == 4, "three Poison and three Volatile Poison on both sides")
	for viewport_size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport_size
		await _frames()
		_seek(panel, 0.3)
		var initial_rects := []
		for entry in panel.entries:
			initial_rects.append(entry.card_grid.get_global_rect())
			for card in entry.cards_ui: _expect(card.modulate.a == 0.0, "cards reserved but hidden before results")
		_seek(panel, 3.8 + panel.catalyst_extra_seconds())
		await _frames()
		var cards := []; var index := 0
		for entry in panel.entries:
			_expect(entry.card_grid.get_global_rect().is_equal_approx(initial_rects[index]), "roll and result use the same reserved space")
			index += 1
			_expect(entry.cards_ui.size() == (2 if entry.status_id == "volatile_poison" else 1), "every damaging roll shows its actual loss")
			for card in entry.cards_ui:
				_expect(card is BattleCard, "same full card presentation as damage resolution")
				_expect(card.get_global_rect().size.is_equal_approx(CARD_GRID.CARD_SIZE * screen._root.scale), "same card size as damage resolution, with no extra shrink")
				_expect(card.modulate.a > 0.99, "all removed cards visible together")
				_expect(card.get_node("RemovalState").text == "× REMOVED" and not "PENDING" in card.text, "committed cards labeled Removed")
				_expect(screen._center_scroll.get_global_rect().encloses(card.get_global_rect()), "card fully inside central play area")
				_expect(entry.cell.get_global_rect().encloses(card.get_global_rect()), "card belongs beneath its own roll")
				for other in cards: _expect(not card.get_global_rect().grow(-0.1).intersects(other.get_global_rect()), "cards never overlap")
				cards.append(card)
		_expect(cards.size() == 18, "all eighteen losses fit simultaneously")
		_expect(not screen._center_scroll.get_v_scroll_bar().visible and not screen._center_scroll.get_h_scroll_bar().visible, "no effects scrollbars")
		await _capture("effects-full-grid-%d" % viewport_size.x)
	_seek(panel, 5.3)
	for entry in panel.entries:
		for card in entry.cards_ui: _expect(card.modulate.a == 0.0, "removed cards finish dissolving")
	_expect(fake.commands.is_empty(), "presentation does not submit gameplay commands")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("EFFECTS CARD GRID: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _seek(panel: Node, seconds: float) -> void:
	panel.set_paused(false); panel.resume_at(seconds); panel.set_paused(true); panel.present_progress()

func _frames() -> void:
	for frame in 12: await process_frame

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_EFFECTS_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("EFFECTS CARD GRID: " + message)
