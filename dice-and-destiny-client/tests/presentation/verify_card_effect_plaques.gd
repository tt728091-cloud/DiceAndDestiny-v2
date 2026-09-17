extends SceneTree

var failed := false
const SUMMARIES := preload("res://content/card_effect_summaries.gd")

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 900)
	var native = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture = native.start_battle("card-plaques", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	var view := BattleViewState.new(); view.apply_result(fixture)
	var cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {})
	for id in cards: _expect(SUMMARIES.TEXT.has(id), "reviewed summary exists for " + id)
	var board := Control.new(); board.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); root.add_child(board)
	var ids := SUMMARIES.TEXT.keys(); ids.sort()
	for mode in ["hand", "disabled", "pending"]:
		for page in range(ceili(ids.size() / 18.0)):
			var displayed: Array[BattleCard] = []
			for index in range(page * 18, mini((page + 1) * 18, ids.size())):
				var card := BattleCard.new(); board.add_child(card)
				card.configure(str(index), ids[index], mode == "hand", mode == "pending")
				card.size = card.custom_minimum_size
				card.position = Vector2(20 + (index % 18 % 6) * 208, 15 + (index % 18 / 6) * 290)
				displayed.append(card)
			for frame in 5: await process_frame
			for card in displayed:
				var plaque := card.get_node("EffectPlaque") as Control
				var label := plaque.find_child("EffectSummary", true, false) as Label
				_expect(label != null and not label.text.is_empty(), card.definition_id + " has visible benefit text")
				_expect(card.get_global_rect().encloses(plaque.get_global_rect()), card.definition_id + " plaque fits " + mode)
				_expect(plaque.position.y >= 43, card.definition_id + " plaque stays below title in " + mode)
				_expect(label.get_line_count() == label.get_visible_line_count(), card.definition_id + " summary is not clipped")
				_expect(label.mouse_filter == Control.MOUSE_FILTER_IGNORE and plaque.mouse_filter == Control.MOUSE_FILTER_IGNORE, "plaque does not intercept card clicks")
				if mode == "pending": _expect(plaque.position.y + plaque.size.y <= card.size.y - 25, "removal badge remains visible")
			if mode == "hand": await _capture("card-plaques-%d" % (page + 1))
			for card in displayed: card.queue_free()
			await process_frame
	print("CARD EFFECT PLAQUES: " + ("FAILED" if failed else "PASSED") + " · %d cards" % ids.size())
	quit(1 if failed else 0)

func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_CINEMATIC_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
