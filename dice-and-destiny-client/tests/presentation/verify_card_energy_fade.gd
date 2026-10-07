extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_energy_notice.gd")
const GAIN := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var base: Dictionary = gateway.start_battle("energy-fade", 43)
	for width in [1280, 1920]:
		root.size = Vector2i(width, width * 9 / 16)
		for scenario in [["take_stock", "offensive", 3, 2, 1], ["nudge", "offensive", 2, 0, 2], ["brace", "defensive", 3, 2, 1], ["second_wind", "offensive", 3, 5, 0], ["battle_focus", "offensive", 3, 4, 1], ["brace", "defensive", 0, 0, 0]]:
			await _scenario(base, scenario)
	print("CARD ENERGY FADE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(base: Dictionary, values: Array) -> void:
	var initial := base.duplicate(true); initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	initial.snapshot.segment = values[1]; initial.snapshot.stage = "planning" if values[1] == "offensive" else "defense_selection"
	initial.snapshot.actors.blade.energy_points = values[2]
	var result := initial.duplicate(true); result.accepted = true
	result.snapshot.actors.blade.energy_points = values[3]
	result.snapshot.actors.blade.hand_count -= 1
	result.events = [{"sequence":100, "type":"card_played", "actor_id":"blade", "segment":values[1], "energy_cost":values[4], "data":{"card_instance_id":"test-card", "card_definition_id":values[0]}}]
	var fake := FakeBattleAuthority.new(); fake.enqueue(result)
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("energy-fade.json")); root.add_child(screen); screen.set_process(false)
	for frame in 4: await process_frame
	screen._send(JSON.stringify({"type":"planning_commit_cards" if values[1] == "offensive" else "commit_interaction", "actor_id":"blade", "battle_id":initial.snapshot.battle_id, "payload":{}}))
	_expect(fake.commands.size() == 1, "accepted card play submitted")
	var notices := screen.get_children().filter(func(node): return node.get_script() == NOTICE and not node.is_queued_for_deletion())
	_expect(notices.size() == (1 if values[4] > 0 else 0), "only an actual payment creates a payment fade: " + values[0])
	for child in screen.get_children():
		if child.get_script() == GAIN: child.set_process(false)
	if not notices.is_empty():
		var notice = notices[0]; notice.set_process(false)
		var label: Label = screen._actor_profiles.blade._stat_labels.energy
		_expect(label.text == str(values[2]) and label.self_modulate.a == 1, "old energy shown before first draw; no final-value flash")
		notice.elapsed = notice.DURATION * 0.25; notice.refresh()
		_expect(label.text == str(values[2]) and is_equal_approx(label.self_modulate.a, 0.5), "old number fades out")
		for frame in 3: await process_frame
		screen._render()
		label = screen._actor_profiles.blade._stat_labels.energy
		_expect(label.text == str(values[2]) and is_equal_approx(label.self_modulate.a, 0.5), "redraw preserves fade progress")
		notice.elapsed = notice.DURATION * 0.75; notice.refresh()
		_expect(label.text == str(values[2] - values[4]) and is_equal_approx(label.self_modulate.a, 0.5), "new number fades in")
		var path := OS.get_environment("DICE_AND_DESTINY_ENERGY_SCREENSHOT")
		if not path.is_empty() and values[0] == "take_stock" and root.size.x == 1920 and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(path)
		# A later payment owns the label before the older node is freed.
		var replacement := NOTICE.new(); replacement.configure(screen, notice.data); screen.add_child(replacement); replacement.set_process(false)
		notice.finish(); replacement.elapsed = replacement.DURATION * 0.25; replacement.refresh(); await process_frame
		_expect(is_equal_approx(label.self_modulate.a, 0.5), "older cleanup cannot cancel a newer fade")
		replacement.elapsed = replacement.DURATION; replacement.refresh(); await process_frame
		_expect(label.self_modulate.a == 1, "payment fade restores full opacity")
	for child in screen.get_children():
		if child.get_script() != GAIN: continue
		for update in child.updates:
			if update.kind != "resource" or update.data.stat != "energy": continue
			var profile: ActorProfile = screen._actor_profiles.blade
			profile.show_resource_gain(update.data, 0.25)
			_expect(profile._stat_labels.energy.text == str(update.data.before) and is_equal_approx(profile._stat_labels.energy.self_modulate.a, 0.5), "energy reward fades out its baseline")
			profile.show_resource_gain(update.data, 0.75)
			_expect(profile._stat_labels.energy.text == str(values[3]) and is_equal_approx(profile._stat_labels.energy.self_modulate.a, 0.5), "energy reward fades into authority value")
			profile.show_resource_gain(update.data, 1)
	_expect(screen._view.actor("blade").energy_points == values[3], "animation never changes authoritative energy")
	# The same event cannot restart an energy fade on resubmission.
	screen._director.queue_result(result, screen._director.last_sequence(), initial.snapshot.actors)
	_expect(screen._director.take_card_energy_updates().is_empty(), "duplicate card events do not replay fade")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CARD ENERGY FADE: " + message)
