extends Control

signal closed
const GOLD := Color("e6c17c")
const MUTED := Color("9cabb7")
const ROSTER := ["adventurer", "venom", "curse", "blade_warden"]
var initial_character := "adventurer"
var loadout_mode := "sandbox"
var _catalog_mode := "sandbox"
var _mode_choice: OptionButton
var _mode_note: Label
var _purchase_overlay: Control
var _purchase_details: VBoxContainer
var _purchase_confirm: Button
var _purchase_cancel: Button
var _pending_purchase: Dictionary = {}
var _admin_button: Button
var _admin_overlay: Control
var _admin_content: VBoxContainer
var _admin_prices: Dictionary = {}
var _admin_budgets: Dictionary = {}
var _admin_draft: Dictionary = {}
var _admin_preview: Label
var _admin_error: Label
var _admin_save: Button
var _admin_search: LineEdit
var _skip_prompt: CheckBox
var _confirmation_options: HBoxContainer
var _confirm_buy: CheckBox
var _confirm_sell: CheckBox
var _transaction_preferences := ConfigFile.new()
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
var _card_workspace: VBoxContainer
var _card_split: HSplitContainer
var _library_pane: VBoxContainer
var _deck_pane: VBoxContainer
var _library_search: LineEdit
var _deck_search: LineEdit
var _library_list: VBoxContainer
var _deck_list: VBoxContainer
var _library_heading: Label
var _deck_heading: Label
var _swap: Button
var _library_buttons: Array[Button] = []
var _deck_buttons: Array[Button] = []
var _roster_buttons: Dictionary = {}
var _entry_buttons: Array[Button] = []
var _error: Label
var _drafts: Dictionary = {}
var _saved: Dictionary = {}
var _apply: Button
var _revert: Button
var _reset: Button
var _save_status: Label
var _quantity: SpinBox
var _confirm: Control
var _cancel_changes: Button
var _discard_changes: Button
var _add_copy: Button
var _pending_action: Callable

func _ready() -> void:
	name = "CharacterCreation"
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_previous_catalog = BattlePresentationCatalog._catalog.duplicate(true)
	_transaction_preferences.load(WorkspacePaths.persistent_file("character_preferences.cfg"))
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
	_mode_choice = OptionButton.new(); _mode_choice.add_item("Sandbox · free editing"); _mode_choice.add_item("Progression · XP")
	_mode_choice.select(1 if loadout_mode == "progression" else 0); header.add_child(_mode_choice)
	_mode_choice.item_selected.connect(_change_mode)
	_button(header, "Reload definitions", func(): _guard_unsaved(reload_catalogs), "reload")
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
	_tabs = TabBar.new(); _tabs.add_tab("Abilities"); _tabs.add_tab("Deck & Library"); _tabs.add_tab("Dice"); _tabs.current_tab = 1
	_tabs.add_theme_font_size_override("font_size", 17); middle.add_child(_tabs)
	_tabs.tab_changed.connect(func(_tab): _populate())
	_list = _scroll(middle)
	_card_workspace = VBoxContainer.new(); _card_workspace.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_card_workspace.add_theme_constant_override("separation", 10); middle.add_child(_card_workspace)
	var card_toolbar := HBoxContainer.new(); _card_workspace.add_child(card_toolbar)
	_label(card_toolbar, "Browse either list. Select a card to edit its copies.", 14, MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_swap = _button(card_toolbar, "Swap sides  ⇄", _swap_card_sides, "swap_sides")
	_card_split = HSplitContainer.new(); _card_split.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_card_split.add_theme_constant_override("separation", 16); _card_workspace.add_child(_card_split)
	_library_pane = VBoxContainer.new(); _library_pane.custom_minimum_size.x = 300
	_library_pane.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _library_pane.add_theme_constant_override("separation", 10); _card_split.add_child(_library_pane)
	_library_heading = _label(_library_pane, "CARD LIBRARY", 16, GOLD)
	_library_search = LineEdit.new(); _library_search.placeholder_text = "Search library…"; _library_search.custom_minimum_size.y = 38
	_library_pane.add_child(_library_search); _library_search.text_changed.connect(func(_text): _populate_entries())
	_library_list = _scroll(_library_pane)
	_deck_pane = VBoxContainer.new(); _deck_pane.custom_minimum_size.x = 300
	_deck_pane.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _deck_pane.add_theme_constant_override("separation", 10); _card_split.add_child(_deck_pane)
	_deck_heading = _label(_deck_pane, "YOUR DECK", 16, GOLD)
	_deck_search = LineEdit.new(); _deck_search.placeholder_text = "Search deck…"; _deck_search.custom_minimum_size.y = 38
	_deck_pane.add_child(_deck_search); _deck_search.text_changed.connect(func(_text): _populate_entries())
	_deck_list = _scroll(_deck_pane)
	var right := _panel(columns, 390)
	_label(right, "INSPECT & CONFIGURE", 12, GOLD)
	_details = _scroll(right)
	var footer := HBoxContainer.new(); footer.add_theme_constant_override("separation", 12); body.add_child(footer)
	_save_status = _label(footer, "", 15, MUTED); _save_status.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_confirmation_options = HBoxContainer.new(); footer.add_child(_confirmation_options)
	_confirm_buy = _confirmation_toggle(_confirmation_options, "Confirm buys", "buy_card")
	_confirm_sell = _confirmation_toggle(_confirmation_options, "Confirm sales", "sell_card")
	_admin_button = _button(footer, "Admin settings", _open_admin, "admin.open")
	_reset = _button(footer, "Reset to template", _reset_draft, "reset")
	_revert = _button(footer, "Revert", _revert_draft, "revert")
	_apply = _button(footer, "Apply deck", _apply_draft, "apply")
	_mode_note = _label(body, "", 13, MUTED)
	_confirm = Control.new(); _confirm.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_confirm)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.75); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _confirm.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _confirm.add_child(center)
	var dialog := _panel(center, 580)
	_label(dialog, "Unsaved deck changes", 28, GOLD)
	_label(dialog, "Discard your unsaved changes? Return to apply the decks you want to keep. Saved decks will stay unchanged.", 18)
	var choices := HBoxContainer.new(); choices.add_theme_constant_override("separation", 12); dialog.add_child(choices)
	_cancel_changes = _button(choices, "Keep editing", func(): _confirm.hide(), "keep_editing")
	_discard_changes = _button(choices, "Discard changes", func(): _confirm.hide(); _pending_action.call(), "discard_changes")
	_focus_pair(_cancel_changes, _discard_changes)
	_confirm.hide()
	_build_purchase_overlay()
	_build_admin_overlay()

