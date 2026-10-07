class_name BattleDiceTray
extends VBoxContainer

signal selection_changed(indices: Array)
var selected: Array = []
var compact_row := false
# Render at native small dimensions; curse numbers stay independently sized.
var hud_compact := false
var dice_columns := 5
const HUD_DIE_SIZE := Vector2(56, 60)
const HUD_ROW_WIDTH := 304.0
var _buttons: Array[Button] = []
var _caption: Label
var _faces: Array[TextureRect] = []
var _numbers: Array[Label] = []
var _normal_symbols: Array[Label] = []
var _curse_symbols: Array[Label] = []
var _mark_labels: Array[Label] = []
var _mark_panels: Array[PanelContainer] = []
var _mark_faces: Array = []
var _bound_labels: Array[Label] = []
var _bindings: Array[Control] = []
var _mark_legend: Label
var _dice: Array = []
var _owned: Array = []
var _pending_curse_faces: Dictionary = {}
var _interactive := false
var _rolling_indices: Array = []
var _roll_started_ms := 0
var _roll_duration_ms := 0

func _ready() -> void:
	_caption = Label.new(); _caption.add_theme_font_size_override("font_size", 14); _caption.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; add_child(_caption)
	var row := GridContainer.new(); row.columns = dice_columns; row.add_theme_constant_override("h_separation", 6); row.add_theme_constant_override("v_separation", 6); add_child(row)
	for index in 5:
		var button := Button.new(); button.custom_minimum_size = HUD_DIE_SIZE if hud_compact else Vector2(70, 76); button.add_theme_font_size_override("font_size", 18 if hud_compact else 23); button.toggle_mode = true; button.text = "—"; button.tooltip_text = "Ready die %d" % (index + 1)
		_style_die(button)
		button.toggled.connect(_toggle.bind(index))
		var slot := VBoxContainer.new(); row.add_child(slot); slot.add_child(button); _buttons.append(button)
		var bound := Label.new(); bound.name = "Entombed%d" % (index + 1); bound.text = "BOUND" if hud_compact else "ENTOMBED"
		bound.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		bound.add_theme_font_size_override("font_size", 9 if hud_compact else 12); bound.add_theme_color_override("font_color", Color("251b14"))
		bound.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		var badge := StyleBoxFlat.new(); badge.bg_color = Color("edc27d"); badge.border_color = Color("edc27d"); badge.set_border_width_all(1); badge.set_corner_radius_all(3)
		bound.add_theme_stylebox_override("normal", badge)
		bound.hide(); slot.add_child(bound); _bound_labels.append(bound)
		_build_mark_map(slot, index)
		var icon := TextureRect.new(); icon.mouse_filter = Control.MOUSE_FILTER_IGNORE; icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
		button.add_child(icon); icon.position = Vector2(19, 7); icon.size = Vector2(32, 32); _faces.append(icon)
		var symbol := Label.new(); symbol.name = "NormalSymbol"; symbol.mouse_filter = Control.MOUSE_FILTER_IGNORE
		symbol.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; symbol.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		symbol.add_theme_font_size_override("font_size", 20 if hud_compact else 23); symbol.add_theme_color_override("font_color", Color("efe5cc")); symbol.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		button.add_child(symbol); _normal_symbols.append(symbol)
		var curse := Label.new(); curse.name = "CurseSymbol"; curse.mouse_filter = Control.MOUSE_FILTER_IGNORE
		curse.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; curse.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		curse.position = Vector2(31, 3) if hud_compact else Vector2(39, 9); curse.size = Vector2(21, 26) if hud_compact else Vector2(24, 28)
		curse.add_theme_font_size_override("font_size", 20 if hud_compact else 23); curse.add_theme_color_override("font_color", Color("e2b2ff")); curse.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		button.add_child(curse); _curse_symbols.append(curse)
		var number := Label.new(); number.mouse_filter = Control.MOUSE_FILTER_IGNORE; number.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; number.add_theme_color_override("font_color", Color("efe5cc")); number.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		number.add_theme_font_size_override("font_size", 20 if hud_compact else 23)
		button.add_child(number); number.position = Vector2(0, 31) if hud_compact else Vector2(0, 42); number.size = Vector2(56, 24) if hud_compact else Vector2(70, 25); _numbers.append(number)
		var binding := preload("res://presentation/dice/entomb_binding.gd").new()
		binding.name = "EntombBinding"; button.add_child(binding); binding.hide(); _bindings.append(binding)

	_mark_legend = Label.new(); _mark_legend.text = "CURSED FACES · highlighted numbers"
	_mark_legend.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_mark_legend.add_theme_font_size_override("font_size", 13)
	_mark_legend.add_theme_color_override("font_color", Color("e2c6fa"))
	_mark_legend.hide(); add_child(_mark_legend)

