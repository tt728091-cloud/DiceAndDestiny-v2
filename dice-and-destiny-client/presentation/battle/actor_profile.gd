class_name ActorProfile
extends PanelContainer

var portrait: TextureRect
var title: Label
var health: ProgressBar
var stats: Label
var statuses: Label
var pending_statuses: Label
var _health_text: Label
var _stat_labels: Dictionary = {}
var _income_markers: Dictionary = {}
var _display_values: Dictionary = {}
var _income_start_values: Dictionary = {}
var _income_final_values: Dictionary = {}
var _status_entries: Array = []
var _defense_status_preview: Dictionary = {}

const NORMAL_STAT_COLOR := Color("e6e8ec")
const INCOME_HIGHLIGHT_COLOR := Color("ffd36a")
const VISUALS := preload("res://content/battle_visuals/library.tres")

func _ready() -> void:
	custom_minimum_size = Vector2(300, 150)
	add_theme_stylebox_override("panel", preload("res://presentation/battle/cinematic_theme.gd").panel(Color("08080860"), Color("00000000"), 10))
	var row := HBoxContainer.new()
	add_child(row)
	portrait = TextureRect.new()
	portrait.custom_minimum_size = Vector2.ZERO
	portrait.visible = false
	portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	row.add_child(portrait)
	var info := VBoxContainer.new()
	info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(info)
	title = Label.new(); title.add_theme_font_size_override("font_size", 28); title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT; title.clip_text = true; info.add_child(title)
	health = ProgressBar.new(); health.show_percentage = false; health.custom_minimum_size.y = 18; info.add_child(health)
	var primary_stats := HBoxContainer.new(); primary_stats.add_theme_constant_override("separation", 10); info.add_child(primary_stats)
	_health_text = Label.new(); _health_text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; primary_stats.add_child(_health_text)
	_add_stat_cell(primary_stats, "energy", "✦ Energy")
	var zone_stats := HBoxContainer.new(); zone_stats.add_theme_constant_override("separation", 6); info.add_child(zone_stats)
	for entry in [["deck", "Deck"], ["hand", "Hand"], ["discard", "Discard"], ["removed", "Removed"]]:
		_add_stat_cell(zone_stats, str(entry[0]), str(entry[1]), 13)
	# Kept as a public compatibility field for callers that previously inspected
	# the combined stats label. The visible profile now uses individual values so
	# income changes can be highlighted without moving the profile.
	stats = Label.new(); stats.visible = false; info.add_child(stats)
	statuses = preload("res://presentation/battle/status_tooltip_label.gd").new(); statuses.add_theme_font_size_override("font_size", 18); statuses.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; info.add_child(statuses)
	pending_statuses = Label.new(); pending_statuses.add_theme_font_size_override("font_size", 18)
	pending_statuses.add_theme_color_override("font_color", Color("c1eca0"))
	pending_statuses.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; pending_statuses.visible = false; info.add_child(pending_statuses)

func _add_stat_cell(parent: HBoxContainer, key: String, caption: String, font_size: int = 15) -> void:
	var cell := VBoxContainer.new(); cell.alignment = BoxContainer.ALIGNMENT_CENTER; cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL; parent.add_child(cell)
	var value := Label.new(); value.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; value.add_theme_font_size_override("font_size", font_size); value.set_meta("caption", caption); cell.add_child(value)
	var marker := Label.new(); marker.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; marker.add_theme_font_size_override("font_size", 11); marker.add_theme_color_override("font_color", INCOME_HIGHLIGHT_COLOR); marker.visible = false; cell.add_child(marker)
	_stat_labels[key] = value
	_income_markers[key] = marker

