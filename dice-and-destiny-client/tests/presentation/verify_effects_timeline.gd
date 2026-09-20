extends SceneTree

# Recorded round 5 -> 6 regression: an empty Defense beat must not show the
# lethal Poison result before the following Effects animation actually plays.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = gateway.start_battle("effects-timeline", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	var event: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/lethal_effects_after_empty_defense.json"))
	fixture.events = [
		{"sequence": 1, "type": "segment_entered", "segment": "defensive", "round": 5},
		{"sequence": 2, "type": "segment_entered", "segment": "damage_resolution", "round": 5},
		{"sequence": 3, "type": "damage_committed", "segment": "damage_resolution", "round": 5},
		{"sequence": 4, "type": "segment_entered", "segment": "ongoing_effects", "round": 6},
		event,
		{"sequence": 6, "type": "battle_completed", "segment": "ongoing_effects", "round": 6, "battle_result": "victory"},
	]
	fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.segment = "ongoing_effects"; fixture.snapshot.stage = "complete"
	fixture.snapshot.round = 6; fixture.snapshot.status = "victory"; fixture.battle_result = "victory"
	for actor_id in event.data.actors_after:
		var actor: Dictionary = fixture.snapshot.actors[actor_id]
		var after: Dictionary = event.data.actors_after[actor_id]
		actor.current_health = after.health; actor.energy_points = after.energy
		for key in ["deck_count", "hand_count", "discard_count", "removed_count", "statuses"]: actor[key] = after[key]
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("effects-timeline.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	# Check synchronously as well as after frames: a one-frame health flash is a bug.
	_expect(screen._director.peek().get("presentation_segment") == "defensive", "empty Defense remains the first beat")
	_before(screen)
	_expect(_has_text(screen, "Round 5 · Presentation"), "Defense header retains round 5")
	for frame in 3: await process_frame
	_before(screen)
	await _capture("defense")
	screen._render()
	_before(screen)
	screen._advance_beat()
	_expect(screen._director.peek().get("type") == "effects_resolved", "next beat is Effects")
	_expect(_has_text(screen, "Round 6 · Effects"), "Effects header advances to round 6")
	_before(screen)
	await process_frame
	var panel = screen._effects_panel
	panel.resume_at(-preload("res://presentation/battle/combat_timing.gd").effects_gather() * 0.5); panel.present_progress()
	await process_frame
	var gathered := 0
	for node in screen._root.get_children():
		if node.get_script() == preload("res://presentation/battle/effects_gather.gd"):
			node._process(0)
			for badge in node.badges.values():
				_expect(badge.modulate.a > 0.0, "status badge travels into Effects")
				gathered += 1
	_expect(gathered == 3, "Bleed, Poison and Volatile Poison gather on their recipient sides")
	await _capture("effects-gather")
	panel.resume_at(1.3); panel._process(0)
	_before(screen)
	await _capture("effects-before")
	var last_health := 2.0
	for elapsed in [4.5, 4.7, 4.9, 5.1, 5.3, 6.2]:
		panel.resume_at(elapsed + panel.catalyst_extra_seconds()); panel._process(0)
		var current: float = screen._actor_profiles.goblin.health.value
		_expect(current <= last_health, "health never rises during damage playback")
		last_health = current
	_expect(last_health == 0, "Poison animation reaches zero")
	_after(screen)
	await _capture("effects-after")
	# A redraw during developer inspection must preserve the settled animation.
	screen._snapshot_panel_open = true
	panel.set_paused(true)
	screen._render()
	_after(screen)
	await process_frame; await process_frame
	_after(screen)
	for entry in screen._effects_panel.entries:
		_expect(entry.die.text != "…", "paused redraw retains landed dice")
	screen._snapshot_panel_open = false
	screen._effects_panel.set_paused(false)
	screen._effects_panel.resume_at(screen._effects_panel.duration + 0.1)
	screen._effects_panel._process(0)
	_after(screen)
	await process_frame; await process_frame
	_after(screen)
	_expect(screen._director.pending_effects_actor_before("goblin").is_empty(), "completed Effects no longer overrides profiles")
	_expect(fake.commands.is_empty(), "presentation does not change gameplay")
	_expect(screen._view.actor("goblin").current_health == 0, "authority snapshot stays final throughout")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("EFFECTS TIMELINE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _before(screen: Node) -> void:
	var enemy = screen._actor_profiles.goblin
	var player = screen._actor_profiles.blade
	_expect(enemy.health.value == 2 and player.health.value == 5, "pre-Effects health held at 2 / 5")
	_expect(enemy._display_values.hand == 2 and enemy._display_values.removed == 18, "pre-Effects card counts held")
	_expect("Poison ×3" in enemy.statuses.text and "Bleed ×2" in player.statuses.text and "Catalyst ×2" in player.statuses.text, "pre-Effects status counts held")

func _after(screen: Node) -> void:
	var enemy = screen._actor_profiles.goblin
	var player = screen._actor_profiles.blade
	_expect(enemy.health.value == 0 and player.health.value == 3, "post-Effects health stays at 0 / 3")
	_expect(enemy._display_values.hand == 0 and enemy._display_values.removed == 20, "post-Effects removed cards stay visible in counts")
	_expect("Poison ×2" in enemy.statuses.text and "Bleed ×1" in player.statuses.text and "Catalyst ×1" in player.statuses.text, "post-Effects status counts stay settled")

func _has_text(node: Node, text: String) -> bool:
	for label in node.find_children("*", "Label", true, false):
		if label.text == text: return true
	return false

func _capture(suffix: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_TIMELINE_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(suffix + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("EFFECTS TIMELINE: " + message)
