extends Control
## The campaign's deck screen, focused on one campaign save (the player's own
## character sheet): its deck, stored cards and abilities, nothing else. Select a deck card to see its card tree and trade
## a copy up or down, buy another or sell one; "Add a new card" buys card-tree
## bases. Every offer arrives pre-checked by the authority (campaign_loadout)
## and every trade goes through it. Character Creation stays the admin editor.
## The Custom Adventurer also has its ability board here: step its damage
## level up or down and pick its defense, each priced in XP by the authority
## (campaign_board), and apply the change (campaign_board_change). In an
## expedition that counts as a deck change: at a site or camp, paying the
## visit's Influence once.

signal closed

const STYLE := preload("res://app/screens/character/character_style.gd")
const CANVAS := preload("res://app/screens/character/card_tree_canvas.gd")
const DIFF := preload("res://app/screens/character/card_tree_diff.gd")

var save_id := ""
## Expedition saves change their deck only at a site or camp: the first change
## of a visit spends `expedition_cost` Influence (cards played there this turn)
## and opens the deck; every further change until the visit ends (camp breaks,
## or the turn ends at a site) costs XP only. The authority charges it through
## `expedition_deck_change` and reports `deck_open`.
var expedition := false
var expedition_cost := 0
var expedition_influence := 0
var expedition_open := false
var _influence: Label
var character_id := ""
## The authority's campaign_loadout view.
var data: Dictionary = {}
## The character's catalog (card and ability definitions).
var catalog: Dictionary = {}
## The card authoring catalog, for "what changes" summaries.
var authoring: Dictionary = {}
## {"kind": "card" | "stored" | "ability" | "add" | "board", "id": ..., "tree": copy tree}
var selection: Dictionary = {}
## The Custom Adventurer's board (campaign_board): the save's board priced at
## `board_choice`, the level and defenses being considered.
var board_view: Dictionary = {}
var board_choice: Dictionary = {}
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
	_influence = STYLE.stat(header, "0", "Influence · opens the deck", STYLE.GAIN).get_child(0)
	_influence.get_parent().visible = expedition
	_influence.get_parent().name = "InfluenceStat"
	for stat in header.get_children().slice(1): stat.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var back := _button(header, "Back to campaign", _close, "prepare.back"); back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	back.custom_minimum_size = Vector2(170, 44)
	_message = STYLE.label(body, "", 15, STYLE.MUTED)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 14); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	# Left: the character's deck, stored cards and abilities.
	var left := _panel(columns, 320)
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
	if expedition:
		_influence.get_parent().visible = true
		_influence.text = "Open" if expedition_open else "%d / %d" % [expedition_influence, expedition_cost]
		_influence.get_parent().tooltip_text = "The deck is open for this visit: every change costs XP only until you leave (break camp, or end the turn at a site)." if expedition_open else "Your first change here spends %d Influence (cards played at this site or camp this turn) and opens the deck; after that, change it as often as your XP allows until you leave. You have %d." % [expedition_cost, expedition_influence]
	var wounds := int(data.get("max_health", data.health)) - int(data.health)
	_health.text = str(int(data.health)) if wounds == 0 else "%d/%d" % [int(data.health), int(data.max_health)]
	_xp.text = str(int(data.xp))
	board_view = {}; board_choice = {}
	_load_board({}, true)
	if not _selection_valid(): selection = _default_selection()
	_render()
	return true

## Prices the save's board at a level and defenses (empty: the saved board).
## quiet: a character without a priced board shows none, without an error.
func _load_board(choice: Dictionary, quiet := false) -> bool:
	var response: Dictionary = _runtime().campaign_board(save_id, choice)
	if response.get("ok") != true:
		var why := str(response.get("error", "unknown error"))
		if not (quiet and "does not build its board" in why): _error("The ability board could not be priced: %s" % why)
		return false
	board_view = response.result
	board_choice = board_view.choice.duplicate(true)
	return true

func _selection_valid() -> bool:
	match str(selection.get("kind", "")):
		"card": return not _entry("deck", str(selection.id), str(selection.get("tree", ""))).is_empty()
		"stored": return not _entry("collection", str(selection.id), str(selection.get("tree", ""))).is_empty()
		"ability": return data.abilities.any(func(a): return a.id == selection.id)
		"board": return not board_view.is_empty()
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
		"board": _render_board()
		_: _render_add()

