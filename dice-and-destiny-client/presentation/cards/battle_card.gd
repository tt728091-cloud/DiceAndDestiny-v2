class_name BattleCard
extends "res://presentation/battle/tooltip_button.gd"

const STANDARD_SIZE := Vector2(185, 248)
const COLORS := preload("res://presentation/cards/card_colors.gd")

var instance_id := ""
var definition_id := ""
var _income_glow: Panel
var _effect_plaque: PanelContainer
var _frame: Control
var _plaque_bottom := 14.0
var _summary_font_size := 14
var _ribbon_height := 0.0
var _enabled := true
var _frame_color: Color = COLORS.FRAMES[COLORS.DEFAULT_FRAME]
var _border_color: Color = COLORS.BORDERS[COLORS.DEFAULT_BORDER]
static var _speckle: Texture2D

const RIBBON_HEIGHT := 20.0
const SEGMENT_ICONS := {"offense": "attack", "defense": "block"}
const SEGMENT_NAMES := {"offense": "Offense", "defense": "Defense"}
const WHEN_WORDS := {"before": "Before roll", "after": "After roll", "any": "Any time"}
const WHEN_SHORT := {"before": "Before", "after": "After", "any": "Any"}
const IVORY := Color("fff6e0")
const TEXT_OUTLINE := Color("050404")
## Iron frame tinted toward the card's frame colour.
const IRON := Color("5d564e")

## Draw targeting feedback in card-local coordinates so fan rotation, hover
## lift, pivots and draw-animation scaling all follow the visible card edge.
func draw_targeting_outline(canvas: CanvasItem) -> void:
	canvas.draw_set_transform_matrix(canvas.get_global_transform_with_canvas().affine_inverse() * get_global_transform_with_canvas())
	var edge := _rounded_points(Rect2(Vector2.ZERO, size).grow(-1.5), _corner_radius()); edge.append(edge[0])
	canvas.draw_polyline(edge, Color("e2b2ff"), 3, true)
	canvas.draw_set_transform_matrix(Transform2D.IDENTITY)

func configure(instance: String, definition: String, enabled: bool, pending_removal: bool = false, compact: bool = false, removed: bool = false) -> void:
	pending_removal = pending_removal or removed
	var small := pending_removal or compact
	instance_id = instance; definition_id = definition; _enabled = enabled
	var data := BattlePresentationCatalog.card(definition)
	_frame_color = COLORS.frame(str(data.get("frame_color", ""))); _border_color = COLORS.border(str(data.get("border_color", "")))
	text = "%s  %d✦%s" % [data.name, int(data.cost), "\n× REMOVED" if removed else "\n⚔ PENDING" if pending_removal else ""]
	clip_text = true
	tooltip_text = BattlePresentationCatalog.card_tooltip(definition)
	custom_minimum_size = Vector2(132, 144) if small else STANDARD_SIZE
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	# The iron frame is drawn in _draw; the button boxes stay empty so no
	# rectangle shows past the rounded border.
	for style_name in ["normal", "hover", "pressed", "disabled"]:
		add_theme_stylebox_override(style_name, StyleBoxEmpty.new())
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
	var inset := _border_width(small) + _band_width(small)
	add_child(illustration); illustration.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); illustration.offset_left = inset; illustration.offset_right = -inset; illustration.offset_top = inset; illustration.offset_bottom = -inset
	illustration.modulate = Color.WHITE if enabled else Color("8f8f8c")
	# Fades, the art edge and the cost plate sit over the art, under the text.
	_frame = Control.new(); _frame.name = "CardFrame"; _frame.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_frame); _frame.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_frame.draw.connect(_draw_frame)
	var title := Label.new(); title.name = "CardTitle"; title.text = str(data.name); title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; title.mouse_filter = Control.MOUSE_FILTER_IGNORE
	title.add_theme_font_override("font", title_font()); title.add_theme_font_size_override("font_size", 12 if small else 15)
	_ivory_text(title, IVORY if enabled else Color("bdb6a8"), 5)
	add_child(title); title.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE); title.offset_left = 33 if small else 44; title.offset_right = -(8 if small else 12); title.offset_top = 6 if small else 11; title.offset_bottom = 30 if small else 37
	var cost := Label.new(); cost.name = "EnergyCost"; cost.text = str(int(data.cost)); cost.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; cost.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; cost.mouse_filter = Control.MOUSE_FILTER_IGNORE
	cost.add_theme_font_size_override("font_size", (13 if small else 15) if int(data.cost) >= 10 else (15 if small else 18))
	# Lining numerals centred on the hexagonal cost plate drawn by the frame.
	cost.add_theme_font_override("font", cinematic.roll_control_font())
	cost.add_theme_color_override("font_color", Color("ffd88a") if enabled else Color("cbbd9e"))
	cost.add_theme_color_override("font_outline_color", Color("2a0e04")); cost.add_theme_constant_override("outline_size", 4)
	cost.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	var center := Vector2(19, 19) if small else Vector2(25, 25); var half := 13.0 if small else 16.0
	add_child(cost); cost.set_anchors_and_offsets_preset(Control.PRESET_TOP_LEFT); cost.offset_left = center.x - half; cost.offset_right = center.x + half; cost.offset_top = center.y - half; cost.offset_bottom = center.y + half
	# Other views restyle the title and cost into header strips; follow them.
	cost.item_rect_changed.connect(_frame.queue_redraw)
	title.item_rect_changed.connect(_fit_title)
	_build_effect_plaque(str(data.effect_summary), pending_removal, compact, data.timing, str(data.play_limit))
	var state := Label.new(); state.name = "RemovalState"; state.text = "× REMOVED" if removed else "× PENDING REMOVAL" if pending_removal else "✦ PLAY" if enabled else "—"; state.add_theme_font_size_override("font_size", 10 if pending_removal else 12); state.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; state.mouse_filter = Control.MOUSE_FILTER_IGNORE
	state.visible = pending_removal; state.add_theme_color_override("font_color", Color("ffb9a0")); state.add_theme_stylebox_override("normal", cinematic.panel(Color("291716e8"), Color.TRANSPARENT, 0))
	add_child(state); state.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE); state.offset_top = -25; state.offset_bottom = -4
	state.offset_left = _border_width(small); state.offset_right = -_border_width(small)
	disabled = not enabled
	if pending_removal: modulate = Color("df9f88")
	call_deferred("_fit_title")

