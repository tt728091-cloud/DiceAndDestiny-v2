class_name StoneDie
extends Control

## Carved-stone die, seen slightly from the front so the lower side shows.
## Symbols are engraved into the top face and the number is cut into its
## upper-right corner. Player dice are pale stone; enemy and faction dice are
## tinted so ownership reads at a glance. Drawn procedurally: no imported art.

enum Mode { FULL, BODY, GLYPH }

# face light, face dark, side light, side dark, ink, highlight
const PALETTES := {
	"player": [Color("ebdfc6"), Color("c5b391"), Color("9a8d76"), Color("4f4538"), Color("1c1712"), Color("fbf3e2")],
	"enemy": [Color("c2c3c0"), Color("8c8f8e"), Color("6a6e6f"), Color("2e3133"), Color("121416"), Color("f1f2ef")],
	"curse": [Color("ad9cc0"), Color("7a678f"), Color("5a4a6b"), Color("2a2133"), Color("170f1f"), Color("e4d8f0")],
	"venom": [Color("b7c296"), Color("7f8c5f"), Color("5d6943"), Color("2c3320"), Color("11160a"), Color("e8efd2")],
	"brine": [Color("a9c4c3"), Color("6c8e8f"), Color("4d6c6d"), Color("223535"), Color("0b1819"), Color("dcecec")],
}
const DIE_PALETTES := {"curse_d6": "curse", "venom_d6": "venom", "brine_d6": "brine"}
const KEPT_GLOW := Color("ffd77a")
const CURSE_BADGE := Color("4b1a78")
const CURSE_INK := Color("f0d9ff")

var mode := Mode.FULL
var die_id := "standard_d6"
var face := 0
var enemy := false
## Extra glyph cut into the lower-right corner, e.g. the Curse mark.
var mark := ""
var _state := Vector2i(-1, -1)

static var _grain: Texture2D
static var _font: SystemFont
static var _glyphs := {}

static func palette(die: String, enemy_owned: bool) -> Array:
	return PALETTES[DIE_PALETTES.get(die, "enemy" if enemy_owned else "player")]

static func ink(die: String, enemy_owned: bool) -> Color:
	return palette(die, enemy_owned)[4]

static func highlight(die: String, enemy_owned: bool) -> Color:
	return palette(die, enemy_owned)[5]

static func font() -> Font:
	if _font == null:
		_font = SystemFont.new()
		_font.font_names = PackedStringArray(["Georgia", "Noto Serif", "DejaVu Serif", "serif"])
		_font.font_weight = 700
	return _font

## Regions of a die of this size: the top face, the glyph square (normal and
## when sharing the face with a mark), the corner number and the mark.
static func layout(die_size: Vector2) -> Dictionary:
	var depth := clampf(roundf(die_size.y * 0.13), 4.0, 14.0)
	var top := Rect2(Vector2.ZERO, Vector2(die_size.x, die_size.y - depth))
	var unit := minf(top.size.x, top.size.y)
	var glyph := unit * 0.56
	var centre := top.position + Vector2(top.size.x * 0.43, top.size.y * 0.58)
	var number_size := Vector2(unit * 0.32, unit * 0.34)
	return {
		"depth": depth,
		"top": top,
		"radius": roundf(unit * 0.15),
		"glyph": Rect2(centre - Vector2(glyph, glyph) * 0.5, Vector2(glyph, glyph)),
		"glyph_marked": Rect2(top.position + Vector2(unit * 0.06, top.size.y * 0.26), Vector2(unit * 0.54, unit * 0.54)),
		"number": Rect2(Vector2(top.end.x - number_size.x - unit * 0.04, top.position.y + unit * 0.02), number_size),
		"mark": Rect2(top.end - Vector2(unit * 0.42, unit * 0.42), Vector2(unit * 0.34, unit * 0.34)),
		"font_size": int(roundf(unit * 0.33)),
	}

