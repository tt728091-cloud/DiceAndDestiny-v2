extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var base: Dictionary = gateway.start_battle("damage-card-layout", 1788900535520209)
	_expect(base.get("accepted") == true, "native content loads")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1600, 1000)]:
		root.size = viewport
		for counts in [[1, 5], [12, 12], [24, 24], [0, 0]]:
			var fixture := base.duplicate(true)
			fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
			fixture.snapshot.segment = "damage_resolution"; fixture.snapshot.stage = "damage_reaction"
			var removals: Array = []
			for side in 2:
				var actor := "goblin" if side == 0 else "blade"
				for index in counts[side]:
					removals.append({"card_id": "%s-%d" % [actor, index], "card_definition_id": ["deep_puncture", "twin_puncture", "accelerant", "antivenom_draught", "measured_dose"][index % 5], "target_actor_id": actor, "original_zone": "deck", "accepted": true, "released": false})
			removals.append({"card_id": "saved", "card_definition_id": "battle_focus", "target_actor_id": "blade", "accepted": true, "released": true})
			fixture.snapshot.settled_damage = {"removals": removals, "sources": []}
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("damage-card-layout.json"))
			root.add_child(screen); screen.set_process(false)
			for frame in 8: await process_frame
			var piles: Control = screen._center.find_child("DamageCardPiles", true, false)
			var cards: Array = []
			for card in piles.find_children("*", "Button", true, false):
				if card is BattleCard: cards.append(card)
			_expect(cards.size() == counts[0] + counts[1], "every pending card appears and released cards disappear")
			_expect(piles.find_children("*", "ScrollContainer", true, false).is_empty(), "damage piles have no scroll containers")
			_expect(not screen._center_scroll.get_h_scroll_bar().visible and not screen._center_scroll.get_v_scroll_bar().visible, "entire damage view fits without scrolling")
			for index in cards.size():
				var card: BattleCard = cards[index]
				var rect := card.get_global_rect()
				_expect(screen._center_scroll.get_global_rect().encloses(rect), "whole card fits the damage panel: %s at %s" % [card.instance_id, viewport])
				_expect(not rect.intersects(screen._hand_dock.get_global_rect()), "damage cards do not cover the hand")
				_expect("Original zone: deck" in card.tooltip_text, "card inspection details preserved")
				for other in range(index + 1, cards.size()):
					_expect(not rect.intersects(cards[other].get_global_rect()), "cards never overlap")
			if counts == [1, 5]:
				for card in cards: _expect(card.scale.is_equal_approx(Vector2.ONE), "ordinary damage wraps at full compact size")
				await _capture("damage-cards-%dx%d" % [viewport.x, viewport.y])
			if counts == [24, 24]: await _capture("damage-cards-full-decks-%dx%d" % [viewport.x, viewport.y])
			screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE CARD LAYOUT: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _capture(name: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_DAMAGE_LAYOUT_SCREENSHOTS")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(path.path_join(name + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("DAMAGE CARD LAYOUT: " + message)
