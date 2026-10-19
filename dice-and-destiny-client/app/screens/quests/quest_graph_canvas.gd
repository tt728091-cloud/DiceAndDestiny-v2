extends Control
## One quest's graph, read-only: stage and end cards in columns by depth (left
## to right), branches stacked vertically, and every outcome an edge coloured
## by its type. An edge leaves its stage, runs down a trunk in the gap and
## enters the next card; its label sits on that last segment, so labels never
## share a row. Everything shown is the authority's described graph; the
## canvas only places it. It sits in a ScrollContainer, which scrolls when the
## graph is larger than the area.
signal node_selected(id: String)
## "" when the pointer leaves a card.
signal node_hovered(id: String)

const STYLE := preload("res://app/screens/character/character_style.gd")
const STAGE_WIDTH := 250.0
const END_WIDTH := 150.0
const COLUMN_GAP := 230.0
const ROW_GAP := 22.0
const MARGIN := 24.0
const TRUNK := 16.0
const TRUNK_STEP := 6.0
const LABEL_WIDTH := 170.0
const OUTCOME_COLOURS := {"success": STYLE.GAIN, "failure": STYLE.LOSS, "abandoned": STYLE.MUTED}
const TIMER_COLOURS := {"urgent": STYLE.AMBER, "seasonal": STYLE.ENERGY, "none": STYLE.DIM}
const END_TITLES := {"success": "Complete", "failure": "Failed", "abandoned": "Abandoned"}

var quest: Dictionary = {}
var selected := ""
## Node id -> its card; edge id -> its label chip; edge id -> its polyline
## (canvas coordinates, from the stage to the next card).
var cards: Dictionary = {}
var labels: Dictionary = {}
var paths: Dictionary = {}
var _queued := false

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_PASS

static func _list(source: Dictionary, key: String) -> Array:
	var value = source.get(key)
	return value if value is Array else []

func show_quest(q: Dictionary, selected_id: String) -> void:
	quest = q; selected = selected_id
	for child in get_children(): remove_child(child); child.queue_free()
	cards.clear(); labels.clear(); paths.clear()
	for node in _list(q, "nodes"): cards[str(node.id)] = _card(node)
	for edge in _list(q, "edges"): labels[str(edge.id)] = _edge_label(edge)
	_queue_layout()

func select(id: String) -> void:
	selected = id
	for key in cards: _style_card(cards[key], _node(key))

func _node(id: String) -> Dictionary:
	for node in _list(quest, "nodes"):
		if str(node.id) == id: return node
	return {}

func _style_card(card: PanelContainer, node: Dictionary) -> void:
	var id := str(node.get("id", ""))
	var colour: Color = OUTCOME_COLOURS.get(str(node.get("outcome", "")), STYLE.BRONZE)
	if node.get("kind", "") == "stage": colour = STYLE.GOLD if node.get("start", false) else Color("2b4150")
	var width := 2 if id == selected else 1
	if id == selected: colour = STYLE.GOLD_BRIGHT
	card.add_theme_stylebox_override("panel", STYLE.box(STYLE.RAISED if node.get("kind", "") == "stage" else STYLE.FIELD, colour, 10, 8, width))

## A stage card: its title, its objective (when the title is not already it),
## and chips for the timer and anything not yet implemented.
func _card(node: Dictionary) -> PanelContainer:
	var card := PanelContainer.new(); card.name = "Node_" + str(node.id).replace(":", "_").replace("#", "_")
	card.set_meta("quest_node", str(node.id)); card.mouse_filter = Control.MOUSE_FILTER_STOP
	var stage: bool = node.get("kind", "") == "stage"
	card.custom_minimum_size.x = STAGE_WIDTH if stage else END_WIDTH
	_style_card(card, node)
	add_child(card)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 4); box.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.add_child(box)
	if stage:
		STYLE.label(box, str(node.get("title", "")), 15, STYLE.GOLD_BRIGHT if node.get("start", false) else STYLE.IVORY).name = "Title"
		var objective := str(node.get("objective", {}).get("text", ""))
		if objective != str(node.get("title", "")): STYLE.label(box, objective, 13, STYLE.MUTED).name = "Objective"
		var chips := HFlowContainer.new(); chips.add_theme_constant_override("h_separation", 4); chips.add_theme_constant_override("v_separation", 4); chips.mouse_filter = Control.MOUSE_FILTER_IGNORE; box.add_child(chips)
		var timer: Dictionary = node.get("timer", {})
		STYLE.chip(chips, str(timer.get("chip", "")), TIMER_COLOURS.get(str(timer.get("type", "none")), STYLE.DIM), 12).name = "TimerChip"
		if node.get("start", false): STYLE.chip(chips, "Start", STYLE.GOLD, 12)
		for badge in _list(node, "badges"):
			if str(badge.get("engine", "")) != "scenario":
				STYLE.chip(chips, "%s · %s" % [str(badge.get("engine_text", "")), str(badge.get("verb", "")).replace("timer:", "")], STYLE.LOSS, 12).name = "Badge"
	else:
		var outcome := str(node.get("outcome", ""))
		var title := STYLE.label(box, str(node.get("title", END_TITLES.get(outcome, "End"))), 14, OUTCOME_COLOURS.get(outcome, STYLE.IVORY)); title.name = "Title"
		title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	for label in card.find_children("*", "Label", true, false): label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var id := str(node.id)
	card.gui_input.connect(func(event: InputEvent):
		if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed: node_selected.emit(id))
	card.mouse_entered.connect(func(): node_hovered.emit(id))
	card.mouse_exited.connect(func(): node_hovered.emit(""))
	card.resized.connect(_queue_layout)
	return card

