extends Node

const MODEL_PATH := "res://models/learned/blade-warden-seed-11-final-v1.json"
const INFERENCE_TIMEOUT_MS := 2000

var _native_authority: Object
var _initialized := false
var _initialization_error := ""

func _ready() -> void:
	if ClassDB.can_instantiate("NativeBattleAuthority"):
		_native_authority = ClassDB.instantiate("NativeBattleAuthority")
	else:
		_initialization_error = "NativeBattleAuthority GDExtension class is unavailable."

func ensure_initialized() -> Dictionary:
	if _initialized:
		return {"ok": true}
	if _native_authority == null:
		return {"ok": false, "error": _initialization_error}
	var result := _request({
		"op": "initialize",
		"model_path": ProjectSettings.globalize_path(MODEL_PATH),
		"content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content"),
		"run_state_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/save/run_players"),
		"diagnostics_path": WorkspacePaths.persistent_file("phase3/learned-battles.jsonl"),
		"timeout_ms": INFERENCE_TIMEOUT_MS,
	})
	_initialized = result.get("ok") == true
	if not _initialized:
		_initialization_error = str(result.get("error", "The learned policy could not be loaded."))
	return result

func start_battle(battle_id: String, human_seat: String, seed: int, rematch: bool = false) -> Dictionary:
	var initialized := ensure_initialized()
	if initialized.get("ok") != true:
		return {"accepted": false, "error": initialized.get("error", _initialization_error)}
	return _request({
		"op": "reset",
		"battle_id": battle_id,
		"human_seat": human_seat,
		"seed": seed,
		"rematch": rematch,
	})

func submit_human(command_json: String) -> Dictionary:
	if not _initialized:
		return {"accepted": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "submit_human", "command_json": command_json})

func advance_model() -> Dictionary:
	if not _initialized:
		return {"accepted": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "advance_model"})

func telemetry() -> Dictionary:
	if not _initialized:
		return {"ok": false, "error": "The learned battle runtime is not initialized."}
	return _request({"op": "telemetry"})

func initialization_error() -> String:
	return _initialization_error

func _request(payload: Dictionary) -> Dictionary:
	var raw := str(_native_authority.learned_battle_request(JSON.stringify(payload)))
	var parsed = JSON.parse_string(raw)
	if parsed is Dictionary:
		return parsed
	return {"accepted": false, "ok": false, "error": "Learned runtime returned invalid JSON.", "raw_result": raw}

func _exit_tree() -> void:
	if is_instance_valid(_native_authority):
		_native_authority.free()
		_native_authority = null
