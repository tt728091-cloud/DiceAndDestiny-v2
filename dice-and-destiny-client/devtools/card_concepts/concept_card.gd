extends Control

## Review-only card face concepts. Nothing in the game loads this file; it
## renders the same catalog data (name, cost, face text, timing, art) in five
## alternative styles so designs can be compared before changing BattleCard.

const STYLES := ["gilded_relic", "tarot", "playing_card", "iron_ember", "full_art"]
const STYLE_NAMES := {
	"gilded_relic": "A · Gilded Relic",
	"tarot": "B · Grimoire Tarot",
	"playing_card": "C · Antique Playing Card",
	"iron_ember": "D · Iron & Ember",
	"full_art": "E · Full Art",
}
const SIZE := Vector2(185, 248)
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const OFFENSE := Color("b4462c")
const DEFENSE := Color("3d6d9e")
const NEUTRAL := Color("8a7246")

var style := "gilded_relic"
var enabled := true
var data: Dictionary = {}
var _art: Texture2D
var _art_region := Rect2()

func setup(card_id: String, style_name: String, playable: bool = true) -> void:
	style = style_name; enabled = playable
	data = BattlePresentationCatalog.card(card_id)
	custom_minimum_size = SIZE; size = SIZE
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_load_art(card_id)
	call(style + "_build")
	if not enabled: modulate = Color(0.68, 0.67, 0.65)

func _draw() -> void:
	call(style + "_draw")

# ---------------------------------------------------------------- shared

func _load_art(card_id: String) -> void:
	var texture: Texture2D = preload("res://assets/battle/fighters/curse.png") if data.targeting.get("selector") == "curse_choice" else CINEMATIC.art(CINEMATIC.card_art_index(card_id))
	if not str(data.illustration_path).is_empty() and ResourceLoader.exists(str(data.illustration_path)): texture = load(str(data.illustration_path))
	if texture is AtlasTexture:
		_art = texture.atlas; _art_region = texture.region
	elif texture != null:
		_art = texture; _art_region = Rect2(Vector2.ZERO, texture.get_size())

## Draws the art cover-cropped into `dest`, clipped to `points`.
func _draw_art(points: PackedVector2Array, dest: Rect2, tint: Color = Color.WHITE, focus_y: float = 0.5) -> void:
	# Dark backing so art with transparent areas never shows the border.
	draw_colored_polygon(points, Color("15130f"))
	if _art == null: return
	var source := _art_region
	var aspect := dest.size.x / dest.size.y
	if source.size.x / source.size.y > aspect:
		var width := source.size.y * aspect
		source = Rect2(source.position.x + (source.size.x - width) * 0.5, source.position.y, width, source.size.y)
	else:
		var height := source.size.x / aspect
		source = Rect2(source.position.x, source.position.y + (source.size.y - height) * focus_y, source.size.x, height)
	var uvs := PackedVector2Array()
	for point in points: uvs.append((source.position + (point - dest.position) / dest.size * source.size) / _art.get_size())
	draw_polygon(points, PackedColorArray([tint]), uvs, _art)

static func _rect_points(rect: Rect2) -> PackedVector2Array:
	return PackedVector2Array([rect.position, Vector2(rect.end.x, rect.position.y), rect.end, Vector2(rect.position.x, rect.end.y)])

static func _rounded_points(rect: Rect2, radius: float, steps: int = 6) -> PackedVector2Array:
	var points := PackedVector2Array()
	var centers := [rect.position + Vector2(radius, radius), Vector2(rect.end.x - radius, rect.position.y + radius), rect.end - Vector2(radius, radius), Vector2(rect.position.x + radius, rect.end.y - radius)]
	for i in 4:
		for s in steps + 1:
			points.append(centers[i] + Vector2.from_angle(PI + i * PI / 2.0 + s * PI / 2.0 / steps) * radius)
	return points

func _vertical_gradient(rect: Rect2, top: Color, bottom: Color) -> void:
	draw_polygon(_rect_points(rect), PackedColorArray([top, top, bottom, bottom]))