## Gives any die Button or Label the carved-stone look. Callers keep setting
## the control's text for tooltips and inspection; the stone hides it unless
## the face is blank (rolling hidden, "—", "HIDDEN").
static func dress(control: Control, die: String, shown_face: int, enemy_owned := false, shown_mark := "") -> StoneDie:
	var stone: StoneDie = control.get_node_or_null("StoneDie")
	if stone == null:
		stone = StoneDie.new(); stone.name = "StoneDie"
		control.add_child(stone); control.move_child(stone, 0)
		var empty := StyleBoxEmpty.new()
		var states := ["normal", "hover", "pressed", "hover_pressed", "disabled", "focus"] if control is BaseButton else ["normal"]
		for state in states: control.add_theme_stylebox_override(state, empty)
	elif stone.die_id == die and stone.face == shown_face and stone.enemy == enemy_owned and stone.mark == shown_mark:
		return stone
	stone.die_id = die; stone.face = shown_face; stone.enemy = enemy_owned; stone.mark = shown_mark
	stone.queue_redraw()
	var text_color := Color.TRANSPARENT if shown_face > 0 else ink(die, enemy_owned)
	var keys := ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"] if control is BaseButton else ["font_color"]
	for key in keys: control.add_theme_color_override(key, text_color)
	control.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	control.add_theme_color_override("font_outline_color", Color.TRANSPARENT)
	return stone

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	if mode == Mode.GLYPH: return
	# Drawn beneath the parent so its text and overlays stay on top.
	show_behind_parent = true
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)

func configure(die: String, shown_face: int, enemy_owned: bool, shown_mark := "") -> void:
	if die == die_id and shown_face == face and enemy_owned == enemy and shown_mark == mark: return
	die_id = die; face = shown_face; enemy = enemy_owned; mark = shown_mark
	queue_redraw()

# Kept and hover highlights follow the parent button without extra wiring.
func _process(_delta: float) -> void:
	var button := get_parent() as BaseButton
	if button == null or mode == Mode.GLYPH: return
	var state := Vector2i(int(button.button_pressed), int(button.is_hovered() and not button.disabled))
	if state != _state:
		_state = state
		queue_redraw()

func _draw() -> void:
	var colors := palette(die_id, enemy)
	if mode == Mode.GLYPH:
		if face > 0: draw_symbol(self, die_id, face, Rect2(Vector2.ZERO, size), colors)
		return
	var regions := layout(size)
	var top: Rect2 = regions.top
	var radius: float = regions.radius
	var depth: float = regions.depth
	var button := get_parent() as BaseButton
	var kept := button != null and button.toggle_mode and button.button_pressed
	var hovered := button != null and button.is_hovered() and not button.disabled
	# Contact shadow, then the visible front side beneath the top face.
	_fill(_rounded(Rect2(Vector2(2, depth * 0.7), size - Vector2(4, depth * 0.4)), radius), Color(0, 0, 0, 0.45))
	if kept:
		for step in 4:
			var spread := 2.0 + step * 2.0
			_outline(_rounded(top.grow(spread), radius + spread), Color(KEPT_GLOW, 0.32 - step * 0.07), 3.0)
	var body := _rounded(Rect2(Vector2(0, 2), Vector2(size.x, size.y - 2)), radius)
	draw_polygon(body, _gradient(body, Rect2(Vector2(0, top.end.y - radius), Vector2(size.x, depth + radius)), colors[2], colors[3], true))
	_outline(body, colors[3], 1.0)
	var face_points := _rounded(top, radius)
	draw_polygon(face_points, _gradient(face_points, top, colors[0], colors[1], false))
	draw_polygon(face_points, PackedColorArray([Color(1, 1, 1, 1)]), _grain_uvs(face_points), grain())
	_outline(face_points, colors[1].darkened(0.15), 1.0)
	draw_line(Vector2(radius, top.end.y + 0.5), Vector2(size.x - radius, top.end.y + 0.5), Color(colors[5], 0.45), 1.0, true)
	# A shallow raised rim, lit from the upper left.
	var inset := maxf(3.0, roundf(minf(top.size.x, top.size.y) * 0.075))
	var rim := _rounded(top.grow(-inset), maxf(2.0, radius - inset * 0.6))
	var offset := Transform2D(0.0, Vector2(1, 1))
	_outline(offset * rim, Color(colors[5], 0.7), 1.5)
	_outline(rim, Color(colors[3], 0.5), 1.2)
	if kept: _outline(face_points, Color("ffe3a0"), 2.0)
	elif hovered: _outline(face_points, Color("ffe3a0", 0.75), 1.5)
	var marked := not mark.is_empty() and face > 0
	# Body-only dice (the tray) letter the mark themselves over this badge.
	if mode == Mode.BODY:
		if marked: draw_style_box(curse_badge(), regions.mark)
		return
	if face <= 0: return
	draw_symbol(self, die_id, face, regions.glyph_marked if marked else regions.glyph, colors)
	draw_engraved_text(self, str(face), regions.number, int(regions.font_size), colors[4], colors[5])
	if marked: draw_mark(self, mark, regions.mark, int(regions.font_size))

