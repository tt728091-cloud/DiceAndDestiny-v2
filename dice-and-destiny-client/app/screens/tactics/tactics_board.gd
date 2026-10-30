extends Control
## The Tactics Sandbox grid (plan §9.2): terrain, creatures, area squares,
## paths, ghosts and attack badges, drawn top-down in 2D. It only renders what
## the screen gives it and reports clicks; it never computes rules.

signal square_clicked(square: Vector2i)
signal creature_clicked(id: String)

const TOOLTIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
const TEAM_COLORS := {"heroes": Color("4fa3ff"), "monsters": Color("e0533f")}

## {terrain: [[id]], colors: {id: Color}, names: {id: String}, costs: {id: String}}
var arena: Dictionary = {}
## id -> {square: Vector2, team, label, name, tooltip, defeated, running, hp, max_hp}
var creatures: Dictionary = {}
## Squares to highlight as legal clicks: Vector2i -> Color
var highlights: Dictionary = {}
## [{squares: [Vector2i], color: Color, label: String}]
var areas: Array = []
## [{points: [Vector2], color: Color, width: float, dashed: bool}]
var paths: Array = []
## id -> Vector2 (a translucent ghost at a final square)
var ghosts: Dictionary = {}
## [{target: String or "", square: Vector2i, text: String, tooltip: String, color: Color}]
var badges: Array = []
## Floating texts during playback: [{position: Vector2 (squares), text, color, born_ms}]
var floaters: Array = []
## Creatures fading as they fall: id -> 0..1
var falling: Dictionary = {}
var clickable_creatures: Dictionary = {}

var _square_buttons: Dictionary = {}
var _token_buttons: Dictionary = {}
var _badge_buttons: Array = []
var _cell := 48.0
var _origin := Vector2.ZERO

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_PASS
	resized.connect(_layout)

func cell_size() -> float: return _cell

## Top-left of a square in board coordinates.
func square_position(square: Vector2) -> Vector2:
	return _origin + square * _cell

func square_center(square: Vector2) -> Vector2:
	return square_position(square) + Vector2(_cell, _cell) * 0.5

func _grid_size() -> Vector2i:
	var rows: Array = arena.get("terrain", [])
	if rows.is_empty(): return Vector2i.ZERO
	return Vector2i(rows[0].size(), rows.size())

func _layout() -> void:
	var grid := _grid_size()
	if grid == Vector2i.ZERO: return
	_cell = floorf(minf(size.x / grid.x, size.y / grid.y))
	_origin = ((size - Vector2(grid) * _cell) * 0.5).floor()
	for key in _square_buttons:
		var button: Control = _square_buttons[key]
		button.position = square_position(Vector2(key)); button.size = Vector2(_cell, _cell)
	_place_tokens()
	_place_badges()
	queue_redraw()

## Rebuilds the clickable squares (one per square) when the arena changes.
func set_arena(value: Dictionary) -> void:
	arena = value
	for key in _square_buttons: _square_buttons[key].queue_free()
	_square_buttons.clear()
	var grid := _grid_size()
	var inspector = get_node_or_null("/root/GameInspector")
	for y in grid.y:
		for x in grid.x:
			var button := TOOLTIP_BUTTON.new(); button.flat = true; button.focus_mode = Control.FOCUS_NONE
			for style in ["normal", "hover", "pressed", "disabled", "focus"]: button.add_theme_stylebox_override(style, StyleBoxEmpty.new())
			var terrain := str(arena.terrain[y][x])
			button.tooltip_text = "(%d, %d) %s · %s" % [x, y, arena.names.get(terrain, terrain), arena.costs.get(terrain, "")]
			var square := Vector2i(x, y)
			button.pressed.connect(func(): square_clicked.emit(square))
			add_child(button); _square_buttons[square] = button
			if inspector != null: inspector.register_control("tactics.board.square.%d.%d" % [x, y], button, button.tooltip_text)
	_layout()

func refresh() -> void:
	_sync_tokens()
	_sync_badges()
	_layout()

func _sync_tokens() -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	for id in _token_buttons.keys():
		if not creatures.has(id): _token_buttons[id].queue_free(); _token_buttons.erase(id)
	for id in creatures:
		if not _token_buttons.has(id):
			var token := Token.new(); token.board = self; token.creature_id = str(id); token.focus_mode = Control.FOCUS_NONE
			token.pressed.connect(func(): creature_clicked.emit(str(id)))
			add_child(token); _token_buttons[id] = token
			if inspector != null: inspector.register_control("tactics.board.creature.%s" % id, token, str(creatures[id].get("name", id)))
		var token: Token = _token_buttons[id]
		token.tooltip_text = str(creatures[id].get("tooltip", ""))
		token.queue_redraw()

