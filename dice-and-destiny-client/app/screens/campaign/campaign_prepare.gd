extends Control
## The campaign's deck screen, focused on one campaign save (the player's own
## character sheet): its deck, stored cards and abilities, nothing else. Select a deck card to see its card tree and trade
## a copy up or down, buy another or sell one; "Add a new card" buys card-tree
## bases. Every offer arrives pre-checked by the authority (campaign_loadout)
## and every trade goes through it. Character Creation stays the admin editor.

signal closed

const STYLE := preload("res://app/screens/character/character_style.gd")
const CANVAS := preload("res://app/screens/character/card_tree_canvas.gd")
const DIFF := preload("res://app/screens/character/card_tree_diff.gd")

var save_id := ""
var character_id := ""
## The authority's campaign_loadout view.
var data: Dictionary = {}
## The character's catalog (card and ability definitions).
var catalog: Dictionary = {}
## The card authoring catalog, for "what changes" summaries.
var authoring: Dictionary = {}
## {"kind": "card" | "stored" | "ability" | "add", "id": ..., "tree": copy tree}
var selection: Dictionary = {}
## A tree card previewed by clicking it in the canvas.
var focus_node := ""

var _previous_catalog: Dictionary
var _title: Label
var _health: Label
var _xp: Label
var _message: Label
var _list: VBoxContainer
var _center_title: Label
var _center_hint: Label
var _canvas: Control
var _center_body: ScrollContainer
var _center_content: VBoxContainer
var _details: VBoxContainer
var _add_button: Button
## The "how many copies?" prompt for a tree trade, and its choice.
var _batch: Control
var _batch_offer: Dictionary = {}
var _batch_count := 1

func _ready() -> void:
	name = "CampaignPrepare"
	theme = STYLE.theme()
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_previous_catalog = BattlePresentationCatalog._catalog.duplicate(true)
	_build()
	reload()

func _exit_tree() -> void:
	BattlePresentationCatalog.configure(_previous_catalog)

# Layout ----------------------------------------------------------------------
func _build() -> void:
	var background := ColorRect.new(); background.color = STYLE.BACKDROP; background.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(background)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 20)
	add_child(margin)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 12); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "DICE & DESTINY  /  CAMPAIGN  /  PREPARE", 12, STYLE.GOLD)
	_title = STYLE.label(titles, "Prepare", 30)
	_health = STYLE.stat(header, "0", "Health · cards in deck", STYLE.HEALTH).get_child(0)
	_xp = STYLE.stat(header, "0", "XP to spend", STYLE.GOLD).get_child(0)
	for stat in header.get_children().slice(1): stat.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var back := _button(header, "Back to campaign", _close, "prepare.back"); back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	back.custom_minimum_size = Vector2(170, 44)
	_message = STYLE.label(body, "", 15, STYLE.MUTED)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 14); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	# Left: the character's deck, stored cards and abilities.
	var left := _panel(columns, 290)
	var list_scroll := ScrollContainer.new(); list_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; list_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; left.add_child(list_scroll)
	_list = VBoxContainer.new(); _list.name = "DeckList"; _list.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _list.add_theme_constant_override("separation", 6); list_scroll.add_child(_list)
	_add_button = STYLE.accent(_button(left, "+  Add a new card", func(): _select({"kind": "add"}), "prepare.add"))
	_add_button.custom_minimum_size.y = 46
	# Center: the selected card's tree, the ability, or the cards to add.
	var center := _panel(columns)
	center.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_center_title = STYLE.label(center, "", 20, STYLE.GOLD_BRIGHT)
	_center_hint = STYLE.label(center, "", 14, STYLE.MUTED)
	_canvas = CANVAS.new(); _canvas.name = "TreeCanvas"; _canvas.size_flags_vertical = Control.SIZE_EXPAND_FILL; _canvas.custom_minimum_size = Vector2(360, 300)
	center.add_child(_canvas)
	_canvas.node_selected.connect(_on_tree_node)
	_center_body = ScrollContainer.new(); _center_body.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center_body.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; center.add_child(_center_body)
	_center_content = VBoxContainer.new(); _center_content.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center_content.add_theme_constant_override("separation", 10); _center_body.add_child(_center_content)
	# Right: the selected card or ability and what you can do with it.
	var right := _panel(columns, 340)
	var details_scroll := ScrollContainer.new(); details_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; details_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; right.add_child(details_scroll)
	_details = VBoxContainer.new(); _details.name = "Details"; _details.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _details.add_theme_constant_override("separation", 10); details_scroll.add_child(_details)

