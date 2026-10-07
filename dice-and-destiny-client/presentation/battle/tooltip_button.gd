extends Button

func _get_tooltip(_at_position: Vector2) -> String:
	return preload("res://presentation/battle/wrapped_tooltip.gd").content(tooltip_text)

func _make_custom_tooltip(for_text: String) -> Object:
	return preload("res://presentation/battle/wrapped_tooltip.gd").create(self, for_text)
