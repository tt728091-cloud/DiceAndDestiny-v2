extends Control
## Guided authoring consumes the native capability registry; publication always
## goes through full authority validation, including existing saved decks.
signal closed
signal deck_requested(card_id: String)
signal draft_accepted(card: Dictionary)
var embedded_draft: Dictionary = {}
const PROGRAM_EDITOR = preload("res://app/screens/character/card_program_editor.gd")
const STYLE = preload("res://app/screens/character/character_style.gd")
const PREVIEW_ID := "__card_authoring_preview__"
const WINDOW_LABELS := {
	"offensive_planning": ["Offensive planning", "Your offensive turn, before and between your attack rolls."],
	"offensive_reaction": ["Offensive reaction", "After an attack's dice are revealed."],
	"defense_before_roll": ["Defense · before any roll", "The Defense screen, only until you roll your first defense this round."],
	"defense_selection": ["Defense · any time", "The Defense screen before, between and after your defense rolls, until you Pass."],
	"defense_reaction": ["Defense · roll review", "After a defense roll's dice land, before that defense applies."],
	"damage_reaction": ["Damage phase (old saves)", "The separate damage phase used only by battles saved before unified defense."],
}
var catalog: Dictionary = {}
var draft: Dictionary = {}
var revision := 0
var _body: VBoxContainer
var _fields: VBoxContainer
var _steps: VBoxContainer
var _preview: RichTextLabel
var _error: Label
var _json: CodeEdit
var _json_dirty := false
var _syncing_json := false
var _template: OptionButton
var _tabs: TabBar
var _scroll: ScrollContainer
var _publish_button: Button
var _deck_button: Button
var _pending_bar: HBoxContainer
var _pending_action: Callable
var _saved_draft := ""
var _published_id := ""
var _validation: Timer
var _rebuild_pending := false
var _window_checks: Dictionary = {}
var _art: TextureRect
var _title: Label
var _status: Label
var _last_notice := ""
var _card_frame: CenterContainer

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	theme = STYLE.theme()
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "top", "right", "bottom"]: margin.add_theme_constant_override("margin_" + side, 20)
	_body = VBoxContainer.new(); _body.add_theme_constant_override("separation", 10); margin.add_child(_body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); _body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "CHARACTER CREATION  /  CARD WORKSHOP" if embedded_draft.is_empty() else "CARD TREE  /  CARD SETTINGS", 12, STYLE.GOLD)
	STYLE.label(titles, "Card Creation", 30)
	_button(header, "Back to tree" if not embedded_draft.is_empty() else "Back to characters", func(): _guard(func(): closed.emit(); queue_free()), "back").size_flags_vertical = Control.SIZE_SHRINK_END
	var toolbar := HBoxContainer.new(); toolbar.add_theme_constant_override("separation", 8); _body.add_child(toolbar); toolbar.visible = embedded_draft.is_empty()
	_button(toolbar, "New blank card", func(): _guard(_new_blank), "new")
	var start := STYLE.label(toolbar, "or start from", 14, STYLE.MUTED); start.autowrap_mode = TextServer.AUTOWRAP_OFF; start.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_template = OptionButton.new(); _template.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _template.fit_to_longest_item = false; toolbar.add_child(_template)
	_button(toolbar, "Edit template", func(): _guard(_load_template), "load").tooltip_text = "Edit the selected card itself."
	_button(toolbar, "Create a copy", func(): _guard(_clone_template), "clone").tooltip_text = "Start a new card from the selected card's settings."
	_tabs = TabBar.new()
	for title in ["Card", "Effects & choices", "Upgrades", "Advanced JSON"]: _tabs.add_tab(title)
	_body.add_child(_tabs); _tabs.tab_changed.connect(_change_tab)
	if not embedded_draft.is_empty(): _tabs.set_tab_disabled(2, true)
	var split := HSplitContainer.new(); split.size_flags_vertical = Control.SIZE_EXPAND_FILL; split.add_theme_constant_override("separation", 14); _body.add_child(split)
	_scroll = ScrollContainer.new(); _scroll.custom_minimum_size.x = 520; _scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _scroll.size_flags_stretch_ratio = 1.6
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; split.add_child(_scroll)
	_fields = VBoxContainer.new(); _fields.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _fields.add_theme_constant_override("separation", 12); _scroll.add_child(_fields)
	var preview_panel := PanelContainer.new(); preview_panel.custom_minimum_size.x = 340; preview_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	preview_panel.add_theme_stylebox_override("panel", STYLE.box("0a121a", STYLE.BRONZE, 14, 10)); split.add_child(preview_panel)
	var preview_scroll := ScrollContainer.new(); preview_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; preview_panel.add_child(preview_scroll)
	var right := VBoxContainer.new(); right.size_flags_horizontal = Control.SIZE_EXPAND_FILL; right.add_theme_constant_override("separation", 10); preview_scroll.add_child(right)
	STYLE.heading(right, "Card preview")
	_card_frame = CenterContainer.new(); right.add_child(_card_frame)
	_title = STYLE.label(right, "", 20, STYLE.GOLD_BRIGHT); _title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_art = TextureRect.new(); _art.custom_minimum_size.y = 220; _art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; _art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; right.add_child(_art); _art.hide()
	var rules_box := STYLE.section(right, 12)
	STYLE.heading(rules_box, "Rules")
	_preview = RichTextLabel.new(); _preview.add_theme_font_size_override("normal_font_size", 17); _preview.add_theme_color_override("default_color", STYLE.IVORY)
	_preview.fit_content = true; _preview.scroll_active = false; _preview.custom_minimum_size.y = 60; rules_box.add_child(_preview)
	_status = STYLE.label(right, "", 14, STYLE.MUTED)
	STYLE.label(right, "Changes affect future battles. Cards in deck, hand and discard count as health; removed cards do not.", 13, STYLE.DIM)
	_json = CodeEdit.new(); _json.custom_minimum_size = Vector2(0, 320); _json.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _json.text_changed.connect(func():
		if not _syncing_json and _json.text != JSON.stringify(draft, "  "): _json_dirty = true; _publish_button.disabled = true)
	_error = STYLE.label(_body, "", 15); _error.custom_minimum_size.y = 38; _error.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_pending_bar = HBoxContainer.new(); _pending_bar.add_theme_constant_override("separation", 8); _body.add_child(_pending_bar); _pending_bar.hide()
	STYLE.label(_pending_bar, "Discard unpublished changes?", 16, STYLE.GOLD).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(_pending_bar, "Keep editing", func(): _pending_bar.hide(), "keep_editing")
	STYLE.accent(_button(_pending_bar, "Discard changes", func(): _pending_bar.hide(); _pending_action.call(), "discard_changes"), STYLE.LOSS)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 8); _body.add_child(actions)
	_button(actions, "Validate and preview", _validate, "validate")
	_publish_button = STYLE.accent(_button(actions, "Use card settings" if not embedded_draft.is_empty() else "Publish for future battles", _publish, "publish"))
	_publish_button.custom_minimum_size.x = 220
	_deck_button = _button(actions, "Open in character deck", func(): closed.emit(); deck_requested.emit(_published_id); queue_free(), "deck")
	_deck_button.disabled = true; _deck_button.visible = embedded_draft.is_empty()
	_validation = Timer.new(); _validation.one_shot = true; _validation.wait_time = 0.25; _validation.timeout.connect(_validate); add_child(_validation)
	if not _reload_catalog(): return
	if embedded_draft.is_empty(): _new_blank()
	else:
		draft = embedded_draft.duplicate(true); _normalize_draft(); _render_fields(); _sync_json(); _validate(); _saved_draft = JSON.stringify(draft)

