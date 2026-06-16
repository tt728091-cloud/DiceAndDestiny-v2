class_name BattleViewState
extends RefCounted

var battle_id := ""
var status := ""
var segment := ""
var round_number := 0
var viewer_actor_id := ""
var actors: Dictionary = {}
var pending_input: Dictionary = {}
var origin: Dictionary = {}

func apply_result(result: Dictionary) -> bool:
	if result.get("accepted") != true:
		return false
	var snapshot = result.get("snapshot")
	if not snapshot is Dictionary:
		return false
	battle_id = str(snapshot.get("battle_id", ""))
	status = str(snapshot.get("status", ""))
	segment = str(snapshot.get("segment", ""))
	round_number = int(snapshot.get("round", 0))
	viewer_actor_id = str(snapshot.get("viewer_actor_id", ""))
	actors = snapshot.get("actors", {})
	pending_input = result.get("pending_input", {})
	origin = snapshot.get("origin", {})
	return not battle_id.is_empty()

func is_scenario() -> bool:
	return origin.get("kind") == "scenario"
