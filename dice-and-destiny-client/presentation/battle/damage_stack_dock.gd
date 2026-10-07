extends ScrollContainer
## All placement follows the live actor HUD and dice, in battlefield coordinates.
var screen: Control
var actor_id := ""
var body: VBoxContainer
var column_width := 314.0
var _focused_source := ""
func configure(owner_screen: Control, actor: String) -> void:
	screen = owner_screen; actor_id = actor
	name = "DamageStacks_" + actor
	z_index = 10
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	vertical_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	body = VBoxContainer.new(); body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.add_theme_constant_override("separation", 6); add_child(body)
	mouse_filter = Control.MOUSE_FILTER_PASS
	column_width = (330.0 if actor == screen.viewer_actor_id else screen._enemy_hud_width()) - 16.0
	size.x = column_width + 16
	body.custom_minimum_size.x = column_width
	body.size.x = column_width
func _process(_delta: float) -> void:
	var profile: Control = screen._actor_profiles.get(actor_id)
	if not is_instance_valid(profile): return
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * profile.get_global_rect()
	# Pending cards share the owner's zone, with independent scrolling for
	# many attacks. Hidden enemy dice never reserve lower-screen space.
	var top := rect.end.y + 8
	var bottom := 682.0
	if actor_id == screen.viewer_actor_id:
		top = 704
		bottom = rect.position.y - 12
	position = Vector2(clampf(rect.position.x, 16, 1904 - rect.size.x), top)
	size = Vector2(rect.size.x, minf(body.get_combined_minimum_size().y, maxf(0, bottom - top)))

	var selected := str(screen._selected_source)
	if not selected.is_empty() and selected != _focused_source:
		for group in body.get_children():
			if group.get_meta("source_id", "") == selected:
				# Wait for the body to lay out before scrolling to this attack.
				if group.get_index() > 0 and group.position.y <= 0: return
				scroll_vertical = int(group.position.y)
				_focused_source = selected