func _request(op: String) -> Dictionary:
	return get_node("/root/LearnedBattleRuntime").card_authoring(op, draft, revision)
func _reload_catalog(select_id: String = "brace") -> bool:
	var response := _request("card_authoring")
	if not response.get("ok", false): _error.text = str(response.get("error", "Card creation unavailable")); _publish_button.disabled = true; return false
	catalog = response.result; revision = int(catalog.revision)
	_template.clear()
	var ids: Array = catalog.templates.keys(); ids.sort()
	for id in ids:
		_template.add_item(str(catalog.templates[id].name)); _template.set_item_metadata(_template.item_count - 1, id)
		if id == select_id: _template.select(_template.item_count - 1)
	return true
func _label(parent: Node, text: String, font_size: int = 16) -> Label:
	var label := Label.new(); label.text = text; label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; label.add_theme_font_size_override("font_size", font_size + 1); label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	if parent is GridContainer:
		# Form captions sit in a fixed column; " · " hints move to tooltips.
		var parts := text.split(" · ", true, 1)
		label.text = parts[0]
		if parts.size() > 1:
			label.tooltip_text = parts[1]; label.mouse_filter = Control.MOUSE_FILTER_PASS
			label.mouse_default_cursor_shape = Control.CURSOR_HELP; label.add_theme_color_override("font_color", STYLE.MUTED.lightened(0.15))
		label.custom_minimum_size.x = 250; label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		if label.tooltip_text.is_empty(): label.add_theme_color_override("font_color", STYLE.MUTED)
	parent.add_child(label); return label
func _fill(control: Control) -> void:
	if control is SpinBox:
		control.custom_minimum_size.x = 150; control.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
	else: control.size_flags_horizontal = Control.SIZE_EXPAND_FILL
func _button(parent: Node, text: String, action: Callable, key: String = "") -> Button:
	var button := Button.new(); button.text = text; button.custom_minimum_size.y = 34; button.pressed.connect(action); parent.add_child(button)
	if not key.is_empty(): button.set_meta("editor_key", key)
	return button
