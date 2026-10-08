extends SceneTree

## Renders a sheet of card faces to a PNG for visual review:
##   DICE_AND_DESTINY_CARD_PREVIEW=/tmp/cards.png ./scripts/godot.sh --script res://devtools/card_face_preview.gd
## Optional: DICE_AND_DESTINY_CARD_PREVIEW_ZOOM=2, _ORIGIN=x,y and _COLORS (below).
## Rows: playable hand cards, disabled cards, pending-removal/compact cards.

const IDS := ["brace", "take_stock", "nudge", "try_again", "strong_swing", "second_wind"]

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.content_scale_mode = Window.CONTENT_SCALE_MODE_DISABLED
	root.size = Vector2i(1320, 900)
	var zoom := float(OS.get_environment("DICE_AND_DESTINY_CARD_PREVIEW_ZOOM")) if OS.has_environment("DICE_AND_DESTINY_CARD_PREVIEW_ZOOM") else 1.0
	# Each battle pins its own catalog; merge the Venom cards into the Adventurer one.
	var gateway := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom").start_battle("card-face-preview-venom", 7))
	var venom_cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {}).duplicate(true)
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer").start_battle("card-face-preview", 43))
	venom_cards.merge(BattlePresentationCatalog._catalog.get("cards", {}), true)
	BattlePresentationCatalog._catalog["cards"] = venom_cards
	# Try face colours: DICE_AND_DESTINY_CARD_PREVIEW_COLORS="brace=blue,nudge=red/white"
	# (frame, then optional border; names or #rrggbb).
	for entry in OS.get_environment("DICE_AND_DESTINY_CARD_PREVIEW_COLORS").split(",", false):
		var parts := entry.split("=")
		if parts.size() != 2 or not venom_cards.has(parts[0]): continue
		var colors := parts[1].split("/")
		venom_cards[parts[0]].presentation["frame_color"] = colors[0]
		if colors.size() > 1: venom_cards[parts[0]].presentation["border_color"] = colors[1]
	var backdrop := TextureRect.new(); backdrop.texture = load("res://assets/battle/wasteland/arena.png")
	backdrop.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; backdrop.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	backdrop.modulate = Color(0.55, 0.55, 0.55)
	root.add_child(backdrop); backdrop.size = Vector2(root.size)
	var board := Control.new(); board.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); root.add_child(board); board.size = Vector2(root.size); board.scale = Vector2(zoom, zoom)
	# Pan a zoomed sheet with DICE_AND_DESTINY_CARD_PREVIEW_ORIGIN=x,y (sheet pixels).
	var origin := OS.get_environment("DICE_AND_DESTINY_CARD_PREVIEW_ORIGIN").split_floats(",")
	if origin.size() == 2: board.position = -Vector2(origin[0], origin[1]) * zoom
	for index in IDS.size():
		_card(board, IDS[index], Vector2(30 + index * 210, 30), true, false, false, (index - 2.5) * 2.0)
		_card(board, IDS[index], Vector2(30 + index * 210, 310), false, false, false, 0)
	var extras := ["black_tax", "accelerant", "agitate"]
	for index in extras.size():
		_card(board, extras[index], Vector2(30 + index * 210, 590), true, false, false, 0)
	_card(board, "brace", Vector2(660, 600), false, true, false, 0)
	_card(board, "nudge", Vector2(810, 600), false, false, true, 0)
	_card(board, "try_again", Vector2(960, 600), false, true, false, 0, true)
	for frame in 8: await process_frame
	await RenderingServer.frame_post_draw
	var path := OS.get_environment("DICE_AND_DESTINY_CARD_PREVIEW")
	if path.is_empty(): path = OS.get_user_data_dir().path_join("card_preview.png")
	root.get_texture().get_image().save_png(path)
	print("CARD PREVIEW: ", path)
	quit(0)

func _card(board: Control, id: String, at: Vector2, enabled: bool, pending: bool, compact: bool, tilt: float, removed: bool = false) -> void:
	if BattlePresentationCatalog.card(id).is_empty(): return
	var card := BattleCard.new(); board.add_child(card)
	card.configure(id, id, enabled, pending, compact, removed)
	card.size = card.custom_minimum_size; card.position = at
	card.pivot_offset = card.size * 0.5; card.rotation_degrees = tilt
