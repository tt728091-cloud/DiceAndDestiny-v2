extends Label

const MAX_TOOLTIP_WIDTH := 420.0
const VIEWPORT_MARGIN := 48.0

func _make_custom_tooltip(for_text: String) -> Object:
	var description := Label.new()
	description.theme_type_variation = &"TooltipLabel"
	description.text = for_text
	description.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	# Godot positions the popup within the viewport; constrain its content first
	# so even long status rules can fit without running off either screen edge.
	description.custom_minimum_size.x = minf(MAX_TOOLTIP_WIDTH, maxf(1.0, get_viewport_rect().size.x - VIEWPORT_MARGIN))
	return description
