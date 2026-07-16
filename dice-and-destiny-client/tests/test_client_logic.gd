extends SceneTree

var failures: Array[String] = []
const PENDING := {"id": "pending-7", "window_id": "window-4", "segment": "offensive", "stage": "planning", "iteration": 3, "reaction_round": 2, "planning_cycle": 5, "source_id": "roll-request-9", "allowed_commands": []}

func _init() -> void:
	_test_builders()
	_test_active_store()
	_test_view_states()
	_test_director_and_fake()
	if failures.is_empty(): print("CLIENT LOGIC TESTS: command, state, and presentation assertions passed"); quit(0)
	else:
		for failure in failures: push_error(failure)
		quit(1)

func _test_builders() -> void:
	_expect(BattleCommandBuilder.start_battle("b1"), "start_battle", {"player": {"instance_id": "blade", "definition_id": "blade_warden"}, "enemies": [{"instance_id": "goblin", "definition_id": "venom_goblin"}]})
	_expect(BattleCommandBuilder.open_battle("b1"), "open_battle", {})
	_expect(BattleCommandBuilder.list_dev_snapshots("b1"), "list_dev_snapshots", {})
	_expect(BattleCommandBuilder.save_dev_snapshot("b1", "blade", "round-2-effects", true), "save_dev_snapshot", {"name": "round-2-effects", "overwrite": true})
	_expect(BattleCommandBuilder.load_dev_snapshot("b1", "blade", "round-2-effects"), "load_dev_snapshot", {"name": "round-2-effects"})
	_expect(BattleCommandBuilder.list_dev_history("b1"), "list_dev_history", {})
	_expect(BattleCommandBuilder.mark_dev_history("b1", "blade", "Roll 5 Dice", "decision", 12, {"selected_indices": [1, 3]}, {"type": "command", "command_type": "planning_roll"}), "mark_dev_history", {"label": "Roll 5 Dice", "kind": "decision", "presented_sequence": 12, "client_state": {"selected_indices": [1, 3]}, "action": {"type": "command", "command_type": "planning_roll"}})
	_expect(BattleCommandBuilder.jump_dev_history("b1", "blade", "history-0000000000000001"), "jump_dev_history", {"point_id": "history-0000000000000001"})
	_expect(BattleCommandBuilder.commit_dev_history("b1", "blade", "preserve"), "commit_dev_history", {"mode": "preserve"})
	_expect(BattleCommandBuilder.return_dev_history_latest("b1", "blade"), "return_dev_history_latest", {})
	_expect(BattleCommandBuilder.replay_dev_history_action("b1", "blade", {"type": "presentation_continue", "watermark": 4}), "replay_dev_history_action", {"action": {"type": "presentation_continue", "watermark": 4}})
	_expect(BattleCommandBuilder.replace_dev_history_future("b1", "blade"), "replace_dev_history_future", {})
	_expect(BattleCommandBuilder.planning_roll("b1", "blade", PENDING), "planning_roll", _plan_payload())
	_expect(BattleCommandBuilder.planning_keep("b1", "blade", PENDING, [0, 2]), "planning_keep", _plan_payload({"kept_indices": [0, 2]}))
	_expect(BattleCommandBuilder.planning_reroll("b1", "blade", PENDING, [1, 3, 4]), "planning_reroll", _plan_payload({"reroll_indices": [1, 3, 4]}))
	var response_keep := BattleCommandBuilder.planning_keep("b1", "blade", PENDING, [0.0, 2.0])
	if '"kept_indices":[0,2]' not in response_keep or '"kept_indices":[0.0,2.0]' in response_keep: failures.append("server-originated float keep indices were not encoded as integers: %s" % response_keep)
	var response_reroll := BattleCommandBuilder.planning_reroll("b1", "blade", PENDING, [1.0, 3.0, 4.0])
	if '"reroll_indices":[1,3,4]' not in response_reroll or '"reroll_indices":[1.0,3.0,4.0]' in response_reroll: failures.append("server-originated float reroll indices were not encoded as integers: %s" % response_reroll)
	_expect(BattleCommandBuilder.planning_commit_cards("b1", "blade", PENDING, ["card-1"], ["blade"], "sword_cut", "poison", 3), "planning_commit_cards", _plan_payload({"card_ids": ["card-1"], "target_ids": ["blade"], "ability_id": "sword_cut", "status_id": "poison", "die_index": 3}))
	_expect(BattleCommandBuilder.planning_select_ability("b1", "blade", PENDING, "sword_cut", ["goblin"]), "planning_select_ability", _plan_payload({"ability_id": "sword_cut", "target_ids": ["goblin"]}))
	_expect(BattleCommandBuilder.planning_select_targets("b1", "blade", PENDING, ["goblin"]), "planning_select_targets", _plan_payload({"target_ids": ["goblin"]}))
	_expect(BattleCommandBuilder.planning_pass("b1", "blade", PENDING), "planning_pass", _plan_payload())
	_expect(BattleCommandBuilder.roll_dice("b1", "blade", PENDING), "roll_dice", {"pending_input_id": "pending-7", "request_id": "roll-request-9"})
	var commitment := {"pending_input_id": "pending-7", "checkpoint": _interaction_checkpoint(), "commitment": {"card_ids": ["card-1"], "proposal_ids": ["source-1"], "target_ids": ["goblin"], "choice_id": "poison", "planning_adjustments": [{"type": "set_die_face", "actor_id": "goblin", "die_index": 4, "face": 5}], "damage_reactions": [{"type": "prevent", "proposal_id": "source-1", "amount": 3}], "value": 6}}
	_expect(BattleCommandBuilder.commit_interaction("b1", "blade", PENDING, ["card-1"], ["source-1"], ["goblin"], "poison", commitment.commitment.planning_adjustments, commitment.commitment.damage_reactions, 6), "commit_interaction", commitment)
	_expect(BattleCommandBuilder.pass_command("b1", "blade", PENDING), "pass", {"pending_input_id": "pending-7", "checkpoint": _interaction_checkpoint()})

