extends VBoxContainer

signal finished
signal phase_changed(phase: String, progress: float)
var summary: Dictionary
var entries: Array = []
var groups: Dictionary = {}
var _started := 0
var _done := false
var _phase: Label
var _closeout: Label
var duration := 7.2
var paused := false
var _pause_started := 0
var _names: Dictionary
var _conversions: Array[Label] = []

func configure(data: Dictionary, names: Dictionary) -> void:
	summary = data
	_names = names
	add_theme_constant_override("separation", 12)
	var heading := _label(self, "ONGOING EFFECTS", 26)
	heading.add_theme_color_override("font_color", Color("b8d9bd"))
	_phase = _label(self, "Effects activate", 17)
	_parse()
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 18); add_child(columns)
	for actor in ["blade", "goblin"]:
		var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.add_theme_constant_override("separation", 12); columns.add_child(column)
		_label(column, str(names.get(actor, actor)), 20)
		var actors_entries: Array = []
		for entry in entries:
			if entry.actor_id == actor: actors_entries.append(entry)
		if actors_entries.is_empty(): _label(column, "No damaging effects", 15)
		for entry in actors_entries:
			var key: String = str(actor) + ":" + str(entry.status_id)
			if not groups.has(key):
				var panel := PanelContainer.new(); column.add_child(panel)
				var style := StyleBoxFlat.new(); style.bg_color = Color("181815ed"); style.border_color = Color("a38b5c"); style.set_border_width_all(1); style.set_corner_radius_all(4); style.content_margin_left = 10; style.content_margin_right = 10; style.content_margin_top = 8; style.content_margin_bottom = 8; panel.add_theme_stylebox_override("panel", style)
				var body := VBoxContainer.new(); panel.add_child(body)
				var status := BattlePresentationCatalog.status(str(entry.status_id)); var title := _label(body, str(status.glyph) + " " + str(status.name), 17); title.tooltip_text = str(status.get("text", ""))
				var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 8); body.add_child(row); groups[key] = row
			var cell := VBoxContainer.new(); cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL; groups[key].add_child(cell)
			entry.cell = cell
			var die_panel := PanelContainer.new(); die_panel.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; cell.add_child(die_panel)
			var die_style := StyleBoxFlat.new(); die_style.bg_color = Color("101b28"); die_style.border_color = Color("95bda8") if entry.status_id == "poison" else Color("b896da") if entry.status_id == "volatile_poison" else Color("c97580"); die_style.set_border_width_all(2); die_style.set_corner_radius_all(10); die_panel.add_theme_stylebox_override("panel", die_style)
			entry.die = _label(die_panel, "…" if entry.face > 0 else str(entry.damage), 34)
			entry.die.custom_minimum_size = Vector2(76, 68)
			entry.die.set_meta("inspection_id", "battle.automatic_effect_die.%s.%d" % [actor, entries.find(entry)])
			entry.result = _label(cell, "", 14); entry.result.custom_minimum_size.y = 24
			entry.cards_ui = []
			var card_row := HBoxContainer.new(); card_row.alignment = BoxContainer.ALIGNMENT_CENTER; cell.add_child(card_row)
			for card in entry.cards:
				var tile := PanelContainer.new(); tile.custom_minimum_size = Vector2(48, 66); card_row.add_child(tile)
				var box := VBoxContainer.new(); tile.add_child(box)
				var definition := BattlePresentationCatalog.card(str(card.get("card_definition_id", "")))
				var art := TextureRect.new(); art.custom_minimum_size = Vector2(38, 34); art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
				art.texture = preload("res://presentation/battle/cinematic_theme.gd").art(preload("res://presentation/battle/cinematic_theme.gd").card_art_index(str(card.get("card_definition_id", ""))))
				if art.texture == null and ResourceLoader.exists(str(definition.art)): art.texture = load(str(definition.art))
				box.add_child(art)
				var caption := _label(box, str(definition.name), 10); caption.custom_minimum_size.x = 46; caption.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
				tile.tooltip_text = "%s · removed from %s" % [definition.name, card.get("original_zone", "deck")]
				tile.modulate.a = 0; entry.cards_ui.append(tile)
	for actor in summary.get("actors_before", {}):
		var before := _counts(summary.actors_before[actor]); var after := _counts(summary.actors_after.get(actor, {}))
		if int(before.get("incubation", 0)) > int(after.get("incubation", 0)) and int(after.get("volatile_poison", 0)) > int(before.get("volatile_poison", 0)):
			var conversion := _label(self, "%s   • Poison   →   ✦ Volatile Poison" % names.get(actor, actor), 20)
			conversion.modulate.a = 0; _conversions.append(conversion)
	_closeout = _label(self, "", 16); _closeout.custom_minimum_size.y = 38
	_closeout.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	if entries.is_empty(): duration = 1.3 if _conversions.is_empty() else 3.0
	_started = Time.get_ticks_msec()

