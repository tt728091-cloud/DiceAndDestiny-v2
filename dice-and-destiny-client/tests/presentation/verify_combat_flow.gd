extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var failed := false
var base: Dictionary

class HandoffRedraw extends Node:
	var redraw: Callable
	func _process(_delta: float) -> void:
		set_process(false)
		redraw.call()

func _initialize() -> void: call_deferred("_run")

func _fixture(stage: String) -> Dictionary:
	var result := base.duplicate(true)
	result.events = []; result.learned_policy = {}; result.pending_input = {}; result.legal_actions = []
	result.snapshot.segment = "damage_resolution" if stage == "damage_reaction" else "offensive" if stage in ["planning", "offensive_reaction"] else "defensive"
	result.snapshot.stage = stage
	result.snapshot.actors.blade.selected_ability = "needlefang"
	result.snapshot.actors.blade.offensive_outcome = {"base_damage": 3, "status_applications": [{"target_actor_id": "goblin", "status_id": "poison", "stacks": 3}]}
	result.snapshot.actors.goblin.selected_ability = "sword_cut"
	result.snapshot.actors.goblin.offensive_outcome = {"base_damage": 4}
	result.snapshot.damage_sources = [
		{"id": "enemy-attack", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 4},
		{"id": "player-attack", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 3}]
	if stage == "planning":
		result.snapshot.actors.blade.selected_ability = ""; result.snapshot.actors.blade.offensive_outcome = {}
		result.snapshot.actors.goblin.selected_ability = ""; result.snapshot.actors.goblin.offensive_outcome = {}
	return result

