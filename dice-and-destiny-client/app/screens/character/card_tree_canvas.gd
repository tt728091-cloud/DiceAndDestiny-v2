extends Control
## Constellation view of one card tree. Cards are medallions joined by paths;
## upgrades sit above the base and cheaper variants below it.
signal node_selected(id: String)
signal edge_selected(id: String)
signal node_moved(id: String, at: Vector2)
signal node_drag_started(id: String)
signal connect_requested(from: String, to: String)
signal context_requested(kind: String, id: String, at: Vector2)

const NODE = preload("res://app/screens/character/card_tree_node.gd")
const CHIP = preload("res://app/screens/character/card_tree_edge_chip.gd")
const STYLE = preload("res://app/screens/character/character_style.gd")
const GRID := 20.0
const TILE := 1400.0
const MIN_ZOOM := 0.25
const MAX_ZOOM := 1.8

var tree: Dictionary = {}
var counts: Dictionary = {}
var collection_counts: Dictionary = {}
var card_art: Dictionary = {}
## Per-node change line, state (authored/owned/available/locked/unowned),
## validation issue and hover text, all supplied by the workshop.
var summaries: Dictionary = {}
var node_states: Dictionary = {}
var edge_states: Dictionary = {}
var issues: Dictionary = {}
var tooltips: Dictionary = {}
var edge_tooltips: Dictionary = {}
var selected := ""
var selected_edge := ""
var editable := false
var title := ""
var zoom := 0.8
var pan := Vector2.ZERO
## Click-to-connect mode: the next card clicked becomes the destination.
var connect_from := ""

var _world: Control
var _overlay: Control
var _nodes: Dictionary = {}
var _chips: Dictionary = {}
var _fit_on_resize := true
var _bg_drag: Dictionary = {}
var _press: Dictionary = {}
var _link_end := Vector2.ZERO
var _link_target := ""
var _hover := ""
var _hover_edges: Dictionary = {}
var _stars: Array = []
var _nebula: Array = []
var _transitions: Array = []

func _ready() -> void:
	clip_contents = true; mouse_filter = Control.MOUSE_FILTER_STOP
	_world = Control.new(); _world.mouse_filter = Control.MOUSE_FILTER_IGNORE; add_child(_world)
	_overlay = Control.new(); _overlay.mouse_filter = Control.MOUSE_FILTER_IGNORE; _overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_overlay)
	_overlay.draw.connect(_draw_overlay)
	var rng := RandomNumberGenerator.new(); rng.seed = 9137
	var tints := [Color("cfe3ff"), Color("ffe9c4"), Color("d9c8ff"), Color("ffffff")]
	for layer in 3:
		for i in [170, 90, 34][layer]:
			_stars.append({"p": Vector2(rng.randf() * TILE, rng.randf() * TILE), "layer": layer, "r": rng.randf_range(0.6, 1.4) * (1.0 + layer * 0.45), "phase": rng.randf() * TAU, "speed": rng.randf_range(0.5, 2.2), "tint": tints[rng.randi() % tints.size()]})
	_nebula = [
		{"texture": _glow(Color(0.45, 0.27, 0.8, 0.30)), "at": Vector2(-520, -360), "size": 1700.0},
		{"texture": _glow(Color(0.10, 0.45, 0.58, 0.24)), "at": Vector2(640, 260), "size": 1500.0},
		{"texture": _glow(Color(0.85, 0.62, 0.30, 0.10)), "at": Vector2(0, 0), "size": 1000.0},
		{"texture": _glow(Color(0.30, 0.20, 0.55, 0.18)), "at": Vector2(380, -620), "size": 1100.0},
	]
	resized.connect(func():
		if _fit_on_resize: home()
		else: _place())
	_place()

func _glow(color: Color) -> GradientTexture2D:
	var g := Gradient.new(); g.set_color(0, color); g.set_color(1, Color(color, 0.0))
	g.add_point(0.45, Color(color, color.a * 0.4))
	var t := GradientTexture2D.new(); t.gradient = g; t.fill = GradientTexture2D.FILL_RADIAL
	t.fill_from = Vector2(0.5, 0.5); t.fill_to = Vector2(1.0, 0.5); t.width = 256; t.height = 256
	return t

func _process(_delta: float) -> void:
	if is_visible_in_tree(): queue_redraw()

