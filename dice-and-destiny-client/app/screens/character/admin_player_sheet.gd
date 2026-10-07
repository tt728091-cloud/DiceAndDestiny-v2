extends Control
signal closed
const STYLE = preload("res://app/screens/character/character_style.gd")

var character_id := "adventurer"
var catalogs: Dictionary = {}
var _cards: Dictionary = {}
var _trees: Dictionary = {}
var _selected := ""
var _character: OptionButton
var _search: LineEdit
var _type: OptionButton
var _scope: OptionButton
var _summary: Label
var _count: Label
var _table: Tree
var _details: VBoxContainer
var _rows: Dictionary = {}
var _back: Button
var _stats: HBoxContainer

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	theme = STYLE.theme()
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 20)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 12); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "ADMIN SETTINGS  /  PLAYER SHEET", 12, STYLE.GOLD)
	STYLE.label(titles, "Admin player sheet · All cards", 30, STYLE.GOLD_BRIGHT)
	_back = _button(header, "Back to admin", func(): closed.emit(); queue_free())
	_back.size_flags_vertical = Control.SIZE_SHRINK_END
	STYLE.label(body, "Complete published catalog, including unowned upgrades and cards from every character type. Inspection only; unpublished drafts are not included.", 14, STYLE.MUTED)
	# Character, its ledger at a glance, then search and filters.
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 12); body.add_child(top)
	var picker := VBoxContainer.new(); picker.add_theme_constant_override("separation", 2); picker.size_flags_vertical = Control.SIZE_SHRINK_CENTER; top.add_child(picker)
	STYLE.label(picker, "CHARACTER", 11, STYLE.MUTED)
	_character = OptionButton.new(); _character.custom_minimum_size = Vector2(220, 40); picker.add_child(_character)
	_character.item_selected.connect(func(_index): character_id = str(_character.get_selected_metadata()); _populate())
	_stats = HBoxContainer.new(); _stats.add_theme_constant_override("separation", 10); _stats.size_flags_horizontal = Control.SIZE_EXPAND_FILL; top.add_child(_stats)
	_summary = STYLE.label(body, "", 15, STYLE.LOSS); _summary.hide()
	var filters_panel := PanelContainer.new(); filters_panel.add_theme_stylebox_override("panel", STYLE.box("0c1620", "1f2e3a", 12, 10)); body.add_child(filters_panel)
	var filters := HBoxContainer.new(); filters.add_theme_constant_override("separation", 10); filters_panel.add_child(filters)
	_search = LineEdit.new(); _search.placeholder_text = "Search name, card ID, rules, or tree…"; _search.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _search.clear_button_enabled = true; filters.add_child(_search)
	_search.text_changed.connect(func(_text): _populate())
	_type = OptionButton.new(); _type.custom_minimum_size.x = 170; _type.fit_to_longest_item = false; filters.add_child(_type); _type.item_selected.connect(func(_index): _populate())
	_type.tooltip_text = "Filter by card pool type."
	_scope = OptionButton.new(); _scope.custom_minimum_size.x = 230; _scope.fit_to_longest_item = false; filters.add_child(_scope)
	for caption in ["All cards", "Base / standalone cards", "Tree variants", "Owned by this character"]: _scope.add_item(caption)
	_scope.item_selected.connect(func(_index): _populate())
	_count = STYLE.label(filters, "", 14, STYLE.MUTED); _count.autowrap_mode = TextServer.AUTOWRAP_OFF; _count.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_button(filters, "Refresh published cards", _reload)
	var split := HSplitContainer.new(); split.size_flags_vertical = Control.SIZE_EXPAND_FILL; split.add_theme_constant_override("separation", 14); body.add_child(split)
	_table = Tree.new(); _table.custom_minimum_size = Vector2(480, 220); _table.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _table.size_flags_stretch_ratio = 2
	_table.columns = 7; _table.hide_root = true; _table.column_titles_visible = true; _table.select_mode = Tree.SELECT_ROW; split.add_child(_table)
	var titles_row := ["Card", "Type", "Card tree", "Energy", "XP", "Deck", "Stored"]
	var widths := [240, 120, 170, 80, 70, 70, 80]
	for i in titles_row.size():
		_table.set_column_title(i, titles_row[i]); _table.set_column_expand(i, i == 0); _table.set_column_custom_minimum_width(i, widths[i]); _table.set_column_clip_content(i, true)
		_table.set_column_title_alignment(i, HORIZONTAL_ALIGNMENT_CENTER if i >= 3 else HORIZONTAL_ALIGNMENT_LEFT)
	_table.item_selected.connect(func():
		var item := _table.get_selected()
		if item != null: _selected = str(item.get_metadata(0)); _inspect())
	var detail_panel := PanelContainer.new(); detail_panel.custom_minimum_size.x = 340; detail_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	detail_panel.add_theme_stylebox_override("panel", STYLE.box("0a121a", STYLE.BRONZE, 14, 10)); split.add_child(detail_panel)
	var scroll := ScrollContainer.new(); scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; detail_panel.add_child(scroll)
	_details = VBoxContainer.new(); _details.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _details.add_theme_constant_override("separation", 10); scroll.add_child(_details)
	_reload()

