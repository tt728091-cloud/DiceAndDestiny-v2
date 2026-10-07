extends Control
signal closed
const CANVAS = preload("res://app/screens/character/card_tree_canvas.gd")
const CARD_EDITOR = preload("res://app/screens/character/card_authoring.gd")
const STYLE = preload("res://app/screens/character/character_style.gd")
const DIFF = preload("res://app/screens/character/card_tree_diff.gd")
const FORGE = preload("res://app/screens/character/card_tree_forge.gd")
const COMPARISON = preload("res://app/screens/character/upgrade_comparison.gd")
const ROW := 360.0
const GAP := 330.0
const START_EFFECTS := ["prevent", "draw", "energy", "remove_status", "curse"]
var admin_token := ""
var character_id := "adventurer"
var draft: Dictionary = {}
var catalog: Dictionary = {}
var state: Dictionary = {}
var _tree_pick: OptionButton
var _base_pick: OptionButton
var _canvas: Control
var _details: VBoxContainer
var _message: Label
var _validity: Label
var _confirm: HBoxContainer
var _pending: Callable
var _selected := ""
var _edge := ""
var _id: LineEdit
var _name: LineEdit
var _publish: Button
var _dirty := false
var _revision := 0
var _xp: Label
var _undo: Array = []
var _redo: Array = []
var _undo_button: Button
var _redo_button: Button
## Active quick-edit session: {mode, node, card, name_touched}.
var _forge: Dictionary = {}
var _issues: Dictionary = {}
var _live: Timer
var _id_touched := false
var _start: Dictionary = {}
var _trade_bar: PanelContainer
var _trade_text: Label
var _pending_trade: Dictionary = {}
var _menu: PopupMenu
var _menu_actions: Array = []
var _comparison: PanelContainer

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); mouse_filter = Control.MOUSE_FILTER_STOP
	theme = STYLE.theme()
	var bg := ColorRect.new(); bg.color = Color("070c13"); bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 18)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 10); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 14); body.add_child(header)
	var titles := VBoxContainer.new(); titles.add_theme_constant_override("separation", 0); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; header.add_child(titles)
	STYLE.label(titles, "DICE & DESTINY  /  CARD TREES", 13, STYLE.GOLD)
	STYLE.label(titles, "Card Trees · Admin" if _admin() else "Card Trees", 28)
	_xp = STYLE.label(header, "", 18, STYLE.GOLD); _xp.autowrap_mode = TextServer.AUTOWRAP_OFF; _xp.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_button(header, "Back to characters", func(): _guard(func(): closed.emit(); queue_free()), "back").size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var toolbar := HBoxContainer.new(); toolbar.add_theme_constant_override("separation", 8); body.add_child(toolbar)
	_tree_pick = OptionButton.new(); _tree_pick.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _tree_pick.fit_to_longest_item = false; toolbar.add_child(_tree_pick)
	_tree_pick.item_selected.connect(func(_i): _guard(_load_tree))
	_tree_pick.tooltip_text = "Choose a saved tree to open it."
	_button(toolbar, "Refresh", func(): _guard(func(): _reload(); _load_tree()), "refresh")
	if _admin():
		_button(toolbar, "New tree", func(): _guard(_new_tree), "new").tooltip_text = "Start a tree from a new or existing base card."
		_button(toolbar, "Steady Guard example", func(): _guard(_example), "example").tooltip_text = "Creates an unpublished example tree with its own new base card."
		_undo_button = _button(toolbar, "Undo", _undo_step, "undo"); _undo_button.tooltip_text = "Undo (⌘Z / Ctrl+Z)"
		_redo_button = _button(toolbar, "Redo", _redo_step, "redo"); _redo_button.tooltip_text = "Redo (⇧⌘Z / Ctrl+Y)"
		_button(toolbar, "Auto-arrange", _arrange, "arrange").tooltip_text = "Lay the tree out by XP: upgrades above, cheaper variants below."
	_button(toolbar, "Center", func(): _canvas.home(), "center").tooltip_text = "Fit the whole tree (F)."
	var identity := HBoxContainer.new(); identity.add_theme_constant_override("separation", 8); identity.visible = _admin(); body.add_child(identity)
	var tree_caption := STYLE.label(identity, "Tree", 15, STYLE.MUTED); tree_caption.autowrap_mode = TextServer.AUTOWRAP_OFF; tree_caption.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_name = _line(identity, "Tree name", "tree.name"); _name.size_flags_stretch_ratio = 2.0
	var id_caption := STYLE.label(identity, "ID", 15, STYLE.MUTED); id_caption.autowrap_mode = TextServer.AUTOWRAP_OFF; id_caption.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_id = _line(identity, "tree_id", "tree.id")
	_id.text_changed.connect(func(v): _id_touched = true; _rename_tree(v); _changed())
	_name.text_changed.connect(func(v):
		draft.name = v
		if not _id_touched and _id.editable:
			var id := _unique_tree_id(STYLE.slug(v)) if not v.strip_edges().is_empty() else ""
			_id.text = id; _rename_tree(id)
		_canvas.title = v; _canvas.queue_redraw(); _changed())
	var split := HSplitContainer.new(); split.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(split)
	_canvas = CANVAS.new(); _canvas.custom_minimum_size = Vector2(420, 260); _canvas.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _canvas.size_flags_stretch_ratio = 2.2; _canvas.editable = _admin(); split.add_child(_canvas)
	_canvas.node_selected.connect(func(id):
		if _forge.get("node", "") != id: _forge = {}
		_selected = id; _edge = ""; _render())
	_canvas.edge_selected.connect(func(id): _forge = {}; _edge = id; _selected = ""; _render())
	_canvas.node_drag_started.connect(func(_id): _snapshot())
	_canvas.node_moved.connect(func(_node_id, _at): _changed())
	_canvas.connect_requested.connect(_connect_cards)
	_canvas.context_requested.connect(_context_menu)
	var scroll := ScrollContainer.new(); scroll.custom_minimum_size.x = 400; scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; split.add_child(scroll)
	_details = VBoxContainer.new(); _details.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _details.add_theme_constant_override("separation", 10); scroll.add_child(_details)
	var status := HBoxContainer.new(); status.add_theme_constant_override("separation", 12); body.add_child(status)
	_message = STYLE.label(status, "", 16); _message.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_validity = STYLE.label(status, "", 15, STYLE.MUTED); _validity.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _validity.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	_confirm = HBoxContainer.new(); body.add_child(_confirm); _confirm.hide()
	STYLE.label(_confirm, "Discard unpublished tree edits?", 16, STYLE.GOLD).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(_confirm, "Keep editing", func(): _confirm.hide(); _sync_picker(), "keep")
	_button(_confirm, "Discard edits", func(): _confirm.hide(); _pending.call(), "discard")
	_trade_bar = PanelContainer.new(); _trade_bar.add_theme_stylebox_override("panel", STYLE.box(Color("1a160c"), STYLE.GOLD, 14, 10)); body.add_child(_trade_bar); _trade_bar.hide()
	var trade_row := HBoxContainer.new(); trade_row.add_theme_constant_override("separation", 10); _trade_bar.add_child(trade_row)
	_trade_text = STYLE.label(trade_row, "", 16, STYLE.GOLD_BRIGHT); _trade_text.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	STYLE.accent(_button(trade_row, "Confirm", func(): _trade_bar.hide(); _trade(_pending_trade), "trade.confirm"))
	_button(trade_row, "Cancel", func(): _trade_bar.hide(); _pending_trade = {}, "trade.cancel")
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 8); actions.visible = _admin(); body.add_child(actions)
	_button(actions, "Validate tree", _validate, "validate")
	_publish = STYLE.accent(_button(actions, "Publish tree & cards", _save, "publish")); _publish.disabled = true
	STYLE.label(actions, "Changes affect future battles. Existing battles keep their cards.", 14, STYLE.MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_live = Timer.new(); _live.one_shot = true; _live.wait_time = 0.6; _live.timeout.connect(func(): _check_tree(false)); add_child(_live)
	_menu = PopupMenu.new(); add_child(_menu); _menu.id_pressed.connect(_menu_pressed)
	_comparison = COMPARISON.new(); add_child(_comparison)
	_update_undo_buttons()
	if not _reload(): return
	if _tree_pick.item_count > 0: _tree_pick.select(0); _load_tree()
	elif _admin(): _new_tree()
	else: _message.text = "No card trees yet. Create one from Admin settings → Manage card trees."

func _admin() -> bool: return not admin_token.is_empty()
func _runtime(): return get_node("/root/LearnedBattleRuntime")
func _label(parent: Node, value: String, font_size: int = 17, color: Color = STYLE.IVORY) -> Label:
	return STYLE.label(parent, value, font_size, color)
func _button(parent: Node, text: String, action: Callable, key: String = "") -> Button:
	var b := Button.new(); b.text = text; b.set_meta("tree_action", key); b.pressed.connect(action); parent.add_child(b); return b
func _line(parent: Node, placeholder: String, key: String) -> LineEdit:
	var field := LineEdit.new(); field.placeholder_text = placeholder; field.size_flags_horizontal = Control.SIZE_EXPAND_FILL; field.set_meta("tree_field", key); parent.add_child(field); return field
func _pick(parent: Node, options: Dictionary, key: String) -> OptionButton:
	var field := OptionButton.new(); field.set_meta("tree_field", key); field.fit_to_longest_item = false; parent.add_child(field)
	for id in options: field.add_item(str(options[id])); field.set_item_metadata(field.item_count - 1, id)
	return field
func _guard(action: Callable) -> void:
	if _dirty: _pending = action; _confirm.show()
	else: action.call()
func _sync_picker() -> void:
	for i in _tree_pick.item_count:
		if _tree_pick.get_item_metadata(i) == draft.get("id", ""): _tree_pick.select(i); return
	_tree_pick.select(-1); _tree_pick.text = "Unsaved tree…"

func _changed() -> void:
	_dirty = true; _publish.disabled = true
	_issues = _local_issues()
	_validity.text = "Unpublished changes · checking…" if not draft.get("nodes", []).is_empty() else ""
	_validity.add_theme_color_override("font_color", STYLE.MUTED)
	if _admin(): _live.start()

func _reload() -> bool:
	var response: Dictionary = _runtime().card_authoring()
	if not response.get("ok", false): _message.text = str(response.get("error")); return false
	catalog = response.result
	response = _runtime().card_trees("card_trees", {}, 0, character_id)
	if not response.get("ok", false): _message.text = str(response.get("error")); return false
	state = response.result; _revision = int(state.revision)
	_xp.text = "%s · %d XP available" % [character_id.capitalize(), int(state.progression.xp)]
	_tree_pick.clear()
	for id in state.trees: _tree_pick.add_item(str(state.trees[id].name)); _tree_pick.set_item_metadata(_tree_pick.item_count - 1, id)
	return true

func _reset_session() -> void:
	_undo.clear(); _redo.clear(); _forge = {}; _issues = {}; _pending_trade = {}; _trade_bar.hide(); _update_undo_buttons()
	_canvas.connect_from = ""

func _new_tree() -> void:
	draft = {"id": "", "name": "", "root": "base", "nodes": [], "edges": []}
	_selected = ""; _edge = ""; _tree_pick.select(-1); _tree_pick.text = "Select a saved tree…"
	_id.text = ""; _id.editable = true; _id_touched = false; _name.text = ""; _dirty = false; _start = {}
	_reset_session()
	_render(); _canvas.home(); _publish.disabled = true; _validity.text = ""
	_message.text = "Create a new base card or choose an existing one to start the tree."

func _load_tree() -> void:
	if _tree_pick.selected < 0: return
	draft = state.trees[_tree_pick.get_selected_metadata()].duplicate(true)
	_id.text = draft.id; _id.editable = false; _id_touched = true; _name.text = draft.name
	_selected = str(draft.root); _edge = ""; _dirty = false; _publish.disabled = true; _validity.text = ""
	_reset_session()
	_render(); _canvas.home()
	_message.text = "Select a card or a connection." if not _admin() else "Select a card to build from it, or a connection to edit its deck rules."

func _set_base() -> void:
	if _base_pick == null or _base_pick.selected < 0: _message.text = "Choose a base card first."; return
	if not draft.nodes.is_empty(): _message.text = "Use New tree to select a different base. Edit this base with Quick edit or the full editor."; return
	var card: Dictionary = catalog.templates[_base_pick.get_selected_metadata()].duplicate(true)
	card.economy.upgrades = []; card.economy.buy = maxi(1, int(card.economy.buy)); card.economy.sell = card.economy.buy
	_start_with(card)

func _start_with(card: Dictionary) -> void:
	_snapshot()
	draft.nodes = [{"id": "base", "card": card, "x": 0.0, "y": 0.0}]
	draft.root = "base"
	if str(draft.get("name", "")).strip_edges().is_empty():
		draft.name = str(card.name) + " paths"; _name.text = draft.name
	if not _id_touched or str(draft.get("id", "")).is_empty():
		var id := _unique_tree_id(STYLE.slug(str(draft.name))); _id.text = id; _rename_tree(id)
	_selected = "base"; _changed(); _render(); _canvas.home()
	_message.text = "Base card set. Use Upgrade ↑ or Cheaper ↓ to grow the tree."

func _node(id: String) -> Dictionary:
	for n in draft.get("nodes", []):
		if n.id == id: return n
	return {}
func _edge_by_id(id: String) -> Dictionary:
	for e in draft.get("edges", []):
		if e.id == id: return e
	return {}
func _collection_counts() -> Dictionary:
	var counts := {}
	for entry in state.get("progression", {}).get("collection", []): counts[entry.card_id] = int(entry.count)
	return counts
func _counts() -> Dictionary:
	var counts := {}
	for entry in state.get("progression", {}).get("decklist", []): counts[entry.card_id] = int(entry.count)
	return counts
func _health() -> int:
	var total := 0
	for entry in state.get("progression", {}).get("decklist", []): total += int(entry.count)
	return total
func _predecessor(id: String) -> Dictionary:
	for e in draft.get("edges", []):
		if e.to == id: return _node(str(e.from))
	return {}
## Every card connecting into this one; a converging card has one per route.
func _predecessors(id: String) -> Array:
	var out: Array = []
	for e in draft.get("edges", []):
		if e.to == id:
			var n := _node(str(e.from))
			if not n.is_empty(): out.append(n)
	return out
func _offers() -> Dictionary:
	return state.get("offers", {}).get(str(draft.get("id", "")), {})

# Rendering -------------------------------------------------------------------
func _render() -> void:
	for c in _details.get_children(): _details.remove_child(c); c.queue_free()
	_render_canvas()
	if _admin() and draft.get("nodes", []).is_empty(): _start_panel(); return
	if not _edge.is_empty(): _edge_details(); return
	var node := _node(_selected)
	if node.is_empty(): _label(_details, "Select a card to view it, or a connection to view its deck rules.", 16, STYLE.MUTED); return
	if not _forge.is_empty() and _admin(): _forge_panel(); return
	_card_preview(_details, node)
	if _admin(): _admin_node_details(node)
	else: _player_node_details(node)

func _render_canvas() -> void:
	_canvas.card_art = {}
	for id in catalog.get("templates", {}): _canvas.card_art[id] = catalog.templates[id].presentation.get("illustration_path", "")
	for n in draft.get("nodes", []): _canvas.card_art[n.card.id] = n.card.presentation.get("illustration_path", "")
	_canvas.collection_counts = _collection_counts()
	_canvas.title = str(draft.get("name", ""))
	_canvas.selected_edge = _edge
	_canvas.summaries = {}; _canvas.tooltips = {}; _canvas.edge_tooltips = {}
	for n in draft.get("nodes", []):
		var before := _predecessors(str(n.id))
		if before.size() == 1: _canvas.summaries[n.id] = DIFF.summary(before[0].card, n.card, catalog, 1)
		elif before.size() > 1: _canvas.summaries[n.id] = "Joins %d paths" % before.size()
		_canvas.tooltips[n.id] = _node_tooltip(n, before)
	for e in draft.get("edges", []): _canvas.edge_tooltips[e.id] = _edge_tooltip(e)
	if _admin():
		_canvas.node_states = {}; _canvas.edge_states = {}
	else: _player_states()
	_canvas.issues = _issues
	_canvas.show_tree(draft, _counts(), _selected)

func _player_states() -> void:
	var counts := _counts(); var stored := _collection_counts()
	var owned := {}
	var nodes := {}; var edges := {}
	for n in draft.get("nodes", []):
		if int(counts.get(n.card.id, 0)) + int(stored.get(n.card.id, 0)) > 0: owned[n.id] = true
		nodes[n.id] = "owned" if owned.has(n.id) else "unowned"
	var root := _node(str(draft.get("root", "")))
	if not root.is_empty() and not owned.has(root.id) and int(state.progression.xp) >= int(root.card.economy.buy): nodes[root.id] = "available"
	var offers := _offers()
	for key in offers:
		var o: Dictionary = offers[key]
		if not owned.has(o.from) or owned.has(o.to): continue
		if o.available: nodes[o.to] = "available"
		elif nodes[o.to] != "available": nodes[o.to] = "locked"
	for e in draft.get("edges", []):
		if owned.has(e.from) and owned.has(e.to): edges[e.id] = "owned"
		elif owned.has(e.from): edges[e.id] = "available" if offers.get(e.id, {}).get("available", false) else "locked"
		else: edges[e.id] = "unowned"
	_canvas.node_states = nodes; _canvas.edge_states = edges

func _node_tooltip(n: Dictionary, predecessors: Array) -> String:
	var card: Dictionary = n.card
	var text := "%s\n%d energy · %d XP%s\n\n%s" % [card.name, int(card.cost.energy), int(card.economy.buy), " · BASE" if n.id == draft.root else "", str(card.presentation.get("rules_text", ""))]
	for before in predecessors:
		var entries := DIFF.with_xp(before.card, card, catalog)
		if not entries.is_empty(): text += "\n\nChanges from %s:\n%s" % [before.card.name, DIFF.plain(entries)]
	if not _admin():
		text += "\n\nIn deck ×%d · Stored ×%d" % [int(_counts().get(card.id, 0)), int(_collection_counts().get(card.id, 0))]
	if _issues.has(n.id): text += "\n\n⚠ " + str(_issues[n.id])
	return text

func _edge_tooltip(e: Dictionary) -> String:
	var a := _node(str(e.from)); var b := _node(str(e.to))
	if a.is_empty() or b.is_empty(): return ""
	var cost := int(b.card.economy.buy) - int(a.card.economy.buy)
	var text := "%s → %s\n%s" % [a.card.name, b.card.name, ("Costs %d XP" % cost) if cost > 0 else ("Refunds %d XP" % -cost) if cost < 0 else "No XP change"]
	text += "\n" + ("Can return to %s for the same difference." % a.card.name if e.reversible else "One-way connection.")
	var cards := _all_cards()
	if e.requirements.is_empty(): text += "\nNo deck rules."
	else:
		text += "\nDeck rules when equipping:"
		for rule in e.requirements:
			var name := str(cards.get(rule.card_id, rule.card_id))
			text += "\n" + (("✕ No %s in deck" % name) if rule.get("maximum", -1) == 0 else ("✓ Requires %d × %s" % [int(rule.minimum), name])) + (" (or variants)" if rule.get("include_descendants", false) else "")
	return text

func _card_preview(parent: Node, node: Dictionary) -> void:
	var card: Dictionary = node.card
	var box := STYLE.section(parent)
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 14); box.add_child(top)
	var frame := PanelContainer.new(); frame.add_theme_stylebox_override("panel", STYLE.box(Color("0b131a"), STYLE.GOLD if node.id == draft.root else STYLE.BRONZE, 4, 8, 2)); top.add_child(frame)
	var art := TextureRect.new(); art.texture = STYLE.texture(str(card.presentation.get("illustration_path", "")))
	art.custom_minimum_size = Vector2(118, 166); art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; frame.add_child(art)
	var text := VBoxContainer.new(); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; text.add_theme_constant_override("separation", 4); top.add_child(text)
	STYLE.heading(text, "Base card" if node.id == draft.root else "Tree variant")
	_label(text, str(card.name), 23, STYLE.GOLD_BRIGHT)
	var predecessors := _predecessors(str(node.id))
	var deltas: Array = predecessors.map(func(before): return "%+d XP from %s" % [int(card.economy.buy) - int(before.card.economy.buy), before.card.name])
	var delta := "" if deltas.is_empty() else " · " + ", ".join(deltas)
	_label(text, "%d XP per copy · %d energy%s\n%d in deck · %d in collection" % [int(card.economy.buy), int(card.cost.energy), delta, int(_counts().get(card.id, 0)), int(_collection_counts().get(card.id, 0))], 15, STYLE.MUTED)
	_label(box, str(card.presentation.get("rules_text", "Edit this card to choose its effects.")), 17)
	for before in predecessors:
		var entries := DIFF.with_xp(before.card, card, catalog)
		if not entries.is_empty():
			STYLE.heading(box, "Changes from " + str(before.card.name))
			var diff := RichTextLabel.new(); diff.bbcode_enabled = true; diff.fit_content = true; diff.scroll_active = false; diff.mouse_filter = Control.MOUSE_FILTER_IGNORE
			diff.add_theme_font_size_override("normal_font_size", 16); diff.text = DIFF.bbcode(entries); diff.set_meta("tree_field", "preview.changes." + str(before.id)); box.add_child(diff)
	if _issues.has(node.id): _label(box, "⚠ " + str(_issues[node.id]), 15, STYLE.LOSS)

