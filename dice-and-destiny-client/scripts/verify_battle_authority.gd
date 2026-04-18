extends SceneTree

const AUTHORITY_SCRIPT := preload("res://scripts/go_gdextension_battle_authority.gd")

func _init() -> void:
	var authority: BattleAuthority = AUTHORITY_SCRIPT.new()
	var command_json := JSON.stringify({
		"battle_id": "battle-1",
		"actor_id": "player",
		"type": "roll_dice",
		"payload": {
			"pool": "offensive"
		}
	})

	var result_json := authority.submit_command(command_json)
	print(result_json)

	var parsed = JSON.parse_string(result_json)
	if not parsed is Dictionary:
		push_error("Authority returned non-object JSON")
		quit(1)
		return

	if parsed.get("accepted") != true:
		push_error("Authority did not accept command: %s" % result_json)
		quit(1)
		return

	var events: Array = parsed.get("events", [])
	if events.is_empty() or events[0].get("type") != "dice_rolled":
		push_error("Authority did not return dice_rolled: %s" % result_json)
		quit(1)
		return

	quit(0)
