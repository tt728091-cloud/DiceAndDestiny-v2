extends Control

signal closed
const GOLD := Color("e6c17c")
const MUTED := Color("9cabb7")
const ROSTER := ["adventurer", "venom", "curse", "blade_warden"]
var initial_character := "adventurer"
var character_id := ""
var catalogs: Dictionary = {}
var character: Dictionary = {}
var selected_kind := ""
var selected_id := ""
var _previous_catalog: Dictionary
var _list: VBoxContainer
var _details: VBoxContainer
var _summary: VBoxContainer
var _portrait: TextureRect
var _tabs: TabBar
var _search: LineEdit
var _roster_buttons: Dictionary = {}
var _entry_buttons: Array[Button] = []
var _error: Label

func _ready() -> void:
	name = "CharacterCreation"
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_previous_catalog = BattlePresentationCatalog._catalog.duplicate(true)
	_build()
	reload_catalogs()

func _style(fill: String, border: String = "293a47", padding: int = 16) -> StyleBoxFlat:
	var s := StyleBoxFlat.new(); s.bg_color = Color(fill); s.border_color = Color(border)
	s.set_border_width_all(1); s.set_corner_radius_all(8)
	s.content_margin_left = padding; s.content_margin_right = padding
	s.content_margin_top = padding; s.content_margin_bottom = padding
	return s

func _label(parent: Node, text: String, font_size: int = 18, color: Color = Color("e9e6de")) -> Label:
	var l := Label.new(); l.text = text; l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.add_theme_font_size_override("font_size", font_size); l.add_theme_color_override("font_color", color)
	parent.add_child(l); return l

func _button(parent: Node, text: String, action: Callable, key: String) -> Button:
	var b := Button.new(); b.text = text; b.custom_minimum_size.y = 44
	b.add_theme_stylebox_override("normal", _style("14222d"))
	b.add_theme_stylebox_override("hover", _style("253a46", "b99a60"))
	b.add_theme_stylebox_override("pressed", _style("30434a", "e6c17c"))
	b.add_theme_stylebox_override("focus", _style("253a4633", "e6c17c", 0))
	b.add_theme_font_size_override("font_size", 16)
	b.pressed.connect(action); parent.add_child(b)
	b.set_meta("character_control", key)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control("character." + key, b, text)
	return b

func _panel(parent: Node, width: float = 0) -> VBoxContainer:
	var p := PanelContainer.new(); p.add_theme_stylebox_override("panel", _style("101b25"))
	p.custom_minimum_size.x = width; p.size_flags_vertical = Control.SIZE_EXPAND_FILL
	parent.add_child(p)
	var v := VBoxContainer.new(); v.add_theme_constant_override("separation", 12); p.add_child(v)
	return v

func _scroll(parent: Node) -> VBoxContainer:
	var s := ScrollContainer.new(); s.size_flags_vertical = Control.SIZE_EXPAND_FILL
	s.size_flags_horizontal = Control.SIZE_EXPAND_FILL; s.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; s.follow_focus = true
	parent.add_child(s)
	var v := VBoxContainer.new(); v.size_flags_horizontal = Control.SIZE_EXPAND_FILL; v.add_theme_constant_override("separation", 8); s.add_child(v)
	return v

func _build() -> void:
	var bg := ColorRect.new(); bg.color = Color("090f17"); bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 24)
	add_child(margin)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 16); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 18); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; header.add_child(titles)
	_label(titles, "DICE & DESTINY  /  CHARACTERS", 12, GOLD)
	_label(titles, "Character Creation", 36)
	_button(header, "Reload definitions", reload_catalogs, "reload")
	_button(header, "Back to battle setup", _close, "back")
	_error = _label(body, "", 16, Color("ffae9f")); _error.hide()
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 16)
	columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	var left := _panel(columns, 240)
	_label(left, "YOUR CHARACTERS", 12, GOLD)
	for id in ROSTER:
		var b := _button(left, id.replace("_", " ").capitalize(), func(): select_character(id), "select." + id)
		b.toggle_mode = true; b.alignment = HORIZONTAL_ALIGNMENT_LEFT
		_roster_buttons[id] = b
	_portrait = TextureRect.new(); _portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; _portrait.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_portrait.custom_minimum_size.y = 60; left.add_child(_portrait)
	_label(left, "Every card is a point of health.", 14, MUTED)
	var middle := _panel(columns); middle.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_summary = VBoxContainer.new(); _summary.add_theme_constant_override("separation", 6); middle.add_child(_summary)
	_tabs = TabBar.new(); _tabs.add_tab("Abilities"); _tabs.add_tab("Deck"); _tabs.add_tab("Dice")
	_tabs.add_theme_font_size_override("font_size", 17); middle.add_child(_tabs)
	_tabs.tab_changed.connect(func(_tab): _search.text = ""; _populate())
	_search = LineEdit.new(); _search.placeholder_text = "Find a card in this deck…"; _search.custom_minimum_size.y = 38
	middle.add_child(_search); _search.text_changed.connect(func(_text): _populate_entries())
	_list = _scroll(middle)
	var right := _panel(columns, 390)
	_label(right, "INSPECT LOADOUT", 12, GOLD)
	_details = _scroll(right)
	_label(body, "Loadout preview  ·  Select a card, ability, or die to inspect it. Deck editing and upgrades come next.", 13, MUTED)

