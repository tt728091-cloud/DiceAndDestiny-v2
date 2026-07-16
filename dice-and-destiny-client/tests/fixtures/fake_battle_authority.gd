class_name FakeBattleAuthority
extends BattleAuthority

var results: Array[String] = []
var commands: Array[String] = []

func enqueue(result: Dictionary) -> void:
	results.append(JSON.stringify(result))

func submit_command(command_json: String) -> String:
	commands.append(command_json)
	if results.is_empty(): return JSON.stringify({"accepted": false, "error": "Fake authority queue exhausted"})
	return results.pop_front()
