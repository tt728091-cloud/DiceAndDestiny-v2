extends SceneTree
## The opponent's offensive planning is private, so its steps (roll, keep,
## reroll, ability) must not rebuild the battle board one by one: each rebuild
## froze the screen ~250 ms and recreated the dice. The board rebuilds once
## when the opponent hands the decision back, and a rebuilt dice tray keeps
## the same stone grain.
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("MODEL PLANNING QUIET: " + message)
func _trays(screen) -> Array:
	return screen._player_dice_dock.find_children("*", "BattleDiceTray", true, false) if is_instance_valid(screen._player_dice_dock) else []
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	for key in ["combat_transition_seconds", "damage_reveal_seconds", "damage_hold_seconds", "damage_removal_seconds", "effects_gather_seconds", "defense_roll_seconds", "defense_effects_seconds", "defense_hold_seconds", "income_animation_seconds"]:
		ProjectSettings.set_setting("dice_and_destiny/presentation/" + key, 0.1)
	var runtime := root.get_node("LearnedBattleRuntime")
	var gateway: RefCounted = LEARNED_GATEWAY.new(runtime, "seat-a", "minion:drowned_oracle_bell_diver:1", "starter")
	gateway.unified_defense = true
	var screen = BATTLE_SCREEN.instantiate(); screen.initial_result = gateway.start_battle("quiet-planning", 7); screen.viewer_actor_id = "blade"; screen.gateway = gateway
	screen.learned_battle_mode = true; screen.learned_human_seat = "seat-a"; screen.learned_seed = 7
	root.add_child(screen)
	var deadline := Time.get_ticks_msec() + 150000
	var planning_rebuilds := 0; var last_tray := 0; var was_model_planning := false; var handbacks := 0
	var grain_seeds := {}
	var steps_seen := 0
	while Time.get_ticks_msec() < deadline and not screen._view.is_complete() and handbacks < 3:
		var model_planning: bool = screen._view.segment == "offensive" and screen._view.stage == "planning" and bool(screen._view.learned_policy.get("model_turn", false))
		var trays := _trays(screen)
		var tid: int = trays[0].get_instance_id() if not trays.is_empty() else 0
		if tid != last_tray and tid != 0:
			# A rebuild that coincides with a new opponent step (rather than the
			# player's own roll animation ending) is the regression.
			if model_planning and was_model_planning and screen._model_quiet_steps > steps_seen:
				planning_rebuilds += 1
				print("rebuild after an opponent planning step (quiet steps %d)" % screen._model_quiet_steps)
			last_tray = tid
			# The same die slot shows the same grain on every rebuild.
			for stone in trays[0].find_children("*", "", true, false).filter(func(n): return n.get("grain_seed") != null and n.get("mode") == 1):
				var slot := str(stone.get_parent().get_index()) + str(stone.get_parent().get_parent().get_index())
				if grain_seeds.has(slot): _expect(grain_seeds[slot] == stone.grain_seed, "die slot %s keeps its grain (%d vs %d)" % [slot, grain_seeds[slot], stone.grain_seed])
				grain_seeds[slot] = stone.grain_seed
		if was_model_planning and not model_planning: handbacks += 1
		steps_seen = screen._model_quiet_steps
		was_model_planning = model_planning
		var legal: Array = screen._view.legal_actions
		if not screen._submitting and not screen._model_thinking and not screen._player_roll_active() and not screen._director.has_beats() and Time.get_ticks_msec() >= screen._interaction_deadline(true) and not bool(screen._view.learned_policy.get("model_turn", false)):
			for kind in ["planning_select_ability", "planning_roll", "roll_dice", "pass", "planning_pass", "commit_interaction"]:
				var hit = legal.filter(func(a): return a.get("type") == kind)
				if not hit.is_empty(): screen._send(JSON.stringify(hit[0])); break
		if screen._director.has_beats() and not screen._model_thinking: screen._advance_beat()
		await process_frame
	_expect(handbacks >= 2, "the opponent planned and handed back at least twice (%d)" % handbacks)
	_expect(screen._model_quiet_steps >= 2, "opponent planning steps were applied without a rebuild (%d)" % screen._model_quiet_steps)
	_expect(planning_rebuilds == 0, "the board was not rebuilt between the opponent's private planning steps (%d rebuilds)" % planning_rebuilds)
	_expect(not grain_seeds.is_empty(), "the dice tray's stones carry a stable grain seed")
	_expect(screen._error_message.is_empty() and not screen._model_error, "no authority or controller error: %s" % screen._error_message)
	print("model planning quiet: %d quiet steps, %d hand-backs, %d rebuilds mid-planning" % [screen._model_quiet_steps, handbacks, planning_rebuilds])
	print("MODEL PLANNING QUIET: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
