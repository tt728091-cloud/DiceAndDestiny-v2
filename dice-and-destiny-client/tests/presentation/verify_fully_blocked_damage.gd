extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const LOG := preload("res://local_client/view_state/combat_log.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	# Recorded round 4: all three attacks blocked, Poison still applies, then
	# next-round Effects remove two enemy cards. Go encodes no removals as null.
	var recorded: Array = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/brine_fully_blocked_damage.json"))
	var prior := {
		"blade": {"current_health": 8, "deck_count": 1, "hand_count": 3, "discard_count": 4, "removed_count": 16},
		"goblin": {"current_health": 4, "deck_count": 2, "hand_count": 1, "discard_count": 1, "removed_count": 12},
		"goblin-2": {"current_health": 16, "deck_count": 11, "hand_count": 1, "discard_count": 4, "removed_count": 0}}
	var result := {"events": recorded, "snapshot": {"battle_id": "blocked-replay", "round": 5, "segment": "offensive", "actors": prior}, "pending_input": {}}
	var log := LOG.new(); log.receive(result)
	_expect("Venom took 0 damage" in log.text() or "Blade took 0 damage" in log.text(), "log records fully prevented incoming damage")
	_expect("lost Brine Surge" in log.text() and "Round 5 · Income" in log.text(), "log survives empty removals and records subsequent effects and income")
	for restored in [false, true]:
		var director := BattlePresentationDirector.new()
		director.queue_result(result, 0, {} if restored else prior)
		_expect(director.peek().get("type") == "combat_damage", "blocked attacks queue Damage before next-round Effects, including save restoration")
		if director.peek().get("type") != "combat_damage": continue
		var batch: Dictionary = director.peek().event.data
		_expect(batch.removals == [], "null removals normalize to an empty list")
		_expect(batch.sources.size() == 3 and batch.status_applications.size() == 1, "blocked attacks and their Poison application survive")
		_expect(int(director.pending_damage_actor_before("blade").get("current_health", -1)) == 8, "no player health lost")
		director.queue_result(result, 0, prior)
		director.advance()
		_expect(director.peek().get("type") == "effects_resolved", "Effects follow Damage without duplicate playback")
		director.advance()
		_expect(director.peek().get("type") == "income_summary", "Income follows Effects")
		director.advance()
		_expect(not director.has_beats(), "Offensive becomes available only after all queued phases")
	if failed:
		print("FULLY BLOCKED DAMAGE: FAILED"); quit(1); return
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-b", "brine-mask-pair", "venom")
	var initial: Dictionary = gateway.start_battle("blocked-screen", 1789919350231688)
	initial.events = []; initial.learned_policy = {}; initial.pending_input = {}; initial.legal_actions = []
	initial.snapshot.round = 5; initial.snapshot.segment = "offensive"
	for actor in prior: initial.snapshot.actors[actor].merge(prior[actor], true)
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("blocked-damage-test.json"))
	root.add_child(screen); screen.set_process(false)
	var update := initial.duplicate(true); update.events = recorded
	screen._apply_model_result(update)
	await process_frame
	_expect(screen._director.peek().get("type") == "combat_damage", "live model response holds the Damage presentation")
	_expect(not screen._enemy_selector.visible, "battlefield nameplates do not cover queued damage panels")
	_expect(_has_label(screen, "No cards lost"), "blocked damage visibly reports no cards lost")
	screen._show_damage_counts(1.0)
	_expect(screen._actor_profiles.blade.health.value == 8, "combat playback leaves player health unchanged")
	screen._select_enemy("goblin-2")
	_expect(screen._actor_profiles["goblin-2"].health.value == 16, "combat does not prematurely show next-round poison damage")
	_expect(_has_label(screen, "No cards lost"), "second incoming attack remains inspectable with zero damage")
	var capture := OS.get_environment("DICE_AND_DESTINY_BLOCKED_DAMAGE_SCREENSHOT")
	if not capture.is_empty() and DisplayServer.get_name() != "headless":
		await create_timer(0.8).timeout
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture)
	# A zero-damage attack without attached statuses must still show its result;
	# status-only batches must retain their application without inventing damage.
	for status_only in [false, true]:
		var variant := update.duplicate(true); variant.events = [recorded[0].duplicate(true)]
		if status_only: variant.events[0].data.sources = null
		else: variant.events[0].data.status_applications = null
		var variant_director := BattlePresentationDirector.new(); variant_director.queue_result(variant, 0, prior)
		_expect(variant_director.peek().get("type") == "combat_damage", "blocked and status-only batches both retain a Damage beat")
	# Null sources/applications also occur for status-only or empty legacy batches.
	var empty := update.duplicate(true)
	empty.events = [{"sequence": 999, "type": "damage_committed", "segment": "damage_resolution", "data": {"sources": null, "removals": null, "status_applications": null, "overage": null}}]
	var empty_log := LOG.new(); empty_log.receive(empty)
	var empty_director := BattlePresentationDirector.new(); empty_director.queue_result(empty)
	_expect(not empty_director.has_beats(), "truly empty batches create no phantom damage")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("FULLY BLOCKED DAMAGE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _has_label(node: Node, text: String) -> bool:
	for label in node.find_children("*", "Label", true, false):
		if text in label.text: return true
	return false

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("FULLY BLOCKED DAMAGE: " + message)
