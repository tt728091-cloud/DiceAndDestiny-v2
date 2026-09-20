extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var base: Dictionary = gateway.start_battle("pending-attack-statuses", 1789679833957118)
	_expect(base.get("accepted") == true, "native content loads")
	base.events = []; base.learned_policy = {}; base.legal_actions = []; base.pending_input = {}
	base.snapshot.segment = "damage_resolution"; base.snapshot.stage = "damage_reaction"
	base.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 1}]
	base.snapshot.actors.goblin.statuses = [{"definition_id": "incubation", "stacks": 1}]
	base.snapshot.settled_damage = {"id": "pending-status-batch", "committed": false, "revealed": true, "removals": [], "sources": [{"id": "needlefang", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 3, "final_amount": 0}], "status_applications": [
		{"source_actor_id": "blade", "target_actor_id": "goblin", "status_id": "poison", "stacks": 1},
		{"source_actor_id": "blade", "target_actor_id": "goblin", "status_id": "poison", "stacks": 2},
		{"source_actor_id": "goblin", "target_actor_id": "blade", "status_id": "bleed", "stacks": 1},
	]}
	var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("pending-attack-statuses.json"))
	root.add_child(screen)
	for frame in 4: await process_frame
	_check_pending(screen)
	var path := OS.get_environment("DICE_AND_DESTINY_PENDING_STATUS_SCREENSHOT")
	if not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(path)
	screen._render(); await process_frame
	_check_pending(screen)
	# An application animation on an unrelated status must not erase queued Poison.
	screen._actor_profiles.goblin.show_status_transition({"kind": "application", "data": {"status_id": "incubation", "before": 0, "after": 1}}, 0.5)
	_expect("Poison ×3 · pending" in screen._actor_profiles.goblin.pending_statuses.text, "status animation preserves pending attacks")
	# Active and incoming stacks stay distinct instead of reporting a false total.
	screen._view.actors.goblin.statuses.append({"definition_id": "poison", "stacks": 1})
	screen._render()
	_expect("Poison ×1" in screen._actor_profiles.goblin.statuses.text and "Poison ×3 · pending" in screen._actor_profiles.goblin.pending_statuses.text, "active stacks separate from proposed additions")
	# A changed batch immediately refreshes the preview.
	screen._view.settled_damage.status_applications = []
	screen._render()
	_expect(not screen._actor_profiles.goblin.pending_statuses.visible, "withdrawn applications disappear")
	# Some completed snapshots retain applications for inspection; never show them pending.
	screen._view.settled_damage.status_applications = base.snapshot.settled_damage.status_applications
	screen._view.settled_damage.committed = true
	screen._view.actors.goblin.statuses = [{"definition_id": "poison", "stacks": 3}]
	screen._render()
	_expect(not screen._actor_profiles.goblin.pending_statuses.visible and "Poison ×3" in screen._actor_profiles.goblin.statuses.text, "committed statuses lose pending marker")
	screen._view.settled_damage.committed = false
	screen._view.segment = "offensive"; screen._view.stage = "planning"
	screen._render()
	_expect(not screen._actor_profiles.goblin.pending_statuses.visible, "stale damage batch does not leak into next segment")
	_expect(base.snapshot.actors.goblin.statuses.size() == 1, "preview never mutates original snapshot")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("PENDING ATTACK STATUSES: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check_pending(screen: Node) -> void:
	var enemy = screen._actor_profiles.goblin
	var player = screen._actor_profiles.blade
	_expect(enemy.pending_statuses.text.count("Poison ×3 · pending") == 1, "blocked Needlefang still previews three Poison once")
	_expect("Bleed ×1 · pending" in player.pending_statuses.text, "incoming player statuses are previewed too")
	_expect("Incubation ×1" in enemy.statuses.text and "Catalyst ×1" in player.statuses.text, "existing statuses remain visible")
	for profile in [enemy, player]:
		_expect(root.get_visible_rect().encloses(profile.pending_statuses.get_global_rect()), "pending statuses fit viewport")
	_expect(screen._view.actor("goblin").statuses.size() == 1, "preview does not apply statuses to authority state")

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("PENDING ATTACK STATUSES: " + message)