func _change_mode(index: int) -> void:
	_mode_choice.select(1 if loadout_mode == "progression" else 0)
	_guard_unsaved(func():
		loadout_mode = "progression" if index == 1 else "sandbox"
		_mode_choice.select(index)
		get_node("/root/LearnedBattleRuntime").selected_loadout_mode = loadout_mode
		reload_catalogs()
	)

func _build_purchase_overlay() -> void:
	_purchase_overlay = Control.new(); _purchase_overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_purchase_overlay)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.8); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _purchase_overlay.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _purchase_overlay.add_child(center)
	var panel := _panel(center, 900); panel.get_parent().custom_minimum_size.y = 650
	_label(panel, "Review XP transaction", 30, GOLD)
	_purchase_details = _scroll(panel)
	_skip_prompt = CheckBox.new(); _skip_prompt.text = "Do not show again"; panel.add_child(_skip_prompt)
	_skip_prompt.add_theme_font_size_override("font_size", 18)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 14); panel.add_child(actions)
	_purchase_cancel = _button(actions, "Cancel", func(): _purchase_overlay.hide(), "purchase.cancel")
	_purchase_confirm = _button(actions, "Confirm purchase", _confirm_purchase, "purchase.confirm")
	_focus_pair(_purchase_cancel, _purchase_confirm)
	_purchase_overlay.hide()

func _confirmation_enabled(kind: String) -> bool:
	return bool(_transaction_preferences.get_value("confirmations", kind, true))

func _confirmation_toggle(parent: Node, caption: String, kind: String) -> CheckBox:
	var toggle := CheckBox.new(); toggle.text = caption
	toggle.button_pressed = _confirmation_enabled(kind)
	toggle.tooltip_text = "Show a review before each card purchase." if kind == "buy_card" else "Show a review before each card sale."
	toggle.toggled.connect(func(enabled): _set_confirmation(kind, enabled))
	parent.add_child(toggle)
	return toggle

func _set_confirmation(kind: String, enabled: bool) -> void:
	_transaction_preferences.set_value("confirmations", kind, enabled)
	_confirm_buy.set_pressed_no_signal(_confirmation_enabled("buy_card"))
	_confirm_sell.set_pressed_no_signal(_confirmation_enabled("sell_card"))
	if _transaction_preferences.save(WorkspacePaths.persistent_file("character_preferences.cfg")) != OK:
		_error.text = "Could not save confirmation preferences. This setting will last only while this screen is open."
		_error.show()

func _focus_pair(first: Button, second: Button) -> void:
	for button in [first, second]:
		var other: Button = second if button == first else first
		for property in ["focus_next", "focus_previous", "focus_neighbor_left", "focus_neighbor_right", "focus_neighbor_top", "focus_neighbor_bottom"]:
			button.set(property, button.get_path_to(other))