func _build_mark_map(slot: VBoxContainer, index: int) -> void:
	var panel := PanelContainer.new(); panel.name = "CurseMap%d" % (index + 1)
	panel.mouse_filter = Control.MOUSE_FILTER_PASS
	var style := StyleBoxFlat.new(); style.bg_color = Color("19161deb"); style.set_corner_radius_all(4)
	style.content_margin_left = 2 if hud_compact else 3; style.content_margin_right = style.content_margin_left; style.content_margin_top = 2; style.content_margin_bottom = 2
	panel.add_theme_stylebox_override("panel", style); slot.add_child(panel); _mark_panels.append(panel)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 3); panel.add_child(body)
	var count := Label.new(); count.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	count.add_theme_font_size_override("font_size", 10 if hud_compact else 12); count.add_theme_color_override("font_color", Color("f4e6ff"))
	count.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	count.mouse_filter = Control.MOUSE_FILTER_IGNORE; body.add_child(count); _mark_labels.append(count)
	var grid := GridContainer.new(); grid.name = "Faces"; grid.columns = 3
	grid.add_theme_constant_override("h_separation", 2); grid.add_theme_constant_override("v_separation", 2)
	body.add_child(grid)
	var faces: Array[Label] = []
	for face in range(1, 7):
		var chip := Label.new(); chip.text = str(face); chip.name = "Face%d" % face
		chip.custom_minimum_size = Vector2(10, 16) if hud_compact else Vector2(20, 20); chip.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		chip.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; chip.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		chip.add_theme_font_size_override("font_size", 14 if hud_compact else 15); chip.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		chip.mouse_filter = Control.MOUSE_FILTER_PASS; grid.add_child(chip); faces.append(chip)
	_mark_faces.append(faces)

	panel.hide()

func display(dice: Array, kept: Array = [], interactive: bool = false, caption: String = "Dice", inspection_prefix: String = "") -> void:
	_dice = dice.duplicate(true)
	_interactive = interactive
	_rolling_indices.clear()
	selected.clear()
	for value in kept: selected.append(int(value))
	selected.sort()
	_caption.text = caption
	_caption.visible = not caption.is_empty()
	for index in _buttons.size():
		var button := _buttons[index]
		_show_face(index, int(dice[index].get("face", dice[index].get("number", 0))) if index < dice.size() else 0)
		button.rotation = 0.0
		button.disabled = not interactive or index >= dice.size()
		button.set_pressed_no_signal(index in selected)
		if not inspection_prefix.is_empty():
			var inspector = get_node_or_null("/root/GameInspector")
			if inspector != null: inspector.register_control("%s.%d" % [inspection_prefix, index], button, button.tooltip_text)

# The screen owns the timestamp, so a utility redraw resumes the same roll.
func animate_roll(indices: Array, started_ms: int, duration_ms: int) -> void:
	_rolling_indices = indices.duplicate()
	_roll_started_ms = started_ms
	_roll_duration_ms = duration_ms
	_update_roll()

func _process(_delta: float) -> void:
	if not _rolling_indices.is_empty(): _update_roll()

# Card feedback supplies its own elapsed time so pauses and board rebuilds
# resume the same physical reroll without modifying the authoritative dice.
func present_forced_roll(index: int, before: int, elapsed: float, seconds: float) -> void:
	if index < 0 or index >= _dice.size(): return
	var rolling := elapsed >= 0.0 and elapsed < seconds
	var face := int(_dice[index].get("face", 0))
	if elapsed < 0.0: face = before
	elif rolling: face = 1 + (int(elapsed / 0.055) + index * 2) % 6
	_show_face(index, face)
	var button := _buttons[index]
	button.pivot_offset = button.size * 0.5
	button.rotation = sin(elapsed * 35.0 + index) * 0.055 if rolling else 0.0
	if rolling: button.tooltip_text = "Forced reroll · die %d" % (index + 1)

func _update_roll() -> void:
	var elapsed := maxi(0, Time.get_ticks_msec() - _roll_started_ms)
	var rolling := elapsed < _roll_duration_ms
	for index in _buttons.size():
		var button := _buttons[index]
		button.add_theme_stylebox_override("disabled", button.get_theme_stylebox("pressed" if index in selected else "normal"))
		button.disabled = rolling or not _interactive or index >= _dice.size() or (index < _owned.size() and bool(_owned[index].get("entombed", false)))
		if index not in _rolling_indices or index >= _dice.size(): continue
		var final_face := int(_dice[index].get("face", _dice[index].get("number", 0)))
		_show_face(index, 1 + (elapsed / 55 + index * 2) % 6 if rolling else final_face)
		button.pivot_offset = button.size * 0.5
		button.rotation = sin(float(elapsed) * 0.035 + index) * 0.055 if rolling else 0.0
		if rolling: button.tooltip_text = "Rolling…"
	if not rolling:
		_rolling_indices.clear()
		display_owned(_owned)

