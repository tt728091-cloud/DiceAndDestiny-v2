extends VBoxContainer
## A fold-out never moves the name/health/status anchors beneath it.
var profile_dock: Control
var expanded := false
var targeting := false
var _effect_until := 0
func reveal_for_effect() -> void:
	_effect_until = Time.get_ticks_msec() + 180
	show()
func _process(_delta: float) -> void:
	if not is_instance_valid(profile_dock): return
	visible = expanded or targeting or Time.get_ticks_msec() < _effect_until
	size = get_combined_minimum_size()
	position = Vector2(profile_dock.position.x + (profile_dock.size.x - size.x) * 0.5, profile_dock.position.y - size.y - 8)