func _line(parent: Node, title: String, value: String, changed: Callable, key: String = "") -> LineEdit:
	_label(parent, title); var line := LineEdit.new(); line.text = value; line.text_changed.connect(changed); parent.add_child(line); line.set_meta("editor_key", key); _fill(line); return line
func _number(parent: Node, title: String, value: int, minimum: int, maximum: int, changed: Callable, key: String = "") -> SpinBox:
	_label(parent, title); var field := SpinBox.new(); field.min_value = minimum; field.max_value = maximum; field.value = value; field.value_changed.connect(func(v): changed.call(int(v))); parent.add_child(field); field.set_meta("editor_key", key); _fill(field); return field
func _select(parent: Node, title: String, values: Array, value: String, changed: Callable, key: String = "") -> OptionButton:
	_label(parent, title); var field := OptionButton.new(); parent.add_child(field); field.set_meta("editor_key", key); _fill(field); field.fit_to_longest_item = false
	for option in values:
		field.add_item(str(option).replace("_", " ").capitalize() if not str(option).is_empty() else "None")
		field.set_item_metadata(field.item_count - 1, option)
		if str(option) == value: field.select(field.item_count - 1)
	field.item_selected.connect(func(i): changed.call(str(values[i])))
	return field
func _check(parent: Node, title: String, value: bool, changed: Callable, key: String = "") -> CheckBox:
	var check := CheckBox.new(); check.text = title; check.button_pressed = value; check.toggled.connect(changed); parent.add_child(check); check.set_meta("editor_key", key); return check
func _guard(action: Callable) -> void:
	if _json_dirty or JSON.stringify(draft) != _saved_draft:
		_pending_action = action; _pending_bar.show()
	else: action.call()
func _unhandled_key_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_cancel"):
		_guard(func(): closed.emit(); queue_free()); get_viewport().set_input_as_handled()
func _load_template() -> void:
	if _template.selected < 0:
		_error.text = "Choose an optional template, or use New blank card."; return
	_validation.stop(); draft = catalog.templates[_template.get_selected_metadata()].duplicate(true)
	if not draft.economy.get("upgrades") is Array: draft.economy.upgrades = []
	_json_dirty = false; _published_id = ""; _deck_button.disabled = true; _last_notice = ""
	_render_fields(); _sync_json(); _validate(); _saved_draft = JSON.stringify(draft)
func _clone_template() -> void:
	if _template.selected < 0:
		_error.text = "Choose an optional template, or use New blank card."; return
	_load_template()
	var base := str(draft.id) + "_custom"; var candidate := base; var n := 2
	while catalog.templates.has(candidate): candidate = base + "_" + str(n); n += 1
	draft.id = candidate
	var base_name := str(draft.name) + " Custom"; var copy_name := base_name; var suffix := 2
	while catalog.templates.values().any(func(c): return c.name == copy_name): copy_name = base_name + " " + str(suffix); suffix += 1
	draft.name = copy_name; draft.economy.upgrades = []
	_tabs.current_tab = 0; _render_fields(); _changed()
func _new_blank() -> void:
	_validation.stop()
	draft = {
		"schema_version": 1, "id": "", "name": "", "type": "action", "access_type": "general",
		"cost": {"energy": 0},
		"play": {"source_zones": ["hand"], "destination": "discard"},
		"targeting": {"selector": "card_program", "minimum": 1, "maximum": 1},
		"economy": {"buy": 10, "sell": 10, "copy_limit": 20, "upgrades": []},
		"presentation": {"rules_text": "", "illustration_path": ""},
		"program": {"version": 1, "windows": ["offensive_planning"], "roll_requirement": "any", "uses_per_round": 0, "uses_per_battle": 0, "steps": []},
	}
	_json_dirty = false; _published_id = ""; _deck_button.disabled = true; _last_notice = ""
	_template.select(-1); _template.text = "Optional template…"
	_pending_bar.hide()
	_tabs.set_block_signals(true); _tabs.current_tab = 0; _tabs.set_block_signals(false)
	_scroll.scroll_vertical = 0
	_title.text = "New card"; _art.texture = null; _art.hide(); _status.text = ""
	_render_fields(); _sync_json(); _validate(); _saved_draft = JSON.stringify(draft)
func _sync_json() -> void:
	_syncing_json = true; _json.text = JSON.stringify(draft, "  "); _syncing_json = false; _json_dirty = false
func _changed(rebuild: bool = false) -> void:
	_sync_json(); _published_id = ""; _deck_button.disabled = true; _publish_button.disabled = true
	_error.text = "Checking configuration…"; _validation.start(); _tone()
	_refresh_window_controls()
	if rebuild and not _rebuild_pending: _rebuild_pending = true; call_deferred("_rebuild")
