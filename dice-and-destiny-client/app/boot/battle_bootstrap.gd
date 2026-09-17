extends Control

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const VIEWER := "blade"
const MODE_PANEL_MAXIMUM_SIZE := Vector2(760, 620)
const MODE_PANEL_VIEWPORT_INSET := Vector2(32, 32)

var gateway: BattleGateway
var store: ActiveBattleStore
var _message: Label
var _buttons: VBoxContainer
var _mode_panel: PanelContainer
var _character_choice: OptionButton
var _model_choice: OptionButton
var _seat_choice: OptionButton

func _ready() -> void:
	gateway = BattleGateway.new() if gateway == null else gateway
	store = ActiveBattleStore.new() if store == null else store
	_build_mode_menu()
	resized.connect(_fit_mode_panel)
	call_deferred("_fit_mode_panel")

func _build_mode_menu() -> void:
	var background := ColorRect.new()
	background.color = Color("0b111a")
	background.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(background)
	var center := CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	_mode_panel = PanelContainer.new()
	_mode_panel.name = "ModePanel"
	_mode_panel.custom_minimum_size = MODE_PANEL_MAXIMUM_SIZE
	center.add_child(_mode_panel)
	var margin := MarginContainer.new()
	for side in ["margin_left", "margin_top", "margin_right", "margin_bottom"]:
		margin.add_theme_constant_override(side, 34)
	_mode_panel.add_child(margin)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 16)
	margin.add_child(content)
	var title := Label.new()
	title.text = "DICE & DESTINY"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_font_size_override("font_size", 40)
	title.add_theme_color_override("font_color", Color("f5c963"))
	content.add_child(title)
	var subtitle := Label.new()
	subtitle.text = "Choose your character and opponent"
	subtitle.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	subtitle.add_theme_font_size_override("font_size", 20)
	content.add_child(subtitle)
	var mode_scroll := ScrollContainer.new()
	mode_scroll.name = "ModeScroll"
	mode_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	mode_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	mode_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	mode_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	mode_scroll.follow_focus = true
	content.add_child(mode_scroll)
	_buttons = VBoxContainer.new()
	_buttons.name = "ModeButtons"
	_buttons.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_buttons.add_theme_constant_override("separation", 12)
	mode_scroll.add_child(_buttons)
	_character_choice = _add_selection("YOUR CHARACTER", [
		["Blade Warden · sword, shields, and bleed", "blade_warden"],
		["Venom · Poison, Incubation, and Catalyst", "venom"],
	], "battle.setup.character")
	_model_choice = _add_selection("ENEMY · LEARNED BLADE WARDEN", [
		["Global Champion CP193 · winner-health v2", "global-champion"],
		["Prior Champion CP38 · winner-health", "prior-global-cp38"],
		["Prior Champion CP480 · 2.4M steps", "prior-global-cp480"],
		["Optimized v3 · 5M seed 22", "optimized-v3"],
		["Decision Quality v2 · seed 22", "decision-v2"],
		["Original v1 · seed 11", "accepted-v1"],
	], "battle.setup.model")
	_seat_choice = _add_selection("YOUR SEAT", [["Seat A", "seat-a"], ["Seat B", "seat-b"]], "battle.setup.seat")
	_add_mode_button("Start Battle", _start_selected, "battle.setup.start")
	_message = Label.new()
	_message.text = "Play a full battle against the selected trained Blade Warden. Venom has 24 cards and six abilities. Rematch keeps your character and opponent."
	_message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_message.add_theme_color_override("font_color", Color("9fb3c8"))
	content.add_child(_message)

func _add_selection(caption: String, choices: Array, control_id: String) -> OptionButton:
	var heading := Label.new()
	heading.text = caption
	heading.add_theme_color_override("font_color", Color("9fb3c8"))
	_buttons.add_child(heading)
	var select := OptionButton.new()
	select.custom_minimum_size.y = 48
	select.add_theme_font_size_override("font_size", 19)
	for choice in choices:
		select.add_item(str(choice[0]))
		select.set_item_metadata(select.item_count - 1, choice[1])
	_buttons.add_child(select)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null:
		inspector.register_control(control_id, select, caption)
	return select

