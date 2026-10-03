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
		value.snapshot.settled_damage.removals.append({"id": "r%d" % i, "card_id": "lost%d" % i, "card_definition_id": "tip_it", "target_actor_id": "goblin", "original_zone": ["deck", "hand", "hand", "deck", "discard"][i], "released_destination": ["deck", "hand", "hand", "deck", "discard"][i], "accepted": i < remaining, "released": i >= remaining})
	return value

func _run() -> void:
	var fake := FakeBattleAuthority.new()
	var initial := damage("first", 5)
	var screen = _screen(initial, fake)
	for frame in 4: await process_frame
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
		for frame in 4: await process_frame
		var previous: Dictionary = screen._view.settled_damage.duplicate(true)
		var result := damage("after-card-%d" % n, 3 - 2*n)
		result.events = [{"type": "damage_prevented_or_modified", "sequence": n+10, "actor_id": "goblin", "data": {"card_definition_id": "emergency_ward", "card_instance_id": "ward%d" % n, "source_id": "incoming", "damage_before": 5-2*n, "damage_after": 3-2*n}}]
		screen._view.apply_result(result)
		screen._capture_damage_feedback(result, previous)
		screen._render()
		for frame in 4: await process_frame
		_check_saved_animation(screen)
		var started: int = screen._damage_feedback.started_ms
		screen._capture_damage_feedback(result, previous)
		_expect(screen._damage_feedback.started_ms == started, "repeated event does not restart saving animation")
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
				var feedback = _feedback(screen)
				feedback.set_process(false)
				feedback.present_progress(0.65)
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
	await _check_native_ward()
	print("DAMAGE RESPONSE FLOW: "+("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)

func _feedback(screen):
	for node in screen._root.get_children():
		if node.get_script() == preload("res://presentation/battle/damage_response_feedback.gd"): return node
	return null

func _check_saved_animation(screen) -> void:
	var feedback = _feedback(screen)
	_expect(feedback != null, "saved-card animation exists")
	if feedback == null: return
	feedback.set_process(false)
	var original: Dictionary = screen._view.actors.duplicate(true)
	feedback.present_progress(0.6)
	for entry in feedback._saved:
		_expect(entry.origin.size.x > 0 and entry.origin.size.y > 0, "uses the actual revealed card rectangle")
		_expect(entry.card.position.is_equal_approx(entry.origin.position), "saved card starts where it was marked for removal")
		_expect(entry.card.get_node("RemovalState").text == "✓ SAVED", "saved badge replaces pending removal")
	for entry in feedback._pending:
		_expect(entry.card.position.is_equal_approx(entry.origin.position), "unsaved cards hold their slots while saved cards leave")
	feedback.present_progress(1.45)
	for entry in feedback._saved:
		_expect(entry.progress > 0 and entry.progress < 1, "saved cards visibly travel before disappearing")
	feedback.present_progress(2.05)
	for entry in feedback._saved:
		_expect(is_zero_approx(entry.card.modulate.a), "saved cards fade only on arrival")
	for pile in feedback._piles.values():
		var profile = screen._actor_profiles[pile.target]
		var actual: Rect2 = feedback.get_global_transform_with_canvas().affine_inverse() * profile.anchor_rect(str(pile.zone))
		var profile_rect: Rect2 = feedback.get_global_transform_with_canvas().affine_inverse() * profile.get_global_rect()
		_expect(pile.label.position.y >= profile_rect.end.y, "saved count avoids health and status text")
		_expect(pile.zone in ["deck", "hand", "discard"], "saved cards use their published pile")
		_expect(pile.rect.is_equal_approx(actual), "saved cards arrive at their destination pile anchor")
		_expect(pile.label.modulate.a > 0.9 and "saved" in pile.label.text, "destination names saved count")
	_expect(screen._view.actors == original, "animation never invents pile count changes")
	feedback.present_progress(2.6)
	_expect(is_zero_approx(feedback.modulate.a), "animation finishes cleanly")
	for grid in screen._damage_grids: _expect(is_equal_approx(grid.modulate.a, 1.0), "live damage grid restored after flight")
	feedback.set_process(true)

func _check_native_ward() -> void:
	var gateway = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var before := {}; var after := {}
	for seed_value in [153, 48, 1, 2, 3]:
		var result: Dictionary = gateway.start_battle("ward-%d" % seed_value, seed_value)
		for step in 100:
			var action := {}
			for candidate in result.get("legal_actions", []):
				var payload: Dictionary = candidate.get("payload", {})
				var ids: Array = payload.get("card_ids", payload.get("commitment", {}).get("card_ids", []))
				if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == "spiteful_ward": action = candidate; break
			if not action.is_empty():
				before = result.duplicate(true); after = gateway.submit(JSON.stringify(action)); break
			if int(result.get("snapshot", {}).get("round", 1)) > 4: break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			for candidate in result.get("legal_actions", []):
				if candidate.get("type") in ["planning_pass", "pass"]: action = candidate; break
			if action.is_empty(): break
			result = gateway.submit(JSON.stringify(action))
		if not after.is_empty(): break
	_expect(after.get("accepted", false), "native Spiteful Ward was legally played")
	if after.is_empty(): return
	for removal in after.snapshot.settled_damage.get("removals", []):
		if removal.get("released", false):
			var expected_zone: String = str(removal.get("original_zone"))
			if removal.get("card_definition_id") == "spiteful_ward" and expected_zone == "hand": expected_zone = "discard"
			_expect(removal.get("released_destination") == expected_zone, "native authority publishes saved-card live destination")
	_expect(after.snapshot.actors.blade.current_health == before.snapshot.actors.blade.current_health, "native prevention preserves health")
	_expect(after.snapshot.actors.blade.deck_count == before.snapshot.actors.blade.deck_count, "native saved deck cards remain in draw pile")
	_expect(after.snapshot.actors.blade.discard_count == before.snapshot.actors.blade.discard_count + 1, "native discard gains only the played ward")
	for width in [1920, 1280]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		var initial: Dictionary = before.duplicate(true)
		initial.events = []; initial.learned_policy = {}; initial.legal_actions = []
		var screen = _screen(initial, FakeBattleAuthority.new())
		for frame in 5: await process_frame
		var previous: Dictionary = screen._view.settled_damage.duplicate(true)
		screen._view.apply_result(after)
		screen._capture_damage_feedback(after, previous)
		screen._render()
		for frame in 5: await process_frame
		_expect(screen._damage_feedback.get("card_id") == "spiteful_ward", "native prevention event selects the actual card")
		_expect(screen._damage_feedback.get("saved", []).size() > 0, "native prevention saves real revealed cards")
		_check_saved_animation(screen)
		var feedback = _feedback(screen)
		feedback.set_process(false)
		var capture := OS.get_environment("DICE_AND_DESTINY_DAMAGE_RESPONSE_SCREENSHOT")
		if not capture.is_empty() and DisplayServer.get_name() != "headless":
			for sample in [0.65, 1.45, 2.05]:
				feedback.present_progress(sample)
				await RenderingServer.frame_post_draw
				root.get_texture().get_image().save_png(capture.get_basename() + "-ward-%d-%.2f.png" % [width, sample])
		screen.queue_free(); await process_frame
