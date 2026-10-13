extends RefCounted
## The Surveyor's Chart: palette, fonts, glyphs, procedural textures and
## drawing helpers shared by the expedition map screen and its layers.
## Everything here is presentation only; the map state comes from the
## authority's viewer-safe view.

const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")

# Paper and ink.
const INK := Color("211e19")
const SEPIA := Color("6e5a3f")
const SEPIA_TEXT := Color("5a4a36")
const PENCIL := Color("8a7a62")
const CORE_PENCIL := Color("6b3f5e")
const PAPER := Color("e8dcc0")
const PAPER_LIGHT := Color("f1e7cf")
const BONE := Color("efe3c6")
const BONE_FADED := Color("e3d6b8")
const PAPER_SHADE := Color("c9b48f")
const CHIT_IMPASSABLE := Color("d8d0c0")
const GOLD_PAPER := Color("8a7146")
const GOLD_LEAF := Color("c9a35a")
const GOLD_BRIGHT := Color("e6c17c")
const COIN := Color("ffdb78")
const RUBRIC := Color("8c2f2a")
const ENEMY_RIM := Color("a63a33")
const ENEMY_INNER := Color("6b2622")
const WAX_RING := Color("5e1d1b")
const RED_ON_STONE := Color("e8826f")
const TARGET := Color("c9772b")
const NIGHT_BLUE := Color("2f4f86")
const GREEN := Color("2f7d4f")
const IVORY := Color("e9e1d1")
const HUD_IVORY := Color("fff3d9")
const JOURNAL_OLD := Color("c9bda6")
const DARK_KEYLINE := Color("100e13")
const ENEMY_FILL := Color("2a2628")
const WARLORD_FILL := Color("1d1a1c")
const PEWTER := Color("3b3a42")
const PEWTER_FACE := Color("8d8e96")
const SUN := Color("f3c64d")
const MOON := Color("9fb3d9")
const INN_GLOW := Color("ffb45a")
const HEALTH_FILL := Color("cf424f")
const HEALTH_EDGE := Color("f48279")
const STATUS := Color("ef9e91")
const ITEM := Color("e0a96d")
const WOUNDED := Color("f08a7e")
const STONE := Color("1c1a1f")
const STONE_KEYLINE := Color("5c4b33")
const DARK_SURFACE := Color("282728")
const BRONZE := Color("756446")
const NIGHT_GRADE := Color(0.78, 0.84, 1.0)
const SELECTED := Color("f4ead3")

## Terrain washes, triple-coded by hue and luminance (lightest to darkest).
const WASH := {
	"plains": Color("b5c27a"), "mountain": Color("a39aa6"), "hills": Color("c49a5e"),
	"lake": Color("6d93b4"), "marsh": Color("7d8f7a"), "forest": Color("4f6e45"),
}
const WASH_UNKNOWN := Color("a39a8a")

## Edge i runs corner i → i+1 (corners at 60i − 30°); its neighbour across.
const EDGE_NEIGHBOUR := [Vector2i(1, 0), Vector2i(0, 1), Vector2i(-1, 1), Vector2i(-1, 0), Vector2i(0, -1), Vector2i(1, -1)]

const HABITATS := {
	"plains": "salt_dunes", "hills": "shrouded_ravine", "forest": "dead_orchard", "marsh": "peat_bog",
	"lake": "storm_shore", "mountain": "fractured_glacier",
}

