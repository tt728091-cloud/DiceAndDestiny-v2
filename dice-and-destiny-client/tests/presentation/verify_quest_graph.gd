extends "res://tests/presentation/verify_character_creation.gd"
## The read-only quest graph viewer by pointer: Character Creation's Quests
## tab opens it; both sets and every shipped quest are listed; the kidnapped
## daughter's failure edge leads to the werewolf stage; the main quest's timer
## chip says urgent with 4 rounds; hovering and clicking a stage shows its
## outcomes; no two cards overlap, every card fits inside the viewer once
## scrolled to, and no text runs outside its card; at 1280×720 and 1920×1080.

const QUESTS := ["stop_the_drowned_regent", "hunt_the_bell_marshal", "take_brinehold_keep", "free_saltreed", "the_wayside_shrine", "the_wrecked_barge", "the_hermits_fire", "kidnapped_daughter", "spring_farms"]

func _run() -> void:
	root.gui_embed_subwindows = true
	for size in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		await _pass(size)
	print("QUEST GRAPH: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _click_at(point: Vector2) -> void:
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	for frame in 3: await process_frame

func _hover(point: Vector2) -> void:
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true)
	for frame in 3: await process_frame

## Scrolls a control into view inside its ScrollContainer, as a player would.
func _reveal(control: Control) -> void:
	var scroll := control.get_parent()
	while scroll != null and not scroll is ScrollContainer: scroll = scroll.get_parent()
	if scroll != null: scroll.ensure_control_visible(control)
	for frame in 3: await process_frame

func _open_quest(viewer: Control, id: String) -> void:
	var button: Button = viewer.quest_buttons.get(id)
	_expect(button != null, "quest %s has a list button" % id)
	if button == null: return
	await _reveal(button)
	await _click(button)
	for frame in 6: await process_frame
	_expect(viewer.selected_quest == id and button.button_pressed, "clicking %s selects it" % id)

func _pass(size: Vector2i) -> void:
	root.size = size
	for frame in 3: await process_frame
	var label := "%dx%d" % [size.x, size.y]
	var screen = SCREEN.new(); root.add_child(screen)
	for frame in 8: await process_frame
	_expect(screen._tabs.get_tab_title(6) == "Quests", "Character Creation has a Quests tab beside the workshops (%s)" % screen._tabs.get_tab_title(6))
	var tab_point: Vector2 = screen._tabs.get_global_transform() * screen._tabs.get_tab_rect(6).get_center()
	_expect(root.get_visible_rect().has_point(tab_point), "the Quests tab is on screen at %s" % label)
	await _click_at(tab_point)
	for frame in 8: await process_frame
	var viewer: Control = screen.get_node_or_null("QuestGraphViewer")
	_expect(viewer != null, "the Quests tab opens the quest graph viewer at %s" % label)
	if viewer == null: screen.queue_free(); await process_frame; return
	_expect(screen._tabs.current_tab != 6, "the tab bar stays on the character's tab")
	_expect(not viewer._message.visible, "the catalog loaded: %s" % viewer._message.text)
	# Both sets and every shipped quest.
	var list_text := _text(viewer._list_box)
	_expect("THE SALT MARCHES" in list_text and "PAPER QUESTS" in list_text, "both sets are listed: %s" % list_text.left(120))
	_expect(viewer.quest_buttons.size() == QUESTS.size(), "every shipped quest and no other is listed (%d)" % viewer.quest_buttons.size())
	for id in QUESTS: _expect(viewer.quest_buttons.has(id), "%s is listed" % id)
	_expect("Main quest" in list_text and "Paper" in list_text and "Scenario" in list_text, "the list shows kind and status chips")
	# The kidnapped daughter: the failure edge leads to the werewolf stage.
	await _open_quest(viewer, "kidnapped_daughter")
	var failure: Dictionary = {}
	for edge in viewer.quest("kidnapped_daughter").get("edges", []):
		if str(edge.from) == "rescue" and str(edge.outcome) == "failure": failure = edge
	_expect(str(failure.get("to", "")) == "turned", "the rescue's failure edge leads to the werewolf stage: %s" % [failure])
	var rescue: Control = viewer.canvas.cards.get("rescue")
	var turned: Control = viewer.canvas.cards.get("turned")
	var path: PackedVector2Array = viewer.canvas.paths.get(str(failure.get("id", "")), PackedVector2Array())
	_expect(rescue != null and turned != null and "She has turned" in _text(turned), "the werewolf stage has its card")
	if rescue != null and turned != null and path.size() >= 2:
		_expect(rescue.get_rect().grow(1).has_point(path[0]) and turned.get_rect().grow(1).has_point(path[path.size() - 1]) and absf(path[path.size() - 1].x - turned.position.x) < 1.0, "the drawn failure edge runs out of the rescue card into the werewolf card (%s)" % [path])
	else: _expect(false, "the failure edge is drawn")
	var chip: Control = viewer.canvas.labels.get(str(failure.get("id", "")))
	_expect(chip != null and "Too late" in _text(chip), "the failure edge carries its label")
	await _check_layout(viewer, label + " kidnapped daughter")
	# Hovering a stage shows its full text, outcomes and consequences.
	if turned != null:
		await _reveal(turned)
		await _hover(turned.get_global_rect().get_center())
		var details := _text(viewer._details)
		_expect(viewer.hovered_node == "turned" and "She has turned" in details and "Cure her" in details and "Hunt the beast" in details and "reputation with {person}" in details and "Not yet implemented" in details, "hovering the werewolf stage shows its outcomes and consequences: %s" % details.strip_edges().replace("\n", " | ").left(200))
		await _capture(viewer, "quest-graph-daughter-" + label)
	# The main quest: urgent, 4 rounds; clicking the deadline stage selects it.
	await _open_quest(viewer, "stop_the_drowned_regent")
	var stop: Control = viewer.canvas.cards.get("stop_him")
	var timer_chip: Label = stop.find_child("TimerChip", true, false) if stop != null else null
	_expect(timer_chip != null and "Urgent" in timer_chip.text and "4 rounds" in timer_chip.text, "the main quest's timer chip says urgent with 4 rounds (%s)" % (timer_chip.text if timer_chip != null else "none"))
	await _check_layout(viewer, label + " main quest")
	var deadline: Control = viewer.canvas.cards.get("deadline")
	if deadline != null:
		await _reveal(deadline)
		await _click_at(deadline.get_global_rect().get_center())
		await _hover(viewer.get_global_rect().end - Vector2(4, 4))
		var details := _text(viewer._details)
		_expect(viewer.selected_node == "deadline" and "The deadline fight" in details and "Lost the deadline fight" in details and "The run is over" in details, "clicking the deadline stage shows its outcomes: %s" % details.strip_edges().replace("\n", " | ").left(200))
	await _capture(viewer, "quest-graph-main-" + label)
	# Every quest lays out cleanly.
	for id in QUESTS:
		if id in ["kidnapped_daughter", "stop_the_drowned_regent"]: continue
		await _open_quest(viewer, id)
		await _check_layout(viewer, "%s %s" % [label, id])
	# Back returns to Character Creation.
	var back: Button = null
	for b in viewer.find_children("*", "Button", true, false):
		if b.get_meta("quest_control", "") == "quests.back": back = b
	await _click(back)
	for frame in 3: await process_frame
	_expect(screen.get_node_or_null("QuestGraphViewer") == null and is_instance_valid(screen), "Back returns to Character Creation")
	screen.queue_free(); await process_frame

