extends BattleAuthority
class_name GoGDExtensionBattleAuthority

var _native_authority: Object

func _init() -> void:
	if ClassDB.can_instantiate("NativeBattleAuthority"):
		_native_authority = ClassDB.instantiate("NativeBattleAuthority")

func _notification(what: int) -> void:
	if what == NOTIFICATION_PREDELETE and is_instance_valid(_native_authority):
		_native_authority.free()
		_native_authority = null

func submit_command(command_json: String) -> String:
	if _native_authority == null:
		return JSON.stringify({
			"accepted": false,
			"error": "NativeBattleAuthority GDExtension class is not available"
		})

	return _native_authority.submit_command(command_json)
