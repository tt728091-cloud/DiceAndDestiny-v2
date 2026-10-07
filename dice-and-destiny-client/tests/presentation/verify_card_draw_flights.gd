extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.size = Vector2i(1920,1080); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var base: Dictionary = gateway.start_battle("draw-flight", 43)
	for width in [1920, 1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		for count in [0, 1, 2]: await _scenario(base, count)
	var native_checked := false
	# Exercise a real authoritative draw in addition to short-deck fixtures.
	for seed in range(1, 40):
		var live: Dictionary = gateway.start_battle("draw-flight-native", seed)
		var action := {}
		for candidate in live.legal_actions:
			var ids: Array = candidate.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and live.snapshot.actors.blade.card_instances[ids[0]].definition_id == "take_stock": action = candidate; break
		if action.is_empty(): continue
		native_checked = true
		live.events = []; live.learned_policy = {}
		var screen = SCREEN.instantiate(); screen.initial_result = live; screen.gateway = gateway; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("draw-flight-native.json")); canvas.add_child(screen)
		await process_frame
		var health: int = screen._view.actor("blade").current_health
		screen._send(JSON.stringify(action))
		var notices := screen.get_children().filter(func(node): return node.get_script() == NOTICE)
		_expect(notices.size() == 1 and notices[0]._draw_cards.size() == 2, "native Take Stock draws exactly two public cards")
		_expect(screen._view.actor("blade").current_health == health, "drawing and discarding played card preserve health")
		screen.active_store.clear(); screen.queue_free(); await process_frame
		break
		
	_expect(native_checked, "native Take Stock scenario exercised")
	print("CARD DRAW FLIGHTS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(base: Dictionary, count: int) -> void:
	var before := base.duplicate(true); before.events = []; before.learned_policy = {}
	before.snapshot.segment = "offensive"; before.snapshot.stage = "planning"
	var actor: Dictionary = before.snapshot.actors.blade
	actor.hand = ["left", "played", "right"]; actor.hand_count = 3; actor.deck_count = count; actor.discard_count = 4
	actor.card_instances = {"left":{"definition_id":"brace"}, "played":{"definition_id":"take_stock"}, "right":{"definition_id":"nudge"}}
	var action := {"type":"planning_commit_cards", "actor_id":"blade", "battle_id":before.snapshot.battle_id, "payload":{"card_ids":["played"], "pending_input_id":"draw-input"}}
	before.legal_actions = [action]; before.pending_input = {"blade":{"id":"draw-input", "input_type":"planning", "allowed_commands":["planning_commit_cards"]}}
	var after := before.duplicate(true); after.accepted = true; after.legal_actions = []
	after.snapshot.actors.blade.hand = ["left", "right"]; after.snapshot.actors.blade.hand_count = 2 + count
	after.snapshot.actors.blade.energy_points -= 1
	after.snapshot.actors.blade.deck_count = 0; after.snapshot.actors.blade.discard_count = 5
	var ids: Array = []
	for i in count:
		var id := "drawn-" + str(i); ids.append(id)
		after.snapshot.actors.blade.hand.append(id)
		after.snapshot.actors.blade.card_instances[id] = {"definition_id":"brace" if i == 0 else "take_stock"}
	after.events = [{"sequence":100,"type":"card_played","actor_id":"blade","energy_cost":1,"data":{"card_definition_id":"take_stock","card_instance_id":"played"}}, {"sequence":101,"type":"cards_drawn","actor_id":"blade","cards":ids,"count":count,"deck_empty":true}]
	var fake := FakeBattleAuthority.new(); fake.enqueue(after)
	var screen = SCREEN.instantiate(); screen.initial_result = before; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("draw-flight-fixture.json")); canvas.add_child(screen); screen.set_process(false)
	for frame in 12: await process_frame
	screen._flow_until = 0
	for entry in screen._timed_buttons: entry.until = 0
	screen._process(0)
	var hand: Control = screen._hand_dock
	hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[1] * Vector2(90,60))
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	var pose: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse() * hand.cards[1].get_global_transform_with_canvas()
	for pressed in [true,false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; click.position = point; canvas.push_input(click,true)
	var notices := screen.get_children().filter(func(node): return node.get_script() == NOTICE)
	_expect(fake.commands.size() == 1 and notices.size() == 1, "pointer plays Take Stock and starts one animation")
	if notices.is_empty(): screen.queue_free(); await process_frame; return
	var notice = notices[0]; notice.set_process(false)
	for frame in 3: await process_frame
	notice._started = true; notice._elapsed = 0.01; notice.refresh()
	_expect(notice._cards[0].position.distance_to(pose.origin) < 1, "played card retains exact clicked hand position")
	_expect(is_equal_approx(notice._cards[0].rotation, pose.get_rotation()), "played card retains fan angle")
	_expect(notice._draw_cards.size() == count, "flights match actual draw count including empty deck")
	_expect(screen._actor_profiles.blade._stat_labels.hand.text == "2", "hand counter waits for draws")
	for card in screen._hand_dock.cards:
		if card.instance_id in ids: _expect(card.modulate.a == 0, "drawn card never flashes in destination")
	notice._elapsed = notice.duration * 0.51; notice.refresh()
	_expect(notice._cards[0].modulate.a < 0.1, "played card fades while green arc resolves")
	if count > 0:
		_expect(notice._draw_cards[ids[0]].visible, "first drawn card flies toward hand")
		_expect(notice._draw_cards[ids[0]].position.y > 650, "flight stays near player HUD: %s, hand %s, profile %s" % [notice._draw_cards[ids[0]].position, screen._hand_dock.get_global_rect(),screen._actor_profiles.blade.anchor_rect("deck")])
		_expect(screen._actor_profiles.blade._stat_labels.hand.text == "3", "counter increments for first actual draw")
		if count == 2: _expect(not notice._draw_cards[ids[1]].visible, "second draw is staggered")
	var path := OS.get_environment("DICE_AND_DESTINY_DRAW_SCREENSHOTS")
	if not path.is_empty() and DisplayServer.get_name() != "headless" and count == 2:
		await RenderingServer.frame_post_draw; canvas.get_texture().get_image().save_png(path + "/draw-flight-%d.png" % canvas.size.x)
	if count == 2:
		notice._elapsed = notice.duration * 0.70; notice.refresh()
		_expect(not notice._draw_cards[ids[0]].visible and notice._draw_cards[ids[1]].visible, "second card flies after first arrives")
		_expect(screen._actor_profiles.blade._stat_labels.hand.text == "4", "counter increments again for second draw")
	screen._render(); notice.refresh()
	for frame in 3: await process_frame
	_expect(notice._draw_cards.size() == count, "rebuild retains same flights without replay")
	notice._elapsed = notice.duration * 0.9; notice.refresh()
	_expect(screen._actor_profiles.blade._stat_labels.hand.text == str(2+count), "final hand count agrees with authority")
	_expect(screen._actor_profiles.blade._stat_labels.deck.text == "0", "final deck count agrees with authority")
	for card in screen._hand_dock.cards: _expect(card.modulate.a == 1, "arrived cards are fully visible")
	for card in notice._draw_cards.values(): _expect(not card.visible, "flight copies disappear after arrival")
	notice._elapsed = notice.duration; notice.refresh(); await process_frame
	_expect(not screen._hand_dock.gain_animation_active, "hand returns to normal visibility behavior")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CARD DRAW FLIGHTS: " + message)
