extends Control

const TIMING := preload("res://presentation/battle/defense_timing.gd")
var origin: Control
var profile: ActorProfile
var gain: Dictionary
var started_ms: int
var arrived := false
var badge: Label

func configure(from: Control, to: ActorProfile, value: Dictionary, start: int) -> void:
	origin = from; profile = to; gain = value; started_ms = start
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); mouse_filter = Control.MOUSE_FILTER_IGNORE
	badge = Label.new(); badge.mouse_filter = Control.MOUSE_FILTER_IGNORE; badge.add_theme_font_size_override("font_size", 18); badge.add_theme_color_override("font_color", Color("c6f594"))
	var status := BattlePresentationCatalog.status(str(gain.status_id)); badge.text = "+%d %s %s" % [int(gain.amount), status.glyph, status.name]; add_child(badge)
	set_meta("inspection_id", "battle.defense_status_flight." + str(gain.target) + "." + str(gain.status_id))
	_process(0)

func _process(_delta: float) -> void:
	if not is_instance_valid(origin) or not is_instance_valid(profile): queue_free(); return
	var t := clampf(((Time.get_ticks_msec() - started_ms) / 1000.0 - TIMING.roll_seconds() - TIMING.effects_seconds() * 0.3) / (TIMING.effects_seconds() * 0.55), 0, 1)
	var start := _local_center(origin)
	var end := _local_center(profile.statuses)
	badge.position = start.lerp(end, t) - Vector2(badge.size.x / 2, 24 + sin(t * PI) * 55)
	badge.modulate.a = sin(t * PI) if t > 0 and t < 1 else 0
	if t >= 1 and not arrived:
		arrived = true
		profile.show_defense_status_preview(str(gain.status_id), int(gain.after))
	queue_redraw()

func _draw() -> void:
	if not is_instance_valid(origin) or not is_instance_valid(profile) or arrived: return
	var t := clampf(((Time.get_ticks_msec() - started_ms) / 1000.0 - TIMING.roll_seconds() - TIMING.effects_seconds() * 0.3) / (TIMING.effects_seconds() * 0.55), 0, 1)
	if t <= 0: return
	var start := _local_center(origin)
	var end := _local_center(profile.statuses)
	var points := PackedVector2Array()
	for index in 18:
		var p := lerpf(maxf(0, t - 0.45), t, float(index) / 17)
		points.append(start.lerp(end, p) - Vector2(0, sin(p * PI) * 55))
	draw_polyline(points, Color(0.67, 0.94, 0.43, sin(t * PI) * 0.8), 3, true)
	draw_circle(points[-1], 5, Color("d6ffb0"))

func _local_center(control: Control) -> Vector2:
	# The cinematic stage scales as a whole; convert viewport positions back
	# into this effect's coordinates before drawing the trail and badge.
	return get_global_transform_with_canvas().affine_inverse() * control.get_global_rect().get_center()
