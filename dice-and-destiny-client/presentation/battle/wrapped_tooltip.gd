extends RefCounted
## Shared text-only hover content. Godot keeps its popup inside the viewport;
## bound the content first so that clamping can work at either screen edge.
const MAX_WIDTH := 440.0
const VIEWPORT_MARGIN := 64.0

static func content(value: String) -> String:
	# Suppress blank content before Godot starts a popup or falls back to its
	# native tooltip after a custom builder returns null.
	return "" if value.strip_edges().is_empty() else value

static func create(owner: Control, value: String) -> Label:
	# Godot calls custom tooltip builders even without tooltip text. Returning
	# an empty Label still creates a visible popup frame.
	if content(value).is_empty(): return null
	var label := Label.new()
	label.theme_type_variation = &"TooltipLabel"
	label.text = value
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	var font := owner.get_theme_font("font", "TooltipLabel")
	var font_size := owner.get_theme_font_size("font_size", "TooltipLabel")
	var natural_width := font.get_multiline_string_size(value, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x
	label.custom_minimum_size.x = maxf(1, minf(natural_width, minf(MAX_WIDTH, owner.get_viewport_rect().size.x - VIEWPORT_MARGIN)))
	# Size the wrapped text before Godot positions the popup. Otherwise its
	# initial single-line height can place the later-expanded label offscreen.
	label.add_theme_font_override("font", font)
	label.add_theme_font_size_override("font_size", font_size)
	label.size.x = label.custom_minimum_size.x
	var paragraph := TextParagraph.new()
	paragraph.add_string(value, font, font_size)
	paragraph.width = label.custom_minimum_size.x
	paragraph.break_flags = TextServer.BREAK_MANDATORY | TextServer.BREAK_WORD_BOUND | TextServer.BREAK_ADAPTIVE
	var spacing := owner.get_theme_constant("line_spacing", "TooltipLabel")
	label.add_theme_constant_override("line_spacing", spacing)
	label.custom_minimum_size.y = ceilf(paragraph.get_size().y + maxi(0, paragraph.get_line_count() - 1) * spacing)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return label