## The Curse mark is a violet brand set into the stone so it reads on every tint.
static func curse_badge() -> StyleBoxFlat:
	var badge := StyleBoxFlat.new()
	badge.bg_color = CURSE_BADGE; badge.border_color = Color("c99cf0"); badge.set_border_width_all(1); badge.set_corner_radius_all(4)
	badge.shadow_color = Color(0, 0, 0, 0.35); badge.shadow_size = 1; badge.shadow_offset = Vector2(0, 1)
	return badge

static func draw_mark(item: CanvasItem, text: String, rect: Rect2, number_size: int) -> void:
	item.draw_style_box(curse_badge(), rect)
	var font_size := int(number_size * 1.3)
	var typeface := font()
	var width := typeface.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x
	var baseline := Vector2(rect.get_center().x - width * 0.5, rect.get_center().y + typeface.get_ascent(font_size) * 0.38)
	item.draw_string(typeface, baseline, text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size, CURSE_INK)

static func draw_engraved_text(item: CanvasItem, text: String, rect: Rect2, font_size: int, color: Color, light: Color) -> void:
	var typeface := font()
	var width := typeface.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x
	var ascent := typeface.get_ascent(font_size)
	var baseline := Vector2(rect.get_center().x - width * 0.5, rect.get_center().y + ascent * 0.38)
	var lift := maxf(1.0, font_size / 16.0)
	item.draw_string(typeface, baseline + Vector2(lift, lift), text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size, Color(light, 0.85))
	item.draw_string(typeface, baseline, text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size, color)

## Engraves a die face's symbol into `rect`: a lit lower edge, the dark cut,
## then any uncarved details left standing in the stone colour.
static func draw_symbol(item: CanvasItem, die: String, shown_face: int, rect: Rect2, colors: Array) -> void:
	var symbol_id := BattlePresentationCatalog.symbol_id_for_die_face(die, shown_face)
	var shapes := glyph(symbol_id)
	if shapes.is_empty():
		var glyph_text := BattlePresentationCatalog.symbol_for_die_face(die, shown_face)
		draw_engraved_text(item, glyph_text, rect, int(rect.size.y * 0.8), colors[4], colors[5])
		return
	var scale := rect.size.x / 100.0
	var lift := maxf(1.0, rect.size.x / 34.0)
	_draw_shapes(item, shapes.solid, Transform2D(0.0, Vector2(scale, scale), 0.0, rect.position + Vector2(lift, lift)), Color(colors[5], 0.85))
	var xf := Transform2D(0.0, Vector2(scale, scale), 0.0, rect.position)
	_draw_shapes(item, shapes.solid, xf, colors[4])
	_draw_shapes(item, shapes.get("inner", []), xf, colors[1])

