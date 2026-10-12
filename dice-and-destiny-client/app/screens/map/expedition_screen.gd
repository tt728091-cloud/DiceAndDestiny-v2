extends Control
## The Expedition map. Everything shown here comes from the authority's
## viewer-safe `expedition_status`; every action is an `expedition_act` the
## authority validates. The client never computes costs, legality, rewards or
## hidden contents. Built in code like the campaign screens; drivable by the
## debug inspector (group `inspectable_battle_screen`, every control registered).

const STYLE := preload("res://app/screens/character/character_style.gd")
const HEX := preload("res://app/screens/map/hex_button.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const CAMPAIGN_PREPARE := preload("res://app/screens/campaign/campaign_prepare.gd")
const CAMPAIGN_SCREEN := "res://app/screens/campaign/campaign_screen.gd"
const BattleCardScene := preload("res://presentation/cards/battle_card.gd")

var save_id := ""
## The finished battle's campaign block, handed over by the battle screen.
var last_battle: Dictionary = {}
var status: Dictionary = {}
var view: Dictionary = {}

var selected_hex := Vector2i(999, 999)
var selected_cards: Array = []
var energy_payment := 0
var use_siege := false
var dismissed_rest := ""
var dismissed_event := ""
var _hex_buttons: Dictionary = {}
var _hex_radius := 30.0
var _map_origin := Vector2.ZERO
var _pan := Vector2.ZERO
var _dragging := false
var _previous_catalog: Dictionary = {}

var _title: Label
var _day: Label
var _message: Label
var _map_area: Control
var _map_canvas: Control
var _hud: VBoxContainer
var _hud_scroll: ScrollContainer
var _hand_row: HBoxContainer
var _overlay: PanelContainer
var _overlay_body: VBoxContainer
var _all_saves: Button
var _back: Button

func _ready() -> void:
	name = "ExpeditionScreen"
	theme = STYLE.theme()
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_to_group("inspectable_battle_screen")
	_previous_catalog = BattlePresentationCatalog._catalog.duplicate(true) if "_catalog" in BattlePresentationCatalog else {}
	_build()
	refresh()

func _exit_tree() -> void:
	if not _previous_catalog.is_empty(): BattlePresentationCatalog.configure(_previous_catalog)

# Layout ----------------------------------------------------------------------
func _build() -> void:
	var background := ColorRect.new(); background.color = STYLE.BACKDROP; background.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(background)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 14)
	add_child(margin)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 8); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "DICE & DESTINY  /  CAMPAIGN  /  EXPEDITION", 12, STYLE.GOLD)
	_title = STYLE.label(titles, "Expedition", 26)
	_day = STYLE.label(titles, "", 16, STYLE.GOLD_BRIGHT)
	_all_saves = _button(header, "All campaigns", _show_saves, "expedition.all_campaigns")
	_all_saves.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_back = _button(header, "Back to menu", _back_to_menu, "expedition.back")
	_back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_message = STYLE.label(body, "", 14, STYLE.MUTED)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 10); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	# Left: the map (pan by dragging) with the hand beneath it.
	var left := VBoxContainer.new(); left.size_flags_horizontal = Control.SIZE_EXPAND_FILL; left.size_flags_vertical = Control.SIZE_EXPAND_FILL; left.add_theme_constant_override("separation", 6); columns.add_child(left)
	var map_panel := PanelContainer.new(); map_panel.add_theme_stylebox_override("panel", STYLE.box("0d141c", "22323f", 4, 10)); map_panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; map_panel.clip_contents = true; left.add_child(map_panel)
	_map_area = Control.new(); _map_area.name = "MapArea"; _map_area.clip_contents = true; _map_area.mouse_filter = Control.MOUSE_FILTER_PASS; map_panel.add_child(_map_area)
	_map_area.gui_input.connect(_map_input)
	_map_area.resized.connect(_layout_map)
	_map_canvas = Control.new(); _map_canvas.name = "MapCanvas"; _map_canvas.mouse_filter = Control.MOUSE_FILTER_PASS; _map_area.add_child(_map_canvas)
	var hand_panel := PanelContainer.new(); hand_panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 8, 10)); hand_panel.custom_minimum_size.y = 196; left.add_child(hand_panel)
	var hand_scroll := ScrollContainer.new(); hand_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; hand_panel.add_child(hand_scroll)
	_hand_row = HBoxContainer.new(); _hand_row.name = "Hand"; _hand_row.add_theme_constant_override("separation", 8); hand_scroll.add_child(_hand_row)
	# Right: the HUD and actions.
	var right := PanelContainer.new(); right.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 10, 10)); right.custom_minimum_size.x = 390; right.size_flags_vertical = Control.SIZE_EXPAND_FILL; columns.add_child(right)
	_hud_scroll = ScrollContainer.new(); _hud_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; right.add_child(_hud_scroll)
	_hud = VBoxContainer.new(); _hud.name = "HUD"; _hud.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _hud.add_theme_constant_override("separation", 8); _hud_scroll.add_child(_hud)
	# Centre overlay for events, camp, rest and battle summaries.
	_overlay = PanelContainer.new(); _overlay.name = "Overlay"; _overlay.add_theme_stylebox_override("panel", STYLE.box("111c27", STYLE.GOLD, 18, 12, 2)); _overlay.visible = false
	_overlay.set_anchors_preset(Control.PRESET_CENTER); add_child(_overlay)
	_overlay_body = VBoxContainer.new(); _overlay_body.add_theme_constant_override("separation", 8); _overlay_body.custom_minimum_size.x = 440; _overlay.add_child(_overlay_body)

# Data -----------------------------------------------------------------------
func refresh() -> void:
	var runtime := _runtime()
	var response: Dictionary = runtime.expedition_status(save_id) if runtime != null else {"ok": false, "error": "The battle runtime is unavailable."}
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The expedition could not be loaded.")))
		return
	_apply(response.result)

