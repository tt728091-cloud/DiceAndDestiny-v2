extends Control
## The read-only quest graph viewer, opened from Character Creation (the admin
## editor). Left: every quest the authority's quest_catalog describes, grouped
## by set, with kind and status chips. Right: the selected quest's header and
## its graph (quest_graph_canvas.gd), and the details of the stage under the
## pointer or clicked: its full text, outcomes and consequences, each with its
## engine status. The client never interprets verbs; it shows the describer's
## strings. No editing, no saving.
signal closed

const STYLE := preload("res://app/screens/character/character_style.gd")
const CANVAS := preload("res://app/screens/quests/quest_graph_canvas.gd")
const KIND_NAMES := {"main": "Main quest", "side": "Side quest", "small": "Small quest", "personal": "Personal quest"}
const STATUS_COLOURS := {"scenario": STYLE.GAIN, "paper": STYLE.AMBER}

var catalog: Dictionary = {}
var selected_quest := ""
var selected_node := ""
var hovered_node := ""
## Quest id -> its button in the list.
var quest_buttons: Dictionary = {}
var canvas: Control
var graph_scroll: ScrollContainer
var _list_box: VBoxContainer
var _header: VBoxContainer
var _details: VBoxContainer
var _message: Label

func _ready() -> void:
	name = "QuestGraphViewer"
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); mouse_filter = Control.MOUSE_FILTER_STOP
	theme = STYLE.theme()
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 18)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 10); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 14); body.add_child(header)
	var titles := VBoxContainer.new(); titles.add_theme_constant_override("separation", 0); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; header.add_child(titles)
	STYLE.label(titles, "DICE & DESTINY  /  QUESTS", 13, STYLE.GOLD)
	STYLE.label(titles, "Quest graphs · read-only", 26)
	var note := STYLE.label(header, "Description only: the Go rules still run the scenario.", 14, STYLE.MUTED)
	note.size_flags_vertical = Control.SIZE_SHRINK_CENTER; note.size_flags_horizontal = Control.SIZE_EXPAND_FILL; note.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	var back := _button(header, "Back to characters", func(): closed.emit(); queue_free(), "quests.back"); back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_message = STYLE.label(body, "", 15, STYLE.LOSS); _message.visible = false
	var split := HSplitContainer.new(); split.size_flags_vertical = Control.SIZE_EXPAND_FILL; split.add_theme_constant_override("separation", 12); body.add_child(split)
	# Left: the quest list by set.
	var left := PanelContainer.new(); left.custom_minimum_size.x = 290; left.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 10, 10)); split.add_child(left)
	var list_scroll := ScrollContainer.new(); list_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; left.add_child(list_scroll)
	_list_box = VBoxContainer.new(); _list_box.name = "QuestList"; _list_box.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _list_box.add_theme_constant_override("separation", 6); list_scroll.add_child(_list_box)
	# Right: the quest's header, its graph and the details under it.
	var right := VBoxContainer.new(); right.size_flags_horizontal = Control.SIZE_EXPAND_FILL; right.add_theme_constant_override("separation", 8); split.add_child(right)
	var head_panel := PanelContainer.new(); head_panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 12, 10)); right.add_child(head_panel)
	_header = VBoxContainer.new(); _header.name = "QuestHeader"; _header.add_theme_constant_override("separation", 4); head_panel.add_child(_header)
	var lower := VSplitContainer.new(); lower.size_flags_vertical = Control.SIZE_EXPAND_FILL; right.add_child(lower)
	var graph_panel := PanelContainer.new(); graph_panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; graph_panel.custom_minimum_size.y = 180
	graph_panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.NIGHT, "22323f", 0, 10)); lower.add_child(graph_panel)
	graph_scroll = ScrollContainer.new(); graph_scroll.name = "GraphScroll"; graph_panel.add_child(graph_scroll)
	canvas = CANVAS.new(); canvas.name = "QuestGraph"; graph_scroll.add_child(canvas)
	canvas.node_selected.connect(func(id): selected_node = id; canvas.select(id); _render_details())
	canvas.node_hovered.connect(func(id): hovered_node = id; _render_details())
	graph_panel.size_flags_stretch_ratio = 1.4
	var details_panel := PanelContainer.new(); details_panel.custom_minimum_size.y = 150; details_panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; details_panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 12, 10)); lower.add_child(details_panel)
	var details_scroll := ScrollContainer.new(); details_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; details_panel.add_child(details_scroll)
	_details = VBoxContainer.new(); _details.name = "Details"; _details.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _details.add_theme_constant_override("separation", 4); details_scroll.add_child(_details)
	var legend := HBoxContainer.new(); legend.add_theme_constant_override("separation", 10); body.add_child(legend)
	for entry in [["Success", STYLE.GAIN], ["Failure", STYLE.LOSS], ["Abandoned", STYLE.MUTED]]: STYLE.chip(legend, "— " + entry[0], entry[1], 12)
	var hint := STYLE.label(legend, "Hover or click a stage for its outcomes; hover an edge label for its consequences. Red chips mark verbs not yet implemented.", 13, STYLE.DIM)
	hint.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_load()