## Glyphs in the battle_icons convention (viewBox -2 -2 36 36).
const SHAPES := {
	"motif_plains": '<path d="M6 22q4-8 8 0M18 20q4-8 8 0M12 27q3-5 6 0"/>',
	"motif_hills": '<path d="M3 24q13-18 26 0M9 26q7-9 14 0"/>',
	"motif_forest": '<path d="M16 3L6 21H26Z"/><path d="M16 21V29"/>',
	"motif_marsh": '<path d="M10 29V12M16 29V6M22 29V13M10 12q-4-4-6-1M22 13q4-4 6-1"/>',
	"motif_lake": '<path d="M3 14q6-6 13 0t13 0M3 22q6-6 13 0t13 0"/>',
	"motif_mountain": '<path d="M3 27L13 8l5 7 3-4 8 16Z"/><path d="M11 14l2 3 3-4"/>',
	"site_village": '<path d="M5 15L16 5l11 10M8 13v15h16V13M21 7v5"/><path d="M14 28v-8h4v8"/>',
	"site_monastery": '<path d="M8 29V12a8 8 0 0 1 16 0v17M5 29h22"/><path d="M16 14v2M12 24h8l-1-5h-6z"/>',
	"site_town": '<path d="M3 29V17h5v-6h4v6h8v-9h4v9h5v12Z"/><path d="M14 29v-6h4v6"/>',
	"site_ruins": '<path d="M8 29V12l4-2M20 29V9l4 2M4 29h24"/><path d="M12 29v-5h5v5"/>',
	"site_grotto": '<path d="M3 29V17Q16 2 29 17v12Z"/><path d="M10 29v-6q6-7 12 0v6Z" fill="#211e19"/>',
	"site_keep": '<path d="M5 29V12H9V8H13V12H19V8H23V12H27V29ZM13 29V21H19V29"/>',
	"site_tower": '<path d="M11 29V8h10v21M9 8h14M16 8V2h6l-6 3"/><path d="M14 29v-6h4v6"/>',
	"sun": '<circle cx="16" cy="16" r="6"/><path d="M16 2v4M16 26v4M2 16h4M26 16h4M6 6l3 3M23 23l3 3M6 26l3-3M23 9l3-3"/>',
	"moon": '<path d="M20 3a13 13 0 1 0 9 22A11 11 0 0 1 20 3Z"/>',
	"closed_eye": '<path d="M4 14q12 12 24 0"/><path d="M8 19l-2 3M16 21v4M24 19l2 3"/>',
	"footprints": '<ellipse cx="11" cy="20" rx="3" ry="5"/><ellipse cx="21" cy="11" rx="3" ry="5"/><circle cx="11" cy="12" r="1.5"/><circle cx="21" cy="3" r="1.5"/>',
	"coin": '<circle cx="16" cy="16" r="12"/><circle cx="16" cy="16" r="6"/>',
	"pawn": '<path d="M16 2.5a4.5 4.5 0 1 1 0 9a4.5 4.5 0 0 1 0-9ZM12 12h8l3 9H9ZM6 29q10-6 20 0Z"/>',
	"pawn_highlight": '<path d="M13 5q-1 2 0 4M11 14l-1 5"/>',
	"compass_diamond": '<path d="M16 2l3 11 11 3-11 3-3 11-3-11-11-3 11-3Z"/>',
	"leaf": '<path d="M6 26C6 12 16 6 28 6c0 12-6 22-20 22M8 24L24 10"/>',
	"tent": '<path d="M3 28L16 6l13 22ZM16 6v22M11 28l5-9 5 9"/>',
	"wax_seal": '<circle cx="16" cy="16" r="11"/>',
	"die": '<rect x="4" y="4" width="24" height="24" rx="5"/><circle cx="10" cy="10" r="1.6"/><circle cx="22" cy="22" r="1.6"/><circle cx="16" cy="16" r="1.6"/>',
	"pennant": '<path d="M9 30V3M9 4H25L20 10 25 16H9"/>',
}

static var _glyphs: Dictionary = {}
static var _speckle: Texture2D
static var _hatch: Texture2D
static var _vignettes: Dictionary = {}
static var _habitat_name := ""
static var _habitat: Texture2D
static var _chit_style: StyleBoxFlat
static var _chip_styles: Dictionary = {}

# Fonts ------------------------------------------------------------------------
# One instance per face, shared by every label and every _draw call, so the
# face is loaded once and text drawn on the chart never shows fallback boxes.
static var _serif: SystemFont
static var _serif_bold: SystemFont
static var _serif_italic: SystemFont
static var _numerals: SystemFont

