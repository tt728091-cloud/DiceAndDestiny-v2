extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
	var base: Dictionary = gateway.start_battle("attack-intents", 43)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for count in [1, 2, 3, 4]:
			var fixture := base.duplicate(true)
			fixture.events = []; fixture.learned_policy = {}
			fixture.snapshot.actors.erase("goblin-2")
			for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
			fixture.snapshot.segment = "defensive"; fixture.snapshot.stage = "defense_selection"
			fixture.snapshot.damage_sources = []
			fixture.pending_input = {"blade": {"id": "intent", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass", "commit_interaction"]}}
			fixture.legal_actions = []
			for i in count:
				var id := "goblin" if i == 0 else "goblin-" + str(i + 1)
				fixture.snapshot.damage_sources.append({"id": "attack-" + id, "source_actor_id": id, "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 5, "status_applications": [{"target_actor_id": "blade", "status_id": "poison", "stacks": 2}]})
				fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "intent", "ability_id": "shedskin", "target_ids": ["attack-" + id]}})
			fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "needlefang", "base_amount": 3})
			var fake := FakeBattleAuthority.new(); fake.enqueue(fixture)
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake)
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("intent-test.json")); screen._auto_pass_disabled = true
			root.add_child(screen); screen.set_process(false)
			for frame in 8: await process_frame
			_expect(screen._attack_intents.size() == count + 1, "every source has an independent intent")
			for id in screen._attack_intents:
				var panel = screen._attack_intents[id]
				_expect(panel.get_parent() == screen._root and panel.get_theme_stylebox("panel") is StyleBoxEmpty, "no central attack box")
				_expect(root.get_visible_rect().encloses(panel.intent.get_global_rect()), "intent fits viewport")
				_expect(screen.attack_anchor_rect(id) == panel.damage.get_global_rect(), "source ID resolves exact live damage number")
				if id != "outgoing":
					_expect(panel.intent_row.get_children().any(func(child): return "Apply 2 Poison" in child.tooltip_text), "effect icon explains application on hover")
			# Switching attackers moves the same choices without covering other intents.
			for i in count:
				var source_id := "attack-goblin" if i == 0 else "attack-goblin-" + str(i + 1)
				await _click(screen._attack_intents[source_id].intent)
				for frame in 4: await process_frame
				_expect(screen._selected_source == source_id, "every enemy remains directly selectable beside the defense choices")
				_check_defense_position(screen, source_id)
			var target := "attack-goblin" if count == 1 else "attack-goblin-" + str(count)
			await _click(screen._attack_intents[target].intent)
			for frame in 6: await process_frame
			_expect(screen._selected_source == target, "clicking attacker intent selects its source")
			_expect(screen._defense_actions("shedskin").size() == 1 and screen._defense_actions("shedskin")[0].payload.target_ids == [target], "defense choices are specific to selected attack")
			await _capture("selection-%d-%d" % [count, viewport.x])
			# The fighter itself is also a hit target, and choosing a defense sends
			# the selected source rather than whichever opponent was focused before.
			await _click(screen._attack_intents[target].fighter_target)
			for frame in 6: await process_frame
			var choice: Button
			for button in screen._ability_dock.find_children("*", "Button", true, false):
				if str(button.get_meta("inspection_id", "")) == "battle.ability.blade.shedskin.choice.0": choice = button
			_expect(choice != null, "selected attack offers a defense button")
			if choice != null:
				fake.enqueue(fixture)
				await _click(choice)
				_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.target_ids == [target], "defense click sends the selected incoming source")
				fake.commands.clear()
			for frame in 6: await process_frame
			# Moving an attacker moves both the icon and animation destination.
			var panel = screen._attack_intents[target]
			var before: Vector2 = panel.intent.global_position
			var fighter: Control = screen._actor_profiles[panel.attacker_id].get_parent().fighter
			fighter.position += Vector2(-20, 12)
			panel._update()
			_expect(panel.intent.global_position.is_equal_approx(before + Vector2(-20, 12) * screen._root.scale), "intent follows fighter movement")
			# Source-targeted cards use the same visible intent, never an old box.
			fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "intent", "card_ids": ["ward"], "target_ids": [target]}})
			screen._view.apply_result(fixture)
			screen._selected_card = {"instance_id": "ward", "definition_id": "spiteful_ward", "source_targeting": true}
			screen._render()
			for frame in 6: await process_frame
			await _click(screen._attack_intents[target].intent)
			_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.target_ids == [target], "card click sends one command for the exact source")
			# Roll + prevention + status gain use the same clock, but new endpoints.
			fixture.snapshot.stage = "defense_reaction"; fixture.pending_input = {}; fixture.legal_actions = []
			fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "source_id": target, "ability_id": "shedskin", "rolled_faces": [1, 4]}}
			fixture.snapshot.actors.blade.statuses = []
			screen._selected_card.clear(); screen._reaction_card_feedback.clear(); screen._view.apply_result(fixture); screen._render()
			for frame in 8: await process_frame
			panel = screen._attack_intents[target]
			await create_timer(0.7).timeout
			panel.started_ms = Time.get_ticks_msec(); panel.data.roll_started_ms = panel.started_ms
			panel._update()
			var launch: Vector2 = panel.dice_controls[0].global_position
			if count == 1 and viewport.x == 1920:
				panel.data.roll_started_ms -= 450
				await _capture("rolling-dice")
				panel.data.roll_started_ms = panel.started_ms
			panel.started_ms -= int((TIMING.roll_seconds() + 0.05) * 1000); panel.data.roll_started_ms = panel.started_ms
			panel._update()
			_expect(panel.dice_controls[0].global_position.distance_to(launch) > 50, "dice travel to a landing location")
			_expect(panel.dice_controls[0].rotation == 0 and panel.dice_controls[0].text.ends_with("1"), "landed die shows authoritative face")
			_expect(panel.gain_origins[0] == panel.benefit_labels[1], "status trail originates at the die that granted it")
			if count == 1 and viewport.x == 1920:
				panel.started_ms = Time.get_ticks_msec() - int((TIMING.roll_seconds() + TIMING.effects_seconds() * 0.4) * 1000)
				for flight in screen._root.get_children():
					if flight.get_script() == preload("res://presentation/battle/defense_status_flight.gd"): flight.started_ms = panel.started_ms
				await _capture("effect-trails")
			panel.started_ms -= int((TIMING.effects_seconds() + 1) * 1000); panel._update()
			for flight in screen._root.get_children():
				if flight.get_script() == preload("res://presentation/battle/defense_status_flight.gd"):
					flight.started_ms = panel.started_ms; flight._process(0)
			_expect(panel.damage.text == "4", "prevention reduces attacker intent from five to four")
			_expect("Catalyst ×1" in screen._actor_profiles.blade.statuses.text, "landed defense benefit reaches the player's status icon")
			await _capture("landed-%d-%d" % [count, viewport.x])
			# Queued/completed defenses cannot be selected a second time.
			fixture.snapshot.stage = "defense_selection"; fixture.snapshot.defense_history = {target: {"finalized": true, "source_id": target, "ability_id": "shedskin", "rolled_faces": [1, 4]}}
			screen._view.apply_result(fixture); screen._render()
			for frame in 4: await process_frame
			_expect(screen._attack_intents[target].intent.disabled, "handled attack cannot take another defense")
			screen.queue_free(); await process_frame
	await _check_preview_and_card_flight(base)
	print("ATTACK INTENTS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check_defense_position(screen: Control, source_id: String) -> void:
	var rail: Control = screen._ability_dock.get_parent()
	var bounds := rail.get_global_rect()
	var target: Rect2 = screen._attack_intents[source_id].intent.get_global_rect()
	_expect(screen._ability_dock.get_meta("defense_source", "") == source_id, "defense dock owns the selected source")
	_expect(bounds.end.x < target.position.x and target.position.x - bounds.end.x < 20, "choices sit immediately to the left of selected attack")
	_expect(root.get_visible_rect().encloses(bounds), "defense choices fit the viewport")
	for other in screen._attack_intents.values():
		_expect(not bounds.intersects(other.intent.get_global_rect()), "choices leave every attack button exposed")

func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true)
		await process_frame