func _apply(result: Dictionary) -> void:
	status = result
	view = status.get("view", {})
	var catalog: Dictionary = status.get("catalog", {})
	if not catalog.is_empty(): BattlePresentationCatalog.configure({"cards": catalog.get("cards", {}), "statuses": catalog.get("statuses", {})})
	selected_cards = selected_cards.filter(func(i): return i < _list(view, "hand").size())
	_render()

func _act(action: Dictionary) -> bool:
	var runtime := _runtime()
	if runtime == null: return false
	var response: Dictionary = runtime.expedition_act(save_id, action, int(status.get("revision", 0)))
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The action was refused.")))
		refresh()
		return false
	_message.text = ""
	selected_cards.clear(); energy_payment = 0; use_siege = false; selected_hex = Vector2i(999, 999)
	_apply(response.result)
	return true

func _show_error(text: String) -> void:
	_message.text = text
	_message.add_theme_color_override("font_color", STYLE.LOSS)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null and inspector.has_method("record_error"): inspector.record_error("expedition: " + text, {"save_id": save_id})

func _runtime() -> Node:
	return get_node_or_null("/root/LearnedBattleRuntime")

## A list from a view dictionary; absent or null lists read as empty.
static func _list(source: Dictionary, key: String) -> Array:
	var value = source.get(key)
	return value if value is Array else []

# Rendering ------------------------------------------------------------------
func _render() -> void:
	if view.is_empty(): return
	_title.text = "%s · %s" % [str(status.get("save_name", "Expedition")), str(view.get("region_name", ""))]
	_day.text = str(view.get("day_label", ""))
	_render_map()
	_render_hand()
	_render_hud()
	_render_overlay()

## Pointy-top axial → canvas pixels.
func _hex_center(q: int, r: int) -> Vector2:
	return _map_origin + _pan + Vector2(_hex_radius * sqrt(3.0) * (q + r * 0.5), _hex_radius * 1.5 * r)

func _layout_map() -> void:
	if view.is_empty() or _map_area == null: return
	_hex_radius = clampf(_map_area.size.x / 30.0, 24.0, 40.0)
	# Centre the region's full footprint (placed or not) in the map area.
	var lo := Vector2(INF, INF); var hi := Vector2(-INF, -INF)
	_map_origin = Vector2.ZERO; var pan := _pan; _pan = Vector2.ZERO
	for slot in _list(view, "slots"):
		for offset in HEX_OFFSETS:
			var c := _hex_center(int(slot.center.q) + int(offset.x), int(slot.center.r) + int(offset.y))
			lo = lo.min(c - Vector2.ONE * _hex_radius); hi = hi.max(c + Vector2.ONE * _hex_radius)
	_pan = pan
	_map_origin = (_map_area.size - (hi - lo)) * 0.5 - lo
	for key in _hex_buttons:
		var button = _hex_buttons[key]
		var parts := str(key).split(",")
		var center := _hex_center(int(parts[0]), int(parts[1]))
		button.radius = _hex_radius
		button.size = Vector2.ONE * _hex_radius * 2.0
		button.position = center - button.size * 0.5
		button.queue_redraw()

const HEX_OFFSETS := [Vector2i(0, 0), Vector2i(1, 0), Vector2i(1, -1), Vector2i(0, -1), Vector2i(-1, 0), Vector2i(-1, 1), Vector2i(0, 1)]

func _render_map() -> void:
	for child in _map_canvas.get_children(): _map_canvas.remove_child(child); child.queue_free()
	_hex_buttons.clear()
	var moves := {}
	for move in _list(view, "moves"): moves["%d,%d" % [int(move.to.q), int(move.to.r)]] = move
	var explores := {}
	for slot in _list(view, "explores"): explores[str(slot.id)] = slot
	var targets := {}
	for target in _list(view, "challenges") + _list(view, "enters"): targets["%d,%d" % [int(target.hex.q), int(target.hex.r)]] = target
	# Unexplored slots first (dashed outlines), then placed hexes on top.
	for slot in _list(view, "slots"):
		if slot.get("placed", false): continue
		for offset in HEX_OFFSETS:
			var q: int = int(slot.center.q) + int(offset.x); var r: int = int(slot.center.r) + int(offset.y)
			var button = _hex_button(q, r, {"q": q, "r": r, "terrain": ""}, "unexplored")
			button.slot_kind = str(slot.kind)
			button.set_meta("slot", str(slot.id))
			if explores.has(str(slot.id)):
				button.highlight = "explore" if slot.get("ready", false) else "blocked"
				button.tooltip_text = "Unexplored %s slot %s\nExplore: %d Move%s" % [slot.kind, slot.id, int(view.config.explore_cost), "" if explores[str(slot.id)].get("ready", false) else "\n" + str(explores[str(slot.id)].get("why", ""))]
			else:
				button.tooltip_text = "Unexplored %s slot %s\nStand next to it to explore" % [slot.kind, slot.id]
	for hex in _list(view, "hexes"):
		var q: int = int(hex.q); var r: int = int(hex.r)
		var state := "unknown"
		if hex.get("in_sight", false): state = "in_sight"
		elif hex.get("seen", false): state = "remembered"
		var button = _hex_button(q, r, hex, state)
		var key := "%d,%d" % [q, r]
		if moves.has(key):
			var move: Dictionary = moves[key]
			if not str(move.get("blocked", "")).is_empty(): button.highlight = "blocked"
			elif not str(move.get("provokes", "")).is_empty(): button.highlight = "provoke"
			else: button.highlight = "move"
		if targets.has(key): button.highlight = "target"
		button.warlord = bool(hex.get("warlord", false))
		button.selected = selected_hex == Vector2i(q, r)
		button.tooltip_text = _hex_tooltip(hex, moves.get(key, {}))
	_layout_map()

