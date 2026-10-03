extends Control
const GREEN := Color("b4f28f")
var stack: Control
var screen: Control
var target := ""
var fragments: Array[Dictionary] = []
var progress := 0.0
var endpoint := Vector2.ZERO
var origin := Vector2.ZERO
func configure(source_stack: Control, owner_screen: Control, actor: String) -> void:
	stack = source_stack; screen = owner_screen; target = actor
	mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 25
	for card in stack.get_children():
		for half in 2:
			var clip := Control.new(); clip.clip_contents = true; clip.mouse_filter = Control.MOUSE_FILTER_IGNORE; add_child(clip)
			var copy := BattleCard.new(); clip.add_child(copy); copy.configure(card.instance_id, card.definition_id, false, true)
			copy.mouse_filter = Control.MOUSE_FILTER_IGNORE
			var title: Label = copy.get_node("CardTitle")
			title.autowrap_mode = TextServer.AUTOWRAP_OFF; title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
			title.offset_left = 29; title.offset_top = 0; title.offset_bottom = stack.STRIDE
			var energy: Label = copy.get_node("EnergyCost")
			energy.offset_left = 4; energy.offset_right = 24; energy.offset_top = 2; energy.offset_bottom = stack.STRIDE - 2
			energy.add_theme_font_size_override("font_size", 14)
			fragments.append({"clip": clip, "copy": copy, "original": card, "half": half})
func present_progress(value: float) -> void:
	progress = value
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var profile: Control = screen._actor_profiles.get(target)
	if not is_instance_valid(profile): return
	endpoint = inverse * profile.anchor_rect("removed").get_center()
	var stack_rect: Rect2 = inverse * stack.get_global_rect()
	var bounds: Rect2 = inverse * stack.dock.get_global_rect()
	origin = Vector2(stack_rect.get_center().x, clampf(stack_rect.position.y + 20, bounds.position.y + 8, bounds.end.y - 8))
	for entry in fragments:
		var rect: Rect2 = inverse * stack.visible_card_rect(entry.original)
		var clip: Control = entry.clip
		clip.visible = rect.has_area() and progress < 0.8
		if not clip.visible: continue
		var direction := -1.0 if int(entry.half) == 0 else 1.0
		var split := smoothstep(0, 1, clampf(progress / 0.55, 0, 1))
		clip.size = Vector2(rect.size.x, rect.size.y * 0.5)
		clip.position = rect.position + Vector2(direction * split * 12, int(entry.half) * clip.size.y + direction * split * 6)
		clip.rotation = direction * split * 0.05
		clip.modulate.a = 1.0 - clampf((progress - 0.4) / 0.4, 0, 1)
		entry.copy.size = entry.original.size
		entry.copy.position = Vector2(0, -int(entry.half) * clip.size.y)
	queue_redraw()
func _draw() -> void:
	for entry in fragments:
		var clip: Control = entry.clip
		if not clip.visible: continue
		var points := PackedVector2Array()
		var y := clip.size.y if int(entry.half) == 0 else 0.0
		for x in range(0, int(clip.size.x) + 1, 6): points.append(clip.position + Vector2(x, y + (2 if (x / 6) % 2 == 0 else -2)))
		if points.size() > 1: draw_polyline(points, Color(0.96, 0.82, 0.59, clip.modulate.a), 3, true)
	var t := clampf((progress - 0.25) / 0.5, 0, 1)
	var fade := 1.0 - clampf((progress - 0.8) / 0.2, 0, 1)
	if t <= 0 or fade <= 0: return
	var points := PackedVector2Array()
	for step in 24:
		var p := lerpf(maxf(0, t - 0.45), t, step / 23.0)
		points.append(origin.lerp(endpoint, p) + Vector2(sin(p * PI) * 25, 0))
	draw_polyline(points, Color(GREEN, fade * 0.18), 10, true)
	draw_polyline(points, Color(GREEN, fade), 3, true)
	draw_circle(points[-1], 4, Color(GREEN, fade))
	if t > 0.8: draw_circle(endpoint, 9 * (1.0 - progress) + 3, Color(GREEN, fade * 0.6))