func _admin_node_details(node: Dictionary) -> void:
	var card: Dictionary = node.card
	var build := STYLE.section(_details)
	STYLE.heading(build, "Grow the tree")
	var grow := HBoxContainer.new(); grow.add_theme_constant_override("separation", 8); build.add_child(grow)
	var up := STYLE.accent(_button(grow, "Upgrade ↑", func(): _open_forge("upgrade"), "add_up")); up.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	up.tooltip_text = "Copy this card one step up and adjust what improves (U)."
	var down := STYLE.accent(_button(grow, "Cheaper ↓", func(): _open_forge("downgrade"), "add_down"), STYLE.AMBER); down.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	down.tooltip_text = "Copy this card one step down with a tradeoff and a lower XP value (D)."
	var edit := HBoxContainer.new(); edit.add_theme_constant_override("separation", 8); build.add_child(edit)
	_button(edit, "Quick edit", func(): _open_forge("edit"), "quick_edit").size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(edit, "Full card editor", func(): _edit_card(node), "edit_card").size_flags_horizontal = Control.SIZE_EXPAND_FILL
	if node.id == draft.root and catalog.templates.has(card.id) and not state.get("trees", {}).has(str(draft.get("id", ""))):
		_label(build, "This base is the existing catalog card “%s”. Editing it changes that card everywhere once published." % catalog.templates[card.id].name, 14, STYLE.AMBER)
	elif node.id == draft.root:
		_label(build, "Editing the base updates this card for future battles. Variants keep their own settings.", 14, STYLE.MUTED)
	else:
		_label(build, "Each card is a complete definition. Editing one card never changes the cards above or below it.", 14, STYLE.MUTED)
	var reuse := STYLE.section(_details)
	STYLE.heading(reuse, "Use existing card as template")
	var templates := {}
	var template_ids: Array = catalog.templates.keys()
	template_ids.sort_custom(func(a, b): return str(catalog.templates[a].name).naturalnocasecmp_to(str(catalog.templates[b].name)) < 0 if catalog.templates[a].name != catalog.templates[b].name else str(a) < str(b))
	for id in template_ids: templates[id] = "%s · %s" % [catalog.templates[id].name, id]
	var template := _pick(reuse, templates, "node.template")
	template.select(-1); template.text = "Choose an existing card…"
	var apply_template := _button(reuse, "Apply card template", func():
		if template.selected >= 0: _apply_card_template(str(node.id), str(template.get_selected_metadata())), "node.apply_template")
	apply_template.disabled = true
	template.item_selected.connect(func(_index): apply_template.disabled = template.selected < 0)
	_label(reuse, "Copies the card's effects, artwork, energy and XP. A unique name is assigned; this card keeps its ID and connections.", 14, STYLE.MUTED)
	var links := STYLE.section(_details)
	STYLE.heading(links, "Connections")
	for e in draft.edges:
		if e.to != node.id and e.from != node.id: continue
		var other := _node(str(e.from if e.to == node.id else e.to))
		var edge_id := str(e.id)
		var text := ("← from %s" if e.to == node.id else "→ to %s") % other.card.name
		if not e.requirements.is_empty(): text += " · %d rule%s" % [e.requirements.size(), "" if e.requirements.size() == 1 else "s"]
		var b := _button(links, text, func(): _edge = edge_id; _selected = ""; _render(), "goto_edge." + edge_id)
		b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	if node.id != draft.root:
		var options := {}
		var blocked := _descendants(str(node.id))
		for n in draft.nodes:
			if n.id != node.id and not blocked.has(n.id) and not draft.edges.any(func(e): return e.from == n.id and e.to == node.id): options[n.id] = n.card.name
		if not options.is_empty():
			var from := _pick(links, options, "connect.from")
			_button(links, "Connect chosen card → this card", func():
				if from.selected >= 0: _connect_cards(str(from.get_selected_metadata()), str(node.id)), "connect")
		_label(links, "Tip: drag the ⊕ knob on any card onto another card, or right-click a card → Connect from here.", 14, STYLE.MUTED)
		var remove := STYLE.accent(_button(_details, "Remove this card & its connections", func(): _remove_node(str(node.id)), "remove_node"), STYLE.LOSS)
		remove.tooltip_text = "Delete (⌫). Undo restores it."

