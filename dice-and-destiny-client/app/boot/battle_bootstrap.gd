extends Control

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const VIEWER := "blade"

var gateway: BattleGateway
var store: ActiveBattleStore
var _message: Label

func _ready() -> void:
	gateway = BattleGateway.new() if gateway == null else gateway
	store = ActiveBattleStore.new() if store == null else store
	_message = Label.new()
	_message.text = "Opening the battle authority…"
	_message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_message.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_message.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(_message)
	call_deferred("_open_or_start")

func _open_or_start() -> void:
	var active := store.load_active()
	if str(active.get("actor_id", VIEWER)) == VIEWER and not str(active.get("battle_id", "")).is_empty():
		var reopened := gateway.open_battle(str(active.battle_id), VIEWER)
		if reopened.get("accepted") == true:
			_handoff(reopened, int(active.get("last_sequence", 0)), str(active.get("snapshot_name", "")), active.get("history_context", {}))
			return
		store.clear()
	var battle_id := _new_battle_id()
	var started := gateway.start_battle(battle_id, VIEWER)
	if started.get("accepted") != true:
		_show_error(str(started.get("error", "The battle could not start.")), started)
		return
	_handoff(started, 0)

func _new_battle_id() -> String:
	return "battle-%d-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]

func _handoff(result: Dictionary, last_sequence: int, snapshot_name: String = "", history_context: Dictionary = {}) -> void:
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

func _show_error(message: String, result: Dictionary) -> void:
	_message.text = "BATTLE AUTHORITY ERROR\n%s\n\n%s" % [message, JSON.stringify(result)]
	_message.add_theme_color_override("font_color", Color("ff8a78"))
