extends SceneTree

var gateway := BattleGateway.new()
var result: Dictionary
var battle_id := ""
var reached := {}
var played_cards := {}
var kept_roll_count := -1
var ui_checked := {}

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	battle_id = "playthrough-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	result = gateway.start_battle(battle_id, "blade")
	if not _ok("start"): return
	for step in 800:
		var snapshot: Dictionary = result.get("snapshot", {})
		var stage := str(snapshot.get("stage", "")); reached[stage] = true
		if not ui_checked.has(stage):
			if not await _verify_real_ui(stage): return
			ui_checked[stage] = true
		if not str(result.get("battle_result", "")).is_empty() or str(snapshot.get("status", "")) in ["victory", "defeat", "draw"]:
			print("REAL PLAYTHROUGH: complete in %d commands; result=%s; stages=%s; cards=%s" % [step, str(result.get("battle_result", snapshot.get("status", ""))), JSON.stringify(reached), JSON.stringify(played_cards)])
			quit(0); return
		var pending: Dictionary = result.get("pending_input", {}).get("blade", {})
		if pending.is_empty(): _fail("nonterminal result has no blade pending: %s" % JSON.stringify(result)); return
		var allowed: Array = pending.get("allowed_commands", [])
		var command_json := _choose(snapshot, pending, allowed)
		if command_json.is_empty(): _fail("unhandled wait stage=%s allowed=%s" % [stage, allowed]); return
		result = gateway.submit(command_json)
		if not _ok("%s %s" % [stage, JSON.parse_string(command_json).get("type", "")]): return
	_fail("battle did not finish within 800 commands")

func _choose(snapshot: Dictionary, pending: Dictionary, allowed: Array) -> String:
	var stage := str(pending.get("stage", "")); var blade: Dictionary = snapshot.get("actors", {}).get("blade", {})
	if stage == "planning":
		var focus := _card(blade, "battle_focus")
		if "planning_commit_cards" in allowed and not focus.is_empty() and not played_cards.has("battle_focus"):
			played_cards["battle_focus"] = true; return BattleCommandBuilder.planning_commit_cards(battle_id, "blade", pending, [focus], ["blade"])
		var history: Array = blade.get("roll_history", [])
		if history.is_empty() and "planning_roll" in allowed: return BattleCommandBuilder.planning_roll(battle_id, "blade", pending)
		var qualified: Array = blade.get("qualified_abilities", [])
		if not qualified.is_empty() and "planning_select_ability" in allowed: return BattleCommandBuilder.planning_select_ability(battle_id, "blade", pending, str(qualified[0]), ["goblin"])
		if not history.is_empty():
			var rolls_used := history.size(); var max_rolls := 3
			if rolls_used != kept_roll_count and rolls_used < max_rolls and "planning_keep" in allowed:
				kept_roll_count = rolls_used; return BattleCommandBuilder.planning_keep(battle_id, "blade", pending, [0])
			if rolls_used < max_rolls and "planning_reroll" in allowed: return BattleCommandBuilder.planning_reroll(battle_id, "blade", pending, [1, 2, 3, 4])
		if "planning_pass" in allowed: return BattleCommandBuilder.planning_pass(battle_id, "blade", pending)
	if stage == "defense_selection":
		var source := _incoming(snapshot, "blade")
		if "planning_select_ability" in allowed and not source.is_empty(): return BattleCommandBuilder.planning_select_ability(battle_id, "blade", pending, "basic_defense", [source])
		if "planning_pass" in allowed: return BattleCommandBuilder.planning_pass(battle_id, "blade", pending)
	if "roll_dice" in allowed: return BattleCommandBuilder.roll_dice(battle_id, "blade", pending)
	if "commit_interaction" in allowed:
		if stage == "status_roll_reaction":
			var antidote := _card(blade, "antidote")
			if not antidote.is_empty() and int(blade.get("energy_points", 0)) > 0 and not blade.get("statuses", []).is_empty():
				played_cards["antidote"] = true; return BattleCommandBuilder.commit_interaction(battle_id, "blade", pending, [antidote], [], [], str(blade.get("statuses", [])[0].get("definition_id", "")))
		if str(snapshot.get("segment", "")) == "damage_resolution" and "damage" in stage:
			var ward := _card(blade, "emergency_ward"); var source := _incoming(snapshot, "blade")
			if not ward.is_empty() and not source.is_empty() and int(blade.get("energy_points", 0)) > 0 and not played_cards.has("emergency_ward"):
				played_cards["emergency_ward"] = true; return BattleCommandBuilder.commit_interaction(battle_id, "blade", pending, [ward], [source])
		if stage == "discard_to_hand_limit":
			var need := int(blade.get("hand_count", 0)) - int(blade.get("max_hand_size", 6)); return BattleCommandBuilder.commit_interaction(battle_id, "blade", pending, blade.get("hand", []).slice(0, need))
	if "pass" in allowed: return BattleCommandBuilder.pass_command(battle_id, "blade", pending)
	return ""

