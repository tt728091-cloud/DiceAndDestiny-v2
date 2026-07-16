class_name BattleDiceTray
extends VBoxContainer

signal selection_changed(indices: Array)
var selected: Array = []
var _buttons: Array[Button] = []
var _caption: Label

func _ready() -> void:
	_caption = Label.new(); add_child(_caption)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); add_child(row)
	for index in 5:
		var button := Button.new(); button.custom_minimum_size = Vector2(52, 58); button.toggle_mode = true; button.text = "—"; button.tooltip_text = "Ready die %d" % (index + 1)
		button.toggled.connect(_toggle.bind(index)); row.add_child(button); _buttons.append(button)

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