func display(actor_id: String, actor: Dictionary, is_player: bool) -> void:
	_defense_status_preview.clear()
	pending_statuses.text = ""; pending_statuses.hide(); statuses.show()
	statuses.remove_theme_color_override("font_color")
	var definition := str(actor.get("definition_id", actor_id))
	var visual: FighterVisualProfile = VISUALS.fighter(definition)
	title.text = visual.display_name if visual != null else definition.replace("_", " ").capitalize()
	var current := int(actor.get("current_health", 0)); var maximum := maxi(1, int(actor.get("max_health", current)))
	health.max_value = maximum; health.value = current
	_health_text.text = "Health %d/%d" % [current, maximum]
	_display_values = {
		"energy": int(actor.get("energy_points", 0)),
		"deck": int(actor.get("deck_count", 0)),
		"hand": int(actor.get("hand_count", 0)),
		"discard": int(actor.get("discard_count", 0)),
		"removed": int(actor.get("removed_count", 0)),
	}
	_refresh_stat_labels()
	stats.text = "Health %d/%d    ✦ Energy %d\nDeck %d  Hand %d  Discard %d  Removed %d" % [current, maximum, int(_display_values.energy), int(_display_values.deck), int(_display_values.hand), int(_display_values.discard), int(_display_values.removed)]
	var status_text: Array[String] = []
	_status_entries = actor.get("statuses", []).duplicate(true)
	for entry in actor.get("statuses", []):
		var id := str(entry.get("definition_id", entry.get("id", "status")))
		status_text.append("%s %s ×%d" % [BattlePresentationCatalog.status(id).glyph, BattlePresentationCatalog.status(id).name, int(entry.get("stacks", 1))])
	statuses.text = "No active statuses" if status_text.is_empty() else "\n".join(status_text)
	portrait.texture = visual.portrait if visual != null else null
	tooltip_text = "%s
%s" % [title.text, stats.text]
	var rules: Array[String] = []
	for entry in _status_entries:
		var data := BattlePresentationCatalog.status(str(entry.get("definition_id", "")))
		rules.append("%s — %s" % [data.name, data.text])
	statuses.tooltip_text = "\n\n".join(rules)
	statuses.mouse_filter = Control.MOUSE_FILTER_STOP

func show_pending_applications(applications: Dictionary) -> void:
	# These are queued additions, separate from stacks already on the character.
	# A separate label also keeps card/status animations from erasing the preview.
	var lines: Array[String] = []
	for id in applications:
		var count := int(applications[id])
		if count <= 0: continue
		var data := BattlePresentationCatalog.status(str(id))
		lines.append("%s %s ×%d · pending" % [data.glyph, data.name, count])
	pending_statuses.text = "\n".join(lines)
	pending_statuses.visible = not lines.is_empty()
	statuses.visible = not _status_entries.is_empty() or lines.is_empty()

func prepare_card_cleanse(status_id: String, before: int) -> Label:
	# Keep the actual final text for settlement; only the affected row fades.
	var final_text := statuses.text
	var other_statuses: Array[String] = []
	for entry in _status_entries:
		var id := str(entry.get("definition_id", entry.get("id", "")))
		if id == status_id: continue
		var data := BattlePresentationCatalog.status(id)
		other_statuses.append("%s %s ×%d" % [data.glyph, data.name, int(entry.get("stacks", 1))])
	statuses.text = "\n".join(other_statuses)
	statuses.visible = not other_statuses.is_empty()
	var data := BattlePresentationCatalog.status(status_id)
	var fading := Label.new(); fading.name = "CleansedStatus"
	fading.text = "%s %s ×%d" % [data.glyph, data.name, before]
	fading.add_theme_font_size_override("font_size", 18)
	fading.add_theme_color_override("font_color", Color("afe079"))
	fading.set_meta("final_text", final_text)
	statuses.get_parent().add_child(fading)
	return fading

func finish_card_cleanse(fading: Label) -> void:
	# Keep the invisible row alive until the card leaves, so its trail retains
	# a stable destination and the profile does not resize during the effect.
	statuses.text = str(fading.get_meta("final_text", "No active statuses"))
	statuses.show()
	fading.modulate.a = 0.0

func show_defense_status_preview(status_id: String, count: int) -> void:
	_defense_status_preview[status_id] = count
	var lines: Array[String] = []
	for entry in _status_entries:
		var id := str(entry.get("definition_id", entry.get("id", "")))
		if _defense_status_preview.has(id): continue
		var data := BattlePresentationCatalog.status(id)
		lines.append("%s %s ×%d" % [data.glyph, data.name, int(entry.get("stacks", 1))])
	for id in _defense_status_preview:
		var data := BattlePresentationCatalog.status(str(id))
		lines.append("%s %s ×%d · pending" % [data.glyph, data.name, int(_defense_status_preview[id])])
	statuses.text = "\n".join(lines)
	statuses.add_theme_color_override("font_color", Color("c1eca0"))

