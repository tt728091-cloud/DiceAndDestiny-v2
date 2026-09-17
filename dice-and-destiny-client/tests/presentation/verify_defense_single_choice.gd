extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("defense-choice-fixture", 1788900535520209)
	_expect(base.get("accepted") == true, "native catalog available")
	for ability in ["shedskin", "barbed_mantle"]:
		await _check(ability, "single")
		await _check(ability, "multiple_sources")
	await _check("shedskin", "catalyst")
	await _check("shedskin", "unavailable")
	await _check("shedskin", "history")
	print("DEFENSE SINGLE CHOICE: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check(ability: String, scenario: String) -> void:
	var fixture := base.duplicate(true)
	fixture.events = []
	fixture.learned_policy = {}
	fixture.pending_input = {"blade": {"id": "defense-input", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability"]}}
	fixture.snapshot.segment = "defensive"
	fixture.snapshot.stage = "defense_selection"
	fixture.snapshot.actors.blade.qualified_abilities = [ability]
	fixture.snapshot.actors.blade.defensive_abilities = [ability]
	fixture.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 1}] if scenario == "catalyst" else []
	var action := {"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "defense-input", "ability_id": ability, "target_ids": ["source-1"]}}
	fixture.legal_actions = [action]
	if scenario in ["catalyst", "multiple_sources"]:
		var alternative := action.duplicate(true)
		if scenario == "catalyst": alternative.payload.spend_catalyst = true
		else: alternative.payload.target_ids = ["source-2"]
		fixture.legal_actions.append(alternative)
	if scenario == "unavailable": fixture.legal_actions = []
	var fake := FakeBattleAuthority.new()
	var next := fixture.duplicate(true)
	next.legal_actions = []
	fake.enqueue(next)
	var screen = SCREEN.instantiate()
	screen.initial_result = fixture
	screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("defense-choice-test.json"))
	root.add_child(screen)
	await process_frame
	await process_frame
	if scenario == "history": screen._history_review = true
	var tile: Button
	for button in screen.find_children("*", "Button", true, false):
		if str(button.get_meta("inspection_id", "")) == "battle.ability.blade." + ability: tile = button
	_expect(tile != null, "defense button exists")
	if tile != null:
		_expect(not tile.disabled, "fixture offers clickable defense")
		tile.pressed.emit()
	await process_frame
	var dialogs := screen.find_children("*", "AcceptDialog", false, false)
	if scenario == "single":
		_expect(fake.commands.size() == 1, ability + " submits on first click")
		if fake.commands.size() == 1:
			_expect(fake.commands[0] == JSON.stringify(action), "exact source and ability submitted without Catalyst payment")
		_expect(dialogs.is_empty(), "single defense option opens no popup")
	elif scenario in ["catalyst", "multiple_sources"]:
		_expect(fake.commands.is_empty(), "waits for a real defense choice")
		_expect(dialogs.size() == 1, "multiple options keep popup")
		if dialogs.size() == 1:
			var choices: Array[Button] = []
			for button in dialogs[0].find_children("*", "Button", true, false):
				if str(button.get_meta("inspection_id", "")).begins_with("battle.venom.choice."): choices.append(button)
			_expect(choices.size() == 2, "both alternatives offered")
			if choices.size() == 2:
				choices[1].pressed.emit()
				_expect(fake.commands.size() == 1, "chosen alternative submits once")
				if fake.commands.size() == 1:
					_expect(fake.commands[0] == JSON.stringify(fixture.legal_actions[1]), "chosen Catalyst payment or source preserved")
	else:
		_expect(fake.commands.is_empty() and dialogs.is_empty(), "unavailable/review clicks do nothing")
	screen.active_store.clear()
	screen.queue_free()
	await process_frame

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("DEFENSE SINGLE CHOICE: " + message)