func reload_catalogs() -> void:
	var runtime = get_node_or_null("/root/LearnedBattleRuntime")
	var response: Dictionary = runtime.character_catalogs() if runtime != null else {"ok": false, "error": "Character catalog unavailable."}
	if not response.get("ok", false):
		_error.text = "Could not load characters: " + str(response.get("error", "Unknown error")); _error.show(); return
	_error.hide(); catalogs = response.get("result", {})
	select_character(character_id if not character_id.is_empty() else initial_character)

func _clear(parent: Node) -> void:
	for child in parent.get_children(): parent.remove_child(child); child.queue_free()

func select_character(id: String) -> void:
	if not catalogs.has(id): return
	character_id = id; character = catalogs[id].combatants[id]
	BattlePresentationCatalog.configure(catalogs[id])
	for key in _roster_buttons:
		_roster_buttons[key].set_pressed_no_signal(key == id)
		_roster_buttons[key].text = catalogs[key].combatants[key].name
	_portrait.texture = load("res://assets/battle/fighters/%s.png" % ("blade_warden" if id == "adventurer" else id))
	_clear(_summary)
	_label(_summary, str(character.name), 28, GOLD)
	var health := 0
	for entry in character.decklist: health += int(entry.count)
	var stats := HBoxContainer.new(); stats.add_theme_constant_override("separation", 20); _summary.add_child(stats)
	_label(stats, "%d  HEALTH" % health, 26, Color("f1ad9f")).autowrap_mode = TextServer.AUTOWRAP_OFF
	_label(stats, "%d cards · %d unique" % [health, character.decklist.size()], 18, MUTED).autowrap_mode = TextServer.AUTOWRAP_OFF
	_label(_summary, "Opening hand %d  ·  Starting energy %d  ·  Hand limit %d" % [character.resources.starting_hand_size, character.resources.starting_energy, character.resources.hand_limit], 14, MUTED)
	_label(_summary, "Each round: draw %d  ·  gain %d energy" % [character.income.cards, character.income.energy], 14, MUTED)
	_search.text = ""; selected_id = ""; selected_kind = ""
	_populate()

func _populate() -> void:
	_search.visible = _tabs.current_tab == 1
	_populate_entries()
	if _entry_buttons.size() > 0: _entry_buttons[0].pressed.emit()
	else: _clear(_details); _label(_details, "Nothing in this category yet.", 16, MUTED)

func _entry(kind: String, id: String, title: String, subtitle: String, badge: String = "") -> void:
	var b := _button(_list, title, func(): inspect_entry(kind, id), "entry." + kind + "." + id)
	for state in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color", "font_hover_pressed_color"]: b.add_theme_color_override(state, Color.TRANSPARENT)
	b.custom_minimum_size.y = 70; b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.tooltip_text = title; b.clip_text = true
	var m := MarginContainer.new(); b.add_child(m); m.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	m.mouse_filter = Control.MOUSE_FILTER_IGNORE
	for side in ["left", "right", "top", "bottom"]: m.add_theme_constant_override("margin_" + side, 10)
	var row := HBoxContainer.new(); m.add_child(row); row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var v := VBoxContainer.new(); v.size_flags_horizontal = Control.SIZE_EXPAND_FILL; row.add_child(v); v.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var name_label := _label(v, title, 17); name_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var sub := _label(v, subtitle, 13, MUTED); sub.max_lines_visible = 1; sub.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; sub.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var tag := _label(row, badge, 18, GOLD); tag.autowrap_mode = TextServer.AUTOWRAP_OFF; tag.mouse_filter = Control.MOUSE_FILTER_IGNORE
	b.set_meta("entry_kind", kind); b.set_meta("entry_id", id); _entry_buttons.append(b)

