class_name LearnedBattleGateway
extends RefCounted

var _runtime: Node
var human_seat := "seat-a"
var model_key := "accepted-v1"
var seed := 0
var character := "blade_warden"
var _configuration_error := ""

func _init(runtime: Node, selected_human_seat: String = "seat-a", selected_model_key: String = "accepted-v1", selected_character: String = "blade_warden") -> void:
	_runtime = runtime
	human_seat = selected_human_seat
	character = selected_character
	model_key = selected_model_key
	var configured: Dictionary = _runtime.select_model(model_key)
	if configured.get("ok") != true:
		_configuration_error = str(configured.get("error", "The learned model selection is invalid."))

func start_battle(battle_id: String, selected_seed: int, rematch: bool = false) -> Dictionary:
	if not _configuration_error.is_empty():
		return {"accepted": false, "error": _configuration_error}
	seed = selected_seed
	return _runtime.start_battle(battle_id, human_seat, seed, rematch, character)

func submit(command_json: String) -> Dictionary:
	return _runtime.submit_human(command_json)

func advance_model() -> Dictionary:
	return _runtime.advance_model()

func telemetry() -> Dictionary:
	return _runtime.telemetry()