func _panel(parent: Node, width: int = 0) -> VBoxContainer:
	var panel := PanelContainer.new(); panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 14, 10))
	if width > 0: panel.custom_minimum_size.x = width
	panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; parent.add_child(panel)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 10); panel.add_child(box)
	return box

# Data ------------------------------------------------------------------------
func reload() -> bool:
	var runtime := _runtime()
	if runtime == null: _error("The battle runtime is unavailable."); return false
	var response: Dictionary = runtime.campaign_loadout(save_id)
	if response.get("ok") != true: _error(str(response.get("error", "The campaign deck could not be loaded."))); return false
	data = response.result
	character_id = str(data.character)
	# Definitions only; the deck and abilities come from the campaign save.
	catalog = data.catalog
	BattlePresentationCatalog.configure(catalog)
	if authoring.is_empty():
		var authored: Dictionary = runtime.card_authoring()
		if authored.get("ok") == true: authoring = authored.result
	_title.text = "Prepare · %s" % str(data.save_name)
	_health.text = str(int(data.health))
	_xp.text = str(int(data.xp))
	if not _selection_valid(): selection = _default_selection()
	_render()
	return true

func _selection_valid() -> bool:
	match str(selection.get("kind", "")):
		"card": return not _entry("deck", str(selection.id), str(selection.get("tree", ""))).is_empty()
		"stored": return not _entry("collection", str(selection.id), str(selection.get("tree", ""))).is_empty()
		"ability": return data.abilities.any(func(a): return a.id == selection.id)
		"add": return true
	return false

func _default_selection() -> Dictionary:
	if data.deck.is_empty(): return {"kind": "add"}
	return {"kind": "card", "id": str(data.deck[0].card_id), "tree": str(data.deck[0].tree)}

func _entry(pile: String, card_id: String, tree: String) -> Dictionary:
	for entry in data.get(pile, []):
		if str(entry.card_id) == card_id and str(entry.tree) == tree: return entry
	return {}

# Rendering -------------------------------------------------------------------
func _render() -> void:
	_render_list()
	for child in _details.get_children(): _details.remove_child(child); child.queue_free()
	for child in _center_content.get_children(): _center_content.remove_child(child); child.queue_free()
	_canvas.visible = false; _center_body.visible = true
	match str(selection.kind):
		"card", "stored": _render_card()
		"ability": _render_ability()
		_: _render_add()

func _render_list() -> void:
	for child in _list.get_children(): _list.remove_child(child); child.queue_free()
	var conflicts: Array = data.get("tree_conflicts", [])
	STYLE.heading(_list, "Your deck · %d cards" % int(data.health))
	if not conflicts.is_empty():
		STYLE.label(_list, "The campaign uses only card-tree cards. Sell or store: %s." % ", ".join(PackedStringArray(conflicts)), 13, STYLE.LOSS)
	for entry in _sorted(data.deck):
		_list_row(entry, "card")
	if not data.collection.is_empty():
		STYLE.heading(_list, "Stored · not in your deck")
		for entry in _sorted(data.collection): _list_row(entry, "stored")
	STYLE.heading(_list, "Abilities")
	for ability in data.abilities:
		var definition: Dictionary = catalog.get("abilities", {}).get(str(ability.id), {})
		var row := _row_button(_list, "%s\n%s%s" % [definition.get("name", ability.id), str(ability.type).capitalize(), "  ·  upgrade available" if ability.get("upgrade", {}).get("available", false) else ""], null, "prepare.ability." + str(ability.id))
		row.button_pressed = selection.get("kind") == "ability" and selection.get("id") == ability.id
		var id := str(ability.id)
		row.pressed.connect(func(): _select({"kind": "ability", "id": id}))
	_add_button.button_pressed = selection.get("kind") == "add"

