extends "res://devtools/card_concepts/concept_card.gd"

## Review-only full-art concepts with a solid trading-card border. Two colours
## configure every card: `border_color` (the outer edge, usually black or
## white) and `frame_color` (the coloured frame, like a card's colour
## identity). Every tint, rim and panel is derived from those two colours, so a
## new colour needs no new art.

const FRAME_STYLES := ["keyline", "modern", "speckled", "iron", "foil"]
const FRAME_NAMES := {
	"keyline": "F · Full Art + Keyline",
	"modern": "G · Full Art + Modern Frame",
	"speckled": "H · Full Art + Speckled Frame",
	"iron": "I · Full Art + Iron Frame",
	"foil": "J · Full Art + Foil",
	"iron_classic": "K · Iron Classic",
	"iron_trim": "L · Iron Trim",
	"iron_forged": "M · Iron Forged",
	"iron_clean": "N · Iron Classic, clean",
}
## Iron Frame follow-ups: the timing line is plain text under a thin rule, as
## on the Full Art card, never a coloured badge.
const IRON_STYLES := ["iron_classic", "iron_trim", "iron_forged"]
## Frame colours modelled on the classic card colour identities.
const PALETTE := {
	"white": Color("e8e2cf"),
	"blue": Color("2368a8"),
	"black": Color("2f2b2c"),
	"red": Color("c4392c"),
	"green": Color("2e7a47"),
	"artifact": Color("8f7558"),
	"gold": Color("c9a24a"),
	"colorless": Color("a5a7a6"),
}
const BORDERS := {
	"black": Color("0e0d0d"),
	"white": Color("f1eee6"),
	"silver": Color("b6bac0"),
	"gold": Color("e2bd3f"),
}
const BORDER_WIDTH := 7.0
const INK := Color("1c1712")
const IVORY := Color("fff6e0")

var frame_color: Color = PALETTE.red
var border_color: Color = BORDERS.black
static var _speckle: Texture2D

func configure_frame(frame: Color, border: Color) -> void:
	frame_color = frame; border_color = border

# ---------------------------------------------------------------- shared

func _pale() -> Color:
	return frame_color.lerp(Color("f4efe4"), 0.78)

## Timing text on pale panels: near-black, warmed slightly toward the frame.
func _panel_ink() -> Color:
	return INK.lerp(frame_color.darkened(0.5), 0.25)

func _rim() -> Color:
	return frame_color.darkened(0.4)

func _frame_rect() -> Rect2:
	return Rect2(Vector2(BORDER_WIDTH, BORDER_WIDTH), SIZE - Vector2(BORDER_WIDTH, BORDER_WIDTH) * 2)

## Mottled grey with dark and light flecks; multiplied by the frame colour it
## gives the printed, speckled frame of older cards.
static func speckle() -> Texture2D:
	if _speckle != null: return _speckle
	var noise := FastNoiseLite.new(); noise.seed = 11; noise.frequency = 0.045; noise.fractal_octaves = 3
	var rng := RandomNumberGenerator.new(); rng.seed = 23
	var image := Image.create(256, 256, false, Image.FORMAT_RGBA8)
	for y in 256:
		for x in 256:
			var value := 0.88 + noise.get_noise_2d(x, y) * 0.14
			var roll := rng.randf()
			if roll < 0.03: value -= rng.randf_range(0.18, 0.35)
			elif roll < 0.055: value += 0.1
			value = clampf(value, 0.0, 1.0)
			image.set_pixel(x, y, Color(value, value, value))
	_speckle = ImageTexture.create_from_image(image)
	return _speckle

func _textured(points: PackedVector2Array, color: Color) -> void:
	var uvs := PackedVector2Array()
	for point in points: uvs.append(point / 256.0)
	draw_polygon(points, PackedColorArray([color]), uvs, speckle())