func _rebuild() -> void:
	_rebuild_pending = false; var y := _scroll.scroll_vertical; _render_fields(); _scroll.set_deferred("scroll_vertical", y)
func _render_fields() -> void:
	if draft.is_empty(): return
	if _json.get_parent() != null: _json.get_parent().remove_child(_json)
	for child in _fields.get_children(): _fields.remove_child(child); child.queue_free()
	_window_checks.clear()
	match _tabs.current_tab:
		0: _card_fields()
		1:
			_label(_fields, "Change the values below and validate to refresh the preview." if draft.get("mechanic") is Dictionary else "Effects resolve in order. Options and conditions are part of the card, not separate cards.", 15)
			_steps = VBoxContainer.new(); _fields.add_child(_steps)
			if draft.has("mechanic"): _mechanic_fields(_steps)
			else: PROGRAM_EDITOR.render(self, _steps, draft.program.steps, "steps", 0)
		2: _upgrade_fields()
		3:
			_label(_fields, "Optional advanced definition. The same authority validation applies. Apply JSON before returning to the guided controls.", 15)
			_fields.add_child(_json); _button(_fields, "Apply advanced definition", _apply_json, "apply_json")
	_refresh_window_controls()
func _card_fields() -> void:
	var identity := _section("Identity")
	var grid := STYLE.form(identity)
	_line(grid, "Card ID · a new ID creates a separate card", str(draft.id), func(v): draft.id = v; _changed(), "id").editable = embedded_draft.is_empty()
	_line(grid, "Name", str(draft.name), func(v): draft.name = v; _changed(), "name")
	_select(grid, "Card family", catalog.get("access_types", {"general": "General"}).keys(), str(draft.get("access_type", "general")), func(v): draft.access_type = v; _changed(), "access_type")
	var art_line := _line(grid, "Illustration path · optional res:// image", str(draft.presentation.get("illustration_path", "")), func(v): draft.presentation.illustration_path = v; _changed(), "art")
	_art_picker(identity, art_line)
	var economy := _section("Cost & economy")
	grid = STYLE.form(economy)
	_number(grid, "Energy cost", int(draft.cost.energy), 0, 100, func(v): draft.cost.energy = v; _changed(), "energy")
	_number(grid, "Buy XP", int(draft.economy.buy), 0, 1000000, func(v): draft.economy.buy = v; draft.economy.sell = mini(int(draft.economy.sell), v) if embedded_draft.is_empty() else v; _changed(true), "buy")
	_number(grid, "Sell XP · cannot exceed buy price", int(draft.economy.sell), 0, int(draft.economy.buy), func(v): draft.economy.sell = v; _changed(), "sell").editable = embedded_draft.is_empty()
	_number(grid, "Copies per deck", int(draft.economy.copy_limit), 1, 100, func(v): draft.economy.copy_limit = v; _changed(), "copies")
	var play := _section("Playing the card")
	grid = STYLE.form(play)
	_select(grid, "Played card goes to", catalog.destinations, str(draft.play.destination), func(v): draft.play.destination = v; _changed(), "destination")
	if not draft.has("mechanic"): _select(grid, "Roll requirement", catalog.roll_requirements.filter(func(v): return v != "before_first" or not _needs_roll(draft.program.steps)), str(draft.program.roll_requirement), func(v): draft.program.roll_requirement = v; _changed(), "roll_requirement")
	for key in ["uses_per_round", "uses_per_battle"]:
		_number(grid, {"uses_per_round": "Uses per round", "uses_per_battle": "Uses per battle"}[key] + " · 0 = unlimited", int(_timing().get(key, 0)), 0, 100, func(v): _timing()[key] = v; _changed(), key)
	if draft.has("mechanic") and not str(draft.mechanic.get("expiration", "")).is_empty():
		_select(grid, "Unused preparation expires", ["offensive_exit", "damage_exit", "next_income", "next_ongoing", "battle"], str(draft.mechanic.expiration), func(v): draft.mechanic.expiration = v; _changed(true), "expiration")
		if draft.mechanic.expiration != "battle": _number(grid, "Rounds · 1 = next applicable checkpoint", int(draft.mechanic.get("rounds", 1)), 1, 100, func(v): draft.mechanic.rounds = v; _changed(), "expiration_rounds")
	var piles := HFlowContainer.new(); piles.add_theme_constant_override("h_separation", 14); play.add_child(piles)
	_multi(piles, "Can be played from · at least one pile", catalog.source_zones, draft.play.source_zones, func(): _changed(), "play_piles", true)
	var timing := _section("Play windows")
	_label(timing, "Disabled windows conflict with one of the card's effects.", 13).add_theme_color_override("font_color", STYLE.MUTED)
	var windows := GridContainer.new(); windows.columns = 2; windows.add_theme_constant_override("h_separation", 18); timing.add_child(windows)
	for window in _available_windows():
		var label: Array = WINDOW_LABELS.get(window, [str(window).replace("_", " ").capitalize(), ""])
		var check := _check(windows, label[0], window in _timing().windows, func(on):
			if on and window not in _timing().windows: _timing().windows.append(window)
			elif not on: _timing().windows.erase(window)
			_changed(), "window." + str(window))
		_window_checks[window] = check

