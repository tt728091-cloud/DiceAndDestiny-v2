extends "res://tests/presentation/verify_card_creation_guided.gd"
# Revive trade: a program card that permanently removes itself to return one
# permanently removed card to hand. Authored through Card Creation, played by
# pointer in a native battle after real enemy damage removed cards.
const BATTLE := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIP := preload("res://presentation/cards/card_rules_tooltip.gd")
const CARD_ID := "ember_return"
const CARD_NAME := "Ember Return"
var canvas: SubViewport

func _run() -> void:
	root.size = Vector2i(1280, 720)
	var runtime = root.get_node("LearnedBattleRuntime")
	var characters = SCREEN.new(); root.add_child(characters); await _frames()
	characters._tabs.current_tab = 3; await _frames()
	await _author(characters.get_node("CardCreationWorkspace"))
	characters.queue_free(); await process_frame
	if failed: _finish(); return
	var deck: Array = runtime.character_catalogs("sandbox").result.adventurer.combatants.adventurer.decklist.duplicate(true)
	deck.append({"card_id": CARD_ID, "count": 3})
	_expect(runtime.save_character_deck("adventurer", deck).get("ok", false), "revive card joins the Adventurer deck")
	var found := _battle_with_wound(runtime)
	_expect(not found.is_empty(), "real enemy damage removed cards while a revive card is in hand")
	if found.is_empty(): _finish(); return
	await _play(found.gateway, found.result)
	_finish()

