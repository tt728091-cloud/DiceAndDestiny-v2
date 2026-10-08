extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TARGETING := preload("res://presentation/battle/die_face_targeting.gd")
const NOTICE := preload("res://presentation/battle/curse_notice.gd")
var failed := false
class RecordedGateway extends RefCounted:
	var result: Dictionary
	var commands: Array = []
	func submit(command: String) -> Dictionary:
		commands.append(JSON.parse_string(command)); return result.duplicate(true)
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var cases := {}
	# Known native seeds: 367/532/550/648 hit, 47/92/103/109/148 miss. Fall
	# back to a wider sweep in case catalog changes shift the outcomes.
	var seeds: Array = [367, 47, 532, 92, 550, 103, 648, 109, 148]
	seeds.append_array(range(1, 401))
	for seed_value in seeds:
		var base: Dictionary = gateway.start_battle("unquiet-%d" % seed_value, seed_value)
		var mark := _action(base, "mark_the_number")
		if mark.is_empty(): continue
		base = gateway.submit(JSON.stringify(mark))
		var action := _action(base, "unquiet_hands")
		if action.is_empty(): continue
		var after: Dictionary = gateway.submit(JSON.stringify(action))
		var roll := {}
		for event in after.events:
			if event.get("type") == "curse_resolved" and event.get("data", {}).get("source_card_id") == "unquiet_hands" and event.data.kind == "owned_roll": roll = event.data
		if roll.is_empty(): continue
		var key := "hit" if roll.cursed else "miss"
		if not cases.has(key): cases[key] = {"before": base, "after": after, "action": action}
		if cases.size() == 2: break
	_expect(cases.size() == 2, "real native plays produce both hit and miss")
	for key in cases:
		for width in [1280, 1920]: await _check(cases[key], key, width)
	print("UNQUIET HANDS FEEDBACK: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check(record: Dictionary, outcome: String, width: int) -> void:
	root.size = Vector2i(width, int(width * 9 / 16.0))
	var before: Dictionary = record.before.duplicate(true)
	var last := 0
	for event in before.events: last = maxi(last, int(event.get("sequence", 0)))
	before.events = []; before.learned_policy.model_turn = false
	var recorder := RecordedGateway.new(); recorder.result = record.after.duplicate(true); recorder.result.learned_policy.model_turn = false
	var screen = SCREEN.instantiate(); screen.initial_result = before; screen.last_presented_sequence = last; screen.gateway = recorder; screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("unquiet-review.json")); root.add_child(screen); await process_frame
	screen.set_process(false)
	var id := str(record.action.payload.card_ids[0])
	var card := BattleCard.new(); card.instance_id = id; card.definition_id = "unquiet_hands"
	screen._on_card_pressed(card); await process_frame
	var selector = _selector(screen)
	_expect(selector != null, "card starts inline targeting before enemy rolls")
	if selector == null: card.free(); screen.queue_free(); await process_frame; return
	await create_timer(0.85).timeout
	_expect(recorder.commands.is_empty(), "card click waits for chosen die")
	_expect(selector.targets.size() == 1 and selector.faces.is_empty(), "only legal cursed die highlighted, no face menu")
	_expect(screen._sole_pass_action().is_empty(), "auto pass waits for targeting")
	var hand_cards: Array = screen._hand_dock.find_children("*", "Button", true, false).filter(func(node): return node is BattleCard and node.instance_id == id)
	_expect(hand_cards.size() == 1 and hand_cards[0].button_pressed, "chosen card stays selected")
	_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), "inline hint fits viewport")
	await _capture("targets-%s-%d" % [outcome, width])
	# Revalidate changes to the legal target set without spending the card.
	var legal: Array = screen._view.legal_actions.duplicate(true)
	var additional: Dictionary = record.action.duplicate(true); additional.payload.status_id = "4"
	screen._view.legal_actions.append(additional); screen._render(); await process_frame
	selector = _selector(screen)
	_expect(selector.targets.size() == 2, "all legal targets highlight independently")
	screen._view.legal_actions = []; screen._render(); await process_frame
	_expect(_selector(screen) == null and screen._selected_card.is_empty(), "lost legality clears stale targeting")
	screen._view.legal_actions = legal; screen._on_card_pressed(card); await process_frame; selector = _selector(screen)
	var escape := InputEventKey.new(); escape.keycode = KEY_ESCAPE; escape.pressed = true; selector._unhandled_key_input(escape); await process_frame
	_expect(_selector(screen) == null and recorder.commands.is_empty(), "Escape cancels without spending")
	screen._on_card_pressed(card); card.free(); await process_frame
	screen._render(); await process_frame; selector = _selector(screen)
	_expect(selector != null, "redraw preserves targeting")
	selector.targets.values()[0].pressed.emit(); await process_frame
	_expect(recorder.commands.size() == 1 and recorder.commands[0] == record.action, "die click submits exact authoritative action")
	_expect(_selector(screen) == null and screen._error_message.is_empty(), "accepted play removes targeting")
	var notices: Array = screen.get_children().filter(func(node): return node.get_script() == NOTICE)
	_expect(notices.size() == 1, "card gets exactly one separate roll animation")
	if notices.is_empty(): screen.queue_free(); await process_frame; return
	var notice = notices[0]; notice.set_process(false)
	await create_timer(0.85).timeout
	_expect(not notice._waiting(), "feedback is ready without a popup")
	var tray: BattleDiceTray = screen._enemy_dice_dock.get_child(0)
	var saved := tray._dice.duplicate(true)
	var old_text := []
	for number in tray._numbers: old_text.append(number.text)
	var seen := {}
	for tick in range(21, 54, 4):
		notice._elapsed = notice.duration * tick / 100.0; notice.refresh()
		seen[notice._roll.text] = true
		_expect(tray._dice == saved, "extra check preserves offensive data")
		for i in old_text.size(): _expect(tray._numbers[i].text == old_text[i], "unrevealed tray stays unrevealed")
	_expect(seen.size() > 1 and notice._roll.visible, "separate preview visibly cycles faces")
	_expect(notice._die_rect.size.x > 0 and is_instance_valid(notice._card), "played card points to exact physical die")
	await _capture("rolling-%s-%d" % [outcome, width])
	if outcome == "hit":
		_expect(not "Curse Count ×1" in screen._actor_profiles.goblin.statuses.text, "Count waits for result trail")
	notice._elapsed = notice.duration * 0.82; notice.refresh(); await process_frame
	_expect("Offensive result unchanged" in notice._label.text, "caption distinguishes extra roll")
	_expect(("Curse hit · +1 Count" if outcome == "hit" else "Clean face · no Count") in notice._label.text, "caption explains actual outcome")
	_expect(("⌁" in notice._roll.text) == (outcome == "hit"), "only cursed result shows negative symbol")
	_expect((notice._count_target.size.x > 0) == (outcome == "hit"), "only a hit travels to Count")
	if outcome == "hit": _expect("Curse Count ×1" in screen._actor_profiles.goblin.statuses.text, "Count increments at arrival")
	_expect(root.get_visible_rect().encloses(notice._label.get_global_rect()), "outcome caption fits")
	await _capture("result-%s-%d" % [outcome, width])
	# A revealed tray must also retain its saved faces throughout this extra roll.
	var revealed := []
	for face in [3, 3, 1, 5, 3]: revealed.append({"die_id": "brine_d6", "face": face})
	tray.display(revealed)
	for tick in [0.1, 0.3, 0.6, 0.82]:
		notice._elapsed = notice.duration * tick; notice.refresh()
		for i in 5: _expect(tray._numbers[i].text == str(revealed[i].face), "revealed offensive face stays fixed during check")
	var elapsed: float = notice._elapsed
	screen._snapshot_panel_open = true; notice._process(0.5)
	_expect(notice._elapsed == elapsed and notice.modulate.a == 0, "inspection pauses the roll")
	screen._snapshot_panel_open = false; screen._render(); await process_frame
	_expect(is_instance_valid(notice) and notice._elapsed == elapsed, "redraw preserves playback")
	screen._director.queue_result(record.after, screen._director.last_sequence())
	_expect(screen._director.take_curse_updates().is_empty(), "duplicate delivery does not repeat")
	notice._process(notice.duration); await process_frame
	_expect(not is_instance_valid(notice), "feedback finishes automatically")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _action(result: Dictionary, id: String) -> Dictionary:
	for action in result.get("legal_actions", []):
		var ids: Array = action.get("payload", {}).get("card_ids", [])
		if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == id: return action
	return {}
func _selector(screen):
	for node in screen._root.get_children():
		if node.get_script() == TARGETING: return node
	return null
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_UNQUIET_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("UNQUIET HANDS: " + message)
