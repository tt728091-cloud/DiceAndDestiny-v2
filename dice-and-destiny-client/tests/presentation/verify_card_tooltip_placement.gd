extends SceneTree
const FAN := preload("res://presentation/cards/fanned_hand.gd")
const TIP := preload("res://presentation/cards/card_rules_tooltip.gd")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var stage: Control
var clicked := -1
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	stage = Control.new(); stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); canvas.add_child(stage)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var fixture: Dictionary = gateway.start_battle("card-tooltip-placement", 43)
	BattlePresentationCatalog.configure(fixture.snapshot.content_catalog)
	canvas.notify_mouse_entered()
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1024, 768)]:
		canvas.size = viewport
		# Match the real battle's scaled, letterboxed design canvas.
		stage.size = Vector2(1920, 1080); stage.scale = Vector2.ONE * minf(viewport.x / 1920.0, viewport.y / 1080.0)
		stage.position = (Vector2(viewport) - stage.size * stage.scale) * 0.5
		var hand_bounds := stage.get_global_transform_with_canvas() * Rect2(475, 790, 965, 290)
		for id in fixture.snapshot.content_catalog.cards:
			var data := BattlePresentationCatalog.card(id)
			var label := TIP.create(stage, "%s (seat-a-card-001) — %s" % [data.name, data.text])
			var extent := label.custom_minimum_size + Vector2(28, 28)
			var rect := Rect2(TIP.place(hand_bounds, hand_bounds, extent, Rect2(Vector2.ZERO, Vector2(viewport))), extent)
			_expect(not rect.intersects(hand_bounds), "authored card rules fit outside hand: " + id)
			label.free()
		for count in [1, 6, 10]:
			var fan = FAN.new(); stage.add_child(fan); fan.position = Vector2(475, 790); fan.size = Vector2(965, 290)
			for i in count:
				var card := BattleCard.new(); card.configure("hand-" + str(i), ["steady_guard", "strong_swing", "nudge", "try_again", "take_stock", "emergency_ward"][i % 6], true); fan.add_card(card)
				card.pressed.connect(func(): clicked = i)
			for frame in 6: await process_frame
			await _move(fan.get_global_transform_with_canvas() * Vector2(480, 15))
			await create_timer(0.35).timeout
			var indices := [0] if count == 1 else [0, count / 2, count - 1]
			for index in indices:
				var point: Vector2 = fan.get_global_transform_with_canvas() * (fan.base_transforms[index] * Vector2(22, 22))
				await _move(point); await create_timer(1.2).timeout
				var label := _find_tip(canvas)
				if label == null:
					await _move(point + Vector2(1, 1)); await create_timer(1.2).timeout; label = _find_tip(canvas)
				var id := "%d-%d-%d" % [viewport.x, count, index]
				_expect(fan.hovered == index, "correct hovered card " + id)
				_expect(label != null, "real hand hover opens rules " + id)
				if label != null:
					var popup := label.get_window(); var rect := Rect2(Vector2(popup.position), Vector2(popup.size))
					var hand_rect: Rect2 = fan.get_global_transform_with_canvas() * Rect2(Vector2.ZERO, fan.size)
					_expect(label.text == fan.cards[index].tooltip_text, "tooltip follows hovered card " + id)
					_expect(not rect.intersects(hand_rect), "no card in the hand is obscured " + id)
					_expect(rect.end.y <= hand_rect.position.y - 10, "rules appear above the hand " + id)
					_expect(Rect2(Vector2.ZERO, Vector2(viewport)).encloses(rect), "rules stay onscreen " + id)
					_expect(rect.size.x <= 480 and label.get_visible_line_count() == label.get_line_count(), "full wrapped text visible " + id)
					var path := OS.get_environment("DICE_AND_DESTINY_FAN_SCREENSHOTS")
					if not path.is_empty() and DisplayServer.get_name() != "headless" and count == 6:
						RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png(path.path_join("hand-tooltip-" + id + ".png"))
				clicked = -1
				for pressed in [true, false]:
					var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true); await process_frame
				_expect(clicked == index, "tooltip does not block card clicks " + id)
			await _move(Vector2(20, 20)); await create_timer(0.4).timeout
			_expect(_find_tip(canvas) == null and fan.reveal == 0, "leaving hand hides tooltip and collapses fan")
			fan.queue_free(); await process_frame
	# Standalone cards near the top use a free side; bottom cards prefer above.
	var bounds := Rect2(0, 0, 1280, 720)
	for avoid in [Rect2(20, 20, 185, 248), Rect2(1075, 20, 185, 248), Rect2(500, 450, 185, 248)]:
		var rect := Rect2(TIP.place(avoid, avoid, Vector2(468, 180), bounds), Vector2(468, 180))
		_expect(bounds.encloses(rect) and not rect.intersects(avoid), "standalone card fallback avoids card at " + str(avoid))
	print("CARD TOOLTIP PLACEMENT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _move(point: Vector2) -> void:
	var event := InputEventMouseMotion.new(); event.position = point; canvas.push_input(event, true); await process_frame
func _find_tip(node: Node) -> Label:
	if node is Label and node.get_script() == TIP and node.is_visible_in_tree(): return node
	for child in node.get_children(true):
		var found := _find_tip(child)
		if found != null: return found
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CARD TOOLTIP: " + message)