func _player_node_details(node: Dictionary) -> void:
	var card: Dictionary = node.card
	var copies := STYLE.section(_details)
	STYLE.heading(copies, "Your copies")
	_label(copies, "Upgrades go to your collection, outside the deck. They spend or refund the XP difference. Deck rules apply only when equipping. Collected cards keep their XP value but give no health.", 14, STYLE.MUTED)
	var equipment: Dictionary = state.get("equipment", {}).get(card.id, {})
	var equip := _button(copies, "Add collected copy to deck · no extra XP", func(): _collection_trade("equip_collection_card", card.id, 0), "equip." + str(card.id))
	equip.disabled = not equipment.get("available", false)
	if equip.disabled: _label(copies, str(equipment.get("reason", "No collected copy")), 14, STYLE.MUTED)
	var unequip := _button(copies, "Move deck copy to collection", func(): _collection_trade("unequip_collection_card", card.id, 0), "unequip." + str(card.id)); unequip.disabled = int(_counts().get(card.id, 0)) < 1
	var sell := _button(copies, "Sell collected copy · refund %d XP" % int(card.economy.buy), func(): _collection_trade("sell_collection_card", card.id, int(card.economy.buy)), "sell_collection." + str(card.id)); sell.disabled = int(_collection_counts().get(card.id, 0)) < 1
	if node.id == draft.root:
		var buy := _button(copies, "Buy base for collection · %d XP" % int(card.economy.buy), func(): _collection_trade("buy_collection_card", card.id, int(card.economy.buy)), "buy_collection." + str(card.id)); buy.disabled = int(state.progression.xp) < int(card.economy.buy)
	var offers := _offers()
	var incoming: Array = []; var outgoing: Array = []
	for key in offers:
		if offers[key].to == node.id: incoming.append(key)
		elif offers[key].from == node.id: outgoing.append(key)
	if not incoming.is_empty():
		var get_box := STYLE.section(_details)
		STYLE.heading(get_box, "Get this card")
		for key in incoming: _offer_row(get_box, key, offers[key], true)
	if not outgoing.is_empty():
		var paths := STYLE.section(_details)
		STYLE.heading(paths, "Paths from here")
		for key in outgoing: _offer_row(paths, key, offers[key], false)