## Hand cards reserved by an incoming attack name their attacker beneath the
## title, high enough to stay readable while the hand is lowered.
func show_threat(source_name: String, detail: String) -> void:
	var ribbon := get_node_or_null("ThreatRibbon") as Button
	if ribbon == null:
		ribbon = Button.new(); ribbon.name = "ThreatRibbon"; ribbon.mouse_filter = Control.MOUSE_FILTER_IGNORE; ribbon.focus_mode = Control.FOCUS_NONE
		ribbon.icon = preload("res://presentation/battle/battle_icons.gd").texture("removed"); ribbon.expand_icon = true
		ribbon.add_theme_constant_override("icon_max_width", 15); ribbon.add_theme_constant_override("h_separation", 4)
		ribbon.add_theme_font_override("font", title_font()); ribbon.add_theme_font_size_override("font_size", 13)
		ribbon.clip_text = true; ribbon.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
		for state in ["normal", "hover", "pressed", "disabled", "focus"]:
			ribbon.add_theme_stylebox_override(state, preload("res://presentation/battle/cinematic_theme.gd").panel(Color("7a1712eb"), Color("e0806a"), 0))
		for state in ["font_color", "font_hover_color", "font_pressed_color", "font_disabled_color", "font_focus_color"]: ribbon.add_theme_color_override(state, Color("ffe1d6"))
		for state in ["icon_normal_color", "icon_hover_color", "icon_pressed_color", "icon_disabled_color", "icon_focus_color"]: ribbon.add_theme_color_override(state, Color("ffb9a0"))
		add_child(ribbon); ribbon.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
		var inset := _border_width(false) + _band_width(false)
		ribbon.offset_left = inset; ribbon.offset_right = -inset; ribbon.offset_top = 37; ribbon.offset_bottom = 57
	ribbon.text = source_name
	tooltip_text = BattlePresentationCatalog.card_tooltip(definition_id) + "\n\nPending removal · " + detail

static func title_font() -> SystemFont:
	var font := SystemFont.new()
	font.font_names = PackedStringArray(["Copperplate", "Cinzel", "Trajan Pro", "Georgia", "serif"])
	font.font_weight = 700
	return font

static func _ivory_text(label: Label, color: Color, outline: int) -> void:
	label.add_theme_color_override("font_color", color)
	label.add_theme_color_override("font_outline_color", TEXT_OUTLINE); label.add_theme_constant_override("outline_size", outline)
	label.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)

