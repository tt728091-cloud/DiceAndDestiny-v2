extends SceneTree
const SCREEN = preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY = preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
var active: Control
var last_banner := ""
var highest_phase := 0
var seen: Array[String] = []
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	_check_round_context()
	canvas = SubViewport.new(); canvas.size = Vector2i(1920,1080); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	RenderingServer.frame_pre_draw.connect(_check_frame)
	for width in [1920,1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width)
	print("ROUND TRANSITION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int) -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("round-transition-%d" % width, 44)
	for step in 100:
		if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
		if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
		else: result = gateway.submit(JSON.stringify(_next_action(result.legal_actions)))
	_expect(result.snapshot.stage == "defense_selection" and int(result.snapshot.round) == 1, "native fixture reaches round one Defense")
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("round-transition.json"))
	canvas.add_child(screen)
	for frame in 12: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	active = screen; last_banner = ""; highest_phase = 0; seen.clear()
	_check_frame()
	_expect(is_instance_valid(screen._auto_pass_button) and not screen._auto_pass_button.disabled, "main Pass is pointer accessible")
	var point: Vector2 = screen._auto_pass_button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion,true)
	for pressed in [true,false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.position = point; click.pressed = pressed; canvas.push_input(click,true)
	var deadline := Time.get_ticks_msec() + 45000
	while Time.get_ticks_msec() < deadline:
		await process_frame; _check_frame()
		if screen._view.round_number == 2 and screen._view.stage == "planning" and not screen._director.has_beats() and not screen._model_thinking:
			break
	_expect(screen._view.round_number == 2 and screen._view.stage == "planning" and not screen._director.has_beats(), "Pass advances to round two Offense")
	for frame in 12: await process_frame; _check_frame()
	_expect("Round 2 · Income" in seen and "Round 2 · Offense" in seen, "transition presents round two Income and Offense")
	active = null; screen.active_store.clear(); screen.queue_free(); await process_frame
func _check_frame() -> void:
	if not is_instance_valid(active): return
	var inverse: Transform2D = active._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * active._player_profile_dock.get_global_rect()
	_expect(rect.position.y >= active.PLAYER_ZONE_TOP and absf(rect.end.y - 1060) < 2, "player HUD remains bottom-right on every frame: " + str(rect))
	var profile: Control = active._actor_profiles[active.viewer_actor_id]
	var profile_rect: Rect2 = inverse * profile.get_global_rect()
	_expect(profile_rect.position.y >= active.PLAYER_ZONE_TOP and profile_rect.end.y <= 1062, "visible player stats stay inside bottom-right dock: " + str(profile_rect))
	var banner: Label = active._root.find_child("RoundBanner", true, false)
	if banner.text != last_banner:
		last_banner = banner.text; seen.append(last_banner)
		print("BANNER: ",last_banner, " / view ",active._view.round_number, " ",active._view.stage, " / beat ",active._director.peek().get("type", ""))
		var round_number := int(banner.text.split(" ")[1])
		var phase := 0
		for pair in [["Effects",1],["Income",2],["Offense",3],["Defense",4],["Damage",5]]:
			if str(pair[0]) in banner.text: phase = int(pair[1])
		var order := round_number * 10 + phase
		_expect(round_number >= 1, "round zero is never displayed")
		_expect(order >= highest_phase, "round/phase never jumps backward: " + str(seen))
		highest_phase = maxi(highest_phase, order)
func _next_action(actions: Array) -> Dictionary:
	for kind in ["planning_roll", "planning_reroll", "roll_dice", "planning_select_ability", "planning_select_targets", "planning_pass", "pass"]:
		for action in actions:
			if action.type == kind: return action
	return {}
func _expect(ok: bool, message: String) -> void:
	if not ok:
		if not failed: push_error("ROUND TRANSITION: " + message)
		failed = true

func _check_round_context() -> void:
	var director := BattlePresentationDirector.new()
	var events := [
		{"sequence": 1, "type": "segment_entered", "segment": "income", "round": 2},
		{"sequence": 2, "type": "cards_drawn", "actor_id": "blade", "count": 1},
		{"sequence": 3, "type": "energy_points_gained", "actor_id": "blade", "energy_points": 2, "round": 0},
		{"sequence": 4, "type": "segment_entered", "segment": "income", "round": 3},
		{"sequence": 5, "type": "cards_drawn", "actor_id": "blade", "count": 1},
	]
	var result := {"snapshot": {"round": 3, "segment": "offensive", "actors": {}}, "events": events}
	director.queue_result(result)
	_expect(int(director.peek().event.round) == 2, "earlier Income inherits its own round instead of final snapshot round")
	director.advance()
	_expect(int(director.peek().event.round) == 3, "later Income retains round three")
	_expect(not events[1].has("round") and events[2].round == 0, "presentation does not mutate authority events")
	var resumed := BattlePresentationDirector.new()
	resumed.queue_result(result, 1)
	_expect(int(resumed.peek().event.round) == 2, "already presented segment still supplies context to fresh resource events")
