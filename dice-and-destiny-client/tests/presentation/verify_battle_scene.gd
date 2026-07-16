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
	reroll_screen.active_store = ActiveBattleStore.new("user://verify_toggle_keep_active.json")
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
	var defense_reveal = packed.instantiate(); var defense_fixture := _fixture()
	defense_fixture.snapshot.segment = "defensive"; defense_fixture.snapshot.stage = "defense_reaction"
	defense_fixture.pending_input.blade.segment = "defensive"; defense_fixture.pending_input.blade.stage = "defense_reaction"; defense_fixture.pending_input.blade.allowed_commands = ["pass"]
	defense_fixture.snapshot.damage_sources = [{"id": "source-player", "source_content_id": "golden_edge", "target_actor_id": "goblin", "base_amount": 5, "final_amount": 0}, {"id": "source-enemy", "source_content_id": "jagged_slash", "target_actor_id": "blade", "base_amount": 4, "final_amount": 0}]
	defense_fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "source-enemy", "rolled_face": 6}, "goblin": {"actor_id": "goblin", "ability_id": "protect", "source_id": "source-player"}}
	defense_fixture.events = [{"sequence": 1, "type": "dice_rolled", "actor_id": "blade", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 6}]}]
	defense_reveal.initial_result = defense_fixture; root.add_child(defense_reveal)
	await process_frame; await process_frame
	var saw_reveal := false; var saw_player_math := false; var saw_enemy_protect := false; var saw_enemy_math := false
	for label in defense_reveal.find_children("*", "Label", true, false):
		if label.text == "DEFENSE REVEAL": saw_reveal = true
		if "Jagged Slash: 4 − 6 = 0 pending" in label.text: saw_player_math = true
		if label.text == "Protect": saw_enemy_protect = true
		if "Golden Edge: 5 → 2 pending" in label.text: saw_enemy_math = true
	var die_ids: Array[String] = []
	for button in defense_reveal.find_children("*", "Button", true, false):
		if button.has_meta("inspection_id") and str(button.get_meta("inspection_id")).begins_with("battle.defense_die."): die_ids.append(str(button.get_meta("inspection_id")))
		if button.text == "Continue Presentation": _fail("defense dice still created a presentation popup"); return
	if not saw_reveal or not saw_player_math or not saw_enemy_protect or not saw_enemy_math: _fail("combined defense mat omitted the player roll or enemy Protect result"); return
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
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": []}})
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshot": {"name": "round-2-effects"}}})
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": [{"name": "round-2-effects", "round": 2, "segment": "ongoing_effects", "stage": "status_damage_reaction", "event_count": 42}]}})
	var loaded_fixture := _fixture(); loaded_fixture.snapshot.battle_id = "battle-snapshot-copy"; loaded_fixture.data = {"loaded_snapshot": {"name": "round-2-effects", "event_count": 42}}
	snapshot_fake.enqueue(loaded_fixture)
	snapshot_fake.enqueue({"accepted": true, "data": {"snapshots": [{"name": "round-2-effects", "round": 2, "segment": "ongoing_effects", "stage": "status_damage_reaction", "event_count": 42}]}})
	var restarted_fixture := _fixture(); restarted_fixture.snapshot.battle_id = "battle-snapshot-copy-two"; restarted_fixture.data = {"loaded_snapshot": {"name": "round-2-effects", "event_count": 42}}
	snapshot_fake.enqueue(restarted_fixture)
	var snapshot_store := ActiveBattleStore.new("user://verify_dev_snapshot_active.json"); snapshot_store.clear()
	var snapshot_screen = packed.instantiate(); snapshot_screen.initial_result = _fixture(); snapshot_screen.gateway = BattleGateway.new(snapshot_fake); snapshot_screen.active_store = snapshot_store; root.add_child(snapshot_screen)
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
	var name_edit: LineEdit = null
	for control in snapshot_screen.find_children("*", "LineEdit", true, false):
		if control.has_meta("inspection_id") and control.get_meta("inspection_id") == "battle.dev_snapshots.name": name_edit = control
	if name_edit == null: _fail("snapshot panel did not expose a named capture field"); return
	name_edit.text = "round-2-effects"; name_edit.text_changed.emit(name_edit.text); await process_frame
	var capture := _button(snapshot_screen, "Capture Current Authority State")
	if capture == null or capture.disabled: _fail("valid snapshot name did not enable capture"); return
	capture.pressed.emit(); await process_frame; await process_frame
	var load_snapshot := _button(snapshot_screen, "Load Selected as New Battle")
	if load_snapshot == null or load_snapshot.disabled: _fail("captured snapshot was not selected for loading"); return
	load_snapshot.pressed.emit(); await process_frame; await process_frame
	if snapshot_fake.commands.size() != 4: _fail("snapshot UI submitted unexpected command sequence: %s" % snapshot_fake.commands); return
	var command_types: Array = []
	for command_json in snapshot_fake.commands: command_types.append(JSON.parse_string(command_json).get("type"))
	if command_types != ["list_dev_snapshots", "save_dev_snapshot", "list_dev_snapshots", "load_dev_snapshot"]: _fail("snapshot UI command order was wrong: %s" % command_types); return
	var active_snapshot := snapshot_store.load_active()
	if active_snapshot.get("battle_id") != "battle-snapshot-copy" or active_snapshot.get("snapshot_name") != "round-2-effects" or int(active_snapshot.get("last_sequence", 0)) != 42: _fail("loaded snapshot was not installed as the active battle: %s" % active_snapshot); return
	var loaded_screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var loaded_screen: Node = loaded_screens[-1]
	if loaded_screen.inspection_state().get("loaded_snapshot_name") != "round-2-effects" or _button(loaded_screen, "DEV SNAPSHOTS") == null: _fail("loaded battle lost restart provenance or developer controls"); return
	_button(loaded_screen, "DEV SNAPSHOTS").pressed.emit(); await process_frame; await process_frame
	var restart_snapshot := _button(loaded_screen, "Restart Loaded Snapshot")
	if restart_snapshot == null or restart_snapshot.disabled: _fail("loaded battle did not enable one-click snapshot restart"); return
	restart_snapshot.pressed.emit(); await process_frame; await process_frame
	if snapshot_fake.commands.size() != 6 or JSON.parse_string(snapshot_fake.commands[5]).get("type") != "load_dev_snapshot": _fail("snapshot restart did not issue a fresh load: %s" % snapshot_fake.commands); return
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
	var history_store := ActiveBattleStore.new("user://verify_dev_history_active.json"); history_store.clear()
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
