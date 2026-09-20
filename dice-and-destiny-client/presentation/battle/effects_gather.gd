extends Control
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var panel: Control
var profiles: Dictionary
var badges: Dictionary = {}

func configure(effects: Control, actors: Dictionary) -> void:
	panel = effects; profiles = actors
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for key in panel.groups:
		var actor: String = str(key).get_slice(":", 0)
		var id: String = str(key).get_slice(":", 1)
		var count := 0
		for status in panel.summary.get("actors_before", {}).get(actor, {}).get("statuses", []):
			if str(status.get("definition_id", "")) == id: count = int(status.get("stacks", 0))
		var badge := Label.new(); badge.mouse_filter = Control.MOUSE_FILTER_IGNORE; badge.text = "%s %s ×%d" % [BattlePresentationCatalog.status(id).glyph, BattlePresentationCatalog.status(id).name, count]
		badge.add_theme_color_override("font_color", Color("d6ffb0")); badge.add_theme_font_size_override("font_size", 19); add_child(badge); badge.modulate.a = 0; badges[key] = badge

func _process(_delta: float) -> void:
	if not is_instance_valid(panel): queue_free(); return
	var t := clampf(1.0 + panel.playback_elapsed() / maxf(0.01, TIMING.effects_gather()), 0.0, 1.0)
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for key in badges:
		var actor: String = str(key).get_slice(":", 0)
		if not profiles.has(actor): continue
		var start: Vector2 = inverse * profiles[actor].statuses.get_global_rect().get_center()
		var destination: Control = panel.groups[key]
		var end: Vector2 = inverse * destination.get_global_rect().get_center()
		badges[key].position = start.lerp(end, t) - Vector2(badges[key].size.x / 2, 25 + sin(t * PI) * 40)
		badges[key].modulate.a = sin(t * PI) if t > 0 and t < 1 else 0
	queue_redraw()

func _draw() -> void:
	if not is_instance_valid(panel): return
	var duration := maxf(0.01, TIMING.effects_gather())
	var t := clampf(1.0 + panel.playback_elapsed() / duration, 0.0, 1.0)
	if t <= 0 or t >= 1: return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for key in panel.groups:
		var actor: String = str(key).get_slice(":", 0)
		if not profiles.has(actor): continue
		var start: Vector2 = inverse * profiles[actor].statuses.get_global_rect().get_center()
		var destination: Control = panel.groups[key]
		var end: Vector2 = inverse * destination.get_global_rect().get_center()
		var points := PackedVector2Array()
		for index in 20:
			var p := lerpf(maxf(0.0, t - 0.45), t, float(index) / 19)
			points.append(start.lerp(end, p) - Vector2(0, sin(p * PI) * 40))
		draw_polyline(points, Color(0.67, 0.94, 0.43, sin(t * PI)), 3, true)
		draw_circle(points[-1], 5, Color("d6ffb0"))