func _populate_entries() -> void:
	_clear(_list); _entry_buttons.clear()
	match _tabs.current_tab:
		0:
			for group in ["offensive", "defensive"]:
				var ids: Array = character.get("ability_board", {}).get(group, [])
				_label(_list, "%s  /  %d" % [group.to_upper(), ids.size()], 12, GOLD)
				for id in ids:
					var info := BattlePresentationCatalog.ability(str(id))
					_entry("abilities", str(id), info.name, info.recipe, "ATK" if group == "offensive" else "DEF")
		1:
			for entry in character.get("decklist", []):
				var info := BattlePresentationCatalog.card(str(entry.card_id))
				if not _search.text.is_empty() and not (str(info.name) + " " + str(info.text)).to_lower().contains(_search.text.to_lower()): continue
				_entry("cards", str(entry.card_id), info.name, "%d energy · %s" % [info.cost, info.effect_summary], "×%d" % int(entry.count))
			if _entry_buttons.is_empty(): _label(_list, "No cards match your search.", 16, MUTED)
		2:
			for entry in character.get("dice_loadout", []):
				var die: Dictionary = catalogs[character_id].dice[entry.dice_id]
				_entry("dice", str(entry.dice_id), die.name, "%d faces" % int(die.side_count), "×%d" % int(entry.count))

func inspect_entry(kind: String, id: String) -> void:
	selected_kind = kind; selected_id = id
	for b in _entry_buttons:
		b.add_theme_stylebox_override("normal", _style("253a46", "b99a60") if b.get_meta("entry_id") == id else _style("14222d"))
	_clear(_details)
	var definition: Dictionary = catalogs[character_id][kind][id]
	_label(_details, str(definition.name), 24, GOLD)
	if kind == "cards":
		var frame := CenterContainer.new(); _details.add_child(frame)
		var card := BattleCard.new(); frame.add_child(card); card.configure("preview", id, false)
		card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.tooltip_text = ""
		_label(_details, "%d energy  ·  %s" % [int(definition.cost.energy), str(definition.type).replace("_", " ")], 14, MUTED)
		_label(_details, BattlePresentationCatalog.card(id).text)
		var timing: Array[String] = []
		for window in definition.play.playable_during:
			var label := str(window.segment).replace("_", " ").capitalize()
			if str(window.segment) == "damage_resolution": label = "Defense / damage response"
			if label not in timing: timing.append(label)
		_label(_details, "PLAY WINDOW\n" + ", ".join(timing), 14, MUTED)
		if definition.play.get("before_first_roll", false): _label(_details, "Before your first offensive roll only.", 14, MUTED)
		_label(_details, "After play → " + str(definition.play.destination).capitalize(), 14, MUTED)
	elif kind == "abilities":
		_label(_details, "%s  ·  %d energy" % [str(definition.type).capitalize(), int(definition.cost.energy)], 14, MUTED)
		_label(_details, BattlePresentationCatalog.ability(id).text)
		for tier in BattlePresentationCatalog.offensive_tier_summaries(id):
			_label(_details, str(tier.recipe), 16, GOLD)
			_label(_details, str(tier.summary), 14)
		var uses := int(definition.get("usage", {}).get("maximum_per_segment", 0))
		if uses > 0: _label(_details, "Up to %d use%s per segment." % [uses, "" if uses == 1 else "s"], 14, MUTED)
	else:
		for face in definition.faces:
			_label(_details, "%d   %s   %s" % [int(face.number), BattlePresentationCatalog.symbol_for_die_face(id, int(face.number)), BattlePresentationCatalog.symbol_name_for_die_face(id, int(face.number))], 17)
	if definition.has("saved_card_destination"):
		_label(_details, "SAVED CARDS", 12, GOLD)
		_label(_details, "Go to discard." if definition.saved_card_destination == "discard" else "Return to their piles. Played cards stay played.", 14)

func _unhandled_key_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_cancel"): _close(); get_viewport().set_input_as_handled()

func _close() -> void:
	closed.emit(); queue_free()

func _exit_tree() -> void:
	BattlePresentationCatalog.configure(_previous_catalog)