func _place_tokens() -> void:
	for id in _token_buttons:
		var token: Control = _token_buttons[id]
		var data: Dictionary = creatures.get(id, {})
		token.size = Vector2(_cell, _cell)
		token.position = square_position(data.get("square", Vector2.ZERO))
		token.visible = not data.get("defeated", false) or falling.has(id)
		token.move_to_front()

func _sync_badges() -> void:
	for badge in _badge_buttons: badge.queue_free()
	_badge_buttons.clear()
	var inspector = get_node_or_null("/root/GameInspector")
	var per_anchor := {}
	for data in badges:
		var badge := TOOLTIP_BUTTON.new(); badge.text = str(data.text); badge.tooltip_text = str(data.get("tooltip", ""))
		badge.focus_mode = Control.FOCUS_NONE; badge.add_theme_font_size_override("font_size", 13)
		var style := StyleBoxFlat.new(); style.bg_color = data.get("color", Color("7a1f17")); style.set_corner_radius_all(6); style.content_margin_left = 5; style.content_margin_right = 5
		for name in ["normal", "hover", "pressed", "disabled"]: badge.add_theme_stylebox_override(name, style)
		badge.add_theme_color_override("font_color", Color("fff6e0"))
		# Badges stack per drawn square and side, whatever they anchor to.
		var target := str(data.get("target", ""))
		var square: Vector2 = creatures[target].get("square", Vector2.ZERO) if creatures.has(target) else Vector2(data.get("square", Vector2i.ZERO))
		var anchor := "%s|%s" % [Vector2i(square.round()), bool(data.get("below", false))]
		badge.set_meta("anchor", data); badge.set_meta("stack", int(per_anchor.get(anchor, 0)))
		per_anchor[anchor] = int(per_anchor.get(anchor, 0)) + 1
		add_child(badge); _badge_buttons.append(badge)
		if inspector != null and data.has("id"): inspector.register_control("tactics.board.badge.%s" % data.id, badge, badge.text)

func _place_badges() -> void:
	for badge in _badge_buttons:
		var data: Dictionary = badge.get_meta("anchor")
		var square: Vector2
		var target := str(data.get("target", ""))
		if not target.is_empty() and creatures.has(target): square = creatures[target].get("square", Vector2.ZERO)
		else: square = Vector2(data.get("square", Vector2i.ZERO))
		badge.size = badge.get_combined_minimum_size()
		var stack := int(badge.get_meta("stack"))
		# Attacks on the viewer hang below its token, the rest above theirs,
		# so neighbours' badges do not cover each other.
		var below := bool(data.get("below", false))
		var at := square_position(square) + Vector2(_cell * 0.5 - badge.size.x * 0.5, -badge.size.y * 0.6 - stack * (badge.size.y + 2))
		if below: at = square_position(square) + Vector2(_cell * 0.5 - badge.size.x * 0.5, _cell * 0.95 + stack * (badge.size.y + 2))
		badge.position = at.clamp(Vector2.ZERO, size - badge.size)
		badge.move_to_front()

