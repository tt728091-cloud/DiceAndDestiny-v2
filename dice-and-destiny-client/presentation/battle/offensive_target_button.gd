extends "res://presentation/battle/tooltip_button.gd"
## A local target choice: pulsing gold stays attached to the enemy artwork.
var _outline: StyleBoxFlat
var _elapsed := 0.0
func _ready() -> void:
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	_outline = preload("res://presentation/battle/cinematic_theme.gd").panel(Color("ffd76a18"), Color("ffdc78"), 12)
	_outline.set_border_width_all(3)
	for state in ["normal", "hover", "pressed", "focus"]: add_theme_stylebox_override(state, _outline)
	_process(0)
func _process(delta: float) -> void:
	_elapsed += delta
	var pulse := (sin(_elapsed * TAU / 1.1) + 1.0) * 0.5
	_outline.border_color = Color(1.0, 0.85, 0.38, lerpf(0.4, 1.0, pulse))
	_outline.bg_color = Color(1.0, 0.78, 0.25, lerpf(0.03, 0.18, pulse))
