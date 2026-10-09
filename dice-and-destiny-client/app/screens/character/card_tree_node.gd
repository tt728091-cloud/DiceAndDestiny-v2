extends Button
## One card in the tree, drawn as a medallion with its art, energy and ownership.
## The canvas owns pointer behaviour; this control only draws and hit-tests.
const STYLE = preload("res://app/screens/character/character_style.gd")
const WIDTH := 300.0
const TOP := 18.0
const BASE_RADIUS := 70.0
const RADIUS := 54.0

var node_id := ""
var is_root := false
var radius := RADIUS
var art: Texture2D
var title := ""
var subtitle := ""
var xp := 0
var energy := 0
var deck_count := 0
var stored_count := 0
## authored (editor), owned, available, locked, unowned.
var state := "authored"
var selected := false
var issue := ""
var editable := false
var link_target := false
var hovered := false
var _name_lines := 1
var _name := TextParagraph.new()

func setup(id: String, root: bool) -> void:
	node_id = id; is_root = root; radius = BASE_RADIUS if root else RADIUS
	flat = true; focus_mode = Control.FOCUS_NONE; clip_contents = false
	for s in ["normal", "hover", "pressed", "disabled", "focus", "hover_pressed"]: add_theme_stylebox_override(s, StyleBoxEmpty.new())
	mouse_entered.connect(func(): hovered = true; queue_redraw())
	mouse_exited.connect(func(): hovered = false; queue_redraw())
	set_meta("tree_node", id)

func refresh() -> void:
	# Wrap the name once, at the exact width it is drawn, so the plate grows to fit it.
	_name.clear()
	_name.width = _plate_rect().size.x - 12
	_name.alignment = HORIZONTAL_ALIGNMENT_CENTER
	_name.max_lines_visible = 2
	_name.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_name.add_string(title, get_theme_default_font(), 24)
	_name_lines = clampi(_name.get_line_count(), 1, 2)
	size = Vector2(WIDTH, TOP + radius * 2 + 22 + _plate_height())
	queue_redraw()

func center_offset() -> Vector2:
	return Vector2(WIDTH * 0.5, TOP + radius)

func place(world: Vector2) -> void:
	position = world - center_offset()

func knob_center() -> Vector2:
	return center_offset() + Vector2(0, -(radius + 7))

func knob_hit(local: Vector2) -> bool:
	return editable and local.distance_to(knob_center()) <= 14

func _plate_height() -> float:
	var line := get_theme_default_font().get_height(24)
	return 10.0 + line * _name_lines + 24.0 + (24.0 if not subtitle.is_empty() else 0.0) + 8.0

func _plate_rect() -> Rect2:
	return Rect2(Vector2(14, TOP + radius * 2 + 14), Vector2(WIDTH - 28, _plate_height()))

func _has_point(point: Vector2) -> bool:
	return point.distance_to(center_offset()) <= radius + 12 or _plate_rect().has_point(point) or knob_hit(point)

func _get_tooltip(_at: Vector2) -> String:
	return preload("res://presentation/battle/wrapped_tooltip.gd").content(tooltip_text)

func _make_custom_tooltip(for_text: String) -> Object:
	return preload("res://presentation/battle/wrapped_tooltip.gd").create(self, for_text)

func _process(_delta: float) -> void:
	if state == "available" or selected or link_target: queue_redraw()

