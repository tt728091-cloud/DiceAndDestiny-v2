extends ScrollContainer
## All placement follows the live actor HUD and dice, in battlefield coordinates.
var screen: Control
var actor_id := ""
var body: VBoxContainer
var column_width := 314.0
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
	var top := rect.end.y + 10
	var bottom := 990.0
	if actor_id == screen.viewer_actor_id:
		bottom = minf(bottom, screen._player_dice_dock.position.y - 8)
		# The action row sits above the dice. Reserve its hit area too: an
		# empty portion of this higher-z scroll container still catches clicks.
		if screen._action_footer.get_child_count() > 0:
			bottom = minf(bottom, screen._action_footer.get_parent().position.y - 8)
		if screen._roll_dock.get_combined_minimum_size().y > 0:
			bottom = minf(bottom, screen._roll_dock.position.y - 8)
	else:
		var dice: Control = screen._enemy_dice_docks.get(actor_id)
		if is_instance_valid(dice): top = maxf(top, (inverse * dice.get_global_rect()).end.y + 10)
	position = Vector2(clampf(rect.position.x, 16, 1904 - rect.size.x), top)
	# Unused scroll space must not intercept clicks on the hand beneath it.
	size = Vector2(rect.size.x, minf(body.get_combined_minimum_size().y, maxf(0, bottom - top)))