func reload_catalogs() -> void:
	var runtime = get_node_or_null("/root/LearnedBattleRuntime")
	var response: Dictionary = runtime.character_catalogs(loadout_mode) if runtime != null else {"ok": false, "error": "Character catalog unavailable."}
	if not response.get("ok", false):
		if not catalogs.is_empty():
			loadout_mode = _catalog_mode
			_mode_choice.select(1 if loadout_mode == "progression" else 0)
			runtime.selected_loadout_mode = loadout_mode
		_error.text = "Could not load characters: " + str(response.get("error", "Unknown error")); _error.show(); return
	_error.hide(); catalogs = response.get("result", {})
	_catalog_mode = loadout_mode
	_drafts.clear(); _saved.clear()
	for id in catalogs:
		_saved[id] = catalogs[id].get("owned_decklist", catalogs[id].combatants[id].decklist).duplicate(true)
		_drafts[id] = _saved[id].duplicate(true)
	select_character(character_id if not character_id.is_empty() else initial_character)

func _clear(parent: Node) -> void:
	for child in parent.get_children(): parent.remove_child(child); child.queue_free()

func select_character(id: String) -> void:
	if not catalogs.has(id): return
	character_id = id; character = catalogs[id].combatants[id].duplicate(true)
	character.decklist = _drafts[id]
	BattlePresentationCatalog.configure(catalogs[id])
	for key in _roster_buttons:
		_roster_buttons[key].set_pressed_no_signal(key == id)
		_roster_buttons[key].text = catalogs[key].combatants[key].name
	_portrait.texture = load("res://assets/battle/fighters/%s.png" % ("blade_warden" if id == "adventurer" else id))
	_refresh_summary()
	_error.visible = catalogs[id].has("loadout_error")
	if _error.visible: _error.text = "Saved deck could not be loaded: " + str(catalogs[id].loadout_error) + ". Apply a valid deck to repair it."
	_deck_search.text = ""; _library_search.text = ""; selected_id = ""; selected_kind = ""
	_populate()
	_refresh_actions()

func _refresh_summary() -> void:
	_clear(_summary)
	_label(_summary, str(character.name), 28, GOLD)
	var health := 0
	for entry in character.decklist: health += int(entry.count)
	var stats := HBoxContainer.new(); stats.add_theme_constant_override("separation", 20); _summary.add_child(stats)
	_label(stats, "%d  HEALTH" % health, 26, Color("f1ad9f")).autowrap_mode = TextServer.AUTOWRAP_OFF
	_label(stats, "%d cards · %d unique" % [health, character.decklist.size()], 18, MUTED).autowrap_mode = TextServer.AUTOWRAP_OFF
	if loadout_mode == "progression":
		var xp := int(catalogs[character_id].progression.xp)
		_label(stats, "%d XP" % xp, 26, GOLD).autowrap_mode = TextServer.AUTOWRAP_OFF
		var deck_value := 0
		for entry in character.decklist: deck_value += int(entry.count) * _card_price(str(entry.card_id))
		_label(_summary, "Card budget: %d XP available + %d XP in deck = %d XP" % [xp, deck_value, xp + deck_value], 15, GOLD)
		var progress: Dictionary = catalogs[character_id].progression
		if int(progress.get("upgrade_spent", 0)) != 0:
			_label(_summary, "Total budget: %d XP · %d XP invested in upgrades" % [int(progress.total_budget), int(progress.upgrade_spent)], 14, MUTED)
	_label(_summary, "Opening hand %d  ·  Starting energy %d  ·  Hand limit %d" % [character.resources.starting_hand_size, character.resources.starting_energy, character.resources.hand_limit], 14, MUTED)
	_label(_summary, "Each round: draw %d  ·  gain %d energy" % [character.income.cards, character.income.energy], 14, MUTED)

func _populate() -> void:
	_card_workspace.visible = _tabs.current_tab == 1
	_list.get_parent().visible = _tabs.current_tab != 1
	_populate_entries()
	var choices := _deck_buttons if _tabs.current_tab == 1 and not _deck_buttons.is_empty() else _entry_buttons
	if not choices.is_empty(): choices[0].pressed.emit()
	else: _clear(_details); _label(_details, "Nothing in this category yet.", 16, MUTED)

func _swap_card_sides() -> void:
	_card_split.move_child(_card_split.get_child(0), 1)
	# Preserve the pane widths as well as each pane's own search and scroll.
	_card_split.split_offset = -_card_split.split_offset

func _entry(kind: String, id: String, title: String, subtitle: String, badge: String = "", target: VBoxContainer = null, source: String = "") -> void:
	var b := _button(_list if target == null else target, title, func(): inspect_entry(kind, id), "entry." + (source + "." if not source.is_empty() else "") + kind + "." + id)
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
	if selected_id == id: b.add_theme_stylebox_override("normal", _style("253a46", "b99a60"))
	b.set_meta("entry_kind", kind); b.set_meta("entry_id", id); _entry_buttons.append(b)
	if source == "library": _library_buttons.append(b)
	elif source == "deck": _deck_buttons.append(b)