func _sorted(entries: Array) -> Array:
	var copy := entries.duplicate()
	copy.sort_custom(func(a, b): return _card_name(str(a.card_id)).naturalnocasecmp_to(_card_name(str(b.card_id))) < 0)
	return copy

func _list_row(entry: Dictionary, kind: String) -> void:
	var id := str(entry.card_id); var tree := str(entry.tree)
	var upgrades := 0
	if kind == "card":
		upgrades = _trades_from(id, tree).filter(func(t): return t.available and int(t.cost) > 0).size()
	var note := "×%d  ·  %d energy" % [int(entry.count), int(BattlePresentationCatalog.card(id).get("cost", 0))]
	if not entry.get("tree_card", true): note += "  ·  not usable in the campaign"
	elif upgrades > 0: note += "  ·  %d ↑" % upgrades
	var row := _row_button(_list, "%s\n%s" % [_card_name(id), note], STYLE.icon(STYLE.card_texture(id), 34, 46), "prepare.%s.%s" % [kind, id])
	if not entry.get("tree_card", true): row.add_theme_color_override("font_color", STYLE.LOSS)
	row.button_pressed = selection.get("kind") == kind and selection.get("id") == id and str(selection.get("tree", "")) == tree
	row.pressed.connect(func(): _select({"kind": kind, "id": id, "tree": tree}))

func _row_button(parent: Node, text: String, icon: Texture2D, control_id: String) -> Button:
	var row := _button(parent, text, func(): pass, control_id)
	row.toggle_mode = true; row.alignment = HORIZONTAL_ALIGNMENT_LEFT; row.icon = icon
	row.custom_minimum_size.y = 54; row.add_theme_font_size_override("font_size", 14)
	row.add_theme_stylebox_override("pressed", STYLE.box("223442", STYLE.GOLD, 10, 8, 2))
	row.add_theme_stylebox_override("hover_pressed", STYLE.box("2a3e4d", STYLE.GOLD_BRIGHT, 10, 8, 2))
	return row

## A deck or stored card: its tree in the middle, its trades on the right.
func _render_card() -> void:
	var id := str(selection.id); var tree_id := _copy_tree(id, str(selection.get("tree", "")))
	var stored: bool = selection.kind == "stored"
	var entry := _entry("collection" if stored else "deck", id, str(selection.get("tree", "")))
	var tree: Dictionary = data.trees.get(tree_id, {})
	if tree.is_empty():
		_center_title.text = _card_name(id)
		_center_hint.text = "This card is not part of any card tree, so the campaign cannot use it. Sell it for XP, or store it."
	else:
		_center_title.text = str(tree.name)
		_center_hint.text = "Click a glowing card to preview it. Upgrade or trade down as many copies as you like; they stay in your deck." if not stored else "Stored cards are not in your deck. Add one back to your deck to upgrade it."
		_show_tree(tree, id)
	var focus := _tree_node_card(tree, focus_node)
	if not focus.is_empty() and str(focus.card.id) != id: _render_preview(id, tree, focus)
	else: _render_owned(id, entry, stored)

