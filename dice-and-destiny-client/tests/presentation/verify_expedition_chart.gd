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
	_expect(radius >= 28.0 and radius <= 48.0, "the hex radius is clamped to 28–48 at %s (%.1f)" % [label, radius])
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
	# The chart layers and model.
	_expect(map.map_model.hexes.size() == 7 and map.map_model.slots.size() == 8 and map.map_model.sight.size() == 7, "the map model holds the start tile, the slots and the sight set")
	_expect(map._marks.mouse_filter == Control.MOUSE_FILTER_IGNORE and map._lines.mouse_filter == Control.MOUSE_FILTER_IGNORE and map._sheet.mouse_filter == Control.MOUSE_FILTER_IGNORE, "the chart layers never take the mouse")
	_expect(not map._marks.is_processing(), "the marks layer idles while nothing is selected or hovered")
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
	await _click(brk)
	for frame in 4: await process_frame
	_expect(map.view.night and map._chart.modulate != Color.WHITE and map._marks.modulate == Color.WHITE, "night grades the chart but not the marks")
	var cont := _control(map, "expedition.rest.continue")
	_expect(cont != null and viewport.encloses(cont.get_global_rect()), "the rest summary's Continue is reachable at %s" % label)
	await _click(cont)
	for frame in 2: await process_frame
	_expect(not map._overlay.visible and not map._scrim.visible, "dismissing the summary clears the scrim")
	map.queue_free(); await process_frame
