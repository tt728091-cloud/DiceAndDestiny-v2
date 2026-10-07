extends "res://presentation/battle/tooltip_button.gd"
## Shared, wrapped tooltip for both the overhead intent and its fighter target.
## Godot positions the popup within the viewport, including right-edge enemies.
func _make_custom_tooltip(for_text: String) -> Object:
	for_text = preload("res://presentation/battle/wrapped_tooltip.gd").content(for_text)
	if for_text.is_empty(): return null
	var panel := PanelContainer.new()
	panel.name = "AttackAbilityTooltip"
	panel.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var ink := preload("res://presentation/battle/cinematic_theme.gd")
	var style := ink.panel(Color("10191ffb"), Color("b8a779"), 5)
	style.content_margin_left = 18; style.content_margin_right = 18
	style.content_margin_top = 14; style.content_margin_bottom = 14
	panel.add_theme_stylebox_override("panel", style)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 10); panel.add_child(body)
	var width := minf(440, get_viewport_rect().size.x - 72)
	var title := Label.new(); title.text = for_text.get_slice("\n", 0)
	title.custom_minimum_size.x = width; title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	title.add_theme_font_size_override("font_size", 22); title.add_theme_color_override("font_color", Color("ffe3a0")); body.add_child(title)
	var detail := Label.new(); detail.text = for_text.substr(for_text.find("\n") + 1)
	detail.custom_minimum_size.x = width; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	detail.add_theme_font_size_override("font_size", 18); detail.add_theme_color_override("font_color", Color("f4ecda")); body.add_child(detail)
	return panel
