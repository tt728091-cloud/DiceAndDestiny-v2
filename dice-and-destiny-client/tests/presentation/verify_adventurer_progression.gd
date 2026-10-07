extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for size in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
		canvas.size = size
		for protect in [false, true]: await _scenario(protect)
	print("ADVENTURER DAMAGE PROGRESSION: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _scenario(protect: bool) -> void:
	# Replay the reported battle: Guarded Strike, Guard, Brace, then enemy pass.
	# The old full-battle tests submitted commands directly and missed GUI occlusion.
	var fixture: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/adventurer_damage_progression.json"))
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result: Dictionary = gateway.start_battle(fixture.battle_id, int(fixture.seed))
	if not result.get("accepted", false): _expect(false, "native battle starts: " + str(result.get("error"))); return
	for step in fixture.trace:
		if step.controller == "human":
			var recorded: Dictionary = JSON.parse_string(JSON.stringify(step.command).replace('"seat-a"', '"blade"').replace('"seat-b"', '"goblin"'))
			var choices: Array = result.legal_actions.filter(func(action): return _semantic(action) == _semantic(recorded))
			if choices.size() != 1: _expect(false, "replay has one current legal match for " + str(recorded)); return
			result = gateway.submit(JSON.stringify(choices[0]))
		else: result = gateway.advance_model()
		if result.get("snapshot", {}).get("segment") == "defensive":
			_expect(_protect_count(result.snapshot.actors.blade) == 2, "Protect is public before defense")
		if not result.get("accepted", false): _expect(false, "replay command accepted: " + str(result.get("error"))); return
	_expect(result.snapshot.stage == "damage_reaction" and result.legal_actions.size() == 2, "reported state has Pass and unspent protection")
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result
	screen.learned_battle_mode = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("adventurer-progress.json"))
	canvas.add_child(screen)
	await create_timer(8).timeout
	_expect(screen._view.round_number == 1 and screen._sole_pass_action().is_empty(), "auto-pass preserves the real protection choice")
	var strip = screen._actor_profiles.blade.statuses
	_expect(_protect_count(screen._view.actor("blade")) == 2 and strip.counts.get("protect") == 2, "authority and visible status count agree")
	_expect(strip.cells.has("protect") and strip.cells.protect.modulate.a == 1.0, "Protect icon is visible on character board")
	_expect("Positive status" in strip.cells.protect.tooltip_text and "next round" in strip.cells.protect.tooltip_text, "status hover explains polarity and expiration")
	_expect(BattlePresentationCatalog.definition("statuses", "protect").polarity == "positive", "catalog declares positive status")
	var hover := InputEventMouseMotion.new(); hover.position = strip.cells.protect.get_global_rect().get_center()
	canvas.push_input(hover, true); await process_frame
	_expect(canvas.gui_get_hovered_control() == strip.cells.protect, "pointer reaches Protect status hover")
	_expect(screen._action_footer.get_children().all(func(button): return not button is Button or "Protect" not in button.text), "no standalone protection button")
	var source_id := str(screen._view.legal_actions.filter(func(action): return action.type == "commit_interaction")[0].payload.commitment.proposal_ids[0])
	await _click(screen._attack_intents[source_id].intent)
	for frame in 6: await process_frame
	var protection: Button = screen._ability_dock.get_node_or_null("ProtectChoice")
	_expect(protection != null and protection.text == "Protect · Prevent 2 damage", "attack popup offers the available status")
	if protection == null: screen.queue_free(); await process_frame; return
	for button in [protection, screen._auto_pass_button]:
		_expect(not button.disabled and Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(button.get_global_rect()), "action enabled and fits viewport")
	var capture_dir := OS.get_environment("DICE_AND_DESTINY_ADVENTURER_SCREENSHOTS")
	if not capture_dir.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw(false)
		canvas.get_texture().get_image().save_png(capture_dir.path_join("adventurer-actions-%d.png" % canvas.size.x))
	if protect:
		await _click(protection)
	else:
		await _click(screen._auto_pass_button)
	var deadline := Time.get_ticks_msec() + 15000
	while Time.get_ticks_msec() < deadline and (screen._view.round_number == 1 or screen._director.has_beats() or screen._held_defense_view != null or screen._card_gain_active()): await process_frame
	_expect(screen._view.round_number == 2 and screen._error_message.is_empty(), "click advances to next round without manual command injection")
	_expect(_protect_count(screen._view.actor("blade")) == 0 and int(screen._actor_profiles.blade.statuses.counts.get("protect", 0)) == 0, "spent or expired Protect is removed from board")
	_expect(int(screen._view.actor("blade").current_health) == (12 if protect else 11), "protection saves the remaining damage; Pass accepts it")
	print("PROGRESSION ", canvas.size, " ", "protect + auto-pass" if protect else "manual Pass", ": round ", screen._view.round_number)
	screen.queue_free(); await process_frame
func _click(button: Button) -> void:
	var viewport := button.get_viewport()
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center()
	viewport.push_input(motion, true); await process_frame
	_expect(viewport.gui_get_hovered_control() == button, "actual pointer hits " + button.text)
	var click := InputEventMouseButton.new(); click.position = motion.position; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true
	viewport.push_input(click, true)
	click = click.duplicate(); click.pressed = false; viewport.push_input(click, true)
	await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ADVENTURER PROGRESSION: " + message)

func _protect_count(actor: Dictionary) -> int:
	var count := 0
	for status in actor.get("statuses", []):
		if status.definition_id == "protect": count += int(status.stacks)
	return count
# Status acquisition adds authority sequence entries; match decisions by their
# meaning and submit the current legal command, preserving checkpoint safety.
func _semantic(value):
	if value is Dictionary:
		var result := {}
		for key in value:
			if key not in ["checkpoint", "pending_input_id"]: result[key] = _semantic(value[key])
		return result
	if value is Array: return value.map(_semantic)
	if value is String and value.begins_with("source-r"): return value.substr(0, value.rfind("-"))
	return value
