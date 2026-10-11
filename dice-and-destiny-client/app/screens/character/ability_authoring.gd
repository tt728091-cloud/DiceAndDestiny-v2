extends Control
## Every edit remains a draft until native validation and atomic publication.
signal closed
const STYLE = preload("res://app/screens/character/character_style.gd")
## Inline lists edited as comma-separated text.
const TEXT_LISTS := ["faces", "faces_any", "allowed_proposal_types"]
const CAPTIONS := {"id": "Tier ID", "maximum_per_segment": "Uses per segment", "energy": "Energy cost", "selector": "Target", "minimum": "Minimum", "maximum": "Maximum", "saved_card_destination": "Saved cards go to", "requires_incoming_proposal": "Needs an incoming attack", "allowed_proposal_types": "Allowed attack types", "target_count": "Number of targets", "offensive_abilities_only": "Only against offensive abilities", "energy_gain_limit": "Energy gain limit · 0 = none", "choose_tier": "Player chooses the tier", "activation_tiers": "Activation tiers", "conditional_bonuses": "Conditional bonuses", "operations": "Effects", "hooks": "Follow-ups", "status_id": "Status", "stack_count": "Stacks", "dice_id": "Die", "dice_count": "Dice rolled", "symbol_id": "Symbol", "tier_id": "Tier", "faces_any": "Any die showing faces", "requires_prevention": "Only if damage was prevented", "reaction_window": "Reaction window", "pass_required": "Requires a pass", "opens": "Opens a reaction window"}
var catalog: Dictionary = {}
var draft: Dictionary = {}
var revision := 0
var _template: OptionButton
var _tabs: TabBar
var _scroll: ScrollContainer
var _fields: VBoxContainer
var _status: Label
var _preview: Label
var _publish: Button
var _json: CodeEdit
var _character: OptionButton
var _board: Dictionary = {}
var _timer: Timer
var _dirty := false
var _notice: HBoxContainer
var _pending: Callable
var _type_drafts: Dictionary = {}
var _type_choice: OptionButton
var _type_hint: Label
var _json_buffer := ""
var _json_pending := false
var _json_initial_text := ""
var _json_warning: HBoxContainer
var _json_warning_text: Label
var _preview_badges: HFlowContainer
var _preview_name: Label

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	theme = STYLE.theme()
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 18)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 8); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "CHARACTER CREATION  /  ABILITY WORKSHOP", 12, STYLE.GOLD)
	_label(titles, "Ability Creation", 28)
	_button(header, "Back to characters", func(): _guard(func(): closed.emit(); queue_free()), "back").size_flags_vertical = Control.SIZE_SHRINK_END
	var tools := HBoxContainer.new(); tools.add_theme_constant_override("separation", 8); body.add_child(tools)
	_button(tools, "New blank ability", func(): _guard(_new_blank), "new")
	var start := STYLE.label(tools, "or start from", 14, STYLE.MUTED); start.autowrap_mode = TextServer.AUTOWRAP_OFF; start.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_template = OptionButton.new(); _template.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _template.fit_to_longest_item = false; tools.add_child(_template)
	_button(tools, "Edit template", func(): _guard(func(): _load(false)), "load").tooltip_text = "Edit the selected ability itself."
	_button(tools, "Create a copy", func(): _guard(func(): _load(true)), "clone").tooltip_text = "Start a new ability from the selected ability's settings."
	_tabs = TabBar.new()
	for text in ["Ability", "Requirements & effects", "Follow-ups", "Character boards", "Advanced JSON"]: _tabs.add_tab(text)
	body.add_child(_tabs); _tabs.tab_changed.connect(_change_tab)
	var split := HSplitContainer.new(); split.size_flags_vertical = Control.SIZE_EXPAND_FILL; split.add_theme_constant_override("separation", 14); body.add_child(split)
	var scroll := ScrollContainer.new(); _scroll = scroll; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.size_flags_stretch_ratio = 1.7; scroll.custom_minimum_size.x = 540; split.add_child(scroll)
	_fields = VBoxContainer.new(); _fields.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _fields.add_theme_constant_override("separation", 10); scroll.add_child(_fields)
	var preview_panel := PanelContainer.new(); preview_panel.custom_minimum_size.x = 300; preview_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	preview_panel.add_theme_stylebox_override("panel", STYLE.box("0a121a", STYLE.BRONZE, 14, 10)); split.add_child(preview_panel)
	var right := ScrollContainer.new(); right.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; preview_panel.add_child(right)
	var preview_box := VBoxContainer.new(); preview_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL; preview_box.add_theme_constant_override("separation", 10); right.add_child(preview_box)
	STYLE.heading(preview_box, "Ability preview")
	_preview_badges = HFlowContainer.new(); _preview_badges.add_theme_constant_override("h_separation", 6); preview_box.add_child(_preview_badges)
	_preview_name = STYLE.label(preview_box, "", 22, STYLE.GOLD_BRIGHT)
	var rules_box := STYLE.section(preview_box, 12)
	STYLE.heading(rules_box, "Rules")
	_preview = _label(rules_box, "", 17); _preview.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	STYLE.label(preview_box, "Rules text is generated from the configuration when it validates.", 13, STYLE.DIM)
	_status = _label(body, "", 15); _status.custom_minimum_size.y = 34; _status.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_json_warning = HBoxContainer.new(); body.add_child(_json_warning); _json_warning.hide()
	_json_warning_text = _label(_json_warning, "", 14); _json_warning_text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _json_warning_text.add_theme_color_override("font_color", STYLE.AMBER)
	_button(_json_warning, "Discard JSON edits", _discard_json, "discard_json")
	_notice = HBoxContainer.new(); _notice.add_theme_constant_override("separation", 8); body.add_child(_notice); _notice.hide()
	_label(_notice, "Discard unpublished edits?", 16).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(_notice, "Keep editing", func(): _notice.hide(), "keep_editing")
	STYLE.accent(_button(_notice, "Discard edits", func(): _notice.hide(); _pending.call(), "discard_edits"), STYLE.LOSS)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 8); body.add_child(actions)
	_button(actions, "Validate and preview", _validate, "validate")
	_publish = STYLE.accent(_button(actions, "Publish for future battles", _save, "publish"))
	STYLE.label(actions, "Active battles keep their pinned definitions.", 13, STYLE.MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_timer = Timer.new(); _timer.one_shot = true; _timer.wait_time = 0.4; _timer.timeout.connect(_validate); add_child(_timer)
	if _reload(): _new_blank()

func _unhandled_key_input(event: InputEvent) -> void:
	if not event.is_action_pressed("ui_cancel"): return
	if _notice.visible: _notice.hide()
	else: _guard(func(): closed.emit(); queue_free())
	get_viewport().set_input_as_handled()

func _change_tab(_index: int) -> void:
	# Navigation never depends on publish eligibility. Keep unreadable JSON
	# separately so it cannot trap navigation or corrupt the guided form draft.
	if is_instance_valid(_json): _read_json()
	_render()
	_validate()

func _reset_json() -> void:
	_json_buffer = ""; _json_pending = false; _json_initial_text = ""
	_json_warning.hide()

func _discard_json() -> void:
	_reset_json(); _render(); _validate()

func _read_json() -> bool:
	if not is_instance_valid(_json): return not _json_pending
	var text := _json.text
	if text != _json_initial_text: _dirty = true
	var parser := JSON.new()
	var error := ""
	if parser.parse(text) != OK: error = "Invalid JSON: " + parser.get_error_message()
	elif not parser.data is Dictionary: error = "The JSON definition must be an object."
	else: error = _form_shape_error(parser.data)
	if not error.is_empty():
		_json_buffer = text; _json_pending = true; _publish.disabled = true
		_status.text = error
		_json_warning_text.text = "Advanced JSON needs correction. Your text is preserved there; other tabs show the last readable draft. Fix it or discard the JSON edits."
		_json_warning.show(); return false
	draft = parser.data
	_json_pending = false; _json_buffer = ""; _json_warning.hide()
	return true

func _form_shape_error(data: Dictionary) -> String:
	if data.get("type") not in ["offensive", "defensive"]: return "Choose an offensive or defensive type in the JSON definition."
	for key in ["id", "name"]:
		if not data.get(key) is String: return "JSON field '%s' must be text." % key
	for key in ["cost", "usage", "presentation", "qualification", "targeting"] if data.type == "offensive" else ["cost", "usage", "presentation", "selection", "resolution"]:
		if not data.get(key) is Dictionary: return "JSON field '%s' must be an object." % key
	if data.has("optional_payment") and not data.optional_payment is Dictionary: return "Optional payment must be an object."
	# These arrays are read directly by the guided editor. Semantic rules remain
	# the native validator's job and do not prevent navigating between tabs.
	if data.has("hooks") and not data.hooks is Array: return "JSON hooks must be an array."
	if not data.has("hooks"): data.hooks = []
	if data.type == "offensive":
		for key in ["activation_tiers", "conditional_bonuses"]:
			if not data.qualification.get(key) is Array: return "JSON qualification.%s must be an array." % key
	else:
		if not data.resolution.get("operations") is Array: return "JSON resolution.operations must be an array."
		if not data.has("saved_card_destination"): data.saved_card_destination = "original"
	return ""

func _request(op: String, character: String = "", board: Dictionary = {}) -> Dictionary:
	return get_node("/root/LearnedBattleRuntime").ability_authoring(op, draft, revision, character, board)
func _reload() -> bool:
	var r := _request("ability_authoring")
	if not r.get("ok", false): _status.text = str(r.get("error")); return false
	catalog = r.result; revision = int(catalog.revision); _template.clear()
	var ids: Array = catalog.templates.keys(); ids.sort()
	for id in ids:
		_template.add_item(str(catalog.templates[id].name)); _template.set_item_metadata(_template.item_count - 1, id)
		if id == draft.get("id", ""): _template.select(_template.item_count - 1)
	return true
func _load(copy: bool) -> void:
	if _template.selected < 0:
		_status.text = "Choose an optional template, or use New blank ability."; return
	_type_drafts.clear()
	_reset_json()
	draft = catalog.templates[_template.get_selected_metadata()].duplicate(true)
	if copy:
		var base := str(draft.id) + "_custom"; var candidate := base; var number := 2
		while catalog.templates.has(candidate): candidate = base + "_" + str(number); number += 1
		draft.id = candidate
		var base_name := str(draft.name) + " Custom"; var copy_name := base_name; number = 2
		while catalog.templates.values().any(func(a): return a.name == copy_name): copy_name = base_name + " " + str(number); number += 1
		draft.name = copy_name
	draft.configuration_version = 1
	if not draft.has("hooks"): draft.hooks = []
	if draft.type == "offensive":
		if not draft.qualification.has("choose_tier"): draft.qualification.choose_tier = false
		if draft.qualification.get("conditional_bonuses") == null: draft.qualification.conditional_bonuses = []
	else:
		if not draft.selection.has("offensive_abilities_only"): draft.selection.offensive_abilities_only = false
		if not draft.resolution.has("energy_gain_limit"): draft.resolution.energy_gain_limit = 0
	if not draft.has("saved_card_destination"): draft.saved_card_destination = "original"
	_dirty = copy; _render(); _validate()

func _new_blank() -> void:
	_timer.stop(); _type_drafts.clear()
	_reset_json()
	_template.select(-1); _template.text = "Optional template…"
	draft = {"schema_version": 1, "configuration_version": 1, "id": "", "name": "", "type": "offensive", "cost": {"energy": 0}, "usage": {"maximum_per_segment": 1}, "presentation": {"rules_text": ""}}
	draft.merge(_blank_type_fields("offensive"))
	_dirty = false
	_tabs.set_block_signals(true); _tabs.current_tab = 0; _tabs.set_block_signals(false)
	_render(); _validate()

func _blank_type_fields(kind: String) -> Dictionary:
	if kind == "defensive":
		return {"hooks": [], "saved_card_destination": "original", "selection": {"requires_incoming_proposal": true, "allowed_proposal_types": ["damage_source"], "target_count": 1, "offensive_abilities_only": false}, "resolution": {"operations": [], "energy_gain_limit": 0, "reaction_window": {"opens": true, "pass_required": true}}}
	return {"hooks": [], "targeting": {"selector": "one_enemy", "minimum": 1, "maximum": 1}, "qualification": {"choose_tier": false, "activation_tiers": [], "conditional_bonuses": []}}

func _update_type_lock() -> void:
	if not is_instance_valid(_type_choice): return
	var existing: bool = catalog.templates.has(str(draft.get("id", "")))
	_type_choice.disabled = existing
	_type_hint.text = "Published abilities keep their type. Use a new ID, Create a copy, or New blank ability to choose a different type." if existing else "Offensive abilities use attack requirements. Defensive abilities respond to incoming damage. Switching type keeps each type's draft settings while this editor is open."

func _select_type(index: int) -> void:
	var kind := "offensive" if index == 0 else "defensive"
	if kind == draft.type: return
	if catalog.templates.has(str(draft.id)):
		_type_choice.select(0 if draft.type == "offensive" else 1); return
	var fields := ["targeting", "qualification", "selection", "resolution", "saved_card_destination", "optional_payment", "hooks"]
	var previous := {}
	for key in fields:
		if draft.has(key): previous[key] = draft[key]
		draft.erase(key)
	_type_drafts[str(draft.type)] = previous.duplicate(true)
	draft.type = kind
	draft.merge(_type_drafts.get(kind, _blank_type_fields(kind)).duplicate(true))
	draft.presentation.rules_text = ""
	_changed(); _render(); _validate()
func _guard(action: Callable) -> void:
	if is_instance_valid(_json) and _json.text != _json_initial_text: _dirty = true
	if _dirty: _pending = action; _notice.show()
	else: action.call()
func _changed(rebuild: bool = false) -> void:
	_dirty = true; _publish.disabled = true; _timer.start()
	_update_type_lock()
	if rebuild: _render.call_deferred()
func _label(parent: Node, text: String, size: int = 18) -> Label:
	var l := Label.new(); l.text = text; l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; l.add_theme_font_size_override("font_size", size)
	if parent is GridContainer:
		l.custom_minimum_size.x = 230; l.add_theme_color_override("font_color", STYLE.MUTED); l.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	parent.add_child(l); return l
func _display(value: String) -> String:
	if catalog.get("statuses", {}).get(value) is Dictionary and catalog.statuses[value].has("name"): return str(catalog.statuses[value].name)
	if catalog.get("dice", {}).get(value) is Dictionary and catalog.dice[value].has("name"): return str(catalog.dice[value].name)
	if catalog.get("symbols", {}).get(value) is Dictionary and catalog.symbols[value].has("name"): return str(catalog.symbols[value].name)
	return value.replace("_", " ").capitalize()
func _caption(key: String) -> String:
	return str(CAPTIONS.get(key, key.replace("_", " ").capitalize()))
func _section(parent: Node, title: String) -> VBoxContainer:
	var depth := int(parent.get_meta("depth", 0))
	var box: VBoxContainer
	if depth >= 1:
		# Inside a list item, nested groups are headed rows rather than more boxes.
		box = VBoxContainer.new(); box.add_theme_constant_override("separation", 6); parent.add_child(box)
	else:
		box = STYLE.section(parent, 14)
	box.set_meta("depth", depth + 1)
	STYLE.heading(box, title)
	return box
func _button(parent: Node, text: String, action: Callable, key: String = "") -> Button:
	var b := Button.new(); b.text = text; b.pressed.connect(action); b.set_meta("editor_key", key); parent.add_child(b); return b
func _line(parent: Node, key: String, data: Dictionary) -> void:
	_label(parent, {"id": "Ability ID", "name": "Name"}.get(key, _caption(key)), 16)
	var field := LineEdit.new(); field.text = str(data.get(key, "")); field.set_meta("editor_key", key); field.text_changed.connect(func(v): data[key] = v; _changed()); parent.add_child(field)
	field.size_flags_horizontal = Control.SIZE_EXPAND_FILL
func _render() -> void:
	_json = null
	_type_choice = null; _type_hint = null
	for node in _fields.get_children(): _fields.remove_child(node); node.queue_free()
	if draft.is_empty(): return
	match _tabs.current_tab:
		0:
			var identity := _section(_fields, "Identity")
			var form := STYLE.form(identity)
			_line(form, "id", draft); _line(form, "name", draft)
			_label(form, "Ability type", 16)
			_type_choice = OptionButton.new(); _type_choice.set_meta("editor_key", "ability_type"); _type_choice.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			_type_choice.add_item("Offensive"); _type_choice.add_item("Defensive"); _type_choice.select(0 if draft.type == "offensive" else 1)
			_type_choice.set_item_metadata(0, "offensive"); _type_choice.set_item_metadata(1, "defensive")
			form.add_child(_type_choice); _type_choice.item_selected.connect(_select_type)
			_type_hint = _label(identity, "", 13); _type_hint.add_theme_color_override("font_color", STYLE.MUTED); _update_type_lock()
			var costs := _section(_fields, "Cost & usage")
			var cost_form := STYLE.form(costs)
			_object(cost_form, draft.cost, "cost"); _object(cost_form, draft.usage, "usage")
			if draft.type == "offensive":
				_object(_section(_fields, "Targeting"), draft.targeting, "targeting")
			else:
				var defense := _section(_fields, "Defense")
				var defense_form := STYLE.form(defense)
				_value(defense_form, "saved_card_destination", draft, "ability")
				_object(defense, draft.selection, "selection")
				var check := CheckBox.new(); check.text = "Optional status payment before defense"; check.button_pressed = draft.get("optional_payment") is Dictionary; defense.add_child(check)
				check.toggled.connect(func(v):
					if v: draft.optional_payment = {"status_id": "catalyst", "stacks": 1, "prevention": 2}
					else: draft.erase("optional_payment")
					_changed(true))
				if check.button_pressed: _object(defense, draft.optional_payment, "optional_payment")
		1:
			_label(_fields, "Add an activation tier, then set its requirements and effects." if draft.type == "offensive" else "Add defensive effects. For a rolled defense, add roll_dice and configure its face outcomes.", 14).add_theme_color_override("font_color", STYLE.MUTED)
			if draft.type == "offensive": _object(_fields, draft.qualification, "qualification")
			else: _object(_fields, draft.resolution, "resolution")
		2:
			_label(_fields, "Follow-ups run once per activation after their checkpoint. Face conditions mean any die; prevention conditions require damage actually prevented.", 14).add_theme_color_override("font_color", STYLE.MUTED)
			if draft.hooks.is_empty():
				var empty := _section(_fields, "No follow-ups yet")
				_label(empty, "Follow-ups add extra effects after the ability resolves, such as drawing a card, gaining energy or applying a status. Add one below.", 14).add_theme_color_override("font_color", STYLE.MUTED)
			_array(_fields, draft.hooks, "hooks")
		3: _boards()
		4:
			_label(_fields, "Complete definition. Unknown fields, invalid references and conflicting settings are rejected by the authority.", 14).add_theme_color_override("font_color", STYLE.MUTED)
			_json_initial_text = _json_buffer if _json_pending else JSON.stringify(draft, "  ")
			_json = CodeEdit.new(); _json.custom_minimum_size.y = 600; _json.text = _json_initial_text; _fields.add_child(_json)
			_json.text_changed.connect(func():
				if _json.text == _json_initial_text: return
				_dirty = true; _publish.disabled = true; _timer.start())

## Scalars share one two-column form; nested objects and lists get their own panels.
func _object(parent: Node, data: Dictionary, path: String) -> void:
	var form: GridContainer = parent if parent is GridContainer else null
	for key in data.keys():
		var value = data[key]
		if value == null: continue
		if str(key) == "type" and path != "ability": continue
		var nested: bool = value is Dictionary or (value is Array and str(key) not in TEXT_LISTS)
		if nested and not parent is GridContainer:
			var box := _section(parent, _caption(str(key)))
			if value is Dictionary: _object(box, value, path + "." + str(key))
			else: _array(box, value, str(key))
			form = null
			continue
		if form == null: form = STYLE.form(parent)
		_value(form, str(key), data, path)
func _value(parent: Node, key: String, data: Dictionary, path: String) -> void:
	var value = data[key]
	if value == null: return
	_label(parent, _caption(key), 16)
	if value is Dictionary:
		var panel := VBoxContainer.new(); panel.add_theme_constant_override("separation", 6); panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; parent.add_child(panel); _object(panel, value, path + "." + key)
	elif value is Array: _array(parent, value, key)
	elif value is bool:
		var b := CheckBox.new(); b.text = "Enabled"; b.button_pressed = value; parent.add_child(b); b.toggled.connect(func(v): data[key] = v; _changed())
		b.set_meta("editor_key", path + "." + key)
	elif value is int or value is float:
		var n := SpinBox.new(); n.min_value = 0; n.max_value = 5 if key == "dice_count" else 6 if key == "face" else 100; n.value = float(value); parent.add_child(n); n.value_changed.connect(func(v): data[key] = int(v); _changed())
		n.custom_minimum_size.x = 150; n.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		n.set_meta("editor_key", path + "." + key)
	else:
		var options: Array = []
		match key:
			"status_id", "result_status_id", "fallback_status_id": options = catalog.statuses.keys()
			"symbol_id": options = catalog.symbols.keys()
			"dice_id": options = catalog.dice.keys()
			"saved_card_destination": options = catalog.destinations
			"selector": options = ["one_enemy", "self"]
			"rounding": options = ["down", "floor"]
			"target": options = ["self", "enemy", "selected_proposal", "source_actor"] if draft.type == "defensive" else ["self", "selected_targets", "enemy", "source_actor", "target_actor"]
			"pattern": options = catalog.patterns
			"kind": options = catalog.special_effects
			"timing": options = ["after_defense"] if draft.type == "defensive" else ["before_defense", "after_damage", "after_provoke"]
		if not options.is_empty():
			var select := OptionButton.new(); parent.add_child(select); select.size_flags_horizontal = Control.SIZE_EXPAND_FILL; select.fit_to_longest_item = false
			select.set_meta("editor_key", path + "." + key)
			for option in options:
				select.add_item(_display(str(option))); select.set_item_metadata(select.item_count - 1, option)
				if str(option) == str(value): select.select(select.item_count - 1)
			select.item_selected.connect(func(i): data[key] = options[i]; _changed())
		else:
			var text := LineEdit.new(); text.text = str(value); parent.add_child(text); text.text_changed.connect(func(v): data[key] = v; _changed())
			text.size_flags_horizontal = Control.SIZE_EXPAND_FILL
func _array(parent: Node, values: Array, key: String) -> void:
	if key in TEXT_LISTS:
		var text := LineEdit.new(); var parts := PackedStringArray()
		for value in values: parts.append(str(value))
		text.text = ", ".join(parts); parent.add_child(text); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		text.placeholder_text = "comma-separated"
		text.text_changed.connect(func(v):
			values.clear()
			for item in v.split(",", false): values.append(item.strip_edges() if key == "allowed_proposal_types" else int(item))
			_changed())
		return
	for i in values.size():
		var card := PanelContainer.new(); card.add_theme_stylebox_override("panel", STYLE.box("0d1822", Color(STYLE.GOLD, 0.3), 12, 8)); parent.add_child(card)
		var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 6); card.add_child(box); box.set_meta("depth", 1)
		var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 8); box.add_child(head)
		var number := STYLE.chip(head, str(i + 1), STYLE.GOLD, 13); number.custom_minimum_size.x = 28; number.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		var title := _label(head, _item_title(key, values[i]), 17); title.size_flags_horizontal = Control.SIZE_EXPAND_FILL; title.add_theme_color_override("font_color", STYLE.GOLD_BRIGHT)
		STYLE.accent(_button(head, "Remove", func(): values.remove_at(i); _changed(true)), STYLE.LOSS)
		if values[i] is Dictionary: _object(box, values[i], key)
	var choices: Array = [key]
	if key == "operations":
		choices = catalog.effects.filter(func(effect): return effect not in (["deal_damage", "provoke"] if draft.type == "defensive" else ["prevent_damage", "scale_damage"]))
		if _tabs.current_tab == 2: choices = choices.filter(func(effect): return effect in ["draw_cards", "gain_resource", "apply_status", "remove_status_stack", "special_effect", "noop"])
	elif key == "all": choices = ["symbol_count", "number_pattern", "exact_faces"]
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); parent.add_child(row)
	var select := OptionButton.new(); row.add_child(select); select.size_flags_horizontal = Control.SIZE_EXPAND_FILL; select.fit_to_longest_item = false
	select.set_meta("editor_key", "add_kind." + key)
	for option in choices: select.add_item(str(option).replace("_", " ").capitalize()); select.set_item_metadata(select.item_count - 1, option)
	select.visible = choices.size() > 1
	STYLE.accent(_button(row, "+ Add " + _caption(key).to_lower().trim_suffix("s"), func(): values.append(_new_item(str(select.get_selected_metadata()))); _changed(true), "add." + key))

