extends SceneTree
# Hand cards an incoming attack has revealed for removal carry a ribbon naming
# that attacker; unthreatened cards and committed damage show none.
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var base: Dictionary = gateway.start_battle("hand-threat-ribbon", 43)
	for width in [1920, 1280]:
		canvas.size = Vector2i(width, width * 9 / 16)
		var fixture := _fixture(base)
		var hand: Array = fixture.snapshot.actors.blade.hand
		var threatened: Array = hand.slice(0, 3)
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("hand-threat-ribbon.json"))
		canvas.add_child(screen); screen.set_process(false)
		for frame in 4: await process_frame
		_expect(screen._unified_defense(), "fixture is the unified Defense board")
		_expect(threatened.size() == 3 and screen._hand_dock.cards.size() > 3, "fixture has threatened and safe hand cards")
		_expect(screen._hand_dock.cards.filter(func(card): return card.get_node_or_null("ThreatRibbon") != null).size() == 3, "exactly the reserved hand cards carry ribbons")
		for card in screen._hand_dock.cards:
			var ribbon := card.get_node_or_null("ThreatRibbon") as Button
			if card.instance_id in threatened:
				_expect(ribbon != null and ribbon.text == "Brine Mask 2", "threatened card names its attacker: " + card.instance_id)
				_expect("Pending removal · Brine Mask 2's Brine Lash · 3 damage" in card.tooltip_text, "hover explains the pending removal")
				if ribbon != null:
					_expect(card.get_global_rect().encloses(ribbon.get_global_rect()), "ribbon stays on the card")
					_expect(ribbon.get_global_rect().position.y > card.get_node("CardTitle").get_global_rect().position.y, "ribbon sits beneath the title")
			else: _expect(ribbon == null, "unthreatened card has no ribbon: " + card.instance_id)
		await _capture("hand-threat-%d" % width)
		screen._hand_dock.reveal = 1.0; screen._hand_dock.keep_visible = true; for frame in 3: await process_frame
		await _capture("hand-threat-open-%d" % width)
		# Once the threat leaves (released or committed), the ribbon goes with it.
		screen._view.settled_damage.removals = []
		screen._render(true); await process_frame
		_expect(screen._hand_dock.cards.all(func(card): return card.get_node_or_null("ThreatRibbon") == null), "released reservations remove every ribbon")
		screen.queue_free(); await process_frame
	print("HAND THREAT RIBBON: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _fixture(base: Dictionary) -> Dictionary:
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []
	fixture.snapshot.unified_defense = true
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
	fixture.pending_input = {"blade": {"id": "choose", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
	fixture.snapshot.actors["goblin-2"] = fixture.snapshot.actors.goblin.duplicate(true)
	var blade: Dictionary = fixture.snapshot.actors.blade
	var hand: Array = blade.hand
	var source := {"id": "incoming-2", "source_actor_id": "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 3, "final_amount": 3}
	fixture.snapshot.damage_sources = [source]
	var removals: Array = []
	for card_id in hand.slice(0, 3):
		removals.append({"card_id": card_id, "card_definition_id": str(blade.card_instances[card_id].definition_id), "original_zone": "hand", "target_actor_id": "blade", "accepted": true, "damage_proposal_ids": ["incoming-2"]})
	fixture.snapshot.settled_damage = {"id": "pending-batch", "sources": [source], "removals": removals}
	return fixture

func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_HAND_THREAT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await process_frame; await process_frame
	RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png(directory.path_join(id + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("HAND THREAT RIBBON: " + message)
