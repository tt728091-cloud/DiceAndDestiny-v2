extends "res://presentation/battle/tooltip_button.gd"
## A local target choice over the enemy artwork: an invisible hit area. The
## aim line and the glowing name show the choice (attack_aim.gd).
func _ready() -> void:
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	for state in ["normal", "hover", "pressed", "focus", "disabled"]: add_theme_stylebox_override(state, StyleBoxEmpty.new())