## The outer border: one solid colour with a faint bevelled edge.
func _draw_border() -> void:
	var body := _rounded_points(Rect2(Vector2.ZERO, SIZE), 10)
	draw_colored_polygon(Transform2D(0, Vector2(0, 3)) * body, Color(0, 0, 0, 0.5))
	draw_colored_polygon(body, border_color)
	var edge := body.duplicate(); edge.append(body[0])
	var light := border_color.get_luminance() > 0.5
	draw_polyline(edge, border_color.darkened(0.3) if light else border_color.lightened(0.18), 1.0, true)

## The Iron & Ember hexagonal cost plate.
func _draw_hex(center: Vector2) -> void:
	var hexagon := PackedVector2Array()
	for i in 6: hexagon.append(center + Vector2.from_angle(PI / 6.0 + i * PI / 3.0) * 15.5)
	draw_colored_polygon(Transform2D(0, Vector2(0, 2)) * hexagon, Color(0, 0, 0, 0.6))
	draw_colored_polygon(hexagon, Color("2a2420"))
	var ring := hexagon.duplicate(); ring.append(hexagon[0])
	draw_polyline(ring, Color("9a9184"), 1.6, true)
	for r in [11.0, 8.0, 5.0]: draw_circle(center, r, Color(1.0, 0.45, 0.1, 0.12))

func _hex_label(center: Vector2) -> void:
	_label(_cost_text(), Rect2(center - Vector2(16, 16), Vector2(32, 32)), 18 if int(data.cost) < 10 else 15, Color("ffd88a"), CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("2a0e04"), 4)

func _title_font() -> Font:
	return font(["Baskerville"], 700)

func _closed(points: PackedVector2Array) -> PackedVector2Array:
	var result := points.duplicate(); result.append(points[0])
	return result

# ------------------------------------------------- F · Keyline
# The Full Art card unchanged except for a solid outer border and a thin
# frame-colour keyline around the art. The lightest touch.

func keyline_build() -> void:
	_label(str(data.name), Rect2(42, 10, SIZE.x - 52, 26), 16, IVORY, _title_font(), HORIZONTAL_ALIGNMENT_LEFT, Color("050404"), 5)
	_hex_label(Vector2(25, 24))
	_label(str(data.effect_summary), Rect2(17, 160, SIZE.x - 34, 48), 15, IVORY, CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("050404"), 4)
	_timing(Rect2(30, 214, SIZE.x - 60, 19), IVORY, 12)

func keyline_draw() -> void:
	_draw_border()
	var frame := _frame_rect(); var shape := _rounded_points(frame, 5)
	_draw_art(shape, frame, Color.WHITE, 0.3)
	var shade := Color(0.02, 0.02, 0.03)
	_clipped_gradient(shape, Rect2(0, 0, SIZE.x, 50), Color(shade, 0.75), Color(shade, 0))
	_clipped_gradient(shape, Rect2(0, 120, SIZE.x, 45), Color(shade, 0), Color(shade, 0.84))
	_clipped_gradient(shape, Rect2(0, 165, SIZE.x, SIZE.y - 165), Color(shade, 0.84), Color(shade, 0.9))
	draw_polyline(_closed(shape), frame_color, 2.4, true)
	draw_polyline(_closed(_rounded_points(frame.grow(-2.2), 3)), Color(frame_color.darkened(0.55), 0.8), 1.0, true)
	draw_line(Vector2(36, 210), Vector2(SIZE.x - 36, 210), Color(frame_color, 0.8), 1, true)
	_box(Rect2(30, 214, SIZE.x - 60, 19), Color(frame_color.darkened(0.6), 0.92), frame_color, 1, 10)
	_draw_hex(Vector2(25, 24))

# ------------------------------------------------- G · Modern Frame
# A modern trading-card frame over full art: a speckled frame band, a pale
# title bar and type line tinted by the frame, and a pale rules box. The art
# still runs behind every bar.

func modern_build() -> void:
	_label(str(data.name), Rect2(45, 13, SIZE.x - 60, 22), 15, INK, _title_font(), HORIZONTAL_ALIGNMENT_LEFT)
	_hex_label(Vector2(25, 24))
	_timing(Rect2(19, 151, SIZE.x - 38, 19), _panel_ink(), 12, null, false, true)
	_label(str(data.effect_summary), Rect2(21, 175, SIZE.x - 42, 56), 14, INK, CINEMATIC.roll_control_font())

