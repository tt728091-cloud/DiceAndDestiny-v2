extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/defense_timing.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for character in ["adventurer", "venom"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", character)
		var base: Dictionary = gateway.start_battle("incoming-list-" + character, 43)
		for width in [1920, 1280]:
			canvas.size = Vector2i(width, width * 9 / 16)
			for count in [1, 4]:
				var fixture := base.duplicate(true)
				fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []
				fixture.snapshot.stage = "defense_selection"; fixture.snapshot.segment = "defensive"
				fixture.pending_input = {"blade": {"id": "choose", "stage": "defense_selection", "segment": "defensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
				fixture.snapshot.damage_sources = []
				for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				for i in count + 1:
					var actor := "goblin" if i in [0, count] else "goblin-" + str(i + 1)
					var id := "incoming-" + str(i)
					fixture.snapshot.damage_sources.append({"id": id, "source_actor_id": actor, "target_actor_id": "blade", "source_content_id": "brine_lash", "base_amount": 7, "status_applications": [{"target_actor_id": "blade", "status_id": "poison", "stacks": 2}]})
					for ability in fixture.snapshot.actors.blade.defensive_abilities:
						fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "choose", "ability_id": ability, "target_ids": [id]}})
				fixture.snapshot.damage_sources.append({"id": "outgoing", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "adventurer_strike", "base_amount": 4})
				var fake := FakeBattleAuthority.new()
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("incoming-list.json"))
				canvas.add_child(screen); screen.set_process(false)
				await _settle()
				_expect(screen._incoming_attack_rows.size() == count + 1 and not screen._incoming_attack_rows.has("outgoing"), "all incoming sources, including two from one enemy, each have one row")
				var row: Button = screen._incoming_attack_rows["incoming-0"]
				_expect(row.amount.text == "7" and row.state_label.text == "Undefended", "new source shows live amount and Undefended")
				_expect(row.body.get_children().any(func(child): return child is Label and "Brine Mask" in child.text), "attacker is named")
				_expect(row.body.get_children().any(func(child): return child is Label and "Poison" in child.text), "attack statuses remain visible")
				_expect("Brine Lash" in row.tooltip_text and "Current attack" in row.tooltip_text, "full shared ability hover")
				for presenter in screen._attack_intents.values():
					if str(presenter.data.actor_id) == screen.viewer_actor_id: _expect(not presenter.roll_area.visible, "defense dice remain concealed until an ability is chosen")
				for button in screen._action_footer.get_children():
					if button is Button:
						var local: Rect2 = screen._root.get_global_transform_with_canvas().affine_inverse() * button.get_global_rect()
						_expect(local.position.x >= 370 and local.end.x <= 434, "Pass is at the far right inside the station")
				await _capture("list-%s-%d-%d" % [character, count, width])
				# Select the final source, forcing the list to scroll for four enemies.
				var target := "incoming-" + str(count)
				screen._incoming_attack_list.ensure_control_visible(screen._incoming_attack_rows[target]); await _settle()
				var row_positions := {}
				for id in screen._incoming_attack_rows: row_positions[id] = screen._incoming_attack_rows[id].global_position
				var reserved: Rect2 = screen._incoming_attack_list.get_global_rect()
				await _click(screen._incoming_attack_rows[target]); await _settle()
				for id in row_positions: _expect(screen._incoming_attack_rows[id].global_position.distance_to(row_positions[id]) <= screen._root.scale.x + 0.01, "opening popup never moves another attack %s: %s -> %s (scroll %s)" % [id, row_positions[id], screen._incoming_attack_rows[id].global_position, screen._incoming_attack_list.scroll_vertical])
				_expect(screen._selected_source == target and screen._defense_choice_local, "list pointer selects exact source")
				var rail: Control = screen._ability_dock.get_parent()
				_expect(rail.get_parent() == screen._root, "choices use a separate popup")
				_expect(rail.get_global_rect().position.x > screen._incoming_attack_list.get_global_rect().end.x, "popup opens to the right of the list")
				_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(rail.get_global_rect()), "whole popup stays onscreen")
				_expect(not rail.get_global_rect().intersects(screen._incoming_attack_list.get_global_rect()), "popup never obscures any incoming attack")
				var station: Rect2 = screen._root.get_node("PlayerControlsFrame").get_global_rect()
				_expect(station.encloses(screen._incoming_attack_list.get_global_rect()), "scrolling list stays inside lower-left panel")
				await _capture("choices-%s-%d-%d" % [character, count, width])
				# Overhead entry still opens the menu beside the enemy, and the local
				# entry switches it back without stale source identity.
				await _click(screen._attack_intents[target].intent); await _settle()
				_expect(not screen._defense_choice_local and screen._ability_dock.get_parent().get_parent() == screen._root, "overhead entry retains adjacent menu")
				screen._incoming_attack_list.ensure_control_visible(screen._incoming_attack_rows[target]); await _settle()
				await _click(screen._incoming_attack_rows[target]); await _settle()
				var choice: Button
				for button in screen._ability_dock.find_children("*", "Button", true, false):
					if not button.disabled and (button is BattleAbilityTile or button.has_meta("ability_action")): choice = button; break
				_expect(choice != null, "legal local defense button is reachable")
				if choice != null:
					await _settle()
					fake.enqueue(fixture)
					await _click(choice); await _settle()
					_expect(fake.commands.size() == 1, "local defense pointer submits exactly once")
					if fake.commands.size() == 1: _expect(JSON.parse_string(fake.commands[0]).payload.target_ids == [target], "defense targets exact source, including same-enemy second attack")
				# Display a real authored defense result with deterministic faces.
				var rolling := fixture.duplicate(true); rolling.legal_actions = []; rolling.pending_input = {}
				rolling.snapshot.stage = "defense_reaction"
				var ability := "adventurer_guard" if character == "adventurer" else "shedskin"
				var selection := {"actor_id": "blade", "source_id": target, "ability_id": ability, "rolled_face": 4, "rolled_faces": [4, 4, 4] if character == "adventurer" else [1, 4]}
				rolling.snapshot.defense_selections = {"blade": selection}
				screen._view.apply_result(rolling); screen._render(true); screen._flow_transition.finish(); await _settle()
				var panel: Control = screen._attack_intents[target]
				panel.started_ms = Time.get_ticks_msec(); panel.data.roll_started_ms = panel.started_ms; panel._damage_settled = false; panel._update(); await _settle()
				row = screen._incoming_attack_rows[target]
				_expect(row.state_label.text == "Defending…" and row.amount.text == "7", "active roll has its own state and unreduced amount")
				_expect(row.disabled, "in-progress defense cannot be chosen again")
				_expect(screen._incoming_attack_rows["incoming-0"].state_label.text == "Undefended", "defending one source leaves another undefended")
				_expect(panel.attack_damage_rect() == row.damage_rect(), "prevention endpoint is local list, never overhead enemy")
				_expect(screen._incoming_attack_list.get_global_rect() == reserved, "dice use the pre-reserved band without shifting list")
				for i in panel.roll_cells.size():
					var die: Button = panel.dice_controls[i]
					_expect(die.is_visible_in_tree(), "selected defense dice are visible")
					_expect(die.size.is_equal_approx(BattleDiceTray.HUD_DIE_SIZE), "defense dice match offensive dice size")
					var expected: Vector2 = screen._root.get_global_transform_with_canvas() * (screen.DEFENSE_DICE_ORIGIN + Vector2(i * 62, 0))
					_expect(die.global_position.is_equal_approx(expected), "dice start at top left and run horizontally: %s != %s" % [die.global_position, expected])
					_expect(not panel.roll_cells[i].get_global_rect().intersects(screen._incoming_attack_list.get_global_rect()), "dice roll above the incoming list")
				panel.started_ms = Time.get_ticks_msec() - int((TIMING.total_seconds() + 1) * 1000); panel.data.roll_started_ms = panel.started_ms; panel._update(); await _settle()
				_expect(row.amount.text == str(panel.data.after) and int(row.amount.text) < 7, "authored prevention updates list with same countdown result")
				_expect(row.state_label.text == "", "Undefended disappears after defense settles")
				_expect(row.get_parent().get_index() == screen._incoming_attack_rows.size(), "defended attack sorts after every undefended attack")
				_expect(screen._incoming_attack_list.scroll_vertical == 0, "next undefended attacks return to top")
				await _capture("result-%s-%d-%d" % [character, count, width])
				# Reloaded history and a queued source stay distinct from untouched attacks.
				var first_completed := selection.duplicate(true); first_completed.source_id = "incoming-0"
				rolling.snapshot.defense_history = {target: selection, "incoming-0": first_completed}
				rolling.snapshot.defense_plans = {"incoming-2": {"ability_id": ability, "source_id": "incoming-2"}} if count > 1 else {}
				screen._view.apply_result(rolling); screen._render(true); screen._flow_transition.finish(); await _settle()
				_expect(screen._incoming_attack_rows[target].state_label.text == "", "completed defense survives redraw/history")
				_expect(screen._incoming_attack_rows["incoming-0"].state_label.text == "", "completed first attack retains completion after redraw")
				if count > 1:
					_expect(screen._incoming_attack_rows["incoming-1"].get_parent().get_index() == 1, "undefended attacks sort above completed first source after reload")
					_expect(screen._incoming_attack_rows["incoming-2"].state_label.text == "Queued" and screen._incoming_attack_rows["incoming-2"].get_parent().get_index() == 3, "queued defense sorts between undefended and completed")
				# Card-first prevention can use the new list without selecting a defense.
				fixture.pending_input.blade.allowed_commands.append("commit_interaction")
				fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "commit_interaction", "payload": {"pending_input_id": "choose", "card_ids": ["ward"], "target_ids": [target]}})
				screen._view.apply_result(fixture)
				screen._selected_card = {"instance_id": "ward", "definition_id": "spiteful_ward", "source_targeting": true}
				screen._render(); await _settle()
				fake.commands.clear(); fake.enqueue(fixture)
				screen._incoming_attack_list.ensure_control_visible(screen._incoming_attack_rows[target]); await _settle()
				await _click(screen._incoming_attack_rows[target])
				_expect(fake.commands.size() == 1, "card-first list click sends one command")
				if fake.commands.size() == 1: _expect(JSON.parse_string(fake.commands[0]).payload.target_ids == [target], "card-first list preserves exact source target")
				screen.active_store.clear(); screen.queue_free(); await process_frame
	for ability_id in ["adventurer_guard", "adventurer_guard_plus"]:
		for auto_pass_disabled in [false, true]:
			await _live_defense(ability_id, auto_pass_disabled)
	print("INCOMING ATTACK LIST: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _live_defense(ability_id: String, auto_pass_disabled: bool) -> void:
	canvas.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	gateway.unified_defense = true
	var result: Dictionary = gateway.start_battle("incoming-list-live", 44)
	for step in 100:
		if not result.get("accepted", false): break
		if result.snapshot.stage == "defense_selection" and not result.learned_policy.get("model_turn", false): break
		if result.learned_policy.get("model_turn", false): result = gateway.advance_model()
		else: result = gateway.submit(JSON.stringify(_next_action(result.legal_actions)))
	_expect(result.get("accepted", false) and result.snapshot.stage == "defense_selection", "native battle reaches defense hub")
	var target := ""
	for action in result.get("legal_actions", []):
		if action.type == "planning_select_ability" and action.payload.get("ability_id") == ability_id: target = str(action.payload.target_ids[0]); break
	_expect(not target.is_empty(), "native battle offers Guard")
	if target.is_empty(): return
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway
	screen.learned_battle_mode = true; screen._auto_pass_disabled = auto_pass_disabled
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("incoming-live.json"))
	canvas.add_child(screen)
	await _settle()
	var deadline := Time.get_ticks_msec() + 12000
	while Time.get_ticks_msec() < deadline and (not screen._incoming_attack_rows.has(target) or screen._incoming_attack_rows[target].disabled): await process_frame
	_expect(screen._incoming_attack_rows.has(target) and not screen._incoming_attack_rows[target].disabled, "native incoming row accepts pointer")
	await _capture("native-before")
	if screen._incoming_attack_rows.has(target):
		screen._incoming_attack_list.ensure_control_visible(screen._incoming_attack_rows[target]); await _settle()
		await _click(screen._incoming_attack_rows[target]); await _settle()
		var choice: Button
		for button in screen._ability_dock.find_children("*", "Button", true, false):
			if str(button.get_meta("inspection_id", "")) == "battle.ability.blade." + ability_id: choice = button
		_expect(choice != null and not choice.disabled, "native local Guard is reachable")
		if choice != null and not choice.disabled:
			await _settle()
			await _click(choice)
			deadline = Time.get_ticks_msec() + 15000
			while Time.get_ticks_msec() < deadline and screen._view.stage != "defense_reaction":
				_expect_no_apply(screen)
				await process_frame
			_expect(screen._view.stage == "defense_reaction" and screen._error_message.is_empty(), "pointer defense rolls through native authority")
			var panel: Control = screen._attack_intents.get(target)
			if panel != null:
				_expect(int(panel.data.prevented) > 0, "live defense actually prevents damage")
				_expect(panel.attack_damage_rect() == screen._incoming_attack_rows[target].damage_rect(), "live reduction targets local row")
			await _settle()
			await _capture("native-rolled")
			_expect_no_apply(screen)
			var original_actions: Array = screen._view.legal_actions.duplicate(true)
			screen._view.legal_actions.append({"type": "commit_interaction", "actor_id": "blade"})
			_expect(screen._sole_pass_action().get("type") == "planning_pass", "optional reactions do not require an Apply button")
			screen._view.legal_actions = original_actions
			# Targeting a die/card must pause automatic completion without ending
			# the participant's remaining defense opportunities.
			screen._selected_card = {"die_targeting": true}
			_expect(screen._sole_pass_action().is_empty(), "card targeting pauses roll completion")
			screen._selected_card.clear()
			_expect(screen._sole_pass_action().get("type") == "planning_pass", "roll completion stays automatic regardless of auto-pass preference")
			deadline = Time.get_ticks_msec() + 12000
			while Time.get_ticks_msec() < deadline and screen._held_defense_view == null and screen._view.stage != "defense_selection":
				_expect_no_apply(screen)
				if screen._view.stage == "defense_reaction" and screen._attack_intents.has(target):
					var preview: Control = screen._attack_intents[target]
					_expect(preview.damage.text == str(preview.data.before), "damage waits for authoritative saved-card feedback")
				await process_frame
			_expect(screen._held_defense_view != null, "finalized defense holds the board for its shared feedback clock")
			await process_frame
			var held_saved: Array = screen._damage_feedback.get("saved", [])
			_expect(not held_saved.is_empty(), "saved-card animation receives released cards while the finalized defense is held")
			var held_found := false
			for control in screen._root.get_children():
				if not control.has_method("prevention_origin"): continue
				held_found = true
				_expect(screen.attack_anchor_rect(target).has_point(control.get_global_transform_with_canvas() * control.prevention_origin()), "saved-card trails start at the local incoming amount")
			_expect(held_found, "native saved-card feedback visible while the defense is held")
			await _verify_saved_animation(screen, target)
			while Time.get_ticks_msec() < deadline and screen._view.stage != "defense_selection": await process_frame
			_expect(screen._view.stage == "defense_selection", "roll automatically returns to same defense hub")
			_expect(is_instance_valid(screen._auto_pass_button) and screen._auto_pass_button.is_visible_in_tree(), "main Pass remains reachable in defense hub")
			_expect(screen._incoming_attack_rows.has(target) and screen._incoming_attack_rows[target].state_label.text == "", "completed native defense clears Undefended")
			var saved: Array = screen._damage_feedback.get("saved", [])
			_expect(not saved.is_empty(), "saved-card animation still receives released cards")
			for card in saved:
				_expect(card.get("origin_rect", Rect2()).position.x > 1450, "saved cards animate from right-side pending list")
				_expect(card.get("released_destination") == (card.get("original_zone") if ability_id.ends_with("_plus") else "discard"), "Guard animation honors authoritative saved destination")
			deadline = Time.get_ticks_msec() + 12000
			while Time.get_ticks_msec() < deadline and (screen._model_thinking or screen._view.learned_policy.get("model_turn", false)): await process_frame
			_expect(screen._view.stage == "defense_selection", "completion does not act as main Pass")
			_expect(screen._view.legal_actions.any(func(action): return action.type == "planning_select_ability"), "the second incoming attack remains defendable")
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _verify_saved_animation(screen, source_id: String) -> void:
	var feedback: Control
	for control in screen._root.get_children():
		if control.has_method("prevention_origin"): feedback = control
	_expect(is_instance_valid(feedback), "saved-card presenter exists")
	if not is_instance_valid(feedback): return
	var overlap := false
	var captured := false
	var deadline := Time.get_ticks_msec() + 3000
	while is_instance_valid(feedback) and Time.get_ticks_msec() < deadline and feedback._elapsed < 2.3:
		await process_frame
		if not is_instance_valid(feedback): break
		var amount := int(screen._attack_intents[source_id].damage.text)
		var flying: bool = feedback._saved.any(func(entry): return entry.progress > 0 and entry.progress < 1)
		if amount < feedback._before and amount > feedback._after and flying:
			overlap = true
			if not captured:
				await _capture("native-synchronized-saves"); captured = true
		for entry in feedback._pending:
			_expect(entry.card.size.y == 24 and entry.card.scale.is_equal_approx(Vector2.ONE), "held headers keep their natural height and text scale")
		for entry in feedback._saved:
			_expect(is_equal_approx(entry.card.scale.x, entry.card.scale.y), "saved headers scale uniformly while flying")
		for grid in feedback._hidden_grids:
			_expect(grid.source_id == source_id, "only the changed attack list is replaced during feedback")
		for grid in screen._damage_grids:
			if grid.source_id == source_id: continue
			_expect(grid.modulate.a == 1, "unrelated card lists retain their live headers")
		_expect(not screen._root.get_children().any(func(node): return node.get_script() == preload("res://presentation/battle/attack_card_flight.gd")), "no duplicate prevention animation")
	_expect(overlap, "damage decreases during saved-card flights")

func _expect_no_apply(screen) -> void:
	for button in screen._action_footer.get_children():
		if button is Button:
			_expect(button.get_meta("inspection_id", "") != "battle.command.planning_pass", "no Apply action during dice playback")
			_expect("Apply" not in button.text, "no Apply caption during dice playback")
	_expect(screen._action_footer.get_child_count() <= 1, "only main Pass occupies the defense footer")

func _next_action(actions: Array) -> Dictionary:
	for kind in ["planning_roll", "planning_reroll", "roll_dice", "planning_select_ability", "planning_select_targets", "planning_pass", "pass"]:
		for action in actions:
			if action.type == kind: return action
	return {}

func _settle() -> void:
	for frame in 12: await process_frame
func _click(control: Control) -> void:
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; canvas.push_input(event, true)
		await process_frame
func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_INCOMING_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png(directory.path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("INCOMING ATTACK LIST: " + message)