func _test_active_store() -> void:
	var path := "user://verify_snapshot_active_store.json"
	var store := ActiveBattleStore.new(path); store.clear()
	var history_context := {"review": true, "point_id": "history-0000000000000001", "client_state": {"selected_indices": [1, 3]}}
	if store.save_active("battle-copy", "blade", 42, "round-2-effects", history_context) != OK: failures.append("active snapshot battle pointer could not be saved"); return
	var restored := store.load_active()
	var normalized_context = JSON.parse_string(JSON.stringify(history_context))
	if restored.get("battle_id") != "battle-copy" or int(restored.get("last_sequence", 0)) != 42 or restored.get("snapshot_name") != "round-2-effects" or restored.get("history_context") != normalized_context: failures.append("active snapshot/history provenance did not round-trip: %s" % restored)
	store.clear()

func _test_view_states() -> void:
	var stages := ["planning", "planning", "planning", "planning", "offensive_reaction", "defense_selection", "defense_roll", "defense_reaction", "status_roll_reaction", "income", "damage_reaction", "damage_reaction", "discard_to_hand_limit", "victory"]
	for stage in stages:
		var view := BattleViewState.new(); var fixture := _fixture(stage)
		if not view.apply_result(fixture): failures.append("view rejected fixture %s" % stage); continue
		if not view.actor("goblin").get("hand", []).is_empty(): failures.append("enemy hand visible in %s" % stage)
		if view.hand_cards().size() != 1: failures.append("viewer hand unresolved in %s" % stage)
	var unsafe := _fixture("planning"); unsafe.snapshot.actors.goblin["hand"] = ["secret"]
	if BattleViewState.new().apply_result(unsafe): failures.append("unsafe enemy hand was accepted")
	var defense_view := BattleViewState.new(); var defense_fixture := _fixture("defense_reaction")
	defense_fixture.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": "basic_defense", "source_id": "enemy-source", "rolled_face": 6}, "goblin": {"actor_id": "goblin", "ability_id": "protect", "source_id": "player-source"}}
	defense_fixture.events = [{"type": "dice_rolled", "actor_id": "blade", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 6}]}]
	if not defense_view.apply_result(defense_fixture) or defense_view.defense_rolls.get("blade", {}).get("face") != 6 or defense_view.defense_selections.get("goblin", {}).get("ability_id") != "protect": failures.append("all revealed defenses were not retained on the battle mat: rolls=%s selections=%s" % [defense_view.defense_rolls, defense_view.defense_selections])
	var reveal_view := BattleViewState.new(); var reveal_fixture := _fixture("offensive_reaction")
	reveal_fixture.events = [{"type": "interaction_revealed", "segment": "offensive", "data": {"commitments": {"goblin": {"ability_id": "jagged_slash", "ai_d100": 7, "simulated_rolls": 1, "dice": [{"face": 1}, {"face": 2}, {"face": 3}, {"face": 4}, {"face": 5}], "outcome": {"base_damage": 4, "status_applications": [], "resource_gains": {}, "targets": ["blade"]}}}}}]
	if not reveal_view.apply_result(reveal_fixture) or reveal_view.rolled_dice("goblin").size() != 5 or reveal_view.offensive_reveal("goblin").get("ai_d100") != 7 or reveal_view.offensive_reveal("goblin").get("outcome", {}).get("base_damage") != 4: failures.append("public enemy offensive outcome was not retained after reveal: %s" % reveal_view.offensive_reveals)
	var reopened_view := BattleViewState.new(); var reopened_fixture := _fixture("offensive_reaction")
	reopened_fixture.snapshot.actors.goblin["selected_ability"] = "jagged_slash"; reopened_fixture.snapshot.actors.goblin["selected_tier"] = "three_swords"; reopened_fixture.snapshot.actors.goblin["selected_targets"] = ["blade"]; reopened_fixture.snapshot.actors.goblin["offensive_outcome"] = {"base_damage": 4, "status_applications": [], "resource_gains": {}, "targets": ["blade"]}; reopened_fixture.snapshot.actors.goblin["roll_history"] = [{"dice": [{"face": 1}, {"face": 2}, {"face": 3}, {"face": 4}, {"face": 5}]}]
	if not reopened_view.apply_result(reopened_fixture) or reopened_view.offensive_reveal("goblin").get("outcome", {}).get("base_damage") != 4 or reopened_view.rolled_dice("goblin").size() != 5: failures.append("reopened offensive reaction lost its snapshot outcome: %s" % reopened_view.actor("goblin"))