func _action(ability: String, source: String, paid: bool = false) -> Dictionary:
	return {"battle_id": base.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "defend", "ability_id": ability, "target_ids": [source], "spend_catalyst": paid}}

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	base = gateway.start_battle("combat-flow", 1789679833957118)
	_expect(base.get("accepted") == true, "native catalog loads")
	var initial := _fixture("planning")
	var fake := FakeBattleAuthority.new()
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = BattleGateway.new(fake)
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("combat-flow-test.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	await _frames()
	_expect(_ability_count(screen) == 4, "all offensive choices start in the rail")
	# Actual command path: selection transforms at the rail before moving to target.
	var defense := _fixture("defense_selection")
	defense.pending_input = {"blade": {"id": "defend", "segment": "defensive", "stage": "defense_selection", "allowed_commands": ["planning_select_ability"]}}
	defense.legal_actions = [_action("shedskin", "enemy-attack"), _action("shedskin", "enemy-attack", true), _action("barbed_mantle", "enemy-attack"), _action("shedskin", "other-source")]
	fake.enqueue(defense)
	screen._send(JSON.stringify({"battle_id": base.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"ability_id": "needlefang", "target_ids": ["goblin"]}}))
	_expect(screen._ability_dock.get_child(0).modulate.a == 0.0, "new outcome is hidden before its first frame, preventing flashes")
	await _frames()
	_expect(not screen._selection_morph.is_empty(), "chosen attack transforms before defense")
	_expect(_ability_count(screen) == 1, "only chosen attack remains in rail")
	await _capture("selected")
	await create_timer(TIMING.transition() + 0.5).timeout
	await _frames()
	_expect(screen._combat_columns.size() == 2, "two recipient lanes exist")
	var left: Control = screen._combat_columns.blade.get_child(0)
	var right: Control = screen._combat_columns.goblin.get_child(0)
	_expect(left.data.source_id == "enemy-attack" and right.data.source_id == "player-attack", "attacks grouped by receiving side")
	_expect(left.get_global_rect().get_center().x < right.get_global_rect().get_center().x, "player receives attacks on center left")
	_expect(screen._ability_dock.get_child_count() == 0, "defenses hidden until incoming attack chosen")
	left.source_selected.emit("enemy-attack")
	await _frames()
	_expect(screen._defense_actions("shedskin").size() == 2, "Catalyst alternatives retained only for chosen source")
	_expect(screen._defense_actions("barbed_mantle").size() == 1, "other valid defense retained")
	_expect(_ability_count(screen) == 2, "two defenses appear")
	await _capture("choose-defense")
	# The model handoff can redraw the same defense-selection stage. Check its
	# very first visible frames, before the roll expansion has even started.
	var established := {}
	for actor in screen._combat_columns:
		var column: Control = screen._combat_columns[actor]
		established[actor] = column.get_child(0).get_global_rect()
	for redraw in 2:
		screen._model_thinking = redraw == 0
		screen._render()
		for frame in 8:
			await process_frame
			if DisplayServer.get_name() != "headless": await RenderingServer.frame_post_draw
			for actor in established:
				var column: Control = screen._combat_columns[actor]
				var actual: Rect2 = column.get_child(0).get_global_rect()
				_expect(actual.size.distance_to(established[actor].size) < 1.0, "same-stage defense redraw preserves panel size from first frame (%s frame %d: %s vs %s, scale %s)" % [actor, frame, actual.size, established[actor].size, column.scale])
				_expect(column.scale.is_equal_approx(Vector2.ONE), "defense text stays at full scale during the model handoff")
			var directory := OS.get_environment("DICE_AND_DESTINY_COMBAT_FLOW_SCREENSHOTS")
			if redraw == 0 and frame == 0 and not directory.is_empty() and DisplayServer.get_name() != "headless":
				root.get_texture().get_image().save_png(directory.path_join("defense-first-frame.png"))

	# Both dice roll and settle in these exact recipient lanes.
	var rolled := defense.duplicate(true); rolled.snapshot.stage = "defense_reaction"; rolled.pending_input = {}; rolled.legal_actions = []
	rolled.snapshot.defense_selections = {"blade": {"ability_id": "shedskin", "source_id": "enemy-attack", "rolled_faces": [1,4]}, "goblin": {"ability_id": "basic_defense", "source_id": "player-attack", "rolled_faces": [5]}}
	screen._view.apply_result(rolled); screen._render(); await _frames()
	await _check_continuity(screen, "defense-expanding")
	_expect(screen._defense_result_panels.size() == 2, "both dice panels present")
	var panel: Control = screen._defense_result_panels[0]
	_expect(panel.get_parent() == screen._combat_columns.blade, "defense lands beneath received attack")
	_expect(panel.data.dice[0].prevention == 1, "per-die prevention is available to trail")
	_expect(panel.gain_origins[0] == panel.benefit_labels[1], "status trail originates beneath actual die")
	await create_timer(TIMING.transition() + 0.1).timeout
	await _capture("rolling")
	for result_panel in screen._defense_result_panels: result_panel.started_ms -= 6000; result_panel._update()
	await _capture("defended")
	var attack_position: Vector2 = panel.get_global_rect().position
	# Redraw after the roll has settled, at the start of an actual frame. A
	# newly added lane may not receive _process before that frame is drawn.
	var settled_rects := {}
	for actor in screen._combat_columns:
		settled_rects[actor] = screen._combat_columns[actor].get_child(0).get_global_rect()
	for redraw in 2:
		await process_frame
		screen._model_thinking = redraw == 0
		var handoff := HandoffRedraw.new()
		handoff.process_priority = 100
		handoff.redraw = screen._render
		root.add_child(handoff)
		for frame in 4:
			if DisplayServer.get_name() != "headless": await RenderingServer.frame_post_draw
			else: await process_frame
			for actor in settled_rects:
				var actual: Rect2 = screen._combat_columns[actor].get_child(0).get_global_rect()
				_expect(actual.size.distance_to(settled_rects[actor].size) < 1.0, "settled defense redraw never exposes provisional columns (%s frame %d: %s vs %s)" % [actor, frame, actual.size, settled_rects[actor].size])
			await process_frame
		handoff.queue_free()
	panel = screen._defense_result_panels[0]
	var damage := _fixture("damage_reaction")
	damage.snapshot.settled_damage = {"id": "flow-batch", "sources": damage.snapshot.damage_sources.duplicate(true), "removals": [], "status_applications": [{"target_actor_id": "goblin", "status_id": "poison", "stacks": 3}], "committed": false}
	damage.snapshot.settled_damage.sources[0].final_amount = 12
	damage.snapshot.settled_damage.sources[1].final_amount = 0
	for index in 12:
		damage.snapshot.settled_damage.removals.append({"card_id": "card-%d" % index, "card_definition_id": "culture_flask", "target_actor_id": "blade", "original_zone": "deck", "accepted": true, "released": false, "damage_proposal_ids": ["enemy-attack"]})
	screen._view.apply_result(damage); screen._render(); await _frames()
	await _check_continuity(screen, "damage-expanding")
	_expect(screen._combat_columns.blade.get_child(0).get_global_rect().position.distance_to(attack_position) < 1.0, "attack stays at same location entering damage")
	await create_timer(TIMING.reveal() + TIMING.transition() + 0.3).timeout
	for size in [Vector2i(1920,1080), Vector2i(1280,720)]:
		root.size = size; await _frames()
		var grid: Control = screen._damage_grids[0]
		_expect(grid.get_child_count() == 12, "all removals beneath correct attack")
		for card in grid.get_children():
			_expect(root.get_visible_rect().encloses(card.get_global_rect()), "every removal fits viewport")
			_expect(grid.get_global_rect().encloses(card.get_global_rect()), "every removal fits grid without scroll")
		await _capture("damage-%d" % size.x)
	# A commit received with future authoritative counts must preserve displayed
	# counts until cards dissolve. No pass button is needed for this beat.
	var finished := _fixture("planning")
	finished.snapshot.round = 2
	finished.snapshot.actors.blade.current_health = 12
	finished.snapshot.actors.blade.deck_count = 7
	finished.snapshot.actors.blade.removed_count = 12
	finished.events = [{"sequence": 1000, "round": 1, "segment": "damage_resolution", "type": "damage_committed", "data": {"batch_id": "flow-batch", "sources": damage.snapshot.settled_damage.sources, "removals": damage.snapshot.settled_damage.removals, "status_applications": []}}]
	screen._apply_model_result(finished); await _frames()
	await _check_continuity(screen, "damage-committing")
	_expect(screen._director.peek().get("type") == "combat_damage", "automatic committed batch cannot be skipped")
	_expect(screen._profile_actor("blade").current_health == 24, "health loss waits for removal")
	var key: String = base.snapshot.battle_id + ":flow-batch"
	_expect(screen._damage_commit_started.has(key), "removal clock established")
	await create_timer(TIMING.hold() + TIMING.removal() + 1.0).timeout
	_expect(not screen._director.has_beats(), "automatic removal finishes without interaction")
	_expect(screen._profile_actor("blade").current_health == 12, "health updates after removal")
	_expect(fake.commands.size() == 1, "presentation sends no extra gameplay commands")
	# Multiple incoming sources share the lane without losing or duplicating a
	# card attributed to more than one source.
	var multi := damage.duplicate(true)
	multi.snapshot.settled_damage.id = "multiple-batch"
	multi.snapshot.settled_damage.sources.append({"id": "follow-up", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "shield_bash", "base_amount": 6, "final_amount": 6})
	for index in 12: multi.snapshot.settled_damage.removals[index].damage_proposal_ids = ["enemy-attack", "follow-up"] if index == 0 else ["enemy-attack"] if index < 6 else ["follow-up"]
	screen._view.apply_result(multi); screen._render(); await _frames()
	await create_timer(TIMING.reveal() + TIMING.transition() + 0.5).timeout
	var shown := {}
	for grid in screen._damage_grids:
		for card in grid.get_children():
			_expect(not shown.has(card.instance_id), "shared-source card displayed once")
			shown[card.instance_id] = true
			_expect(screen._center_scroll.get_global_rect().encloses(card.get_global_rect()), "multiple attacks and their cards fit the play area")
	_expect(shown.size() == 12 and screen._combat_columns.blade.get_child_count() == 2, "all cards and both incoming attacks remain visible")
	await _capture("multiple-attacks")
	# Lower abilities use the same morph and slide to the top, with no recipe.
	screen._view.apply_result(_fixture("planning")); screen._render(); await _frames()
	var gland := _fixture("offensive_reaction")
	gland.snapshot.actors.blade.selected_ability = "venom_gland"
	gland.snapshot.actors.blade.selected_tier = "base"
	gland.snapshot.actors.blade.offensive_outcome = {}
	gland.snapshot.damage_sources = []
	fake.enqueue(gland)
	screen._send(JSON.stringify({"battle_id": base.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"ability_id": "venom_gland", "target_ids": ["goblin"]}}))
	await _frames()
	_expect(screen._selection_morph.ability_id == "venom_gland" and "Catalyst ×2" in screen._selection_morph.text and "Poison ×1" in screen._selection_morph.text, "Venom Gland previews its actual benefits before joint reveal")
	_expect(_ability_count(screen) == 1 and screen._ability_dock.get_child(0).ability_id == "venom_gland", "lower chosen ability occupies top slot")
	await create_timer(TIMING.transition() + 0.1).timeout
	await _capture("venom-gland")
	while not screen._selection_morph.is_empty() or Time.get_ticks_msec() <= screen._flow_until: await process_frame
	# Save restoration has only final actors, so committed events reconstruct
	# their own pre-loss counts instead of flashing the final health early.
	var director := BattlePresentationDirector.new()
	director.queue_result(finished)
	_expect(director.pending_damage_actor_before("blade").current_health == 24, "save restore reconstructs pre-removal health")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("COMBAT FLOW: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _frames() -> void:
	for frame in 5: await process_frame

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_COMBAT_FLOW_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name + ".png"))

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("COMBAT FLOW: " + message)

