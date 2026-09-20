extends VBoxContainer

const TIMING := preload("res://presentation/battle/combat_timing.gd")
const CARD_GRID := preload("res://presentation/cards/damage_card_grid.gd")

signal finished
signal phase_changed(phase: String, progress: float)
var summary: Dictionary
var entries: Array = []
var groups: Dictionary = {}
var _started := 0
var _done := false
var _phase: Label
var duration := 7.2
var paused := false
var _pause_started := 0
var _names: Dictionary
var _has_conversion := false
var visible_actor_ids: Array = ["blade", "goblin"]

func configure(data: Dictionary, names: Dictionary) -> void:
	summary = data
	_names = names
	add_theme_constant_override("separation", 8)
	# The phase rail already names Effects; use the space for readable cards.
	_phase = _label(self, "Effects activate", 17)
	_phase.add_theme_color_override("font_color", Color("b8d9bd"))
	_parse()
	duration = 7.2 + catalyst_extra_seconds()
	var stacked := visible_actor_ids.size() > 2
	var columns: BoxContainer = VBoxContainer.new() if stacked else HBoxContainer.new()
	columns.add_theme_constant_override("separation", 18); add_child(columns)
	for actor in visible_actor_ids:
		var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.add_theme_constant_override("separation", 12); columns.add_child(column)
		_label(column, str(names.get(actor, actor)), 20)
		var actors_entries: Array = []
		for entry in entries:
			if entry.actor_id == actor: actors_entries.append(entry)
		if actors_entries.is_empty(): _label(column, "No damaging effects", 15)
		# Each enemy owns a full-width row. Its toxin groups share that row,
		# so both enemies' rolls and losses remain on screen together.
		var group_parent: BoxContainer = column
		if stacked:
			group_parent = HBoxContainer.new(); group_parent.add_theme_constant_override("separation", 12); column.add_child(group_parent)
		for entry in actors_entries:
			var key: String = str(actor) + ":" + str(entry.status_id)
			if not groups.has(key):
				var panel := PanelContainer.new(); panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; group_parent.add_child(panel)
				var style := StyleBoxFlat.new(); style.bg_color = Color("181815ed"); style.border_color = Color("a38b5c"); style.set_border_width_all(1); style.set_corner_radius_all(4); style.content_margin_left = 10; style.content_margin_right = 10; style.content_margin_top = 8; style.content_margin_bottom = 8; panel.add_theme_stylebox_override("panel", style)
				var body := VBoxContainer.new(); panel.add_child(body)
				var count := int(_counts(summary.get("actors_before", {}).get(actor, {})).get(entry.status_id, 0))
				var status := BattlePresentationCatalog.status(str(entry.status_id)); var title := _label(body, "%s %s ×%d" % [status.glyph, status.name, count], 17); title.tooltip_text = str(status.get("text", ""))
				var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 8); body.add_child(row); groups[key] = row
			var cell := VBoxContainer.new(); cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL; groups[key].add_child(cell)
			entry.cell = cell
			var die_panel := PanelContainer.new(); die_panel.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; cell.add_child(die_panel)
			var die_style := StyleBoxFlat.new(); die_style.bg_color = Color("101b28"); die_style.border_color = Color("95bda8") if entry.status_id == "poison" else Color("b896da") if entry.status_id == "volatile_poison" else Color("c97580"); die_style.set_border_width_all(2); die_style.set_corner_radius_all(10); die_panel.add_theme_stylebox_override("panel", die_style)
			entry.die = _label(die_panel, "…" if entry.face > 0 else str(entry.damage), 28)
			entry.die.custom_minimum_size = Vector2(54, 38) if stacked else Vector2(58, 46)
			entry.die.set_meta("inspection_id", "battle.automatic_effect_die.%s.%d" % [actor, entries.find(entry)])
			entry.result = _label(cell, "", 14); entry.result.custom_minimum_size.y = 18
			entry.cards_ui = []
			# Reserve the maximum loss below each toxin roll before dice start:
			# one full card for Poison, two stacked full cards for Volatile Poison.
			# This is the same 132 × 144 card/grid used in damage resolution.
			var toxin: bool = entry.status_id in ["poison", "volatile_poison"]
			var slots := 2 if entry.status_id == "volatile_poison" else 1
			var card_grid := CARD_GRID.new(); cell.add_child(card_grid)
			card_grid.size_flags_horizontal = Control.SIZE_SHRINK_CENTER if toxin else Control.SIZE_EXPAND_FILL
			card_grid.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
			var rows := slots if toxin else maxi(1, ceili((entry.cards.size() + entry.excess) / 3.0))
			var card_size := Vector2(100, 96) if stacked else CARD_GRID.CARD_SIZE
			card_grid.custom_minimum_size = Vector2(card_size.x, rows * card_size.y + (rows - 1) * CARD_GRID.GAP)
			entry.card_grid = card_grid
			for card in entry.cards:
				var tile := BattleCard.new(); card_grid.add_child(tile)
				tile.configure(str(card.get("card_id", "")), str(card.get("card_definition_id", "")), false, false, true, true)
				tile.tooltip_text += " · Removed from " + str(card.get("original_zone", "deck"))
				tile.modulate.a = 0; entry.cards_ui.append(tile)
			# Excess damage has no card to remove. Account for it in the same
			# reserved slots rather than making a successful roll look incomplete.
			for point in int(entry.excess):
				var empty := PanelContainer.new(); card_grid.add_child(empty)
				var empty_style := StyleBoxFlat.new(); empty_style.bg_color = Color("101b28dd"); empty_style.border_color = Color("a38b5c"); empty_style.set_border_width_all(1); empty_style.set_corner_radius_all(4)
				empty.add_theme_stylebox_override("panel", empty_style)
				var caption := _label(empty, "No cards left\n\n1 excess damage", 16)
				caption.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
				caption.add_theme_color_override("font_color", Color("d8c5a0"))
				empty.modulate.a = 0; entry.cards_ui.append(empty)
	# Empty slots keep the three-roll toxin grid identical before and after
	# results, including clears and Volatile Poison's no-damage face.
	for key in groups:
		if str(key).get_slice(":", 1) not in ["poison", "volatile_poison"]: continue
		while groups[key].get_child_count() < 3:
			var slot := Control.new(); slot.custom_minimum_size.x = 100 if stacked else CARD_GRID.CARD_SIZE.x
			slot.size_flags_horizontal = Control.SIZE_EXPAND_FILL; groups[key].add_child(slot)
	for actor in summary.get("actors_before", {}):
		var before := _counts(summary.actors_before[actor]); var after := _counts(summary.actors_after.get(actor, {}))
		if int(before.get("incubation", 0)) > int(after.get("incubation", 0)) and int(after.get("volatile_poison", 0)) > int(before.get("volatile_poison", 0)):
			_has_conversion = true
	if entries.is_empty(): duration = 3.0 if _has_conversion else 1.3
	_started = Time.get_ticks_msec()

