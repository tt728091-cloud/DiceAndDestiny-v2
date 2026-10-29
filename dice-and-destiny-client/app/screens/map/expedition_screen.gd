extends Control
## The Expedition map. Everything shown here comes from the authority's
## viewer-safe `expedition_status`; every action is an `expedition_act` the
## authority validates. The client never computes costs, legality, rewards or
## hidden contents. Built in code like the campaign screens; drivable by the
## debug inspector (group `inspectable_battle_screen`, every control registered).
##
## Look: "The Surveyor's Chart" — a torn parchment chart on the battle's stone
## table, watercolour-washed hex tiles with ink motifs, bone chips and pewter
## "?" chips, gold and red pieces drawn by an ungraded marks layer, a stone
## ledger with a bone-framed stat strip and parchment cards, and overlays as a
## parchment sheet over a darkened habitat painting.

const STYLE := preload("res://app/screens/character/character_style.gd")
const CHART := preload("res://app/screens/map/map_style.gd")
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const HEX := preload("res://app/screens/map/hex_button.gd")
const SHEET := preload("res://app/screens/map/map_sheet.gd")
const LINES := preload("res://app/screens/map/map_line_layer.gd")
const MARKS := preload("res://app/screens/map/map_marks_layer.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const CAMPAIGN_PREPARE := preload("res://app/screens/campaign/campaign_prepare.gd")
const CAMPAIGN_SCREEN := "res://app/screens/campaign/campaign_screen.gd"
const BattleCardScene := preload("res://presentation/cards/battle_card.gd")
const HEAL_FEEDBACK := preload("res://app/screens/map/heal_feedback.gd")

const HEX_OFFSETS := [Vector2i(0, 0), Vector2i(1, 0), Vector2i(1, -1), Vector2i(0, -1), Vector2i(-1, 0), Vector2i(-1, 1), Vector2i(0, 1)]
const TERRAIN_ORDER := ["plains", "hills", "forest", "marsh", "lake", "mountain"]
const NONE := Vector2i(999, 999)

var save_id := ""
## The finished battle's campaign block, handed over by the battle screen.
var last_battle: Dictionary = {}
var status: Dictionary = {}
var view: Dictionary = {}

var selected_hex := NONE
var selected_cards: Array = []
var energy_payment := 0
var use_siege := false
var dismissed_rest := ""
var dismissed_event := ""
# The arrangement being built for a look effect: indexes into view.look.cards.
var _look_key := ""
var _look_draw := -1
var _look_top: Array = []
var _look_bottom: Array = []
## What the chart layers draw, built from the view by _render_map.
var map_model: Dictionary = {}
var _hex_buttons: Dictionary = {}
var _hex_radius := 30.0
var _map_origin := Vector2.ZERO
var _pan := Vector2.ZERO
var _dragging := false
var _previous_catalog: Dictionary = {}
var _last_player := NONE
var _survey_key := false
var _survey_locked := false
var _legend_filter := ""
static var _legend_open := false

var _save_label: Label
var _message: Label
var _map_panel: PanelContainer
var _map_area: Control
var _map_canvas: Control
var _chart: Control
var _sheet: Control
var _lines: Control
## The hover area over the chart's cartouche: hovering it shows the calendar.
var _cartouche: Control
var _marks: Control
var _vignette: TextureRect
var _scrim: ColorRect
var _hud: VBoxContainer
var _hud_scroll: ScrollContainer
var _footer: HBoxContainer
var _hand_row: HBoxContainer
var _overlay: PanelContainer
var _overlay_body: VBoxContainer
var _overlay_backdrop: TextureRect
var _overlay_sheet: PanelContainer
var _overlay_scroll: ScrollContainer
var _overlay_footer: VBoxContainer  # actions pinned below the scrolling body
## The short dice-roll flourish played over the camp sheet for each heal.
var _heal_feedback: Control
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
	# No banner: the chart's cartouche names the region and the day, and the
	# campaign name and the way out sit at the top of the ledger, so the map
	# takes the full height.
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 10); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	# Left: the chart on the table (drag to pan) with the hand beneath it.
	var left := VBoxContainer.new(); left.size_flags_horizontal = Control.SIZE_EXPAND_FILL; left.size_flags_vertical = Control.SIZE_EXPAND_FILL; left.add_theme_constant_override("separation", 6); columns.add_child(left)
	_map_panel = PanelContainer.new(); _map_panel.add_theme_stylebox_override("panel", CHART.stone_box(Color(0, 0, 0, 0), CHART.STONE_KEYLINE, 4, 4)); _map_panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; _map_panel.clip_contents = true; left.add_child(_map_panel)
	_map_area = Control.new(); _map_area.name = "MapArea"; _map_area.clip_contents = true; _map_area.mouse_filter = Control.MOUSE_FILTER_PASS; _map_panel.add_child(_map_area)
	_map_area.gui_input.connect(_map_input)
	_map_area.resized.connect(_layout_map)
	var table := TextureRect.new(); table.name = "Table"; table.texture = load("res://assets/battle/board_background.png")
	table.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; table.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; table.modulate = Color(0.85, 0.85, 0.9)
	table.mouse_filter = Control.MOUSE_FILTER_IGNORE; table.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _map_area.add_child(table)
	_chart = Control.new(); _chart.name = "Chart"; _chart.mouse_filter = Control.MOUSE_FILTER_PASS; _chart.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _map_area.add_child(_chart)
	_sheet = SHEET.new(); _sheet.name = "Sheet"; _sheet.screen = self; _sheet.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _chart.add_child(_sheet)
	_map_canvas = Control.new(); _map_canvas.name = "MapCanvas"; _map_canvas.mouse_filter = Control.MOUSE_FILTER_PASS; _map_canvas.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _chart.add_child(_map_canvas)
	_lines = LINES.new(); _lines.name = "Lines"; _lines.screen = self; _lines.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _chart.add_child(_lines)
	# Passes clicks and drags on to the map area, so the chart still pans under it.
	_cartouche = Control.new(); _cartouche.name = "Cartouche"; _cartouche.mouse_filter = Control.MOUSE_FILTER_PASS; _chart.add_child(_cartouche)
	_cartouche.mouse_entered.connect(_hover_cartouche.bind(true)); _cartouche.mouse_exited.connect(_hover_cartouche.bind(false))
	_register("expedition.cartouche", _cartouche, "The calendar: month, season, Day or Night")
	_vignette = TextureRect.new(); _vignette.name = "Vignette"; _vignette.texture = CHART.vignette(false)
	_vignette.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; _vignette.stretch_mode = TextureRect.STRETCH_SCALE
	_vignette.mouse_filter = Control.MOUSE_FILTER_IGNORE; _vignette.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _map_area.add_child(_vignette)
	_marks = MARKS.new(); _marks.name = "Marks"; _marks.screen = self; _marks.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _map_area.add_child(_marks)
	_scrim = ColorRect.new(); _scrim.name = "Scrim"; _scrim.color = Color(0.02, 0.03, 0.04, 0.55); _scrim.mouse_filter = Control.MOUSE_FILTER_STOP
	_scrim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _scrim.visible = false; _map_area.add_child(_scrim)
	var hand_panel := PanelContainer.new(); hand_panel.add_theme_stylebox_override("panel", CHART.stone_box(Color(CHART.DARK_SURFACE, 0.92), CHART.BRONZE, 8, 6)); hand_panel.custom_minimum_size.y = 196; left.add_child(hand_panel)
	var hand_scroll := ScrollContainer.new(); hand_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; hand_panel.add_child(hand_scroll)
	_hand_row = HBoxContainer.new(); _hand_row.name = "Hand"; _hand_row.add_theme_constant_override("separation", 10); hand_scroll.add_child(_hand_row)
	# Right: the stone ledger with a pinned footer.
	var right := PanelContainer.new(); right.add_theme_stylebox_override("panel", CHART.stone_box(CHART.STONE, CHART.STONE_KEYLINE, 10, 6)); right.custom_minimum_size.x = 390; right.size_flags_vertical = Control.SIZE_EXPAND_FILL; columns.add_child(right)
	var stone := TextureRect.new(); stone.name = "Stone"; stone.texture = load("res://assets/battle/board_background.png")
	stone.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; stone.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; stone.modulate = Color(0.5, 0.5, 0.55)
	stone.mouse_filter = Control.MOUSE_FILTER_IGNORE; right.add_child(stone)
	var ledger := VBoxContainer.new(); ledger.name = "Ledger"; ledger.add_theme_constant_override("separation", 6); ledger.theme = CHART.ledger_theme(); right.add_child(ledger)
	var top := HBoxContainer.new(); top.name = "LedgerTop"; top.add_theme_constant_override("separation", 6); ledger.add_child(top)
	_save_label = STYLE.label(top, "", 15, CINEMATIC.IVORY); CHART.stone_text(_save_label, true)
	_save_label.autowrap_mode = TextServer.AUTOWRAP_OFF; _save_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; _save_label.clip_text = true
	_save_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _save_label.mouse_filter = Control.MOUSE_FILTER_STOP
	_all_saves = _stone_button(top, "Campaigns", _show_saves, "expedition.all_campaigns"); _all_saves.tooltip_text = "All campaigns"
	_back = _stone_button(top, "Menu", _back_to_menu, "expedition.back"); _back.tooltip_text = "Back to the main menu"
	for button in [_all_saves, _back]: button.add_theme_font_size_override("font_size", 14)
	_message = STYLE.label(ledger, "", 14, CHART.RED_ON_STONE); CHART.stone_text(_message, false, CHART.RED_ON_STONE); _message.visible = false
	_hud_scroll = ScrollContainer.new(); _hud_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; _hud_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; ledger.add_child(_hud_scroll)
	_hud = VBoxContainer.new(); _hud.name = "HUD"; _hud.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _hud.add_theme_constant_override("separation", 8); _hud_scroll.add_child(_hud)
	_footer = HBoxContainer.new(); _footer.name = "Footer"; _footer.custom_minimum_size.y = 48; _footer.add_theme_constant_override("separation", 6); ledger.add_child(_footer)
	# Centre overlay: a bone-framed scene with a painting and a parchment sheet.
	_overlay = PanelContainer.new(); _overlay.name = "Overlay"; _overlay.add_theme_stylebox_override("panel", CINEMATIC.bone_panel(Color.WHITE, 10)); _overlay.visible = false
	_overlay.clip_contents = true; _overlay.theme = CHART.ledger_theme()
	add_child(_overlay)
	var backdrop := Control.new(); backdrop.name = "Backdrop"; backdrop.mouse_filter = Control.MOUSE_FILTER_IGNORE; _overlay.add_child(backdrop)
	_overlay_backdrop = TextureRect.new(); _overlay_backdrop.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; _overlay_backdrop.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	_overlay_backdrop.mouse_filter = Control.MOUSE_FILTER_IGNORE; _overlay_backdrop.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); backdrop.add_child(_overlay_backdrop)
	var overlay_vignette := TextureRect.new(); overlay_vignette.texture = CHART.vignette(true); overlay_vignette.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; overlay_vignette.stretch_mode = TextureRect.STRETCH_SCALE
	overlay_vignette.mouse_filter = Control.MOUSE_FILTER_IGNORE; overlay_vignette.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); backdrop.add_child(overlay_vignette)
	var sheet_margin := MarginContainer.new(); sheet_margin.mouse_filter = Control.MOUSE_FILTER_IGNORE
	for side in ["left", "right", "top", "bottom"]: sheet_margin.add_theme_constant_override("margin_" + side, 24)
	_overlay.add_child(sheet_margin)
	_overlay_sheet = PanelContainer.new(); _overlay_sheet.name = "Sheet"
	var paper := CINEMATIC.paper()
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]: paper.set_content_margin(side, 18)
	_overlay_sheet.add_theme_stylebox_override("panel", paper)
	_overlay_sheet.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; _overlay_sheet.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	sheet_margin.add_child(_overlay_sheet)
	var sheet_box := VBoxContainer.new(); sheet_box.add_theme_constant_override("separation", 8); _overlay_sheet.add_child(sheet_box)
	_overlay_scroll = ScrollContainer.new(); _overlay_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; _overlay_scroll.custom_minimum_size = Vector2(560, 120); sheet_box.add_child(_overlay_scroll)
	_overlay_body = VBoxContainer.new(); _overlay_body.add_theme_constant_override("separation", 8); _overlay_body.custom_minimum_size.x = 540; _overlay_body.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _overlay_scroll.add_child(_overlay_body)
	# The sheet's actions sit under the scrolling body so a long camp (many
	# wounds, a heal log) never scrolls Break camp out of reach.
	_overlay_footer = VBoxContainer.new(); _overlay_footer.add_theme_constant_override("separation", 8); _overlay_footer.size_flags_horizontal = Control.SIZE_EXPAND_FILL; sheet_box.add_child(_overlay_footer)
	_heal_feedback = HEAL_FEEDBACK.new(); _heal_feedback.name = "HealFeedback"; add_child(_heal_feedback)

