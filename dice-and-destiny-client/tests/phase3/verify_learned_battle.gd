extends SceneTree

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")

var _failed := false

class RecoveringLearnedGateway:
	extends RefCounted
	var _actual: RefCounted
	var _failures_remaining := 1

	func _init(actual: RefCounted) -> void:
		_actual = actual

	func advance_model() -> Dictionary:
		if _failures_remaining > 0:
			_failures_remaining -= 1
			return {"accepted": false, "error": "synthetic learned inference failure; inspect phase3 diagnostics"}
		return _actual.advance_model()

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var runtime = root.get_node_or_null("LearnedBattleRuntime")
	if runtime == null:
		_fail("LearnedBattleRuntime autoload is unavailable")
		return
	var initialized: Dictionary = runtime.ensure_initialized()
	if initialized.get("ok") != true:
		_fail("accepted policy failed to initialize: %s" % JSON.stringify(initialized))
		return

	var menu = BOOTSTRAP.instantiate()
	root.add_child(menu)
	await process_frame
	var menu_text := _button_text(menu)
	_expect("Classic Battle" in menu_text, "classic mode is missing from the visible menu")
	_expect("Human Seat A" in menu_text and "Human Seat B" in menu_text, "both learned human seat assignments are missing from the visible menu")
	var classic_button := _find_button_containing(menu, "Classic Battle")
	_expect(classic_button != null and classic_button.visible and not classic_button.disabled, "classic D100 mode is not graphically actionable")
	if classic_button != null:
		classic_button.pressed.emit()
	await process_frame
	var classic_screen = _current_battle_screen()
	_expect(classic_screen != null, "classic menu click did not open the battle screen")
	if classic_screen != null:
		_expect(not classic_screen.learned_battle_mode, "classic menu click opened learned mode")
		_expect(str(classic_screen._view.actor("goblin").get("definition_id", "")) == "venom_goblin", "classic menu click did not preserve the Venom Goblin D100 path")
		classic_screen.queue_free()
	await process_frame

	var learned_menu = BOOTSTRAP.instantiate()
	root.add_child(learned_menu)
	await process_frame
	var learned_button := _find_button_containing(learned_menu, "Learned Mirror · Human Seat A")
	_expect(learned_button != null and learned_button.visible and not learned_button.disabled, "learned Seat A mode is not graphically actionable")
	if learned_button != null:
		learned_button.pressed.emit()
	await process_frame
	await process_frame
	var route_screen = _current_battle_screen()
	_expect(route_screen != null and route_screen.learned_battle_mode and route_screen.learned_human_seat == "seat-a", "learned Seat A menu click did not open the configured learned battle screen")
	if route_screen == null:
		quit(1)
		return
	var gateway: RefCounted = route_screen.gateway
	var view: Dictionary = route_screen.initial_result
	_expect(view.get("accepted") == true, "seat-a learned battle did not start")
	_expect(_safe_human_view(view), "seat-a human view leaked learned-seat private state")
	_expect(str(view.get("snapshot", {}).get("actors", {}).get("goblin", {}).get("definition_id", "")) == "blade_warden", "learned opponent is not a Blade Warden")
	_expect(str(view.get("learned_policy", {}).get("model_id", "")) == "blade-warden-maskable-ppo-seed-11-final-v1", "wrong frozen model metadata")

	# Prove a visible player control, rather than a raw gateway submission, owns
	# the normal UI-to-authority route.
	route_screen._director.clear()
	route_screen._render()
	await process_frame
	var roll_button := _find_button(route_screen, "Roll 5 Dice")
	_expect(roll_button != null and roll_button.visible and not roll_button.disabled, "visible planning-roll UI control is missing or disabled")
	var before_route: Dictionary = gateway.telemetry()
	var before_human := int(before_route.get("result", {}).get("battle", {}).get("human_decisions", 0))
	if roll_button != null:
		roll_button.pressed.emit()
		await process_frame
	var after_route: Dictionary = gateway.telemetry()
	_expect(int(after_route.get("result", {}).get("battle", {}).get("human_decisions", 0)) == before_human + 1, "visible planning-roll click did not reach the Go authority")
	_expect(route_screen._view.rolls_used("blade") == 1, "visible planning-roll click did not update the human viewer state")
	route_screen.queue_free()
	await process_frame

	# Replay the owner's reported seed to its first offensive reaction. One
	# visible player pass gives the opponent priority. When Tip It changes the
	# revealed attack, the player correctly receives a second visible response
	# opportunity before the battle advances to defense.
	var owner_view: Dictionary = gateway.start_battle("learned-owner-reaction-regression", 1785634159965507, true)
	_expect(owner_view.get("accepted") == true, "owner defense regression battle did not reset")
	var sharpen_blade_id := _card_instance(owner_view, "sharpen_blade")
	_expect(not sharpen_blade_id.is_empty(), "owner reaction fixture did not draw Sharpen Blade")
	var owner_commands := [
		BattleCommandBuilder.planning_roll(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {})),
	]
	owner_view = gateway.submit(owner_commands[0])
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_keep(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), [3, 4]))
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_reroll(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), [0, 1, 2]))
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_keep(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), [0, 3, 4]))
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_reroll(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), [1, 2]))
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_commit_cards(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), [sharpen_blade_id], [], "sword_cut"))
	if owner_view.get("accepted") == true: owner_view = gateway.submit(BattleCommandBuilder.planning_select_ability(str(owner_view.get("snapshot", {}).get("battle_id", "")), "blade", owner_view.get("pending_input", {}).get("blade", {}), "sword_cut", ["goblin"]))
	_expect(owner_view.get("accepted") == true, "owner keep/reroll/Sword Cut trace could not be replayed: %s" % JSON.stringify(owner_view))
	var reaction_guard := 0
	while reaction_guard < 1200 and not (
		owner_view.get("learned_policy", {}).get("model_turn") != true
		and str(owner_view.get("snapshot", {}).get("stage", "")) == "offensive_reaction"
	):
		reaction_guard += 1
		if owner_view.get("battle_result", "") in ["victory", "defeat", "draw"]:
			break
		if owner_view.get("learned_policy", {}).get("model_turn") == true:
			owner_view = gateway.advance_model()
		else: break
		if owner_view.get("accepted") != true:
			break
	_expect(
		reaction_guard < 1200
		and owner_view.get("accepted") == true
		and owner_view.get("learned_policy", {}).get("model_turn") != true
		and str(owner_view.get("snapshot", {}).get("stage", "")) == "offensive_reaction",
		"owner seed did not reach the human offensive-reaction regression state: %s" % JSON.stringify(owner_view)
	)
	if str(owner_view.get("snapshot", {}).get("stage", "")) == "offensive_reaction":
		var owner_screen = BATTLE_SCREEN.instantiate()
		owner_screen.initial_result = owner_view
		owner_screen.gateway = gateway
		owner_screen.learned_battle_mode = true
		owner_screen.learned_human_seat = "seat-a"
		owner_screen.learned_seed = 1785634159965507
		root.add_child(owner_screen)
		await process_frame
		owner_screen._director.clear()
		owner_screen._render()
		await process_frame
		var owner_pass := _find_button(owner_screen, "Pass / Acknowledge")
		_expect(owner_pass != null and not owner_pass.disabled, "owner offensive reaction did not expose its first visible pass")
		if owner_pass != null: owner_pass.pressed.emit()
		for attempt in range(300):
			var owner_state: Dictionary = owner_screen.inspection_state()
			if owner_state.get("model_thinking") != true and str(owner_state.get("stage", "")) == "offensive_reaction" and int(owner_state.get("pending_input", {}).get("reaction_round", 0)) == 2: break
			await create_timer(0.01).timeout
		var reacted_state: Dictionary = owner_screen.inspection_state()
		var reaction_notice := str(reacted_state.get("reaction_notice", ""))
		var enemy_reaction_dice: Array = reacted_state.get("rolled_dice", {}).get("goblin", [])
		_expect(str(reacted_state.get("stage", "")) == "offensive_reaction" and int(reacted_state.get("pending_input", {}).get("reaction_round", 0)) == 2, "Tip It did not return a second visible response opportunity: %s" % reacted_state)
		_expect("Tip It" in reaction_notice and "Venom Strike → Shield Bash" in reaction_notice and "die 1 changed to face 5" in reaction_notice, "opponent reaction was not explained: %s" % reaction_notice)
		_expect(enemy_reaction_dice.size() == 5 and int(enemy_reaction_dice[0].get("face", 0)) == 5, "opponent tray did not show the post-Tip-It die: %s" % [enemy_reaction_dice])
		var response_pass := _find_button(owner_screen, "Pass / Acknowledge")
		_expect(response_pass != null and not response_pass.disabled, "post-Tip-It response opportunity was not graphically actionable")
		if response_pass != null: response_pass.pressed.emit()
		for attempt in range(300):
			var owner_state: Dictionary = owner_screen.inspection_state()
			if owner_state.get("model_thinking") != true and str(owner_state.get("stage", "")) == "defense_selection": break
			await create_timer(0.01).timeout
		_expect(str(owner_screen.inspection_state().get("stage", "")) == "defense_selection", "second visible reaction pass did not advance to defense")
		var owner_source := _find_inspection_button_prefix(owner_screen, "battle.source.")
		_expect(owner_source != null and not owner_source.disabled, "defense selection did not expose the incoming source")
		if owner_source != null: owner_source.pressed.emit()
		await process_frame
		var basic_defense := _find_inspection_button(owner_screen, "battle.ability.blade.basic_defense")
		_expect(basic_defense != null and not basic_defense.disabled, "Basic Defense was not graphically selectable after the reaction")
		if basic_defense != null: basic_defense.pressed.emit()
		for attempt in range(300):
			var owner_state: Dictionary = owner_screen.inspection_state()
			if owner_state.get("model_thinking") != true and str(owner_state.get("stage", "")) == "defense_roll": break
			await create_timer(0.01).timeout
		var defense_state: Dictionary = owner_screen.inspection_state()
		var owner_pending: Dictionary = defense_state.get("pending_input", {})
		_expect(str(defense_state.get("stage", "")) == "defense_roll", "graphical Basic Defense did not reach defense roll: %s" % defense_state)
		_expect(not owner_pending.has("source_id"), "defense-roll pending input unexpectedly supplied a request/source id")
		var owner_die := _find_inspection_button(owner_screen, "battle.defense_die.blade.pending")
		var owner_source_text := _button_text(owner_screen)
		_expect(owner_die != null and not owner_die.disabled, "owner defense-roll blank die was not graphically actionable")
		_expect("Pending" in owner_source_text and "Final 0" not in owner_source_text, "owner defense-roll source still showed unresolved damage as Final 0")
		var before_defense: Dictionary = gateway.telemetry()
		var before_defense_human := int(before_defense.get("result", {}).get("battle", {}).get("human_decisions", 0))
		if owner_die != null:
			owner_die.pressed.emit()
			await process_frame
		var after_defense: Dictionary = gateway.telemetry()
		var after_defense_battle: Dictionary = after_defense.get("result", {}).get("battle", {})
		_expect(int(after_defense_battle.get("human_decisions", 0)) == before_defense_human + 1, "visible owner defense die click did not reach the Go authority")
		_expect(int(after_defense_battle.get("stale_actions", 0)) == 0, "visible owner defense die click was still rejected as stale")
		_expect(str(owner_screen.inspection_state().get("error", "")).is_empty(), "visible owner defense die click left the battle in an error state")
		owner_die = null
		owner_screen.queue_free()
		await process_frame

	# Reset after the focused click so the deterministic full integration battle
	# still starts from its documented seed and initial decision.
	view = gateway.start_battle("godot-phase3-seat-a-full", 20260803, true)
	_expect(view.get("accepted") == true, "seat-a full learned battle did not reset")

	var first_command := _preferred_command(view)
	_expect(not first_command.is_empty(), "seat-a human received no graphical command candidate")
	if not first_command.is_empty():
		view = gateway.submit(JSON.stringify(first_command))
		_expect(view.get("accepted") == true, "UI alias command was not routed through authority validation")
		var stale: Dictionary = gateway.submit(JSON.stringify(first_command))
		_expect(stale.get("accepted") != true, "stale UI command was accepted")

	var action_guard := 0
	while view.get("battle_result", "") not in ["victory", "defeat", "draw"] and action_guard < 1200:
		action_guard += 1
		if view.get("learned_policy", {}).get("model_turn") == true:
			view = gateway.advance_model()
		else:
			var command := _preferred_command(view)
			if command.is_empty():
				_fail("human proxy found no graphical action route at %s" % JSON.stringify(view.get("snapshot", {})))
				break
			view = gateway.submit(JSON.stringify(command))
		if view.get("accepted") != true:
			_fail("integrated battle command failed: %s" % JSON.stringify(view))
			break
		_expect(_safe_human_view(view), "integrated human view leaked private model state")
	_expect(action_guard < 1200 and view.get("battle_result", "") in ["victory", "defeat", "draw"], "integrated learned battle did not reach an authority result")

	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = view
	screen.gateway = gateway
	screen.learned_battle_mode = true
	screen.learned_human_seat = "seat-a"
	screen.learned_seed = 20260803
	root.add_child(screen)
	await process_frame
	screen._director.clear()
	screen._render()
	await process_frame
	var screen_text := _button_text(screen) + " " + _label_text(screen)
	_expect("Learned Policy" in screen_text, "battle presentation does not identify the learned opponent")
	_expect("Rematch · Same Seats" in screen_text, "graphical learned rematch control is missing")
	_expect("New Battle · Change Seat or Mode" in screen_text, "graphical new-battle control is missing")
	var rematch_button := _find_button(screen, "Rematch · Same Seats")
	_expect(rematch_button != null and rematch_button.visible and not rematch_button.disabled, "graphical rematch control is not actionable")
	if rematch_button != null:
		rematch_button.pressed.emit()
		await process_frame
	var rematch_telemetry: Dictionary = gateway.telemetry()
	var rematch_lifetime: Dictionary = rematch_telemetry.get("result", {}).get("lifetime", {})
	_expect(int(rematch_lifetime.get("battles_started", 0)) == 4 and int(rematch_lifetime.get("rematches", 0)) == 3, "graphical rematch did not reset the persistent session")
	_expect(not screen._view.is_complete(), "graphical rematch left the result screen on the completed battle")
	screen.queue_free()
	await process_frame

	var seat_b_gateway: RefCounted = LEARNED_GATEWAY.new(runtime, "seat-b")
	var seat_b: Dictionary = seat_b_gateway.start_battle("godot-phase3-seat-b", 20260804, true)
	_expect(seat_b.get("accepted") == true, "seat-b learned rematch did not start")
	_expect(seat_b.get("learned_policy", {}).get("model_turn") == true, "seat-b assignment did not begin with a visible opponent turn")
	var error_screen = BATTLE_SCREEN.instantiate()
	error_screen.initial_result = seat_b
	error_screen.gateway = RecoveringLearnedGateway.new(seat_b_gateway)
	error_screen.learned_battle_mode = true
	error_screen.learned_human_seat = "seat-b"
	error_screen.learned_seed = 20260804
	root.add_child(error_screen)
	for attempt in range(100):
		if error_screen.inspection_state().get("model_error") == true: break
		await create_timer(0.01).timeout
	var error_state: Dictionary = error_screen.inspection_state()
	_expect(error_state.get("model_error") == true and "synthetic learned inference failure" in str(error_state.get("error", "")), "asynchronous learned error did not produce actionable diagnostics")
	var retry_button := _find_button(error_screen, "Retry Learned Decision")
	_expect(retry_button != null and _find_button(error_screen, "Return to Mode Menu") != null, "learned error did not expose graphical retry and exit controls")
	if retry_button != null:
		retry_button.pressed.emit()
	await process_frame
	_expect(error_screen.inspection_state().get("model_thinking") == true, "retry did not restore the visible opponent-thinking state")
	for attempt in range(100):
		var state: Dictionary = error_screen.inspection_state()
		if state.get("model_thinking") != true and state.get("model_error") != true: break
		await create_timer(0.01).timeout
	var recovered_state: Dictionary = error_screen.inspection_state()
	_expect(recovered_state.get("ready") == true and recovered_state.get("model_error") == false, "Godot did not recover after retrying the unchanged learned decision")
	error_screen.queue_free()
	await process_frame

	var telemetry: Dictionary = seat_b_gateway.telemetry()
	var lifetime: Dictionary = telemetry.get("result", {}).get("lifetime", {})
	_expect(int(lifetime.get("model_load_count", 0)) == 1, "rematch reloaded the learned model")
	_expect(int(lifetime.get("battles_started", 0)) == 5 and int(lifetime.get("rematches", 0)) == 4, "rematch lifetime counters are incorrect")
	var battle: Dictionary = telemetry.get("result", {}).get("battle", {})
	_expect(int(battle.get("model_decisions", 0)) > 0, "accepted checkpoint did not complete the asynchronous opponent decision")
	_expect(int(battle.get("fallback_count", -1)) == 0 and int(battle.get("timeouts", -1)) == 0, "model runtime used a fallback or timed out")

	if not _failed:
		print("PHASE 3 GODOT: mode menu, UI routing, learned runtime, result controls, both seats, and persistent rematch passed")
	quit(1 if _failed else 0)