func _hex_button(q: int, r: int, data: Dictionary, state: String):
	var button = HEX.new()
	button.name = "Hex_%d_%d" % [q, r]
	button.data = data
	button.state = state
	button.pressed.connect(_select_hex.bind(Vector2i(q, r)))
	_map_canvas.add_child(button)
	_hex_buttons["%d,%d" % [q, r]] = button
	_register("expedition.hex.%d.%d" % [q, r], button, "Select hex (%d,%d)" % [q, r])
	return button

func _hex_tooltip(hex: Dictionary, move: Dictionary) -> String:
	var lines: Array = ["(%d,%d) · %s" % [int(hex.q), int(hex.r), str(hex.get("tile_name", ""))]]
	var terrain := str(hex.get("terrain", ""))
	var config: Dictionary = view.get("config", {}).get("terrain", {}).get(terrain, {})
	if config.get("impassable", false): lines.append("%s · impassable" % terrain.capitalize())
	else: lines.append("%s · Day %d / Night %d Move%s" % [terrain.capitalize(), int(config.get("day", 0)), int(config.get("night", 0)), " · cover" if config.get("cover", false) else ""])
	if not hex.get("seen", false): lines.append("Contents unknown")
	elif not hex.get("in_sight", false): lines.append("Out of sight · remembered")
	if hex.get("site") != null:
		var site: Dictionary = hex.site
		lines.append("%s (%s)" % [site.name, site.type])
		lines.append(str(site.get("description", "")))
		if site.get("defended", false): lines.append("Held by %s%s" % [site.get("defender_name", "enemies"), " · Fortified" if site.get("fortified", false) else ""])
		if site.get("goal", false): lines.append("The Warlord's goal")
		if site.get("conquered", false): lines.append("Conquered by the Warlord: no services")
		if site.get("cleared", false): lines.append("Cleared")
		if site.get("freed", false): lines.append("Freed")
		for service in _list(site, "services"): lines.append("· %s%s" % [service.label, "" if service.get("available", false) else " (%s)" % service.get("reason", "")])
		if site.get("has_deck_change", false): lines.append("· Deck changes: %d Influence each" % int(site.get("deck_change", 0)))
	if hex.get("group") != null:
		var group: Dictionary = hex.group
		lines.append("%s ×%d%s" % [group.name, int(group.count), " (last seen here)" if group.get("remembered", false) else ""])
		lines.append(str(group.get("habit_text", "")))
		lines.append("Victory: %d XP, %d gold%s" % [int(group.xp), int(group.gold), " · Lieutenant" if group.get("lieutenant", false) else ""])
		if group.get("ambushable", false): lines.append("Ambush possible from here now")
	if hex.get("warlord", false):
		var w: Dictionary = view.get("warlord", {})
		lines.append("The Warlord: %s" % w.get("name", ""))
		lines.append("Marching to %s · %d per rest%s" % [w.get("goal_name", ""), int(w.get("march_per_rest", 2)), " · Fortified" if w.get("fortified", false) else ""])
		if w.get("next_step") != null: lines.append("Next step: (%d,%d)" % [int(w.next_step.q), int(w.next_step.r)])
		lines.append("Lieutenants alive: %d%s" % [int(w.get("lieutenants", 0)), " · Conqueror" if w.get("conqueror", false) else ""])
		lines.append(str(w.get("habit_text", "")))
	if not move.is_empty():
		if not str(move.get("blocked", "")).is_empty(): lines.append("Cannot move: %s" % move.blocked)
		else:
			lines.append("Move here: %d Move" % int(move.cost))
			if not str(move.get("provokes", "")).is_empty(): lines.append("Provokes %s: a fight starts at once" % move.provokes)
			if move.get("event", false): lines.append("An event waits here")
	return "\n".join(lines)

