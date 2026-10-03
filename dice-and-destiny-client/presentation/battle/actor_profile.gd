class_name ActorProfile
extends PanelContainer

signal pile_requested(zone: String)
var _player_piles := false

var portrait: TextureRect
var title: Label
var health: ProgressBar
var stats: Label
var statuses
var pending_statuses
var _health_text: Label
var _stat_labels: Dictionary = {}
var _stat_cells: Dictionary = {}
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const STATUS_STRIP := preload("res://presentation/battle/status_icon_strip.gd")
const HUD_THEME := preload("res://presentation/battle/cinematic_theme.gd")
var _income_markers: Dictionary = {}
var _display_values: Dictionary = {}
var _income_start_values: Dictionary = {}
var _income_final_values: Dictionary = {}
var _status_entries: Array = []
var _defense_status_preview: Dictionary = {}
var compact := false

const NORMAL_STAT_COLOR := HUD_THEME.HUD_IVORY
const INCOME_HIGHLIGHT_COLOR := Color("ffd36a")
const STAT_HINTS := {"energy": "Available to spend on cards and abilities.", "deck": "Cards available to draw. Damage uses these after discard. No automatic refill.", "hand": "Cards currently held.", "discard": "Still counts as health. Damage takes these first, then draw, then hand. No automatic redraw.", "removed": "Cards permanently removed from this battle."}
const VISUALS := preload("res://content/battle_visuals/library.tres")

func _ready() -> void:
	compact = get_parent() is Control and get_parent().size.x < 270
	mouse_filter = Control.MOUSE_FILTER_PASS
	add_theme_stylebox_override("panel", preload("res://presentation/battle/cinematic_theme.gd").panel(Color.TRANSPARENT, Color.TRANSPARENT, 6))
	var info := VBoxContainer.new(); info.add_theme_constant_override("separation", 3); add_child(info)
	portrait = TextureRect.new(); portrait.hide(); info.add_child(portrait)
	var heading := HBoxContainer.new(); heading.alignment = BoxContainer.ALIGNMENT_CENTER; info.add_child(heading)
	title = Label.new(); title.add_theme_font_size_override("font_size", 18 if compact else 24); title.clip_text = true
	HUD_THEME.hud_lettering(title, true)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; heading.add_child(title)
	_add_stat_cell(heading, "energy", "Energy", 20)
	health = ProgressBar.new(); health.show_percentage = false; health.custom_minimum_size.y = 24; info.add_child(health)
	var style := StyleBoxFlat.new(); style.bg_color = Color("211923"); style.border_color = Color("100e13"); style.set_border_width_all(3); style.set_corner_radius_all(6)
	style.shadow_color = Color("09080ccc"); style.shadow_size = 3; style.shadow_offset = Vector2(0, 2)
	health.add_theme_stylebox_override("background", style)
	var fill := StyleBoxFlat.new(); fill.bg_color = Color("cf424f"); fill.border_color = Color("f48279"); fill.set_border_width_all(2); fill.set_corner_radius_all(4)
	# Leave the dark outer frame visible even at full health.
	fill.expand_margin_left = -3; fill.expand_margin_right = -3; fill.expand_margin_top = -3; fill.expand_margin_bottom = -3
	health.add_theme_stylebox_override("fill", fill)
	_health_text = Label.new(); _health_text.add_theme_font_size_override("font_size", 21); _health_text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	HUD_THEME.hud_lettering(_health_text, true)
	_health_text.mouse_filter = Control.MOUSE_FILTER_IGNORE; health.add_child(_health_text); _health_text.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	statuses = STATUS_STRIP.new(); info.add_child(statuses)
	pending_statuses = STATUS_STRIP.new(); pending_statuses.hide(); info.add_child(pending_statuses)
	var zones := HBoxContainer.new(); zones.alignment = BoxContainer.ALIGNMENT_CENTER; zones.add_theme_constant_override("separation", 7 if compact else 12); info.add_child(zones)
	for key in ["deck", "hand", "discard", "removed"]: _add_stat_cell(zones, key, key.capitalize(), 19)
	stats = Label.new(); stats.hide(); info.add_child(stats)