func _render_list() -> void:
	for child in _list.get_children(): _list.remove_child(child); child.queue_free()
	var conflicts: Array = data.get("tree_conflicts", [])
	var deck_xp: int = data.deck.reduce(func(total, e): return total + int(e.count) * int(e.sell.cost), 0)
	STYLE.heading(_list, "Your deck · %d cards · %d XP" % [int(data.health), deck_xp])
	if not conflicts.is_empty():
		STYLE.label(_list, "The campaign uses only card-tree cards. Sell or store: %s." % ", ".join(PackedStringArray(conflicts)), 13, STYLE.LOSS)
	for entry in _sorted(data.deck):
		_list_row(entry, "card")
	if not data.get("wounds", []).is_empty():
		STYLE.heading(_list, "Wounded · frozen until mended")
		for entry in _sorted(data.wounds): _wounded_row(entry)
	if not data.collection.is_empty():
		STYLE.heading(_list, "Stored · not in your deck")
		for entry in _sorted(data.collection): _list_row(entry, "stored")
	STYLE.heading(_list, "Abilities")
	if not board_view.is_empty():
		var saved := _saved_board()
		var row := _row_button(_list, "Ability board · %s\nLevel avg %.2f  ·  %s" % [str(board_view.template_name), float(saved.average), _defense_names(saved.defenses)], null, "prepare.board")
		row.button_pressed = selection.get("kind") == "board"
		row.pressed.connect(func(): _select({"kind": "board"}))
		_row_xp(row, int(board_view.xp.held), 1)
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
	_row_xp(row, int(entry.sell.cost), int(entry.count))

## A wounded copy: shown for its XP, but frozen until it is mended. It can't
## be played, sold, upgraded, traded down or stored (the authority refuses it).
func _wounded_row(entry: Dictionary) -> void:
	var id := str(entry.card_id)
	var row := _row_button(_list, "%s\n×%d  ·  wounded · frozen" % [_card_name(id), int(entry.count)], STYLE.icon(STYLE.card_texture(id), 34, 46), "prepare.wounded." + id)
	row.disabled = true; row.toggle_mode = false
	row.tooltip_text = "Frozen until mended: a wounded copy can't be played, sold, upgraded, traded down or stored. It keeps its XP and starts each battle in your removed pile. " + ("Heal wounds at camp or mend them at a site." if expedition else "Clearing the campaign's last encounter mends every wound.")
	row.add_theme_color_override("font_disabled_color", STYLE.LOSS.lightened(0.2))
	_row_xp(row, int(entry.value), int(entry.count))

## The XP a row's copies are worth, on the row's right: "10 XP × 3" over "30 XP".
func _row_xp(row: Button, each: int, count: int) -> void:
	for state in ["normal", "hover", "pressed", "hover_pressed", "disabled", "focus"]:
		var style := row.get_theme_stylebox(state).duplicate() as StyleBox
		style.content_margin_right = 84
		row.add_theme_stylebox_override(state, style)
	var column := VBoxContainer.new(); column.name = "XP"; column.mouse_filter = Control.MOUSE_FILTER_IGNORE
	column.alignment = BoxContainer.ALIGNMENT_CENTER; column.add_theme_constant_override("separation", 0)
	column.anchor_left = 1.0; column.anchor_right = 1.0; column.anchor_top = 0.0; column.anchor_bottom = 1.0
	column.offset_left = -84; column.offset_right = -10
	row.add_child(column)
	var each_label := STYLE.label(column, "%d XP × %d" % [each, count], 12, STYLE.MUTED)
	var total := STYLE.label(column, "%d XP" % (each * count), 16, STYLE.GOLD)
	for label in [each_label, total]:
		label.autowrap_mode = TextServer.AUTOWRAP_OFF; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT

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
	var wounded := int(_entry("wounds", id, str(entry.tree)).get("count", 0))
	if wounded > 0 and not stored:
		var frozen := STYLE.label(_details, "%d wounded cop%s frozen until mended. These trades use your %d live cop%s." % [wounded, "y is" if wounded == 1 else "ies are", int(entry.count), "y" if int(entry.count) == 1 else "ies"], 14, STYLE.LOSS)
		frozen.name = "FrozenWounds"
	if stored:
		var copies := STYLE.section(_details); STYLE.heading(copies, "Stored copy")
		_offer_button(copies, entry.equip, "Add a copy to your deck", "prepare.equip." + id)
		_offer_button(copies, entry.sell, "Sell a copy · +%d XP" % int(entry.sell.cost), "prepare.sell_stored." + id)
		return
	var trades := _trades_from(id, str(entry.tree))
	var ups := trades.filter(func(t): return int(t.cost) >= 0)
	var downs := trades.filter(func(t): return int(t.cost) < 0)
	if not ups.is_empty():
		var box := STYLE.section(_details); STYLE.heading(box, "Upgrade")
		for trade in ups: _trade_row(box, id, trade)
	if not downs.is_empty():
		var box := STYLE.section(_details); STYLE.heading(box, "Trade down")
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