func _offer_row(parent: Node, key: String, offer: Dictionary, incoming: bool) -> void:
	var source := _node(str(offer.from)); var target := _node(str(offer.to))
	if source.is_empty() or target.is_empty(): return
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 10); parent.add_child(row)
	var art := TextureRect.new(); art.texture = STYLE.texture(str((source if incoming else target).card.presentation.get("illustration_path", "")))
	art.custom_minimum_size = Vector2(44, 60); art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; row.add_child(art)
	var text := VBoxContainer.new(); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; text.add_theme_constant_override("separation", 2); row.add_child(text)
	_label(text, ("From " + str(source.card.name)) if incoming else str(target.card.name), 16)
	var summary := DIFF.summary(source.card, target.card, catalog, 2)
	if not summary.is_empty(): _label(text, summary, 14, STYLE.GAIN.lerp(STYLE.IVORY, 0.3))
	var cost: int = int(target.card.economy.buy) - int(source.card.economy.buy)
	var verb := "Upgrade" if cost > 0 else "Downgrade" if cost < 0 else "Switch"
	var price := ("%d XP" % cost) if cost > 0 else ("refund %d XP" % -cost) if cost < 0 else "free"
	var button := _button(text, "%s · %s" % [verb, price], func(): _request_trade(offer), "trade." + key)
	if cost > 0: STYLE.accent(button)
	elif cost < 0: STYLE.accent(button, STYLE.AMBER)
	button.disabled = not offer.available
	button.mouse_entered.connect(func(): _comparison.present(button, str(source.card.name), str(target.card.name), str(source.card.presentation.get("rules_text", "")), str(target.card.presentation.get("rules_text", "")), absi(cost), cost < 0))
	if button.disabled: _label(text, str(offer.reason), 13, STYLE.MUTED)

# Starting a tree -------------------------------------------------------------
func _start_panel() -> void:
	STYLE.heading(_details, "Start a new tree")
	_label(_details, "A tree grows from one base card. Upgrades sit above it and cheaper variants below.", 15, STYLE.MUTED)
	if _start.is_empty():
		_start = {"name": "", "source": "effect:prevent", "energy": 1, "amount": 1, "xp": 10, "art": ""}
	var create := STYLE.section(_details)
	STYLE.heading(create, "Create a new base card")
	var name := _line(create, "Card name, e.g. Steady Guard", "start.name"); name.text = str(_start.name)
	name.text_changed.connect(func(v): _start.name = v)
	var sources := {}
	for effect in START_EFFECTS:
		if catalog.get("effects", {}).has(effect): sources["effect:" + effect] = "New · " + str(catalog.effects[effect].label)
	var ids: Array = catalog.get("templates", {}).keys()
	ids.sort_custom(func(a, b): return str(catalog.templates[a].name).naturalnocasecmp_to(str(catalog.templates[b].name)) < 0)
	for id in ids: sources["copy:" + str(id)] = "Copy of " + str(catalog.templates[id].name)
	_label(create, "Start from", 14, STYLE.MUTED)
	var source := _pick(create, sources, "start.source")
	for i in source.item_count:
		if source.get_item_metadata(i) == _start.source: source.select(i)
	source.item_selected.connect(func(i): _start.source = str(source.get_item_metadata(i)); _start.art = ""; _render())
	var from_effect := str(_start.source).begins_with("effect:")
	var numbers := HBoxContainer.new(); numbers.add_theme_constant_override("separation", 8); create.add_child(numbers)
	if from_effect:
		_spin(numbers, "Energy", int(_start.energy), 0, 10, func(v): _start.energy = v, "start.energy")
		_spin(numbers, "Amount", int(_start.amount), 1, 20, func(v): _start.amount = v, "start.amount")
	_spin(numbers, "XP value", int(_start.xp), 1, 1000, func(v): _start.xp = v, "start.xp")
	var arts := STYLE.card_art()
	if str(_start.art).is_empty(): _start.art = _default_art(str(_start.source), arts)
	_label(create, "Artwork", 14, STYLE.MUTED)
	var art_pick := OptionButton.new(); art_pick.set_meta("tree_field", "start.art"); art_pick.fit_to_longest_item = false; create.add_child(art_pick)
	for path in arts:
		var tex := STYLE.texture(path)
		art_pick.add_icon_item(_thumbnail(tex), path.get_file().get_basename().replace("_", " ").capitalize()); art_pick.set_item_metadata(art_pick.item_count - 1, path)
		if path == _start.art: art_pick.select(art_pick.item_count - 1)
	art_pick.item_selected.connect(func(i): _start.art = str(art_pick.get_item_metadata(i)))
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); create.add_child(row)
	STYLE.accent(_button(row, "Create base card", _create_base, "start.create")).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(row, "Design in full editor…", func():
		var card := _start_card()
		if card.is_empty(): return
		_edit_card_dict(card, func(result): _start_with(result)), "start.full")
	_label(create, "The new card exists only in this tree until you publish it. Existing cards are not changed.", 14, STYLE.MUTED)
	var existing := STYLE.section(_details)
	STYLE.heading(existing, "Or use an existing card")
	_label(existing, "The base is that actual card: editing it changes it everywhere once published. Cards in your deck or collection are listed first.", 14, STYLE.MUTED)
	_base_pick = OptionButton.new(); _base_pick.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _base_pick.fit_to_longest_item = false; existing.add_child(_base_pick)
	var owned := _counts(); var stored := _collection_counts()
	var candidates: Array = catalog.get("templates", {}).keys().filter(func(id): return not _tree_owned_card(str(id)))
	candidates.sort_custom(func(a, b):
		var oa := int(owned.get(a, 0)) + int(stored.get(a, 0)) > 0; var ob := int(owned.get(b, 0)) + int(stored.get(b, 0)) > 0
		if oa != ob: return oa
		return str(catalog.templates[a].name).naturalnocasecmp_to(str(catalog.templates[b].name)) < 0)
	for id in candidates:
		var mine := int(owned.get(id, 0)) + int(stored.get(id, 0)) > 0
		_base_pick.add_icon_item(_thumbnail(STYLE.texture(str(catalog.templates[id].presentation.get("illustration_path", "")))), str(catalog.templates[id].name) + ("  · owned" if mine else ""))
		_base_pick.set_item_metadata(_base_pick.item_count - 1, id)
	_base_pick.select(-1); _base_pick.text = "Choose a card…"
	_button(existing, "Set base card", _set_base, "base")

