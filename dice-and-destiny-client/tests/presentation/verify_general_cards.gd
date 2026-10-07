extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var completed := 0
var canvas: SubViewport
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
var rolled: Dictionary
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	base = gateway.start_battle("general-card-ui", 43)
	rolled = base
	for action in base.legal_actions:
		if action.type == "planning_roll": rolled = gateway.submit(JSON.stringify(action)); break
	for width in [1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for id in ["matchmaker", "turn_the_die", "disrupt", "second_guard", "reclaim", "reinforce", "dispel", "triage"]:
			await _scenario(id)
		await _scenario("reinforce", true)
	_expect(completed == 18, "all eighteen scenarios completed")
	print("GENERAL CARD UI: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

# Program-card choices as the authority sends them: a plain start plus one
# start-and-target action per first choice (card-first targeting).
func _choices(id: String) -> Array:
	match id:
		"matchmaker": return [{"kind": "offensive_die", "actor": "blade", "die": 0, "face": 4, "label": "Adventurer · die 1 → 4"}, {"kind": "offensive_die", "actor": "blade", "die": 1, "face": 5, "label": "Adventurer · die 2 → 5"}]
		"turn_the_die": return [{"kind": "offensive_die", "actor": "blade", "die": 0, "face": 6, "label": "Adventurer · die 1 → 6"}, {"kind": "offensive_die", "actor": "blade", "die": 1, "face": 5, "label": "Adventurer · die 2 → 5"}]
		"disrupt": return [{"kind": "offensive_die", "actor": "goblin", "die": 0, "label": "Brine Mask · die 1 (6)"}, {"kind": "offensive_die", "actor": "goblin", "die": 1, "label": "Brine Mask · die 2 (6)"}]
		"second_guard": return [{"kind": "defensive_die", "actor": "blade", "die": 0, "label": "Adventurer · die 1 (2)"}, {"kind": "defensive_die", "actor": "blade", "die": 1, "label": "Adventurer · die 2 (3)"}]
		"reclaim": return [{"kind": "card", "actor": "blade", "card": "nudge-x", "label": "Nudge"}, {"kind": "card", "actor": "blade", "card": "brace-x", "label": "Brace"}]
		"reinforce": return [{"kind": "option", "option": 0, "label": "Prevent 2 damage · 1 energy"}, {"kind": "option", "option": 1, "label": "Prevent 4 damage · 2 energy"}]
		"dispel": return [{"kind": "status", "actor": "goblin", "status": "protect", "label": "Brine Mask · Protect (2)"}, {"kind": "status", "actor": "goblin", "status": "catalyst", "label": "Brine Mask · Catalyst (1)"}]
		"triage": return [{"kind": "threatened_card", "actor": "blade", "card": "nudge-x", "source": "incoming", "label": "Save Nudge from Brine Mask · Brine Lash"}, {"kind": "threatened_card", "actor": "blade", "card": "brace-x", "source": "incoming", "label": "Save Brace from Brine Mask · Brine Lash"}]
	return []

func _scenario(id: String, multiple: bool = false) -> void:
	var dice_card := id in ["matchmaker", "turn_the_die", "disrupt"]
	var fixture: Dictionary = (rolled if dice_card else base).duplicate(true); fixture.events = []; fixture.learned_policy = {}
	var planning := id in ["matchmaker", "turn_the_die", "reclaim", "dispel"]
	fixture.snapshot.unified_defense = true
	fixture.snapshot.stage = "planning" if planning else ("offensive_reaction" if id == "disrupt" else ("defense_reaction" if id == "second_guard" else "defense_selection"))
	fixture.snapshot.segment = "offensive" if planning or id == "disrupt" else "defensive"
	fixture.snapshot.actors.blade.hand = ["test-card"]; fixture.snapshot.actors.blade.hand_count = 1
	fixture.snapshot.actors.blade.card_instances = {"test-card": {"instance_id": "test-card", "definition_id": id}}
	fixture.snapshot.actors.blade.energy_points = 5
	if id == "disrupt": fixture.snapshot.actors.goblin.dice = fixture.snapshot.actors.blade.dice.duplicate(true)
	var command := "planning_commit_cards" if planning else "commit_interaction"
	fixture.pending_input = {"blade": {"id":"general-input", "segment":fixture.snapshot.segment, "stage":fixture.snapshot.stage, "allowed_commands":[command, "pass"]}}
	fixture.snapshot.damage_sources = [{"id":"incoming", "source_actor_id":"goblin", "target_actor_id":"blade", "source_content_id":"brine_lash", "base_amount":4, "final_amount":4}]
	if multiple:
		var second: Dictionary = fixture.snapshot.damage_sources[0].duplicate(true); second.id = "incoming-2"
		fixture.snapshot.damage_sources.append(second)
	fixture.snapshot.settled_damage = {"id":"batch", "sources":fixture.snapshot.damage_sources.duplicate(true), "removals":[]}
	fixture.legal_actions = [_action(fixture, command, {"verb": "start", "label": "Play card", "die": 0})]
	for choice in _choices(id):
		var start: Dictionary = {"verb": "start", "then": "target" if choice.kind != "option" else "option", "die": 0}; start.merge(choice, true)
		fixture.legal_actions.append(_action(fixture, command, start))
	var fake := FakeBattleAuthority.new()
	var after := fixture.duplicate(true); after.pending_input = {}; after.legal_actions = []; after.snapshot.actors.blade.hand = []
	fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("general-card-ui.json")); canvas.add_child(screen); screen.set_process(false)
	await _ready_hand(screen)
	_expect(screen._card_legal(id), id + " is playable")
	await _click_card(screen)
	if dice_card:
		# Die effects target the dice on the board, card-first and unspent.
		_expect(fake.commands.is_empty() and screen._selected_card.get("die_targeting", false), id + " click starts die targeting without spending")
		var marks: Array = screen._mark_actions()
		_expect(marks.size() == 2, id + " highlights both legal dice")
		await _click_card(screen)
		_expect(fake.commands.is_empty() and screen._selected_card.is_empty(), id + " second click cancels without spending")
		await _ready_hand(screen); await _click_card(screen)
		if marks.size() == 2:
			var parts: PackedStringArray = screen._mark_choice(screen._mark_actions()[1]).split(":")
			screen._select_mark_die(parts[0], int(parts[1]))
			for frame in 8: await process_frame
			_expect(fake.commands.size() == 1 and fake.commands[0] == JSON.stringify(fixture.legal_actions[2]), id + " submits exact selected authority action")
	else:
		var dialogs := screen.find_children("*", "AcceptDialog", false, false)
		_expect(dialogs.size() == 1 and fake.commands.is_empty(), id + " click opens choices without spending")
		if dialogs.size() == 1:
			var dialog: AcceptDialog = dialogs[0]
			_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(dialog.position), Vector2(dialog.size))), id + " prompt fits viewport")
			var buttons := _choice_buttons(dialog)
			_expect(buttons.size() == 2, id + " shows both choices")
			if buttons.size() == 2:
				var scroll: ScrollContainer = buttons[1].get_parent().get_parent()
				_expect(buttons[1].get_global_rect().end.y <= scroll.get_global_rect().end.y, id + " both options visible without scrolling")
				for button in buttons:
					_expect(button.text != "Confirm" and "seat-" not in button.text and "{" not in button.text, id + " readable choice label: " + button.text)
				if id == "reinforce":
					_expect(buttons[0].text == "Prevent 2 damage · 1 energy" and buttons[1].text == "Prevent 4 damage · 2 energy", "boost labels show authored amounts and total costs")
				await _capture(id)
				# Cancel the choice, then reopen it by clicking the card.
				dialog.get_ok_button().pressed.emit()
				for frame in 8: await process_frame
				_expect(fake.commands.is_empty() and screen._selected_card.is_empty(), id + " cancel clears selection without spending")
				await _ready_hand(screen); await _click_card(screen)
				var reopened := screen.find_children("*", "AcceptDialog", false, false)
				_expect(reopened.size() == 1, id + " choice reopens after cancel")
				if reopened.size() != 1: screen.queue_free(); await process_frame; return
				_choice_buttons(reopened[0])[1].pressed.emit()
				for frame in 8: await process_frame
				_expect(fake.commands.size() == 1 and fake.commands[0] == JSON.stringify(fixture.legal_actions[2]), id + " submits exact selected authority action")
	_expect(screen._error_message.is_empty(), id + " no client error")
	completed += 1
	screen.queue_free(); await process_frame
