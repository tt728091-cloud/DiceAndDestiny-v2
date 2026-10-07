extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var screen: Control
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.size = Vector2i(1920, 1080)
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("wound-review", 43)
	for step in 2000:
		if not result.get("accepted", false): _expect(false, "native command: " + str(result.get("error"))); break
		if result.snapshot.get("status", "") != "active": break
		for actor_id in result.snapshot.actors:
			var committed := 0
			for wound in result.snapshot.get("wounds", []):
				if wound.target_actor_id == actor_id: committed += wound.cards.size()
			_expect(committed == int(result.snapshot.actors[actor_id].get("removed_count", 0)), "live wounds match committed losses throughout combat")
		if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
		else:
			var actions: Array = result.get("legal_actions", [])
			var action: Dictionary = {}
			for kind in ["planning_roll", "planning_reroll", "roll_dice", "planning_select_ability", "planning_select_targets", "planning_pass", "pass", "commit_interaction"]:
				for candidate in actions:
					if candidate.type == kind: action = candidate; break
				if not action.is_empty(): break
			if action.is_empty(): _expect(false, "no action at " + str(result.snapshot.get("stage"))); break
			result = gateway.submit(JSON.stringify(action))
	_expect(result.get("accepted", false) and result.snapshot.get("status", "active") != "active", "real native battle reaches completion")
	var wounds: Array = result.snapshot.get("wounds", [])
	_expect(wounds.size() > 1, "native completion has multiple wounds")
	var totals := {}; var cards_seen := {}; var sources := {}
	for wound in wounds:
		var owner: String = wound.target_actor_id
		_expect(result.snapshot.actors.has(owner), "actor alias mapped in wound")
		var source_key := str(wound.batch_id) + ":" + str(wound.source_id)
		_expect(not sources.has(source_key), "source remains one wound per batch"); sources[source_key] = true
		totals[owner] = int(totals.get(owner, 0)) + wound.cards.size()
		for card in wound.cards:
			_expect(not cards_seen.has(card.card_id), "lost instance recorded once"); cards_seen[card.card_id] = true
			_expect(not str(card.get("card_definition_id", "")).is_empty(), "exact card definition retained")
	for owner in result.snapshot.actors:
		_expect(int(totals.get(owner, 0)) == int(result.snapshot.actors[owner].get("removed_count", 0)), "wounds match actual damage losses for " + owner)
	var fixture := result.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	for viewport in [Vector2i(1280, 720), Vector2i(1920, 1080), Vector2i(1024, 768)]:
		fixture.battle_result = "victory" if viewport.x == 1280 else "defeat" if viewport.x == 1920 else "draw"
		canvas.size = viewport
		if viewport.x == 1024:
			for id in ["goblin-3", "goblin-4"]:
				fixture.snapshot.actors[id] = fixture.snapshot.actors.goblin.duplicate(true)
				fixture.snapshot.actors[id].removed_count = 0
				fixture.snapshot.actors[id].current_health = fixture.snapshot.actors[id].max_health
		screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.learned_battle_mode = true; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("wounds-ui.json")); canvas.add_child(screen); screen.set_process(false)
		for frame in 10: await process_frame
		var button: Button = screen._center.get_node_or_null("ReviewBattle")
		_expect(button != null, "completion screen exposes Review button")
		if button == null: screen.queue_free(); await process_frame; continue
		# Check the right edges too: the second enemy HUD overlaps the result
		# buttons here, so a center-only click misses the reported regression.
		for profile in screen._actor_profiles.values():
			_expect(screen._center_scroll.z_index > profile.get_parent().z_index, "completion draws above every actor HUD")
		var actions := 0
		for control in screen._center.get_children():
			if not control is Button: continue
			actions += 1
			var bounds: Rect2 = control.get_global_rect()
			await _move(Vector2(bounds.end.x-12,bounds.get_center().y))
			_expect(canvas.gui_get_hovered_control() == control, "right edge of completion action receives pointer: " + control.text)
			await create_timer(1.2).timeout
			_expect(not _has_visible_popup(canvas), "completion action does not open an empty hover box: " + control.text)
			for profile in screen._actor_profiles.values():
				var overlap: Rect2 = bounds.intersection(profile.get_global_rect())
				if not overlap.has_area(): continue
				await _move(overlap.get_center())
				_expect(canvas.gui_get_hovered_control() == control, "overlapping enemy HUD cannot intercept completion action")
		_expect(actions == 3, "Review, Rematch and New Battle remain available")
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/completion-layer-%d.png" % viewport.x)
		await _click(button)
		var review = screen._root.get_node_or_null("WoundReview")
		_expect(review != null, "pointer opens review")
		if review == null: screen.queue_free(); await process_frame; continue
		for actor in review.tabs:
			await _click(review.tabs[actor])
			var own: Array = wounds.filter(func(w): return w.target_actor_id == actor)
			_expect(review.rows.get_child_count() == maxi(1, own.size()), "each hit renders a separate wound group")
			_expect(Rect2(Vector2.ZERO, Vector2(viewport)).encloses(review.panel.get_global_rect()), "review fits " + str(viewport))
			if own.is_empty(): continue
			var row: Button = review.rows.get_child(0).get_child(0).get_child(2)
			await _move(row.get_global_rect().get_center())
			_expect(is_instance_valid(review.preview), "hover previews lost card")
			if is_instance_valid(review.preview):
				_expect(review.preview.size == BattleCard.STANDARD_SIZE, "preview matches hand card size")
				_expect(not review.preview.get_global_rect().intersects(review.scroll.get_global_rect()), "preview never overlaps card list")
			var path := OS.get_environment("DICE_AND_DESTINY_FAN_SCREENSHOTS")
			if not path.is_empty() and DisplayServer.get_name() != "headless":
				RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png(path.path_join("wounds-%d-%s.png" % [viewport.x, actor]))
		await _click(review.close_button)
		_expect(not screen._root.has_node("WoundReview"), "Close removes overlay")
		await _click(button)
		_expect(screen._root.has_node("WoundReview"), "review reopens")
		var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; canvas.push_input(escape, true); await process_frame
		_expect(not screen._root.has_node("WoundReview"), "Escape closes review")
		screen.queue_free(); await process_frame
	# Maximum supported roster; long scroll, zero wounds, and older missing records.
	var names := {"blade": "Adventurer", "goblin": "Brine Mask 1", "goblin-2": "Brine Mask 2", "goblin-3": "Blade Warden", "goblin-4": "Venom"}
	var data: Dictionary = fixture.snapshot.duplicate(true)
	data.actors["goblin-3"] = {"removed_count": 0}; data.actors["goblin-4"] = {"removed_count": 4}
	var review := preload("res://presentation/battle/wound_review.gd").new()
	review.configure(data, names, "blade")
	canvas.size = Vector2i(1920, 1080); canvas.add_child(review)
	for frame in 8: await process_frame
	_expect(review.tabs.size() == 5, "review supports player plus four enemies")
	for tab in review.tabs.values(): _expect(review.panel.get_global_rect().encloses(tab.get_global_rect()), "all actor tabs fit")
	review.scroll.scroll_vertical = 100000
	for frame in 6: await process_frame
	var last: Control = review.rows.get_child(review.rows.get_child_count() - 1)
	_expect(review.scroll.get_global_rect().intersects(last.get_global_rect()), "last wound reachable by scrolling")
	await _click(review.tabs["goblin-3"])
	_expect(review.summary.text.contains("0 wounds") and review.rows.get_child(0).text == "No recorded wounds.", "empty character has honest no-wound state")
	await _click(review.tabs["goblin-4"])
	_expect(review.rows.get_child(0).text.contains("4 other removed cards"), "old or non-damage losses never fabricated into wounds")
	review.queue_free(); await process_frame
	print("WOUND REVIEW: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _move(point: Vector2) -> void:
	var event := InputEventMouseMotion.new(); event.position = point; canvas.push_input(event, true)
	for frame in 4: await process_frame
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center(); await _move(point)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true)
		for frame in 4: await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("WOUND REVIEW: " + message)
func _has_visible_popup(node: Node) -> bool:
	if node is Popup and node.visible: return true
	for child in node.get_children(true):
		if _has_visible_popup(child): return true
	return false
