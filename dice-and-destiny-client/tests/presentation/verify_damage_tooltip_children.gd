extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const FEEDBACK := preload("res://presentation/battle/damage_response_feedback.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var fixture: Dictionary = gateway.start_battle("tooltip-children", 43)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.unified_defense = false
	fixture.snapshot.segment = "damage_resolution"; fixture.snapshot.stage = "damage_reaction"
	var source := {"id": "incoming", "target_actor_id": "blade", "source_actor_id": "goblin", "source_content_id": "brine_lash", "base_amount": 3, "final_amount": 3}
	var removals: Array = []
	for i in 3:
		removals.append({"card_id": "threatened-" + str(i), "card_definition_id": "curse_bloom", "target_actor_id": "blade", "original_zone": "discard", "accepted": true, "released": false, "damage_proposal_ids": ["incoming"]})
	fixture.snapshot.settled_damage = {"id": "tooltip-batch", "sources": [source], "removals": removals}
	var screen = SCREEN.instantiate(); screen.initial_result = fixture
	screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("tooltip-children.json"))
	root.add_child(screen); screen.set_process(false)
	await create_timer(0.8).timeout
	var grid: Control = screen._damage_grids[0]
	grid.started_ms = Time.get_ticks_msec() - 5000
	grid.refresh_playback()
	var cards: Array[BattleCard] = grid.card_children()
	_expect(cards.size() == 3, "fixture contains three threatened cards")
	var pending: Array = []
	for i in cards.size():
		var removal: Dictionary = removals[i].duplicate(true)
		removal.origin_rect = grid.visible_card_rect(cards[i])
		pending.append(removal)
	# Reproduce native tooltip attachment, with non-card children interleaved
	# so both type safety and card-only ordering/timing are exercised.
	var popup := PopupPanel.new(); grid.add_child(popup); grid.move_child(popup, 0)
	var decoration := Control.new(); decoration.mouse_filter = Control.MOUSE_FILTER_IGNORE
	decoration.position = Vector2(7, 11); decoration.modulate.a = 0.4
	grid.add_child(decoration); grid.move_child(decoration, 2)
	var helper := Node.new(); grid.add_child(helper)
	grid._layout_cards(); grid.refresh_playback()
	_expect(grid.card_children() == cards, "tooltips and helpers do not change card identities or order")
	_expect(grid.custom_minimum_size.y == 3 * grid.STRIDE, "popup does not add a card row")
	for i in cards.size():
		_expect(cards[i].position.y == i * grid.STRIDE and cards[i].modulate.a == 1, "layout and reveal operate only on cards")
	_expect(decoration.position == Vector2(7, 11) and is_equal_approx(decoration.modulate.a, 0.4), "unrelated controls are untouched")
	var motion := InputEventMouseMotion.new()
	motion.position = grid.get_global_transform_with_canvas() * grid.origin_icon_rect(cards[0]).get_center()
	root.push_input(motion, true); await process_frame; root.push_input(motion, true)
	grid._update_hover()
	_expect(grid.tooltip_text == "Pulled from discard pile", "pile tooltip works with popup attached")
	# Keep the popup attached across repeated playback and hover frames.
	await create_timer(1.0).timeout
	grid._update_hover()
	_expect(grid.tooltip_text == "Pulled from discard pile", "pile hover survives repeated playback frames")
	motion.position = grid.visible_card_rect(cards[0]).get_center()
	root.push_input(motion, true); await process_frame; grid._update_hover()
	_expect(grid.hovered_id == cards[0].instance_id and is_instance_valid(grid._hover_card), "full card preview still opens")
	var feedback := FEEDBACK.new(); screen._root.add_child(feedback)
	feedback.configure({"source_id": "incoming", "started_ms": Time.get_ticks_msec(), "instance_id": "played", "card_id": "brace", "actor_id": "blade", "before": 4, "after": 3, "saved": [], "pending": pending}, screen)
	feedback.set_process(false)
	feedback.present_progress(1.0)
	_expect(grid in feedback._hidden_grids and grid.modulate.a == 0, "pending-card feedback matches cards despite popup child")
	_expect(feedback._pending.all(func(entry): return entry.card.visible), "pending headers remain visible during handoff")
	feedback.present_progress(2.3)
	_expect(grid.modulate.a == 1, "feedback restores live stack after handoff")
	feedback.queue_free(); await process_frame
	grid.removal_started_ms = Time.get_ticks_msec()
	grid.refresh_playback()
	_expect(is_instance_valid(grid._tear) and grid._tear.fragments.size() == 6, "removal creates two fragments per card only")
	for card in cards: _expect(card.modulate.a == 0, "committed card gives way to tear animation")
	for frame in 6: await process_frame
	popup.free(); decoration.free(); helper.free()
	grid._layout_cards()
	_expect(grid.card_children() == cards and grid.custom_minimum_size.y == 72, "closing popup preserves stack")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE TOOLTIP CHILDREN: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE TOOLTIP CHILDREN: " + message)