func _item_title(key: String, item) -> String:
	if item is Dictionary and item.has("type"): return str(item.type).replace("_", " ").capitalize()
	if key == "activation_tiers": return "Tier"
	if key == "conditional_bonuses": return "Bonus"
	if key == "hooks": return "Follow-up · " + str(item.get("timing", "")).replace("_", " ") if item is Dictionary else "Follow-up"
	return _caption(key).trim_suffix("s")
func _new_item(kind: String) -> Dictionary:
	match kind:
		"activation_tiers", "conditional_bonuses": return {"id": "tier_" + str(Time.get_ticks_msec()), "requirements": {"all": [{"type": "number_pattern", "pattern": "pair_or_better"}]}, "operations": [{"type": "deal_damage", "target": "selected_targets", "amount": 2}]}
		"hooks": return {"timing": "after_defense" if draft.type == "defensive" else "after_damage", "tier_id": "", "faces_any": [], "requires_prevention": false, "operations": [{"type": "draw_cards", "target": "self", "amount": 1}]}
		"symbol_count": return {"type": kind, "symbol_id": "sword", "minimum": 2, "maximum": 5}
		"number_pattern": return {"type": kind, "pattern": "pair_or_better"}
		"exact_faces": return {"type": kind, "faces": [1, 2, 3, 4, 5]}
		"outcomes": return {"faces": [1], "operations": [{"type": "prevent_damage", "target": "selected_proposal", "amount": 1}]}
		"roll_dice": return {"type": kind, "target": "self", "dice_id": "standard_d6", "dice_count": 1, "outcomes": [{"faces": [1,2,3,4,5,6], "operations": [{"type": "prevent_damage" if draft.type == "defensive" else "deal_damage", "target": "selected_proposal" if draft.type == "defensive" else "selected_targets", "amount": "rolled_face"}]}]}
		"apply_status", "remove_status_stack": return {"type": kind, "target": "self", "status_id": "protect", "stack_count": 1}
		"special_effect": return {"type": kind, "target": "enemy" if draft.type == "defensive" else "selected_targets", "special": {"kind": "curse", "fallback_status_id": "poison", "fallback_stacks": 1, "amount": 1, "limit": 2, "face": 1, "status_id": "curse_count", "threshold": 6, "result_status_id": "blind", "stacks": 1}}
		"scale_damage": return {"type": kind, "target": "selected_proposal", "numerator": 1, "denominator": 2, "rounding": "down"}
		"gain_resource": return {"type": kind, "target": "self", "resource": "energy", "amount": 1}
		"noop", "apply_incubation", "incubation_or_poison": return {"type": kind, "target": "enemy"}
	return {"type": kind, "target": "selected_proposal" if kind == "prevent_damage" else ("self" if kind == "draw_cards" else "selected_targets"), "amount": 1}