func _preferred_command(view: Dictionary) -> Dictionary:
	var actions: Array = view.get("legal_actions", [])
	for kind in ["planning_roll", "planning_select_ability", "planning_reroll", "roll_dice", "planning_pass", "pass", "commit_interaction"]:
		for index in range(actions.size() - 1, -1, -1):
			if str(actions[index].get("type", "")) == kind:
				return actions[index]
	return actions[0] if not actions.is_empty() else {}

func _card_instance(view: Dictionary, definition_id: String) -> String:
	var actor: Dictionary = view.get("snapshot", {}).get("actors", {}).get("blade", {})
	var instances: Dictionary = actor.get("card_instances", {})
	for instance_id in actor.get("hand", []):
		if str(instances.get(instance_id, {}).get("definition_id", "")) == definition_id:
			return str(instance_id)
	return ""

func _safe_human_view(view: Dictionary) -> bool:
	var snapshot: Dictionary = view.get("snapshot", {})
	var opponent: Dictionary = view.get("snapshot", {}).get("actors", {}).get("goblin", {})
	if not opponent.get("hand", []).is_empty() or not opponent.get("card_instances", {}).is_empty():
		return false
	# Opponent dice are intentionally public after reveal. They must remain
	# hidden only during simultaneous planning and other blind windows.
	if str(snapshot.get("stage", "")) == "planning" and not opponent.get("roll_history", []).is_empty():
		return false
	return true

func _button_text(node: Node) -> String:
	var values: Array[String] = []
	for child in node.find_children("*", "Button", true, false):
		values.append(str(child.text))
	return "\n".join(values)

func _label_text(node: Node) -> String:
	var values: Array[String] = []
	for child in node.find_children("*", "Label", true, false):
		values.append(str(child.text))
	return "\n".join(values)

func _find_button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if str(child.text) == text:
			return child
	return null

func _find_button_containing(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if text in str(child.text):
			return child
	return null

func _find_inspection_button(node: Node, inspection_id: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if str(child.get_meta("inspection_id", "")) == inspection_id:
			return child
	return null

func _find_inspection_button_prefix(node: Node, inspection_prefix: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if str(child.get_meta("inspection_id", "")).begins_with(inspection_prefix):
			return child
	return null

func _current_battle_screen():
	var screens := get_nodes_in_group("inspectable_battle_screen")
	return screens[-1] if not screens.is_empty() else null

func _expect(condition: bool, message: String) -> void:
	if not condition:
		_fail(message)

func _fail(message: String) -> void:
	_failed = true
	push_error("PHASE 3 GODOT: %s" % message)
