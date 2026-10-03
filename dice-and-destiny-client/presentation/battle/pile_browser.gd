extends Control
## Read-only inventory from the viewer's authority compositions, never draw order.
signal closed
signal pile_changed(zone: String)
const STYLE := preload("res://presentation/battle/cinematic_theme.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const PILES := {"deck": "Draw pile", "discard": "Discard pile", "removed": "Removed pile"}
var zone := ""
var actor: Dictionary
var panel: PanelContainer
var rows: VBoxContainer
var scroll: ScrollContainer
var preview: BattleCard
var preview_slot: Control
var preview_hint: Label
var tabs: Dictionary = {}
var close_button: Button
var heading: Label

func _ready() -> void:
	name = "PileBrowser"
	z_index = 90
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var shade := ColorRect.new(); shade.color = Color("00000080"); shade.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(shade); shade.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	gui_input.connect(func(event: InputEvent):
		if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
			accept_event(); closed.emit()
	)
	panel = PanelContainer.new(); panel.mouse_filter = Control.MOUSE_FILTER_STOP
	panel.add_theme_stylebox_override("panel", STYLE.panel(Color("10171dfa"), STYLE.GOLD, 20))
	add_child(panel); panel.position = Vector2(550, 210); panel.size = Vector2(820, 620)
	var frame := VBoxContainer.new(); frame.add_theme_constant_override("separation", 14); panel.add_child(frame)
	var top := HBoxContainer.new(); frame.add_child(top)
	heading = Label.new(); heading.add_theme_font_size_override("font_size", 28); heading.size_flags_horizontal = Control.SIZE_EXPAND_FILL; top.add_child(heading)
	close_button = Button.new(); close_button.text = "Close"; close_button.pressed.connect(func(): closed.emit()); top.add_child(close_button)
	var navigation := HBoxContainer.new(); navigation.add_theme_constant_override("separation", 10); frame.add_child(navigation)
	for key in PILES:
		var tab := Button.new(); tab.icon = ICONS.texture(key); tab.expand_icon = true; tab.add_theme_constant_override("icon_max_width", 20)
		tab.toggle_mode = true; tab.size_flags_horizontal = Control.SIZE_EXPAND_FILL; navigation.add_child(tab); tabs[key] = tab
		tab.pressed.connect(func(): _show_zone(key); pile_changed.emit(key))
	var hint := Label.new(); hint.text = "Cards listed alphabetically · Hover a card to inspect"; hint.add_theme_font_size_override("font_size", 16); frame.add_child(hint)
	var body := HBoxContainer.new(); body.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_theme_constant_override("separation", 24); frame.add_child(body)
	scroll = ScrollContainer.new(); scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; scroll.follow_focus = true; body.add_child(scroll)
	rows = VBoxContainer.new(); rows.size_flags_horizontal = Control.SIZE_EXPAND_FILL; rows.add_theme_constant_override("separation", 2); scroll.add_child(rows)
	preview_slot = Control.new(); preview_slot.custom_minimum_size = BattleCard.STANDARD_SIZE; body.add_child(preview_slot)
	preview_hint = Label.new(); preview_hint.text = "Hover a card\nto see its details"; preview_hint.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; preview_hint.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	preview_slot.add_child(preview_hint); preview_hint.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)

func configure(owner: Dictionary, selected_zone: String) -> void:
	actor = owner
	_show_zone(selected_zone)

func _show_zone(selected_zone: String) -> void:
	zone = selected_zone
	_clear_preview()
	for row in rows.get_children(): rows.remove_child(row); row.queue_free()
	scroll.scroll_vertical = 0
	for key in tabs:
		tabs[key].text = "%s · %d" % [PILES[key], int(actor.get(key + "_count", 0))]
		tabs[key].set_pressed_no_signal(key == zone)
	heading.text = "%s · %d cards" % [PILES[zone], int(actor.get(zone + "_count", 0))]
	var composition: Dictionary = actor.get(zone + "_composition", {}) if actor.get(zone + "_composition") is Dictionary else {}
	var definitions := composition.keys()
	definitions.sort_custom(func(a, b): return str(BattlePresentationCatalog.card(a).name).naturalnocasecmp_to(str(BattlePresentationCatalog.card(b).name)) < 0)
	for definition in definitions:
		var data := BattlePresentationCatalog.card(definition)
		for copy in int(composition[definition]):
			var row := Button.new(); row.text = "%d✦   %s" % [int(data.cost), data.name]; row.alignment = HORIZONTAL_ALIGNMENT_LEFT
			row.add_theme_font_size_override("font_size", 18)
			row.custom_minimum_size.y = 32; row.clip_text = true; row.set_meta("definition_id", definition)
			for state in ["normal", "hover", "pressed", "focus"]:
				var paper := STYLE.paper(Color("fff0c9") if state != "normal" else Color("ddd0b9"))
				paper.content_margin_top = 2; paper.content_margin_bottom = 2; paper.content_margin_left = 8; paper.content_margin_right = 8
				row.add_theme_stylebox_override(state, paper)
			for state in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color"]: row.add_theme_color_override(state, STYLE.INK)
			row.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
			row.mouse_entered.connect(_show_preview.bind(str(definition))); row.mouse_exited.connect(_clear_preview)
			row.focus_entered.connect(_show_preview.bind(str(definition))); row.focus_exited.connect(_clear_preview)
			rows.add_child(row)
	if rows.get_child_count() == 0:
		var empty := Label.new(); empty.text = "This pile is empty." if int(actor.get(zone + "_count", 0)) == 0 else "Card details are unavailable."; rows.add_child(empty)

func _show_preview(definition: String) -> void:
	_clear_preview()
	preview_hint.hide()
	preview = BattleCard.new(); preview_slot.add_child(preview); preview.configure("", definition, false)
	preview.tooltip_text = ""; preview.mouse_filter = Control.MOUSE_FILTER_IGNORE; preview.size = BattleCard.STANDARD_SIZE

func _clear_preview() -> void:
	if is_instance_valid(preview): preview.hide(); preview.queue_free()
	preview = null
	if is_instance_valid(preview_hint): preview_hint.show()

func _unhandled_key_input(event: InputEvent) -> void:
	if is_visible_in_tree() and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled(); closed.emit()