# Ability board (Custom Adventurer) -------------------------------------------
## The saved board: its level's step, average and defenses.
func _saved_board() -> Dictionary:
	var saved: Dictionary = board_view.get("saved", {})
	var level := float(saved.get("level", board_view.staircase.default_target))
	var k := _step_at(level)
	return {"step": k, "average": float(board_view.staircase.steps[k].average), "defenses": saved.get("defenses", [])}

func _step_at(target: float) -> int:
	var k := 0
	var steps: Array = board_view.get("staircase", {}).get("steps", [])
	for i in steps.size():
		if float(steps[i].at) <= target + 1e-9: k = i
	return k

func _defense_names(ids: Array) -> String:
	var names: Array[String] = []
	for id in ids:
		if str(id) == "none": continue
		names.append(_defense_entry(str(id)).get("name", str(id)))
	return "no defense" if names.is_empty() else ", ".join(names)

func _defense_entry(id: String) -> Dictionary:
	for d in board_view.get("xp", {}).get("defenses", []):
		if str(d.ability) == id: return d
	return {}

## The shown step of the board being considered.
func _board_step() -> int: return int(board_view.staircase.step)

func _render_board() -> void:
	var sc: Dictionary = board_view.staircase
	var steps: Array = sc.steps
	var k := _board_step(); var lowest := int(sc.lowest)
	var xp: Dictionary = board_view.xp
	var rate := int(xp.get("rates", {}).get("damage_point", 40))
	_center_title.text = "Ability board · %s · %s" % [str(board_view.die_name), str(board_view.template_name)]
	_center_hint.text = "Step the damage level up for %d XP per point of average damage it adds, or back down for a refund, and pick your defense. The authority prices every change." % rate
	if bool(board_view.get("moving", false)):
		var moving := STYLE.section(_center_content); STYLE.heading(moving, "Moving onto the priced board")
		var note := STYLE.label(moving, "You still fight with your original abilities. Your first change here moves them onto this board, the same shapes with priced damage, and your defense becomes the menu defense shown below. The board is part of your kit, so moving doesn't cost XP: you pay only for the change itself.", 14, STYLE.AMBER)
		note.name = "BoardMoving"
	# Damage level.
	var level := STYLE.section(_center_content); STYLE.heading(level, "Damage level")
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); level.add_child(row)
	var down := _button(row, "◀", func(): _board_level(k - 1), "prepare.board.level_down"); down.disabled = k <= lowest
	down.custom_minimum_size = Vector2(56, 44)
	var info := STYLE.label(row, "Step %d of %d · avg %.2f per turn · %d XP over the standard" % [k - lowest + 1, steps.size() - lowest, float(steps[k].average), int(steps[k].get("xp", 0))], 16, STYLE.GOLD_BRIGHT)
	info.size_flags_horizontal = Control.SIZE_EXPAND_FILL; info.name = "BoardLevel"
	var up := _button(row, "▶", func(): _board_level(k + 1), "prepare.board.level_up"); up.disabled = k + 1 >= steps.size()
	up.custom_minimum_size = Vector2(56, 44)
	if k + 1 < steps.size():
		STYLE.label(level, "Next step: %s · %+.3f avg · +%d XP" % [_board_changes(k + 1), float(steps[k + 1].average) - float(steps[k].average), int(steps[k + 1].get("xp", 0)) - int(steps[k].get("xp", 0))], 13, STYLE.MUTED)
	if k > lowest:
		STYLE.label(level, "This step: %s · %d XP" % [_board_changes(k), int(steps[k].get("xp", 0)) - int(steps[k - 1].get("xp", 0))], 13, STYLE.GOLD)
	# The abilities at this step.
	var abilities := STYLE.section(_center_content); STYLE.heading(abilities, "Abilities at this level")
	STYLE.label(abilities, "Raise one tier: the level climbs to the first step where it deals more, and the board rises with it.", 12, STYLE.MUTED)
	var saved_step := int(_saved_board().step)
	var raises: Array = sc.get("raises", [])
	for i in board_view.tiers.size():
		var tier: Dictionary = board_view.tiers[i]
		var now := int(steps[k].damage[i]); var was := int(steps[saved_step].damage[i])
		var text := "%s → %d damage" % [str(tier.label), now]
		if now != was: text += "  (now %d)" % was
		var line := HBoxContainer.new(); line.add_theme_constant_override("separation", 8); abilities.add_child(line)
		var what := STYLE.label(line, text, 14, STYLE.GOLD_BRIGHT if now != was else STYLE.IVORY)
		what.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var raise = raises[i] if i < raises.size() else null
		if raise is Dictionary:
			var to_step := int(raise.step)
			var others: Array = raise.get("also", [])
			var plus := _button(line, "Raise to %d · %d XP%s" % [int(raise.to), int(raise.get("xp", 0)), (" · +%d more" % others.size()) if not others.is_empty() else ""], func(): _board_level(to_step), "prepare.board.raise.%d" % i)
			plus.tooltip_text = _raise_text(raise, board_view.tiers)
		else:
			STYLE.label(line, "at the top", 12, STYLE.DIM)
	# Defenses: one list per slot.
	var chosen: Array = board_choice.get("defenses", [])
	for slot in chosen.size():
		var box := STYLE.section(_center_content); STYLE.heading(box, "Defense" if chosen.size() == 1 else "Defense %d" % (slot + 1))
		var saved_defenses: Array = xp.get("saved_defenses", [])
		var saved_here := str(saved_defenses[slot]) if slot < saved_defenses.size() else "none"
		var none_change := int(xp.get("none_change", [])[slot]) if slot < xp.get("none_change", []).size() else 0
		var none := _button(box, "No defense · %s" % _defense_change("none" == saved_here, none_change, 0), func(): _board_defense(slot, "none"), "prepare.board.defense.%d.none" % slot)
		none.toggle_mode = true; none.set_pressed_no_signal(str(chosen[slot]) == "none")
		for d in xp.get("defenses", []):
			var id := str(d.ability)
			var what := ("blocks %.0f%% of attacks" % (float(d.block) * 100.0)) if bool(d.blocks) else ("prevents %.2f per roll" % float(d.prevention))
			var change := int(d.get("change", [])[slot]) if slot < d.get("change", []).size() else int(d.xp)
			var b := _button(box, "%s · %s · %s" % [str(d.name), what, _defense_change(id == saved_here, change, int(d.xp))], func(): _board_defense(slot, id), "prepare.board.defense.%d.%s" % [slot, id])
			b.toggle_mode = true; b.set_pressed_no_signal(str(chosen[slot]) == id)
			b.disabled = id in chosen and str(chosen[slot]) != id
			b.tooltip_text = str(catalog.get("abilities", {}).get(id, {}).get("presentation", {}).get("rules_text", ""))
		var rates: Dictionary = xp.get("rates", {})
		if not rates.is_empty():
			STYLE.label(box, "Flat defenses cost %d XP per point of average prevention. Blocks cost their block chance × %.0f%% of (total XP + %d): they grow as you earn XP." % [int(rates.prevention_point), float(rates.block.share) * 100.0, int(rates.block.offset)], 12, STYLE.DIM)
	# Details: the change and its price.
	STYLE.label(_details, "Ability board", 22, STYLE.GOLD_BRIGHT)
	STYLE.label(_details, "Your board holds %d XP now (damage level and defense). %d XP of %d total is left to spend." % [int(xp.held), int(xp.available), int(xp.total)], 15, STYLE.MUTED)
	var cost := int(xp.cost)
	var changed := cost != 0 or JSON.stringify(board_choice) != JSON.stringify(board_view.saved)
	var box := STYLE.section(_details); STYLE.heading(box, "This change")
	STYLE.label(box, "Level avg %.2f → %.2f · %s → %s" % [float(_saved_board().average), float(steps[k].average), _defense_names(board_view.saved.get("defenses", [])), _defense_names(chosen)], 14, STYLE.IVORY)
	var label := ("Apply · %d XP" % cost) if cost > 0 else (("Apply · +%d XP back" % -cost) if cost < 0 else "Apply · no XP")
	if expedition and not expedition_open: label += " · opens the deck (%d Influence)" % expedition_cost
	var apply := STYLE.accent(_button(box, label, _apply_board, "prepare.board.apply"), STYLE.GOLD if cost >= 0 else STYLE.AMBER)
	apply.name = "ApplyBoard"
	apply.disabled = not changed or not bool(xp.get("affordable", true))
	if not bool(xp.get("affordable", true)): STYLE.label(box, _reason(str(xp.get("reason", "not enough XP"))), 13, STYLE.LOSS)
	elif not changed: STYLE.label(box, "This is your board. Step the level or pick a defense to change it.", 13, STYLE.MUTED)
	else: STYLE.label(box, "%d XP left after it." % int(xp.left), 13, STYLE.MUTED)
	if changed: _button(box, "Undo · back to your board", func(): _load_board({}); _render(), "prepare.board.reset")