# Long names shrink to one line, then wrap onto two smaller lines.
func _fit_title() -> void:
	var title := get_node_or_null("CardTitle") as Label
	if title == null or title.autowrap_mode == TextServer.AUTOWRAP_OFF and title.text_overrun_behavior != TextServer.OVERRUN_NO_TRIMMING: return
	var font := title.get_theme_font("font")
	var largest := 12 if custom_minimum_size.x < STANDARD_SIZE.x else 15
	var width := title.size.x
	if width <= 1: return
	var point_size := largest
	while point_size > largest - 3 and font.get_string_size(title.text, HORIZONTAL_ALIGNMENT_LEFT, -1, point_size).x > width: point_size -= 1
	var fits := font.get_string_size(title.text, HORIZONTAL_ALIGNMENT_LEFT, -1, point_size).x <= width
	title.autowrap_mode = TextServer.AUTOWRAP_OFF if fits else TextServer.AUTOWRAP_WORD_SMART
	if not fits: point_size = largest - 4
	if title.get_theme_font_size("font_size") != point_size: title.add_theme_font_size_override("font_size", point_size)

func _build_effect_plaque(summary: String, pending_removal: bool, compact: bool, timing: Array = [], play_limit: String = "") -> void:
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	_plaque_bottom = 27.0 if pending_removal else 9.0 if compact else 14.0
	_effect_plaque = PanelContainer.new(); _effect_plaque.name = "EffectPlaque"
	_effect_plaque.mouse_filter = Control.MOUSE_FILTER_IGNORE
	# Text sits straight on the art's dark lower fade, as on a full-art card.
	_effect_plaque.add_theme_stylebox_override("panel", StyleBoxEmpty.new())
	add_child(_effect_plaque)
	var label := Label.new(); label.name = "EffectSummary"; label.text = summary
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.add_theme_font_override("font", cinematic.roll_control_font())
	_summary_font_size = 10 if compact or pending_removal else 14
	label.add_theme_font_size_override("font_size", _summary_font_size)
	_ivory_text(label, IVORY if _enabled else Color("d6cebd"), 3 if compact or pending_removal else 4)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var column := VBoxContainer.new(); column.name = "EffectColumn"; column.mouse_filter = Control.MOUSE_FILTER_IGNORE
	column.add_theme_constant_override("separation", 3)
	_effect_plaque.add_child(column); column.add_child(label)
	# Small cards keep only the effect; the hover still states the timing.
	_ribbon_height = 0.0
	if not compact and not pending_removal and not timing.is_empty():
		column.add_child(_timing_ribbon(timing, play_limit))
		_ribbon_height = RIBBON_HEIGHT + 3
	resized.connect(_layout_effect_plaque)
	_effect_plaque.minimum_size_changed.connect(_layout_effect_plaque, CONNECT_DEFERRED)
	_effect_plaque.item_rect_changed.connect(func(): if is_instance_valid(_frame): _frame.queue_redraw())
	call_deferred("_layout_effect_plaque")

func _layout_effect_plaque() -> void:
	if not is_instance_valid(_effect_plaque): return
	_effect_plaque.size.x = maxf(60.0, size.x - 2.0 * _plaque_inset())
	var summary := _effect_plaque.find_child("EffectSummary", true, false) as Label
	# Measure at the final card width, not a provisional Container width. A
	# long compact summary may shrink; normal hand cards retain their font.
	if size.x >= custom_minimum_size.x and size.y >= custom_minimum_size.y:
		var font := summary.get_theme_font("font")
		var point_size := _summary_font_size
		var available := maxf(1, size.y - _plaque_bottom - 43 - 6 - _ribbon_height)
		while point_size > 8 and _summary_height(summary, font, _effect_plaque.size.x, point_size) > available:
			point_size -= 1
		if summary.get_theme_font_size("font_size") != point_size: summary.add_theme_font_size_override("font_size", point_size)
	_effect_plaque.size.y = _effect_plaque.get_combined_minimum_size().y
	_effect_plaque.position = Vector2(_plaque_inset(), size.y - _plaque_bottom - _effect_plaque.size.y)

func _plaque_inset() -> float:
	return 13.0 if custom_minimum_size.x < STANDARD_SIZE.x else 18.0

