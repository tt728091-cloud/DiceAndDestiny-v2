extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var actors := {}
	for actor_id in ["blade", "goblin", "enemy_2", "enemy_3", "enemy_4"]:
		actors[actor_id] = {"health": 20, "energy": 1, "deck_count": 15, "hand_count": 5, "discard_count": 0, "removed_count": 0, "statuses": []}
	var empty := {"actors_before": actors, "actors_after": actors.duplicate(true), "steps": null}
	for steps in [null, []]:
		empty.steps = steps
		var director := BattlePresentationDirector.new()
		var result := _result(empty)
		director.queue_result(result)
		_expect(director.peek().get("type") == "income_summary", "empty Effects goes directly to Income for all five actors")
		_expect(director.pending_effects_actor_before("blade").is_empty(), "no empty Effects profile override")
		director.queue_result(result)
		_expect(director._queue.size() == 1, "repeated result cannot replay skipped Effects or duplicate Income")
		_expect(director.pending_income_actor("blade").get("card_count") == 1, "card draw animation is retained")
		_expect(director.pending_income_actor("blade").get("energy_gain") == 1, "energy animation is retained")
		director.advance()
		_expect(not director.has_beats() and director.last_sequence() == 6, "Income flows to Offense and acknowledges skipped events")
		var only_empty := BattlePresentationDirector.new()
		only_empty.queue_result({"events": result.events.slice(0, 2)})
		_expect(not only_empty.has_beats() and only_empty.last_sequence() == 2, "empty-only results do not leave a wait or unacknowledged event")

	# Idle persistent statuses alone do not create work; actual expiry, conversion,
	# resource changes and even zero-damage rolls on any combatant still do.
	var idle := empty.duplicate(true)
	idle.actors_before.enemy_4.statuses = [{"id": "curse", "stacks": 2}]
	idle.actors_after.enemy_4.statuses = idle.actors_before.enemy_4.statuses.duplicate(true)
	_expect_first(idle, "income_summary", "unchanged persistent status does not pause the game")
	for actor_id in actors:
		for field in ["health", "energy", "deck_count", "hand_count", "discard_count", "removed_count", "statuses"]:
			var changed := empty.duplicate(true)
			changed.actors_after[actor_id][field] = [{"id": "poison", "stacks": 1}] if field == "statuses" else int(changed.actors_after[actor_id][field]) + 1
			_expect_first(changed, "effects_resolved", "%s %s change remains visible" % [actor_id, field])
	var expiry := idle.duplicate(true); expiry.actors_after.enemy_4.statuses = []
	_expect_first(expiry, "effects_resolved", "status expiry without damage remains visible")
	var rolled := empty.duplicate(true)
	rolled.steps = [{"type": "dice_rolled", "actor_id": "enemy_4", "data": {"damage": 0}}]
	_expect_first(rolled, "effects_resolved", "zero-damage roll remains visible")
	_expect_first({}, "effects_resolved", "incomplete historical summary is preserved conservatively")
	var legacy := BattlePresentationDirector.new()
	legacy.queue_result({"events": [{"sequence": 1, "type": "segment_entered", "segment": "ongoing_effects"}]})
	_expect(not legacy.has_beats() and legacy.last_sequence() == 1, "bare Effects marker creates no empty screen")

	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "curse")
	var fixture: Dictionary = gateway.start_battle("empty-effects-flow", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	# Drive the authority across a real round boundary with no player statuses.
	var native_empty := 0
	var native_result := fixture
	for step in 100:
		for event in native_result.get("events", []):
			if event.get("type") == "effects_resolved" and event.data.actors_before == event.data.actors_after:
				native_empty += 1
				empty = event.data.duplicate(true)
				_expect_first(event.data, "income_summary", "native empty Effects summary is skipped")
		if native_empty > 0: break
		if native_result.get("learned_policy", {}).get("model_turn", false):
			native_result = gateway.advance_model()
		else:
			var chosen := {}
			for kind in ["planning_pass", "pass", "planning_roll", "roll_dice"]:
				for action in native_result.get("legal_actions", []):
					if action.type == kind: chosen = action; break
				if not chosen.is_empty(): break
			if chosen.is_empty(): break
			native_result = gateway.submit(JSON.stringify(chosen))
		_expect(native_result.get("accepted") == true, "native round advancement is accepted")
	_expect(native_empty > 0, "native round supplies an empty Effects summary")
	var boundary := BattlePresentationDirector.new()
	boundary.queue_result(native_result)
	var saw_damage := false
	var saw_income := false
	while boundary.has_beats():
		var beat := boundary.peek()
		_expect(beat.type != "effects_resolved" and beat.get("presentation_segment") != "ongoing_effects", "real round boundary has no empty Effects beat")
		if beat.type == "combat_damage":
			saw_damage = true
			boundary.advance()
			_expect(boundary.peek().get("type") == "income_summary", "real Damage proceeds immediately to Income")
			continue
		if beat.type == "income_summary": saw_income = true
		boundary.advance()
	_expect(saw_damage and saw_income, "real authority result includes Damage and Income")
	for mode in ["grave_interest", "preserve"]:
		var modified := empty.duplicate(true)
		modified.steps = [{"type": "curse_resolved", "data": {"kind": "conversion", "count_before": 0, "count_after": 0, "damage": 0, "mode": mode}}]
		_expect_first(modified, "effects_resolved", "conversion modifier remains visible: " + mode)
	var timeline := _result(empty)
	fixture.events = timeline.events
	fixture.learned_policy = {}; fixture.pending_input = {}; fixture.legal_actions = []
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"; fixture.snapshot.round = 2
	# Use a real hand instance for the Income card animation.
	var hand: Array = fixture.snapshot.actors.blade.hand
	fixture.events[3].cards = hand.slice(0, 1)
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("empty-effects-flow.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	_expect(screen._director.peek().get("type") == "income_summary", "first rendered frame is Income")
	_check_no_empty_effects(screen)
	for frame in 5:
		await process_frame
		_check_no_empty_effects(screen)
	_expect(screen._income_drawn_cards.size() == 1, "Income still displays its drawn card")
	var path := OS.get_environment("DICE_AND_DESTINY_EMPTY_EFFECTS_SCREENSHOT")
	if not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(path)
	screen._finish_income_presentation(screen._income_animation_generation)
	_expect(not screen._director.has_beats(), "Income ends directly in offensive planning")
	_check_no_empty_effects(screen)
	_expect(fake.commands.is_empty(), "skipping presentation sends no gameplay commands")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("EMPTY EFFECTS FLOW: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _result(data: Dictionary) -> Dictionary:
	return {"snapshot": {"segment": "offensive"}, "events": [
		{"sequence": 1, "type": "segment_entered", "segment": "ongoing_effects", "round": 2},
		{"sequence": 2, "type": "effects_resolved", "segment": "ongoing_effects", "round": 2, "data": data},
		{"sequence": 3, "type": "segment_entered", "segment": "income", "round": 2},
		{"sequence": 4, "type": "cards_drawn", "segment": "income", "round": 2, "actor_id": "blade", "cards": ["drawn-card"], "count": 1},
		{"sequence": 5, "type": "energy_points_gained", "segment": "income", "round": 2, "actor_id": "blade", "energy_points": 2, "amount": 1},
		{"sequence": 6, "type": "segment_entered", "segment": "offensive", "round": 2},
	]}

func _expect_first(data: Dictionary, kind: String, message: String) -> void:
	var director := BattlePresentationDirector.new()
	director.queue_result(_result(data))
	_expect(director.peek().get("type") == kind, message)

func _check_no_empty_effects(screen: Node) -> void:
	_expect(not is_instance_valid(screen._effects_panel), "no Effects panel is instantiated")
	for label in screen.find_children("*", "Label", true, false):
		_expect(label.text not in ["No damaging effects", "No ongoing effects to resolve", "Effects settle", "Round 2 · Effects"], "no empty Effects text flashes onscreen")

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("EMPTY EFFECTS FLOW: " + message)