func _validate() -> bool:
	_badges()
	var ok := _validate_draft()
	_tone()
	return ok

func _badges() -> void:
	if not is_instance_valid(_preview_badges) or draft.is_empty(): return
	_preview_name.text = str(draft.get("name", "")) if not str(draft.get("name", "")).strip_edges().is_empty() else "New ability"
	for child in _preview_badges.get_children(): _preview_badges.remove_child(child); child.queue_free()
	var offensive := str(draft.get("type", "offensive")) == "offensive"
	STYLE.chip(_preview_badges, "Offensive" if offensive else "Defensive", STYLE.OFFENSE if offensive else STYLE.DEFENSE)
	STYLE.chip(_preview_badges, "%d energy" % int(draft.get("cost", {}).get("energy", 0)) if draft.get("cost") is Dictionary else "energy", STYLE.ENERGY)
	if offensive and draft.get("qualification") is Dictionary: STYLE.chip(_preview_badges, "%d tier%s" % [draft.qualification.get("activation_tiers", []).size(), "" if draft.qualification.get("activation_tiers", []).size() == 1 else "s"], STYLE.GOLD)

func _tone() -> void:
	var text := _status.text
	var color := STYLE.GAIN if text.begins_with("Valid") or text.begins_with("Published") or text.begins_with("Ability board saved") else STYLE.LOSS
	if text.begins_with("Enter an ID"): color = STYLE.MUTED
	_status.add_theme_color_override("font_color", color)
	_status.add_theme_stylebox_override("normal", STYLE.box(Color(color, 0.1), Color(color, 0.45), 14, 8))

