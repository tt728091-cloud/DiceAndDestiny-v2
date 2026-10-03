extends Control
## Read-only, authority-owned committed losses. One group per hit, never per round.
signal closed
const STYLE := preload("res://presentation/battle/cinematic_theme.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const TIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
var snapshot: Dictionary
var actor_names: Dictionary
var selected_actor := ""
var panel: PanelContainer
var rows: VBoxContainer
var scroll: ScrollContainer
var summary: Label
var tabs: Dictionary = {}
var preview_slot: Control
var preview: BattleCard
var close_button: Button

func _ready() -> void:
	name = "WoundReview"
	z_index = 100
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var shade := ColorRect.new(); shade.color = Color("000000b0"); shade.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(shade); shade.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	panel = PanelContainer.new(); panel.mouse_filter = Control.MOUSE_FILTER_STOP
	panel.add_theme_stylebox_override("panel", STYLE.panel(Color("10171d"), STYLE.GOLD, 24))
	add_child(panel); panel.position = Vector2(350, 120); panel.size = Vector2(1220, 840)
	var frame := VBoxContainer.new(); frame.add_theme_constant_override("separation", 16); panel.add_child(frame)
	var top := HBoxContainer.new(); frame.add_child(top)
	var title := Label.new(); title.text = "Battle Review · Wounds"; title.add_theme_font_size_override("font_size", 30); title.add_theme_color_override("font_color", STYLE.GOLD)
	title.size_flags_horizontal = Control.SIZE_EXPAND_FILL; top.add_child(title)
	close_button = Button.new(); close_button.text = "Close"; close_button.custom_minimum_size = Vector2(100, 44)
	close_button.pressed.connect(func(): closed.emit()); top.add_child(close_button)
	var note := Label.new(); note.text = "Each wound is one hit after prevention. Only cards actually lost are listed."; note.add_theme_font_size_override("font_size", 18); frame.add_child(note)
	var navigation := HBoxContainer.new(); navigation.name = "Characters"; navigation.add_theme_constant_override("separation", 8); frame.add_child(navigation)
	for actor_id in actor_names:
		var tab := Button.new(); tab.text = str(actor_names[actor_id]); tab.toggle_mode = true
		tab.size_flags_horizontal = Control.SIZE_EXPAND_FILL; tab.custom_minimum_size.y = 44
		navigation.add_child(tab); tabs[actor_id] = tab; tab.pressed.connect(_show_actor.bind(str(actor_id)))
	summary = Label.new(); summary.add_theme_font_size_override("font_size", 24); summary.add_theme_color_override("font_color", Color("f1b4a3")); frame.add_child(summary)
	var body := HBoxContainer.new(); body.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_theme_constant_override("separation", 24); frame.add_child(body)
	scroll = ScrollContainer.new(); scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; scroll.follow_focus = true; body.add_child(scroll)
	rows = VBoxContainer.new(); rows.size_flags_horizontal = Control.SIZE_EXPAND_FILL; rows.add_theme_constant_override("separation", 16); scroll.add_child(rows)
	var preview_column := VBoxContainer.new(); preview_column.custom_minimum_size.x = BattleCard.STANDARD_SIZE.x; body.add_child(preview_column)
	var hint := Label.new(); hint.text = "Hover a lost card to inspect"; hint.add_theme_font_size_override("font_size", 16); preview_column.add_child(hint)
	preview_slot = Control.new(); preview_slot.custom_minimum_size = BattleCard.STANDARD_SIZE; preview_column.add_child(preview_slot)
	var footer := Label.new(); footer.text = "Wounds are recorded separately for future recovery. Healing is not available yet."; footer.add_theme_font_size_override("font_size", 16); frame.add_child(footer)
	_show_actor(selected_actor)
	close_button.grab_focus()

func configure(data: Dictionary, names: Dictionary, viewer: String) -> void:
	snapshot = data.duplicate(true); actor_names = names.duplicate(); selected_actor = viewer

func _show_actor(actor_id: String) -> void:
	selected_actor = actor_id
	_clear_preview()
	for child in rows.get_children(): rows.remove_child(child); child.queue_free()
	scroll.scroll_vertical = 0
	for key in tabs: tabs[key].set_pressed_no_signal(key == actor_id)
	var wounds: Array = snapshot.get("wounds", [])
	var own: Array = wounds.filter(func(wound): return wound.get("target_actor_id", "") == actor_id)
	var lost := 0
	for wound in own: lost += wound.get("cards", []).size()
	summary.text = "%s · %d wound%s · %d card%s lost" % [actor_names.get(actor_id, actor_id), own.size(), "" if own.size() == 1 else "s", lost, "" if lost == 1 else "s"]
	var unrecorded := int(snapshot.get("actors", {}).get(actor_id, {}).get("removed_count", 0)) - lost
	if unrecorded > 0:
		var unavailable := Label.new(); unavailable.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		unavailable.text = "%d other removed card%s have no wound record (older battle damage or removal for other reasons)." % [unrecorded, "" if unrecorded == 1 else "s"]
		unavailable.add_theme_font_size_override("font_size", 16); rows.add_child(unavailable)
	for index in own.size(): _add_wound(own[index], index + 1)
	if own.is_empty():
		var empty := Label.new(); empty.text = "No recorded wounds."
		empty.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; empty.custom_minimum_size.y = 100; rows.add_child(empty)

func _add_wound(wound: Dictionary, number: int) -> void:
	var box := PanelContainer.new(); box.set_meta("wound_id", wound.get("id", ""))
	box.add_theme_stylebox_override("panel", STYLE.panel(Color("1d2930"), Color("647078"), 14)); rows.add_child(box)
	var list := VBoxContainer.new(); list.add_theme_constant_override("separation", 6); box.add_child(list)
	var cards: Array = wound.get("cards", [])
	var heading := Label.new(); heading.text = "Wound %d   ·   %d damage / %d card%s lost" % [number, cards.size(), cards.size(), "" if cards.size() == 1 else "s"]
	heading.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; heading.add_theme_font_size_override("font_size", 22); heading.add_theme_color_override("font_color", Color("f1b4a3")); list.add_child(heading)
	var source_id := str(wound.get("source_content_id", ""))
	var source_name := source_id.replace("_", " ").capitalize()
	for kind in ["abilities", "cards", "statuses"]:
		var definition := BattlePresentationCatalog.definition(kind, source_id)
		if not definition.is_empty(): source_name = str(definition.get("name", source_name)); break
	if source_name.is_empty(): source_name = "Damage"
	var source := Label.new(); source.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	source.text = "Round %d · %s\n%s · %s" % [int(wound.get("round", 0)), str(wound.get("segment", "")).replace("_", " ").capitalize(), actor_names.get(wound.get("source_actor_id", ""), "Unknown source"), source_name]
	source.add_theme_font_size_override("font_size", 17); list.add_child(source)
	for card in cards:
		var definition_id := str(card.get("card_definition_id", ""))
		var info := BattlePresentationCatalog.card(definition_id)
		var zone := str(card.get("original_zone", ""))
		var row := TIP_BUTTON.new(); row.alignment = HORIZONTAL_ALIGNMENT_LEFT; row.clip_text = true; row.custom_minimum_size.y = 34
		row.text = str(info.name) if not definition_id.is_empty() else "Card details unavailable"
		row.icon = ICONS.texture(zone); row.expand_icon = true; row.add_theme_constant_override("icon_max_width", 20)
		row.tooltip_text = "Lost from %s" % {"deck": "draw pile", "discard": "discard pile", "hand": "hand"}.get(zone, zone)
		row.set_meta("card_id", card.get("card_id", "")); row.set_meta("definition_id", definition_id)
		list.add_child(row)
		row.mouse_entered.connect(_show_preview.bind(definition_id)); row.mouse_exited.connect(_clear_preview)
		row.focus_entered.connect(_show_preview.bind(definition_id)); row.focus_exited.connect(_clear_preview)

func _show_preview(definition_id: String) -> void:
	_clear_preview()
	if definition_id.is_empty(): return
	preview = BattleCard.new(); preview_slot.add_child(preview); preview.configure("", definition_id, false)
	preview.tooltip_text = ""; preview.mouse_filter = Control.MOUSE_FILTER_IGNORE; preview.size = BattleCard.STANDARD_SIZE

func _clear_preview() -> void:
	if is_instance_valid(preview): preview.hide(); preview.queue_free()
	preview = null

func _unhandled_key_input(event: InputEvent) -> void:
	if is_visible_in_tree() and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled(); closed.emit()
