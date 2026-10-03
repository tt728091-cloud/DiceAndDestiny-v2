extends VBoxContainer
## HUD attachment in the fighter's local coordinate system. The fighter may
## move/scale independently; effects read the resulting live control bounds.
var fighter: Control
var ground_anchor := Vector2(0.5, 1.0)
var attachment_offset := Vector2(0, 10)
func attach(to: Control) -> void:
	fighter = to
	ground_anchor = fighter.get_meta("ground_anchor", ground_anchor)
	mouse_filter = Control.MOUSE_FILTER_PASS
	_process(0)
func _process(_delta: float) -> void:
	if not is_instance_valid(fighter): return
	var ground := fighter.get_global_transform_with_canvas() * (fighter.size * ground_anchor)
	var point: Vector2 = get_parent().get_global_transform_with_canvas().affine_inverse() * ground
	position = point + attachment_offset - Vector2(size.x * 0.5, 0)
	# Containers can grow for many stacks; never retain a previous taller size.
	size.y = get_combined_minimum_size().y