func _add_stat_cell(parent: HBoxContainer, key: String, caption: String, font_size: int = 19) -> void:
	var cell := HBoxContainer.new(); cell.alignment = BoxContainer.ALIGNMENT_CENTER; cell.add_theme_constant_override("separation", 4); cell.tooltip_text = caption; parent.add_child(cell)
	cell.gui_input.connect(_pile_input.bind(key))
	var icon := TextureRect.new(); icon.texture = ICONS.texture(key); icon.custom_minimum_size = Vector2(18, 22) if compact else Vector2(24, 24); icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE; cell.add_child(icon)
	var value := Label.new(); value.add_theme_font_size_override("font_size", 16 if compact else font_size); value.set_meta("caption", ""); value.mouse_filter = Control.MOUSE_FILTER_IGNORE; cell.add_child(value)
	HUD_THEME.hud_lettering(value, true)
	# Floating deltas never change the size or position of the HUD's anchors.
	var marker := Label.new(); marker.position = Vector2(0, -24); marker.add_theme_font_size_override("font_size", 14); marker.mouse_filter = Control.MOUSE_FILTER_IGNORE; marker.hide(); value.add_child(marker)
	HUD_THEME.hud_lettering(marker, true)
	_stat_labels[key] = value; _stat_cells[key] = cell; _income_markers[key] = marker

## All rectangles are in canvas coordinates. Callers convert to their own
## canvas once, at draw time; never cache a pixel endpoint across frames.
func anchor_rect(kind: String, status_id: String = "") -> Rect2:
	if kind == "status": return statuses.bounds(status_id)
	if kind == "pending_status": return pending_statuses.bounds(status_id)
	if kind == "health": return health.get_global_rect()
	if kind == "name": return title.get_global_rect()
	if _stat_cells.has(kind): return _stat_cells[kind].get_global_rect()
	return get_global_rect()

func _pile_input(event: InputEvent, zone: String) -> void:
	if not _player_piles or zone not in ["deck", "discard", "removed"]: return
	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
		_stat_cells[zone].accept_event()
		pile_requested.emit(zone)

func display(actor_id: String, actor: Dictionary, is_player: bool) -> void:
	_player_piles = is_player
	for zone in ["deck", "discard", "removed"]:
		_stat_cells[zone].mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND if is_player else Control.CURSOR_ARROW
	_defense_status_preview.clear()
	pending_statuses.set_counts({}); pending_statuses.hide(); statuses.show()
	statuses.remove_theme_color_override("font_color")
	var definition := str(actor.get("definition_id", actor_id))
	var visual: FighterVisualProfile = VISUALS.fighter(definition)
	title.text = visual.display_name if visual != null else definition.replace("_", " ").capitalize()
	var current := int(actor.get("current_health", 0)); var maximum := maxi(1, int(actor.get("max_health", current)))
	health.max_value = maximum; health.value = current
	_health_text.text = "%d/%d" % [current, maximum]
	_display_values = {
		"energy": int(actor.get("energy_points", 0)),
		"deck": int(actor.get("deck_count", 0)),
		"hand": int(actor.get("hand_count", 0)),
		"discard": int(actor.get("discard_count", 0)),
		"removed": int(actor.get("removed_count", 0)),
	}
	_refresh_stat_labels()
	stats.text = "Health %d/%d    ✦ Energy %d\nDeck %d  Hand %d  Discard %d  Removed %d" % [current, maximum, int(_display_values.energy), int(_display_values.deck), int(_display_values.hand), int(_display_values.discard), int(_display_values.removed)]
	_status_entries = actor.get("statuses", []).duplicate(true)
	var values := {}
	for entry in _status_entries: values[str(entry.get("definition_id", entry.get("id", "status")))] = int(entry.get("stacks", 1))
	statuses.set_counts(values)
	portrait.texture = visual.portrait if visual != null else null
	tooltip_text = "%s
%s" % [title.text, stats.text]
	var rules: Array[String] = []
	for entry in _status_entries:
		var data := BattlePresentationCatalog.status(str(entry.get("definition_id", "")))
		rules.append("%s — %s" % [data.name, data.text])
	statuses.tooltip_text = "\n\n".join(rules)
	statuses.mouse_filter = Control.MOUSE_FILTER_PASS

func show_pending_applications(applications: Dictionary) -> void:
	pending_statuses.set_counts(applications, true)
	pending_statuses.visible = applications.values().any(func(count): return int(count) > 0)
	statuses.show()

func prepare_card_cleanse(status_id: String, before: int) -> Label:
	var final_counts: Dictionary = statuses.counts.duplicate()
	var shown := final_counts.duplicate(); shown[status_id] = before; statuses.set_counts(shown)
	var slot: Control = statuses.ensure_slot(status_id)
	var count: Label = slot.get_node("Count")
	var fading := Label.new(); fading.name = "CleansedStatus"; fading.text = str(before)
	HUD_THEME.hud_lettering(fading, true)
	fading.add_theme_font_size_override("font_size", 20); fading.mouse_filter = Control.MOUSE_FILTER_IGNORE
	count.add_child(fading); fading.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); count.self_modulate.a = 0
	fading.set_meta("final_counts", final_counts); fading.set_meta("status_id", status_id)
	fading.set_meta("fading", true)
	return fading

