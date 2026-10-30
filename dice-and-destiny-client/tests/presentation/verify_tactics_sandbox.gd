extends SceneTree
## Tactics Sandbox by pointer (real native authority, docs/game-design/
## tactics-sandbox-plan.md §11). The sandbox plays on the battle screen with the
## grid in the battlefield's place: the menu tab and setup; the battle's own
## HUD (dice, ability rail, hand, stats, enemy stats at the upper right,
## utilities); Sprint; every movement intention by clicking (a square, an enemy
## → Pursue or Flee, Hold); a card and energy committed, with each card's Move;
## lock in; the revealed paths during defense and the playback after the
## commit; targets chosen in range on the grid; running with only Evade on
## offer; dragging and zooming the grid; resuming a saved battle; a won and a
## lost battle's end; a defense card choosing between two attack badges; and
## tokens, enemy stats and popups inside the viewport for 1–4 enemies.

const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const SETUP := preload("res://app/screens/tactics/tactics_setup.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const BATTLE_SCREEN_SCRIPT := preload("res://app/screens/battle/battle_screen.gd")
const SEED := 11
const ME := "blade"
var failed := false
var runtime: Node

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	runtime = root.get_node("LearnedBattleRuntime")
	var screen = await _setup_by_pointer()
	if screen != null:
		await _battle_hud(screen)
		await _sprint_and_intentions(screen)
		screen = _current_screen()
		await _attack_in_range(screen)
		screen = _current_screen()
		await _running_with_evade(screen)
		screen = _current_screen()
		await _drag_and_zoom(screen)
		screen = _current_screen()
		await _resume(screen)
	await _win_and_lose()
	await _area_aim()
	await _card_picks_its_attack()
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
		if node.get_script() == BATTLE_SCREEN_SCRIPT and node.tactics_mode and not node.is_queued_for_deletion(): return node
	return null

func _open(gateway: TacticsGateway, view: Dictionary):
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = view; screen.viewer_actor_id = ME; screen.gateway = gateway
	screen.active_store = TacticsGateway.active_store(); screen.tactics_mode = true
	root.add_child(screen)
	await _frames(10)
	return screen

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

## Clicks a registered control once it is visible and enabled (the battle
## screen holds buttons through its review times); false if it never is.
func _press(node: Node, id: String, wait_ms: int = 6000) -> bool:
	var deadline := Time.get_ticks_msec() + wait_ms
	while Time.get_ticks_msec() < deadline:
		var control := _find(node, id)
		if control != null and not (control is BaseButton and control.disabled):
			await _click(control)
			return true
		await process_frame
	return false

func _square(screen, x: int, y: int) -> void:
	var mark := _find(screen, "battle.tactics.square.%d.%d" % [x, y])
	if mark != null: await _click(mark)

## Opens the hand as hovering does, then clicks the card itself.
func _click_hand(screen, id: String) -> void:
	var hand = screen._hand_dock
	hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == id: index = i
	if index < 0: return
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	await _click_at(point)

func _tv(screen) -> Dictionary: return screen._tactics.tv()

func _plan(screen) -> Dictionary:
	var plan = _tv(screen).get("plan")
	return plan if plan is Dictionary else {}

func _step(screen) -> String: return "movement" if screen._tactics.in_movement() else screen._simultaneous_step()

func _over(screen) -> bool: return screen._view.is_complete()

## Lets submissions, rolls and presentation beats finish before the next check.
func _settle(screen) -> void:
	await _frames(4)
	var deadline := Time.get_ticks_msec() + 5000
	while Time.get_ticks_msec() < deadline and (screen._submitting or screen._director.has_beats() or screen._player_roll_active()):
		if screen._director.has_beats(): screen._advance_beat()
		await process_frame
	screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._render()
	await _frames(3)

func _has_status(screen, actor: String, id: String) -> bool:
	for status in screen._view.actor(actor).get("statuses", []):
		if str(status.definition_id) == id: return true
	return false

func _hand(screen) -> Array: return screen._view.hand_cards().map(func(entry): return str(entry.instance_id))

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
	await _frames(10)
	var screen = _current_screen()
	_expect(screen != null, "Start opens the battle screen with the grid")
	if screen == null: return null
	var active: Dictionary = TacticsGateway.active_store().load_active()
	_expect(str(active.get("battle_id", "")) == screen._view.battle_id, "the sandbox keeps its own resume pointer")
	_expect(_step(screen) == "offense_before" and _tv(screen).get("arena", {}).get("id") == "open_field", "round 1 opens on the secret Offense Before step on Open Field")
	_expect(_find(screen, "battle.tactics.creature.%s" % ME) != null and _find(screen, "battle.tactics.creature.enemy-1") != null, "the hero and the eel stand on the grid")
	return screen

func _select(option: OptionButton, id: String) -> void:
	for i in option.item_count:
		if str(option.get_item_metadata(i)) == id: option.select(i); option.item_selected.emit(i)

# --- The battle screen's own HUD ---------------------------------------------------------

func _battle_hud(screen) -> void:
	var away := InputEventMouseMotion.new(); away.position = Vector2(root.size) * Vector2(0.5, 0.2); root.push_input(away, true)
	await _settle(screen)
	await _frames(30)
	var root_rect: Rect2 = screen._root.get_global_rect()
	var design := func(control: Control) -> Rect2: return screen._root.get_global_transform_with_canvas().affine_inverse() * control.get_global_rect()
	_expect(screen.get_node_or_null("CinematicBattle/BattleScenery") != null, "the battle's scenery is the backdrop")
	_expect(screen._root.get_node_or_null("TacticsBattlefield") != null, "the grid takes the battlefield's place")
	_expect(_find(screen, "battle.command.planning_roll") != null and _find(screen, "battle.command.planning_pass") != null, "Roll and the skip control sit in the battle's roll dock")
	var sprint := _find(screen, "battle.command.planning_pass")
	_expect(sprint != null and sprint.get_node_or_null("CompactCaption") != null and sprint.get_node("CompactCaption").text == "Sprint", "skipping offense reads Sprint")
	for ability in ["adventurer_strike", "magic_missile", "fireball", "charge"]:
		_expect(_find(screen, "battle.ability.%s.%s" % [ME, ability]) != null, "the ability rail lists %s" % ability)
	var hand_cards: Array = screen._hand_dock.cards
	_expect(hand_cards.size() == _hand(screen).size() and hand_cards.size() > 0, "the hand is the battle's fanned hand (%d cards)" % hand_cards.size())
	_expect(screen._hand_dock.reveal < 0.5, "the hand starts lowered")
	var card_rect: Rect2 = design.call(hand_cards[0])
	# Hover the lowered cards' visible top edge, as a player does.
	var dock: Control = screen._hand_dock
	var hover := InputEventMouseMotion.new(); hover.position = dock.get_global_transform_with_canvas() * (dock.size * Vector2(0.5, 0.7)); root.push_input(hover, true)
	await _frames(40)
	_expect(screen._hand_dock.reveal > 0.5, "hovering the hand pulls it up")
	var mouse := InputEventMouseMotion.new(); mouse.position = Vector2(root.size) * Vector2(0.5, 0.3); root.push_input(mouse, true); await _frames(30)
	_expect(card_rect.size.x > 150, "cards keep the battle's size (%d wide)" % int(card_rect.size.x))
	var player: Rect2 = design.call(screen._player_profile_dock)
	_expect(player.position.x > 1400 and player.end.y > 900, "your stats stay at the lower right")
	var enemy = screen._actor_profiles.get("enemy-1")
	var enemy_rect: Rect2 = design.call(enemy) if enemy != null else Rect2()
	_expect(enemy != null and enemy_rect.position.x > 1500 and enemy_rect.position.y < 300, "the enemy's stats stand at the upper right: %s" % enemy_rect)
	for utility in ["enemy", "log", "timeline", "inspect", "settings"]:
		_expect(_find(screen, "battle.utility.%s" % utility) != null, "the %s utility is in the top bar" % utility)
	_expect(root_rect.size.x > 0, "the battle root is laid out")

# --- Sprint and every intention ------------------------------------------------------

func _sprint_and_intentions(screen) -> void:
	await _settle(screen)
	await _press(screen, "battle.command.planning_pass")
	await _frames(6)
	var dialog = screen.get_node_or_null("SkipUnrolledOffense")
	_expect(dialog != null and dialog.title == "Sprint this round?" and "Sprint" in dialog.get_ok_button().text, "Sprint asks before giving up the attack")
	# The dialog is an embedded window: its buttons sit at the window's offset.
	if dialog != null: await _click_at(Vector2(dialog.position) + dialog.get_ok_button().get_global_rect().get_center())
	await _settle(screen)
	_expect(_step(screen) == "movement", "after the offense reveal the movement step opens")
	_expect(_plan(screen).get("sprint", false) and _plan(screen).intention.kind == "hold", "the plan starts as Hold and knows it sprints")
	_expect(_find(screen, "battle.tactics.plan") != null and _find(screen, "battle.command.tactics_movement_plan") != null, "the movement plan sits in the ability rail with Lock in the roll dock")
	await _square(screen, 4, 4)
	_expect(_plan(screen).intention.kind == "move_to" and int(_plan(screen).intention.square.x) == 4, "clicking a square plans Move to")
	_expect(int(_plan(screen).budget.total) == 2 and _plan(screen).budget.running, "a sprint doubles the base speed and always runs")
	_expect(_plan(screen).get("preview") is Dictionary and _plan(screen).preview.path.size() == 3, "the authority's preview path shows the planned steps")
	_expect(not screen._tactics.board.paths.is_empty(), "the preview path is drawn on the grid")
	for kind in ["pursue", "flee"]:
		await _click(_find(screen, "battle.tactics.creature.enemy-1"))
		_expect(await _press(screen, "battle.tactics.intent.%s" % kind), "clicking the eel offers %s" % kind)
		await _settle(screen)
		_expect(_plan(screen).intention.kind == kind and _plan(screen).intention.target == "enemy-1", "the plan is %s the eel" % kind)
	_expect(await _press(screen, "battle.tactics.hold"), "Hold is a button")
	await _settle(screen)
	_expect(_plan(screen).intention.kind == "hold", "the plan holds")
	await _square(screen, 4, 4)
	await _settle(screen)
	var card: String = _hand(screen)[0]
	var tag: Label = null
	for hand_card in screen._hand_dock.cards:
		if hand_card.instance_id == card: tag = hand_card.get_node_or_null("MoveTag")
	_expect(tag != null and tag.text == "+2 Move", "a hand card shows the Move it adds while sprinting (+2)")
	await _click_hand(screen, card)
	await _settle(screen)
	_expect(card in _plan(screen).get("cards", []), "clicking a hand card commits it")
	_expect(await _press(screen, "battle.tactics.energy.more"), "+5 energy is offered")
	await _settle(screen)
	_expect(int(_plan(screen).get("energy", 0)) == 5 and int(_plan(screen).budget.total) == 2 + 2 + 1, "the budget is base ×2, +2 for the card while sprinting, +1 for 5 energy: %s" % _plan(screen).budget)
	_expect("RUNNING" in screen._tactics.plan_text(), "the Running warning shows")
	var energy := int(screen._view.actor(ME).energy_points)
	_expect(await _press(screen, "battle.command.tactics_movement_plan"), "Lock in")
	# Check the defense view before the empty defense completes on its own.
	var resolved := Time.get_ticks_msec() + 5000
	while Time.get_ticks_msec() < resolved and not (_tv(screen).get("movement") is Dictionary and _step(screen) == "defense_before"): await process_frame
	await _frames(2)
	var movement = _tv(screen).get("movement")
	_expect(movement is Dictionary and int(movement.round) == 1, "the movement resolved")
	_expect(not card in _hand(screen), "the committed card left the hand")
	_expect(int(screen._view.actor(ME).energy_points) == energy - 5, "the committed energy was spent")
	_expect(_has_status(screen, ME, "running"), "Running shows on your stats")
	var board = screen._tactics.board
	_expect(_step(screen) == "defense_before" and not board.ghosts.is_empty() and not board.paths.is_empty(), "during defense tokens stay put with revealed paths and ghosts")
	_expect(board.creatures[ME].square == screen._tactics.square(movement.start[ME]), "during defense your token stays on its starting square")
	await _to_next_round(screen)
	_expect(screen._view.round_number == 2, "round 2 begins")
	_expect(not screen._tactics.playback.is_empty(), "the playback starts after the final commit")
	var final_square: Vector2 = screen._tactics.square(_tv(screen).creatures[ME].position)
	screen._tactics.start_playback(_tv(screen).movement)
	await _frames(12)
	_expect(screen._tactics.board.creatures[ME].square != final_square, "your token moves along its path during the playback")
	var deadline := Time.get_ticks_msec() + 9000
	while Time.get_ticks_msec() < deadline and not screen._tactics.playback.is_empty(): await process_frame
	_expect(screen._tactics.playback.is_empty() and screen._tactics.board.creatures[ME].square == final_square, "the playback ends on the final squares")
	_expect(not _has_status(screen, ME, "running"), "Running expired at the end of the round")
	_expect(await _press(screen, "battle.utility.timeline"), "the Timeline utility opens")
	var timeline := _find(screen, "battle.tactics.timeline") as RichTextLabel
	_expect(timeline != null and "steps" in timeline.text and "%" in timeline.text, "the timeline lists steps with exact times and percentages")
	_expect(await _press(screen, "battle.tactics.replay"), "Replay round")
	_expect(not screen._tactics.playback.is_empty() and not screen._utility_panels.timeline.visible, "the round replays on the open grid")
	var wait := Time.get_ticks_msec() + 9000
	while Time.get_ticks_msec() < wait and not screen._tactics.playback.is_empty(): await process_frame

## Passes defenses until the next round's Offense (or the end).
func _to_next_round(screen) -> void:
	var round: int = screen._view.round_number
	for i in 16:
		if screen._view.round_number > round or _over(screen): return
		if _step(screen).begins_with("defense") or screen._view.stage == "discard_to_hand_limit":
			if screen._view.stage == "discard_to_hand_limit":
				await _discard_to_limit(screen)
			elif not await _press(screen, "battle.command.planning_pass", 2500): await _settle(screen)
		await _settle(screen)

func _discard_to_limit(screen) -> void:
	var need: int = maxi(0, int(screen._view.actor(ME).get("hand_count", 0)) - int(screen._view.actor(ME).get("max_hand_size", 6)))
	for card in _hand(screen).slice(0, need):
		await _click_hand(screen, card)
	await _press(screen, "battle.hand_limit.commit", 2000)

# --- Attacks in range ------------------------------------------------------------------

func _ability_button(screen) -> Control:
	var tiles: Array = []
	for control in screen.find_children("*", "Button", true, false):
		var id := str(control.get_meta("inspection_id", ""))
		if id.begins_with("battle.ability.%s." % ME) and not control.disabled and control.is_visible_in_tree() and not control.is_queued_for_deletion(): tiles.append(control)
	# A tier or option button chooses directly; a plain tile otherwise.
	for control in tiles:
		if str(control.get_meta("inspection_id")).count(".") >= 4: return control
	for control in tiles:
		var ability := str(control.get_meta("inspection_id")).trim_prefix("battle.ability.%s." % ME)
		if BattlePresentationCatalog.inline_tiers(ability, screen._view.actor(ME)).is_empty(): return control
	return null

## Rolls until an attack is legal, checks that the grid offers exactly the
## authority's legal targets, and aims it by clicking one.
func _attack_in_range(screen) -> void:
	var attacked := false
	for round in 8:
		if _over(screen): break
		await _press(screen, "battle.command.planning_roll", 3000)
		await _settle(screen)
		for reroll in 3:
			var button := _ability_button(screen)
			if button != null:
				await _click(button)
				await _settle(screen)
				var legal := {}
				for id in screen._offensive_target_actions(): legal[str(id)] = true
				var squares: Array = screen._tactics.aim_actions()
				var highlighted: Dictionary = screen._tactics.board.clickable_creatures
				if not squares.is_empty():
					_expect(screen._tactics.board.highlights.size() == squares.size(), "an area attack lights exactly its legal squares (%d)" % squares.size())
					var target = squares[0].payload.target_square
					await _square(screen, int(target.x), int(target.y))
				else:
					_expect(not legal.is_empty() and legal.keys().all(func(id): return highlighted.get(id, false)) and highlighted.keys().all(func(id): return legal.has(id)), "aiming highlights exactly the legal targets in range: %s" % [legal.keys()])
					if legal.is_empty(): break
					var target: String = legal.keys()[0]
					_expect(await _press(screen, "battle.offensive_target.%s" % target), "the target is clicked on its token")
				await _settle(screen)
				var mine := false
				for attack in _tv(screen).get("attacks", []):
					if str(attack.attacker) == ME: mine = true
				_expect(mine, "your attack is revealed with its delivery")
				attacked = true
				break
			if not await _press(screen, "battle.command.planning_reroll", 1500): break
			await _settle(screen)
		if _step(screen) == "offense_rolling": await _press(screen, "battle.command.planning_pass", 2000); await _settle(screen)
		if _step(screen) == "movement":
			await _click(_find(screen, "battle.tactics.creature.enemy-1"))
			await _press(screen, "battle.tactics.intent.pursue", 1500)
			await _settle(screen)
			await _press(screen, "battle.command.tactics_movement_plan")
			await _settle(screen)
		await _to_next_round(screen)
		if attacked: break
	_expect(attacked, "an attack was aimed at a target in range")

# --- Running: only on-the-move defenses ---------------------------------------------------

func _running_with_evade(screen) -> void:
	var checked := false
	for round in 14:
		if _over(screen): break
		if _step(screen) == "offense_before": await _press(screen, "battle.command.planning_roll", 3000); await _settle(screen)
		if _step(screen) == "offense_rolling":
			await _press(screen, "battle.command.planning_pass", 2000)
			await _settle(screen)
		if _step(screen) != "movement": await _to_next_round(screen); continue
		var incoming := false
		for attack in _tv(screen).get("attacks", []):
			if str(attack.get("target", "")) == ME and str(attack.delivery) == "direct": incoming = true
		await _click(_find(screen, "battle.tactics.creature.enemy-1"))
		# Run away from a direct attack: it still lands, and you ran.
		await _press(screen, "battle.tactics.intent.flee" if incoming else "battle.tactics.intent.pursue", 1500)
		await _settle(screen)
		if incoming: await _press(screen, "battle.tactics.energy.more", 1500); await _settle(screen)
		await _press(screen, "battle.command.tactics_movement_plan")
		await _settle(screen)
		if incoming and _step(screen) == "defense_before" and _has_status(screen, ME, "running"):
			var source := ""
			for candidate in screen._view.damage_sources:
				if str(candidate.target_actor_id) == ME: source = str(candidate.id)
			_expect(not source.is_empty() and screen._incoming_attack_rows.has(source), "the direct attack is in Incoming Attacks")
			screen._select_attack_intent(source)
			await _frames(4)
			var guard := _find(screen, "battle.ability.%s.adventurer_guard" % ME) as Button
			var evade := _find(screen, "battle.ability.%s.evade" % ME) as Button
			_expect(guard == null or guard.disabled, "while running Guard cannot defend")
			_expect(guard == null or "Running" in guard.tooltip_text, "Guard's hover says why")
			_expect(evade != null and not evade.disabled, "while running Evade is on offer")
			_expect(_tv(screen).get("disabled_defenses", {}).has("adventurer_guard"), "the authority lists Guard as disabled")
			if evade != null:
				await _click(evade)
				await _settle(screen)
				var used := false
				for id in screen._view.raw_snapshot.get("defense_history", {}):
					if str(screen._view.raw_snapshot.defense_history[id].get("ability_id", "")) == "evade": used = true
				_expect(used, "Evade defended the attack")
			checked = true
		await _to_next_round(screen)
		if checked: break
	_expect(checked, "a round with Running and an incoming attack was defended with Evade")

# --- Drag and zoom ----------------------------------------------------------------------

func _drag_and_zoom(screen) -> void:
	if screen == null or _over(screen): return
	await _settle(screen)
	var cell: float = screen._tactics.board.cell_size()
	var mark := _find(screen, "battle.tactics.square.5.4")
	if mark == null: return
	var start := mark.get_global_rect().get_center()
	var pan_before: Vector2 = screen._tactics.pan
	var plan_before := _plan(screen).duplicate(true)
	var move := InputEventMouseMotion.new(); move.position = start; root.push_input(move, true); await process_frame
	var press := InputEventMouseButton.new(); press.position = start; press.button_index = MOUSE_BUTTON_LEFT; press.pressed = true; root.push_input(press, true); await process_frame
	for i in 6:
		var drag := InputEventMouseMotion.new(); drag.position = start + Vector2(15 * (i + 1), 8 * (i + 1)); drag.relative = Vector2(15, 8); drag.button_mask = MOUSE_BUTTON_MASK_LEFT
		root.push_input(drag, true); await process_frame
	var release := InputEventMouseButton.new(); release.position = start + Vector2(90, 48); release.button_index = MOUSE_BUTTON_LEFT; release.pressed = false; root.push_input(release, true)
	await _frames(4)
	_expect(screen._tactics.pan.distance_to(pan_before) > 50, "dragging the grid moves it (%s)" % str(screen._tactics.pan - pan_before))
	_expect(_plan(screen) == plan_before, "a drag is not a click")
	var zoom: float = screen._tactics.zoom
	var wheel := InputEventMouseButton.new(); wheel.position = start; wheel.button_index = MOUSE_BUTTON_WHEEL_UP; wheel.pressed = true; root.push_input(wheel, true)
	await _frames(3)
	_expect(screen._tactics.zoom > zoom, "the wheel zooms the grid")
	screen._render(true); await _frames(3)
	_expect(screen._tactics.board.cell_size() > cell and not screen._tactics.pan.is_zero_approx(), "pan and zoom survive the screen's rebuild")
	screen._tactics.pan = Vector2.ZERO; screen._tactics.zoom = 1.0; screen._render(true); await _frames(2)

# --- Resume -------------------------------------------------------------------------------

func _resume(screen) -> void:
	if screen == null: return
	var battle_id: String = screen._view.battle_id
	var round: int = screen._view.round_number
	var positions: Dictionary = _tv(screen).get("creatures", {}).duplicate(true)
	var finished := _over(screen)
	if finished: return
	_expect(await _press(screen, "battle.utility.settings") and await _press(screen, "battle.tactics.menu"), "Settings → Leave to the main menu")
	await _frames(6)
	var menu = null
	for node in root.get_children():
		if node.has_method("_open_tactics") and not node.is_queued_for_deletion(): menu = node
	if menu != null: await _press(menu, "battle.setup.tactics")
	await _frames(6)
	var setup = null
	for node in root.get_children():
		if node.get_script() == SETUP and not node.is_queued_for_deletion(): setup = node
	_expect(setup != null and await _press(setup, "tactics.setup.resume"), "the sandbox tab offers Resume")
	await _frames(10)
	var resumed = _current_screen()
	_expect(resumed != null and resumed._view.battle_id == battle_id and resumed._view.round_number == round, "the saved battle resumes at the same round on the battle screen")
	if resumed != null:
		var same := true
		for id in positions:
			if _tv(resumed).creatures[id].position != positions[id].position: same = false
		_expect(same, "every creature resumes on the same square")
		resumed.queue_free()
		await _frames(2)

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
		TacticsGateway.active_store().save_active(gateway.battle_id, ME)
		var screen = await _open(gateway, view)
		for i in 20:
			if not screen._director.has_beats(): break
			screen._advance_beat(); await _frames(2)
		await _frames(6)
		_expect(screen._view.is_complete() and screen._view.battle_result == outcome, "a %s ends on the battle screen" % outcome)
		_expect(TacticsGateway.active_store().load_active().is_empty(), "a finished battle clears the resume pointer")
		# The last round plays first, with the end panel stepped aside.
		var played := Time.get_ticks_msec() + 15000
		while Time.get_ticks_msec() < played and not screen._tactics.playback.is_empty(): await process_frame
		await _frames(4)
		_expect(_find(screen, "battle.complete.tactics_replay_all") != null and _find(screen, "battle.complete.tactics_replay") != null and _find(screen, "battle.complete.tactics_menu") != null, "after the last round plays, the end offers Replay whole battle, Replay last round and Main menu")
		await _whole_replay(screen, gateway)
		_expect(await _press(screen, "battle.complete.tactics_new"), "New sandbox battle from the end")
		await _frames(6)
		var setup = null
		for node in root.get_children():
			if node.get_script() == SETUP and not node.is_queued_for_deletion(): setup = node
		_expect(setup != null, "it returns to the sandbox setup")
		if setup != null: setup.queue_free()
		await _frames(2)
	_expect(outcomes.has("victory") and outcomes.has("defeat"), "both a won and a lost battle were played: %s" % str(outcomes.keys()))

## Replay whole battle plays every resolved round in order on a clear grid,
## then the end of battle comes back; Stop ends it at once.
func _whole_replay(screen, gateway: TacticsGateway) -> void:
	var deadline := Time.get_ticks_msec() + 15000
	while Time.get_ticks_msec() < deadline and not screen._tactics.playback.is_empty(): await process_frame
	var rounds: Array = gateway.history().get("rounds", [])
	_expect(rounds.size() == screen._view.round_number and rounds.size() > 1, "the authority kept every round of the battle (%d)" % rounds.size())
	_expect(await _press(screen, "battle.complete.tactics_replay_all"), "Replay whole battle")
	await _frames(3)
	_expect(screen._tactics._rounds_total == rounds.size() and int(screen._tactics.playback.movement.round) == 1, "the replay starts from round 1 of %d" % rounds.size())
	_expect(not screen._center_scroll.visible and _find(screen, "battle.tactics.stop_replay") != null, "the end panel steps aside and Stop replay shows")
	_expect(screen._tactics.board.banner.begins_with("Round 1 · 1 of %d" % rounds.size()), "the grid shows the round: %s" % screen._tactics.board.banner)
	screen._tactics.speed = 4.0
	var seen: Array = []
	deadline = Time.get_ticks_msec() + 4000 * rounds.size()
	while Time.get_ticks_msec() < deadline and not screen._tactics.playback.is_empty():
		var round := int(screen._tactics.playback.movement.round)
		if seen.is_empty() or seen[-1] != round: seen.append(round)
		await process_frame
	_expect(seen == range(1, rounds.size() + 1), "every round played in order: %s" % [seen])
	await _frames(4)
	_expect(screen._tactics.playback.is_empty() and _find(screen, "battle.complete.tactics_new") != null and screen._center_scroll.visible, "the end of battle comes back after the replay")
	_expect(await _press(screen, "battle.complete.tactics_replay_all"), "Replay whole battle again")
	await _frames(3)
	_expect(await _press(screen, "battle.tactics.stop_replay"), "Stop replay")
	await _frames(4)
	_expect(screen._tactics.playback.is_empty() and _find(screen, "battle.complete.tactics_new") != null, "Stop ends the replay and shows the end of battle")
	screen._tactics.speed = 1.0

## A plain proxy through the authority's legal actions: roll, the first attack,
## pursue the nearest enemy, the first defense, then pass.
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
		var me: Dictionary = tactics.creatures[ME]
		var best := ""; var best_d := 999
		for id in tactics.creatures:
			if id == ME or tactics.creatures[id].get("defeated", false): continue
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

# --- An area attack aims at a square ------------------------------------------------

## Fireball is legal and can be centred on an enemy's own square.
func _fireball_ready(view: Dictionary) -> bool:
	var enemy: Dictionary = view.snapshot.tactics.creatures.get("enemy-1", {})
	return enemy.has("position") and view.get("legal_actions", []).any(func(a): return a.type == "planning_select_ability" and str(a.payload.get("ability_id", "")) == "fireball" and a.payload.get("target_square") is Dictionary and int(a.payload.target_square.x) == int(enemy.position.x) and int(a.payload.target_square.y) == int(enemy.position.y))

## Plays a seeded battle through the authority until Fireball is legal, then
## aims it on the battle screen: the legal squares light up and a click on one
## reveals the attack on that square.
func _area_aim() -> void:
	var found := {}
	for attempt in 10:
		var gateway := TacticsGateway.new(runtime)
		var view := gateway.start("tactics-area-%d" % attempt, {"hero": "adventurer_movement", "enemies": ["tactics_bell_diver"], "arena": "open_field", "seed": 300 + attempt})
		for i in 400:
			if view.get("accepted") != true or not str(view.get("battle_result", "")).is_empty(): break
			if _fireball_ready(view): found = {"gateway": gateway, "view": view}; break
			var reroll: Array = view.legal_actions.filter(func(a): return a.type == "planning_reroll")
			if not reroll.is_empty():
				reroll.sort_custom(func(a, b): return a.payload.reroll_indices.size() > b.payload.reroll_indices.size())
				view = gateway.submit(reroll[0]); continue
			var skip: Array = view.legal_actions.filter(func(a): return a.type == "planning_pass" and str(view.snapshot.get("simultaneous_step", "")) == "offense_rolling")
			view = gateway.submit(skip[0] if not skip.is_empty() else _proxy_choice(view, view.legal_actions))
		if not found.is_empty(): break
	_expect(not found.is_empty(), "a seeded battle reaches a Fireball that can be centred on the enemy")
	if found.is_empty(): return
	TacticsGateway.active_store().save_active(found.gateway.battle_id, ME)
	var screen = await _open(found.gateway, found.view)
	await _settle(screen)
	# Opening mid-battle first plays back the previous round.
	var played := Time.get_ticks_msec() + 15000
	while Time.get_ticks_msec() < played and not screen._tactics.playback.is_empty(): await process_frame
	_expect(await _press(screen, "battle.ability.%s.fireball" % ME), "Fireball is chosen on the ability rail")
	await _settle(screen)
	var squares: Array = screen._tactics.aim_actions()
	_expect(not squares.is_empty() and screen._tactics.board.highlights.size() == squares.size(), "the grid lights exactly Fireball's legal squares (%d)" % squares.size())
	_expect(_find(screen, "battle.tactics.cancel_aim") != null, "the rail offers Choose a square · Cancel")
	if squares.is_empty(): screen.queue_free(); return
	# The enemy's token stands on a lit square: clicking it aims Fireball
	# there rather than opening the enemy's stats.
	var target = _tv(screen).creatures["enemy-1"].position
	_expect(screen._tactics.board.highlights.has(Vector2i(int(target.x), int(target.y))), "the enemy's own square is lit")
	await _click(_find(screen, "battle.tactics.creature.enemy-1"))
	await _settle(screen)
	var aimed := false
	for attack in _tv(screen).get("attacks", []):
		if str(attack.attacker) == ME and str(attack.ability_id) == "fireball" and attack.get("square") is Dictionary and int(attack.square.x) == int(target.x) and int(attack.square.y) == int(target.y): aimed = true
	_expect(aimed, "clicking the enemy on a lit square centres Fireball on it")
	_expect(not screen._tactics.board.areas.is_empty(), "its 3 × 3 area is drawn on the grid")
	screen.queue_free()
	await _frames(2)

# --- A defense card with two attacks to choose from targets their badges ---------------

## The card's start actions, one per incoming attack it can be played on.
func _card_sources(view: Dictionary, card: String) -> Dictionary:
	var by_source := {}
	for action in view.get("legal_actions", []):
		var payload: Dictionary = action.get("payload", {})
		var commitment: Dictionary = payload.get("commitment", {}) if payload.get("commitment") is Dictionary else {}
		var choice = JSON.parse_string(str(commitment.get("choice_id", ""))) if str(commitment.get("choice_id", "")).begins_with("{") else {}
		if commitment.get("card_ids", []) != [card] or not choice is Dictionary or str(choice.get("source", "")).is_empty(): continue
		by_source[str(choice.source)] = action
	return by_source if by_source.size() > 1 else {}

func _card_picks_its_attack() -> void:
	var found := {}
	for attempt in 12:
		var gateway := TacticsGateway.new(runtime)
		var view := gateway.start("tactics-card-target-%d" % attempt, {"hero": "starter_movement", "enemies": ["tactics_brine_mask", "tactics_drowned_regent"], "arena": "open_field", "seed": 900 + attempt})
		for i in 600:
			if view.get("accepted") != true or not str(view.get("battle_result", "")).is_empty() or view.get("legal_actions", []).is_empty(): break
			if str(view.snapshot.get("simultaneous_step", "")) == "defense_before":
				for card in view.snapshot.actors[ME].get("hand", []):
					if not _card_sources(view, str(card)).is_empty(): found = {"gateway": gateway, "view": view, "card": str(card)}; break
			if not found.is_empty(): break
			view = gateway.submit(_proxy_choice(view, view.legal_actions))
		if not found.is_empty(): break
	_expect(not found.is_empty(), "a defense step offered a card on two incoming attacks")
	if found.is_empty(): return
	TacticsGateway.active_store().save_active(found.gateway.battle_id, ME)
	var screen = await _open(found.gateway, found.view)
	await _settle(screen)
	var sources := _card_sources(found.view, found.card)
	_expect(sources.keys().all(func(id): return screen._attack_intents.has(id)), "both incoming attacks have their badge on the grid")
	await _click_hand(screen, found.card)
	await _frames(4)
	_expect(screen._selected_card.get("source_targeting", false), "clicking the card makes the attack badges its targets")
	var remaining := func(source_id: String) -> int:
		for source in screen._view.damage_sources:
			if str(source.id) == source_id: return int(source.get("final_amount", 0))
		return -1
	var chosen: String = sources.keys()[0]
	var other: String = sources.keys()[1]
	var before := {chosen: remaining.call(chosen), other: remaining.call(other)}
	screen._play_source_card(chosen)
	await _settle(screen)
	_expect(not found.card in _hand(screen), "the card was played")
	_expect(remaining.call(chosen) < int(before[chosen]) and remaining.call(other) == int(before[other]), "it prevented damage on the chosen attack only: %s" % before)
	screen.queue_free()
	await _frames(2)

# --- Layout: 1–4 enemies, popups inside the viewport ------------------------------

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
		var screen = await _open(gateway, view)
		var viewport: Rect2 = screen.get_viewport_rect()
		var inside := true
		for control in screen.find_children("*", "Control", true, false):
			var id := str(control.get_meta("inspection_id", ""))
			if (id.begins_with("battle.tactics.creature.") or id.begins_with("battle.tactics.badge.") or id.begins_with("battle.enemy.select.")) and control.is_visible_in_tree() and not viewport.encloses(control.get_global_rect()):
				inside = false
				print("    outside: %s %s" % [id, control.get_global_rect()])
		_expect(inside, "%d enemies at %dx%d: every token, label and enemy name is inside the viewport" % [mixes[i].size(), size.x, size.y])
		var docks: Array = screen._enemy_ids().map(func(id): return screen._actor_profiles[id].get_parent().get_global_rect())
		var apart := true
		for a in docks.size():
			for b in range(a + 1, docks.size()):
				if docks[a].intersects(docks[b]): apart = false
		_expect(apart and docks.all(func(rect): return viewport.encloses(rect)), "%d enemies at %dx%d: the enemies' stats stack at the upper right without overlapping" % [mixes[i].size(), size.x, size.y])
		var tokens: Array = []
		for control in screen.find_children("*", "Control", true, false):
			if str(control.get_meta("inspection_id", "")).begins_with("battle.tactics.creature.enemy"): tokens.append(control)
		tokens.sort_custom(func(a, b): return a.global_position.x < b.global_position.x)
		for token in [tokens[0], tokens[-1]] if not tokens.is_empty() else []:
			if not screen._tactics.in_movement(): break
			await _click(token)
			var popup: Control = screen._tactics._popup
			_expect(is_instance_valid(popup) and viewport.encloses(popup.get_global_rect()), "the Pursue/Flee popup stays inside the viewport at x=%d" % int(token.global_position.x))
			screen._tactics.hide_popup()
		for edge in [Rect2(Vector2(2, 2), Vector2(4, 4)), Rect2(viewport.size - Vector2(6, 6), Vector2(4, 4))]:
			screen._tactics.show_popup("Edge check", [{"text": "Pursue", "id": "edge.pursue", "call": func(): pass}, {"text": "Flee from", "id": "edge.flee", "call": func(): pass}], edge)
			await _frames(2)
			_expect(viewport.encloses(screen._tactics._popup.get_global_rect()), "a popup opened at %s stays inside the viewport" % str(edge.position))
			screen._tactics.hide_popup()
		for control in screen.find_children("*", "Button", true, false):
			if not str(control.get_meta("inspection_id", "")).begins_with("battle.tactics.creature.") or control.tooltip_text.is_empty(): continue
			var tip = control._make_custom_tooltip(control.tooltip_text)
			_expect(tip != null and tip.custom_minimum_size.x <= viewport.size.x - 32, "the %s hover wraps inside the viewport" % str(control.get_meta("inspection_id")))
			if tip != null: tip.free()
			break
		screen.queue_free()
		await _frames(2)
	root.size = Vector2i(1280, 720)