func _spin(parent: Node, caption: String, value: int, low: int, high: int, apply: Callable, key: String) -> void:
	var box := VBoxContainer.new(); box.size_flags_horizontal = Control.SIZE_EXPAND_FILL; parent.add_child(box)
	_label(box, caption, 13, STYLE.MUTED)
	var spin := SpinBox.new(); spin.min_value = low; spin.max_value = high; spin.value = value; spin.set_meta("tree_field", key); box.add_child(spin)
	spin.value_changed.connect(func(v): apply.call(int(v)))

func _thumbnail(texture: Texture2D) -> Texture2D:
	if texture == null: return null
	var image := texture.get_image()
	if image == null: return null
	image = image.duplicate() as Image
	if image.is_compressed(): image.decompress()
	image.resize(24, 34, Image.INTERPOLATE_BILINEAR)
	return ImageTexture.create_from_image(image)

func _default_art(source: String, arts: Array) -> String:
	if source.begins_with("copy:"): return str(catalog.templates.get(source.trim_prefix("copy:"), {}).get("presentation", {}).get("illustration_path", ""))
	var effect := source.trim_prefix("effect:")
	for id in catalog.get("templates", {}):
		var c: Dictionary = catalog.templates[id]
		if c.get("program") is Dictionary and not c.program.get("steps", []).is_empty() and c.program.steps[0].effect == effect:
			var path := str(c.presentation.get("illustration_path", ""))
			if not path.is_empty(): return path
	return arts[0] if not arts.is_empty() else ""

func _start_card() -> Dictionary:
	var name := str(_start.name).strip_edges()
	if name.is_empty(): _message.text = "Enter a name for the new base card."; return {}
	if _used_names("").has(name.to_lower()): _message.text = "Another card is already named %s. Choose a different name." % name; return {}
	var id := _unique_card_id(STYLE.slug(name))
	var card: Dictionary
	var source := str(_start.source)
	if source.begins_with("copy:"):
		card = catalog.templates[source.trim_prefix("copy:")].duplicate(true)
	else:
		var effect := source.trim_prefix("effect:")
		var step := FORGE.new_step(catalog, effect)
		if catalog.effects[effect].get("parameters", {}).has("amount"): step.params.amount = int(_start.amount)
		if effect == "remove_status": step.params.stacks = int(_start.amount)
		card = {"schema_version": 1, "type": "action", "access_type": "general", "cost": {"energy": int(_start.energy)},
			"play": {"source_zones": ["hand"], "destination": "discard"},
			"targeting": {"selector": "card_program", "minimum": 1, "maximum": 1},
			"presentation": {"rules_text": "", "illustration_path": ""},
			"program": {"version": 1, "windows": catalog.effects[effect].windows.duplicate(), "roll_requirement": "any", "uses_per_round": 0, "uses_per_battle": 0, "steps": [step]}}
	card.id = id; card.name = name
	card.economy = {"buy": int(_start.xp), "sell": int(_start.xp), "copy_limit": int(card.get("economy", {}).get("copy_limit", 20)), "upgrades": []}
	if not str(_start.art).is_empty(): card.presentation.illustration_path = _start.art
	var response: Dictionary = _runtime().card_authoring("validate_card", card)
	if not response.get("ok", false): _message.text = str(response.get("error")); return {}
	card = response.result.card
	card.economy.upgrades = []; card.economy.sell = card.economy.buy
	return card

func _create_base() -> void:
	var card := _start_card()
	if not card.is_empty(): _start_with(card)

# Forge -----------------------------------------------------------------------
func _open_forge(mode: String) -> void:
	var node := _node(_selected)
	if node.is_empty(): return
	if str(draft.get("id", "")).strip_edges().is_empty():
		if str(draft.get("name", "")).strip_edges().is_empty(): draft.name = str(_node(str(draft.root)).card.name) + " paths"; _name.text = draft.name
		var id := _unique_tree_id(STYLE.slug(str(draft.name))); _id.text = id; _rename_tree(id)
	_forge = {"mode": mode, "node": str(node.id), "card": node.card.duplicate(true), "name_touched": mode == "edit", "fresh": true}
	_render()

func _forge_panel() -> void:
	var node := _node(str(_forge.node))
	if node.is_empty(): _forge = {}; _render(); return
	var forge = FORGE.new(); forge.catalog = catalog; forge.runtime = _runtime(); forge.session = _forge
	var mode := str(_forge.mode)
	forge.used_names = _used_names(str(node.card.id) if mode == "edit" else "")
	var compare: Dictionary = node if mode != "edit" else _predecessor(str(node.id))
	if compare.is_empty(): compare = {"card": node.card}
	forge.compare_card = compare.card; forge.compare_name = str(compare.card.name)
	_details.add_child(forge)
	var price := int(node.card.economy.buy) + (3 if mode == "upgrade" else -2)
	forge.start(mode, price, _forge.get("fresh", false)); _forge.fresh = false
	forge.committed.connect(_commit_forge)
	forge.cancelled.connect(func(): _forge = {}; _render(); _message.text = "No changes made.")
	forge.full_editor_requested.connect(func(card): _edit_card_dict(card, func(result):
		if _forge.is_empty(): return
		_forge.card = result; _render()))

func _commit_forge(card: Dictionary) -> void:
	var node := _node(str(_forge.node))
	if node.is_empty(): return
	_snapshot()
	card.economy.upgrades = []; card.economy.sell = card.economy.buy
	if _forge.mode == "edit":
		card.id = node.card.id; node.card = card
		_forge = {}; _changed(); _render(); _message.text = "Updated %s." % card.name; return
	var i := 1
	while draft.nodes.any(func(n): return n.id == "node_" + str(i)): i += 1
	var id := "node_" + str(i)
	card.id = str(draft.id) + "_variant_" + id
	var at := _place_new(node, int(card.economy.buy) >= int(node.card.economy.buy))
	draft.nodes.append({"id": id, "card": card, "x": at.x, "y": at.y})
	_connect(str(node.id), id)
	_forge = {}; _selected = id; _changed(); _render(); _canvas.home()
	_message.text = "Created %s. Keep building from it, or right-click any card for more." % card.name

## New cards sit one row above (upgrades) or below (cheaper) their parent.
## Exclusive siblings in that row are re-centred under the parent when space allows.
func _place_new(parent: Dictionary, up: bool) -> Vector2:
	var row_y := float(parent.y) + (-ROW if up else ROW)
	var siblings: Array = []
	for e in draft.edges:
		if e.from != parent.id: continue
		var child := _node(str(e.to))
		if child.is_empty() or absf(float(child.y) - row_y) > 60: continue
		if draft.edges.filter(func(x): return x.to == child.id).size() == 1: siblings.append(child)
	siblings.sort_custom(func(a, b): return float(a.x) < float(b.x))
	var k := siblings.size() + 1
	var xs: Array = []
	for i in k: xs.append(snappedf(float(parent.x) + (i - (k - 1) * 0.5) * GAP, 20.0))
	var moving := siblings.map(func(s): return s.id)
	var clear := true
	for n in draft.nodes:
		if n.id in moving or absf(float(n.y) - row_y) > 150: continue
		for x in xs:
			if absf(float(n.x) - float(x)) < GAP - 20: clear = false
	if clear:
		for i in siblings.size(): siblings[i].x = xs[i]; siblings[i].y = row_y
		return Vector2(xs[k - 1], row_y)
	for step in 16:
		for sign in ([1.0] if step == 0 else [1.0, -1.0]):
			var x := snappedf(float(parent.x) + sign * step * GAP, 20.0)
			if not draft.nodes.any(func(n): return absf(float(n.y) - row_y) < 150 and absf(float(n.x) - x) < GAP - 20): return Vector2(x, row_y)
	return Vector2(float(parent.x) + 16 * GAP, row_y)

# Graph editing ---------------------------------------------------------------
func _connect(from: String, to: String) -> String:
	for e in draft.edges:
		if e.from == from and e.to == to: return str(e.id)
	var i := 1
	while draft.edges.any(func(e): return e.id == "edge_" + str(i)): i += 1
	draft.edges.append({"id": "edge_" + str(i), "from": from, "to": to, "reversible": true, "requirements": []})
	return "edge_" + str(i)