## No two cards overlap; labels overlap neither cards nor each other; every
## card and label text stays inside its card; every card fits inside the
## viewer once scrolled to; the graph area is inside the viewport.
func _check_layout(viewer: Control, label: String) -> void:
	var cards: Dictionary = viewer.canvas.cards
	var labels: Dictionary = viewer.canvas.labels
	_expect(not cards.is_empty(), "the graph has cards (%s)" % label)
	var ids: Array = cards.keys()
	for i in ids.size():
		for j in range(i + 1, ids.size()):
			if cards[ids[i]].get_rect().intersects(cards[ids[j]].get_rect()):
				_expect(false, "cards %s and %s overlap (%s)" % [ids[i], ids[j], label])
	var chips: Array = labels.keys()
	for i in chips.size():
		var chip: Control = labels[chips[i]]
		for id in ids:
			if chip.get_rect().intersects(cards[id].get_rect()): _expect(false, "edge label %s overlaps card %s (%s)" % [chips[i], id, label])
		for j in range(i + 1, chips.size()):
			if chip.get_rect().intersects(labels[chips[j]].get_rect()): _expect(false, "edge labels %s and %s overlap (%s)" % [chips[i], chips[j], label])
	for container in cards.values() + labels.values():
		for text in container.find_children("*", "Label", true, false):
			if not container.get_global_rect().grow(0.5).encloses(text.get_global_rect()):
				_expect(false, "text runs outside its card in %s (%s): %s" % [container.name, label, text.text.left(40)])
	var bounds := Rect2(Vector2.ZERO, viewer.canvas.custom_minimum_size)
	for id in ids: _expect(bounds.encloses(cards[id].get_rect()), "card %s lies inside the graph (%s)" % [id, label])
	var scroll: ScrollContainer = viewer.graph_scroll
	_expect(root.get_visible_rect().encloses(scroll.get_global_rect()), "the graph area is inside the viewport (%s)" % label)
	var larger: bool = viewer.canvas.custom_minimum_size.x > scroll.size.x or viewer.canvas.custom_minimum_size.y > scroll.size.y
	if larger: _expect(scroll.get_h_scroll_bar().max_value > 0 or scroll.get_v_scroll_bar().max_value > 0, "a graph larger than its area scrolls (%s)" % label)
	for id in ids:
		scroll.ensure_control_visible(cards[id])
		for frame in 2: await process_frame
		_expect(scroll.get_global_rect().grow(1).encloses(cards[id].get_global_rect()), "card %s fits inside the viewer once scrolled to (%s): %s in %s" % [id, label, cards[id].get_global_rect(), scroll.get_global_rect()])
	scroll.scroll_horizontal = 0; scroll.scroll_vertical = 0
	for frame in 2: await process_frame
