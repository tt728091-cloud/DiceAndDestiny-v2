extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	base = gateway.start_battle("selector-fixture", 3)
	base.events = []; base.learned_policy = {}
	var single := base.duplicate(true); single.snapshot.actors.erase("goblin-2")
	var original = _screen(single)
	await process_frame; await process_frame
	var player_rect: Rect2 = original._player_dice_dock.get_rect()
	var enemy_rect: Rect2 = original._enemy_profile_dock.get_rect()
	var enemy_dice_rect: Rect2 = original._enemy_dice_dock.get_rect()
	var hand_rect: Rect2 = original._hand_dock.get_rect()
	var ability_rect: Rect2 = original._ability_dock.get_parent().get_rect()
	var center_rect: Rect2 = original._center.get_rect()
	original.queue_free(); await process_frame
	var multi = _screen(base)
	await process_frame; await process_frame
	_expect(multi._player_dice_dock.get_rect() == player_rect, "player dice retain original coordinates")
	_expect(multi._hand_dock.get_rect() == hand_rect and multi._ability_dock.get_parent().get_rect() == ability_rect and multi._center.get_rect() == center_rect, "hand, ability rail, and central battle retain original layout")
	_expect(multi._enemy_profile_dock.get_rect() == enemy_rect and multi._enemy_dice_dock.get_rect() == enemy_dice_rect, "enemy HUD retains original alignment")
	_expect(multi._player_dice_dock.position.y == multi._enemy_dice_dock.position.y, "both dice rows share a baseline")
	_expect(multi._player_dice_dock.get_child(0)._buttons[0].global_position.y == multi._enemy_dice_dock.get_child(0)._buttons[0].global_position.y, "actual dice faces align, including caption spacing")
	_expect(multi._enemy_profile_dock.get_child_count() == 1, "no selector above enemy profile")
	var scenery: Control = multi._root.get_node("BattleScenery")
	var right: TextureRect = scenery.get_node("Fighter_enemy")
	var left: TextureRect = scenery.get_node("Fighter_enemy_2")
	_expect(right.get_index() > left.get_index() and right.position.y > left.position.y, "right mask is drawn in foreground")
	for button in multi._enemy_buttons.values():
		_expect(button.get_node("Health").show_percentage == false, "nameplate health has no numeric caption")
		_expect(button.find_children("*", "Label", true, false).size() == 1, "nameplate contains only name and health bar")
	await _capture("brine-field-selection.png")
	var fake: FakeBattleAuthority = multi.gateway._authority
	multi._selected_indices = [0, 2]
	await _click(multi._enemy_buttons["goblin-2"]); await process_frame
	_expect(multi._selected_indices == [0, 2], "switching enemy preserves kept dice")
	_expect(multi._actor_profiles.keys().size() == 2 and multi._actor_profiles.has("goblin-2"), "selected enemy occupies original profile")
	_expect(fake.commands.is_empty(), "selection is local until an action is committed")
	var second := {"type": "planning_select_ability", "payload": {"ability_id": "needlefang", "target_ids": ["goblin-2"]}}
	var first := second.duplicate(true); first.payload.target_ids = ["goblin"]
	_expect(multi._action_in_focus(second) and not multi._action_in_focus(first), "attack choices follow battlefield selection")
	var card := {"type": "planning_commit_cards", "payload": {"commitment": {"card_ids": ["a"], "target_ids": ["goblin"]}}}
	_expect(not multi._action_in_focus(card), "enemy-targeted cards cannot select hidden enemy")
	var width: float = multi._enemy_profile_dock.size.x
	multi.queue_free(); await process_frame
	var five := base.duplicate(true)
	for index in range(3, 6): five.snapshot.actors["goblin-" + str(index)] = base.snapshot.actors.goblin.duplicate(true)
	multi = _screen(five); await process_frame; await process_frame
	_expect(multi._enemy_buttons.size() == 5 and multi._actor_profiles.size() == 2, "five nameplates still use two actor panels")
	_expect(multi._enemy_profile_dock.size.x == width and multi._player_dice_dock.get_rect() == player_rect, "five enemies cannot expand original HUD geometry")
	multi._enemy_buttons["goblin-5"].pressed.emit(); await process_frame; await process_frame
	_expect(multi._focused_enemy == "goblin-5", "fifth nameplate selects enemy")
	for id in multi._enemy_buttons:
		for other in multi._enemy_buttons:
			if id == other: continue
			_expect(not multi._enemy_buttons[id].get_rect().intersects(multi._enemy_buttons[other].get_rect()), "nameplates stay separately clickable")
	_expect(multi._actor_profiles["goblin-5"].title.text == "Brine Mask 5", "profile identifies the selected duplicate")
	_expect(multi._root.get_node("BattleScenery").has_node("Fighter_enemy_5"), "all five enemy sprites in central battlefield")
	await _capture("brine-five-selector.png")
	multi.queue_free(); await process_frame
	await _defense()
	await _effects()
	await _damage()
	await _defense_plan_review()
	await _offensive_defeat()
	print("ENEMY SELECTOR: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _defense() -> void:
	var fixture := base.duplicate(true)
	fixture.pending_input = {"blade": {"id": "defend", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
	fixture.snapshot.damage_sources = []
	fixture.legal_actions = []
	fixture.snapshot.actors.blade.qualified_abilities = ["barbed_mantle"]
	fixture.snapshot.actors.blade.defensive_abilities = ["barbed_mantle"]
	for index in 2:
		var id := "source-" + str(index + 1)
		fixture.snapshot.damage_sources.append({"id": id, "source_actor_id": "goblin" if index == 0 else "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 6})
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "defend", "ability_id": "barbed_mantle", "target_ids": [id]}})
	fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin-2", "source_content_id": "needlefang", "base_amount": 4})
	var screen = _screen(fixture)
	await process_frame; await process_frame
	_expect(screen._combat_columns.blade.get_child_count() == 2, "both incoming attacks stay visible during selection")
	_expect(not screen._enemy_selector.visible, "no enemy switching controls during defense")
	var fake: FakeBattleAuthority = screen.gateway._authority
	fake.enqueue(fixture)
	var choose: Button
	for button in screen.find_children("*", "Button", true, false):
		if button.get_meta("inspection_id", "") == "battle.defense.choose.source-2.barbed_mantle.false": choose = button
	_expect(choose != null, "second attack owns a defense button")
	if choose != null: await _click(choose)
	_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.target_ids == ["source-2"], "inline defense sends exact source without changing enemy view")
	_expect(screen._display_damage_sources(fixture.snapshot.damage_sources).size() == 3, "both attacks retained")
	await _capture("brine-stacked-selection.png")
	# Completed first defense must not leak into the next source or be repeated.
	fixture.snapshot.defense_history = {"source-2": {"source_id": "source-2", "ability_id": "barbed_mantle", "rolled_face": 2, "rolled_faces": [2], "finalized": true}}
	fixture.snapshot.damage_sources[1].prevention = 1
	fixture.legal_actions = [fixture.legal_actions[0]]
	screen._view.apply_result(fixture); screen._render(); await process_frame
	_expect(screen._focused_enemy == "goblin" and screen._selected_source == "source-1", "next defense selects remaining attacker")
	_expect(screen._combat_columns.blade.get_child_count() == 2, "completed defense remains visible beside remaining choice")
	var data: Dictionary = screen._compact_defense_data("blade", screen._status_counts(), fixture.snapshot.damage_sources[1])
	_expect(data.gains.is_empty() and data.read_only, "historical defense does not replay status gains")
	fixture.snapshot.damage_sources = [fixture.snapshot.damage_sources[0]]
	screen._view.apply_result(fixture); screen._render(); await process_frame
	_expect("Miss" in screen._enemy_buttons["goblin-2"].tooltip_text and screen._combat_columns.blade.get_child_count() == 1, "miss has no defendable source")
	screen.queue_free(); await process_frame

func _effects() -> void:
	var fixture := base.duplicate(true)
	var event := {"type": "effects_resolved", "segment": "ongoing_effects", "round": 1, "data": {"actors_before": {}, "actors_after": {}, "steps": []}}
	var rolls: Array = []; var sources: Array = []; var removals: Array = []
	for actor in ["goblin", "goblin-2"]:
		event.data.actors_before[actor] = {"health": 16, "deck_count": 14, "hand_count": 2, "discard_count": 0, "removed_count": 0, "statuses": [{"definition_id": "poison", "stacks": 3}, {"definition_id": "volatile_poison", "stacks": 1}]}
		event.data.actors_after[actor] = {"health": 12, "deck_count": 10, "hand_count": 2, "discard_count": 0, "removed_count": 4, "statuses": [{"definition_id": "poison", "stacks": 2}, {"definition_id": "volatile_poison", "stacks": 1}]}
		for index in 4:
			var status := "volatile_poison" if index == 3 else "poison"
			rolls.append({"actor_id": actor, "source_content_id": status, "die": {"face": [1, 4, 5, 2][index]}})
			if index == 2: continue
			var id: String = actor + "-toxin-" + str(index); var amount := 2 if index == 3 else 1
			sources.append({"id": id, "source_actor_id": actor, "target_actor_id": actor, "source_content_id": status, "final_amount": amount})
			for point in amount:
				removals.append({"card_id": id + "-" + str(point), "card_definition_id": "brine_surge", "target_actor_id": actor, "accepted": true, "original_zone": "deck", "damage_proposal_ids": [id]})
	event.data.steps = [{"type": "proposal_batch_committed", "data": {"rolls": rolls}}, {"type": "damage_committed", "data": {"sources": sources, "removals": removals}}]
	event.sequence = 101
	fixture.events = [event]; fixture.legal_actions = []; fixture.pending_input = {}
	var screen = _screen(fixture)
	await process_frame; await process_frame
	_expect(is_instance_valid(screen._effects_panel), "effects animation starts in original screen")
	if is_instance_valid(screen._effects_panel):
		screen._effects_panel.resume_at(3.2); screen._effects_panel.present_progress()
		_expect(screen._effects_panel.visible_actor_ids == ["blade", "goblin", "goblin-2"], "all actor effects share one timeline")
		_expect(not screen._enemy_selector.visible, "no switching required for effects")
		for entry in screen._effects_panel.entries:
			_expect(entry.has("die") and entry.die.is_visible_in_tree(), "every effect has a visible die")
		screen._effects_panel.set_paused(true)
		screen._effects_panel.resume_at(3.9 + screen._effects_panel.catalyst_extra_seconds()); screen._effects_panel.present_progress()
		await create_timer(0.8).timeout
		for actor in ["goblin", "goblin-2"]:
			var actor_entries: Array = screen._effects_panel.entries.filter(func(entry): return entry.actor_id == actor)
			var lost := 0
			for entry in actor_entries:
				lost += entry.cards.size()
				_expect(screen._center_scroll.get_global_rect().encloses(entry.die.get_global_rect()), "both enemies' toxin dice fit on screen")
				for card in entry.cards_ui: _expect(screen._center_scroll.get_global_rect().encloses(card.get_global_rect()), "both enemies' toxin removals fit on screen")
			_expect(actor_entries.size() == 4 and lost == 4, "each enemy displays all four rolls and its four unique losses")
		await _capture("brine-stacked-effects.png")
	screen.queue_free(); await process_frame

func _damage() -> void:
	var fixture := base.duplicate(true)
	fixture.snapshot.segment = "damage_resolution"; fixture.snapshot.stage = "damage_reaction"
	fixture.pending_input = {}; fixture.legal_actions = []
	var sources: Array = []
	for index in 2:
		sources.append({"id": "hit-" + str(index + 1), "source_actor_id": "goblin" if index == 0 else "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 5, "final_amount": 3 if index == 0 else 5, "status_applications": null})
	var removals: Array = []
	for i in 8:
		removals.append({"card_id": "lost-" + str(i), "card_definition_id": "pinprick", "target_actor_id": "blade", "accepted": true, "original_zone": "deck", "damage_proposal_ids": ["hit-1", "hit-2"]})
	sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin-2", "source_content_id": "needlefang", "base_amount": 4, "final_amount": 2})
	for i in 2: removals.append({"card_id": "enemy-lost-" + str(i), "card_definition_id": "brine_surge", "target_actor_id": "goblin-2", "accepted": true, "original_zone": "deck", "damage_proposal_ids": ["outgoing"]})
	fixture.snapshot.damage_sources = sources
	fixture.snapshot.settled_damage = {"id": "separate-hits", "sources": sources, "removals": removals}
	var screen = _screen(fixture); await process_frame; await process_frame
	var panel: Control = screen._combat_columns.blade.get_child(0)
	_expect(panel.data.attack_name == "Brine Mask 1" and panel.data.after == 3, "first attack shows three damage, not sum")
	_expect(screen._damage_grids[0].get_child_count() == 3, "first attack shows its three cards")
	var first_cards: Array = screen._damage_grids[0].get_children().map(func(card): return card.instance_id)
	_expect(screen._combat_columns.blade.get_child_count() == 2, "both damage panels shown simultaneously")
	panel = screen._combat_columns.blade.get_child(1)
	_expect(panel.data.attack_name == "Brine Mask 2" and panel.data.after == 5, "second panel shows its own five damage")
	_expect(screen._damage_grids[1].get_child_count() == 5, "second attack shows its five cards")
	for card in screen._damage_grids[1].get_children(): _expect(card.instance_id not in first_cards, "card losses are never duplicated across attacks")
	panel.source_selected.emit("hit-2"); await process_frame
	_expect(screen._selected_source == "hit-2", "clicking an attack targets a reaction without hiding the other")
	_expect(screen.gateway._authority.commands.is_empty(), "reaction target selection does not commit damage")
	await create_timer(1.6).timeout
	_expect(screen._combat_columns["goblin-2"].get_child_count() == 1 and screen._damage_grids[2].get_child_count() == 2, "outgoing attack and enemy removals stay visible on the right")
	for grid in screen._damage_grids:
		for card in grid.get_children(): _expect(screen._center_scroll.get_global_rect().encloses(card.get_global_rect()), "all removal cards fit above the hand")
	await _capture("brine-stacked-damage.png")
	# Prevent only the second hit; authority releases two removals from the batch.
	sources[1].final_amount = 3
	removals[6]["released"] = true; removals[7]["released"] = true
	screen._view.apply_result(fixture); screen._render(); await process_frame
	_expect(screen._combat_columns.blade.get_child(1).data.after == 3 and screen._damage_grids[1].get_child_count() == 3, "reaction updates second attack and its cards")
	_expect(screen._combat_columns.blade.get_child(0).data.after == 3, "reaction leaves other attack damage unchanged")
	# A miss adds no phantom attack or removal cards.
	fixture.snapshot.damage_sources = [sources[0]]; fixture.snapshot.settled_damage.sources = [sources[0]]
	screen._view.apply_result(fixture); screen._render(); await process_frame
	panel = screen._combat_columns.blade.get_child(0)
	_expect(screen._combat_columns.blade.get_child_count() == 1 and panel.data.after == 3, "miss adds no extra attack")
	screen.queue_free(); await process_frame

func _defense_plan_review() -> void:
	var fixture := base.duplicate(true)
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
	fixture.snapshot.damage_sources = [
		{"id": "first", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 6},
		{"id": "second", "source_actor_id": "goblin-2", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 4}]
	fixture.snapshot.defense_plans = {"first": {"actor_id": "blade", "source_id": "first", "ability_id": "shedskin", "finalized": false}}
	fixture.pending_input = {"blade": {"id": "choose-all", "allowed_commands": ["planning_pass"]}}
	fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_pass", "payload": {"pending_input_id": "choose-all"}}]
	var screen = _screen(fixture); screen._auto_pass_disabled = false
	await process_frame; await process_frame
	_expect(screen._focused_enemy == "goblin-2" and screen._selected_source == "second", "queued first defense selects remaining decision")
	_expect(screen._sole_pass_action().is_empty(), "sole pass is never automatically chosen during defense selection")
	_expect("Queued" in screen._combat_columns.blade.get_child(0).data.note and "Needs defense" in screen._combat_columns.blade.get_child(1).data.note, "both decision states are visible")
	var fake: FakeBattleAuthority = screen.gateway._authority
	fake.enqueue(fixture); screen._pass_planning()
	_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.get("source_id", "").is_empty(), "Pass All Remaining sends global pass explicitly")
	fake.commands.clear()
	fixture.snapshot.stage = "defense_reaction"; fixture.snapshot.defense_plans = {}
	fixture.snapshot.defense_history = {"first": {"actor_id": "blade", "source_id": "first", "ability_id": "shedskin", "rolled_faces": [1,1], "rolled_face": 1, "finalized": true}}
	fixture.snapshot.damage_sources[0]["prevention"] = 2
	fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "source_id": "second", "ability_id": "shedskin", "rolled_faces": [1,4], "rolled_face": 1, "finalized": false}}
	fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin-2", "source_content_id": "needlefang", "base_amount": 4})
	fixture.snapshot.defense_selections["goblin-2"] = {"actor_id": "goblin-2", "source_id": "outgoing", "ability_id": "salt_veil", "rolled_faces": [4], "rolled_face": 4, "finalized": false}
	fixture.pending_input.blade = {"id": "review-all", "allowed_commands": ["pass"]}
	fixture.legal_actions = [{"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "pass", "payload": {"pending_input_id": "review-all"}}]
	screen._view.apply_result(fixture); screen._render(true); await process_frame
	_expect(screen._focused_enemy == "goblin-2", "playback automatically focuses active defense")
	await create_timer(1.0).timeout
	for key in screen._defense_animation_times: screen._defense_animation_times[key] = Time.get_ticks_msec() - 20000
	_expect(screen._defense_final_review() and not screen._sole_pass_action().is_empty(), "completed results allow automatic Continue when it is the only action")
	# Keep this layout fixture on screen; automatic transitions are exercised by
	# verify_simultaneous_defenses with queued authority responses.
	screen._auto_pass_disabled = true
	screen._auto_pass_if_only_action()
	_expect(fake.commands.is_empty(), "debug pause prevents automatic advance")
	_expect(screen._defense_result_panels.size() == 3 and screen._defense_result_panels[0].data.after == 4 and screen._defense_result_panels[1].data.after == 3, "both results readable without switching")
	await create_timer(6.0).timeout
	_expect(fake.commands.is_empty() and screen._view.stage == "defense_reaction", "explicit debug pause preserves completed results")
	await _capture("brine-defense-summary.png")
	_expect(screen._defense_result_panels.filter(func(panel): return panel.data.actor_id == "blade").all(func(panel): return panel.dice_controls.size() == 2), "both defense rolls remain visible for review")
	screen.queue_free(); await process_frame

func _offensive_defeat() -> void:
	var fixture := base.duplicate(true)
	fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
	fixture.pending_input = {}; fixture.legal_actions = []
	fixture.snapshot.actors["goblin-2"].current_health = 0
	fixture.snapshot.actors["goblin-2"].defeat_state = "defeated"
	fixture.snapshot.actors["goblin-2"].selected_ability = ""
	fixture.snapshot.damage_sources = [{"id": "survivor", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 4}]
	var screen := _screen(fixture)
	screen._view.offensive_reveals["goblin-2"] = {"ability_id": "brine_lash", "targets": ["blade"], "outcome": {"base_damage": 6}}
	screen._render(true)
	await process_frame
	_expect(screen._selected_attack("goblin-2").is_empty(), "canceled enemy attack cannot reappear from a cached reveal")
	_expect(screen._combat_columns.blade.get_child_count() == 1 and screen._combat_columns.blade.get_child(0).data.source_id == "survivor", "only the surviving enemy needs a defense")
	screen.queue_free(); await process_frame

func _screen(fixture: Dictionary) -> Control:
	var screen = SCREEN.instantiate(); screen.initial_result = fixture
	screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("selector.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	return screen
func _click(button: Control) -> void:
	var point := button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; motion.global_position = point
	root.push_input(motion); await process_frame
	var press := InputEventMouseButton.new(); press.position = point; press.global_position = point; press.button_index = MOUSE_BUTTON_LEFT; press.pressed = true
	root.push_input(press); await process_frame
	var release := press.duplicate(); release.pressed = false
	root.push_input(release); await process_frame

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_MINION_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(name))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ENEMY SELECTOR: " + message)
