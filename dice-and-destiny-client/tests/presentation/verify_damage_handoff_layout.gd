extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var failed := false

class LateHandoff extends Node:
	var action: Callable
	func _process(_delta: float) -> void:
		set_process(false)
		action.call()

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var result: Dictionary = native.start_battle("damage-handoff", 1789679833957118)
	result.events = []; result.learned_policy = {}
	result.snapshot.segment = "damage_resolution"; result.snapshot.stage = "damage_reaction"
	result.snapshot.settled_damage = {"id": "handoff", "sources": [], "removals": [], "committed": false}
	for actor in ["blade", "goblin"]:
		var count := 4 if actor == "blade" else 3
		result.snapshot.settled_damage.sources.append({"id": actor + "-incoming", "source_actor_id": "goblin" if actor == "blade" else "blade", "target_actor_id": actor, "source_content_id": "sword_cut" if actor == "blade" else "needlefang", "base_amount": count, "final_amount": count})
		for index in count:
			result.snapshot.settled_damage.removals.append({"card_id": actor + str(index), "card_definition_id": "culture_flask" if actor == "blade" else "tip_it", "target_actor_id": actor, "accepted": true, "released": false, "original_zone": "deck", "damage_proposal_ids": [actor + "-incoming"]})
	_set_priority(result, "before-pass")
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("damage-handoff.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen); screen.set_process(false)
	await create_timer(TIMING.review_seconds() + 0.2).timeout
	var panels := {}; var cards := {}
	for actor in screen._combat_columns:
		panels[actor] = screen._combat_columns[actor].get_child(0).get_global_rect()
	for grid in screen._damage_grids:
		for card in grid.get_children(): cards[card.instance_id] = card.get_global_rect()
	_expect(cards.size() == 7, "both sides display every reviewed card")
	var next := result.duplicate(true); _set_priority(next, "after-pass")
	fake.enqueue(next)
	screen._auto_pass_disabled = false
	screen._auto_pass_if_only_action()
	screen._auto_pass_preview_started_ms = Time.get_ticks_msec() - 10000
	screen._auto_pass_if_only_action()
	screen._auto_pass_highlight_ms = Time.get_ticks_msec() - 1000
	# Run after the board's ordinary processing, as the automatic handoff can.
	# Observe the very first drawn frame, not one process tick later.
	for handoff_index in 3:
		await process_frame
		var handoff := LateHandoff.new(); handoff.process_priority = 100
		handoff.action = screen._auto_pass_if_only_action if handoff_index == 0 else func():
			screen._model_thinking = handoff_index == 1
			screen._render()
		root.add_child(handoff)
		for frame in 4:
			if DisplayServer.get_name() != "headless": await RenderingServer.frame_post_draw
			else: await process_frame
			for actor in panels:
				var actual: Rect2 = screen._combat_columns[actor].get_child(0).get_global_rect()
				_expect(actual.size.distance_to(panels[actor].size) < 1.0, "damage panel geometry survives handoff from first frame")
			var visible := 0
			for grid in screen._damage_grids:
				for card in grid.get_children():
					_expect(card.get_global_rect().size.distance_to(cards[card.instance_id].size) < 1.0, "reviewed card size survives handoff")
					_expect(card.modulate.a > 0.99, "reviewed card stays visible on first handoff frame (%d / %d)" % [handoff_index, frame])
					visible += 1
			_expect(visible == 7, "handoff retains all seven cards")
			var capture := OS.get_environment("DICE_AND_DESTINY_DAMAGE_HANDOFF_SCREENSHOT")
			if handoff_index == 0 and frame == 0 and not capture.is_empty() and DisplayServer.get_name() != "headless":
				root.get_texture().get_image().save_png(capture)
			await process_frame
		handoff.queue_free()
	_expect(fake.commands.size() == 1, "automatic pass submits exactly once")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE HANDOFF LAYOUT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _set_priority(result: Dictionary, input_id: String) -> void:
	result.pending_input = {"blade": {"id": input_id, "segment": "damage_resolution", "stage": "damage_reaction", "allowed_commands": ["pass"]}}
	result.legal_actions = [{"battle_id": result.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": input_id}}]

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE HANDOFF: " + message)
