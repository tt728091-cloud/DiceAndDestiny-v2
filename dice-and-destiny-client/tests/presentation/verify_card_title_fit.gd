extends SceneTree

## Long card names shrink, then wrap, but never extend past the card frame,
## at standard and compact sizes. Short names keep their full size.

var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	BattleViewState.new().apply_result(gateway.start_battle("card-title-fit", 43))
	var cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {})
	var board := Control.new(); root.add_child(board); board.size = Vector2(1600, 900)
	for name in ["Steady Guard", "Steady Guard · Field Dressing", "A Considerably Longer Card Name Than Fits"]:
		cards.steady_guard.name = name
		for compact in [false, true]:
			var card := BattleCard.new(); board.add_child(card)
			card.configure("steady_guard-1", "steady_guard", true, false, compact)
			card.size = card.custom_minimum_size
			for frame in 3: await process_frame
			var title: Label = card.get_node("CardTitle")
			var font := title.get_theme_font("font"); var point_size := title.get_theme_font_size("font_size")
			var inside := title.position.x >= 0 and title.position.x + title.size.x <= card.size.x
			var tag := "%s%s" % [name, " (compact)" if compact else ""]
			_expect(inside, "%s: title box stays inside the card (%s of %s)" % [tag, title.size, card.size])
			_expect(title.clip_text, "%s: title clips instead of spilling" % tag)
			var widest := font.get_string_size(title.text, HORIZONTAL_ALIGNMENT_LEFT, -1, point_size).x
			_expect(widest <= title.size.x or title.autowrap_mode != TextServer.AUTOWRAP_OFF, "%s: unwrapped text fits its width" % tag)
			if name == "Steady Guard":
				_expect(point_size == (12 if compact else 15), "%s: short names keep full size" % tag)
	print("CARD TITLE FIT: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
