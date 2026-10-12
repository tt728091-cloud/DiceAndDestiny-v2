extends "res://tests/presentation/verify_campaign.gd"
## The Expedition map by pointer: menu → new expedition → the map; select a
## hex and pay with cards and energy to move; explore; the 1-card rule; a
## provoked fight → the real battle → the result → back to the map with the
## returned piles; challenges until the tile is free; an event choice; a site
## service paid with Influence and a deck change at the site through Prepare;
## Make camp and the rest summary; Night; the Warlord's deadline fight and the
## region's end screen. Then a 1920×1080 pass for layout screenshots.

const EXPEDITION_SCRIPT := "res://app/screens/map/expedition_screen.gd"
const PREPARE_SCRIPT := "res://app/screens/campaign/campaign_prepare.gd"
const VIEW_STATE := preload("res://local_client/view_state/battle_view_state.gd")
## A seed whose Warlord stands on the east slot in round 1 and loses to the
## proxy when caught there (found with EXPEDITION_SEED_PROBE=1-60).
const VICTORY_SEED := 8

var runtime
var map: Control
var seed_used := 0

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	runtime = root.get_node("LearnedBattleRuntime")
	seed_used = _probe_seed()
	_expect(seed_used > 0, "a seed whose east tile holds an event site was found")
	if seed_used == 0: _finish_map(); return
	if not OS.get_environment("EXPEDITION_SEED_PROBE").is_empty():
		# Offline helper: report how a straight Warlord hunt ends per seed.
		var parts := OS.get_environment("EXPEDITION_SEED_PROBE").split("-")
		for seed in range(int(parts[0]), int(parts[-1]) + 1):
			print("expedition seed probe: seed %d → %s" % [seed, await _victory_pass(seed)])
		_finish_map(); return
	await _full_flow(seed_used)
	await _layout_pass(Vector2i(1920, 1080), seed_used)
	await _victory_flow(VICTORY_SEED)
	_finish_map()

