extends SceneTree
## Simultaneous play by pointer against two Brine Masks (real native authority):
## a secret Before card, Roll locks in, the reveal, then the roll; no dice card
## on your own locked-in attack between the attacks and the defenses; and a
## Defense with two incoming attacks: threatened cards shown before any choice,
## each defense rolling the moment it is chosen (both at once, the screen never
## locked), its dice resting beside its attack, Second Guard on those dice, and
## automatic completion.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
## Cards that play in the Offense Before step without a target choice.
const BEFORE_CARDS := ["take_stock", "second_wind"]
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	await _offense_by_pointer(runtime)
	await _between_by_pointer(runtime)
	await _defense_by_pointer(runtime)
	print("SIMULTANEOUS PLAY: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _gateway(runtime) -> RefCounted:
	var gateway: RefCounted = GATEWAY.new(runtime, "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	gateway.simultaneous_play = true
	return gateway

## Runs opponent turns until the human has a decision.
func _human_turn(gateway: RefCounted, result: Dictionary) -> Dictionary:
	for step in 200:
		if result.get("accepted") != true or result.get("battle_result", "") != "": return result
		if not result.get("learned_policy", {}).get("model_turn", false): return result
		result = gateway.advance_model()
	return result

func _before_card(result: Dictionary) -> String:
	var blade: Dictionary = result.snapshot.actors.blade
	for id in blade.get("hand", []):
		if str(blade.card_instances[id].definition_id) in BEFORE_CARDS:
			for action in result.get("legal_actions", []):
				if str(action.get("payload", {}).get("card_ids", [""])[0]) == str(id): return str(id)
	return ""

func _screen(gateway: RefCounted, result: Dictionary, seed: int, store: String):
	var screen = SCREEN.instantiate()
	screen.initial_result = result; screen.viewer_actor_id = "blade"; screen.gateway = gateway
	screen.learned_battle_mode = true; screen.learned_human_seat = "seat-a"; screen.learned_seed = seed
	screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file(store))
	root.add_child(screen)
	for frame in 8: await process_frame
	screen._flow_until = 0
	return screen

func _offense_by_pointer(runtime) -> void:
	var gateway: RefCounted; var result: Dictionary; var card := ""; var seed := 0
	for candidate in range(1, 40):
		gateway = _gateway(runtime)
		result = _human_turn(gateway, gateway.start_battle("simultaneous-offense-%d" % candidate, candidate))
		_expect(result.get("accepted") == true, "the battle starts: %s" % str(result.get("error", "")))
		if result.get("accepted") != true: return
		card = _before_card(result)
		if not card.is_empty(): seed = candidate; break
	_expect(not card.is_empty(), "a seed opens with a Before card in hand")
	if card.is_empty(): return
	_expect(result.snapshot.get("simultaneous_play", false) and result.snapshot.get("simultaneous_step", "") == "offense_before", "the rule is pinned and Offense opens on the secret Before step")
	var screen = await _screen(gateway, result, seed, "simultaneous-offense.json")
	# A secret Before card: it resolves for the player alone and waits for the reveal.
	var card_button := await _inspection(screen, "battle.card.%s" % card)
	_expect(card_button != null and not card_button.disabled, "the Before card is playable by pointer")
	if card_button == null: screen.queue_free(); return
	await _click_hand(screen, card)
	await _settle(screen)
	var secret: Array = screen._secret_plays()
	_expect(secret.size() == 1 and str(secret[0].card_instance_id) == card, "the card is a secret play: %s" % [secret])
	# No text narrates secret plays; a card still waiting stays dimmed in hand.
	_expect(not screen.find_children("*", "Label", true, false).any(func(l): return "Secret until the reveal" in l.text), "no secret-play text on screen")
	_expect(screen._view.rolled_dice("blade").is_empty(), "no dice yet")
	# Roll locks in; the opponents lock in; the reveal comes before every roll.
	var roll := await _inspection(screen, "battle.command.planning_roll")
	_expect(roll != null and not roll.disabled and "locks you in" in screen._offensive_roll_hint(), "Roll is offered as the lock-in")
	if roll == null: screen.queue_free(); return
	await _click(roll)
	_expect(not screen.find_children("*", "Label", true, false).any(func(l): return "waiting for the others" in l.text), "no waiting text after locking in")
	var deadline := Time.get_ticks_msec() + 20000
	while Time.get_ticks_msec() < deadline and screen._view.rolled_dice("blade").size() != 5: await process_frame
	_expect(screen._view.rolled_dice("blade").size() == 5 and screen._view.rolls_used("blade") == 1, "everyone's first roll follows the reveal")
	var revealed: Array = screen._view.raw_snapshot.get("revealed_plays", [])
	_expect(revealed.any(func(play): return str(play.card_instance_id) == card and str(play.actor_id) == "blade"), "the reveal lists the secret play: %s" % [revealed])
	_expect(screen.find_child("SecretPlaysReveal", true, false) == null, "no pop-up lists the revealed plays")
	_expect(screen._secret_plays().is_empty(), "nothing is secret after the reveal")
	_expect(screen._view.raw_snapshot.get("simultaneous_step", "") == "offense_rolling", "rolling follows the reveal")
	# Rolling: cards only reach the player's own dice and statuses.
	for action in screen._view.legal_actions:
		var choice = JSON.parse_string(str(action.get("payload", {}).get("status_id", action.get("payload", {}).get("commitment", {}).get("choice_id", "{}"))))
		if choice is Dictionary and choice.has("actor") and str(choice.actor) != "":
			_expect(str(choice.actor) == "blade", "rolling offers only self-targeted plays: %s" % [choice])
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or opponent error")
	screen.queue_free(); await process_frame

func _choice(action: Dictionary) -> Dictionary:
	var choice = JSON.parse_string(str(action.get("payload", {}).get("commitment", {}).get("choice_id", "{}")))
	return choice if choice is Dictionary else {}

## Plays to the step between the attacks and the defenses (Offense After =
## Defense Before) after the player locked an attack in, holding a dice card.
func _between_with_die_card(runtime) -> Array:
	for candidate in range(1, 60):
		var gateway := _gateway(runtime)
		var result := _human_turn(gateway, gateway.start_battle("simultaneous-between-%d" % candidate, candidate))
		var chose_round := 0
		for step in 300:
			if result.get("accepted") != true or result.get("battle_result", "") != "" or int(result.snapshot.round) > 3: break
			var snap: Dictionary = result.snapshot
			var actions: Array = result.get("legal_actions", [])
			if snap.get("simultaneous_step", "") == "defense_before" and snap.get("segment", "") == "defensive" and chose_round == int(snap.round):
				var blade: Dictionary = snap.actors.blade
				for id in blade.get("hand", []):
					if str(blade.card_instances[id].definition_id) in ["nudge", "try_again"]: return [gateway, result, candidate, str(id)]
			var chosen: Dictionary = {}
			for kind in ["planning_roll", "planning_select_ability", "planning_pass", "pass", "roll_dice"]:
				if kind == "planning_select_ability" and chose_round == int(snap.round): continue
				for action in actions:
					if action.type == kind and str(action.get("payload", {}).get("source_id", "")).is_empty(): chosen = action; break
				if not chosen.is_empty(): break
			if chosen.is_empty(): break
			if chosen.type == "planning_select_ability": chose_round = int(snap.round)
			result = _human_turn(gateway, gateway.submit(JSON.stringify(chosen)))
	return []

## Your own attack is final once locked in: a dice card in hand is not offered
## on it, so it never holds up Defense.
func _between_by_pointer(runtime) -> void:
	var found := _between_with_die_card(runtime)
	_expect(not found.is_empty(), "a seed reaches Defense Before after an attack, holding a dice card")
	if found.is_empty(): return
	var gateway: RefCounted = found[0]; var result: Dictionary = found[1]; var card: String = found[3]
	_expect(result.snapshot.get("segment", "") == "defensive" and result.snapshot.get("simultaneous_step", "") == "defense_before", "the attacks lead straight into Defense Before, with no Offense After stop")
	var screen = await _screen(gateway, result, int(found[2]), "simultaneous-between.json")
	var definition := str(result.snapshot.actors.blade.card_instances[card].definition_id)
	_expect(not screen._card_legal(definition), "%s is not offered on your own locked-in attack" % definition)
	_expect(not screen._view.legal_actions.any(func(a): return str(a.get("payload", {}).get("commitment", {}).get("card_ids", [""])[0]) == card), "the authority offers no play of it either")
	# Lock in; at the reveal the enemy defends the player's attack on its own
	# station, above its name and health, not out on the battlefield.
	var lock := await _enabled(screen, "battle.command.planning_pass")
	if lock != null: await _click(lock)
	var deadline := Time.get_ticks_msec() + 20000
	var station: Control = null
	while Time.get_ticks_msec() < deadline and station == null and screen._view.round_number == int(result.snapshot.round):
		for presenter in screen._attack_intents.values():
			if str(presenter.data.actor_id) != "blade" and presenter.quick_roll() and presenter.roll_area.visible: station = presenter
		await process_frame
	if station != null:
		await create_timer(0.3).timeout
		station = null
		for presenter in screen._attack_intents.values():
			if str(presenter.data.actor_id) != "blade" and presenter.quick_roll() and presenter.roll_area.visible: station = presenter
	if station != null:
		var profile: Rect2 = screen._actor_profiles[str(station.data.actor_id)].get_global_rect()
		var die: Rect2 = station.dice_controls[0].get_global_rect()
		_expect(die.end.y <= profile.position.y + 1 and die.get_center().x >= profile.position.x - 60 and die.get_center().x <= profile.end.x + 60, "the enemy's defense rolls above its name and health: die %s, profile %s" % [die, profile])
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or opponent error")
	screen.queue_free(); await process_frame

## Plays to a Defense Before step with two incoming attacks on the player.
func _two_attack_defense(runtime) -> Array:
	for candidate in range(1, 60):
		var gateway := _gateway(runtime)
		var result := _human_turn(gateway, gateway.start_battle("simultaneous-defense-%d" % candidate, candidate))
		for step in 300:
			if result.get("accepted") != true or result.get("battle_result", "") != "" or int(result.snapshot.round) > 3: break
			var snap: Dictionary = result.snapshot
			if snap.get("simultaneous_step", "") == "defense_before":
				var incoming := 0
				for source in snap.get("damage_sources", []):
					if str(source.target_actor_id) == "blade" and int(source.final_amount) > 0: incoming += 1
				if incoming >= 2: return [gateway, result, candidate]
			var actions: Array = result.get("legal_actions", [])
			var chosen: Dictionary = {}
			for kind in ["planning_roll", "planning_pass", "pass", "roll_dice"]:
				for action in actions:
					if action.type == kind and str(action.get("payload", {}).get("source_id", "")).is_empty(): chosen = action; break
				if not chosen.is_empty(): break
			if chosen.is_empty(): break
			result = _human_turn(gateway, gateway.submit(JSON.stringify(chosen)))
	return []

func _defense_by_pointer(runtime) -> void:
	var found := _two_attack_defense(runtime)
	_expect(not found.is_empty(), "a seed reaches a Defense with two incoming attacks")
	if found.is_empty(): return
	var gateway: RefCounted = found[0]; var result: Dictionary = found[1]
	var screen = await _screen(gateway, result, int(found[2]), "simultaneous-defense.json")
	var sources: Array = []
	for source in screen._view.damage_sources:
		if str(source.target_actor_id) == "blade" and int(source.final_amount) > 0: sources.append(str(source.id))
	_expect(sources.size() >= 2 and sources.all(func(id): return screen._attack_intents.has(id)), "both incoming attacks are on the board")
	# Before any defense is chosen, each attack already shows the cards it threatens.
	for id in sources:
		var zones: Dictionary = screen._damage_zone_counts.get(id, {})
		var dock: Control = screen._damage_stack_docks.get("attack:" + id)
		_expect(zones.values().reduce(func(sum, n): return sum + int(n), 0) > 0 and is_instance_valid(dock) and dock.is_visible_in_tree(), "before any defense choice, %s shows its threatened cards: %s" % [id, zones])
		_expect(is_instance_valid(screen._incoming_attack_rows.get(id)), "%s is in the Incoming Attacks list" % id)
	_expect(screen.find_children("*", "Button", true, false).all(func(b): return b.get_meta("inspection_id", "") != "battle.command.roll_dice"), "there is no Roll Defenses button")
	# Guard on the first attack rolls at once; Guard+ on the second while the
	# first is still rolling. Both rolls are on screen together.
	var started := {}
	for pair in [[sources[0], "adventurer_guard"], [sources[1], "adventurer_guard_plus"]]:
		screen._select_attack_intent(pair[0])
		for frame in 3: await process_frame
		var tile := await _inspection(screen, "battle.ability.blade.%s" % pair[1])
		_expect(tile != null and not tile.disabled, "%s can defend %s right away" % [pair[1], pair[0]])
		if pair[1] == "adventurer_guard_plus":
			# Guard is spent for this Defense: the pop-up lists only what is left.
			var offered: Array = screen._ability_dock.find_children("*", "Button", true, false).filter(func(b): return str(b.get_meta("inspection_id", "")).begins_with("battle.ability.blade.") and b.is_visible_in_tree()).map(func(b): return str(b.get_meta("inspection_id", "")))
			_expect(offered == ["battle.ability.blade.adventurer_guard_plus"], "only the defenses still usable are offered: %s" % [offered])
		if tile == null: screen.queue_free(); return
		await _click(tile)
		for frame in 3: await process_frame
		var history: Dictionary = screen._view.raw_snapshot.get("defense_history", {}).get(pair[0], {})
		_expect(str(history.get("ability_id", "")) == pair[1] and not history.get("rolled_faces", []).is_empty(), "choosing %s rolls and resolves it at once: %s" % [pair[1], history])
		var presenter = screen._attack_intents.get(pair[0])
		_expect(is_instance_valid(presenter) and presenter.quick_roll() and presenter.roll_area.visible, "%s's dice roll on the battlefield" % pair[0])
		started[pair[0]] = Time.get_ticks_msec()
	var both_rolling: bool = sources.slice(0, 2).all(func(id): return is_instance_valid(screen._attack_intents.get(id)) and screen._attack_intents[id].roll_area.visible and Time.get_ticks_msec() - screen._attack_intents[id].started_ms < 1600)
	_expect(both_rolling, "both defenses animate at the same time")
	_expect(Time.get_ticks_msec() - int(started[sources[0]]) < 1600, "the second defense was chosen while the first was still rolling")
	# Prevention shows at once: each attack's damage drops by what its defense prevented.
	for id in sources.slice(0, 2):
		var history: Dictionary = screen._view.raw_snapshot.get("defense_history", {}).get(id, {})
		var source: Dictionary = screen._view.damage_sources.filter(func(s): return str(s.id) == id)[0]
		_expect(int(source.final_amount) == int(source.base_amount) - int(history.get("prevented", 0)) or int(source.final_amount) == 0, "%s shows its prevention: %s" % [id, source])
	# The dice come to rest, small, beside each attack's badge.
	var deadline := Time.get_ticks_msec() + 4000
	while Time.get_ticks_msec() < deadline and sources.slice(0, 2).any(func(id): return not _resting(screen, id)): await process_frame
	for id in sources.slice(0, 2):
		_expect(_resting(screen, id), "%s's dice rest above its badge" % id)
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or opponent error")
	await _quiet_opponent_feedback(screen, sources[0])
	await _second_guard_on_resting_dice(screen, sources[0])
	# Automatic completion: nothing left to choose, so the step locks in and
	# Defense finishes without a Pass.
	screen._auto_pass_disabled = false
	deadline = Time.get_ticks_msec() + 30000
	while Time.get_ticks_msec() < deadline and screen._view.round_number == 1 and not screen._view.is_complete():
		_expect(screen._view.segment != "damage_resolution", "no separate Damage segment")
		# With a card still playable the step waits for Pass; otherwise it completes itself.
		if screen._sole_pass_action().is_empty() and not screen._submitting and not screen._model_thinking and screen._view.legal_actions.any(func(a): return a.type == "commit_interaction") and Time.get_ticks_msec() > screen._quick_rolls_finished_at():
			var pass_button := await _enabled(screen, "battle.command.planning_pass")
			if pass_button != null: await _click(pass_button)
		await process_frame
	_expect(screen._view.round_number > 1 or screen._view.is_complete(), "Defense finishes once nothing is left to do")
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or opponent error at the end")
	screen.queue_free(); await process_frame

## An enemy's card effect shows only its result: the attack number pulses
## and counts up, the energy number pulses; no card and no caption appear.
func _quiet_opponent_feedback(screen, source_id: String) -> void:
	var presenter = screen._attack_intents.get(source_id)
	var attacker: String = presenter.attacker_id
	var ability := str(presenter.source.get("source_content_id", ""))
	var after := int(presenter.damage.text)
	var energy := int(screen._view.actor(attacker).get("energy_points", 0))
	var card := {"definition_id": "brine_surge", "instance_id": "quiet-surge", "actor_id": attacker}
	var updates: Array[Dictionary] = [
		{"kind": "attack_bonus", "data": {"target_actor_id": attacker, "ability_id": ability, "before": after - 1, "after": after, "amount": 1, "card_definition_id": "brine_surge", "card_instance_id": "quiet-surge"}, "card_feedback": {"key": "quiet", "cards": [card]}},
		{"kind": "resource", "data": {"target_actor_id": attacker, "stat": "energy", "caption": "Energy", "amount": 5, "before": maxi(0, energy - 5), "after": energy}, "card_feedback": {"key": "quiet", "cards": [card]}}]
	var notice = preload("res://presentation/battle/card_gain_notice.gd").new()
	notice.configure(screen, updates); screen.add_child(notice)
	var peak := 1.0
	var deadline := Time.get_ticks_msec() + 3000
	while Time.get_ticks_msec() < deadline and is_instance_valid(notice):
		_expect(notice.quiet and notice._cards.all(func(c): return not c.visible) and not notice._label.visible, "an enemy card shows no card and no caption")
		peak = maxf(peak, presenter.damage.scale.x)
		await process_frame
	_expect(not is_instance_valid(notice), "the quiet result is short")
	_expect(peak > 1.15, "the attack number pulses: peak scale %.2f" % peak)
	_expect(int(presenter.damage.text) == after, "and ends on the new total: %s" % presenter.damage.text)

func _resting(screen, id: String) -> bool:
	var presenter = screen._attack_intents.get(id)
	if not is_instance_valid(presenter) or presenter.roll_cells.is_empty() or not presenter.roll_area.visible: return false
	var badge: Rect2 = presenter.attack_damage_rect()
	var cell: Control = presenter.roll_cells[0]
	var die: Rect2 = presenter.dice_controls[0].get_global_rect()
	return absf(cell.scale.x - presenter.RESTING_SCALE) < 0.02 and die.end.y <= badge.position.y + 1 and die.end.y >= badge.position.y - 40 and absf(die.get_center().x - badge.get_center().x) < 120

## Second Guard targets a resting defense die by pointer and submits exactly
## that action (a fake authority built from the live result).
func _second_guard_on_resting_dice(live, source_id: String) -> void:
	var fixture: Dictionary = {"accepted": true, "snapshot": live._view.raw_snapshot.duplicate(true), "events": [], "legal_actions": [], "learned_policy": {}}
	var blade: Dictionary = fixture.snapshot.actors.blade
	blade.hand = ["sg-card"]; blade.hand_count = 1; blade.energy_points = 20
	blade.card_instances["sg-card"] = {"instance_id": "sg-card", "definition_id": "second_guard"}
	fixture.pending_input = {"blade": {"id": "sg-input", "segment": "defensive", "stage": fixture.snapshot.stage, "allowed_commands": ["commit_interaction", "planning_pass"]}}
	var dice: Array = fixture.snapshot.defense_history[source_id].rolled_faces
	fixture.legal_actions.append(_sg_action(fixture, {"verb": "start", "label": "Play card"}))
	for i in dice.size():
		fixture.legal_actions.append(_sg_action(fixture, {"verb": "start", "then": "target", "kind": "defensive_die", "actor": "blade", "die": i, "source": source_id, "label": "Adventurer · Guard die %d" % (i + 1)}))
	var fake := FakeBattleAuthority.new()
	var after := fixture.duplicate(true); after.legal_actions = []; after.snapshot.actors.blade.hand = []
	fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.viewer_actor_id = "blade"; screen.gateway = BattleGateway.new(fake)
	screen._auto_pass_disabled = true
	screen._quick_roll_clocks = live._quick_roll_clocks.duplicate()
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("simultaneous-second-guard.json"))
	live.hide()
	root.add_child(screen)
	for frame in 8: await process_frame
	await _settle(screen)
	var deadline := Time.get_ticks_msec() + 4000
	while Time.get_ticks_msec() < deadline and not screen._hand_dock.cards.any(func(c): return c.instance_id == "sg-card" and not c.disabled): await process_frame
	await _click_hand(screen, "sg-card")
	await _settle(screen)
	var targets: Array = screen.find_children("*", "Button", true, false).filter(func(b): return b.get_meta("defensive_die_target", false) and b.is_visible_in_tree() and not b.disabled)
	_expect(fake.commands.is_empty() and targets.size() == dice.size(), "Second Guard highlights the dice beside the attack: %d of %d" % [targets.size(), dice.size()])
	if not targets.is_empty():
		await _click(targets[0])
		for frame in 6: await process_frame
		_expect(fake.commands.size() == 1 and fake.commands[0] == JSON.stringify(fixture.legal_actions[1]), "clicking a resting die submits that die's action")
	screen.queue_free(); await process_frame
	live.show()