func _place() -> void:
	if not is_instance_valid(_world): return
	_world.position = size * 0.5 + pan; _world.scale = Vector2.ONE * zoom
	queue_redraw(); _overlay.queue_redraw()

func home() -> void:
	_fit_on_resize = true
	var nodes: Array = tree.get("nodes", [])
	if nodes.is_empty(): zoom = 0.8; pan = Vector2.ZERO; _place(); return
	var bounds := Rect2()
	for i in nodes.size():
		var n: Dictionary = nodes[i]
		var r := _radius(str(n.id))
		var rect := Rect2(Vector2(n.x, n.y) - Vector2(NODE.WIDTH * 0.5, r + 30), Vector2(NODE.WIDTH, r * 2 + 120))
		bounds = rect if i == 0 else bounds.merge(rect)
	bounds = bounds.grow(60)
	# Leave room for the title plate above and the legend below.
	var room := size - Vector2(0, 110)
	zoom = clampf(minf(room.x / bounds.size.x, room.y / bounds.size.y), 0.3, 1.0)
	pan = -bounds.get_center() * zoom + Vector2(0, 18); _place()

func focus_node(id: String) -> void:
	var n := _node(id)
	if n.is_empty(): return
	_fit_on_resize = false
	var target := -Vector2(n.x, n.y) * zoom
	create_tween().tween_method(func(v: Vector2): pan = v; _place(), pan, target, 0.35).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)

func _node(id: String) -> Dictionary:
	for n in tree.get("nodes", []):
		if n.id == id: return n
	return {}

func _radius(id: String) -> float:
	return NODE.BASE_RADIUS if id == str(tree.get("root", "")) else NODE.RADIUS

func _world_point(global: Vector2) -> Vector2:
	var local: Vector2 = get_global_transform().affine_inverse() * global
	return (local - size * 0.5 - pan) / zoom

func show_tree(value: Dictionary, owned: Dictionary, focus: String = "") -> void:
	tree = value; counts = owned; selected = focus
	if not is_instance_valid(_world): return
	for child in _world.get_children(): _world.remove_child(child); child.queue_free()
	_nodes.clear(); _chips.clear(); _press = {}
	if _hover != "" and _node(_hover).is_empty(): _set_hover("")
	for n in tree.get("nodes", []):
		var node := NODE.new(); node.setup(str(n.id), n.id == tree.root); _world.add_child(node)
		_nodes[str(n.id)] = node
		_apply_node(node, n)
		node.gui_input.connect(_node_input.bind(node))
		var id := str(n.id)
		node.mouse_entered.connect(func(): _set_hover(id))
		node.mouse_exited.connect(_leave.bind(id))
	for edge in tree.get("edges", []):
		var chip := CHIP.new(); chip.setup(str(edge.id)); _world.add_child(chip)
		_chips[str(edge.id)] = chip
		var a := _node(str(edge.from)); var b := _node(str(edge.to))
		if not a.is_empty() and not b.is_empty(): chip.cost = int(b.card.economy.buy) - int(a.card.economy.buy)
		chip.selected = edge.id == selected_edge
		chip.dim = edge_states.get(edge.id, "") in ["unowned", "locked"]
		for rule in edge.get("requirements", []):
			chip.rules.append({"art": STYLE.texture(str(card_art.get(rule.card_id, ""))), "forbidden": rule.get("maximum", -1) == 0})
		chip.tooltip_text = str(edge_tooltips.get(edge.id, "Select this connection to inspect its deck requirements."))
		chip.refresh()
		var edge_id := str(edge.id)
		chip.pressed.connect(func(): edge_selected.emit(edge_id))
		chip.gui_input.connect(func(event):
			if event is InputEventMouseButton and event.pressed and event.button_index == MOUSE_BUTTON_RIGHT:
				context_requested.emit("edge", edge_id, chip.get_global_transform() * event.position); chip.accept_event()
			elif _forward(event, chip): chip.accept_event())
	_refresh_chips(); _place()

