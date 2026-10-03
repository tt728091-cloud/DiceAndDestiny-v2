extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const FOLDOUT := preload("res://presentation/battle/number_face_foldout.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	for id in ["shared_misfortune", "rotten_numeral"]:
		var base := {}; var instance := ""
		for seed_value in range(1, 101):
			base = gateway.start_battle("foldout-%s-%d" % [id, seed_value], seed_value)
			for action in base.get("legal_actions", []):
				var ids: Array = action.get("payload", {}).get("card_ids", [])
				if ids.size() == 1 and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == id: instance = ids[0]; break
			if not instance.is_empty(): break
		_expect(not instance.is_empty(), "native hand contains card")
		if instance.is_empty(): continue
		base.events = []; base.learned_policy = {}
		var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = gateway; screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("foldout-test.json")); root.add_child(screen); await process_frame
		var card := BattleCard.new(); card.instance_id = instance; card.definition_id = id
		var energy: int = screen._view.actor("blade").energy_points
		screen._on_card_pressed(card); await process_frame
		var selector = _selector(screen)
		_expect(selector != null and selector.faces.size() == 6, "six faces shown without modal")
		_expect(screen._sole_pass_action().is_empty(), "selection blocks auto pass")
		_expect(screen._view.actor("blade").energy_points == energy, "opening selector spends nothing")
		for viewport in [Vector2i(1280, 720), Vector2i(1920, 1080)]:
			root.size = viewport
			for frame in 10: await process_frame
			selector._elapsed = 1; selector.refresh(); await process_frame
			_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), "foldout fits viewport")
			_expect(not selector.panel.get_global_rect().intersects(screen._enemy_dice_dock.get_global_rect()), "foldout sits to left of dice")
			_expect(not selector.panel.get_global_rect().intersects(screen._hand_dock.get_global_rect()), "hand remains accessible")
			for face in range(1, 7): _expect(not selector.faces[face].disabled, "native valid face enabled")
			var dir := OS.get_environment("DICE_AND_DESTINY_FOLDOUT_SCREENSHOTS")
			if not dir.is_empty() and DisplayServer.get_name() != "headless":
				await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join("%s-%d.png" % [id, viewport.x]))
		# Click card again to cancel, then reopen without spending it.
		screen._on_card_pressed(card); await process_frame
		_expect(_selector(screen) == null and screen._selected_card.is_empty(), "repeat card click cancels")
		_expect(screen._view.actor("blade").energy_points == energy, "cancel spends nothing")
		screen._on_card_pressed(card); await process_frame
		var legal: Array = screen._view.legal_actions.duplicate(true)
		var fifth: Dictionary = screen._number_face_actions()[5]
		screen._view.legal_actions = legal.filter(func(action): return action != fifth)
		screen._render(); await process_frame
		_expect(_selector(screen).faces[5].disabled, "unavailable face disabled")
		screen._commit_number_face(5)
		_expect(screen._view.actor("blade").energy_points == energy, "stale face cannot submit")
		screen._view.legal_actions = legal; screen._render(); await process_frame
		_selector(screen).faces[5].pressed.emit(); await process_frame
		_expect(screen._error_message.is_empty(), "authority accepts face click")
		_expect(screen._view.actor("blade").energy_points == energy - int(BattlePresentationCatalog.card(id).cost), "play spends the card cost exactly once")
		_expect(_selector(screen) == null, "successful play closes foldout")
		var marked := 0
		for die in screen._view.actor("goblin").owned_dice:
			if die.get("cursed_faces") != null and die.cursed_faces.any(func(face): return int(face) == 5): marked += 1
		_expect(marked == (2 if id == "shared_misfortune" else 3), "chosen number marks correct count of enemy dice")
		var notices := screen.get_children().filter(func(child): return child.get_script() == preload("res://presentation/battle/curse_notice.gd"))
		_expect(notices.size() == 1, "card creates one shared face animation")
		if not notices.is_empty():
			var notice = notices[0]; notice.set_process(false)
			notice._elapsed = notice.duration * 0.57; notice.refresh(); await process_frame; notice.refresh()
			var capture_dir := OS.get_environment("DICE_AND_DESTINY_FOLDOUT_SCREENSHOTS")
			if not capture_dir.is_empty() and DisplayServer.get_name() != "headless":
				await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(capture_dir.path_join(id + "-batch.png"))
			_expect(notice._batch_targets.size() == marked and notice._face_batch_end() == marked, "native card shows every face in one beat")
			notice._process(notice.duration * 0.44); await process_frame
			_expect(not is_instance_valid(notice), "native multi-face animation finishes after one duration")
		card.free(); screen.active_store.clear(); screen.queue_free(); await process_frame
	print("NUMBER FACE FOLDOUT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _selector(screen):
	for child in screen._root.get_children():
		if child.get_script() == FOLDOUT: return child
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("NUMBER FACE FOLDOUT: " + message)