static func serif() -> SystemFont:
	if _serif == null:
		_serif = SystemFont.new()
		_serif.font_names = PackedStringArray(["Georgia", "Noto Serif", "DejaVu Serif", "serif"])
	return _serif

static func serif_bold() -> SystemFont:
	if _serif_bold == null:
		_serif_bold = SystemFont.new()
		_serif_bold.font_names = PackedStringArray(["Georgia", "Noto Serif", "DejaVu Serif", "serif"])
		_serif_bold.font_weight = 700
	return _serif_bold

static func serif_italic() -> SystemFont:
	if _serif_italic == null:
		_serif_italic = SystemFont.new()
		_serif_italic.font_names = PackedStringArray(["Georgia", "Noto Serif", "DejaVu Serif", "serif"])
		_serif_italic.font_italic = true
	return _serif_italic

static func numerals() -> SystemFont:
	if _numerals == null: _numerals = CINEMATIC.roll_control_font()
	return _numerals

## INK serif text and the battle's dark tooltips for the ledger and overlays.
static func ledger_theme() -> Theme:
	var t := Theme.new()
	var font := serif()
	t.default_font = font; t.default_font_size = 14
	t.set_font("font", "Label", font); t.set_font_size("font_size", "Label", 14)
	t.set_color("font_color", "Label", INK)
	t.set_color("font_shadow_color", "Label", Color.TRANSPARENT)
	t.set_font("font", "Button", font); t.set_font_size("font_size", "Button", 14)
	t.set_font("font", "TooltipLabel", font); t.set_font_size("font_size", "TooltipLabel", 18)
	t.set_stylebox("panel", "TooltipPanel", CINEMATIC.panel(DARK_SURFACE, CINEMATIC.GOLD, 14))
	t.set_color("font_color", "TooltipLabel", IVORY)
	return t

# Glyphs -------------------------------------------------------------------------
## A 48 px texture of a shape in a colourway: "ink" (ink stroke with a cream
## keyline), "ivory" (ivory on ink), "gold" (filled gold silhouette), "sepia",
## "red" and "paper".
static func glyph(id: String, colourway := "ink") -> Texture2D:
	var key := id + ":" + colourway
	if _glyphs.has(key): return _glyphs[key]
	var shape: String = SHAPES.get(id, ICONS.SHAPES.get(id, '<path d="M16 2L30 16 16 30 2 16Z"/>'))
	var svg := ""
	match colourway:
		"ivory":
			svg = '<g fill="none" stroke="#100e13" stroke-width="5.5">%s</g><g fill="none" stroke="#fff3d9" stroke-width="2.8">%s</g>' % [shape, shape]
		"gold":
			var fill := shape.replace('<path d=', '<path fill="#e6c17c" d=')
			svg = '<g fill="#e6c17c" stroke="#211e19" stroke-width="1.6">%s</g>' % fill
			if id == "pawn": svg += '<g fill="none" stroke="#fff1c9" stroke-width="1.5">%s</g>' % SHAPES["pawn_highlight"]
		"sepia":
			svg = '<g fill="none" stroke="#f1e7cf" stroke-opacity="0.5" stroke-width="5">%s</g><g fill="none" stroke="#6e5a3f" stroke-width="2.6">%s</g>' % [shape, shape]
		"red":
			svg = '<g fill="none" stroke="#f1e7cf" stroke-opacity="0.6" stroke-width="5.5">%s</g><g fill="none" stroke="#8c2f2a" stroke-width="2.8">%s</g>' % [shape, shape]
		"paper":
			svg = '<g fill="none" stroke="#efe3c6" stroke-width="2.8">%s</g>' % shape
		"wax":
			svg = '<circle cx="16" cy="16" r="11" fill="#8c2f2a" stroke="#5e1d1b" stroke-width="2"/><circle cx="12" cy="12" r="2.5" fill="#c5564d"/>'
		_:
			svg = '<g fill="none" stroke="#f1e7cf" stroke-opacity="0.6" stroke-width="5.5">%s</g><g fill="none" stroke="#211e19" stroke-width="2.8">%s</g>' % [shape, shape]
	var document := '<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="-2 -2 36 36"><g stroke-linecap="round" stroke-linejoin="round">%s</g></svg>' % svg
	var image := Image.new(); image.load_svg_from_string(document)
	_glyphs[key] = ImageTexture.create_from_image(image)
	return _glyphs[key]

