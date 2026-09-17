extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _fixture(input_id: String, segment: String = "offensive", stage: String = "offensive_reaction", other: String = "") -> Dictionary:
	var actions := [{"battle_id": "auto-pass", "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": input_id, "checkpoint": {"window_id": "window-" + input_id, "stage": stage, "iteration": 1}}}]
	if not other.is_empty(): actions.append({"actor_id": "blade", "type": other, "payload": {"pending_input_id": input_id}})
	var result := {"accepted": true, "events": [], "legal_actions": actions, "pending_input": {"blade": {"id": input_id, "stage": stage, "segment": segment, "allowed_commands": ["pass", "commit_interaction", "planning_roll", "planning_select_ability"]}}, "snapshot": {"battle_id": "auto-pass", "status": "active", "round": 1, "segment": segment, "stage": stage, "actors": {"blade": {"definition_id": "venom", "current_health": 24, "max_health": 24, "hand": [], "card_instances": {}}, "goblin": {"definition_id": "blade_warden", "current_health": 20, "max_health": 20}}}}
	if stage == "defense_reaction":
		result.snapshot.damage_sources = [{"id": "incoming", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 3}]
		result.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "incoming", "rolled_face": 2, "rolled_faces": [2]}}
	return result

func _screen(fixture: Dictionary, fake: FakeBattleAuthority):
	var screen = SCREEN.instantiate()
	screen.gateway = BattleGateway.new(fake)
	screen.initial_result = fixture
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("auto-pass-test.json"))
	root.add_child(screen)
	screen.set_process(false)
	return screen

