extends PanelContainer
signal source_selected(source_id: String)
signal damage_settled

var _damage_settled := false

var data: Dictionary
var started_ms: int
var damage: Label
var block: Label
var gain_origins: Array[Label] = []
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var dice_controls: Array[Button] = []
var benefit_labels: Array[Label] = []
var _body: VBoxContainer

func configure(result: Dictionary, start: int, compact: bool = false) -> void:
	data = result; started_ms = start
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var style := StyleBoxFlat.new(); style.bg_color = Color("181815ed"); style.border_color = Color("a38b5c"); style.set_border_width_all(1); style.set_corner_radius_all(4)
	style.content_margin_left = 18; style.content_margin_right = 18; style.content_margin_top = 16; style.content_margin_bottom = 16
	add_theme_stylebox_override("panel", style)
	_body = VBoxContainer.new(); _body.add_theme_constant_override("separation", 4 if compact else 10); add_child(_body)
	if not compact: _label(str(data.actor_name).to_upper(), 16, Color("a7cbd6"))
	var attack := Button.new(); attack.text = str(data.attack_name) + " → " + str(data.actor_name); attack.flat = true; attack.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; attack.disabled = bool(data.read_only); attack.pressed.connect(func(): source_selected.emit(str(data.source_id))); attack.set_meta("inspection_id", "battle.source." + str(data.source_id)); _body.add_child(attack)
	damage = _label(str(data.before), 32 if compact else 46, Color("ffd19a")); damage.set_meta("inspection_id", "battle.defense_damage." + str(data.actor_id))
	_label("DAMAGE INCOMING", 12, Color("96a6b5"))
	var block_area := Control.new(); block_area.custom_minimum_size.y = 28 if compact else 40; _body.add_child(block_area)
	block = Label.new(); block.text = "PREVENT %d" % int(data.prevented) if int(data.prevented) > 0 else ""; block.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; block.add_theme_font_size_override("font_size", 23); block.add_theme_color_override("font_color", Color("81e2e9")); block_area.add_child(block); block.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	var attack_statuses := _label(str(data.attack_statuses), 15, Color("c4a8ef")); attack_statuses.custom_minimum_size.y = 0 if compact else 22
	var title := _label(str(data.ability_name), 18 if compact else 20); title.tooltip_text = str(data.rules)
	var dice := HBoxContainer.new(); dice.alignment = BoxContainer.ALIGNMENT_CENTER; dice.add_theme_constant_override("separation", 12); _body.add_child(dice)
	for index in data.dice.size():
		var face: Dictionary = data.dice[index]
		var cell := VBoxContainer.new(); dice.add_child(cell)
		var die := Button.new(); die.disabled = true; die.custom_minimum_size = Vector2(54, 54) if compact else Vector2(72, 72); die.add_theme_font_size_override("font_size", 24)
		die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(data.die_id), int(face.face)), int(face.face)]
		dice_controls.append(die)
		die.tooltip_text = str(face.benefit); die.set_meta("inspection_id", "battle.defense_die.%s%s" % [data.actor_id, "" if index == 0 else ".%d" % index])
		var die_style := StyleBoxFlat.new(); die_style.bg_color = Color("36352d"); die_style.border_color = Color("a6987e"); die_style.set_border_width_all(2); die_style.set_corner_radius_all(10); die.add_theme_stylebox_override("disabled", die_style); die.add_theme_color_override("font_disabled_color", Color("e2f3fb")); cell.add_child(die)
		var benefit := Label.new(); benefit.text = str(face.benefit); benefit.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; benefit.add_theme_font_size_override("font_size", 14); benefit.add_theme_color_override("font_color", Color("9de0d6")); cell.add_child(benefit); benefit_labels.append(benefit)
	for gain in data.gains:
		var status := BattlePresentationCatalog.status(str(gain.status_id))
		var caption := _label("+%d %s %s" % [int(gain.amount), status.glyph, status.name], 20, Color("b8e889"))
		caption.tooltip_text = "Pending until defense responses finish."
		gain_origins.append(caption)
	if not str(data.note).is_empty(): _label(str(data.note), 14, Color("a3b5c2"))
	_update()

func _label(text: String, font_size: int, color: Color = Color("e9edf2")) -> Label:
	var label := Label.new(); label.text = text; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", color); _body.add_child(label); return label

func _process(_delta: float) -> void:
	if is_instance_valid(damage): _update()

func _update() -> void:
	var elapsed := maxf(0, (Time.get_ticks_msec() - started_ms) / 1000.0)
	var rolling := elapsed < TIMING.roll_seconds() or bool(data.get("awaiting_roll", false))
	var effects := maxf(0.0, elapsed - TIMING.roll_seconds()) / TIMING.effects_seconds()
	for index in dice_controls.size():
		var die := dice_controls[index]
		var face := 1 + (int(elapsed * 16) + index * 3) % 6 if rolling else int(data.dice[index].face)
		die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(data.die_id), face), face]
		die.pivot_offset = die.size * 0.5
		die.rotation = sin(elapsed * 26 + index) * 0.045 if rolling else 0.0
		die.tooltip_text = "Rolling…" if rolling else str(data.dice[index].benefit)
		benefit_labels[index].modulate.a = 0.0 if rolling else clampf(effects / 0.15, 0, 1)
	var progress := 0.0 if rolling else clampf((effects - 0.2) / 0.45, 0, 1)
	damage.text = str(roundi(lerpf(float(data.before), float(data.after), progress)))
	damage.modulate.a = 1.0 - 0.6 * sin(progress * PI)
	damage.add_theme_color_override("font_color", Color("ffd19a").lerp(Color("8fe1e6"), progress))
	block.modulate.a = 0.0 if rolling else clampf(effects / 0.15, 0, 1) * (1.0 - 0.35 * clampf((effects - 0.8) / 0.2, 0, 1))
	block.position.y = 8 - 14 * clampf(effects / 0.6, 0, 1)
	for origin in gain_origins: origin.modulate.a = 0.0 if rolling else 1.0 - clampf((effects - 0.3) / 0.2, 0, 1)
	if progress >= 1.0 and not _damage_settled:
		_damage_settled = true
		damage_settled.emit()