func _unhandled_key_input(event: InputEvent) -> void:
	if event.is_action_pressed("ui_cancel"):
		closed.emit(); queue_free(); get_viewport().set_input_as_handled()

func _label(parent: Node, value: String, font_size: int = 17, color: Color = STYLE.IVORY) -> Label:
	return STYLE.label(parent, value, font_size, color)
func _button(parent: Node, caption: String, action: Callable) -> Button:
	var b := Button.new(); b.text = caption; b.pressed.connect(action); parent.add_child(b); return b
func _reload() -> void:
	var runtime = get_node("/root/LearnedBattleRuntime")
	var response: Dictionary = runtime.character_catalogs("progression")
	if not response.get("ok", false): _summary.text = "Could not refresh: " + str(response.get("error")); _summary.show(); return
	_summary.hide()
	catalogs = response.result
	_cards.clear(); _character.clear()
	for id in ["adventurer", "venom", "curse", "blade_warden"]:
		if not catalogs.has(id): continue
		_character.add_item(str(catalogs[id].combatants[id].name)); _character.set_item_metadata(_character.item_count - 1, id)
		if id == character_id: _character.select(_character.item_count - 1)
		_cards.merge(catalogs[id].cards, true)
	var trees: Dictionary = runtime.card_trees("card_trees", {}, 0, character_id)
	_trees = trees.get("result", {}).get("trees", {}) if trees.get("ok", false) else {}
	var selected_type := str(_type.get_selected_metadata()) if _type.selected >= 0 else ""
	_type.clear(); _type.add_item("All types"); _type.set_item_metadata(0, "")
	var types: Dictionary = catalogs[character_id].access.types
	var ids: Array = types.keys(); ids.sort()
	for id in ids:
		_type.add_item(str(types[id])); _type.set_item_metadata(_type.item_count - 1, id)
		if id == selected_type: _type.select(_type.item_count - 1)
	_populate()
var _icons: Dictionary = {}
func _icon(id: String) -> Texture2D:
	if not _icons.has(id):
		var path := str(_cards[id].get("presentation", {}).get("illustration_path", ""))
		var tex := STYLE.texture(path)
		if tex == null:
			var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
			tex = cinematic.art(cinematic.card_art_index(id))
		_icons[id] = STYLE.icon(tex, 22, 30)
	return _icons[id]
func _allowed(id: String) -> bool:
	var required := _card_type(id)
	return required == "general" or required == str(catalogs[character_id].access.character_types.get(character_id, "general"))
func _counts(key: String) -> Dictionary:
	var result := {}
	for entry in catalogs[character_id].progression.get(key, []): result[entry.card_id] = int(entry.count)
	return result
func _membership(id: String) -> Dictionary:
	return catalogs[character_id].economy.get("card_tree_membership", {}).get(id, {})
func _card_type(id: String) -> String:
	return str(catalogs[character_id].access.card_types.get(id, "general"))
func _type_name(id: String) -> String:
	return str(catalogs[character_id].access.types.get(id, id))
func _price(id: String) -> int:
	var economy: Dictionary = catalogs[character_id].economy
	return int(economy.get("card_prices", {}).get(id, economy.default_card_price))
func _tree_name(id: String) -> String:
	var membership := _membership(id)
	return str(_trees.get(membership.get("tree_id", ""), {}).get("name", membership.get("tree_id", "")))
