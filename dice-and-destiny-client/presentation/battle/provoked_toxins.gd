extends VBoxContainer

# Presentation only: every landed face comes from the authority snapshot.
const ROLL_SECONDS := 1.0
const CATALYST_CUE_SECONDS := 0.8
const RESULT_HOLD_SECONDS := 1.4
var entries: Array[Dictionary] = []
var paused := false
var _paused_at := 0

func configure(rolls: Array, names: Dictionary, outcomes: Array[String], events: Array = []) -> void:
	add_theme_constant_override("separation", 18)
	_label(self, "PROVOKED TOXINS", 26)
	var sides := HBoxContainer.new(); sides.add_theme_constant_override("separation", 20); add_child(sides)
	var rows := {}
	for actor in ["blade", "goblin"]:
		var has_roll := false
		for roll in rolls:
			if roll.get("actor_id") == actor: has_roll = true
		if not has_roll: continue
		var plate := PanelContainer.new(); plate.size_flags_horizontal = Control.SIZE_EXPAND_FILL; sides.add_child(plate)
		var backdrop := StyleBoxFlat.new(); backdrop.bg_color = Color("181815ed"); backdrop.border_color = Color("a38b5c"); backdrop.set_border_width_all(1); backdrop.set_corner_radius_all(4)
		backdrop.content_margin_left = 18; backdrop.content_margin_right = 18; backdrop.content_margin_top = 16; backdrop.content_margin_bottom = 16; plate.add_theme_stylebox_override("panel", backdrop)
		var column := VBoxContainer.new(); column.add_theme_constant_override("separation", 12); plate.add_child(column)
		_label(column, str(names.get(actor, actor)), 20)
		var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 16); column.add_child(row); rows[actor] = row
	var ordinals := {}
	for index in rolls.size():
		var roll: Dictionary = rolls[index]
		var actor := str(roll.get("actor_id", ""))
		var cell := VBoxContainer.new(); cell.custom_minimum_size.x = 145; rows[actor].add_child(cell)
		var status := BattlePresentationCatalog.status(str(roll.get("source_content_id", roll.get("status_id", ""))))
		_label(cell, str(status.glyph) + " " + str(status.name), 17)
		var cue := _label(cell, "", 21); cue.custom_minimum_size.y = 30; cue.add_theme_color_override("font_color", Color("d0a6ff"))
		var die := Button.new(); die.disabled = true; die.custom_minimum_size = Vector2(82, 82); die.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; die.add_theme_font_size_override("font_size", 28); cell.add_child(die)
		var style := StyleBoxFlat.new(); style.bg_color = Color("242423"); style.border_color = Color("a6987e"); style.set_border_width_all(3); style.set_corner_radius_all(8); die.add_theme_stylebox_override("disabled", style); die.add_theme_color_override("font_disabled_color", Color("eee3c9"))
		die.set_meta("inspection_id", "battle.effect_die.%s.%d" % [actor, int(ordinals.get(actor, 0))]); ordinals[actor] = int(ordinals.get(actor, 0)) + 1
		var result := _label(cell, "", 17); result.custom_minimum_size.y = 54
		entries.append({"die": die, "cue": cue, "result": result, "face": int(roll.get("die", {}).get("face", 0)), "die_id": str(roll.get("die", {}).get("die_id", "standard_d6")), "rerolled": bool(roll.get("rerolled", false)), "catalyst": false, "started": Time.get_ticks_msec(), "outcome": outcomes[index]})
	# A reopened snapshot can include the Catalyst event that produced its face.
	# Reconstruct only the known prior face; never invent historical outcomes.
	for event in events:
		var data: Dictionary = event.get("data", {})
		var index := int(data.get("die_index", -1))
		if event.get("type") == "dice_rolled" and data.get("source_type") == "catalyst" and index >= 0 and index < entries.size():
			entries[index].catalyst = true; entries[index].previous_face = int(data.face_before)
	_process(0)

func update_rolls(rolls: Array, outcomes: Array[String], events: Array) -> void:
	for index in mini(rolls.size(), entries.size()):
		var roll: Dictionary = rolls[index]; var entry := entries[index]
		var face := int(roll.get("die", {}).get("face", 0))
		var rerolled := bool(roll.get("rerolled", false))
		if face != entry.face or (rerolled and not entry.rerolled):
			entry.catalyst = false
			for event in events:
				var data: Dictionary = event.get("data", {})
				if event.get("type") == "dice_rolled" and data.get("source_type") == "catalyst" and int(data.get("die_index", -1)) == index: entry.catalyst = true
			entry.previous_face = entry.face
			entry.started = Time.get_ticks_msec()
			entry.face = face
			entry.rerolled = rerolled
		entry.outcome = outcomes[index]
	_process(0)

func set_paused(value: bool) -> void:
	if paused == value: return
	paused = value
	if paused: _paused_at = Time.get_ticks_msec()
	else:
		for entry in entries: entry.started += Time.get_ticks_msec() - _paused_at

func ready_to_continue() -> bool:
	if paused: return false
	for entry in entries:
		var seconds := (Time.get_ticks_msec() - int(entry.started)) / 1000.0
		if seconds < ROLL_SECONDS + RESULT_HOLD_SECONDS + (CATALYST_CUE_SECONDS if entry.catalyst else 0.0): return false
	return true

func _process(_delta: float) -> void:
	if paused: return
	for entry in entries:
		var elapsed := (Time.get_ticks_msec() - int(entry.started)) / 1000.0
		var cue_time := CATALYST_CUE_SECONDS if entry.catalyst else 0.0
		var rolling := elapsed >= cue_time and elapsed < cue_time + ROLL_SECONDS
		var face := int(entry.get("previous_face", entry.face)) if elapsed < cue_time else 1 + int(elapsed * 19) % 6 if rolling else int(entry.face)
		entry.die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(entry.die_id, face), face]
		entry.die.pivot_offset = entry.die.size * 0.5
		entry.die.rotation = sin(elapsed * 30) * 0.07 if rolling else 0.0
		entry.die.modulate = Color("d0a6ff") if entry.catalyst and elapsed < cue_time + ROLL_SECONDS else Color.WHITE
		entry.cue.text = "Catalyst −1 · ↻" if entry.catalyst and elapsed < cue_time + ROLL_SECONDS else ""
		entry.result.text = "" if elapsed < cue_time + ROLL_SECONDS else str(entry.outcome).get_slice("\n", 1)

func _label(parent: Node, value: String, font_size: int) -> Label:
	var label := Label.new(); label.text = value; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", font_size); parent.add_child(label); return label
