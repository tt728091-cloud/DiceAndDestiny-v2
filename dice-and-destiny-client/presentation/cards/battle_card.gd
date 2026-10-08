class_name BattleCard
extends "res://presentation/battle/tooltip_button.gd"

const STANDARD_SIZE := Vector2(185, 248)

var instance_id := ""
var definition_id := ""
var _income_glow: Panel
var _effect_plaque: PanelContainer
var _plaque_bottom := 16.0
var _summary_font_size := 14
var _ribbon_height := 0.0

const RIBBON_HEIGHT := 20.0
const SEGMENT_ICONS := {"offense": "attack", "defense": "block"}
const SEGMENT_NAMES := {"offense": "Offense", "defense": "Defense"}
const SEGMENT_TINTS := {"offense": Color("9a3b2240"), "defense": Color("2b5f9a40"), "both": Color("5b4a3040")}
const WHEN_WORDS := {"before": "Before roll", "after": "After roll", "any": "Any time"}
const WHEN_SHORT := {"before": "Before", "after": "After", "any": "Any"}

## Draw targeting feedback in card-local coordinates so fan rotation, hover
## lift, pivots and draw-animation scaling all follow the visible paper edge.
func draw_targeting_outline(canvas: CanvasItem) -> void:
	canvas.draw_set_transform_matrix(canvas.get_global_transform_with_canvas().affine_inverse() * get_global_transform_with_canvas())
	canvas.draw_rect(Rect2(Vector2.ZERO, size).grow(-2), Color("e2b2ff"), false, 3, true)
	canvas.draw_set_transform_matrix(Transform2D.IDENTITY)

func configure(instance: String, definition: String, enabled: bool, pending_removal: bool = false, compact: bool = false, removed: bool = false) -> void:
	pending_removal = pending_removal or removed
	instance_id = instance; definition_id = definition
	var data := BattlePresentationCatalog.card(definition)
	text = "%s  %d✦%s" % [data.name, int(data.cost), "\n× REMOVED" if removed else "\n⚔ PENDING" if pending_removal else ""]
	clip_text = true
	tooltip_text = BattlePresentationCatalog.card_tooltip(definition)
	custom_minimum_size = Vector2(132, 144) if pending_removal or compact else STANDARD_SIZE
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
	illustration.texture = preload("res://assets/battle/fighters/curse.png") if data.targeting.get("selector") == "curse_choice" else cinematic.art(cinematic.card_art_index(definition))
	if not str(data.illustration_path).is_empty() and ResourceLoader.exists(str(data.illustration_path)): illustration.texture = load(str(data.illustration_path))
	if illustration.texture == null and ResourceLoader.exists(str(data.art)): illustration.texture = load(str(data.art))
	illustration.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; illustration.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	add_child(illustration); illustration.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); illustration.offset_left = 5; illustration.offset_right = -5; illustration.offset_top = 43; illustration.offset_bottom = -9
	illustration.modulate = Color.WHITE if enabled else Color("a6aba8")
	var title := Label.new(); title.name = "CardTitle"; title.text = str(data.name); title.add_theme_font_size_override("font_size", 14 if pending_removal or compact else 16); title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; title.mouse_filter = Control.MOUSE_FILTER_IGNORE; title.add_theme_color_override("font_color", cinematic.INK); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	add_child(title); title.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE); title.offset_left = 39; title.offset_right = -7; title.offset_top = 3; title.offset_bottom = 41
	var cost := Label.new(); cost.name = "EnergyCost"; cost.text = str(int(data.cost)); cost.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; cost.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; cost.add_theme_font_size_override("font_size", 18); cost.add_theme_stylebox_override("normal", cinematic.panel(Color("204f67"), Color("b6a679"), 2)); cost.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(cost); cost.set_anchors_and_offsets_preset(Control.PRESET_TOP_LEFT); cost.offset_left = 5; cost.offset_right = 33; cost.offset_top = 6; cost.offset_bottom = 34
	_build_effect_plaque(str(data.effect_summary), pending_removal, compact, data.timing, str(data.play_limit))
	var state := Label.new(); state.name = "RemovalState"; state.text = "× REMOVED" if removed else "× PENDING REMOVAL" if pending_removal else "✦ PLAY" if enabled else "—"; state.add_theme_font_size_override("font_size", 10 if pending_removal else 12); state.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; state.mouse_filter = Control.MOUSE_FILTER_IGNORE
	state.visible = pending_removal; state.add_theme_color_override("font_color", Color("ffb9a0")); state.add_theme_stylebox_override("normal", cinematic.panel(Color("291716e8"), Color.TRANSPARENT, 0))
	add_child(state); state.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE); state.offset_top = -25; state.offset_bottom = -4
	disabled = not enabled
	if pending_removal: modulate = Color("df9f88")


