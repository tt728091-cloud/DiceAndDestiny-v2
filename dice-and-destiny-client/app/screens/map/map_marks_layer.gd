extends Control
## Everything on the chart that is information rather than geography, drawn
## above the night-graded paper so gold and red keep their saturation: the
## legend veil, the sight ring, highlights, the Warlord's route, the hover
## path, site chips, "?" chips, cost chits, the pieces, survey labels, the
## hover cartouche and the calendar hover. All of it comes from the screen's
## map_model, which the screen builds from the authority's viewer-safe view.

const STYLE := preload("res://app/screens/map/map_style.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const SITE_TYPES := ["village", "monastery", "town", "ruins", "grotto", "event", "keep", "tower"]

## The widest a hover box's text runs before it wraps.
const MAX_HOVER_WIDTH := 380.0

var screen: Node
var slide := {"from": Vector2i.ZERO, "t": 1.0, "active": false}
## Where the hover box was last drawn (empty while none shows), for layout checks.
var hover_box := Rect2()
var _tween: Tween

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_process(false)

func _process(_delta: float) -> void:
	queue_redraw()
	if screen == null: set_process(false); return
	var model: Dictionary = screen.map_model
	if not slide.active and str(model.get("selected", "")).is_empty() and str(model.get("hover", "")).is_empty() and not model.get("cartouche_hover", false): set_process(false)

func wake() -> void:
	set_process(true)

func start_slide(from: Vector2i) -> void:
	slide.from = from; slide.t = 0.0; slide.active = true
	if _tween != null: _tween.kill()
	_tween = create_tween().set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	_tween.tween_method(func(t: float): slide.t = t; queue_redraw(), 0.0, 1.0, 0.18)
	_tween.finished.connect(func(): slide.active = false; queue_redraw())
	set_process(true)

func _center(cell: Vector2i) -> Vector2:
	return screen._hex_center(cell.x, cell.y)

func _draw() -> void:
	if screen == null or screen.map_model.is_empty(): return
	var model: Dictionary = screen.map_model
	var r: float = screen._hex_radius
	var hexes: Dictionary = model.hexes
	var overlay_open: bool = model.get("overlay_open", false)
	var night: bool = model.get("night", false)
	var center_of := func(q: int, rr: int) -> Vector2: return screen._hex_center(q, rr)
	# A. Legend veil.
	var filter := str(model.get("legend_filter", ""))
	if not filter.is_empty():
		for key in hexes:
			var entry: Dictionary = hexes[key]
			if not _matches(entry, filter): draw_colored_polygon(STYLE.corners(_center(entry.cell), r - 0.6), Color(STYLE.PAPER, 0.55))
	# B. Sight ring.
	if not overlay_open and not model.sight.is_empty():
		for edge in STYLE.outer_edges(model.sight, center_of, r):
			draw_dashed_line(edge[0] + Vector2(0, 1), edge[1] + Vector2(0, 1), Color(STYLE.PAPER_LIGHT, 0.5), 1.0, 5.0, true, true)
			draw_dashed_line(edge[0], edge[1], Color(STYLE.INK, 0.5), 1.5, 5.0, true, true)
	# C. Highlights.
	var in_turn: bool = model.get("phase", "") == "turn" and not overlay_open
	if in_turn:
		for key in hexes:
			var entry: Dictionary = hexes[key]
			var c := _center(entry.cell)
			match str(entry.highlight):
				"move": STYLE.draw_highlight(self, c, r, STYLE.GOLD_LEAF)
				"target": STYLE.draw_highlight(self, c, r, STYLE.TARGET)
				"provoke":
					STYLE.draw_highlight(self, c, r, STYLE.ENEMY_RIM)
					STYLE.draw_dashed_ring(self, STYLE.corners(c, r + 1.0), STYLE.ENEMY_RIM, 1.5, 4.0)
					var dot := c + Vector2(-0.50 * r, -0.42 * r)
					draw_circle(dot, 0.18 * r, STYLE.RUBRIC)
					draw_arc(dot, 0.18 * r, 0.0, TAU, 24, STYLE.WAX_RING, 1.0, true)
					STYLE.draw_centered(self, STYLE.serif_bold(), dot, "!", maxi(13, int(0.36 * r)), STYLE.IVORY)
				"blocked": STYLE.draw_dashed_ring(self, STYLE.corners(c, r - 2.0), Color(STYLE.SEPIA, 0.8), 1.0, 4.0)
		for id in model.slots:
			var slot: Dictionary = model.slots[id]
			if slot.placed or not slot.adjacent: continue
			var ready: bool = slot.ready
			var color: Color = STYLE.GOLD_BRIGHT if ready else Color(STYLE.SEPIA, 0.9)
			for edge in STYLE.outer_edges(slot.hexes, center_of, r):
				if ready:
					draw_line(edge[0], edge[1], Color(STYLE.INK, 0.8), 1.0, true)
					draw_line(edge[0], edge[1], Color(color, 0.12), 8.0, true)
					draw_line(edge[0], edge[1], Color(color, 0.28), 5.0, true)
					draw_line(edge[0], edge[1], color, 2.5, true)
				else:
					draw_dashed_line(edge[0], edge[1], color, 1.5, 4.0, true, true)
			var pill_center := _center(slot.center)
			var pill := STYLE.draw_chip(self, pill_center, 0.95 * r, 0.46 * r, not ready)
			STYLE.draw_glyph(self, STYLE.glyph("compass_diamond", "ink" if ready else "sepia"), Vector2(pill.position.x + 0.28 * r, pill_center.y), 0.30 * r)
			STYLE.draw_centered(self, STYLE.numerals(), Vector2(pill.end.x - 0.30 * r, pill_center.y), str(slot.explore_cost), maxi(14, int(0.38 * r)), STYLE.INK if ready else STYLE.SEPIA)
	# Selected and hover rings.
	var selected := str(model.get("selected", ""))
	var hover := str(model.get("hover", ""))
	if not hover.is_empty() and hover != selected and hexes.has(hover) and not overlay_open:
		STYLE.draw_ring(self, STYLE.corners(_center(hexes[hover].cell), r - 2.5), Color(STYLE.SELECTED, 0.5), 1.5)
	if not selected.is_empty() and hexes.has(selected):
		var c := _center(hexes[selected].cell)
		var pulse := 0.7 + 0.3 * sin(TAU * Time.get_ticks_msec() / 1200.0)
		STYLE.draw_ring(self, STYLE.corners(c, r - 1.5), STYLE.INK, 1.0)
		STYLE.draw_ring(self, STYLE.corners(c, r - 2.5), Color(STYLE.SELECTED, pulse), 3.0)
	# D. The Warlord's route.
	var warlord: Dictionary = model.get("warlord", {})
	if warlord.get("known", false) and warlord.get("next_step") != null:
		var from_cell: Vector2i = model.get("warlord_cell", Vector2i(999, 999))
		if from_cell != Vector2i(999, 999):
			var points: Array = [_center(from_cell), _center(Vector2i(int(warlord.next_step.q), int(warlord.next_step.r)))]
			if warlord.get("goal") != null and hexes.has("%d,%d" % [int(warlord.goal.q), int(warlord.goal.r)]):
				points.append(_center(Vector2i(int(warlord.goal.q), int(warlord.goal.r))))
			for i in points.size() - 1:
				draw_dashed_line(points[i] + Vector2(0, 1), points[i + 1] + Vector2(0, 1), Color(STYLE.PAPER_LIGHT, 0.6), 1.0, 4.0, true, true)
				draw_dashed_line(points[i], points[i + 1], STYLE.RUBRIC, 1.5, 4.0, true, true)
	if warlord.get("goal") != null and hexes.has("%d,%d" % [int(warlord.goal.q), int(warlord.goal.r)]):
		var goal_chip := _center(Vector2i(int(warlord.goal.q), int(warlord.goal.r))) + Vector2(0, -0.50 * r)
		if warlord.get("goal_taken", false): draw_circle(goal_chip, 0.42 * r, Color(STYLE.RUBRIC, 0.35))
		draw_arc(goal_chip, 0.42 * r, 0.0, TAU, 32, STYLE.RUBRIC, 1.5, true)
	# E. Hover path.
	if in_turn and not hover.is_empty() and model.moves.has(hover) and str(model.moves[hover].get("blocked", "")).is_empty():
		draw_dashed_line(_center(model.player), _center(hexes[hover].cell), Color(STYLE.INK, 0.7), 1.5, 2.5, true, true)
	# F–G. Site chips and "?" chips.
	for key in hexes:
		var entry: Dictionary = hexes[key]
		var hex: Dictionary = entry.data
		var c := _center(entry.cell)
		var chip_center := c + Vector2(0, -0.50 * r)
		if hex.get("site") != null:
			var site: Dictionary = hex.site
			var ghost: bool = entry.state == "remembered"
			var rect := STYLE.draw_chip(self, chip_center, 0.66 * r, 0.40 * r, ghost)
			var type := str(site.get("type", ""))
			if type == "event":
				STYLE.draw_centered(self, STYLE.serif_bold(), chip_center, "?", maxi(13, int(0.44 * r)), STYLE.SEPIA if ghost else STYLE.INK)
			else:
				var glyph_colour := "sepia" if ghost else ("red" if site.get("conquered", false) else "ink")
				STYLE.draw_glyph(self, STYLE.glyph("site_" + type, glyph_colour), chip_center, 0.46 * r)
			if site.get("fortified", false): draw_rect(rect.grow(2.0), STYLE.RUBRIC, false, 1.5)
			if site.get("goal", false): draw_rect(rect.grow(3.0), STYLE.GOLD_LEAF, false, 1.5)
			if site.get("defended", false):
				var wax := rect.end - Vector2(1, 1)
				draw_circle(wax, 0.09 * r, STYLE.RUBRIC); draw_arc(wax, 0.09 * r, 0.0, TAU, 16, STYLE.WAX_RING, 1.0, true)
			if site.get("conquered", false):
				var seal := rect.end - Vector2(2, 2)
				draw_circle(seal, 0.13 * r, STYLE.RUBRIC); draw_arc(seal, 0.13 * r, 0.0, TAU, 16, STYLE.WAX_RING, 1.0, true)
			if site.get("freed", false) or site.get("cleared", false) or site.get("resolved", false):
				var mast := Vector2(rect.end.x - 2, rect.position.y)
				draw_line(mast, mast + Vector2(0, -0.26 * r), STYLE.INK, 2.0, true)
				draw_colored_polygon(PackedVector2Array([mast + Vector2(0, -0.26 * r), mast + Vector2(0.18 * r, -0.20 * r), mast + Vector2(0, -0.13 * r)]), STYLE.INK)
			var inn: bool = night and ((type in ["village", "town"] and not site.get("conquered", false) and not site.get("defended", false)) or (type == "monastery" and site.get("open", false)))
			if inn:
				var dot := Vector2(chip_center.x, rect.end.y + 0.08 * r)
				draw_circle(dot, 0.30 * r, Color(STYLE.INN_GLOW, 0.2)); draw_circle(dot, 0.14 * r, Color(STYLE.INN_GLOW, 0.6))
		elif entry.state == "unknown":
			STYLE.draw_chip(self, chip_center, 0.66 * r, 0.40 * r, false, true)
			STYLE.draw_centered(self, STYLE.serif_bold(), chip_center, "?", maxi(13, int(0.44 * r)), STYLE.BONE, 2, STYLE.DARK_KEYLINE)
	# H. Cost chits.
	var survey: bool = model.get("survey", false)
	var terrains: Dictionary = screen.view.get("config", {}).get("terrain", {})
	for key in hexes:
		var entry: Dictionary = hexes[key]
		var hex: Dictionary = entry.data
		var c := _center(entry.cell)
		var pair := ""
		if survey:
			var config: Dictionary = terrains.get(str(hex.get("terrain", "")), {})
			pair = "%d/%d" % [int(config.get("day", 0)), int(config.get("night", 0))]
		var chit_center := c + Vector2(0.56 * r, -0.44 * r) if pair.is_empty() else c + Vector2(0.50 * r, -0.44 * r)
		STYLE.draw_chit(self, chit_center, maxf(10.0, 0.24 * r), int(hex.get("cost", 0)), bool(hex.get("passable", true)), bool(hex.get("cover", false)), night, entry.state != "in_sight", pair)
	# I. Pieces.
	var player: Vector2i = model.player
	var player_key := "%d,%d" % [player.x, player.y]
	for key in hexes:
		var entry: Dictionary = hexes[key]
		var hex: Dictionary = entry.data
		var c := _center(entry.cell) + Vector2(0, 0.36 * r)
		var is_player: bool = key == player_key
		var group = hex.get("group")
		var has_warlord: bool = hex.get("warlord", false)
		var shared: bool = is_player and (group != null or has_warlord)
		var other := c + Vector2(0.33 * r, 0) if shared else c
		if group != null:
			var live: bool = group.get("in_sight", false) and entry.state == "in_sight"
			STYLE.draw_enemy_piece(self, other, r, int(group.get("count", 1)), live)
			if live:
				var habit := str(group.get("habit", ""))
				var pip := other + Vector2(0.24 * r, -0.24 * r)
				var resting: bool = (habit == "sleeper" and night) or (habit == "night_hunter" and not night)
				draw_circle(pip, 0.12 * r, STYLE.MOON if resting else (Color.WHITE if habit == "always_alert" else STYLE.SUN))
				draw_arc(pip, 0.12 * r, 0.0, TAU, 16, STYLE.DARK_KEYLINE, 1.0, true)
				if habit == "always_alert": draw_circle(pip, 0.04 * r, STYLE.INK)
				if group.get("ambushable", false):
					var eye := other + Vector2(-0.26 * r, -0.26 * r)
					draw_circle(eye, 0.16 * r, STYLE.BONE)
					STYLE.draw_glyph(self, STYLE.glyph("closed_eye"), eye, 0.26 * r)
				if group.get("lieutenant", false): STYLE.draw_glyph(self, ICONS.texture("lieutenant"), other + Vector2(0.30 * r, -0.38 * r), 0.30 * r)
				if group.get("roams", false): STYLE.draw_glyph(self, STYLE.glyph("footprints"), other + Vector2(0, 0.44 * r), 0.22 * r, Color(1, 1, 1, 0.8))
		if has_warlord:
			STYLE.draw_warlord_piece(self, other, r, bool(warlord.get("in_sight", false)), bool(warlord.get("fortified", false)))
	# Hunter's Marks beyond the explored map: the enemy's position, nothing more.
	for m in model.get("marks", []):
		var at = m.get("hex")
		if not m.get("off_map", false) or not at is Dictionary: continue
		var mc := _center(Vector2i(int(at.q), int(at.r))) + Vector2(0, 0.36 * r)
		draw_arc(mc, 0.62 * r, 0.0, TAU, 32, STYLE.GOLD_LEAF, 2.0, true)
		if str(m.get("target", "")) == "warlord": STYLE.draw_warlord_piece(self, mc, r, false, false)
		else: STYLE.draw_enemy_piece(self, mc, r, 1, false)
	# The player, sliding when they just moved.
	if hexes.has(player_key):
		var base := _center(player) + Vector2(0, 0.36 * r)
		var shared_player: bool = hexes[player_key].data.get("group") != null or hexes[player_key].data.get("warlord", false)
		if shared_player: base += Vector2(-0.33 * r, 0)
		if slide.active: base = _center(slide.from).lerp(_center(player), slide.t) + Vector2(0, 0.36 * r)
		STYLE.draw_player_piece(self, base, r)
	# J. Survey labels.
	if survey:
		for id in model.slots:
			var slot: Dictionary = model.slots[id]
			if not slot.placed: continue
			var key := "%d,%d" % [slot.center.x, slot.center.y]
			if not hexes.has(key): continue
			STYLE.draw_centered(self, STYLE.serif_bold(), _center(slot.center) + Vector2(0, 0.74 * r), str(hexes[key].data.get("tile_name", "")).to_upper(), 13, Color(STYLE.INK, 0.85), 2, STYLE.BONE)
	# K. Hover cartouche, only while the pointer is really over the hovered hex.
	hover_box = Rect2()
	var pointer_near := not hover.is_empty() and get_local_mouse_position().distance_to(_center(_cell_of(hover, hexes, model))) <= r * 1.2
	if pointer_near and not overlay_open and model.get("hover_lines", []).size() > 0:
		_hover_box(model.hover_lines, true)
	# L. The calendar, while the pointer is over the chart's cartouche.
	elif model.get("cartouche_hover", false) and not overlay_open and model.get("cartouche_lines", []).size() > 0:
		_hover_box(model.cartouche_lines, false)

## The bone hover box beside the pointer, kept inside the chart: the first
## line in ink, the rest in sepia (danger lines red for hexes), long lines
## wrapped at MAX_HOVER_WIDTH.
func _hover_box(source: Array, danger_lines: bool) -> void:
	var font := STYLE.serif()
	var lines: Array = []
	for i in source.size():
		for part in _wrap(font, str(source[i]), 14 if i == 0 else 12, MAX_HOVER_WIDTH): lines.append([part, i])
	var w := 0.0
	for line in lines: w = maxf(w, font.get_string_size(line[0], HORIZONTAL_ALIGNMENT_LEFT, -1, 14 if line[1] == 0 else 12).x)
	var h := 14.0 + 20.0 * lines.size()
	var at := get_local_mouse_position() + Vector2(18, 14)
	at = at.clamp(Vector2.ZERO, (size - Vector2(w + 12, h)).max(Vector2.ZERO))
	hover_box = Rect2(at, Vector2(w + 12, h))
	draw_style_box(STYLE.chip_style(STYLE.BONE, Color(STYLE.INK, 0.6)), hover_box)
	for k in lines.size():
		var text: String = lines[k][0]
		var first: bool = lines[k][1] == 0
		var danger := danger_lines and ("provokes" in text or "Cannot move" in text or "held by" in text or "Ambush" in text or text.begins_with("The Warlord"))
		var colour: Color = STYLE.INK if first else (STYLE.RUBRIC if danger else STYLE.SEPIA_TEXT)
		draw_string(font, at + Vector2(6, 18 + 20 * k), text, HORIZONTAL_ALIGNMENT_LEFT, -1, 14 if first else 12, colour)

## Greedy word wrap of one line to a pixel width.
static func _wrap(font: Font, text: String, font_size: int, width: float) -> Array:
	if font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x <= width: return [text]
	var out: Array = []
	var line := ""
	for word in text.split(" "):
		var next := word if line.is_empty() else line + " " + word
		if not line.is_empty() and font.get_string_size(next, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x > width:
			out.append(line); line = word
		else: line = next
	if not line.is_empty(): out.append(line)
	return out

func _cell_of(key: String, hexes: Dictionary, model: Dictionary) -> Vector2i:
	if hexes.has(key): return hexes[key].cell
	var parts := key.split(",")
	return Vector2i(int(parts[0]), int(parts[1])) if parts.size() == 2 else Vector2i(999, 999)

func _matches(entry: Dictionary, filter: String) -> bool:
	var hex: Dictionary = entry.data
	if filter in SITE_TYPES: return hex.get("site") != null and str(hex.site.get("type", "")) == filter
	match filter:
		"enemy": return hex.get("group") != null
		"warlord": return hex.get("warlord", false)
		"you": return hex.get("player", false)
		"in_sight", "remembered", "unknown": return entry.state == filter
		"countryside", "core": return false
	return str(hex.get("terrain", "")) == filter
