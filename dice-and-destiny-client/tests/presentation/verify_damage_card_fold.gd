extends SceneTree
## Threatened-card lists start folded. Cards an enemy attack threatens hang
## beneath that attack's badge; the player's outgoing attacks stay under the
## enemy HUD. Folded lists keep pile totals and skip card flights and tears.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/combat_timing.gd")
const FEEDBACK := preload("res://presentation/battle/damage_response_feedback.gd")
const ZONES := ["discard", "deck", "hand", "hand", "deck", "hand"]
var failed := false
func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	var base: Dictionary = gateway.start_battle("damage-card-fold", 7)
	for unified in [false, true]:
		var screen = await _screen(_fixture(base, unified))
		await _check(screen, unified)
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE CARD FOLD: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _source(sources: Array, removals: Array, id: String, attacker: String, target: String, ability: String, amount: int) -> void:
	sources.append({"id": id, "target_actor_id": target, "source_actor_id": attacker, "source_content_id": ability, "base_amount": amount, "final_amount": amount})
	for index in amount:
		removals.append({"card_id": id + "-" + str(index), "card_definition_id": "steady_guard", "target_actor_id": target, "original_zone": ZONES[index], "accepted": true, "released": false, "damage_proposal_ids": [id]})

func _fixture(base: Dictionary, unified: bool) -> Dictionary:
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.unified_defense = unified
	fixture.snapshot.segment = "defensive" if unified else "damage_resolution"
	fixture.snapshot.stage = "defense_selection" if unified else "damage_reaction"
	var sources: Array = []; var removals: Array = []
	# Brine Mask 1 attacks twice, so its second badge must clear the first list.
	_source(sources, removals, "in-a", "goblin", "blade", "brine_lash", 6)
	_source(sources, removals, "in-b", "goblin", "blade", "brine_lash", 2)
	_source(sources, removals, "in-c", "goblin-2", "blade", "brine_lash", 4)
	_source(sources, removals, "out-1", "blade", "goblin", "guarded_strike", 4)
	_source(sources, removals, "out-2", "blade", "goblin-2", "guarded_strike", 3)
	fixture.snapshot.settled_damage = {"id": "fold", "sources": sources, "removals": removals}
	if unified: fixture.snapshot.damage_sources = sources.duplicate(true)
	return fixture

func _screen(fixture: Dictionary) -> Control:
	var screen = SCREEN.instantiate(); screen.initial_result = fixture
	screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("damage-card-fold.json"))
	root.add_child(screen); screen.set_process(false)
	await create_timer(0.75).timeout
	for frame in 6: await process_frame
	return screen

func _grid(screen: Control, source_id: String) -> Control:
	for grid in screen._damage_grids:
		if grid.source_id == source_id: return grid
	return null

func _heading(screen: Control, source_id: String) -> Button:
	return _grid(screen, source_id).get_parent().get_node("StackHeading").get_child(0)

func _chevron(screen: Control, source_id: String) -> Button:
	return _grid(screen, source_id).get_parent().get_node("StackHeading").find_child("FoldCards_*", false, false)

func _expected(grid: Control) -> Dictionary:
	var expected := {}
	for card in grid.card_children(): expected[card.get_meta("removal_origin_zone")] = int(expected.get(card.get_meta("removal_origin_zone"), 0)) + 1
	return expected