func _section(title: String) -> VBoxContainer:
	var box := STYLE.section(_fields, 16)
	STYLE.heading(box, title)
	return box

## Thumbnails of the available card art; picking one fills the path field.
func _art_picker(parent: Node, field: LineEdit) -> void:
	var strip := HFlowContainer.new(); strip.add_theme_constant_override("h_separation", 6); strip.add_theme_constant_override("v_separation", 6); parent.add_child(strip)
	var current := str(draft.presentation.get("illustration_path", ""))
	var none := _button(strip, "No art", func(): field.text = ""; draft.presentation.illustration_path = ""; _changed(true), "art.none")
	none.custom_minimum_size = Vector2(70, 70)
	if current.is_empty(): STYLE.accent(none)
	for path in STYLE.card_art():
		var pick := _button(strip, "", func(): field.text = path; draft.presentation.illustration_path = path; _changed(true), "art.pick." + path.get_file().get_basename())
		pick.icon = STYLE.icon(STYLE.texture(path), 46, 62); pick.custom_minimum_size = Vector2(60, 76); pick.expand_icon = false
		pick.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
		pick.tooltip_text = path.get_file().get_basename().replace("_", " ").capitalize()
		if path == current: pick.add_theme_stylebox_override("normal", STYLE.box("223442", STYLE.GOLD, 4, 8, 2))

func _multi(parent: Node, title: String, values: Array, selected: Array, changed: Callable, key: String, require_one: bool = false) -> void:
	_label(parent, title, 14).add_theme_color_override("font_color", STYLE.MUTED)
	for value in values:
		var check := _check(parent, str(value).replace("_", " ").capitalize(), value in selected, func(on):
			if on and value not in selected: selected.append(value)
			elif not on:
				if require_one and selected.size() == 1: _error.text = "Choose at least one pile."; _changed(true); return
				selected.erase(value)
			changed.call(), key + "." + str(value))
		check.set_meta("choice_value", value)
func _step_windows(step: Dictionary) -> Array:
	var result: Array = catalog.effects.get(step.effect, {}).get("windows", []).duplicate()
	if catalog.effects.get(step.effect, {}).get("target") == "offensive_die" and step.target.owner == "enemy": result = ["offensive_reaction"]
	for option in step.get("choices", []):
		for child in option.steps: result = _intersection(result, _step_windows(child))
	return result
func _intersection(a: Array, b: Array) -> Array:
	return a.filter(func(v): return v in b)
func _compatible_windows(steps: Array) -> Array:
	var result: Array = catalog.windows.duplicate()
	for step in steps: result = _intersection(result, _step_windows(step))
	return result
func _refresh_window_controls() -> void:
	if draft.is_empty(): return
	var allowed := _available_windows() if draft.has("mechanic") else _compatible_windows(draft.program.steps)
	for w in _window_checks:
		_window_checks[w].disabled = w not in allowed
		var rules: String = WINDOW_LABELS.get(w, ["", ""])[1]
		_window_checks[w].tooltip_text = (rules + "\n" if not rules.is_empty() else "") + "Unavailable because one or more effects cannot run in this window." if w not in allowed else rules
func _reconcile_windows() -> void:
	if draft.has("mechanic"): return
	if _needs_roll(draft.program.steps) and draft.program.roll_requirement == "before_first":
		draft.program.roll_requirement = "after_first"; _last_notice = "Roll requirement changed: these effects need rolled dice."
	var allowed := _available_windows() if draft.has("mechanic") else _compatible_windows(draft.program.steps)
	var kept := _intersection(_timing().windows, allowed)
	if kept != _timing().windows:
		_last_notice = "Play windows updated to match the effects."
		_timing().windows = kept if not kept.is_empty() else allowed.duplicate()
	elif kept.is_empty(): _timing().windows = allowed.duplicate()
func _new_step(effect: String) -> Dictionary:
	var rules: Dictionary = catalog.target_rules[effect]
	var step := {"effect": effect, "params": {}, "target": {"owner": "self", "mode": "one", "count": 1, "selection": "choose"}}
	if not rules.modes.is_empty(): step.target.mode = rules.modes[0]
	if "zones" in rules.filters: step.target.zones = ["hand"]
	if effect == "apply_status": step.params.status_id = catalog.statuses.keys()[0]
	if effect == "choice": step.choices = [{"name": "Option 1", "energy": 0, "steps": [_new_step("energy")]}]
	return step