# Procedural textures ----------------------------------------------------------
## A 256×256 seamless mottle: white with the noise as alpha.
static func speckle() -> Texture2D:
	if _speckle != null: return _speckle
	var noise := FastNoiseLite.new(); noise.noise_type = FastNoiseLite.TYPE_SIMPLEX_SMOOTH; noise.frequency = 0.035; noise.fractal_octaves = 3; noise.seed = 7
	var grey := noise.get_seamless_image(256, 256)
	var image := Image.create(256, 256, false, Image.FORMAT_RGBA8)
	for y in 256:
		for x in 256:
			image.set_pixel(x, y, Color(1, 1, 1, grey.get_pixel(x, y).r))
	_speckle = ImageTexture.create_from_image(image)
	return _speckle

## A 32×32 diagonal pencil hatch that tiles.
static func hatch() -> Texture2D:
	if _hatch != null: return _hatch
	var image := Image.create(32, 32, false, Image.FORMAT_RGBA8)
	image.fill(Color(1, 1, 1, 0))
	for i in 32:
		image.set_pixel(i, 31 - i, Color(1, 1, 1, 1))
		if 30 - i >= 0: image.set_pixel(i, 30 - i, Color(1, 1, 1, 0.5))
	_hatch = ImageTexture.create_from_image(image)
	return _hatch

## The radial darkening over the map area (transparent to black at the edge).
static func vignette(night: bool) -> Texture2D:
	var key := "night" if night else "day"
	if _vignettes.has(key): return _vignettes[key]
	var gradient := Gradient.new()
	gradient.offsets = PackedFloat32Array([0.0, 0.55, 1.0])
	gradient.colors = PackedColorArray([Color(0, 0, 0, 0), Color(0, 0, 0, 0), Color(0, 0, 0, 0.55 if night else 0.35)])
	var texture := GradientTexture2D.new(); texture.gradient = gradient; texture.width = 512; texture.height = 288
	texture.fill = GradientTexture2D.FILL_RADIAL; texture.fill_from = Vector2(0.5, 0.5); texture.fill_to = Vector2(1.0, 0.5)
	_vignettes[key] = texture
	return texture

## The torn parchment sheet, stretched between its 44 px torn bands.
static func chart_paper() -> StyleBoxTexture:
	var style := StyleBoxTexture.new()
	style.texture = load("res://assets/battle/wasteland/parchment.png")
	style.region_rect = Rect2(0, 0, 2172, 724)
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		style.set_texture_margin(side, 44)
		style.set_content_margin(side, 0)
	return style

## A parchment card (torn nine-slice) holding a VBox.
static func paper_card(parent: Node, padding: int = 10) -> VBoxContainer:
	var panel := PanelContainer.new()
	var style := CINEMATIC.paper()
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]: style.set_content_margin(side, padding)
	panel.add_theme_stylebox_override("panel", style)
	parent.add_child(panel)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 6); box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	panel.add_child(box)
	return box

## A small parchment chip style (for tags under 40 px).
static func chip_style(fill: Color = BONE, rim: Color = Color(INK, 0.4), padding_x: int = 6, padding_y: int = 2) -> StyleBoxFlat:
	var style := StyleBoxFlat.new(); style.bg_color = fill; style.border_color = rim
	style.set_border_width_all(1); style.set_corner_radius_all(3)
	style.content_margin_left = padding_x; style.content_margin_right = padding_x
	style.content_margin_top = padding_y; style.content_margin_bottom = padding_y
	return style

static func stone_box(fill: Color = STONE, rim: Color = STONE_KEYLINE, padding: int = 10, radius: int = 6) -> StyleBoxFlat:
	var style := StyleBoxFlat.new(); style.bg_color = fill; style.border_color = rim
	style.set_border_width_all(1); style.set_corner_radius_all(radius)
	style.content_margin_left = padding; style.content_margin_right = padding
	style.content_margin_top = padding; style.content_margin_bottom = padding
	return style

