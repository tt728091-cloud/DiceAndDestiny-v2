class_name BattleDiceTray
extends VBoxContainer

signal selection_changed(indices: Array)
var selected: Array = []
var _buttons: Array[Button] = []
var _caption: Label
var _faces: Array[TextureRect] = []
var _numbers: Array[Label] = []

func _ready() -> void:
	_caption = Label.new(); _caption.add_theme_font_size_override("font_size", 14); _caption.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; add_child(_caption)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); add_child(row)
	for index in 5:
		var button := Button.new(); button.custom_minimum_size = Vector2(70, 76); button.add_theme_font_size_override("font_size", 23); button.toggle_mode = true; button.text = "—"; button.tooltip_text = "Ready die %d" % (index + 1)
		_style_die(button)
		button.toggled.connect(_toggle.bind(index)); row.add_child(button); _buttons.append(button)
		var icon := TextureRect.new(); icon.mouse_filter = Control.MOUSE_FILTER_IGNORE; icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
		button.add_child(icon); icon.position = Vector2(19, 7); icon.size = Vector2(32, 32); _faces.append(icon)
		var number := Label.new(); number.mouse_filter = Control.MOUSE_FILTER_IGNORE; number.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; number.add_theme_color_override("font_color", Color("efe5cc")); number.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		button.add_child(number); number.position = Vector2(0, 42); number.size = Vector2(70, 25); _numbers.append(number)

func display(dice: Array, kept: Array = [], interactive: bool = false, caption: String = "Dice", inspection_prefix: String = "") -> void:
	selected.clear()
	for value in kept: selected.append(int(value))
	selected.sort()
	_caption.text = caption
	for index in _buttons.size():
		var button := _buttons[index]
		if index < dice.size():
			var face := int(dice[index].get("face", dice[index].get("number", 0)))
			var die_id := str(dice[index].get("die_id", "standard_d6"))
			button.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(die_id, face), face]
			button.tooltip_text = "Die %d: face %d, %s%s" % [index + 1, face, BattlePresentationCatalog.symbol_name_for_die_face(die_id, face), " (kept)" if index in selected else ""]
		else: button.text = "—"; button.tooltip_text = "Ready die %d" % (index + 1)
		var venom_face := index < dice.size() and str(dice[index].get("die_id", "")) == "venom_d6"
		_faces[index].visible = venom_face; _numbers[index].visible = venom_face
		if venom_face:
			var face := int(dice[index].get("face", dice[index].get("number", 0)))
			_faces[index].texture = load("res://assets/battle/wasteland/%s.svg" % ("fang" if face <= 3 else "gland" if face <= 5 else "coil"))
			_numbers[index].text = str(face)
		# Hovering a kept die uses Godot’s separate hover-pressed text color.
		# Keep the native accessibility text hidden behind the illustrated face.
		for key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]: button.add_theme_color_override(key, Color.TRANSPARENT if venom_face else Color("efe5cc"))
		button.disabled = not interactive or index >= dice.size()
		button.button_pressed = index in selected
		if not inspection_prefix.is_empty():
			var inspector = get_node_or_null("/root/GameInspector")
			if inspector != null: inspector.register_control("%s.%d" % [inspection_prefix, index], button, button.tooltip_text)

func _toggle(pressed: bool, index: int) -> void:
	if pressed and index not in selected: selected.append(index)
	if not pressed: selected.erase(index)
	selected.sort()
	selection_changed.emit(selected.duplicate())

func _style_die(button: Button) -> void:
	var ink := Color("efe5cc")
	for state in ["normal", "hover", "pressed", "disabled"]:
		var style := StyleBoxFlat.new()
		style.bg_color = Color("242423")
		style.border_color = Color("ffe0a0") if state in ["hover", "pressed"] else Color("a69c83")
		style.border_width_left = 2; style.border_width_top = 3; style.border_width_right = 4; style.border_width_bottom = 6
		style.set_corner_radius_all(7); style.shadow_color = Color("000000b0"); style.shadow_size = 4; style.shadow_offset = Vector2(2, 4)
		style.content_margin_left = 4; style.content_margin_right = 4; style.content_margin_top = 2; style.content_margin_bottom = 4
		if state == "pressed": style.shadow_color = Color("e8c78070"); style.shadow_size = 7
		button.add_theme_stylebox_override(state, style)
	for color_key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]: button.add_theme_color_override(color_key, ink)
	button.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
