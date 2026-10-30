extends SceneTree
## Tactics Sandbox by pointer (real native authority, docs/game-design/
## tactics-sandbox-plan.md §11): the menu tab and setup; Sprint; every movement
## intention by clicking (a square, a creature → Pursue or Flee, Hold); a card
## and energy committed; lock in; the revealed paths during defense and the
## playback after the commit; targets chosen in range; running with only Evade
## on offer; resuming a saved battle; a won and a lost battle's end screen; and
## the board's badges, popups and tooltips inside the viewport for 1–4 enemies
## at both screen edges.

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const SETUP := preload("res://app/screens/tactics/tactics_setup.gd")
const SCREEN := preload("res://app/screens/tactics/tactics_screen.gd")
const BOARD := preload("res://app/screens/tactics/tactics_board.gd")
const SEED := 11
var failed := false
var runtime: Node

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	runtime = root.get_node("LearnedBattleRuntime")
	var screen = await _setup_by_pointer()
	if screen != null:
		await _sprint_and_intentions(screen)
		screen = _current_screen()
		await _attack_in_range(screen)
		screen = _current_screen()
		await _running_with_evade(screen)
		screen = _current_screen()
		await _resume(screen)
	await _win_and_lose()
	await _badges_do_not_overlap()
	await _layout(Vector2i(1280, 720))
	await _layout(Vector2i(1920, 1080))
	print("TACTICS SANDBOX: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

# --- Helpers ------------------------------------------------------------------

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
	else: print("  ok: " + message)

func _frames(n: int = 4) -> void:
	for i in n: await process_frame

func _current_screen():
	for node in root.get_children():
		if node.get_script() == SCREEN and not node.is_queued_for_deletion(): return node
	return null

func _find(node: Node, id: String) -> Control:
	for control in node.find_children("*", "Control", true, false):
		if control.get_meta("inspection_id", "") == id and not control.is_queued_for_deletion() and control.is_visible_in_tree(): return control
	return null

func _click_at(point: Vector2) -> void:
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed
		root.push_input(event, true); await process_frame
	await _frames(3)

func _click(control: Control) -> void:
	await _click_at(control.get_global_rect().get_center())

## Clicks a registered control by its inspection id; false when missing or disabled.
func _press(node: Node, id: String) -> bool:
	var control := _find(node, id)
	if control == null or (control is BaseButton and control.disabled): return false
	await _click(control)
	return true

func _plan(screen) -> Dictionary:
	var plan = screen._tv.get("plan")
	return plan if plan is Dictionary else {}

func _step(screen) -> String: return screen._step()

## Lets an auto-pass or playback finish before the next check.
func _settle(screen) -> void:
	await _frames(6)
	var deadline := Time.get_ticks_msec() + 3000
	while Time.get_ticks_msec() < deadline and (screen._busy or screen._actions.size() == 1 and _step(screen).begins_with("defense")): await process_frame

# --- Setup ----------------------------------------------------------------------

func _setup_by_pointer():
	var menu = BOOTSTRAP.instantiate(); root.add_child(menu)
	await _frames(6)
	var entry := _find(menu, "battle.setup.tactics")
	_expect(entry != null and not entry.disabled, "the main menu shows the Tactics Sandbox tab")
	if entry == null: return null
	await _click(entry)
	await _frames(6)
	var setup = null
	for node in root.get_children():
		if node.get_script() == SETUP: setup = node
	_expect(setup != null, "the tab opens the sandbox setup")
	if setup == null: return null
	_expect(setup.catalog.heroes.size() == 2 and setup.catalog.enemies.size() == 4 and setup.catalog.arenas.size() == 4, "the setup offers both heroes, four enemies and four arenas")
	_select(setup._hero, "starter_movement")
	_select(setup._enemies[0], "tactics_ribbon_eel")
	_select(setup._arena, "open_field")
	setup._seed.text = str(SEED)
	await _press(setup, "tactics.setup.start")
	await _frames(8)
	var screen = _current_screen()
	_expect(screen != null, "Start creates the battle and opens the board")
	if screen == null: return null
	var active: Dictionary = TacticsGateway.active_store().load_active()
	_expect(str(active.get("battle_id", "")) == str(screen._snap.get("battle_id")), "the sandbox keeps its own resume pointer")
	_expect(_step(screen) == "offense_before" and screen._tv.get("arena", {}).get("id") == "open_field", "round 1 opens on the secret Offense Before step on Open Field")
	_expect(_find(screen, "tactics.board.creature.hero") != null and _find(screen, "tactics.board.creature.enemy-1") != null, "the hero and the eel stand on the grid")
	return screen

func _select(option: OptionButton, id: String) -> void:
	for i in option.item_count:
		if str(option.get_item_metadata(i)) == id: option.select(i); option.item_selected.emit(i)

func _card_tag(card: Control) -> String:
	if card == null: return ""
	for child in card.get_children():
		if child is Label and "Move" in child.text: return child.text
	return ""

# --- Sprint and every intention ------------------------------------------------------

func _sprint_and_intentions(screen) -> void:
	var sprint := _find(screen, "tactics.command.sprint")
	_expect(sprint != null and sprint.text == "Sprint (skip attack)", "the skip control reads Sprint (skip attack)")
	await _click(sprint)
	await _settle(screen)
	_expect(_step(screen) == "movement", "after the offense reveal the movement step opens")
	_expect(_plan(screen).get("sprint", false) and _plan(screen).intention.kind == "hold", "the plan starts as Hold and knows it sprints")
	await _click(_find(screen, "tactics.board.square.4.4"))
	_expect(_plan(screen).intention.kind == "move_to" and int(_plan(screen).intention.square.x) == 4, "clicking a square plans Move to")
	_expect(int(_plan(screen).budget.total) == 2 and _plan(screen).budget.running, "a sprint doubles the base speed and always runs")
	_expect(_plan(screen).get("preview") is Dictionary and _plan(screen).preview.path.size() == 3, "the authority's preview path shows the planned steps")
	for kind in ["pursue", "flee"]:
		await _click(_find(screen, "tactics.board.creature.enemy-1"))
		_expect(await _press(screen, "tactics.intent.%s" % kind), "clicking the eel offers %s" % kind)
		await _frames(4)
		_expect(_plan(screen).intention.kind == kind and _plan(screen).intention.target == "enemy-1", "the plan is %s the eel" % kind)
	_expect(await _press(screen, "tactics.intent.hold"), "Hold is a button")
	_expect(_plan(screen).intention.kind == "hold", "the plan holds")
	await _click(_find(screen, "tactics.board.square.4.4"))
	var card: String = str(screen._actor("hero").hand[0])
	_expect(_card_tag(_find(screen, "tactics.card.%s" % card)) == "+2 Move", "a hand card shows the Move it adds while sprinting (+2)")
	await _click(_find(screen, "tactics.card.%s" % card))
	_expect(card in _plan(screen).get("cards", []), "clicking a hand card commits it")
	_expect(await _press(screen, "tactics.energy.more"), "+5 energy is offered")
	_expect(int(_plan(screen).get("energy", 0)) == 5 and int(_plan(screen).budget.total) == 2 + 2 + 1, "the budget is base ×2, +2 for the card while sprinting, +1 for 5 energy: %s" % _plan(screen).budget)
	_expect("RUNNING" in screen._budget.text, "the Running warning shows")
	var energy := int(screen._actor("hero").energy_points)
	_expect(await _press(screen, "tactics.command.lock_in"), "Lock in plan")
	await _frames(4)
	var movement = screen._tv.get("movement")
	_expect(movement is Dictionary and int(movement.round) == 1, "the movement resolved")
	_expect(card in screen._actor("hero").get("card_instances", {}) and not card in screen._actor("hero").get("hand", []), "the committed card left the hand")
	_expect(int(screen._actor("hero").energy_points) == energy - 5, "the committed energy was spent")
	_expect(_has_status(screen, "hero", "running"), "Running shows on the hero")
	_expect(_step(screen) == "defense_before" and not screen._board.ghosts.is_empty() and not screen._board.paths.is_empty(), "during defense tokens stay put with revealed paths and ghosts")
	_expect(screen._board.creatures.hero.square == screen._square(movement.start.hero), "during defense the hero's token stays on its starting square")
	await _to_next_round(screen)
	_expect(int(screen._snap.round) == 2, "round 2 begins")
	_expect(not screen._playback.is_empty(), "the playback starts after the final commit")
	var final_square: Vector2 = screen._square(screen._tv.creatures.hero.position)
	await _frames(20)
	var moving: Vector2 = screen._board.creatures.hero.square
	_expect(moving != final_square or screen._playback.is_empty(), "the hero's token moves along its path during the playback")
	var deadline := Time.get_ticks_msec() + 9000
	while Time.get_ticks_msec() < deadline and not screen._playback.is_empty(): await process_frame
	_expect(screen._playback.is_empty() and screen._board.creatures.hero.square == final_square, "the playback ends on the final squares")
	_expect(not _has_status(screen, "hero", "running"), "Running expired at the end of the round")
	_expect(await _press(screen, "tactics.playback.replay"), "Replay round")
	_expect(not screen._playback.is_empty(), "the round replays")
	_expect(await _press(screen, "tactics.timeline.toggle"), "the debug timeline opens")
	_expect("steps" in screen._timeline_text.text and "%" in screen._timeline_text.text, "the timeline lists steps with exact times and percentages")
	await _press(screen, "tactics.timeline.toggle")

func _has_status(screen, actor: String, id: String) -> bool:
	for status in screen._actor(actor).get("statuses", []):
		if str(status.definition_id) == id: return true
	return false

## Passes defenses until the next round's Offense (or the end).
func _to_next_round(screen) -> void:
	var round := int(screen._snap.get("round", 0))
	for i in 12:
		if int(screen._snap.get("round", 0)) > round or not str(screen._result.get("battle_result", "")).is_empty(): return
		if _step(screen).begins_with("defense"):
			if not await _press(screen, "tactics.command.pass"): await _settle(screen)
		else: await _settle(screen)
		await _settle(screen)

# --- Attacks in range ------------------------------------------------------------------

## Rolls until an attack is legal, checks that the board offers exactly the
## authority's legal targets, and aims it by clicking one.
func _attack_in_range(screen) -> void:
	var attacked := false
	for round in 8:
		if not str(screen._result.get("battle_result", "")).is_empty(): break
		await _press(screen, "tactics.command.roll")
		await _settle(screen)
		for reroll in 3:
			var tile := _first_enabled_ability(screen)
			if tile != null:
				var ability := str(tile.get_meta("inspection_id")).trim_prefix("tactics.ability.")
				await _click(tile)
				var tier := _first_with_prefix(screen, "tactics.tier.")
				if tier != null: await _click(tier)
				var legal_targets := {}
				for action in screen._ability_actions(ability, str(screen._aiming.get("tier", ""))):
					for id in action.payload.get("target_ids", []): legal_targets[str(id)] = true
				var highlighted: Dictionary = screen._board.clickable_creatures
				_expect(not legal_targets.is_empty() and legal_targets.keys().all(func(id): return highlighted.get(id, false)) and highlighted.keys().all(func(id): return legal_targets.has(id)), "aiming %s highlights exactly its legal targets in range: %s" % [ability, legal_targets.keys()])
				var target: String = legal_targets.keys()[0] if not legal_targets.is_empty() else ""
				if target.is_empty(): break
				await _click(_find(screen, "tactics.board.creature.%s" % target))
				await _frames(4)
				var found := false
				var revealed: Array = screen._tv.get("attacks", [])
				if screen._tv.get("movement") is Dictionary and int(screen._tv.movement.round) == int(screen._snap.round): revealed = revealed + screen._tv.movement.get("attacks", [])
				for attack in revealed:
					if str(attack.attacker) == "hero" and str(attack.get("target", "")) == target: found = true
				_expect(found, "the hero's %s on %s is revealed with its delivery" % [ability, target])
				attacked = true
				break
			if not await _press(screen, "tactics.command.reroll"): break
			await _settle(screen)
		if _step(screen) == "offense_rolling": await _press(screen, "tactics.command.pass"); await _settle(screen)
		if _step(screen) == "movement":
			await _click(_find(screen, "tactics.board.creature.enemy-1"))
			await _press(screen, "tactics.intent.pursue")
			await _press(screen, "tactics.command.lock_in")
			await _settle(screen)
		await _defend_first(screen)
		await _to_next_round(screen)
		if attacked: break
	_expect(attacked, "an attack was aimed at a target in range")

func _first_enabled_ability(screen) -> Control:
	for control in screen.find_children("*", "Button", true, false):
		if str(control.get_meta("inspection_id", "")).begins_with("tactics.ability.") and not control.disabled and control.is_visible_in_tree(): return control
	return null

func _first_with_prefix(screen, prefix: String) -> Control:
	for control in screen.find_children("*", "Button", true, false):
		if str(control.get_meta("inspection_id", "")).begins_with(prefix) and not control.disabled and control.is_visible_in_tree(): return control
	return null

func _defend_first(screen) -> void:
	if _step(screen) != "defense_before": return
	var defense := _first_with_prefix(screen, "tactics.defense.")
	if defense != null and not defense.text.begins_with("No"): await _click(defense); await _settle(screen)

# --- Running: only on-the-move defenses ---------------------------------------------------

func _running_with_evade(screen) -> void:
	var checked := false
	for round in 12:
		if not str(screen._result.get("battle_result", "")).is_empty(): break
		if _step(screen) == "offense_before": await _press(screen, "tactics.command.roll"); await _settle(screen)
		if _step(screen) == "offense_rolling":
			var tile := _first_enabled_ability(screen)
			if tile != null:
				var ability := str(tile.get_meta("inspection_id")).trim_prefix("tactics.ability.")
				if ability != "charge":
					await _click(tile)
					var tier := _first_with_prefix(screen, "tactics.tier.")
					if tier != null: await _click(tier)
					for id in screen._board.clickable_creatures: await _click(_find(screen, "tactics.board.creature.%s" % id)); break
					for square in screen._board.highlights: await _click(_find(screen, "tactics.board.square.%d.%d" % [square.x, square.y])); break
			if _step(screen) == "offense_rolling": await _press(screen, "tactics.command.pass")
			await _settle(screen)
		if _step(screen) != "movement": await _to_next_round(screen); continue
		var incoming := false
		for attack in screen._tv.get("attacks", []):
			if str(attack.get("target", "")) == "hero" and str(attack.delivery) == "direct": incoming = true
		if incoming:
			# Run away from a direct attack: it still lands, and the hero ran.
			await _click(_find(screen, "tactics.board.creature.enemy-1"))
			await _press(screen, "tactics.intent.flee")
			await _press(screen, "tactics.energy.more")
		else:
			await _click(_find(screen, "tactics.board.creature.enemy-1"))
			await _press(screen, "tactics.intent.pursue")
		await _press(screen, "tactics.command.lock_in")
		await _settle(screen)
		if incoming and _step(screen) == "defense_before" and _has_status(screen, "hero", "running"):
			var guard := _first_disabled(screen, ".adventurer_guard")
			var evade := _first_with_prefix_suffix(screen, "tactics.defense.", ".evade")
			_expect(guard != null and guard.disabled and "Running" in guard.tooltip_text, "while running Guard shows disabled with the reason")
			_expect(evade != null and not evade.disabled, "while running Evade is on offer")
			_expect(screen._tv.get("disabled_defenses", {}).has("adventurer_guard"), "the authority lists Guard as disabled")
			if evade != null:
				await _click(evade)
				await _settle(screen)
				var used := false
				for id in screen._snap.get("defense_history", {}):
					if str(screen._snap.defense_history[id].get("ability_id", "")) == "evade": used = true
				_expect(used, "Evade defended the attack")
			checked = true
		await _to_next_round(screen)
		if checked: break
	_expect(checked, "a round with Running and an incoming attack was defended with Evade")

func _first_disabled(screen, suffix: String) -> Button:
	for control in screen.find_children("*", "Button", true, false):
		var id := str(control.get_meta("inspection_id", ""))
		if id.begins_with("tactics.defense.") and id.ends_with(suffix) and control.is_visible_in_tree(): return control
	return null

func _first_with_prefix_suffix(screen, prefix: String, suffix: String) -> Button:
	for control in screen.find_children("*", "Button", true, false):
		var id := str(control.get_meta("inspection_id", ""))
		if id.begins_with(prefix) and id.ends_with(suffix) and control.is_visible_in_tree(): return control
	return null

# --- Resume -------------------------------------------------------------------------------

func _resume(screen) -> void:
	if screen == null: return
	var battle_id := str(screen._snap.get("battle_id", ""))
	var round := int(screen._snap.get("round", 0))
	var positions: Dictionary = screen._tv.get("creatures", {}).duplicate(true)
	var finished := not str(screen._result.get("battle_result", "")).is_empty()
	_expect(await _press(screen, "tactics.menu"), "Menu leaves the battle")
	await _frames(6)
	var menu = null
	for node in root.get_children():
		if node.has_method("_open_tactics") and not node.is_queued_for_deletion(): menu = node
	if menu != null: await _press(menu, "battle.setup.tactics")
	await _frames(6)
	var setup = null
	for node in root.get_children():
		if node.get_script() == SETUP: setup = node
	if finished:
		_expect(setup != null and _find(setup, "tactics.setup.resume") == null, "a finished battle offers no resume")
		return
	_expect(setup != null and await _press(setup, "tactics.setup.resume"), "the sandbox tab offers Resume")
	await _frames(8)
	var resumed = _current_screen()
	_expect(resumed != null and str(resumed._snap.get("battle_id")) == battle_id and int(resumed._snap.round) == round, "the saved battle resumes at the same round")
	if resumed != null:
		var same := true
		for id in positions:
			if resumed._tv.creatures[id].position != positions[id].position: same = false
		_expect(same, "every creature resumes on the same square")
		resumed.queue_free()

# --- Win and lose ---------------------------------------------------------------------------

func _win_and_lose() -> void:
	var outcomes := {}
	for attempt in 14:
		if outcomes.has("victory") and outcomes.has("defeat"): break
		var enemies := ["tactics_bell_diver"] if not outcomes.has("victory") else ["tactics_ribbon_eel", "tactics_bell_diver", "tactics_brine_mask", "tactics_drowned_regent"]
		var gateway := TacticsGateway.new(runtime)
		var view := gateway.start("tactics-end-%d" % attempt, {"hero": "adventurer_movement", "enemies": enemies, "arena": "open_field", "seed": 500 + attempt})
		_expect(view.get("accepted") == true, "a proxy battle starts: %s" % str(view.get("error", "")))
		if view.get("accepted") != true: return
		view = _play_out(gateway, view)
		var outcome := str(view.get("battle_result", ""))
		if outcome.is_empty() or outcomes.has(outcome): continue
		outcomes[outcome] = true
		var store := TacticsGateway.active_store(); store.save_active(gateway.battle_id, "hero")
		var screen = SCREEN.new(); screen.gateway = gateway; screen.store = store; screen.initial_result = view
		root.add_child(screen)
		await _frames(8)
		var title := _find(screen, "tactics.end.result")
		_expect(title != null and title.text == outcome.to_upper(), "a %s shows its end screen" % outcome)
		_expect(TacticsGateway.active_store().load_active().is_empty(), "a finished battle clears the resume pointer")
		_expect(await _press(screen, "tactics.end.new"), "New sandbox battle from the end screen")
		await _frames(6)
		var setup = null
		for node in root.get_children():
			if node.get_script() == SETUP and not node.is_queued_for_deletion(): setup = node
		_expect(setup != null, "it returns to the sandbox setup")
		if setup != null: setup.queue_free()
		await _frames(2)
	_expect(outcomes.has("victory") and outcomes.has("defeat"), "both a won and a lost battle were played: %s" % str(outcomes.keys()))

## A plain proxy through the authority's legal actions: roll, the attack with
## the most damage, pursue the nearest enemy, the first defense, then pass.
func _play_out(gateway: TacticsGateway, view: Dictionary) -> Dictionary:
	for i in 900:
		if view.get("accepted") != true or not str(view.get("battle_result", "")).is_empty(): return view
		var actions: Array = view.get("legal_actions", [])
		if actions.is_empty(): return view
		view = gateway.submit(_proxy_choice(view, actions))
	return view

func _proxy_choice(view: Dictionary, actions: Array) -> Dictionary:
	var by_type := {}
	for action in actions:
		var type := str(action.type)
		if not by_type.has(type): by_type[type] = []
		by_type[type].append(action)
	if by_type.has("planning_roll"): return by_type.planning_roll[0]
	var step := str(view.snapshot.get("simultaneous_step", ""))
	if by_type.has("planning_select_ability") and step != "defense_before":
		return by_type.planning_select_ability[0]
	if by_type.has("tactics_movement_plan"):
		var tactics: Dictionary = view.snapshot.tactics
		var me: Dictionary = tactics.creatures.hero
		var best := ""; var best_d := 999
		for id in tactics.creatures:
			if id == "hero" or tactics.creatures[id].get("defeated", false): continue
			var d: int = maxi(absi(int(tactics.creatures[id].position.x) - int(me.position.x)), absi(int(tactics.creatures[id].position.y) - int(me.position.y)))
			if d < best_d: best = id; best_d = d
		for action in by_type.tactics_movement_plan:
			var p: Dictionary = action.payload
			if not p.get("lock_in", false) and p.intention.kind == "pursue" and str(p.intention.get("target", "")) == best and p.get("card_ids", []).is_empty() and int(p.get("energy", 0)) == 0 and str(tactics.plan.intention.get("target", "")) != best:
				return action
		for action in by_type.tactics_movement_plan:
			if action.payload.get("lock_in", false): return action
	if by_type.has("planning_select_ability"): return by_type.planning_select_ability[0]
	if by_type.has("planning_reroll"): return by_type.planning_reroll[-1]
	for action in by_type.get("planning_pass", []):
		if str(action.payload.get("source_id", "")).is_empty(): return action
	if by_type.has("commit_interaction") and str(view.snapshot.get("stage", "")) == "discard_to_hand_limit": return by_type.commit_interaction[0]
	return actions[0]

# --- Badges on neighbours: an area on you, a direct attack on you, your attacks on them ---

func _badges_do_not_overlap() -> void:
	var board = BOARD.new(); board.size = Vector2(760, 570); root.add_child(board)
	var row := []
	for x in 12: row.append("floor")
	var rows := []
	for y in 9: rows.append(row)
	board.set_arena({"terrain": rows, "colors": {}, "names": {}, "costs": {}})
	var creature := func(square: Vector2, team: String, label: String) -> Dictionary:
		return {"square": square, "team": team, "label": label, "name": label, "viewer": team == "heroes", "defeated": false, "running": false, "hp": 10, "max_hp": 10, "tooltip": ""}
	board.creatures = {"hero": creature.call(Vector2(1, 1), "heroes", "AM"), "enemy-1": creature.call(Vector2(2, 1), "monsters", "DR1"), "enemy-2": creature.call(Vector2(2, 2), "monsters", "BD2")}
	board.badges = [
		{"id": "enemy-1.tide_crown", "text": "DR1 Tide Crown", "square": Vector2i(1, 1), "below": true},
		{"id": "enemy-1.crown_strike", "text": "DR1 Crown Strike", "target": "hero", "below": true},
		{"id": "hero.magic_missile", "text": "AM Magic Missile", "target": "enemy-1"},
		{"id": "hero.fireball", "text": "AM Fireball", "square": Vector2i(2, 1)},
		{"id": "hero.charge", "text": "AM Charge", "target": "enemy-2"},
		{"id": "enemy-2.sunken_toll", "text": "BD2 Sunken Toll", "target": "hero", "below": true},
	]
	board.refresh()
	await _frames(4)
	var rects: Array = board._badge_buttons.map(func(b): return b.get_global_rect())
	var clear := rects.size() == 6
	for i in rects.size():
		for j in range(i + 1, rects.size()):
			if rects[i].intersects(rects[j]): clear = false; print("    overlap: %s %s" % [rects[i], rects[j]])
	_expect(clear, "badges on neighbouring creatures (an area and two attacks on you, three of yours on them, one diagonally below) never overlap")
	board.queue_free()
	await _frames(2)

# --- Layout: 1–4 enemies, popups and tooltips inside the viewport ------------------------------

func _layout(size: Vector2i) -> void:
	root.size = size
	await _frames(4)
	var mixes := [["tactics_ribbon_eel"], ["tactics_ribbon_eel", "tactics_brine_mask"], ["tactics_bell_diver", "tactics_brine_mask", "tactics_drowned_regent"], ["tactics_ribbon_eel", "tactics_bell_diver", "tactics_brine_mask", "tactics_drowned_regent"]]
	for i in mixes.size():
		var gateway := TacticsGateway.new(runtime)
		var view := gateway.start("tactics-layout-%d-%d" % [size.x, i], {"hero": "starter_movement", "enemies": mixes[i], "arena": "pillar_hall", "seed": 77 + i})
		if view.get("accepted") != true: _expect(false, "layout battle starts"); continue
		# Sprint, so the movement step opens with every enemy's attack revealed.
		for action in view.legal_actions:
			if action.type == "planning_pass": view = gateway.submit(action); break
		var screen = SCREEN.new(); screen.gateway = gateway; screen.store = TacticsGateway.active_store(); screen.initial_result = view
		root.add_child(screen)
		await _frames(10)
		var viewport: Rect2 = screen.get_viewport_rect()
		var inside := true
		for control in screen.find_children("*", "Control", true, false):
			var id := str(control.get_meta("inspection_id", ""))
			if (id.begins_with("tactics.board.creature.") or id.begins_with("tactics.board.badge.")) and control.is_visible_in_tree() and not viewport.encloses(control.get_global_rect()):
				inside = false
				print("    outside: %s %s" % [id, control.get_global_rect()])
		_expect(inside, "%d enemies at %dx%d: every token and badge is inside the viewport" % [mixes[i].size(), size.x, size.y])
		# The creature popup at the left and right edges stays inside.
		var tokens: Array = []
		for control in screen.find_children("*", "Control", true, false):
			if str(control.get_meta("inspection_id", "")).begins_with("tactics.board.creature.enemy"): tokens.append(control)
		tokens.sort_custom(func(a, b): return a.global_position.x < b.global_position.x)
		for token in [tokens[0], tokens[-1]] if not tokens.is_empty() else []:
			if screen._step() != "movement": break
			await _click(token)
			var popup: Control = screen._popup
			_expect(popup.visible and viewport.encloses(popup.get_global_rect()), "the Pursue/Flee popup stays inside the viewport at x=%d" % int(token.global_position.x))
			screen._hide_popup()
		# The same popup opened at both screen edges is clamped inside.
		for edge in [Vector2(2, 2), viewport.size - Vector2(2, 2)]:
			var move := InputEventMouseMotion.new(); move.position = edge; root.push_input(move, true); await _frames(2)
			screen._show_popup("Edge check", [{"text": "Pursue", "id": "", "call": func(): pass}, {"text": "Flee from", "id": "", "call": func(): pass}])
			await _frames(2)
			_expect(viewport.encloses(screen._popup.get_global_rect()), "a popup opened at %s stays inside the viewport" % str(edge))
			screen._hide_popup()
		for control in screen.find_children("*", "Button", true, false):
			var id := str(control.get_meta("inspection_id", ""))
			if not (id.begins_with("tactics.board.badge.") or id.begins_with("tactics.board.creature.")) or control.tooltip_text.is_empty(): continue
			var tip = control._make_custom_tooltip(control.tooltip_text)
			_expect(tip != null and tip.custom_minimum_size.x <= viewport.size.x - 32, "the %s hover wraps inside the viewport" % id)
			if tip != null: tip.free()
			break
		screen.queue_free()
		await _frames(2)
	root.size = Vector2i(1280, 720)
