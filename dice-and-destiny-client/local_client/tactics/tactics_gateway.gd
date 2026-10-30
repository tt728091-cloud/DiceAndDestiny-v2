class_name TacticsGateway
extends RefCounted
## Tactics Sandbox gateway: the setup catalog, starting, resuming and playing
## a sandbox battle through the authority's tactics_* ops. The client only
## renders the viewer-safe result and submits the legal actions it lists.

const ACTIVE_FILENAME := "active_tactics_battle.json"
var _runtime: Node
var battle_id := ""

func _init(runtime: Node) -> void:
	_runtime = runtime

## The sandbox's own resume pointer, beside (never instead of) the normal
## active battle pointer.
static func active_store() -> ActiveBattleStore:
	return ActiveBattleStore.new(WorkspacePaths.persistent_file(ACTIVE_FILENAME))

func catalog() -> Dictionary:
	return _runtime.tactics_request("tactics_catalog")

func start(id: String, setup: Dictionary) -> Dictionary:
	battle_id = id
	return _runtime.tactics_request("tactics_start", {"battle_id": id, "tactics": setup})

func open(id: String) -> Dictionary:
	battle_id = id
	return _runtime.tactics_request("tactics_open", {"battle_id": id})

## Every resolved round of the battle, oldest first, as the hero may see it
## (movement, attacks, hits, health), for replaying the whole battle.
func history() -> Dictionary:
	return _runtime.tactics_request("tactics_history", {"battle_id": battle_id})

## Takes a command as the battle screen sends it (JSON text) or a legal
## action dictionary.
func submit(command) -> Dictionary:
	var command_json: String = command if command is String else JSON.stringify(command)
	return _runtime.tactics_request("tactics_submit", {"battle_id": battle_id, "command_json": command_json})