func _map_input(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT:
		_dragging = event.pressed
	elif event is InputEventMouseMotion and _dragging and event.button_mask & MOUSE_BUTTON_MASK_LEFT:
		_pan += event.relative
		_layout_map()

func _select_hex(hex: Vector2i) -> void:
	selected_hex = Vector2i(999, 999) if selected_hex == hex else hex
	_render()

func _render_hand() -> void:
	for child in _hand_row.get_children(): _hand_row.remove_child(child); child.queue_free()
	var hand: Array = _list(view, "hand")
	STYLE.label(_hand_row, "Hand\n%d / %d" % [hand.size(), int(view.get("hand_size", 4))], 13, STYLE.MUTED).custom_minimum_size.x = 56
	for index in hand.size():
		var card: Dictionary = hand[index]
		var column := VBoxContainer.new(); column.add_theme_constant_override("separation", 4); _hand_row.add_child(column)
		var face := BattleCardScene.new(); column.add_child(face)
		face.configure("hand-%d" % index, str(card.id), true, false, true)
		face.mouse_filter = Control.MOUSE_FILTER_IGNORE; face.focus_mode = Control.FOCUS_NONE
		if card.get("map_only", false): STYLE.chip(column, "map only", STYLE.AMBER, 11)
		elif not str(card.get("map_effect", "")).is_empty(): STYLE.chip(column, "map: " + str(card.get("map_effect", "")), STYLE.GAIN, 11)
		else: STYLE.chip(column, "1 Move / 1 Influence", STYLE.MUTED, 11)
		var pay := _button(column, "Selected" if index in selected_cards else "Select", _toggle_card.bind(index), "expedition.card.%d" % index)
		pay.toggle_mode = true; pay.button_pressed = index in selected_cards
		pay.tooltip_text = "%s\n\nSelect it to spend it: 1 Move for walking or exploring, 1 Influence at a site, or discard it." % BattlePresentationCatalog.card_tooltip(str(card.id))
		if not str(card.get("map_effect", "")).is_empty():
			var play := _button(column, "Play on map" if int(card.get("energy_cost", 0)) == 0 else "Play (%d energy)" % int(card.energy_cost), func(): _act({"kind": "play_map_card", "card": str(card.id)}), "expedition.play.%d" % index)
			play.disabled = not card.get("map_playable", false)
			play.tooltip_text = str(card.get("map_reason", "")) if play.disabled else "Play %s for its map effect" % card.name

func _toggle_card(index: int) -> void:
	if index in selected_cards: selected_cards.erase(index)
	else: selected_cards.append(index)
	selected_cards.sort()
	_render()

func _payment_cards() -> Array:
	var hand: Array = _list(view, "hand")
	var ids: Array = []
	for index in selected_cards:
		if index < hand.size(): ids.append(str(hand[index].id))
	return ids

func _render_hud() -> void:
	for child in _hud.get_children(): _hud.remove_child(child); child.queue_free()
	var turn: Dictionary = view.get("turn", {})
	# Resources.
	var stats := HBoxContainer.new(); stats.add_theme_constant_override("separation", 10); _hud.add_child(stats)
	STYLE.stat(stats, "%d" % int(view.get("energy", 0)), "Energy · base %d" % int(view.get("base_energy", 0)), STYLE.ENERGY)
	STYLE.stat(stats, "%d" % int(view.get("gold", 0)), "Gold", STYLE.GOLD)
	STYLE.stat(stats, "%d" % int(status.get("xp", 0)), "XP", STYLE.GOLD_BRIGHT)
	STYLE.stat(stats, "%d/%d" % [int(view.get("health", 0)), int(view.get("max_health", 0))], "Health", STYLE.HEALTH)
	# Piles with hover lists.
	var piles := HBoxContainer.new(); piles.add_theme_constant_override("separation", 8); _hud.add_child(piles)
	for pile in [["Draw", "draw"], ["Spent", "spent"], ["Wounds", "wounds"]]:
		var ids: Array = view.get(pile[1], [])
		var chip := STYLE.chip(piles, "%s %d" % [pile[0], ids.size()], STYLE.IVORY, 13)
		chip.mouse_filter = Control.MOUSE_FILTER_STOP
		chip.tooltip_text = "%s (%d)%s" % [pile[0], ids.size(), "\n" + ", ".join(ids.map(func(id): return str(view.get("card_names", {}).get(id, id)))) if not ids.is_empty() else ""]
		chip.name = "Pile_" + pile[1]
	# Statuses.
	var statuses: Array = _list(view, "statuses")
	if not statuses.is_empty():
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); _hud.add_child(row)
		for st in statuses:
			var chip := STYLE.chip(row, "%s ×%d" % [st.name, int(st.stacks)], STYLE.LOSS, 13)
			chip.mouse_filter = Control.MOUSE_FILTER_STOP; chip.tooltip_text = "%s\n%s" % [st.name, st.rules]; chip.name = "Status_" + str(st.id)
	# Inventory.
	var items: Array = _list(view, "inventory")
	if not items.is_empty():
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); _hud.add_child(row)
		for item in items:
			var chip := STYLE.chip(row, "%s ×%d" % [item.name, int(item.count)], STYLE.AMBER, 13)
			chip.mouse_filter = Control.MOUSE_FILTER_STOP; chip.tooltip_text = "%s\n%s" % [item.name, item.description]; chip.name = "Item_" + str(item.id)
	# The Warlord.
	var w: Dictionary = view.get("warlord", {})
	var warlord_text := "Warlord %s: " % w.get("name", "")
	if w.get("defeated", false): warlord_text += "beaten"
	elif not w.get("known", false): warlord_text += "not yet seen · marching to %s" % w.get("goal_name", "")
	else: warlord_text += "%s at (%d,%d) · to %s%s" % ["in sight" if w.get("in_sight", false) else "last seen", int(w.last_seen.q) if w.get("last_seen") != null else 0, int(w.last_seen.r) if w.get("last_seen") != null else 0, w.get("goal_name", ""), " · goal taken" if w.get("goal_taken", false) else ""]
	warlord_text += " · Lieutenants %d" % int(w.get("lieutenants", 0))
	STYLE.label(_hud, warlord_text, 13, STYLE.MUTED)
	# Turn.
	var phase := str(view.get("phase", ""))
	var turn_text := "Turn %d · Move %d · cards played/discarded this turn: %d (need ≥ 1)%s" % [int(turn.get("number", 0)), int(turn.get("move", 0)), int(turn.get("cards_used", 0)), " · fought" if turn.get("fought", false) else ""]
	if int(turn.get("influence", 0)) > 0: turn_text += "\nInfluence %d at %s (this turn only)" % [int(turn.influence), str(turn.get("influence_site", ""))]
	if turn.get("action_used", false): turn_text += "\nAction taken this turn"
	turn_text += "\nSight %d hex%s" % [int(turn.get("sight", 1)), "" if int(turn.get("sight", 1)) == 1 else "es"]
	STYLE.label(_hud, turn_text, 13, STYLE.IVORY)
	# Payment: selected cards and energy.
	if phase == "turn":
		var pay_box := STYLE.section(_hud, 8)
		pay_box.get_parent().add_theme_stylebox_override("panel", STYLE.box(STYLE.RAISED, "22323f", 8, 8))
		var per := int(view.get("config", {}).get("energy_per_move", 5))
		var moves_paid := selected_cards.size() + energy_payment / per
		STYLE.label(pay_box, "Payment: %d card%s + %d energy = %d Move (pool %d)" % [selected_cards.size(), "" if selected_cards.size() == 1 else "s", energy_payment, moves_paid, int(turn.get("move", 0))], 13, STYLE.GOLD_BRIGHT)
		var energy_row := HBoxContainer.new(); energy_row.add_theme_constant_override("separation", 6); pay_box.add_child(energy_row)
		var minus := _button(energy_row, "−%d energy" % per, func(): energy_payment = maxi(0, energy_payment - per); _render(), "expedition.energy.minus")
		minus.disabled = energy_payment <= 0
		var plus := _button(energy_row, "+%d energy" % per, func(): energy_payment = mini(int(view.get("energy", 0)) / per * per, energy_payment + per); _render(), "expedition.energy.plus")
		plus.disabled = energy_payment + per > int(view.get("energy", 0))
		var discard := _button(energy_row, "Discard selected", func(): _act({"kind": "discard", "cards": _payment_cards()}), "expedition.discard")
		discard.disabled = selected_cards.is_empty()
	# Selected hex actions.
	_render_selected(phase, turn)
	# Site here.
	if view.get("site_here") != null and phase == "turn": _render_site(view.site_here)
	# Turn actions.
	if phase == "turn":
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); _hud.add_child(row)
		var end := STYLE.accent(_button(row, "End turn", func(): _act({"kind": "end_turn"}), "expedition.end_turn"))
		end.disabled = not turn.get("can_end_turn", false)
		end.tooltip_text = str(turn.get("end_turn_reason", "Keep your unplayed cards and draw up to your hand size"))
		var camp := _button(row, "Make camp", func(): _act({"kind": "make_camp"}), "expedition.make_camp")
		camp.tooltip_text = "Rest now: forfeit the cards in hand, reshuffle Spent, reset energy, camp services, then the Warlord marches and Day/Night flips"
	if phase == "victory" or phase == "defeat":
		STYLE.label(_hud, "The expedition is over: %s." % phase, 15, STYLE.GOLD_BRIGHT if phase == "victory" else STYLE.LOSS)
	# Log.
	var log_text: Array = _list(view, "log")
	if not log_text.is_empty():
		var log_label := STYLE.label(_hud, "\n".join(log_text.slice(maxi(0, log_text.size() - 6))), 12, STYLE.DIM)
		log_label.name = "Log"