func _apply_node(node, n: Dictionary) -> void:
	var id := str(n.id)
	node.title = str(n.card.name)
	node.art = STYLE.texture(str(n.card.get("presentation", {}).get("illustration_path", "")))
	node.subtitle = str(summaries.get(id, ""))
	node.xp = int(n.card.economy.buy); node.energy = int(n.card.get("cost", {}).get("energy", 0))
	node.deck_count = int(counts.get(n.card.id, 0)); node.stored_count = int(collection_counts.get(n.card.id, 0))
	node.state = str(node_states.get(id, "authored")); node.selected = id == selected
	node.issue = str(issues.get(id, "")); node.editable = editable
	node.tooltip_text = str(tooltips.get(id, n.card.get("presentation", {}).get("rules_text", "")))
	node.refresh(); node.place(Vector2(n.x, n.y))

func _refresh_chips() -> void:
	for edge in tree.get("edges", []):
		var chip = _chips.get(str(edge.id))
		if chip == null: continue
		var ends := _edge_ends(edge)
		if ends.is_empty(): chip.hide(); continue
		# Prefer the midpoint, but slide along the path to keep name plates readable.
		var best: Vector2 = (ends[0] + ends[1]) * 0.5
		for t in [0.5, 0.42, 0.58, 0.34, 0.66, 0.26, 0.74, 0.18, 0.82, 0.1, 0.9]:
			var at: Vector2 = ends[0].lerp(ends[1], t)
			var rect := Rect2(at - chip.size * 0.5, chip.size)
			if not _nodes.values().any(func(node): return node.footprint().intersects(rect)): best = at; break
		chip.position = best - chip.size * 0.5

func _edge_ends(edge: Dictionary) -> Array:
	var a := _node(str(edge.from)); var b := _node(str(edge.to))
	if a.is_empty() or b.is_empty(): return []
	var pa := Vector2(a.x, a.y); var pb := Vector2(b.x, b.y)
	if pa.distance_to(pb) < 1.0: return []
	var dir := (pb - pa).normalized()
	return [pa + dir * (_radius(str(a.id)) + 10), pb - dir * (_radius(str(b.id)) + 10)]

func _leave(id: String) -> void:
	if _hover == id: _set_hover("")

func _set_hover(id: String) -> void:
	_hover = id; _hover_edges = {}
	var stack: Array = [id] if id != "" else []
	var seen := {}
	while not stack.is_empty():
		var current: String = stack.pop_back()
		if seen.has(current): continue
		seen[current] = true
		for e in tree.get("edges", []):
			if e.to == current: _hover_edges[e.id] = true; stack.append(str(e.from))
	queue_redraw()

func play_transition(from_id: String, to_id: String) -> void:
	_transitions.append({"from": from_id, "to": to_id, "start": Time.get_ticks_msec() / 1000.0})

# Pointer handling. A press on a card selects it on release unless the pointer
# moved; editors drag cards, players drag the view. The ring knob draws links.
func _node_input(event: InputEvent, node) -> void:
	var id: String = node.node_id
	# Derive global points from the control; synthetic events may omit global_position.
	var global: Vector2 = node.get_global_transform() * event.position if event is InputEventMouse else Vector2.ZERO
	if event is InputEventMouseButton:
		if event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
			var n := _node(id)
			_press = {"id": id, "start": global, "moved": false, "link": node.knob_hit(event.position), "origin": Vector2(n.x, n.y), "pan": pan}
			if _press.link: _link_end = _world_point(global)
			node.accept_event()
		elif event.button_index == MOUSE_BUTTON_LEFT and not event.pressed and _press.get("id", "") == id:
			var press := _press; _press = {}
			node.accept_event()
			if press.link: _finish_link(id, global)
			elif press.moved:
				if editable:
					var n := _node(id)
					n.x = snappedf(float(n.x), GRID); n.y = snappedf(float(n.y), GRID)
					node.place(Vector2(n.x, n.y)); _refresh_chips(); queue_redraw()
					node_moved.emit(id, Vector2(n.x, n.y))
			elif not connect_from.is_empty():
				var from := connect_from; connect_from = ""; _overlay.queue_redraw()
				if from != id: connect_requested.emit(from, id)
			else: node_selected.emit(id)
		elif event.button_index == MOUSE_BUTTON_RIGHT and event.pressed:
			node.accept_event(); context_requested.emit("node", id, global)
		elif _forward(event, node): node.accept_event()
	elif event is InputEventMouseMotion and _press.get("id", "") == id and event.button_mask & MOUSE_BUTTON_MASK_LEFT:
		node.accept_event()
		if not _press.moved and global.distance_to(_press.start) > 5:
			_press.moved = true
			if editable and not _press.link: node_drag_started.emit(id)
		if not _press.moved: return
		if _press.link:
			_link_end = _world_point(global); _mark_link_target(_node_at(global, id)); queue_redraw()
		elif editable:
			var n := _node(id)
			var at: Vector2 = _press.origin + (_world_point(global) - _world_point(_press.start))
			n.x = at.x; n.y = at.y; node.place(at); _refresh_chips(); queue_redraw()
		else:
			_fit_on_resize = false; pan = _press.pan + (global - _press.start); _place()
	elif _forward(event, node): node.accept_event()