func _button(parent: Node, text: String, action: Callable, key: String) -> Button:
	var b := Button.new(); b.text = text; b.set_meta("quest_control", key); b.pressed.connect(action); b.focus_mode = Control.FOCUS_NONE; parent.add_child(b); return b

static func _list(source: Dictionary, key: String) -> Array:
	var value = source.get(key)
	return value if value is Array else []

func _load() -> void:
	var runtime = get_node_or_null("/root/LearnedBattleRuntime")
	var response: Dictionary = runtime.quest_catalog() if runtime != null else {"ok": false, "error": "The battle runtime is unavailable."}
	if response.get("ok") != true:
		_message.text = str(response.get("error", "The quest catalog could not be loaded.")); _message.visible = true
		return
	catalog = response.result
	_render_list()
	var sets: Array = _list(catalog, "sets")
	if not sets.is_empty() and not _list(sets[0], "quests").is_empty(): select_quest(str(sets[0].quests[0]))

func quest(id: String) -> Dictionary:
	for q in _list(catalog, "quests"):
		if str(q.id) == id: return q
	return {}

func _render_list() -> void:
	for child in _list_box.get_children(): child.queue_free()
	quest_buttons.clear()
	for set in _list(catalog, "sets"):
		var heading := STYLE.heading(_list_box, str(set.name)); heading.name = "Set_" + str(set.id)
		STYLE.label(_list_box, str(set.get("text", "")), 12, STYLE.DIM)
		for id in _list(set, "quests"):
			var q := quest(str(id))
			var item := VBoxContainer.new(); item.add_theme_constant_override("separation", 2); _list_box.add_child(item)
			var button := _button(item, str(q.get("name", id)), func(): select_quest(str(id)), "quests.quest." + str(id))
			button.toggle_mode = true; button.alignment = HORIZONTAL_ALIGNMENT_LEFT
			button.add_theme_font_size_override("font_size", 15)
			quest_buttons[str(id)] = button
			var chips := HFlowContainer.new(); chips.add_theme_constant_override("h_separation", 4); item.add_child(chips)
			STYLE.chip(chips, KIND_NAMES.get(str(q.get("kind", "")), str(q.get("kind", ""))), STYLE.GOLD, 11)
			STYLE.chip(chips, str(q.get("status", "")).capitalize(), STATUS_COLOURS.get(str(q.get("status", "")), STYLE.MUTED), 11)
		var gap := Control.new(); gap.custom_minimum_size.y = 6; _list_box.add_child(gap)

func select_quest(id: String) -> void:
	var q := quest(id)
	if q.is_empty(): return
	selected_quest = id; selected_node = str(q.get("start", "")); hovered_node = ""
	for key in quest_buttons: quest_buttons[key].set_pressed_no_signal(key == id)
	_render_header(q)
	canvas.show_quest(q, selected_node)
	graph_scroll.scroll_horizontal = 0; graph_scroll.scroll_vertical = 0
	_render_details()