func _render_selected(phase: String, turn: Dictionary) -> void:
	if selected_hex == Vector2i(999, 999) or phase != "turn": return
	var box := STYLE.section(_hud, 8)
	box.get_parent().add_theme_stylebox_override("panel", STYLE.box(STYLE.RAISED, STYLE.GOLD, 8, 8))
	var key := "%d,%d" % [selected_hex.x, selected_hex.y]
	STYLE.label(box, "Selected hex (%d,%d)" % [selected_hex.x, selected_hex.y], 14, STYLE.GOLD_BRIGHT)
	var hex_data: Dictionary = {}
	for hex in _list(view, "hexes"):
		if int(hex.q) == selected_hex.x and int(hex.r) == selected_hex.y: hex_data = hex
	if not hex_data.is_empty(): STYLE.label(box, _hex_tooltip(hex_data, {}), 12, STYLE.IVORY)
	for move in _list(view, "moves"):
		if int(move.to.q) != selected_hex.x or int(move.to.r) != selected_hex.y: continue
		var blocked := str(move.get("blocked", ""))
		var label := "Move here · %d Move" % int(move.cost) if blocked.is_empty() else "Cannot move: %s" % blocked
		if blocked.is_empty() and not str(move.get("provokes", "")).is_empty(): label += " · provokes %s" % move.provokes
		var button := STYLE.accent(_button(box, label, func(): _act({"kind": "move", "target": {"q": selected_hex.x, "r": selected_hex.y}, "cards": _payment_cards(), "energy": energy_payment}), "expedition.move"))
		button.disabled = not blocked.is_empty()
	for slot in _list(view, "explores"):
		var owns := false
		for offset in HEX_OFFSETS:
			if int(slot.center.q) + int(offset.x) == selected_hex.x and int(slot.center.r) + int(offset.y) == selected_hex.y: owns = true
		if not owns: continue
		var label := "Explore %s · %d Move" % [slot.id, int(view.config.explore_cost)]
		if not slot.get("ready", false): label += " (%s)" % slot.get("why", "")
		var button := STYLE.accent(_button(box, label, func(): _act({"kind": "explore", "slot": str(slot.id), "cards": _payment_cards(), "energy": energy_payment}), "expedition.explore"))
		button.disabled = not slot.get("ready", false)
	for target in _list(view, "challenges"):
		if int(target.hex.q) != selected_hex.x or int(target.hex.r) != selected_hex.y: continue
		var label := "Challenge %s%s%s" % [target.name, " · Ambush!" if target.get("ambush", false) else "", " · Fortified" if target.get("fortified", false) else ""]
		STYLE.accent(_button(box, label, func(): _act({"kind": "challenge", "group": str(target.id)}), "expedition.challenge"), STYLE.LOSS)
	for target in _list(view, "enters"):
		if int(target.hex.q) != selected_hex.x or int(target.hex.r) != selected_hex.y: continue
		var label := "Enter %s%s%s" % [target.name, " · Ambush!" if target.get("ambush", false) else "", " · Fortified" if target.get("fortified", false) and not use_siege else ""]
		STYLE.accent(_button(box, label, func(): _act({"kind": "enter_site", "site": str(target.id), "item": "siege_hooks" if use_siege else ""}), "expedition.enter"), STYLE.LOSS)
		if target.get("siege", false):
			var siege := _button(box, "Use Siege Hooks (defenders not Fortified)", func(): use_siege = not use_siege; _render(), "expedition.siege")
			siege.toggle_mode = true; siege.button_pressed = use_siege