func _effect_available(sequence: Array, effect: String, depth: int) -> bool:
	if sequence.size() >= int(catalog.limits.steps) or effect == "choice" and depth >= int(catalog.limits.depth): return false
	if effect == "sacrifice" and not sequence.is_empty(): return false
	var combined: Array = draft.program.steps.duplicate(true)
	# The UI also validates the whole tree after adding; this intersection excludes
	# impossible timing pairs even when the new effect is inside an option.
	return not _intersection(_compatible_windows(combined), catalog.effects[effect].windows).is_empty()
func _upgrade_fields() -> void:
	STYLE.heading(_fields, "Upgrade branches")
	_label(_fields, "Each branch replaces one owned copy. Upgrade XP must cover any increase in deck value. Publish a new destination card first, then add it here.", 14).add_theme_color_override("font_color", STYLE.MUTED)
	var branches: Array = draft.economy.upgrades if draft.economy.get("upgrades") is Array else []
	draft.economy.upgrades = branches
	for i in branches.size():
		var branch: Dictionary = branches[i]
		var targets: Array = catalog.templates.keys().filter(func(id): return id != draft.id and (id == branch.to or not branches.any(func(b): return b.to == id)))
		targets.sort()
		_select(_fields, "Branch %d · destination card" % [i + 1], targets, str(branch.to), func(v): branch.to = v; branch.xp = maxi(int(branch.xp), _upgrade_minimum(v)); _changed(true), "upgrade.%d.target" % i)
		_number(_fields, "Upgrade XP", int(branch.xp), _upgrade_minimum(str(branch.to)), 1000000, func(v): branch.xp = v; _changed(), "upgrade.%d.xp" % i)
		_button(_fields, "Remove branch", func(): branches.remove_at(i); _changed(true), "upgrade.%d.remove" % i)
	var available: Array = catalog.templates.keys().filter(func(id): return id != draft.id and not branches.any(func(b): return b.to == id)); available.sort()
	var add := _button(_fields, "Add upgrade branch", func(): branches.append({"to": available[0], "xp": _upgrade_minimum(available[0])}); _changed(true), "upgrade.add")
	add.disabled = available.is_empty()
func _upgrade_minimum(id: String) -> int:
	return maxi(0, int(catalog.templates.get(id, {}).get("economy", {}).get("buy", 0)) - int(draft.economy.buy))
func _apply_json() -> bool:
	var parser := JSON.new()
	if parser.parse(_json.text) != OK or not parser.data is Dictionary: _error.text = "The advanced definition must be a valid JSON object."; _publish_button.disabled = true; return false
	var previous := draft; draft = parser.data
	var response := _request("validate_card")
	if not response.get("ok", false): draft = previous; _error.text = str(response.get("error")); _publish_button.disabled = true; return false
	draft = response.result.card; _normalize_draft(); _json_dirty = false; _render_fields(); _sync_json(); _validate(); return not _publish_button.disabled
func _validate() -> void:
	_validate_draft()
	_tone(); _refresh_card_preview()

func _validate_draft() -> void:
	if draft.is_empty(): return
	_validation.stop()
	if _json_dirty: _apply_json(); return
	if str(draft.get("id", "")).strip_edges().is_empty() or str(draft.get("name", "")).strip_edges().is_empty():
		_error.text = "Enter a card ID and name, then add Effects & choices."
		_publish_button.disabled = true; _preview.text = "Build your card using the Card and Effects & choices tabs."; return
	if draft.get("program") is Dictionary and draft.program.steps.is_empty():
		_error.text = "Add at least one effect in Effects & choices."
		_publish_button.disabled = true; _preview.text = "Choose what this card does before publishing."; return
	var response := _request("validate_card")
	if not response.get("ok", false):
		_error.text = str(response.get("error")); _publish_button.disabled = true; _preview.text = "Correct the configuration to see its validated rules."; return
	draft.presentation = response.result.card.presentation
	var path := str(draft.presentation.get("illustration_path", ""))
	if not path.is_empty() and (not path.begins_with("res://") or path.get_extension().to_lower() not in ["png", "jpg", "jpeg", "webp", "svg"] or not ResourceLoader.exists(path)):
		_error.text = "Illustration must be an existing res:// PNG, JPEG, WebP or SVG image, or leave it empty."; _publish_button.disabled = true; return
	_set_valid(); _sync_json()

## Colours the status line: green when valid or published, red for problems.
func _tone() -> void:
	if not is_instance_valid(_error): return
	var text := _error.text
	var color := STYLE.GAIN if text.begins_with("Definition valid") or text.begins_with("Published") else STYLE.MUTED if text.begins_with("Checking") or text.is_empty() else STYLE.LOSS
	_error.add_theme_color_override("font_color", color)
	_error.add_theme_stylebox_override("normal", STYLE.box(Color(color, 0.1), Color(color, 0.45), 14, 8))

