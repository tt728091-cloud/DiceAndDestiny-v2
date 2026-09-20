class_name BattleCard
extends Button

var instance_id := ""
var definition_id := ""
var _income_glow: Panel
var _effect_plaque: PanelContainer
var _plaque_bottom := 16.0

func configure(instance: String, definition: String, enabled: bool, pending_removal: bool = false, compact: bool = false, removed: bool = false) -> void:
	pending_removal = pending_removal or removed
	instance_id = instance; definition_id = definition
	var data := BattlePresentationCatalog.card(definition)
	text = "%s  %d✦%s" % [data.name, int(data.cost), "\n× REMOVED" if removed else "\n⚔ PENDING" if pending_removal else ""]
	clip_text = true
	tooltip_text = "%s (%s) — %s" % [data.name, instance, data.text]
	custom_minimum_size = Vector2(132, 144) if pending_removal or compact else Vector2(185, 248)
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	for style_name in ["normal", "hover", "pressed", "disabled"]:
		var border := Color("a38b5c") if enabled else Color("696657")
		if style_name in ["hover", "pressed"]: border = Color("f4d48b")
		var style := cinematic.paper(Color.WHITE if enabled else Color("bcb5a8"))
		add_theme_stylebox_override(style_name, style)
		# Preserve native button text for accessibility and inspection. The visible
		# title/cost are independently laid out above the illustration.
		add_theme_color_override("font_" + style_name + "_color" if style_name != "normal" else "font_color", Color.TRANSPARENT)
	add_theme_color_override("font_hover_pressed_color", Color.TRANSPARENT)
	add_theme_color_override("font_focus_color", Color.TRANSPARENT)
	var illustration := TextureRect.new(); illustration.mouse_filter = Control.MOUSE_FILTER_IGNORE
	illustration.texture = cinematic.art(cinematic.card_art_index(definition))
	if illustration.texture == null and ResourceLoader.exists(str(data.art)): illustration.texture = load(str(data.art))
	illustration.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; illustration.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	add_child(illustration); illustration.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); illustration.offset_left = 5; illustration.offset_right = -5; illustration.offset_top = 43; illustration.offset_bottom = -9
	illustration.modulate = Color.WHITE if enabled else Color("a6aba8")
	var title := Label.new(); title.text = str(data.name); title.add_theme_font_size_override("font_size", 14 if pending_removal or compact else 16); title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; title.mouse_filter = Control.MOUSE_FILTER_IGNORE; title.add_theme_color_override("font_color", cinematic.INK); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	add_child(title); title.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE); title.offset_left = 9; title.offset_right = -32; title.offset_top = 3; title.offset_bottom = 41
	var cost := Label.new(); cost.text = str(int(data.cost)); cost.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; cost.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; cost.add_theme_font_size_override("font_size", 18); cost.add_theme_stylebox_override("normal", cinematic.panel(Color("204f67"), Color("b6a679"), 2)); cost.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(cost); cost.set_anchors_and_offsets_preset(Control.PRESET_TOP_RIGHT); cost.offset_left = -33; cost.offset_right = -5; cost.offset_top = 6; cost.offset_bottom = 34
	_build_effect_plaque(str(data.effect_summary), pending_removal, compact)
	var state := Label.new(); state.name = "RemovalState"; state.text = "× REMOVED" if removed else "× PENDING REMOVAL" if pending_removal else "✦ PLAY" if enabled else "—"; state.add_theme_font_size_override("font_size", 10 if pending_removal else 12); state.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; state.mouse_filter = Control.MOUSE_FILTER_IGNORE
	state.visible = pending_removal; state.add_theme_color_override("font_color", Color("ffb9a0")); state.add_theme_stylebox_override("normal", cinematic.panel(Color("291716e8"), Color.TRANSPARENT, 0))
	add_child(state); state.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE); state.offset_top = -25; state.offset_bottom = -4
	disabled = not enabled
	if pending_removal: modulate = Color("df9f88")


func _build_effect_plaque(summary: String, pending_removal: bool, compact: bool) -> void:
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	_plaque_bottom = 27.0 if pending_removal else 16.0
	_effect_plaque = PanelContainer.new(); _effect_plaque.name = "EffectPlaque"
	_effect_plaque.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var frame := cinematic.panel(Color("211e19"), Color("c6b38d"), 3)
	frame.set_corner_radius_all(6)
	frame.shadow_color = Color("171511cc"); frame.shadow_size = 2
	_effect_plaque.add_theme_stylebox_override("panel", frame)
	add_child(_effect_plaque)
	var paper := PanelContainer.new(); paper.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var parchment := cinematic.paper()
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]: parchment.set_content_margin(side, 5)
	paper.add_theme_stylebox_override("panel", parchment); _effect_plaque.add_child(paper)
	var label := Label.new(); label.name = "EffectSummary"; label.text = summary
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.add_theme_font_override("font", cinematic.roll_control_font())
	label.add_theme_font_size_override("font_size", 10 if compact or pending_removal else 14)
	label.add_theme_color_override("font_color", cinematic.INK)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE; paper.add_child(label)
	resized.connect(_layout_effect_plaque)
	_effect_plaque.minimum_size_changed.connect(_layout_effect_plaque, CONNECT_DEFERRED)
	call_deferred("_layout_effect_plaque")

func _layout_effect_plaque() -> void:
	if not is_instance_valid(_effect_plaque): return
	_effect_plaque.size.x = maxf(60.0, size.x - 24.0)
	_effect_plaque.size.y = _effect_plaque.get_combined_minimum_size().y
	_effect_plaque.position = Vector2(12, size.y - _plaque_bottom - _effect_plaque.size.y)

func prepare_income_draw() -> void:
	modulate = Color(1, 1, 1, 0)
	scale = Vector2(0.94, 0.94)
	_income_glow = Panel.new()
	_income_glow.set_anchors_preset(Control.PRESET_FULL_RECT)
	_income_glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var glow_style := StyleBoxFlat.new()
	glow_style.bg_color = Color("ffe28a20")
	glow_style.border_color = Color("fff0a8ff")
	glow_style.set_border_width_all(3)
	glow_style.set_corner_radius_all(7)
	glow_style.shadow_color = Color("ffd36a99")
	glow_style.shadow_size = 12
	_income_glow.add_theme_stylebox_override("panel", glow_style)
	add_child(_income_glow)

func animate_income_draw(duration_seconds: float) -> void:
	var duration := maxf(0.05, duration_seconds)
	pivot_offset = size * 0.5
	var settled_position := position
	position.y -= 18.0
	var tween := create_tween().bind_node(self).set_parallel(true)
	tween.tween_property(self, "modulate", Color.WHITE, duration * 0.22).set_delay(duration * 0.18).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "position", settled_position, duration * 0.28).set_delay(duration * 0.18).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", Vector2.ONE, duration * 0.28).set_delay(duration * 0.18).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	if is_instance_valid(_income_glow):
		_income_glow.modulate = Color.WHITE
		tween.tween_property(_income_glow, "modulate:a", 0.0, duration * 0.42).set_delay(duration * 0.48).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_IN)