func modern_draw() -> void:
	_draw_border()
	var frame := _frame_rect()
	_textured(_rounded_points(frame, 5), frame_color)
	var art := frame.grow(-4.5)
	_draw_art(_rect_points(art), art, Color.WHITE, 0.25)
	_box(art.grow(0.5), Color.TRANSPARENT, frame_color.darkened(0.55), 1, 1)
	_box(Rect2(13, 12, SIZE.x - 26, 24), _pale(), _rim(), 2, 12, 3)
	_box(Rect2(13, 150, SIZE.x - 26, 21), _pale(), _rim(), 2, 10, 3)
	_box(Rect2(16, 173, SIZE.x - 32, 60), Color(_pale().lightened(0.25), 0.95), Color(_rim(), 0.8), 1, 2)
	_draw_hex(Vector2(25, 24))

# ------------------------------------------------- H · Speckled Frame
# Old-print character: a wide speckled frame with a bevelled art edge and a
# frame-tinted parchment rules box with scorched edges.

func speckled_build() -> void:
	_label(str(data.name), Rect2(44, 16, SIZE.x - 60, 22), 15, IVORY, _title_font(), HORIZONTAL_ALIGNMENT_LEFT, Color("050404"), 5)
	_hex_label(Vector2(26, 26))
	var box := _speckled_box()
	_label(str(data.effect_summary), Rect2(box.position.x + 7, box.position.y + 5, box.size.x - 14, box.size.y - 30), 14, INK, CINEMATIC.roll_control_font())
	_timing(Rect2(box.position.x + 4, box.end.y - 23, box.size.x - 8, 18), _panel_ink(), 12)

func _speckled_box() -> Rect2:
	return Rect2(20, 160, SIZE.x - 40, 70)

func speckled_draw() -> void:
	_draw_border()
	var frame := _frame_rect()
	_textured(_rounded_points(frame, 4), frame_color)
	var art := frame.grow(-7)
	_draw_art(_rect_points(art), art, Color.WHITE, 0.3)
	var shade := Color(0.02, 0.02, 0.03)
	_vertical_gradient(Rect2(art.position, Vector2(art.size.x, 40)), Color(shade, 0.7), Color(shade, 0))
	# Bevel: shadowed top/left, lit bottom/right, as if the art sits recessed.
	var dark := frame_color.darkened(0.65); var light := frame_color.lightened(0.4)
	draw_line(art.position, Vector2(art.end.x, art.position.y), dark, 2, true)
	draw_line(art.position, Vector2(art.position.x, art.end.y), dark, 2, true)
	draw_line(Vector2(art.position.x, art.end.y), art.end, light, 1.5, true)
	draw_line(Vector2(art.end.x, art.position.y), art.end, light, 1.5, true)
	draw_polyline(_closed(_rounded_points(frame, 4)), Color(light, 0.5), 1, true)
	# Parchment rules box tinted by the frame, with a scorched rim.
	var box := _speckled_box()
	var paper: Texture2D = paper_texture()
	var region := Rect2(paper.get_size() * Vector2(0.25, 0.2), paper.get_size() * Vector2(0.3, 0.55))
	var points := _rect_points(box); var uvs := PackedVector2Array()
	for point in points: uvs.append((region.position + (point - box.position) / box.size * region.size) / paper.get_size())
	draw_colored_polygon(Transform2D(0, Vector2(1, 2)) * points, Color(0, 0, 0, 0.5))
	draw_polygon(points, PackedColorArray([frame_color.lerp(Color("f6e7c4"), 0.8)]), uvs, paper)
	draw_polyline(_closed(_rect_points(box.grow(-3))), Color(0.3, 0.18, 0.06, 0.18), 6, true)
	draw_polyline(_closed(points), Color(0.18, 0.1, 0.04, 0.85), 1.6, true)
	_draw_hex(Vector2(26, 26))

