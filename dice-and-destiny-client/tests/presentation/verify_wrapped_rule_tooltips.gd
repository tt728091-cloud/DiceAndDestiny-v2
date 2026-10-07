extends SceneTree
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const WRAPPED := preload("res://presentation/battle/wrapped_tooltip.gd")
var failed := false
var canvas: SubViewport
var stage: Control
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas)
	stage = Control.new(); stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); canvas.add_child(stage)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var fixture: Dictionary = gateway.start_battle("wrapped-rules", 43)
	BattlePresentationCatalog.configure(fixture.snapshot.content_catalog)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1024, 768)]:
		canvas.size = viewport
		stage.size = Vector2(viewport)
		# Exercise all authored rules, including long poison and curse explanations.
		for category in ["abilities", "cards"]:
			for id in fixture.snapshot.content_catalog.get(category, {}):
				var data: Dictionary = BattlePresentationCatalog.ability(id) if category == "abilities" else BattlePresentationCatalog.card(id)
				var value := str(data.name) + "\n\n" + str(data.text)
				var label := WRAPPED.create(stage, value); stage.add_child(label)
				for frame in 2: await process_frame
				_expect(label.text == value and label.autowrap_mode == TextServer.AUTOWRAP_WORD_SMART, "complete rules wrap: " + id)
				_expect(label.size.x <= 441 and label.size.y < viewport.y - 64, "rules fit readable bounds: " + id)
				label.queue_free(); await process_frame
		for right in [false, true]:
			var tile := BattleAbilityTile.new(); stage.add_child(tile)
			tile.configure("adventurer_guard_plus", true, false, true, fixture.snapshot.actors.blade); tile.cinematic_compact(); tile.minimal_rail()
			tile.position = Vector2(viewport.x - 390 if right else 12, viewport.y - 90)
			for frame in 3: await process_frame
			await _hover(tile, tile.get_global_rect().position + Vector2(40, 20), "guard-%d-%s" % [viewport.x, right])
			for child in tile.get_children():
				if child is Button and child.text == "ⓘ":
					await _hover(child, child.get_global_rect().get_center(), "guard-info-%d-%s" % [viewport.x, right])
			tile.queue_free(); await process_frame
			for boosted in [false, true]:
				var actor := {"ability_modifiers": [], "statuses": []}
				if boosted:
					actor.ability_modifiers = [{"ability_id": "adventurer_small_straight", "bonus_id": "strong_swing", "status_id": "strong_swing_ready", "expires_after_offensive": true}]
					actor.statuses = [{"definition_id": "strong_swing_ready", "stacks": 1}]
				var straight := BattleAbilityTile.new(); stage.add_child(straight)
				straight.configure("adventurer_small_straight", true, false, true, actor); straight.cinematic_compact(); straight.minimal_rail()
				straight.position = Vector2(viewport.x - 390 if right else 12, viewport.y - 110)
				_expect(straight.tooltip_text.count("Small Straight") == 1, "one Small Straight heading")
				_expect(straight.tooltip_text.count("Strong Swing:") == int(boosted) and straight.tooltip_text.count("even if unused") == int(boosted), "bonus and expiry shown once only when active")
				for frame in 3: await process_frame
				await _hover(straight, straight.get_global_rect().position + Vector2(40, 20), "straight-%d-%s-%s" % [viewport.x, right, boosted])
				straight.queue_free(); await process_frame
			var card := BattleCard.new(); stage.add_child(card); card.configure("test-card", "brace", true)
			card.position = Vector2(viewport.x - 205 if right else 12, viewport.y - 270)
			await process_frame
			await _hover(card, card.get_global_rect().get_center(), "card-%d-%s" % [viewport.x, right])
			card.queue_free(); await process_frame
		var status := preload("res://presentation/battle/status_tooltip_label.gd").new()
		status.mouse_filter = Control.MOUSE_FILTER_STOP
		status.text = "Status rules"; status.tooltip_text = "Protect\n" + str(BattlePresentationCatalog.status("protect").text)
		stage.add_child(status); status.position = Vector2(viewport.x - 160, viewport.y - 40)
		await process_frame
		await _hover(status, status.get_global_rect().get_center(), "status-%d" % viewport.x)
		status.queue_free(); await process_frame
	print("WRAPPED RULE TOOLTIPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _hover(control: Control, point: Vector2, suffix: String) -> void:
	if DisplayServer.get_name() == "headless": return
	canvas.notify_mouse_entered()
	var motion := InputEventMouseMotion.new(); motion.position = Vector2(500, 30); canvas.push_input(motion, true)
	await process_frame
	motion = InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true)
	await create_timer(1.2).timeout
	var label := _find_label(canvas, control.tooltip_text)
	# Initial window/layout notifications can cancel the first hover timer.
	if label == null:
		canvas.notify_mouse_entered()
		motion = InputEventMouseMotion.new(); motion.position = point + Vector2(1, 1); canvas.push_input(motion, true)
		await create_timer(1.2).timeout
		label = _find_label(canvas, control.tooltip_text)
	_expect(label != null, "actual hover opens " + suffix)
	if label != null:
		var window := label.get_window()
		_expect(label.get_visible_line_count() == label.get_line_count(), "every wrapped line is visible " + suffix)
		_expect(label.text == control.tooltip_text and label.get_line_count() > 1, "full rules wrap over multiple lines " + suffix)
		_expect(window.size.x <= 480, "popup stays a reasonable width " + suffix)
		_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(window.position), Vector2(window.size))), "popup stays inside viewport " + suffix)
	var directory := OS.get_environment("DICE_AND_DESTINY_TOOLTIP_SCREENSHOTS")
	if not directory.is_empty():
		RenderingServer.force_draw(false)
		canvas.get_texture().get_image().save_png(directory.path_join("wrapped-" + suffix + ".png"))
	canvas.notify_mouse_exited(); await process_frame
func _find_label(node: Node, value: String) -> Label:
	if node is Label and node.text == value and node.is_visible_in_tree(): return node
	for child in node.get_children(true):
		var found := _find_label(child, value)
		if found != null: return found
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("WRAPPED RULE TOOLTIPS: " + message)
