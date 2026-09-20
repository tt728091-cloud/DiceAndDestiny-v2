extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080) if OS.get_environment("DICE_AND_DESTINY_EFFECTS_LARGE") == "1" else Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = gateway.start_battle("automatic-effects-visual", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	var path := "res://tests/fixtures/automatic_effects.json"
	var raw := FileAccess.get_file_as_string(path).replace('"player"', '"blade"').replace('"enemy"', '"goblin"')
	var event: Dictionary = JSON.parse_string(raw)
	event.sequence = 1
	fixture.events = [event]; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("automatic-effects.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	for frame in 5: await process_frame
	var panel = screen._effects_panel
	_expect(is_instance_valid(panel), "single automatic Effects screen")
	if not is_instance_valid(panel): quit(1); return
	_expect(panel.entries.size() == 5, "four toxin dice and one Bleed group")
	var cards := 0; var catalyst := 0
	for entry in panel.entries:
		cards += entry.cards.size()
		_expect(entry.cards.size() == int(entry.damage), "lost cards allocated to the correct damage amount")
		if entry.catalyst:
			catalyst += 1
			_expect(entry.status_id == "volatile_poison" and entry.original_face == 6 and entry.face == 2, "Catalyst priority and forced reroll preserved")
	_expect(cards == 6 and catalyst == 1, "six real lost cards and one Catalyst reroll")
	_expect(panel._has_conversion, "Incubation conversion timing preserved")
	for button in screen.find_children("*", "Button", true, false):
		_expect(not str(button.get_meta("inspection_id", "")).begins_with("battle.command."), "no gameplay controls during Effects")
	for label in panel.find_children("*", "Label", true, false):
		_expect(not "After Poison roll responses" in label.text, "no ongoing rules wall")
	panel.resume_at(1.3); panel._process(0); await process_frame
	for entry in panel.entries:
		if entry.face > 0: _expect(entry.die.text == str(entry.original_face), "original roll visible before Catalyst")
	await _capture("original")
	panel.resume_at(1.8); panel._process(0); await process_frame
	for entry in panel.entries:
		if entry.catalyst: _expect(entry.result.text == "Catalyst ↻", "Catalyst visibly names the reroll")
	panel.resume_at(2.4); panel._process(0); await process_frame
	var trails: Array = panel.catalyst_trails()
	_expect(trails.size() == 1 and trails[0].holder == "blade", "Catalyst trail comes from the recorded holder")
	if not trails.is_empty():
		_expect(trails[0].travel == 1.0, "green trail reaches the die before reroll begins")
		_expect(trails[0].target.text == "6", "clearing result held throughout extended Catalyst cue")
		var anchor: Vector2 = screen._actor_profiles.blade.status_anchor("catalyst")
		_expect(screen._actor_profiles.blade.statuses.get_global_rect().has_point(anchor), "trail originates inside Catalyst status list")
	await _capture("catalyst")
	panel.resume_at(2.8); panel._process(0); await process_frame
	for entry in panel.entries:
		if entry.catalyst: _expect(entry.result.text == "Catalyst ↻" and entry.die.rotation != 0, "extended reroll stays animated and labeled")
	var extra: float = panel.catalyst_extra_seconds()
	_expect(is_equal_approx(extra, 0.75), "Catalyst animation gets three quarters of a second more")
	panel.resume_at(3.8 + extra); panel._process(0); await process_frame
	for entry in panel.entries:
		for card in entry.cards_ui: _expect(card.modulate.a > 0.9, "cards visible beneath their damaging effect")
	await _capture("cards")
	screen._render(); await process_frame; panel = screen._effects_panel
	_expect(panel.playback_elapsed() >= 3.8, "redraw preserves playback position")
	panel.set_paused(true)
	var held: float = panel.playback_elapsed()
	await create_timer(0.15).timeout
	_expect(absf(panel.playback_elapsed() - held) < 0.02, "developer inspection freezes playback")
	panel.set_paused(false)
	panel.resume_at(6.2 + extra); panel._process(0); await process_frame
	_expect("Bleed 3 → 2" in screen._log.text and "Volatile Poison 1 → 2" in screen._log.text, "status closeout recorded in combat log")
	for label in panel.find_children("*", "Label", true, false):
		_expect(" → " not in label.text, "state-change summary stays out of the play area")
	for entry in panel.entries:
		if entry.status_id == "bleed": _expect(entry.die.text == "2", "Bleed counter visibly decreases")
	await _capture("closeout")
	panel.resume_at(panel.duration + 0.1); await process_frame; await process_frame
	_expect(screen._director.peek().get("type") != "effects_resolved", "Effects finishes with Disable auto-pass checked")
	_expect(fake.commands.is_empty(), "animation never submits gameplay commands")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("AUTOMATIC EFFECTS VISUAL: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _capture(suffix: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_EFFECTS_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join("effects-" + suffix + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("AUTOMATIC EFFECTS: " + message)
