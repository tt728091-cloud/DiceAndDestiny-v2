class_name WorkspacePaths
extends RefCounted

const RUNTIME_ROOT_ENV := "DICE_AND_DESTINY_RUNTIME_ROOT"


static func runtime_root() -> String:
	var configured := OS.get_environment(RUNTIME_ROOT_ENV).strip_edges()
	if not configured.is_empty():
		return configured.simplify_path()
	if OS.has_feature("editor"):
		return ProjectSettings.globalize_path("res://.godot/runtime/workspace")
	return ProjectSettings.globalize_path("user://")


static func runtime_dir(relative_path: String) -> String:
	var path := runtime_root().path_join(_safe_relative_path(relative_path))
	DirAccess.make_dir_recursive_absolute(path)
	return path


static func persistent_file(filename: String) -> String:
	if OS.get_environment(RUNTIME_ROOT_ENV).strip_edges().is_empty() and not OS.has_feature("editor"):
		return ProjectSettings.globalize_path("user://").path_join(_safe_relative_path(filename))
	return runtime_dir("user").path_join(_safe_relative_path(filename))


static func _safe_relative_path(value: String) -> String:
	var normalized := value.strip_edges().replace("\\", "/").simplify_path()
	assert(not normalized.is_empty(), "workspace runtime path must not be empty")
	assert(not normalized.is_absolute_path(), "workspace runtime path must be relative")
	assert(normalized != ".." and not normalized.begins_with("../"), "workspace runtime path must stay below the runtime root")
	return normalized