func _test_director_and_fake() -> void:
	var fake := FakeBattleAuthority.new(); fake.enqueue(_fixture("planning"))
	var gateway := BattleGateway.new(fake); var result := gateway.submit(BattleCommandBuilder.open_battle("b1"))
	if result.get("accepted") != true or fake.commands.size() != 1: failures.append("fake authority injection failed")
	var director := BattlePresentationDirector.new(); var with_events := _fixture("planning")
	with_events.events = [{"sequence": 1, "type": "segment_entered", "to": "income"}, {"sequence": 2, "type": "cards_drawn", "actor_id": "blade", "cards": ["card-1"]}, {"sequence": 3, "type": "energy_points_gained", "actor_id": "blade", "energy_points": 3}, {"sequence": 4, "type": "cards_drawn", "actor_id": "goblin", "count": 1}, {"sequence": 5, "type": "energy_points_gained", "actor_id": "goblin", "energy_points": 2}]
	director.queue_result(with_events)
	var seen: Array = []
	while director.has_beats():
		seen.append(director.peek().sequence)
		if director.peek().get("type") == "income_summary":
			var income_actors: Dictionary = director.peek().event.data.actors
			if income_actors.blade.cards != ["card-1"] or income_actors.blade.energy_points != 3 or income_actors.goblin.card_count != 1 or income_actors.goblin.energy_points != 2: failures.append("income summary omitted an actor result: %s" % income_actors)
		director.advance()
	if seen != [2]: failures.append("income presentation retained a redundant segment pause: %s" % seen)
	director.queue_result(with_events, 5)
	if director.has_beats(): failures.append("reopen replayed presented events")
	if fake.commands.size() != 1: failures.append("automatic presentation sent gameplay command")
	var opening_events := _fixture("planning")
	opening_events.events = [
		{"sequence": 1, "type": "cards_drawn", "actor_id": "blade", "cards": ["opening-player"]},
		{"sequence": 2, "type": "cards_drawn", "actor_id": "goblin", "count": 4},
		{"sequence": 3, "type": "segment_entered", "to": "ongoing_effects"},
		{"sequence": 4, "type": "segment_entered", "to": "income"},
		{"sequence": 5, "type": "cards_drawn", "actor_id": "goblin", "count": 1},
	]
	var opening_director := BattlePresentationDirector.new()
	opening_director.queue_result(opening_events)
	var beats: Array = []
	while opening_director.has_beats(): beats.append(opening_director.peek().duplicate(true)); opening_director.advance()
	if beats.size() != 2: failures.append("active Income retained its default pause or opening draws leaked: %s" % beats)
	elif beats[0].get("detail") != "No ongoing effects to resolve": failures.append("empty Effects segment did not retain its fallback: %s" % beats[0])
	elif beats[1].get("type") != "income_summary" or beats[1].event.data.actors.goblin.card_count != 1: failures.append("enemy income was omitted or leaked hidden cards: %s" % beats[1])
	var damage_director := BattlePresentationDirector.new()
	damage_director.queue_result({"events": [{"sequence": 1, "type": "damage_cards_revealed", "data": {"cards": [{"card_id": "one"}, {"card_id": "two"}, {"card_id": "three"}]}}]})
	if damage_director.has_beats(): failures.append("damage reveal created a popup instead of using the damage board: %s" % damage_director.peek())
	var committed_director := BattlePresentationDirector.new()
	committed_director.queue_result({"events": [{"sequence": 1, "type": "cards_permanently_removed", "cards": ["card-1"], "target_actor_id": "goblin"}, {"sequence": 2, "type": "cards_permanently_removed", "cards": ["card-2"], "target_actor_id": "goblin"}, {"sequence": 3, "type": "damage_committed", "data": {"removals": null}}]})
	if committed_director.has_beats(): failures.append("committed damage created per-card presentation pauses: %s" % committed_director.peek())
	var defense_director := BattlePresentationDirector.new()
	defense_director.queue_result({"events": [
		{"sequence": 1, "type": "segment_entered", "segment": "defensive"},
		{"sequence": 2, "type": "defense_selected", "actor_id": "goblin", "segment": "defensive", "data": {"ability_id": "basic_defense", "source_id": "source-1", "rolled_face": 6}},
		{"sequence": 3, "type": "segment_entered", "segment": "damage_resolution"},
	]})
	if defense_director.peek().get("detail") != "No damage to resolve": failures.append("empty Damage segment omitted its fallback: %s" % defense_director.peek())
	defense_director.advance()
	if defense_director.has_beats(): failures.append("unexpected presentation beats remained after empty Damage fallback")
	var effects_director := BattlePresentationDirector.new()
	effects_director.queue_result({"events": [{"sequence": 1, "type": "segment_entered", "segment": "ongoing_effects"}, {"sequence": 2, "type": "status_changed", "segment": "ongoing_effects", "data": {"status_id": "poison"}}]})
	if effects_director.peek().get("type") != "status_changed": failures.append("active Effects retained its redundant segment pause: %s" % effects_director.peek())
	var defense_roll_director := BattlePresentationDirector.new()
	defense_roll_director.queue_result({"events": [{"sequence": 1, "type": "dice_rolled", "actor_id": "blade", "segment": "defensive", "pool": "defensive", "source_id": "basic_defense", "dice": [{"face": 5}]}]})
	if defense_roll_director.has_beats(): failures.append("defensive die incorrectly created a presentation popup: %s" % defense_roll_director.peek())

