extends Control

var panel: Control
var profiles: Dictionary

func configure(effects: Control, actors: Dictionary) -> void:
	panel = effects; profiles = actors
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	z_index = 25

func _process(_delta: float) -> void:
	if not is_instance_valid(panel): queue_free(); return
	queue_redraw()

func _draw() -> void:
	if not is_instance_valid(panel): return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for trail in panel.catalyst_trails():
		var profile: ActorProfile = profiles.get(str(trail.holder))
		if not is_instance_valid(profile): continue
		var start: Vector2 = inverse * profile.status_anchor("catalyst")
		var destination: Vector2 = inverse * trail.target.get_global_rect().get_center()
		var tip := start.lerp(destination, float(trail.travel))
		var alpha := float(trail.alpha)
		draw_line(start, tip, Color(0.55, 0.94, 0.48, alpha * 0.18), 9, true)
		draw_line(start, tip, Color(0.69, 1.0, 0.58, alpha), 2.5, true)
		draw_circle(start, 5, Color(0.80, 1.0, 0.68, alpha))
		draw_circle(tip, 5, Color(0.80, 1.0, 0.68, alpha))
