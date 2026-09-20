extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _fixture(input_id: String, segment: String = "offensive", stage: String = "offensive_reaction", other: String = "") -> Dictionary:
	var actions := [{"battle_id": "auto-pass", "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": input_id, "checkpoint": {"window_id": "window-" + input_id, "stage": stage, "iteration": 1}}}]
	if not other.is_empty(): actions.append({"actor_id": "blade", "type": other, "payload": {"pending_input_id": input_id}})
	return {"accepted": true, "events": [], "legal_actions": actions, "pending_input": {"blade": {"id": input_id, "stage": stage, "segment": segment, "allowed_commands": ["pass", "commit_interaction", "planning_roll", "planning_select_ability"]}}, "snapshot": {"battle_id": "auto-pass", "status": "active", "round": 1, "segment": segment, "stage": stage, "actors": {"blade": {"definition_id": "venom", "current_health": 24, "max_health": 24, "hand": [], "card_instances": {}}, "goblin": {"definition_id": "blade_warden", "current_health": 20, "max_health": 20}}}}

func _screen(fixture: Dictionary, fake: FakeBattleAuthority):
	var screen = SCREEN.instantiate()
	screen.gateway = BattleGateway.new(fake)
	screen.initial_result = fixture
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("auto-pass-test.json"))
	root.add_child(screen)
	screen.set_process(false)
	return screen


func damage(id: String, remaining: int) -> Dictionary:
	var value := _fixture(id, "damage_resolution", "damage_reaction")
	value.snapshot.settled_damage = {"id": "batch-1", "sources": [{"id": "incoming", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 5, "final_amount": remaining, "reaction_prevention": 5 - remaining}], "removals": []}
	for i in 5:
		value.snapshot.settled_damage.removals.append({"id": "r%d" % i, "card_id": "lost%d" % i, "card_definition_id": "tip_it", "target_actor_id": "goblin", "accepted": i < remaining, "released": i >= remaining})
	return value

func _run() -> void:
	var fake := FakeBattleAuthority.new()
	var initial := damage("first", 5)
	var screen = _screen(initial, fake)
	await process_frame
	# First reveal retains the short review; priority returns within the same
	# damage batch never impose another review or manual acknowledgement.
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty(), "initial cards get review time")
	screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - ceili((preload("res://presentation/battle/combat_timing.gd").reveal() + preload("res://presentation/battle/combat_timing.gd").hold() + 0.5) * 1000)
	screen._auto_pass_if_only_action()
	screen._auto_pass_highlight_ms = Time.get_ticks_msec() - 300
	fake.enqueue(damage("second", 5))
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1, "initial sole pass submitted")
	for n in 2:
		var previous: Dictionary = screen._view.settled_damage.duplicate(true)
		var result := damage("after-card-%d" % n, 3 - 2*n)
		result.events = [{"type": "damage_prevented_or_modified", "sequence": n+10, "actor_id": "goblin", "data": {"card_definition_id": "emergency_ward", "card_instance_id": "ward%d" % n, "source_id": "incoming", "damage_before": 5-2*n, "damage_after": 3-2*n}}]
		screen._view.apply_result(result)
		screen._capture_damage_feedback(result, previous)
		screen._render()
		_expect(screen._damage_feedback.saved.size() == 2, "two publicly revealed cards saved")
		_expect("Emergency Ward" in str(screen._damage_feedback.text), "played card named")
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == n+1, "animation gets time before continuation")
		if n == 0:
			screen._model_thinking = true
			screen._render()
			_expect(screen.find_children("*", "HBoxContainer", true, false).size() > 0, "board remains rendered during inference")
			screen._model_thinking = false
			var capture := OS.get_environment("DICE_AND_DESTINY_DAMAGE_RESPONSE_SCREENSHOT")
			if not capture.is_empty() and DisplayServer.get_name() != "headless":
				await process_frame
				await RenderingServer.frame_post_draw
				root.get_texture().get_image().save_png(capture)
		screen._reaction_card_feedback.expires_ms = 0
		fake.enqueue(damage("next-%d" % n, 3-2*n))
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == n+2, "no repeated delay for same damage batch")
	# A real card choice and the explicit debugging toggle still stop automation.
	screen._view.legal_actions.append({"type":"commit_interaction"})
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 3, "real choice retained")
	screen._view.legal_actions.pop_back()
	screen._set_auto_pass_disabled(true)
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 3, "debug toggle retained")
	screen.queue_free(); await process_frame
	print("DAMAGE RESPONSE FLOW: "+("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