func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_INTENT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.7 if id.begins_with("selection") else 0.05).timeout; RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ATTACK INTENTS: " + message)

func _check_preview_and_card_flight(base: Dictionary) -> void:
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.damage_sources = []
	fixture.snapshot.actors.blade.selected_ability = "venom_gland"
	fixture.snapshot.actors.blade.selected_tier = "base"
	fixture.snapshot.actors.blade.selected_targets = ["goblin"]
	fixture.snapshot.actors.goblin.selected_ability = "brine_lash"
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("intent-preview.json")); screen._auto_pass_disabled = true
	root.add_child(screen); screen.set_process(false)
	for frame in 6: await process_frame
	var preview: Control = screen.ability_intent("blade", "venom_gland")
	_expect(preview != null, "player's locked status-only ability has an intent")
	_expect(not screen._attack_intents.has("preview:goblin"), "unrevealed enemy ability remains hidden")
	if preview != null:
		_expect(not preview.damage.visible, "status-only ability does not display a zero damage sword")
		_expect(preview.intent_row.get_children().any(func(child): return "Poison" in child.tooltip_text), "status-only intent explains Poison")
	# A card that modifies damage must end its trail at the exact intent number.
	screen._view.segment = "defensive"; screen._view.stage = "defense_reaction"
	screen._view.damage_sources = [{"id": "card-hit", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 5, "reaction_prevention": 1}]
	screen._reaction_card_feedback = {"battle_id": screen._view.battle_id, "actor_id": "blade", "source_id": "card-hit", "card_id": "spined_rebuttal", "instance_id": "test-card", "text": "Spined Rebuttal", "started_ms": Time.get_ticks_msec() - 600, "expires_ms": Time.get_ticks_msec() + 2200, "changes": [{"source_id": "card-hit", "before": 5, "after": 4}]}
	screen._render()
	for frame in 6: await process_frame
	var flight: Control
	for child in screen._root.get_children():
		if child.get_script() == preload("res://presentation/battle/attack_card_flight.gd"): flight = child
	_expect(flight != null, "card damage changes create a source-aware trail")
	if flight != null:
		# Freeze the tested point in the animation; rendering load must not
		# advance this assertion beyond the trail arrival.
		flight.feedback.started_ms = Time.get_ticks_msec() - 600
		flight._process(0)
		_expect(screen._attack_intents["card-hit"].damage.text == "5", "card holds prior damage until trail arrives")
		var expected: Vector2 = flight.get_global_transform_with_canvas().affine_inverse() * screen.attack_anchor_rect("card-hit").get_center()
		_expect(flight.endpoints.size() == 1 and flight.endpoints[0].is_equal_approx(expected), "card trail follows live attack anchor")
		flight.feedback.started_ms -= 1500; flight._process(0)
		_expect(screen._attack_intents["card-hit"].damage.text == "4", "card arrival shows reduced damage")
	screen.queue_free(); await process_frame