func _fixture(stage: String) -> Dictionary:
	var terminal := stage == "victory"
	return {"accepted": true, "battle_result": "victory" if terminal else "", "events": [], "pending_input": {} if terminal or stage == "income" else {"blade": PENDING.merged({"stage": stage, "allowed_commands": ["pass"]}, true)}, "snapshot": {"battle_id": "fixture", "status": "victory" if terminal else "active", "battle_result": "victory" if terminal else "", "viewer_actor_id": "blade", "round": 2, "segment": "income" if stage == "income" else "damage_resolution" if "damage" in stage or stage == "discard_to_hand_limit" else "defensive" if stage.begins_with("defense") else "ongoing_effects" if "status" in stage else "offensive", "stage": stage, "damage_sources": [{"id": "source-1", "source_actor_id": "goblin", "source_content_id": "venom_strike", "target_actor_id": "blade", "base_amount": 3, "final_amount": 2}], "settled_damage": {"removals": [{"card_id": "card-1", "card_definition_id": "tip_it", "target_actor_id": "blade", "original_zone": "hand", "accepted": true, "released": false}]}, "actors": {"blade": {"definition_id": "blade_warden", "hand": ["card-1"], "hand_count": 1, "card_instances": {"card-1": {"instance_id": "card-1", "definition_id": "tip_it"}}, "current_health": 19, "max_health": 20, "offensive_abilities": ["sword_cut"], "defensive_abilities": ["basic_defense"], "roll_history": [{"dice": [{"face": 1, "symbols": ["sword"]}]}]}, "goblin": {"definition_id": "venom_goblin", "hand_count": 2, "current_health": 10, "max_health": 12, "offensive_abilities": ["venom_strike"], "defensive_abilities": ["protect"]}}}}

func _plan_payload(extra: Dictionary = {}) -> Dictionary:
	var value := {"pending_input_id": "pending-7", "checkpoint": {"window_id": "window-4", "segment": "offensive", "stage": "planning", "iteration": 3, "planning_cycle": 5}}
	for key in extra: value[key] = extra[key]
	return value
func _interaction_checkpoint() -> Dictionary: return {"window_id": "window-4", "stage": "planning", "iteration": 3, "reaction_round": 2, "planning_cycle": 5}
func _expect(json: String, kind: String, payload: Dictionary) -> void:
	var parsed = JSON.parse_string(json)
	var normalized_payload = JSON.parse_string(JSON.stringify(payload))
	if not parsed is Dictionary or parsed.get("battle_id") != "b1" or parsed.get("actor_id") != "blade" or parsed.get("type") != kind or parsed.get("payload") != normalized_payload: failures.append("builder mismatch %s: %s" % [kind, json])