func _start_selected() -> void:
	_start_learned(str(_seat_choice.get_selected_metadata()), str(_model_choice.get_selected_metadata()))

func _add_mode_button(text: String, callback: Callable, control_id: String) -> void:
	var button := Button.new()
	button.text = text
	button.custom_minimum_size.y = 82
	button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	button.add_theme_font_size_override("font_size", 18)
	button.pressed.connect(callback)
	_buttons.add_child(button)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null:
		inspector.register_control(control_id, button, text.replace("\n", " — "))

func _fit_mode_panel() -> void:
	if _mode_panel == null:
		return
	var available := Vector2(
		maxf(1.0, size.x - MODE_PANEL_VIEWPORT_INSET.x),
		maxf(1.0, size.y - MODE_PANEL_VIEWPORT_INSET.y)
	)
	_mode_panel.custom_minimum_size = Vector2(
		minf(MODE_PANEL_MAXIMUM_SIZE.x, available.x),
		minf(MODE_PANEL_MAXIMUM_SIZE.y, available.y)
	)

func _start_classic() -> void:
	_set_buttons_disabled(true)
	_message.text = "Opening the classic battle authority…"
	var active := store.load_active()
	if str(active.get("actor_id", VIEWER)) == VIEWER and not str(active.get("battle_id", "")).is_empty():
		var reopened := gateway.open_battle(str(active.battle_id), VIEWER)
		if reopened.get("accepted") == true:
			_handoff_classic(reopened, int(active.get("last_sequence", 0)), str(active.get("snapshot_name", "")), active.get("history_context", {}))
			return
		store.clear()
	var battle_id := _new_battle_id("classic")
	var started := gateway.start_battle(battle_id, VIEWER)
	if started.get("accepted") != true:
		_show_error(str(started.get("error", "The battle could not start.")), started)
		return
	_handoff_classic(started, 0)

func _start_learned(human_seat: String, model_key: String) -> void:
	_set_buttons_disabled(true)
	_message.text = "Loading the selected frozen learned policy…"
	await get_tree().process_frame
	var runtime := get_node_or_null("/root/LearnedBattleRuntime")
	if runtime == null:
		_show_error("The learned battle runtime autoload is unavailable.", {})
		return
	var learned_gateway: RefCounted = LEARNED_GATEWAY.new(runtime, human_seat, model_key, str(_character_choice.get_selected_metadata()) if _character_choice != null else "blade_warden")
	var seed := int(Time.get_unix_time_from_system() * 1000000.0) ^ Time.get_ticks_usec()
	var result: Dictionary = learned_gateway.start_battle(_new_battle_id("learned"), seed)
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "The learned battle could not start.")), result)
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = VIEWER
	screen.gateway = learned_gateway
	screen.active_store = store
	screen.learned_battle_mode = true
	screen.learned_human_seat = human_seat
	screen.learned_seed = seed
	get_tree().root.add_child(screen)
	queue_free()

func _new_battle_id(prefix: String = "battle") -> String:
	return "%s-%d-%d-%d" % [prefix, Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]

func _handoff_classic(result: Dictionary, last_sequence: int, snapshot_name: String = "", history_context: Dictionary = {}) -> void:
	var battle_id := str(result.get("snapshot", {}).get("battle_id", ""))
	if battle_id.is_empty() or store.save_active(battle_id, VIEWER, last_sequence, snapshot_name, history_context) != OK:
		_show_error("The active battle record could not be saved.", result)
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = VIEWER
	screen.gateway = gateway
	screen.active_store = store
	screen.last_presented_sequence = last_sequence
	screen.loaded_snapshot_name = snapshot_name
	screen.history_context = history_context
	get_tree().root.add_child(screen)
	queue_free()

func _set_buttons_disabled(disabled: bool) -> void:
	for child in _buttons.get_children():
		if child is Button:
			child.disabled = disabled

func _show_error(message: String, result: Dictionary) -> void:
	_message.text = "BATTLE START ERROR\n%s\n\n%s" % [message, JSON.stringify(result)]
	_message.add_theme_color_override("font_color", Color("ff8a78"))
	_set_buttons_disabled(false)
