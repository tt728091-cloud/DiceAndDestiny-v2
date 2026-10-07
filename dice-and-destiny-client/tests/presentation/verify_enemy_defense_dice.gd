extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	var base: Dictionary = gateway.start_battle("enemy-defense-dice", 43)
	for viewport in [Vector2i(1280,720), Vector2i(1920,1080), Vector2i(1024,768)]:
		canvas.size = viewport
		for count in [1,4]:
			var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
			fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
			fixture.snapshot.actors.erase("goblin-2")
			for i in range(2,count+1): fixture.snapshot.actors["goblin-"+str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
			fixture.snapshot.damage_sources = []; fixture.snapshot.defense_selections = {}
			for actor_id in fixture.snapshot.actors:
				if actor_id == "blade": continue
				fixture.snapshot.damage_sources.append({"id": actor_id+"-attack", "source_actor_id": "blade", "target_actor_id": actor_id, "source_content_id": "adventurer_strike", "base_amount": 6})
				fixture.snapshot.defense_selections[actor_id] = {"actor_id": actor_id, "source_id": actor_id+"-attack", "ability_id": "salt_veil", "rolled_face": 5, "rolled_faces": [5]}
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("enemy-defense-dice.json"))
			canvas.add_child(screen); screen.set_process(false); await _settle()
			_expect(screen._defense_result_panels.size() == count, "one defense roll for each enemy")
			for panel in screen._defense_result_panels:
				panel.set_process(false)
				var seen := {}
				for step in 5:
					panel.started_ms = Time.get_ticks_msec() - 80 * step
					panel.data.roll_started_ms = panel.started_ms; panel._update()
					seen[panel.dice_controls[0].text] = true
					_expect(panel.dice_controls[0].is_visible_in_tree(), "revealed enemy die visible throughout roll")
					_expect(panel.effect_origin.text == "Salt Veil" and panel.effect_origin.modulate.a == 1, "defense name visible while rolling")
					_expect(panel.benefit_labels[0].modulate.a == 0 and panel.damage.text == "6", "result and prevention wait for landing")
				_expect(seen.size() > 1, "defense face cycles during roll")
				await _settle()
				_check_layout(screen,panel)
				var dock: Control = screen.dice_dock(str(panel.data.actor_id))
				dock.expanded = true; await _settle(); panel._update(); await _settle()
				_expect(panel.roll_cells[0].get_global_rect().end.y <= dock.get_global_rect().position.y, "expanded offensive dice have a separate row")
				dock.expanded = false; await _settle()
				panel.started_ms = Time.get_ticks_msec() - int((TIMING.roll_seconds()+TIMING.effects_seconds()*0.4)*1000)
				panel.data.roll_started_ms = panel.started_ms; panel._update(); await _settle()
				_expect(panel.dice_controls[0].text.ends_with("5") and panel.benefit_labels[0].text == "Prevent 3" and panel.benefit_labels[0].modulate.a > 0.99, "landed authoritative five reveals Prevent 3")
				_expect(panel._prevention_progress[0] > 0 and panel._prevention_progress[0] < 1 and panel._prevention_origin(0) == panel.dice_controls[0], "green prevention trail starts at the landed die")
				_check_layout(screen,panel)
			if DisplayServer.get_name() != "headless":
				RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/enemy-defense-%d-%d.png" % [viewport.x,count])
			for panel in screen._defense_result_panels:
				panel.started_ms = Time.get_ticks_msec() - int(TIMING.total_seconds()*1000)
				panel.data.roll_started_ms = panel.started_ms; panel._update()
				_expect(panel.damage.text == "3", "six damage settles at three after defense")
			screen.queue_free(); await process_frame
	for width in [1024, 1920]:
		for human in [false, true]: await _finalized_prevention(base, width, human)
	await _native_sequence()
	print("ENEMY DEFENSE DICE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check_layout(screen: Control,panel: Control) -> void:
	var die: Rect2 = panel.dice_controls[0].get_global_rect()
	var title: Rect2 = panel.effect_origin.get_global_rect()
	var profile: Rect2 = screen._actor_profiles[str(panel.data.actor_id)].get_global_rect()
	_expect(die.end.y < profile.position.y and title.end.y <= die.position.y, "name, die, and profile are vertically separated: %s %s %s" % [title,die,profile])
	_expect(absf(die.get_center().x-profile.get_center().x) < 1 and absf(title.get_center().x-die.get_center().x) < 1, "defense is centered above enemy name")
	_expect(Rect2(Vector2.ZERO,Vector2(canvas.size)).encloses(die) and Rect2(Vector2.ZERO,Vector2(canvas.size)).encloses(title), "defense stays inside viewport")
func _settle() -> void:
	for frame in 6: await process_frame
func _expect(ok: bool,message: String) -> void:
	if not ok: failed = true; push_error("ENEMY DEFENSE DICE: " + message)

func _finalized_prevention(base: Dictionary, width: int, human: bool) -> void:
	canvas.size = Vector2i(width, width * 9 / 16)
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.pending_input = {}; fixture.legal_actions = []
	fixture.snapshot.round = 5; fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_reaction"
	fixture.snapshot.unified_defense = true; fixture.snapshot.defense_history = {}; fixture.snapshot.defense_plans = {}
	fixture.snapshot.actors.goblin.current_health = 12
	var source := {"id": "prevention-six", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "adventurer_strike", "base_amount": 6, "final_amount": 6}
	fixture.snapshot.damage_sources = [source]
	fixture.snapshot.defense_selections = {"goblin": {"actor_id": "goblin", "source_id": source.id, "ability_id": "salt_veil", "rolled_face": 4, "rolled_faces": [4], "finalized": false}}
	var removals := []
	for index in 6:
		var card_id := "public-reservation-%d" % index
		removals.append({"card_id": card_id, "card_definition_id": "brine_surge", "target_actor_id": "goblin", "original_zone": "deck", "accepted": true, "released": false, "damage_proposal_ids": [source.id]})
	fixture.snapshot.settled_damage = {"id": "held-prevention", "sources": [source.duplicate(true)], "removals": removals, "committed": false}
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("finalized-prevention.json"))
	canvas.add_child(screen); screen.set_process(false); await _settle()
	var panel = screen._attack_intents[source.id]; panel.set_process(false)
	panel.started_ms = Time.get_ticks_msec() - 9000; panel.data.roll_started_ms = panel.started_ms; panel._update()
	_expect(panel.damage.text == "6" and panel.data.damage_pending, "preview holds six until authoritative finalization")
	var final := fixture.duplicate(true)
	final.snapshot.round = 6; final.snapshot.segment = "income"; final.snapshot.stage = "planning"
	final.snapshot.actors.goblin.current_health = 8
	final.snapshot.defense_selections = {}; final.snapshot.damage_sources = []
	final.snapshot.settled_damage.sources[0].final_amount = 4
	final.snapshot.settled_damage.committed = true
	for i in 2:
		final.snapshot.settled_damage.removals[i].released = true
		final.snapshot.settled_damage.removals[i].released_destination = "discard"
	final.events = [
		{"type": "damage_prevented_or_modified", "sequence": 100001, "actor_id": "goblin", "segment": "defensive", "data": {"source_id": source.id, "ability_id": "salt_veil", "damage_before": 6, "damage_after": 4}},
		{"type": "defense_selected", "sequence": 100002, "actor_id": "goblin", "segment": "defensive", "data": {"source_id": source.id, "ability_id": "salt_veil"}}
	]
	if human:
		screen.gateway._authority.enqueue(final)
		screen._send(JSON.stringify({"type": "pass", "actor_id": "blade", "payload": {}}))
	else: screen._apply_model_result(final)
	await _settle()
	_expect(screen._held_defense_view != null and screen._view.round_number == 5, "hold Round 5 Defense across Income handoff")
	if screen._held_defense_view == null: screen.queue_free(); await process_frame; return
	panel = screen._attack_intents[source.id]; panel.set_process(false)
	var feedbacks: Array = screen._root.get_children().filter(func(node): return node.get_script() == preload("res://presentation/battle/damage_response_feedback.gd"))
	_expect(feedbacks.size() == 1, "one synchronized saved-card animation")
	for feedback in feedbacks:
		feedback.set_process(false)
		_expect(feedback._saved.size() == 2 and feedback._pending.size() == 4, "two saved cards and four remaining cards keep source ownership: %d saved, %d pending" % [feedback._saved.size(), feedback._pending.size()])
	for elapsed in [0.0, TIMING.effects_seconds() * 0.4, TIMING.effects_seconds() + 0.1]:
		panel.started_ms = Time.get_ticks_msec() - ceili((TIMING.roll_seconds() + elapsed) * 1000)
		panel.data.roll_started_ms = Time.get_ticks_msec() - 9000
		panel._update()
		for feedback in feedbacks: feedback.present_progress(elapsed)
		_expect(screen._actor_profiles.goblin.health.value == 12, "health stays at twelve during prevention")
		if elapsed == 0: _expect(panel.target_heading.text.begins_with("6"), "attack starts at six")
		elif elapsed < TIMING.effects_seconds():
			_expect(panel.roll_area.visible and panel._prevention_progress[0] > 0 and panel._prevention_progress[0] < 1, "visible die-to-attack prevention trail")
			_expect(panel.attack_damage_rect() == panel.target_heading.get_global_rect(), "trail lands at outgoing attack below defender")
			if DisplayServer.get_name() != "headless":
				await process_frame
				RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/prevention-trail-%d-%s.png" % [width, human])
		else:
			_expect(panel.target_heading.text.begins_with("4"), "attack visibly settles at four before Income")
			if DisplayServer.get_name() != "headless":
				await process_frame
				RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/prevention-settled-%d-%s.png" % [width, human])
		screen._process(0)
		_expect(screen._view.round_number == 5, "phase cannot advance during the outcome hold")
	screen._defense_outcome_until = 0; screen._process(0); await _settle()
	_expect(screen._held_defense_view == null and screen._view.round_number == 6 and screen._actor_profiles.goblin.health.value == 8, "only after prevention does Income reveal eight health")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _native_sequence() -> void:
	canvas.size = Vector2i(1920,1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("native-enemy-defense",44)
	for step in 100:
		if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn",false): break
		if result.learned_policy.get("model_turn",false): result = gateway.advance_model()
		else: result = gateway.submit(JSON.stringify(_next_action(result.legal_actions)))
	_expect(result.snapshot.stage == "defense_selection", "native battle reaches Defense")
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("native-enemy-defense.json"))
	canvas.add_child(screen); await _settle()
	screen._director.clear(); screen._flow_until = 0
	for item in screen._timed_buttons: item.until = 0
	screen._process(0)
	canvas.notify_mouse_entered()
	var point: Vector2 = screen._auto_pass_button.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion,true)
	for pressed in [true,false]:
		var click := InputEventMouseButton.new(); click.button_index = MOUSE_BUTTON_LEFT; click.position = point; click.pressed = pressed; canvas.push_input(click,true)
	var faces := {}; var landed := false
	var prevention_trail := false; var reduced_attack := false; var positive_prevention := false
	var deadline := Time.get_ticks_msec()+45000
	while Time.get_ticks_msec() < deadline:
		await process_frame
		for panel in screen._attack_intents.values():
			if panel.data.actor_id == "blade" or panel.data.ability_name != "Salt Veil" or panel.dice_controls.is_empty() or not panel.roll_area.visible: continue
			_expect(panel.dice_controls[0].is_visible_in_tree(), "native Salt Veil displays its die")
			if int(panel.data.prevented) > 0 and int(panel.data.before) > 0:
				positive_prevention = true
				if panel._prevention_progress.any(func(t): return t > 0 and t < 1): prevention_trail = true
				if int(panel.damage.text) < int(panel.data.before): reduced_attack = true
			if panel.dice_controls[0].tooltip_text == "Rolling…": faces[panel.dice_controls[0].text] = true
			elif panel.benefit_labels[0].modulate.a > 0.99: landed = true
		if screen._view.round_number == 2 and screen._view.stage == "planning" and not screen._director.has_beats() and not screen._model_thinking: break
	_expect(positive_prevention and prevention_trail and reduced_attack, "native prevention trail visibly reduces outgoing attack before advancing")
	_expect(faces.size() > 1 and landed, "native Pass presents cycling defense dice then landed prevention")
	_expect(screen._view.round_number == 2 and screen._view.stage == "planning", "native defense sequence finishes and advances")
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _next_action(actions: Array) -> Dictionary:
	for kind in ["planning_roll","planning_reroll","roll_dice","planning_select_ability","planning_select_targets","planning_pass","pass"]:
		for action in actions:
			if action.type == kind: return action
	return {}
