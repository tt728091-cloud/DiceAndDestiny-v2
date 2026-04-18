extends RefCounted
class_name BattleAuthority

func submit_command(_command_json: String) -> String:
	return JSON.stringify({
		"accepted": false,
		"error": "BattleAuthority.submit_command is not implemented"
	})