## Gestures report positions local to the child; convert them to the canvas.
func _forward(event: InputEvent, control: Control) -> bool:
	var local := size * 0.5
	if event is InputEventMouse or event is InputEventGesture: local = _local(control.get_global_transform() * event.position)
	return _view_input(event, local)

func _local(global: Vector2) -> Vector2:
	return get_global_transform().affine_inverse() * global

func _finish_link(from: String, global: Vector2) -> void:
	var target := _node_at(global, from)
	_mark_link_target("")
	queue_redraw()
	if not target.is_empty(): connect_requested.emit(from, target)

func _mark_link_target(id: String) -> void:
	if id == _link_target: return
	if _nodes.has(_link_target): _nodes[_link_target].link_target = false; _nodes[_link_target].queue_redraw()
	_link_target = id
	if _nodes.has(id): _nodes[id].link_target = true

func _node_at(global: Vector2, exclude: String = "") -> String:
	for id in _nodes:
		if id == exclude: continue
		var node = _nodes[id]
		if node._has_point(node.get_global_transform().affine_inverse() * global): return id
	return ""

## Zoom and gesture input shared by the background, cards and connection chips.
func _view_input(event: InputEvent, local: Vector2) -> bool:
	if event is InputEventMouseButton and event.pressed and event.button_index in [MOUSE_BUTTON_WHEEL_UP, MOUSE_BUTTON_WHEEL_DOWN]:
		var step := 1.12 if event.factor <= 0.0 else pow(1.12, maxf(event.factor, 0.2))
		_zoom_at(local, step if event.button_index == MOUSE_BUTTON_WHEEL_UP else 1.0 / step); return true
	if event is InputEventMagnifyGesture:
		_zoom_at(local, event.factor); return true
	if event is InputEventPanGesture:
		_fit_on_resize = false
		if event.ctrl_pressed or event.meta_pressed: _zoom_at(local, 1.0 - event.delta.y * 0.08)
		else: pan -= event.delta * 22.0; _place()
		return true
	return false

func _zoom_at(local: Vector2, factor: float) -> void:
	var previous := zoom
	zoom = clampf(zoom * factor, MIN_ZOOM, MAX_ZOOM)
	pan = local - size * 0.5 - (local - size * 0.5 - pan) * zoom / previous
	_fit_on_resize = false; _place()

func _gui_input(event: InputEvent) -> void:
	var global: Vector2 = get_global_transform() * event.position if event is InputEventMouse else Vector2.ZERO
	if event is InputEventMouseButton:
		if event.button_index in [MOUSE_BUTTON_LEFT, MOUSE_BUTTON_MIDDLE]:
			if event.pressed: _bg_drag = {"start": event.position, "pan": pan, "moved": false}
			else:
				if event.button_index == MOUSE_BUTTON_LEFT and not _bg_drag.get("moved", true):
					if not connect_from.is_empty(): connect_from = ""; _overlay.queue_redraw()
				_bg_drag = {}
			accept_event(); return
		if event.button_index == MOUSE_BUTTON_RIGHT and event.pressed:
			context_requested.emit("canvas", "", global); accept_event(); return
	if event is InputEventMouseMotion:
		if not connect_from.is_empty(): _link_end = _world_point(global); _mark_link_target(_node_at(global, connect_from))
		if not _bg_drag.is_empty():
			if not _bg_drag.moved and event.position.distance_to(_bg_drag.start) > 3: _bg_drag.moved = true
			if _bg_drag.moved:
				_fit_on_resize = false; pan = _bg_drag.pan + (event.position - _bg_drag.start); _place()
			accept_event(); return
	if _view_input(event, event.position if event is InputEventMouse or event is InputEventGesture else size * 0.5): accept_event()