func _array(value) -> Array:
	return value if value is Array else []

func _parse() -> void:
	var sources: Array = []; var removals: Array = []; var rolls: Array = []; var catalysts: Array = []
	for step in _array(summary.get("steps", [])):
		var data: Dictionary = step.get("data", {})
		if step.get("type") == "proposal_batch_committed" and data.has("rolls"): rolls.append_array(_array(data.rolls))
		if step.get("type") == "dice_rolled" and data.get("source_type") == "catalyst": catalysts.append(data)
		if step.get("type") == "damage_committed": sources.append_array(_array(data.get("sources", []))); removals.append_array(_array(data.get("removals", [])))
	var assigned := {}
	for index in rolls.size():
		var roll: Dictionary = rolls[index]; var face := int(roll.get("die", {}).get("face", 0))
		var entry := {"actor_id": str(roll.actor_id), "status_id": str(roll.source_content_id), "face": face, "original_face": face, "catalyst": false, "damage": 0, "sources": [], "cards": []}
		for catalyst in catalysts:
			if int(catalyst.get("die_index", -1)) == index:
				entry.original_face = int(catalyst.face_before); entry.catalyst = true
		if _damages_on_face(str(roll.source_content_id), face):
			for source in sources:
				if not assigned.has(source.id) and source.target_actor_id == roll.actor_id and source.source_content_id == roll.source_content_id:
					entry.damage += int(source.final_amount); entry.sources.append(source.id); assigned[source.id] = true; break
		entries.append(entry)
	for source in sources:
		if assigned.has(source.id): continue
		var entry: Dictionary = {}
		for existing in entries:
			if existing.face == 0 and existing.actor_id == source.target_actor_id and existing.status_id == source.source_content_id: entry = existing; break
		if entry.is_empty():
			entry = {"actor_id": source.target_actor_id, "status_id": source.source_content_id, "face": 0, "original_face": 0, "catalyst": false, "damage": 0, "sources": [], "cards": []}; entries.append(entry)
		entry.damage += int(source.final_amount); entry.sources.append(source.id)
	# A card can be linked to several sources. Show each actually lost card once.
	var used_cards := {}
	for entry in entries:
		for card in removals:
			if entry.cards.size() >= int(entry.damage): break
			if not card.get("accepted", false) or card.get("released", false) or used_cards.has(card.card_id): continue
			for source_id in _array(card.get("damage_proposal_ids", [])):
				if source_id in entry.sources: entry.cards.append(card); used_cards[card.card_id] = true; break

func _damages_on_face(status_id: String, face: int) -> bool:
	for trigger in BattlePresentationCatalog.definition("statuses", status_id).get("triggers", []):
		for operation in trigger.get("operations", []):
			for outcome in operation.get("outcomes", []):
				if _matches_face(face, outcome.get("faces", [])):
					for effect in outcome.get("operations", []):
						if effect.get("type") == "deal_damage": return true
	return false

func _label(parent: Node, text: String, font_size: int) -> Label:
	var label := Label.new(); label.text = text; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", font_size); parent.add_child(label); return label

func set_paused(value: bool) -> void:
	if value == paused: return
	paused = value
	if paused: _pause_started = Time.get_ticks_msec()
	else: _started += Time.get_ticks_msec() - _pause_started

func playback_elapsed() -> float:
	return ((_pause_started if paused else Time.get_ticks_msec()) - _started) / 1000.0

func resume_at(seconds: float) -> void:
	_started = Time.get_ticks_msec() - int(seconds * 1000)