static func _draw_shapes(item: CanvasItem, shapes: Array, xf: Transform2D, color: Color) -> void:
	var scale := xf.get_scale().x
	for shape in shapes:
		match str(shape[0]):
			"poly":
				var points: PackedVector2Array = xf * PackedVector2Array(shape[1])
				item.draw_colored_polygon(points, color)
				var closed := points.duplicate(); closed.append(points[0])
				item.draw_polyline(closed, color, 1.0, true)
			"circle":
				item.draw_circle(xf * Vector2(shape[1]), float(shape[2]) * scale, color, true, -1.0, true)
			"ring":
				item.draw_arc(xf * Vector2(shape[1]), float(shape[2]) * scale, 0.0, TAU, 40, color, float(shape[3]) * scale, true)
			"line":
				var line: PackedVector2Array = xf * PackedVector2Array(shape[1])
				var width := float(shape[2]) * scale
				item.draw_polyline(line, color, width, true)
				item.draw_circle(line[0], width * 0.5, color, true, -1.0, true)
				item.draw_circle(line[line.size() - 1], width * 0.5, color, true, -1.0, true)

# ---- Symbols, authored in a 100×100 box ----------------------------------

static func glyph(symbol_id: String) -> Dictionary:
	if not _glyphs.has(symbol_id): _glyphs[symbol_id] = _build_glyph(symbol_id)
	return _glyphs[symbol_id]