func _render_owned(id: String, entry: Dictionary, stored: bool) -> void:
	_card_face(_details, id)
	STYLE.label(_details, _card_name(id), 22, STYLE.GOLD_BRIGHT)
	STYLE.label(_details, "%d %s · worth %d XP each" % [int(entry.count), "stored" if stored else "in your deck", int(entry.sell.cost)], 15, STYLE.MUTED)
	if stored:
		var copies := STYLE.section(_details); STYLE.heading(copies, "Stored copy")
		_offer_button(copies, entry.equip, "Add a copy to your deck", "prepare.equip." + id)
		_offer_button(copies, entry.sell, "Sell a copy · +%d XP" % int(entry.sell.cost), "prepare.sell_stored." + id)
		return
	var trades := _trades_from(id, str(entry.tree))
	var ups := trades.filter(func(t): return int(t.cost) >= 0)
	var downs := trades.filter(func(t): return int(t.cost) < 0)
	if not ups.is_empty():
		var box := STYLE.section(_details); STYLE.heading(box, "Upgrade one copy")
		for trade in ups: _trade_row(box, id, trade)
	if not downs.is_empty():
		var box := STYLE.section(_details); STYLE.heading(box, "Trade one copy down")
		for trade in downs: _trade_row(box, id, trade)
	if ups.is_empty() and downs.is_empty() and entry.get("tree_card", true):
		STYLE.label(_details, "No connections lead on from this card.", 14, STYLE.MUTED)
	var copies := STYLE.section(_details); STYLE.heading(copies, "Copies")
	var base := _base_offer(id)
	if not base.is_empty(): _offer_button(copies, base, "Buy another copy · %d XP" % int(base.cost), "prepare.buy." + id)
	else: STYLE.label(copies, "More copies come from upgrading the tree's base card.", 13, STYLE.MUTED)
	_offer_button(copies, entry.sell, "Sell a copy · +%d XP" % int(entry.sell.cost), "prepare.sell." + id, STYLE.AMBER)
	_offer_button(copies, entry.store, "Store a copy (−1 health, keeps its XP)", "prepare.store." + id, STYLE.DEFENSE)

## A card clicked in the tree: what it is and how to reach it from this copy.
func _render_preview(id: String, tree: Dictionary, node: Dictionary) -> void:
	var target := str(node.card.id)
	_card_face(_details, target)
	STYLE.label(_details, _card_name(target), 22, STYLE.GOLD_BRIGHT)
	var from := _tree_node_card(tree, _node_id(tree, id))
	if not from.is_empty():
		var change := DIFF.summary(from.card, node.card, authoring, 3)
		if not change.is_empty(): STYLE.label(_details, "Compared with %s: %s" % [_card_name(id), change], 14, STYLE.MUTED)
	var box := STYLE.section(_details)
	var direct: Array = _trades_from(id, str(selection.get("tree", ""))).filter(func(t): return str(t.request.target_id) == target)
	if direct.is_empty():
		STYLE.label(box, "Not connected to %s. Reach it one step at a time from a card you hold." % _card_name(id), 14, STYLE.MUTED)
	else:
		_trade_row(box, id, direct[0])
	_button(_details, "← Back to %s" % _card_name(id), func(): focus_node = ""; _render(), "prepare.preview_back")

func _trade_row(parent: Node, from: String, trade: Dictionary) -> void:
	var target := str(trade.request.target_id)
	var row := VBoxContainer.new(); row.add_theme_constant_override("separation", 4); parent.add_child(row)
	var tree: Dictionary = data.trees.get(str(trade.tree_id), {})
	var change := DIFF.summary(_tree_node_card(tree, str(trade.from_node)).get("card", {}), _tree_node_card(tree, str(trade.to_node)).get("card", {}), authoring, 3)
	STYLE.label(row, "→ " + _card_name(target), 17, STYLE.GOLD_BRIGHT)
	if not change.is_empty(): STYLE.label(row, change, 13, STYLE.MUTED)
	var label := ("Upgrade · %d XP" % int(trade.cost)) if int(trade.cost) > 0 else ("Trade down · +%d XP" % -int(trade.cost)) if int(trade.cost) < 0 else "Switch · free"
	if int(trade.get("max_count", 1)) > 1 and int(trade.cost) != 0: label += " each"
	_offer_button(row, trade, label, "prepare.trade.%s.%s" % [from, target], STYLE.GOLD if int(trade.cost) >= 0 else STYLE.AMBER)

func _offer_button(parent: Node, offer: Dictionary, text: String, control_id: String, color: Color = STYLE.GOLD) -> Button:
	var button := STYLE.accent(_button(parent, text, func(): _trade(offer), control_id), color)
	button.disabled = not offer.get("available", false)
	if button.disabled: STYLE.label(parent, _reason(str(offer.get("reason", ""))), 13, STYLE.LOSS)
	return button