func prepare_income(actor_income: Dictionary) -> void:
	_income_final_values = _display_values.duplicate(true)
	_income_start_values = _pre_income_values(actor_income)
	var card_count := int(actor_income.get("card_count", 0))
	var energy_gain := int(actor_income.get("energy_gain", 1 if actor_income.has("energy_points") else 0))
	if energy_gain > 0:
		_show_income_marker("energy", "+%d" % energy_gain)
	if card_count > 0:
		_show_income_marker("deck", "−%d" % card_count)
		_show_income_marker("hand", "+%d" % card_count)
	_display_values = _income_start_values.duplicate(true)
	_refresh_stat_labels()

func prepare_before_income(actor_income: Dictionary) -> void:
	_display_values = _pre_income_values(actor_income)
	_refresh_stat_labels()

func _pre_income_values(actor_income: Dictionary) -> Dictionary:
	var values := _display_values.duplicate(true)
	var card_count := int(actor_income.get("card_count", 0))
	var energy_gain := int(actor_income.get("energy_gain", 1 if actor_income.has("energy_points") else 0))
	if energy_gain > 0: values["energy"] = maxi(0, int(values.energy) - energy_gain)
	if card_count > 0:
		values["deck"] = int(values.deck) + card_count
		values["hand"] = maxi(0, int(values.hand) - card_count)
	return values

func animate_income(duration_seconds: float) -> void:
	if _income_final_values.is_empty(): return
	var duration := maxf(0.05, duration_seconds)
	var tween := create_tween().bind_node(self)
	tween.tween_interval(duration * 0.18)
	tween.tween_method(_apply_income_progress, 0.0, 1.0, duration * 0.32).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)
	tween.tween_callback(_settle_income_values)
	tween.tween_method(_fade_income_highlights, 0.0, 1.0, duration * 0.35)
	tween.tween_callback(_finish_income_animation)

func _show_income_marker(key: String, change: String) -> void:
	var marker: Label = _income_markers.get(key)
	var value: Label = _stat_labels.get(key)
	if marker == null or value == null: return
	marker.text = "▲ %s" % change; marker.visible = true; marker.modulate = Color.WHITE
	value.add_theme_color_override("font_color", INCOME_HIGHLIGHT_COLOR)

func _apply_income_progress(progress: float) -> void:
	for key in _income_final_values:
		var start := int(_income_start_values.get(key, _income_final_values[key]))
		var finish := int(_income_final_values[key])
		_display_values[key] = roundi(lerpf(float(start), float(finish), progress))
	_refresh_stat_labels()

func _settle_income_values() -> void:
	_display_values = _income_final_values.duplicate(true)
	_refresh_stat_labels()

func _fade_income_highlights(progress: float) -> void:
	for key in _income_markers:
		var marker: Label = _income_markers[key]
		if not marker.visible: continue
		marker.modulate.a = 1.0 - progress
		var value: Label = _stat_labels[key]
		value.add_theme_color_override("font_color", INCOME_HIGHLIGHT_COLOR.lerp(NORMAL_STAT_COLOR, progress))

func _finish_income_animation() -> void:
	for key in _income_markers:
		var marker: Label = _income_markers[key]; marker.visible = false; marker.modulate = Color.WHITE
		var value: Label = _stat_labels[key]; value.add_theme_color_override("font_color", NORMAL_STAT_COLOR)

func _refresh_stat_labels() -> void:
	for key in _stat_labels:
		var label: Label = _stat_labels[key]
		label.text = "%s %d" % [str(label.get_meta("caption", key.capitalize())), int(_display_values.get(key, 0))]

