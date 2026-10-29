extends "res://tests/presentation/verify_campaign.gd"
## The chart's drawing contract: outer-edge walking, glyph rasterisation,
## the procedural textures, the hex radius clamp at both window sizes, the
## hexagon hit-test, and the ledger, footer and overlays staying inside the
## viewport for every overlay state the screen can show.

const CHART := preload("res://app/screens/map/map_style.gd")
const HEX := preload("res://app/screens/map/hex_button.gd")
const EXPEDITION_SCRIPT := "res://app/screens/map/expedition_screen.gd"

var runtime

func _control(screen: Node, key: String) -> Button:
	if screen == null: return null
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("expedition_control", "") == key and not button.is_queued_for_deletion() and button.is_visible_in_tree(): return button
	return null

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	runtime = root.get_node("LearnedBattleRuntime")
	_geometry()
	_glyphs_and_textures()
	await _screen_checks(Vector2i(1280, 720))
	await _screen_checks(Vector2i(1920, 1080))
	print("EXPEDITION CHART: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _geometry() -> void:
	var tile: Array = []
	for offset in [Vector2i(0, 0), Vector2i(1, 0), Vector2i(1, -1), Vector2i(0, -1), Vector2i(-1, 0), Vector2i(-1, 1), Vector2i(0, 1)]: tile.append(Vector2i(3, 2) + offset)
	_expect(CHART.outer_edge_count(tile) == 18, "a lone seven-hex tile has 18 outer edges (%d)" % CHART.outer_edge_count(tile))
	var disc: Array = []
	for q in range(-2, 3):
		for r in range(-2, 3):
			if absi(q) <= 2 and absi(r) <= 2 and absi(q + r) <= 2: disc.append(Vector2i(q, r))
	_expect(disc.size() == 19 and CHART.outer_edge_count(disc) == 30, "a radius-2 disc of 19 hexes has 30 outer edges (%d)" % CHART.outer_edge_count(disc))
	_expect(CHART.inner_edges(tile, func(q, r): return Vector2(q, r), 1.0).size() == 12, "a seven-hex tile has 12 interior edges")
	# Every edge's neighbour direction matches the pointy-top centre formula.
	var center := func(q: int, r: int) -> Vector2: return Vector2(sqrt(3.0) * (q + r * 0.5), 1.5 * r)
	for i in 6:
		var n: Vector2i = CHART.EDGE_NEIGHBOUR[i]
		var a: Vector2 = CHART.corners(center.call(0, 0), 1.0)[i]; var b: Vector2 = CHART.corners(center.call(0, 0), 1.0)[(i + 1) % 6]
		var mid := (a + b) * 0.5
		_expect(mid.distance_to(center.call(n.x, n.y)) < mid.distance_to(center.call(-n.x, -n.y)), "edge %d faces its neighbour" % i)

func _glyphs_and_textures() -> void:
	for id in CHART.SHAPES.keys():
		var tex: Texture2D = CHART.glyph(id)
		_expect(tex != null and tex.get_width() == 48 and tex.get_height() == 48, "glyph %s rasterises to 48 px" % id)
	for colourway in ["ivory", "gold", "sepia", "red", "paper", "wax"]:
		_expect(CHART.glyph("pawn", colourway) != null, "the %s colourway rasterises" % colourway)
	var speckle: Texture2D = CHART.speckle()
	_expect(speckle.get_width() == 256 and speckle.get_height() == 256 and speckle.get_image().get_format() == Image.FORMAT_RGBA8, "the speckle is a 256×256 RGBA8 texture")
	var hatch: Texture2D = CHART.hatch()
	_expect(hatch.get_width() == 32 and hatch.get_height() == 32 and hatch.get_image().get_pixel(0, 31).a == 1.0 and hatch.get_image().get_pixel(5, 5).a == 0.0, "the hatch is a 32×32 diagonal")
	_expect(CHART.vignette(false) != null and CHART.vignette(true) != null, "the vignettes exist")
	_expect(CHART.habitat_for("fight", "marsh", "", true, false) == "ash_caldera" and CHART.habitat_for("fight", "marsh", "", false, false) == "peat_bog" and CHART.habitat_for("event", "plains", "grotto", false, true) == "mushroom_cavern", "paintings follow terrain, site and night")
	_expect(CHART.habitat_texture("peat_bog") != null, "a habitat painting loads")

## Hovering the cartouche with the pointer shows the calendar in the chart's
## hover box: the month, the season and year, Day or Night with today's sight
## and terrain costs, and the season's rules and effects, inside the viewport.
func _calendar_hover(map: Control, label: String) -> void:
	var cartouche: Control = map._cartouche
	var area: Rect2 = map._map_area.get_global_rect()
	_expect(cartouche != null and cartouche.get_global_rect().has_area() and area.encloses(cartouche.get_global_rect()), "the cartouche's hover area lies on the chart at %s (%s)" % [label, cartouche.get_global_rect() if cartouche != null else Rect2()])
	if cartouche == null: return
	var motion := InputEventMouseMotion.new(); motion.position = cartouche.get_global_rect().get_center(); root.push_input(motion, true)
	for frame in 4: await process_frame
	var clock: Dictionary = map.view.get("clock", {})
	var lines: Array = map.map_model.get("cartouche_lines", [])
	var text := "\n".join(lines)
	_expect(map.map_model.get("cartouche_hover", false) and lines.size() >= 5, "hovering the cartouche shows the calendar at %s: %s" % [label, lines])
	_expect(str(lines[0]) == "Late Summer · Day" if not lines.is_empty() else false, "the calendar names the month and Day/Night (%s)" % [lines])
	_expect("Summer · Year 1" in text and "Day · sight %d" % int(map.view.turn.sight) in text and "Day Move: Plains 1" in text, "the calendar shows the season, the year, today's sight and terrain: %s" % text)
	_expect(not str(clock.get("season_rules", "")).is_empty() and str(clock.get("season_rules", "")) in text and "Seasonal effects: none yet" in text, "the calendar shows the season's rules and its (empty) effects: %s" % text)
	var box: Rect2 = map._marks.hover_box
	var global_box := Rect2(map._marks.get_global_position() + box.position, box.size)
	_expect(box.has_area() and Rect2(Vector2.ZERO, map._marks.size).encloses(box) and root.get_visible_rect().encloses(global_box), "the calendar's hover box stays inside the chart and the viewport at %s (%s)" % [label, global_box])
	_expect(absf(global_box.position.y - cartouche.get_global_rect().end.y - 6.0) < 1.0 and absf(global_box.position.x - cartouche.get_global_rect().position.x) < 1.0, "the calendar hangs under the cartouche at %s (%s under %s)" % [label, global_box, cartouche.get_global_rect()])
	await _capture(map, "expedition-calendar-" + label)
	motion = InputEventMouseMotion.new(); motion.position = map._hud_scroll.get_global_rect().get_center(); root.push_input(motion, true)
	for frame in 4: await process_frame
	_expect(not map.map_model.get("cartouche_hover", false) and not map._marks.hover_box.has_area(), "leaving the cartouche hides the calendar at %s" % label)

func _screen_checks(size: Vector2i) -> void:
	root.size = size
	for frame in 3: await process_frame
	var created: Dictionary = runtime.campaign_new("starter", "Chart %d" % size.x, "expedition", "", 6)
	_expect(created.get("ok") == true, "an expedition starts")
	if created.get("ok") != true: return
	var map = load(EXPEDITION_SCRIPT).new(); map.save_id = str(created.result.id); root.add_child(map)
	for frame in 8: await process_frame
	var label := "%dx%d" % [size.x, size.y]
	var radius: float = map._hex_radius
	_expect(radius >= 28.0 and radius <= 64.0, "the hex radius is clamped to 28–64 at %s (%.1f)" % [label, radius])
	# No banner: the map reaches the top of the window, the parchment covers
	# the whole map area (no table bars at the sides), and the day line is short.
	var map_rect: Rect2 = map._map_area.get_global_rect()
	_expect(map_rect.position.y <= 24.0, "the map reaches the top of the window at %s (y %.0f)" % [label, map_rect.position.y])
	_expect(map._sheet.sheet_rect.encloses(Rect2(Vector2.ZERO, map._map_area.size).grow(-map._sheet.EDGE)), "the parchment fills the map area at %s (%s in %s)" % [label, map._sheet.sheet_rect, map._map_area.size])
	_expect(map.chart_day_label() == "Late Summer · Day · 1/4", "the cartouche day line is Late Summer · Day · 1/4 (%s)" % map.chart_day_label())
	var all_saves := _control(map, "expedition.all_campaigns"); var back := _control(map, "expedition.back")
	var ledger_rect: Rect2 = map._hud_scroll.get_parent().get_global_rect()
	_expect(all_saves != null and back != null and ledger_rect.encloses(all_saves.get_global_rect()) and ledger_rect.encloses(back.get_global_rect()), "Campaigns and Menu sit at the top of the ledger at %s" % label)
	_expect(not "round 1 of 4" in _text(map) and not "CAMPAIGN  /  EXPEDITION" in _text(map), "the old banner is gone at %s" % label)
	await _capture(map, "expedition-chart-layout-" + label)
	# Every slot hex, placed or not, lies inside the map area without panning.
	var area: Rect2 = map._map_area.get_global_rect()
	var inside := true
	for key in map._hex_buttons:
		if not area.encloses(map._hex_buttons[key].get_global_rect()): inside = false
	_expect(inside, "the whole region footprint fits the map area at %s" % label)
	# The hexagon hit-test: the centre hits, the bounding-box corner misses.
	var button = map._hex_buttons["0,0"]
	_expect(button._has_point(button.size * 0.5) and not button._has_point(Vector2.ZERO) and not button._has_point(Vector2(button.size.x, 0)), "clicks count only inside the hexagon")
	_expect(map._hex_buttons["2,1"].state == "unexplored" and map._hex_buttons["2,1"]._has_point(map._hex_buttons["2,1"].size * 0.5), "an unexplored slot hex is still clickable at its centre")
	# Hex buttons never show the engine's black tooltip; the chart's cartouche is the hover.
	var tooltips := 0
	for key in map._hex_buttons:
		if not map._hex_buttons[key].tooltip_text.is_empty(): tooltips += 1
	_expect(tooltips == 0, "no hex button carries an engine tooltip (%d do)" % tooltips)
	var hover: Array = map._hover_lines("-1,1")
	_expect(hover.size() >= 3 and "Hollow Ford" in str(hover[2]) and "Move here" in str(hover[-1]), "the hover cartouche names the hex, its terrain, the site and the move: %s" % [hover])
	_expect(map._hover_lines("2,1").size() == 2 and "Unexplored" in str(map._hover_lines("2,1")[0]), "an unexplored hex hovers as its slot")
	# The chart layers and model.
	_expect(map.map_model.hexes.size() == 7 and map.map_model.slots.size() == 8 and map.map_model.sight.size() == 7, "the map model holds the start tile, the slots and the sight set")
	_expect(map._marks.mouse_filter == Control.MOUSE_FILTER_IGNORE and map._lines.mouse_filter == Control.MOUSE_FILTER_IGNORE and map._sheet.mouse_filter == Control.MOUSE_FILTER_IGNORE, "the chart layers never take the mouse")
	_expect(not map._marks.is_processing(), "the marks layer idles while nothing is selected or hovered")
	await _calendar_hover(map, label)
	# Ledger and footer inside the viewport; End turn and Make camp pinned.
	var viewport := root.get_visible_rect()
	var ledger: Control = map._hud_scroll.get_parent()
	_expect(viewport.encloses(ledger.get_global_rect()), "the ledger is inside the viewport at %s" % label)
	for id in ["expedition.end_turn", "expedition.make_camp"]:
		var control := _control(map, id)
		_expect(control != null and viewport.encloses(control.get_global_rect()) and control.get_global_rect().end.y > ledger.get_global_rect().end.y - 70, "%s is pinned at the ledger's foot (%s)" % [id, label])
	# The legend opens with its chips and cost table, and Survey mode labels tiles.
	await _click(_control(map, "expedition.legend.toggle"))
	for frame in 2: await process_frame
	_expect(_control(map, "expedition.legend.forest") == null and _control(map, "expedition.legend.village") != null and _control(map, "expedition.survey") != null, "the legend lists sites and the survey toggle")
	await _click(_control(map, "expedition.survey"))
	for frame in 2: await process_frame
	_expect(map.inspection_state().survey == true, "the survey toggle turns survey mode on")
	await _click(_control(map, "expedition.survey"))
	await _click(_control(map, "expedition.legend.toggle"))
	for frame in 2: await process_frame
	# Selecting a hex keeps the hex buttons (no rebuild) and shows the paper card.
	var before = map._hex_buttons["1,0"]
	await _click(before)
	for frame in 2: await process_frame
	_expect(map._hex_buttons["1,0"] == before and map.selected_hex == Vector2i(1, 0) and map.map_model.selected == "1,0" and map._marks.is_processing(), "selecting a hex keeps the buttons, updates the model and wakes the marks layer")
	_expect(viewport.encloses(_control(map, "expedition.move").get_global_rect()), "the selected-hex card's Move button is inside the viewport at %s" % label)
	# Overlays: the camp, the rest summary and (through the real fight path later) the others are enclosed.
	await _click(_control(map, "expedition.make_camp"))
	for frame in 4: await process_frame
	_expect(map._overlay.visible and viewport.encloses(map._overlay.get_global_rect()), "the camp overlay is inside the viewport at %s" % label)
	_expect(map._scrim.visible and map._overlay_backdrop.texture != null, "the camp scene has a scrim and a painting")
	for node in map._overlay.find_children("*", "Label", true, false):
		if node.is_visible_in_tree(): _expect(viewport.encloses(node.get_global_rect()), "camp text inside the viewport (%s): %s" % [label, node.text.left(30)])
	var brk := _control(map, "expedition.camp.break")
	_expect(brk != null and viewport.encloses(brk.get_global_rect()), "Break camp is reachable at %s" % label)
	# The old mend-roll services are gone; the camp heals wounds by basic aid and bandages.
	var mends := 0
	for control in map.find_children("*", "Button", true, false):
		if str(control.get_meta("expedition_control", "")).begins_with("expedition.camp.mend"): mends += 1
	_expect(mends == 0, "no mend-roll control exists at camp (%d found)" % mends)
	_expect("No wounds" in _text(map._overlay) and "fresh hand of %d" % int(map.view.camp.drew) in _text(map._overlay), "an unwounded camp says so and explains the fresh hand")
	await _click(brk)
	for frame in 4: await process_frame
	_expect(map.view.night and map._chart.modulate != Color.WHITE and map._marks.modulate == Color.WHITE, "night grades the chart but not the marks")
	var cont := _control(map, "expedition.rest.continue")
	_expect(cont != null and viewport.encloses(cont.get_global_rect()), "the rest summary's Continue is reachable at %s" % label)
	await _click(cont)
	for frame in 2: await process_frame
	_expect(not map._overlay.visible and not map._scrim.visible, "dismissing the summary clears the scrim")
	map.queue_free(); await process_frame