func _face_is_cursed(index: int, face: int) -> bool:
	if face <= 0 or index >= _owned.size() or _pending_curse_faces.has(Vector2i(index, face)): return false
	var marks = _owned[index].get("cursed_faces", [])
	if not marks is Array: return false
	# Authority JSON uses floating-point numbers; compare their integer faces.
	return marks.any(func(value): return int(value) == face)

func _show_face(index: int, face: int) -> void:
	var button := _buttons[index]
	var die_id := str(_dice[index].get("die_id", "standard_d6")) if index < _dice.size() else "standard_d6"
	var symbol := BattlePresentationCatalog.symbol_for_die_face(die_id, face)
	var cursed := _face_is_cursed(index, face)
	var curse_glyph := str(BattlePresentationCatalog.status("curse_count").glyph)
	button.text = "%s%s\n%d" % [symbol, " " + curse_glyph if cursed else "", face] if face > 0 else "—"
	button.tooltip_text = "Die %d: face %d, %s%s" % [index + 1, face, BattlePresentationCatalog.symbol_name_for_die_face(die_id, face), " (kept)" if index in selected else ""] if face > 0 else "Ready die %d" % (index + 1)
	if cursed: button.tooltip_text += "\nCurse symbol · physical rolls of this face add 1 Curse Count."
	var illustrated := face > 0 and die_id == "venom_d6"
	_faces[index].visible = illustrated
	_numbers[index].visible = face > 0
	_numbers[index].text = str(face)
	_normal_symbols[index].visible = face > 0 and not illustrated
	_normal_symbols[index].text = symbol
	_curse_symbols[index].visible = cursed
	_curse_symbols[index].text = curse_glyph
	var symbol_position := Vector2(6, 7) if cursed else Vector2(19, 7)
	var symbol_size := Vector2(28, 32) if cursed else Vector2(32, 32)
	if hud_compact:
		symbol_position = Vector2(4, 3) if cursed else Vector2(15, 3)
		symbol_size = Vector2(26, 27)
	_normal_symbols[index].position = symbol_position; _normal_symbols[index].size = symbol_size
	_faces[index].position = symbol_position; _faces[index].size = symbol_size
	if illustrated:
		_faces[index].texture = load("res://assets/battle/wasteland/%s.svg" % ("fang" if face <= 3 else "gland" if face <= 5 else "coil"))
	# Separate face controls preserve both symbols and their own colors. Native
	# text retains the same information for accessibility and inspection.
	for key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]:
		button.add_theme_color_override(key, Color.TRANSPARENT if face > 0 else Color("efe5cc"))

func _toggle(pressed: bool, index: int) -> void:
	if pressed and index not in selected: selected.append(index)
	if not pressed: selected.erase(index)
	selected.sort()
	selection_changed.emit(selected.duplicate())

func _style_die(button: Button) -> void:
	var ink := Color("efe5cc")
	for state in ["normal", "hover", "pressed", "disabled"]:
		var style := StyleBoxFlat.new()
		style.bg_color = preload("res://presentation/battle/cinematic_theme.gd").DARK_SURFACE
		style.border_color = Color("ffe0a0") if state in ["hover", "pressed"] else Color("a69c83")
		style.border_width_left = 2; style.border_width_top = 3; style.border_width_right = 4; style.border_width_bottom = 6
		style.set_corner_radius_all(7); style.shadow_color = Color("000000b0"); style.shadow_size = 4; style.shadow_offset = Vector2(2, 4)
		style.content_margin_left = 4; style.content_margin_right = 4; style.content_margin_top = 2; style.content_margin_bottom = 4
		if hud_compact:
			style.set_border_width_all(2); style.border_width_bottom = 3; style.set_corner_radius_all(4)
			style.shadow_size = 2; style.shadow_offset = Vector2(1, 2)
			style.content_margin_top = 1; style.content_margin_bottom = 1
		if state == "pressed": style.shadow_color = Color("e8c78070"); style.shadow_size = 7
		button.add_theme_stylebox_override(state, style)
	for color_key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]: button.add_theme_color_override(color_key, ink)
	button.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)