func show_effects_progress(before: Dictionary, after: Dictionary, phase: String, progress: float) -> void:
	var cards_progress := progress if phase == "cards" else 1.0
	var current := roundi(lerpf(float(before.get("health", 0)), float(after.get("health", 0)), cards_progress))
	health.value = current; _health_text.text = "Health %d/%d" % [current, int(health.max_value)]
	for key in ["deck", "hand", "discard", "removed"]:
		_display_values[key] = roundi(lerpf(float(before.get(key + "_count", 0)), float(after.get(key + "_count", 0)), cards_progress))
	_display_values.energy = int(before.get("energy", _display_values.energy)); _refresh_stat_labels()
	var initial := {}; var final := {}
	for status in before.get("statuses", []) if before.get("statuses") is Array else []: initial[str(status.definition_id)] = int(status.stacks)
	for status in after.get("statuses", []) if after.get("statuses") is Array else []: final[str(status.definition_id)] = int(status.stacks)
	for id in final:
		if not initial.has(id): initial[id] = 0
	var lines: Array[String] = []
	for id in initial:
		var count := roundi(lerpf(float(initial[id]), float(final.get(id, 0)), progress if phase == "statuses" else 0.0))
		if count <= 0: continue
		var data := BattlePresentationCatalog.status(str(id)); lines.append("%s %s ×%d" % [data.glyph, data.name, count])
	statuses.text = "No active statuses" if lines.is_empty() else "\n".join(lines)
	statuses.modulate = Color.WHITE.lerp(Color("b5e591"), sin(progress * PI)) if phase == "statuses" else Color.WHITE

func show_resource_gain(data: Dictionary, progress: float) -> void:
	var key := str(data.get("stat", ""))
	var label: Label = _stat_labels.get(key)
	if label == null or int(_display_values.get(key, -1)) != int(data.get("after", -2)): return
	var amount := roundi(lerpf(float(data.before), float(data.after), progress))
	label.text = "%s %d" % [str(label.get_meta("caption", "")), amount]
	label.add_theme_color_override("font_color", NORMAL_STAT_COLOR.lerp(INCOME_HIGHLIGHT_COLOR, sin(progress * PI)))

func show_status_transition(update: Dictionary, progress: float) -> void:
	var counts := {}
	for entry in _status_entries:
		counts[str(entry.get("definition_id", ""))] = int(entry.get("stacks", 0))
	var data: Dictionary = update.data
	var applied_id := str(data.get("status_id", "incubation"))
	# A later action may already have changed the target again. Never restore
	# stale counts merely to finish an older visual effect.
	var current := int(counts.get(applied_id, 0)) == int(data.get("after", 0)) if update.kind != "conversion" else int(counts.get("poison", 0)) == int(data.poison_after) and int(counts.get("volatile_poison", 0)) == int(data.volatile_after)
	if not current: statuses.modulate = Color.WHITE; return
	if update.kind != "conversion":
		counts[applied_id] = roundi(lerpf(float(data.before), float(data.after), progress))
	else:
		counts.poison = roundi(lerpf(float(data.poison_before), float(data.poison_after), progress))
		counts.volatile_poison = roundi(lerpf(float(data.volatile_before), float(data.volatile_after), progress))
	var lines: Array[String] = []
	for id in counts:
		if int(counts[id]) <= 0: continue
		var status := BattlePresentationCatalog.status(str(id))
		lines.append("%s %s ×%d" % [status.glyph, status.name, int(counts[id])])
	statuses.text = "No active statuses" if lines.is_empty() else "\n".join(lines)
	statuses.modulate = Color.WHITE.lerp(Color("d5afff") if update.kind == "conversion" else Color("b7e39a"), sin(progress * PI))

func status_anchor(status_id: String) -> Vector2:
	# Statuses share a multiline label. Locate the displayed row rather than
	# pointing to the center of the entire list (or to stale snapshot ordering).
	var data := BattlePresentationCatalog.status(status_id)
	var lines := statuses.text.split("\n")
	var font := statuses.get_theme_font("font")
	var font_size := statuses.get_theme_font_size("font_size")
	var line_height := font.get_height(font_size) + statuses.get_theme_constant("line_spacing")
	for index in lines.size():
		if not lines[index].begins_with("%s %s ×" % [data.glyph, data.name]): continue
		var width := minf(statuses.size.x, font.get_string_size(lines[index], HORIZONTAL_ALIGNMENT_LEFT, -1, font_size).x)
		return statuses.get_global_transform_with_canvas() * Vector2(width * 0.5, line_height * (index + 0.5))
	return statuses.get_global_rect().get_center()
