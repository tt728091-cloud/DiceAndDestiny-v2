extends Control

signal enemy_selected(actor_id: String)
var buttons: Dictionary = {}
const STYLE := preload("res://presentation/battle/cinematic_theme.gd")
const PLATE_SIZE := Vector2(156, 43)

func configure(entries: Array, selected: String) -> void:
	name = "BattlefieldEnemySelector"
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	var occupied: Array[Rect2] = []
	for entry in entries:
		var id := str(entry.id)
		var fighter: Rect2 = entry.rect
		var position_on_field := Vector2(fighter.position.x + fighter.size.x * 0.42 - PLATE_SIZE.x / 2.0, fighter.position.y - PLATE_SIZE.y - 8)
		position_on_field.x = clampf(position_on_field.x, 470, 1490 - PLATE_SIZE.x)
		position_on_field.y = maxf(155, position_on_field.y)
		var area := Rect2(position_on_field, PLATE_SIZE)
		while occupied.any(func(other): return other.grow(3).intersects(area)):
			area.position.y -= PLATE_SIZE.y + 6
		occupied.append(area)
		var button := Button.new(); button.name = "Nameplate_" + id
		button.set_meta("battle_utility", true); button.set_meta("actor_id", id)
		button.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		button.tooltip_text = "%s · %s\nSelect to target and inspect this enemy." % [entry.name, entry.state]
		add_child(button); button.position = area.position; button.size = area.size; buttons[id] = button
		var selected_enemy: bool = id == selected
		button.add_theme_stylebox_override("normal", STYLE.panel(Color("292419e8") if selected_enemy else Color("101416db"), Color("f4ce70") if selected_enemy else Color("746953"), 3))
		var label := Label.new(); label.text = str(entry.name); label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		label.mouse_filter = Control.MOUSE_FILTER_IGNORE; label.add_theme_font_size_override("font_size", 17)
		label.add_theme_color_override("font_color", Color("ffe4a1") if selected_enemy else Color("e2d8c5"))
		button.add_child(label); label.position = Vector2(3, 2); label.size = Vector2(150, 25)
		var health := ProgressBar.new(); health.name = "Health"; health.mouse_filter = Control.MOUSE_FILTER_IGNORE
		health.max_value = maxi(1, int(entry.maximum)); health.value = entry.health; health.show_percentage = false
		button.add_child(health); health.position = Vector2(9, 31); health.size = Vector2(138, 6)
		if entry.defeated: button.modulate.a = 0.5
		button.pressed.connect(func(): enemy_selected.emit(id))
