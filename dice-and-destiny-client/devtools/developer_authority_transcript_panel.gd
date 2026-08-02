class_name DeveloperAuthorityTranscriptPanel
extends CanvasLayer

const ENABLE_ENV := "DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT"
const PATH_ENV := "DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH"
const PROJECT_SETTING := "dice_and_destiny/development/enable_authority_transcript"
const RELATIVE_PATH := "debug/authority-transcript.jsonl"
const MAX_BUFFERED_RECORDS := 3000
const MAX_RENDERED_RECORDS := 600
const POLL_SECONDS := 0.2

var current_battle_id := ""
var _path := ""
var _offset := 0
var _poll_elapsed := 0.0
var _records: Array[Dictionary] = []
var _visible_records: Array[Dictionary] = []
var _actor_names: Dictionary = {}
var _open := false
var _paused := false
var _follow_latest := true
var _filter := "All"
var _search := ""
var _last_error := ""

var _overlay: Control
var _panel: PanelContainer
var _text: RichTextLabel
var _battle_label: Label
var _path_label: Label
var _status_label: Label
var _follow_check: CheckBox
var _pause_check: CheckBox


static func is_enabled() -> bool:
	if not OS.is_debug_build():
		return false
	if OS.get_environment(ENABLE_ENV) != "1":
		return false
	if not bool(ProjectSettings.get_setting(PROJECT_SETTING, false)):
		return false
	var expected := WorkspacePaths.persistent_file(RELATIVE_PATH).simplify_path()
	var configured := OS.get_environment(PATH_ENV).strip_edges().simplify_path()
	return not configured.is_empty() and configured == expected


func _ready() -> void:
	if not is_enabled():
		queue_free()
		return
	layer = 100
	_path = WorkspacePaths.persistent_file(RELATIVE_PATH)
	_build_panel()
	_tail_file()
	set_process(true)


func _process(delta: float) -> void:
	_poll_elapsed += delta
	if _poll_elapsed < POLL_SECONDS:
		return
	_poll_elapsed = 0.0
	var changed := _tail_file()
	if changed and _open and not _paused:
		_render_records()


func set_current_battle(battle_id: String) -> void:
	if current_battle_id == battle_id:
		return
	current_battle_id = battle_id
	if is_instance_valid(_battle_label):
		_battle_label.text = "Battle: %s" % (current_battle_id if not current_battle_id.is_empty() else "waiting for authority")
	if _open and not _paused:
		_render_records()


func toggle_panel() -> void:
	_open = not _open
	if is_instance_valid(_overlay):
		_overlay.visible = _open
	if _open:
		_tail_file()
		_render_records()


func is_open() -> bool:
	return _open


func inspection_state() -> Dictionary:
	return {
		"enabled": is_enabled(),
		"open": _open,
		"path": _path,
		"battle_id": current_battle_id,
		"records_buffered": _records.size(),
		"records_visible": _visible_records.size(),
		"filter": _filter,
		"search": _search,
		"paused": _paused,
		"follow_latest": _follow_latest,
		"last_error": _last_error,
		"panel_bounds": _panel.get_global_rect() if is_instance_valid(_panel) else Rect2(),
		"overlay_mouse_filter": _overlay.mouse_filter if is_instance_valid(_overlay) else Control.MOUSE_FILTER_STOP,
	}


