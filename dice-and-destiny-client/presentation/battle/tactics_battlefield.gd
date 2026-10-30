extends Control
## The Tactics Sandbox grid inside the battle screen (plan §9.2). It takes the
## battlefield's place behind the usual HUD: terrain, creature tokens, area
## squares, paths, ghosts and missed-attack labels, drawn top-down in 2D. Each
## token carries its actor id, so the battle screen treats it as that
## creature's fighter (attack intents, offensive targets, the aim line).
## Drag the grid to move it, scroll to zoom. It only renders what it is given
## and reports clicks; it never computes rules.

signal square_clicked(square: Vector2i)
signal creature_clicked(id: String)

const TOOLTIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
const TEAM_COLORS := {"heroes": Color("4fa3ff"), "monsters": Color("e0533f")}
## The open battlefield between the left controls, the enemy column and the
## hand, in the battle screen's 1920 × 1080 design space. The grid starts
## fitted inside it; dragging may move it anywhere.
const FREE_AREA := Rect2(452, 96, 1040, 600)
const DRAG_THRESHOLD := 6.0

## Pan offset and zoom, owned by the screen so they survive each rebuild.
var view_state: Object
## {terrain: [[id]], colors: {id: Color}, names: {id: String}, costs: {id: String}}
var arena: Dictionary = {}
## id -> {square: Vector2, team, label, name, tooltip, defeated, running, hp, max_hp, viewer}
var creatures: Dictionary = {}
## Squares to highlight as legal clicks: Vector2i -> Color
var highlights: Dictionary = {}
## [{squares: [Vector2i], color: Color}]
var areas: Array = []
## [{points: [Vector2], color: Color, width: float, dashed: bool}]
var paths: Array = []
## id -> Vector2 (a translucent ghost at a final square)
var ghosts: Dictionary = {}
## Labels on the grid: area attacks at their square, attacks that missed at
## their target. [{id, target or square, text, tooltip, color, below}]
var badges: Array = []
## Floating texts during playback: [{position: Vector2 (squares), text, color, born_ms}]
var floaters: Array = []
## Creatures fading as they fall: id -> 0..1
var falling: Dictionary = {}
var clickable_creatures: Dictionary = {}

var tokens: Dictionary = {}
var _square_marks: Dictionary = {}
var _badge_buttons: Array = []
var _cell := 48.0
var _origin := Vector2.ZERO
var _press := Vector2.ZERO
var _pressed := false
var _dragging := false

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP
	resized.connect(layout)

func cell_size() -> float: return _cell

## Top-left of a square in board coordinates.
func square_position(square: Vector2) -> Vector2:
	return _origin + square * _cell

func square_center(square: Vector2) -> Vector2:
	return square_position(square) + Vector2(_cell, _cell) * 0.5

func grid_size() -> Vector2i:
	var rows: Array = arena.get("terrain", [])
	if rows.is_empty(): return Vector2i.ZERO
	return Vector2i(rows[0].size(), rows.size())

func grid_rect() -> Rect2:
	return Rect2(_origin, Vector2(grid_size()) * _cell)

func square_at(point: Vector2) -> Vector2i:
	var grid := grid_size()
	var local := (point - _origin) / _cell
	var square := Vector2i(floori(local.x), floori(local.y))
	if square.x < 0 or square.y < 0 or square.x >= grid.x or square.y >= grid.y: return Vector2i(-1, -1)
	return square

## Only the grid itself takes the pointer; the rest of the battlefield stays
## open for the HUD exactly as on the ordinary battle screen.
func _has_point(point: Vector2) -> bool:
	return grid_rect().grow(4).has_point(point)

func layout() -> void:
	var grid := grid_size()
	if grid == Vector2i.ZERO: return
	var zoom: float = view_state.zoom if view_state != null else 1.0
	var pan: Vector2 = view_state.pan if view_state != null else Vector2.ZERO
	_cell = floorf(minf(FREE_AREA.size.x / grid.x, FREE_AREA.size.y / grid.y) * zoom)
	_origin = (FREE_AREA.get_center() - Vector2(grid) * _cell * 0.5 + pan).floor()
	for key in _square_marks:
		var mark: Control = _square_marks[key]
		mark.position = square_position(Vector2(key)); mark.size = Vector2(_cell, _cell)
	place_tokens()
	_place_badges()
	queue_redraw()