func _render_site(site: Dictionary) -> void:
	var box := STYLE.section(_hud, 8)
	box.get_parent().add_theme_stylebox_override("panel", STYLE.box(STYLE.RAISED, "22323f", 8, 8))
	STYLE.label(box, "%s (%s)" % [site.name, site.type], 15, STYLE.GOLD_BRIGHT)
	if not site.get("open", false):
		STYLE.label(box, str(site.get("closed", "closed")), 12, STYLE.MUTED)
		return
	STYLE.label(box, str(site.get("description", "")), 12, STYLE.MUTED)
	var influence := _button(box, "Play %d selected card%s here for Influence" % [selected_cards.size(), "" if selected_cards.size() == 1 else "s"], func(): _act({"kind": "use_site", "service": "influence", "cards": _payment_cards()}), "expedition.site.influence")
	influence.disabled = selected_cards.is_empty()
	for service in _list(site, "services"):
		var id := "expedition.site.%s" % service.id + (".%s" % service.item if not str(service.get("item", "")).is_empty() else "")
		var action := {"kind": "use_site", "service": str(service.id), "item": str(service.get("item", "")), "count": 1}
		if service.id == "trainer": action["gold"] = int(service.gold)
		var button := _button(box, str(service.label), func(): _act(action), id)
		button.disabled = not service.get("available", false)
		if not service.get("available", false): button.tooltip_text = str(service.get("reason", ""))
	if site.get("has_deck_change", false):
		var prepare := _button(box, "Change deck · %d Influence per change" % int(site.get("deck_change", 0)), _open_prepare, "expedition.prepare")
		prepare.tooltip_text = "Open Prepare: buy, sell, upgrade or trade down cards and abilities (XP), paying this site's Influence per change"

