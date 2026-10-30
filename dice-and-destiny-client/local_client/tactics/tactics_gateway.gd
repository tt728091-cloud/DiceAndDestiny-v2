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

func submit(action: Dictionary) -> Dictionary:
	return _runtime.tactics_request("tactics_submit", {"battle_id": battle_id, "command_json": JSON.stringify(action)})