func _summary_height(label: Label, font: Font, width: float, point_size: int) -> float:
	var measured := font.get_multiline_string_size(label.text, HORIZONTAL_ALIGNMENT_CENTER, width, point_size)
	var lines := ceili(measured.y / font.get_height(point_size))
	return measured.y + maxi(0, lines - 1) * label.get_theme_constant("line_spacing")

## The timing line replaces the "Play: …" sentence: a thin rule in the frame
## colour, then a segment icon (sword for Offense, shield for Defense) and
## when in that turn the card plays.
func _timing_ribbon(timing: Array, play_limit: String) -> MarginContainer:
	var ribbon := MarginContainer.new(); ribbon.name = "TimingRibbon"; ribbon.mouse_filter = Control.MOUSE_FILTER_IGNORE
	ribbon.custom_minimum_size.y = RIBBON_HEIGHT
	ribbon.add_theme_constant_override("margin_left", 16); ribbon.add_theme_constant_override("margin_right", 16)
	var rule := PanelContainer.new(); rule.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var line := StyleBoxFlat.new(); line.bg_color = Color.TRANSPARENT
	line.border_color = Color(_frame_color.lightened(0.15), 0.75 if _enabled else 0.4); line.border_width_top = 1
	line.content_margin_top = 4
	rule.add_theme_stylebox_override("panel", line); ribbon.add_child(rule)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	row.add_theme_constant_override("separation", 3)
	rule.add_child(row)
	var groups := timing_groups(timing)
	for index in groups.size():
		if index > 0: row.add_child(_ribbon_label("·"))
		for segment in groups[index].segments:
			var icon := TextureRect.new(); icon.name = "%sIcon" % SEGMENT_NAMES.get(segment, segment)
			icon.texture = preload("res://presentation/battle/battle_icons.gd").texture(SEGMENT_ICONS.get(segment, "dice"))
			icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
			icon.custom_minimum_size = Vector2(15, 15); icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
			row.add_child(icon)
		row.add_child(_ribbon_label(groups[index].label))
	if not play_limit.is_empty(): row.add_child(_ribbon_label("· " + play_limit))
	return ribbon

func _ribbon_label(value: String) -> Label:
	var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
	var label := Label.new(); label.text = value; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_override("font", cinematic.roll_control_font())
	label.add_theme_font_size_override("font_size", 12)
	_ivory_text(label, Color("f1e3c2") if _enabled else Color("cfc6b4"), 3)
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

# ---------------------------------------------------------------- iron frame
# A solid outer border, a speckled iron band tinted toward the card's frame
# colour, full art inside, dark fades behind the title and rules, and the
# energy cost on a hexagonal iron plate. Every colour derives from the card's
# frame_color and border_color (see card_colors.gd).

func _small() -> bool:
	return custom_minimum_size.x < STANDARD_SIZE.x

func _border_width(small: bool) -> float:
	return 5.0 if small else 7.0

func _band_width(small: bool) -> float:
	return 4.0 if small else 5.0

func _corner_radius() -> float:
	return minf(10.0, minf(size.x, size.y) * 0.5)

func _metal() -> Color:
	var metal := IRON.lerp(_frame_color, 0.55)
	return metal if _enabled else metal.lerp(Color("4a4846"), 0.6)

func _art_rect() -> Rect2:
	return Rect2(Vector2.ZERO, size).grow(-(_border_width(_small()) + _band_width(_small())))

## Mottled grey with dark and light flecks; multiplied by the metal colour it
## gives the iron band its texture.
static func speckle() -> Texture2D:
	if _speckle != null: return _speckle
	var noise := FastNoiseLite.new(); noise.seed = 11; noise.frequency = 0.045; noise.fractal_octaves = 3
	var rng := RandomNumberGenerator.new(); rng.seed = 23
	var image := Image.create(256, 256, false, Image.FORMAT_RGBA8)
	for y in 256:
		for x in 256:
			var value := 0.88 + noise.get_noise_2d(x, y) * 0.14
			var roll := rng.randf()
			if roll < 0.03: value -= rng.randf_range(0.18, 0.35)
			elif roll < 0.055: value += 0.1
			value = clampf(value, 0.0, 1.0)
			image.set_pixel(x, y, Color(value, value, value))
	_speckle = ImageTexture.create_from_image(image)
	return _speckle