func _render_overlay() -> void:
	for child in _overlay_body.get_children(): _overlay_body.remove_child(child); child.queue_free()
	_overlay.visible = false
	var phase := str(view.get("phase", ""))
	var body := _overlay_body
	if not last_battle.is_empty():
		_overlay.visible = true
		_render_battle_result(body)
		return
	if view.get("last_rest") != null and dismissed_rest != str(view.get("round", 0)):
		_overlay.visible = true
		var rest: Dictionary = view.last_rest
		var camp: Dictionary = rest.get("camp", {})
		STYLE.label(body, "Rest summary", 20, STYLE.GOLD_BRIGHT)
		var lines: Array = ["%d Spent cards reshuffled; energy %d → %d." % [int(camp.get("reshuffled", 0)), int(camp.get("energy_before", 0)), int(view.get("base_energy", 0))]]
		for roll in _list(camp, "mend_rolls"): lines.append("Mending roll: %d%s vs %d · %s" % [int(roll.roll), " +%d" % int(roll.bonus) if int(roll.bonus) > 0 else "", int(roll.needed), "mended" if roll.success else "failed"])
		lines.append("The Warlord marched %d hex%s%s." % [int(rest.get("warlord_steps", 0)), "" if int(rest.get("warlord_steps", 0)) == 1 else "es", " and took his goal" if rest.get("goal_taken", false) else ""])
		if not _list(rest, "patrols").is_empty(): lines.append("Patrols moved: %s" % ", ".join(rest.patrols))
		lines.append("%s · round %d. Drew %d." % ["Night falls" if rest.get("night_after", false) else "Day breaks", int(rest.get("round_after", 0)), int(rest.get("drew", 0))])
		if rest.get("deadline", false): lines.append("The last round is over: the Warlord comes to you.")
		STYLE.label(body, "\n".join(lines), 13, STYLE.IVORY)
		_button(body, "Continue", func(): dismissed_rest = str(view.get("round", 0)); _render(), "expedition.rest.continue")
		return
	if view.get("pending_battle") != null:
		_overlay.visible = true
		var pb: Dictionary = view.pending_battle
		STYLE.label(body, "A fight: %s" % pb.name, 20, STYLE.LOSS)
		STYLE.label(body, "%s · %s ×%d%s" % [str(pb.kind).capitalize(), pb.opponent, int(pb.count), " · Ambush!" if pb.get("ambush", false) else ""], 14, STYLE.IVORY)
		for st in _list(pb, "enemy_statuses"):
			var chip := STYLE.chip(body, "Enemy: %s ×%d" % [st.name, int(st.stacks)], STYLE.LOSS, 13); chip.mouse_filter = Control.MOUSE_FILTER_STOP; chip.tooltip_text = str(st.rules)
		for st in _list(pb, "player_statuses"):
			var chip := STYLE.chip(body, "You: %s ×%d" % [st.name, int(st.stacks)], STYLE.GAIN, 13); chip.mouse_filter = Control.MOUSE_FILTER_STOP; chip.tooltip_text = str(st.rules)
		STYLE.label(body, "The battle starts from your hand (%d), draw pile (%d), Spent (%d), wounds (%d) and %d energy." % [_list(view, "hand").size(), _list(view, "draw").size(), _list(view, "spent").size(), _list(view, "wounds").size(), int(pb.get("energy_in", 0))], 13, STYLE.MUTED)
		STYLE.accent(_button(body, "Fight", _start_fight, "expedition.fight"), STYLE.LOSS)
		return
	if view.get("event") != null:
		_overlay.visible = true
		var ev: Dictionary = view.event
		STYLE.label(body, str(ev.name), 20, STYLE.GOLD_BRIGHT)
		STYLE.label(body, str(ev.text), 14, STYLE.IVORY)
		STYLE.label(body, "Selected cards (%d) are played here as Influence." % selected_cards.size(), 12, STYLE.MUTED)
		for choice in _list(ev, "choices"):
			var button := _button(body, str(choice.text), func(): _act({"kind": "event_choice", "choice": str(choice.id), "cards": _payment_cards()}), "expedition.event." + str(choice.id))
			button.disabled = not choice.get("available", false)
			if not choice.get("available", false): button.tooltip_text = str(choice.get("reason", ""))
		return
	if view.get("last_event") != null and dismissed_event != _event_key(view.last_event):
		_overlay.visible = true
		var ev: Dictionary = view.last_event
		STYLE.label(body, "Event: %s" % ev.get("event", ""), 18, STYLE.GOLD_BRIGHT)
		if int(ev.get("roll", 0)) > 0: STYLE.label(body, "You rolled %d (needed %d)." % [int(ev.roll), int(ev.needed)], 14, STYLE.IVORY)
		STYLE.label(body, str(ev.get("text", "")), 14, STYLE.IVORY)
		_button(body, "Continue", func(): dismissed_event = _event_key(ev); _render(), "expedition.event.continue")
		return
	if phase == "camp" and view.get("camp") != null:
		_overlay.visible = true
		var camp: Dictionary = view.camp
		STYLE.label(body, "Camp · %s%s" % [str(camp.reason).replace("_", " "), " · at the inn (+%d to mending rolls)" % int(camp.inn_bonus) if camp.get("inn", false) else ""], 20, STYLE.GOLD_BRIGHT)
		STYLE.label(body, "%d Spent cards reshuffled into the draw pile; energy reset to %d.%s" % [int(camp.reshuffled), int(view.get("base_energy", 0)), " %d unplayed cards forfeited." % int(camp.forfeited) if int(camp.get("forfeited", 0)) > 0 else ""], 13, STYLE.IVORY)
		if camp.get("deadline", false): STYLE.label(body, "This was the last round: when you break camp the Warlord comes to you.", 13, STYLE.LOSS)
		STYLE.label(body, "Wounds: %d · bandages: %d · Influence: %d (play cards for more)" % [_list(view, "wounds").size(), int(camp.bandages), int(view.get("turn", {}).get("influence", 0))], 13, STYLE.MUTED)
		for roll in _list(camp, "mend_rolls"):
			STYLE.label(body, "Mending roll for %s: %d%s vs %d · %s (%s)" % [view.get("card_names", {}).get(roll.card, roll.card), int(roll.roll), " +%d" % int(roll.bonus) if int(roll.bonus) > 0 else "", int(roll.needed), "mended" if roll.success else "failed", roll.paid], 12, STYLE.GAIN if roll.success else STYLE.LOSS)
		var influence := _button(body, "Play %d selected card%s for Influence" % [selected_cards.size(), "" if selected_cards.size() == 1 else "s"], func(): _act({"kind": "camp_service", "service": "influence", "cards": _payment_cards()}), "expedition.camp.influence")
		influence.disabled = selected_cards.is_empty()
		var bandage := _button(body, "Mend roll with a bandage (%d+)" % int(camp.roll_needed), func(): _act({"kind": "camp_service", "service": "mend_roll", "item": "bandage"}), "expedition.camp.mend_bandage")
		bandage.disabled = int(camp.bandages) < 1 or _list(view, "wounds").size() == 0
		var paid := _button(body, "Mend roll for %d Influence (%d+)" % [int(camp.influence_per_roll), int(camp.roll_needed)], func(): _act({"kind": "camp_service", "service": "mend_roll"}), "expedition.camp.mend_influence")
		paid.disabled = int(view.get("turn", {}).get("influence", 0)) < int(camp.influence_per_roll) or _list(view, "wounds").size() == 0
		_button(body, "Change deck · %d Influence per change" % int(camp.deck_change), _open_prepare, "expedition.prepare")
		STYLE.accent(_button(body, "Break camp", func(): _act({"kind": "break_camp"}), "expedition.camp.break"))
		return
	if phase == "victory" or phase == "defeat":
		_overlay.visible = true
		STYLE.label(body, "The region is won!" if phase == "victory" else "The run is over", 22, STYLE.GOLD_BRIGHT if phase == "victory" else STYLE.LOSS)
		var lb: Dictionary = view.get("last_battle", {}) if view.get("last_battle") != null else {}
		if phase == "victory": STYLE.label(body, "The Warlord is beaten. XP earned this expedition: %d. Extra relic choices pending: %d. Every wound is mended." % [int(view.get("xp_earned", 0)), int(view.get("pending_relic_choices", 0))], 14, STYLE.IVORY)
		else: STYLE.label(body, "You lost to the Warlord. This save is marked ended; its XP and cards remain for the records.", 14, STYLE.IVORY)
		if not lb.is_empty(): STYLE.label(body, "Last battle: %s · %s" % [lb.get("name", ""), lb.get("result", "")], 12, STYLE.MUTED)
		_button(body, "All campaigns", _show_saves, "expedition.end.campaigns")

## One resolved event: its site, the event and the round and turn it resolved
## in. Dismissing it keeps it dismissed until another event resolves.
static func _event_key(ev: Dictionary) -> String:
	return "%s/%s/%s/%s" % [ev.get("site", ""), ev.get("event", ""), ev.get("round", 0), ev.get("turn", 0)]

