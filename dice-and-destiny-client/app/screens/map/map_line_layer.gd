extends Control
## Ink lines graded with the paper: the dotted pencil wedges of unexplored
## slots (sepia for countryside, red-violet for core) and the slab outline of
## every placed tile, so the exploration unit reads as a cut piece.

const STYLE := preload("res://app/screens/map/map_style.gd")

var screen: Node

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _draw() -> void:
	if screen == null or screen.map_model.is_empty(): return
	var model: Dictionary = screen.map_model
	var r: float = screen._hex_radius
	var center_of := func(q: int, rr: int) -> Vector2: return screen._hex_center(q, rr)
	for id in model.slots:
		var slot: Dictionary = model.slots[id]
		var cells: Array = slot.hexes
		if slot.placed:
			var lit := false
			for cell in cells:
				var hex: Dictionary = model.hexes.get("%d,%d" % [cell.x, cell.y], {})
				if hex.get("state", "") == "in_sight": lit = true
			for edge in STYLE.outer_edges(cells, center_of, r):
				draw_line(edge[0], edge[1], Color(STYLE.INK, 0.6 if lit else 0.4), 2.5, true)
			continue
		var pencil: Color = STYLE.CORE_PENCIL if slot.kind == "core" else STYLE.PENCIL
		var hovered := false
		if not str(model.hover).is_empty():
			for cell in cells:
				if "%d,%d" % [cell.x, cell.y] == model.hover: hovered = true
		if model.legend_filter == slot.kind: hovered = true
		for edge in STYLE.inner_edges(cells, center_of, r):
			draw_dashed_line(edge[0], edge[1], Color(STYLE.PENCIL, 0.25), 1.0, 2.0, true, true)
		for edge in STYLE.outer_edges(cells, center_of, r):
			draw_dashed_line(edge[0], edge[1], Color(pencil, 1.0 if hovered else 0.85), 1.6 if hovered else 1.2, 3.5, true, true)