## A vertical gradient limited to the inside of `shape`.
func _clipped_gradient(shape: PackedVector2Array, rect: Rect2, top: Color, bottom: Color) -> void:
	for piece in Geometry2D.intersect_polygons(shape, _rect_points(rect)):
		var colors := PackedColorArray()
		for point in piece: colors.append(top.lerp(bottom, clampf((point.y - rect.position.y) / rect.size.y, 0, 1)))
		draw_polygon(piece, colors)

func _box(rect: Rect2, fill: Color, border: Color = Color.TRANSPARENT, width: int = 0, radius: int = 0, shadow: int = 0) -> void:
	var box := StyleBoxFlat.new(); box.bg_color = fill; box.border_color = border
	box.set_border_width_all(width); box.set_corner_radius_all(radius); box.anti_aliasing = true
	if shadow > 0: box.shadow_size = shadow; box.shadow_color = Color(0, 0, 0, 0.45); box.shadow_offset = Vector2(0, 2)
	box.draw(get_canvas_item(), rect)

func _segment() -> String:
	var segments: Array = data.timing.map(func(tag): return str(tag.get("segment", "")))
	if segments.is_empty(): return ""
	return segments[0] if segments.count(segments[0]) == segments.size() else "both"

func _segment_color() -> Color:
	return {"offense": OFFENSE, "defense": DEFENSE}.get(_segment(), NEUTRAL)

static func font(names: Array, weight: int = 400, italic: bool = false) -> SystemFont:
	var result := SystemFont.new(); result.font_names = PackedStringArray(names + ["Georgia", "serif"])
	result.font_weight = weight; result.font_italic = italic
	return result

func _label(text: String, rect: Rect2, font_size: int, color: Color, face: Font = null, align := HORIZONTAL_ALIGNMENT_CENTER, outline: Color = Color.TRANSPARENT, outline_size: int = 0) -> Label:
	var label := Label.new(); label.text = text; label.position = rect.position; label.size = rect.size
	label.horizontal_alignment = align; label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", color)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	if face != null: label.add_theme_font_override("font", face)
	if outline_size > 0: label.add_theme_color_override("font_outline_color", outline); label.add_theme_constant_override("outline_size", outline_size)
	add_child(label)
	# Shrink long summaries until they fit their box.
	var size_now := font_size
	var used := face if face != null else label.get_theme_font("font")
	while size_now > 9 and used.get_multiline_string_size(text, align, rect.size.x, size_now).y > rect.size.y:
		size_now -= 1
	label.add_theme_font_size_override("font_size", size_now)
	label.custom_minimum_size = Vector2(rect.size.x, 0); label.size = rect.size; label.position = rect.position
	return label

## Icon(s) plus "Any time · 1/round", centered in `rect`.
func _timing(rect: Rect2, color: Color, font_size: int = 12, face: Font = null, upper: bool = false, left: bool = false) -> void:
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_BEGIN if left else BoxContainer.ALIGNMENT_CENTER; row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_theme_constant_override("separation", 3); row.position = rect.position; row.size = rect.size
	add_child(row)
	var groups := BattleCard.timing_groups(data.timing)
	for index in groups.size():
		if index > 0: row.add_child(_inline("·", color, font_size, face))
		for segment in groups[index].segments:
			var icon := TextureRect.new(); icon.texture = ICONS.texture({"offense": "attack", "defense": "block"}.get(segment, "dice"))
			icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
			icon.custom_minimum_size = Vector2(font_size + 3, font_size + 3); icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
			row.add_child(icon)
		var words: String = groups[index].label
		row.add_child(_inline(words.to_upper() if upper else words, color, font_size, face))
	if not str(data.play_limit).is_empty(): row.add_child(_inline("· " + str(data.play_limit), color, font_size, face))

func _inline(text: String, color: Color, font_size: int, face: Font) -> Label:
	var label := Label.new(); label.text = text; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", color)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	label.add_theme_font_override("font", face if face != null else CINEMATIC.roll_control_font())
	return label

static var _paper: Texture2D

## Parchment at a third of its source size for the paper-based concepts.
static func paper_texture() -> Texture2D:
	if _paper == null:
		var source: Texture2D = load("res://assets/battle/wasteland/parchment.png")
		var image := source.get_image()
		if image == null: return source
		if image.is_compressed(): image.decompress()
		image.resize(image.get_width() / 3, image.get_height() / 3, Image.INTERPOLATE_LANCZOS)
		_paper = ImageTexture.create_from_image(image)
	return _paper