## The battle's dark gold-edged button, for the stone.
func _stone_button(parent: Node, text: String, callback: Callable, control_id: String) -> Button:
	var button := _button(parent, text, callback, control_id)
	button.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	button.add_theme_font_override("font", CHART.serif()); button.add_theme_font_size_override("font_size", 16)
	button.add_theme_stylebox_override("normal", CINEMATIC.panel(CHART.DARK_SURFACE, CINEMATIC.GOLD))
	button.add_theme_stylebox_override("hover", CINEMATIC.panel(Color("34372eea"), Color("e5c47b")))
	button.add_theme_stylebox_override("pressed", CINEMATIC.panel(Color("4a4230f2"), Color("f4d187")))
	button.add_theme_stylebox_override("disabled", CINEMATIC.panel(CHART.DARK_SURFACE, Color("625d4c")))
	button.add_theme_color_override("font_color", CINEMATIC.IVORY)
	return button

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
	var heals_before: int = _list(view.get("camp", {}) if view.get("camp") is Dictionary else {}, "heals").size()
	var response: Dictionary = runtime.expedition_act(save_id, action, int(status.get("revision", 0)))
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The action was refused.")))
		refresh()
		return false
	_set_message("")
	selected_cards.clear(); energy_payment = 0; use_siege = false; selected_hex = NONE
	_apply(response.result)
	if str(action.get("kind", "")) == "camp_service" and str(action.get("service", "")) in ["basic_aid", "bandage"]:
		_show_heals(_list(view.get("camp", {}) if view.get("camp") is Dictionary else {}, "heals").slice(heals_before))
	return true

## Plays the dice-roll flourish for the authority's new healing rolls, over the
## camp sheet, with the healed cards flying to the draw pile counter.
func _show_heals(heals: Array) -> void:
	if heals.is_empty(): return
	await get_tree().process_frame
	var target_cell: Control = _hud.find_child("Pile_draw", true, false)
	var target: Vector2 = target_cell.get_global_rect().get_center() if target_cell != null else get_viewport_rect().size * Vector2(0.9, 0.15)
	var area: Rect2 = _overlay_sheet.get_global_rect() if _overlay.visible else _map_area.get_global_rect()
	_heal_feedback.play(heals, area, target, view.get("card_names", {}))

## The ledger's message line shows only while it has something to say.
func _set_message(text: String) -> void:
	_message.text = text
	_message.visible = not text.is_empty()

## The chart cartouche's day line: the authority's clock label and the round
## ("Late Summer · Day · 1/4"), or after the last round its label with the
## month ("The deadline (Midautumn)").
func chart_day_label() -> String:
	var clock := _clock()
	var round := int(view.get("round", 0)); var rounds := int(view.get("rounds", 0))
	if rounds <= 0 or round > rounds:
		var month := str(clock.get("month_name", ""))
		return "%s (%s)" % [str(view.get("day_label", "")), month] if not month.is_empty() else str(view.get("day_label", ""))
	return "%s · %d/%d" % [str(clock.get("label", "")), round, rounds]

## The view's world clock (month, season, Day/Night, year), as the authority sent it.
func _clock() -> Dictionary:
	return view.get("clock") if view.get("clock") is Dictionary else {}

## The cartouche's hover: the month and Day/Night, the season and year,
## today's sight and terrain costs (the legend's numbers), and the season's
## rules and effects. Every value is the authority's; nothing is computed here.
func cartouche_lines() -> Array:
	var clock := _clock()
	if clock.is_empty(): return []
	var night: bool = clock.get("night", false)
	var half := "Night" if night else "Day"
	var lines: Array = [str(clock.get("label", "")), "%s · Year %d" % [str(clock.get("season_name", "")), int(clock.get("year", 0))]]
	lines.append("%s · sight %d" % [half, int(view.get("turn", {}).get("sight", 0))])
	var terrains: Dictionary = view.get("config", {}).get("terrain", {})
	var costs: Array = []
	var order: Array = TERRAIN_ORDER.duplicate()
	for key in terrains:
		if key not in order: order.append(key)
	for terrain in order:
		if not terrains.has(terrain): continue
		var config: Dictionary = terrains[terrain]
		costs.append("%s %s" % [str(terrain).capitalize(), "✕" if config.get("impassable", false) else str(int(config.get("night" if night else "day", 0)))])
	if not costs.is_empty(): lines.append("%s Move: %s" % [half, " · ".join(costs)])
	lines.append("%s: %s" % [str(clock.get("season_name", "")), str(clock.get("season_rules", ""))])
	var effects: Array = _list(clock, "season_effects")
	lines.append("Seasonal effects: " + ("none yet" if effects.is_empty() else " · ".join(effects)))
	return lines

func _hover_cartouche(on: bool) -> void:
	if map_model.is_empty(): return
	map_model["cartouche_hover"] = on
	map_model["cartouche_lines"] = cartouche_lines() if on else []
	_marks.wake(); _marks.queue_redraw()

func _show_error(text: String) -> void:
	_set_message(text)
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
	_save_label.text = str(status.get("save_name", "Expedition"))
	_save_label.tooltip_text = "%s · %s · %s" % [str(status.get("save_name", "Expedition")), str(view.get("region_name", "")), str(view.get("day_label", ""))]
	var night: bool = view.get("night", false)
	_chart.modulate = CHART.NIGHT_GRADE if night else Color.WHITE
	_vignette.texture = CHART.vignette(night)
	_render_map()
	_render_hand()
	_render_hud()
	_render_overlay()
	_scrim.visible = _overlay.visible
	map_model["overlay_open"] = _overlay.visible
	_marks.queue_redraw()

## Pointy-top axial → canvas pixels.
func _hex_center(q: int, r: int) -> Vector2:
	return _map_origin + _pan + Vector2(_hex_radius * sqrt(3.0) * (q + r * 0.5), _hex_radius * 1.5 * r)

func _layout_map() -> void:
	if view.is_empty() or _map_area == null: return
	# The radius fits the region's whole footprint (placed or not) into the map area.
	var min_r := 999; var max_r := -999
	for slot in _list(view, "slots"):
		for offset in HEX_OFFSETS:
			var rr: int = int(slot.center.r) + offset.y
			min_r = mini(min_r, rr); max_r = maxi(max_r, rr)
	var rows := maxi(1, max_r - min_r + 1)
	_hex_radius = clampf(minf(_map_area.size.x / 25.0, _map_area.size.y / (rows * 1.5 + 2.0)), 28.0, 64.0)
	# Centre the footprint in the map area.
	var lo := Vector2(INF, INF); var hi := Vector2(-INF, -INF)
	_map_origin = Vector2.ZERO; var pan := _pan; _pan = Vector2.ZERO
	for slot in _list(view, "slots"):
		for offset in HEX_OFFSETS:
			var c := _hex_center(int(slot.center.q) + offset.x, int(slot.center.r) + offset.y)
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
	_sheet.layout()
	_cartouche.position = _sheet.cartouche_rect.position; _cartouche.size = _sheet.cartouche_rect.size
	_sheet.queue_redraw(); _lines.queue_redraw(); _marks.queue_redraw()

func _render_map() -> void:
	for child in _map_canvas.get_children(): _map_canvas.remove_child(child); child.queue_free()
	_hex_buttons.clear()
	var moves := {}
	for move in _list(view, "moves"): moves["%d,%d" % [int(move.to.q), int(move.to.r)]] = move
	var explores := {}
	for slot in _list(view, "explores"): explores[str(slot.id)] = slot
	var targets := {}
	for target in _list(view, "challenges") + _list(view, "enters"): targets["%d,%d" % [int(target.hex.q), int(target.hex.r)]] = target
	var model := {"night": view.get("night", false), "phase": str(view.get("phase", "")), "survey": _survey_key or _survey_locked, "legend_filter": _legend_filter,
		"hover": "", "selected": "%d,%d" % [selected_hex.x, selected_hex.y] if selected_hex != NONE else "", "hexes": {}, "slots": {}, "moves": moves, "targets": targets,
		"warlord": view.get("warlord", {}), "marks": _list(view, "marks"), "sight": [], "overlay_open": false, "hover_lines": [], "warlord_cell": NONE,
		"cartouche_hover": map_model.get("cartouche_hover", false), "cartouche_lines": cartouche_lines() if map_model.get("cartouche_hover", false) else []}
	var player := Vector2i(int(view.get("player", {}).get("q", 0)), int(view.get("player", {}).get("r", 0)))
	model["player"] = player
	var slot_of := {}
	# Unexplored slots first (the line layer draws their wedges), then placed hexes.
	for slot in _list(view, "slots"):
		var center := Vector2i(int(slot.center.q), int(slot.center.r))
		var cells: Array = []
		for offset in HEX_OFFSETS: cells.append(center + offset)
		var explore: Dictionary = explores.get(str(slot.id), {})
		model.slots[str(slot.id)] = {"kind": str(slot.kind), "center": center, "placed": slot.get("placed", false), "adjacent": not explore.is_empty(), "ready": explore.get("ready", false), "why": str(explore.get("why", "")), "explore_cost": int(view.get("config", {}).get("explore_cost", 2)), "hexes": cells}
		for cell in cells: slot_of["%d,%d" % [cell.x, cell.y]] = str(slot.id)
		if slot.get("placed", false): continue
		for cell in cells:
			var button = _hex_button(cell.x, cell.y, {"q": cell.x, "r": cell.y, "terrain": ""}, "unexplored")
			button.slot_kind = str(slot.kind)
			button.set_meta("slot", str(slot.id))
			if not explore.is_empty(): button.highlight = "explore" if explore.get("ready", false) else "blocked"
	for hex in _list(view, "hexes"):
		var q: int = int(hex.q); var r: int = int(hex.r)
		var state := "unknown"
		if hex.get("in_sight", false): state = "in_sight"
		elif hex.get("seen", false): state = "remembered"
		var button = _hex_button(q, r, hex, state)
		var key := "%d,%d" % [q, r]
		var highlight := ""
		if moves.has(key):
			var move: Dictionary = moves[key]
			if not str(move.get("blocked", "")).is_empty(): highlight = "blocked"
			elif not str(move.get("provokes", "")).is_empty(): highlight = "provoke"
			else: highlight = "move"
		if targets.has(key): highlight = "target"
		button.highlight = highlight
		button.warlord = bool(hex.get("warlord", false))
		button.selected = selected_hex == Vector2i(q, r)
		# The hover cartouche on the chart is the hex's only tooltip.
		button.tooltip_text = ""
		model.hexes[key] = {"data": hex, "state": state, "highlight": highlight, "slot": slot_of.get(key, ""), "cell": Vector2i(q, r)}
		if hex.get("in_sight", false): model.sight.append(Vector2i(q, r))
		if hex.get("warlord", false): model["warlord_cell"] = Vector2i(q, r)
	if model.warlord_cell == NONE and view.get("warlord", {}).get("last_seen") != null:
		model["warlord_cell"] = Vector2i(int(view.warlord.last_seen.q), int(view.warlord.last_seen.r))
	map_model = model
	if _last_player != NONE and _last_player != player and view.get("phase", "") == "turn": _marks.start_slide(_last_player)
	_last_player = player
	_layout_map()

func _hex_button(q: int, r: int, data: Dictionary, state: String):
	var button = HEX.new()
	button.name = "Hex_%d_%d" % [q, r]
	button.data = data
	button.state = state
	button.screen = self
	button.pressed.connect(_select_hex.bind(Vector2i(q, r)))
	var key := "%d,%d" % [q, r]
	button.mouse_entered.connect(_hover_hex.bind(key))
	button.mouse_exited.connect(_hover_hex.bind(""))
	_map_canvas.add_child(button)
	_hex_buttons[key] = button
	_register("expedition.hex.%d.%d" % [q, r], button, "Select hex (%d,%d)" % [q, r])
	return button