## What picking a defense changes against the saved board's defense in its
## slot (the authority's numbers): "yours", "costs 46 XP" or "47 XP back",
## with the defense's own price.
func _defense_change(yours: bool, change: int, price: int) -> String:
	var held := " (%d XP)" % price if price > 0 else ""
	if yours: return "yours" + held
	if change > 0: return "costs %d XP%s" % [change, held]
	if change < 0: return "%d XP back%s" % [-change, held]
	return "no XP change" + held

## A tier raise's full story for its tooltip: the climb, its XP and the
## other tiers it raises on the way.
func _raise_text(raise: Dictionary, tiers: Array) -> String:
	var label := func(id: String) -> String:
		for t in tiers:
			if str(t.tier) == id: return str(t.label)
		return id
	var text := "%s %d → %d for %d XP: the level climbs to avg %.2f." % [label.call(str(raise.tier)), int(raise.from), int(raise.to), int(raise.get("xp", 0)), float(raise.average)]
	var others: Array[String] = []
	for o in raise.get("also", []): others.append("%s %d → %d" % [label.call(str(o.tier)), int(o.from), int(o.to)])
	if not others.is_empty(): text += " It also raises " + ", ".join(others) + "."
	return text

## A step's raises, "Cinder ×4 5 → 6".
func _board_changes(k: int) -> String:
	var steps: Array = board_view.staircase.steps
	var out: Array[String] = []
	for tier in steps[k].get("raised", []):
		for i in board_view.tiers.size():
			if str(board_view.tiers[i].tier) == str(tier):
				out.append("%s %d → %d" % [str(board_view.tiers[i].label), int(steps[k - 1].damage[i]), int(steps[k].damage[i])])
	return ", ".join(out)