func _reason(text: String) -> String:
	if text == "not enough XP": return "Not enough XP."
	if text.is_empty(): return "Unavailable."
	return text[0].to_upper() + text.substr(1) + ("" if text.ends_with(".") else ".")

func _render_ability() -> void:
	var id := str(selection.id)
	var ability: Dictionary = data.abilities.filter(func(a): return a.id == id)[0]
	var definition: Dictionary = catalog.get("abilities", {}).get(id, {})
	_center_title.text = str(definition.get("name", id))
	_center_hint.text = "%s ability" % str(ability.type).capitalize()
	_ability_rules(_center_content, id, "Equipped")
	for key in ["upgrade", "downgrade"]:
		if ability.has(key): _ability_rules(_center_content, str(ability[key].to), "Next tier" if key == "upgrade" else "Previous tier")
	STYLE.label(_details, str(definition.get("name", id)), 22, STYLE.GOLD_BRIGHT)
	STYLE.label(_details, "%s ability" % str(ability.type).capitalize(), 15, STYLE.MUTED)
	if ability.has("upgrade"):
		var box := STYLE.section(_details); STYLE.heading(box, "Upgrade")
		STYLE.label(box, "→ " + _ability_name(str(ability.upgrade.to)), 17, STYLE.GOLD_BRIGHT)
		_offer_button(box, ability.upgrade, "Upgrade · %d XP" % int(ability.upgrade.cost), "prepare.upgrade_ability." + id)
	if ability.has("downgrade"):
		var box := STYLE.section(_details); STYLE.heading(box, "Sell back this tier")
		STYLE.label(box, "→ " + _ability_name(str(ability.downgrade.to)), 17, STYLE.GOLD_BRIGHT)
		_offer_button(box, ability.downgrade, "Downgrade · +%d XP" % int(ability.downgrade.cost), "prepare.downgrade_ability." + id, STYLE.AMBER)
	if not ability.has("upgrade") and not ability.has("downgrade"):
		STYLE.label(_details, "No upgrades for this ability yet.", 14, STYLE.MUTED)

func _ability_rules(parent: Node, id: String, caption: String) -> void:
	var definition: Dictionary = catalog.get("abilities", {}).get(id, {})
	var box := STYLE.section(parent); STYLE.heading(box, "%s · %s" % [caption, definition.get("name", id)])
	STYLE.label(box, str(definition.get("presentation", {}).get("rules_text", "")), 16)

## The card-tree bases: every new card enters the deck as a base.
func _render_add() -> void:
	_center_title.text = "Add a new card"
	_center_hint.text = "Each card tree starts from one base card. Buy a base, then upgrade it in its tree. Every card is a point of health."
	var grid := HFlowContainer.new(); grid.name = "Bases"; grid.add_theme_constant_override("h_separation", 14); grid.add_theme_constant_override("v_separation", 14); _center_content.add_child(grid)
	for base in data.bases:
		var id := str(base.request.id)
		var tile := VBoxContainer.new(); tile.add_theme_constant_override("separation", 6); tile.custom_minimum_size.x = 190; grid.add_child(tile)
		_card_face(tile, id)
		STYLE.label(tile, "%s · %d in deck" % [str(base.tree_name).trim_suffix(" paths"), _deck_count(id)], 13, STYLE.MUTED)
		_offer_button(tile, base, "Buy · %d XP" % int(base.cost), "prepare.buy." + id)
	STYLE.label(_details, "Add a new card", 22, STYLE.GOLD_BRIGHT)
	STYLE.label(_details, "Buying a base adds a copy to your deck (+1 health). Select any deck card to upgrade it through its tree, or sell it back for its full XP.", 15, STYLE.MUTED)