static func _rounded_points(rect: Rect2, radius: float, steps: int = 6) -> PackedVector2Array:
	radius = clampf(radius, 0.0, minf(rect.size.x, rect.size.y) * 0.5)
	var points := PackedVector2Array()
	var centers := [rect.position + Vector2(radius, radius), Vector2(rect.end.x - radius, rect.position.y + radius), rect.end - Vector2(radius, radius), Vector2(rect.position.x + radius, rect.end.y - radius)]
	for i in 4:
		for s in steps + 1:
			points.append(centers[i] + Vector2.from_angle(PI + i * PI / 2.0 + s * PI / 2.0 / steps) * radius)
	return points

static func _rect_points(rect: Rect2) -> PackedVector2Array:
	return PackedVector2Array([rect.position, Vector2(rect.end.x, rect.position.y), rect.end, Vector2(rect.position.x, rect.end.y)])

func _draw() -> void:
	if size.x < 20 or size.y < 16: return
	var body := _rounded_points(Rect2(Vector2.ZERO, size), _corner_radius())
	draw_colored_polygon(Transform2D(0, Vector2(0, 3)) * body, Color(0, 0, 0, 0.5))
	draw_colored_polygon(body, _border_color)
	var edge := body.duplicate(); edge.append(body[0])
	draw_polyline(edge, _border_color.darkened(0.3) if _border_color.get_luminance() > 0.5 else _border_color.lightened(0.18), 1.0, true)
	var band := Rect2(Vector2.ZERO, size).grow(-_border_width(_small()))
	if band.size.x < 4 or band.size.y < 4: return
	var plate := _rounded_points(band, 4)
	var uvs := PackedVector2Array()
	for point in plate: uvs.append(point / 256.0)
	draw_polygon(plate, PackedColorArray([_metal()]), uvs, speckle())
	plate.append(plate[0])
	draw_polyline(plate, Color(_metal().lightened(0.45), 0.7), 1, true)

func _draw_frame() -> void:
	var art := _art_rect()
	if art.size.x > 8 and art.size.y > 24:
		var shade := Color(0.02, 0.02, 0.03)
		var title_fade := minf(46.0, art.size.y * 0.3)
		_gradient(Rect2(art.position, Vector2(art.size.x, title_fade)), Color(shade, 0.75), Color(shade, 0))
		# The rules fade follows the plaque, however tall its text grows.
		var text_top := art.end.y - 40.0
		if is_instance_valid(_effect_plaque) and _effect_plaque.visible: text_top = clampf(_effect_plaque.position.y - 6.0, art.position.y + title_fade, art.end.y)
		var fade := minf(46.0, text_top - art.position.y - title_fade)
		if fade > 0: _gradient(Rect2(art.position.x, text_top - fade, art.size.x, fade), Color(shade, 0), Color(shade, 0.84))
		if art.end.y > text_top: _gradient(Rect2(art.position.x, text_top, art.size.x, art.end.y - text_top), Color(shade, 0.84), Color(shade, 0.9))
		var keyline := _rect_points(art.grow(1)); keyline.append(keyline[0])
		_frame.draw_polyline(keyline, _metal().darkened(0.5), 1, true)
	_draw_cost_plate()

func _gradient(rect: Rect2, top: Color, bottom: Color) -> void:
	_frame.draw_polygon(_rect_points(rect), PackedColorArray([top, top, bottom, bottom]))

# The hexagonal iron plate behind the energy cost, centred on its label.
func _draw_cost_plate() -> void:
	var cost := get_node_or_null("EnergyCost") as Control
	if cost == null or not cost.visible: return
	var center := cost.position + cost.size * 0.5
	var radius := minf(cost.size.x, cost.size.y) * 0.48
	var hexagon := PackedVector2Array()
	for i in 6: hexagon.append(center + Vector2.from_angle(PI / 6.0 + i * PI / 3.0) * radius)
	_frame.draw_colored_polygon(Transform2D(0, Vector2(0, 2)) * hexagon, Color(0, 0, 0, 0.6))
	_frame.draw_colored_polygon(hexagon, Color("2a2420"))
	var ring := hexagon.duplicate(); ring.append(hexagon[0])
	_frame.draw_polyline(ring, Color("9a9184") if _enabled else Color("77726b"), 1.6, true)
	if _enabled:
		for ratio in [0.71, 0.52, 0.32]: _frame.draw_circle(center, radius * ratio, Color(1.0, 0.45, 0.1, 0.12))

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
	glow_style.set_corner_radius_all(10)
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
