extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1280, 720)
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
	_expect(panel._conversions.size() == 1, "Incubation conversion visual")
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
	panel.resume_at(3.8); panel._process(0); await process_frame
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
	panel.resume_at(6.2); panel._process(0); await process_frame
	_expect("Bleed 3 → 2" in panel._closeout.text and "Volatile Poison 1 → 2" in panel._closeout.text, "status closeout shows actual results")
	for entry in panel.entries:
		if entry.status_id == "bleed": _expect(entry.die.text == "2", "Bleed counter visibly decreases")
	await _capture("closeout")
	panel.resume_at(7.3); await process_frame; await process_frame
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
