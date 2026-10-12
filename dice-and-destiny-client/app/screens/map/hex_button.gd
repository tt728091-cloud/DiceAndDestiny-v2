class_name ExpeditionHexButton
extends Button
## One hex of the expedition map: a pointy-top hexagon drawn from the
## authority's view (terrain, fog, site, enemy, player) that only accepts
## clicks inside the hexagon. It is a Button so the debug inspector can
## activate it like any other control.

const ICONS := preload("res://presentation/battle/battle_icons.gd")

const TERRAIN_COLORS := {
	"plains": Color("7da85a"), "hills": Color("a88a55"), "forest": Color("3f6f3c"),
	"marsh": Color("5f7a62"), "lake": Color("3b6f9c"), "mountain": Color("6e6a73"),
}
const SITE_GLYPHS := {
	"village": "⌂", "monastery": "⛩", "town": "♜", "ruins": "☗", "grotto": "◓", "event": "?", "keep": "♖", "tower": "♖",
}

var radius := 34.0
var data: Dictionary = {}
var state := "unknown"   # in_sight, remembered, unknown, unexplored
var selected := false
var highlight := ""      # "", "move", "explore", "blocked", "provoke", "target"
var slot_kind := ""      # for unexplored slot hexes
var warlord := false

func _init() -> void:
	flat = true
	focus_mode = Control.FOCUS_NONE
	toggle_mode = false
	mouse_filter = Control.MOUSE_FILTER_STOP
	for name in ["normal", "hover", "pressed", "focus", "disabled", "hover_pressed"]:
		add_theme_stylebox_override(name, StyleBoxEmpty.new())
	add_theme_color_override("font_color", Color.TRANSPARENT)

## Pointy-top hexagon corners around the centre.
static func corners(center: Vector2, r: float) -> PackedVector2Array:
	var points := PackedVector2Array()
	for i in 6:
		var angle := deg_to_rad(60.0 * i - 30.0)
		points.append(center + Vector2(cos(angle), sin(angle)) * r)
	return points

func _has_point(point: Vector2) -> bool:
	return Geometry2D.is_point_in_polygon(point, corners(size * 0.5, radius))

func _draw() -> void:
	var center := size * 0.5
	var polygon := corners(center, radius - 1.0)
	var terrain := str(data.get("terrain", ""))
	var fill: Color = TERRAIN_COLORS.get(terrain, Color("3a3f4a"))
	match state:
		"unexplored":
			fill = Color("1b2230") if slot_kind != "core" else Color("2b1f2c")
		"remembered":
			fill = fill.darkened(0.45).lerp(Color("4a4f58"), 0.35)
		"unknown":
			fill = fill.darkened(0.3)
	draw_colored_polygon(polygon, fill)
	if state == "remembered" or state == "unknown":
		# Hatching: revealed terrain out of sight.
		for i in range(-3, 4):
			var offset := i * radius * 0.45
			draw_line(center + Vector2(-radius * 0.7, offset - radius * 0.3), center + Vector2(radius * 0.7, offset + radius * 0.3), Color(0, 0, 0, 0.18), 1.0)
	var edge := Color("2a2f38")
	var width := 1.5
	if state == "in_sight": edge = Color("e6c17c"); width = 2.0
	if highlight == "move": edge = Color("8fd3ff"); width = 3.0
	if highlight == "explore": edge = Color("ffd166"); width = 3.0
	if highlight == "provoke": edge = Color("ff6b6b"); width = 3.0
	if highlight == "target": edge = Color("ff9f43"); width = 3.0
	if selected: edge = Color("ffffff"); width = 4.0
	if state == "unexplored":
		edge = Color("4a5668"); width = 1.0
		var outline := corners(center, radius - 1.0)
		for i in 6:
			draw_dashed_line(outline[i], outline[(i + 1) % 6], edge, 1.0, 5.0)
	else:
		var outline := corners(center, radius - 1.0)
		outline.append(outline[0])
		draw_polyline(outline, edge, width, true)
	var font := get_theme_default_font()
	if state == "unexplored":
		if slot_kind == "core":
			draw_string(font, center + Vector2(-16, 4), "core", HORIZONTAL_ALIGNMENT_CENTER, 32, 11, Color("b08aa8"))
		return
	# Contents: site glyph, enemy marker, the player and the Warlord.
	if data.get("site") != null:
		var site: Dictionary = data.site
		var glyph: String = SITE_GLYPHS.get(str(site.get("type", "")), "•")
		var color := Color("fff0cc")
		if site.get("defended", false): color = Color("ffb3b3")
		if site.get("conquered", false): color = Color("ff6b6b")
		if site.get("cleared", false) or site.get("resolved", false) or site.get("freed", false): color = Color("cfd8dc")
		draw_string(font, center + Vector2(-9, -6), glyph, HORIZONTAL_ALIGNMENT_CENTER, 18, 18, color)
	elif state == "unknown":
		draw_string(font, center + Vector2(-5, 6), "?", HORIZONTAL_ALIGNMENT_CENTER, 10, 16, Color("c9d1d9"))
	if data.get("group") != null:
		var group: Dictionary = data.group
		var color := Color("ff6b6b") if group.get("in_sight", false) else Color("c97a7a")
		draw_circle(center + Vector2(0, 10), 7.0, color)
		draw_string(font, center + Vector2(-5, 14), str(group.get("count", 1)), HORIZONTAL_ALIGNMENT_CENTER, 10, 11, Color("1a0a0a"))
	if warlord:
		draw_circle(center + Vector2(0, 10), 8.0, Color("d94c8a"))
		draw_string(font, center + Vector2(-5, 15), "W", HORIZONTAL_ALIGNMENT_CENTER, 10, 12, Color("1a0a10"))
	if data.get("player", false):
		draw_circle(center + Vector2(0, -2), 9.0, Color("ffd166"))
		draw_circle(center + Vector2(0, -2), 9.0, Color("3a2a00"), false, 2.0)
	if state != "unexplored" and data.has("cost") and bool(data.get("passable", true)):
		draw_string(font, center + Vector2(radius * 0.35, radius * 0.75), str(int(data.cost)), HORIZONTAL_ALIGNMENT_RIGHT, 16, 10, Color("e9e1d1"))
