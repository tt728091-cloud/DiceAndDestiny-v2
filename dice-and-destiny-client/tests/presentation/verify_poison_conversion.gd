extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("inline-status-fixture", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loaded")
	for actor in ["goblin", "blade"]:
		for kind in ["conversion", "incubation"]: await _check(actor, kind)
	var director := BattlePresentationDirector.new()
	director.queue_result({"events": [{"sequence": 200, "type": "proposal_batch_committed", "data": {"poison_conversion": {"converted": false}}}]})
	_expect(not director.has_beats() and director.take_status_updates().is_empty(), "failed conversion never displays an upgrade")
	print("INLINE VENOM STATUS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check(actor: String, kind: String) -> void:
	var fixture := base.duplicate(true)
	fixture.learned_policy = {}; fixture.pending_input = {}; fixture.legal_actions = []
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
	var data: Dictionary
	if kind == "conversion":
		fixture.snapshot.actors[actor].statuses = [{"definition_id": "poison", "stacks": 2}, {"definition_id": "volatile_poison", "stacks": 1}]
		data = {"poison_conversion": {"converted": true, "target_actor_id": actor, "poison_before": 3, "poison_after": 2, "volatile_before": 0, "volatile_after": 1}}
	else:
		fixture.snapshot.actors[actor].statuses = [{"definition_id": "incubation", "stacks": 1}]
		data = {"incubation_application": {"target_actor_id": actor, "before": 0, "after": 1}}
	fixture.events = [{"sequence": 100, "type": "proposal_batch_committed", "segment": "defensive", "data": data}]
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.gateway = BattleGateway.new(fake); screen.initial_result = fixture
	screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("inline-status-test.json"))
	root.add_child(screen)
	for frame in 5: await process_frame
	_expect(not screen._director.has_beats(), "no separate presentation page")
	_expect(screen._view.stage == "planning", "next decision remains on screen")
	var notices := _notices(screen)
	_expect(notices.size() == 1, "one inline status notice")
	await create_timer(0.75).timeout
	var profile: ActorProfile = screen._actor_profiles[actor]
	_expect(("Volatile Poison ×1" if kind == "conversion" else "Incubation ×1") in profile.statuses.text, "target status counter updates")
	if not notices.is_empty():
		_expect(root.get_visible_rect().encloses(notices[0]._label.get_global_rect()), "notice fits viewport")
		_expect(notices[0].mouse_filter == Control.MOUSE_FILTER_IGNORE, "notice cannot block gameplay clicks")
	var capture := OS.get_environment("DICE_AND_DESTINY_INLINE_STATUS_SCREENSHOTS")
	if actor == "goblin" and not capture.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture.path_join("inline-" + kind + ".png"))
	screen._render(); await process_frame
	_expect(_notices(screen).size() == 1, "redraw keeps the original animation without replaying")
	await create_timer(1.2).timeout
	_expect(_notices(screen).is_empty(), "feedback fades away automatically")
	_expect(fake.commands.is_empty(), "no acknowledgement command")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _notices(screen: Node) -> Array:
	var result := []
	for child in screen.get_children():
		if child.get_script() == preload("res://presentation/battle/status_change_notice.gd"): result.append(child)
	return result

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("INLINE STATUS: " + message)
