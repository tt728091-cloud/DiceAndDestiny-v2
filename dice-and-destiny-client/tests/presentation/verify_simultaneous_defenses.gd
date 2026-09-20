extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	var initial: Dictionary = gateway.start_battle("simultaneous-defenses", 43)
	initial.events = []; initial.learned_policy = {}; initial.pending_input = {}; initial.legal_actions = []
	initial.snapshot.segment = "defensive"; initial.snapshot.stage = "defense_reaction"
	initial.snapshot.damage_sources = [
		{"id": "first", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 6},
		{"id": "second", "source_actor_id": "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 5},
		{"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 4}]
	var first := {"actor_id": "blade", "source_id": "first", "ability_id": "shedskin", "rolled_face": 1, "rolled_faces": [1, 4]}
	var second := {"actor_id": "blade", "source_id": "second", "ability_id": "shedskin", "rolled_face": 6, "rolled_faces": [6, 5]}
	var enemy := {"actor_id": "goblin", "source_id": "outgoing", "ability_id": "salt_veil", "rolled_face": 4, "rolled_faces": [4]}
	initial.snapshot.defense_selections = {"blade": first, "goblin": enemy}
	initial.snapshot.defense_plans = {"second": second}
	initial.snapshot.actors.blade.statuses = []
	var first_wave := initial.duplicate(true)
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen._auto_pass_disabled = true
	screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("simultaneous.json"))
	root.add_child(screen); screen.set_process(false)
	await process_frame; await process_frame
	var panels: Array = screen._defense_result_panels
	_expect(panels.size() == 3, "all three defense panels exist")
	_expect(panels[0].dice_controls.size() == 2 and panels[1].dice_controls.size() == 2 and panels[2].dice_controls.size() == 1, "all five authoritative dice present at the first checkpoint")
	var shared: int = screen._defense_shared_start
	for panel in panels: _expect(int(panel.data.roll_started_ms) == shared, "all dice start on one clock")
	_expect(panels[1].data.effects_pending and panels[1].data.gains.is_empty(), "queued status effects wait for their source")
	await create_timer(TIMING.roll_seconds() + TIMING.effects_seconds() + 0.8).timeout
	_expect(panels[1].dice_controls[0].text.ends_with("6") and panels[1].dice_controls[1].text.ends_with("5"), "second defense already reveals its final dice")
	_expect(screen._actor_profiles.blade.statuses.text.contains("Catalyst ×1 · pending"), "first defense previews exactly one Catalyst")
	_expect(panels[1].damage.text == "5", "queued prevention has not animated early")
	first.finalized = true; enemy.finalized = true
	initial.snapshot.defense_history = {"first": first, "outgoing": enemy}
	initial.snapshot.defense_plans = {}
	initial.snapshot.defense_selections = {"blade": second}
	initial.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 1}]
	initial.snapshot.damage_sources[0].prevention = 1
	initial.snapshot.damage_sources[2].prevention = 2
	screen._view.apply_result(initial); screen._render(true)
	await process_frame; await process_frame
	var next: Control = screen._defense_result_panels[1]
	_expect(int(next.data.roll_started_ms) == shared, "second effects wave reuses the original roll clock")
	_expect(not next.data.effects_pending and next.data.gains.size() == 2, "second wave activates its own status effects")
	_expect(next.data.gains[-1].after == 2, "completed defense is not added to the Catalyst preview twice")
	next._update()
	_expect(next.dice_controls[0].text.ends_with("6") and next.dice_controls[1].text.ends_with("5"), "second wave never spins the dice again")
	_expect(screen._focused_enemy == "goblin-2", "HUD follows the second enemy for status animation")
	_expect(screen._sole_pass_action().is_empty(), "debug pause remains available")
	await create_timer(TIMING.effects_seconds() + 1).timeout
	_expect(next.damage.text == "5", "second source damage remains independent")
	_expect(screen._actor_profiles.blade.statuses.text.contains("Catalyst ×2 · pending"), "second flight displays two pending Catalyst")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	await _automatic_chain(first_wave, initial, false)
	await _automatic_chain(first_wave, initial, true)
	print("SIMULTANEOUS DEFENSES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _automatic_chain(first: Dictionary, second: Dictionary, response_card: bool) -> void:
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_roll_seconds", 0.2)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_effects_seconds", 0.6)
	ProjectSettings.set_setting("dice_and_destiny/presentation/defense_hold_seconds", 0.1)
	var first_result := first.duplicate(true); var second_result := second.duplicate(true)
	for pair in [[first_result, "first-wave"], [second_result, "second-wave"]]:
		pair[0].pending_input = {"blade": {"id": pair[1], "stage": "defense_reaction", "allowed_commands": ["pass", "commit_interaction"]}}
		pair[0].legal_actions = [{"battle_id": first.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": pair[1]}}]
	if response_card: second_result.legal_actions.append({"actor_id": "blade", "type": "commit_interaction"})
	var damage := second_result.duplicate(true)
	damage.snapshot.segment = "damage_resolution"; damage.snapshot.stage = "damage_reaction"
	damage.pending_input = {}; damage.legal_actions = []
	damage.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 2}]
	var fake := FakeBattleAuthority.new(); fake.enqueue(second_result); fake.enqueue(damage)
	var screen = SCREEN.instantiate(); screen.initial_result = first_result
	screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("auto-defense-chain.json"))
	root.add_child(screen)
	var seen_one := false; var seen_two := false; var overcounted := false
	var deadline := Time.get_ticks_msec() + 6000
	while Time.get_ticks_msec() < deadline:
		var label: String = screen._actor_profiles.blade.statuses.text
		seen_one = seen_one or label.contains("Catalyst ×1 · pending")
		seen_two = seen_two or label.contains("Catalyst ×2 · pending")
		overcounted = overcounted or label.contains("Catalyst ×3")
		if fake.commands.size() == 2: break
		await process_frame
	_expect(seen_one and seen_two, "both status flights arrive before progressing")
	_expect(not overcounted, "automatic playback never overcounts Catalyst")
	_expect(fake.commands.size() == (1 if response_card else 2), "each wave auto-passes unless a card response is available")
	_expect(screen._view.stage == ("defense_reaction" if response_card else "damage_reaction"), "final effects flow into damage or wait for a real choice")
	if response_card:
		screen._view.legal_actions.remove_at(1)
		deadline = Time.get_ticks_msec() + 3000
		while Time.get_ticks_msec() < deadline and fake.commands.size() < 2: await process_frame
		_expect(fake.commands.size() == 2, "automatic progression resumes when only pass remains")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("SIMULTANEOUS DEFENSES: " + message)