# Tree canvas -----------------------------------------------------------------
func _show_tree(tree: Dictionary, card_id: String) -> void:
	_canvas.visible = true; _center_body.visible = false
	var counts := {}; var stored := {}
	for entry in data.deck: counts[_count_key(str(entry.card_id), str(entry.tree))] = int(entry.count)
	for entry in data.collection: stored[_count_key(str(entry.card_id), str(entry.tree))] = int(entry.count)
	_canvas.collection_counts = stored
	_canvas.card_art = {}; _canvas.summaries = {}; _canvas.tooltips = {}; _canvas.edge_tooltips = {}
	var states := {}; var edges := {}
	var from := _node_id(tree, card_id)
	var trades := _trades_from(card_id, str(selection.get("tree", "")))
	for n in tree.nodes:
		_canvas.card_art[n.card.id] = n.card.get("presentation", {}).get("illustration_path", "")
		var key := _count_key(str(n.card.id), str(tree.id) if n.get("shared", false) else "")
		states[n.id] = "owned" if int(counts.get(key, 0)) + int(stored.get(key, 0)) > 0 else "unowned"
		for e in tree.edges:
			if e.to == n.id:
				var before := _tree_node_card(tree, str(e.from))
				if not before.is_empty(): _canvas.summaries[n.id] = DIFF.summary(before.card, n.card, authoring, 1); break
	for trade in trades:
		if states.get(trade.to_node) != "owned": states[trade.to_node] = "available" if trade.available else "locked"
		edges[trade.edge] = "available" if trade.available else "locked"
	for e in tree.edges:
		if states.get(e.from) == "owned" and states.get(e.to) == "owned": edges[e.id] = "owned"
	_canvas.node_states = states; _canvas.edge_states = edges
	_canvas.title = str(tree.name)
	var focus := focus_node if not _tree_node_card(tree, focus_node).is_empty() else from
	var first: bool = _canvas.tree.get("id", "") != tree.id
	_canvas.show_tree(tree, counts, focus)
	if first: _canvas.call_deferred("home")

func _on_tree_node(node_id: String) -> void:
	var tree: Dictionary = data.trees.get(_copy_tree(str(selection.get("id", "")), str(selection.get("tree", ""))), {})
	var node := _tree_node_card(tree, node_id)
	if node.is_empty(): return
	var card_id := str(node.card.id)
	var copy_tree := str(tree.id) if node.get("shared", false) else ""
	# A card you hold switches to it; any other card is previewed.
	if card_id != str(selection.id) and not _entry("deck", card_id, copy_tree).is_empty():
		_select({"kind": "card", "id": card_id, "tree": copy_tree}); return
	focus_node = "" if card_id == str(selection.id) else node_id
	_render()

func _count_key(card_id: String, copy_tree: String) -> String:
	return "%s/%s" % [copy_tree, card_id] if not copy_tree.is_empty() else card_id

# Actions ---------------------------------------------------------------------
func _select(value: Dictionary) -> void:
	selection = value; focus_node = ""
	_render()

func _trade(offer: Dictionary) -> void:
	if not offer.get("available", false): return
	# With more than one copy that could go, ask how many first.
	if str(offer.request.kind) == "tree_card_deck" and int(offer.get("max_count", 1)) > 1:
		_ask_count(offer); return
	_submit(offer, 1)