func set_arena(value: Dictionary, inspect: Callable) -> void:
	arena = value
	for key in _square_marks: _square_marks[key].queue_free()
	_square_marks.clear()
	var grid := grid_size()
	for y in grid.y:
		for x in grid.x:
			# Inspection marks only: clicks reach the grid, which tells a click
			# from a drag. The pointer test clicks their centres.
			var mark := Control.new(); mark.mouse_filter = Control.MOUSE_FILTER_IGNORE
			add_child(mark); _square_marks[Vector2i(x, y)] = mark
			inspect.call(mark, "battle.tactics.square.%d.%d" % [x, y], _square_text(Vector2i(x, y)))
	layout()

func _square_text(square: Vector2i) -> String:
	var terrain := str(arena.terrain[square.y][square.x])
	return "(%d, %d) %s · %s" % [square.x, square.y, arena.names.get(terrain, terrain), arena.costs.get(terrain, "")]

func _get_tooltip(at_position: Vector2) -> String:
	var square := square_at(at_position)
	return "" if square.x < 0 else _square_text(square)

func refresh(inspect: Callable) -> void:
	for id in tokens.keys():
		if not creatures.has(id): tokens[id].queue_free(); tokens.erase(id)
	for id in creatures:
		if not tokens.has(id):
			var token := Token.new(); token.board = self; token.creature_id = str(id); token.focus_mode = Control.FOCUS_NONE
			token.name = "Fighter_" + str(id)
			token.set_meta("actor_id", str(id)); token.set_meta("ground_anchor", Vector2(0.5, 1.0))
			token.pressed.connect(func(): creature_clicked.emit(str(id)))
			add_child(token); tokens[id] = token
			inspect.call(token, "battle.tactics.creature.%s" % id, str(creatures[id].get("name", id)))
		var token: Token = tokens[id]
		token.tooltip_text = str(creatures[id].get("tooltip", ""))
		token.queue_redraw()
	_sync_badges(inspect)
	layout()

func place_tokens() -> void:
	for id in tokens:
		var token: Control = tokens[id]
		var data: Dictionary = creatures.get(id, {})
		token.size = Vector2(_cell, _cell)
		token.position = square_position(data.get("square", Vector2.ZERO))
		token.visible = not data.get("defeated", false) or falling.has(id)
		token.queue_redraw()

func _gui_input(event: InputEvent) -> void:
	if view_state == null: return
	if event is InputEventMouseButton:
		if event.button_index in [MOUSE_BUTTON_WHEEL_UP, MOUSE_BUTTON_WHEEL_DOWN] and event.pressed:
			# Zoom about the pointer: the square under it stays under it.
			var before: float = view_state.zoom
			view_state.zoom = clampf(before * (1.12 if event.button_index == MOUSE_BUTTON_WHEEL_UP else 1.0 / 1.12), 0.6, 2.4)
			var anchor: Vector2 = (event.position - _origin) / _cell
			layout()
			view_state.pan += event.position - (_origin + anchor * _cell)
			layout()
			accept_event()
		elif event.button_index in [MOUSE_BUTTON_LEFT, MOUSE_BUTTON_RIGHT, MOUSE_BUTTON_MIDDLE]:
			if event.pressed:
				_pressed = true; _dragging = false; _press = event.position
			else:
				if _pressed and not _dragging and event.button_index == MOUSE_BUTTON_LEFT:
					var square := square_at(event.position)
					if square.x >= 0: square_clicked.emit(square)
				_pressed = false; _dragging = false
			accept_event()
	elif event is InputEventMouseMotion and _pressed:
		if not _dragging and event.position.distance_to(_press) > DRAG_THRESHOLD: _dragging = true
		if _dragging:
			view_state.pan += event.relative
			layout()
		accept_event()

func _sync_badges(inspect: Callable) -> void:
	for badge in _badge_buttons: badge.queue_free()
	_badge_buttons.clear()
	for data in badges:
		var badge := TOOLTIP_BUTTON.new(); badge.text = str(data.text); badge.tooltip_text = str(data.get("tooltip", ""))
		badge.focus_mode = Control.FOCUS_NONE; badge.add_theme_font_size_override("font_size", 15)
		var style := StyleBoxFlat.new(); style.bg_color = data.get("color", Color("7a1f17")); style.set_corner_radius_all(6); style.content_margin_left = 6; style.content_margin_right = 6
		for state in ["normal", "hover", "pressed", "disabled"]: badge.add_theme_stylebox_override(state, style)
		badge.add_theme_color_override("font_color", Color("fff6e0"))
		badge.set_meta("anchor", data)
		add_child(badge); _badge_buttons.append(badge)
		if data.has("id"): inspect.call(badge, "battle.tactics.badge.%s" % data.id, badge.text)