# Drawing ---------------------------------------------------------------------
func _draw() -> void:
	var t := Time.get_ticks_msec() / 1000.0
	draw_rect(Rect2(Vector2.ZERO, size), STYLE.NIGHT)
	for n in _nebula:
		var s: float = n.size * (0.4 + zoom * 0.6)
		var c: Vector2 = size * 0.5 + pan * 0.6 + n.at * zoom * 0.6
		draw_texture_rect(n.texture, Rect2(c - Vector2(s, s) * 0.5, Vector2(s, s)), false)
	var tiles_x := int(ceil(size.x / TILE)) + 1; var tiles_y := int(ceil(size.y / TILE)) + 1
	for star in _stars:
		var p: Vector2 = star.p + pan * [0.1, 0.25, 0.45][star.layer]
		p = Vector2(fposmod(p.x, TILE), fposmod(p.y, TILE)) - Vector2(TILE, TILE) * 0.5
		var a: float = (0.3 + 0.7 * (0.5 + 0.5 * sin(t * star.speed + star.phase))) * (0.45 + 0.25 * star.layer)
		for ox in tiles_x:
			for oy in tiles_y:
				var at := p + Vector2(ox, oy) * TILE
				if at.x < -4 or at.y < -4 or at.x > size.x + 4 or at.y > size.y + 4: continue
				draw_circle(at, star.r, Color(star.tint, a))
				if star.layer == 2 and a > 0.55:
					var l: float = star.r * 4.0
					draw_line(at - Vector2(l, 0), at + Vector2(l, 0), Color(star.tint, a * 0.5), 1)
					draw_line(at - Vector2(0, l), at + Vector2(0, l), Color(star.tint, a * 0.5), 1)
	draw_set_transform(size * 0.5 + pan, 0, Vector2.ONE * zoom)
	var root := _node(str(tree.get("root", "")))
	if not root.is_empty(): _draw_sigil(Vector2(root.x, root.y), t)
	for edge in tree.get("edges", []): _draw_edge(edge, t)
	var link_from := str(_press.get("id", "")) if _press.get("link", false) and _press.get("moved", false) else connect_from
	if not link_from.is_empty():
		var a := _node(link_from)
		if not a.is_empty():
			var from := Vector2(a.x, a.y); var to := _link_end
			if _link_target != "": var b := _node(_link_target); to = Vector2(b.x, b.y)
			_dashes(from, to, STYLE.GOLD_BRIGHT, 4.0, t * 60.0)
			draw_circle(to, 6, STYLE.GOLD_BRIGHT)
	_draw_transitions(t)
	draw_set_transform(Vector2.ZERO, 0, Vector2.ONE)

func _draw_sigil(c: Vector2, t: float) -> void:
	var gold := STYLE.GOLD
	draw_circle(c, 250, Color(0.9, 0.7, 0.35, 0.025))
	draw_arc(c, 190, 0, TAU, 128, Color(gold, 0.13), 2, true)
	draw_arc(c, 250, 0, TAU, 128, Color(gold, 0.08), 1.5, true)
	draw_arc(c, 300, 0, TAU, 160, Color(gold, 0.04), 1, true)
	var spin := t * 0.04
	for k in 48:
		var a := spin + TAU * k / 48.0
		var d := Vector2(cos(a), sin(a))
		var long := k % 4 == 0
		draw_line(c + d * (212 if long else 216), c + d * (228 if long else 224), Color(gold, 0.16 if long else 0.09), 1.5, true)
	for tri in 2:
		var pts := PackedVector2Array()
		for k in 4:
			var a := -spin * 0.5 + tri * PI / 3.0 + TAU * (k % 3) / 3.0 - PI / 2.0
			pts.append(c + Vector2(cos(a), sin(a)) * 250)
		draw_polyline(pts, Color(gold, 0.05), 1.5, true)
	for k in 6:
		var a := -spin * 0.5 + TAU * k / 6.0 - PI / 2.0
		var d := Vector2(cos(a), sin(a)) * 250
		var perp := Vector2(-d.y, d.x).normalized() * 6
		draw_colored_polygon(PackedVector2Array([c + d * 0.96, c + d + perp, c + d * 1.04, c + d - perp]), Color(gold, 0.18))

