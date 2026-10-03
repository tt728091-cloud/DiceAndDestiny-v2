extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var f: Dictionary = gateway.start_battle("stack-commit", 43)
	f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
	var before: Dictionary = f.snapshot.actors.duplicate(true)
	var batch := {"id": "three-attacks", "sources": [], "removals": [], "actors_before": before}
	for i in 3:
		var id := "attack-" + str(i)
		batch.sources.append({"id": id, "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": [4,2,3][i], "final_amount": [4,2,3][i]})
		for j in [4,2,3][i]: batch.removals.append({"card_id": id + "-" + str(j), "card_definition_id": "curse_bloom", "accepted": true, "target_actor_id": "blade", "original_zone": "deck", "damage_proposal_ids": [id]})
	# The authoritative state already contains the removals; presentation must
	# temporarily hold the previous totals and count to this exact final state.
	f.snapshot.actors.blade.current_health = 15; f.snapshot.actors.blade.removed_count = 9; f.snapshot.actors.blade.deck_count = 10
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("stack-commit.json"))
	root.add_child(screen); screen.set_process(false)
	screen._director._queue = [{"type": "combat_damage", "event": {"data": batch}, "watermark": 1}]
	var key: String = screen._view.battle_id + ":three-attacks"
	screen._damage_card_times[key] = Time.get_ticks_msec() - 10000
	screen._damage_commit_started[key] = Time.get_ticks_msec() + 60000
	screen._render()
	for frame in 12: await process_frame
	_expect(screen._damage_grids.size() == 3, "all three committed sources preserved")
	var clock: int = screen._damage_grids[0].removal_started_ms
	for grid in screen._damage_grids: _expect(grid.removal_started_ms == clock, "every stack tears on the same clock")
	screen._damage_commit_started[key] = Time.get_ticks_msec()
	for grid in screen._damage_grids: grid.removal_started_ms = int(screen._damage_commit_started[key])
	var previous := 0; var observed := {}; var deadline := Time.get_ticks_msec() + 5000
	while Time.get_ticks_msec() < deadline and screen._director.has_beats():
		var profile: Control = screen._actor_profiles.blade
		var removed := int(profile._stat_labels.removed.text)
		_expect(removed >= previous, "removed total never reverses")
		_expect(int(profile.health.value) == 24 - removed, "health loss and Remove counter stay synchronized")
		observed[removed] = true; previous = removed
		await process_frame
	_expect(not screen._director.has_beats(), "animation completes without another click")
	_expect(int(screen._actor_profiles.blade._stat_labels.removed.text) == 9 and screen._actor_profiles.blade.health.value == 15, "combined losses reach exact final totals once")
	_expect(observed.size() >= 8, "counter ticks rapidly through individual losses")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE STACK COMMIT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE STACK COMMIT: " + message)