func _card(actor: Dictionary, definition: String) -> String:
	for id in actor.get("hand", []):
		if actor.get("card_instances", {}).get(id, {}).get("definition_id") == definition: return str(id)
	return ""
func _incoming(snapshot: Dictionary, target: String) -> String:
	for source in snapshot.get("damage_sources", []):
		if source.get("target_actor_id") == target: return str(source.get("id", ""))
	return ""

func _verify_real_ui(stage: String) -> bool:
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = result
	screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("playthrough_ui_active.json"))
	screen.last_presented_sequence = 999999
	root.add_child(screen)
	await process_frame
	await process_frame
	var pending: Dictionary = result.get("pending_input", {}).get("blade", {})
	var allowed: Array = pending.get("allowed_commands", [])
	var required := {
		"planning_roll": "Roll 5 Dice",
		"planning_reroll": "Reroll Unkept",
		"pass": "Pass / Acknowledge",
	}
	if _has_button(screen, "Keep Selected", false):
		var summary := _button_summary(screen); screen.free(); _fail("real UI still exposes the removed Keep Selected action; buttons=%s" % summary); return false
	var view := BattleViewState.new()
	if not view.apply_result(result): screen.free(); _fail("real UI result could not populate control lifecycle view"); return false
	if "planning_pass" in allowed:
		var pass_label := "Pass Defense" if stage == "defense_selection" else "Pass Planning"
		if not _has_button(screen, pass_label, true):
			var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s lacks enabled planning_pass control; buttons=%s" % [stage, summary]); return false
	for command_type in required:
		var should_show: bool = command_type in allowed
		if command_type == "planning_roll": should_show = should_show and view.rolls_used("blade") == 0
		elif command_type == "planning_reroll": should_show = should_show and view.rolls_used("blade") > 0 and view.rolls_used("blade") < view.max_rolls("blade")
		if should_show and not _has_button(screen, required[command_type], true):
			var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s lacks enabled %s control; buttons=%s" % [stage, command_type, summary]); return false
		if not should_show and _has_button(screen, required[command_type], true):
			var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s exposes invalid %s control; buttons=%s" % [stage, command_type, summary]); return false
	var pending_defense_die := _button_with_inspection_id(screen, "battle.defense_die.blade.pending")
	var enemy_pending_defense_die := _button_with_inspection_id(screen, "battle.defense_die.goblin.pending")
	var pending_effect_dice := _buttons_with_inspection_prefix(screen, "battle.effect_die.blade.pending.")
	var enemy_effect_dice := _buttons_with_inspection_prefix(screen, "battle.effect_die.goblin")
	if "roll_dice" in allowed and stage == "defense_roll" and (pending_defense_die == null or pending_defense_die.disabled or not pending_defense_die.text.is_empty()):
		var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s lacks an enabled blank defense die; buttons=%s" % [stage, summary]); return false
	if "roll_dice" in allowed and stage == "status_roll" and (pending_effect_dice.is_empty() or pending_effect_dice[0].disabled or not pending_effect_dice[0].text.is_empty()):
		var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s lacks enabled blank effect dice; buttons=%s" % [stage, summary]); return false
	if enemy_pending_defense_die != null:
		var summary := _button_summary(screen); screen.free(); _fail("real UI exposed a player-controlled enemy defense die; buttons=%s" % summary); return false
	if stage == "status_roll" and not enemy_effect_dice.is_empty():
		var summary := _button_summary(screen); screen.free(); _fail("real UI exposed player-controlled enemy effect dice; buttons=%s" % summary); return false
	if "roll_dice" not in allowed and (pending_defense_die != null or not pending_effect_dice.is_empty()):
		var summary := _button_summary(screen); screen.free(); _fail("real UI stage %s exposes an invalid blank die; buttons=%s" % [stage, summary]); return false
	if _has_button(screen, "Roll Defense Die", false):
		var summary := _button_summary(screen); screen.free(); _fail("real UI still exposes the removed Roll Defense Die action; buttons=%s" % summary); return false
	if "planning_select_ability" in allowed:
		var should_have_ability: bool = stage == "defense_selection" or not result.get("snapshot", {}).get("actors", {}).get("blade", {}).get("qualified_abilities", []).is_empty()
		if stage == "defense_selection":
			var source_button: Button = _button_containing(screen, "→ blade")
			if source_button == null:
				screen.free(); _fail("defense UI lacks incoming source selector"); return false
			source_button.pressed.emit()
		if should_have_ability and not _has_enabled_ability(screen):
			screen.free(); _fail("real UI stage %s lacks an enabled ability" % stage); return false
	if stage == "discard_to_hand_limit" and _button_containing(screen, "Discard ") == null:
		screen.free(); _fail("hand-limit UI lacks discard commit control"); return false
	if result.get("battle_result", "") != "" and not _has_button(screen, "Play Again", true):
		screen.free(); _fail("terminal UI lacks Play Again"); return false
	if OS.get_environment("CAPTURE_REAL_SEGMENTS") == "1":
		var output := ProjectSettings.globalize_path("res://tests/screenshots/real-%s.png" % stage)
		DirAccess.make_dir_recursive_absolute(output.get_base_dir())
		var image := root.get_texture().get_image()
		if image == null or image.save_png(output) != OK:
			screen.free(); _fail("could not capture real UI stage %s" % stage); return false
	screen.free()
	return true

