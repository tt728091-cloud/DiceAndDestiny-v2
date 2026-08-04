extends Node

const ACCEPTED_V1_MODEL_PATH := "res://models/learned/blade-warden-seed-11-final-v1.json"
const PHASE2_V2_MODEL_PATH := "res://models/learned/blade-warden-decision-quality-seed-22-v2.json"
const PHASE2_V2_MODEL_SHA256 := "0e6ea5d84c316a7709c9c1b98b983e8f76e486c6e1625c479c55d2860bd23a86"
const OPTIMIZED_V3_MODEL_PATH := "res://models/learned/blade-warden-optimized-5m-seed-22-v3.json"
const OPTIMIZED_V3_MODEL_SHA256 := "529a6b4d6ad347d5ba86b5e000cb5fceec306414cdf0af3405713a2bc5c32ebb"
const PHASE2_OPT_IN_ENV := "DICE_AND_DESTINY_PHASE2_DECISION_MODEL"
const MODEL_ACCEPTED_V1 := "accepted-v1"
const MODEL_DECISION_V2 := "decision-v2"
const MODEL_OPTIMIZED_V3 := "optimized-v3"
const INFERENCE_TIMEOUT_MS := 2000

var _native_authority: Object
var _initialized := false
var _initialized_model_key := ""
var _selected_model_key := MODEL_ACCEPTED_V1
var _initialization_error := ""

func _ready() -> void:
	if OS.get_environment(PHASE2_OPT_IN_ENV) == "1":
		_selected_model_key = MODEL_DECISION_V2
	if ClassDB.can_instantiate("NativeBattleAuthority"):
		_native_authority = ClassDB.instantiate("NativeBattleAuthority")
	else:
		_initialization_error = "NativeBattleAuthority GDExtension class is unavailable."

func select_model(model_key: String) -> Dictionary:
	if model_key not in [MODEL_ACCEPTED_V1, MODEL_DECISION_V2, MODEL_OPTIMIZED_V3]:
		return {"ok": false, "error": "Unknown learned model selection: %s" % model_key}
	_selected_model_key = model_key
	_initialized = _initialized_model_key == model_key
	_initialization_error = ""
	return {"ok": true, "model_key": model_key}

func selected_model_key() -> String:
	return _selected_model_key

func ensure_initialized() -> Dictionary:
	if _initialized:
		return {"ok": true}
	if _native_authority == null:
		return {"ok": false, "error": _initialization_error}
	var result := _request({
		"op": "initialize",
		"replace_session": not _initialized_model_key.is_empty() and _initialized_model_key != _selected_model_key,
		"model_path": ProjectSettings.globalize_path(_selected_model_path()),
		"model_sha256": _selected_model_sha256(),
		"content_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/content"),
		"run_state_root": ProjectSettings.globalize_path("res://../dice-and-destiny-server/save/run_players"),
		"diagnostics_path": WorkspacePaths.persistent_file("phase3/learned-battles.jsonl"),
		"timeout_ms": INFERENCE_TIMEOUT_MS,
	})
	_initialized = result.get("ok") == true
	if _initialized:
		_initialized_model_key = _selected_model_key
	if not _initialized:
		_initialization_error = str(result.get("error", "The learned policy could not be loaded."))
	return result

func _selected_model_path() -> String:
	match _selected_model_key:
		MODEL_DECISION_V2:
			return PHASE2_V2_MODEL_PATH
		MODEL_OPTIMIZED_V3:
			return OPTIMIZED_V3_MODEL_PATH
		_:
			return ACCEPTED_V1_MODEL_PATH

func _selected_model_sha256() -> String:
	match _selected_model_key:
		MODEL_DECISION_V2:
			return PHASE2_V2_MODEL_SHA256
		MODEL_OPTIMIZED_V3:
			return OPTIMIZED_V3_MODEL_SHA256
		_:
			return ""

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
