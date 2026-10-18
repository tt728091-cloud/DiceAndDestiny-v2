extends Control
## The torn parchment chart under the hexes: its shadow on the stone table,
## the paper, a cartouche with the region's name and the calendar line (the
## authority's month, Day/Night and round), a gold rule and a compass rose.
## It covers the region's footprint (panning with the hexes) and always the
## whole visible map area, so no table shows at the sides however wide the
## window is.

const STYLE := preload("res://app/screens/map/map_style.gd")
const HEX_OFFSETS := [Vector2i(0, 0), Vector2i(1, 0), Vector2i(1, -1), Vector2i(0, -1), Vector2i(-1, 0), Vector2i(-1, 1), Vector2i(0, 1)]

## The sliver of stone table left showing around the sheet's torn edge.
const EDGE := 6.0

var screen: Node
var sheet_rect := Rect2()
## Where the cartouche (name, rule and calendar line) sits; the screen lays
## its hover area over it.
var cartouche_rect := Rect2()
var _cartouche_at := Vector2.ZERO
var _paper: StyleBoxTexture

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE

## Places the sheet over the region's footprint and the cartouche in its
## visible top-left corner; false while there is nothing to draw.
func layout() -> bool:
	# Not laid out yet: the screen lays the sheet out again once it is sized.
	if screen == null or screen.view.is_empty() or size.x <= 2.0 * EDGE or size.y <= 2.0 * EDGE: return false
	var r: float = screen._hex_radius
	var lo := Vector2(INF, INF); var hi := Vector2(-INF, -INF)
	for slot in screen._list(screen.view, "slots"):
		for offset in HEX_OFFSETS:
			var c: Vector2 = screen._hex_center(int(slot.center.q) + offset.x, int(slot.center.r) + offset.y)
			lo = lo.min(c - Vector2.ONE * r); hi = hi.max(c + Vector2.ONE * r)
	if lo.x == INF: return false
	lo -= Vector2(1.6 * r, 2.9 * r); hi += Vector2(1.6 * r, 1.6 * r)
	var sz := (hi - lo).max(Vector2(16.0 * r, 12.0 * r))
	sheet_rect = Rect2(lo, sz).merge(Rect2(Vector2.ZERO, size).grow(-EDGE))
	# Kept inside the visible area however the chart is panned.
	_cartouche_at = Vector2(maxf(sheet_rect.position.x, 0.0) + 1.2 * r, maxf(sheet_rect.position.y, 0.0) + 0.8 * r)
	var title_w := STYLE.serif_bold().get_string_size(_title(), HORIZONTAL_ALIGNMENT_LEFT, -1, 18).x
	var day_w := 22.0 + STYLE.serif().get_string_size(screen.chart_day_label(), HORIZONTAL_ALIGNMENT_LEFT, -1, 15).x
	cartouche_rect = Rect2(_cartouche_at - Vector2(4, 2), Vector2(maxf(9.0 * r, maxf(title_w, day_w)) + 8, 54))
	return true

func _title() -> String:
	return str(screen.view.get("region_name", "")).to_upper()

func _draw() -> void:
	if not layout(): return
	var r: float = screen._hex_radius
	# Shadow on the table, then the paper.
	draw_rect(STYLE.shift(sheet_rect.grow(2.0), Vector2(9, 12)), Color(0, 0, 0, 0.18))
	draw_rect(STYLE.shift(sheet_rect, Vector2(5, 7)), Color(0, 0, 0, 0.38))
	if _paper == null: _paper = STYLE.chart_paper()
	_paper.draw(get_canvas_item(), sheet_rect)
	# Cartouche: region name, gold rule, calendar line.
	var at := _cartouche_at
	draw_string(STYLE.serif_bold(), at + Vector2(0, 18), _title(), HORIZONTAL_ALIGNMENT_LEFT, -1, 18, STYLE.INK)
	draw_line(at + Vector2(0, 24), at + Vector2(9.0 * r, 24), STYLE.GOLD_LEAF, 1.0, true)
	var night: bool = screen.view.get("night", false)
	var day_at := at + Vector2(0, 42)
	STYLE.draw_glyph(self, STYLE.glyph("moon" if night else "sun"), day_at + Vector2(8, -5), 16.0)
	draw_string(STYLE.serif(), day_at + Vector2(22, 0), screen.chart_day_label(), HORIZONTAL_ALIGNMENT_LEFT, -1, 15, STYLE.INK)
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