func _descendants(id: String) -> Dictionary:
	var seen := {id: true}; var stack: Array = [id]
	while not stack.is_empty():
		var current: String = stack.pop_back()
		for e in draft.edges:
			if e.from == current and not seen.has(e.to): seen[e.to] = true; stack.append(str(e.to))
	return seen

func _connect_cards(from: String, to: String) -> void:
	if not _admin() or from == to: return
	var a := _node(from); var b := _node(to)
	if a.is_empty() or b.is_empty(): return
	if to == str(draft.root): _message.text = "Nothing can lead into the base card. Connect from the base instead."; return
	if draft.edges.any(func(e): return e.from == from and e.to == to): _message.text = "Those cards are already connected."; return
	if _descendants(to).has(from): _message.text = "That connection would form a loop: %s already leads to %s." % [b.card.name, a.card.name]; return
	_snapshot()
	var edge := _connect(from, to)
	_changed(); _render()
	_message.text = "Connected %s → %s. Select the connection (%s) to add deck rules." % [a.card.name, b.card.name, edge]

func _remove_node(id: String) -> void:
	if id == str(draft.root) or _node(id).is_empty(): return
	_snapshot()
	var name := str(_node(id).card.name)
	draft.nodes = draft.nodes.filter(func(n): return n.id != id); draft.edges = draft.edges.filter(func(e): return e.from != id and e.to != id)
	_selected = str(draft.root); _forge = {}; _changed(); _render()
	_message.text = "Removed %s. Undo (⌘Z) restores it." % name

func _remove_edge(id: String) -> void:
	var edge := _edge_by_id(id)
	if edge.is_empty(): return
	_snapshot(); draft.edges.erase(edge); _edge = ""; _changed(); _render()

func _rename_tree(id: String) -> void:
	var previous := str(draft.get("id", ""))
	draft.id = id
	if previous == id: return
	# Variant IDs follow tree_id_variant_node_id; keep rules pointing at the same cards.
	var renamed := {}
	for n in draft.get("nodes", []):
		if n.id == draft.root: continue
		var next := id + "_variant_" + str(n.id)
		renamed[n.card.id] = next; n.card.id = next
	for e in draft.get("edges", []):
		for rule in e.requirements:
			if renamed.has(rule.card_id): rule.card_id = renamed[rule.card_id]
	if is_instance_valid(_canvas) and not draft.get("nodes", []).is_empty(): _render_canvas()

func _apply_card_template(node_id: String, template_id: String) -> void:
	if not _admin() or not catalog.get("templates", {}).has(template_id): return
	var node := _node(node_id)
	if node.is_empty(): return
	_snapshot()
	var card: Dictionary = catalog.templates[template_id].duplicate(true)
	card.id = node.card.id
	card.name = _template_name(str(card.name), str(card.id))
	# Tree variants retain their stable identity and use graph connections, not
	# a copied card's legacy upgrade links. The source definition stays independent.
	card.economy.buy = maxi(1, int(card.economy.buy))
	card.economy.sell = card.economy.buy; card.economy.upgrades = []
	node.card = card
	_changed(); _render()
	_message.text = "Applied %s to this card. Review it, then validate and publish the tree. Tree cards use equal buy/sell XP (minimum 1 XP)." % str(card.name)

func _template_name(source_name: String, card_id: String) -> String:
	var used := _used_names(card_id)
	var candidate := source_name
	var suffix := str(draft.name).strip_edges()
	if suffix.is_empty(): suffix = "Tree variant"
	var index := 1
	while used.has(candidate.strip_edges().to_lower()):
		candidate = "%s · %s%s" % [source_name, suffix, "" if index == 1 else " %d" % index]
		index += 1
	return candidate

func _edit_card(node: Dictionary) -> void:
	_edit_card_dict(node.card, func(card):
		if card.id != node.card.id: _message.text = "A tree card keeps its ID. Reopen its editor and keep the original ID."; return
		_snapshot(); node.card = card; _changed(); _render())

func _edit_card_dict(source: Dictionary, accepted: Callable) -> void:
	var editor = CARD_EDITOR.new(); editor.embedded_draft = source.duplicate(true); add_child(editor)
	editor.draft_accepted.connect(func(card):
		card.economy.upgrades = []; card.economy.sell = card.economy.buy
		accepted.call(card))

func _all_cards() -> Dictionary:
	var cards: Dictionary = catalog.get("card_names", {}).duplicate()
	for n in draft.get("nodes", []): cards[n.card.id] = n.card.name
	return cards

## Lowercase names already taken by other cards (catalog plus this draft).
func _used_names(except_card: String) -> Dictionary:
	var used := {}
	var cards := _all_cards()
	for id in cards:
		if str(id) != except_card: used[str(cards[id]).strip_edges().to_lower()] = true
	return used

func _tree_owned_card(card_id: String) -> bool:
	for id in state.get("trees", {}):
		if id == draft.get("id", ""): continue
		for n in state.trees[id].nodes:
			if n.card.id == card_id: return true
	return false

func _unique_card_id(base: String) -> String:
	var cards := _all_cards()
	var candidate := base; var n := 2
	while cards.has(candidate) or candidate.contains("_variant_"): candidate = "%s_%d" % [base, n]; n += 1
	return candidate

func _unique_tree_id(base: String) -> String:
	var candidate := base; var n := 2
	while state.get("trees", {}).has(candidate): candidate = "%s_%d" % [base, n]; n += 1
	return candidate

func _unique_name(base: String) -> String:
	var used := _used_names("")
	var candidate := base; var n := 2
	while used.has(candidate.to_lower()): candidate = "%s %d" % [base, n]; n += 1
	return candidate

## Layered layout by XP direction: each upgrade one row up, each cheaper variant
## one row down; converging cards centre under their predecessors.
func _arrange() -> void:
	if not _admin() or draft.get("nodes", []).is_empty(): return
	_snapshot()
	var buy := {}
	for n in draft.nodes: buy[n.id] = int(n.card.economy.buy)
	var indegree := {}
	for n in draft.nodes: indegree[n.id] = 0
	for e in draft.edges: indegree[e.to] = int(indegree.get(e.to, 0)) + 1
	var queue: Array = [str(draft.root)]
	for n in draft.nodes:
		if n.id != draft.root and indegree[n.id] == 0: queue.append(str(n.id))
	var order: Array = []
	while not queue.is_empty():
		var id: String = queue.pop_front(); order.append(id)
		for e in draft.edges:
			if e.from != id: continue
			indegree[e.to] -= 1
			if indegree[e.to] == 0: queue.append(str(e.to))
	var rank := {str(draft.root): 0}
	for id in order:
		if not rank.has(id): rank[id] = 0
		for e in draft.edges:
			if e.from != id: continue
			var r: int = rank[id] + (1 if buy[e.to] >= buy[id] else -1)
			if not rank.has(e.to) or absi(r) > absi(rank[e.to]): rank[e.to] = r
	var rows := {}
	for id in order:
		if not rows.has(rank[id]): rows[rank[id]] = []
		rows[rank[id]].append(id)
	var x := {str(draft.root): 0.0}
	var zero: Array = rows.get(0, []).filter(func(id): return id != draft.root)
	for i in zero.size(): x[zero[i]] = (i + 1) * GAP
	var ranks: Array = rows.keys().filter(func(r): return r > 0); ranks.sort()
	var lower: Array = rows.keys().filter(func(r): return r < 0); lower.sort(); lower.reverse()
	for r in ranks + lower:
		var ids: Array = rows[r]
		var desired := {}
		for id in ids:
			var total := 0.0; var count := 0
			for e in draft.edges:
				if e.to == id and x.has(e.from): total += float(x[e.from]); count += 1
			desired[id] = total / count if count > 0 else 0.0
		ids.sort_custom(func(a, b): return desired[a] < desired[b])
		var placed: Array = []
		for i in ids.size():
			var px: float = desired[ids[i]]
			if i > 0: px = maxf(px, placed[i - 1] + GAP)
			placed.append(px)
		var shift := 0.0
		for i in ids.size(): shift += desired[ids[i]] - placed[i]
		shift /= ids.size()
		for i in ids.size(): x[ids[i]] = placed[i] + shift
	for n in draft.nodes:
		n.x = snappedf(float(x.get(n.id, 0.0)), 20.0); n.y = -float(rank.get(n.id, 0)) * ROW
	_changed(); _render(); _canvas.home()
	_message.text = "Arranged by XP: upgrades above, cheaper variants below."