func _draw_edge(edge: Dictionary, t: float) -> void:
	var ends := _edge_ends(edge)
	if ends.is_empty(): return
	var s: Vector2 = ends[0]; var e: Vector2 = ends[1]
	var dir := (e - s).normalized()
	var state := str(edge_states.get(edge.id, "authored"))
	var highlighted: bool = _hover_edges.has(edge.id) or edge.id == selected_edge or (not selected.is_empty() and (edge.from == selected or edge.to == selected))
	var core: Color
	match state:
		"owned":
			draw_line(s, e, Color(STYLE.GOLD, 0.12), 20, true); draw_line(s, e, Color(STYLE.GOLD, 0.28), 11, true)
			draw_line(s, e, Color("3a2c14"), 7, true); core = Color("f6d48a"); draw_line(s, e, core, 4, true)
		"available":
			draw_line(s, e, Color("1a2029"), 9, true); draw_line(s, e, Color(STYLE.GOLD, 0.25), 3, true)
			core = STYLE.GOLD; _dashes(s, e, core, 4.0, t * 42.0)
		"locked":
			draw_line(s, e, Color("1a2029"), 9, true); core = Color("9a5a50"); draw_line(s, e, core, 3, true)
		"unowned":
			draw_line(s, e, Color("161c25"), 9, true); core = Color("3d4858"); draw_line(s, e, core, 3, true)
		_:
			draw_line(s, e, Color("241e14"), 10, true); core = Color("b08d57"); draw_line(s, e, core, 3.5, true)
	if highlighted:
		draw_line(s, e, Color(STYLE.GOLD_BRIGHT, 0.16), 18, true)
		core = STYLE.GOLD_BRIGHT; draw_line(s, e, core, 4, true)
	var perp := Vector2(-dir.y, dir.x)
	var tip := s.lerp(e, 0.72)
	draw_polyline(PackedVector2Array([tip - dir * 13 + perp * 9, tip, tip - dir * 13 - perp * 9]), core, 3.5, true)
	if edge.get("reversible", false):
		var back := s.lerp(e, 0.26)
		draw_polyline(PackedVector2Array([back + dir * 10 + perp * 7, back, back + dir * 10 - perp * 7]), Color(core, 0.45), 2.5, true)

func _dashes(from: Vector2, to: Vector2, color: Color, width: float, offset: float) -> void:
	var length := from.distance_to(to)
	if length < 1: return
	var dir := (to - from) / length
	var d := fposmod(offset, 30.0) - 30.0
	while d < length:
		var a := maxf(d, 0.0); var b := minf(d + 16.0, length)
		if b > a: draw_line(from + dir * a, from + dir * b, color, width, true)
		d += 30.0

func _draw_transitions(t: float) -> void:
	var keep: Array = []
	for tr in _transitions:
		var a := _node(tr.from); var b := _node(tr.to)
		var p: float = (t - tr.start) / 0.9
		if a.is_empty() or b.is_empty() or p > 1.6: continue
		keep.append(tr)
		var from := Vector2(a.x, a.y); var to := Vector2(b.x, b.y)
		if p < 1.0:
			var eased := ease(p, -2.0)
			for k in 6:
				var q := maxf(0.0, eased - k * 0.04)
				draw_circle(from.lerp(to, q), 14 - k * 2, Color(STYLE.GOLD_BRIGHT, 0.5 - k * 0.07))
			draw_circle(from.lerp(to, eased), 24, Color(STYLE.GOLD_BRIGHT, 0.25))
		else:
			var burst := (p - 1.0) / 0.6
			draw_arc(to, _radius(str(b.id)) + 10 + burst * 70, 0, TAU, 64, Color(STYLE.GOLD_BRIGHT, 1.0 - burst), 4, true)
	_transitions = keep

