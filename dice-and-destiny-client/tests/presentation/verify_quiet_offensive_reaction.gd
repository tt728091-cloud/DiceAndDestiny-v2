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


func _run() -> void:
	var fake := FakeBattleAuthority.new()
	var screen = _screen(_fixture("planning", "offensive", "planning", "planning_roll"), fake)
	screen._auto_pass_disabled = true
	await process_frame
	var board: Control = screen._root
	var response := _fixture("empty")
	screen._view.apply_result(response); screen._render()
	_expect(screen._quiet_offensive_reaction() and screen._root == board, "empty reaction keeps prior board")
	fake.enqueue(_fixture("defense", "defensive", "defense_selection", "planning_select_ability"))
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1 and screen._view.stage == "defense_selection", "empty final reaction skips even with debug toggle")
	var real := _fixture("card", "offensive", "offensive_reaction", "commit_interaction")
	screen._view.apply_result(real); screen._render()
	_expect(not screen._quiet_offensive_reaction(), "real reaction card is shown")
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1, "never auto-selects a card")
	# Re-entry after opponent dice edit must not be hidden or auto-passed.
	screen._view.apply_result(_fixture("reselect", "offensive", "planning", "planning_select_ability")); screen._render()
	_expect(not screen._quiet_offensive_reaction(), "reselection stays interactive")
	screen._auto_pass_if_only_action()
	_expect(fake.commands.size() == 1, "replacement ability remains the player's choice")
	# Keep history and failed decisions inspectable.
	screen._view.apply_result(response)
	screen._history_review = true
	_expect(not screen._quiet_offensive_reaction() and screen._sole_pass_action().is_empty(), "history inspection stays paused")
	screen.queue_free(); await process_frame
	print("QUIET OFFENSIVE REACTION: "+("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
