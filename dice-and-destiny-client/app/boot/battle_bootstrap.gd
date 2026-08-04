extends Control

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const VIEWER := "blade"

var gateway: BattleGateway
var store: ActiveBattleStore
var _message: Label
var _buttons: VBoxContainer

func _ready() -> void:
	gateway = BattleGateway.new() if gateway == null else gateway
	store = ActiveBattleStore.new() if store == null else store
	_build_mode_menu()

func _build_mode_menu() -> void:
	var background := ColorRect.new()
	background.color = Color("0b111a")
	background.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(background)
	var center := CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(760, 1000)
	center.add_child(panel)
	var margin := MarginContainer.new()
	for side in ["margin_left", "margin_top", "margin_right", "margin_bottom"]:
		margin.add_theme_constant_override(side, 34)
	panel.add_child(margin)
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
	subtitle.text = "Choose a local battle mode"
	subtitle.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	subtitle.add_theme_font_size_override("font_size", 20)
	content.add_child(subtitle)
	_buttons = VBoxContainer.new()
	_buttons.add_theme_constant_override("separation", 12)
	content.add_child(_buttons)
	_add_mode_button(
		"Classic Battle\nBlade Warden vs Venom Goblin (D100)",
		_start_classic,
		"battle.mode.classic"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat A · Old v1\nBlade Warden vs Frozen Seed-11 Learned Blade Warden",
		_start_learned.bind("seat-a", "accepted-v1"),
		"battle.mode.learned.v1.seat_a"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat B · Old v1\nBlade Warden vs Frozen Seed-11 Learned Blade Warden",
		_start_learned.bind("seat-b", "accepted-v1"),
		"battle.mode.learned.v1.seat_b"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat A · New v2\nBlade Warden vs Decision-Quality Seed-22 Blade Warden",
		_start_learned.bind("seat-a", "decision-v2"),
		"battle.mode.learned.v2.seat_a"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat B · New v2\nBlade Warden vs Decision-Quality Seed-22 Blade Warden",
		_start_learned.bind("seat-b", "decision-v2"),
		"battle.mode.learned.v2.seat_b"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat A · Strongest v3\nBlade Warden vs Optimized 5M Seed-22 Blade Warden",
		_start_learned.bind("seat-a", "optimized-v3"),
		"battle.mode.learned.v3.seat_a"
	)
	_add_mode_button(
		"Learned Mirror · Human Seat B · Strongest v3\nBlade Warden vs Optimized 5M Seed-22 Blade Warden",
		_start_learned.bind("seat-b", "optimized-v3"),
		"battle.mode.learned.v3.seat_b"
	)
	_message = Label.new()
	_message.text = "Learned battles are inference-only. Choose the preserved v1, decision-quality v2, or strongest optimized v3 opponent."
	_message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_message.add_theme_color_override("font_color", Color("9fb3c8"))
	content.add_child(_message)

func _add_mode_button(text: String, callback: Callable, control_id: String) -> void:
	var button := Button.new()
	button.text = text
	button.custom_minimum_size.y = 82
	button.add_theme_font_size_override("font_size", 18)
	button.pressed.connect(callback)
	_buttons.add_child(button)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null:
		inspector.register_control(control_id, button, text.replace("\n", " — "))

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
	var learned_gateway: RefCounted = LEARNED_GATEWAY.new(runtime, human_seat, model_key)
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
