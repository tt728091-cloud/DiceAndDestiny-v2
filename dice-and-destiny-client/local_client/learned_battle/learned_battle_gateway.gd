class_name LearnedBattleGateway
extends RefCounted

var _runtime: Node
var human_seat := "seat-a"
var seed := 0

func _init(runtime: Node, selected_human_seat: String = "seat-a") -> void:
	_runtime = runtime
	human_seat = selected_human_seat

func start_battle(battle_id: String, selected_seed: int, rematch: bool = false) -> Dictionary:
	seed = selected_seed
	return _runtime.start_battle(battle_id, human_seat, seed, rematch)

func submit(command_json: String) -> Dictionary:
	return _runtime.submit_human(command_json)

func advance_model() -> Dictionary:
	return _runtime.advance_model()

func telemetry() -> Dictionary:
	return _runtime.telemetry()