## Live card rendered with the same component as battle hands.
func _refresh_card_preview() -> void:
	if not is_instance_valid(_card_frame) or draft.is_empty(): return
	for child in _card_frame.get_children(): _card_frame.remove_child(child); child.queue_free()
	var cards = BattlePresentationCatalog._catalog.get("cards")
	if not cards is Dictionary:
		cards = {}; BattlePresentationCatalog._catalog["cards"] = cards
	var shown: Dictionary = draft.duplicate(true)
	if str(shown.get("name", "")).strip_edges().is_empty(): shown.name = "New card"
	# Render under the draft's own ID so default artwork matches what battle shows,
	# then restore whatever catalog entry that ID had.
	var key := str(draft.get("id", "")).strip_edges()
	if key.is_empty(): key = PREVIEW_ID
	var had: bool = cards.has(key); var previous = cards.get(key)
	cards[key] = shown
	var card := BattleCard.new(); _card_frame.add_child(card); card.configure("preview", key, true)
	card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.focus_mode = Control.FOCUS_NONE
	card.scale = Vector2(1.25, 1.25); card.custom_minimum_size = BattleCard.STANDARD_SIZE * 1.25
	if had: cards[key] = previous
	else: cards.erase(key)
func _set_valid() -> void:
	_preview.text = str(draft.presentation.rules_text)
	_title.text = "%s · %d energy" % [draft.name, int(draft.cost.energy)]
	_status.text = "Play: %s\nAfter play: %s\nBuy: %d XP · Sell: %d XP · Up to %d copies" % [", ".join(_timing().windows).replace("_", " "), draft.play.destination, int(draft.economy.buy), int(draft.economy.sell), int(draft.economy.copy_limit)]
	var path := str(draft.presentation.get("illustration_path", ""))
	_art.texture = load(path) if not path.is_empty() and ResourceLoader.exists(path) else null; _art.visible = false
	_error.text = "Definition valid." if _last_notice.is_empty() else "Definition valid. " + _last_notice
	_publish_button.disabled = false
func _publish() -> void:
	_validate()
	if _publish_button.disabled: return
	if not embedded_draft.is_empty():
		draft_accepted.emit(draft.duplicate(true)); queue_free(); return
	var response := _request("publish_card")
	if not response.get("ok", false): _error.text = str(response.get("error")); return
	revision = int(response.result.revision); draft = response.result.card; _normalize_draft(); _sync_json(); _saved_draft = JSON.stringify(draft)
	_published_id = str(draft.id); _deck_button.disabled = false
	_reload_catalog(_published_id); _render_fields(); _set_valid()
	_error.text = "Published %s · revision %d. Open it in your character deck to add a copy." % [draft.name, revision]
	_tone()
func _exit_tree() -> void:
	if is_instance_valid(_json) and _json.get_parent() == null: _json.queue_free()

func _change_tab(index: int) -> void:
	if _json_dirty and index != 3 and not _apply_json():
		_tabs.set_block_signals(true); _tabs.current_tab = 3; _tabs.set_block_signals(false); return
	_render_fields()

func _needs_roll(steps: Array) -> bool:
	for step in steps:
		if catalog.effects[step.effect].target == "offensive_die" and step.target.owner == "self" or step.effect == "ability_bonus" and step.target.get("qualified", false): return true
		for option in step.get("choices", []):
			if _needs_roll(option.steps): return true
	return false

func _may_draw(steps: Array) -> bool:
	for step in steps:
		if step.effect == "draw" and step.target.owner != "enemy": return true
		for option in step.get("choices", []):
			if _may_draw(option.steps): return true
	return false
func _draw_before(steps: Array, wanted: String, path: String = "steps", prior: bool = false) -> bool:
	for i in steps.size():
		var key := path + "." + str(i)
		if key == wanted: return prior
		for j in steps[i].get("choices", []).size():
			var child := key + ".option." + str(j) + ".steps"
			if wanted.begins_with(child + "."): return _draw_before(steps[i].choices[j].steps, wanted, child, prior)
		prior = prior or _may_draw([steps[i]])
	return false

func _normalize_draft() -> void:
	if not draft.get("economy") is Dictionary: draft.economy = {"buy": 10, "sell": 10, "copy_limit": 20, "upgrades": []}
	if not draft.economy.get("upgrades") is Array: draft.economy.upgrades = []

func _timing() -> Dictionary:
	return draft.mechanic if draft.has("mechanic") else draft.program
func _available_windows() -> Array:
	return catalog.mechanics[draft.mechanic.kind].windows if draft.has("mechanic") else catalog.windows