## The edge's label chip; hovering it names the cause and the consequences.
func _edge_label(edge: Dictionary) -> PanelContainer:
	var colour: Color = OUTCOME_COLOURS.get(str(edge.get("outcome", "")), STYLE.MUTED)
	var chip := PanelContainer.new(); chip.name = "Edge_" + str(edge.id).replace("#", "_")
	chip.set_meta("quest_edge", str(edge.id))
	chip.add_theme_stylebox_override("panel", STYLE.box(Color("0b131a"), colour, 6, 6, 1))
	chip.custom_minimum_size.x = LABEL_WIDTH; chip.mouse_filter = Control.MOUSE_FILTER_STOP
	var lines: Array = ["%s · when: %s" % [str(edge.get("outcome", "")).capitalize(), str(edge.get("when", {}).get("text", ""))]]
	for c in _list(edge, "consequences"): lines.append("• " + str(c.get("text", "")))
	chip.tooltip_text = "\n".join(lines)
	add_child(chip)
	var label := STYLE.label(chip, str(edge.get("label", "")), 12, colour.lightened(0.2)); label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	chip.resized.connect(_queue_layout)
	return chip

func _queue_layout() -> void:
	if _queued: return
	_queued = true
	_layout.call_deferred()

## Columns by depth; within a column, nodes follow their parents' order. Each
## row is tall enough for its card and the labels entering it.
func _layout() -> void:
	_queued = false
	if quest.is_empty(): return
	var nodes: Array = _list(quest, "nodes")
	var edges: Array = _list(quest, "edges")
	var incoming := {}; var outgoing := {}
	for edge in edges:
		if not incoming.has(str(edge.to)): incoming[str(edge.to)] = []
		if not outgoing.has(str(edge.from)): outgoing[str(edge.from)] = []
		incoming[str(edge.to)].append(edge); outgoing[str(edge.from)].append(edge)
	var columns := {}
	var max_depth := 0
	for node in nodes:
		var depth := int(node.get("depth", 0))
		max_depth = maxi(max_depth, depth)
		if not columns.has(depth): columns[depth] = []
		columns[depth].append(node)
	# A card outside a container only ever grows; autowrapped text measures
	# tall before it has its width, so settle every card and label at its
	# minimum height first (a changed size lays out again).
	for id in cards: cards[id].reset_size()
	for id in labels: labels[id].reset_size()
	var rank := {}
	var order := {}
	for i in nodes.size(): order[str(nodes[i].id)] = i
	for depth in range(max_depth + 1):
		var column: Array = columns.get(depth, [])
		# Children follow their parents' rows, then the parents' edge order;
		# the start comes first.
		var key := {}
		for node in column:
			var id := str(node.id)
			var best := INF
			for edge in incoming.get(id, []):
				var parent := str(edge.from)
				if rank.has(parent): best = minf(best, float(rank[parent]) * 1000.0 + float(outgoing[parent].find(edge)))
			if node.get("start", false): best = -1.0
			elif best == INF: best = 1000000.0 + float(order[id])
			key[id] = best
		column.sort_custom(func(a, b): return key[str(a.id)] < key[str(b.id)] or (key[str(a.id)] == key[str(b.id)] and order[str(a.id)] < order[str(b.id)]))
		var y := MARGIN
		for i in column.size():
			var node: Dictionary = column[i]
			var id := str(node.id)
			rank[id] = i
			var card: Control = cards[id]
			var row := card.size.y
			for edge in incoming.get(id, []): row = maxf(row, labels[str(edge.id)].size.y + 8.0)
			card.position = Vector2(MARGIN + depth * (STAGE_WIDTH + COLUMN_GAP), y + (row - card.size.y) * 0.5)
			y += row + ROW_GAP
	var bounds := Rect2(Vector2.ZERO, Vector2.ONE * MARGIN)
	for id in cards: bounds = bounds.merge(cards[id].get_rect())
	# Edges: out of the stage's right side, down a trunk in the gap, into the
	# next card's left side, where the label sits.
	for edge in edges:
		var from: Control = cards.get(str(edge.from)); var to: Control = cards.get(str(edge.to))
		if from == null or to == null: continue
		var outs: Array = outgoing[str(edge.from)]; var ins: Array = incoming[str(edge.to)]
		var i := outs.find(edge); var j := ins.find(edge)
		var start := Vector2(from.position.x + from.size.x, from.position.y + from.size.y * 0.5 + (i - (outs.size() - 1) * 0.5) * minf(8.0, from.size.y / maxf(1.0, outs.size())))
		var finish := Vector2(to.position.x, to.position.y + to.size.y * 0.5 + (j - (ins.size() - 1) * 0.5) * 14.0)
		var trunk := start.x + TRUNK + i * TRUNK_STEP
		paths[str(edge.id)] = PackedVector2Array([start, Vector2(trunk, start.y), Vector2(trunk, finish.y), finish])
		var chip: Control = labels[str(edge.id)]
		chip.position = Vector2((trunk + finish.x) * 0.5 - chip.size.x * 0.5, finish.y - chip.size.y * 0.5)
		bounds = bounds.merge(chip.get_rect())
	custom_minimum_size = bounds.end + Vector2.ONE * MARGIN
	queue_redraw()

func _draw() -> void:
	for edge in _list(quest, "edges"):
		var points: PackedVector2Array = paths.get(str(edge.id), PackedVector2Array())
		if points.size() < 2: continue
		var colour: Color = OUTCOME_COLOURS.get(str(edge.get("outcome", "")), STYLE.MUTED)
		draw_polyline(points, colour, 2.0, true)
		var tip := points[points.size() - 1]
		draw_colored_polygon(PackedVector2Array([tip, tip + Vector2(-9, -5), tip + Vector2(-9, 5)]), colour)
