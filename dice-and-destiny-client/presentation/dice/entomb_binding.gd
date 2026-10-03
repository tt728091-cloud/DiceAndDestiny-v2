extends Control

# A persistent frame around the physical die, clear of its symbol and number.
const GOLD := Color("edc27d")
const STONE := Color("55463c")
var started_ms := -1

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	set_process(false)

func _process(_delta: float) -> void:
	if visible and started_ms >= 0:
		queue_redraw()
		if progress() >= 1.0: set_process(false)

func progress() -> float:
	return 1.0 if started_ms < 0 else clampf(float(Time.get_ticks_msec() - started_ms) / 850.0, 0.0, 1.0)

func _draw() -> void:
	var t := progress()
	var settle := 1.0 - pow(1.0 - minf(t / 0.65, 1.0), 3)
	var spread := (1.0 - settle) * 15.0
	var edge := Rect2(Vector2(1, 1), size - Vector2(2, 2))
	draw_style_box(_frame(), edge)
	# Paired metal chains clasp the sides without covering the die face.
	for side in [-1, 1]:
		var x := 5.0 - spread if side < 0 else size.x - 5.0 + spread
		draw_line(Vector2(x, 8), Vector2(x, size.y - 8), STONE, 7, true)
		for link in range(4):
			var y := 12.0 + link * (size.y - 24.0) / 3.0
			draw_style_box(_link(), Rect2(x - 3.0, y - 5.0, 6.0, 10.0))
	for y in [5.0, size.y - 5.0]:
		for x in [5.0, size.x - 5.0]: draw_circle(Vector2(x, y), 3, GOLD)
	if t < 1.0:
		var flash := sin(t * PI)
		draw_rect(edge.grow(3 + flash * 5), Color(GOLD, flash * 0.75), false, 2, true)

func _frame() -> StyleBoxFlat:
	var style := StyleBoxFlat.new(); style.bg_color = Color("78522c24")
	style.border_color = GOLD; style.set_border_width_all(2); style.set_corner_radius_all(7)
	return style

func _link() -> StyleBoxFlat:
	var style := StyleBoxFlat.new(); style.bg_color = STONE
	style.border_color = GOLD; style.set_border_width_all(1); style.set_corner_radius_all(2)
	return style
