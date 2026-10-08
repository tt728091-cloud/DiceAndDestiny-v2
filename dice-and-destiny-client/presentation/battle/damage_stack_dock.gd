extends ScrollContainer
## All placement follows the live actor HUD and dice, in battlefield coordinates.
## Cards threatened by an enemy attack hang beneath that attack's badge instead.
const ICONS := preload("res://presentation/battle/battle_icons.gd")
var screen: Control
var actor_id := ""
var body: VBoxContainer
var column_width := 314.0
var attack: Control
var fold_key := ""
var _focused_source := ""
func configure(owner_screen: Control, actor: String, attack_presenter: Control = null) -> void:
	screen = owner_screen; actor_id = actor; attack = attack_presenter
	name = "DamageStacks_" + actor if attack == null else "AttackCards_" + str(attack.data.source_id)
	z_index = 10
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	vertical_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	body = VBoxContainer.new(); body.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body.add_theme_constant_override("separation", 6); add_child(body)
	mouse_filter = Control.MOUSE_FILTER_PASS
	column_width = (330.0 if actor == screen.viewer_actor_id and attack == null else screen._enemy_hud_width()) - 16.0
	size.x = column_width + 16
	body.custom_minimum_size.x = column_width if attack == null else 0.0
	body.size.x = column_width

## A folded list keeps each attack's heading and pile totals only.
func show_cards() -> void:
	for node in body.find_children("*", "Control", true, false):
		if not node.has_meta("fold_key"): continue
		var shown: bool = screen.damage_cards_shown(str(node.get_meta("fold_key")))
		if node.has_meta("removal_detail"): node.visible = shown
		elif node.has_meta("cards_toggle"):
			node.icon = ICONS.texture("chevron_open" if shown else "chevron_closed")
			node.tooltip_text = "Hide the cards being removed" if shown else "Show the cards being removed"
			node.set_meta("cards_shown", shown)

func _process(_delta: float) -> void:
	if attack != null:
		_follow_attack()
		return
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

func _follow_attack() -> void:
	# The list belongs to the badge: it moves and hides with it, and stops
	# above the attacker's own dice/name so neither is ever covered.
	var badge: Control = attack.intent if is_instance_valid(attack) else null
	# While another attack from this enemy shows its cards, keep only the badge.
	var open_attack: String = screen.open_attack_for(str(attack.attacker_id)) if is_instance_valid(attack) else ""
	visible = is_instance_valid(badge) and badge.visible and (open_attack.is_empty() or open_attack == str(attack.data.source_id))
	if not visible: return
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * badge.get_global_rect()
	var top := rect.end.y + 4
	var bottom := 682.0
	var profile: Control = screen._actor_profiles.get(str(attack.attacker_id))
	if is_instance_valid(profile): bottom = (inverse * profile.get_global_rect()).position.y - 8
	var dice: Control = screen.dice_dock(str(attack.attacker_id))
	if is_instance_valid(dice) and dice.visible: bottom = minf(bottom, (inverse * dice.get_global_rect()).position.y - 8)
	# Leave room for this enemy's later badges and their folded rows.
	# Folded, leave room for this enemy's later badges and their rows; while a
	# list is open those badges sit beside its badge instead.
	if open_attack.is_empty():
		for other in screen._attack_intents.values():
			if other.attacker_id == attack.attacker_id and other.actor_slot > attack.actor_slot: bottom -= 58 + 32
	# Folded, the row is only as wide as its text so the fighter stays clickable.
	var width := column_width + 16 if screen.damage_cards_shown(fold_key) else body.get_combined_minimum_size().x + 4
	size = Vector2(width, minf(body.get_combined_minimum_size().y, maxf(28, bottom - top)))
	# Whole pixels keep the headers crisp and their flight rects exact.
	position = Vector2(clampf(rect.position.x, 16, 1904 - size.x), top).round()

## Space this badge's list occupies, so a second attack from the same enemy
## starts below its cards rather than beneath them.
func reserved_height() -> float:
	return size.y + 4 if visible else 0.0