func _draw() -> void:
	var c := center_offset()
	var t := Time.get_ticks_msec() / 1000.0
	var player := state != "authored"
	var lit := state == "owned" or not player
	var ring := STYLE.GOLD if state == "owned" else STYLE.BRONZE.lightened(0.15) if not player else Color("3b4552")
	if state == "available": ring = STYLE.GOLD.lerp(Color("3b4552"), 0.35 + 0.25 * sin(t * 3.0))
	if hovered or link_target: ring = ring.lightened(0.3)
	if selected: ring = STYLE.GOLD_BRIGHT
	if not issue.is_empty(): ring = STYLE.LOSS
	# Soft halo: owned and selected cards glow; available cards breathe.
	var glow := 0.0
	if state == "owned": glow = 0.9
	elif state == "available": glow = 0.45 + 0.35 * sin(t * 3.0)
	elif not player: glow = 0.35
	if selected or link_target: glow = 1.1
	var glow_color := STYLE.GOLD if issue.is_empty() else STYLE.LOSS
	for i in 5:
		draw_circle(c, radius + 6 + i * 6, Color(glow_color, glow * (0.16 - i * 0.03)))
	# Frame: dark iron disc with a metal rim; the base gets a second rim and spikes.
	if is_root:
		for k in 8:
			var a := TAU * k / 8.0 + PI / 8.0
			var dir := Vector2(cos(a), sin(a))
			var perp := Vector2(-dir.y, dir.x)
			draw_colored_polygon(PackedVector2Array([c + dir * (radius + 22), c + dir * (radius + 6) + perp * 7, c + dir * (radius + 6) - perp * 7]), Color(ring, 0.85))
		draw_circle(c, radius + 14, Color("0d1118"))
		draw_arc(c, radius + 14, 0, TAU, 72, Color(ring, 0.8), 2, true)
	draw_circle(c, radius + 8, Color("12161d"))
	draw_arc(c, radius + 8, 0, TAU, 72, ring, 3.5, true)
	draw_arc(c, radius + 2, 0, TAU, 72, Color(ring.darkened(0.45), 0.9), 2, true)
	_draw_art(c, lit)
	# Rim shading makes the art read as set into the frame.
	draw_arc(c, radius - 5, 0, TAU, 72, Color(0, 0, 0, 0.45), 10, true)
	if player and state != "owned":
		draw_circle(c, radius, Color(0.02, 0.03, 0.06, 0.5 if state == "available" else 0.66))
	if state == "locked": _draw_lock(c)
	if selected:
		var spin := t * 0.6
		for k in 16:
			var a0 := spin + TAU * k / 16.0
			draw_arc(c, radius + 15, a0, a0 + TAU / 32.0, 6, STYLE.GOLD_BRIGHT, 2.5, true)
	var font := get_theme_default_font()
	# Energy pip at the lower left of the rim.
	var pip := c + Vector2(-0.72, 0.72) * (radius + 6)
	draw_circle(pip, 18, Color("0d1720")); draw_circle(pip, 15, STYLE.ENERGY.darkened(0.25) if lit or state == "available" else Color("2c3a48"))
	draw_arc(pip, 18, 0, TAU, 24, Color("a9d6f5"), 1.5, true)
	_centered(font, str(energy), pip, 19, Color.WHITE)
	# Ownership badges at the upper right.
	var badge := c + Vector2(0.72, -0.72) * (radius + 6)
	if deck_count > 0:
		draw_circle(badge, 14, STYLE.GOLD); draw_arc(badge, 14, 0, TAU, 24, Color("2a2010"), 1.5, true)
		_centered(font, "×%d" % deck_count, badge, 14, Color("1a140a"))
		badge += Vector2(0, 30)
	if stored_count > 0:
		draw_circle(badge, 13, Color("4b5b6b")); draw_arc(badge, 13, 0, TAU, 24, Color("c9d6e0"), 1.5, true)
		_centered(font, "%d" % stored_count, badge, 14, Color.WHITE)
	if not issue.is_empty():
		var warn := c + Vector2(-0.72, -0.72) * (radius + 6)
		draw_circle(warn, 13, STYLE.LOSS); _centered(font, "!", warn, 18, Color("2a0d0a"))
	if editable and (hovered or selected):
		var k := knob_center()
		draw_circle(k, 11, Color("0d1720")); draw_arc(k, 11, 0, TAU, 24, STYLE.GOLD, 2, true)
		draw_line(k - Vector2(5, 0), k + Vector2(5, 0), STYLE.GOLD, 2); draw_line(k - Vector2(0, 5), k + Vector2(0, 5), STYLE.GOLD, 2)
	# Name plate.
	var plate := _plate_rect()
	var border := STYLE.GOLD if selected else Color(ring, 0.7)
	draw_style_box(STYLE.box(Color(0.03, 0.05, 0.08, 0.88), border, 0, 8, 2 if selected else 1), plate)
	var name_color := STYLE.IVORY if lit or state == "available" else STYLE.MUTED
	var name_at := Vector2(plate.position.x + 6, plate.position.y + 8)
	_name.draw_outline(get_canvas_item(), name_at, 4, Color(0, 0, 0, 0.8))
	_name.draw(get_canvas_item(), name_at, name_color)
	var y := name_at.y + font.get_ascent(24) + font.get_height(24) * (_name_lines - 1) + 24.0
	var xp_line := ("BASE · %d XP" % xp) if is_root else ("%d XP" % xp)
	draw_string(font, Vector2(plate.position.x + 6, y), xp_line, HORIZONTAL_ALIGNMENT_CENTER, plate.size.x - 12, 18, STYLE.GOLD if lit else STYLE.DIM)
	if not subtitle.is_empty():
		y += 24
		var line := _fit(font, subtitle, plate.size.x - 16, 18)
		draw_string(font, Vector2(plate.position.x + 8, y), line, HORIZONTAL_ALIGNMENT_CENTER, plate.size.x - 16, 18, STYLE.GAIN.lerp(STYLE.IVORY, 0.35) if lit or state == "available" else STYLE.DIM)