func _place_badges() -> void:
	var placed: Array[Rect2] = []
	for badge in _badge_buttons:
		var data: Dictionary = badge.get_meta("anchor")
		var target := str(data.get("target", ""))
		var square: Vector2 = creatures[target].get("square", Vector2.ZERO) if not target.is_empty() and creatures.has(target) else Vector2(data.get("square", Vector2i.ZERO))
		badge.size = badge.get_combined_minimum_size()
		var below := bool(data.get("below", false))
		var at := square_position(square) + Vector2(_cell * 0.5 - badge.size.x * 0.5, _cell * 0.95 if below else -badge.size.y * 0.6)
		# Labels never cover each other: each steps away from those placed.
		var step: float = (badge.size.y + 2) * (1.0 if below else -1.0)
		for attempt in 12:
			var rect := Rect2(at, badge.size)
			if not placed.any(func(other: Rect2): return other.intersects(rect)): break
			at += Vector2(0, step)
		placed.append(Rect2(at, badge.size))
		badge.position = at

func _draw() -> void:
	var grid := grid_size()
	if grid == Vector2i.ZERO: return
	draw_rect(grid_rect().grow(5), Color(0.05, 0.04, 0.03, 0.82))
	for y in grid.y:
		for x in grid.x:
			var terrain := str(arena.terrain[y][x])
			var rect := Rect2(square_position(Vector2(x, y)), Vector2(_cell, _cell))
			var color: Color = arena.colors.get(terrain, Color("333a44")); color.a = 0.88
			draw_rect(rect.grow(-1), color)
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
		var color: Color = TEAM_COLORS.get(team, Color.GRAY); color.a = 0.55
		draw_arc(square_center(ghosts[id]), _cell * 0.36, 0, TAU, 32, color, 3)
		draw_string(get_theme_default_font(), square_center(ghosts[id]) + Vector2(-_cell * 0.36, _cell * 0.08), str(creatures.get(id, {}).get("label", "?")), HORIZONTAL_ALIGNMENT_CENTER, _cell * 0.72, int(_cell * 0.24), Color(1, 1, 1, 0.6))
	var now := Time.get_ticks_msec()
	for floater in floaters:
		var age := (now - int(floater.born_ms)) / 1000.0
		if age < 0 or age > 1.6: continue
		var width := _cell * 4.0
		var at := square_center(floater.position) + Vector2(-width * 0.5, -_cell * 0.55 - age * _cell * 0.4)
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
		if data.get("focused", false): draw_arc(center, cell * 0.44, 0, TAU, 32, Color("a68b51"), 2.5)
		if bool(board.clickable_creatures.get(creature_id, false)): draw_arc(center, cell * 0.47, 0, TAU, 32, Color(1, 1, 0.5, 0.9), 3)
		var font := get_theme_default_font()
		draw_string(font, center + Vector2(-cell * 0.34, cell * 0.11), str(data.get("label", "?")), HORIZONTAL_ALIGNMENT_CENTER, cell * 0.68, int(cell * 0.3), Color(1, 1, 1, fade))
		var max_hp := maxi(1, int(data.get("max_hp", 1)))
		var hp := clampi(int(data.get("hp", 0)), 0, max_hp)
		var bar := Rect2(Vector2(cell * 0.14, cell * 0.84), Vector2(cell * 0.72, cell * 0.08))
		draw_rect(bar, Color(0, 0, 0, 0.6 * fade))
		draw_rect(Rect2(bar.position, Vector2(bar.size.x * hp / max_hp, bar.size.y)), Color(0.85, 0.27, 0.22, fade))
		if data.get("running", false):
			draw_string(font, Vector2(cell * 0.62, cell * 0.3), "»", HORIZONTAL_ALIGNMENT_LEFT, -1, int(cell * 0.32), Color(1, 0.85, 0.3, fade))