func _board_level(k: int) -> void:
	var sc: Dictionary = board_view.staircase
	k = clampi(k, int(sc.lowest), sc.steps.size() - 1)
	var next := board_choice.duplicate(true)
	if k == int(sc.lowest): next.erase("level")
	else: next.level = float(sc.steps[k].at)
	if _load_board(next): _render()

func _board_defense(slot: int, id: String) -> void:
	var next := board_choice.duplicate(true)
	next.defenses[slot] = id
	if _load_board(next): _render()

func _apply_board() -> void:
	var cost := int(board_view.xp.cost)
	var response: Dictionary = _runtime().campaign_board_change(save_id, board_choice, int(board_view.revision), cost)
	if response.get("ok") != true:
		_error("Not completed: %s" % str(response.get("error", "unknown error"))); reload(); return
	var view: Dictionary = response.result.get("view", {})
	if expedition and not view.is_empty():
		expedition_influence = int(view.get("turn", {}).get("influence", expedition_influence))
		var visit = view.get("camp") if str(view.get("phase", "")) == "camp" else view.get("site_here")
		if visit is Dictionary: expedition_open = bool(visit.get("deck_open", expedition_open))
	selection = {"kind": "board"}
	reload()
	_message.text = "Ability board changed · %s" % (("−%d XP" % cost) if cost > 0 else (("+%d XP" % -cost) if cost < 0 else "no XP"))
	_message.add_theme_color_override("font_color", STYLE.GAIN)

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
	var response: Dictionary
	if expedition:
		response = _runtime().expedition_deck_change(save_id, str(request.kind), str(request.id), int(data.revision), int(offer.cost) * count, str(request.target_id), str(request.tree), count)
		if response.get("ok") == true:
			var view: Dictionary = response.result.get("view", {})
			expedition_influence = int(view.get("turn", {}).get("influence", expedition_influence))
			var visit = view.get("camp") if str(view.get("phase", "")) == "camp" else view.get("site_here")
			if visit is Dictionary: expedition_open = bool(visit.get("deck_open", expedition_open))
	else:
		response = _runtime().campaign_purchase(save_id, str(request.kind), str(request.id), int(data.revision), int(offer.cost) * count, str(request.target_id), str(request.tree), count)
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