func _populate_entries() -> void:
	_clear(_list); _entry_buttons.clear(); _library_buttons.clear(); _deck_buttons.clear()
	match _tabs.current_tab:
		0:
			for group in ["offensive", "defensive"]:
				var ids: Array = character.get("ability_board", {}).get(group, [])
				_label(_list, "%s  /  %d" % [group.to_upper(), ids.size()], 12, GOLD)
				for id in ids:
					var info := BattlePresentationCatalog.ability(str(id))
					_entry("abilities", str(id), info.name, info.recipe, "ATK" if group == "offensive" else "DEF")
		1:
			_populate_card_lists()
		2:
			for entry in character.get("dice_loadout", []):
				var die: Dictionary = catalogs[character_id].dice[entry.dice_id]
				_entry("dice", str(entry.dice_id), die.name, "%d faces" % int(die.side_count), "×%d" % int(entry.count))

func _populate_card_lists() -> void:
	var library_scroll: ScrollContainer = _library_list.get_parent()
	var deck_scroll: ScrollContainer = _deck_list.get_parent()
	var library_position := library_scroll.scroll_vertical
	var deck_position := deck_scroll.scroll_vertical
	_clear(_library_list); _clear(_deck_list)
	_library_heading.text = "CARD LIBRARY · %d" % catalogs[character_id].cards.size()
	_deck_heading.text = "YOUR DECK · %d cards" % _health()
	var ids: Array = catalogs[character_id].cards.keys()
	ids.sort_custom(func(a, b): return str(catalogs[character_id].cards[a].name).naturalnocasecmp_to(str(catalogs[character_id].cards[b].name)) < 0)
	for id in ids:
		var info := BattlePresentationCatalog.card(str(id))
		if not _matches_card(info, _library_search.text): continue
		var price_prefix := "%d XP · " % int(_economy_map("card_prices").get(str(id), catalogs[character_id].economy.default_card_price)) if loadout_mode == "progression" else ""
		_entry("cards", str(id), info.name, price_prefix + "%d energy · %s" % [info.cost, info.effect_summary], "×%d" % _card_count(str(id)), _library_list, "library")
	for entry in character.get("decklist", []):
		var info := BattlePresentationCatalog.card(str(entry.card_id))
		if not _matches_card(info, _deck_search.text): continue
		_entry("cards", str(entry.card_id), info.name, "%d energy · %s" % [info.cost, info.effect_summary], "×%d" % int(entry.count), _deck_list, "deck")
	if _library_buttons.is_empty(): _label(_library_list, "No library cards match your search.", 16, MUTED)
	if _deck_buttons.is_empty(): _label(_deck_list, "Your deck is empty. Add cards from the library." if character.decklist.is_empty() else "No deck cards match your search.", 16, MUTED)
	library_scroll.set_deferred("scroll_vertical", library_position)
	deck_scroll.set_deferred("scroll_vertical", deck_position)

func _matches_card(info: Dictionary, query: String) -> bool:
	return query.strip_edges().is_empty() or (str(info.name) + " " + str(info.text)).to_lower().contains(query.strip_edges().to_lower())