func _hover_hex(key: String) -> void:
	if map_model.is_empty(): return
	map_model["hover"] = key
	map_model["hover_lines"] = _hover_lines(key)
	_marks.wake(); _marks.queue_redraw(); _lines.queue_redraw()

## The hover cartouche: the hex, its terrain and cost, what stands there, and
## what a move there would mean.
func _hover_lines(key: String) -> Array:
	if key.is_empty(): return []
	if map_model.hexes.has(key):
		var hex: Dictionary = map_model.hexes[key].data
		var lines: Array = ["(%d,%d) · %s" % [int(hex.q), int(hex.r), str(hex.get("tile_name", ""))], _terrain_line(hex)]
		if not hex.get("seen", false): lines.append("Contents unknown")
		elif not hex.get("in_sight", false): lines.append("Out of sight · remembered")
		if hex.get("site") != null:
			var site: Dictionary = hex.site
			var marks: Array = []
			if site.get("defended", false): marks.append("held by %s" % site.get("defender_name", "enemies"))
			if site.get("fortified", false): marks.append("Fortified")
			if site.get("conquered", false): marks.append("conquered")
			if site.get("goal", false): marks.append("the Warlord's goal")
			lines.append("%s (%s)%s" % [site.name, site.type, " · " + " · ".join(marks) if not marks.is_empty() else ""])
		if hex.get("group") != null:
			var group: Dictionary = hex.group
			lines.append("%s ×%d%s%s" % [group.name, int(group.count), " · last seen here" if group.get("remembered", false) else "", " · Ambush possible" if group.get("ambushable", false) else ""])
		if hex.get("warlord", false): lines.append("The Warlord · %s" % view.get("warlord", {}).get("name", ""))
		var move: Dictionary = map_model.moves.get(key, {})
		if not move.is_empty():
			if not str(move.get("blocked", "")).is_empty(): lines.append("Cannot move: %s" % move.blocked)
			else: lines.append("Move here · %d Move%s" % [int(move.cost), " · provokes %s!" % move.provokes if not str(move.get("provokes", "")).is_empty() else ""])
		return lines
	var slot_id: String = ""
	for id in map_model.slots:
		for cell in map_model.slots[id].hexes:
			if "%d,%d" % [cell.x, cell.y] == key: slot_id = id
	if slot_id.is_empty(): return []
	var slot: Dictionary = map_model.slots[slot_id]
	var why := ""
	if slot.adjacent and not slot.ready: why = " · " + str(slot.why)
	elif not slot.adjacent: why = " · stand next to it"
	return ["Unexplored %s" % slot.kind, "Explore · %d Move%s" % [slot.explore_cost, why]]

func _terrain_line(hex: Dictionary) -> String:
	var terrain := str(hex.get("terrain", ""))
	var config: Dictionary = view.get("config", {}).get("terrain", {}).get(terrain, {})
	if config.get("impassable", false): return "%s · impassable" % terrain.capitalize()
	return "%s · Day %d / Night %d Move%s" % [terrain.capitalize(), int(config.get("day", 0)), int(config.get("night", 0)), " · cover" if config.get("cover", false) else ""]

func _hex_tooltip(hex: Dictionary, move: Dictionary) -> String:
	var lines: Array = ["(%d,%d) · %s" % [int(hex.q), int(hex.r), str(hex.get("tile_name", ""))]]
	lines.append(_terrain_line(hex))
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
		if site.get("has_deck_change", false): lines.append("· Deck changes: %d Influence opens the deck for the visit" % int(site.get("deck_change", 0)))
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

func _unhandled_key_input(event: InputEvent) -> void:
	if event is InputEventKey and event.keycode == KEY_TAB and not event.echo:
		_survey_key = event.pressed
		_set_survey()
		get_viewport().set_input_as_handled()

func _set_survey() -> void:
	if map_model.is_empty(): return
	map_model["survey"] = _survey_key or _survey_locked
	_marks.queue_redraw()

func _select_hex(hex: Vector2i) -> void:
	selected_hex = NONE if selected_hex == hex else hex
	for key in _hex_buttons: _hex_buttons[key].selected = key == "%d,%d" % [selected_hex.x, selected_hex.y]
	if not map_model.is_empty(): map_model["selected"] = "%d,%d" % [selected_hex.x, selected_hex.y] if selected_hex != NONE else ""
	_render_hud()
	_marks.wake(); _marks.queue_redraw()

# The hand --------------------------------------------------------------------
func _render_hand() -> void:
	for child in _hand_row.get_children(): _hand_row.remove_child(child); child.queue_free()
	var hand: Array = _list(view, "hand")
	var count := STYLE.label(_hand_row, "Hand\n%d / %d" % [hand.size(), int(view.get("hand_size", 4))], 13, CHART.HUD_IVORY); CHART.stone_text(count); count.custom_minimum_size.x = 56
	for index in hand.size():
		var card: Dictionary = hand[index]
		var is_selected := index in selected_cards
		var lift := MarginContainer.new(); lift.add_theme_constant_override("margin_top", 0 if is_selected else 6); _hand_row.add_child(lift)
		var column := VBoxContainer.new(); column.add_theme_constant_override("separation", 4); lift.add_child(column)
		var frame := PanelContainer.new()
		var glow := StyleBoxFlat.new(); glow.bg_color = Color(0, 0, 0, 0); glow.set_corner_radius_all(6)
		if is_selected:
			glow.border_color = CHART.GOLD_BRIGHT; glow.set_border_width_all(2); glow.shadow_color = Color(CHART.GOLD_BRIGHT, 0.35); glow.shadow_size = 6
		frame.add_theme_stylebox_override("panel", glow); frame.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; column.add_child(frame)
		var face := BattleCardScene.new(); frame.add_child(face)
		face.configure("hand-%d" % index, str(card.id), true, false, true)
		face.mouse_filter = Control.MOUSE_FILTER_IGNORE; face.focus_mode = Control.FOCUS_NONE
		var tag_text := "1 Move / 1 Influence"; var tag_color := CHART.INK
		if card.get("map_only", false): tag_text = "map only"; tag_color = CHART.GOLD_PAPER
		elif not str(card.get("map_effect", "")).is_empty(): tag_text = "map: " + str(card.get("map_effect", "")); tag_color = CHART.GREEN
		var tag := CHART.ink_label(column, tag_text, 11, tag_color, false, false); tag.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		tag.add_theme_stylebox_override("normal", CHART.chip_style())
		var pay := CHART.paper_button(_button(column, "Selected" if is_selected else "Select", _toggle_card.bind(index), "expedition.card.%d" % index), 14)
		pay.toggle_mode = true; pay.button_pressed = is_selected
		if is_selected: CHART.rim(pay, CHART.GOLD_LEAF)
		pay.tooltip_text = "%s\n\nSelect it to spend it: 1 Move for walking or exploring, 1 Influence at a site, or discard it." % BattlePresentationCatalog.card_tooltip(str(card.id))
		if not str(card.get("map_effect", "")).is_empty():
			var play := CHART.paper_button(_button(column, "Play on map" if int(card.get("energy_cost", 0)) == 0 else "Play (%d energy)" % int(card.energy_cost), func(): pass, "expedition.play.%d" % index), 14)
			play.pressed.connect(_play_map_card.bind(card, play))
			play.disabled = not card.get("map_playable", false)
			if not play.disabled: CHART.rim(play, CHART.GOLD_LEAF)
			play.tooltip_text = str(card.get("map_reason", "")) if play.disabled else "Play %s for its map effect" % card.name

## Plays a card for its map effect. A card that needs a target (a terrain, an
## enemy to mark, a site to lure from) offers the authority's list first.
func _play_map_card(card: Dictionary, anchor: Control) -> void:
	var targets: Array = _list(card, "targets")
	if targets.is_empty():
		_act({"kind": "play_map_card", "card": str(card.id)})
		return
	var field: String = {"mark": "group", "lure": "site"}.get(str(card.get("map_kind", "")), "choice")
	var menu := PopupMenu.new(); menu.name = "MapTargets"; add_child(menu)
	for i in targets.size(): menu.add_item(str(targets[i].label), i)
	menu.id_pressed.connect(func(i: int): _act({"kind": "play_map_card", "card": str(card.id), field: str(targets[i].id)}))
	menu.popup_hide.connect(func(): menu.queue_free.call_deferred())
	var at := anchor.get_global_rect()
	menu.popup(Rect2i(Vector2i(at.position) - Vector2i(0, 26 * targets.size() + 12), Vector2i(maxi(240, int(at.size.x)), 0)))

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

# The ledger ------------------------------------------------------------------
func _render_hud() -> void:
	for child in _hud.get_children(): _hud.remove_child(child); child.queue_free()
	for child in _footer.get_children(): _footer.remove_child(child); child.queue_free()
	var turn: Dictionary = view.get("turn", {})
	var phase := str(view.get("phase", ""))
	_render_stat_strip()
	_render_warlord_line()
	_render_turn_line(turn)
	if phase == "turn": _render_payment(turn)
	_render_legend()
	_render_selected(phase, turn)
	if view.get("site_here") != null and phase == "turn": _render_site(view.site_here)
	_render_journal()
	# Footer.
	if phase == "turn":
		var end := CHART.paper_button(_button(_footer, "End turn", func(): _act({"kind": "end_turn"}), "expedition.end_turn"), 16, true)
		end.size_flags_horizontal = Control.SIZE_EXPAND_FILL; end.icon = CHART.glyph("wax_seal", "wax"); end.add_theme_constant_override("icon_max_width", 16)
		end.disabled = not turn.get("can_end_turn", false)
		if not end.disabled: CHART.rim(end, CHART.GOLD_LEAF, 2.5)
		end.tooltip_text = str(turn.get("end_turn_reason", "Keep your unplayed cards and draw up to your hand size"))
		var camp := CHART.paper_button(_button(_footer, "Make camp", func(): _act({"kind": "make_camp"}), "expedition.make_camp"), 15)
		camp.icon = CHART.glyph("tent"); camp.add_theme_constant_override("icon_max_width", 16)
		camp.tooltip_text = "Rest now: forfeit the cards in hand, reshuffle Spent, reset energy, camp services, then the Warlord marches and Day/Night flips"
	elif phase == "victory" or phase == "defeat":
		var ended := STYLE.label(_footer, "The region is won" if phase == "victory" else "The run is over", 16, Color("ffe2a0") if phase == "victory" else CHART.WOUNDED)
		CHART.stone_text(ended, true, Color("ffe2a0") if phase == "victory" else CHART.WOUNDED); ended.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		ended.name = "Ended"

