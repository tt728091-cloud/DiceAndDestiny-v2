class_name ActorProfile
extends PanelContainer

var portrait: TextureRect
var title: Label
var health: ProgressBar
var stats: Label
var statuses: Label
var _health_text: Label
var _stat_labels: Dictionary = {}
var _income_markers: Dictionary = {}
var _display_values: Dictionary = {}
var _income_start_values: Dictionary = {}
var _income_final_values: Dictionary = {}

const NORMAL_STAT_COLOR := Color("e6e8ec")
const INCOME_HIGHLIGHT_COLOR := Color("ffd36a")

func _ready() -> void:
	custom_minimum_size = Vector2(250, 205)
	var row := HBoxContainer.new()
	add_child(row)
	portrait = TextureRect.new()
	portrait.custom_minimum_size = Vector2(100, 170)
	portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	row.add_child(portrait)
	var info := VBoxContainer.new()
	info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(info)
	title = Label.new(); title.add_theme_font_size_override("font_size", 22); info.add_child(title)
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
	statuses = Label.new(); statuses.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; info.add_child(statuses)

func _add_stat_cell(parent: HBoxContainer, key: String, caption: String, font_size: int = 15) -> void:
	var cell := VBoxContainer.new(); cell.alignment = BoxContainer.ALIGNMENT_CENTER; cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL; parent.add_child(cell)
	var value := Label.new(); value.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; value.add_theme_font_size_override("font_size", font_size); value.set_meta("caption", caption); cell.add_child(value)
	var marker := Label.new(); marker.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; marker.add_theme_font_size_override("font_size", 11); marker.add_theme_color_override("font_color", INCOME_HIGHLIGHT_COLOR); marker.visible = false; cell.add_child(marker)
	_stat_labels[key] = value
	_income_markers[key] = marker

func display(actor_id: String, actor: Dictionary, is_player: bool) -> void:
	var definition := str(actor.get("definition_id", actor_id))
	title.text = BattlePresentationCatalog.ability(definition).get("name", definition.replace("_", " ").capitalize())
	if definition == "blade_warden": title.text = "Blade Warden"
	if definition == "venom_goblin": title.text = "Venom Goblin"
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
	for entry in actor.get("statuses", []):
		var id := str(entry.get("definition_id", entry.get("id", "status")))
		status_text.append("%s %s ×%d" % [BattlePresentationCatalog.status(id).glyph, BattlePresentationCatalog.status(id).name, int(entry.get("stacks", 1))])
	statuses.text = "No active statuses" if status_text.is_empty() else "\n".join(status_text)
	var path := "res://assets/battle/portraits/blade_warden.png" if is_player else "res://assets/battle/portraits/venom_goblin.png"
	if ResourceLoader.exists(path): portrait.texture = load(path)
	tooltip_text = "%s authoritative profile" % title.text

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
