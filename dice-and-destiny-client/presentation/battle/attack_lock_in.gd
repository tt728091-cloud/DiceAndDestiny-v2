extends Control
## The chosen attack locks onto its target: the aim line flares, then a short
## impact at the enemy's name. Lives on the screen so a board rebuild after the
## command does not cut it short.
const AIM := preload("res://presentation/battle/attack_aim.gd")
const DURATION := 0.42
var screen: Control
var start := Vector2.ZERO # board coordinates
var finish := Vector2.ZERO
var name_size := Vector2.ZERO
var _elapsed := 0.0

func configure(owner_screen: Control, from_global: Vector2, target: Rect2) -> void:
	screen = owner_screen
	name = "AttackLockIn"; mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 45
	var to_board: Transform2D = screen._root.get_global_transform().affine_inverse()
	start = to_board * from_global; finish = to_board * target.get_center()
	name_size = target.size / screen._root.get_global_transform().get_scale()

func _process(delta: float) -> void:
	_elapsed += delta
	if _elapsed >= DURATION or not is_instance_valid(screen._root): queue_free(); return
	var board: Transform2D = screen.get_global_transform().affine_inverse() * screen._root.get_global_transform()
	position = board.origin; scale = board.get_scale()
	queue_redraw()

func _draw() -> void:
	var t := _elapsed / DURATION
	var fade := 1.0 - smoothstep(0.55, 1.0, t)
	var points := AIM.arc(start, finish)
	var flare := 1.0 - smoothstep(0.0, 0.5, t)
	draw_polyline(points, Color(1.0, 0.12, 0.05, 0.35 * fade), 22.0 * (0.6 + flare), true)
	draw_polyline(points, Color(1.0, 0.9, 0.82, fade), 4.0 + 3.0 * flare, true)
	# Impact on the name: a glow box, an expanding ring and short sparks.
	var hit := smoothstep(0.1, 0.45, t)
	var box := Rect2(finish - name_size * 0.5 - Vector2(14, 8), name_size + Vector2(28, 16))
	draw_rect(box, Color(1.0, 0.18, 0.1, 0.32 * fade), true)
	draw_rect(box, Color(1.0, 0.4, 0.3, 0.9 * fade), false, 3.0)
	var radius := lerpf(10.0, 52.0, hit)
	draw_arc(finish, radius, 0, TAU, 48, Color(1.0, 0.35, 0.22, 0.85 * fade), 4.0, true)
	for k in 8:
		var dir := Vector2.RIGHT.rotated(k * TAU / 8.0)
		draw_line(finish + dir * radius * 0.7, finish + dir * (radius + 14.0), Color(1.0, 0.75, 0.55, fade), 3.0, true)