func _validate_draft() -> bool:
	if not _read_json():
		_publish.disabled = true; return false
	if str(draft.get("id", "")).strip_edges().is_empty() or str(draft.get("name", "")).strip_edges().is_empty():
		_status.text = "Enter an ID and name, choose Offensive or Defensive, then add Requirements & effects."
		_preview.text = "New " + str(draft.get("type", "offensive")) + " ability"; _publish.disabled = true; return false
	var r := _request("validate_ability"); _publish.disabled = not r.get("ok", false)
	if not r.get("ok", false): _status.text = str(r.get("error")); _preview.text = "Complete the configuration to preview its rules."; return false
	_preview.text = str(r.result.rules)
	draft.presentation.rules_text = r.result.rules
	_status.text = "Valid configuration · ready to publish"; _tone(); return true
func _save() -> void:
	_timer.stop()
	if not _validate(): return
	var r := _request("publish_ability")
	if not r.get("ok", false): _status.text = str(r.get("error")); return
	_dirty = false; revision = int(r.result.revision); catalog.templates[draft.id] = r.result.ability
	_reload()
	_update_type_lock()
	_status.text = "Published " + str(draft.name) + ". Assign it in Character boards."; _tone()
func _boards() -> void:
	var intro := _section(_fields, "Character boards")
	_label(intro, "Choose a character, then its complete ability board. Assignments affect future battles in both editor modes.", 14).add_theme_color_override("font_color", STYLE.MUTED)
	_character = OptionButton.new(); intro.add_child(_character)
	for id in ["adventurer", "venom", "curse", "blade_warden", "starter"]: _character.add_item(id.replace("_", " ").capitalize()); _character.set_item_metadata(_character.item_count - 1, id)
	var list := VBoxContainer.new(); list.add_theme_constant_override("separation", 10); _fields.add_child(list)
	_character.item_selected.connect(func(_i): _board_list(list))
	_board_list(list)
