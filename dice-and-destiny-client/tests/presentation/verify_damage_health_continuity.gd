extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var recorded: Array = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/molt_damage_effects_handoff.json"))
	var prior := {
		"blade": {"current_health": 4, "hand_count": 3, "deck_count": 0, "discard_count": 1, "removed_count": 20, "energy_points": 1},
		"goblin": {"current_health": 13, "hand_count": 4, "deck_count": 4, "discard_count": 5, "removed_count": 7, "energy_points": 3}}
	var result := {"events": recorded, "snapshot": {"segment": "offensive", "actors": prior}, "pending_input": {}}
	var director := BattlePresentationDirector.new()
	director.queue_result(result, 0, prior)
	_expect(director.peek().type == "combat_damage", "recorded combat is shown before Effects")
	for actor in prior:
		_expect(director.pending_damage_actor_before(actor) == prior[actor], "recorded handoff never restores next-round losses or rewrites previous health: " + actor)
	# New authority correctly removes the played Molt from discard: 4 -> 3.
	# Following Bleed removes the other three cards. Exercise live and restored UI.
	var corrected := recorded.duplicate(true)
	corrected[0].data.removals[0].original_zone = "discard"
	var effects: Dictionary = corrected[-1].data
	effects.actors_before.blade.health = 3; effects.actors_before.blade.discard_count = 0; effects.actors_before.blade.removed_count = 21
	effects.actors_after.blade.health = 0; effects.actors_after.blade.hand_count = 0; effects.actors_after.blade.removed_count = 24
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = native.start_battle("damage-health-continuity", 1789765347477784)
	initial.events = []; initial.learned_policy = {}; initial.pending_input = {}; initial.legal_actions = []
	for actor in prior: initial.snapshot.actors[actor].merge(prior[actor], true)
	for restored in [false, true]:
		var screen = SCREEN.instantiate(); screen.initial_result = initial; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("damage-health-continuity.json"))
		root.add_child(screen); screen.set_process(false)
		screen._director.clear()
		var next := initial.duplicate(true); next.events = corrected
		screen._director.queue_result(next, 0, {} if restored else prior)
		screen._render()
		_expect(screen._actor_profiles.blade.health.value == 4, "combat begins at four health, including restored playback")
		_expect(screen._actor_profiles.goblin.health.value == 13, "enemy combat begins at thirteen, never sixteen")
		screen._show_damage_counts(1.0)
		_expect(screen._actor_profiles.blade.health.value == 3, "one combat removal costs one health")
		_expect(screen._actor_profiles.goblin.health.value == 12, "enemy loses one health")
		_expect(screen._director.pending_damage_actor_before("blade").discard_count == 1, "Molt is removed from discard, not hand")
		screen._director.advance(); screen._render()
		_expect(screen._director.peek().type == "effects_resolved", "Effects follow combat")
		_expect(screen._actor_profiles.blade.health.value == 3, "Effects start at three with no health jump")
		_expect(screen._actor_profiles.goblin.health.value == 12, "enemy Effects start at twelve")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE HEALTH CONTINUITY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE HEALTH CONTINUITY: " + message)