func inspect_entry(kind: String, id: String) -> void:
	selected_kind = kind; selected_id = id
	for b in _entry_buttons:
		b.add_theme_stylebox_override("normal", _style("253a46", "b99a60") if b.get_meta("entry_id") == id else _style("14222d"))
	_clear(_details)
	_quantity = null
	var definition: Dictionary = catalogs[character_id][kind][id]
	_label(_details, str(definition.name), 24, GOLD)
	if kind == "cards":
		if loadout_mode == "progression":
			_progression_actions(kind, id)
		else:
			var quantity_row := HBoxContainer.new(); quantity_row.add_theme_constant_override("separation", 8); _details.add_child(quantity_row)
			_label(quantity_row, "Copies in deck", 16, MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
			_quantity = SpinBox.new(); _quantity.min_value = 0; _quantity.max_value = int(catalogs[character_id].deck_limits.max_copies)
			_quantity.step = 1; _quantity.value = _card_count(id); _quantity.custom_minimum_size = Vector2(100, 44); quantity_row.add_child(_quantity)
			_quantity.value_changed.connect(func(value): _set_card_count(id, int(value)))
			_add_copy = _button(_details, "Add a copy", func(): _set_card_count(id, _card_count(id) + 1), "add." + id)
			_add_copy.disabled = _card_count(id) >= int(catalogs[character_id].deck_limits.max_copies)
			_label(_details, "Set to 0 to remove · up to %d copies" % int(catalogs[character_id].deck_limits.max_copies), 13, MUTED)
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
		if loadout_mode == "progression": _progression_actions(kind, id)
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
	if event.is_action_pressed("ui_cancel") and _admin_overlay.visible:
		_admin_overlay.hide(); get_viewport().set_input_as_handled(); return
	if event.is_action_pressed("ui_cancel"):
		if _purchase_overlay.visible: _purchase_overlay.hide()
		elif _confirm.visible: _confirm.hide()
		else: _close()
		get_viewport().set_input_as_handled()

func _close() -> void:
	_guard_unsaved(func(): closed.emit(); queue_free())

func _guard_unsaved(action: Callable) -> void:
	for id in _drafts:
		if _dirty(id):
			_pending_action = action; _confirm.show(); _cancel_changes.grab_focus(); return
	action.call()

func _deck_counts(deck: Array) -> Dictionary:
	var counts := {}
	for entry in deck: counts[str(entry.card_id)] = int(entry.count)
	return counts

func _dirty(id: String) -> bool:
	return _deck_counts(_drafts[id]) != _deck_counts(_saved[id])

func _card_count(id: String) -> int:
	return int(_deck_counts(_drafts[character_id]).get(id, 0))

func _health() -> int:
	var total := 0
	for entry in _drafts[character_id]: total += int(entry.count)
	return total

func _refresh_actions() -> void:
	var progression := loadout_mode == "progression"
	_admin_button.visible = progression
	_confirmation_options.visible = progression
	_apply.visible = not progression; _revert.visible = not progression; _reset.visible = not progression
	_mode_note.text = "Buying and selling save immediately for your next battle. Cards sell for their current purchase price. Battle rewards and discovery come next." if progression else "Sandbox: freely edit and apply a test deck. Progression has a separate deck and XP balance."
	if progression:
		_save_status.text = "%d XP available · buy and sell cards at equal prices" % int(catalogs[character_id].progression.xp)
		_save_status.add_theme_color_override("font_color", GOLD)
		if _health() == 0: _save_status.text = "Deck empty · buy at least one card before starting a battle."
		return
	var dirty := _dirty(character_id)
	var valid := _health() >= 1 and _health() <= int(catalogs[character_id].deck_limits.max_cards)
	_apply.disabled = (not dirty and not catalogs[character_id].has("loadout_error")) or not valid
	_revert.disabled = not dirty
	_reset.disabled = _deck_counts(_drafts[character_id]) == _deck_counts(catalogs[character_id].combatants[character_id].decklist)
	_save_status.text = "Unsaved changes · %d health" % _health() if dirty else "Saved deck · ready for a new battle"
	_save_status.add_theme_color_override("font_color", GOLD if dirty else MUTED)
	if not valid: _save_status.text = "Deck needs 1–%d cards before applying." % int(catalogs[character_id].deck_limits.max_cards)
	for id in _roster_buttons: _roster_buttons[id].text = str(catalogs[id].combatants[id].name) + (" *" if _dirty(id) else "")

func _set_card_count(id: String, count: int) -> void:
	count = clampi(count, 0, int(catalogs[character_id].deck_limits.max_copies))
	var found := false
	var deck: Array = _drafts[character_id]
	for index in range(deck.size() - 1, -1, -1):
		if str(deck[index].card_id) != id: continue
		found = true
		if count == 0: deck.remove_at(index)
		else: deck[index].count = count
	if not found and count > 0: deck.append({"card_id": id, "count": count})
	character.decklist = deck
	if is_instance_valid(_quantity): _quantity.set_value_no_signal(count)
	if is_instance_valid(_add_copy): _add_copy.disabled = count >= int(catalogs[character_id].deck_limits.max_copies)
	_refresh_summary(); _populate_entries(); _refresh_actions()

func _refresh_draft() -> void:
	character.decklist = _drafts[character_id]
	_refresh_summary(); _populate_entries(); _refresh_actions()
	if selected_kind == "cards" and not selected_id.is_empty(): inspect_entry(selected_kind, selected_id)

func _revert_draft() -> void:
	_drafts[character_id] = _saved[character_id].duplicate(true); _refresh_draft()

func _reset_draft() -> void:
	_drafts[character_id] = catalogs[character_id].combatants[character_id].decklist.duplicate(true); _refresh_draft()

func _apply_draft() -> void:
	if is_instance_valid(_quantity): _quantity.apply()
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").save_character_deck(character_id, _drafts[character_id])
	if not response.get("ok", false):
		_error.text = "Could not save deck: " + str(response.get("error", "Unknown error")); _error.show(); return
	_error.hide(); catalogs[character_id].erase("loadout_error")
	_saved[character_id] = response.result.duplicate(true)
	_drafts[character_id] = _saved[character_id].duplicate(true)
	_refresh_draft()
	_save_status.text = "Deck saved · used in your next battle"


func _exit_tree() -> void:
	BattlePresentationCatalog.configure(_previous_catalog)

func _economy_map(key: String) -> Dictionary:
	var value = catalogs[character_id].economy.get(key)
	return value if value is Dictionary else {}

func _card_price(id: String) -> int:
	return int(_economy_map("card_prices").get(id, catalogs[character_id].economy.default_card_price))

func _progression_actions(kind: String, id: String) -> void:
	var progress: Dictionary = catalogs[character_id].progression
	if kind == "cards":
		_label(_details, "%d %s equipped" % [_card_count(id), "copy" if _card_count(id) == 1 else "copies"], 16, MUTED)
		var price := _card_price(id)
		var buy := _button(_details, "Buy a copy · %d XP" % price, func(): _review_purchase("buy_card", id, price), "buy." + id)
		buy.disabled = int(progress.xp) < price or _card_count(id) >= int(catalogs[character_id].deck_limits.max_copies) or _health() >= int(catalogs[character_id].deck_limits.max_cards)
		var sell := _button(_details, "Sell a copy · +%d XP" % price, func(): _review_purchase("sell_card", id, price), "sell." + id)
		sell.disabled = _card_count(id) < 1
		if int(progress.xp) < price: _label(_details, "Need %d more XP to buy a copy." % (price - int(progress.xp)), 14, MUTED)
	var upgrades := _economy_map("card_upgrades" if kind == "cards" else "ability_upgrades")
	if upgrades.has(id):
		var upgrade: Dictionary = upgrades[id]
		var target: Dictionary = catalogs[character_id][kind][upgrade.to]
		var action := "upgrade_card" if kind == "cards" else "upgrade_ability"
		var button := _button(_details, "Upgrade → %s · %d XP" % [target.name, int(upgrade.xp)], func(): _review_purchase(action, id, int(upgrade.xp)), "upgrade." + id)
		button.disabled = int(progress.xp) < int(upgrade.xp) or (kind == "cards" and (_card_count(id) < 1 or _card_count(str(upgrade.to)) >= int(catalogs[character_id].deck_limits.max_copies))) or (kind == "abilities" and str(upgrade.to) in (character.ability_board.offensive + character.ability_board.defensive))
	if kind == "abilities":
		var has_downgrade := false
		for previous_id in upgrades:
			var path: Dictionary = upgrades[previous_id]
			if str(path.to) != id: continue
			has_downgrade = true
			var previous: Dictionary = catalogs[character_id].abilities[previous_id]
			var refund := int(path.xp)
			var button := _button(_details, "Downgrade → %s · +%d XP" % [previous.name, refund], func(): _review_purchase("downgrade_ability", id, refund, str(previous_id)), "downgrade." + id + "." + str(previous_id))
			button.disabled = int(progress.get("upgrade_spent", 0)) < refund or str(previous_id) in (character.ability_board.offensive + character.ability_board.defensive)
		if not upgrades.has(id) and not has_downgrade: _label(_details, "No upgrade or downgrade configured.", 14, MUTED)

func _review_purchase(kind: String, id: String, cost: int, downgrade_target: String = "") -> void:
	var progress: Dictionary = catalogs[character_id].progression
	_pending_purchase = {"character": character_id, "kind": kind, "id": id, "cost": cost, "revision": int(progress.revision)}
	_clear(_purchase_details)
	var collection := "abilities" if kind in ["upgrade_ability", "downgrade_ability"] else "cards"
	var current: Dictionary = catalogs[character_id][collection][id]
	var upgrade := kind in ["upgrade_card", "upgrade_ability", "downgrade_ability"]
	var downgrade := kind == "downgrade_ability"
	var selling := kind == "sell_card"
	var target_id := str(_economy_map("ability_upgrades" if collection == "abilities" else "card_upgrades")[id].to) if upgrade and not downgrade else id
	if downgrade: target_id = downgrade_target
	_pending_purchase.target_id = target_id
	_skip_prompt.set_pressed_no_signal(false)
	_skip_prompt.visible = not upgrade
	_focus_pair(_purchase_cancel, _purchase_confirm)
	if not upgrade:
		_skip_prompt.tooltip_text = "Skip future sale prompts. Turn Confirm sales back on below the deck lists." if selling else "Skip future buy prompts. Turn Confirm buys back on below the deck lists."
		var cycle: Array[Button] = [_skip_prompt, _purchase_cancel, _purchase_confirm]
		for index in cycle.size():
			cycle[index].focus_next = cycle[index].get_path_to(cycle[(index + 1) % cycle.size()])
			cycle[index].focus_previous = cycle[index].get_path_to(cycle[(index + cycle.size() - 1) % cycle.size()])
		if not _confirmation_enabled(kind):
			_confirm_purchase()
			return
	var target: Dictionary = catalogs[character_id][collection][target_id]
	var card_delta := -1 if selling else 1 if kind == "buy_card" else 0
	_label(_purchase_details, "%s → %s" % [current.name, target.name] if upgrade else ("Sell one %s" if selling else "Add one %s") % target.name, 24, GOLD)
	_label(_purchase_details, "XP: %d → %d    ·    Health: %d → %d" % [int(progress.xp), int(progress.xp) + (cost if selling or downgrade else -cost), _health(), _health() + card_delta], 22)
	if kind == "upgrade_card": _label(_purchase_details, "Replaces one owned copy. Other copies remain unchanged.", 16, MUTED)
	elif kind == "upgrade_ability": _label(_purchase_details, "Replaces the equipped ability in its slot.", 16, MUTED)
	elif downgrade: _label(_purchase_details, "Returns the previous ability tier to the same slot and refunds %d XP. You can buy this upgrade again." % cost, 16, MUTED)
	else: _label(_purchase_details, "Equipped copies: %d → %d" % [_card_count(id), _card_count(id) + card_delta], 16, MUTED)
	if selling:
		_label(_purchase_details, "Returns the current purchase price to your XP balance. You can buy this card again from the library.", 18, MUTED)
		if _health() == 1: _label(_purchase_details, "This empties your deck. Buy at least one card before starting a battle.", 18, GOLD)
	if upgrade:
		_label(_purchase_details, "BEFORE", 14, GOLD)
		_label(_purchase_details, str(current.presentation.rules_text), 18)
	_label(_purchase_details, "AFTER" if upgrade else "CARD RULES", 14, GOLD)
	_label(_purchase_details, str(target.presentation.rules_text), 18)
	_purchase_confirm.text = ("Receive %d XP" if selling or downgrade else "Spend %d XP") % cost
	_purchase_confirm.disabled = false
	_purchase_overlay.show(); _purchase_cancel.grab_focus()

func _confirm_purchase() -> void:
	_purchase_confirm.disabled = true
	var p := _pending_purchase
	if loadout_mode != "progression" or character_id != str(p.character):
		_purchase_overlay.hide(); return
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").purchase_progression(str(p.character), str(p.kind), str(p.id), int(p.revision), int(p.cost), str(p.target_id) if p.kind == "downgrade_ability" else "")
	_purchase_overlay.hide()
	if not response.get("ok", false):
		_error.text = "Transaction not completed: " + str(response.get("error", "Unknown error")) + ". Reload definitions to refresh."; _error.show(); return
	_error.hide()
	var progress: Dictionary = response.result
	catalogs[character_id].progression = progress
	catalogs[character_id].owned_decklist = progress.decklist.duplicate(true)
	catalogs[character_id].combatants[character_id].ability_board = progress.ability_board.duplicate(true)
	character.ability_board = progress.ability_board.duplicate(true)
	_saved[character_id] = progress.decklist.duplicate(true); _drafts[character_id] = _saved[character_id].duplicate(true)
	selected_kind = "abilities" if p.kind in ["upgrade_ability", "downgrade_ability"] else "cards"; selected_id = str(p.target_id)
	_refresh_draft()
	inspect_entry(selected_kind, selected_id)
	if str(p.kind) in ["buy_card", "sell_card"] and _skip_prompt.button_pressed:
		_set_confirmation(str(p.kind), false)

func _build_admin_overlay() -> void:
	_admin_overlay = Control.new(); _admin_overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_admin_overlay)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.85); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _admin_overlay.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _admin_overlay.add_child(center)
	_admin_content = _panel(center, 1150); _admin_content.get_parent().custom_minimum_size.y = 850
	_admin_overlay.hide()