static func _build_glyph(symbol_id: String) -> Dictionary:
	match symbol_id:
		"sword":
			# Authored upright, then turned so the blade points up and right.
			var turn := Transform2D(PI / 4.0, Vector2(1.18, 1.18), 0.0, Vector2(50, 50)) * Transform2D(0.0, Vector2(-50, -50))
			return {
				"solid": [
					["poly", turn * PackedVector2Array([Vector2(50, 0), Vector2(64, 15), Vector2(64, 60), Vector2(36, 60), Vector2(36, 15)])],
					["poly", turn * _rounded(Rect2(19, 58, 62, 14), 5.0)],
					["poly", turn * PackedVector2Array([Vector2(43, 70), Vector2(57, 70), Vector2(57, 87), Vector2(43, 87)])],
					["circle", turn * Vector2(50, 92), 9.0 * 1.18],
				],
				"inner": [["line", turn * PackedVector2Array([Vector2(50, 14), Vector2(50, 54)]), 5.0 * 1.18]],
			}
		"shield":
			return {
				"solid": [["poly", _path("M50 5 L89 17 L89 44 C89 70 71 86 50 96 C29 86 11 70 11 44 L11 17 Z")]],
				"inner": [["line", PackedVector2Array([Vector2(50, 17), Vector2(50, 85)]), 6.0], ["line", PackedVector2Array([Vector2(21, 42), Vector2(79, 42)]), 6.0]],
			}
		"gold_coin":
			return {
				"solid": [["circle", Vector2(50, 50), 42.0]],
				"inner": [["ring", Vector2(50, 50), 30.0, 5.0], ["poly", PackedVector2Array([Vector2(50, 33), Vector2(62, 50), Vector2(50, 67), Vector2(38, 50)])]],
			}
		"curse_skull":
			return {
				"solid": [["poly", _path("M50 6 C75 6 90 23 90 46 C90 60 82 68 75 72 L75 86 C75 90 72 93 68 93 L32 93 C28 93 25 90 25 86 L25 72 C18 68 10 60 10 46 C10 23 25 6 50 6 Z")]],
				"inner": [
					["circle", Vector2(34, 47), 11.0], ["circle", Vector2(66, 47), 11.0],
					["poly", PackedVector2Array([Vector2(50, 60), Vector2(57, 71), Vector2(43, 71)])],
					["line", PackedVector2Array([Vector2(40, 80), Vector2(40, 90)]), 4.0], ["line", PackedVector2Array([Vector2(50, 80), Vector2(50, 90)]), 4.0], ["line", PackedVector2Array([Vector2(60, 80), Vector2(60, 90)]), 4.0],
				],
			}
		"curse_shroud":
			return {
				"solid": [["poly", _path("M50 5 C75 5 87 26 87 52 L87 95 L73 85 L61 95 L50 85 L39 95 L27 85 L13 95 L13 52 C13 26 25 5 50 5 Z")]],
				"inner": [["poly", _ellipse(Vector2(50, 42), Vector2(15, 19))]],
			}
		"curse_omen":
			return {"solid": [["poly", PackedVector2Array([Vector2(50, 3), Vector2(60, 40), Vector2(97, 50), Vector2(60, 60), Vector2(50, 97), Vector2(40, 60), Vector2(3, 50), Vector2(40, 40)])]]}
		# Venom shapes follow the wasteland ability icons (48-unit originals).
		"fang":
			return {"solid": [["poly", _path("M10 7 C15 3 19 7 24 8 C29 7 34 3 39 8 C44 15 38 23 36 32 C34 42 31 46 28 40 L25 27 Q23 23 21 29 L17 41 Q13 47 11 34 C10 25 4 14 10 7 Z", 100.0 / 48.0)]]}
		"gland":
			return {
				"solid": [["poly", _path("M18 4 H30 V10 H28 V17 L36 22 V42 Q24 47 12 42 V22 L20 17 V10 H18 Z", 100.0 / 48.0)]],
				"inner": [["poly", _path("M17 27 Q24 24 31 28 V39 Q24 43 17 39 Z", 100.0 / 48.0)], ["line", PackedVector2Array([Vector2(39, 25), Vector2(61, 25)]), 5.0]],
			}
		"coil":
			return {"solid": [["ring", Vector2(50, 50), 42.0, 7.0], ["line", _path("M29 34 C14 41 10 22 19 16 C28 9 39 23 28 28 C22 31 19 23 24 21", 100.0 / 48.0), 9.0]]}
		"brine":
			var waves := []
			for base in [26.0, 50.0, 74.0]:
				var points := PackedVector2Array()
				for step in 41:
					var x := 10.0 + step * 2.0
					points.append(Vector2(x, base - 6.0 * sin(PI * (x - 10.0) / 20.0)))
				waves.append(["line", points, 11.0])
			return {"solid": waves}
		"salt":
			return {
				"solid": [["poly", PackedVector2Array([Vector2(50, 5), Vector2(88, 27), Vector2(88, 73), Vector2(50, 95), Vector2(12, 73), Vector2(12, 27)])]],
				"inner": [["line", PackedVector2Array([Vector2(50, 50), Vector2(50, 10)]), 4.0], ["line", PackedVector2Array([Vector2(50, 50), Vector2(84, 70)]), 4.0], ["line", PackedVector2Array([Vector2(50, 50), Vector2(16, 70)]), 4.0]],
			}
	return {}

## Absolute M/L/H/V/C/Q/Z path data sampled into one point list.
static func _path(data: String, scale := 1.0) -> PackedVector2Array:
	const ARGUMENTS := {"M": 2, "L": 2, "H": 1, "V": 1, "C": 6, "Q": 4}
	var tokens := []
	for found in RegEx.create_from_string("[MLHVCQZ]|-?\\d*\\.?\\d+").search_all(data): tokens.append(found.get_string())
	var points := PackedVector2Array()
	var cursor := Vector2.ZERO
	var command := ""
	var i := 0
	while i < tokens.size():
		if not str(tokens[i]).is_valid_float():
			command = tokens[i]; i += 1
			continue
		var count: int = ARGUMENTS.get(command, 0)
		if count == 0 or i + count > tokens.size(): break
		var a: Array[float] = []
		for value in tokens.slice(i, i + count): a.append(float(value))
		i += count
		match command:
			"M", "L": cursor = Vector2(a[0], a[1]); points.append(cursor)
			"H": cursor = Vector2(a[0], cursor.y); points.append(cursor)
			"V": cursor = Vector2(cursor.x, a[0]); points.append(cursor)
			"C":
				var end := Vector2(a[4], a[5])
				for step in range(1, 11): points.append(cursor.bezier_interpolate(Vector2(a[0], a[1]), Vector2(a[2], a[3]), end, step / 10.0))
				cursor = end
			"Q":
				var control := Vector2(a[0], a[1]); var end := Vector2(a[2], a[3])
				for step in range(1, 9):
					var t := step / 8.0
					points.append(cursor.lerp(control, t).lerp(control.lerp(end, t), t))
				cursor = end
	var cleaned := PackedVector2Array()
	for point in points:
		if cleaned.is_empty() or not cleaned[cleaned.size() - 1].is_equal_approx(point * scale): cleaned.append(point * scale)
	if cleaned.size() > 2 and cleaned[0].is_equal_approx(cleaned[cleaned.size() - 1]): cleaned.remove_at(cleaned.size() - 1)
	return cleaned