func _cost_text() -> String:
	return str(int(data.cost))

# ------------------------------------------------- A · Gilded Relic
# A classic collectible-card frame: dark bronze body, gilt rules, a title
# plate, a sapphire cost gem and a parchment rules box.

func gilded_relic_build() -> void:
	_label(str(data.name), Rect2(36, 7, SIZE.x - 46, 24), 15, Color("f3dfa6"), font(["Copperplate"], 700), HORIZONTAL_ALIGNMENT_CENTER, Color("140f0a"), 3)
	_label(_cost_text(), Rect2(3, 3, 32, 32), 18 if int(data.cost) < 10 else 15, Color("fffaf0"), CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("0b1f33"), 4)
	_label(str(data.effect_summary), Rect2(17, 160, SIZE.x - 34, 50), 14, CINEMATIC.INK, CINEMATIC.roll_control_font())
	_timing(Rect2(14, 212, SIZE.x - 28, 18), _segment_color().darkened(0.35), 12)

func gilded_relic_draw() -> void:
	var gold := Color("c9a55a"); var gold_dark := Color("7a5d2c")
	_box(Rect2(Vector2.ZERO, SIZE), Color("2b2219"), gold_dark, 2, 10, 6)
	_box(Rect2(Vector2(4, 4), SIZE - Vector2(8, 8)), Color.TRANSPARENT, gold, 1, 8)
	# Art with gilt keyline.
	var art := Rect2(10, 36, SIZE.x - 20, 116)
	_draw_art(_rect_points(art), art)
	_box(art.grow(1), Color.TRANSPARENT, gold, 2, 2)
	# Title plate.
	_box(Rect2(30, 6, SIZE.x - 38, 26), Color("1a140e"), gold, 1, 4)
	# Rules box: parchment with a segment-coloured base rule.
	var rules := Rect2(10, 156, SIZE.x - 20, 80)
	_box(rules, Color("eadcbc"), gold, 2, 5)
	draw_line(Vector2(rules.position.x + 14, rules.end.y - 23), Vector2(rules.end.x - 14, rules.end.y - 23), Color(gold_dark, 0.6), 1, true)
	_box(Rect2(rules.position.x + 2, rules.end.y - 6, rules.size.x - 4, 4), _segment_color(), Color.TRANSPARENT, 0, 2)
	# Sapphire cost gem.
	var center := Vector2(19, 19)
	draw_circle(center + Vector2(0, 1.5), 16, Color(0, 0, 0, 0.5))
	draw_circle(center, 15.5, gold_dark)
	draw_circle(center, 14, gold)
	draw_circle(center, 11.5, Color("173a5e"))
	draw_circle(center + Vector2(-2.5, -3), 7, Color(0.45, 0.7, 1.0, 0.25))
	draw_arc(center, 11.5, PI * 1.1, PI * 1.6, 12, Color(1, 1, 1, 0.45), 1.5, true)

# ------------------------------------------------- B · Grimoire Tarot
# Ivory and black like a tarot plate: an arched art window, the cost in a
# black disc at the crown, and an uppercase name plate along the bottom.

func tarot_build() -> void:
	_label(_cost_text(), Rect2(SIZE.x * 0.5 - 15, 4, 30, 30), 17 if int(data.cost) < 10 else 14, Color("efe4c8"), CINEMATIC.roll_control_font())
	_label(str(data.effect_summary), Rect2(16, 148, SIZE.x - 32, 38), 13, Color("231c14"), font(["Baskerville"], 400, true))
	_timing(Rect2(14, 192, SIZE.x - 28, 16), _segment_color().darkened(0.3), 10, font(["Copperplate"], 700), true)
	_label(str(data.name).to_upper(), Rect2(16, 213, SIZE.x - 32, 24), 13, Color("efe4c8"), font(["Copperplate"], 700))