func _array(value) -> Array:
	return value if value is Array else []

func _parse() -> void:
	var sources: Array = []; var removals: Array = []; var rolls: Array = []; var catalysts: Array = []
	var excess_by_actor := {}
	for step in _array(summary.get("steps", [])):
		var data: Dictionary = step.get("data", {})
		if step.get("type") == "proposal_batch_committed" and data.has("rolls"): rolls.append_array(_array(data.rolls))
		if step.get("type") == "dice_rolled" and data.get("source_type") == "catalyst": catalysts.append(data)
		if step.get("type") == "damage_committed":
			sources.append_array(_array(data.get("sources", []))); removals.append_array(_array(data.get("removals", [])))
			for actor in data.get("overage", {}):
				excess_by_actor[actor] = int(excess_by_actor.get(actor, 0)) + int(data.overage[actor])
	var assigned := {}
	for index in rolls.size():
		var roll: Dictionary = rolls[index]; var face := int(roll.get("die", {}).get("face", 0))
		var entry := {"actor_id": str(roll.actor_id), "status_id": str(roll.source_content_id), "face": face, "original_face": face, "catalyst": false, "damage": 0, "sources": [], "cards": []}
		for catalyst in catalysts:
			if int(catalyst.get("die_index", -1)) == index:
				entry.original_face = int(catalyst.face_before); entry.catalyst = true
				entry.catalyst_holder = str(catalyst.get("holder", ""))
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
		entry.excess = mini(maxi(0, int(entry.damage) - entry.cards.size()), int(excess_by_actor.get(entry.actor_id, 0)))
		excess_by_actor[entry.actor_id] = int(excess_by_actor.get(entry.actor_id, 0)) - int(entry.excess)

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
	present_progress()
	if playback_elapsed() >= duration: _done = true; finished.emit()

