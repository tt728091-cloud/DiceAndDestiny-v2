extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = gateway.start_battle("round-roll-limits", 1789679833957118)
	initial.events = []; initial.learned_policy = {}
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("round-roll-limits.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	screen.set_process(false)
	# Consecutive rounds: normal, Entangle consumed at entry, then expired.
	# No status remains to infer the penalty from, and no roll event has fired yet.
	for round_index in 3:
		var limit := 2 if round_index == 1 else 3
		for used in range(limit + 1):
			var result := initial.duplicate(true)
			result.snapshot.round = round_index + 1
			result.snapshot.actors.blade.statuses = []
			result.snapshot.actors.blade.dice = {"pool": "offensive", "dice": [], "max_rolls": limit, "rolls_used": used, "rolls_remaining": limit - used}
			result.snapshot.actors.blade.roll_history = []
			for roll in used: result.snapshot.actors.blade.roll_history.append({"dice": []})
			result.pending_input.blade.allowed_commands = ["planning_roll"] if used == 0 else (["planning_reroll"] if used < limit else [])
			if used > 0:
				result.events = [{"type": "dice_rolled", "segment": "offensive", "actor_id": "blade", "max_rolls": limit}]
			_expect(screen._view.apply_result(result), "snapshot accepted")
			screen._render()
			await process_frame
			_expect(screen._view.max_rolls("blade") == limit, "round %d: correct maximum before and after rolling" % (round_index + 1))
			var caption := "%d / %d" % [limit - used, limit]
			var found := false
			for label in screen._roll_dock.find_children("*", "Label", true, false):
				if label.text == caption: found = true
			_expect(found, "roll button displays " + caption)
	# Reopening a reduced-roll battle must not depend on receiving past events.
	var reopened := BattleViewState.new()
	var reduced := initial.duplicate(true)
	reduced.snapshot.actors.blade.dice = {"pool": "offensive", "max_rolls": 2, "rolls_used": 0}
	_expect(reopened.apply_result(reduced) and reopened.max_rolls("blade") == 2, "reopened pre-roll snapshot preserves Entangle limit")
	# A reaction temporarily removes reroll permission, not the round's budget.
	reduced.snapshot.actors.blade.dice.rolls_used = 1
	reduced.snapshot.actors.blade.roll_history = []
	reduced.pending_input.blade.allowed_commands = ["pass_priority"]
	_expect(reopened.apply_result(reduced) and reopened.max_rolls("blade") == 2, "reaction does not change total rolls")
	_expect(reopened.rolls_used("blade") == 1, "snapshot roll count does not require historical roll events")
	reduced.events = [{"type": "dice_rolled", "actor_id": "blade", "max_rolls": 3}]
	_expect(reopened.apply_result(reduced) and reopened.max_rolls("blade") == 2, "current snapshot wins over stale roll events")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("ROUND ROLL LIMITS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("ROUND ROLL LIMITS: " + message)