func tarot_draw() -> void:
	var ink := Color("1b1612"); var gold := Color("b8954f")
	_box(Rect2(Vector2.ZERO, SIZE), Color("e9dec4"), ink, 0, 8, 6)
	_box(Rect2(Vector2(5, 5), SIZE - Vector2(10, 10)), Color.TRANSPARENT, ink, 2, 4)
	_box(Rect2(Vector2(9, 9), SIZE - Vector2(18, 18)), Color.TRANSPARENT, Color(ink, 0.6), 1, 2)
	# Arched art window.
	var art := Rect2(16, 26, SIZE.x - 32, 120)
	var radius := art.size.x * 0.5
	var arch := PackedVector2Array()
	for s in 25: arch.append(Vector2(art.position.x + radius, art.position.y + radius) + Vector2.from_angle(PI + s * PI / 24.0) * radius)
	arch.append(art.end); arch.append(Vector2(art.position.x, art.end.y))
	_draw_art(arch, art, Color.WHITE, 0.35)
	var outline := arch.duplicate(); outline.append(arch[0])
	draw_polyline(outline, ink, 2.5, true)
	draw_polyline(_inset_arch(art, 4), Color(gold, 0.85), 1.0, true)
	# Crown disc for the cost, overlapping the arch.
	var crown := Vector2(SIZE.x * 0.5, 19)
	draw_circle(crown, 15, ink); draw_arc(crown, 12.5, 0, TAU, 32, gold, 1.2, true)
	# Corner diamonds.
	for corner in [Vector2(13, 13), Vector2(SIZE.x - 13, 13), Vector2(13, SIZE.y - 13), Vector2(SIZE.x - 13, SIZE.y - 13)]:
		draw_colored_polygon(PackedVector2Array([corner + Vector2(0, -3.5), corner + Vector2(3.5, 0), corner + Vector2(0, 3.5), corner + Vector2(-3.5, 0)]), gold)
	# Ornamental rule between text and timing.
	var y := 189.0
	draw_line(Vector2(40, y), Vector2(SIZE.x - 40, y), Color(ink, 0.5), 1, true)
	draw_colored_polygon(PackedVector2Array([Vector2(SIZE.x * 0.5, y - 3), Vector2(SIZE.x * 0.5 + 3, y), Vector2(SIZE.x * 0.5, y + 3), Vector2(SIZE.x * 0.5 - 3, y)]), ink)
	# Name plate, edged in the segment colour.
	var plate := Rect2(14, 212, SIZE.x - 28, 26)
	_box(plate, ink, _segment_color().lightened(0.15), 1, 2)

func _inset_arch(art: Rect2, inset: float) -> PackedVector2Array:
	var radius := art.size.x * 0.5 - inset
	var center := Vector2(art.position.x + art.size.x * 0.5, art.position.y + art.size.x * 0.5)
	var points := PackedVector2Array()
	points.append(Vector2(art.position.x + inset, art.end.y - inset))
	for s in 25: points.append(center + Vector2.from_angle(PI + s * PI / 24.0) * radius)
	points.append(Vector2(art.end.x - inset, art.end.y - inset)); points.append(points[0])
	return points

# ------------------------------------------------- C · Antique Playing Card
# Restrained aged card stock: gently rounded parchment, a printed double
# rule, the cost as a red rubber stamp, and rules printed straight on paper.

func playing_card_build() -> void:
	_label(str(data.name), Rect2(14, 8, SIZE.x - 54, 26), 16, Color("2a2018"), font(["Baskerville"], 700), HORIZONTAL_ALIGNMENT_LEFT)
	_label(_cost_text(), Rect2(SIZE.x - 40, 5, 30, 30), 17 if int(data.cost) < 10 else 14, Color("8c2a1c"), font(["Baskerville"], 700))
	_label(str(data.effect_summary), Rect2(16, 160, SIZE.x - 32, 48), 14, Color("2a2018"), CINEMATIC.roll_control_font())
	_timing(Rect2(14, 214, SIZE.x - 28, 18), _segment_color().darkened(0.25), 12)