# ------------------------------------------------- I · Iron Frame
# Iron & Ember's riveted metal frame, tinted toward the frame colour, around
# full art with the text on dark fades and a frame-coloured spine.

func iron_build() -> void:
	_label(str(data.name), Rect2(44, 11, SIZE.x - 56, 26), 15, IVORY, font(["Copperplate"], 700), HORIZONTAL_ALIGNMENT_LEFT, Color("050404"), 5)
	_hex_label(Vector2(25, 25))
	_label(str(data.effect_summary), Rect2(20, 158, SIZE.x - 40, 50), 15, IVORY, CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("050404"), 4)
	_timing(Rect2(30, 213, SIZE.x - 60, 20), IVORY, 12)

func iron_draw() -> void:
	_draw_border()
	var frame := _frame_rect()
	var metal := Color("5d564e").lerp(frame_color, 0.55)
	_textured(_rounded_points(frame, 4), metal)
	var art := frame.grow(-5)
	_draw_art(_rect_points(art), art, Color.WHITE, 0.3)
	var shade := Color(0.06, 0.055, 0.05)
	_vertical_gradient(Rect2(art.position, Vector2(art.size.x, 46)), Color(shade, 0.75), Color(shade, 0))
	_vertical_gradient(Rect2(art.position.x, 115, art.size.x, 45), Color(shade, 0), Color(shade, 0.9))
	draw_polygon(_rect_points(Rect2(art.position.x, 160, art.size.x, art.end.y - 160)), PackedColorArray([Color(shade, 0.9)]))
	_box(art.grow(1), Color.TRANSPARENT, metal.darkened(0.5), 1, 1)
	draw_polyline(_closed(_rounded_points(frame, 4)), Color(metal.lightened(0.45), 0.7), 1, true)
	for point in [Vector2(10.5, 10.5), Vector2(SIZE.x - 10.5, 10.5), Vector2(10.5, SIZE.y - 10.5), Vector2(SIZE.x - 10.5, SIZE.y - 10.5), Vector2(10.5, SIZE.y * 0.5), Vector2(SIZE.x - 10.5, SIZE.y * 0.5)]:
		draw_circle(point, 3.0, metal.darkened(0.6)); draw_circle(point - Vector2(0.5, 0.5), 2.1, metal.lightened(0.35)); draw_circle(point - Vector2(0.9, 0.9), 0.7, Color(1, 1, 1, 0.6))
	var spine := frame_color.lightened(0.15)
	_box(Rect2(art.position.x + 2, 160, 3, art.end.y - 166), spine, Color.TRANSPARENT, 0, 1)
	_box(Rect2(art.end.x - 5, 160, 3, art.end.y - 166), spine, Color.TRANSPARENT, 0, 1)
	_box(Rect2(30, 213, SIZE.x - 60, 20), Color(frame_color.darkened(0.6), 0.92), spine, 1, 10)
	_draw_hex(Vector2(25, 25))

# ------------------------------------------------- J · Foil
# A collectible foil: a coloured outer border, silver rims, a holographic
# sheen and sparkles over full art, and frame-tinted glass panels.

func foil_build() -> void:
	_label(str(data.name), Rect2(44, 12, SIZE.x - 58, 24), 15, INK, _title_font(), HORIZONTAL_ALIGNMENT_LEFT)
	_hex_label(Vector2(25, 24))
	_label(str(data.effect_summary), Rect2(20, 164, SIZE.x - 40, 46), 14, INK, CINEMATIC.roll_control_font())
	_timing(Rect2(20, 214, SIZE.x - 40, 18), _panel_ink(), 12)