## Paper button lettering and faces (the battle's paper_button) plus a rim.
static func paper_button(button: Button, font_size: int = 14, bold: bool = false) -> Button:
	CINEMATIC.paper_button(button)
	button.add_theme_font_override("font", serif_bold() if bold else serif())
	button.add_theme_font_size_override("font_size", font_size)
	button.add_theme_color_override("font_disabled_color", Color("625a4c"))
	return button

## A coloured rim drawn over a button (gold for the primary action, red for danger).
static func rim(button: Button, color: Color, width: float = 2.0) -> Button:
	button.set_meta("rim_color", color); button.set_meta("rim_width", width)
	if not button.has_meta("rim_connected"):
		button.set_meta("rim_connected", true)
		button.draw.connect(func():
			if button.disabled: return
			var c: Color = button.get_meta("rim_color", color)
			button.draw_rect(Rect2(Vector2.ZERO, button.size).grow(-1.0), c, false, float(button.get_meta("rim_width", width))))
	button.queue_redraw()
	return button

## Ivory lettering with an ink keyline, for text directly on the stone.
static func stone_text(label: Label, bold: bool = false, color: Color = HUD_IVORY) -> Label:
	CINEMATIC.hud_lettering(label, bold)
	label.add_theme_color_override("font_color", color)
	return label

static func caption(parent: Node, text: String, color: Color = GOLD_PAPER, size: int = 12) -> Label:
	var label := Label.new(); label.text = text.to_upper(); label.autowrap_mode = TextServer.AUTOWRAP_OFF
	var font := FontVariation.new(); font.base_font = serif(); font.spacing_glyph = 1
	label.add_theme_font_override("font", font); label.add_theme_font_size_override("font_size", size)
	label.add_theme_color_override("font_color", color); label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	parent.add_child(label); return label

static func gold_rule(parent: Node, alpha: float = 0.7) -> HSeparator:
	var rule := HSeparator.new()
	var line := StyleBoxLine.new(); line.color = Color(GOLD_LEAF, alpha); line.thickness = 1
	rule.add_theme_stylebox_override("separator", line)
	parent.add_child(rule); return rule

static func ink_rule(parent: Node) -> HSeparator:
	var rule := HSeparator.new()
	var line := StyleBoxLine.new(); line.color = Color(INK, 0.7); line.thickness = 1
	rule.add_theme_stylebox_override("separator", line)
	parent.add_child(rule); return rule

static func ink_label(parent: Node, text: String, size: int = 14, color: Color = INK, bold: bool = false, wrap: bool = true) -> Label:
	var label := Label.new(); label.text = text; label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART if wrap else TextServer.AUTOWRAP_OFF
	if wrap: label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	label.add_theme_font_override("font", serif_bold() if bold else serif())
	label.add_theme_font_size_override("font_size", size); label.add_theme_color_override("font_color", color)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	parent.add_child(label); return label

static func numeral_label(parent: Node, text: String, size: int = 15, color: Color = INK) -> Label:
	var label := ink_label(parent, text, size, color)
	label.add_theme_font_override("font", numerals()); label.autowrap_mode = TextServer.AUTOWRAP_OFF
	return label

static func icon_rect(parent: Node, texture: Texture2D, px: int) -> TextureRect:
	var rect := TextureRect.new(); rect.texture = texture; rect.custom_minimum_size = Vector2(px, px)
	rect.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; rect.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	rect.mouse_filter = Control.MOUSE_FILTER_IGNORE; rect.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	parent.add_child(rect); return rect

# Drawing helpers ---------------------------------------------------------------
static func shift(rect: Rect2, offset: Vector2) -> Rect2:
	return Rect2(rect.position + offset, rect.size)

static func corners(center: Vector2, r: float) -> PackedVector2Array:
	var points := PackedVector2Array()
	for i in 6:
		var angle := deg_to_rad(60.0 * i - 30.0)
		points.append(center + Vector2(cos(angle), sin(angle)) * r)
	return points

