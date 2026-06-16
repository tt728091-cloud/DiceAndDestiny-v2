extends SceneTree

const GATEWAY_SCRIPT := preload("res://local_client/battle_gateway/battle_gateway.gd")
const STORE_SCRIPT := preload("res://local_client/battle_gateway/active_battle_store.gd")

func _init() -> void:
	var state_root := ProjectSettings.globalize_path("user://scenario_headless_state")
	var active_path := "user://scenario_headless_active.json"
	_remove_tree(state_root)
	DirAccess.remove_absolute(ProjectSettings.globalize_path(active_path))

	var gateway: BattleGateway = GATEWAY_SCRIPT.new()
	var listed := gateway.list_scenarios()
	if listed.get("accepted") != true:
		_fail("list_scenarios failed: %s" % JSON.stringify(listed))
		return
	var entries: Array = listed.get("data", {}).get("scenarios", [])
	if entries.is_empty():
		_fail("No scenarios were returned")
		return

	var started := gateway.start_named_scenario("round-2-poisoned-player")
	if started.get("accepted") != true:
		_fail("start_scenario failed: %s" % JSON.stringify(started))
		return
	var snapshot: Dictionary = started.get("snapshot", {})
	var battle_id := str(snapshot.get("battle_id", ""))
	var player: Dictionary = snapshot.get("actors", {}).get("player", {})
	var enemy: Dictionary = snapshot.get("actors", {}).get("goblin-1", {})
	if battle_id.is_empty() or snapshot.get("origin", {}).get("kind") != "scenario":
		_fail("Scenario provenance or battle ID is missing: %s" % JSON.stringify(started))
		return
	if int(player.get("current_health", 0)) != 16 or int(player.get("removed_count", 0)) != 4:
		_fail("Player damage was not displayed correctly: %s" % JSON.stringify(player))
		return
	if enemy.has("hand") and not enemy.get("hand", []).is_empty():
		_fail("Enemy hidden hand was exposed: %s" % JSON.stringify(enemy))
		return
	var pending: Dictionary = started.get("pending_input", {}).get("player", {})
	var rolled := gateway.submit_pending_command(battle_id, "player", "roll_dice", pending)
	if rolled.get("accepted") != true:
		_fail("Production planning command failed: %s" % JSON.stringify(rolled))
		return
	started = rolled
	snapshot = rolled.get("snapshot", {})

	var store: ActiveBattleStore = STORE_SCRIPT.new(active_path)
	if store.save_active(battle_id, "player") != OK:
		_fail("Could not save active battle ID")
		return
	var restored := store.load_active()
	if restored.get("battle_id") != battle_id:
		_fail("Active battle ID did not persist")
		return

	var checkpoint_path := state_root.path_join("%s.json" % battle_id)
	var before := FileAccess.get_file_as_bytes(checkpoint_path)
	if before.is_empty():
		_fail("Scenario checkpoint was not written to the configured state root")
		return
	var reopened := gateway.open_battle(battle_id, "player")
	var after := FileAccess.get_file_as_bytes(checkpoint_path)
	if reopened.get("accepted") != true or reopened.get("events", []).size() != 0:
		_fail("open_battle failed or emitted events: %s" % JSON.stringify(reopened))
		return
	if before != after:
		_fail("open_battle mutated the durable checkpoint")
		return

	print(JSON.stringify({
		"accepted": true,
		"battle_id": battle_id,
		"round": snapshot.get("round"),
		"segment": snapshot.get("segment"),
	}))
	store.clear()
	_remove_tree(state_root)
	quit(0)

func _fail(message: String) -> void:
	push_error(message)
	quit(1)

func _remove_tree(path: String) -> void:
	if not DirAccess.dir_exists_absolute(path):
		return
	var directory := DirAccess.open(path)
	if directory == null:
		return
	directory.list_dir_begin()
	var name := directory.get_next()
	while not name.is_empty():
		var child := path.path_join(name)
		if directory.current_is_dir():
			_remove_tree(child)
		else:
			DirAccess.remove_absolute(child)
		name = directory.get_next()
	directory.list_dir_end()
	DirAccess.remove_absolute(path)