# Undo ------------------------------------------------------------------------
func _snapshot() -> void:
	if not _admin(): return
	_undo.append({"draft": draft.duplicate(true), "selected": _selected, "edge": _edge})
	if _undo.size() > 100: _undo.pop_front()
	_redo.clear(); _update_undo_buttons()

func _undo_step() -> void:
	if _undo.is_empty(): return
	_redo.append({"draft": draft.duplicate(true), "selected": _selected, "edge": _edge})
	_restore(_undo.pop_back()); _message.text = "Undone."

func _redo_step() -> void:
	if _redo.is_empty(): return
	_undo.append({"draft": draft.duplicate(true), "selected": _selected, "edge": _edge})
	_restore(_redo.pop_back()); _message.text = "Redone."

func _restore(snapshot: Dictionary) -> void:
	draft = snapshot.draft; _selected = snapshot.selected; _edge = snapshot.edge; _forge = {}
	_name.text = str(draft.get("name", "")); _id.text = str(draft.get("id", ""))
	_update_undo_buttons(); _changed(); _render()

func _update_undo_buttons() -> void:
	if _undo_button == null: return
	_undo_button.disabled = _undo.is_empty(); _redo_button.disabled = _redo.is_empty()

func _unhandled_key_input(event: InputEvent) -> void:
	if not (event is InputEventKey and event.pressed and not event.echo): return
	if get_children().any(func(c): return c.get_script() == CARD_EDITOR): return
	var command: bool = event.ctrl_pressed or event.meta_pressed
	var handled := true
	match event.keycode:
		KEY_ESCAPE:
			if not _canvas.connect_from.is_empty(): _canvas.connect_from = ""; _canvas.queue_redraw(); _canvas._overlay.queue_redraw()
			elif _trade_bar.visible: _trade_bar.hide()
			elif not _forge.is_empty(): _forge = {}; _render()
			elif _confirm.visible: _confirm.hide(); _sync_picker()
			else: _guard(func(): closed.emit(); queue_free())
		KEY_F: _canvas.home()
		KEY_Z when command and _admin():
			if event.shift_pressed: _redo_step()
			else: _undo_step()
		KEY_Y when command and _admin(): _redo_step()
		KEY_DELETE, KEY_BACKSPACE when _admin():
			if not _edge.is_empty(): _remove_edge(_edge)
			elif not _selected.is_empty() and _selected != str(draft.get("root", "")): _remove_node(_selected)
			else: handled = false
		KEY_U when _admin() and not command and not _selected.is_empty(): _open_forge("upgrade")
		KEY_D when _admin() and not command and not _selected.is_empty(): _open_forge("downgrade")
		_: handled = false
	if handled: get_viewport().set_input_as_handled()

# Context menu ----------------------------------------------------------------
func _menu_pressed(index: int) -> void:
	if index >= 0 and index < _menu_actions.size(): _menu_actions[index].call()

func _context_menu(kind: String, id: String, at: Vector2) -> void:
	_menu.clear(); _menu_actions.clear()
	var add := func(text: String, action: Callable, disabled: bool):
		_menu.add_item(text, _menu_actions.size()); _menu.set_item_disabled(_menu.item_count - 1, disabled); _menu_actions.append(action)
	match kind:
		"node":
			if _forge.get("node", "") != id: _forge = {}
			_selected = id; _edge = ""; _render()
			var node := _node(id)
			if _admin():
				add.call("Upgrade ↑", func(): _open_forge("upgrade"), false)
				add.call("Cheaper variant ↓", func(): _open_forge("downgrade"), false)
				add.call("Quick edit", func(): _open_forge("edit"), false)
				add.call("Full card editor…", func(): _edit_card(_node(id)), false)
				_menu.add_separator(); _menu_actions.append(func(): pass)
				add.call("Connect from here…", func(): _canvas.connect_from = id; _canvas._overlay.queue_redraw(), false)
				add.call("Center on card", func(): _canvas.focus_node(id), false)
				_menu.add_separator(); _menu_actions.append(func(): pass)
				add.call("Remove card", func(): _remove_node(id), id == str(draft.root))
			else:
				add.call("Center on card", func(): _canvas.focus_node(id), false)
				var offers := _offers()
				for key in offers:
					var offer: Dictionary = offers[key]
					if offer.from != id: continue
					var target := _node(str(offer.to))
					var cost := int(offer.cost)
					add.call("%s %s · %s" % ["Upgrade to" if cost > 0 else "Downgrade to", target.card.name, ("%d XP" % cost) if cost >= 0 else ("refund %d XP" % -cost)], func(): _request_trade(offer), not offer.available)
			if node.is_empty(): return
		"edge":
			_forge = {}; _edge = id; _selected = ""; _render()
			var edge := _edge_by_id(id)
			if _admin():
				add.call("Allow returning" if not edge.get("reversible", false) else "Make one-way", func(): _snapshot(); edge.reversible = not edge.reversible; _changed(); _render(), false)
				add.call("Remove connection", func(): _remove_edge(id), false)
			else: add.call("Show requirements", func(): pass, false)
		_:
			add.call("Center view", func(): _canvas.home(), false)
			if _admin():
				add.call("Auto-arrange", _arrange, draft.get("nodes", []).is_empty())
				add.call("Undo", _undo_step, _undo.is_empty())
				add.call("Redo", _redo_step, _redo.is_empty())
	if _menu.item_count == 0: return
	_menu.reset_size()
	_menu.position = Vector2i(get_window().position) + Vector2i(at) if not get_window().gui_embed_subwindows else Vector2i(at)
	_menu.popup()

# Connection details ----------------------------------------------------------
func _edge_details() -> void:
	var edge := _edge_by_id(_edge)
	if edge.is_empty(): return
	var a := _node(str(edge.from)); var b := _node(str(edge.to))
	var box := STYLE.section(_details)
	STYLE.heading(box, "Connection")
	_label(box, "%s → %s" % [a.card.name, b.card.name], 22, STYLE.GOLD_BRIGHT)
	var cost := int(b.card.economy.buy) - int(a.card.economy.buy)
	_label(box, ("Upgrading costs %d XP" % cost) if cost > 0 else ("Downgrading refunds %d XP" % -cost) if cost < 0 else "No XP difference", 16, STYLE.GOLD if cost > 0 else STYLE.AMBER)
	var entries := DIFF.changes(a.card, b.card, catalog)
	if not entries.is_empty():
		var diff := RichTextLabel.new(); diff.bbcode_enabled = true; diff.fit_content = true; diff.scroll_active = false; diff.mouse_filter = Control.MOUSE_FILTER_IGNORE
		diff.add_theme_font_size_override("normal_font_size", 16); diff.text = DIFF.bbcode(entries); box.add_child(diff)
	var rules := STYLE.section(_details)
	STYLE.heading(rules, "Deck requirements")
	_label(rules, "These rules restrict equipping, not upgrading. The resulting deck must meet every rule on at least one complete path, and keep meeting them while the card is equipped.", 14, STYLE.MUTED)
	var cards := _all_cards()
	if edge.requirements.is_empty(): _label(rules, "No rules · open path.", 15)
	for index in edge.requirements.size():
		var rule: Dictionary = edge.requirements[index]
		var name := str(cards.get(rule.card_id, rule.card_id))
		var forbidden: bool = rule.get("maximum", -1) == 0
		var label := RichTextLabel.new(); label.bbcode_enabled = true; label.fit_content = true; label.scroll_active = false; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
		label.add_theme_font_size_override("normal_font_size", 16)
		label.text = ("[color=#ef9e91]✕ [s]" + name.replace("[", "[lb]") + "[/s][/color] · Must not be in deck") if forbidden else "[color=#8fe2af]✓[/color] Requires %d × %s" % [int(rule.minimum), name.replace("[", "[lb]")]
		rules.add_child(label)
		if rule.get("include_descendants", false): _label(rules, "Includes every descendant of that card in its tree.", 14, STYLE.MUTED)
		if _admin():
			var controls := HBoxContainer.new(); controls.add_theme_constant_override("separation", 8); rules.add_child(controls)
			var limit := SpinBox.new(); limit.min_value = 1; limit.max_value = 100; limit.value = maxi(1, int(rule.minimum)); limit.visible = not forbidden; controls.add_child(limit)
			limit.value_changed.connect(func(v): _snapshot(); rule.minimum = int(v); _changed())
			var descendants := CheckBox.new(); descendants.text = "Include descendant variants"; descendants.button_pressed = rule.get("include_descendants", false); controls.add_child(descendants)
			descendants.toggled.connect(func(on): _snapshot(); rule.include_descendants = on; _changed())
			_button(controls, "Remove rule", func(): _snapshot(); edge.requirements.remove_at(index); _changed(); _render(), "rule.remove." + str(index))
	if _admin():
		var choice := _pick(rules, cards, "rule.card")
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); rules.add_child(row)
		for forbidden in [false, true]:
			var b2 := _button(row, "Forbid card" if forbidden else "Require card", func():
				if choice.selected < 0: return
				_snapshot()
				var rule := {"card_id": str(choice.get_selected_metadata()), "minimum": 0 if forbidden else 1, "include_descendants": false}
				if forbidden: rule.maximum = 0
				edge.requirements.append(rule); _changed(); _render(), "rule.forbid" if forbidden else "rule.require")
			b2.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var options := STYLE.section(_details)
		var reverse := CheckBox.new(); reverse.text = "Allow return to %s (refunds the same difference)" % a.card.name; reverse.button_pressed = edge.reversible; options.add_child(reverse)
		reverse.toggled.connect(func(on): _snapshot(); edge.reversible = on; _changed(); _render_canvas())
		STYLE.accent(_button(options, "Remove connection", func(): _remove_edge(str(edge.id)), "remove_edge"), STYLE.LOSS)

