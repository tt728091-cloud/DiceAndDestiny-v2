extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	OS.set_environment("DICE_AND_DESTINY_ENABLE_SNAPSHOTS", "1")
	var packed: PackedScene = load("res://app/screens/battle/battle_screen.tscn")
	for size in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = size
		var screen = packed.instantiate(); screen.initial_result = _fixture(); root.add_child(screen)
		await process_frame; await process_frame
		if screen.get_rect().size.x < size.x or screen.find_children("*", "ActorProfile", true, false).size() != 2: _fail("scene/component layout failed at %s" % size); return
		var buttons := screen.find_children("*", "Button", true, false)
		if buttons.is_empty(): _fail("scene has no accessible controls"); return
		for button in buttons:
			if button.text.is_empty() and button.tooltip_text.is_empty(): _fail("button lacks accessible text"); return
		var texts: Array[String] = []
		for button in buttons: texts.append(button.text)
		if "Roll 5 Dice" not in texts or "Keep Selected" in texts or "Reroll Unkept" in texts: _fail("initial planning exposed controls that are invalid before the first roll: %s" % texts); return
		screen.queue_free(); await process_frame
	var fake := FakeBattleAuthority.new()
	fake.enqueue(_rolled_fixture("p2", 1, []))
	fake.enqueue(_rolled_fixture("p3", 2, [0, 2]))
	var reroll_screen = packed.instantiate()
	reroll_screen.initial_result = _rolled_fixture("p1", 1, [])
	reroll_screen.gateway = BattleGateway.new(fake)
	reroll_screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_toggle_keep_active.json"))
	root.add_child(reroll_screen)
	await process_frame; await process_frame
	if _has_button(reroll_screen, "Keep Selected") or not _has_button(reroll_screen, "Reroll Unkept"):
		_fail("rolled planning did not expose the single-action reroll control"); return
	var player_tray: BattleDiceTray = reroll_screen.find_children("*", "BattleDiceTray", true, false)[0]
	var player_dice := player_tray.find_children("*", "Button", true, false)
	player_dice[0].button_pressed = true; player_dice[2].button_pressed = true
	await process_frame
	if _int_array(reroll_screen.inspection_state().get("selected_dice", [])) != [0, 2]:
		_fail("clicking dice did not immediately establish the kept set"); return
	_button(reroll_screen, "Reroll Unkept").pressed.emit()
	await process_frame; await process_frame
	if fake.commands.size() != 2:
		_fail("reroll UI did not chain exactly one keep and one reroll command: %s" % fake.commands); return
	var keep_command: Dictionary = JSON.parse_string(fake.commands[0]); var reroll_command: Dictionary = JSON.parse_string(fake.commands[1])
	if keep_command.get("type") != "planning_keep" or _int_array(keep_command.get("payload", {}).get("kept_indices", [])) != [0, 2]:
		_fail("reroll UI submitted the wrong kept dice: %s" % fake.commands[0]); return
	if reroll_command.get("type") != "planning_reroll" or _int_array(reroll_command.get("payload", {}).get("reroll_indices", [])) != [1, 3, 4]:
		_fail("reroll UI submitted the wrong unkept dice: %s" % fake.commands[1]); return
	if reroll_command.get("payload", {}).get("pending_input_id") != "p2":
		_fail("reroll UI did not use the refreshed authority checkpoint: %s" % fake.commands[1]); return
	if _int_array(reroll_screen.inspection_state().get("selected_dice", [])) != [0, 2]:
		_fail("kept selection did not persist after reroll"); return
	player_tray = reroll_screen.find_children("*", "BattleDiceTray", true, false)[0]
	player_dice = player_tray.find_children("*", "Button", true, false)
	if not player_dice[0].button_pressed or player_dice[1].button_pressed or not player_dice[2].button_pressed:
		_fail("rerendered dice did not visually preserve the kept highlights"); return
	player_dice[0].button_pressed = false
	await process_frame
	if _int_array(reroll_screen.inspection_state().get("selected_dice", [])) != [2]:
		_fail("a previously kept die could not be unselected for the next reroll"); return
	reroll_screen.active_store.clear(); reroll_screen.queue_free(); await process_frame
	var offensive_reveal = packed.instantiate(); var offensive_fixture := _fixture()
	offensive_fixture.snapshot.stage = "offensive_reaction"; offensive_fixture.pending_input.blade.stage = "offensive_reaction"; offensive_fixture.pending_input.blade.allowed_commands = ["pass"]
	offensive_fixture.snapshot.actors.blade.roll_history = [{"dice": [{"face": 4}, {"face": 1}, {"face": 4}, {"face": 2}, {"face": 5}]}]
	offensive_fixture.events = [{"sequence": 1, "type": "interaction_revealed", "segment": "offensive", "data": {"commitments": {
		"blade": {"ability_id": "sword_cut", "tier_id": "five_swords", "targets": ["goblin"], "outcome": {"base_damage": 7, "status_applications": [{"status_id": "bleed", "stacks": 1}, {"status_id": "bleed", "stacks": 1}], "resource_gains": {}, "targets": ["goblin"]}},
		"goblin": {"ability_id": "jagged_slash", "tier_id": "three_swords", "ai_d100": 7, "simulated_rolls": 1, "targets": ["blade"], "dice": [{"face": 1}, {"face": 2}, {"face": 3}, {"face": 4}, {"face": 5}], "outcome": {"base_damage": 4, "status_applications": [], "resource_gains": {}, "targets": ["blade"]}}
	}}}]
	offensive_reveal.initial_result = offensive_fixture; root.add_child(offensive_reveal)
	await process_frame; await process_frame
	var enemy_die_ids: Array[String] = []; var saw_enemy_caption := false; var saw_player_outcome := false; var saw_enemy_outcome := false; var leaked_generic_rules := false
	for label in offensive_reveal.find_children("*", "Label", true, false):
		if label.text == "ENEMY OFFENSIVE DICE · Simulated rolls 1": saw_enemy_caption = true
		if "Sword Cut\nPending: 7 damage\nApplies: Bleed ×2\nTarget: Venom Goblin" in label.text: saw_player_outcome = true
		if "Jagged Slash\nPending: 4 damage\nTarget: Blade Warden" in label.text: saw_enemy_outcome = true
		if "Deal 5 / 6 / 7 damage" in label.text or "Deal 4 / 5 / 6 damage" in label.text: leaked_generic_rules = true
	for button in offensive_reveal.find_children("*", "Button", true, false):
		if button.has_meta("inspection_id") and str(button.get_meta("inspection_id")).begins_with("battle.die.goblin.") and button.text != "—": enemy_die_ids.append(str(button.get_meta("inspection_id")))
	if not saw_enemy_caption or enemy_die_ids.size() != 5: _fail("offensive reaction did not reveal all five simulated enemy dice: %s" % enemy_die_ids); return
	if not saw_player_outcome or not saw_enemy_outcome or leaked_generic_rules: _fail("offensive reaction did not show only resolved outcomes: player=%s enemy=%s generic=%s" % [saw_player_outcome, saw_enemy_outcome, leaked_generic_rules]); return
	offensive_reveal.queue_free(); await process_frame
	var presenting = packed.instantiate(); var presentation_fixture := _fixture()
	presentation_fixture.events = [{"sequence": 1, "type": "segment_entered", "to": "ongoing_effects"}]
	presenting.initial_result = presentation_fixture; root.add_child(presenting)
	await process_frame; await process_frame
	var found_continue := false
	for button in presenting.find_children("*", "Button", true, false):
		if button.text == "Continue Presentation": found_continue = true
		elif button.text != "DEV SNAPSHOTS" and not button.disabled: _fail("gameplay control remained enabled during automatic presentation: %s" % button.text); return
	if not found_continue: _fail("automatic presentation did not expose its continue control"); return
	presenting.queue_free(); await process_frame
	ProjectSettings.set_setting("dice_and_destiny/presentation/income_animation_seconds", 0.5)
	var income_screen = packed.instantiate(); var income_fixture := _fixture()
	income_fixture.snapshot.actors.blade.energy_points = 2; income_fixture.snapshot.actors.blade.deck_count = 15; income_fixture.snapshot.actors.blade.hand_count = 5; income_fixture.snapshot.actors.blade.hand = ["income-card"]; income_fixture.snapshot.actors.blade.card_instances = {"income-card": {"instance_id": "income-card", "definition_id": "loaded_die"}}
	income_fixture.snapshot.actors.goblin.energy_points = 1; income_fixture.snapshot.actors.goblin.deck_count = 9; income_fixture.snapshot.actors.goblin.hand_count = 3
	income_fixture.events = [
		{"sequence": 1, "type": "segment_entered", "to": "ongoing_effects"},
		{"sequence": 2, "type": "segment_entered", "to": "income"},
		{"sequence": 3, "type": "cards_drawn", "actor_id": "blade", "cards": ["income-card"]},
		{"sequence": 4, "type": "energy_points_gained", "actor_id": "blade", "energy_points": 2},
		{"sequence": 5, "type": "cards_drawn", "actor_id": "goblin", "count": 1},
		{"sequence": 6, "type": "energy_points_gained", "actor_id": "goblin", "energy_points": 1},
	]
	income_screen.initial_result = income_fixture; root.add_child(income_screen)
	await process_frame; await process_frame
	if _button(income_screen, "Continue Presentation") == null: _fail("pre-income Effects presentation was unavailable"); return
	var pre_income_markers := 0; var effects_showed_pre_income_energy := false; var effects_showed_pre_income_hand := false
	for effects_label in income_screen.find_children("*", "Label", true, false):
		if effects_label.visible and str(effects_label.text).begins_with("▲ "): pre_income_markers += 1
		if effects_label.text == "✦ Energy 1": effects_showed_pre_income_energy = true
		if effects_label.text == "Hand 4": effects_showed_pre_income_hand = true
	if pre_income_markers != 0 or not effects_showed_pre_income_energy or not effects_showed_pre_income_hand: _fail("Effects exposed post-income totals before the income animation"); return
	_button(income_screen, "Continue Presentation").pressed.emit(); await process_frame; await process_frame
	if _button(income_screen, "Continue Presentation") != null: _fail("income animation retained the manual Continue Presentation gate"); return
	var visible_markers := 0; var saw_pre_income_energy := false; var saw_pre_income_hand := false
	for income_label in income_screen.find_children("*", "Label", true, false):
		if income_label.visible and str(income_label.text).begins_with("▲ "): visible_markers += 1
		if income_label.text == "✦ Energy 1": saw_pre_income_energy = true
		if income_label.text == "Hand 4": saw_pre_income_hand = true
	var income_cards := income_screen.find_children("*", "BattleCard", true, false)
	if visible_markers < 6 or not saw_pre_income_energy or not saw_pre_income_hand or income_cards.size() != 1: _fail("income board did not preview all resource ticks and the drawn hand card: markers=%d cards=%d" % [visible_markers, income_cards.size()]); return
	await create_timer(0.55).timeout; await process_frame; await process_frame
	if income_screen.inspection_state().get("presentation_active", true) or _button(income_screen, "Roll 5 Dice") == null: _fail("income presentation did not advance itself into offense"); return
	ProjectSettings.set_setting("dice_and_destiny/presentation/income_animation_seconds", 2.0)
	income_screen.queue_free(); await process_frame
	var status_damage = packed.instantiate(); var status_fixture := _fixture()
	status_fixture.snapshot.segment = "ongoing_effects"; status_fixture.snapshot.stage = "status_damage_reaction"
	status_fixture.pending_input.blade.segment = "ongoing_effects"; status_fixture.pending_input.blade.stage = "status_damage_reaction"; status_fixture.pending_input.blade.allowed_commands = ["commit_interaction", "pass"]
	status_fixture.snapshot.actors.blade.hand = ["ward-1"]; status_fixture.snapshot.actors.blade.hand_count = 1
	status_fixture.snapshot.actors.blade.card_instances = {"ward-1": {"instance_id": "ward-1", "definition_id": "emergency_ward"}}
	status_fixture.snapshot.damage_sources = [{"id": "stale-player-attack", "source_content_id": "sword_cut", "target_actor_id": "goblin", "base_amount": 7, "prevention": 0, "final_amount": 0}, {"id": "stale-enemy-attack", "source_content_id": "venom_strike", "target_actor_id": "blade", "base_amount": 3, "prevention": 6, "final_amount": 0}]
	status_fixture.snapshot.settled_damage = {"sources": [{"id": "poison-1", "source_content_id": "poison", "target_actor_id": "blade", "base_amount": 1, "final_amount": 1}, {"id": "bleed-1", "source_content_id": "bleed", "target_actor_id": "goblin", "base_amount": 1, "final_amount": 1}, {"id": "bleed-2", "source_content_id": "bleed", "target_actor_id": "goblin", "base_amount": 1, "final_amount": 1}], "removals": [{"card_id": "ward-1", "card_definition_id": "emergency_ward", "target_actor_id": "blade", "original_zone": "hand", "accepted": true, "released": false}], "status_applications": [{"target_actor_id": "blade", "status_id": "poison", "stacks": 2}, {"target_actor_id": "goblin", "status_id": "bleed", "stacks": 1}, {"target_actor_id": "goblin", "status_id": "bleed", "stacks": 1}]}
	status_fixture.events = [{"sequence": 1, "type": "damage_cards_revealed", "segment": "ongoing_effects", "data": {"cards": [{"card_id": "ward-1"}]}}]
	status_damage.initial_result = status_fixture; root.add_child(status_damage)
	await process_frame; await process_frame
	var saw_status_context := false; var saw_status_card := false; var saw_player_poison := false; var saw_enemy_bleed := false; var saw_poison_damage := false; var saw_bleed_damage := false; var leaked_stale_attack := false
	for label in status_damage.find_children("*", "Label", true, false):
		if label.text == "STATUS DAMAGE · ONGOING EFFECTS": saw_status_context = true
		if label.text == "BLADE WARDEN  ☠ Poison ×2": saw_player_poison = true
		if label.text == "VENOM GOBLIN  ◆ Bleed ×2": saw_enemy_bleed = true
		if label.text == "Poison → Blade Warden\n1 incoming · 0 prevented · 1 pending": saw_poison_damage = true
		if label.text == "Bleed → Venom Goblin\n2 incoming · 0 prevented · 2 pending": saw_bleed_damage = true
	for button in status_damage.find_children("*", "Button", true, false):
		if button.text.begins_with("Emergency Ward") and not button.disabled: _fail("Emergency Ward was enabled outside the damage-resolution segment"); return
		if button.has_meta("inspection_id") and str(button.get_meta("inspection_id")) == "battle.damage_card.ward-1": saw_status_card = true
		if button.text == "Continue Presentation": _fail("status damage reveal created a redundant presentation popup"); return
		if "Sword Cut" in button.text or "Venom Strike" in button.text: leaked_stale_attack = true
	if not saw_status_context or not saw_status_card or not saw_player_poison or not saw_enemy_bleed or not saw_poison_damage or not saw_bleed_damage or leaked_stale_attack: _fail("status damage board omitted current totals or rendered stale offensive sources"); return
	status_damage.queue_free(); await process_frame
	var defense_roll_fake := FakeBattleAuthority.new()
	var defense_roll_result := _fixture()
	defense_roll_result.snapshot.segment = "defensive"; defense_roll_result.snapshot.stage = "defense_reaction"
	defense_roll_result.pending_input.blade.segment = "defensive"; defense_roll_result.pending_input.blade.stage = "defense_reaction"; defense_roll_result.pending_input.blade.allowed_commands = ["pass"]
	defense_roll_result.snapshot.damage_sources = [{"id": "source-player", "source_content_id": "golden_edge", "target_actor_id": "goblin", "base_amount": 5, "final_amount": 0}, {"id": "source-enemy", "source_content_id": "jagged_slash", "target_actor_id": "blade", "base_amount": 4, "final_amount": 0}]
	defense_roll_result.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "source-enemy", "rolled_face": 5}, "goblin": {"actor_id": "goblin", "ability_id": "basic_defense", "source_id": "source-player", "rolled_face": 3}}
	defense_roll_result.events = [{"sequence": 1, "type": "dice_rolled", "actor_id": "blade", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 5}]}, {"sequence": 2, "type": "dice_rolled", "actor_id": "goblin", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 3}]}]
	defense_roll_fake.enqueue(defense_roll_result)
	var defense_roll = packed.instantiate(); var defense_roll_fixture := _fixture()
	defense_roll_fixture.snapshot.segment = "defensive"; defense_roll_fixture.snapshot.stage = "defense_roll"
	defense_roll_fixture.pending_input.blade.segment = "defensive"; defense_roll_fixture.pending_input.blade.stage = "defense_roll"; defense_roll_fixture.pending_input.blade.allowed_commands = ["roll_dice"]
	defense_roll_fixture.snapshot.damage_sources = defense_roll_result.snapshot.damage_sources.duplicate(true)
	defense_roll_fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "source-enemy"}}
	defense_roll_fixture.snapshot.actors.blade.selected_ability = "sword_cut"
	defense_roll.initial_result = defense_roll_fixture; defense_roll.gateway = BattleGateway.new(defense_roll_fake); defense_roll.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_defense_die_active.json")); root.add_child(defense_roll)
	await process_frame; await process_frame
	var pending_die: Button = null; var saw_selected_defense := false; var leaked_roll_label := false; var leaked_enemy_before_reveal := false
	for label in defense_roll.find_children("*", "Label", true, false):
		if label.has_meta("inspection_id") and label.get_meta("inspection_id") == "battle.defense_selected.blade" and label.text.begins_with("Basic Defense"): saw_selected_defense = true
	for button in defense_roll.find_children("*", "Button", true, false):
		if button.text == "Roll Defense Die": leaked_roll_label = true
		if button.has_meta("inspection_id") and button.get_meta("inspection_id") == "battle.defense_die.blade.pending": pending_die = button
		if button.has_meta("inspection_id") and str(button.get_meta("inspection_id")).begins_with("battle.defense_die.goblin"): leaked_enemy_before_reveal = true
	if not saw_selected_defense or pending_die == null or not pending_die.text.is_empty() or leaked_roll_label or leaked_enemy_before_reveal: _fail("pre-reveal defense UI did not expose only the player's blank die"); return
	var pre_roll_die_position := pending_die.global_position
	pending_die.pressed.emit(); await process_frame; await process_frame
	if defense_roll_fake.commands.size() != 1 or JSON.parse_string(defense_roll_fake.commands[0]).get("type") != "roll_dice": _fail("blank defense die did not submit roll_dice: %s" % defense_roll_fake.commands); return
	var player_rolled_die: Button = null; var enemy_rolled_die: Button = null; var leaked_enemy_pending_die := false
	for button in defense_roll.find_children("*", "Button", true, false):
		if button.has_meta("inspection_id") and button.get_meta("inspection_id") == "battle.defense_die.blade" and "5" in button.text: player_rolled_die = button
		if button.has_meta("inspection_id") and button.get_meta("inspection_id") == "battle.defense_die.goblin" and "3" in button.text: enemy_rolled_die = button
		if button.has_meta("inspection_id") and button.get_meta("inspection_id") == "battle.defense_die.goblin.pending": leaked_enemy_pending_die = true
	if player_rolled_die == null or enemy_rolled_die == null or leaked_enemy_pending_die: _fail("defense reveal did not show both secret results without an enemy roll control"); return
	if player_rolled_die.global_position.distance_to(pre_roll_die_position) > 1.0 or enemy_rolled_die.global_position.x <= player_rolled_die.global_position.x: _fail("defense dice moved during reveal instead of updating the fixed two-column mat"); return
	pending_die = null; player_rolled_die = null; enemy_rolled_die = null
	defense_roll.active_store.clear(); defense_roll.queue_free(); await process_frame
	var effect_roll_fake := FakeBattleAuthority.new()
	var effect_roll_result := _fixture()
	effect_roll_result.snapshot.segment = "ongoing_effects"; effect_roll_result.snapshot.stage = "status_roll_reaction"
	effect_roll_result.pending_input.blade.segment = "ongoing_effects"; effect_roll_result.pending_input.blade.stage = "status_roll_reaction"; effect_roll_result.pending_input.blade.allowed_commands = ["commit_interaction", "pass"]
	effect_roll_result.snapshot.actors.blade.statuses = [{"instance_id": "poison-blade", "definition_id": "poison", "stacks": 2}]
	effect_roll_result.snapshot.actors.goblin.statuses = [{"instance_id": "poison-goblin", "definition_id": "poison", "stacks": 1}]
	effect_roll_result.snapshot.effect_rolls = [{"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "die": {"index": 0, "die_id": "standard_d6", "face": 2}}, {"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "die": {"index": 1, "die_id": "standard_d6", "face": 6}}, {"actor_id": "goblin", "status_instance_id": "poison-goblin", "status_id": "poison", "die": {"index": 0, "die_id": "standard_d6", "face": 4}}]
	var effect_roll_hidden := _fixture()
	effect_roll_hidden.snapshot.segment = "ongoing_effects"; effect_roll_hidden.snapshot.stage = "status_roll"
	effect_roll_hidden.pending_input.blade.segment = "ongoing_effects"; effect_roll_hidden.pending_input.blade.stage = "status_roll"; effect_roll_hidden.pending_input.blade.allowed_commands = ["roll_dice"]
	effect_roll_hidden.snapshot.actors.blade.statuses = effect_roll_result.snapshot.actors.blade.statuses.duplicate(true)
	effect_roll_hidden.snapshot.actors.goblin.statuses = effect_roll_result.snapshot.actors.goblin.statuses.duplicate(true)
	effect_roll_hidden.snapshot.effect_rolls = [{"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "resolved": true, "die": {"index": 0, "die_id": "standard_d6", "face": 0}}, {"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "die": {"index": 1, "die_id": "standard_d6", "face": 0}}]
	effect_roll_fake.enqueue(effect_roll_hidden); effect_roll_fake.enqueue(effect_roll_result)
	var effect_roll = packed.instantiate(); var effect_roll_fixture := _fixture()
	effect_roll_fixture.snapshot.segment = "ongoing_effects"; effect_roll_fixture.snapshot.stage = "status_roll"
	effect_roll_fixture.pending_input.blade.segment = "ongoing_effects"; effect_roll_fixture.pending_input.blade.stage = "status_roll"; effect_roll_fixture.pending_input.blade.allowed_commands = ["roll_dice"]
	effect_roll_fixture.snapshot.actors.blade.statuses = effect_roll_result.snapshot.actors.blade.statuses.duplicate(true)
	effect_roll_fixture.snapshot.actors.goblin.statuses = effect_roll_result.snapshot.actors.goblin.statuses.duplicate(true)
	effect_roll_fixture.snapshot.effect_rolls = [{"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "die": {"index": 0, "die_id": "standard_d6", "face": 0}}, {"actor_id": "blade", "status_instance_id": "poison-blade", "status_id": "poison", "die": {"index": 1, "die_id": "standard_d6", "face": 0}}]
	effect_roll.initial_result = effect_roll_fixture; effect_roll.gateway = BattleGateway.new(effect_roll_fake); effect_roll.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_effect_die_active.json")); root.add_child(effect_roll)
	await process_frame; await process_frame
	var pending_effect_dice: Array[Button] = []; var leaked_enemy_effect_die := false
	for button in effect_roll.find_children("*", "Button", true, false):
		if not button.has_meta("inspection_id"): continue
		var id := str(button.get_meta("inspection_id"))
		if id.begins_with("battle.effect_die.blade.pending."): pending_effect_dice.append(button)
		if id.begins_with("battle.effect_die.goblin"): leaked_enemy_effect_die = true
	if pending_effect_dice.size() != 2 or not pending_effect_dice[0].text.is_empty() or not pending_effect_dice[1].text.is_empty() or leaked_enemy_effect_die: _fail("pre-reveal effects UI did not expose only the player's blank Poison dice"); return
	var pre_effect_positions := [pending_effect_dice[0].global_position, pending_effect_dice[1].global_position]
	pending_effect_dice[0].pressed.emit(); await process_frame; await process_frame
	var first_effect_command: Dictionary = JSON.parse_string(effect_roll_fake.commands[0]) if effect_roll_fake.commands.size() == 1 else {}
	if first_effect_command.get("type") != "roll_dice" or _int_array(first_effect_command.get("payload", {}).get("reroll_indices", [])) != [0]: _fail("first blank effect die did not submit its own roll index: %s" % effect_roll_fake.commands); return
	var hidden_effect_die: Button = null; var second_pending_effect_die: Button = null
	for button in effect_roll.find_children("*", "Button", true, false):
		if not button.has_meta("inspection_id"): continue
		if button.get_meta("inspection_id") == "battle.effect_die.blade.hidden.0": hidden_effect_die = button
		if button.get_meta("inspection_id") == "battle.effect_die.blade.pending.1": second_pending_effect_die = button
	if hidden_effect_die == null or second_pending_effect_die == null or "HIDDEN" not in hidden_effect_die.text or hidden_effect_die.global_position.distance_to(pre_effect_positions[0]) > 1.0 or second_pending_effect_die.global_position.distance_to(pre_effect_positions[1]) > 1.0: _fail("first Poison die did not remain hidden and fixed while the second awaited its click"); return
	second_pending_effect_die.pressed.emit(); await process_frame; await process_frame
	var second_effect_command: Dictionary = JSON.parse_string(effect_roll_fake.commands[1]) if effect_roll_fake.commands.size() == 2 else {}
	if second_effect_command.get("type") != "roll_dice" or _int_array(second_effect_command.get("payload", {}).get("reroll_indices", [])) != [1]: _fail("second blank effect die did not submit its own roll index: %s" % effect_roll_fake.commands); return
	var revealed_player_effect_dice: Array[Button] = []; var revealed_enemy_effect_die: Button = null; var saw_damage_outcome := false; var saw_removal_outcome := false
	for button in effect_roll.find_children("*", "Button", true, false):
		if not button.has_meta("inspection_id"): continue
		var id := str(button.get_meta("inspection_id"))
		if id.begins_with("battle.effect_die.blade.") and not id.contains("pending"): revealed_player_effect_dice.append(button)
		if id == "battle.effect_die.goblin.0": revealed_enemy_effect_die = button
	for label in effect_roll.find_children("*", "Label", true, false):
		if label.text == "Poison · 2\n1 damage pending": saw_damage_outcome = true
		if label.text == "Poison · 6\nRemove 1 Poison stack": saw_removal_outcome = true
	if revealed_player_effect_dice.size() != 2 or revealed_enemy_effect_die == null or not saw_damage_outcome or not saw_removal_outcome: _fail("effects reveal omitted player/enemy dice or Poison outcome text"); return
	for index in 2:
		if revealed_player_effect_dice[index].global_position.distance_to(pre_effect_positions[index]) > 1.0: _fail("player effect dice moved during reveal instead of filling in place"); return
	if revealed_enemy_effect_die.global_position.x <= revealed_player_effect_dice[0].global_position.x: _fail("enemy effect result did not populate the reserved right column"); return
	pending_effect_dice.clear(); revealed_player_effect_dice.clear(); revealed_enemy_effect_die = null
	effect_roll.active_store.clear(); effect_roll.queue_free(); await process_frame
	var defense_reveal = packed.instantiate(); var defense_fixture := _fixture()
	defense_fixture.snapshot.segment = "defensive"; defense_fixture.snapshot.stage = "defense_reaction"
	defense_fixture.pending_input.blade.segment = "defensive"; defense_fixture.pending_input.blade.stage = "defense_reaction"; defense_fixture.pending_input.blade.allowed_commands = ["pass"]
	defense_fixture.snapshot.damage_sources = [{"id": "source-player", "source_content_id": "golden_edge", "target_actor_id": "goblin", "base_amount": 5, "final_amount": 0}, {"id": "source-enemy", "source_content_id": "jagged_slash", "target_actor_id": "blade", "base_amount": 4, "final_amount": 0}]
	defense_fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "source-enemy", "rolled_face": 6}, "goblin": {"actor_id": "goblin", "ability_id": "protect", "source_id": "source-player"}}
	defense_fixture.events = [{"sequence": 1, "type": "dice_rolled", "actor_id": "blade", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 6}]}]
	defense_reveal.initial_result = defense_fixture; root.add_child(defense_reveal)
	await process_frame; await process_frame
	var saw_player_math := false; var saw_enemy_protect := false; var saw_enemy_math := false
	for label in defense_reveal.find_children("*", "Label", true, false):
		if "Jagged Slash: 4 − 6 = 0 pending" in label.text: saw_player_math = true
		if label.text == "Protect": saw_enemy_protect = true
		if "Golden Edge: 5 → 2 pending" in label.text: saw_enemy_math = true
	var die_ids: Array[String] = []
	for button in defense_reveal.find_children("*", "Button", true, false):
		if button.has_meta("inspection_id") and str(button.get_meta("inspection_id")).begins_with("battle.defense_die."): die_ids.append(str(button.get_meta("inspection_id")))
		if button.text == "Continue Presentation": _fail("defense dice still created a presentation popup"); return
	if not saw_player_math or not saw_enemy_protect or not saw_enemy_math: _fail("combined defense mat omitted the player roll or enemy Protect result"); return
	if die_ids != ["battle.defense_die.blade"]: _fail("non-rolling Protect created a die or player defense die was missing: %s" % die_ids); return
	defense_reveal.queue_free(); await process_frame
	var no_defense = packed.instantiate(); var no_defense_fixture: Dictionary = defense_fixture.duplicate(true)
	no_defense_fixture.snapshot.defense_selections.erase("goblin")
	no_defense.initial_result = no_defense_fixture; root.add_child(no_defense)
	await process_frame; await process_frame
	var saw_no_enemy_defense := false; var saw_unchanged_attack := false
	for label in no_defense.find_children("*", "Label", true, false):
		if label.text == "NO DEFENSE USED": saw_no_enemy_defense = true
		if label.text == "Golden Edge: 5 → 5 pending": saw_unchanged_attack = true
	if not saw_no_enemy_defense or not saw_unchanged_attack: _fail("defense reveal did not explicitly show an undefended player attack"); return
	no_defense.queue_free(); await process_frame
	var snapshot_fake := FakeBattleAuthority.new()
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": [{"name": "existing-checkpoint", "round": 1, "segment": "defensive", "stage": "defense_reaction", "event_count": 12}]}})
	snapshot_fake.enqueue({"accepted": true, "data": {"point": {"presented_sequence": 0}, "timeline": {"branch": {}, "points": []}}})
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshot": {"name": "round-2-effects"}}})
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": [{"name": "round-2-effects", "round": 2, "segment": "ongoing_effects", "stage": "status_damage_reaction", "event_count": 42}]}})
	var loaded_fixture := _fixture(); loaded_fixture.snapshot.battle_id = "battle-snapshot-copy"; loaded_fixture.data = {"loaded_snapshot": {"name": "round-2-effects", "event_count": 42}}
	snapshot_fake.enqueue(loaded_fixture)
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": [{"name": "round-2-effects", "round": 2, "segment": "ongoing_effects", "stage": "status_damage_reaction", "event_count": 42}]}})
	var restarted_fixture := _fixture(); restarted_fixture.snapshot.battle_id = "battle-snapshot-copy-two"; restarted_fixture.data = {"loaded_snapshot": {"name": "round-2-effects", "event_count": 42}}
	snapshot_fake.enqueue(restarted_fixture)
	var snapshot_store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_dev_snapshot_active.json")); snapshot_store.clear()
	var snapshot_capture_fixture := _fixture()
	snapshot_capture_fixture.events = [{"sequence": 1, "type": "segment_entered", "to": "ongoing_effects"}]
	var snapshot_screen = packed.instantiate(); snapshot_screen.initial_result = snapshot_capture_fixture; snapshot_screen.gateway = BattleGateway.new(snapshot_fake); snapshot_screen.active_store = snapshot_store; root.add_child(snapshot_screen)
	await process_frame; await process_frame
	var toggle := _button(snapshot_screen, "DEV SNAPSHOTS")
	if toggle == null: _fail("debug build did not expose the gated developer snapshot controls"); return
	toggle.pressed.emit(); await process_frame; await process_frame
	var snapshot_panel: PanelContainer = null
	for control in snapshot_screen.find_children("*", "PanelContainer", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.dev_snapshots.panel": snapshot_panel = control
	if snapshot_panel == null: _fail("snapshot dialog did not expose its styled panel surface"); return
	var snapshot_panel_style := snapshot_panel.get_theme_stylebox("panel")
	if not snapshot_panel_style is StyleBoxFlat or snapshot_panel_style.bg_color.a < 0.99: _fail("snapshot dialog surface is transparent and allows battle text to bleed through"); return
	var initial_load := _button(snapshot_screen, "Load Selected as New Battle")
	var snapshot_list: ItemList = null
	for control in snapshot_screen.find_children("*", "ItemList", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.dev_snapshots.list": snapshot_list = control
	if snapshot_list == null or initial_load == null or not initial_load.disabled: _fail("snapshot list did not begin with load disabled before a selection"); return
	snapshot_list.select(0); snapshot_list.item_selected.emit(0); await process_frame
	if initial_load.disabled: _fail("first snapshot selection did not immediately enable loading"); return
	var name_edit: LineEdit = null
	for control in snapshot_screen.find_children("*", "LineEdit", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.dev_snapshots.name": name_edit = control
	if name_edit == null: _fail("snapshot panel did not expose a named capture field"); return
	name_edit.text = "round-2-effects"; name_edit.text_changed.emit(name_edit.text); await process_frame
	var capture := _button(snapshot_screen, "Capture Current Authority State")
	if capture == null or capture.disabled: _fail("valid snapshot name did not enable capture during an active presentation"); return
	capture.pressed.emit(); await process_frame; await process_frame
	var load_snapshot := _button(snapshot_screen, "Load Selected as New Battle")
	if load_snapshot == null or load_snapshot.disabled: _fail("captured snapshot was not selected for loading"); return
	load_snapshot.pressed.emit(); await process_frame; await process_frame
	if snapshot_fake.commands.size() != 5: _fail("snapshot UI submitted unexpected command sequence: %s" % snapshot_fake.commands); return
	var command_types: Array = []
	for command_json in snapshot_fake.commands: command_types.append(JSON.parse_string(command_json).get("type"))
	if command_types != ["list_dev_snapshots", "mark_dev_history", "save_dev_snapshot", "list_dev_snapshots", "load_dev_snapshot"]: _fail("snapshot UI did not checkpoint the exact visible state before saving: %s" % command_types); return
	var snapshot_checkpoint: Dictionary = JSON.parse_string(snapshot_fake.commands[1])
	if snapshot_checkpoint.get("payload", {}).get("kind") != "presentation" or int(snapshot_checkpoint.get("payload", {}).get("presented_sequence", -1)) != 0: _fail("snapshot capture did not preserve the unacknowledged presentation cursor: %s" % snapshot_checkpoint); return
	var active_snapshot := snapshot_store.load_active()
	if active_snapshot.get("battle_id") != "battle-snapshot-copy" or active_snapshot.get("snapshot_name") != "round-2-effects" or int(active_snapshot.get("last_sequence", 0)) != 42: _fail("loaded snapshot was not installed as the active battle: %s" % active_snapshot); return
	var loaded_screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var loaded_screen: Node = loaded_screens[-1]
	if loaded_screen.inspection_state().get("loaded_snapshot_name") != "round-2-effects" or _button(loaded_screen, "DEV SNAPSHOTS") == null: _fail("loaded battle lost restart provenance or developer controls"); return
	_button(loaded_screen, "DEV SNAPSHOTS").pressed.emit(); await process_frame; await process_frame
	var restart_snapshot := _button(loaded_screen, "Restart Loaded Snapshot")
	if restart_snapshot == null or restart_snapshot.disabled: _fail("loaded battle did not enable one-click snapshot restart"); return
	restart_snapshot.pressed.emit(); await process_frame; await process_frame
	if snapshot_fake.commands.size() != 7 or JSON.parse_string(snapshot_fake.commands[6]).get("type") != "load_dev_snapshot": _fail("snapshot restart did not issue a fresh load: %s" % snapshot_fake.commands); return
	active_snapshot = snapshot_store.load_active()
	if active_snapshot.get("battle_id") != "battle-snapshot-copy-two" or active_snapshot.get("snapshot_name") != "round-2-effects": _fail("snapshot restart did not install the second independent battle: %s" % active_snapshot); return
	loaded_screens = get_nodes_in_group("inspectable_battle_screen"); loaded_screen = loaded_screens[-1]
	snapshot_store.clear(); loaded_screen.queue_free(); await process_frame
	OS.set_environment("DICE_AND_DESTINY_ENABLE_HISTORY", "1")
	var history_id := "history-0000000000000001"
	var later_history_id := "history-0000000000000002"
	var history_point := {"id": history_id, "label": "Roll 5 Dice", "kind": "decision", "action_type": "command", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 1, "presented_sequence": 0}
	var later_history_point := {"id": later_history_id, "parent_point_id": history_id, "label": "Pass Planning", "kind": "decision", "action_type": "command", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 2, "presented_sequence": 0}
	var current_history_id := "history-0000000000000009"
	var current_history_point := {"id": current_history_id, "parent_point_id": later_history_id, "label": "Current: Offensive · Planning", "kind": "decision", "action_type": "", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 2, "presented_sequence": 0}
	var base_timeline := {"branch": {"battle_id": "fixture-scene", "root_battle_id": "fixture-scene", "head_point_id": later_history_id, "status": "active"}, "points": [history_point, later_history_point]}
	var active_timeline := {"branch": {"battle_id": "fixture-scene", "root_battle_id": "fixture-scene", "head_point_id": current_history_id, "status": "active"}, "points": [history_point, later_history_point, current_history_point]}
	var review_branch := {"battle_id": "battle-history-review", "root_battle_id": "fixture-scene", "head_point_id": current_history_id, "cursor_point_id": history_id, "parent_battle_id": "fixture-scene", "latest_battle_id": "fixture-scene", "base_point_id": history_id, "status": "review"}
	var review_timeline := {"branch": review_branch, "points": [history_point, later_history_point, current_history_point]}
	var replay_branch: Dictionary = review_branch.merged({"status": "replay"}, true)
	var replay_timeline := {"branch": replay_branch, "points": [history_point, later_history_point, current_history_point]}
	var history_fake := FakeBattleAuthority.new()
	history_fake.enqueue({"accepted": true, "data": {"timeline": base_timeline}})
	history_fake.enqueue({"accepted": true, "data": {"point": current_history_point, "timeline": active_timeline}})
	var review_fixture := _fixture(); review_fixture.snapshot.battle_id = "battle-history-review"; review_fixture.data = {"history": {"review": true, "point": history_point, "branch": review_branch, "origin_battle_id": "fixture-scene", "presented_sequence": 0, "client_state": {"selected_indices": [1, 3]}}}
	history_fake.enqueue(review_fixture)
	history_fake.enqueue({"accepted": true, "data": {"timeline": review_timeline}})
	var resumed_fixture := _fixture(); resumed_fixture.snapshot.battle_id = "battle-history-review"; resumed_fixture.data = {"history": {"review": false, "replay": true, "branch": replay_branch, "timeline": replay_timeline, "point": history_point, "mode": "preserve", "presented_sequence": 0, "client_state": {"selected_indices": [1, 3]}}}
	history_fake.enqueue(resumed_fixture)
	history_fake.enqueue({"accepted": true, "data": {"timeline": replay_timeline}})
	var advanced_branch: Dictionary = replay_branch.merged({"cursor_point_id": later_history_id}, true); var advanced_timeline := {"branch": advanced_branch, "points": [history_point, later_history_point, current_history_point]}
	var advanced_fixture := _fixture(); advanced_fixture.snapshot.battle_id = "battle-history-review"; advanced_fixture.data = {"history": {"review": false, "replay": true, "branch": advanced_branch, "timeline": advanced_timeline, "point": later_history_point, "presented_sequence": 0, "client_state": {}}}
	history_fake.enqueue(advanced_fixture)
	history_fake.enqueue({"accepted": true, "data": {"timeline": advanced_timeline}})
	var divergence_result := {"accepted": false, "error": "recorded history action differs", "data": {"history": {"divergence": true, "expected_label": "Pass Planning", "future_point_count": 2, "point": later_history_point}}}
	history_fake.enqueue(divergence_result)
	history_fake.enqueue(divergence_result)
	var truncated_branch := advanced_branch.merged({"status": "active", "head_point_id": history_id, "cursor_point_id": "", "discarded_head_point_id": later_history_id}, true); var truncated_timeline := {"branch": truncated_branch, "points": [history_point]}
	history_fake.enqueue({"accepted": true, "data": {"history": {"review": false, "replay": false, "branch": truncated_branch, "timeline": truncated_timeline}}})
	var replacement_point := {"id": "history-0000000000000003", "parent_point_id": history_id, "label": "Roll 5 Dice", "kind": "decision", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 2, "presented_sequence": 0}
	var replacement_timeline := {"branch": truncated_branch.merged({"head_point_id": replacement_point.id}, true), "points": [history_point, replacement_point]}
	history_fake.enqueue({"accepted": true, "data": {"point": replacement_point, "timeline": replacement_timeline}})
	var replacement_result := _rolled_fixture("history-replaced", 1, []); replacement_result.snapshot.battle_id = "battle-history-review"; history_fake.enqueue(replacement_result)
	var endpoint_point := {"id": "history-0000000000000004", "parent_point_id": replacement_point.id, "label": "Offensive Dice 1/3", "kind": "decision", "action_type": "", "round": 1, "segment": "offensive", "stage": "planning", "event_count": 3, "presented_sequence": 0}
	var endpoint_timeline := {"branch": truncated_branch.merged({"head_point_id": endpoint_point.id}, true), "points": [history_point, replacement_point, endpoint_point]}
	history_fake.enqueue({"accepted": true, "data": {"point": endpoint_point, "timeline": endpoint_timeline}})
	var history_store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_dev_history_active.json")); history_store.clear()
	var history_screen = packed.instantiate(); history_screen.initial_result = _fixture(); history_screen.gateway = BattleGateway.new(history_fake); history_screen.active_store = history_store; root.add_child(history_screen)
	await process_frame; await process_frame
	var point_button: Button = null
	for control in history_screen.find_children("*", "Button", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.history.point.%s" % history_id: point_button = control
	if point_button == null: _fail("history bar did not expose the recorded decision point"); return
	point_button.pressed.emit(); await process_frame; await process_frame
	var history_screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen"); var review_screen: Node = history_screens[-1]
	var review_state: Dictionary = review_screen.inspection_state()
	if not review_state.get("history_review", false) or review_state.get("history_point_id") != history_id or _int_array(review_state.get("selected_dice", [])) != [1, 3]: _fail("history jump did not restore review context and local selections: %s" % review_state); return
	var roll_in_review := _button(review_screen, "Roll 5 Dice")
	var preserve_history := _button(review_screen, "Resume Here · Keep Existing Future")
	if roll_in_review == null or not roll_in_review.disabled or preserve_history == null or preserve_history.disabled: _fail("history review was not read-only or branch controls were unavailable"); return
	preserve_history.pressed.emit(); await process_frame; await process_frame
	history_screens = get_nodes_in_group("inspectable_battle_screen"); var resumed_screen: Node = history_screens[-1]
	var resumed_state: Dictionary = resumed_screen.inspection_state()
	if resumed_state.get("history_review", true) or not resumed_state.get("history_replay", false) or resumed_state.get("history_points", []).size() != 3 or _button(resumed_screen, "Roll 5 Dice").disabled: _fail("preserved history branch dropped its forward timeline or did not enter replay: %s" % resumed_state); return
	_button(resumed_screen, "Roll 5 Dice").pressed.emit(); await process_frame; await process_frame
	history_screens = get_nodes_in_group("inspectable_battle_screen"); var advanced_screen: Node = history_screens[-1]
	var advanced_state: Dictionary = advanced_screen.inspection_state()
	if not advanced_state.get("history_replay", false) or advanced_state.get("history_point_id") != later_history_id or advanced_state.get("history_points", []).size() != 3: _fail("matching recorded action did not move the cursor forward while retaining history: %s" % advanced_state); return
	_button(advanced_screen, "Roll 5 Dice").pressed.emit(); await process_frame; await process_frame
	if advanced_screen.inspection_state().get("history_divergence_pending", {}).is_empty() or _button(advanced_screen, "Cancel · Keep Existing Future") == null: _fail("changed replay action did not prompt before replacing the future"); return
	_button(advanced_screen, "Cancel · Keep Existing Future").pressed.emit(); await process_frame
	if not advanced_screen.inspection_state().get("history_divergence_pending", {}).is_empty() or advanced_screen.inspection_state().get("history_points", []).size() != 3: _fail("canceling divergence did not keep the complete future"); return
	_button(advanced_screen, "Roll 5 Dice").pressed.emit(); await process_frame; await process_frame
	var confirm_divergence := _button(advanced_screen, "Replace Future and Continue")
	if confirm_divergence == null: _fail("second changed action did not restore the divergence confirmation"); return
	confirm_divergence.pressed.emit(); await process_frame; await process_frame
	var replaced_state: Dictionary = advanced_screen.inspection_state()
	var retained_ids: Array = []
	for point_value in replaced_state.get("history_points", []): retained_ids.append(str(point_value.get("id", "")))
	if replaced_state.get("history_replay", true) or later_history_id in retained_ids or retained_ids != [history_id, replacement_point.id, endpoint_point.id]: _fail("confirmed divergence did not replace current/future history with the new action and resulting state: %s" % replaced_state); return
	var history_command_types: Array = []
	for command_json in history_fake.commands: history_command_types.append(JSON.parse_string(command_json).get("type"))
	if history_command_types != ["list_dev_history", "mark_dev_history", "jump_dev_history", "list_dev_history", "commit_dev_history", "list_dev_history", "replay_dev_history_action", "list_dev_history", "replay_dev_history_action", "replay_dev_history_action", "replace_dev_history_future", "mark_dev_history", "planning_roll", "mark_dev_history"]: _fail("history UI command order was wrong: %s" % [history_command_types]); return
	var active_history := history_store.load_active()
	if active_history.get("battle_id") != "battle-history-review" or active_history.has("history_context"): _fail("diverged branch retained stale review/replay context: %s" % active_history); return
	history_store.clear(); advanced_screen.queue_free(); await process_frame
	print("BATTLE SCENE: reusable components and 1920x1080 / 1280x720 layouts passed"); quit(0)

func _fixture() -> Dictionary:
	return {"accepted": true, "events": [], "pending_input": {"blade": {"id": "p", "window_id": "w", "segment": "offensive", "stage": "planning", "iteration": 1, "planning_cycle": 1, "allowed_commands": ["planning_roll", "planning_pass"]}}, "snapshot": {"battle_id": "fixture-scene", "status": "active", "viewer_actor_id": "blade", "round": 1, "segment": "offensive", "stage": "planning", "actors": {"blade": {"definition_id": "blade_warden", "hand": [], "card_instances": {}, "current_health": 20, "max_health": 20, "deck_count": 16, "hand_count": 4, "offensive_abilities": ["sword_cut", "shield_bash", "golden_edge", "perfect_form"], "defensive_abilities": ["basic_defense", "protect"]}, "goblin": {"definition_id": "venom_goblin", "current_health": 12, "max_health": 12, "deck_count": 10, "hand_count": 2, "offensive_abilities": ["jagged_slash", "venom_strike", "crushing_advance", "greedy_blow"], "defensive_abilities": ["basic_defense", "protect"]}}}}

func _rolled_fixture(pending_id: String, rolls: int, kept: Array) -> Dictionary:
	var result := _fixture()
	result.pending_input.blade.id = pending_id
	result.pending_input.blade.allowed_commands = ["planning_keep", "planning_reroll", "planning_pass"]
	var history: Array = []
	for number in rolls:
		var faces := [3, 5, 1, 6, 3] if number == 0 else [3, 2, 1, 4, 5]
		var dice: Array = []
		for index in faces.size(): dice.append({"index": index, "face": faces[index], "number": faces[index]})
		history.append({"number": number + 1, "dice": dice, "kept_indices": kept.duplicate() if number == rolls - 1 else []})
	result.snapshot.actors.blade.roll_history = history
	return result

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _has_button(node: Node, text: String) -> bool:
	return _button(node, text) != null

func _int_array(values: Array) -> Array:
	var result: Array = []
	for value in values: result.append(int(value))
	return result

func _fail(message: String) -> void: push_error(message); quit(1)
