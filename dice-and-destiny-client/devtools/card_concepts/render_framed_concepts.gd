extends SceneTree

## Renders the full-art framed concepts (F–J) to PNGs:
##   DICE_AND_DESTINY_CARD_CONCEPTS=/tmp/concepts ./scripts/godot.sh --script res://devtools/card_concepts/render_framed_concepts.gd
## Writes framed_<style>.png (every frame colour), framed_borders.png and
## framed_compare.png.

const FRAMED := preload("res://devtools/card_concepts/framed_card.gd")
const CARDS := ["steady_guard", "take_stock", "nudge", "try_again", "strong_swing", "accelerant", "black_tax", "second_wind"]
const COLORS := ["white", "blue", "black", "red", "green", "artifact", "gold", "colorless"]
## The foil concept shows off a coloured border; the rest use black.
const STYLE_BORDERS := {"foil": "gold"}

var _directory := ""

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.content_scale_mode = Window.CONTENT_SCALE_MODE_DISABLED
	root.size = Vector2i(1320, 900)
	_directory = OS.get_environment("DICE_AND_DESTINY_CARD_CONCEPTS")
	if _directory.is_empty(): _directory = OS.get_user_data_dir().path_join("card_concepts")
	DirAccess.make_dir_recursive_absolute(_directory)
	# Each battle pins its own catalog; merge Venom and Curse cards into the Adventurer one.
	var gateway := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom").start_battle("framed-concepts-venom", 7))
	var cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {}).duplicate(true)
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer").start_battle("framed-concepts", 43))
	cards.merge(BattlePresentationCatalog._catalog.get("cards", {}), true)
	BattlePresentationCatalog._catalog["cards"] = cards
	# DICE_AND_DESTINY_CARD_CONCEPT_SET=iron renders only the Iron follow-ups.
	var concept_set := OS.get_environment("DICE_AND_DESTINY_CARD_CONCEPT_SET")
	if concept_set in FRAMED.FRAME_NAMES and not concept_set in FRAMED.FRAME_STYLES:
		# One finished design: a close-up, every frame colour, and a hand.
		await _closeup(concept_set); await _sheet(concept_set); await _hand(concept_set)
		print("FRAMED CONCEPTS: ", _directory); quit(0); return
	var styles: Array = FRAMED.IRON_STYLES if concept_set == "iron" else FRAMED.FRAME_STYLES
	for style in styles: await _sheet(style)
	if styles == FRAMED.FRAME_STYLES: await _borders()
	await _compare(styles)
	print("FRAMED CONCEPTS: ", _directory)
	quit(0)

func _board() -> Control:
	var backdrop := TextureRect.new(); backdrop.texture = load("res://assets/battle/wasteland/arena.png")
	backdrop.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; backdrop.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	backdrop.self_modulate = Color(0.5, 0.5, 0.5); backdrop.size = Vector2(root.size)
	root.add_child(backdrop)
	var board := Control.new(); board.theme = preload("res://presentation/battle/cinematic_theme.gd").create()
	backdrop.add_child(board); board.size = Vector2(root.size)
	return board

func _title(board: Control, text: String, at: Vector2, font_size: int = 22) -> void:
	var label := Label.new(); label.text = text; label.position = at
	label.add_theme_font_size_override("font_size", font_size)
	preload("res://presentation/battle/cinematic_theme.gd").hud_lettering(label, true)
	board.add_child(label)

func _card(board: Control, id: String, style: String, frame: String, border: String, at: Vector2, zoom: float, tilt: float = 0.0) -> void:
	var card: Control = FRAMED.new(); board.add_child(card)
	card.configure_frame(FRAMED.PALETTE[frame], FRAMED.BORDERS[border])
	card.setup(id, style, true)
	card.pivot_offset = FRAMED.SIZE * 0.5; card.scale = Vector2(zoom, zoom); card.rotation_degrees = tilt
	card.position = at + FRAMED.SIZE * (zoom - 1.0) * 0.5