# Validation & publishing -----------------------------------------------------
func _local_issues() -> Dictionary:
	var out := {}
	var nodes: Array = draft.get("nodes", [])
	if nodes.is_empty(): return out
	var reached := _descendants(str(draft.root))
	var names := {}
	var used := {}
	var cards: Dictionary = catalog.get("card_names", {})
	var mine := {}
	for n in nodes: mine[n.card.id] = true
	for id in cards:
		if not mine.has(id): used[str(cards[id]).strip_edges().to_lower()] = true
	for n in nodes:
		var key := str(n.card.name).strip_edges().to_lower()
		if not reached.has(n.id): out[n.id] = "Not connected to the base card."
		elif int(n.card.economy.buy) < 1: out[n.id] = "XP value must be at least 1."
		elif names.has(key) or used.has(key): out[n.id] = "Another card is already named %s." % n.card.name
		names[key] = true
	return out

func _check_tree(announce: bool) -> bool:
	if not _admin(): return false
	if draft.get("nodes", []).is_empty():
		_validity.text = ""
		if announce: _message.text = "Choose a base card first."
		return false
	var response: Dictionary = _runtime().card_trees("validate_card_tree", draft, _revision, character_id, admin_token)
	var ok: bool = response.get("ok", false)
	var error := str(response.get("error", ""))
	_issues = _local_issues()
	if not ok:
		for n in draft.nodes:
			if error.contains(str(n.card.id)) or error.contains(str(n.card.name)): _issues[n.id] = error
	_publish.disabled = not ok
	_validity.text = ("✓ Valid · ready to publish" if _dirty else "✓ Valid · published") if ok else "⚠ " + error
	_validity.add_theme_color_override("font_color", STYLE.GAIN if ok else STYLE.LOSS)
	if announce: _message.text = "Tree valid · ready to publish." if ok else error
	if is_instance_valid(_canvas):
		_canvas.issues = _issues
		for n in draft.nodes:
			if _canvas._nodes.has(n.id):
				_canvas._nodes[n.id].issue = str(_issues.get(n.id, "")); _canvas._nodes[n.id].tooltip_text = _node_tooltip(n, _predecessors(str(n.id))); _canvas._nodes[n.id].queue_redraw()
	return ok

func _validate() -> bool:
	_live.stop()
	return _check_tree(true)

func _save() -> void:
	if not _validate(): return
	var response: Dictionary = _runtime().card_trees("publish_card_tree", draft, _revision, character_id, admin_token)
	if not response.get("ok", false): _message.text = str(response.get("error")); return
	var id := str(draft.id); _dirty = false; _reload()
	for i in _tree_pick.item_count:
		if _tree_pick.get_item_metadata(i) == id: _tree_pick.select(i)
	_load_tree(); _message.text = "Published tree and all card variants. Open Card Trees outside Admin to use it."
	_validity.text = "✓ Published"; _validity.add_theme_color_override("font_color", STYLE.GAIN)

# Player transactions ---------------------------------------------------------
func _request_trade(offer: Dictionary) -> void:
	if not offer.get("available", false): _message.text = str(offer.get("reason", "Unavailable")); return
	var source := _node(str(offer.from)); var target := _node(str(offer.to))
	var cost := int(offer.cost)
	var text := ("Upgrade %s → %s for %d XP?" % [source.card.name, target.card.name, cost]) if cost > 0 else ("Downgrade %s → %s and refund %d XP?" % [source.card.name, target.card.name, -cost]) if cost < 0 else "Switch %s → %s?" % [source.card.name, target.card.name]
	if int(_collection_counts().get(offer.from_card, 0)) > 0: text += "  Uses a stored copy; your deck is unchanged."
	else: text += "  Takes one copy out of your deck (health %d → %d). The new card goes to your collection." % [_health(), _health() - 1]
	_pending_trade = offer; _trade_text.text = text; _trade_bar.show()

func _trade(offer: Dictionary) -> void:
	if offer.is_empty(): return
	var response: Dictionary = _runtime().purchase_progression(character_id, "tree_card", str(offer.from_card), int(state.progression.revision), int(offer.cost), str(offer.to_card))
	_pending_trade = {}
	if not response.get("ok", false): _message.text = str(response.get("error")); return
	var id := str(draft.id); var target := str(offer.to); _reload(); draft = state.trees[id].duplicate(true); _sync_picker(); _selected = target; _edge = ""; _render()
	_canvas.play_transition(str(offer.from), target)
	_message.text = "Variant obtained and saved in your collection. Add it to the deck when its requirements are met."

func _collection_trade(kind: String, id: String, cost: int) -> void:
	var response: Dictionary = _runtime().purchase_progression(character_id, kind, id, int(state.progression.revision), cost)
	if not response.get("ok", false): _message.text = str(response.get("error")); return
	var tree_id := str(draft.id); _reload(); draft = state.trees[tree_id].duplicate(true); _sync_picker(); _render(); _message.text = "Collection and deck saved. Only equipped cards count as health."

# Example ---------------------------------------------------------------------
## An unpublished example with its own new base card, so it never edits Brace.
func _example() -> void:
	_new_tree()
	var name := _unique_name("Steady Guard")
	draft.name = name + " paths"; _name.text = draft.name
	var tree_id := _unique_tree_id(STYLE.slug(draft.name)); _id.text = tree_id; draft.id = tree_id; _id_touched = true
	var base: Dictionary
	if catalog.templates.has("brace"): base = catalog.templates.brace.duplicate(true)
	else:
		_start = {"name": name, "source": "effect:prevent", "energy": 1, "amount": 1, "xp": 10, "art": ""}
		base = _start_card()
	base.id = _unique_card_id(STYLE.slug(name)); base.name = name
	base.cost.energy = 1; base.economy = {"buy": 10, "sell": 10, "copy_limit": int(base.get("economy", {}).get("copy_limit", 20)), "upgrades": []}
	base.program.steps[0].params = {"amount": 1, "destination": "discard"}
	draft.nodes = [{"id": "base", "card": base, "x": 0.0, "y": 0.0}]
	var variant := func(id: String, suffix: String, buy: int, energy: int, amount: int, destination: String) -> void:
		var card: Dictionary = base.duplicate(true)
		card.id = tree_id + "_variant_" + id; card.name = name + " · " + suffix
		card.economy.buy = buy; card.economy.sell = buy; card.cost.energy = energy
		card.program.steps[0].params = {"amount": amount, "destination": destination}
		draft.nodes.append({"id": id, "card": card, "x": 0.0, "y": 0.0})
	variant.call("node_1", "Deeper guard", 13, 1, 2, "discard")
	variant.call("node_2", "Original piles", 14, 1, 1, "original")
	variant.call("node_3", "Complete guard", 17, 1, 2, "original")
	variant.call("node_4", "Heavy guard", 8, 2, 1, "discard")
	for pair in [["base", "node_1"], ["base", "node_2"], ["node_1", "node_3"], ["node_2", "node_3"], ["base", "node_4"]]: _connect(pair[0], pair[1])
	# Generate readable rules without publishing anything.
	for n in draft.nodes:
		var response: Dictionary = _runtime().card_authoring("validate_card", n.card)
		if response.get("ok", false): n.card.presentation = response.result.card.presentation
	_arrange(); _undo.clear(); _update_undo_buttons()
	_selected = "base"; _changed(); _render(); _canvas.home()
	_message.text = "Example draft with a new base card, %s (1 energy · prevent 1). Brace is unchanged. Edit, then publish when ready." % name
