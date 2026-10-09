extends SceneTree

## Renders the review-only card face concepts to PNGs:
##   DICE_AND_DESTINY_CARD_CONCEPTS=/tmp/concepts ./scripts/godot.sh --script res://devtools/card_concepts/render_card_concepts.gd
## Writes concept_<style>.png per style and concepts_compare.png.

const CONCEPT := preload("res://devtools/card_concepts/concept_card.gd")
const SHEET := [["steady_guard", true], ["take_stock", true], ["nudge", true], ["try_again", true], ["strong_swing", true], ["accelerant", true], ["black_tax", true], ["second_wind", false]]
const COMPARE := ["steady_guard", "nudge"]

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
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom").start_battle("card-concepts-venom", 7))
	var cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {}).duplicate(true)
	BattleViewState.new().apply_result(gateway.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer").start_battle("card-concepts", 43))
	cards.merge(BattlePresentationCatalog._catalog.get("cards", {}), true)
	BattlePresentationCatalog._catalog["cards"] = cards
	for style in CONCEPT.STYLES: await _sheet(style)
	await _compare()
	print("CARD CONCEPTS: ", _directory)
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

func _card(board: Control, id: String, style: String, playable: bool, at: Vector2, zoom: float, tilt: float = 0.0) -> void:
	var card: Control = CONCEPT.new(); board.add_child(card)
	card.setup(id, style, playable)
	card.pivot_offset = CONCEPT.SIZE * 0.5; card.scale = Vector2(zoom, zoom); card.rotation_degrees = tilt
	card.position = at + CONCEPT.SIZE * (zoom - 1.0) * 0.5

func _sheet(style: String) -> void:
	var board := _board()
	_title(board, CONCEPT.STYLE_NAMES[style], Vector2(24, 8))
	var zoom := 1.55
	for index in SHEET.size():
		var at := Vector2(30 + (index % 4) * (CONCEPT.SIZE.x * zoom + 28), 50 + (index / 4) * (CONCEPT.SIZE.y * zoom + 34))
		_card(board, SHEET[index][0], style, SHEET[index][1], at, zoom)
	await _save(board.get_parent(), "concept_" + style)

func _compare() -> void:
	var board := _board()
	var zoom := 1.2
	for column in CONCEPT.STYLES.size():
		var style: String = CONCEPT.STYLES[column]
		var x := 22 + column * (CONCEPT.SIZE.x * zoom + 32)
		_title(board, CONCEPT.STYLE_NAMES[style], Vector2(x, 12), 16)
		for row in COMPARE.size():
			_card(board, COMPARE[row], style, true, Vector2(x, 52 + row * (CONCEPT.SIZE.y * zoom + 40)), zoom, (row - 0.5) * 3.0)
	await _save(board.get_parent(), "concepts_compare")

func _save(node: Node, file: String) -> void:
	for frame in 6: await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(_directory.path_join(file + ".png"))
	node.queue_free()
	await process_frame
