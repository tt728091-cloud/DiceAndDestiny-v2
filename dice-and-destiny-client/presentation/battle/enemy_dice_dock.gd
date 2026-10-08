extends VBoxContainer
## A fold-out never moves the name/health/status anchors beneath it.
var profile_dock: Control
var expanded := false
var targeting := false
var _effect_until := 0
func reveal_for_effect() -> void:
	_effect_until = Time.get_ticks_msec() + 180
	show()

func _notification(what: int) -> void:
	# Board rebuilds can happen after this frame's _process. Container sorting
	# still runs before drawing, so anchor there instead of flashing at (0, 0).
	if what == NOTIFICATION_SORT_CHILDREN: place()

func _process(_delta: float) -> void:
	place()

func place() -> void:
	# Transition ghosts are reparented away from the profile's coordinate space.
	if not is_instance_valid(profile_dock) or profile_dock.get_parent() != get_parent(): return
	visible = expanded or targeting or Time.get_ticks_msec() < _effect_until
	size = get_combined_minimum_size()
	position = Vector2(profile_dock.position.x + (profile_dock.size.x - size.x) * 0.5, profile_dock.position.y - size.y - 8)