func _sheet(style: String) -> void:
	var board := _board()
	var border: String = STYLE_BORDERS.get(style, "black")
	_title(board, "%s   ·   %s border, every frame colour" % [FRAMED.FRAME_NAMES[style], border], Vector2(24, 6))
	var zoom := 1.42
	for index in CARDS.size():
		var at := Vector2(40 + (index % 4) * (FRAMED.SIZE.x * zoom + 46), 46 + (index / 4) * (FRAMED.SIZE.y * zoom + 52))
		_card(board, CARDS[index], style, COLORS[index], border, at, zoom)
		_title(board, COLORS[index] + " frame", at + Vector2(0, FRAMED.SIZE.y * zoom + 4), 15)
	await _save(board.get_parent(), "framed_" + style)

func _borders() -> void:
	var board := _board()
	_title(board, "Border colour is separate from frame colour (G · Modern and F · Keyline)", Vector2(24, 6))
	var zoom := 1.25
	var combos := [["black", "blue"], ["white", "blue"], ["silver", "artifact"], ["gold", "red"], ["white", "green"]]
	for row in 2:
		var style := "modern" if row == 0 else "keyline"
		for column in combos.size():
			var at := Vector2(30 + column * (FRAMED.SIZE.x * zoom + 26), 44 + row * (FRAMED.SIZE.y * zoom + 44))
			_card(board, ["nudge", "steady_guard", "try_again", "strong_swing", "take_stock"][column], style, combos[column][1], combos[column][0], at, zoom)
			_title(board, "%s border · %s frame" % combos[column], at + Vector2(0, FRAMED.SIZE.y * zoom + 3), 14)
	await _save(board.get_parent(), "framed_borders")

func _compare(styles: Array) -> void:
	var board := _board()
	var few := styles.size() <= 3
	var zoom := 1.6 if few else 1.2
	for column in styles.size():
		var style: String = styles[column]
		var x := (60 if few else 22) + column * (FRAMED.SIZE.x * zoom + (90 if few else 32))
		_title(board, FRAMED.FRAME_NAMES[style].split(" + ")[0] + " · " + FRAMED.FRAME_NAMES[style].split(" + ")[-1] if " + " in FRAMED.FRAME_NAMES[style] else FRAMED.FRAME_NAMES[style], Vector2(x, 10), 18 if few else 16)
		var border: String = STYLE_BORDERS.get(style, "black")
		_card(board, "strong_swing" if few else "steady_guard", style, "red" if few else "blue", border, Vector2(x, 46), zoom, 0.0 if few else -1.5)
		if few: _card(board, "steady_guard", style, "blue", border, Vector2(x, 46 + FRAMED.SIZE.y * zoom + 30), zoom)
		else: _card(board, "nudge", style, "red", border, Vector2(x, 52 + FRAMED.SIZE.y * zoom + 40), zoom, 1.5)
	await _save(board.get_parent(), "framed_compare" if not few else "iron_compare")

func _closeup(style: String) -> void:
	var board := _board()
	_title(board, FRAMED.FRAME_NAMES[style] + "   ·   close-up", Vector2(24, 6))
	var zoom := 2.0
	var picks := [["strong_swing", "red"], ["steady_guard", "blue"], ["nudge", "black"]]
	for index in picks.size():
		_card(board, picks[index][0], style, picks[index][1], "black", Vector2(40 + index * (FRAMED.SIZE.x * zoom + 46), 50), zoom)
	await _save(board.get_parent(), style + "_closeup")

## The hand at in-game scale, fanned along the bottom of the battlefield.
func _hand(style: String) -> void:
	var board := _board()
	_title(board, FRAMED.FRAME_NAMES[style] + "   ·   hand at game scale", Vector2(24, 6))
	var picks := [["second_wind", "colorless"], ["strong_swing", "red"], ["try_again", "red"], ["nudge", "red"], ["steady_guard", "blue"], ["take_stock", "green"]]
	var zoom := 1.15
	for index in picks.size():
		var offset := index - (picks.size() - 1) * 0.5
		var at := Vector2(root.size.x * 0.5 - FRAMED.SIZE.x * 0.5 + offset * 150, root.size.y - FRAMED.SIZE.y * zoom - 40 + absf(offset) * absf(offset) * 6)
		_card(board, picks[index][0], style, picks[index][1], "black", at, zoom, offset * 4.0)
	await _save(board.get_parent(), style + "_hand")

func _save(node: Node, file: String) -> void:
	for frame in 6: await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(_directory.path_join(file + ".png"))
	node.queue_free()
	await process_frame