func _mechanic_fields(parent: VBoxContainer) -> void:
	if draft.mechanic.kind == "roll_table": _roll_table_fields(parent); return
	var spec: Dictionary = catalog.mechanics[draft.mechanic.kind]
	_label(parent, str(spec.label).capitalize(), 22)
	_label(parent, "Choose the effect values below. Available targets and timing follow this card’s rules.", 15)
	var form := STYLE.form(parent)
	if draft.mechanic.kind in ["coagulate", "emergency_molt", "antivenom_draught", "spined_rebuttal", "spiteful_ward"]:
		_select(form, "Saved cards go to", ["original", "discard", "hand", "deck", "removed"], str(draft.get("saved_card_destination", "original")), func(v): draft.saved_card_destination = v; _changed(), "saved_card_destination")
	var fields: Array = spec.parameters.keys(); fields.sort()
	for field in fields:
		var param: Dictionary = spec.parameters[field]
		var value = draft.mechanic.params.get(field, param.default)
		var caption := str(field).trim_suffix("_id").replace("_", " ").capitalize()
		var key := "mechanic.param." + str(field)
		match str(param.type):
			"integer": _number(form, caption, int(value), int(param.get("minimum", 0)), int(param.get("maximum", 100)), func(v): draft.mechanic.params[field] = v; _changed(), key)
			"boolean": _label(form, ""); _check(form, caption, bool(value), func(v): draft.mechanic.params[field] = v; _changed(), key)
			"integers": PROGRAM_EDITOR._numbers(self, form, caption + " · comma-separated", value, func(v): draft.mechanic.params[field] = v; _changed(), key)
			"status", "ability":
				var ids: Array = catalog.statuses.keys() if param.type == "status" else param.get("options", catalog.abilities.keys().filter(func(id): return catalog.abilities[id].type == "offensive")); ids.sort()
				_select(form, caption, ids, str(value), func(v): draft.mechanic.params[field] = v; _changed(), key)

func _roll_table_fields(parent: VBoxContainer) -> void:
	var roll: Dictionary = draft.operations[0]
	_label(parent, "Dice outcome table", 22)
	_label(parent, "Every face must appear exactly once. Each rolled die resolves its matching row, in order.", 15)
	_select(parent, "Target", ["one_enemy", "self"], str(draft.targeting.selector), func(v): draft.targeting.selector = v; _changed(), "roll.target")
	_label(parent, "Rolls use the target’s owned dice and their numbered faces (1–6). Curse and physical-die rules still apply.", 15)
	_number(parent, "Number of dice", int(roll.dice_count), 1, 100, func(v): roll.dice_count = v; _changed(), "roll.count")
	for i in roll.outcomes.size():
		var outcome: Dictionary = roll.outcomes[i]
		var key := "roll.outcome." + str(i)
		_label(parent, "Outcome %d" % [i + 1], 18)
		PROGRAM_EDITOR._numbers(self, parent, "Faces · comma-separated", outcome.faces, func(v): outcome.faces = v; _changed(), key + ".faces")
		for j in outcome.operations.size():
			var op: Dictionary = outcome.operations[j]
			var op_key := key + ".effect." + str(j)
			_select(parent, "Effect", ["noop", "deal_damage", "apply_status", "draw_cards", "gain_resource"], str(op.type), func(v):
				var updated := {"type": v, "target": "selected_targets"}
				if v == "apply_status": updated.status_id = "poison"; updated.stack_count = 1
				elif v != "noop": updated.amount = 1
				if v == "gain_resource": updated.resource = "energy"
				outcome.operations[j] = updated; _changed(true), op_key + ".type")
			if op.type != "noop":
				_select(parent, "Effect recipient", ["selected_targets", "self"], str(op.get("target", "selected_targets")), func(v): op.target = v; _changed(), op_key + ".target")
				if op.type == "apply_status":
					_select(parent, "Status", catalog.statuses.keys(), str(op.status_id), func(v): op.status_id = v; _changed(), op_key + ".status")
					_number(parent, "Stacks", int(op.stack_count), 1, 100, func(v): op.stack_count = v; _changed(), op_key + ".stacks")
				else: _number(parent, "Amount", int(op.amount), 0, 100, func(v): op.amount = v; _changed(), op_key + ".amount")
			_button(parent, "Remove effect", func(): outcome.operations.remove_at(j); _changed(true), op_key + ".remove").disabled = outcome.operations.size() == 1
		_button(parent, "Add effect", func(): outcome.operations.append({"type": "noop"}); _changed(true), key + ".add").disabled = outcome.operations.size() >= 32
		_button(parent, "Remove outcome", func(): roll.outcomes.remove_at(i); _changed(true), key + ".remove").disabled = roll.outcomes.size() == 1
	_button(parent, "Add outcome", func(): roll.outcomes.append({"id": "outcome_" + str(Time.get_ticks_usec()), "faces": [], "operations": [{"type": "noop"}]}); _changed(true), "roll.add").disabled = roll.outcomes.size() >= 100