func playing_card_draw() -> void:
	var ink := Color("3a2c20")
	var body := _rounded_points(Rect2(Vector2.ZERO, SIZE), 11)
	draw_colored_polygon(Transform2D(0, Vector2(1.5, 3)) * body, Color(0, 0, 0, 0.4))
	var paper: Texture2D = paper_texture()
	var uvs := PackedVector2Array()
	var region := Rect2(paper.get_size() * Vector2(0.3, 0.12), paper.get_size() * Vector2(0.2, 0.76))
	for point in body: uvs.append((region.position + point / SIZE * region.size) / paper.get_size())
	draw_polygon(body, PackedColorArray([Color("f3e7cf")]), uvs, paper)
	var edge := body.duplicate(); edge.append(body[0])
	draw_polyline(edge, Color(0.45, 0.32, 0.18, 0.55), 1.2, true)
	# Printed double rule.
	_box(Rect2(Vector2(6, 6), SIZE - Vector2(12, 12)), Color.TRANSPARENT, Color(ink, 0.75), 1, 7)
	_box(Rect2(Vector2(8.5, 8.5), SIZE - Vector2(17, 17)), Color.TRANSPARENT, Color(ink, 0.35), 1, 6)
	# Art inset with an ink rule.
	var art := Rect2(14, 37, SIZE.x - 28, 112)
	_draw_art(_rect_points(art), art, Color(1, 0.97, 0.9))
	_box(art.grow(1.5), Color.TRANSPARENT, ink, 1, 1)
	# Fleuron divider under the art.
	var y := 155.0
	draw_line(Vector2(32, y), Vector2(SIZE.x * 0.5 - 7, y), Color(ink, 0.6), 1, true)
	draw_line(Vector2(SIZE.x * 0.5 + 7, y), Vector2(SIZE.x - 32, y), Color(ink, 0.6), 1, true)
	draw_circle(Vector2(SIZE.x * 0.5, y), 2.5, _segment_color())
	draw_line(Vector2(40, 211), Vector2(SIZE.x - 40, 211), Color(ink, 0.25), 1, true)
	# Rubber-stamp cost: an uneven double ring in faded red ink.
	var stamp := Vector2(SIZE.x - 25, 20)
	draw_arc(stamp, 13, 0, TAU, 40, Color("8c2a1c", 0.85), 1.8, true)
	draw_arc(stamp, 10.5, 0.4, TAU - 0.3, 40, Color("8c2a1c", 0.5), 1.0, true)

# ------------------------------------------------- D · Iron & Ember
# Dark slate in a riveted iron frame to match the battlefield; the art fades
# into the body, the cost burns in a hex plate, and a coloured spine marks
# Offense (ember) or Defense (steel).

func iron_ember_build() -> void:
	_label(str(data.name), Rect2(14, 124, SIZE.x - 28, 24), 15, Color("f4e6c6"), font(["Copperplate"], 700), HORIZONTAL_ALIGNMENT_CENTER, Color("0b0908"), 5)
	_label(_cost_text(), Rect2(4, 4, 32, 32), 18 if int(data.cost) < 10 else 15, Color("ffd88a"), CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("2a0e04"), 4)
	_label(str(data.effect_summary), Rect2(16, 156, SIZE.x - 32, 52), 14, Color("ece2cc"), CINEMATIC.roll_control_font())
	_timing(Rect2(18, 215, SIZE.x - 36, 18), _segment_color().lightened(0.45), 12)