# Mark data persists even when the offensive result is hidden or a die is rolling.
# Six fixed face positions make growth visible without widening the dice tray.
func display_owned(dice: Array) -> void:
	_owned = dice.duplicate(true)
	var maps: Array = []
	var any_marks := false
	for i in _buttons.size():
		var owned: Dictionary = _owned[i] if i < _owned.size() else {}
		var faces: Array[int] = []
		var raw = owned.get("cursed_faces", [])
		if raw is Array:
			for value in raw:
				var face := int(value)
				if face >= 1 and face <= 6 and face not in faces: faces.append(face)
		any_marks = any_marks or not faces.is_empty() or bool(owned.get("entombed", false))
		faces = faces.filter(func(face): return not _pending_curse_faces.has(Vector2i(i, face)))
		faces.sort(); maps.append(faces)
	_mark_legend.visible = any_marks and not compact_row
	for i in _buttons.size():
		var owned: Dictionary = _owned[i] if i < _owned.size() else {}
		var marks: Array = maps[i]
		var bound := bool(owned.get("entombed", false))
		var numbers: Array[String] = []
		for face in marks: numbers.append(str(int(face)))
		var detail := "Die %d · %d of 6 faces cursed\nCursed faces: %s%s" % [i + 1, marks.size(), ", ".join(numbers) if not marks.is_empty() else "none", "\nENTOMBED · must join rerolls; priority for extra rolls; released by physically rolling a cursed face" if bound else ""]
		var rolling := i in _rolling_indices and Time.get_ticks_msec() - _roll_started_ms < _roll_duration_ms
		if not rolling:
			_show_face(i, int(_dice[i].get("face", _dice[i].get("number", 0))) if i < _dice.size() else 0)
			_buttons[i].tooltip_text += "\n" + detail
		_mark_panels[i].visible = any_marks or (i < _owned.size() and not compact_row)
		_mark_panels[i].tooltip_text = detail
		_mark_labels[i].text = ("D%d %d/6" if hud_compact else "D%d · %d/6") % [i + 1, marks.size()] if any_marks else "D%d · clean" % (i + 1)
		_mark_labels[i].get_parent().get_node("Faces").visible = any_marks
		_bindings[i].visible = bound
		_bound_labels[i].visible = bound
		_bound_labels[i].tooltip_text = detail
		for face in range(1, 7):
			var chip: Label = _mark_faces[i][face - 1]
			var cursed := face in marks
			chip.set_meta("cursed", cursed)
			chip.tooltip_text = "Die %d · face %d · %s" % [i + 1, face, "CURSED" if cursed else "not cursed"]
			var style := StyleBoxFlat.new(); style.set_corner_radius_all(3)
			style.bg_color = Color("72428c") if cursed else Color("25232a")
			style.border_color = Color("e6b5ff") if cursed else Color.TRANSPARENT; style.set_border_width_all(1 if cursed else 0)
			chip.add_theme_stylebox_override("normal", style)
			chip.add_theme_color_override("font_color", Color.WHITE if cursed else Color("d5ccdf") if hud_compact else Color("a3a0a8"))
		_buttons[i].modulate = Color.WHITE
		_buttons[i].disabled = rolling or not _interactive or i >= _dice.size() or bound

# Stage only the visual mark while a defense effect travels to this face.
# The authoritative owned dice and saved offensive results remain intact.
func set_curse_face_pending(index: int, face: int, pending: bool) -> void:
	var key := Vector2i(index, face)
	if _pending_curse_faces.has(key) == pending: return
	if pending: _pending_curse_faces[key] = true
	else: _pending_curse_faces.erase(key)
	display_owned(_owned)

# A chosen face turns over once; it never cycles through random numbers.
func present_face_change(index: int, before: int, progress: float) -> void:
	if index < 0 or index >= _dice.size(): return
	var t := clampf(progress, 0.0, 1.0)
	_show_face(index, before if t < 0.5 else int(_dice[index].get("face", 0)))
	var button := _buttons[index]
	button.pivot_offset = button.size * 0.5
	button.scale = Vector2(maxf(0.06, absf(cos(t * PI))), 1.0)
	button.rotation = sin(t * TAU) * 0.045
	button.modulate = Color.WHITE.lerp(Color("e2b2ff"), sin(t * PI) * 0.4)
	if t >= 1.0:
		button.scale = Vector2.ONE; button.rotation = 0.0; button.modulate = Color.WHITE

# The screen retains this timestamp across redraws so opening panels does not
# replay the binding. Loading an already entombed die shows its settled frame.
func animate_entombment(index: int, started_ms: int) -> void:
	if index < 0 or index >= _bindings.size(): return
	_bindings[index].started_ms = started_ms
	_bindings[index].set_process(started_ms >= 0 and _bindings[index].progress() < 1.0)
	_bindings[index].queue_redraw()