static func draw_centered(canvas: CanvasItem, font: Font, center: Vector2, text: String, size: int, color: Color, outline_size: int = 0, outline_color: Color = BONE) -> void:
	var width := font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, size).x
	var baseline := center.y + (font.get_ascent(size) - font.get_descent(size)) * 0.5
	var at := Vector2(center.x - width * 0.5, baseline)
	if outline_size > 0: canvas.draw_string_outline(font, at, text, HORIZONTAL_ALIGNMENT_LEFT, -1, size, outline_size, outline_color)
	canvas.draw_string(font, at, text, HORIZONTAL_ALIGNMENT_LEFT, -1, size, color)

static func draw_glyph(canvas: CanvasItem, texture: Texture2D, center: Vector2, px: float, modulate: Color = Color.WHITE) -> void:
	if texture == null: return
	canvas.draw_texture_rect(texture, Rect2(center - Vector2.ONE * px * 0.5, Vector2.ONE * px), false, modulate)

static func draw_ring(canvas: CanvasItem, points: PackedVector2Array, color: Color, width: float) -> void:
	var closed := PackedVector2Array(points); closed.append(points[0])
	canvas.draw_polyline(closed, color, width, true)

static func draw_dashed_ring(canvas: CanvasItem, points: PackedVector2Array, color: Color, width: float, dash: float) -> void:
	for i in 6: canvas.draw_dashed_line(points[i], points[(i + 1) % 6], color, width, dash, true, true)

## The keyline + colour + glow stack of a highlight ring.
static func draw_highlight(canvas: CanvasItem, center: Vector2, r: float, color: Color, glow: bool = true) -> void:
	draw_ring(canvas, corners(center, r - 1.5), Color(INK, 0.8), 1.0)
	var ring := corners(center, r - 2.5)
	if glow:
		draw_ring(canvas, ring, Color(color, 0.12), 8.0)
		draw_ring(canvas, ring, Color(color, 0.28), 5.0)
	draw_ring(canvas, ring, color, 2.5)

## A parchment cost chit with a lining numeral; red for 3+, blue at night.
static func draw_chit(canvas: CanvasItem, center: Vector2, radius: float, cost: int, passable: bool, cover: bool, night: bool, faded: bool, survey_pair: String = "") -> void:
	var font := numerals()
	var size := maxi(14, int(radius * 1.55))
	var ink := SEPIA if faded else INK
	if not survey_pair.is_empty():
		var w := radius * 2.9; var h := radius * 1.7
		var rect := Rect2(center - Vector2(w, h) * 0.5, Vector2(w, h))
		canvas.draw_rect(shift(rect.grow(1.0), Vector2(0, 1.5)), Color(0, 0, 0, 0.25))
		canvas.draw_style_box(_chit_box(BONE_FADED if faded else BONE, ink), rect)
		if not passable:
			_draw_cross(canvas, center, radius * 0.5, ink)
			return
		var parts := survey_pair.split("/")
		var day_w := font.get_string_size(parts[0], HORIZONTAL_ALIGNMENT_LEFT, -1, size).x
		var slash_w := font.get_string_size("/", HORIZONTAL_ALIGNMENT_LEFT, -1, size).x
		var night_w := font.get_string_size(parts[1], HORIZONTAL_ALIGNMENT_LEFT, -1, size).x
		var total := day_w + slash_w + night_w
		var x := center.x - total * 0.5
		var baseline := center.y + (font.get_ascent(size) - font.get_descent(size)) * 0.5
		canvas.draw_string(font, Vector2(x, baseline), parts[0], HORIZONTAL_ALIGNMENT_LEFT, -1, size, RUBRIC if int(parts[0]) >= 3 and not faded else ink)
		canvas.draw_string(font, Vector2(x + day_w, baseline), "/", HORIZONTAL_ALIGNMENT_LEFT, -1, size, SEPIA)
		canvas.draw_string(font, Vector2(x + day_w + slash_w, baseline), parts[1], HORIZONTAL_ALIGNMENT_LEFT, -1, size, SEPIA if faded else NIGHT_BLUE)
		return
	if radius < 10.0:
		if passable: draw_centered(canvas, font, center, str(cost), 14, ink, 2, BONE)
		else: _draw_cross(canvas, center, 5.0, ink)
		return
	canvas.draw_circle(center + Vector2(1, 1.5), radius, Color(0, 0, 0, 0.25))
	canvas.draw_circle(center, radius, CHIT_IMPASSABLE if not passable else (BONE_FADED if faded else BONE))
	canvas.draw_arc(center, radius, 0.0, TAU, 32, ink, 1.0, true)
	if not passable:
		_draw_cross(canvas, center, radius * 0.55, ink)
	else:
		var color := ink
		if not faded:
			if cost >= 3: color = RUBRIC
			elif night: color = NIGHT_BLUE
		draw_centered(canvas, font, center, str(cost), size, color)
	if cover: draw_glyph(canvas, glyph("leaf"), center - Vector2(radius * 0.75, radius * 0.75), radius * 0.85)