func _render_stat_strip() -> void:
	var outer := PanelContainer.new(); outer.add_theme_stylebox_override("panel", CINEMATIC.panel(CHART.DARK_SURFACE, CHART.BRONZE, 0)); _hud.add_child(outer)
	var frame := PanelContainer.new(); frame.add_theme_stylebox_override("panel", CINEMATIC.bone_panel(Color.WHITE, 10)); outer.add_child(frame)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 4); frame.add_child(box)
	var tiles := HBoxContainer.new(); tiles.add_theme_constant_override("separation", 6); box.add_child(tiles)
	_stat_tile(tiles, "%d" % int(view.get("energy", 0)), "Energy", "base %d" % int(view.get("base_energy", 0)), null)
	_stat_tile(tiles, "%d" % int(view.get("gold", 0)), "Gold", "", CHART.glyph("coin", "gold"))
	# XP to spend, and the authority's total: everything the hero has, spent and unspent.
	var xp_tile := _stat_tile(tiles, "%d" % int(status.get("xp", 0)), "XP", "", ICONS.texture("energy"))
	xp_tile.name = "XPTile"; xp_tile.mouse_filter = Control.MOUSE_FILTER_STOP
	_register("expedition.stat.xp", xp_tile, "XP")
	xp_tile.tooltip_text = "XP to spend: %d · Total XP: %d\nTotal XP is everything you have: XP to spend plus the XP in your cards, wounded cards and upgrades. Selling or trading down gives XP back; only rewards raise the total." % [int(status.get("xp", 0)), int(status.get("xp_total", 0))]
	# Influence: the cards played at this site (or camp) this turn. It is lost
	# when the turn ends or you leave, so the strip says where it counts.
	var turn: Dictionary = view.get("turn", {})
	var influence := int(turn.get("influence", 0))
	var where := str(turn.get("influence_site", ""))
	var place := "camp" if where == "camp" else (_site_name(where) if not where.is_empty() else "")
	var tile := _stat_tile(tiles, "%d" % influence, "Influence", place if influence > 0 else "this turn", CHART.glyph("crown", "gold") if CHART.SHAPES.has("crown") else null, Color("9fe0a8") if influence > 0 else Color.TRANSPARENT)
	tile.name = "InfluenceTile"; tile.mouse_filter = Control.MOUSE_FILTER_STOP
	tile.tooltip_text = ("%d Influence at %s this turn: spend it there on deck changes, mending or services. It is lost when the turn ends or you leave." % [influence, place]) if influence > 0 else "Influence: play cards at a site (or at camp) for 1 Influence each. It lasts for this turn at that place."
	var health := VBoxContainer.new(); health.size_flags_horizontal = Control.SIZE_EXPAND_FILL; health.alignment = BoxContainer.ALIGNMENT_CENTER; health.add_theme_constant_override("separation", 2); tiles.add_child(health)
	var bar := ProgressBar.new(); bar.custom_minimum_size = Vector2(0, 22); bar.max_value = maxi(1, int(view.get("max_health", 0))); bar.value = int(view.get("health", 0)); bar.show_percentage = false
	bar.add_theme_stylebox_override("background", CINEMATIC.bone_health_frame())
	var fill := StyleBoxFlat.new(); fill.bg_color = CHART.HEALTH_FILL; fill.border_color = CHART.HEALTH_EDGE; fill.set_border_width_all(2); fill.set_corner_radius_all(4)
	fill.expand_margin_left = -5; fill.expand_margin_right = -5; fill.expand_margin_top = -3; fill.expand_margin_bottom = -3
	bar.add_theme_stylebox_override("fill", fill); health.add_child(bar)
	var value := Label.new(); value.text = "%d/%d" % [int(view.get("health", 0)), int(view.get("max_health", 0))]; value.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; value.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	value.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); value.add_theme_font_override("font", CHART.numerals()); value.add_theme_font_size_override("font_size", 16); CHART.stone_text(value, true); value.mouse_filter = Control.MOUSE_FILTER_IGNORE; bar.add_child(value)
	var caption := STYLE.label(health, "HEALTH", 12, STYLE.GOLD); caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; caption.autowrap_mode = TextServer.AUTOWRAP_OFF
	# Counts with hover lists.
	var counts := HBoxContainer.new(); counts.add_theme_constant_override("separation", 12); counts.alignment = BoxContainer.ALIGNMENT_CENTER; box.add_child(counts)
	for pile in [["Draw", "draw", "deck"], ["Spent", "spent", "discard"], ["Wounds", "wounds", "removed"]]:
		var ids: Array = _list(view, pile[1])
		var cell := HBoxContainer.new(); cell.name = "Pile_" + pile[1]; cell.mouse_filter = Control.MOUSE_FILTER_STOP; cell.add_theme_constant_override("separation", 4); counts.add_child(cell)
		var text := "%s %d" % [pile[0], ids.size()]
		if pile[1] == "wounds":
			# A wound is one grouping of damage; the health lost is its cards.
			var lost: Array = _list(view, "wound_cards")
			if ids.size() > 0: text += " · %d lost" % lost.size()
			cell.tooltip_text = _wounds_tooltip(ids, lost)
		else:
			cell.tooltip_text = "%s (%d)%s" % [pile[0], ids.size(), "\n" + _card_names(ids) if not ids.is_empty() else ""]
		CHART.icon_rect(cell, ICONS.texture(pile[2]), 18)
		var label := STYLE.label(cell, text, 14, CHART.HUD_IVORY); label.autowrap_mode = TextServer.AUTOWRAP_OFF
		CHART.stone_text(label, false, CHART.WOUNDED if pile[1] == "wounds" and ids.size() > 0 else CHART.HUD_IVORY)
	# Statuses and inventory.
	var statuses: Array = _list(view, "statuses")
	var items: Array = _list(view, "inventory")
	if not statuses.is_empty() or not items.is_empty():
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); row.alignment = BoxContainer.ALIGNMENT_CENTER; box.add_child(row)
		for st in statuses: _chip(row, "Status_" + str(st.id), str(st.id), "%s ×%d" % [st.name, int(st.stacks)], "%s\n%s" % [st.name, st.rules], CHART.STATUS)
		for item in items: _chip(row, "Item_" + str(item.id), "", "%s ×%d" % [item.name, int(item.count)], "%s\n%s" % [item.name, item.description], CHART.ITEM)

func _stat_tile(parent: Node, value: String, caption: String, sub: String, icon: Texture2D, color: Color = Color.TRANSPARENT) -> VBoxContainer:
	var cell := VBoxContainer.new(); cell.size_flags_horizontal = Control.SIZE_EXPAND_FILL; cell.alignment = BoxContainer.ALIGNMENT_CENTER; cell.add_theme_constant_override("separation", 0); parent.add_child(cell)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 4); cell.add_child(row)
	if icon != null: CHART.icon_rect(row, icon, 16)
	var number := Label.new(); number.text = value; number.add_theme_font_override("font", CHART.numerals()); number.add_theme_font_size_override("font_size", 26); CHART.stone_text(number, true, color if color.a > 0 else CHART.HUD_IVORY); number.mouse_filter = Control.MOUSE_FILTER_IGNORE; row.add_child(number)
	var cap := STYLE.label(cell, caption.to_upper(), 12, STYLE.GOLD); cap.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; cap.autowrap_mode = TextServer.AUTOWRAP_OFF
	if not sub.is_empty():
		var small := STYLE.label(cell, sub, 12, Color("9c9d97")); small.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; small.autowrap_mode = TextServer.AUTOWRAP_OFF
		small.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; small.clip_text = true; small.custom_minimum_size.x = 56
	return cell

## Card names from the view's catalog, duplicates grouped as "2 × Steady Guard".
## A site's display name from the view, or a tidy version of its id.
func _site_name(id: String) -> String:
	for hex in _list(view, "hexes"):
		var site = hex.get("site")
		if site is Dictionary and str(site.get("id", "")) == id: return str(site.get("name", id))
	return id.capitalize().replace("_", " ")

func _card_names(ids: Array) -> String:
	var counts := {}
	var order: Array = []
	for id in ids:
		var name := str(view.get("card_names", {}).get(id, id))
		if not counts.has(name): counts[name] = 0; order.append(name)
		counts[name] += 1
	return ", ".join(order.map(func(name): return name if counts[name] == 1 else "%d × %s" % [counts[name], name]))

## One block per wound grouping: its damage, source, when it was taken, its
## camp treatment, and the cards it holds.
func _wounds_tooltip(wounds: Array, lost: Array) -> String:
	if wounds.is_empty(): return "Wounds (0)"
	var blocks: Array = ["Wounds %d · %d health lost" % [wounds.size(), lost.size()]]
	for k in wounds.size():
		var w: Dictionary = wounds[k]
		var line := "Wound %d · %d damage (of %d) · %s · %s" % [k + 1, int(w.get("damage", 0)), int(w.get("size", 0)), str(w.get("source", "")), str(w.get("taken", ""))]
		if w.get("treated", false): line += " · treated"
		if w.get("bandaged", false): line += " · bandaged"
		blocks.append(line + "\n" + _card_names(_list(w, "cards")))
	return "\n".join(blocks)

## "untreated", "treated" or "treated · bandaged" for a wound at camp.
static func _wound_state(w: Dictionary) -> String:
	if w.get("bandaged", false): return "treated · bandaged"
	return "treated" if w.get("treated", false) else "untreated"

func _chip(parent: Node, node_name: String, icon_id: String, text: String, tooltip: String, color: Color) -> HBoxContainer:
	var cell := PanelContainer.new(); cell.name = node_name; cell.mouse_filter = Control.MOUSE_FILTER_STOP; cell.tooltip_text = tooltip
	var style := StyleBoxFlat.new(); style.bg_color = Color(color, 0.14); style.border_color = Color(color, 0.55); style.set_border_width_all(1); style.set_corner_radius_all(10)
	style.content_margin_left = 8; style.content_margin_right = 8; style.content_margin_top = 2; style.content_margin_bottom = 2
	cell.add_theme_stylebox_override("panel", style); parent.add_child(cell)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 4); cell.add_child(row)
	if not icon_id.is_empty(): CHART.icon_rect(row, ICONS.texture(icon_id), 16)
	var label := STYLE.label(row, text, 13, color.lightened(0.25)); label.autowrap_mode = TextServer.AUTOWRAP_OFF
	return row

func _render_warlord_line() -> void:
	var w: Dictionary = view.get("warlord", {})
	var text := "%s · " % w.get("name", "The Warlord")
	if w.get("defeated", false): text += "beaten"
	elif not w.get("known", false): text += "not yet seen · marching to %s" % w.get("goal_name", "")
	else: text += "%s at (%d,%d) · to %s%s" % ["in sight" if w.get("in_sight", false) else "last seen", int(w.last_seen.q) if w.get("last_seen") != null else 0, int(w.last_seen.r) if w.get("last_seen") != null else 0, w.get("goal_name", ""), " · goal taken" if w.get("goal_taken", false) else ""]
	text += " · Lieutenants %d" % int(w.get("lieutenants", 0))
	if w.get("fortified", false): text += " · FORTIFIED"
	if w.get("conqueror", false): text += " · CONQUEROR"
	var holder := PanelContainer.new(); _hud.add_child(holder)
	if w.get("in_sight", false):
		var style := StyleBoxFlat.new(); style.bg_color = Color(CHART.ENEMY_RIM, 0.18); style.border_color = CHART.ENEMY_RIM; style.set_border_width_all(1); style.set_corner_radius_all(4)
		style.content_margin_left = 6; style.content_margin_right = 6; style.content_margin_top = 6; style.content_margin_bottom = 6
		holder.add_theme_stylebox_override("panel", style)
	else: holder.add_theme_stylebox_override("panel", StyleBoxEmpty.new())
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); holder.add_child(row)
	CHART.icon_rect(row, ICONS.texture("conqueror"), 16).size_flags_vertical = Control.SIZE_SHRINK_BEGIN
	var label := STYLE.label(row, text, 14, CHART.RED_ON_STONE); CHART.stone_text(label, false, CHART.RED_ON_STONE); label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	label.name = "WarlordLine"
	# Hunter's Marks: where each marked enemy is now.
	for m in _list(view, "marks"):
		var at = m.get("hex")
		var where := "at (%d,%d)" % [int(at.q), int(at.r)] if at is Dictionary else ""
		if m.get("off_map", false): where += ", beyond the explored map"
		var mark := STYLE.label(_hud, "Marked: %s %s · %d rest%s left" % [m.name, where, int(m.rests_left), "" if int(m.rests_left) == 1 else "s"], 13, CHART.GOLD_PAPER)
		CHART.stone_text(mark, false, CHART.GOLD_PAPER); mark.name = "Mark_" + str(m.target).replace(":", "_")

func _render_turn_line(turn: Dictionary) -> void:
	CHART.gold_rule(_hud)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); _hud.add_child(row)
	CHART.icon_rect(row, CHART.glyph("moon" if view.get("night", false) else "sun", "ivory"), 16).size_flags_vertical = Control.SIZE_SHRINK_BEGIN
	var used := int(turn.get("cards_used", 0))
	var text := "Turn %d · Move %d · Played %d (need 1) · Sight %d" % [int(turn.get("number", 0)), int(turn.get("move", 0)), used, int(turn.get("sight", 1))]
	if int(turn.get("influence", 0)) > 0: text += " · Influence %d at %s" % [int(turn.influence), str(turn.get("influence_site", "")).capitalize().replace("_", " ")]
	if turn.get("fought", false): text += " · fought"
	if turn.get("action_used", false): text += " · action taken"
	var label := STYLE.label(row, text, 14, CHART.HUD_IVORY); CHART.stone_text(label); label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	label.name = "TurnLine"
	if used < 1 and not turn.get("fought", false): label.add_theme_color_override("font_color", CHART.RED_ON_STONE)
	# This turn's map card effects (cheap terrain, no provoking, ...).
	var effects: Array = _list(turn, "effects")
	if not effects.is_empty():
		var line := STYLE.label(_hud, "This turn: " + " · ".join(effects), 13, Color("9fe0a8")); CHART.stone_text(line, false, Color("9fe0a8"))
		line.name = "TurnEffects"
	CHART.gold_rule(_hud)

