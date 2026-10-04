extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var completed := 0
var canvas: SubViewport
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	base = gateway.start_battle("general-card-ui", 43)
	for width in [1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for id in ["matchmaker", "turn_the_die", "disrupt", "second_guard", "reclaim", "reinforce", "dispel", "triage"]:
			await _scenario(id)
		await _scenario("reinforce", true)
	_expect(completed == 18, "all eighteen scenarios completed")
	print("GENERAL CARD UI: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(id: String, multiple: bool = false) -> void:
	var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}
	var planning := id in ["matchmaker", "turn_the_die", "reclaim", "dispel"]
	fixture.snapshot.unified_defense = true
	fixture.snapshot.stage = "planning" if planning else ("offensive_reaction" if id == "disrupt" else ("defense_reaction" if id == "second_guard" else "defense_selection"))
	fixture.snapshot.segment = "offensive" if planning or id == "disrupt" else "defensive"
	fixture.snapshot.actors.blade.hand = ["test-card"]; fixture.snapshot.actors.blade.hand_count = 1
	fixture.snapshot.actors.blade.card_instances = {"test-card": {"instance_id": "test-card", "definition_id": id}}
	fixture.snapshot.actors.blade.energy_points = 5
	var command := "planning_cards" if planning else "commit_interaction"
	fixture.pending_input = {"blade": {"id":"general-input", "segment":fixture.snapshot.segment, "stage":fixture.snapshot.stage, "allowed_commands":[command, "pass"]}}
	fixture.snapshot.damage_sources = [{"id":"incoming", "source_actor_id":"goblin", "target_actor_id":"blade", "source_content_id":"brine_lash", "base_amount":4, "final_amount":4}]
	fixture.snapshot.settled_damage = {"id":"batch", "sources":fixture.snapshot.damage_sources.duplicate(true), "removals":[]}
	fixture.legal_actions = []
	var kinds := {"matchmaker":"copy_die", "turn_the_die":"flip_die", "disrupt":"reroll_enemy_die", "second_guard":"reroll_defense_dice", "reclaim":"recover_discard", "reinforce":"boost_prevention", "dispel":"dispel_positive", "triage":"save_threatened_card"}
	for i in 2:
		var choice := {"kind":kinds[id], "actor":"seat-b" if id in ["disrupt", "dispel"] else "seat-a", "die":i, "copy":2, "face":4+i, "indices":[i], "definition":"nudge" if i == 0 else "brace", "status":"protect", "zone":"discard" if i == 0 else "hand"}
		var target := "goblin" if id in ["disrupt", "dispel"] else "blade"
		if id in ["reinforce", "triage", "second_guard"]: choice.source = "incoming"; target = "incoming"
		if id == "reinforce": choice.boost = i == 1
		var payload := {"pending_input_id":"general-input"}
		if planning: payload.merge({"card_ids":["test-card"], "target_ids":[target], "status_id":JSON.stringify(choice)})
		else: payload.commitment = {"card_ids":["test-card"], "proposal_ids":[target], "choice_id":JSON.stringify(choice)}
		fixture.legal_actions.append({"battle_id":fixture.snapshot.battle_id, "actor_id":"blade", "type":command, "payload":payload})
	if multiple:
		var source: Dictionary = fixture.snapshot.damage_sources[0].duplicate(true); source.id = "incoming-2"
		fixture.snapshot.damage_sources.append(source); fixture.snapshot.settled_damage.sources.append(source.duplicate(true))
		for original in fixture.legal_actions.duplicate(true):
			var action: Dictionary = original.duplicate(true)
			action.payload.commitment.proposal_ids = ["incoming-2"]
			var choice: Dictionary = JSON.parse_string(action.payload.commitment.choice_id); choice.source = "incoming-2"
			action.payload.commitment.choice_id = JSON.stringify(choice)
			fixture.legal_actions.append(action)
	var fake := FakeBattleAuthority.new()
	var after := fixture.duplicate(true); after.pending_input = {}; after.legal_actions = []; after.snapshot.actors.blade.hand = []
	fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("general-card-ui.json")); canvas.add_child(screen); screen.set_process(false)
	for frame in 12: await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	screen._hand_dock.held_open = true; screen._hand_dock.reveal = 1; screen._hand_dock._layout(); await process_frame
	_expect(screen._card_legal(id), id + " is playable")
	var hand: Control = screen._hand_dock
	await _click(hand.get_global_transform_with_canvas() * (hand.base_transforms[0] * Vector2(90, 100)))
	for frame in 8: await process_frame
	if multiple:
		_expect(fake.commands.is_empty() and screen._selected_card.get("source_targeting", false), "multiple attacks wait for source selection")
		await _click(screen._attack_intents["incoming-2"].intent.get_global_rect().get_center())
		for frame in 8: await process_frame
	var dialogs := screen.find_children("*", "AcceptDialog", false, false)
	_expect(dialogs.size() == 1 and fake.commands.is_empty(), id + " click opens choices without spending")
	if dialogs.size() == 1:
		var dialog: AcceptDialog = dialogs[0]
		_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(dialog.position), Vector2(dialog.size))), id + " prompt fits viewport")
		var buttons: Array = []
		for button in dialog.find_children("*", "Button", true, false):
			if str(button.get_meta("inspection_id", "")).begins_with("battle.venom.choice."): buttons.append(button)
		_expect(buttons.size() == 2, id + " shows both choices")
		if buttons.size() == 2:
			var scroll: ScrollContainer = buttons[1].get_parent().get_parent()
			_expect(buttons[1].get_global_rect().end.y <= scroll.get_global_rect().end.y, id + " both options visible without scrolling")
			for button in buttons:
				_expect(button.text != "Confirm" and "seat-" not in button.text and "{" not in button.text, id + " readable choice label")
			if id == "reinforce":
				_expect(buttons[0].text == "Prevent 2 damage · 1 energy" and buttons[1].text == "Prevent 4 damage · 2 energy", "boost labels show authored amounts and total costs")
			await _capture(id)
			# Cancel the choice, then reopen it by clicking the card.
			dialog.get_ok_button().pressed.emit()
			for frame in 8: await process_frame
			_expect(fake.commands.is_empty() and screen._selected_card.is_empty(), id + " cancel clears selection without spending")
			screen._flow_until = 0
			for item in screen._timed_buttons: item.until = 0
			screen._process(0)
			screen._hand_dock.held_open = true; screen._hand_dock.reveal = 1; screen._hand_dock._layout(); await process_frame
			hand = screen._hand_dock
			await _click(hand.get_global_transform_with_canvas() * (hand.base_transforms[0] * Vector2(90, 100)))
			for frame in 8: await process_frame
			if multiple:
				await _click(screen._attack_intents["incoming-2"].intent.get_global_rect().get_center())
				for frame in 8: await process_frame
			var reopened := screen.find_children("*", "AcceptDialog", false, false)
			_expect(reopened.size() == 1, id + " choice reopens after cancel")
			if reopened.size() != 1: screen.queue_free(); await process_frame; return
			buttons.clear()
			for button in reopened[0].find_children("*", "Button", true, false):
				if str(button.get_meta("inspection_id", "")).begins_with("battle.venom.choice."): buttons.append(button)
			buttons[1].pressed.emit()
			for frame in 8: await process_frame
			_expect(fake.commands.size() == 1 and fake.commands[0] == JSON.stringify(fixture.legal_actions[3 if multiple else 1]), id + " submits exact selected authority action")
	_expect(screen._error_message.is_empty(), id + " no client error")
	completed += 1
	screen.queue_free(); await process_frame
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