static func _ellipse(centre: Vector2, radii: Vector2) -> PackedVector2Array:
	var points := PackedVector2Array()
	for step in 24: points.append(centre + Vector2(cos(TAU * step / 24.0) * radii.x, sin(TAU * step / 24.0) * radii.y))
	return points

static func _rounded(rect: Rect2, radius: float) -> PackedVector2Array:
	var r := minf(radius, minf(rect.size.x, rect.size.y) * 0.5)
	var points := PackedVector2Array()
	var corners := [rect.position + Vector2(rect.size.x - r, r), rect.end - Vector2(r, r), rect.position + Vector2(r, rect.size.y - r), rect.position + Vector2(r, r)]
	for corner in 4:
		for step in 7:
			var angle := -PI / 2.0 + (corner + step / 6.0) * PI / 2.0
			points.append(corners[corner] + Vector2(cos(angle), sin(angle)) * r)
	return points

# ---- Stone surface --------------------------------------------------------

## Grain and mottling as a dark, mostly transparent tile laid over the face.
static func grain() -> Texture2D:
	if _grain != null: return _grain
	var fine := FastNoiseLite.new(); fine.seed = 7; fine.frequency = 0.22; fine.fractal_octaves = 2
	var mottle := FastNoiseLite.new(); mottle.seed = 11; mottle.frequency = 0.045; mottle.fractal_octaves = 3
	var fine_image := fine.get_seamless_image(96, 96)
	var mottle_image := mottle.get_seamless_image(96, 96)
	var image := Image.create(96, 96, false, Image.FORMAT_RGBA8)
	for y in 96:
		for x in 96:
			var speck := fine_image.get_pixel(x, y).r
			var blotch := smoothstep(0.45, 0.85, mottle_image.get_pixel(x, y).r)
			image.set_pixel(x, y, Color(0.12, 0.09, 0.06, clampf(speck * 0.16 + blotch * 0.2, 0.0, 0.34)))
	_grain = ImageTexture.create_from_image(image)
	return _grain

func _grain_uvs(points: PackedVector2Array) -> PackedVector2Array:
	# Each die samples its own patch so neighbouring dice are not identical.
	var shift := Vector2(fposmod(get_instance_id() * 0.37, 1.0), fposmod(get_instance_id() * 0.61, 1.0))
	var uvs := PackedVector2Array()
	for point in points: uvs.append(point / 96.0 + shift)
	return uvs

func _gradient(points: PackedVector2Array, bounds: Rect2, from: Color, to: Color, vertical: bool) -> PackedColorArray:
	var colors := PackedColorArray()
	for point in points:
		var local := (point - bounds.position) / bounds.size
		var t := clampf(local.y if vertical else (local.x + local.y) * 0.5, 0.0, 1.0)
		colors.append(from.lerp(to, t))
	return colors

func _fill(points: PackedVector2Array, color: Color) -> void:
	draw_colored_polygon(points, color)

func _outline(points: PackedVector2Array, color: Color, width: float) -> void:
	var closed := points.duplicate(); closed.append(points[0])
	draw_polyline(closed, color, width, true)