func _render_payment(turn: Dictionary) -> void:
	var card := CHART.paper_card(_hud)
	var per := int(view.get("config", {}).get("energy_per_move", 5))
	var moves_paid := selected_cards.size() + energy_payment / per
	var grid := GridContainer.new(); grid.columns = 2; grid.add_theme_constant_override("h_separation", 12); grid.add_theme_constant_override("v_separation", 4); card.add_child(grid)
	CHART.ink_label(grid, "Cards selected", 14, CHART.INK, false, false)
	var cards := CHART.numeral_label(grid, "%d" % selected_cards.size(), 15); cards.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT; cards.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	CHART.ink_label(grid, "Energy", 14, CHART.INK, false, false)
	var energy := CHART.numeral_label(grid, "%d  (= %d Move)" % [energy_payment, energy_payment / per], 15); energy.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT; energy.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	CHART.ink_rule(card); CHART.ink_rule(card)
	var total := GridContainer.new(); total.columns = 2; total.add_theme_constant_override("h_separation", 12); card.add_child(total)
	CHART.ink_label(total, "Move", 15, CHART.INK, true, false)
	var sum := CHART.numeral_label(total, "%d · pool %d" % [moves_paid, int(turn.get("move", 0))], 16); sum.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT; sum.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	sum.name = "PaymentTotal"
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); card.add_child(row)
	var minus := CHART.paper_button(_button(row, "−%d energy" % per, func(): energy_payment = maxi(0, energy_payment - per); _render(), "expedition.energy.minus"), 14)
	minus.disabled = energy_payment <= 0; minus.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var plus := CHART.paper_button(_button(row, "+%d energy" % per, func(): energy_payment = mini(int(view.get("energy", 0)) / per * per, energy_payment + per); _render(), "expedition.energy.plus"), 14)
	plus.disabled = energy_payment + per > int(view.get("energy", 0)); plus.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var discard := CHART.paper_button(_button(row, "Discard selected", func(): _act({"kind": "discard", "cards": _payment_cards()}), "expedition.discard"), 14)
	discard.disabled = selected_cards.is_empty(); discard.size_flags_horizontal = Control.SIZE_EXPAND_FILL

func _render_legend() -> void:
	var card := CHART.paper_card(_hud, 8)
	var toggle := CHART.paper_button(_button(card, "Legend", func(): _legend_open = not _legend_open; _render_hud(), "expedition.legend.toggle"), 13)
	toggle.icon = ICONS.texture("chevron_open" if _legend_open else "chevron_closed"); toggle.add_theme_constant_override("icon_max_width", 14); toggle.alignment = HORIZONTAL_ALIGNMENT_LEFT
	toggle.flat = true
	if not _legend_open: return
	var flow := HFlowContainer.new(); flow.add_theme_constant_override("h_separation", 4); flow.add_theme_constant_override("v_separation", 4); card.add_child(flow)
	var entries: Array = [["village", "Village"], ["monastery", "Monastery"], ["town", "Town"], ["ruins", "Ruins"], ["grotto", "Grotto"], ["event", "Event ?"], ["keep", "Keep"], ["tower", "Tower"], ["enemy", "Enemy group"], ["warlord", "The Warlord"], ["you", "You"], ["in_sight", "In sight"], ["remembered", "Remembered"], ["unknown", "Unknown"], ["countryside", "Countryside"], ["core", "Core"]]
	for entry in entries:
		var id: String = entry[0]
		var chip := CHART.paper_button(_button(flow, entry[1], func(): pass, "expedition.legend." + id), 13)
		chip.flat = true
		chip.add_theme_stylebox_override("normal", CHART.chip_style(Color(0, 0, 0, 0), Color(CHART.INK, 0.25), 6, 2))
		chip.add_theme_stylebox_override("hover", CHART.chip_style(Color("fff0cc"), Color(CHART.INK, 0.5), 6, 2))
		chip.icon = _legend_icon(id); chip.add_theme_constant_override("icon_max_width", 18)
		chip.mouse_entered.connect(func(): _legend_filter = id; map_model["legend_filter"] = id; _marks.queue_redraw(); _lines.queue_redraw())
		chip.mouse_exited.connect(func(): _legend_filter = ""; map_model["legend_filter"] = ""; _marks.queue_redraw(); _lines.queue_redraw())
	# The cost table.
	var terrains: Dictionary = view.get("config", {}).get("terrain", {})
	var grid := GridContainer.new(); grid.columns = 4; grid.add_theme_constant_override("h_separation", 10); grid.add_theme_constant_override("v_separation", 2); card.add_child(grid)
	CHART.caption(grid, ""); CHART.caption(grid, "Terrain"); CHART.caption(grid, "Day", CHART.GOLD_PAPER if view.get("night", false) else CHART.INK); CHART.caption(grid, "Night", CHART.INK if view.get("night", false) else CHART.NIGHT_BLUE)
	var order: Array = TERRAIN_ORDER.duplicate()
	for key in terrains:
		if key not in order: order.append(key)
	for terrain in order:
		if not terrains.has(terrain): continue
		var config: Dictionary = terrains[terrain]
		CHART.icon_rect(grid, CHART.glyph("motif_" + terrain), 16)
		CHART.ink_label(grid, str(terrain).capitalize() + (" · cover" if config.get("cover", false) else ""), 13, CHART.INK, false, false)
		if config.get("impassable", false):
			CHART.ink_label(grid, "✕", 13, CHART.INK, false, false); CHART.ink_label(grid, "✕", 13, CHART.INK, false, false)
		else:
			CHART.numeral_label(grid, "%d" % int(config.get("day", 0)), 13, CHART.INK)
			CHART.numeral_label(grid, "%d" % int(config.get("night", 0)), 13, CHART.NIGHT_BLUE)
	var survey := CHART.paper_button(_button(card, "Survey (hold Tab)", func(): _survey_locked = not _survey_locked; _set_survey(); _render_hud(), "expedition.survey"), 13)
	survey.toggle_mode = true; survey.button_pressed = _survey_locked; survey.flat = true
	if _survey_locked: CHART.rim(survey, CHART.GOLD_LEAF)

func _legend_icon(id: String) -> Texture2D:
	match id:
		"village", "monastery", "town", "ruins", "grotto", "keep", "tower": return CHART.glyph("site_" + id)
		"event": return null
		"enemy": return ICONS.texture("removed")
		"warlord": return ICONS.texture("conqueror")
		"you": return CHART.glyph("pawn", "gold")
		"in_sight": return CHART.glyph("sun")
		"remembered": return CHART.glyph("moon", "sepia")
		"unknown": return CHART.glyph("compass_diamond", "sepia")
		"countryside": return CHART.glyph("compass_diamond")
		"core": return CHART.glyph("compass_diamond", "red")
	return null

func _render_selected(phase: String, turn: Dictionary) -> void:
	if selected_hex == NONE or phase != "turn": return
	var card := CHART.paper_card(_hud)
	var hex_data: Dictionary = {}
	for hex in _list(view, "hexes"):
		if int(hex.q) == selected_hex.x and int(hex.r) == selected_hex.y: hex_data = hex
	var heading := CHART.ink_label(card, "(%d, %d) · %s" % [selected_hex.x, selected_hex.y, str(hex_data.get("tile_name", "Unexplored"))], 15, CHART.INK, true)
	heading.name = "SelectedHeading"
	CHART.gold_rule(card)
	if not hex_data.is_empty():
		CHART.ink_label(card, _terrain_line(hex_data), 13)
		if not hex_data.get("seen", false): CHART.ink_label(card, "Contents unknown", 12, CHART.SEPIA_TEXT)
		elif not hex_data.get("in_sight", false): CHART.ink_label(card, "Out of sight · remembered", 12, CHART.SEPIA_TEXT)
		var details := _hex_tooltip(hex_data, {}).split("\n")
		for i in range(2, details.size()):
			var line := str(details[i])
			if line.begins_with("Contents unknown") or line.begins_with("Out of sight"): continue
			var danger := line.begins_with("Held by") or line.begins_with("Ambush possible") or line.begins_with("Conquered")
			CHART.ink_label(card, line, 13, CHART.RUBRIC if danger else CHART.INK)
	for move in _list(view, "moves"):
		if int(move.to.q) != selected_hex.x or int(move.to.r) != selected_hex.y: continue
		var blocked := str(move.get("blocked", ""))
		var label := "Move here · %d Move" % int(move.cost) if blocked.is_empty() else "Cannot move: %s" % blocked
		if blocked.is_empty() and not str(move.get("provokes", "")).is_empty():
			label += " · provokes %s" % move.provokes
			CHART.ink_label(card, "Provokes %s: a fight starts at once" % move.provokes, 13, CHART.RUBRIC)
		var button := CHART.paper_button(_button(card, label, func(): _act({"kind": "move", "target": {"q": selected_hex.x, "r": selected_hex.y}, "cards": _payment_cards(), "energy": energy_payment}), "expedition.move"), 14, true)
		button.disabled = not blocked.is_empty()
		if not button.disabled: CHART.rim(button, CHART.GOLD_LEAF)
	for slot in _list(view, "explores"):
		var owns := false
		for offset in HEX_OFFSETS:
			if int(slot.center.q) + int(offset.x) == selected_hex.x and int(slot.center.r) + int(offset.y) == selected_hex.y: owns = true
		if not owns: continue
		var label := "Explore %s · %d Move" % [slot.id, int(view.config.explore_cost)]
		if not slot.get("ready", false): label += " (%s)" % slot.get("why", "")
		var button := CHART.paper_button(_button(card, label, func(): _act({"kind": "explore", "slot": str(slot.id), "cards": _payment_cards(), "energy": energy_payment}), "expedition.explore"), 14, true)
		button.disabled = not slot.get("ready", false)
		if not button.disabled: CHART.rim(button, CHART.GOLD_LEAF)
	for target in _list(view, "challenges"):
		if int(target.hex.q) != selected_hex.x or int(target.hex.r) != selected_hex.y: continue
		var label := "Challenge %s%s%s" % [target.name, " · AMBUSH!" if target.get("ambush", false) else "", " · FORTIFIED" if target.get("fortified", false) else ""]
		CHART.rim(CHART.paper_button(_button(card, label, func(): _act({"kind": "challenge", "group": str(target.id)}), "expedition.challenge"), 14, true), CHART.RUBRIC)
	for target in _list(view, "enters"):
		if int(target.hex.q) != selected_hex.x or int(target.hex.r) != selected_hex.y: continue
		var label := "Enter %s%s%s" % [target.name, " · AMBUSH!" if target.get("ambush", false) else "", " · FORTIFIED" if target.get("fortified", false) and not use_siege else ""]
		CHART.rim(CHART.paper_button(_button(card, label, func(): _act({"kind": "enter_site", "site": str(target.id), "item": "siege_hooks" if use_siege else ""}), "expedition.enter"), 14, true), CHART.RUBRIC)
		if target.get("siege", false):
			var siege := CHART.paper_button(_button(card, "Use Siege Hooks (defenders not Fortified)", func(): use_siege = not use_siege; _render(), "expedition.siege"), 13)
			siege.toggle_mode = true; siege.button_pressed = use_siege
			if use_siege: CHART.rim(siege, CHART.GOLD_LEAF)