static func _draw_cross(canvas: CanvasItem, center: Vector2, half: float, color: Color) -> void:
	canvas.draw_line(center + Vector2(-half, -half), center + Vector2(half, half), color, 1.5, true)
	canvas.draw_line(center + Vector2(-half, half), center + Vector2(half, -half), color, 1.5, true)

static func _chit_box(fill: Color, rim: Color) -> StyleBoxFlat:
	var key := fill.to_html() + rim.to_html()
	if _chip_styles.has(key): return _chip_styles[key]
	var style := StyleBoxFlat.new(); style.bg_color = fill; style.border_color = rim
	style.set_border_width_all(1); style.set_corner_radius_all(3)
	_chip_styles[key] = style
	return style

## A bone chip (site slot) or its ghost; returns the rect drawn.
static func draw_chip(canvas: CanvasItem, center: Vector2, w: float, h: float, ghost: bool = false, pewter: bool = false) -> Rect2:
	var rect := Rect2(center - Vector2(w, h) * 0.5, Vector2(w, h))
	canvas.draw_rect(shift(rect, Vector2(0, 1.5)), Color(0, 0, 0, 0.25))
	if pewter:
		canvas.draw_style_box(_chit_box(PEWTER, DARK_KEYLINE), rect)
		canvas.draw_rect(rect.grow(-2.0), Color(PEWTER_FACE, 0.85))
	else:
		canvas.draw_style_box(_chit_box(BONE_FADED if ghost else BONE, SEPIA if ghost else INK), rect)
	return rect

## A counter's shadow, fill and inner ring.
static func draw_disc(canvas: CanvasItem, center: Vector2, radius: float, fill: Color, rim: Color, rim_width: float, inner: Color) -> void:
	canvas.draw_arc(center + Vector2(1, 2), radius, 0.0, TAU, 40, Color(0, 0, 0, 0.3), 1.5, true)
	canvas.draw_circle(center, radius, fill)
	canvas.draw_arc(center, radius - rim_width * 0.5 - 1.0, 0.0, TAU, 40, inner, 1.0, true)
	canvas.draw_arc(center, radius, 0.0, TAU, 40, rim, rim_width, true)

static func draw_dashed_disc(canvas: CanvasItem, center: Vector2, radius: float, color: Color, width: float) -> void:
	for i in 12:
		if i % 2 == 0: canvas.draw_arc(center, radius, TAU * i / 12.0, TAU * (i + 1) / 12.0, 4, color, width, true)

static func draw_enemy_piece(canvas: CanvasItem, center: Vector2, r: float, count: int, live: bool) -> void:
	var radius := r * 0.30
	if live:
		draw_disc(canvas, center, radius, ENEMY_FILL, ENEMY_RIM, 2.0, ENEMY_INNER)
		draw_centered(canvas, numerals(), center, str(count), maxi(14, int(r * 0.40)), IVORY, 2, DARK_KEYLINE)
	else:
		draw_dashed_disc(canvas, center, radius, SEPIA, 1.5)
		draw_centered(canvas, numerals(), center, str(count), maxi(14, int(r * 0.40)), SEPIA)

