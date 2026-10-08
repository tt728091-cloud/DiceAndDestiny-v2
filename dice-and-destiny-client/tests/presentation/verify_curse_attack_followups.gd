extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	base = native.start_battle("curse-attack-followups", 1788900535520209)
	_expect(base.get("accepted", false), "native Curse catalog loads")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for tier in [3, 4, 5]:
			for stage in ["defense_selection", "defense_reaction", "damage_reaction"]:
				await _check(tier, stage, viewport)
	print("CURSE ATTACK FOLLOWUPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check(tier: int, stage: String, viewport: Vector2i) -> void:
	var f := base.duplicate(true); f.events = []; f.learned_policy = {}; f.legal_actions = []; f.pending_input = {}
	f.snapshot.stage = stage; f.snapshot.segment = "damage_resolution" if stage == "damage_reaction" else "defensive"
	f.snapshot.actors.blade.selected_ability = "hexbrand"; f.snapshot.actors.blade.selected_tier = "skull_%d" % tier
	# Five Skulls can choose a lower tier; modified/blocked damage must not
	# determine the number of Curse applications shown on the target panel.
	f.snapshot.actors.blade.dice = []
	for i in 5: f.snapshot.actors.blade.dice.append({"index": i, "die_id": "curse_d6", "face": 1})
	var source := {"id": "hexbrand-hit", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "hexbrand", "base_amount": 9 if tier == 3 else tier, "prevention": 5 if stage == "damage_reaction" else 0, "final_amount": 0}
	var incoming := {"id": "brine-hit", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 3, "final_amount": 3}
	f.snapshot.damage_sources = [incoming, source]
	f.snapshot.settled_damage = {"id": "hexbrand-batch", "sources": [source], "removals": [], "status_applications": []}
	if stage == "defense_reaction": f.snapshot.defense_selections = {"goblin": {"actor_id": "goblin", "source_id": "hexbrand-hit", "ability_id": "basic_defense", "rolled_faces": [6], "rolled_face": 6}}
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("curse-followups.json")); root.add_child(screen)
	for frame in 8: await process_frame
	var expected := "Apply %d Curse after damage (even if blocked)" % (tier - 2)
	var shown := 0
	# The attack panel body is hidden and defense result panels sit on _root;
	# the follow-up's visible form is the intent icon + count, covered by
	# verify_intent_effect_counts and verify_hexbrand_curse_sequence.
	for panel in screen._root.find_children("*", "PanelContainer", true, false):
		if panel.get_script() != screen.DEFENSE_RESULT: continue
		if panel.data.source_id == "hexbrand-hit":
			shown += 1
			_expect(panel.data.attack_statuses.begins_with(expected), "chosen tier's Curse amount accompanies target damage")
			if stage == "damage_reaction": _expect(panel.data.after == 0, "blocked damage fixture really shows zero damage")
		elif panel.data.source_id == "brine-hit": _expect(not "Curse" in panel.data.attack_statuses, "Curse does not leak onto the unrelated incoming attack")
	_expect(shown == 1, "one target panel per attack")
	var applications := [{"target_actor_id": "goblin", "status_id": "poison", "stacks": 2}]
	_expect("Poison ×2 pending" in screen._attack_source_effect_text(source, applications), "ordinary status summary remains alongside Curse")
	_expect(expected in screen._attack_source_effect_text(source, applications), "ordinary statuses cannot replace bespoke Curse effect")
	# Recorded damage playback uses the actor state from that batch.
	_expect("Apply 3 Curse after damage" in screen._attack_source_effect_text(source, [], {"blade": {"selected_tier": "skull_5"}}), "damage history uses its recorded tier")
	_expect(expected in screen._attack_source_effect_text(source, [], {"blade": {"current_health": 24}}), "sparse damage baseline retains live selected tier")
	var reversed := source.duplicate(); reversed.source_actor_id = "goblin"; reversed.target_actor_id = "blade"
	screen._view.actors.goblin.selected_tier = "skull_4"
	_expect("Apply 2 Curse after damage" in screen._attack_source_effect_text(reversed, []), "incoming Curse uses its own attacker tier")
	var status_source := source.duplicate(); status_source.source_content_id = "curse_count"
	_expect(screen._attack_source_effect_text(status_source, []).is_empty(), "Curse Count damage does not invent additional Curse applications")
	var dir := OS.get_environment("DICE_AND_DESTINY_CURSE_FOLLOWUP_SCREENSHOTS")
	if tier == 5 and not dir.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join("curse-followup-%s-%d.png" % [stage, viewport.x]))
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSE ATTACK FOLLOWUPS: " + message)