func _process(_delta: float) -> void:
	for slot in statuses.cells.values():
		var count: Label = slot.get_node("Count")
		var ghost := count.get_node_or_null("CleansedStatus") as Label
		if is_instance_valid(ghost) and ghost.get_meta("fading", false): slot.get_child(0).modulate.a = ghost.modulate.a

func finish_card_cleanse(fading: Label) -> void:
	statuses.set_counts(fading.get_meta("final_counts", {}))
	fading.set_meta("fading", false); fading.modulate.a = 0
	var slot: Control = statuses.ensure_slot(str(fading.get_meta("status_id")))
	slot.get_node("Count").self_modulate.a = 1; slot.get_child(0).modulate.a = 1

func show_resource_preview(stat: String, value: int) -> void:
	_display_values[stat] = value
	_refresh_stat_labels()

func show_defense_status_preview(status_id: String, count: int) -> void:
	_defense_status_preview[status_id] = count
	var values := {}
	for entry in _status_entries: values[str(entry.get("definition_id", ""))] = int(entry.get("stacks", 1))
	values.merge(_defense_status_preview, true)
	statuses.set_counts(values, false, _defense_status_preview)

func prepare_income(actor_income: Dictionary) -> void:
	_income_final_values = _display_values.duplicate(true)
	_income_start_values = _pre_income_values(actor_income)
	var card_count := int(actor_income.get("card_count", 0))
	var energy_gain := int(actor_income.get("energy_gain", 1 if actor_income.has("energy_points") else 0))
	if energy_gain > 0:
		_show_income_marker("energy", "+%d" % energy_gain)
	if actor_income.get("grave_debt", false):
		_show_income_marker("energy", "Grave Debt −%d\n+%d Energy · consumed" % [int(actor_income.get("energy_prevented", 0)), energy_gain])
		_income_markers.energy.text = _income_markers.energy.text.trim_prefix("▲ ")
		_income_markers.energy.add_theme_color_override("font_color", Color("e2b2ff"))
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
	marker.add_theme_color_override("font_color", INCOME_HIGHLIGHT_COLOR)
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
		label.text = str(int(_display_values.get(key, 0)))
		_stat_cells[key].tooltip_text = "%s · %s\n%s" % [key.capitalize(), label.text, STAT_HINTS[key]]
		if _player_piles and key in ["deck", "discard", "removed"]: _stat_cells[key].tooltip_text += "\nClick to view cards."

func show_effects_progress(before: Dictionary, after: Dictionary, phase: String, progress: float) -> void:
	var cards_progress := progress if phase == "cards" else 1.0
	var current := roundi(lerpf(float(before.get("health", 0)), float(after.get("health", 0)), cards_progress))
	health.value = current; _health_text.text = "%d/%d" % [current, int(health.max_value)]
	for key in ["deck", "hand", "discard", "removed"]:
		_display_values[key] = roundi(lerpf(float(before.get(key + "_count", 0)), float(after.get(key + "_count", 0)), cards_progress))
	_display_values.energy = int(before.get("energy", _display_values.energy)); _refresh_stat_labels()
	var initial := {}; var final := {}
	for status in before.get("statuses", []) if before.get("statuses") is Array else []: initial[str(status.definition_id)] = int(status.stacks)
	for status in after.get("statuses", []) if after.get("statuses") is Array else []: final[str(status.definition_id)] = int(status.stacks)
	for id in final:
		if not initial.has(id): initial[id] = 0
	var values := {}
	for id in initial: values[id] = roundi(lerpf(float(initial[id]), float(final.get(id, 0)), progress if phase == "statuses" else 0.0))
	statuses.set_counts(values)
	statuses.modulate = Color.WHITE.lerp(Color("b5e591"), sin(progress * PI)) if phase == "statuses" else Color.WHITE

func show_resource_gain(data: Dictionary, progress: float) -> void:
	var key := str(data.get("stat", ""))
	var label: Label = _stat_labels.get(key)
	if label == null or int(_display_values.get(key, -1)) != int(data.get("after", -2)): return
	var amount := roundi(lerpf(float(data.before), float(data.after), progress))
	label.text = str(amount)
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
	statuses.set_counts(counts)
	statuses.modulate = Color.WHITE.lerp(Color("d5afff") if update.kind == "conversion" else Color("b7e39a"), sin(progress * PI))

func status_anchor(status_id: String) -> Vector2:
	return anchor_rect("status", status_id).get_center()