func _submit(offer: Dictionary, count: int) -> void:
	var request: Dictionary = offer.request
	var response: Dictionary = _runtime().campaign_purchase(save_id, str(request.kind), str(request.id), int(data.revision), int(offer.cost) * count, str(request.target_id), str(request.tree), count)
	if response.get("ok") != true:
		_error("Not completed: %s" % str(response.get("error", "unknown error")))
		reload(); return
	var change := int(offer.get("xp_change", 0)) * count
	var spent := ("−%d XP" % -change) if change < 0 else ("+%d XP" % change) if change > 0 else "no XP"
	var kind := str(request.kind)
	var text := ""
	match kind:
		"tree_card_deck": text = "%s → %s%s · %s" % [_card_name(str(request.id)), _card_name(str(request.target_id)), " ×%d" % count if count > 1 else "", spent]
		"buy_card": text = "Bought %s · %s" % [_card_name(str(request.id)), spent]
		"sell_card", "sell_collection_card": text = "Sold %s · %s" % [_card_name(str(request.id)), spent]
		"unequip_collection_card": text = "Stored %s" % _card_name(str(request.id))
		"equip_collection_card": text = "Added %s to your deck" % _card_name(str(request.id))
		"upgrade_ability", "downgrade_ability": text = "%s → %s · %s" % [_ability_name(str(request.id)), _ability_name(str(request.get("target_id", ""))) if kind == "downgrade_ability" else _ability_name(str(_ability_upgrade_target(str(request.id)))), spent]
	# Follow the traded copy, so the next step up its tree is right there.
	if kind == "tree_card_deck": selection = {"kind": "card", "id": str(request.target_id), "tree": str(request.tree) if _is_shared(str(request.target_id)) else ""}
	elif kind == "buy_card": selection = {"kind": "card", "id": str(request.id), "tree": ""}
	elif kind == "upgrade_ability": selection = {"kind": "ability", "id": _ability_upgrade_target(str(request.id))}
	elif kind == "downgrade_ability": selection = {"kind": "ability", "id": str(request.target_id)}
	focus_node = ""
	reload()
	_message.text = text; _message.add_theme_color_override("font_color", STYLE.GAIN)

## How many copies to trade along one connection: 1 up to the largest batch the
## authority accepts now (copies held, XP, copy limits, deck rules).
func _ask_count(offer: Dictionary) -> void:
	_close_count()
	_batch_offer = offer
	var most := int(offer.max_count)
	_batch_count = most
	var from := str(offer.request.id); var target := str(offer.request.target_id)
	var held := int(_entry("deck", from, str(selection.get("tree", ""))).get("count", most))
	var upgrade := int(offer.cost) >= 0
	_batch = Control.new(); _batch.name = "CountPrompt"; _batch.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _batch.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(_batch)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.7); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _batch.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _batch.add_child(center)
	var panel := PanelContainer.new(); panel.custom_minimum_size.x = 480; panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, STYLE.GOLD, 22, 12, 2)); center.add_child(panel)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 12); panel.add_child(box)
	STYLE.heading(box, "Upgrade copies" if upgrade else "Trade copies down")
	STYLE.label(box, "%s → %s" % [_card_name(from), _card_name(target)], 22, STYLE.GOLD_BRIGHT)
	var note := "You have %d %s in your deck. How many do you want to %s?" % [held, _card_name(from), "upgrade" if upgrade else "trade down"]
	if most < held: note += " Up to %d can go now." % most
	STYLE.label(box, note, 15, STYLE.MUTED)
	var counts := HFlowContainer.new(); counts.name = "Counts"; counts.add_theme_constant_override("h_separation", 8); counts.add_theme_constant_override("v_separation", 8); box.add_child(counts)
	for n in range(1, most + 1):
		var choice := _button(counts, ("All %d" % n) if n == held and n > 1 else str(n), func(): _batch_count = n; _refresh_count(), "prepare.batch.count.%d" % n)
		choice.toggle_mode = true; choice.autowrap_mode = TextServer.AUTOWRAP_OFF; choice.custom_minimum_size = Vector2(64, 44)
		choice.add_theme_stylebox_override("pressed", STYLE.box("223442", STYLE.GOLD, 10, 8, 2))
		choice.set_meta("batch_count", n)
	var total := STYLE.label(box, "", 16, STYLE.IVORY); total.name = "Total"
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 10); box.add_child(actions)
	var cancel := _button(actions, "Cancel", _close_count, "prepare.batch.cancel"); cancel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var confirm := STYLE.accent(_button(actions, "", _confirm_count, "prepare.batch.confirm"), STYLE.GOLD if upgrade else STYLE.AMBER)
	confirm.name = "Confirm"; confirm.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	for button in [cancel, confirm]: button.custom_minimum_size.y = 48; button.autowrap_mode = TextServer.AUTOWRAP_OFF
	_refresh_count()
	confirm.grab_focus()