func foil_draw() -> void:
	_draw_border()
	var frame := _frame_rect(); var shape := _rounded_points(frame, 5)
	_draw_art(shape, frame, Color.WHITE, 0.3)
	# Holographic sheen: a soft diagonal rainbow band, then sparkles.
	var band := PackedVector2Array([Vector2(frame.position.x, 60), Vector2(frame.end.x, 10), Vector2(frame.end.x, 70), Vector2(frame.position.x, 125)])
	for piece in Geometry2D.intersect_polygons(shape, band):
		var colors := PackedColorArray()
		for point in piece: colors.append(Color(1.0, 0.75, 0.9, 0.07).lerp(Color(0.7, 1.0, 1.0, 0.07), point.x / SIZE.x))
		draw_polygon(piece, colors)
	var rng := RandomNumberGenerator.new(); rng.seed = hash(str(data.name))
	for sparkle in 22:
		var at := Vector2(rng.randf_range(frame.position.x + 4, frame.end.x - 4), rng.randf_range(38, 150))
		var reach := rng.randf_range(1.5, 3.5); var glow := Color(1, 1, 1, rng.randf_range(0.35, 0.8))
		draw_line(at - Vector2(reach, 0), at + Vector2(reach, 0), glow, 1, true)
		draw_line(at - Vector2(0, reach), at + Vector2(0, reach), glow, 1, true)
	var silver := Color("dfe3e8")
	draw_polyline(_closed(shape), silver, 2.0, true)
	draw_polyline(_closed(_rounded_points(frame.grow(-2), 3)), Color(0.2, 0.22, 0.25, 0.7), 1.0, true)
	_box(Rect2(12, 11, SIZE.x - 24, 26), Color(_pale(), 0.92), silver, 2, 5, 3)
	_box(Rect2(14, 160, SIZE.x - 28, 75), Color(_pale().lightened(0.2), 0.9), silver, 2, 6, 3)
	draw_line(Vector2(36, 211), Vector2(SIZE.x - 36, 211), Color(_rim(), 0.5), 1, true)
	_draw_hex(Vector2(25, 24))

# ------------------------------------------------- Iron follow-ups (K–M)

## Border, metal band, full art and the Full Art fades. Returns the art rect.
func _iron_body(band: float, metal: Color, radius: float = 4.0) -> Rect2:
	_draw_border()
	var frame := _frame_rect()
	_textured(_rounded_points(frame, radius), metal)
	var art := frame.grow(-band)
	_draw_art(_rect_points(art), art, Color.WHITE, 0.3)
	var shade := Color(0.02, 0.02, 0.03)
	_vertical_gradient(Rect2(art.position, Vector2(art.size.x, 46)), Color(shade, 0.75), Color(shade, 0))
	_vertical_gradient(Rect2(art.position.x, 118, art.size.x, 46), Color(shade, 0), Color(shade, 0.84))
	_vertical_gradient(Rect2(art.position.x, 164, art.size.x, art.end.y - 164), Color(shade, 0.84), Color(shade, 0.9))
	return art

func _metal() -> Color:
	return Color("5d564e").lerp(frame_color, 0.55)

func _rivet(point: Vector2, metal: Color, radius: float = 3.0) -> void:
	draw_circle(point + Vector2(0.4, 0.8), radius, Color(0, 0, 0, 0.45))
	draw_circle(point, radius, metal.darkened(0.6))
	draw_circle(point - Vector2(0.5, 0.5), radius * 0.7, metal.lightened(0.35))
	draw_circle(point - Vector2(0.9, 0.9), radius * 0.25, Color(1, 1, 1, 0.6))

## The Full Art timing line: a thin rule, then plain ivory text.
func _iron_text() -> void:
	_label(str(data.effect_summary), Rect2(20, 160, SIZE.x - 40, 48), 15, IVORY, CINEMATIC.roll_control_font(), HORIZONTAL_ALIGNMENT_CENTER, Color("050404"), 4)
	_timing(Rect2(24, 214, SIZE.x - 48, 18), Color("f1e3c2"), 12)

func _iron_title(left: float = 44) -> void:
	_label(str(data.name), Rect2(left, 11, SIZE.x - left - 12, 26), 15, IVORY, font(["Copperplate"], 700), HORIZONTAL_ALIGNMENT_LEFT, Color("050404"), 5)