func _sg_action(fixture: Dictionary, choice: Dictionary) -> Dictionary:
	return {"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "sg-input", "commitment": {"card_ids": ["sg-card"], "choice_id": JSON.stringify(choice)}}}

func _settle(screen) -> void:
	for frame in 4: await process_frame
	var deadline := Time.get_ticks_msec() + 4000
	while Time.get_ticks_msec() < deadline and (screen._submitting or screen._model_thinking or screen._director.has_beats() or screen._player_roll_active()):
		if screen._director.has_beats(): screen._advance_beat()
		await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._render()
	for frame in 3: await process_frame

## Waits until a footer button unlocks (the threatened cards' reveal holds it).
func _enabled(screen: Node, id: String) -> Button:
	var deadline := Time.get_ticks_msec() + 8000
	while Time.get_ticks_msec() < deadline:
		for button in screen.find_children("*", "Button", true, false):
			if button.get_meta("inspection_id", "") == id and not button.is_queued_for_deletion() and button.is_visible_in_tree() and not button.disabled: return button
		await process_frame
	return null

func _inspection(screen: Node, id: String) -> Button:
	for attempt in 30:
		for button in screen.find_children("*", "Button", true, false):
			if button.get_meta("inspection_id", "") == id and not button.is_queued_for_deletion() and button.is_visible_in_tree():
				if not button.disabled: return button
		await process_frame
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == id and not button.is_queued_for_deletion(): return button
	return null

## Opens the hand as hovering does, then clicks the card itself.
func _click_hand(screen, id: String) -> void:
	var hand = screen._hand_dock
	hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == id: index = i
	_expect(index >= 0, "the card is in the live hand")
	if index < 0: return
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; root.push_input(click, true)
	for frame in 3: await process_frame

func _click(button: Button) -> void:
	var point := button.get_global_rect().get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	for frame in 3: await process_frame

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