func _open_admin() -> void:
	# Read every character before presenting a coherent economy snapshot.
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").character_catalogs("progression")
	if not response.get("ok", false):
		_error.text = str(response.get("error", "Could not load admin settings")); _error.show(); return
	catalogs = response.result
	_admin_draft = catalogs[character_id].admin_settings.duplicate(true)
	_admin_draft.revision = int(_admin_draft.revision)
	if not _admin_draft.get("card_prices") is Dictionary: _admin_draft.card_prices = {}
	for id in _admin_draft.card_prices: _admin_draft.card_prices[id] = int(_admin_draft.card_prices[id])
	_admin_draft.budgets = {}
	# Recreate the modal so discarded scroll contents cannot inflate its centering container.
	_admin_overlay.queue_free(); _build_admin_overlay()
	_admin_prices.clear(); _admin_budgets.clear()
	_label(_admin_content, "Admin · XP economy", 30, GOLD)
	_label(_admin_content, "Card prices apply to every character. Total budgets include available XP, the equipped deck, and upgrade investment. Changes save together after all characters are checked.", 17, MUTED)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 24); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; _admin_content.add_child(columns)
	var left := VBoxContainer.new(); left.custom_minimum_size.x = 490; left.size_flags_horizontal = Control.SIZE_EXPAND_FILL; columns.add_child(left)
	_label(left, "CARD BUY / SELL PRICE", 16, GOLD)
	_admin_search = LineEdit.new(); _admin_search.placeholder_text = "Search cards…"; left.add_child(_admin_search)
	var prices := _scroll(left)
	var cards: Dictionary = {}
	for catalog in catalogs.values(): cards.merge(catalog.cards)
	var ids: Array = cards.keys(); ids.sort_custom(func(a, b): return str(cards[a].name) < str(cards[b].name))
	for id in ids:
		var row := HBoxContainer.new(); prices.add_child(row)
		_label(row, str(cards[id].name), 17).size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var value := int(_admin_draft.card_prices.get(id, _admin_base_price(id)))
		var spin := _admin_spin(row, value, 1); _admin_prices[id] = spin
		spin.value_changed.connect(func(amount): _admin_draft.card_prices[id] = int(amount); _refresh_admin_preview())
		row.set_meta("search_name", str(cards[id].name).to_lower())
	_admin_search.text_changed.connect(func(query):
		for row in prices.get_children(): row.visible = query.to_lower() in str(row.get_meta("search_name"))
	)
	var right := _scroll(columns); right.get_parent().custom_minimum_size.x = 500
	_label(right, "CHARACTER TOTAL BUDGETS", 16, GOLD)
	for id in ROSTER:
		var row := HBoxContainer.new(); right.add_child(row)
		_label(row, str(catalogs[id].combatants[id].name), 17).size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var budget := int(catalogs[id].progression.total_budget); _admin_draft.budgets[id] = budget
		var spin := _admin_spin(row, budget, 0); _admin_budgets[id] = spin
		spin.value_changed.connect(func(amount): _admin_draft.budgets[id] = int(amount); _refresh_admin_preview())
	_label(right, "AFTER APPLYING", 16, GOLD)
	_admin_preview = _label(right, "", 17); _admin_preview.custom_minimum_size.x = 470
	_admin_error = _label(_admin_content, "", 16, Color("ffae9f")); _admin_error.custom_minimum_size.x = 1080; _admin_error.hide()
	var actions := HBoxContainer.new(); _admin_content.add_child(actions)
	_button(actions, "Close · discard edits", func(): _admin_overlay.hide(), "admin.close")
	_admin_save = _button(actions, "Apply economy changes", _save_admin, "admin.save")
	_refresh_admin_preview(); _admin_overlay.show(); _admin_search.grab_focus()