func _board_list(parent: Node) -> void:
	for child in parent.get_children(): parent.remove_child(child); child.queue_free()
	var id := str(_character.get_selected_metadata()); _board = catalog.boards[id].duplicate(true)
	var ids: Array = catalog.templates.keys(); ids.sort_custom(func(a, b): return str(catalog.templates[a].name).naturalnocasecmp_to(str(catalog.templates[b].name)) < 0)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 12); parent.add_child(columns)
	for kind in ["offensive", "defensive"]:
		var box := _section(columns, kind.capitalize())
		box.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
		for ability_id in ids:
			var a: Dictionary = catalog.templates[ability_id]
			if a.type != kind: continue
			var check := CheckBox.new(); check.text = str(a.name); check.button_pressed = ability_id in _board[kind]; box.add_child(check)
			check.disabled = ability_id not in catalog.get("compatible", {}).get(id, [])
			check.tooltip_text = str(a.get("presentation", {}).get("rules_text", ""))
			if check.disabled: check.text += " · needs compatible dice or type"
			check.toggled.connect(func(v):
				if v: _board[kind].append(ability_id)
				else: _board[kind].erase(ability_id))
	STYLE.accent(_button(parent, "Apply ability board", func():
		var r := _request("assign_abilities", id, _board)
		if r.get("ok", false): revision = int(r.result.revision); catalog.boards[id] = _board.duplicate(true); _status.text = "Ability board saved for " + id
		else: _status.text = str(r.get("error"))
		_tone(), "assign"))
