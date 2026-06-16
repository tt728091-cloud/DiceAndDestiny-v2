extends Control

var initial_result: Dictionary = {}
var viewer_actor_id := "player"
var _view_state := BattleViewState.new()
var _gateway := BattleGateway.new()
var _root: VBoxContainer

func _ready() -> void:
	if not _view_state.apply_result(initial_result):
		_show_error(str(initial_result.get("error", "Battle snapshot is missing")))
		return
	_render()

func _render() -> void:
	if _root != null:
		_root.queue_free()
	_root = VBoxContainer.new()
	_root.set_anchors_preset(Control.PRESET_FULL_RECT)
	_root.offset_left = 32.0
	_root.offset_top = 32.0
	_root.offset_right = -32.0
	_root.offset_bottom = -32.0
	_root.add_theme_constant_override("separation", 10)
	add_child(_root)

	var title := Label.new()
	title.text = "Scenario Battle" if _view_state.is_scenario() else "Battle"
	title.add_theme_font_size_override("font_size", 24)
	_root.add_child(title)

	var location := Label.new()
	location.text = "Round %d, %s" % [_view_state.round_number, _view_state.segment]
	_root.add_child(location)

	for actor_id in _view_state.actors:
		var actor: Dictionary = _view_state.actors[actor_id]
		var actor_label := Label.new()
		actor_label.text = "%s | health %d/%d | removed %d | statuses %s" % [
			actor_id,
			int(actor.get("current_health", 0)),
			int(actor.get("max_health", 0)),
			int(actor.get("removed_count", 0)),
			JSON.stringify(actor.get("statuses", [])),
		]
		_root.add_child(actor_label)

	var wait_label := Label.new()
	wait_label.text = "Pending input: %s" % JSON.stringify(_view_state.pending_input)
	wait_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_root.add_child(wait_label)
	_add_pending_controls()

func _add_pending_controls() -> void:
	var pending = _view_state.pending_input.get(viewer_actor_id)
	if not pending is Dictionary:
		return
	var allowed: Array = pending.get("allowed_commands", [])
	for command_type in ["roll_dice", "planning_pass", "planning_lock_in", "pass"]:
		if command_type not in allowed:
			continue
		var button := Button.new()
		button.text = command_type.replace("_", " ").capitalize()
		button.pressed.connect(_submit_pending.bind(command_type, pending))
		_root.add_child(button)

func _submit_pending(command_type: String, pending: Dictionary) -> void:
	var result := _gateway.submit_pending_command(
		_view_state.battle_id,
		viewer_actor_id,
		command_type,
		pending
	)
	if not _view_state.apply_result(result):
		_show_error(str(result.get("error", "Battle command failed")))
		return
	initial_result = result
	_render()

func _show_error(message: String) -> void:
	var label := Label.new()
	label.text = message
	add_child(label)