func _admin_base_price(id: String) -> int:
	var catalog: Dictionary = catalogs[character_id]
	if not catalog.cards.has(id):
		for candidate in catalogs.values():
			if candidate.cards.has(id): catalog = candidate; break
	var prices = catalog.economy.get("card_prices")
	return int(prices.get(id, catalog.economy.default_card_price)) if prices is Dictionary else int(catalog.economy.default_card_price)

func _admin_spin(parent: Node, amount: int, minimum: int) -> SpinBox:
	var spin := SpinBox.new(); spin.min_value = minimum; spin.max_value = 1000000; spin.step = 1; spin.value = amount
	spin.custom_minimum_size.x = 150; spin.suffix = "XP"; parent.add_child(spin); return spin

func _refresh_admin_preview() -> void:
	var lines: PackedStringArray = []; var valid := true
	for id in ROSTER:
		var catalog: Dictionary = catalogs[id]; var progress: Dictionary = catalog.progression
		var value := 0
		var prices = catalog.economy.get("card_prices")
		for entry in progress.decklist:
			var base := int(prices.get(entry.card_id, catalog.economy.default_card_price)) if prices is Dictionary else int(catalog.economy.default_card_price)
			value += int(entry.count) * int(_admin_draft.card_prices.get(entry.card_id, base))
		var spent := int(progress.get("upgrade_spent", 0)); var budget := int(_admin_draft.budgets[id]); var available := budget - value - spent
		lines.append("%s · %d XP total\n%d in deck + %d in upgrades · %d XP available" % [catalog.combatants[id].name, budget, value, spent, available])
		if available < 0: valid = false
	_admin_preview.text = "\n\n".join(lines)
	_admin_error.text = "A character is over budget. Increase its budget here, or close and sell cards before changing prices."
	_admin_error.visible = not valid; _admin_save.disabled = not valid

func _save_admin() -> void:
	_admin_save.disabled = true
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").save_economy_admin(_admin_draft)
	if not response.get("ok", false):
		_admin_error.text = str(response.get("error", "Could not save economy")); _admin_error.show(); _admin_save.disabled = false; return
	_admin_overlay.hide()
	var kind := selected_kind; var id := selected_id
	reload_catalogs()
	if not id.is_empty(): inspect_entry(kind, id)