func _render_site(site: Dictionary) -> void:
	var card := CHART.paper_card(_hud)
	var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 8); card.add_child(head)
	var type := str(site.get("type", ""))
	var glyph := _legend_icon(type)
	if glyph != null: CHART.icon_rect(head, glyph, 22)
	var names := VBoxContainer.new(); names.add_theme_constant_override("separation", 0); names.size_flags_horizontal = Control.SIZE_EXPAND_FILL; head.add_child(names)
	var heading := CHART.ink_label(names, str(site.name), 15, CHART.INK, true); heading.name = "SiteHeading"
	CHART.caption(names, type)
	if not site.get("open", false):
		var closed := CHART.ink_label(card, str(site.get("closed", "closed")), 12, CHART.SEPIA_TEXT); closed.add_theme_font_override("font", CHART.serif_italic())
		return
	CHART.ink_label(card, str(site.get("description", "")), 13)
	var turn_state: Dictionary = view.get("turn", {})
	var here := int(turn_state.get("influence", 0)) if str(turn_state.get("influence_site", "")) == str(site.get("id", "")) else 0
	var held := CHART.ink_label(card, "Your Influence here this turn: %d" % here, 14, CHART.GREEN if here > 0 else CHART.SEPIA_TEXT, here > 0)
	held.name = "SiteInfluence"
	var influence := CHART.paper_button(_button(card, "Play %d selected card%s here for Influence" % [selected_cards.size(), "" if selected_cards.size() == 1 else "s"], func(): _act({"kind": "use_site", "service": "influence", "cards": _payment_cards()}), "expedition.site.influence"), 14)
	influence.disabled = selected_cards.is_empty()
	if not influence.disabled: CHART.rim(influence, CHART.GOLD_LEAF)
	var per_influence := int(view.get("config", {}).get("energy_per_influence", 5))
	var bought := CHART.paper_button(_button(card, "Pay %d energy for 1 Influence" % per_influence, func(): _act({"kind": "use_site", "service": "influence", "energy": per_influence}), "expedition.site.energy_influence"), 14)
	bought.disabled = int(view.get("energy", 0)) < per_influence
	for service in _list(site, "services"):
		var id := "expedition.site.%s" % service.id + (".%s" % service.item if not str(service.get("item", "")).is_empty() else "")
		var action := {"kind": "use_site", "service": str(service.id), "item": str(service.get("item", "")), "count": 1}
		var button := CHART.paper_button(_button(card, str(service.label), func(): _act(action), id), 14)
		button.disabled = not service.get("available", false)
		if not service.get("available", false): button.tooltip_text = str(service.get("reason", ""))
	if site.get("has_deck_change", false):
		var open: bool = site.get("deck_open", false)
		var label := "Change deck · open until the turn ends" if open else "Change deck · %d Influence for the visit" % int(site.get("deck_change", 0))
		var prepare := CHART.rim(CHART.paper_button(_button(card, label, _open_prepare, "expedition.prepare"), 14), CHART.GOLD_LEAF)
		prepare.tooltip_text = "The deck is open: buy, sell, upgrade or trade down as often as your XP allows until the turn ends" if open else "Open Prepare: the first change here spends %d Influence (cards played here this turn); after that, change your deck as often as your XP allows until the turn ends" % int(site.get("deck_change", 0))

func _render_journal() -> void:
	var log_text: Array = _list(view, "log")
	if log_text.is_empty(): return
	var caption := STYLE.label(_hud, "JOURNAL", 12, STYLE.GOLD); caption.autowrap_mode = TextServer.AUTOWRAP_OFF
	CHART.gold_rule(_hud)
	var box := VBoxContainer.new(); box.name = "Log"; box.add_theme_constant_override("separation", 2); _hud.add_child(box)
	var recent := log_text.slice(maxi(0, log_text.size() - 6))
	if recent.size() > 1:
		var older := STYLE.label(box, "\n".join(recent.slice(0, recent.size() - 1)), 13, CHART.JOURNAL_OLD)
		older.add_theme_font_override("font", CHART.serif_italic()); CHART.stone_text(older, false, CHART.JOURNAL_OLD)
	var newest := STYLE.label(box, str(recent[-1]), 13, CHART.HUD_IVORY)
	newest.add_theme_font_override("font", CHART.serif_italic()); CHART.stone_text(newest)

# Overlays ---------------------------------------------------------------------
func _render_overlay() -> void:
	for child in _overlay_body.get_children(): _overlay_body.remove_child(child); child.queue_free()
	for child in _overlay_footer.get_children(): _overlay_footer.remove_child(child); child.queue_free()
	_overlay.visible = false
	var phase := str(view.get("phase", ""))
	var body := _overlay_body
	var context := ""
	var site_type := ""
	if not last_battle.is_empty():
		context = "battle_result"
		_render_battle_result(body)
	elif view.get("look") != null:
		context = "look"
		_render_look(body, view.look)
	elif view.get("last_rest") != null and dismissed_rest != str(view.get("round", 0)):
		context = "rest"
		var rest: Dictionary = view.last_rest
		var camp: Dictionary = rest.get("camp", {})
		var after: Dictionary = rest.get("clock_after") if rest.get("clock_after") is Dictionary else {}
		var month := " · " + str(after.month_name) if not str(after.get("month_name", "")).is_empty() else ""
		_overlay_heading(body, "%s%s · round %d" % ["Night falls" if rest.get("night_after", false) else "Day breaks", month, int(rest.get("round_after", 0))], CHART.glyph("moon" if rest.get("night_after", false) else "sun"))
		CHART.ink_label(body, "%d Spent cards reshuffled; energy %d → %d." % [int(camp.get("reshuffled", 0)), int(camp.get("energy_before", 0)), int(view.get("base_energy", 0))], 14)
		if int(rest.get("healed", 0)) > 0: CHART.ink_label(body, "Healed %d health at camp." % int(rest.healed), 14, CHART.GREEN)
		for roll in _list(camp, "heals"): _heal_line(body, roll)
		var marched := "The Warlord marched %d hex%s%s." % [int(rest.get("warlord_steps", 0)), "" if int(rest.get("warlord_steps", 0)) == 1 else "es", " and took his goal" if rest.get("goal_taken", false) else ""]
		CHART.ink_label(body, marched, 14, CHART.RUBRIC if rest.get("goal_taken", false) else CHART.INK)
		if not _list(rest, "patrols").is_empty(): CHART.ink_label(body, "Patrols moved: %s" % ", ".join(rest.patrols), 14)
		CHART.ink_label(body, "A fresh hand of %d." % int(rest.get("drew", 0)), 14)
		if rest.get("deadline", false): CHART.ink_label(body, "The last round is over: the Warlord comes to you.", 14, CHART.RUBRIC)
		CHART.rim(CHART.paper_button(_button(body, "Continue", func(): dismissed_rest = str(view.get("round", 0)); _render(), "expedition.rest.continue"), 15, true), CHART.GOLD_LEAF)
	elif view.get("pending_battle") != null:
		context = "fight"
		var pb: Dictionary = view.pending_battle
		site_type = str(view.get("site_here", {}).get("type", "")) if view.get("site_here") != null else ""
		var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 10); body.add_child(head)
		var piece := Control.new(); piece.custom_minimum_size = Vector2(48, 48); head.add_child(piece)
		var warlord: bool = pb.get("warlord", false); var count := int(pb.get("count", 1))
		piece.draw.connect(func():
			if warlord: CHART.draw_warlord_piece(piece, Vector2(24, 24), 60.0, true, bool(pb.get("fortified", false)))
			else: CHART.draw_enemy_piece(piece, Vector2(24, 24), 60.0, count, true))
		var titles := VBoxContainer.new(); titles.add_theme_constant_override("separation", 2); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; head.add_child(titles)
		CHART.ink_label(titles, "A fight · %s ×%d%s" % [pb.name, count, " · AMBUSH!" if pb.get("ambush", false) else ""], 22, CHART.RUBRIC, true)
		CHART.ink_label(titles, "%s · %s" % [str(pb.kind).capitalize(), pb.opponent], 12, CHART.SEPIA_TEXT)
		CHART.gold_rule(body)
		if not _list(pb, "enemy_statuses").is_empty() or not _list(pb, "player_statuses").is_empty():
			var strip := PanelContainer.new(); strip.add_theme_stylebox_override("panel", CINEMATIC.panel(CHART.DARK_SURFACE, CHART.BRONZE, 6)); body.add_child(strip)
			var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); strip.add_child(row)
			for st in _list(pb, "enemy_statuses"): _status_cell(row, "Enemy", st)
			for st in _list(pb, "player_statuses"): _status_cell(row, "You", st)
		var piles := HBoxContainer.new(); piles.add_theme_constant_override("separation", 10); body.add_child(piles)
		for pile in [["hand", _list(view, "hand").size()], ["deck", _list(view, "draw").size()], ["discard", _list(view, "spent").size()], ["removed", _list(view, "wound_cards").size()]]:
			CHART.icon_rect(piles, ICONS.texture(pile[0]), 16); CHART.numeral_label(piles, "%d" % pile[1], 14)
		CHART.ink_label(piles, "and %d energy" % int(pb.get("energy_in", 0)), 13, CHART.SEPIA_TEXT, false, false)
		CHART.rim(CHART.paper_button(_button(body, "Fight", _start_fight, "expedition.fight"), 16, true), CHART.RUBRIC)
		# A provoked fight can still be refused with a Fall back card.
		for held in _list(view, "hand"):
			if str(held.get("map_kind", "")) == "fall_back" and held.get("map_playable", false):
				var back := CHART.paper_button(_button(body, "Fall back · play %s and step back to the hex you came from" % held.name, func(): _act({"kind": "play_map_card", "card": str(held.id)}), "expedition.fall_back"), 15)
				CHART.rim(back, CHART.GOLD_LEAF)
	elif view.get("event") != null:
		context = "event"
		var ev: Dictionary = view.event
		site_type = "event"
		_overlay_heading(body, str(ev.name), null)
		CHART.ink_label(body, str(ev.text), 14)
		CHART.ink_label(body, "Selected cards (%d) are played here as Influence." % selected_cards.size(), 12, CHART.SEPIA_TEXT)
		var first := true
		for choice in _list(ev, "choices"):
			var button := CHART.paper_button(_button(body, str(choice.text), func(): _act({"kind": "event_choice", "choice": str(choice.id), "cards": _payment_cards(), "energy": _event_energy(choice)}), "expedition.event." + str(choice.id)), 15)
			button.disabled = not choice.get("available", false)
			if not choice.get("available", false): button.tooltip_text = str(choice.get("reason", ""))
			elif first: CHART.rim(button, CHART.GOLD_LEAF); first = false
	elif view.get("last_event") != null and dismissed_event != _event_key(view.last_event):
		context = "event"
		var ev: Dictionary = view.last_event
		_overlay_heading(body, "Event · %s" % str(ev.get("event", "")).capitalize().replace("_", " "), null)
		if int(ev.get("roll", 0)) > 0:
			var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); body.add_child(row)
			CHART.icon_rect(row, CHART.glyph("die"), 20)
			CHART.numeral_label(row, "You rolled %d (needed %d)." % [int(ev.roll), int(ev.needed)], 18)
		CHART.ink_label(body, str(ev.get("text", "")), 14)
		CHART.rim(CHART.paper_button(_button(body, "Continue", func(): dismissed_event = _event_key(ev); _render(), "expedition.event.continue"), 15, true), CHART.GOLD_LEAF)
	elif phase == "camp" and view.get("camp") != null:
		context = "camp"
		var camp: Dictionary = view.camp
		_render_camp(body, camp)
	elif phase == "victory" or phase == "defeat":
		context = "region_end"
		_overlay_heading(body, "The region is won!" if phase == "victory" else "The run is over", ICONS.texture("conqueror") if phase == "victory" else null, CHART.INK if phase == "victory" else CHART.RUBRIC)
		var lb: Dictionary = view.get("last_battle", {}) if view.get("last_battle") != null else {}
		if phase == "victory": CHART.ink_label(body, "The Warlord is beaten. XP earned this expedition: %d. Extra relic choices pending: %d. Every wound is mended." % [int(view.get("xp_earned", 0)), int(view.get("pending_relic_choices", 0))], 14)
		else: CHART.ink_label(body, "You lost to the Warlord. This save is marked ended; its XP and cards remain for the records.", 14)
		if not lb.is_empty(): CHART.ink_label(body, "Last battle: %s · %s" % [lb.get("name", ""), lb.get("result", "")], 12, CHART.SEPIA_TEXT)
		CHART.rim(CHART.paper_button(_button(body, "All campaigns", _show_saves, "expedition.end.campaigns"), 15, true), CHART.GOLD_LEAF)
	if context.is_empty(): return
	_overlay.visible = true
	_dress_overlay(context, site_type)