func _action(fixture: Dictionary, command: String, choice: Dictionary) -> Dictionary:
	var payload := {"pending_input_id": "general-input"}
	if command == "planning_commit_cards": payload.merge({"card_ids": ["test-card"], "status_id": JSON.stringify(choice)})
	else: payload.commitment = {"card_ids": ["test-card"], "choice_id": JSON.stringify(choice)}
	return {"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": command, "payload": payload}
func _choice_buttons(dialog: AcceptDialog) -> Array:
	var buttons: Array = []
	for button in dialog.find_children("*", "Button", true, false):
		if str(button.get_meta("inspection_id", "")).begins_with("battle.venom.choice."): buttons.append(button)
	return buttons
func _ready_hand(screen) -> void:
	for frame in 12: await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	screen._hand_dock.held_open = true; screen._hand_dock.reveal = 1; screen._hand_dock._layout(); await process_frame
func _click_card(screen) -> void:
	var hand: Control = screen._hand_dock
	await _click(hand.get_global_transform_with_canvas() * (hand.base_transforms[0] * Vector2(90, 100)))
	for frame in 8: await process_frame
func _click(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true); await process_frame
func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_GENERAL_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless" or id not in ["reinforce", "matchmaker", "triage"]: return
	RenderingServer.force_draw(); canvas.get_texture().get_image().save_png(directory.path_join("%s-%d.png" % [id, canvas.size.x]))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("GENERAL CARD UI: " + message)
