extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("intent-counts", 43)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for stage in ["offensive_reaction", "defense_selection", "defense_reaction"]:
			for tier in ["skull_3", "skull_4", "skull_5"]:
				var fixture := base.duplicate(true)
				fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
				fixture.snapshot.stage = stage
				fixture.snapshot.segment = "offensive" if stage == "offensive_reaction" else "defensive"
				fixture.snapshot.damage_sources = []
				for actor in ["blade", "goblin"]:
					fixture.snapshot.actors[actor].selected_ability = "hexbrand"
					fixture.snapshot.actors[actor].selected_tier = tier
					var target := "goblin" if actor == "blade" else "blade"
					fixture.snapshot.damage_sources.append({"id": actor + "-attack", "source_actor_id": actor, "target_actor_id": target, "source_content_id": "hexbrand", "base_amount": 4, "status_applications": [{"target_actor_id": target, "status_id": "poison", "stacks": 1}, {"target_actor_id": target, "status_id": "poison", "stacks": 2}, {"target_actor_id": target, "status_id": "bleed", "stacks": 1}]})
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("intent-counts.json"))
				root.add_child(screen); screen.set_process(false)
				for frame in 6: await process_frame
				for actor in ["blade", "goblin"]:
					var panel: Control = screen._attack_intents[actor + "-attack"]
					var expected := str(int(tier.trim_prefix("skull_")) - 2)
					_expect(_count(panel, "curse_count") == "×" + expected, "Curse amount follows selected tier for " + actor + " in " + stage)
					_expect(_count(panel, "poison") == "×3", "repeated Poison applications combine into one visible count")
					_expect(_count(panel, "bleed") == "×1", "single-stack effects retain explicit counts")
					_expect(root.get_visible_rect().encloses(panel.intent.get_global_rect()), "combined effect counts fit the viewport")
				if stage == "defense_selection" and tier == "skull_4": await _capture("effect-counts-" + str(viewport.x))
				screen.queue_free(); await process_frame
	var grasp := BattlePresentationCatalog.ability_followup_intents("grasp_of_the_sarcophagus")
	_expect(grasp.size() == 2 and grasp[0].count == "2" and grasp[1].count == "1", "Grasp distinguishes two Curse applications and one Entomb")
	var rattle := BattlePresentationCatalog.ability_followup_intents("funeral_rattle")
	_expect(rattle[0].icon == "dice" and rattle[0].count == "≤3" and rattle[1].count == "1?", "random rolls and conditional Blind do not pretend to be guaranteed stacks")
	print("INTENT EFFECT COUNTS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _count(panel: Control, effect: String) -> String:
	var children: Array = panel.intent_row.get_children()
	for i in range(children.size() - 1):
		if children[i].get_meta("intent_effect", "") == effect and children[i + 1] is Label:
			return children[i + 1].text
	return ""
func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_INTENT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.1).timeout
	RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("INTENT EFFECT COUNTS: " + message)