func _build_panel() -> void:
	_overlay = Control.new()
	_overlay.set_anchors_preset(Control.PRESET_FULL_RECT)
	# The transcript is a docked developer tool, not a modal. Ignore mouse input
	# across the empty overlay so the exposed battle board remains interactive.
	_overlay.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_overlay.visible = false
	add_child(_overlay)

	_panel = PanelContainer.new()
	# The enemy/combat-log column occupies the rightmost quarter of the battle
	# screen. Dock beneath the play area and stop just before that column.
	_panel.anchor_left = 0.0
	_panel.anchor_top = 0.54
	_panel.anchor_right = 0.76
	_panel.anchor_bottom = 1.0
	_panel.offset_left = 18.0
	_panel.offset_top = 0.0
	_panel.offset_right = -14.0
	_panel.offset_bottom = -18.0
	_panel.mouse_filter = Control.MOUSE_FILTER_STOP
	var style := StyleBoxFlat.new()
	style.bg_color = Color("071013f6")
	style.border_color = Color("4e7782")
	style.set_border_width_all(2)
	style.set_corner_radius_all(10)
	style.shadow_color = Color(0, 0, 0, 0.75)
	style.shadow_size = 18
	_panel.add_theme_stylebox_override("panel", style)
	_overlay.add_child(_panel)

	var margin := MarginContainer.new()
	for side in ["margin_left", "margin_top", "margin_right", "margin_bottom"]:
		margin.add_theme_constant_override(side, 16)
	_panel.add_child(margin)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 8)
	margin.add_child(content)

	var header := HBoxContainer.new()
	content.add_child(header)
	var title := Label.new()
	title.text = "DEV TRANSCRIPT"
	title.add_theme_font_size_override("font_size", 24)
	title.add_theme_color_override("font_color", Color("7ed8ef"))
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(title)
	var close := Button.new()
	close.text = "Close"
	close.pressed.connect(toggle_panel)
	header.add_child(close)

	_battle_label = Label.new()
	_battle_label.text = "Battle: %s" % (current_battle_id if not current_battle_id.is_empty() else "waiting for authority")
	content.add_child(_battle_label)
	_path_label = Label.new()
	_path_label.text = "JSONL: %s" % _path
	_path_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	_path_label.tooltip_text = _path
	_path_label.add_theme_color_override("font_color", Color("9fb3ba"))
	content.add_child(_path_label)

	var filters := HBoxContainer.new()
	filters.add_theme_constant_override("separation", 6)
	content.add_child(filters)
	var selector := OptionButton.new()
	for label in ["All", "Public", "Human", "Opponent", "Hidden", "System", "Errors"]:
		selector.add_item(label)
	selector.item_selected.connect(func(index: int):
		_filter = selector.get_item_text(index)
		if not _paused: _render_records()
	)
	filters.add_child(selector)
	var search := LineEdit.new()
	search.placeholder_text = "Search summary, IDs, and details"
	search.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	search.text_changed.connect(func(value: String):
		_search = value.strip_edges().to_lower()
		if not _paused: _render_records()
	)
	filters.add_child(search)
	_follow_check = CheckBox.new()
	_follow_check.text = "Follow latest"
	_follow_check.button_pressed = true
	_follow_check.toggled.connect(func(value: bool):
		_follow_latest = value
		if value: _scroll_latest.call_deferred()
	)
	filters.add_child(_follow_check)
	_pause_check = CheckBox.new()
	_pause_check.text = "Pause view"
	_pause_check.toggled.connect(func(value: bool):
		_paused = value
		if not value: _render_records()
	)
	filters.add_child(_pause_check)

	var actions := HBoxContainer.new()
	actions.add_theme_constant_override("separation", 6)
	content.add_child(actions)
	var copy_visible := Button.new()
	copy_visible.text = "Copy Visible"
	copy_visible.pressed.connect(_copy_visible)
	actions.add_child(copy_visible)
	var copy_battle := Button.new()
	copy_battle.text = "Copy Battle JSONL"
	copy_battle.pressed.connect(_copy_battle)
	actions.add_child(copy_battle)
	var export_battle := Button.new()
	export_battle.text = "Export Battle"
	export_battle.pressed.connect(_export_battle)
	actions.add_child(export_battle)
	_status_label = Label.new()
	_status_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_status_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	actions.add_child(_status_label)

	_text = RichTextLabel.new()
	_text.bbcode_enabled = true
	_text.scroll_active = true
	_text.scroll_following = false
	_text.selection_enabled = true
	_text.context_menu_enabled = true
	_text.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_text.add_theme_font_size_override("normal_font_size", 14)
	content.add_child(_text)


func _tail_file() -> bool:
	if _path.is_empty() or not FileAccess.file_exists(_path):
		return false
	var file := FileAccess.open(_path, FileAccess.READ)
	if file == null:
		_set_error("Could not open transcript: %s" % error_string(FileAccess.get_open_error()))
		return false
	var length := file.get_length()
	if length < _offset:
		_offset = 0
		_records.clear()
		_actor_names.clear()
	file.seek(_offset)
	var pending_bytes := file.get_buffer(length - _offset)
	var last_newline := -1
	for index in range(pending_bytes.size() - 1, -1, -1):
		if pending_bytes[index] == 10:
			last_newline = index
			break
	if last_newline < 0:
		return false
	var complete_bytes := pending_bytes.slice(0, last_newline + 1)
	var complete_text := complete_bytes.get_string_from_utf8()
	var changed := false
	for line in complete_text.split("\n", false):
		if line.is_empty():
			continue
		var parser := JSON.new()
		if parser.parse(line) != OK or not parser.data is Dictionary:
			_set_error("Invalid complete JSONL record near byte %d" % _offset)
			return changed
		var parsed: Dictionary = parser.data
		_records.append(parsed)
		_remember_actor_names(parsed)
		while _records.size() > MAX_BUFFERED_RECORDS:
			_records.pop_front()
		changed = true
	_offset += last_newline + 1
	if changed:
		_last_error = ""
	return changed


func _render_records() -> void:
	if not is_instance_valid(_text):
		return
	_visible_records.clear()
	for record in _records:
		if _record_matches(record):
			_visible_records.append(record)
	var render_records := _visible_records.slice(maxi(0, _visible_records.size() - MAX_RENDERED_RECORDS))
	var lines: Array[String] = []
	var previous_group := ""
	for record in render_records:
		var battle_id := str(record.get("battle_id", "unscoped"))
		var round_number := int(record.get("round", 0))
		var segment := str(record.get("segment", "system"))
		var group := "%s|%d|%s" % [battle_id, round_number, segment]
		if group != previous_group:
			lines.append("\n[font_size=16][color=#f2bd6a]BATTLE %s · ROUND %d · %s[/color][/font_size]" % [_bb(battle_id), round_number, _bb(segment.replace("_", " ").to_upper())])
			previous_group = group
		var visibility := str(record.get("visibility", "DEBUG_SYSTEM"))
		var sequence := int(record.get("sequence", 0))
		var actor := _actor_label(record)
		var summary := str(record.get("summary", record.get("kind", "record")))
		lines.append("[color=%s][%s][/color] [color=#74888f]#%d[/color] [color=#b7c8cd]%s[/color]  %s" % [_visibility_color(visibility), _bb(visibility), sequence, _bb(actor), _bb(summary)])
	_text.text = "\n".join(lines) if not lines.is_empty() else "No transcript records match the current filters."
	_status_label.text = "%d visible · %d buffered%s" % [_visible_records.size(), _records.size(), " · %s" % _last_error if not _last_error.is_empty() else ""]
	if _follow_latest:
		_scroll_latest.call_deferred()


