class_name ActiveBattleStore
extends RefCounted

const SAVE_FILENAME := "active_battle.json"

var _save_path: String

func _init(save_path: String = "") -> void:
	_save_path = WorkspacePaths.persistent_file(SAVE_FILENAME) if save_path.is_empty() else save_path

func save_active(battle_id: String, actor_id: String, last_sequence: int = 0, snapshot_name: String = "", history_context: Dictionary = {}) -> Error:
	var parent := ProjectSettings.globalize_path(_save_path).get_base_dir()
	var directory_error := DirAccess.make_dir_recursive_absolute(parent)
	if directory_error != OK:
		return directory_error
	var file := FileAccess.open(_save_path, FileAccess.WRITE)
	if file == null:
		return FileAccess.get_open_error()
	var record := {
		"battle_id": battle_id,
		"actor_id": actor_id,
		"last_sequence": last_sequence,
	}
	if not snapshot_name.is_empty(): record["snapshot_name"] = snapshot_name
	if not history_context.is_empty(): record["history_context"] = history_context
	file.store_string(JSON.stringify(record))
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