func _finish_map() -> void:
	print("EXPEDITION MAP: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

## Through the authority's ops only: find a seed whose east tile is the Reed
## Shallows (a lone, unwary Brine Mask and the wayside shrine event), so the
## pointer run fights the region's weakest group and plays an event scene.
## Probe saves are deleted again.
func _probe_seed() -> int:
	for seed in range(1, 120):
		var created: Dictionary = runtime.campaign_new("starter", "Probe", "expedition", "", seed)
		if created.get("ok") != true: return 0
		var id := str(created.result.id)
		var view: Dictionary = runtime.expedition_status(id).result.view
		var hand: Array = view.hand
		var ok: bool = runtime.expedition_act(id, {"kind": "move", "target": {"q": 1, "r": 0}, "cards": [str(hand[0].id)]}, 0).get("ok") == true
		view = runtime.expedition_status(id).result.view; hand = view.hand
		ok = ok and runtime.expedition_act(id, {"kind": "explore", "slot": "east", "cards": [str(hand[0].id), str(hand[1].id)]}, 0).get("ok") == true
		view = runtime.expedition_status(id).result.view
		var east := ""
		for hex in view.hexes:
			if int(hex.q) == 2 and int(hex.r) == 1: east = str(hex.get("tile", ""))
		runtime.campaign_delete(id)
		if ok and east == "reed_shallows":
			print("expedition map test: seed %d places %s east" % [seed, east])
			return seed
	return 0

# A competent human proxy -----------------------------------------------------
## Plays the battle through the authority like an attentive player: roll,
## re-roll the dice that do not serve the best reachable attack, choose the
## highest-damage qualified tier, defend every attack with the guard ability
## and play prevention cards whenever they are legal. Only legal actions the
## authority lists are ever submitted.
func _play_out(gateway: RefCounted, view: Dictionary) -> Dictionary:
	var guard := 0
	while view.get("battle_result", "") not in ["victory", "defeat", "draw"] and guard < 3000:
		guard += 1
		if view.get("learned_policy", {}).get("model_turn") == true:
			view = gateway.advance_model()
		else:
			var state = VIEW_STATE.new(); state.apply_result(view)
			view = gateway.submit(JSON.stringify(_choose(state, view.get("legal_actions", []))))
		if view.get("accepted") != true:
			_expect(false, "battle command failed: %s" % str(view.get("error", "")))
			break
	_expect(view.get("battle_result", "") in ["victory", "defeat", "draw"], "the battle reached a result")
	return view

func _choose(state, actions: Array) -> Dictionary:
	if actions.is_empty(): return {}
	var by_type := {}
	for action in actions:
		var type := str(action.get("type", ""))
		if not by_type.has(type): by_type[type] = []
		by_type[type].append(action)
	# Prevention cards: a played card stays health, so play them when legal.
	for action in by_type.get("commit_interaction", []):
		var commitment: Dictionary = action.get("payload", {}).get("commitment", {})
		if not commitment.get("proposal_ids", []).is_empty() and not commitment.get("card_ids", []).is_empty(): return action
	var me: Dictionary = state.actor("blade")
	match str(state.stage):
		"planning":
			if by_type.has("planning_roll"): return by_type.planning_roll[0]
			var dice: Array = state.rolled_dice("blade")
			if int(state.choose_count("blade")) > 0 and by_type.has("planning_keep"):
				return _keep_best(dice, int(state.choose_count("blade")), by_type.planning_keep)
			var best: Dictionary = {}
			var best_damage := -1
			for action in by_type.get("planning_select_ability", []):
				var damage := _tier_damage(state, str(action.payload.get("ability_id", "")), str(action.payload.get("tier_id", "")))
				if damage > best_damage: best = action; best_damage = damage
			var rerolls: Array = by_type.get("planning_reroll", [])
			var selected := str(me.get("selected_ability", ""))
			if selected.is_empty() and not rerolls.is_empty() and best_damage < 6:
				var reroll := _reroll_toward(dice, rerolls)
				if not reroll.is_empty(): return reroll
			if selected.is_empty() and not best.is_empty(): return best
			if by_type.has("planning_pass"): return by_type.planning_pass[0]
		"defense_selection":
			for action in by_type.get("planning_select_ability", []):
				if not action.payload.get("spend_catalyst", false): return action
			if by_type.has("planning_pass"): return by_type.planning_pass[0]
		"defense_roll":
			if by_type.has("roll_dice"): return by_type.roll_dice[0]
	for type in ["roll_dice", "planning_pass", "pass", "commit_interaction"]:
		if by_type.has(type): return by_type[type][0]
	return actions[0]

func _symbol_count(dice: Array, symbol: String) -> int:
	var count := 0
	for die in dice:
		if symbol in die.get("symbols", []): count += 1
	return count

## Keep the dice that serve the strongest symbol among the rolled dice.
func _goal_symbol(dice: Array) -> String:
	return "gold_coin" if _symbol_count(dice, "gold_coin") >= 3 and _symbol_count(dice, "sword") < 3 else "sword"

func _reroll_toward(dice: Array, rerolls: Array) -> Dictionary:
	var goal := _goal_symbol(dice)
	var indices: Array = []
	for index in dice.size():
		if goal not in dice[index].get("symbols", []): indices.append(int(dice[index].get("index", index)))
	if indices.is_empty(): return {}
	for action in rerolls:
		var listed: Array = action.payload.get("reroll_indices", []).map(func(i): return int(i))
		listed.sort()
		if listed == indices: return action
	return {}

func _keep_best(dice: Array, count: int, keeps: Array) -> Dictionary:
	var goal := _goal_symbol(dice)
	var order: Array = range(dice.size())
	order.sort_custom(func(a, b): return (goal in dice[a].get("symbols", [])) and not (goal in dice[b].get("symbols", [])))
	var wanted: Array = order.slice(0, count).map(func(i): return int(dice[i].get("index", i)))
	wanted.sort()
	for action in keeps:
		var listed: Array = action.payload.get("kept_indices", []).map(func(i): return int(i))
		listed.sort()
		if listed == wanted: return action
	return keeps[0]

func _tier_damage(state, ability_id: String, tier_id: String) -> int:
	var definition: Dictionary = state.content_definition("abilities", ability_id)
	var tiers: Array = definition.get("qualification", {}).get("activation_tiers", [])
	var total := 0
	for index in tiers.size():
		var tier: Dictionary = tiers[index]
		if not tier_id.is_empty() and str(tier.get("id", "")) != tier_id: continue
		for operation in tier.get("operations", []):
			if str(operation.get("type", "")) == "deal_damage": total = maxi(total, int(operation.get("amount", 0)))
		if tier_id.is_empty(): break
	return total if total > 0 else 1

# Helpers --------------------------------------------------------------------
func _control(screen: Node, key: String) -> Button:
	if screen == null: return null
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("expedition_control", "") == key and not button.is_queued_for_deletion() and button.is_visible_in_tree(): return button
	return null

func _state() -> Dictionary:
	return map.inspection_state() if map != null else {}

func _press(key: String) -> bool:
	var button := _control(map, key)
	_expect(button != null, "control %s is present (phase %s, round %s, overlay: %s)" % [key, _state().get("phase", ""), _state().get("round", ""), _text(map._overlay).strip_edges().replace("\n", " | ") if map != null else ""])
	if button == null: return false
	_expect(not button.disabled, "control %s is enabled" % key)
	if button.disabled: return false
	await _click(button)
	for frame in 3: await process_frame
	return true

func _move_cost(q: int, r: int) -> int:
	for move in map.view.get("moves", []):
		if int(move.to.q) == q and int(move.to.r) == r and str(move.get("blocked", "")).is_empty(): return int(move.cost)
	return -1

## Select a hex, pay for it with cards (and energy for what cards cannot
## cover), then press Move. Returns false if the move was not offered.
func _move_to(q: int, r: int) -> bool:
	var cost := _move_cost(q, r)
	if cost < 0: return false
	await _select(q, r)
	var pool := int(map.view.turn.move)
	var need := maxi(0, cost - pool)
	var hand: int = map.view.hand.size()
	var cards := mini(need, hand)
	for index in cards: await _press("expedition.card.%d" % index)
	var energy := need - cards
	for n in energy: await _press("expedition.energy.plus")
	if not await _press("expedition.move"): return false
	var player: Dictionary = _state().player
	return int(player.q) == q and int(player.r) == r or _state().pending_battle != null

## Clicking a hex toggles its selection; select it only if it is not selected.
func _select(q: int, r: int) -> void:
	var selected = _state().get("selected_hex")
	if selected != null and int(selected[0]) == q and int(selected[1]) == r: return
	await _press("expedition.hex.%d.%d" % [q, r])
	selected = _state().get("selected_hex")
	_expect(selected != null and int(selected[0]) == q and int(selected[1]) == r, "clicking hex (%d,%d) selects it" % [q, r])

func _neighbors(q: int, r: int) -> Array:
	return [Vector2i(q + 1, r), Vector2i(q + 1, r - 1), Vector2i(q, r - 1), Vector2i(q - 1, r), Vector2i(q - 1, r + 1), Vector2i(q, r + 1)]

## Shortest path over placed, passable, enemy-free hexes (steps, not cost).
## Hexes next to a living group are avoided when another path exists, so the
## walk does not provoke a fight by accident.
func _path(to: Vector2i, quiet := true) -> Array:
	var hexes := {}
	var threatened := {}
	for hex in map.view.hexes:
		if hex.get("group") != null or hex.get("warlord", false):
			for n in _neighbors(int(hex.q), int(hex.r)): threatened[n] = true
	for hex in map.view.hexes:
		var at := Vector2i(int(hex.q), int(hex.r))
		var free: bool = hex.get("passable", false) and hex.get("group") == null and not hex.get("warlord", false) and not (hex.get("site") != null and hex.site.get("defended", false))
		if quiet and threatened.has(at) and at != to: free = false
		if free or at == to: hexes[at] = true
	var start := Vector2i(int(map.view.player.q), int(map.view.player.r))
	var previous := {start: start}
	var queue := [start]
	while not queue.is_empty():
		var current: Vector2i = queue.pop_front()
		if current == to: break
		for next in _neighbors(current.x, current.y):
			if hexes.has(next) and not previous.has(next):
				previous[next] = current; queue.append(next)
	if not previous.has(to): return _path(to, false) if quiet else []
	var path := [to]
	while path[0] != start: path.push_front(previous[path[0]])
	path.pop_front()
	return path

## Walks a path step by step, ending turns to redraw cards when the hand and
## energy run out. Stops when a fight starts.
func _walk(to: Vector2i) -> bool:
	for step in 40:
		if Vector2i(int(map.view.player.q), int(map.view.player.r)) == to: return true
		if map.view.phase != "turn": return false
		var path := _path(to)
		if path.is_empty(): _expect(false, "no path to %s" % to); return false
		var next: Vector2i = path[0]
		var cost := _move_cost(next.x, next.y)
		var affordable: bool = cost >= 0 and int(map.view.turn.move) + map.view.hand.size() + int(map.view.energy) / 5 >= cost
		if not affordable:
			if not bool(map.view.turn.can_end_turn):
				if map.view.hand.size() > 0:
					await _press("expedition.card.0"); await _press("expedition.discard")
				else: _expect(false, "stuck: cannot end the turn"); return false
			await _press("expedition.end_turn")
			await _settle_overlays()
			continue
		if not await _move_to(next.x, next.y): return false
		if _state().pending_battle != null: return false
	return false

## Dismisses rest and event summaries and finishes a camp.
func _settle_overlays() -> void:
	for round in 4:
		if _control(map, "expedition.rest.continue") != null: await _press("expedition.rest.continue")
		elif _control(map, "expedition.event.continue") != null: await _press("expedition.event.continue")
		elif _control(map, "expedition.battle.continue") != null: await _press("expedition.battle.continue")
		elif map.view.phase == "camp": await _press("expedition.camp.break")
		else: return

## Fights the map's pending battle through the real battle screen and the
## gateway, returns to the map and reads the result.
func _fight() -> String:
	_expect(_state().pending_battle != null, "a battle is pending")
	var pending: Dictionary = _state().pending_battle
	var before_turn := int(map.view.turn.number)
	if not await _press("expedition.fight"): return ""
	for frame in 8: await process_frame
	var battle: Control = _find(BATTLE_SCRIPT)
	_expect(battle != null and _find(EXPEDITION_SCRIPT) == null, "Fight opens the real battle screen")
	if battle == null: return ""
	_expect(str(battle._view.campaign.expedition.name) == str(pending.name), "the battle carries the expedition context")
	var gateway: RefCounted = battle.gateway
	var result := _play_out(gateway, battle.initial_result)
	battle.queue_free(); await process_frame
	battle = BATTLE_SCREEN.instantiate()
	battle.initial_result = result; battle.gateway = gateway; battle.learned_battle_mode = true
	root.add_child(battle); await process_frame
	battle._director.clear(); battle._render()
	for frame in 4: await process_frame
	var you: Dictionary = battle._view.actor("blade")
	print("expedition map test: battle %s · %s · you %d/%d energy %d · enemy %s %d/%d · rounds %s" % [str(pending.get("id", "")), str(result.get("battle_result", "")), int(you.get("current_health", 0)), int(you.get("max_health", 0)), int(you.get("energy", -1)), str(pending.get("opponent", "")), int(battle._view.actor(battle._focused_enemy).get("current_health", 0)), int(battle._view.actor(battle._focused_enemy).get("max_health", 0)), str(result.get("round", result.get("turn", "")))])
	var reward: Label = battle.find_child("CampaignReward", true, false)
	_expect(reward != null and "Back to the map" in reward.text, "the result panel shows the map consequences")
	var outcome: Dictionary = result.get("campaign", {}).get("outcome", {})
	_expect(not outcome.is_empty() and str(outcome.get("result", "")) == str(result.get("battle_result", "")), "the authority recorded the expedition battle")
	await _capture(battle, "expedition-battle-result-%s" % str(result.get("battle_result", "")))
	var proceed := _find_button(battle, "Continue · Back to the map")
	_expect(proceed != null, "the result panel offers Continue · Back to the map (result %s, reward %s)" % [str(result.get("battle_result", "")), reward.text if reward != null else "none"])
	if proceed == null:
		battle.queue_free(); await process_frame
		return ""
	_expect(battle._center_scroll.get_global_rect().encloses(proceed.get_global_rect()), "Continue is inside the result panel without scrolling")
	await _click(proceed)
	for frame in 8: await process_frame
	map = _find(EXPEDITION_SCRIPT)
	_expect(map != null and _find(BATTLE_SCRIPT) == null, "Continue returns to the map")
	if map == null: return ""
	_expect(_control(map, "expedition.battle.continue") != null, "the map shows the battle's consequences")
	var report: Dictionary = outcome.get("report", {}) if outcome.get("report") != null else {}
	var live: int = map.view.hand.size() + map.view.draw.size() + map.view.spent.size()
	var reported: int = report.get("hand_after", []).size() + int(report.get("draw_after", 0)) + int(report.get("spent_after", 0))
	_expect(live == reported, "the piles came back as the battle left them (%d live cards vs %d reported)" % [live, reported])
	_expect(int(map.view.energy) == int(report.get("energy_kept", -1)) or map.view.phase == "camp", "energy kept is what the authority reports")
	_expect(map.view.wounds.size() == int(report.get("wounds", -1)), "wounds match the report")
	_expect(_state().pending_battle == null, "the fight is over")
	_expect(int(map.view.turn.number) > before_turn or map.view.phase == "camp" or map.view.phase in ["victory", "defeat"], "a fight ends the turn")
	await _capture(map, "expedition-map-after-battle-%s" % str(result.get("battle_result", "")))
	await _press("expedition.battle.continue")
	return str(result.get("battle_result", ""))

# The flow -------------------------------------------------------------------
func _full_flow(seed: int) -> void:
	var menu = MENU.instantiate(); root.add_child(menu)
	for frame in 6: await process_frame
	await _click(menu._menu_actions[2])
	for frame in 6: await process_frame
	var campaign: Control = _find(CAMPAIGN_SCRIPT)
	_expect(campaign != null, "Campaign opens from the menu")
	if campaign == null: return
	var expedition_mode := _campaign_control(campaign, "campaign.mode.expedition")
	_expect(expedition_mode != null and expedition_mode.button_pressed, "new campaigns default to the Expedition mode")
	_expect("random seed" in campaign._new_note.text, "the note explains the region is seeded")
	campaign._new_seed.text = str(seed)
	await _click(_campaign_control(campaign, "campaign.begin"))
	for frame in 8: await process_frame
	map = _find(EXPEDITION_SCRIPT)
	_expect(map != null and _find(CAMPAIGN_SCRIPT) == null, "Begin opens the map for an expedition")
	if map == null: return
	var state := _state()
	_expect(state.ready and int(state.placed_tiles) == 1 and state.hand.size() == 4 and int(state.energy) == 10 and int(state.round) == 1 and not state.night, "the map starts with the start tile, four cards and base energy")
	_expect("Day 1 · round 1 of 4" in map._day.text, "the HUD shows Day 1 · round 1 of 4")
	_expect(_control(map, "expedition.hex.0.0") != null and _control(map, "expedition.hex.2.1") != null, "every placed and unexplored hex is a registered control")
	_expect(_state().explores.is_empty(), "nothing is explorable from the start hex")
	await _capture(map, "expedition-map-start-1280")

	# Turn 1: the village next door. A card pays the step; two more cards are
	# Influence there; a deck change through Prepare spends it.
	_expect(await _move_to(-1, 1), "a card pays for the plains step to the village")
	state = _state()
	_expect(int(state.player.q) == -1 and int(state.spent) == 1 and int(state.turn.cards_used) == 1 and state.hand.size() == 3, "the paid card went to Spent")
	_expect(state.site_here != null and state.site_here.get("open", false) and str(state.site_here.type) == "village", "Hollow Ford is open by day")
	await _press("expedition.card.0"); await _press("expedition.card.1")
	_expect(await _press("expedition.site.influence"), "two cards played at the village")
	_expect(int(map.view.turn.influence) == 2 and str(map.view.turn.influence_site) == "hollow_ford" and map.view.hand.size() == 1, "they are 2 Influence here, this turn")
	await _capture(map, "expedition-village-1280")
	var xp := int(_state().xp)
	var draw := int(_state().draw)
	await _press("expedition.prepare")
	for frame in 6: await process_frame
	var prepare: Control = _find(PREPARE_SCRIPT)
	_expect(prepare != null and prepare.expedition and prepare._influence.text == "2 / 2", "Prepare opens from the site with the Influence shown")
	if prepare != null:
		await _tap(_prepare_control(prepare, "prepare.add"))
		for frame in 3: await process_frame
		await _tap(_prepare_control(prepare, "prepare.buy.trailblaze"))
		for frame in 4: await process_frame
		_expect(prepare._influence.text == "0 / 2" and int(prepare.data.xp) == xp - 10, "buying Trailblaze spent 10 XP and 2 Influence")
		await _capture(prepare, "expedition-prepare-1280")
		await _tap(_prepare_control(prepare, "prepare.buy.trailblaze"))
		for frame in 3: await process_frame
		_expect("Influence" in prepare._message.text, "a second change without Influence is refused by the authority")
		await _click(_prepare_control(prepare, "prepare.back"))
		for frame in 6: await process_frame
	map = _find(EXPEDITION_SCRIPT)
	_expect(map != null and int(_state().draw) == draw + 1 and int(_state().xp) == xp - 10 and int(map.view.turn.influence) == 0, "the bought card tops the draw pile and the Influence is spent")
	_expect(await _press("expedition.end_turn"), "the turn ends after cards were played")
	state = _state()
	_expect(int(state.turn.number) == 2 and state.hand.size() == 4, "the hand refills to four")

	# Turn 2: End turn is blocked until a card is used; a discard unlocks it.
	# Then the tile's east edge and Explore east, paid with a card and energy.
	var end_turn := _control(map, "expedition.end_turn")
	_expect(end_turn != null and end_turn.disabled and "play or discard" in end_turn.tooltip_text, "End turn is blocked until a card is played or discarded")
	await _press("expedition.card.0"); await _press("expedition.discard")
	end_turn = _control(map, "expedition.end_turn")
	_expect(end_turn != null and not end_turn.disabled, "a discard unlocks End turn")
	_expect(await _move_to(0, 0) and await _move_to(1, 0), "two cards walk back across the start tile")
	_expect(not _state().explores.is_empty(), "the east slot is explorable from the tile's edge")
	await _select(2, 0)
	await _press("expedition.card.0")
	await _press("expedition.energy.plus")
	_expect(await _press("expedition.explore"), "Explore pays with a card and energy")
	state = _state()
	_expect(int(state.placed_tiles) == 2 and int(state.energy) == 5 and state.hand.size() == 0, "the east tile is placed; energy and cards were spent")
	await _capture(map, "expedition-map-explored-1280")
	await _press("expedition.end_turn")
	_expect(int(_state().turn.number) == 3 and _state().hand.size() == 4, "turn 3 begins with four cards")

	# Turn 3: a provoked fight. (2,0) and (3,0) are both next to the group.
	_expect(await _move_to(2, 0), "two cards pay for the forest step")
	var provoked := false
	for move in map.view.moves:
		if int(move.to.q) == 3 and int(move.to.r) == 0: provoked = not str(move.get("provokes", "")).is_empty() and str(move.get("blocked", "")).is_empty()
	_expect(provoked, "the view warns that (3,0) provokes the group")
	var results: Array = []
	if provoked:
		await _move_to(3, 0)
		var pending: Dictionary = _state().pending_battle if _state().pending_battle != null else {}
		_expect(str(pending.get("kind", "")) == "provoked" and pending.get("ambush", false) == false, "a provoked fight starts at once without an Ambush")
		await _capture(map, "expedition-provoked-1280")
		results.append(await _fight())
	# Challenge the group again until it is beaten.
	for attempt in 4:
		await _settle_overlays()
		var alive := false
		for hex in map.view.hexes:
			if int(hex.q) == 2 and int(hex.r) == 1 and hex.get("group") != null: alive = true
		if not alive or map.view.phase != "turn": break
		if map.view.hand.size() == 0 and bool(map.view.turn.can_end_turn): await _press("expedition.end_turn"); await _settle_overlays()
		if not await _walk(Vector2i(2, 0)) and _state().pending_battle == null: _expect(false, "could not reach (2,0)"); break
		if _state().pending_battle == null:
			await _select(2, 1)
			if _control(map, "expedition.challenge") == null:
				var st := _state()
				_expect(false, "the group at (2,1) can be challenged from (2,0): challenges=%s phase=%s player=%s turn=%s" % [st.challenges, st.phase, st.player, st.turn]); break
			await _press("expedition.challenge")
		results.append(await _fight())
	print("expedition map test: fights against the east group: %s" % [results])
	_expect("victory" in results, "the east group was beaten within the attempts: %s" % [results])
	await _settle_overlays()

	# The wayside shrine on the east tile: an event scene and a choice.
	var event_hex := Vector2i(999, 999)
	for hex in map.view.hexes:
		if hex.get("site") != null and str(hex.site.type) == "event" and not hex.site.get("resolved", false): event_hex = Vector2i(int(hex.q), int(hex.r))
	_expect(event_hex != Vector2i(999, 999), "the east tile holds an unresolved event site")
	if event_hex != Vector2i(999, 999) and map.view.phase == "turn":
		_expect(await _walk(event_hex), "walked to the event site")
		_expect(map.view.phase == "event" and map.view.get("event") != null, "stepping on the event site opens its scene")
		await _capture(map, "expedition-event-1280")
		var chosen := ""
		var choices: Array = map.view.event.choices if map.view.get("event") != null else []
		for choice in choices:
			if choice.get("available", false) and int(choice.get("influence", 0)) == 0 and int(choice.get("gold", 0)) == 0 and str(choice.get("item", "")).is_empty() and chosen.is_empty(): chosen = str(choice.id)
		_expect(not chosen.is_empty(), "a choice needing nothing is offered")
		if not chosen.is_empty():
			await _press("expedition.event." + chosen)
			_expect(map.view.phase in ["turn", "battle", "camp"] and map.view.get("last_event") != null and str(map.view.last_event.choice) == chosen, "the authority resolved the choice")
			_expect(_control(map, "expedition.event.continue") != null, "the event's outcome is shown")
			await _capture(map, "expedition-event-result-1280")
			if _state().pending_battle != null: await _fight()
			await _settle_overlays()

	# Make camp: the camp panel, Influence-paid mending, the rest summary.
	if map.view.phase == "turn":
		var at_inn: bool = map.view.get("site_here") != null and str(map.view.site_here.get("type", "")) in ["village", "town"]
		await _press("expedition.make_camp")
		_expect(map.view.phase == "camp" and bool(map.view.camp.get("inn", false)) == at_inn, "Make camp opens the camp; the inn only at a village or town")
		_expect(_control(map, "expedition.prepare") != null, "the deck can be changed at camp")
		if map.view.wounds.size() > 0 and map.view.hand.size() >= 3:
			for index in 3: await _press("expedition.card.%d" % index)
			await _press("expedition.camp.influence")
			_expect(int(map.view.turn.influence) == 3, "three cards are 3 Influence at camp")
			await _press("expedition.camp.mend_influence")
			_expect(map.view.camp.mend_rolls.size() == 1 and int(map.view.turn.influence) == 0, "3 Influence buy one mending roll")
		await _capture(map, "expedition-camp-1280")
		var round_before := int(map.view.round)
		var night_before: bool = map.view.night
		await _press("expedition.camp.break")
		_expect(_control(map, "expedition.rest.continue") != null, "the rest summary is shown")
		await _capture(map, "expedition-rest-1280")
		_expect(int(map.view.round) == round_before + 1 and map.view.night != night_before and map.view.get("last_rest") != null and int(map.view.last_rest.warlord_steps) >= 0, "breaking camp advances the round, flips day and night and marches the Warlord")
		await _press("expedition.rest.continue")
	if map.view.night:
		_expect("Night" in map._day.text, "the HUD shows Night")
		for hex in map.view.hexes:
			if str(hex.get("terrain", "")) == "forest": _expect(int(hex.cost) == 3, "forest costs 3 at night")
	# Camp through the remaining rounds until the Warlord comes.
	for round in 6:
		if _state().pending_battle != null or map.view.phase in ["victory", "defeat"]: break
		if map.view.phase == "turn": await _press("expedition.make_camp")
		if map.view.phase == "camp": await _press("expedition.camp.break")
		await _settle_overlays()
	var deadline: Dictionary = _state().pending_battle if _state().pending_battle != null else {}
	_expect(str(deadline.get("kind", "")) == "deadline" and deadline.get("warlord", false) and not deadline.get("ambush", false), "after the last round the Warlord comes: %s" % [deadline])
	await _capture(map, "expedition-deadline-1280")
	var ending := await _fight()
	await _settle_overlays()
	_expect(map.view.phase in ["victory", "defeat"] and str(map.view.result) in ["victory", "defeat"], "the region ends after the Warlord fight")
	_expect(_control(map, "expedition.end.campaigns") != null, "the end screen offers the campaign list")
	await _capture(map, "expedition-end-%s-1280" % ending)
	await _press("expedition.end.campaigns")
	for frame in 6: await process_frame
	campaign = _find(CAMPAIGN_SCRIPT)
	_expect(campaign != null and ("Expedition · won" in _text(campaign._save_list) or "Expedition · run over" in _text(campaign._save_list)), "the campaign list shows the ended expedition")
	if campaign != null: campaign.queue_free(); await process_frame

## The region victory by pointer: a fresh expedition, explore east, catch the
## Warlord on the road and beat him; the victory end screen follows.
func _victory_flow(seed: int) -> void:
	root.size = Vector2i(1280, 720)
	for frame in 3: await process_frame
	var result := await _victory_pass(seed)
	_expect(result == "victory", "seed %d: the Warlord caught on the road is beaten by pointer (%s)" % [seed, result])
	if result != "victory": return
	await _settle_overlays()
	var state := _state()
	_expect(state.phase == "victory" and str(state.result) == "victory" and state.wounds.size() == 0 and int(state.xp) > 100, "the region is won: wounds mended, XP earned (%s, xp %s)" % [state.phase, state.xp])
	_expect(_control(map, "expedition.end.campaigns") != null and "region is won" in _text(map._overlay), "the victory end screen is shown")
	await _capture(map, "expedition-end-victory-1280")
	await _press("expedition.end.campaigns")
	for frame in 6: await process_frame
	var campaign: Control = _find(CAMPAIGN_SCRIPT)
	_expect(campaign != null and "Expedition · won" in _text(campaign._save_list), "the campaign list shows the won expedition")
	if campaign != null: campaign.queue_free(); await process_frame

## Starts an expedition for a seed, explores east and hunts the Warlord by
## pointer when he stands there; returns the fight's result ("" if none).
func _victory_pass(seed: int) -> String:
	var created: Dictionary = runtime.campaign_new("starter", "Victory %d" % seed, "expedition", "", seed)
	if created.get("ok") != true: return "no campaign"
	var screen = load(EXPEDITION_SCRIPT).new(); screen.save_id = str(created.result.id); root.add_child(screen)
	for frame in 6: await process_frame
	map = screen
	var result := ""
	if await _move_to(1, 0):
		await _select(2, 0); await _press("expedition.card.0"); await _press("expedition.energy.plus")
		if await _press("expedition.explore"):
			for attempt in 6:
				await _settle_overlays()
				if _state().pending_battle != null:
					result = await _fight(); break
				if map.view.phase != "turn": break
				var warlord := Vector2i(999, 999)
				for hex in map.view.hexes:
					if hex.get("warlord", false): warlord = Vector2i(int(hex.q), int(hex.r))
				if warlord == Vector2i(999, 999): result = "warlord not in sight"; break
				var player := Vector2i(int(map.view.player.q), int(map.view.player.r))
				var adjacent := player in _neighbors(warlord.x, warlord.y)
				if not adjacent:
					var walked := false
					for n in _neighbors(warlord.x, warlord.y):
						if not _path(n).is_empty() and await _walk(n): walked = true; break
					if _state().pending_battle != null: result = await _fight(); break
					if not walked:
						if bool(map.view.turn.can_end_turn): await _press("expedition.end_turn")
						else: await _press("expedition.card.0"); await _press("expedition.discard"); await _press("expedition.end_turn")
					continue
				await _select(warlord.x, warlord.y)
				if _control(map, "expedition.challenge") == null:
					if bool(map.view.turn.can_end_turn): await _press("expedition.end_turn")
					else: await _press("expedition.card.0"); await _press("expedition.discard"); await _press("expedition.end_turn")
					continue
				await _press("expedition.challenge")
				_expect(_state().pending_battle != null and _state().pending_battle.get("warlord", false), "challenging the Warlord starts his fight")
				await _capture(map, "expedition-warlord-challenge-1280")
				result = await _fight(); break
	if result != "victory":
		if is_instance_valid(screen): screen.queue_free(); await process_frame
		map = _find(EXPEDITION_SCRIPT)
	return result

## A second expedition at 1920×1080 for layout screenshots.
func _layout_pass(size: Vector2i, seed: int) -> void:
	root.size = size
	for frame in 3: await process_frame
	var created: Dictionary = runtime.campaign_new("starter", "Layout %d" % size.x, "expedition", "", seed)
	_expect(created.get("ok") == true, "a layout expedition starts")
	if created.get("ok") != true: return
	var screen = load(EXPEDITION_SCRIPT).new(); screen.save_id = str(created.result.id); root.add_child(screen)
	for frame in 8: await process_frame
	map = screen
	var label := "%dx%d" % [size.x, size.y]
	await _check_layout(label)
	await _capture(map, "expedition-map-start-" + label)
	_expect(await _move_to(1, 0), "layout pass: move")
	await _select(2, 0); await _press("expedition.card.0"); await _press("expedition.energy.plus"); await _press("expedition.explore")
	await _select(2, 1)
	await _check_layout(label)
	await _capture(map, "expedition-map-explored-" + label)
	await _press("expedition.make_camp")
	await _check_layout(label)
	await _capture(map, "expedition-camp-" + label)
	await _press("expedition.camp.break")
	await _check_layout(label)
	await _capture(map, "expedition-rest-" + label)
	screen.queue_free(); await process_frame

## Every visible control and overlay lies inside the viewport.
func _check_layout(label: String) -> void:
	var viewport := root.get_visible_rect()
	for node in [map._overlay, map._hud_scroll.get_parent(), map._hand_row.get_parent().get_parent()]:
		if node != null and node.is_visible_in_tree():
			_expect(viewport.encloses(node.get_global_rect()), "%s stays inside the viewport (%s): %s" % [node.name, label, node.get_global_rect()])
	for button in map.find_children("*", "Button", true, false):
		if button.is_visible_in_tree() and not str(button.get_meta("expedition_control", "")).begins_with("expedition.hex."):
			_expect(viewport.encloses(button.get_global_rect()), "%s inside the viewport (%s)" % [button.get_meta("expedition_control", button.name), label])
	for label_node in map._overlay.find_children("*", "Label", true, false):
		if label_node.is_visible_in_tree():
			_expect(viewport.encloses(label_node.get_global_rect()), "overlay text inside the viewport (%s): %s" % [label, label_node.text.left(40)])