func _record_matches(record: Dictionary) -> bool:
	var visibility := str(record.get("visibility", ""))
	var controller := str(record.get("controller", ""))
	var actor := str(record.get("actor_id", ""))
	var kind := str(record.get("kind", ""))
	match _filter:
		"Public":
			if visibility != "PUBLIC": return false
		"Human":
			if controller != "human" and actor != str(record.get("human_actor_id", "")): return false
		"Opponent":
			if controller not in ["learned_policy", "d100", "external"] and actor != str(record.get("model_actor_id", "")): return false
		"Hidden":
			if visibility not in ["PRIVATE_SELF", "PRIVATE_OPPONENT"]: return false
		"System":
			if visibility != "DEBUG_SYSTEM": return false
		"Errors":
			if not ("error" in kind or "rejected" in kind or "timeout" in kind): return false
	if _search.is_empty():
		return true
	return _search in JSON.stringify(record).to_lower()


func _remember_actor_names(record: Dictionary) -> void:
	var battle_id := str(record.get("battle_id", ""))
	var actors = record.get("details", {}).get("actors", {})
	if battle_id.is_empty() or not actors is Dictionary:
		return
	for actor_id in actors:
		var actor = actors[actor_id]
		if actor is Dictionary and not str(actor.get("name", "")).is_empty():
			_actor_names["%s|%s" % [battle_id, actor_id]] = str(actor.get("name"))


func _actor_label(record: Dictionary) -> String:
	var actor_id := str(record.get("actor_id", ""))
	if actor_id.is_empty():
		return "Authority"
	var key := "%s|%s" % [str(record.get("battle_id", "")), actor_id]
	var actor_name := str(_actor_names.get(key, ""))
	return "%s (%s)" % [actor_name, actor_id] if not actor_name.is_empty() else actor_id


func _scroll_latest() -> void:
	if is_instance_valid(_text):
		_text.scroll_to_line(maxi(0, _text.get_line_count() - 1))


func _copy_visible() -> void:
	DisplayServer.clipboard_set(_jsonl(_visible_records))
	_status_label.text = "Copied %d visible record(s)" % _visible_records.size()


func _copy_battle() -> void:
	var records := _read_battle_records()
	DisplayServer.clipboard_set(_jsonl(records))
	_status_label.text = "Copied %d record(s) for %s" % [records.size(), current_battle_id]


func _export_battle() -> void:
	var records := _read_battle_records()
	if records.is_empty():
		_status_label.text = "No records found for the current battle"
		return
	var safe_id := current_battle_id.validate_filename()
	var export_path := WorkspacePaths.persistent_file("debug/authority-transcript-%s.jsonl" % safe_id)
	DirAccess.make_dir_recursive_absolute(export_path.get_base_dir())
	var file := FileAccess.open(export_path, FileAccess.WRITE)
	if file == null:
		_set_error("Could not export battle: %s" % error_string(FileAccess.get_open_error()))
		return
	file.store_string(_jsonl(records))
	file.flush()
	_status_label.text = "Exported %d records · %s" % [records.size(), export_path]


func _read_battle_records() -> Array[Dictionary]:
	var result: Array[Dictionary] = []
	if current_battle_id.is_empty() or not FileAccess.file_exists(_path):
		return result
	var file := FileAccess.open(_path, FileAccess.READ)
	if file == null:
		return result
	while not file.eof_reached():
		var line := file.get_line()
		if line.is_empty(): continue
		var parsed = JSON.parse_string(line)
		if parsed is Dictionary and str(parsed.get("battle_id", "")) == current_battle_id:
			result.append(parsed)
	return result


func _jsonl(records: Array[Dictionary]) -> String:
	var lines: Array[String] = []
	for record in records:
		lines.append(JSON.stringify(record))
	return "\n".join(lines) + ("\n" if not lines.is_empty() else "")


func _set_error(message: String) -> void:
	_last_error = message
	push_error("[AuthorityTranscript] %s" % message)
	if is_instance_valid(_status_label):
		_status_label.text = message


func _visibility_color(visibility: String) -> String:
	match visibility:
		"PUBLIC": return "#72ddb7"
		"PRIVATE_SELF": return "#7ed8ef"
		"PRIVATE_OPPONENT": return "#f28b78"
		_: return "#c5a6ff"


func _bb(value: String) -> String:
	return value.replace("[", "[​").replace("]", "​]")