# K · Iron Classic: Iron Frame as reviewed, with the plain timing line.
func iron_classic_build() -> void:
	_iron_title(); _hex_label(Vector2(25, 25)); _iron_text()

func iron_classic_draw() -> void:
	var metal := _metal()
	var art := _iron_body(5, metal)
	_box(art.grow(1), Color.TRANSPARENT, metal.darkened(0.5), 1, 1)
	draw_polyline(_closed(_rounded_points(_frame_rect(), 4)), Color(metal.lightened(0.45), 0.7), 1, true)
	for point in [Vector2(10.5, 10.5), Vector2(SIZE.x - 10.5, 10.5), Vector2(10.5, SIZE.y - 10.5), Vector2(SIZE.x - 10.5, SIZE.y - 10.5), Vector2(10.5, SIZE.y * 0.5), Vector2(SIZE.x - 10.5, SIZE.y * 0.5)]:
		_rivet(point, metal)
	var spine := frame_color.lightened(0.15)
	_box(Rect2(art.position.x + 2, 160, 3, art.end.y - 166), spine, Color.TRANSPARENT, 0, 1)
	_box(Rect2(art.end.x - 5, 160, 3, art.end.y - 166), spine, Color.TRANSPARENT, 0, 1)
	draw_line(Vector2(36, 210), Vector2(SIZE.x - 36, 210), Color(spine, 0.7), 1, true)
	_draw_hex(Vector2(25, 25))

# L · Iron Trim: no side strips; a frame-coloured trim line runs inside the
# iron, and riveted corner brackets hold the art.
func iron_trim_build() -> void:
	_iron_title(); _hex_label(Vector2(25, 25)); _iron_text()

func iron_trim_draw() -> void:
	var metal := _metal()
	var art := _iron_body(5, metal)
	var trim := frame_color.lightened(0.1)
	draw_polyline(_closed(_rect_points(art.grow(-1.5))), trim, 2.0, true)
	draw_polyline(_closed(_rect_points(art.grow(-3.5))), Color(0, 0, 0, 0.5), 1.0, true)
	draw_polyline(_closed(_rounded_points(_frame_rect(), 4)), Color(metal.lightened(0.45), 0.7), 1, true)
	# L-shaped brackets at each corner, each with one rivet.
	for corner in 4:
		var at: Vector2 = [art.position, Vector2(art.end.x, art.position.y), art.end, Vector2(art.position.x, art.end.y)][corner]
		var x := 1.0 if corner in [0, 3] else -1.0; var y := 1.0 if corner in [0, 1] else -1.0
		var bracket := PackedVector2Array([at + Vector2(-2 * x, -2 * y), at + Vector2(18 * x, -2 * y), at + Vector2(18 * x, 4 * y), at + Vector2(4 * x, 4 * y), at + Vector2(4 * x, 18 * y), at + Vector2(-2 * x, 18 * y)])
		draw_colored_polygon(Transform2D(0, Vector2(0.5, 1.2)) * bracket, Color(0, 0, 0, 0.45))
		_textured(bracket, metal.lightened(0.08))
		draw_polyline(_closed(bracket), metal.darkened(0.55), 1.0, true)
		if corner != 0: _rivet(at + Vector2(1 * x, 1 * y), metal, 2.2)
	draw_line(Vector2(36, 210), Vector2(SIZE.x - 36, 210), Color(trim, 0.75), 1, true)
	_draw_hex(Vector2(25, 25))

# M · Iron Forged: a heavier bevelled band with rust flecks, rivets all
# round, a frame-coloured accent under the title and an ornamented rule.
func iron_forged_build() -> void:
	_iron_title(46); _hex_label(Vector2(26, 26)); _iron_text()

