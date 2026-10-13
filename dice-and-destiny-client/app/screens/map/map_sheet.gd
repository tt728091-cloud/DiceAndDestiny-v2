extends Control
## The torn parchment chart under the hexes: its shadow on the stone table,
## the paper, a cartouche with the region's name and the day line, a gold
## rule and a compass rose. It is positioned from the screen's hex geometry,
## so it pans with the hexes.

const STYLE := preload("res://app/screens/map/map_style.gd")
const HEX_OFFSETS := [Vector2i(0, 0), Vector2i(1, 0), Vector2i(1, -1), Vector2i(0, -1), Vector2i(-1, 0), Vector2i(-1, 1), Vector2i(0, 1)]

var screen: Node
var sheet_rect := Rect2()
var _paper: StyleBoxTexture

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE

func _draw() -> void:
	if screen == null or screen.view.is_empty(): return
	var r: float = screen._hex_radius
	var lo := Vector2(INF, INF); var hi := Vector2(-INF, -INF)
	for slot in screen._list(screen.view, "slots"):
		for offset in HEX_OFFSETS:
			var c: Vector2 = screen._hex_center(int(slot.center.q) + offset.x, int(slot.center.r) + offset.y)
			lo = lo.min(c - Vector2.ONE * r); hi = hi.max(c + Vector2.ONE * r)
	if lo.x == INF: return
	lo -= Vector2(1.6 * r, 2.9 * r); hi += Vector2(1.6 * r, 1.6 * r)
	var sz := (hi - lo).max(Vector2(16.0 * r, 12.0 * r))
	sheet_rect = Rect2(lo, sz)
	# Shadow on the table, then the paper.
	draw_rect(STYLE.shift(sheet_rect.grow(2.0), Vector2(9, 12)), Color(0, 0, 0, 0.18))
	draw_rect(STYLE.shift(sheet_rect, Vector2(5, 7)), Color(0, 0, 0, 0.38))
	if _paper == null: _paper = STYLE.chart_paper()
	_paper.draw(get_canvas_item(), sheet_rect)
	# Cartouche: region name, day line, gold rule — kept inside the visible area.
	var at := Vector2(maxf(sheet_rect.position.x, 0.0) + 1.2 * r, maxf(sheet_rect.position.y, 0.0) + 0.8 * r)
	var title := str(screen.view.get("region_name", "")).to_upper()
	draw_string(STYLE.serif_bold(), at + Vector2(0, 18), title, HORIZONTAL_ALIGNMENT_LEFT, -1, 18, STYLE.INK)
	draw_line(at + Vector2(0, 24), at + Vector2(9.0 * r, 24), STYLE.GOLD_LEAF, 1.0, true)
	var night: bool = screen.view.get("night", false)
	var day_at := at + Vector2(0, 42)
	STYLE.draw_glyph(self, STYLE.glyph("moon" if night else "sun"), day_at + Vector2(8, -5), 16.0)
	draw_string(STYLE.serif(), day_at + Vector2(22, 0), str(screen.view.get("day_label", "")), HORIZONTAL_ALIGNMENT_LEFT, -1, 14, STYLE.INK)
	# Compass rose at the bottom-right of the visible sheet.
	var rose := Vector2(minf(sheet_rect.end.x, size.x), minf(sheet_rect.end.y, size.y)) - Vector2(1.5 * r, 1.4 * r)
	var big := 0.9 * r
	var diagonal := _star(rose, big, PI * 0.25)
	draw_colored_polygon(diagonal, STYLE.PAPER)
	STYLE.draw_ring(self, diagonal, STYLE.INK, 1.0)
	var upright := _star(rose, big, 0.0)
	draw_colored_polygon(upright, STYLE.BONE)
	STYLE.draw_ring(self, upright, STYLE.INK, 1.0)
	draw_colored_polygon(PackedVector2Array([rose + Vector2(0, -big), rose + Vector2(0.22 * big, -0.22 * big), rose + Vector2(-0.22 * big, -0.22 * big)]), STYLE.GOLD_LEAF)
	STYLE.draw_centered(self, STYLE.serif_bold(), rose + Vector2(0, -big - 0.3 * r), "N", 12, STYLE.INK)

static func _star(center: Vector2, radius: float, rotation: float) -> PackedVector2Array:
	var points := PackedVector2Array()
	var shape := [Vector2(0, -1), Vector2(0.22, -0.22), Vector2(1, 0), Vector2(0.22, 0.22), Vector2(0, 1), Vector2(-0.22, 0.22), Vector2(-1, 0), Vector2(-0.22, -0.22)]
	for p in shape: points.append(center + (p * radius).rotated(rotation))
	return points