func _process(_delta: float) -> void:
	if _started == 0 or _done or paused: return
	var elapsed := playback_elapsed()
	_phase.text = ("Rolling effects" if _has_dice() else "Bleed") if elapsed < 1.6 else "Catalyst" if elapsed < 2.9 and _has_catalyst() else "Damage" if elapsed < 4.8 else "Effects settle"
	for entry in entries:
		if entry.face > 0:
			var rolling: bool = elapsed < 0.9 or (entry.catalyst and elapsed >= 2.1 and elapsed < 2.65)
			entry.die.text = str(1 + int(elapsed * 19) % 6) if rolling else str(entry.original_face if elapsed < 2.1 else entry.face)
			entry.die.rotation = sin(elapsed * 32) * 0.08 if rolling else 0.0
			entry.die.modulate = Color("d0a6ff") if entry.catalyst and elapsed >= 1.6 and elapsed < 2.9 else Color.WHITE
			entry.result.text = "Catalyst ↻" if entry.catalyst and elapsed >= 1.6 and elapsed < 2.9 else ""
		if elapsed >= 2.9:
			entry.result.text = "%d damage" % int(entry.damage) if entry.damage > 0 else "Clears" if _clears(entry) else "No damage"
		if elapsed > 5.2 and not entry.cards.is_empty(): entry.result.text = "%d card%s lost" % [entry.cards.size(), "" if entry.cards.size() == 1 else "s"]
		for index in entry.cards_ui.size():
			var card: Control = entry.cards_ui[index]
			var reveal := clampf((elapsed - 3.0 - index * 0.12) / 0.4, 0, 1)
			var dissolve := clampf((elapsed - 4.5 - index * 0.08) / 0.6, 0, 1)
			card.modulate = Color(1, 1 - dissolve * 0.6, 1 - dissolve * 0.6, reveal * (1 - dissolve))
		if elapsed > 5.2 and entry.face == 0:
			var before := _counts(summary.get("actors_before", {}).get(entry.actor_id, {}))
			var after := _counts(summary.get("actors_after", {}).get(entry.actor_id, {}))
			entry.die.text = str(roundi(lerpf(float(before.get(entry.status_id, 0)), float(after.get(entry.status_id, 0)), clampf((elapsed - 5.2) / 0.7, 0, 1))))
			entry.result.text = "%d cards lost · %s left" % [entry.cards.size(), entry.die.text]
		if elapsed > 5.2 and _clears(entry): entry.die.modulate.a = 1 - clampf((elapsed - 5.2) / 0.65, 0, 1)
	for conversion in _conversions:
		var conversion_start := 0.1 if entries.is_empty() else 5.2
		var blend := clampf((elapsed - conversion_start) / 0.7, 0, 1)
		conversion.modulate = Color("8fe18b").lerp(Color("d0a6ff"), blend)
		conversion.modulate.a = clampf((elapsed - conversion_start) / 0.3, 0, 1)
	phase_changed.emit("cards" if elapsed < 5.2 else "statuses", clampf((elapsed - (4.5 if elapsed < 5.2 else 5.2)) / 0.7, 0, 1))
	if elapsed > 5.2: _closeout.text = _status_changes(clampf((elapsed - 5.2) / 0.7, 0, 1))
	if entries.is_empty():
		_phase.text = "Effects settle"; _closeout.text = _status_changes(clampf(elapsed / 0.7, 0, 1))
		phase_changed.emit("statuses", clampf(elapsed / 0.7, 0, 1))
	if elapsed >= duration: _done = true; finished.emit()

func _has_dice() -> bool:
	for entry in entries:
		if entry.face > 0: return true
	return false

func _has_catalyst() -> bool:
	for entry in entries:
		if entry.catalyst: return true
	return false

func _clears(entry: Dictionary) -> bool:
	for trigger in BattlePresentationCatalog.definition("statuses", str(entry.status_id)).get("triggers", []):
		for operation in trigger.get("operations", []):
			for outcome in operation.get("outcomes", []):
				if _matches_face(int(entry.face), outcome.get("faces", [])):
					for effect in outcome.get("operations", []):
						if effect.get("type") == "remove_status_stack": return true
	return false

func _status_changes(progress: float) -> String:
	var changes: Array[String] = []
	for actor in summary.get("actors_before", {}):
		var before := _counts(summary.actors_before[actor]); var after := _counts(summary.actors_after.get(actor, {}))
		for id in after:
			if not before.has(id): before[id] = 0
		for id in before:
			var finish := int(after.get(id, 0))
			if int(before[id]) == finish: continue
			changes.append("%s · %s %d → %d" % [_names.get(actor, actor), BattlePresentationCatalog.status(str(id)).name, int(before[id]), roundi(lerpf(float(before[id]), finish, progress))])
	return "\n".join(changes) if not changes.is_empty() else "Effects complete"

func _counts(actor: Dictionary) -> Dictionary:
	var result := {}
	for status in _array(actor.get("statuses", [])): result[str(status.definition_id)] = int(status.stacks)
	return result

func _matches_face(face: int, faces: Array) -> bool:
	for value in faces:
		if int(value) == face: return true
	return false
