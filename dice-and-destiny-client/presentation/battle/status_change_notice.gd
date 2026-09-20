extends Control

# Lives outside the rebuilt battle layout so a new decision cannot cut off the
# feedback. It never captures input or blocks progression.
const DURATION := 1.8
var screen: Control
var update: Dictionary
var _label: Label
var _elapsed := 0.0
var _battle_id := ""

func configure(owner_screen: Control, change: Dictionary) -> void:
	screen = owner_screen
	update = change
	_battle_id = screen._view.battle_id
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	z_index = 30
	_label = Label.new()
	_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.add_theme_font_size_override("font_size", 19)
	_label.add_theme_color_override("font_outline_color", Color("09121c"))
	_label.add_theme_constant_override("outline_size", 5)
	add_child(_label)
	_label.text = "+1 Incubation" if update.kind == "incubation" else "Poison → Volatile Poison"
	if update.kind == "application": _label.text = "+%d %s" % [int(update.data.after) - int(update.data.before), BattlePresentationCatalog.status(str(update.data.status_id)).name]
	if update.kind == "resource": _label.text = "+%d %s" % [int(update.data.amount), str(update.data.caption)]
	if not str(update.get("card_name", "")).is_empty(): _label.text = str(update.card_name) + "\n" + _label.text
	_label.modulate = Color("d5afff") if update.kind == "conversion" else Color("b7e39a")

func _process(delta: float) -> void:
	_elapsed += delta
	refresh()

func refresh() -> void:
	if not is_instance_valid(screen) or screen._view.battle_id != _battle_id: queue_free(); return
	var profile: ActorProfile = screen._actor_profiles.get(str(update.data.target_actor_id))
	var lane := 0
	for sibling in screen.get_children():
		if sibling == self: break
		if sibling.get_script() == get_script() and sibling.update.data.target_actor_id == update.data.target_actor_id: lane += 1
	if is_instance_valid(profile):
		var bounds := profile.statuses.get_global_rect()
		position = screen.get_global_transform_with_canvas().affine_inverse() * (bounds.position + Vector2(0, bounds.size.y + 10 + lane * 52 - minf(_elapsed, 1.0) * 10))
		if update.kind == "resource": profile.show_resource_gain(update.data, clampf(_elapsed / 0.85, 0, 1))
		else: profile.show_status_transition(update, clampf(_elapsed / 0.85, 0, 1))
	_label.modulate.a = clampf(_elapsed / 0.15, 0, 1) * clampf((DURATION - _elapsed) / 0.45, 0, 1)
	if _elapsed >= DURATION: queue_free()
