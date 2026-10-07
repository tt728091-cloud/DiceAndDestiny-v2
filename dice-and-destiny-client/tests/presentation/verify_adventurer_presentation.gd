extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TARGETING := preload("res://presentation/battle/die_face_targeting.gd")
var failed := false
var base: Dictionary
class Recorder extends RefCounted:
	var commands: Array = []
	func submit(value: String) -> Dictionary:
		commands.append(JSON.parse_string(value))
		return {"accepted": false, "error": "Recorded test command"}
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	base = gateway.start_battle("adventurer-presentation", 43)
	_expect(base.get("accepted") == true, "native starter loads")
	if not base.get("accepted", false): quit(1); return
	base.events = []; base.learned_policy = {}; base.pending_input = {}; base.legal_actions = []
	var sizes := [Vector2i(1280, 720), Vector2i(1920, 1080)]
	if OS.get_environment("DICE_AND_DESTINY_ADVENTURER_WIDTH") == "1280": sizes = [Vector2i(1280, 720)]
	if OS.get_environment("DICE_AND_DESTINY_ADVENTURER_WIDTH") == "1920": sizes = [Vector2i(1920, 1080)]
	for size in sizes:
		print("ADVENTURER UI: viewport ", size)
		root.size = size
		await _targeting(size)
		await _guard(size)
		await _protection(size)
		for count in [1, 2, 3, 4]: await _tooltips(size, count)
	print("ADVENTURER UI: native cards")
	await _native_cards(gateway)
	print("ADVENTURER PRESENTATION: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _screen(fixture: Dictionary, gateway = null):
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = Recorder.new() if gateway == null else gateway
	screen._auto_pass_disabled = true; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("adventurer-ui.json"))
	root.add_child(screen); screen.set_process(false)
	return screen
func _dice(faces: Array) -> Array:
	var result := []
	for f in faces: result.append({"die_id": "standard_d6", "face": f, "value": f, "symbols": ["sword" if f <= 3 else "shield" if f <= 5 else "gold_coin"]})
	return result
func _selector(screen):
	for child in screen._root.get_children():
		if child.get_script() == TARGETING: return child
	return null
func _targeting(size: Vector2i) -> void:
	var fixture := base.duplicate(true)
	fixture.snapshot.stage = "planning"; fixture.snapshot.segment = "offensive"
	fixture.snapshot.actors.blade.dice = {"pool": "offensive", "dice": _dice([1, 3, 4, 5, 6])}
	fixture.snapshot.actors.blade.roll_history = [{"dice": _dice([1, 3, 4, 5, 6])}]
	# Program cards: start-and-target actions carry each first die choice.
	for face in [2, 4]: fixture.legal_actions.append(_program_start("nudge-test", {"kind": "offensive_die", "actor": "blade", "die": 1, "face": face, "label": "Adventurer · die 2: 3 → %d" % face}))
	for die in [3, 4]: fixture.legal_actions.append(_program_start("reroll-test", {"kind": "offensive_die", "actor": "blade", "die": die, "label": "Adventurer · die %d" % (die + 1)}))
	var recorder := Recorder.new(); var screen = _screen(fixture, recorder)
	for frame in 5: await process_frame
	for id in ["brace", "nudge", "try_again", "strong_swing", "take_stock", "second_wind"]:
		var card := BattleCard.new(); screen._root.add_child(card); card.configure(id, id, true)
		_expect(not BattlePresentationCatalog.card(id).text.is_empty() and not card._effect_plaque.find_child("EffectSummary", true, false).text.is_empty(), id + " has shared card rules and effect text")
		card.queue_free()
	var card := BattleCard.new(); card.instance_id = "nudge-test"; card.definition_id = "nudge"
	screen._on_card_pressed(card); card.free()
	for frame in 5: await process_frame
	var selector = _selector(screen)
	_expect(selector != null, "Nudge uses existing board targeting")
	if selector != null:
		_expect(selector.targets.size() == 1 and recorder.commands.is_empty(), "only legal own die highlighted, no premature spend")
		selector.targets["blade:1"].pressed.emit()
		for frame in 5: await process_frame
		selector = _selector(screen)
		_expect(selector.faces.size() == 2, "Nudge offers only adjacent legal faces")
		_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), "face foldout fits viewport")
		await _capture("adventurer-nudge-%d" % size.x)
		selector.faces[0].pressed.emit(); await process_frame
		_expect(recorder.commands.size() == 1 and JSON.parse_string(recorder.commands[0].payload.status_id).face == 2, "Nudge sends exact legal face command")
	screen._error_message = ""; screen._selected_card.clear(); screen._render(); await process_frame
	card = BattleCard.new(); card.instance_id = "reroll-test"; card.definition_id = "try_again"
	screen._on_card_pressed(card); card.free(); await process_frame
	selector = _selector(screen)
	_expect(selector != null, "Try Again highlights owned dice")
	if selector != null:
		_expect(selector.targets.size() == 2, "Try Again highlights each legal die")
		selector.targets["blade:4"].pressed.emit(); await process_frame
		_expect(recorder.commands.size() == 2 and int(JSON.parse_string(recorder.commands[1].payload.status_id).die) == 4, "Try Again submits selected die")
	screen.queue_free(); await process_frame
