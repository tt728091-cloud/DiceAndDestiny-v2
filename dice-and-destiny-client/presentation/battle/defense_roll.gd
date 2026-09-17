extends VBoxContainer

var started_ms: int
var dice: Array[Button] = []
var die_ids: Array[String] = []
var paused := false

func configure(ability_id: String, catalyst_paid: bool, start: int, inspecting: bool) -> void:
	started_ms = start
	paused = inspecting
	alignment = BoxContainer.ALIGNMENT_CENTER
	add_theme_constant_override("separation", 16)
	var title := Label.new(); title.text = BattlePresentationCatalog.ability(ability_id).name; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 24); add_child(title)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 14); add_child(row)
	var definition := BattlePresentationCatalog.definition("abilities", ability_id)
	for operation in definition.get("resolution", {}).get("operations", []):
		if operation.get("type") != "roll_dice": continue
		for index in int(operation.get("dice_count", 1)):
			var die := Button.new(); die.disabled = true; die.mouse_filter = Control.MOUSE_FILTER_IGNORE; die.custom_minimum_size = Vector2(90, 90); die.add_theme_font_size_override("font_size", 28)
			var style := StyleBoxFlat.new(); style.bg_color = Color("172635"); style.border_color = Color("89bec8"); style.set_border_width_all(2); style.set_corner_radius_all(12); die.add_theme_stylebox_override("disabled", style); die.add_theme_color_override("font_disabled_color", Color("e2f3fb"))
			row.add_child(die); dice.append(die); die_ids.append(str(operation.get("dice_id", "standard_d6")))
	var hint := Label.new(); hint.text = "Rolling defense…" if not paused else "Defense roll · inspection paused"; hint.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; add_child(hint)
	if catalyst_paid:
		var paid := Label.new(); paid.text = "Catalyst spent · +2 prevention"; paid.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; paid.add_theme_color_override("font_color", Color("b7e39a")); add_child(paid)

func _process(_delta: float) -> void:
	var elapsed := 0.0 if paused else maxf(0.0, (Time.get_ticks_msec() - started_ms) / 1000.0)
	for index in dice.size():
		var face := 1 + (int(elapsed * 18) + index * 3) % 6
		dice[index].text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(die_ids[index], face), face] if not paused else "—"
		dice[index].rotation = sin(elapsed * 28 + index) * 0.06 if not paused else 0.0