static func draw_warlord_piece(canvas: CanvasItem, center: Vector2, r: float, live: bool, fortified: bool) -> void:
	var radius := r * 0.36
	if live:
		draw_disc(canvas, center, radius, WARLORD_FILL, GOLD_LEAF, 2.5, Color("8a6f3a"))
		if fortified: canvas.draw_arc(center, radius + 2.0, 0.0, TAU, 40, ENEMY_RIM, 1.5, true)
		draw_glyph(canvas, ICONS.texture("conqueror"), center, r * 0.50)
	else:
		draw_dashed_disc(canvas, center, radius, SEPIA, 1.5)
		draw_glyph(canvas, ICONS.texture("conqueror"), center, r * 0.50, Color(1, 1, 1, 0.6))

static func draw_player_piece(canvas: CanvasItem, base: Vector2, r: float) -> void:
	canvas.draw_arc(base, r * 0.32 + 3.0, 0.0, TAU, 40, Color(GOLD_BRIGHT, 0.12), 6.0, true)
	canvas.draw_arc(base, r * 0.32 + 2.0, 0.0, TAU, 40, Color(GOLD_BRIGHT, 0.30), 4.0, true)
	canvas.draw_arc(base, r * 0.32 + 1.0, 0.0, TAU, 40, Color(GOLD_BRIGHT, 0.9), 2.0, true)
	draw_disc(canvas, base, r * 0.30, BONE, INK, 1.5, Color(INK, 0.3))
	var h := r * 0.72; var w := r * 0.48
	canvas.draw_texture_rect(glyph("pawn", "gold"), Rect2(base + Vector2(-w * 0.5, r * 0.08 - h), Vector2(w, h)), false)

# Geometry ----------------------------------------------------------------------
## Corner pairs of every edge whose neighbour is not in `cells`.
static func outer_edges(cells: Array, center_of: Callable, r: float) -> Array:
	var set := {}
	for cell in cells: set[Vector2i(cell)] = true
	var out: Array = []
	for cell in cells:
		var at := Vector2i(cell)
		var points := corners(center_of.call(at.x, at.y), r)
		for i in 6:
			if not set.has(at + EDGE_NEIGHBOUR[i]): out.append([points[i], points[(i + 1) % 6]])
	return out

static func outer_edge_count(cells: Array) -> int:
	return outer_edges(cells, func(q, r): return Vector2(q, r), 1.0).size()

## Interior edges of a cell set, each once.
static func inner_edges(cells: Array, center_of: Callable, r: float) -> Array:
	var set := {}
	for cell in cells: set[Vector2i(cell)] = true
	var out: Array = []
	for cell in cells:
		var at := Vector2i(cell)
		var points := corners(center_of.call(at.x, at.y), r)
		for i in 3:
			if set.has(at + EDGE_NEIGHBOUR[i]): out.append([points[i], points[(i + 1) % 6]])
	return out

# Paintings -----------------------------------------------------------------------
static func habitat_for(kind: String, terrain: String, site_type: String, warlord: bool, night: bool) -> String:
	var by_terrain: String = HABITATS.get(terrain, "salt_dunes")
	var by_site := by_terrain
	match site_type:
		"ruins", "keep", "tower": by_site = "shrouded_ravine"
		"town": by_site = "flooded_aqueduct"
		"grotto": by_site = "mushroom_cavern" if night else "crystal_cavern"
	match kind:
		"fight": return "ash_caldera" if warlord else (by_site if not site_type.is_empty() else by_terrain)
		"event": return by_site
		"region_end": return "ash_caldera"
	return by_terrain

static func habitat_texture(name: String) -> Texture2D:
	if name == _habitat_name and _habitat != null: return _habitat
	var path := "res://assets/battle/habitats/%s.png" % name
	if not ResourceLoader.exists(path): return null
	_habitat = load(path); _habitat_name = name
	return _habitat
