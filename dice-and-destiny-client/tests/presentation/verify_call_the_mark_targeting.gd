extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TARGETING := preload("res://presentation/battle/die_face_targeting.gd")
var failed := false
class RecordingGateway extends RefCounted:
	var commands: Array = []
	func submit(command: String) -> Dictionary:
		commands.append(JSON.parse_string(command)); return {"accepted": false, "error": "Test captured submission"}
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("mark-targeting", 9)
	base.events = []; base.learned_policy = {}; base.pending_input = {}
	base.snapshot.segment = "offensive"; base.snapshot.stage = "offensive_reaction"; base.snapshot.flow.stage = "offensive_reaction"
	for actor in ["blade", "goblin"]:
		base.snapshot.actors[actor].roll_history = [{"dice": [{"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 1}, {"die_id": "brine_d6", "face": 5}, {"die_id": "brine_d6", "face": 3}]}]
		base.snapshot.actors[actor].dice = {"pool": "offensive", "dice": base.snapshot.actors[actor].roll_history[0].dice}
	base.legal_actions = []
	for key in ["goblin:0:1", "goblin:1:1", "goblin:1:5", "blade:0:1", "blade:0:6"]:
		base.legal_actions.append({"type": "commit_interaction", "actor_id": "blade", "payload": {"commitment": {"card_ids": ["mark-card"], "choice_id": key}}})
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		var recorder := RecordingGateway.new()
		var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = recorder; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("mark-targeting.json")); root.add_child(screen); await process_frame
		# Feed a revealed board through the view, preserving real content/styles.
		screen._view.stage = "offensive_reaction"
		var card := BattleCard.new(); card.instance_id = "mark-card"; card.definition_id = "call_the_mark"
		screen._on_card_pressed(card); card.free(); await process_frame
		var selector = _selector(screen)
		_expect(selector != null, "card starts inline targeting")
		if selector == null: screen.queue_free(); quit(1); return
		# Revealed dice are authoritative in gameplay; this UI fixture supplies
		# their face/symbol data directly to exercise both sides of the board.
		_setup_dice(screen)
		for frame in 8: await process_frame
		_expect(selector.targets.size() == 3, "only dice with legal choices are highlighted")
		_expect(recorder.commands.is_empty(), "selecting card spends nothing")
		_expect(screen._sole_pass_action().is_empty(), "auto pass waits for choice")
		await _capture("targets-%d" % width)
		selector.targets["goblin:1"].pressed.emit(); await process_frame; _setup_dice(screen)
		selector = _selector(screen)
		for frame in 8: await process_frame
		_expect(selector.faces.size() == 2 and recorder.commands.is_empty(), "multi-face die unfolds legal faces without playing")
		_expect(selector.panel.get_global_rect().end.x < screen._enemy_dice_dock.get_global_rect().position.x, "enemy faces unfold to left")
		_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), "face selector fits viewport")
		await _capture("faces-%d" % width)
		screen._render(); await process_frame; _setup_dice(screen); selector = _selector(screen)
		_expect(selector.faces.size() == 2, "redraw preserves selected die")
		selector.targets["blade:0"].pressed.emit(); await process_frame; _setup_dice(screen); selector = _selector(screen)
		for frame in 8: await process_frame
		_expect(selector.panel.get_global_rect().position.x > screen._player_dice_dock.get_global_rect().end.x, "own faces unfold inward")
		await _capture("own-%d" % width)
		var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; selector._unhandled_key_input(escape); await process_frame
		_expect(_selector(screen) == null and recorder.commands.is_empty(), "cancel clears highlights without sending")
		screen._selected_card = {"instance_id": "mark-card", "definition_id": "call_the_mark", "die_targeting": true}; screen._render(); await process_frame; _setup_dice(screen)
		selector = _selector(screen); selector.targets["goblin:0"].pressed.emit(); await process_frame
		_expect(recorder.commands.size() == 1 and recorder.commands[0].payload.commitment.choice_id == "goblin:0:1", "single-face die submits exact legal action")
		screen._error_message = ""; screen._selected_card = {"instance_id": "mark-card", "definition_id": "call_the_mark", "die_targeting": true, "mark_die": "goblin:1"}; screen._render(); await process_frame; _setup_dice(screen)
		selector = _selector(screen); selector.faces[1].pressed.emit(); await process_frame
		_expect(recorder.commands.size() == 2 and recorder.commands[1].payload.commitment.choice_id == "goblin:1:5", "face click submits chosen face")
		screen._error_message = ""; screen._selected_card = {"instance_id": "mark-card", "definition_id": "call_the_mark", "die_targeting": true}; screen._view.legal_actions = []; screen._render(); await process_frame
		_expect(_selector(screen) == null and screen._selected_card.is_empty(), "lost legality clears selection")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	await _native_check(gateway)
	print("CALL THE MARK TARGETING: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _setup_dice(screen) -> void:
	for dock in [screen._player_dice_dock, screen._enemy_dice_dock]:
		var tray: BattleDiceTray = dock.get_child(0)
		tray.display([{"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 1}, {"die_id": "brine_d6", "face": 5}, {"die_id": "brine_d6", "face": 3}])
func _selector(screen):
	for child in screen._root.get_children():
		if child.get_script() == TARGETING: return child
	return null
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_MARK_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CALL THE MARK TARGETING: " + message)

func _card_action(result: Dictionary, id: String) -> Dictionary:
	for action in result.get("legal_actions", []):
		var payload: Dictionary = action.get("payload", {})
		var ids: Array = payload.get("card_ids", payload.get("commitment", {}).get("card_ids", []))
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == id: return action
	return {}

func _native_check(gateway) -> void:
	var before := {}; var choice := {}
	for seed_value in range(1, 101):
		var result: Dictionary = gateway.start_battle("mark-native-%d" % seed_value, seed_value)
		var hand_has_mark := false
		for instance in result.snapshot.actors.blade.get("hand", []):
			if result.snapshot.actors.blade.card_instances.get(instance, {}).get("definition_id") == "call_the_mark": hand_has_mark = true
		# The card is not legal until reaction; inspect the dealt instances.
		if not hand_has_mark: continue
		var setup := _card_action(result, "mark_the_number")
		if setup.is_empty(): setup = _card_action(result, "black_fingerprint")
		if setup.is_empty(): continue
		result = gateway.submit(JSON.stringify(setup))
		for step in 40:
			choice = _card_action(result, "call_the_mark")
			if not choice.is_empty(): before = result.duplicate(true); break
			if result.snapshot.round > 1: break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			var action := {}
			for candidate in result.get("legal_actions", []):
				if candidate.get("type") in ["planning_pass", "pass"]: action = candidate; break
			if action.is_empty(): break
			result = gateway.submit(JSON.stringify(action))
		if not before.is_empty(): break
	_expect(not before.is_empty(), "native battle reaches legal Call the Mark")
	if before.is_empty(): return
	var presented := 0
	for event in before.events: presented = maxi(presented, int(event.get("sequence", 0)))
	before.events = []; before.learned_policy.model_turn = false
	var screen = SCREEN.instantiate(); screen.initial_result = before; screen.last_presented_sequence = presented; screen.gateway = gateway; screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("mark-native.json")); root.add_child(screen); await process_frame
	var card := BattleCard.new(); card.instance_id = str(choice.payload.commitment.card_ids[0]); card.definition_id = "call_the_mark"
	var energy := int(screen._view.actor("blade").energy_points)
	screen._on_card_pressed(card); card.free(); await process_frame
	var selector = _selector(screen)
	_expect(selector != null, "native card highlights legal dice")
	if selector != null:
		var parts: PackedStringArray = screen._mark_choice(choice).split(":")
		selector.targets["%s:%s" % [parts[0], parts[1]]].pressed.emit(); await process_frame
		if not screen._selected_card.is_empty(): screen._commit_mark_face(choice); await process_frame
		_expect(screen._error_message.is_empty(), "native authority accepts inline choice")
		_expect(int(screen._view.actor("blade").energy_points) == energy - 1, "card costs exactly one Energy after selection")
		_expect(int(screen._view.rolled_dice(parts[0])[int(parts[1])].face) == int(parts[2]), "native target receives selected face")
		_expect(_selector(screen) == null, "successful play removes selector")
		await _check_flip(screen, parts)

	screen.active_store.clear(); screen.queue_free(); await process_frame

func _check_flip(screen, parts: PackedStringArray) -> void:
	var notices: Array = screen.get_children().filter(func(child): return child.get_script() == preload("res://presentation/battle/curse_notice.gd"))
	_expect(notices.size() == 1, "face change creates exactly one visual")
	if notices.is_empty(): return
	var notice = notices[0]; notice.set_process(false)
	# A newly mounted reaction snapshot has not seen the opponent's earlier
	# attack-card reveal; let that queued feedback finish before the flip.
	if notice._waiting(): await create_timer(3.0).timeout
	_expect(not notice._waiting(), "flip follows any earlier queued feedback")
	var change: Dictionary = notice.feedback.changes[0]
	_expect(change.kind == "offensive_face_set" and not is_instance_valid(notice._card), "compact face-set feedback without another card popup")
	var index := int(parts[1])
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		for frame in 8: await process_frame
		var dock: Control = screen._player_dice_dock if parts[0] == "blade" else screen._enemy_dice_dock
		var tray: BattleDiceTray = dock.get_child(0)
		var saved: Array = tray._dice.duplicate(true)
		for tick in range(0, 116):
			notice._elapsed = tick / 100.0; notice.refresh()
			_expect(tray._numbers[index].text in [str(int(change.face_before)), str(int(change.face))], "flip shows only old and chosen faces")
			_expect(not notice._roll.visible, "no roll preview")
			for other in 5:
				if other != index: _expect(tray._numbers[other].text == str(int(saved[other].face)) and tray._buttons[other].scale == Vector2.ONE, "other dice remain unchanged")
		notice._elapsed = 0.28; notice.refresh()
		_expect(tray._numbers[index].text == str(int(change.face_before)) and tray._buttons[index].scale.x < 0.8, "old face folds toward edge")
		await _capture("flip-before-%d" % width)
		notice._elapsed = 0.52; notice.refresh()
		_expect(tray._numbers[index].text == str(int(change.face)) and tray._buttons[index].scale.x < 0.8, "chosen face unfolds")
		await _capture("flip-after-%d" % width)
		screen._render(); await process_frame; notice.refresh()
		tray = (screen._player_dice_dock if parts[0] == "blade" else screen._enemy_dice_dock).get_child(0)
		_expect(tray._buttons[index].scale.x < 0.8, "redraw resumes current flip")
		notice._elapsed = 0.9; notice.refresh()
		_expect(tray._buttons[index].scale == Vector2.ONE and tray._buttons[index].rotation == 0.0, "face settles upright")
		_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()) and notice.modulate.a > 0.5, "caption is visible and stays on screen")
		_expect(tray._dice == saved, "presentation never changes authoritative dice")
		await _capture("flip-settled-%d" % width)
	var elapsed: float = notice._elapsed
	screen._snapshot_panel_open = true; notice._process(0.3)
	_expect(notice._elapsed == elapsed, "inspection pauses flip")
	screen._snapshot_panel_open = false
	screen._director.queue_result({"snapshot": screen._view.raw_snapshot, "events": screen._view.events}, 0, {})
	_expect(screen._director.take_curse_updates().is_empty(), "repeated events do not replay flip")
	notice._elapsed = 0.3; notice.refresh(); notice.queue_free(); await process_frame
	var tray: BattleDiceTray = (screen._player_dice_dock if parts[0] == "blade" else screen._enemy_dice_dock).get_child(0)
	_expect(tray._buttons[index].scale == Vector2.ONE and tray._numbers[index].text == str(int(change.face)), "early cleanup restores final face and transform")
