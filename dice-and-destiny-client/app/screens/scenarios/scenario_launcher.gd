extends Control

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")

var _gateway := BattleGateway.new()
var _store := ActiveBattleStore.new()
var _scenario_list: ItemList
var _status: Label
var _scenarios: Array = []

func _ready() -> void:
	_build_ui()
	if not _scenario_tools_enabled():
		_status.text = (
			"Scenario tooling requires a debug scenario build and "
			+ "DICE_AND_DESTINY_ENABLE_SCENARIOS=1 at process startup."
		)
		return
	var active := _store.load_active()
	if not active.is_empty():
		var reopened := _gateway.open_battle(
			str(active.get("battle_id", "")),
			str(active.get("actor_id", "player"))
		)
		if reopened.get("accepted") == true:
			_handoff_to_battle(reopened, str(active.get("actor_id", "player")))
			return
		_store.clear()
	_refresh_catalog()

func _scenario_tools_enabled() -> bool:
	return (
		OS.is_debug_build()
		and OS.get_environment("DICE_AND_DESTINY_ENABLE_SCENARIOS") == "1"
		and bool(ProjectSettings.get_setting(
		"dice_and_destiny/development/enable_scenarios",
		false
		))
	)

func _build_ui() -> void:
	var root := VBoxContainer.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.offset_left = 32.0
	root.offset_top = 32.0
	root.offset_right = -32.0
	root.offset_bottom = -32.0
	root.add_theme_constant_override("separation", 12)
	add_child(root)

	var title := Label.new()
	title.text = "Development Scenario Launcher"
	title.add_theme_font_size_override("font_size", 24)
	root.add_child(title)

	_scenario_list = ItemList.new()
	_scenario_list.custom_minimum_size = Vector2(0, 260)
	root.add_child(_scenario_list)

	var launch := Button.new()
	launch.text = "Launch Selected Scenario"
	launch.pressed.connect(_launch_selected)
	root.add_child(launch)

	_status = Label.new()
	_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	root.add_child(_status)

func _refresh_catalog() -> void:
	var result := _gateway.list_scenarios()
	if result.get("accepted") != true:
		_status.text = str(result.get("error", "Could not list scenarios"))
		return
	_scenarios = result.get("data", {}).get("scenarios", [])
	_scenario_list.clear()
	for entry in _scenarios:
		_scenario_list.add_item("%s: %s" % [entry.get("id", ""), entry.get("name", "")])
	if not _scenarios.is_empty():
		_scenario_list.select(0)
	_status.text = "%d scenario(s) available." % _scenarios.size()

func _launch_selected() -> void:
	var selected := _scenario_list.get_selected_items()
	if selected.is_empty():
		_status.text = "Select a scenario."
		return
	var entry: Dictionary = _scenarios[selected[0]]
	var result := _gateway.start_named_scenario(str(entry.get("id", "")))
	if result.get("accepted") != true:
		_status.text = str(result.get("error", "Scenario launch failed"))
		return
	_handoff_to_battle(result, "player")

func _handoff_to_battle(result: Dictionary, actor_id: String) -> void:
	var snapshot: Dictionary = result.get("snapshot", {})
	var battle_id := str(snapshot.get("battle_id", ""))
	if battle_id.is_empty() or _store.save_active(battle_id, actor_id) != OK:
		_status.text = "Could not persist the active battle ID."
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = actor_id
	get_tree().root.add_child(screen)
	queue_free()