func _draw_overlay() -> void:
	var o := _overlay
	var font := get_theme_default_font()
	for i in 8:
		o.draw_rect(Rect2(Vector2.ONE * i * 4, size - Vector2.ONE * i * 8), Color(0, 0, 0, 0.07), false, 8)
	o.draw_rect(Rect2(Vector2(1, 1), size - Vector2(2, 2)), Color(STYLE.BRONZE, 0.95), false, 2)
	o.draw_rect(Rect2(Vector2(8, 8), size - Vector2(16, 16)), Color(STYLE.BRONZE, 0.35), false, 1)
	for corner in [Vector2(0, 0), Vector2(1, 0), Vector2(0, 1), Vector2(1, 1)]:
		var at := Vector2(8 + corner.x * (size.x - 16), 8 + corner.y * (size.y - 16))
		var sx := 1.0 if corner.x == 0 else -1.0; var sy := 1.0 if corner.y == 0 else -1.0
		o.draw_line(at, at + Vector2(46 * sx, 0), STYLE.GOLD, 2); o.draw_line(at, at + Vector2(0, 46 * sy), STYLE.GOLD, 2)
		var d := at + Vector2(14 * sx, 14 * sy)
		o.draw_colored_polygon(PackedVector2Array([d + Vector2(0, -6), d + Vector2(6, 0), d + Vector2(0, 6), d + Vector2(-6, 0)]), STYLE.GOLD)
	if not title.is_empty():
		var text := title.to_upper()
		var w := font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1, 21).x
		var plate := Rect2(Vector2(size.x * 0.5 - w * 0.5 - 34, 14), Vector2(w + 68, 40))
		for side in [-1.0, 1.0]:
			var edge_x := plate.position.x if side < 0 else plate.end.x
			o.draw_line(Vector2(edge_x, 34), Vector2(edge_x + side * 90, 34), Color(STYLE.GOLD, 0.55), 1.5)
			var d := Vector2(edge_x + side * 98, 34)
			o.draw_colored_polygon(PackedVector2Array([d + Vector2(0, -5), d + Vector2(5, 0), d + Vector2(0, 5), d + Vector2(-5, 0)]), Color(STYLE.GOLD, 0.7))
		o.draw_style_box(STYLE.box(Color(0.05, 0.06, 0.09, 0.94), STYLE.GOLD, 0, 6, 2), plate)
		o.draw_rect(plate.grow(-4), Color(STYLE.GOLD, 0.25), false, 1)
		o.draw_string(font, Vector2(plate.position.x + 34, 34 + font.get_ascent(21) * 0.5 - 3), text, HORIZONTAL_ALIGNMENT_LEFT, -1, 21, STYLE.GOLD_BRIGHT)
	var legend := "↑ Upgrades cost XP     ↓ Cheaper variants refund XP"
	o.draw_string(font, Vector2(22, size.y - 20), legend, HORIZONTAL_ALIGNMENT_LEFT, -1, 14, Color(STYLE.MUTED, 0.9))
	var hint := "Drag to pan · Scroll or pinch to zoom · Right-click for actions" + (" · Drag ⊕ onto a card to connect" if editable else "")
	var hw := font.get_string_size(hint, HORIZONTAL_ALIGNMENT_LEFT, -1, 14).x
	if hw + font.get_string_size(legend, HORIZONTAL_ALIGNMENT_LEFT, -1, 14).x + 70 < size.x:
		o.draw_string(font, Vector2(size.x - hw - 22, size.y - 20), hint, HORIZONTAL_ALIGNMENT_LEFT, -1, 14, Color(STYLE.MUTED, 0.9))
	if not connect_from.is_empty():
		var from_name := str(_node(connect_from).get("card", {}).get("name", ""))
		var banner := "Connecting from %s · click the destination card · Esc to cancel" % from_name
		var bw := font.get_string_size(banner, HORIZONTAL_ALIGNMENT_LEFT, -1, 16).x
		var rect := Rect2(Vector2(size.x * 0.5 - bw * 0.5 - 16, 64), Vector2(bw + 32, 32))
		o.draw_style_box(STYLE.box(Color(0.1, 0.08, 0.03, 0.95), STYLE.GOLD, 0, 16, 1), rect)
		o.draw_string(font, Vector2(rect.position.x + 16, 80 + font.get_ascent(16) * 0.5 - 3), banner, HORIZONTAL_ALIGNMENT_LEFT, -1, 16, STYLE.GOLD_BRIGHT)
	if tree.get("nodes", []).is_empty():
		var empty := "Choose or create a base card to begin this tree" if editable else "No card tree selected"
		var ew := font.get_string_size(empty, HORIZONTAL_ALIGNMENT_LEFT, -1, 20).x
		o.draw_string(font, size * 0.5 - Vector2(ew * 0.5, 0), empty, HORIZONTAL_ALIGNMENT_LEFT, -1, 20, Color(STYLE.GOLD, 0.8))