## The energy an event choice needs on top of this site's Influence and the
## selected cards (5 energy = 1 Influence); the authority checks the payment.
func _event_energy(choice: Dictionary) -> int:
	var per := int(view.get("config", {}).get("energy_per_influence", 5))
	var turn: Dictionary = view.get("turn", {})
	var here := int(turn.get("influence", 0)) if str(turn.get("influence_site", "")) == str(view.get("event", {}).get("site", "")) else 0
	var short := maxi(0, int(choice.get("influence", 0)) - here - selected_cards.size())
	return short * per if short * per <= int(view.get("energy", 0)) else 0

## A look effect: the top of the draw pile, top first. The player picks the
## card to draw, the cards for the bottom, and the order the rest go back on
## top; anything left keeps its order under the placed cards.
func _render_look(body: VBoxContainer, look: Dictionary) -> void:
	var cards: Array = _list(look, "cards")
	var key := "%s:%s" % [look.card, ",".join(cards.map(func(c): return str(c.id)))]
	if key != _look_key: _look_key = key; _look_draw = -1; _look_top = []; _look_bottom = []
	var draws := int(look.get("draw", 0)) > 0
	var bottom_ok: bool = look.get("bottom", false)
	_overlay_heading(body, "%s · the top %d of your draw pile" % [look.card_name, cards.size()], ICONS.texture("deck"))
	var how := "Top card first. " + ("Choose the card to draw. " if draws else "") + ("Send any to the bottom. " if bottom_ok else "") + "Then click the rest in the order you want them back on top; any you leave stay under them in their order."
	CHART.ink_label(body, how, 13, CHART.SEPIA_TEXT)
	for i in cards.size():
		var c: Dictionary = cards[i]
		var row := HBoxContainer.new(); row.name = "Look_%d" % i; row.add_theme_constant_override("separation", 6); body.add_child(row)
		var state := ""
		if i == _look_draw: state = "drawn"
		elif i in _look_bottom: state = "bottom"
		elif i in _look_top: state = "back on top · %d" % (_look_top.find(i) + 1)
		var name_label := CHART.ink_label(row, "%d. %s%s" % [i + 1, c.name, "  →  " + state if not state.is_empty() else ""], 14, CHART.GREEN if not state.is_empty() else CHART.INK, true)
		name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL; name_label.mouse_filter = Control.MOUSE_FILTER_STOP
		name_label.tooltip_text = BattlePresentationCatalog.card_tooltip(str(c.id))
		if draws:
			var take := CHART.paper_button(_button(row, "Draw", func(): _look_place(i, "draw"), "expedition.look.draw.%d" % i), 13)
			if i == _look_draw: CHART.rim(take, CHART.GOLD_LEAF)
		var next := CHART.paper_button(_button(row, "Put back next", func(): _look_place(i, "top"), "expedition.look.top.%d" % i), 13)
		next.disabled = i in _look_top
		if bottom_ok:
			var low := CHART.paper_button(_button(row, "Bottom", func(): _look_place(i, "bottom"), "expedition.look.bottom.%d" % i), 13)
			if i in _look_bottom: CHART.rim(low, CHART.GOLD_LEAF)
	var footer := _overlay_footer
	CHART.paper_button(_button(footer, "Start over", func(): _look_draw = -1; _look_top = []; _look_bottom = []; _render(), "expedition.look.reset"), 14)
	var done := CHART.paper_button(_button(footer, "Done", func():
		_act({"kind": "resolve_look", "card": str(cards[_look_draw].id) if _look_draw >= 0 else "", "cards": _look_top.map(func(i): return str(cards[i].id)), "bottom": _look_bottom.map(func(i): return str(cards[i].id))}), "expedition.look.done"), 16, true)
	done.disabled = draws and _look_draw < 0
	if not done.disabled: CHART.rim(done, CHART.GOLD_LEAF)
	else: done.tooltip_text = "Choose the card to draw first"

## Places one looked-at card: drawn, back on top next, or on the bottom.
func _look_place(index: int, where: String) -> void:
	_look_top.erase(index); _look_bottom.erase(index)
	if _look_draw == index: _look_draw = -1
	match where:
		"draw": _look_draw = index
		"top": _look_top.append(index)
		"bottom": _look_bottom.append(index)
	_render()

func _overlay_heading(body: Node, text: String, icon: Texture2D, color: Color = CHART.INK) -> void:
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); body.add_child(row)
	if icon != null: CHART.icon_rect(row, icon, 22)
	var label := CHART.ink_label(row, text, 22, color, true); label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	CHART.gold_rule(body)

## The camp sheet: why the round ended and what the rest did (the fresh hand
## is drawn by the authority when the rest begins), every wound grouping with
## its basic aid and bandage, the heal log, Influence and the camp actions.
func _render_camp(body: VBoxContainer, camp: Dictionary) -> void:
	var reason := str(camp.get("reason", ""))
	var base := int(view.get("base_energy", 0))
	var inn_bonus := int(camp.get("inn_bonus", 0))
	var title: String = {"deck_empty": "Camp · the deck ran out", "make_camp": "Camp", "defeat": "Camp · after the defeat"}.get(reason, "Camp")
	if camp.get("inn", false): title += " · at the inn (+%d to healing rolls)" % inn_bonus
	_overlay_heading(body, title, CHART.glyph("tent"))
	var opening := ""
	match reason:
		"deck_empty": opening = "Round %d is over: no cards were left in your hand or draw pile at the end of turn %d. %d Spent cards reshuffled into the draw pile; a fresh hand of %d drawn; energy reset to %d." % [int(camp.get("round", 0)), int(camp.get("turn", 0)), int(camp.get("reshuffled", 0)), int(camp.get("drew", 0)), base]
		"make_camp": opening = "%d unplayed cards forfeited and reshuffled with %d Spent; a fresh hand of %d drawn; energy reset to %d." % [int(camp.get("forfeited", 0)), int(camp.get("reshuffled", 0)), int(camp.get("drew", 0)), base]
		"defeat": opening = "Driven back; round %d ends here. %d Spent cards reshuffled into the draw pile; a fresh hand of %d drawn; energy reset to %d." % [int(camp.get("round", 0)), int(camp.get("reshuffled", 0)), int(camp.get("drew", 0)), base]
		_: opening = "%d Spent cards reshuffled into the draw pile; a fresh hand of %d drawn; energy reset to %d." % [int(camp.get("reshuffled", 0)), int(camp.get("drew", 0)), base]
	CHART.ink_label(body, opening, 14)
	if camp.get("deadline", false): CHART.ink_label(body, "This was the last round: when you break camp the Warlord comes to you.", 14, CHART.RUBRIC)
	# Wounds: one row per grouping, with the camp's two healing steps.
	var wounds: Array = _list(view, "wounds")
	var lost: Array = _list(view, "wound_cards")
	CHART.gold_rule(body)
	if wounds.is_empty():
		CHART.ink_label(body, "No wounds", 14, CHART.INK, true)
	else:
		var heading := "Wounds · %d (%d health lost)" % [wounds.size(), lost.size()]
		if inn_bonus > 0: heading += " · +%d at the inn" % inn_bonus
		CHART.ink_label(body, heading, 14, CHART.INK, true)
		for k in wounds.size(): _camp_wound_row(body, k, wounds[k], camp)
		if int(camp.get("untreated", 0)) > 1:
			var all := CHART.paper_button(_button(body, "Treat every wound · basic aid", func(): _act({"kind": "camp_service", "service": "basic_aid", "wound": "all"}), "expedition.camp.aid_all"), 13)
			all.tooltip_text = "Basic first aid on each of the %d untreated wounds, in order: 1d%d%s damage healed from each" % [int(camp.get("untreated", 0)), int(camp.get("basic_aid_die", 0)), " +%d" % inn_bonus if inn_bonus > 0 else ""]
	var heals: Array = _list(camp, "heals")
	if not heals.is_empty():
		CHART.ink_rule(body)
		for roll in heals: _heal_line(body, roll)
	CHART.gold_rule(body)
	var pack := "Bandages %d" % int(camp.get("bandages", 0))
	if int(camp.get("spare_bandages", 0)) > 0: pack += " (%d spare, gone when camp breaks)" % int(camp.spare_bandages)
	if int(camp.get("aid_bonus", 0)) > 0: pack += " · Field Medic: +%d to basic aid" % int(camp.aid_bonus)
	CHART.ink_label(body, "%s · Influence %d (play cards from your fresh hand, or energy, for more)" % [pack, int(view.get("turn", {}).get("influence", 0))], 13, CHART.SEPIA_TEXT)
	var footer := _overlay_footer
	var influence := CHART.paper_button(_button(footer, "Play %d selected card%s for Influence" % [selected_cards.size(), "" if selected_cards.size() == 1 else "s"], func(): _act({"kind": "camp_service", "service": "influence", "cards": _payment_cards()}), "expedition.camp.influence"), 14)
	influence.disabled = selected_cards.is_empty()
	var per_influence := int(view.get("config", {}).get("energy_per_influence", 5))
	var bought := CHART.paper_button(_button(footer, "Pay %d energy for 1 Influence" % per_influence, func(): _act({"kind": "camp_service", "service": "influence", "energy": per_influence}), "expedition.camp.energy_influence"), 14)
	bought.disabled = int(view.get("energy", 0)) < per_influence
	var deck_open: bool = camp.get("deck_open", false)
	var deck := CHART.paper_button(_button(footer, "Change deck · open until you break camp" if deck_open else "Change deck · %d Influence, then change freely until you break camp" % int(camp.get("deck_change", 0)), _open_prepare, "expedition.prepare"), 14)
	deck.tooltip_text = "The deck is open: buy, sell, upgrade or trade down as often as your XP allows; it closes when you break camp" if deck_open else "Open Prepare: the first change spends %d Influence (cards played at camp); after that, change your deck as often as your XP allows until you break camp" % int(camp.get("deck_change", 0))
	CHART.rim(CHART.paper_button(_button(footer, "Break camp", func(): _act({"kind": "break_camp"}), "expedition.camp.break"), 16, true), CHART.RUBRIC if camp.get("deadline", false) else CHART.GOLD_LEAF)

## One wound at camp: what it is, its cards, and its basic aid and bandage
## buttons. Eligibility and the dice come from the view, never from the client.
func _camp_wound_row(body: VBoxContainer, index: int, w: Dictionary, camp: Dictionary) -> void:
	var id := str(w.get("id", ""))
	var panel := PanelContainer.new(); panel.name = "Wound_" + id
	panel.add_theme_stylebox_override("panel", CHART.chip_style(Color(CHART.BONE, 0.55), Color(CHART.INK, 0.25), 8, 5))
	body.add_child(panel)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 2); panel.add_child(box)
	var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 6); box.add_child(head)
	var line := CHART.ink_label(head, "Wound %d · %d damage · %s · %s" % [index + 1, int(w.get("damage", 0)), str(w.get("source", "")), str(w.get("taken", ""))], 13, CHART.INK, true)
	line.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var state := _wound_state(w)
	CHART.ink_label(head, state, 13, CHART.GREEN if state != "untreated" else CHART.RUBRIC, false, false)
	CHART.ink_label(box, _card_names(_list(w, "cards")), 12, CHART.SEPIA_TEXT)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); box.add_child(row)
	var inn_bonus := int(camp.get("inn_bonus", 0))
	var bonus := " +%d" % inn_bonus if inn_bonus > 0 else ""
	var aid_bonus := inn_bonus + int(camp.get("aid_bonus", 0))
	var aid := CHART.paper_button(_button(row, "Basic aid · heals 1d%d%s" % [int(camp.get("basic_aid_die", 0)), " +%d" % aid_bonus if aid_bonus > 0 else ""], func(): _act({"kind": "camp_service", "service": "basic_aid", "wound": id}), "expedition.camp.aid." + id), 13)
	aid.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	aid.disabled = not w.get("basic_aid", false)
	if aid.disabled: aid.tooltip_text = str(w.get("reason", "")) if not str(w.get("reason", "")).is_empty() else "treated at this camp"
	else: aid.tooltip_text = "Every character's basic first aid: roll 1d%d%s and heal that much damage from this wound, once per wound per camp" % [int(camp.get("basic_aid_die", 0)), bonus]
	var bandage := CHART.paper_button(_button(row, "Bandage · heals 1d%d%s · %d left" % [int(camp.get("bandage_die", 0)), bonus, int(camp.get("bandages", 0))], func(): _act({"kind": "camp_service", "service": "bandage", "wound": id}), "expedition.camp.bandage." + id), 13)
	bandage.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	bandage.disabled = not w.get("bandage", false)
	if not bandage.disabled: bandage.tooltip_text = "Spend a bandage from your pack: roll 1d%d%s and heal that much damage from this treated wound, once per wound per camp" % [int(camp.get("bandage_die", 0)), bonus]
	elif not w.get("treated", false): bandage.tooltip_text = "treat it with basic aid first"
	elif int(camp.get("bandages", 0)) < 1: bandage.tooltip_text = "no bandages"
	else: bandage.tooltip_text = str(w.get("reason", ""))
	if not aid.disabled: CHART.rim(aid, CHART.GOLD_LEAF)
	elif not bandage.disabled: CHART.rim(bandage, CHART.GOLD_LEAF)

