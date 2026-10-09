extends SceneTree

## A card that reaches the hand without being drawn flies from the pile it
## left: discard for recovery (Reclaim), removed for a revive trade. Ordinary
## draws keep the deck origin.

var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	BattleViewState.new().apply_result(gateway.start_battle("card-recovery-origin", 43))
	var instances := {"reclaim-0": {"definition_id": "reclaim"}, "nudge-0": {"definition_id": "nudge"}, "try_again-0": {"definition_id": "try_again"}, "take_stock-0": {"definition_id": "take_stock"}, "strong_swing-0": {"definition_id": "strong_swing"}}
	var cases := [
		["discard", {"discard_composition": {"nudge": 1}}, {"discard_composition": {"reclaim": 1}}, "nudge-0"],
		["removed", {"removed_composition": {"try_again": 1}}, {"removed_composition": {"reclaim": 1}}, "try_again-0"],
		["deck", {"deck_count": 3}, {"deck_count": 2, "discard_composition": {"reclaim": 1}}, "strong_swing-0"],
	]
	for entry in cases:
		var before: Dictionary = {"hand": ["reclaim-0", "take_stock-0"], "hand_count": 2, "card_instances": instances}
		before.merge(entry[1], true)
		var after: Dictionary = {"hand": ["take_stock-0", entry[3]], "hand_count": 2, "card_instances": instances}
		after.merge(entry[2], true)
		var director := BattlePresentationDirector.new()
		var events := [{"type": "card_played", "actor_id": "blade", "sequence": 1, "data": {"card_definition_id": "reclaim", "card_instance_id": "reclaim-0"}}]
		director._queue_card_gains(events, {"blade": before}, {"blade": after})
		var hand: Array = director._status_updates.filter(func(u): return u.data.get("stat") == "hand")
		_expect(hand.size() == 1 and entry[3] in hand[0].data.drawn_ids, "%s: the new hand card gets a flight" % entry[0])
		if hand.size() == 1:
			var origin := str(hand[0].data.drawn_from.get(entry[3], "deck"))
			_expect(origin == entry[0], "%s: flight starts at %s, got %s" % [entry[0], entry[0], origin])
	print("CARD RECOVERY ORIGIN: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