func _has_button(node: Node, exact_text: String, enabled: bool) -> bool:
	for child in _all_buttons(node):
		if child.text == exact_text and (not enabled or not child.disabled): return true
	return false

func _button_containing(node: Node, text_part: String) -> Button:
	for child in _all_buttons(node):
		if text_part in child.text: return child
	return null

func _button_with_inspection_id(node: Node, inspection_id: String) -> Button:
	for child in _all_buttons(node):
		if child.has_meta("inspection_id") and child.get_meta("inspection_id") == inspection_id: return child
	return null

func _buttons_with_inspection_prefix(node: Node, prefix: String) -> Array[Button]:
	var result: Array[Button] = []
	for child in _all_buttons(node):
		if child.has_meta("inspection_id") and str(child.get_meta("inspection_id")).begins_with(prefix): result.append(child)
	return result

func _has_enabled_ability(node: Node) -> bool:
	for child in _all_buttons(node):
		if child is BattleAbilityTile and not child.disabled: return true
	return false

func _button_summary(node: Node) -> Array:
	var result := []
	for child in _all_buttons(node): result.append({"text": child.text, "disabled": child.disabled})
	return result

func _all_buttons(node: Node) -> Array[Button]:
	var result: Array[Button] = []
	for child in node.get_children():
		if child is Button: result.append(child)
		result.append_array(_all_buttons(child))
	return result
func _ok(step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)]); return false
func _fail(message: String) -> void: push_error(message); quit(1)