func iron_forged_draw() -> void:
	var metal := _metal().darkened(0.08)
	var frame := _frame_rect()
	var art := _iron_body(8, metal, 3)
	# Rust flecks, seeded per card.
	var rng := RandomNumberGenerator.new(); rng.seed = hash(str(data.name)) + 5
	for fleck in 26:
		var side := rng.randi_range(0, 3)
		var t := rng.randf()
		var depth := rng.randf_range(1.5, 6.5)
		var at: Vector2 = [Vector2(lerpf(frame.position.x, frame.end.x, t), frame.position.y + depth), Vector2(frame.end.x - depth, lerpf(frame.position.y, frame.end.y, t)), Vector2(lerpf(frame.position.x, frame.end.x, t), frame.end.y - depth), Vector2(frame.position.x + depth, lerpf(frame.position.y, frame.end.y, t))][side]
		draw_circle(at, rng.randf_range(0.6, 1.6), Color(0.45, 0.22, 0.08, rng.randf_range(0.25, 0.55)))
	# Bevel: outer edge lit top/left, inner edge lit bottom/right.
	var light := Color(metal.lightened(0.5), 0.8); var dark := Color(metal.darkened(0.6), 0.9)
	draw_line(frame.position, Vector2(frame.end.x, frame.position.y), light, 1.2, true)
	draw_line(frame.position, Vector2(frame.position.x, frame.end.y), light, 1.2, true)
	draw_line(Vector2(frame.position.x, frame.end.y), frame.end, dark, 1.2, true)
	draw_line(Vector2(frame.end.x, frame.position.y), frame.end, dark, 1.2, true)
	draw_line(art.position, Vector2(art.end.x, art.position.y), dark, 1.5, true)
	draw_line(art.position, Vector2(art.position.x, art.end.y), dark, 1.5, true)
	draw_line(Vector2(art.position.x, art.end.y), art.end, light, 1.2, true)
	draw_line(Vector2(art.end.x, art.position.y), art.end, light, 1.2, true)
	for point in [Vector2(11, 11), Vector2(SIZE.x - 11, 11), Vector2(11, SIZE.y - 11), Vector2(SIZE.x - 11, SIZE.y - 11), Vector2(11, SIZE.y * 0.36), Vector2(SIZE.x - 11, SIZE.y * 0.36), Vector2(11, SIZE.y * 0.66), Vector2(SIZE.x - 11, SIZE.y * 0.66), Vector2(SIZE.x * 0.5, 11), Vector2(SIZE.x * 0.5, SIZE.y - 11)]:
		_rivet(point, metal, 2.6)
	# Frame-coloured accents: under the title and an ornamented rule.
	var accent := frame_color.lightened(0.15)
	draw_line(Vector2(46, 37), Vector2(SIZE.x - 22, 37), Color(accent, 0.8), 1.5, true)
	var y := 210.0
	draw_line(Vector2(34, y), Vector2(SIZE.x * 0.5 - 6, y), Color(accent, 0.75), 1, true)
	draw_line(Vector2(SIZE.x * 0.5 + 6, y), Vector2(SIZE.x - 34, y), Color(accent, 0.75), 1, true)
	draw_colored_polygon(PackedVector2Array([Vector2(SIZE.x * 0.5, y - 3.5), Vector2(SIZE.x * 0.5 + 3.5, y), Vector2(SIZE.x * 0.5, y + 3.5), Vector2(SIZE.x * 0.5 - 3.5, y)]), accent)
	_draw_hex(Vector2(26, 26))

# N · Iron Classic, clean: K without the side strips and without rivets.
func iron_clean_build() -> void:
	_iron_title(); _hex_label(Vector2(25, 25)); _iron_text()

func iron_clean_draw() -> void:
	var metal := _metal()
	var art := _iron_body(5, metal)
	_box(art.grow(1), Color.TRANSPARENT, metal.darkened(0.5), 1, 1)
	draw_polyline(_closed(_rounded_points(_frame_rect(), 4)), Color(metal.lightened(0.45), 0.7), 1, true)
	draw_line(Vector2(36, 210), Vector2(SIZE.x - 36, 210), Color(frame_color.lightened(0.15), 0.7), 1, true)
	_draw_hex(Vector2(25, 25))