func _populate() -> void:
	if catalogs.is_empty(): return
	var deck := _counts("decklist"); var stored := _counts("collection")
	var progress: Dictionary = catalogs[character_id].progression
	var health := 0; var collected := 0
	for amount in deck.values(): health += int(amount)
	for amount in stored.values(): collected += int(amount)
	_summary.text = "%s · %d health / deck cards · %d stored · %d XP available" % [_character.get_item_text(_character.selected), health, collected, int(progress.xp)]
	for child in _stats.get_children(): _stats.remove_child(child); child.queue_free()
	STYLE.stat(_stats, "%d  HEALTH" % health, "cards in deck", STYLE.HEALTH)
	STYLE.stat(_stats, "%d  STORED" % collected, "in collection", STYLE.IVORY)
	STYLE.stat(_stats, "%d XP" % int(progress.xp), "available", STYLE.GOLD)
	STYLE.stat(_stats, "%d XP" % int(progress.get("total_budget", 0)), "total budget", STYLE.MUTED)
	_table.clear(); _rows.clear(); var base := _table.create_item()
	var ids: Array = _cards.keys(); ids.sort_custom(func(a, b): return str(_cards[a].name).naturalnocasecmp_to(str(_cards[b].name)) < 0 if _cards[a].name != _cards[b].name else str(a) < str(b))
	var query := _search.text.strip_edges().to_lower()
	for id in ids:
		var card: Dictionary = _cards[id]; var membership := _membership(id); var variant: bool = membership.get("is_variant", false)
		if not query.is_empty() and not (str(card.name) + " " + str(id) + " " + str(card.presentation.get("rules_text", "")) + " " + _tree_name(id)).to_lower().contains(query): continue
		if _type.selected > 0 and _card_type(id) != str(_type.get_selected_metadata()): continue
		if _scope.selected == 1 and variant or _scope.selected == 2 and not variant: continue
		if _scope.selected == 3 and int(deck.get(id, 0)) + int(stored.get(id, 0)) == 0: continue
		var item := _table.create_item(base); item.set_metadata(0, id); _rows[id] = item
		var source := ("%s · variant" if variant else "%s · base") % _tree_name(id) if not membership.is_empty() else "—"
		var values := [card.name, _type_name(_card_type(id)), source, str(int(card.cost.energy)), str(_price(id)), str(deck.get(id, 0)), str(stored.get(id, 0))]
		for column in values.size():
			item.set_text(column, str(values[column])); item.set_text_alignment(column, HORIZONTAL_ALIGNMENT_CENTER if column >= 3 else HORIZONTAL_ALIGNMENT_LEFT); item.set_tooltip_text(column, "%s\n%s\n%s" % [card.name, id, _tree_name(id)])
		item.set_icon(0, _icon(id)); item.set_icon_max_width(0, 22)
		item.set_custom_color(1, STYLE.type_color(_card_type(id)))
		item.set_custom_color(2, STYLE.AMBER if variant else STYLE.GAIN if not membership.is_empty() else STYLE.DIM)
		item.set_custom_color(3, STYLE.ENERGY.lightened(0.2)); item.set_custom_color(4, STYLE.GOLD)
		for column in [5, 6]: item.set_custom_color(column, STYLE.IVORY if int(values[column]) > 0 else STYLE.DIM)
		if not _allowed(id): item.set_custom_color(0, STYLE.MUTED)
		if id == _selected: item.select(0)
	_count.text = "%d shown / %d published" % [_rows.size(), _cards.size()]
	_count.tooltip_text = "Grey card names are outside this character's type."; _count.mouse_filter = Control.MOUSE_FILTER_PASS
	if not _rows.has(_selected): _selected = str(_rows.keys()[0]) if not _rows.is_empty() else ""
	if _rows.has(_selected): _rows[_selected].select(0)
	_inspect()
func _inspect() -> void:
	for child in _details.get_children(): _details.remove_child(child); child.queue_free()
	if _selected.is_empty():
		STYLE.heading(_details, "Card details")
		_label(_details, "No cards match these filters.", 16, STYLE.MUTED); return
	var card: Dictionary = _cards[_selected]; var membership := _membership(_selected)
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 14); _details.add_child(top)
	var path := str(card.presentation.get("illustration_path", ""))
	var tex := STYLE.texture(path)
	if tex == null:
		var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
		tex = cinematic.art(cinematic.card_art_index(_selected))
	STYLE.thumbnail(top, tex, Vector2(120, 168), STYLE.GOLD if _allowed(_selected) else STYLE.BRONZE)
	var head := VBoxContainer.new(); head.size_flags_horizontal = Control.SIZE_EXPAND_FILL; head.add_theme_constant_override("separation", 6); top.add_child(head)
	_label(head, str(card.name), 24, STYLE.GOLD_BRIGHT)
	_label(head, "Card ID: " + _selected, 13, STYLE.DIM)
	var chips := HFlowContainer.new(); chips.add_theme_constant_override("h_separation", 6); chips.add_theme_constant_override("v_separation", 6); head.add_child(chips)
	STYLE.chip(chips, _type_name(_card_type(_selected)), STYLE.type_color(_card_type(_selected)))
	STYLE.chip(chips, "%d energy" % int(card.cost.energy), STYLE.ENERGY)
	STYLE.chip(chips, "%d XP" % _price(_selected), STYLE.GOLD)
	var rules := STYLE.section(_details, 12)
	STYLE.heading(rules, "Rules")
	_label(rules, str(card.presentation.get("rules_text", "No rules text configured.")), 16)
	var owned := STYLE.section(_details, 12)
	STYLE.heading(owned, "This character")
	_label(owned, "In deck: %d · In collection: %d" % [int(_counts("decklist").get(_selected, 0)), int(_counts("collection").get(_selected, 0))], 16)
	var access := _label(owned, "Character type allows this card." if _allowed(_selected) else "Outside this character's type. Shown here because this is the complete admin catalog.", 14, STYLE.GAIN if _allowed(_selected) else Color("ffd2c8"))
	if not _allowed(_selected): access.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.LOSS, 0.12), Color(STYLE.LOSS, 0.5), 10, 8))
	if not membership.is_empty():
		var tree := STYLE.section(_details, 12)
		STYLE.heading(tree, "Card tree")
		_label(tree, "Tree: " + _tree_name(_selected), 16)
		_label(tree, "Upgrade variant · acquire through its tree; direct purchasing is disabled." if membership.get("is_variant", false) else "Base card for this tree.", 14, STYLE.AMBER if membership.get("is_variant", false) else STYLE.MUTED)
	var note := _label(_details, "This sheet is read-only. Visibility here does not grant ownership, bypass deck requirements, or make a card purchasable.", 14, STYLE.MUTED)
	note.add_theme_stylebox_override("normal", STYLE.box("0c1620", "1f2e3a", 12, 8))