func _render_battle_result(body: VBoxContainer) -> void:
	var outcome: Dictionary = last_battle.get("outcome", {})
	var report: Dictionary = outcome.get("report", {}) if outcome.get("report") != null else {}
	var expedition: Dictionary = last_battle.get("expedition", {})
	STYLE.label(body, "Battle: %s · %s" % [expedition.get("name", ""), str(outcome.get("result", last_battle.get("error", "not recorded"))).capitalize()], 20, STYLE.GAIN if outcome.get("result", "") == "victory" else STYLE.LOSS)
	if outcome.is_empty():
		STYLE.label(body, "The result was not recorded: %s" % last_battle.get("error", "unknown error"), 13, STYLE.LOSS)
	else:
		var lines: Array = []
		if int(outcome.get("xp_awarded", 0)) > 0: lines.append("+%d XP (now %d)%s" % [int(outcome.xp_awarded), int(outcome.get("xp", 0)), " · +%d early-strike XP" % int(report.get("early_xp", 0)) if int(report.get("early_xp", 0)) > 0 else ""])
		if int(report.get("gold", 0)) > 0 or int(report.get("loot", 0)) > 0: lines.append("+%d gold%s" % [int(report.get("gold", 0)), " and %d loot" % int(report.get("loot", 0)) if int(report.get("loot", 0)) > 0 else ""])
		lines.append("Energy: brought %d, ended %d, kept %d" % [int(report.get("energy_in", 0)), int(report.get("energy_end", 0)), int(report.get("energy_kept", 0))])
		lines.append("Cards back on the map: hand %d, draw %d, Spent %d · wounds %d" % [_list(report, "hand_after").size(), int(report.get("draw_after", 0)), int(report.get("spent_after", 0)), int(report.get("wounds", 0))])
		if int(report.get("defeat_wounds", 0)) > 0: lines.append("Defeat: %d defeat wounds on top of your pre-battle wounds; driven back; the round ends with a rest." % int(report.defeat_wounds))
		if not str(report.get("site_freed", "")).is_empty(): lines.append("%s is freed." % report.site_freed)
		if report.get("lieutenant", false) and outcome.get("result", "") == "victory": lines.append("A Lieutenant is slain: the Warlord loses a Lieutenant's Favor.")
		if report.get("region_won", false): lines.append("The region is won! %d relic choice(s) pending; wounds mended: %d." % [int(report.get("relic_choices", 0)), int(report.get("wounds_mended", 0))])
		if report.get("run_over", false): lines.append("The Warlord has won. The run is over.")
		STYLE.label(body, "\n".join(lines), 13, STYLE.IVORY)
	_button(body, "Continue", func(): last_battle = {}; _render(), "expedition.battle.continue")

# Actions that leave the screen --------------------------------------------------
func _start_fight() -> void:
	var pending: Dictionary = status.get("pending_battle", {}) if status.get("pending_battle") != null else {}
	if pending.is_empty(): return
	_message.text = "Preparing the fight…"
	await get_tree().process_frame
	var gateway: RefCounted = LEARNED_GATEWAY.new(_runtime(), "seat-a", "minion:%s:%d" % [str(pending.opponent), int(pending.opponent_count)], str(status.get("character", "starter")))
	gateway.unified_defense = true
	gateway.loadout_mode = "expedition"
	gateway.campaign_save = save_id
	gateway.expedition = true
	var result: Dictionary = gateway.start_battle(str(pending.id), 0)
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "The battle could not start.")))
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = "blade"
	screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new()
	screen.learned_battle_mode = true
	screen.learned_human_seat = "seat-a"
	screen.learned_seed = 0
	get_tree().root.add_child(screen)
	queue_free()

func _open_prepare() -> void:
	var screen = CAMPAIGN_PREPARE.new()
	screen.save_id = save_id
	screen.expedition = true
	screen.expedition_cost = int(view.get("camp", {}).get("deck_change", 0)) if view.get("phase", "") == "camp" else int(view.get("site_here", {}).get("deck_change", 0))
	screen.expedition_influence = int(view.get("turn", {}).get("influence", 0))
	screen.closed.connect(func(): show(); refresh())
	get_tree().root.add_child(screen)
	hide()

func _show_saves() -> void:
	var screen = load(CAMPAIGN_SCREEN).new()
	get_tree().root.add_child(screen)
	queue_free()

func _back_to_menu() -> void:
	get_tree().change_scene_to_file("res://app/boot/battle_bootstrap.tscn")
	queue_free()

# Inspector ---------------------------------------------------------------------
func inspection_state() -> Dictionary:
	var turn: Dictionary = view.get("turn", {})
	return {
		"ready": not view.is_empty(), "screen": "expedition", "save_id": save_id, "stage": "expedition_" + str(view.get("phase", "")), "segment": "expedition",
		"round": int(view.get("round", 0)), "night": view.get("night", false), "phase": str(view.get("phase", "")), "day_label": str(view.get("day_label", "")),
		"player": view.get("player", {}), "energy": int(view.get("energy", 0)), "gold": int(view.get("gold", 0)), "xp": int(status.get("xp", 0)),
		"hand": _list(view, "hand").map(func(c): return str(c.id)), "draw": _list(view, "draw").size(), "spent": _list(view, "spent").size(), "wounds": _list(view, "wounds"),
		"turn": turn, "moves": _list(view, "moves"), "explores": _list(view, "explores"), "challenges": _list(view, "challenges"), "enters": _list(view, "enters"),
		"site_here": view.get("site_here"), "pending_battle": view.get("pending_battle"), "event": view.get("event"), "camp": view.get("camp"), "last_battle": view.get("last_battle"), "last_rest": view.get("last_rest"),
		"statuses": _list(view, "statuses"), "inventory": _list(view, "inventory"), "warlord": view.get("warlord", {}), "result": str(view.get("result", "")),
		"selected_hex": [selected_hex.x, selected_hex.y] if selected_hex != Vector2i(999, 999) else null, "selected_cards": selected_cards, "energy_payment": energy_payment,
		"placed_tiles": _list(view, "slots").filter(func(s): return s.get("placed", false)).size(), "seen_hexes": _list(view, "hexes").filter(func(h): return h.get("seen", false)).size(),
		"message": _message.text,
	}

func _button(parent: Node, text: String, callback: Callable, control_id: String) -> Button:
	var button := Button.new(); button.text = text; button.pressed.connect(callback)
	button.set_meta("expedition_control", control_id)
	parent.add_child(button)
	_register(control_id, button, text)
	return button

func _register(control_id: String, control: Control, caption: String) -> void:
	control.set_meta("expedition_control", control_id)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, control, caption)
