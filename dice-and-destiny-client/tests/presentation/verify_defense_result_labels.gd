extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	var base: Dictionary = gateway.start_battle("defense-labels", 43)
	for viewport in [Vector2i(1920,1080), Vector2i(1280,720), Vector2i(1024,768)]:
		canvas.size = viewport
		var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
		fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
		fixture.snapshot.damage_sources = []
		fixture.snapshot.defense_history = {}
		fixture.snapshot.defense_selections = {}
		for i in 2:
			var id := "attack-%d" % i
			fixture.snapshot.damage_sources.append({"id": id, "source_actor_id": "goblin" if i == 0 else "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 7, "final_amount": 5, "prevention": 2})
			fixture.snapshot.defense_history[id] = {"actor_id": "blade", "source_id": id, "ability_id": "adventurer_guard_plus", "rolled_face": 2, "rolled_faces": [2,6,2] if i == 0 else [2,3,6], "finalized": true}
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-labels.json"))
		canvas.add_child(screen); screen.set_process(false)
		for frame in 8: await process_frame
		var panel := _check_station(screen, "attack-1", ["Prevent 1", "Prevent 1", "+5 Energy"])
		if panel != null:
			for i in panel.benefit_labels.size()-1:
				_expect(not panel.benefit_labels[i].get_global_rect().intersects(panel.benefit_labels[i+1].get_global_rect()), "neighboring labels have separate bounds")
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/defense-labels-%d.png" % viewport.x)
		# Defend in reverse source order, then clear the selection as finalization does.
		screen._view.defense_selections = {"blade": fixture.snapshot.defense_history["attack-0"].duplicate(true)}
		_check_station(screen, "attack-0", ["Prevent 1", "+5 Energy", "Prevent 1"])
		screen._view.defense_selections.clear()
		screen._render()
		for frame in 8: await process_frame
		_check_station(screen, "attack-0", ["Prevent 1", "+5 Energy", "Prevent 1"])
		# A new round must not keep the previous round's remembered source.
		screen._view.round_number += 1
		_check_station(screen, "attack-1", ["Prevent 1", "Prevent 1", "+5 Energy"])
		# The second six is capped: its longer explanation must not widen its
		# fixed dice cell into the next die's prevention label.
		screen._view.raw_snapshot.defense_history["attack-1"].rolled_faces = [6,6,3]
		screen._render()
		for frame in 8: await process_frame
		panel = _check_station(screen, "attack-1", ["+5 Energy", "Energy already granted", "Prevent 1"])
		if panel != null:
			for i in panel.benefit_labels.size()-1:
				_expect(not panel.benefit_labels[i].get_global_rect().intersects(panel.benefit_labels[i+1].get_global_rect()), "capped energy explanation stays in its own dice cell")
			_expect(panel.dice_controls[1].tooltip_text == "Energy already granted", "capped die retains its full explanation on hover")
			_expect(panel.benefit_labels[1].get_visible_line_count() <= 2, "long benefit stays within two lines")
			_expect(not panel.benefit_labels[1].get_global_rect().intersects(panel.effect_origin.get_global_rect()), "wrapped benefit stays above the defense name")
		if DisplayServer.get_name() != "headless":
			RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/defense-energy-cap-%d.png" % viewport.x)
		screen.queue_free(); await process_frame
	print("DEFENSE RESULT LABELS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check_station(screen: Control, source_id: String, captions: Array) -> Control:
	var visible := []
	for panel in screen._attack_intents.values():
		panel.set_process(false)
		panel.started_ms = Time.get_ticks_msec()-9000; panel.data.roll_started_ms = panel.started_ms; panel._update()
		if panel.roll_area.is_visible_in_tree(): visible.append(panel)
	_expect(visible.size() == 1, "exactly one visible defense roll; got %d" % visible.size())
	if visible.size() != 1: return null
	var panel: Control = visible[0]
	_expect(panel.data.source_id == source_id, "dice station retains the latest selected source")
	_expect(panel.benefit_labels.size() == captions.size(), "one benefit label per die")
	for i in mini(panel.benefit_labels.size(), captions.size()):
		_expect(panel.benefit_labels[i].text == captions[i] and panel.benefit_labels[i].is_visible_in_tree(), "visible label matches the chosen roll")
	return panel
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
