extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("blind-feedback", 13)
	base.learned_policy = {}; base.legal_actions = []; base.pending_input = {}; base.events = []
	for width in [1280, 1920]:
		for face in [1, 2, 3, 6]: await _check(base, width, face)
	print("BLIND FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check(base: Dictionary, width: int, face: int) -> void:
	root.size = Vector2i(width, int(width * 9 / 16.0))
	var result := base.duplicate(true)
	result.snapshot.segment = "offensive"; result.snapshot.stage = "blind_reaction"
	result.snapshot.actors.goblin.statuses = [{"definition_id": "blind", "stacks": 1}]
	result.snapshot.blind_check = {"actor_id": "goblin", "status_id": "blind", "face": face, "die_id": "standard_d6", "ability_id": "brine_lash"}
	result.events = [{"sequence": 1000, "type": "dice_rolled", "segment": "offensive", "round": 1, "actor_id": "goblin", "data": {"source_id": "blind", "source_type": "status", "ability_id": "brine_lash", "rolls": [{"die": {"die_id": "standard_d6", "face": face}}]}}]
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("blind-feedback.json")); root.add_child(screen); await process_frame
	screen.set_process(false)
	var panel = screen._center.get_node_or_null("BlindCheck")
	_expect(panel != null, "Blind roll gets a visible panel")
	if panel == null: screen.queue_free(); await process_frame; return
	panel.set_process(false)
	_expect(screen._director.peek().type == "blind_roll" and screen._sole_pass_action().is_empty(), "roll gates reactions and auto-pass")
	var seen := {}
	for t in [0.1, 0.2, 0.4, 0.6]:
		panel.elapsed = t; panel.refresh(); seen[panel.die.text] = true
	_expect(seen.size() > 1 and "Rolling" in panel.outcome.text, "check visibly rolls")
	panel.elapsed = 1.1; panel.refresh(); await process_frame
	_expect(panel.die.text == str(face) and "Reactions may change" in panel.outcome.text, "authoritative face remains provisional")
	_expect("Brine Lash" in panel.attack.text, "selected ability is named")
	await _capture("roll-%d-%d" % [face, width])
	var elapsed: float = panel.elapsed
	screen._snapshot_panel_open = true; panel._process(0.5)
	_expect(panel.elapsed == elapsed, "inspection pauses")
	screen._snapshot_panel_open = false; panel._process(0.1); elapsed = panel.elapsed
	screen._render(); await process_frame; panel = screen._center.get_node("BlindCheck"); panel.set_process(false)
	_expect(panel.elapsed >= elapsed, "redraw preserves progress")
	panel._process(1.0); await process_frame; await process_frame
	panel = screen._center.get_node_or_null("BlindCheck")
	_expect(panel != null and panel.mode == "pending" and panel.die.text == str(face), "face remains visible during real reaction window")
	var resolved := result.duplicate(true); resolved.snapshot.erase("blind_check"); resolved.snapshot.stage = "defense_select"; resolved.snapshot.segment = "defensive"; resolved.snapshot.actors.goblin.statuses = []
	resolved.events = [{"sequence": 1001, "type": "blind_resolved", "segment": "offensive", "round": 1, "actor_id": "goblin", "data": {"face": face, "die_id": "standard_d6", "ability_id": "brine_lash", "cancelled": face <= 2}}, {"sequence": 1002, "type": "segment_entered", "segment": "defensive", "round": 1}]
	screen._apply_model_result(resolved); await process_frame
	panel = screen._center.get_node_or_null("BlindCheck")
	_expect(panel != null and screen._director.peek().type == "blind_result", "confirmed outcome precedes defense")
	if panel != null:
		panel.set_process(false); panel.elapsed = 1.0; panel.refresh(); await process_frame
		_expect(panel.die.text == str(face), "resolution does not roll a second time")
		_expect(("ATTACK CANCELLED" if face <= 2 else "ATTACK REMAINS") in panel.outcome.text, "both outcomes are explicit")
		_expect(("No offensive ability" if face <= 2 else "Still active") in panel.attack.text, "ability changes visibly only on cancellation")
		_expect("Blind consumed" in panel.outcome.text, "status consumption explained")
		_expect(root.get_visible_rect().encloses(panel.get_global_rect()), "panel fits viewport")
		await _capture("result-%d-%d" % [face, width])
		screen._director.queue_result(resolved, screen._director.last_sequence())
		_expect(screen._director._queue.size() == 1, "duplicate outcome does not replay")
		panel._process(2.0); await process_frame; await process_frame
		_expect(not screen._director.has_beats(), "outcome advances automatically")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_BLIND_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		for frame in 5: await process_frame
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("BLIND FEEDBACK: " + message)