func _finish() -> void:
	print("REVIVE TRADE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

# The program editor offers the removed pile; a one-for-one trade validates.
func _author(ui: Control) -> void:
	await _click(_node(ui, "new")); await _frames()
	_line_edit(ui, "id", CARD_ID); _line_edit(ui, "name", CARD_NAME)
	ui._tabs.current_tab = 1; await _frames()
	_choose(_node(ui, "steps.effect"), "move_cards"); await _click_visible(ui, _node(ui, "steps.add")); await _frames()
	var removed := _node(ui, "steps.0.zones.removed") as CheckBox
	_expect(removed != null and removed.text == "Removed" and not removed.button_pressed, "Select cards from offers Removed")
	if removed == null: return
	await _click_visible(ui, removed); await _frames()
	await _click_visible(ui, _node(ui, "steps.0.zones.hand")); await _frames()
	_expect(ui.draft.program.steps[0].target.zones == ["removed"], "pointer selects only the removed pile: %s" % [ui.draft.program.steps[0].target.zones])
	await _click_visible(ui, _node(ui, "steps.0.exclude_recovery")); await _frames()
	_choose(_node(ui, "steps.0.param.destination"), "hand"); await _frames()
	ui._validate(); await _frames()
	_expect(ui._publish_button.disabled, "a free revive (played card discarded) is rejected: " + ui._error.text)
	ui._tabs.current_tab = 0; await _frames()
	_choose(_node(ui, "destination"), "removed"); await _frames()
	ui._validate(); await _frames()
	_expect(not ui._publish_button.disabled, "revive trade validates: " + ui._error.text)
	_expect(ui.draft.program.windows.any(func(w): return w in ["offensive_planning", "offensive_before_roll"]), "revive plays during offensive planning: %s" % [ui.draft.program.windows])
	_expect(ui._preview.text.contains("Revive one of your permanently removed cards") and ui._preview.text.contains("permanently removed instead of discarded"), "preview explains the trade: " + ui._preview.text)
	await _click(ui._publish_button)
	_expect(ui._error.text.begins_with("Published"), "publish revive card: " + ui._error.text)

# Drive the native battle until enemy damage left a wound of at least two
# revivable cards and the revive card offers them before the player's roll.
func _battle_with_wound(runtime: Node) -> Dictionary:
	for seed in range(1, 31):
		var gateway = GATEWAY.new(runtime, "seat-a", "brine-mask", "adventurer"); gateway.unified_defense = true
		var result: Dictionary = gateway.start_battle("revive-trade-%d" % seed, seed)
		for step in 300:
			if not result.get("accepted", false) or str(result.snapshot.get("winner", "")) != "": break
			var blade: Dictionary = result.snapshot.actors.blade
			if int(blade.removed_count) >= 2 and _revive_targets(result).size() >= 2: return {"gateway": gateway, "result": result}
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
			var next := _next(result.get("legal_actions", []))
			if next.is_empty(): break
			result = gateway.submit(JSON.stringify(next))
	return {}

func _revive_targets(result: Dictionary, instance := "") -> Array:
	var targets := []
	for action in result.get("legal_actions", []):
		var payload: Dictionary = action.get("payload", {})
		var ids: Array = payload.get("card_ids", [])
		if ids.size() != 1 or result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") != CARD_ID: continue
		if not instance.is_empty() and ids[0] != instance: continue
		var choice = JSON.parse_string(str(payload.get("status_id", "")))
		if choice is Dictionary and choice.get("then", "") != "": targets.append(choice)
	return targets

func _next(actions: Array) -> Dictionary:
	for kind in ["roll_dice", "planning_pass", "pass", "planning_roll", "planning_select_ability"]:
		for action in actions:
			if action.type == kind: return action
	return {}

func _play(gateway, result: Dictionary) -> void:
	canvas = SubViewport.new(); canvas.size = Vector2i(1920, 1080); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas); canvas.notify_mouse_entered()
	var blade: Dictionary = result.snapshot.actors.blade.duplicate(true)
	var revive := ""
	for id in blade.hand:
		if blade.card_instances[id].definition_id == CARD_ID: revive = id; break
	var targets := _revive_targets(result, revive)
	var old_wound: Dictionary = result.snapshot.wounds.filter(func(w): return w.target_actor_id == "blade")[0].duplicate(true)
	var screen = BATTLE.instantiate(); screen.gateway = gateway; screen.initial_result = result; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("revive-trade.json")); canvas.add_child(screen); screen.set_process(false)
	await _settle(screen)
	var profile = screen._actor_profiles.blade
	var health_before: float = profile.health.value
	var removed_before := int(blade.removed_count)
	var hand_before := int(blade.hand.size())
	_expect(health_before == float(blade.current_health) and health_before < profile.health.max_value, "displayed health shows the real wound")
	_expect(profile.wound_bar.segments.size() == 1 and int(profile.wound_bar.segments[0].get_meta("damage")) == old_wound.cards.size(), "wound bar shows the enemy hit")
	var hand = screen._hand_dock; hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == revive: index = i
	_expect(index >= 0, "revive card is visible in hand")
	if index < 0: return
	# Rules on hover: the trade and the self-removal.
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	await _move(point); await create_timer(1.2).timeout
	var tip := _find_tip(canvas)
	if tip == null: await _move(point + Vector2(1, 1)); await create_timer(1.2).timeout; tip = _find_tip(canvas)
	var tooltip: String = hand.cards[index].tooltip_text
	_expect(tip != null and tip.text == tooltip, "hovering the revive card opens its rules")
	for line in ["Revive one of your permanently removed cards", "heals 1 health of the wound that removed it", "permanently removed instead of discarded (−1 health)"]:
		_expect(line in tooltip, "rules mention: " + line)
	var face := _text(hand.cards[index])
	_expect("Revive 1 removed card" in face and "Removes itself" in face, "face summarises the trade: " + face)
	# Card-first: the click offers every revivable removed card.
	await _click_point(point); await _frames()
	var dialogs: Array = screen.find_children("*", "AcceptDialog", false, false).filter(func(d): return d.visible)
	_expect(dialogs.size() == 1, "card click opens the removed-card chooser")
	if dialogs.is_empty(): return
	var dialog: AcceptDialog = dialogs[0]
	var choices: Array = dialog.find_children("*", "Button", true, false).filter(func(b): return b.text.ends_with("· removed"))
	_expect(choices.size() == targets.size() and choices.size() >= 2, "chooser lists each revivable removed card: %s" % [choices.map(func(b): return b.text)])
	var names: Array = old_wound.cards.map(func(c): return "%s · removed" % BattlePresentationCatalog.card(str(c.card_definition_id)).name)
	_expect(choices.all(func(b): return b.text in names), "choices name the removed cards")
	_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(dialog.position, dialog.size)), "chooser stays onscreen")
	var chosen: Button = choices[0]
	var chosen_choice: Dictionary = targets.filter(func(c): return c.label == chosen.text)[0]
	var revived := str(chosen_choice.card)
	await _click_point(Vector2(dialog.position) + chosen.get_global_rect().get_center())
	# The revived card flies into hand from the removed pile, never the deck.
	var notices: Array = screen.get_children().filter(func(n): return n.get_meta("feedback_notice", false) and n.get("_draw_sources") != null)
	_expect(notices.size() == 1 and notices[0]._draw_sources.get(revived) == "removed", "revived card's flight starts at the removed pile")
	# Let the real presentation play the trade: health never moves.
	screen.set_process(true)
	var low := 999.0; var high := -1.0; var start := Vector2.INF
	for frame in 300:
		await process_frame
		var shown = screen._actor_profiles.get("blade")
		if is_instance_valid(shown): low = minf(low, shown.health.value); high = maxf(high, shown.health.value)
		if start == Vector2.INF and not notices.is_empty() and is_instance_valid(notices[0]) and notices[0]._draw_cards.has(revived):
			var flight: BattleCard = notices[0]._draw_cards[revived]
			if flight.visible: start = flight.get_global_transform_with_canvas() * (flight.size * 0.5)
		if not screen._director.has_beats() and frame > 120 and (notices.is_empty() or not is_instance_valid(notices[0])): break
	_expect(low == health_before and high == health_before, "health stays %s during the trade animation (%s..%s)" % [health_before, low, high])
	var cells: Dictionary = screen._actor_profiles.blade._stat_cells
	_expect(start != Vector2.INF and start.distance_to(cells.removed.get_global_rect().get_center()) < start.distance_to(cells.deck.get_global_rect().get_center()), "flight leaves from the removed pile: %s" % start)
	screen.set_process(false)
	await _settle(screen)
	var after: Dictionary = screen._view.actor("blade")
	profile = screen._actor_profiles.blade
	_expect(revived in after.hand and screen._error_message.is_empty(), "chosen card returns to hand")
	var revived_definition := str(blade.card_instances[revived].definition_id)
	_expect(int(after.removed_composition.get(revived_definition, 0)) == int(blade.removed_composition.get(revived_definition, 0)) - 1, "revived card leaves the removed pile")
	_expect(revive not in after.hand, "revive card leaves hand")
	_expect(int(after.removed_count) == removed_before and int(after.hand.size()) == hand_before, "one card out, one in: removed %d→%d hand %d→%d" % [removed_before, int(after.removed_count), hand_before, after.hand.size()])
	_expect(int(after.removed_composition.get(CARD_ID, 0)) == 1, "revive card ends in the removed pile")
	_expect(int(after.current_health) == int(blade.current_health) and profile.health.value == health_before, "health unchanged by the trade")
	_expect(screen._hand_dock.cards.any(func(c): return c.instance_id == revived) and not screen._hand_dock.cards.any(func(c): return c.instance_id == revive), "hand dock shows the revived card, not the spent one")
	_expect(profile._stat_cells.removed.tooltip_text.contains("Removed"), "removed pile keeps its hover text")
	# The wound that removed it shrinks; the card's own removal is a new wound.
	var wounds: Array = screen._view.raw_snapshot.wounds.filter(func(w): return w.target_actor_id == "blade")
	var shrunk: Array = wounds.filter(func(w): return w.id == old_wound.id)
	_expect(shrunk.size() == (1 if old_wound.cards.size() > 1 else 0), "old wound kept only while it still has cards")
	if not shrunk.is_empty():
		_expect(shrunk[0].cards.size() == old_wound.cards.size() - 1 and not shrunk[0].cards.any(func(c): return c.card_id == revived), "revived card leaves its wound")
	var trade: Array = wounds.filter(func(w): return w.get("source_content_id") == CARD_ID)
	_expect(trade.size() == 1 and trade[0].cards.size() == 1 and trade[0].cards[0].card_id == revive, "revive card's own removal is a new wound")
	var segments: Array = profile.wound_bar.segments
	_expect(segments.size() == wounds.size() and segments.size() == 2, "wound bar redraws one segment per remaining wound")
	var unit: float = (profile.health.size.x - 10) / profile.health.max_value
	var missing := int(profile.health.max_value - profile.health.value)
	_expect(segments.reduce(func(total, s): return total + int(s.get_meta("damage")), 0) == missing, "wound segments still cover exactly the missing health")
	for segment in segments:
		_expect(segment.visible and profile.health.get_global_rect().encloses(segment.get_global_rect()), "wound segment visible inside health bar")
		_expect(absf(segment.size.x - unit * int(segment.get_meta("damage"))) < 0.1, "segment width matches its cards")
		await _move(segment.get_global_rect().get_center())
		_expect(canvas.gui_get_hovered_control() == segment, "pointer reaches wound segment")
	var old_segment: Button = segments.filter(func(s): return s.get_meta("wound_id") == old_wound.id).front()
	var new_segment: Button = segments.filter(func(s): return s.get_meta("wound_id") == trade[0].id).front() if not trade.is_empty() else null
	if old_segment != null:
		_expect(int(old_segment.get_meta("damage")) == old_wound.cards.size() - 1 and ("%d damage" % (old_wound.cards.size() - 1)) in old_segment.tooltip_text, "old wound tooltip counts its remaining cards: " + old_segment.tooltip_text)
		_expect("Brine" in old_segment.tooltip_text or "Round" in old_segment.tooltip_text, "old wound still names its round and source")
	_expect(new_segment != null and "1 damage" in new_segment.tooltip_text and CARD_NAME in new_segment.tooltip_text, "new wound names the revive card: " + (new_segment.tooltip_text if new_segment else "missing"))
	if new_segment != null:
		var bubble: Control = new_segment._make_custom_tooltip(new_segment.tooltip_text); screen._root.add_child(bubble)
		await _frames(); _expect(bubble.size.x <= 440 and bubble.size.y < canvas.size.y, "wound tooltip wraps within viewport"); bubble.queue_free()
	# Rebuilds keep the trade intact.
	screen._render(); await _frames(); profile = screen._actor_profiles.blade
	_expect(profile.wound_bar.segments.size() == 2 and profile.health.value == health_before, "redraw retains both wounds and health")
	if DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; DirAccess.make_dir_recursive_absolute("res://.godot/layout-review")
		canvas.get_texture().get_image().save_png("res://.godot/layout-review/revive-trade.png")
	screen.queue_free(); await process_frame

func _settle(screen: Control) -> void:
	for frame in 6: await process_frame
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	for frame in 6: await process_frame
func _move(point: Vector2) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
func _click_point(point: Vector2) -> void:
	await _move(point)
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; canvas.push_input(click, true); await process_frame
func _find_tip(node: Node) -> Label:
	if node is Label and node.get_script() == TIP and node.is_visible_in_tree(): return node
	for child in node.get_children(true):
		var found := _find_tip(child)
		if found != null: return found
	return null