## Rect covering the medallion and its name plate, for layout checks.
func footprint() -> Rect2:
	return Rect2(position + Vector2(14, TOP - 6), Vector2(WIDTH - 28, radius * 2 + 20 + _plate_height() + 4))

func _fit(font: Font, text: String, width: float, font_size: int) -> String:
	if font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x <= width: return text
	var out := text
	while out.length() > 1 and font.get_string_size(out + "…", HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x > width: out = out.substr(0, out.length() - 1)
	return out.strip_edges() + "…"

func _draw_art(c: Vector2, lit: bool) -> void:
	var points := PackedVector2Array(); var uvs := PackedVector2Array(); var colors := PackedColorArray()
	var tint := Color.WHITE if lit or state == "available" else Color(0.55, 0.58, 0.66)
	if art == null:
		draw_circle(c, radius, Color("1b2a38"))
		draw_circle(c, radius * 0.7, Color("22384a"))
		_centered(get_theme_default_font(), title.substr(0, 1).to_upper(), c, int(radius * 0.9), Color(STYLE.GOLD, 0.85))
		return
	# Card art is portrait. Crop a centred square slightly above the middle.
	var aspect := float(art.get_width()) / maxf(1.0, float(art.get_height()))
	var span := Vector2(1.0, aspect) if aspect < 1.0 else Vector2(1.0 / aspect, 1.0)
	var middle := Vector2(0.5, clampf(0.42, span.y * 0.5, 1.0 - span.y * 0.5))
	for i in 48:
		var a := TAU * i / 48.0
		var d := Vector2(cos(a), sin(a))
		points.append(c + d * radius); uvs.append(middle + d * span * 0.5); colors.append(tint)
	draw_polygon(points, colors, uvs, art)

func _draw_lock(c: Vector2) -> void:
	var body := Rect2(c + Vector2(-13, -4), Vector2(26, 20))
	draw_arc(c + Vector2(0, -6), 9, PI, TAU, 16, STYLE.MUTED, 3.5, true)
	draw_rect(body, STYLE.MUTED)
	draw_circle(c + Vector2(0, 5), 3, Color("12161d"))

func _centered(font: Font, text: String, at: Vector2, font_size: int, color: Color) -> void:
	var w := font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x
	draw_string(font, at + Vector2(-w * 0.5, font.get_ascent(font_size) * 0.5 - font.get_descent(font_size) * 0.35), text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size, color)
