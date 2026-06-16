class_name ActiveBattleStore
extends RefCounted

const SAVE_PATH := "user://active_battle.json"

var _save_path: String

func _init(save_path: String = SAVE_PATH) -> void:
	_save_path = save_path

func save_active(battle_id: String, actor_id: String) -> Error:
	var file := FileAccess.open(_save_path, FileAccess.WRITE)
	if file == null:
		return FileAccess.get_open_error()
	file.store_string(JSON.stringify({
		"battle_id": battle_id,
		"actor_id": actor_id,
	}))
	return OK

func load_active() -> Dictionary:
	if not FileAccess.file_exists(_save_path):
		return {}
	var file := FileAccess.open(_save_path, FileAccess.READ)
	if file == null:
		return {}
	var parsed = JSON.parse_string(file.get_as_text())
	return parsed if parsed is Dictionary else {}

func clear() -> void:
	if FileAccess.file_exists(_save_path):
		DirAccess.remove_absolute(ProjectSettings.globalize_path(_save_path))