## One camp healing roll, as logged at camp and in the rest summary.
func _heal_line(body: Node, roll: Dictionary) -> Label:
	var method := "Basic aid" if str(roll.get("method", "")) == "basic_aid" else "A bandage"
	var closed: bool = roll.get("closed", false)
	var names := _card_names(_list(roll, "cards"))
	var text := "%s on the %s wound: rolled %d%s → healed %d%s · %s" % [method, str(roll.get("source", "")), int(roll.get("roll", 0)), " +%d" % int(roll.get("bonus", 0)) if int(roll.get("bonus", 0)) > 0 else "", int(roll.get("healed", 0)), " (%s)" % names if not names.is_empty() else "", "the wound is healed" if closed else "%d damage remains" % int(roll.get("remaining", 0))]
	return CHART.ink_label(body, text, 13, CHART.GREEN if closed else CHART.INK)

func _status_cell(row: Node, who: String, st: Dictionary) -> void:
	var cell := HBoxContainer.new(); cell.mouse_filter = Control.MOUSE_FILTER_STOP; cell.tooltip_text = str(st.rules); cell.add_theme_constant_override("separation", 4); row.add_child(cell)
	var caption := STYLE.label(cell, who + ":", 12, STYLE.GOLD); caption.autowrap_mode = TextServer.AUTOWRAP_OFF
	CHART.icon_rect(cell, ICONS.texture(str(st.id)), 20)
	var label := STYLE.label(cell, "%s ×%d" % [st.name, int(st.stacks)], 14, CINEMATIC.IVORY); label.autowrap_mode = TextServer.AUTOWRAP_OFF; CHART.stone_text(label)

## The painting behind the sheet, and the scene's size and place over the map.
func _dress_overlay(context: String, site_type: String) -> void:
	var night: bool = view.get("night", false)
	var player_hex: Dictionary = {}
	for hex in _list(view, "hexes"):
		if hex.get("player", false): player_hex = hex
	var terrain := str(player_hex.get("terrain", ""))
	var warlord: bool = view.get("pending_battle") != null and view.pending_battle.get("warlord", false)
	var painting := CHART.habitat_for(context, terrain, site_type, warlord, night)
	_overlay_backdrop.texture = CHART.habitat_texture(painting)
	var tint := Color(0.42, 0.46, 0.6) if night else Color(0.5, 0.5, 0.56)
	if context == "region_end": tint = Color(0.7, 0.62, 0.45) if str(view.get("phase", "")) == "victory" else Color(0.5, 0.3, 0.3)
	_overlay_backdrop.modulate = tint
	# Size the scene to its text: once now, and again after a frame so wrapped
	# lines are measured at their real width.
	_fit_overlay(false)
	_fit_overlay.call_deferred(true)

func _fit_overlay(measured: bool) -> void:
	if not _overlay.visible or _map_panel == null: return
	var map_rect := _map_panel.get_global_rect()
	var viewport := get_viewport_rect().grow(-20)
	var width := minf(1040.0, map_rect.size.x - 40.0)
	var content: float = (_overlay_body.size.y if measured else _overlay_body.get_combined_minimum_size().y) + 8.0
	var footer: float = _overlay_footer.get_combined_minimum_size().y + (8.0 if _overlay_footer.get_child_count() > 0 else 0.0)
	var max_height := maxf(180.0, map_rect.size.y - 24.0)
	var height := clampf(content + footer + 48.0 + 36.0 + 20.0, 180.0, max_height)
	_overlay_scroll.custom_minimum_size = Vector2(minf(560.0, width - 80.0), clampf(content, 80.0, height - footer - 48.0 - 36.0))
	_overlay.size = Vector2(width, height)
	var at := map_rect.get_center() - _overlay.size * 0.5
	at = at.clamp(viewport.position, (viewport.end - _overlay.size).max(viewport.position))
	_overlay.global_position = at
	if measured: return
	# A second measured pass on the next frame.
	get_tree().process_frame.connect(func(): _fit_overlay(true), CONNECT_ONE_SHOT)

## One resolved event: its site, the event and the round and turn it resolved
## in. Dismissing it keeps it dismissed until another event resolves.
static func _event_key(ev: Dictionary) -> String:
	return "%s/%s/%s/%s" % [ev.get("site", ""), ev.get("event", ""), ev.get("round", 0), ev.get("turn", 0)]

func _render_battle_result(body: VBoxContainer) -> void:
	var outcome: Dictionary = last_battle.get("outcome", {})
	var report: Dictionary = outcome.get("report", {}) if outcome.get("report") != null else {}
	var expedition: Dictionary = last_battle.get("expedition", {})
	var result := str(outcome.get("result", last_battle.get("error", "not recorded")))
	_overlay_heading(body, "Battle · %s · %s" % [expedition.get("name", ""), result.capitalize()], null, CHART.GREEN if result == "victory" else (CHART.RUBRIC if result == "defeat" else CHART.INK))
	if outcome.is_empty():
		CHART.ink_label(body, "The result was not recorded: %s" % last_battle.get("error", "unknown error"), 13, CHART.RUBRIC)
	else:
		var lines: Array = []
		if int(outcome.get("xp_awarded", 0)) > 0: lines.append("+%d XP (now %d)%s" % [int(outcome.xp_awarded), int(outcome.get("xp", 0)), " · +%d early-strike XP" % int(report.get("early_xp", 0)) if int(report.get("early_xp", 0)) > 0 else ""])
		if int(report.get("gold", 0)) > 0 or int(report.get("loot", 0)) > 0: lines.append("+%d gold%s" % [int(report.get("gold", 0)), " and %d loot" % int(report.get("loot", 0)) if int(report.get("loot", 0)) > 0 else ""])
		lines.append("Energy: brought %d, ended %d, kept %d" % [int(report.get("energy_in", 0)), int(report.get("energy_end", 0)), int(report.get("energy_kept", 0))])
		var wounds := "wounds %d (%d health lost)" % [int(report.get("wounds", 0)), int(report.get("wound_cards", 0))]
		if result == "victory" and int(report.get("new_wounds", 0)) > 0: wounds += " · +%d new" % int(report.new_wounds)
		lines.append("Cards back on the map: hand %d, draw %d, Spent %d · %s" % [_list(report, "hand_after").size(), int(report.get("draw_after", 0)), int(report.get("spent_after", 0)), wounds])
		for line in lines: CHART.ink_label(body, line, 14)
		if int(report.get("held_back", 0)) > 0: CHART.ink_label(body, "A battle can't leave you better off than you went in: %d card%s went back to worse piles." % [int(report.held_back), "" if int(report.held_back) == 1 else "s"], 14, CHART.SEPIA_TEXT)
		if int(report.get("defeat_wounds", 0)) > 0: CHART.ink_label(body, "Defeat: one defeat wound of %d damage on top of your pre-battle wounds; driven back; the round ends with a rest." % int(report.defeat_wounds), 14, CHART.RUBRIC)
		if result == "draw": CHART.ink_label(body, "A draw: nothing is recorded; your pre-battle cards, wounds and energy are back.%s" % (" The Warlord attacks again." if str(report.get("kind", "")) == "deadline" else ""), 14)
		if not str(report.get("site_freed", "")).is_empty(): CHART.ink_label(body, "%s is freed." % report.site_freed, 14, CHART.GREEN)
		if report.get("lieutenant", false) and result == "victory":
			var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); body.add_child(row)
			CHART.icon_rect(row, ICONS.texture("lieutenant"), 16); CHART.ink_label(row, "A Lieutenant is slain: the Warlord loses a Lieutenant's Favor.", 14)
		if report.get("region_won", false): CHART.ink_label(body, "The region is won! %d relic choice(s) pending; wounds mended: %d health." % [int(report.get("relic_choices", 0)), int(report.get("wounds_mended", 0))], 14, CHART.GREEN)
		if report.get("run_over", false): CHART.ink_label(body, "The Warlord has won. The run is over.", 14, CHART.RUBRIC)
	CHART.rim(CHART.paper_button(_button(body, "Continue", func(): last_battle = {}; _render(), "expedition.battle.continue"), 15, true), CHART.GOLD_LEAF)

# Actions that leave the screen --------------------------------------------------
func _start_fight() -> void:
	var pending: Dictionary = status.get("pending_battle", {}) if status.get("pending_battle") != null else {}
	if pending.is_empty(): return
	_set_message("Preparing the fight…")
	await get_tree().process_frame
	var gateway: RefCounted = LEARNED_GATEWAY.new(_runtime(), "seat-a", "minion:%s:%d" % [str(pending.opponent), int(pending.opponent_count)], str(status.get("character", "starter")))
	gateway.unified_defense = true
	gateway.simultaneous_play = true
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
	screen.expedition_open = bool(view.get("camp", {}).get("deck_open", false)) if view.get("phase", "") == "camp" else bool(view.get("site_here", {}).get("deck_open", false))
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
		"player": view.get("player", {}), "energy": int(view.get("energy", 0)), "gold": int(view.get("gold", 0)), "xp": int(status.get("xp", 0)), "xp_total": int(status.get("xp_total", 0)),
		"hand": _list(view, "hand").map(func(c): return str(c.id)), "draw": _list(view, "draw").size(), "spent": _list(view, "spent").size(), "wounds": _list(view, "wounds"), "wound_cards": _list(view, "wound_cards").size(),
		"turn": turn, "moves": _list(view, "moves"), "explores": _list(view, "explores"), "challenges": _list(view, "challenges"), "enters": _list(view, "enters"),
		"site_here": view.get("site_here"), "pending_battle": view.get("pending_battle"), "event": view.get("event"), "camp": view.get("camp"), "last_battle": view.get("last_battle"), "last_rest": view.get("last_rest"),
		"statuses": _list(view, "statuses"), "inventory": _list(view, "inventory"), "warlord": view.get("warlord", {}), "result": str(view.get("result", "")),
		"selected_hex": [selected_hex.x, selected_hex.y] if selected_hex != NONE else null, "selected_cards": selected_cards, "energy_payment": energy_payment,
		"placed_tiles": _list(view, "slots").filter(func(s): return s.get("placed", false)).size(), "seen_hexes": _list(view, "hexes").filter(func(h): return h.get("seen", false)).size(),
		"message": _message.text, "chart_day": chart_day_label(), "clock": _clock(), "cartouche": {"hover": map_model.get("cartouche_hover", false), "lines": map_model.get("cartouche_lines", [])}, "heal_feedback": {"active": _heal_feedback.active(), "last": _heal_feedback.last, "shown": _heal_feedback.shown.size()}, "hex_radius": _hex_radius, "survey": _survey_key or _survey_locked, "legend_open": _legend_open, "marks_processing": _marks.is_processing() if _marks != null else false,
		"hexes": _list(view, "hexes"), "slots": _list(view, "slots"),
	}

func _button(parent: Node, text: String, callback: Callable, control_id: String) -> Button:
	var button := Button.new(); button.text = text; button.pressed.connect(callback)
	button.focus_mode = Control.FOCUS_NONE
	button.set_meta("expedition_control", control_id)
	parent.add_child(button)
	_register(control_id, button, text)
	return button

func _register(control_id: String, control: Control, caption: String) -> void:
	control.set_meta("expedition_control", control_id)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, control, caption)
