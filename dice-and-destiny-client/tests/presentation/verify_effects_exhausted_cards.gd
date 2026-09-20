extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const CARD_GRID := preload("res://presentation/cards/damage_card_grid.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = gateway.start_battle("effects-exhausted-cards", 1789756018528910)
	var summary: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/effects_exhausted_cards.json"))
	fixture.events = [{"sequence": 1, "type": "effects_resolved", "segment": "ongoing_effects", "round": 5, "data": summary}]
	fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.segment = "ongoing_effects"; fixture.snapshot.round = 5
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("effects-exhausted-test.json")); screen._auto_pass_disabled = true
	root.add_child(screen); screen.set_process(false)
	var panel = screen._effects_panel
	panel.set_paused(true)
	var card_ids := {}; var volatile_cards := 0; var excess := 0; var enemy_cards := 0
	for entry in panel.entries:
		if entry.actor_id != "goblin": continue
		for card in entry.cards:
			_expect(not card_ids.has(card.card_id), "no card is displayed twice")
			card_ids[card.card_id] = true
		enemy_cards += entry.cards.size()
		if entry.status_id == "volatile_poison":
			volatile_cards += entry.cards.size(); excess += int(entry.excess)
			_expect(entry.cards_ui.size() == 2, "both damage points represented under every Volatile Poison roll")
			_expect(entry.cards.size() + int(entry.excess) == 2, "actual loss plus excess accounts for full damage")
	_expect(enemy_cards == 6 and volatile_cards == 3 and excess == 3, "real battle removed all six remaining cards, with three excess damage")
	_expect("3 excess damage; no cards left to remove" in screen._view.combat_log.text(), "excess damage explained in combat log")
	for viewport_size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport_size
		panel.set_paused(false); panel.resume_at(3.8 + panel.catalyst_extra_seconds()); panel.set_paused(true); panel.present_progress()
		for frame in 12: await process_frame
		var rects := []
		for entry in panel.entries:
			for tile in entry.cards_ui:
				_expect(tile.modulate.a > 0.99, "every card and excess marker visible together")
				_expect(tile.get_global_rect().size.is_equal_approx(CARD_GRID.CARD_SIZE * screen._root.scale), "full card size preserved")
				_expect(screen._center_scroll.get_global_rect().encloses(tile.get_global_rect()), "every slot fits in play area")
				for rect in rects: _expect(not tile.get_global_rect().grow(-0.1).intersects(rect), "slots never overlap")
				rects.append(tile.get_global_rect())
		var directory := OS.get_environment("DICE_AND_DESTINY_EFFECTS_SCREENSHOTS")
		if not directory.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(directory.path_join("effects-exhausted-%d.png" % viewport_size.x))
	panel.set_paused(false); panel.resume_at(6.3 + panel.catalyst_extra_seconds()); panel.set_paused(true); panel.present_progress()
	for entry in panel.entries:
		if entry.excess > 0: _expect("excess" in entry.result.text, "settled roll still explains excess damage")
		for tile in entry.cards_ui: _expect(tile.modulate.a == 0.0, "cards and markers finish together")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("EFFECTS EXHAUSTED CARDS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("EFFECTS EXHAUSTED CARDS: " + message)