func _render_header(q: Dictionary) -> void:
	for child in _header.get_children(): child.queue_free()
	var row := HFlowContainer.new(); row.add_theme_constant_override("h_separation", 8); row.add_theme_constant_override("v_separation", 4); _header.add_child(row)
	var title := STYLE.label(row, str(q.get("name", "")), 20, STYLE.GOLD_BRIGHT); title.autowrap_mode = TextServer.AUTOWRAP_OFF; title.name = "QuestName"
	STYLE.chip(row, KIND_NAMES.get(str(q.get("kind", "")), ""), STYLE.GOLD, 12)
	var status := STYLE.chip(row, str(q.get("status", "")).capitalize(), STATUS_COLOURS.get(str(q.get("status", "")), STYLE.MUTED), 12)
	status.tooltip_text = str(q.get("status_text", "")); status.mouse_filter = Control.MOUSE_FILTER_STOP
	if not str(q.get("region_name", "")).is_empty(): STYLE.chip(row, str(q.region_name), STYLE.ENERGY, 12)
	STYLE.label(_header, str(q.get("status_text", "")), 12, STATUS_COLOURS.get(str(q.get("status", "")), STYLE.MUTED))
	var hook: Dictionary = q.get("hook", {})
	_engine_line(_header, "Hook: " + str(hook.get("text", "")), hook, 13, STYLE.IVORY)
	var triggers: Array = _list(q, "trigger").map(func(t): return str(t.get("text", "")))
	var trigger_line: Dictionary = {}
	for t in _list(q, "trigger"):
		if str(t.get("engine", "")) != "scenario": trigger_line = t
	_engine_line(_header, "Starts when: " + " and ".join(triggers), trigger_line, 13, STYLE.MUTED)
	var roles: Array = _list(q, "roles").map(func(r): return "{%s} (%s): %s" % [str(r.name), str(r.type), str(r.text)])
	if not roles.is_empty(): STYLE.label(_header, "Roles: " + " · ".join(roles), 12, STYLE.DIM)

## A line of text with a red "not yet implemented" note when its verb is.
func _engine_line(parent: Node, text: String, line: Dictionary, font_size: int, colour: Color) -> Label:
	var suffix := "" if str(line.get("engine", "scenario")) == "scenario" else "  [%s]" % str(line.get("engine_text", ""))
	var label := STYLE.label(parent, text + suffix, font_size, colour if suffix.is_empty() else colour.lerp(STYLE.LOSS, 0.35))
	return label

func _edges_from(q: Dictionary, id: String) -> Array:
	return _list(q, "edges").filter(func(e): return str(e.from) == id)

func _render_details() -> void:
	for child in _details.get_children(): child.queue_free()
	var q := quest(selected_quest)
	var id := hovered_node if not hovered_node.is_empty() else selected_node
	var node: Dictionary = {}
	for n in _list(q, "nodes"):
		if str(n.id) == id: node = n
	if node.is_empty(): return
	if str(node.get("kind", "")) == "end":
		STYLE.label(_details, "%s: the quest ends here." % str(node.get("title", "")), 15, CANVAS.OUTCOME_COLOURS.get(str(node.get("outcome", "")), STYLE.IVORY))
		for edge in _list(q, "edges"):
			if str(edge.to) == id: _outcome_lines(edge)
		return
	STYLE.label(_details, str(node.get("title", "")), 16, STYLE.GOLD_BRIGHT).name = "DetailTitle"
	if not str(node.get("text", "")).is_empty(): STYLE.label(_details, str(node.text), 13, STYLE.IVORY)
	_engine_line(_details, "Objective: " + str(node.get("objective", {}).get("text", "")), node.get("objective", {}), 13, STYLE.IVORY)
	_engine_line(_details, "Timer: " + str(node.get("timer", {}).get("text", "")), node.get("timer", {}), 13, STYLE.MUTED)
	for edge in _edges_from(q, id): _outcome_lines(edge)

func _outcome_lines(edge: Dictionary) -> void:
	var colour: Color = CANVAS.OUTCOME_COLOURS.get(str(edge.get("outcome", "")), STYLE.IVORY)
	var when: Dictionary = edge.get("when", {})
	_engine_line(_details, "%s · %s — when: %s" % [str(edge.get("outcome", "")).capitalize(), str(edge.get("label", "")), str(when.get("text", ""))], when, 13, colour).name = "Outcome"
	if not str(edge.get("text", "")).is_empty(): STYLE.label(_details, "    " + str(edge.text), 12, STYLE.MUTED)
	for c in _list(edge, "consequences"): _engine_line(_details, "    • " + str(c.get("text", "")), c, 12, STYLE.IVORY)
	var next := str(edge.get("to", ""))
	if not next.begins_with("end:"): STYLE.label(_details, "    → next stage: %s" % next, 12, STYLE.DIM)

func _unhandled_key_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and event.keycode == KEY_ESCAPE:
		get_viewport().set_input_as_handled(); closed.emit(); queue_free()
