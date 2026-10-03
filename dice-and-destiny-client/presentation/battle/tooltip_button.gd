extends Button

func _make_custom_tooltip(for_text: String) -> Object:
	return preload("res://presentation/battle/wrapped_tooltip.gd").create(self, for_text)