func _ability_count(screen: Node) -> int:
	var count := 0
	for node in screen._ability_dock.find_children("*", "Button", true, false):
		if node is BattleAbilityTile: count += 1
	return count

# Sample every frame, not just the settled screenshots: surviving source frames
# and common labels must never participate in a whole-panel fade.
func _check_continuity(screen: Node, capture_name: String) -> void:
	var transition: Control = screen._flow_transition
	for frame in 16:
		if transition.persistent_panels.size() == 2: break
		await process_frame
	_expect(transition.persistent_panels.size() == 2, "both established source panels continue through " + capture_name)
	var sizes := {}
	var samples := 0
	var captured := false
	var started := Time.get_ticks_msec()
	while Time.get_ticks_msec() - started < 3000:
		var active := false
		for item in transition.persistent_panels.values():
			if is_instance_valid(item.shell): active = true
		if not active: break
		for key in transition.persistent_panels:
			var item: Dictionary = transition.persistent_panels[key]
			if not is_instance_valid(item.shell): continue
			samples += 1
			_expect(item.shell.size.x > 300.0, "frame never collapses to a provisional layout width")
			_expect(item.shell.is_visible_in_tree() and item.shell.modulate.a == 1.0 and item.shell.self_modulate.a == 1.0, "source frame never fades during " + capture_name)
			_expect(item.destination.modulate.a == 1.0, "new source contents are not hidden by a parent fade")
			if sizes.has(key): _expect(item.shell.size.y >= float(sizes[key]) - 0.1, "source frame expands continuously")
			sizes[key] = item.shell.size.y
			if capture_name == "damage-committing":
				for part in item.destination._body.get_children():
					if part.get_meta("flow_part", "") == "cards":
						_expect(part.modulate.a == 1.0, "reviewed cards keep their own removal animation without reappearing")
			var title_visible := false
			for part in item.destination._body.get_children():
				if part.get_meta("flow_part", "") == "attack":
					_expect(part.self_modulate.a == 0.0, "replacement title hidden until retained title hands over")
			for part in item.parts:
				if is_instance_valid(part) and part.get_meta("flow_part", "") == "attack":
					title_visible = part.is_visible_in_tree() and part.modulate.a == 1.0 and part.self_modulate.a == 1.0
			_expect(title_visible, "unchanged attack title remains fully visible")
		if not captured and Time.get_ticks_msec() - started >= TIMING.transition() * 450.0:
			captured = true; await _capture(capture_name)
		await process_frame
	_expect(samples > 2, "sampled intermediate transition frames")
	for column in screen._combat_columns.values():
		for panel in column.get_children():
			_expect(panel.modulate.a == 1.0 and panel.self_modulate.a == 1.0, "live frame takes over without fading")
