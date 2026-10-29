extends "res://tests/presentation/verify_campaign.gd"
## The Custom Adventurer's ability board in Prepare, by pointer: step the
## damage level up and apply it for its XP, drop the defense for Holdfast's
## 140 XP back, and pick a block defense priced from the total XP. Then an
## expedition at camp: the first board change spends the camp's Influence and
## opens the deck. Last, a Starter expedition buys a step of its own board.

const PREPARE_SCRIPT := "res://app/screens/campaign/campaign_prepare.gd"

func _run() -> void:
	root.gui_embed_subwindows = true
	root.size = Vector2i(1280, 720)
	_seed_card_trees()
	var runtime = root.get_node("LearnedBattleRuntime")
	var started: Dictionary = runtime.campaign_new("custom_adventurer", "Board test")
	_expect(started.get("ok") == true, "a Custom Adventurer campaign starts: %s" % str(started.get("error", "")))
	if started.get("ok") != true: _finish(); return
	var prepare = await _open_prepare(str(started.result.id), false)
	_expect(prepare._xp.text == "150", "the campaign starts with 150 XP to spend (the Holdfast board is part of the kit)")
	var row := _prepare_control(prepare, "prepare.board")
	_expect(row != null and "Ability board" in row.text and "Holdfast" in row.text, "the deck list shows the ability board with its defense")
	_expect(row != null and "140 XP" in _text(row), "the board row shows the XP it holds")
	await _tap(row)
	for frame in 4: await process_frame
	_expect(prepare.selection.get("kind") == "board" and "Ability board" in prepare._center_title.text, "selecting the board shows it")
	var apply := _prepare_control(prepare, "prepare.board.apply")
	_expect(apply != null and apply.disabled, "nothing to apply until something changes")
	# Step the level up: the change costs the step's XP.
	await _tap(_prepare_control(prepare, "prepare.board.level_up"))
	var step := int(prepare.board_view.staircase.step)
	var step_xp := int(prepare.board_view.staircase.steps[step].xp)
	_expect(step_xp > 0 and int(prepare.board_view.xp.cost) == step_xp, "the next step costs its XP: %d" % step_xp)
	_expect("Apply · %d XP" % step_xp == _prepare_control(prepare, "prepare.board.apply").text, "Apply shows the cost: " + _prepare_control(prepare, "prepare.board.apply").text)
	_expect("(now " in _text(prepare._center_content), "the abilities the step raises show their old damage")
	await _capture(prepare, "prepare-board-level")
	await _tap(_prepare_control(prepare, "prepare.board.apply"))
	for frame in 4: await process_frame
	_expect(prepare._xp.text == str(150 - step_xp), "applying spends the step's XP: %s" % prepare._xp.text)
	_expect(absf(float(prepare.board_view.saved.get("level", 0.0)) - float(prepare.board_view.staircase.steps[step].at)) < 1e-9, "the save holds the new level: %s vs %s" % [str(prepare.board_view.saved.get("level")), str(prepare.board_view.staircase.steps[step].at)])
	# No defense: Holdfast's 140 XP comes back.
	await _tap(_prepare_control(prepare, "prepare.board.defense.0.none"))
	_expect(int(prepare.board_view.xp.cost) == -140, "dropping Holdfast refunds 140 XP: %s" % str(prepare.board_view.xp.cost))
	await _tap(_prepare_control(prepare, "prepare.board.apply"))
	for frame in 4: await process_frame
	_expect(prepare._xp.text == str(150 - step_xp + 140), "no defense: %s XP" % prepare._xp.text)
	# A block defense costs its block chance × ½ × (total XP + 200).
	await _tap(_prepare_control(prepare, "prepare.board.defense.0.clean_deflect"))
	var total := int(prepare.board_view.xp.total)
	var block := roundi(1.0 / 6.0 * 0.5 * float(total + 200))
	_expect(int(prepare.board_view.xp.cost) == block, "Clean Deflect costs %d XP at %d total XP: %s" % [block, total, str(prepare.board_view.xp.cost)])
	_expect("blocks 17% of attacks" in _prepare_control(prepare, "prepare.board.defense.0.clean_deflect").text, "the block names its chance")
	await _tap(_prepare_control(prepare, "prepare.board.apply"))
	for frame in 4: await process_frame
	_expect(prepare._xp.text == str(150 - step_xp + 140 - block), "the block is bought: %s XP" % prepare._xp.text)
	_expect("Clean Deflect" in _prepare_control(prepare, "prepare.board").text, "the board row names the new defense")
	await _capture(prepare, "prepare-board-block")
	prepare.queue_free(); await process_frame

	# An expedition at camp: the board change opens the deck with Influence.
	var x: Dictionary = runtime.campaign_new("custom_adventurer", "Board camp", "expedition", "", 5)
	_expect(x.get("ok") == true, "a Custom Adventurer expedition starts: %s" % str(x.get("error", "")))
	if x.get("ok") != true: _finish(); return
	var save_id := str(x.result.id)
	var camp: Dictionary = runtime.expedition_act(save_id, {"kind": "make_camp"})
	_expect(camp.get("ok") == true, "make camp: %s" % str(camp.get("error", "")))
	var view: Dictionary = camp.result.view
	var fee := int(view.config.deck_change_influence.camp)
	var cards := []
	for card in view.hand.slice(0, fee): cards.append(str(card.id))
	var paid: Dictionary = runtime.expedition_act(save_id, {"kind": "camp_service", "service": "influence", "cards": cards}, int(camp.result.revision))
	_expect(paid.get("ok") == true and int(paid.result.view.turn.influence) == fee, "camp Influence paid: %s" % str(paid.get("error", "")))
	prepare = await _open_prepare(save_id, true, fee, fee)
	await _tap(_prepare_control(prepare, "prepare.board"))
	await _tap(_prepare_control(prepare, "prepare.board.level_up"))
	var label := _prepare_control(prepare, "prepare.board.apply").text
	_expect("opens the deck (%d Influence)" % fee in label, "Apply says it opens the deck: " + label)
	var cost := int(prepare.board_view.xp.cost)
	await _tap(_prepare_control(prepare, "prepare.board.apply"))
	for frame in 4: await process_frame
	_expect(prepare.expedition_open and prepare.expedition_influence == 0 and prepare._influence.text == "Open", "the board change spent the camp's Influence and opened the deck")
	_expect(prepare._xp.text == str(150 - cost), "the expedition spent the step's XP: %s" % prepare._xp.text)
	prepare.queue_free(); await process_frame

	# The Starter holds its own priced board in an expedition too.
	var s: Dictionary = runtime.campaign_new("starter", "Starter board", "expedition", "", 5)
	_expect(s.get("ok") == true, "a Starter expedition starts: %s" % str(s.get("error", "")))
	if s.get("ok") != true: _finish(); return
	var starter_id := str(s.result.id)
	camp = runtime.expedition_act(starter_id, {"kind": "make_camp"})
	view = camp.result.view
	cards = []
	for card in view.hand.slice(0, fee): cards.append(str(card.id))
	paid = runtime.expedition_act(starter_id, {"kind": "camp_service", "service": "influence", "cards": cards}, int(camp.result.revision))
	_expect(paid.get("ok") == true, "Starter camp Influence paid: %s" % str(paid.get("error", "")))
	prepare = await _open_prepare(starter_id, true, fee, fee)
	_expect(prepare._xp.text == "150", "the Starter keeps 150 XP to spend (its board is part of the kit)")
	row = _prepare_control(prepare, "prepare.board")
	_expect(row != null and "Starter" in row.text and "Holdfast" in row.text, "the Starter's deck list shows its ability board: %s" % (row.text if row != null else "missing"))
	await _tap(row)
	for frame in 4: await process_frame
	_expect(prepare.board_view.get("moving") != true, "a new Starter save already holds its board")
	_expect("yours" in _prepare_control(prepare, "prepare.board.defense.0.holdfast").text, "the saved defense is marked as yours")
	_expect("costs 47 XP" in _prepare_control(prepare, "prepare.board.defense.0.bastion").text, "Bastion shows what it costs over Holdfast: " + _prepare_control(prepare, "prepare.board.defense.0.bastion").text)
	_expect("93 XP back" in _prepare_control(prepare, "prepare.board.defense.0.light_guard").text, "Light Guard shows the XP it gives back: " + _prepare_control(prepare, "prepare.board.defense.0.light_guard").text)
	_expect("140 XP back" in _prepare_control(prepare, "prepare.board.defense.0.none").text, "No defense gives Holdfast's 140 XP back")
	# Raise one tier: Sword ×5 (the ladder's top) jumps the level to the
	# first step where it deals more, for that climb's XP.
	var raise: Dictionary = prepare.board_view.staircase.raises[2]
	var raise_button := _prepare_control(prepare, "prepare.board.raise.2")
	_expect(raise_button != null and ("· %d XP" % int(raise.xp)) in raise_button.text, "the tier shows what one more point costs: %s" % (raise_button.text if raise_button != null else "missing"))
	await _tap(raise_button)
	_expect(int(prepare.board_view.staircase.step) == int(raise.step) and int(prepare.board_view.xp.cost) == int(raise.xp), "raising the tier climbs to its step for %d XP: step %s cost %s" % [int(raise.xp), str(prepare.board_view.staircase.step), str(prepare.board_view.xp.cost)])
	_expect(int(prepare.board_view.staircase.steps[int(raise.step)].damage[2]) == int(raise.to), "the tier deals %d there" % int(raise.to))
	await _tap(_prepare_control(prepare, "prepare.board.reset"))
	await _tap(_prepare_control(prepare, "prepare.board.level_up"))
	cost = int(prepare.board_view.xp.cost)
	_expect(cost > 0, "the Starter's next step costs XP: %d" % cost)
	await _tap(_prepare_control(prepare, "prepare.board.apply"))
	for frame in 4: await process_frame
	_expect(prepare._xp.text == str(150 - cost) and prepare.expedition_open, "the Starter bought the step at camp: %s XP" % prepare._xp.text)
	var names := []
	for ability in prepare.data.abilities: names.append(str(prepare.catalog.get("abilities", {}).get(str(ability.id), {}).get("name", "")))
	_expect(not names.has("") and names.has("Holdfast") and names.size() == 7, "the Starter's abilities list names its board abilities: %s" % str(names))
	await _capture(prepare, "prepare-board-starter")
	prepare.queue_free(); await process_frame
	_finish()

func _open_prepare(save_id: String, expedition: bool, cost: int = 0, influence: int = 0) -> Control:
	var prepare = load(PREPARE_SCRIPT).new()
	prepare.save_id = save_id
	prepare.expedition = expedition; prepare.expedition_cost = cost; prepare.expedition_influence = influence
	root.add_child(prepare)
	for frame in 6: await process_frame
	return prepare

func _finish() -> void:
	print("CAMPAIGN BOARD: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
