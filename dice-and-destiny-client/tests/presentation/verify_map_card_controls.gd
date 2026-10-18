extends "res://tests/presentation/verify_campaign.gd"
## The map card controls by pointer, on a real expedition view: a card that
## needs a target offers the authority's targets; the look dialog builds the
## draw, bottom and top order the authority receives; a provoked fight offers
## Fall back; energy pays for Influence at a site. The screen records the
## actions it would send instead of sending them.

const EXPEDITION_SCRIPT := "res://app/screens/map/expedition_screen.gd"

class RecordingScreen extends "res://app/screens/map/expedition_screen.gd":
	var acted: Array = []
	func _act(action: Dictionary) -> bool:
		acted.append(action)
		return true

var screen

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	var created: Dictionary = runtime.campaign_new("starter", "Map cards", "expedition", "", 2)
	_expect(created.get("ok") == true, "an expedition starts")
	if created.get("ok") != true: _done(); return
	var id := str(created.result.id)
	var status: Dictionary = runtime.expedition_status(id).result
	var base: Dictionary = status.view.duplicate(true)
	screen = RecordingScreen.new(); screen.save_id = id; root.add_child(screen)
	for frame in 8: await process_frame
	# 1. A target card: the play button opens the terrain list.
	var view: Dictionary = base.duplicate(true)
	view.hand = [{"id": "pathfinder_paths_variant_node_4", "name": "Know the Land", "map_effect": "You know this ground.", "map_only": true, "map_playable": true, "energy_cost": 5, "map_kind": "move", "targets": [{"id": "forest", "label": "Forest"}, {"id": "hills", "label": "Hills"}]}]
	status.view = view; screen._apply(status)
	for frame in 3: await process_frame
	await _click(_screen_control("expedition.play.0"))
	for frame in 3: await process_frame
	var menu: PopupMenu = screen.get_node_or_null("MapTargets")
	_expect(menu != null and menu.item_count == 2 and menu.get_item_text(0) == "Forest", "the play button lists the terrains to choose from")
	if menu != null:
		menu.id_pressed.emit(1)
		for frame in 2: await process_frame
	_expect(not screen.acted.is_empty() and screen.acted[-1] == {"kind": "play_map_card", "card": "pathfinder_paths_variant_node_4", "choice": "hills"}, "choosing Hills plays the card with that terrain: %s" % [screen.acted])
	# 2. The look dialog: draw the second card, bottom the first, the third on top.
	view = base.duplicate(true)
	view.look = {"card": "foresight_paths_variant_node_4", "card_name": "Clear the Path", "draw": 1, "bottom": true, "cards": [{"id": "nudge", "name": "Nudge"}, {"id": "strong_swing", "name": "Strong Swing"}, {"id": "try_again", "name": "Try Again"}]}
	status.view = view; screen._apply(status)
	for frame in 3: await process_frame
	_expect(screen._overlay.visible and "Clear the Path · the top 3 of your draw pile" in _text(screen._overlay), "the look dialog shows the top of the draw pile")
	var done := _screen_control("expedition.look.done")
	_expect(done != null and done.disabled, "Done waits for the card to draw")
	await _click(_screen_control("expedition.look.draw.1"))
	await _click(_screen_control("expedition.look.bottom.0"))
	await _click(_screen_control("expedition.look.top.2"))
	for frame in 2: await process_frame
	_expect("Strong Swing  →  drawn" in _text(screen._overlay) and "Nudge  →  bottom" in _text(screen._overlay) and "Try Again  →  back on top · 1" in _text(screen._overlay), "the dialog shows the arrangement: %s" % _text(screen._overlay).replace("\n", " | "))
	await _click(_screen_control("expedition.look.done"))
	_expect(screen.acted[-1] == {"kind": "resolve_look", "card": "strong_swing", "cards": ["try_again"], "bottom": ["nudge"]}, "Done sends the arrangement: %s" % [screen.acted[-1]])
	# 3. A provoked fight offers Fall back.
	view = base.duplicate(true)
	view.pending_battle = {"id": "b", "kind": "provoked", "name": "Brine Mask", "opponent": "drowned_oracle_brine_mask", "count": 1, "enemy_statuses": [], "player_statuses": [], "energy_in": 10}
	view.hand = [{"id": "unseen_paths_variant_node_1", "name": "Fall Back", "map_effect": "Not this fight, not here.", "map_only": true, "map_playable": true, "map_kind": "fall_back"}]
	status.view = view; screen._apply(status)
	for frame in 3: await process_frame
	var back := _screen_control("expedition.fall_back")
	_expect(back != null and "Fall Back" in back.text, "the provoked fight offers Fall back")
	if back != null:
		await _click(back)
		_expect(screen.acted[-1] == {"kind": "play_map_card", "card": "unseen_paths_variant_node_1"}, "Fall back plays the card: %s" % [screen.acted[-1]])
	# 4. Energy for Influence at a site.
	view = base.duplicate(true)
	view.energy = 10
	view.site_here = {"id": "test_village", "name": "Test Village", "type": "village", "open": true, "description": "", "services": []}
	status.view = view; screen._apply(status)
	for frame in 3: await process_frame
	var buy := _screen_control("expedition.site.energy_influence")
	_expect(buy != null and not buy.disabled and "5 energy for 1 Influence" in buy.text, "a site sells Influence for energy")
	if buy != null:
		await _tap(buy)
		_expect(screen.acted[-1] == {"kind": "use_site", "service": "influence", "energy": 5}, "the button pays 5 energy: %s" % [screen.acted[-1]])
	screen.queue_free()
	runtime.campaign_delete(id)
	_done()

func _screen_control(key: String) -> Button:
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("expedition_control", "") == key and not button.is_queued_for_deletion() and button.is_visible_in_tree(): return button
	return null

func _done() -> void:
	print("MAP CARD CONTROLS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
