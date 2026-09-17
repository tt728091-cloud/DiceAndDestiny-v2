extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void: call_deferred("_run")

func _fixture(id: String, face: int = 6, rerolled: bool = false) -> Dictionary:
	var fixture := base.duplicate(true)
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "status_roll_reaction"
	fixture.snapshot.effect_rolls = [{"actor_id": "goblin", "source_content_id": "volatile_poison", "resolved": true, "rerolled": rerolled, "die": {"die_id": "standard_d6", "face": face}}, {"actor_id": "goblin", "source_content_id": "volatile_poison", "resolved": true, "die": {"die_id": "standard_d6", "face": 2}}]
	fixture.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 1 if rerolled else 2}]
	fixture.snapshot.actors.goblin.statuses = [{"definition_id": "volatile_poison", "stacks": 2}]
	fixture.events = []; fixture.learned_policy = {}
	fixture.pending_input = {"blade": {"id": id, "segment": "offensive", "stage": "status_roll_reaction", "allowed_commands": ["pass", "commit_interaction"]}}
	fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": id}}]
	if rerolled: fixture.events = [{"sequence": 10, "type": "dice_rolled", "segment": "offensive", "actor_id": "goblin", "data": {"source_type": "catalyst", "holder": "blade", "die_index": 0, "face_before": 6}}]
	return fixture

func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = native.start_battle("provoked-toxins", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog loads")
	for final_face in [2, 6]:
		var fake := FakeBattleAuthority.new()
		fake.enqueue(_fixture("rerolled", final_face, true))
		var done := _fixture("damage", final_face, true)
		done.snapshot.stage = "status_damage_reaction"; done.snapshot.effect_rolls = []; done.events = []; done.pending_input = {}; done.legal_actions = []
		fake.enqueue(done)
		var screen = SCREEN.instantiate(); screen.gateway = BattleGateway.new(fake); screen.initial_result = _fixture("original")
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("provoked-test.json"))
		screen._auto_pass_disabled = true
		root.add_child(screen); screen.set_process(false)
		for frame in 3: await process_frame
		var panel = screen._provoked_panel
		var die = panel.entries[0].die
		var other_die = panel.entries[1].die
		screen.learned_battle_mode = true
		screen._schedule_model_if_needed({"learned_policy": {"model_turn": true}})
		_expect(not screen._model_thinking, "opponent automation waits for the visible roll")
		screen.learned_battle_mode = false
		_expect(is_instance_valid(panel), "provoked toxins use animated display")
		for label in screen._center.find_children("*", "Label", true, false):
			_expect(not label.text.contains("At Ongoing Effects") and not label.text.contains("These dice have already rolled"), "legacy rules wall removed")
		screen._auto_pass_if_only_action()
		_expect(fake.commands.is_empty(), "first roll remains visible")
		_age(panel, 1.2)
		_expect(die.text.ends_with("\n6") and other_die.text.ends_with("\n2"), "original clearing six is visible before Catalyst")
		await _capture("original-%d" % final_face)
		_age(panel, 2.5)
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "forced handoff needs no Continue even with debug auto-pass disabled")
		_expect(screen._provoked_panel == panel and panel.entries[0].die == die, "same panel and die retained across authority handoff")
		_expect(panel.entries[0].cue.text.contains("Catalyst") and die.text.ends_with("\n6"), "Catalyst announces use while original six stays visible")
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "reroll cannot be skipped by automatic passage")
		_age(panel, 1.0)
		_expect(die.rotation != 0 and other_die.rotation == 0, "only selected die animates again")
		await _capture("reroll-%d" % final_face)
		_age(panel, 2.0)
		_expect(die.text.ends_with("\n%d" % final_face) and other_die.text.ends_with("\n2"), "authoritative final face lands without changing other die")
		_expect(panel.entries[0].result.text.contains("2 damage pending") if final_face == 2 else panel.entries[0].result.text.contains("Remove 1 Volatile Poison"), "correct damage or clearing result shown")
		await _capture("result-%d" % final_face)
		screen._render(); await process_frame
		_expect(screen._provoked_panel == panel and panel.entries[0].die == die and die.text.ends_with("\n%d" % final_face), "redraw does not replay Catalyst or replace dice")
		# A real card response remains a choice rather than an automatic pass.
		screen._view.legal_actions.append({"actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "rerolled"}})
		_age(panel, 3.3); screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "legal reaction choices preserved")
		screen._view.legal_actions.pop_back()
		panel.set_paused(true); screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "developer inspection pauses playback and passage")
		panel.set_paused(false); screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 2 and screen._view.stage == "status_damage_reaction", "settled result proceeds once to actual damage choices")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	# Exercise the actual frame-driven automation without simulated button clicks.
	var auto_fake := FakeBattleAuthority.new(); auto_fake.enqueue(_fixture("auto-retry", 2, true))
	var complete := _fixture("auto-done", 2, true); complete.snapshot.stage = "status_damage_reaction"; complete.events = []; complete.legal_actions = []; complete.pending_input = {}
	auto_fake.enqueue(complete)
	var live = SCREEN.instantiate(); live.gateway = BattleGateway.new(auto_fake); live.initial_result = _fixture("auto-original"); live._auto_pass_disabled = true
	live.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("provoked-live.json")); root.add_child(live)
	await create_timer(6.0).timeout
	_expect(auto_fake.commands.size() == 2 and live._view.stage == "status_damage_reaction", "entire original roll and Catalyst reroll finish without a click")
	live.active_store.clear(); live.queue_free(); await process_frame
	print("PROVOKED TOXINS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _age(panel, seconds: float) -> void:
	# Only age entries from their own current roll; unaffected dice are already settled.
	for entry in panel.entries:
		entry.started = Time.get_ticks_msec() - int((seconds if entry.catalyst else maxf(seconds, 2.5)) * 1000)
	panel._process(0)

func _capture(name: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_PROVOKE_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(name + ".png"))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("PROVOKED TOXINS: " + message)