func _run() -> void:
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_roll_seconds", 0.2)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_effects_seconds", 1.8)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_hold_seconds", 1.0)
	for phase in [["offensive", "offensive_reaction"], ["defensive", "defense_reaction"], ["damage_resolution", "damage_reaction"], ["ongoing_effects", "status_roll_reaction"], ["offensive", "status_damage_reaction"], ["income", "income"], ["offensive", "venom_status_reaction"]]:
		var fake := FakeBattleAuthority.new()
		var fixture := _fixture(phase[1], phase[0], phase[1])
		fake.enqueue(_fixture("real-choice", "offensive", "planning", "planning_roll"))
		var screen = _screen(fixture, fake)
		screen.set_process(true)
		for frame in 5: await process_frame
		if phase[0] in ["damage_resolution", "defensive"] or phase[1] == "status_damage_reaction":
			_expect(fake.commands.is_empty(), "results remain visible for review")
			if phase[0] == "defensive":
				await create_timer(2.8).timeout
				_expect(fake.commands.is_empty(), "defense cannot auto-pass before three seconds")
				await create_timer(0.6).timeout
			else:
				await create_timer(3.0).timeout
		_expect(fake.commands.size() == 1, "sole pass progresses automatically during " + phase[1])
		if fake.commands.size() == 1:
			_expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[0]), "submits exact current authority command")
		await _close(screen)
	# Do not infer automatic passage from allowed categories or disabled visuals.
	for other in ["commit_interaction", "planning_roll", "planning_select_ability", "roll_dice"]:
		var fake := FakeBattleAuthority.new()
		var screen = _screen(_fixture("choice", "offensive", "planning", other), fake)
		screen.set_process(true)
		for frame in 3: await process_frame
		_expect(fake.commands.is_empty(), "stops for legal " + other)
		await _close(screen)
	for guard in ["_history_review", "_history_replay", "_submitting", "_model_thinking", "_model_error", "_snapshot_panel_open", "_auto_pass_disabled"]:
		var fake := FakeBattleAuthority.new()
		var screen = _screen(_fixture("guard", "defensive", "defense_reaction") if guard == "_auto_pass_disabled" else _fixture("guard"), fake)
		screen.set(guard, true)
		screen._auto_pass_if_only_action()
		_expect(fake.commands.is_empty(), "respects " + guard)
		await _close(screen)
	for invalid in ["missing", "foreign", "stale", "model_turn", "not_allowed"]:
		var fake := FakeBattleAuthority.new()
		var fixture := _fixture("guard")
		match invalid:
			"missing": fixture.legal_actions = []
			"foreign": fixture.legal_actions[0].actor_id = "goblin"
			"stale": fixture.legal_actions[0].payload.pending_input_id = "old"
			"model_turn": fixture.learned_policy = {"model_turn": true}
			"not_allowed": fixture.pending_input.blade.allowed_commands = []
		var screen = _screen(fixture, fake)
		screen._auto_pass_if_only_action()
		_expect(fake.commands.is_empty(), "ignores " + invalid)
		await _close(screen)
	# Skip planning only when there truly is no roll, card or ability available.
	for segment in ["offensive", "defensive"]:
		var fixture := _fixture("no-planning-moves", segment, "planning")
		fixture.legal_actions[0].type = "planning_pass"
		fixture.pending_input.blade.allowed_commands = ["planning_pass"]
		var planning_fake := FakeBattleAuthority.new()
		planning_fake.enqueue(_fixture("next-choice", "offensive", "planning", "planning_roll"))
		var planning_screen = _screen(fixture, planning_fake)
		planning_screen.set_process(true)
		for frame in 3: await process_frame
		_expect(planning_fake.commands.size() == 1, "automatically passes exhausted " + segment + " planning")
		await _close(planning_screen)
	# Existing presentations finish before sending the forced acknowledgement.
	var fake := FakeBattleAuthority.new()
	var screen = _screen(_fixture("after-presentation"), fake)
	screen._director.queue_result({"events": [{"type": "status_changed", "sequence": 1}]})
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty(), "waits for presentation")
	fake.enqueue(_fixture("after-presentation"))
	screen._director.clear()
	screen.set_process(true)
	for frame in 5: await process_frame
	_expect(fake.commands.size() == 1, "same input is never repeatedly submitted")
	await _close(screen)
	# A no-choice response handoff never waits for a manual debug acknowledgement.
	for phase in ["defense_reaction", "damage_reaction", "offensive_reaction"]:
		fake = FakeBattleAuthority.new()
		var handoff := _fixture("handoff", "defensive", phase)
		handoff.snapshot.pass_hands_off_priority = true
		fake.enqueue(_fixture("final-review", "defensive", "defense_reaction"))
		screen = _screen(handoff, fake)
		screen._auto_pass_disabled = true
		screen._auto_pass_if_only_action()
		if phase == "defense_reaction":
			_expect(fake.commands.is_empty(), "defense handoff first preserves visible results")
			screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - int(screen.DEFENSE_TIMING.total_seconds() * 1000)
			screen._auto_pass_if_only_action()
			screen._auto_pass_highlight_ms = Time.get_ticks_msec() - screen.DAMAGE_AUTO_PASS_CLICK_MS
			screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "no-choice response hands off despite debug pause")
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == 1, "debug pause still stops final acknowledgement")
		screen._view.apply_result(handoff)
		screen._view.legal_actions.append({"type": "commit_interaction"})
		_expect(screen._sole_pass_action().is_empty(), "handoff hint never skips a real card choice")
		await _close(screen)
	# Review is followed by a visible pressed state, then the exact pass command.
	fake = FakeBattleAuthority.new()
	fake.enqueue(_fixture("after-damage", "offensive", "planning", "planning_roll"))
	screen = _screen(_fixture("damage-preview", "damage_resolution", "damage_reaction"), fake)
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty() and not screen._auto_pass_button.button_pressed, "damage review starts without pressing pass")
	screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - screen.DAMAGE_AUTO_PASS_REVIEW_MS
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty() and screen._auto_pass_button.button_pressed, "pass visibly presses before advancing")
	_expect(screen._auto_pass_button.has_theme_stylebox_override("pressed"), "automatic press has a visible highlight")
	screen._render()
	_expect(screen._auto_pass_button.button_pressed, "highlight survives a UI refresh")
	var capture_path := OS.get_environment("DICE_AND_DESTINY_AUTO_PASS_SCREENSHOT")
	if not capture_path.is_empty() and DisplayServer.get_name() != "headless":
		await process_frame
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture_path)
	screen._auto_pass_highlight_ms = Time.get_ticks_msec() - screen.DAMAGE_AUTO_PASS_CLICK_MS
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1, "passes after visible highlight")
	await _close(screen)
	# A changed decision or available card cancels the pending automatic click.
	for change in ["card", "history", "new_input", "manual", "disable"]:
		fake = FakeBattleAuthority.new()
		fake.enqueue(_fixture("after-manual", "offensive", "planning", "planning_roll"))
		screen = _screen(_fixture("cancel-preview", "damage_resolution", "damage_reaction"), fake)
		screen._auto_pass_if_only_action()
		screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - screen.DAMAGE_AUTO_PASS_REVIEW_MS
		screen._auto_pass_if_only_action()
		screen._auto_pass_highlight_ms = Time.get_ticks_msec() - screen.DAMAGE_AUTO_PASS_CLICK_MS
		match change:
			"card": screen._view.legal_actions.append({"type": "commit_interaction"})
			"history": screen._history_review = true
			"new_input": screen._view.apply_result(_fixture("new-damage", "damage_resolution", "damage_reaction"))
			"manual": screen._auto_pass_button.pressed.emit()
			"disable": screen._auto_pass_toggle.button_pressed = true
		screen._auto_pass_if_only_action()
		_expect(fake.commands.size() == (1 if change == "manual" else 0), "no stale/double pass after " + change)
		_expect(not screen._auto_pass_button.button_pressed, "clears highlight after " + change)
		if change == "new_input": _expect(screen._auto_pass_preview_input.ends_with("new-damage"), "new damage gets a full review period")
		await _close(screen)
	# The debug checkbox survives redraws, permits manual pass, and resumes with
	# a full defense review rather than inheriting an expired countdown.
	fake = FakeBattleAuthority.new()
	fake.enqueue(_fixture("manual-next", "defensive", "defense_reaction"))
	screen = _screen(_fixture("toggle-defense", "defensive", "defense_reaction"), fake)
	screen._auto_pass_if_only_action()
	screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - 5000
	screen._auto_pass_toggle.button_pressed = true
	screen._render()
	_expect(screen._auto_pass_toggle.button_pressed, "debug preference survives redraw")
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty(), "checked toggle blocks expired countdown")
	screen._auto_pass_button.pressed.emit()
	_expect(fake.commands.size() == 1 and screen._auto_pass_toggle.button_pressed, "manual pass works and preference survives next decision")
	screen._auto_pass_toggle.button_pressed = false
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1 and screen._auto_pass_highlight_ms < 0, "unchecking starts a fresh defense review")
	_expect(screen._auto_pass_preview_input.ends_with("manual-next"), "new review uses current decision")
	await _close(screen)
	# Walk consecutive forced windows, stopping as soon as a real choice appears.
	fake = FakeBattleAuthority.new()
	fake.enqueue(_fixture("chain-2", "defensive", "defense_reaction"))
	fake.enqueue(_fixture("chain-3", "damage_resolution", "damage_reaction"))
	fake.enqueue(_fixture("chain-stop", "offensive", "planning", "commit_interaction"))
	screen = _screen(_fixture("chain-1"), fake)
	screen.set_process(true)
	for frame in 8: await process_frame
	await create_timer(6.3).timeout
	_expect(fake.commands.size() == 3, "consecutive passes stop at a real choice")
	await _close(screen)
	fake = FakeBattleAuthority.new()
	fake.enqueue({"accepted": false, "error": "test rejection"})
	screen = _screen(_fixture("rejected"), fake)
	screen.set_process(true)
	for frame in 5: await process_frame
	_expect(fake.commands.size() == 1 and not screen._error_message.is_empty(), "rejection stops without retry loop")
	await _close(screen)
	print("AUTOMATIC PASS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _close(screen) -> void:
	screen.active_store.clear()
	screen.queue_free()
	await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("AUTOMATIC PASS: " + message)