func _draw() -> void:
	var grid := _grid_size()
	if grid == Vector2i.ZERO: return
	draw_rect(Rect2(_origin - Vector2(4, 4), Vector2(grid) * _cell + Vector2(8, 8)), Color("10151d"))
	for y in grid.y:
		for x in grid.x:
			var terrain := str(arena.terrain[y][x])
			var rect := Rect2(square_position(Vector2(x, y)), Vector2(_cell, _cell))
			draw_rect(rect.grow(-1), arena.colors.get(terrain, Color("333a44")))
			if terrain == "swamp":
				for i in 3: draw_line(rect.position + Vector2(_cell * 0.2, _cell * (0.3 + 0.2 * i)), rect.position + Vector2(_cell * 0.8, _cell * (0.3 + 0.2 * i)), Color("5d8f7a66"), 2)
			if terrain == "wall": draw_rect(rect.grow(-4), Color("252a33"), false, 2)
	for area in areas:
		for square in area.squares:
			draw_rect(Rect2(square_position(Vector2(square)), Vector2(_cell, _cell)).grow(-2), area.color)
	for square in highlights:
		draw_rect(Rect2(square_position(Vector2(square)), Vector2(_cell, _cell)).grow(-3), highlights[square], false, 3)
	for path in paths:
		var points: Array = path.points
		for i in range(1, points.size()):
			var a := square_center(points[i - 1]); var b := square_center(points[i])
			if path.get("dashed", false):
				var steps := int(a.distance_to(b) / 8.0)
				for s in steps:
					if s % 2 == 0: draw_line(a.lerp(b, float(s) / steps), a.lerp(b, float(s + 1) / steps), path.color, path.get("width", 3.0))
			else: draw_line(a, b, path.color, path.get("width", 3.0))
		if points.size() > 1: draw_circle(square_center(points[-1]), _cell * 0.1, path.color)
	for id in ghosts:
		var team := str(creatures.get(id, {}).get("team", ""))
		var color: Color = TEAM_COLORS.get(team, Color.GRAY); color.a = 0.35
		draw_arc(square_center(ghosts[id]), _cell * 0.36, 0, TAU, 32, color, 3)
		draw_string(get_theme_default_font(), square_center(ghosts[id]) + Vector2(-_cell * 0.36, _cell * 0.08), str(creatures.get(id, {}).get("label", "?")), HORIZONTAL_ALIGNMENT_CENTER, _cell * 0.72, int(_cell * 0.24), Color(1, 1, 1, 0.45))
	var now := Time.get_ticks_msec()
	for floater in floaters:
		var age := (now - int(floater.born_ms)) / 1000.0
		if age < 0 or age > 1.6: continue
		var width := _cell * 4.0
		var at := square_center(floater.position) + Vector2(-width * 0.5, -_cell * 0.55 - age * _cell * 0.4)
		at.x = clampf(at.x, 0.0, size.x - width)
		var color: Color = floater.color; color.a = clampf(1.6 - age, 0, 1)
		var font := get_theme_default_font()
		draw_string_outline(font, at, str(floater.text), HORIZONTAL_ALIGNMENT_CENTER, width, int(_cell * 0.26), 5, Color(0, 0, 0, color.a))
		draw_string(font, at, str(floater.text), HORIZONTAL_ALIGNMENT_CENTER, width, int(_cell * 0.26), color)

## A creature's round token: team colour, a short label, health and Running.
class Token extends "res://presentation/battle/tooltip_button.gd":
	var board
	var creature_id := ""
	func _ready() -> void:
		flat = true
		for style in ["normal", "hover", "pressed", "disabled", "focus"]: add_theme_stylebox_override(style, StyleBoxEmpty.new())
	func _draw() -> void:
		if board == null: return
		var data: Dictionary = board.creatures.get(creature_id, {})
		var cell: float = size.x
		var center := size * 0.5
		var color: Color = board.TEAM_COLORS.get(str(data.get("team", "")), Color.GRAY)
		var fade := 1.0 - float(board.falling.get(creature_id, 0.0))
		# Defeated creatures leave the grid once their fall has played.
		if data.get("defeated", false) and not board.falling.has(creature_id): fade = 0.0
		color.a = fade
		draw_circle(center, cell * 0.38, Color(0, 0, 0, 0.45 * fade))
		draw_circle(center, cell * 0.34, color)
		if data.get("viewer", false): draw_arc(center, cell * 0.4, 0, TAU, 32, Color(1, 0.86, 0.4, fade), 2.5)
		if bool(board.clickable_creatures.get(creature_id, false)): draw_arc(center, cell * 0.46, 0, TAU, 32, Color(1, 1, 0.5, 0.9), 3)
		var font := get_theme_default_font()
		draw_string(font, center + Vector2(-cell * 0.34, cell * 0.11), str(data.get("label", "?")), HORIZONTAL_ALIGNMENT_CENTER, cell * 0.68, int(cell * 0.3), Color(1, 1, 1, fade))
		var max_hp := maxi(1, int(data.get("max_hp", 1)))
		var hp := clampi(int(data.get("hp", 0)), 0, max_hp)
		var bar := Rect2(Vector2(cell * 0.14, cell * 0.84), Vector2(cell * 0.72, cell * 0.08))
		draw_rect(bar, Color(0, 0, 0, 0.6 * fade))
		draw_rect(Rect2(bar.position, Vector2(bar.size.x * hp / max_hp, bar.size.y)), Color(0.4, 0.9, 0.4, fade))
		if data.get("running", false):
			draw_string(font, Vector2(cell * 0.62, cell * 0.3), "»", HORIZONTAL_ALIGNMENT_LEFT, -1, int(cell * 0.32), Color(1, 0.85, 0.3, fade))