func iron_ember_draw() -> void:
	var iron := Color("5d564e"); var iron_light := Color("9a9184")
	_box(Rect2(Vector2.ZERO, SIZE), Color("1b1918"), Color("0b0a09"), 2, 6, 6)
	var art := Rect2(7, 7, SIZE.x - 14, 142)
	_draw_art(_rect_points(art), art)
	_vertical_gradient(Rect2(art.position.x, art.end.y - 50, art.size.x, 50), Color(0.106, 0.098, 0.094, 0), Color(0.106, 0.098, 0.094, 1))
	_vertical_gradient(Rect2(art.position.x, art.position.y, art.size.x, 30), Color(0, 0, 0, 0.45), Color(0, 0, 0, 0))
	# Iron frame with bevel and rivets.
	_box(Rect2(Vector2(3, 3), SIZE - Vector2(6, 6)), Color.TRANSPARENT, iron, 4, 5)
	_box(Rect2(Vector2(2, 2), SIZE - Vector2(4, 4)), Color.TRANSPARENT, Color(iron_light, 0.5), 1, 6)
	for point in [Vector2(5, 5), Vector2(SIZE.x - 5, 5), Vector2(5, SIZE.y - 5), Vector2(SIZE.x - 5, SIZE.y - 5), Vector2(5, SIZE.y * 0.5), Vector2(SIZE.x - 5, SIZE.y * 0.5)]:
		draw_circle(point, 3.2, Color("2d2925")); draw_circle(point - Vector2(0.6, 0.6), 2.2, iron_light); draw_circle(point - Vector2(1, 1), 0.8, Color(1, 1, 1, 0.6))
	# Segment spine along the rules area.
	var spine := _segment_color()
	_box(Rect2(7, 154, 3, SIZE.y - 165), spine, Color.TRANSPARENT, 0, 1)
	_box(Rect2(SIZE.x - 10, 154, 3, SIZE.y - 165), spine, Color.TRANSPARENT, 0, 1)
	# Timing chip.
	_box(Rect2(24, 214, SIZE.x - 48, 20), Color(spine.darkened(0.55), 0.9), Color(spine, 0.9), 1, 10)
	# Hex cost plate with an ember glow.
	var center := Vector2(20, 20)
	var hexagon := PackedVector2Array()
	for i in 6: hexagon.append(center + Vector2.from_angle(PI / 6.0 + i * PI / 3.0) * 15.5)
	draw_colored_polygon(Transform2D(0, Vector2(0, 2)) * hexagon, Color(0, 0, 0, 0.6))
	draw_colored_polygon(hexagon, Color("2a2420"))
	var ring := hexagon.duplicate(); ring.append(hexagon[0])
	draw_polyline(ring, iron_light, 1.6, true)
	for r in [11.0, 8.0, 5.0]: draw_circle(center, r, Color(1.0, 0.45, 0.1, 0.12))

# ------------------------------------------------- E · Full Art
# The illustration covers the whole card; text sits on dark gradients and the
# rim glows in the segment colour. The most modern and minimal option.

func full_art_build() -> void:
	_label(str(data.name), Rect2(38, 6, SIZE.x - 46, 28), 16, Color("fff6e0"), font(["Baskerville"], 700), HORIZONTAL_ALIGNMENT_LEFT, Color("050404"), 5)
	_label(_cost_text(), Rect2(4, 4, 30, 30), 18 if int(data.cost) < 10 else 14, Color("ffffff"), CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("04121c"), 4)
	_label(str(data.effect_summary), Rect2(14, 164, SIZE.x - 28, 50), 15, Color("fff6e0"), CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("050404"), 4)
	_timing(Rect2(16, 218, SIZE.x - 32, 18), Color("f1e3c2"), 12)

func full_art_draw() -> void:
	var rim := _segment_color().lightened(0.25)
	var body := _rounded_points(Rect2(Vector2.ZERO, SIZE), 12)
	draw_colored_polygon(Transform2D(0, Vector2(0, 3)) * body, Color(0, 0, 0, 0.5))
	_draw_art(body, Rect2(Vector2.ZERO, SIZE), Color.WHITE, 0.3)
	var shade := Color(0.02, 0.02, 0.03)
	_clipped_gradient(body, Rect2(0, 0, SIZE.x, 46), Color(shade, 0.75), Color(shade, 0))
	_clipped_gradient(body, Rect2(0, 120, SIZE.x, 50), Color(shade, 0), Color(shade, 0.82))
	_clipped_gradient(body, Rect2(0, 170, SIZE.x, SIZE.y - 170), Color(shade, 0.82), Color(shade, 0.88))
	var edge := body.duplicate(); edge.append(body[0])
	draw_polyline(edge, Color(rim, 0.35), 5, true)
	draw_polyline(edge, rim, 1.6, true)
	# Thin divider and a glowing cost rune.
	draw_line(Vector2(36, 214), Vector2(SIZE.x - 36, 214), Color(rim, 0.5), 1, true)
	var center := Vector2(19, 19)
	for r in [17.0, 15.5]: draw_circle(center, r, Color(rim, 0.18))
	draw_circle(center, 13.5, Color("0d1720"))
	draw_arc(center, 13.5, 0, TAU, 36, rim, 1.6, true)
	draw_arc(center, 10.5, 0, TAU, 36, Color(rim, 0.4), 1, true)
