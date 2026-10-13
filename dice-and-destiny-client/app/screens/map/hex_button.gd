class_name ExpeditionHexButton
extends Button
## One hex of the expedition chart: the static tile material only — a
## watercolour wash over the parchment, mottle, pencil hatch for unknown
## ground, repeated ink motifs, a bevel and the ink grid. Everything that is
## information rather than geography (chips, pieces, highlights, cost chits)
## is drawn by the marks layer above. The button accepts clicks only inside
## the hexagon so the debug inspector can activate it like any other control.

const STYLE := preload("res://app/screens/map/map_style.gd")

const MOTIF_COUNT := {"forest": 3, "marsh": 3, "plains": 2, "hills": 2, "lake": 2, "mountain": 2}
const ANCHORS := [Vector2(-0.42, 0.10), Vector2(0.42, 0.10), Vector2(0.0, -0.12)]

var radius := 34.0
var data: Dictionary = {}
var state := "unknown"   # in_sight, remembered, unknown, unexplored
var selected := false
var highlight := ""      # "", "move", "explore", "blocked", "provoke", "target"
var slot_kind := ""      # for unexplored slot hexes
var warlord := false
## The screen, for the pan offset that keeps the mottle continuous across hexes.
var screen: Node

func _init() -> void:
	flat = true
	focus_mode = Control.FOCUS_NONE
	toggle_mode = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	for name in ["normal", "hover", "pressed", "focus", "disabled", "hover_pressed"]:
		add_theme_stylebox_override(name, StyleBoxEmpty.new())
	add_theme_color_override("font_color", Color.TRANSPARENT)

## Pointy-top hexagon corners around the centre.
static func corners(center: Vector2, r: float) -> PackedVector2Array:
	return STYLE.corners(center, r)

func _has_point(point: Vector2) -> bool:
	return Geometry2D.is_point_in_polygon(point, corners(size * 0.5, radius))

func _draw() -> void:
	if state == "unexplored": return
	var center := size * 0.5
	var points := corners(center, radius - 0.6)
	var terrain := str(data.get("terrain", ""))
	# a. Wash: the watercolour pool over the parchment.
	var wash: Color = STYLE.WASH.get(terrain, STYLE.WASH_UNKNOWN)
	var alpha := 0.72
	match state:
		"remembered": wash = wash.lerp(STYLE.PAPER_SHADE, 0.6); alpha = 0.26
		"unknown": alpha = 0.34
	draw_colored_polygon(points, Color(wash, alpha))
	# b. Mottle in world space so the grain never swims when panning.
	var pan: Vector2 = screen._pan if screen != null and "_pan" in screen else Vector2.ZERO
	var uvs := PackedVector2Array()
	for p in points: uvs.append((p + position - pan) / 256.0)
	var light := 0.16; var dark := 0.12
	if state == "remembered": light = 0.10; dark = 0.08
	elif state == "unknown": light = 0.12; dark = 0.10
	draw_colored_polygon(points, Color(1, 1, 1, light), uvs, STYLE.speckle())
	var shifted := PackedVector2Array()
	for uv in uvs: shifted.append(uv + Vector2(0.37, 0.61))
	draw_colored_polygon(points, Color(0, 0, 0, dark), shifted, STYLE.speckle())
	# c. Pencil hatch over ground whose contents were never seen.
	if state == "unknown":
		var hatch_uvs := PackedVector2Array()
		for uv in uvs: hatch_uvs.append(uv * 8.0)
		draw_colored_polygon(points, Color(STYLE.PENCIL, 0.30), hatch_uvs, STYLE.hatch())
	# d. Motifs, stable per hex.
	_draw_motifs(center, terrain)
	# e. Bevel: light on the upper-left edges, shade on the lower-right.
	var inset := corners(center, radius - 2.0)
	var faded := state != "in_sight"
	draw_polyline(PackedVector2Array([inset[3], inset[4], inset[5], inset[0]]), Color(STYLE.PAPER_LIGHT, 0.125 if faded else 0.25), 1.5, true)
	draw_polyline(PackedVector2Array([inset[0], inset[1], inset[2], inset[3]]), Color(STYLE.INK, 0.175 if faded else 0.35), 1.5, true)
	# f. Grid.
	match state:
		"in_sight": STYLE.draw_ring(self, points, Color(STYLE.INK, 0.35), 1.0)
		"remembered": STYLE.draw_ring(self, points, Color(STYLE.SEPIA, 0.45), 1.0)
		_: STYLE.draw_dashed_ring(self, points, Color(STYLE.PENCIL, 0.8), 1.0, 3.0)

func _draw_motifs(center: Vector2, terrain: String) -> void:
	if not MOTIF_COUNT.has(terrain): return
	var texture: Texture2D = STYLE.glyph("motif_" + terrain)
	var modulate := Color.WHITE
	if state == "remembered": modulate = Color(STYLE.SEPIA, 0.7)
	elif state == "unknown": modulate = Color(STYLE.PENCIL, 0.55)
	var rng := RandomNumberGenerator.new()
	rng.seed = hash(Vector2i(int(data.get("q", 0)), int(data.get("r", 0))))
	var count: int = MOTIF_COUNT[terrain]
	for index in count:
		var anchor: Vector2 = ANCHORS[index] if terrain != "mountain" else [ANCHORS[2], ANCHORS[1]][index]
		var jitter := Vector2(rng.randf_range(-0.08, 0.08), rng.randf_range(-0.08, 0.08))
		var size_factor := 0.42
		if terrain == "mountain": size_factor = 0.60 if index == 0 else 0.34
		var s := radius * size_factor
		var at := center + (anchor + jitter) * radius
		draw_texture_rect(texture, Rect2(at - Vector2.ONE * s * 0.5, Vector2.ONE * s), false, modulate)