func present_progress() -> void:
	# Repaint the saved playback position even while inspection pauses the clock.
	# Completion is dispatched only by _process, never during a screen rebuild.
	var elapsed := _visual_timeline(maxf(0.0, playback_elapsed()))
	var gathering := clampf(1.0 + playback_elapsed() / maxf(0.01, preload("res://presentation/battle/combat_timing.gd").effects_gather()), 0.0, 1.0)
	for group in groups.values(): group.get_parent().get_parent().modulate.a = gathering
	_phase.text = ("Rolling effects" if _has_dice() else "Bleed") if elapsed < 1.6 else "Catalyst" if elapsed < 2.9 and _has_catalyst() else "Damage" if elapsed < 4.8 else "Effects settle"
	for entry in entries:
		if not entry.has("die"): continue
		if entry.face > 0:
			var rolling: bool = elapsed < 0.9 or (entry.catalyst and elapsed >= 2.1 and elapsed < 2.65)
			entry.die.text = str(1 + int(elapsed * 19) % 6) if rolling else str(entry.original_face if elapsed < 2.1 else entry.face)
			entry.die.rotation = sin(elapsed * 32) * 0.08 if rolling else 0.0
			entry.die.modulate = Color("d0a6ff") if entry.catalyst and elapsed >= 1.6 and elapsed < 2.9 else Color.WHITE
			entry.result.add_theme_color_override("font_color", Color("c9f6a5") if entry.catalyst and elapsed >= 1.6 and elapsed < 2.9 else Color.WHITE)
			entry.result.text = "Catalyst ↻" if entry.catalyst and elapsed >= 1.6 and elapsed < 2.9 else ""
		if elapsed >= 2.9:
			entry.result.text = "%d damage" % int(entry.damage) if entry.damage > 0 else "Clears" if _clears(entry) else "No damage"
		if elapsed > 5.2 and not entry.cards.is_empty(): entry.result.text = "%d card%s lost" % [entry.cards.size(), "" if entry.cards.size() == 1 else "s"]
		if elapsed > 5.2 and entry.excess > 0: entry.result.text = "%d cards lost · %d excess" % [entry.cards.size(), entry.excess]
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
			if entry.excess > 0: entry.result.text += " · %d excess" % entry.excess
		if elapsed > 5.2 and _clears(entry): entry.die.modulate.a = 1 - clampf((elapsed - 5.2) / 0.65, 0, 1)
	if playback_elapsed() < 0: _phase.text = "Effects activate"
	_present_profile_progress(elapsed)
	if entries.is_empty(): _phase.text = "Effects settle"

func _present_profile_progress(elapsed: float) -> void:
	if entries.is_empty():
		phase_changed.emit("statuses", clampf(elapsed / 0.7, 0, 1))
	else:
		phase_changed.emit("cards" if elapsed < 5.2 else "statuses", clampf((elapsed - (4.5 if elapsed < 5.2 else 5.2)) / 0.7, 0, 1))

func catalyst_cue_seconds() -> float:
	return maxf(0.1, TIMING.seconds("catalyst_cue_seconds", 0.9))

func catalyst_roll_seconds() -> float:
	return maxf(0.1, TIMING.seconds("catalyst_reroll_seconds", 0.9))

func catalyst_extra_seconds() -> float:
	return catalyst_cue_seconds() + catalyst_roll_seconds() - 1.05 if _has_catalyst() else 0.0

func _visual_timeline(elapsed: float) -> float:
	# Stretch only the Catalyst cue and second roll. Shift every subsequent
	# card/status update together so none can settle before the dice land.
	if not _has_catalyst() or elapsed < 1.6: return elapsed
	var cue := catalyst_cue_seconds()
	var roll := catalyst_roll_seconds()
	if elapsed < 1.6 + cue: return 1.6 + (elapsed - 1.6) * 0.5 / cue
	if elapsed < 1.6 + cue + roll: return 2.1 + (elapsed - 1.6 - cue) * 0.55 / roll
	return elapsed - catalyst_extra_seconds()

func catalyst_trails() -> Array[Dictionary]:
	var trails: Array[Dictionary] = []
	var elapsed := playback_elapsed() - 1.6
	var cue := catalyst_cue_seconds()
	var end := cue + catalyst_roll_seconds()
	if elapsed < 0.0 or elapsed >= end: return trails
	for entry in entries:
		if not entry.has("die"): continue
		if not entry.catalyst or str(entry.get("catalyst_holder", "")).is_empty(): continue
		trails.append({"holder": entry.catalyst_holder, "target": entry.die, "travel": clampf(elapsed / (cue * 0.8), 0.0, 1.0), "alpha": minf(1.0, (end - elapsed) / 0.3)})
	return trails

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

func _counts(actor: Dictionary) -> Dictionary:
	var result := {}
	for status in _array(actor.get("statuses", [])): result[str(status.definition_id)] = int(status.stacks)
	return result

func _matches_face(face: int, faces: Array) -> bool:
	for value in faces:
		if int(value) == face: return true
	return false
