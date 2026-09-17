extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_roll_seconds", 0.2)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_effects_seconds", 1.8)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_hold_seconds", 1.0)
	root.size = Vector2i(1280, 720)
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = native.start_battle("enemy-only-defense", 1789503213)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	fixture.events = []; fixture.learned_policy = {}
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
	fixture.snapshot.pass_hands_off_priority = true
	fixture.snapshot.damage_sources = [{"id": "needlefang", "source_actor_id": "blade", "source_content_id": "needlefang", "target_actor_id": "goblin", "base_amount": 3, "status_applications": [{"status_id": "poison", "stacks": 3, "target_actor_id": "goblin"}]}]
	fixture.snapshot.defense_selections = {"goblin": {"actor_id": "goblin", "ability_id": "basic_defense", "source_id": "needlefang", "rolled_face": 3, "rolled_faces": [3]}}
	fixture.pending_input = {"blade": {"id": "first-handoff", "segment": "defensive", "stage": "defense_reaction", "allowed_commands": ["pass"]}}
	fixture.legal_actions = [_pass("first-handoff")]
	var another := fixture.duplicate(true)
	another.pending_input.blade.id = "second-handoff"
	another.legal_actions = [_pass("second-handoff")]
	var next := fixture.duplicate(true)
	next.snapshot.segment = "offensive"; next.snapshot.stage = "planning"; next.snapshot.round = 2
	next.pending_input = {}; next.legal_actions = []; next.snapshot.pass_hands_off_priority = false
	var fake := FakeBattleAuthority.new(); fake.enqueue(another); fake.enqueue(next)
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("enemy-only-defense.json"))
	root.add_child(screen)
	await process_frame; await process_frame
	_expect(screen._defense_result_panels.size() == 1, "one result panel when only enemy defends")
	if screen._defense_result_panels.is_empty():
		screen.queue_free(); quit(1); return
	var panel = screen._defense_result_panels[0]
	_expect(panel.data.actor_id == "goblin", "enemy defense is displayed")
	_expect(panel.data.before == 3 and panel.data.after == 0 and panel.data.prevented == 3, "Needlefang's three damage is fully blocked")
	_expect(panel.damage.text == "3", "animation starts at three incoming damage")
	_expect(panel.block.text == "PREVENT 3" and panel.is_visible_in_tree(), "block feedback is visible")
	var found_die := false
	for button in panel.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.defense_die.goblin":
			found_die = true
	_expect(found_die, "enemy defense die has a stable slot while rolling")
	await create_timer(1.5).timeout
	_expect(fake.commands.is_empty() and panel.damage.text == "0", "damage animates to zero before any handoff")
	_expect(root.get_visible_rect().encloses(panel.get_global_rect()), "result panel remains inside viewport")
	await create_timer(1.3).timeout
	_expect(fake.commands.is_empty() and screen._view.stage == "defense_reaction", "enemy-only defense remains visible at 2.8 seconds")
	var capture := OS.get_environment("DICE_AND_DESTINY_DEFENSE_REVIEW_SCREENSHOT")
	if not capture.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture)
	await create_timer(0.65).timeout
	_expect(fake.commands.size() == 2 and screen._view.stage == "planning", "review completes then same-roll handoffs progress without a second delay")
	# A new round must receive its own defense review.
	fixture.snapshot.round = 2
	fixture.pending_input.blade.id = "next-round-defense"
	fixture.legal_actions = [_pass("next-round-defense")]
	screen.set_process(false)
	screen._view.apply_result(fixture); screen._render(); screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 2 and screen._auto_pass_preview_input.ends_with("next-round-defense"), "new round gets a fresh review")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("ENEMY-ONLY DEFENSE REVIEW: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _pass(id: String) -> Dictionary:
	return {"battle_id": "enemy-only-defense", "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": id}}

func _expect(ok: bool, message: String) -> void:
	if not ok:
		failed = true
		push_error("ENEMY-ONLY DEFENSE REVIEW: " + message)
