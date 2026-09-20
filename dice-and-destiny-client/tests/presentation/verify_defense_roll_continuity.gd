extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = gateway.start_battle("defense-roll-continuity", 1789679833957118)
	initial.learned_policy = {}; initial.events = []; initial.pending_input = {}; initial.legal_actions = []
	initial.snapshot.segment = "defensive"; initial.snapshot.stage = "defense_reaction"
	initial.snapshot.damage_sources = [
		{"id": "enemy-attack", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 6},
		{"id": "player-attack", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 3}]
	initial.snapshot.defense_selections = {
		"blade": {"actor_id": "blade", "ability_id": "shedskin", "source_id": "enemy-attack", "rolled_face": 4, "rolled_faces": [4, 3], "catalyst_paid": true},
		"goblin": {"actor_id": "goblin", "ability_id": "basic_defense", "source_id": "player-attack", "rolled_face": 5, "rolled_faces": [5]}}
	# Native dice events identify the ability, not the incoming damage source.
	# Depending on who rolled last, one or both actors occur in this response.
	for actors in [["blade"], ["goblin"], ["blade", "goblin"]]:
		var rolled := initial.duplicate(true)
		for actor in actors:
			var selection: Dictionary = rolled.snapshot.defense_selections[actor]
			var dice: Array = []
			for face in selection.rolled_faces: dice.append({"face": face})
			rolled.events.append({"type": "dice_rolled", "segment": "defensive", "pool": "defensive", "actor_id": actor, "source_id": selection.ability_id, "dice": dice})
		var screen = SCREEN.instantiate(); screen.initial_result = rolled
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-roll-continuity.json")); screen._auto_pass_disabled = true
		root.add_child(screen)
		for frame in 5: await process_frame
		var key: String = screen._defense_review_key()
		var start_times := {}
		# Advance the real playback clocks to settled results without slowing the test.
		for clock in screen._defense_animation_times: screen._defense_animation_times[clock] -= 10000
		for panel in screen._defense_result_panels:
			panel.started_ms -= 10000; panel._update()
			start_times[panel.data.actor_id] = panel.started_ms
		for update in [initial, rolled, initial]:
			# Empty-event priority handoff, duplicate events, then another redraw.
			screen._apply_model_result(update.duplicate(true))
			_expect(screen._defense_review_key() == key, "same defense keeps its identity across event/snapshot handoffs " + str(actors))
			for frame in 5:
				await process_frame
				for panel in screen._defense_result_panels:
					_expect(panel.started_ms == start_times[panel.data.actor_id], "settled animation clock never restarts")
					for index in panel.dice_controls.size():
						_expect(panel.dice_controls[index].tooltip_text != "Rolling…", "settled dice never roll again")
						_expect(panel.dice_controls[index].text.ends_with("\n%d" % int(panel.data.dice[index].face)), "settled faces remain visible")
		_expect(screen._view.defense_rolls.blade.rolled_faces == [4, 3], "all Shedskin dice preserved")
		_expect(screen._view.defense_rolls.blade.catalyst_paid, "Catalyst payment preserved")
		# An actual changed face must still be presented as a new result.
		var changed := initial.duplicate(true)
		changed.snapshot.defense_selections.blade.rolled_face = 2
		changed.snapshot.defense_selections.blade.rolled_faces = [2, 3]
		screen._apply_model_result(changed)
		_expect(screen._defense_review_key() != key, "real dice changes remain distinguishable")
		_expect(screen._defense_result_panels[0].started_ms > int(start_times.blade), "real dice changes start new playback")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DEFENSE ROLL CONTINUITY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("DEFENSE ROLL CONTINUITY: " + message)