func _refresh_count() -> void:
	if _batch == null: return
	var per := int(_batch_offer.cost)
	var upgrade := per >= 0
	for choice in _batch.find_child("Counts", true, false).get_children(): choice.set_pressed_no_signal(int(choice.get_meta("batch_count")) == _batch_count)
	var copies := "%d %s" % [_batch_count, "copy" if _batch_count == 1 else "copies"]
	var xp := int(data.xp) - per * _batch_count
	(_batch.find_child("Total", true, false) as Label).text = "%s · %s · %d XP left" % [copies, ("−%d XP" % (per * _batch_count)) if upgrade else ("+%d XP" % (-per * _batch_count)), xp]
	(_batch.find_child("Confirm", true, false) as Button).text = ("Upgrade %s · %d XP" % [copies, per * _batch_count]) if upgrade else ("Trade down %s · +%d XP" % [copies, -per * _batch_count])

func _confirm_count() -> void:
	var offer := _batch_offer; var count := _batch_count
	_close_count()
	_submit(offer, count)

func _close_count() -> void:
	if _batch != null: _batch.queue_free()
	_batch = null

func _ability_upgrade_target(id: String) -> String:
	for ability in data.abilities:
		if ability.id == id and ability.has("upgrade"): return str(ability.upgrade.to)
	return id

func _close() -> void:
	closed.emit()
	queue_free()

func _unhandled_input(event: InputEvent) -> void:
	if not event.is_action_pressed("ui_cancel"): return
	get_viewport().set_input_as_handled()
	if _batch != null: _close_count()
	else: _close()

# Helpers ---------------------------------------------------------------------
func _trades_from(card_id: String, copy_tree: String) -> Array:
	return data.get("trades", []).filter(func(t): return str(t.request.id) == card_id and (not _is_shared(card_id) or str(t.request.tree) == copy_tree))

func _base_offer(card_id: String) -> Dictionary:
	for base in data.get("bases", []):
		if str(base.request.id) == card_id: return base
	return {}

func _deck_count(card_id: String) -> int:
	var total := 0
	for entry in data.deck:
		if str(entry.card_id) == card_id: total += int(entry.count)
	return total

## The tree a copy belongs to: its own tag for shared cards, else the one tree holding the card.
func _copy_tree(card_id: String, copy_tree: String) -> String:
	if not copy_tree.is_empty(): return copy_tree
	for id in data.get("tree_order", []):
		if not _node_id(data.trees[id], card_id).is_empty(): return id
	return ""

func _is_shared(card_id: String) -> bool:
	for id in data.get("tree_order", []):
		for n in data.trees[id].nodes:
			if n.card.id == card_id: return n.get("shared", false)
	return false

func _node_id(tree: Dictionary, card_id: String) -> String:
	for n in tree.get("nodes", []):
		if n.card.id == card_id: return str(n.id)
	return ""

func _tree_node_card(tree: Dictionary, node_id: String) -> Dictionary:
	for n in tree.get("nodes", []):
		if n.id == node_id: return n
	return {}

func _card_name(id: String) -> String:
	return str(catalog.get("cards", {}).get(id, {}).get("name", id))

func _ability_name(id: String) -> String:
	return str(catalog.get("abilities", {}).get(id, {}).get("name", id))

func _card_face(parent: Node, id: String) -> void:
	var frame := CenterContainer.new(); parent.add_child(frame)
	var card := BattleCard.new(); frame.add_child(card); card.configure("preview", id, true)
	card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.tooltip_text = ""; card.focus_mode = Control.FOCUS_NONE

func _error(text: String) -> void:
	_message.text = text
	_message.add_theme_color_override("font_color", STYLE.LOSS)

func _runtime() -> Node:
	return get_node_or_null("/root/LearnedBattleRuntime")

func _button(parent: Node, text: String, callback: Callable, control_id: String) -> Button:
	var button := Button.new(); button.text = text; button.pressed.connect(callback)
	# Only buttons inside the side panels wrap; the header's must keep their width.
	if parent is VBoxContainer: button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	button.set_meta("prepare_control", control_id)
	parent.add_child(button)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, button, text)
	return button