func _check(screen: Control, unified: bool) -> void:
	var label := "unified" if unified else "legacy"
	await _capture("damage-fold-default-%s.png" % label)
	_expect(not screen._damage_stack_docks.has("blade") and screen._root.find_child("DamageStacks_blade", true, false) == null, "%s: no lower-right player stack" % label)
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	for id in ["in-a", "in-b", "in-c", "out-1", "out-2"]:
		var grid: Control = _grid(screen, id)
		_expect(grid != null and not grid.is_visible_in_tree(), "%s: %s list starts folded" % [label, id])
		if grid == null: return
		var row: Control = grid.get_parent().get_node("StackHeading")
		var tally: Control = row.get_node("RemovalTally")
		_expect(tally.is_visible_in_tree() and tally.counts == _expected(grid), "%s: %s totals each pile: %s" % [label, id, tally.counts])
		for zone in tally.counts: _expect(tally.get_node("Count_" + zone).text == str(tally.counts[zone]), "%s: %s shows %s count" % [label, id, zone])
		if id.begins_with("in-"):
			var badge: Control = screen._attack_intents[id].intent
			var dock: Control = screen._damage_stack_docks["attack:" + id]
			var badge_rect: Rect2 = inverse * badge.get_global_rect()
			var dock_rect: Rect2 = inverse * dock.get_global_rect()
			_expect(badge.is_visible_in_tree() and dock.is_visible_in_tree(), "%s: %s badge carries its list" % [label, id])
			_expect(absf(dock_rect.position.y - badge_rect.end.y - 4) < 2 and absf(dock_rect.position.x - badge_rect.position.x) < 2, "%s: %s list hangs beneath its badge: %s vs %s" % [label, id, dock_rect, badge_rect])
			_expect(_heading(screen, id).text == "%d cards" % grid.card_children().size(), "%s: %s heading counts its cards" % [label, id])
			_expect(dock_rect.size.x < dock.column_width, "%s: folded %s row is only as wide as its text" % [label, id])
	_expect("from hand" in _grid(screen, "in-a").get_parent().find_child("RemovalTally", true, false).tooltip_text, "%s: tally hover names its pile" % label)
	if unified:
		var row: Control = screen._incoming_attack_rows.get("in-a")
		_expect(row != null and row.find_child("RemovalTally", true, false).counts == screen._damage_zone_counts["in-a"], "incoming attack row shows pile totals")

	# Pointer: the badge and its card-count row never fold; only the chevron does.
	await _click(screen._attack_intents["in-a"].intent)
	for frame in 3: await process_frame
	_expect(not _grid(screen, "in-a").is_visible_in_tree(), "%s: clicking the attack badge leaves its cards folded" % label)
	if unified: _expect(screen._selected_source == "in-a", "unified: clicking the attack badge selects it for defense")
	await _click(_heading(screen, "in-a"))
	for frame in 3: await process_frame
	_expect(not _grid(screen, "in-a").is_visible_in_tree(), "%s: clicking the card count leaves its cards folded" % label)
	await _click(_chevron(screen, "in-a"))
	for frame in 3: await process_frame
	_expect(_grid(screen, "in-a").is_visible_in_tree(), "%s: the chevron shows the attack's cards" % label)
	_expect(not _grid(screen, "in-b").is_visible_in_tree() and not _grid(screen, "in-c").is_visible_in_tree(), "%s: other attacks stay folded" % label)
	var list_rect: Rect2 = inverse * screen._damage_stack_docks["attack:in-a"].get_global_rect()
	var first: Rect2 = inverse * screen._attack_intents["in-a"].intent.get_global_rect()
	var second: Rect2 = inverse * screen._attack_intents["in-b"].intent.get_global_rect()
	_expect(is_equal_approx(second.position.y, first.position.y) and not second.intersects(first) and not second.intersects(list_rect), "%s: while a list is open, the enemy's other badge sits beside it: %s vs %s" % [label, second, first])
	# The other attack keeps only its chevron, beside its badge, clear of the list.
	var sibling_chevron: Rect2 = inverse * _chevron(screen, "in-b").get_global_rect()
	_expect(_chevron(screen, "in-b").is_visible_in_tree() and not _heading(screen, "in-b").is_visible_in_tree(), "%s: the other attack keeps only its badge and chevron" % label)
	_expect(not sibling_chevron.intersects(list_rect) and not sibling_chevron.intersects(first) and not sibling_chevron.intersects(second) and sibling_chevron.position.x >= second.end.x and absf(sibling_chevron.get_center().y - second.get_center().y) < 2, "%s: its chevron sits beside its badge: %s vs %s" % [label, sibling_chevron, second])
	var profile: Rect2 = inverse * screen._actor_profiles.goblin.get_global_rect()
	_expect(list_rect.end.y <= profile.position.y, "%s: open list never covers the attacker's name" % label)
	_expect(_grid(screen, "in-a").visible_card_rect(_grid(screen, "in-a").card_children()[4]).size.y >= _grid(screen, "in-a").STRIDE * screen._root.scale.y - 1, "%s: open list shows five full rows" % label)
	if unified and screen._ability_dock.get_parent().visible and screen._ability_dock.has_meta("defense_source"):
		var popup: Rect2 = inverse * screen._ability_dock.get_parent().get_global_rect()
		_expect(not popup.intersects(list_rect) and not popup.intersects(profile), "%s: defense choices sit clear of the open list and the name: %s" % [label, popup])
	await _capture("damage-fold-attack-open-%s.png" % label)
	screen._render(true); for frame in 3: await process_frame
	_expect(_grid(screen, "in-a").is_visible_in_tree(), "%s: unfolded state survives a board rebuild" % label)
	await _click(screen._attack_intents["in-a"].intent)
	for frame in 3: await process_frame
	_expect(_grid(screen, "in-a").is_visible_in_tree(), "%s: clicking the attack badge leaves an open list open" % label)
	# Opening the enemy's other attack by its beside chevron folds the first:
	# one list per enemy, and the first attack's chevron moves beside its badge.
	await _click(_chevron(screen, "in-b"))
	for frame in 3: await process_frame
	_expect(_grid(screen, "in-b").is_visible_in_tree() and not _grid(screen, "in-a").is_visible_in_tree(), "%s: opening a sibling attack folds the first" % label)
	var b_list: Rect2 = inverse * screen._damage_stack_docks["attack:in-b"].get_global_rect()
	var a_chevron: Rect2 = inverse * _chevron(screen, "in-a").get_global_rect()
	_expect(_chevron(screen, "in-a").is_visible_in_tree() and not a_chevron.intersects(b_list) and not a_chevron.intersects(inverse * screen._attack_intents["in-b"].intent.get_global_rect()), "%s: the first attack's chevron stays reachable beside its badge" % label)
	_expect(_heading(screen, "in-b").is_visible_in_tree() and _grid(screen, "in-b").get_parent().get_node("StackHeading").get_node("RemovalTally").is_visible_in_tree(), "%s: the opened attack shows its full heading again" % label)
	await _click(_chevron(screen, "in-b"))
	for frame in 3: await process_frame
	_expect(not _grid(screen, "in-b").is_visible_in_tree(), "%s: the chevron folds it again" % label)
	var restacked: Rect2 = inverse * screen._attack_intents["in-b"].intent.get_global_rect()
	_expect(restacked.position.y > (inverse * screen._attack_intents["in-a"].intent.get_global_rect()).end.y, "%s: all folded, the enemy's badges stack again" % label)
	_expect((inverse * screen._damage_stack_docks["attack:in-b"].get_global_rect()).end.y <= profile.position.y + 1, "%s: stacked rows stay above the name" % label)
	var fold: Button = _chevron(screen, "in-c")
	await _click(fold)
	_expect(_grid(screen, "in-c").is_visible_in_tree() and fold.get_meta("cards_shown") == true, "%s: chevron unfolds the list" % label)
	await _click(fold)

	# Pointer: an enemy's outgoing-damage chevron folds that enemy's list; its heading does not.
	await _click(_heading(screen, "out-1"))
	_expect(not _grid(screen, "out-1").is_visible_in_tree(), "%s: enemy heading leaves its list folded" % label)
	await _click(_chevron(screen, "out-1"))
	_expect(_grid(screen, "out-1").is_visible_in_tree() and not _grid(screen, "out-2").is_visible_in_tree(), "%s: enemy chevron unfolds only that enemy" % label)
	await _click(_chevron(screen, "out-1"))
	_expect(not _grid(screen, "out-1").is_visible_in_tree(), "%s: enemy chevron refolds" % label)

	# Prevention on folded lists animates only the reduction: no card flights.
	for id in ["in-a", "out-1"]:
		var grid: Control = _grid(screen, id)
		var card: BattleCard = grid.card_children()[0]
		var removal := {"card_id": card.instance_id, "card_definition_id": card.definition_id, "target_actor_id": grid.target_actor, "released_destination": "discard", "origin_rect": Rect2(40, 600, 300, 24)}
		var data := {"source_id": id, "card_id": "emergency_ward", "instance_id": "played", "started_ms": Time.get_ticks_msec(), "before": 6, "after": 3, "actor_id": "blade", "saved": [removal], "pending": [removal]}
		var folded = FEEDBACK.new(); screen._root.add_child(folded); folded.configure(data, screen)
		_expect(folded._saved.is_empty() and folded._piles.is_empty() and folded._pending.is_empty(), "%s: folded %s list has no saved-card flight" % [label, id])
		folded.present_progress(1.0)
		_expect(folded._held_groups.is_empty() and grid.get_parent().custom_minimum_size.y == 0, "%s: folded list does not reserve card rows" % label)
		_expect(screen._attack_intents[id].damage.text == "3", "%s: damage reduction still animates" % label)
		folded.queue_free()
		screen._toggle_damage_cards(grid.fold_key)
		var open = FEEDBACK.new(); screen._root.add_child(open); open.configure(data, screen)
		_expect(open._saved.size() == 1, "%s: open %s list keeps its saved-card flight" % [label, id])
		open.queue_free(); await process_frame
		screen._toggle_damage_cards(grid.fold_key)

	# Committed removal tears only the lists that are open.
	screen._toggle_damage_cards("source:in-c")
	await process_frame
	for grid in screen._damage_grids:
		grid.removal_started_ms = Time.get_ticks_msec() - int(TIMING.removal() * 500)
		grid.refresh_playback(); grid.set_process(false)
		var shown: bool = screen.damage_cards_shown(grid.fold_key)
		_expect(is_instance_valid(grid._tear) == shown, "%s: %s list %s" % [label, grid.source_id, "tears" if shown else "skips the card tear while folded"])

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_FOLD_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name))

func _click(button: Control) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; motion.global_position = point
	root.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.position = point; click.global_position = point; click.pressed = pressed
		root.push_input(click, true); await process_frame

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE CARD FOLD: " + message)