func _build_effect_plaque(summary: String, pending_removal: bool, compact: bool, timing: Array = [], play_limit: String = "") -> void:
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
	_summary_font_size = 10 if compact or pending_removal else 14
	label.add_theme_font_size_override("font_size", _summary_font_size)
	label.add_theme_color_override("font_color", cinematic.INK)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var column := VBoxContainer.new(); column.name = "EffectColumn"; column.mouse_filter = Control.MOUSE_FILTER_IGNORE
	column.add_theme_constant_override("separation", 3)
	paper.add_child(column); column.add_child(label)
	# Small cards keep only the effect; the hover still states the timing.
	_ribbon_height = 0.0
	if not compact and not pending_removal and not timing.is_empty():
		column.add_child(_timing_ribbon(timing, play_limit))
		_ribbon_height = RIBBON_HEIGHT + 3
	resized.connect(_layout_effect_plaque)
	_effect_plaque.minimum_size_changed.connect(_layout_effect_plaque, CONNECT_DEFERRED)
	call_deferred("_layout_effect_plaque")

func _layout_effect_plaque() -> void:
	if not is_instance_valid(_effect_plaque): return
	_effect_plaque.size.x = maxf(60.0, size.x - 24.0)
	var summary := _effect_plaque.find_child("EffectSummary", true, false) as Label
	# Measure at the final card width, not a provisional Container width. A
	# long compact summary may shrink; normal hand cards retain their font.
	if size.x >= custom_minimum_size.x and size.y >= custom_minimum_size.y:
		var font := summary.get_theme_font("font")
		var point_size := _summary_font_size
		var available := maxf(1, size.y - _plaque_bottom - 43 - 16 - _ribbon_height)
		while point_size > 8 and _summary_height(summary, font, size.x - 40, point_size) > available:
			point_size -= 1
		if summary.get_theme_font_size("font_size") != point_size: summary.add_theme_font_size_override("font_size", point_size)
	_effect_plaque.size.y = _effect_plaque.get_combined_minimum_size().y
	_effect_plaque.position = Vector2(12, size.y - _plaque_bottom - _effect_plaque.size.y)

func _summary_height(label: Label, font: Font, width: float, point_size: int) -> float:
	var measured := font.get_multiline_string_size(label.text, HORIZONTAL_ALIGNMENT_CENTER, width, point_size)
	var lines := ceili(measured.y / font.get_height(point_size))
	return measured.y + maxi(0, lines - 1) * label.get_theme_constant("line_spacing")

## The timing ribbon replaces the "Play: …" sentence: a segment icon (sword
## for Offense, shield for Defense) and when in that turn the card plays.
func _timing_ribbon(timing: Array, play_limit: String) -> PanelContainer:
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	var ribbon := PanelContainer.new(); ribbon.name = "TimingRibbon"; ribbon.mouse_filter = Control.MOUSE_FILTER_IGNORE
	ribbon.custom_minimum_size.y = RIBBON_HEIGHT
	# Tinted by segment so Offense and Defense cards read apart at a glance.
	var segments := timing.map(func(tag): return str(tag.get("segment", "")))
	var tint: Color = SEGMENT_TINTS.get(segments[0], SEGMENT_TINTS.both) if segments.count(segments[0]) == segments.size() else SEGMENT_TINTS.both
	var band := StyleBoxFlat.new(); band.bg_color = tint; band.set_corner_radius_all(3)
	band.content_margin_left = 4; band.content_margin_right = 4; band.content_margin_top = 1; band.content_margin_bottom = 1
	ribbon.add_theme_stylebox_override("panel", band)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_theme_constant_override("separation", 3)
	ribbon.add_child(row)
	var groups := timing_groups(timing)
	for index in groups.size():
		if index > 0: row.add_child(_ribbon_label("·"))
		for segment in groups[index].segments:
			var icon := TextureRect.new(); icon.name = "%sIcon" % SEGMENT_NAMES.get(segment, segment)
			icon.texture = preload("res://presentation/battle/battle_icons.gd").texture(SEGMENT_ICONS.get(segment, "dice"))
			icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
			icon.custom_minimum_size = Vector2(16, 16); icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
			row.add_child(icon)
		row.add_child(_ribbon_label(groups[index].label))
	if not play_limit.is_empty(): row.add_child(_ribbon_label("· " + play_limit))
	return ribbon

func _ribbon_label(value: String) -> Label:
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	var label := Label.new(); label.text = value; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_override("font", cinematic.roll_control_font())
	label.add_theme_font_size_override("font_size", 12)
	label.add_theme_color_override("font_color", cinematic.INK)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	return label

## Groups timing tags for the ribbon. Segments with the same timing share one
## label ("⚔🛡 Any time"); different timings use short labels side by side.
static func timing_groups(timing: Array) -> Array[Dictionary]:
	var groups: Array[Dictionary] = []
	var full := timing.size() == 1 or timing.all(func(tag): return _timing_label(tag, false) == _timing_label(timing[0], false))
	for tag in timing:
		var label := _timing_label(tag, not full)
		if full and not groups.is_empty():
			groups[0].segments.append(str(tag.get("segment", "")))
			continue
		groups.append({"segments": [str(tag.get("segment", ""))], "label": label})
	return groups

static func _timing_label(tag: Dictionary, short: bool) -> String:
	var when := str(tag.get("when", ""))
	var reaction := bool(tag.get("reaction", false))
	if when.is_empty(): return "Reaction"
	var words := str((WHEN_SHORT if short else WHEN_WORDS).get(when, when))
	if reaction: words += " + React" if short else " / Reaction"
	return words

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

func _make_custom_tooltip(for_text: String) -> Object:
	return preload("res://presentation/cards/card_rules_tooltip.gd").create(self, for_text)