func _guard(size: Vector2i) -> void:
	for faces in [[1, 4, 6], [6, 6, 6]]:
		var fixture := base.duplicate(true); fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
		fixture.snapshot.damage_sources = [{"id": "incoming", "source_actor_id": "goblin", "source_content_id": "brine_lash", "target_actor_id": "blade", "base_amount": 7}]
		fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "adventurer_guard", "source_id": "incoming", "rolled_face": faces[0], "rolled_faces": faces}}
		var screen = _screen(fixture)
		for frame in 6: await process_frame
		var panel = screen._defense_result_panels[0]
		_expect(panel.data.dice.size() == 3 and panel.dice_controls.size() == 3, "three shared animated defense dice")
		_expect(panel.data.prevented == (3 if faces[0] == 1 else 0), "prevention preview matches symbols")
		_expect(panel.data.gains.size() == 1 and panel.data.gains[0].amount == 1 and panel.data.gains[0].get("resource", false), "Coin energy shown once even for triple six")
		var gain = panel.data.gains[0]
		var flight = preload("res://presentation/battle/defense_status_flight.gd").new(); screen._root.add_child(flight)
		flight.configure(panel.gain_origins[0], screen._actor_profiles.blade, gain, Time.get_ticks_msec())
		var expected: Vector2 = flight.get_global_transform_with_canvas().affine_inverse() * screen._actor_profiles.blade.anchor_rect("energy").get_center()
		_expect(flight.destination().distance_to(expected) < 0.1, "energy flight targets live energy counter")
		await _capture("adventurer-guard-%d-%d" % [size.x, faces[0]])
		screen.queue_free(); await process_frame
func _tooltips(size: Vector2i, count: int) -> void:
	for stage in ["offensive_reaction", "defense_selection"]:
		var fixture := base.duplicate(true); fixture.snapshot.stage = stage; fixture.snapshot.segment = "offensive" if stage == "offensive_reaction" else "defensive"
		fixture.snapshot.damage_sources = []
		for n in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(n)] = fixture.snapshot.actors.goblin.duplicate(true)
		for id in fixture.snapshot.actors:
			var player: bool = id == "blade"; var actor: Dictionary = fixture.snapshot.actors[id]
			actor.planning_committed = true
			actor.selected_ability = "adventurer_strike" if player else "brine_lash"; actor.selected_tier = "4_swords" if player else "brine_2"
			actor.dice = {"pool": "offensive", "dice": _dice([1, 2, 3, 3, 6])}
			fixture.snapshot.damage_sources.append({"id": id + "-attack", "source_actor_id": id, "source_content_id": actor.selected_ability, "target_actor_id": "goblin" if player else "blade", "base_amount": 5})
		var screen = _screen(fixture)
		for frame in 4: await process_frame
		for id in fixture.snapshot.actors:
			var intent: Control = screen._attack_intents[id + "-attack"].intent
			_expect(root.get_visible_rect().encloses(intent.get_global_rect()), "revealed attack intent stays inside viewport " + id)
		var panel = screen._attack_intents["blade-attack"]
		_expect("4 Sword" in panel.intent.tooltip_text and "5 DMG" in panel.intent.tooltip_text and "1 · 2 · 3 · 3 · 6" in panel.intent.tooltip_text, "hover explains selected tier and actual dice")
		for id in fixture.snapshot.actors.blade.offensive_abilities:
			for tier in BattlePresentationCatalog.definition("abilities", id).qualification.activation_tiers:
				var hint := BattlePresentationCatalog.attack_ability_tooltip(id, {}, {"ability_id": id, "tier_id": tier.id}, _dice([1, 2, 3, 3, 6]))
				var popup: Control = panel.intent._make_custom_tooltip(hint); screen._root.add_child(popup)
				await process_frame
				_expect(not hint.is_empty() and "Selected:" in hint and "Misses" in hint, id + " full rules/tier/miss contract")
				_expect(popup.size.x < 1280 and popup.size.y < 650, id + " wrapped tooltip fits small viewport")
				popup.queue_free(); await process_frame
		await _capture("adventurer-%s-%d-%d" % [stage, size.x, count])
		screen.queue_free(); await process_frame
func _protection(size: Vector2i) -> void:
	var fixture := base.duplicate(true); fixture.snapshot.segment = "damage_resolution"; fixture.snapshot.stage = "damage_reaction"
	fixture.snapshot.damage_sources = []
	for n in range(1, 5):
		var actor_id := "goblin" if n == 1 else "goblin-" + str(n)
		if n > 1: fixture.snapshot.actors[actor_id] = fixture.snapshot.actors.goblin.duplicate(true)
		var source_id := "incoming-%d" % n
		fixture.snapshot.damage_sources.append({"id": source_id, "source_actor_id": actor_id, "source_content_id": "brine_lash", "target_actor_id": "blade", "base_amount": 4})
		fixture.legal_actions.append({"type": "commit_interaction", "actor_id": "blade", "payload": {"commitment": {"choice_id": "spend_round_prevention", "proposal_ids": [source_id]}}})
	fixture.snapshot.actors.blade.statuses = [{"definition_id": "protect", "stacks": 2}]
	fixture.snapshot.settled_damage = {"id": "protection-batch", "sources": fixture.snapshot.damage_sources, "removals": []}
	var recorder := Recorder.new(); var screen = _screen(fixture, recorder)
	for frame in 5: await process_frame
	_expect(screen._action_footer.get_children().all(func(child): return not child is Button or "protection" not in child.text), "no floating protection footer")
	screen._attack_intents["incoming-4"].intent.pressed.emit()
	for frame in 5: await process_frame
	var choice: Button = screen._ability_dock.get_node_or_null("ProtectChoice")
	_expect(choice != null, "selected attack exposes Protect choice")
	if choice != null:
		_expect(root.get_visible_rect().encloses(choice.get_global_rect()), "Protect choice fits viewport")
		# The retired generic entry cannot reopen a target dialog, even with
		# four eligible sources. It resolves only the already-selected attack.
		screen._show_venom_choices(fixture.legal_actions.filter(func(action): return action.get("payload", {}).get("commitment", {}).get("choice_id") == "spend_round_prevention"), "Guarded Strike")
		await process_frame
		_expect(screen.get_children().all(func(child): return not child is AcceptDialog), "legacy Protect entry cannot create a target modal")
		_expect(recorder.commands.size() == 1 and recorder.commands[0].payload.commitment.proposal_ids == ["incoming-4"], "protection submits only selected source")
	# Cardless protection uses the same saved-card feedback without inventing a played card.
	var feedback := {"events": [{"type": "damage_prevented_or_modified", "sequence": 44, "actor_id": "blade", "data": {"ability_id": "guarded_strike", "status_id": "protect", "source_id": "incoming-4", "damage_before": 4, "damage_after": 2}}]}
	screen._capture_damage_feedback(feedback, {})
	screen._error_message = ""; screen._render(); await process_frame
	var panels: Array = screen._root.get_children().filter(func(child): return child.get_script() == preload("res://presentation/battle/damage_response_feedback.gd"))
	_expect(panels.size() == 1 and not panels[0]._played.visible and "Protect" in screen._damage_feedback.text, "protection animation names ability and hides card")
	await _capture("adventurer-protection-%d" % size.x)
	screen.queue_free(); await process_frame
func _native_cards(gateway) -> void:
	for card_id in ["nudge", "try_again", "take_stock", "second_wind", "strong_swing"]:
		var tested := false
		for seed_value in range(1, 45):
			var result: Dictionary = gateway.start_battle("starter-card-%s-%d" % [card_id, seed_value], seed_value)
			for action in result.get("legal_actions", []):
				if card_id != "strong_swing" and action.get("type") == "planning_roll": result = gateway.submit(JSON.stringify(action)); break
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model()
			for action in _complete_card_actions(result, card_id):
				var screen = _screen(result, gateway); await process_frame
				screen._director.clear(); screen._send(JSON.stringify(action))
				_expect(screen._error_message.is_empty(), card_id + " native play succeeds")
				if card_id in ["nudge", "try_again"]:
					_expect(screen.get_children().any(func(child): return child.get_script() == preload("res://presentation/battle/curse_notice.gd")), card_id + " queues shared die-change animation")
				if card_id in ["take_stock", "second_wind"]:
					_expect(screen.get_children().any(func(child): return child.get_script() == preload("res://presentation/battle/card_gain_notice.gd")), card_id + " queues shared resource/card animation")
				screen.queue_free(); await process_frame; tested = true; break
			if tested: break
		_expect(tested, card_id + " exercised through native gateway")
func _program_start(card: String, choice: Dictionary) -> Dictionary:
	var start := {"verb": "start", "then": "target", "die": 0}; start.merge(choice, true)
	return {"type": "planning_commit_cards", "actor_id": "blade", "payload": {"card_ids": [card], "status_id": JSON.stringify(start)}}
# Program cards: prefer a start that also chooses its first target, so one
# command completes the card; cards without choices use the plain start.
func _complete_card_actions(result: Dictionary, card_id: String) -> Array:
	var plain: Array = []
	for action in result.get("legal_actions", []):
		var ids: Array = action.get("payload", {}).get("card_ids", [])
		if ids.size() != 1 or result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") != card_id: continue
		var choice = JSON.parse_string(str(action.payload.get("status_id", "")))
		if choice is Dictionary and not str(choice.get("then", "")).is_empty(): return [action]
		plain.append(action)
	return plain
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_ADVENTURER_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		RenderingServer.force_draw()
		root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ADVENTURER: " + message)
