extends Control

const SEGMENTS := [["ongoing_effects", "Effects"], ["income", "Income"], ["offensive", "Offensive"], ["defensive", "Defensive"], ["damage_resolution", "Damage"]]
const INCOME_DURATION_SETTING := "dice_and_destiny/presentation/income_animation_seconds"
const CARD_GAIN_NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
const MODEL_TIMEOUT_MS := 2000
const TRANSCRIPT_PANEL := preload("res://devtools/developer_authority_transcript_panel.gd")
const SCENERY := preload("res://presentation/battle/battle_scenery.gd")

## Null uses the visual library's default. Encounter art never selects combat rules.
@export var encounter_visual: BattleEncounterVisual

var initial_result: Dictionary = {}
var viewer_actor_id := "blade"
var gateway: Object
var active_store: ActiveBattleStore
var last_presented_sequence := 0
var loaded_snapshot_name := ""
var history_context: Dictionary = {}
var learned_battle_mode := false
var learned_human_seat := "seat-a"
var learned_seed := 0

var _focused_enemy := "goblin"
var _enemy_selector: Control
var _enemy_buttons: Dictionary = {}
var _defense_focus_key := ""
var _defense_playback_source := ""
var _view := BattleViewState.new()
var _director := BattlePresentationDirector.new()
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
var _hand_dock: VBoxContainer
var _enemy_attack_dock: VBoxContainer
var _ability_dock: VBoxContainer
var _roll_dock: VBoxContainer
var _player_dice_dock: VBoxContainer
var _enemy_dice_dock: VBoxContainer
var _player_profile_dock: VBoxContainer
var _enemy_profile_dock: VBoxContainer
var _utility_panels: Dictionary = {}
var _utility_contents: Dictionary = {}
var _open_utility := ""
var _root: Control
var _center: VBoxContainer
var _log: RichTextLabel
var _error: Label
var _error_message := ""
var _submitting := false
var _selected_indices: Array = []
var _selection_roll_number := -1
var _player_roll_feedback: Dictionary = {}
var _selected_source := ""
var _selected_card: Dictionary = {}
var _hand_limit_selection: Array = []
var _snapshot_panel_open := false
var _snapshot_name := ""
var _selected_snapshot_name := ""
var _snapshot_overwrite := false
var _snapshot_entries: Array = []
var _snapshot_message := ""
var _history_entries: Array = []
var _history_branch: Dictionary = {}
var _history_message := ""
var _history_review := false
var _history_replay := false
var _history_point_id := ""
var _history_origin_battle_id := ""
var _history_pending_divergence: Dictionary = {}
var _history_scroll_value := 0
var _history_follow_latest := true
var _history_scroll_adjusting := false
var _actor_profiles: Dictionary = {}
var _income_drawn_cards: Array[BattleCard] = []
var _income_animation_generation := 0
var _cleanse_animation_times: Dictionary = {}
var _effects_panel: Control
var _effects_beat_key := ""
var _effects_elapsed := 0.0
var _model_thinking := false
var _model_thread: Thread
var _model_started_ms := 0
var _model_timeout_warning := false
var _model_error := false
var _reaction_notice := ""
var _transcript_panel: CanvasLayer
var _last_auto_pass_input := ""
const COMBAT_TIMING := preload("res://presentation/battle/combat_timing.gd")
var _flow_transition: Control
var _flow_state := ""
var _flow_until := 0
var _timed_buttons: Array[Dictionary] = []
var _selection_morph: Dictionary = {}
var _combat_columns: Dictionary = {}
var _damage_card_times: Dictionary = {}
var _damage_grids: Array = []
var _damage_commit_started := {}
const DEFENSE_TIMING := preload("res://presentation/battle/defense_timing.gd")
const DAMAGE_AUTO_PASS_CLICK_MS := 250
var _auto_pass_disabled := false
var _damage_reviewed_batch := ""
var _defense_reviewed_result := ""
var _damage_feedback: Dictionary = {}
var _defense_auto_roll_key := ""
var _last_defense_auto_roll := ""
var _auto_pass_toggle: CheckBox
var _auto_pass_preview_input := ""
var _auto_pass_preview_started_ms := 0
var _auto_pass_highlight_ms := -1
var _auto_pass_button: Button
var _ability_upgrade_feedback: Dictionary = {}
var _reaction_card_feedback: Dictionary = {}
var _center_scroll: ScrollContainer
var _action_footer: HBoxContainer
var _content_scroll_key := ""
var _defense_animation_times: Dictionary = {}
var _defense_shared_round := ""
var _defense_shared_start := 0
var _defense_shared_first_wave := ""
var _defense_result_panels: Array = []
var _provoked_panel: VBoxContainer
var _selected_attack_tiles: Dictionary = {}
const DEFENSE_RESULT := preload("res://presentation/battle/defense_result.gd")
const DEFENSE_STATUS_FLIGHT := preload("res://presentation/battle/defense_status_flight.gd")

func _reaction_feedback_active() -> bool:
	return _reaction_card_feedback.get("battle_id") == _view.battle_id and Time.get_ticks_msec() < int(_reaction_card_feedback.get("expires_ms", 0))

func _ready() -> void:
	theme = CINEMATIC.create()
	add_to_group("inspectable_battle_screen")
	resized.connect(_layout_cinematic_root)
	gateway = BattleGateway.new() if gateway == null else gateway
	active_store = ActiveBattleStore.new() if active_store == null else active_store
	if not _view.apply_result(initial_result):
		_build_error_only(str(initial_result.get("error", "Battle snapshot is missing or unsafe.")))
		return
	if TRANSCRIPT_PANEL.is_enabled():
		_transcript_panel = TRANSCRIPT_PANEL.new()
		_transcript_panel.set_current_battle(_view.battle_id)
		add_child(_transcript_panel)
	_apply_history_context(history_context)
	_director.configure_learned_battle(learned_battle_mode)
	_director.queue_result(initial_result, last_presented_sequence)
	if _history_tools_enabled():
		_refresh_history()
		_record_history_arrival()
	_render()
	if learned_battle_mode:
		call_deferred("_schedule_model_if_needed", initial_result)

func _process(_delta: float) -> void:
	if _player_roll_active(): return
	if not _player_roll_feedback.is_empty():
		_player_roll_feedback.clear()
		_render()
	for item in _timed_buttons:
		if is_instance_valid(item.button) and Time.get_ticks_msec() >= int(item.until):
			item.button.disabled = _submitting or _model_thinking or _history_review or _director.has_beats()
	if Time.get_ticks_msec() < _flow_until or not _selection_morph.is_empty(): return
	if not _model_thinking and not _director.has_beats() and bool(_view.learned_policy.get("model_turn", false)):
		_schedule_model_if_needed({"learned_policy": _view.learned_policy})
	if _model_thread == null or not _model_thread.is_started():
		if not _reaction_card_feedback.is_empty() and not _reaction_feedback_active():
			_reaction_card_feedback.clear()
			if not _history_review and not _history_replay and not _snapshot_panel_open:
				_schedule_model_if_needed({"learned_policy": _view.learned_policy})
		if _provoked_toxin_reaction() and bool(_view.learned_policy.get("model_turn", false)):
			_schedule_model_if_needed({"learned_policy": _view.learned_policy})
			if _model_thinking: return
		if _auto_roll_defense_if_only_action(): return
		_auto_pass_if_only_action()
		return
	var elapsed := Time.get_ticks_msec() - _model_started_ms
	if _model_thread.is_alive():
		if elapsed > MODEL_TIMEOUT_MS and not _model_timeout_warning:
			_model_timeout_warning = true
			_error_message = "Learned-policy inference exceeded %d ms. No fallback action will be submitted." % MODEL_TIMEOUT_MS
			_render()
		return
	var result = _model_thread.wait_to_finish()
	_model_thread = null
	_model_thinking = false
	_submitting = false
	if not result is Dictionary:
		_model_error = true
		_show_error("The learned policy returned an invalid response.")
		_render()
		return
	_apply_model_result(result)

func _auto_pass_if_only_action() -> void:
	if Time.get_ticks_msec() < _flow_until or not _selection_morph.is_empty(): return
	if _provoked_toxin_reaction() and is_instance_valid(_provoked_panel) and not _provoked_panel.ready_to_continue(): return
	var action := _sole_pass_action()
	if action.is_empty():
		_reset_auto_pass_preview()
		return
	var key := _view.battle_id + ":" + str(_pending().get("id", ""))
	if key == _last_auto_pass_input: return
	var review_ms := 0
	if _view.stage == "defense_reaction":
		if not _defense_result_panels.is_empty() and _defense_reviewed_result != _defense_review_key():
			var review_seconds := DEFENSE_TIMING.total_seconds()
			if _multiple_enemies() and _defense_shared_first_wave != _defense_review_key(): review_seconds -= DEFENSE_TIMING.roll_seconds()
			review_ms = ceili(review_seconds * 1000.0)
	elif _continuous_damage_response():
		if _damage_reviewed_batch != _damage_batch_key(): review_ms = ceili(COMBAT_TIMING.review_seconds() * 1000.0)
	# Even a no-choice handoff can be the last human checkpoint before the
	# opponent passes and the authority finishes defense/damage in one command.
	# Show the defense first, then allow subsequent handoffs without replaying it.
	if _pass_hands_off_priority() and _view.stage != "defense_reaction" and not _continuous_damage_response(): review_ms = 0
	if review_ms > 0:
		var now := Time.get_ticks_msec()
		if key != _auto_pass_preview_input:
			_reset_auto_pass_preview()
			_auto_pass_preview_input = key
			_auto_pass_preview_started_ms = now
			if _view.stage == "defense_reaction" and _multiple_enemies():
				# Use the animation clock: card choices and priority handoffs must
				# not restart a review delay after these effects have already landed.
				_auto_pass_preview_started_ms = _defense_effects_finished_at() - review_ms
		if now - _auto_pass_preview_started_ms < review_ms: return
		if _auto_pass_highlight_ms < 0:
			_auto_pass_highlight_ms = now
			_style_auto_pass_button(true)
			return
		if now - _auto_pass_highlight_ms < DAMAGE_AUTO_PASS_CLICK_MS: return
	# Re-evaluate legality every frame, including after the review and highlight.
	# _send preserves normal authority validation, transcript, saves and history.
	if _continuous_damage_response(): _damage_reviewed_batch = _damage_batch_key()
	if _view.stage == "defense_reaction": _defense_reviewed_result = _defense_review_key()
	_reset_auto_pass_preview()
	_last_auto_pass_input = key
	_send(JSON.stringify(action))

func _defense_review_key() -> String:
	var rolls: Array = []
	var actor_ids := _view.defense_rolls.keys(); actor_ids.sort()
	for actor_id in actor_ids:
		var roll: Dictionary = _view.defense_rolls[actor_id]
		var faces: Array[int] = []
		for face in roll.get("rolled_faces", [roll.get("face", 0)]): faces.append(int(face))
		rolls.append([str(actor_id), str(roll.get("ability_id", "")), str(roll.get("source_id", "")), faces, bool(roll.get("catalyst_paid", false))])
	return "%s:%d:%s" % [_view.battle_id, _view.round_number, JSON.stringify(rolls)]

func _sole_pass_action() -> Dictionary:
	if _view.stage == "defense_selection": return {}
	if _selected_card.get("source_targeting", false): return {}
	if _card_gain_active(): return {}
	if _auto_pass_disabled and not _provoked_toxin_reaction() and not _inline_status_application() and not _pass_hands_off_priority() and _view.stage != "offensive_reaction": return {}
	if _reaction_feedback_active(): return {}
	if _submitting or _model_thinking or _model_error or not _error_message.is_empty(): return {}
	if _history_review or _history_replay or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return {}
	if _director.has_beats() or _view.is_complete() or bool(_view.learned_policy.get("model_turn", false)): return {}
	# Allowed command categories are too broad: a reaction may allow cards even
	# when none can actually be played. Use the authority's concrete legal list.
	if _view.legal_actions.size() != 1: return {}
	var action: Dictionary = _view.legal_actions[0]
	var command_type := str(action.get("type", ""))
	if command_type not in ["pass", "planning_pass"] or not _view.allowed(command_type): return {}
	if str(action.get("actor_id", "")) != viewer_actor_id: return {}
	var pending := _pending()
	var input_id := str(pending.get("id", ""))
	if input_id.is_empty() or str(action.get("payload", {}).get("pending_input_id", "")) != input_id: return {}
	return action

func _set_auto_pass_disabled(disabled: bool) -> void:
	_auto_pass_disabled = disabled
	_defense_reviewed_result = ""
	# Cancel even an already-highlighted automatic click. Re-enabling starts a
	# fresh review period; manual acknowledgement is always available.
	_reset_auto_pass_preview()

func _reset_auto_pass_preview() -> void:
	_auto_pass_preview_input = ""
	_auto_pass_preview_started_ms = 0
	_auto_pass_highlight_ms = -1
	_style_auto_pass_button(false)

func _style_auto_pass_button(highlight: bool) -> void:
	if not is_instance_valid(_auto_pass_button): return
	_auto_pass_button.toggle_mode = highlight
	_auto_pass_button.set_pressed_no_signal(highlight)
	if highlight:
		var style := StyleBoxFlat.new()
		style.bg_color = Color("9b702c")
		style.border_color = Color("f5cf78")
		style.set_border_width_all(2)
		style.set_corner_radius_all(4)
		_auto_pass_button.add_theme_stylebox_override("pressed", style)
	else:
		_auto_pass_button.remove_theme_stylebox_override("pressed")

func _exit_tree() -> void:
	if _model_thread != null and _model_thread.is_started():
		_model_thread.wait_to_finish()
		_model_thread = null

func _card_gain_active() -> bool:
	for child in get_children():
		if child.get_script() == CARD_GAIN_NOTICE and not child.is_queued_for_deletion(): return true
	return false

func _schedule_model_if_needed(result: Dictionary) -> void:
	if _player_roll_active(): return
	if _card_gain_active(): return
	if Time.get_ticks_msec() < _flow_until or not _selection_morph.is_empty() or _director.has_beats(): return
	if _view.stage == "defense_reaction" and not _defense_result_panels.is_empty():
		for panel in _defense_result_panels:
			if Time.get_ticks_msec() - panel.started_ms < DEFENSE_TIMING.total_seconds() * 1000.0: return
	if not learned_battle_mode or _view.is_complete() or _model_thinking or _model_error:
		return
	if _director.has_pending_status_animation() or _reaction_feedback_active(): return
	if _provoked_toxin_reaction() and is_instance_valid(_provoked_panel) and not _provoked_panel.ready_to_continue(): return
	var metadata: Dictionary = result.get("learned_policy", {})
	if metadata.get("model_turn") != true:
		return
	_reset_auto_pass_preview()
	_model_thinking = true
	_submitting = true
	_model_timeout_warning = false
	_model_started_ms = Time.get_ticks_msec()
	_model_thread = Thread.new()
	var start_error := _model_thread.start(func(): return gateway.advance_model())
	if start_error != OK:
		_model_thread = null
		_model_thinking = false
		_submitting = false
		_model_error = true
		_show_error("Could not start the learned-policy inference worker (error %d)." % start_error)
		_render()
		return
	# Inference is a wait on the current board, not a presentation segment.
	# Lock gameplay in place without rebuilding or transitioning its controls.
	for control in _root.find_children("*", "BaseButton", true, false):
		if not control.get_meta("battle_utility", false): control.disabled = true

func _apply_model_result(result: Dictionary) -> void:
	if result.get("accepted") != true:
		_model_error = true
		_show_error(str(result.get("error", "The learned policy could not choose an action.")), result)
		_render()
		return
	var previous_actors := _view.actors
	var previous_damage := _view.settled_damage.duplicate(true)
	if not _view.apply_result(result):
		_model_error = true
		_show_error("The learned-policy result was not a safe human-viewer snapshot.", result)
		_render()
		return
	_capture_ability_upgrades(previous_actors)
	_capture_damage_feedback(result, previous_damage)
	_capture_offensive_reaction_notice(result)
	_clear_reaction_notice_for_new_round()
	_error_message = ""
	_model_error = false
	_selected_card.clear()
	_selected_source = ""
	_hand_limit_selection.clear()
	_selected_indices.clear()
	_director.queue_result(result, _director.last_sequence(), previous_actors)
	_render()
	call_deferred("_schedule_model_if_needed", result)

func _render(force: bool = false) -> void:
	_sync_enemy_focus()
	# Roll commands are authority work, not a separate visual beat. Keep the
	# selected board for this handoff; both rolls animate in the result panels.
	if not force and _view.stage == "defense_roll" and is_instance_valid(_root) and not _history_review and not _history_replay and not _snapshot_panel_open and not _model_error and _error_message.is_empty():
		for control in _root.find_children("*", "BaseButton", true, false):
			if not control.get_meta("battle_utility", false): control.disabled = true
		return
	if is_instance_valid(_effects_panel): _effects_elapsed = _effects_panel.playback_elapsed()
	_income_animation_generation += 1
	var scroll_key := "%s:%d:%s:%s" % [_view.battle_id, _view.round_number, _view.segment, _view.stage]
	var saved_scroll := _center_scroll.scroll_vertical if is_instance_valid(_center_scroll) and scroll_key == _content_scroll_key else 0
	_content_scroll_key = scroll_key
	if is_instance_valid(_transcript_panel): _transcript_panel.set_current_battle(_view.battle_id)
	if is_instance_valid(_provoked_panel):
		if _provoked_toxin_reaction():
			_provoked_panel.reparent(self); _provoked_panel.hide()
		else:
			_provoked_panel.queue_free(); _provoked_panel = null
	var flow_state := "%s:%d:%s:%s:%s:%s" % [_view.battle_id, _view.round_number, _view.stage, _director.peek().get("type", ""), _selection_morph.get("ability_id", ""), _director.peek().get("event", {}).get("sequence", _view.settled_damage.get("id", "") if _continuous_damage_response() else "")]
	var animate_flow := is_instance_valid(_root) and flow_state != _flow_state and not _history_review and not _history_replay
	_flow_state = flow_state
	if not is_instance_valid(_flow_transition):
		_flow_transition = preload("res://presentation/battle/combat_transition.gd").new(); add_child(_flow_transition)
	if not animate_flow: _flow_transition.finish()
	if animate_flow:
		_flow_transition.capture(_root)
		_flow_until = Time.get_ticks_msec() + ceili(COMBAT_TIMING.transition() * 1000.0)
	_timed_buttons.clear()
	_combat_columns.clear()
	_damage_grids.clear()
	_selected_attack_tiles.clear()
	_actor_profiles.clear()
	_defense_result_panels.clear()
	_income_drawn_cards.clear()
	if is_instance_valid(_root): _root.queue_free()
	_root = Control.new(); _root.name = "CinematicBattle"; add_child(_root)
	_root.theme = theme
	_layout_cinematic_root()
	var scenery := SCENERY.new(); scenery.name = "BattleScenery"
	_root.add_child(scenery); scenery.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	scenery.display(_battlefield_visual(), _view.actors)
	_player_profile_dock = _cinematic_box("PlayerProfile", Rect2(38, 22, 395, 220))
	_enemy_profile_dock = _cinematic_box("EnemyProfile", Rect2(1510, 22, 380, 220))
	_player_dice_dock = _cinematic_box("PlayerDice", Rect2(28, 245, 390, 100))
	_enemy_dice_dock = _cinematic_box("EnemyDice", Rect2(1510, 245, 380, 100))
	_ability_dock = _cinematic_scroll_box("AbilityRail", Rect2(28, 448, 390, 370))
	_enemy_attack_dock = _cinematic_scroll_box("EnemyAttackRail", Rect2(1510, 358, 380, 625))
	_hand_dock = _cinematic_box("HandDock", Rect2(475, 790, 965, 264))
	_roll_dock = _cinematic_box("RollControls", Rect2(28, 358, 248, 80))
	var footer := _cinematic_box("BattleActionFooter", Rect2(285, 358, 133, 80))
	_action_footer = HBoxContainer.new(); _action_footer.alignment = BoxContainer.ALIGNMENT_CENTER; footer.add_child(_action_footer)
	var full_height: bool = _director.peek().get("type") == "effects_resolved" or (not _central_combat() and _view.hand_cards().is_empty() and _view.stage != "planning")
	_center_scroll = ScrollContainer.new(); _center_scroll.name = "BattleContentScroll"; _root.add_child(_center_scroll)
	_place_cinematic(_center_scroll, Rect2(460, 165, 1030, 825) if _director.peek().get("type") == "effects_resolved" else Rect2(460, 215, 1030, 775) if full_height else Rect2(460, 215, 1030, 550))
	_center_scroll.follow_focus = true

	_center = VBoxContainer.new(); _center.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_theme_constant_override("separation", 10); _center_scroll.add_child(_center)
	_build_cinematic_utilities()
	var header := _cinematic_box("PhaseRail", Rect2(570, 18, 780, 130))
	_build_header(header)
	_build_player_column(null)
	_build_side_attacks()
	_build_center()
	_build_enemy_column(null)
	_build_enemy_selector(scenery)
	_show_pending_damage_statuses()
	if _director.peek().get("type") == "effects_resolved" and is_instance_valid(_effects_panel):
		_effects_panel.present_progress()
	_player_profile_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_enemy_profile_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_enemy_dice_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_player_dice_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	call_deferred("_layout_left_controls")
	# Show the planning reference only when the rail is not resolving an attack.
	if _ability_dock.get_child_count() == 0 and _view.segment in ["offensive", "income"] and not _director.has_beats():
		_build_ability_row("YOUR ABILITIES", _as_array(_view.actor("blade").get("offensive_abilities", [])), "blade")
	if _utility_contents.enemy.get_child_count() == 0:
		_build_ability_row("ENEMY ABILITIES", _as_array(_view.actor(_focused_enemy).get("offensive_abilities", [])), _focused_enemy)
	_build_ability_row("ENEMY DEFENSES", _as_array(_view.actor(_focused_enemy).get("defensive_abilities", [])), _focused_enemy)
	if _history_tools_enabled(): _build_history_bar(_utility_contents.history)
	for key in _utility_panels: _root.move_child(_utility_panels[key], _root.get_child_count() - 1)
	var card_updates := {}
	for update in _director.take_status_updates():
		if _history_review or _history_replay: continue
		if update.has("card_feedback"):
			var key := str(update.card_feedback.key)
			if not card_updates.has(key): card_updates[key] = []
			card_updates[key].append(update)
			continue
		var notice := preload("res://presentation/battle/status_change_notice.gd").new()
		notice.configure(self, update)
		add_child(notice)
	for key in card_updates:
		var changes: Array[Dictionary] = []; changes.assign(card_updates[key])
		var notice := CARD_GAIN_NOTICE.new(); notice.configure(self, changes); add_child(notice)
	# Apply surviving notices to newly built profiles before the first frame.
	for notice in get_children():
		if notice.get_script() in [preload("res://presentation/battle/status_change_notice.gd"), CARD_GAIN_NOTICE]:
			notice.refresh()
	if not _defense_result_panels.is_empty(): call_deferred("_start_defense_status_flights", _income_animation_generation)
	for control in [_hand_dock, _player_dice_dock, _enemy_dice_dock, _roll_dock, _action_footer.get_parent()]:
		if control.get_child_count() > 0: control.set_meta("flow_key", str(control.name))
	_build_error(_center)
	_center_scroll.set_deferred("scroll_vertical", saved_scroll)
	if _snapshot_panel_open: _build_snapshot_panel()
	if not _history_pending_divergence.is_empty(): _build_history_divergence_panel()
	move_child(_flow_transition, get_child_count() - 1)
	if animate_flow:
		_flow_transition.prepare(_root)
		_present_flow.call_deferred(_income_animation_generation)
	if _player_roll_active():
		for control in _root.find_children("*", "BaseButton", true, false):
			if not control.get_meta("battle_utility", false): control.disabled = true

func _present_flow(generation: int) -> void:
	# Nested lanes/grids need several Container layout passes. Preserve the old
	# panel until those rectangles settle; a provisional narrow rectangle would
	# squeeze the frame and text during the resize.
	var previous: Array = []
	var stable := 0
	for frame in 12:
		await get_tree().process_frame
		if generation != _income_animation_generation: return
		var rects: Array = []
		for column in _combat_columns.values():
			for panel in column.get_children(): rects.append(panel.get_global_rect())
		stable = stable + 1 if rects == previous else 0
		previous = rects
		if stable >= 2: break
	var prior_deadline := _flow_until
	_flow_until = Time.get_ticks_msec() + ceili(COMBAT_TIMING.transition() * 1000.0)
	var delay := maxi(0, _flow_until - prior_deadline)
	if _defense_shared_start >= prior_deadline: _defense_shared_start += delay
	for key in _defense_animation_times:
		if int(_defense_animation_times[key]) >= prior_deadline: _defense_animation_times[key] += delay
	for panel in _defense_result_panels:
		if panel.started_ms >= prior_deadline: panel.started_ms += delay
		if int(panel.data.get("roll_started_ms", -1)) >= prior_deadline: panel.data.roll_started_ms += delay
	for key in _damage_card_times:
		if int(_damage_card_times[key]) >= prior_deadline: _damage_card_times[key] += delay
	for key in _damage_commit_started:
		if int(_damage_commit_started[key]) >= prior_deadline: _damage_commit_started[key] += delay
	for grid in _damage_grids:
		if grid.started_ms >= prior_deadline: grid.started_ms += delay
		if grid.removal_started_ms >= prior_deadline: grid.removal_started_ms += delay
	for item in _timed_buttons:
		if item.until >= prior_deadline: item.until += delay
	_flow_transition.present(_root)

func _interaction_deadline(full_review: bool) -> int:
	var deadline := _flow_until
	for panel in _defense_result_panels:
		deadline = maxi(deadline, panel.started_ms + ceili((DEFENSE_TIMING.total_seconds() if full_review else DEFENSE_TIMING.roll_seconds() + DEFENSE_TIMING.effects_seconds()) * 1000.0))
	for grid in _damage_grids:
		deadline = maxi(deadline, grid.started_ms + ceili((COMBAT_TIMING.reveal() + 0.25 + (COMBAT_TIMING.hold() if full_review else 0.0)) * 1000.0))
	return deadline

func _lock_button_until(button: Button, deadline: int) -> void:
	if button.disabled or deadline <= Time.get_ticks_msec(): return
	button.disabled = true
	_timed_buttons.append({"button": button, "until": deadline})

func _central_combat() -> bool:
	if not _selection_morph.is_empty(): return false
	return (_view.segment in ["defensive", "damage_resolution"] and not _director.has_beats()) or _director.peek().get("type") == "combat_damage"

func _combat_sides() -> Dictionary:
	if not _combat_columns.is_empty(): return _combat_columns
	var lanes := HBoxContainer.new(); lanes.name = "CombatLanes"; lanes.size_flags_vertical = Control.SIZE_EXPAND_FILL; lanes.add_theme_constant_override("separation", 24); _center.add_child(lanes)
	for target in ["blade", _focused_enemy]:
		var lane := preload("res://presentation/battle/combat_lane.gd").new(); lane.name = "Incoming_" + target; lanes.add_child(lane); _combat_columns[target] = lane.body
	if _multiple_enemies():
		for enemy in _enemy_ids(): _combat_columns[enemy] = _combat_columns[_focused_enemy]
	return _combat_columns

func _source_flow(panel: Control, source: Dictionary) -> void:
	panel.set_meta("flow_key", "source:" + str(source.get("id", "")))
	panel.set_meta("flow_origin", "ability:%s:%s" % [source.get("source_actor_id", ""), source.get("source_content_id", "")])
	if not _source_card_actions(str(source.get("id", ""))).is_empty():
		var target: Button = panel.highlight_card_target("Play %s against %s · %s" % [BattlePresentationCatalog.card(str(_selected_card.definition_id)).name, _actor_display_name(str(source.source_actor_id)), BattlePresentationCatalog.ability(str(source.source_content_id)).name])
		target.pressed.connect(_play_source_card.bind(str(source.id)))

func _layout_cinematic_root() -> void:
	if not is_instance_valid(_root): return
	# A shared design coordinate system keeps art and live hit targets aligned.
	# Extra aspect-ratio space is letterboxed rather than stretching the fighters.
	var factor := minf(size.x / 1920.0, size.y / 1080.0)
	_root.size = Vector2(1920, 1080)
	_root.scale = Vector2.ONE * factor
	_root.position = (size - _root.size * factor) * 0.5

func _layout_left_controls() -> void:
	if not is_instance_valid(_player_profile_dock) or not is_instance_valid(_ability_dock): return
	# Share one dice baseline and reserve room beneath either actor’s statuses.
	var player_bottom := _player_profile_dock.position.y + _player_profile_dock.get_combined_minimum_size().y
	var enemy_bottom := _enemy_profile_dock.position.y + _enemy_profile_dock.get_combined_minimum_size().y
	var top := maxf(245.0, maxf(player_bottom, enemy_bottom) + 48.0)
	var enemy_top := top
	_enemy_dice_dock.position.y = enemy_top
	_enemy_dice_dock.size.y = 100.0
	_player_dice_dock.position.y = top
	_player_dice_dock.size.y = 100.0
	_enemy_attack_dock.get_parent().position.y = enemy_top + 113.0
	_enemy_attack_dock.get_parent().size.y = maxf(80.0, 985.0 - enemy_top - 113.0)
	if _utility_panels.has("log"):
		var log_top := _enemy_dice_dock.position.y + _enemy_dice_dock.size.y + 18.0
		_place_cinematic(_utility_panels.log, Rect2(_enemy_dice_dock.position.x, log_top, _enemy_dice_dock.size.x, maxf(80.0, 985.0 - log_top)))
	_roll_dock.position.y = top + 113.0
	_action_footer.get_parent().position.y = top + 113.0
	var rail := _ability_dock.get_parent() as ScrollContainer
	rail.position.y = top + 203.0
	rail.size.y = maxf(80.0, 1055.0 - rail.position.y)

func _place_cinematic(control: Control, rect: Rect2) -> void:
	control.position = rect.position
	control.size = rect.size

func _cinematic_box(id: String, rect: Rect2) -> VBoxContainer:
	var box := VBoxContainer.new(); box.name = id; box.add_theme_constant_override("separation", 8)
	_root.add_child(box); _place_cinematic(box, rect)
	return box

func _cinematic_scroll_box(id: String, rect: Rect2) -> VBoxContainer:
	var scroll := ScrollContainer.new(); scroll.name = id; scroll.follow_focus = true
	_root.add_child(scroll); _place_cinematic(scroll, rect)
	var body := VBoxContainer.new(); body.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.add_child(body)
	return body

func set_encounter_visual(value: BattleEncounterVisual) -> void:
	encounter_visual = value
	if is_instance_valid(_root):
		_root.get_node("BattleScenery").display(_battlefield_visual(), _view.actors)

func _build_cinematic_utilities() -> void:
	_utility_panels.clear(); _utility_contents.clear()
	var utilities := HBoxContainer.new(); utilities.name = "BattleUtilities"; utilities.add_theme_constant_override("separation", 8)
	_root.add_child(utilities); _place_cinematic(utilities, Rect2(1460, 1000, 435, 54))
	var entries := [["enemy", "Enemy abilities"], ["log", "Log"], ["inspect", "Inspect"], ["settings", "⚙"]]
	if _history_tools_enabled(): entries.append(["history", "History"])
	for entry in entries:
		var id := str(entry[0])
		var button := Button.new(); button.text = str(entry[1]); button.custom_minimum_size.y = 48; button.size_flags_horizontal = Control.SIZE_EXPAND_FILL; button.add_theme_font_size_override("font_size", 16); button.toggle_mode = true; button.button_pressed = _open_utility == id
		for style_name in ["normal", "hover", "pressed", "disabled"]:
			button.add_theme_stylebox_override(style_name, CINEMATIC.panel(Color("292d25ef") if style_name in ["hover", "pressed"] else Color("11171def"), CINEMATIC.GOLD, 7))
		button.pressed.connect(func():
			_open_utility = "" if _open_utility == id else id
			for key in _utility_panels: _utility_panels[key].visible = key == _open_utility
			for sibling in utilities.get_children(): sibling.set_pressed_no_signal(sibling.get_meta("utility") == _open_utility)
		)
		button.set_meta("battle_utility", true); button.set_meta("utility", id); utilities.add_child(button); _inspect(button, "battle.utility." + id, "Open Settings" if id == "settings" else "Open " + str(entry[1]))
		var panel := PanelContainer.new(); panel.name = "Utility_" + id; panel.add_theme_stylebox_override("panel", CINEMATIC.panel(Color("10171df5")))
		_root.add_child(panel); _place_cinematic(panel, Rect2(520, 195, 880, 565)); panel.visible = _open_utility == id
		var frame := VBoxContainer.new(); panel.add_child(frame)
		var heading := HBoxContainer.new(); frame.add_child(heading)
		var title := Label.new(); title.text = "Combat log" if id == "log" else str(entry[1]); title.add_theme_font_size_override("font_size", 24 if id == "log" else 28); title.size_flags_horizontal = Control.SIZE_EXPAND_FILL; heading.add_child(title)
		var close := Button.new(); close.set_meta("battle_utility", true); close.text = "Close"; close.pressed.connect(func(): _open_utility = ""; panel.hide(); button.set_pressed_no_signal(false)); heading.add_child(close)
		var content := VBoxContainer.new(); content.size_flags_horizontal = Control.SIZE_EXPAND_FILL; content.add_theme_constant_override("separation", 12)
		if id == "log":
			# RichTextLabel owns scrolling and fills the side panel; nesting it in
			# another scroll container would leave most of the tall dock unused.
			content.size_flags_vertical = Control.SIZE_EXPAND_FILL; frame.add_child(content)
		else:
			var scroll := ScrollContainer.new(); scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; scroll.follow_focus = true; frame.add_child(scroll); scroll.add_child(content)
		_utility_panels[id] = panel; _utility_contents[id] = content
	_auto_pass_toggle = CheckBox.new(); _auto_pass_toggle.text = "Disable auto-pass"; _auto_pass_toggle.set_pressed_no_signal(_auto_pass_disabled)
	_auto_pass_toggle.tooltip_text = "Debug: pause before the final acknowledgement. Empty Offensive reactions, no-choice handoffs to the opponent, Effects, and status applications remain automatic. Real card choices always wait for you."
	_auto_pass_toggle.toggled.connect(_set_auto_pass_disabled); _utility_contents.settings.add_child(_auto_pass_toggle)
	_inspect(_auto_pass_toggle, "battle.disable_auto_pass", _auto_pass_toggle.tooltip_text)
	var hint := Label.new(); hint.text = "Hover cards, abilities, dice, or status counters for their details.\nVenom symbols: ✧ Fang · ⚗ Gland · ◉ Coil\nKept dice glow gold. Scroll your hand to see additional cards.\nDefense and damage results keep their existing review timing."; hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _utility_contents.settings.add_child(hint)
	var details := Label.new(); details.text = "Battle %s\nRound %d · %s\n%s\n\nCard and ability descriptions use this battle’s active rules, including upgrades." % [_view.battle_id, _view.round_number, _segment_name(_view.segment), _view.stage.replace("_", " ").capitalize()]; details.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _utility_contents.inspect.add_child(details)

func _build_header(parent: VBoxContainer) -> void:
	var bar := HBoxContainer.new(); bar.alignment = BoxContainer.ALIGNMENT_CENTER; bar.add_theme_constant_override("separation", 20); parent.add_child(bar)
	var display_segment := _view.segment
	var display_round := _view.round_number
	var display_stage := "attacks_selected" if _quiet_offensive_reaction() else _board_stage()
	if display_stage in ["defense_roll", "defense_reaction"]: display_stage = "defense"
	if _provoked_toxin_reaction(): display_stage = "provoked_toxins"
	if _director.has_beats():
		var beat: Dictionary = _director.peek()
		var beat_event: Dictionary = _as_dictionary(beat.get("event", {}))
		display_round = int(beat_event.get("round", display_round))
		var event_segment := str(beat.get("presentation_segment", ""))
		if event_segment.is_empty(): event_segment = str(beat_event.get("segment", beat_event.get("to", "")))
		if not event_segment.is_empty(): display_segment = event_segment
		if beat.get("type") == "combat_damage": display_stage = "damage_resolution"
		elif beat.get("type") == "defense_selected": display_stage = "defense_reveal"
		elif beat.get("type") == "effects_resolved": display_stage = "effects"
		elif beat.get("type") == "income_summary": display_stage = "income_results"
		elif beat.get("type") == "card_cleanse": display_stage = "card_played"
		elif beat.get("type") == "poison_conversion": display_stage = "poison_upgraded"
		elif beat.get("type") == "segment_entered": display_stage = "presentation"
	if not _selection_morph.is_empty(): display_segment = "offensive"; display_stage = "attack_selected"
	for pair in SEGMENTS:
		var label := Label.new(); label.text = "●\n%s" % pair[1]; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", 18)
		label.add_theme_color_override("font_color", Color("ffe3a0") if display_segment == pair[0] else Color("b4ab9b")); bar.add_child(label)
	var rule := HSeparator.new(); parent.add_child(rule)
	var round := Label.new(); round.text = "Round %d · %s" % [display_round, display_stage.replace("_", " ").capitalize()]; round.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; round.add_theme_color_override("font_color", CINEMATIC.INK); round.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); round.add_theme_font_size_override("font_size", 27); round.add_theme_stylebox_override("normal", CINEMATIC.paper()); round.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; parent.add_child(round); parent.move_child(round, 0)
	if learned_battle_mode:
		var policy_badge := Label.new()
		var policy_schema := str(_view.learned_policy.get("observation_schema", ""))
		var policy_label := "NEW V2" if policy_schema == "dice-and-destiny-observation-v2" else "OLD V1"
		policy_badge.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		policy_badge.text = "BRINE MASK · Keeps every 3" if _is_single_ability_opponent() else "LEARNED BATTLE · %s · HUMAN %s" % [policy_label, learned_human_seat.to_upper()]
		policy_badge.add_theme_color_override("font_color", Color("9de0ff"))
		policy_badge.tooltip_text = "Three rolls · 2 damage per 3 · Salt Veil rolls 1D6: half, rounded up · Brine Surge costs 1 energy for +1 damage" if _is_single_ability_opponent() else "Frozen policy %s · no training or fallback" % str(_view.learned_policy.get("model_id", "unknown"))
		_utility_contents.inspect.add_child(policy_badge)
		_inspect(policy_badge, "battle.learned_policy.badge", policy_badge.tooltip_text)
	if _snapshot_tools_enabled():
		var snapshots := Button.new(); snapshots.text = "DEV SNAPSHOTS"; snapshots.pressed.connect(_toggle_snapshot_panel); _utility_contents.inspect.add_child(snapshots)
		_inspect(snapshots, "battle.dev_snapshots.toggle", "Open the developer snapshot controls")
	if is_instance_valid(_transcript_panel):
		var transcript := Button.new(); transcript.text = "DEV TRANSCRIPT"; transcript.pressed.connect(_transcript_panel.toggle_panel); _utility_contents.inspect.add_child(transcript)
		_inspect(transcript, "battle.dev_transcript.toggle", "Open the read-only authority transcript panel")

func _profile_actor(actor_id: String) -> Dictionary:
	var actor: Dictionary = _view.actor(actor_id).duplicate(true)
	var damage_before := _director.pending_damage_actor_before(actor_id)
	if not damage_before.is_empty():
		for key in ["current_health", "energy_points", "deck_count", "hand_count", "discard_count", "removed_count", "statuses"]:
			if damage_before.has(key): actor[key] = damage_before[key]
		return actor
	var before: Dictionary = _director.pending_effects_actor_before(actor_id)
	if before.is_empty(): return actor
	actor["current_health"] = before.get("health", actor.get("current_health", 0))
	actor["energy_points"] = before.get("energy", actor.get("energy_points", 0))
	for key in ["deck_count", "hand_count", "discard_count", "removed_count", "statuses"]:
		if before.has(key): actor[key] = before[key]
	return actor

func _build_player_column(_parent: HBoxContainer) -> void:
	var column := _player_profile_dock
	var profile := ActorProfile.new(); column.add_child(profile); profile.display("blade", _profile_actor("blade"), true); _actor_profiles["blade"] = profile
	if learned_battle_mode:
		profile.title.text = _actor_display_name("blade")
		profile.title.tooltip_text = "You · %s" % learned_human_seat.to_upper()
	var income := _income_actor_data("blade")
	if not income.is_empty(): profile.prepare_income(income)
	else:
		var upcoming_income := _upcoming_income_actor_data("blade")
		if not upcoming_income.is_empty(): profile.prepare_before_income(upcoming_income)
	if _director.peek().get("type") == "effects_resolved": return
	var dice := BattleDiceTray.new(); _player_dice_dock.add_child(dice)
	var rolled := _view.rolled_dice("blade"); var actor := _view.actor("blade")
	var history: Array = actor.get("roll_history", [])
	if history.size() != _selection_roll_number:
		_selected_indices.clear()
		if not history.is_empty():
			for index in history[-1].get("kept_indices", []): _selected_indices.append(int(index))
		_selected_indices.sort()
		_selection_roll_number = history.size()
	var kept: Array = _selected_indices.duplicate()
	var planning := _view.stage == "planning" and (_view.allowed("planning_keep") or _view.allowed("planning_reroll")) and not _submitting and not _history_review
	dice.display(rolled, kept, planning, "", "battle.die.blade")
	if _player_roll_active():
		dice.animate_roll(_player_roll_feedback.indices, int(_player_roll_feedback.started_ms), int(_player_roll_feedback.duration_ms))
	dice.selection_changed.connect(func(indices): _selected_indices = indices)
	var controls := HBoxContainer.new(); controls.alignment = BoxContainer.ALIGNMENT_CENTER; _roll_dock.add_child(controls)
	if _view.segment == "offensive":
		# Keep the shared roll control visible even when the authority has closed
		# planning, so a skipped offense cannot look like a missing character UI.
		var initial_roll := _view.rolls_used("blade") == 0
		var command := "planning_roll" if initial_roll else "planning_reroll"
		var roll := Button.new(); roll.custom_minimum_size = Vector2(248, 72)
		roll.text = "Roll 5 Dice" if initial_roll else "Reroll Unkept"
		var roll_font := CINEMATIC.roll_control_font()
		roll.add_theme_font_override("font", roll_font)
		CINEMATIC.paper_button(roll)
		for style_name in ["normal", "hover", "pressed", "disabled"]: roll.get_theme_stylebox(style_name).content_margin_bottom = 29
		var remaining := Label.new(); remaining.text = "%d / %d" % [maxi(0, _view.max_rolls("blade") - _view.rolls_used("blade")), _view.max_rolls("blade")]; remaining.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; remaining.add_theme_font_size_override("font_size", 17); remaining.add_theme_color_override("font_color", CINEMATIC.INK); remaining.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); remaining.mouse_filter = Control.MOUSE_FILTER_IGNORE
		remaining.add_theme_font_override("font", roll_font)
		roll.add_child(remaining); remaining.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE); remaining.offset_top = -30; remaining.offset_bottom = -7
		roll.disabled = not _view.allowed(command) or _submitting or _director.has_beats() or _history_review or _view.rolls_used("blade") >= _view.max_rolls("blade")
		roll.tooltip_text = _offensive_roll_hint()
		roll.pressed.connect(func():
			if initial_roll: _send(BattleCommandBuilder.planning_roll(_view.battle_id, "blade", _pending()))
			else: _reroll_unkept()
		)
		controls.add_child(roll)
		_inspect(roll, "battle.command.%s" % command, roll.tooltip_text)
		if _view.stage == "offensive_reaction" and initial_roll: _show_skipped_offense_hint()

func _show_skipped_offense_hint() -> void:
	if not is_instance_valid(_player_profile_dock) or _player_profile_dock.has_node("SkippedOffenseExplanation"): return
	var explanation := Label.new(); explanation.name = "SkippedOffenseExplanation"
	explanation.text = "Your offensive ability was skipped.\nYou can roll again next round."
	explanation.add_theme_font_size_override("font_size", 17)
	explanation.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_player_profile_dock.add_child(explanation)

func _offensive_roll_hint() -> String:
	if _view.stage == "offensive_reaction": return "Offensive planning has ended. Your next offensive roll is next round."
	if _view.stage != "planning": return "Finish the current response before resuming offensive planning."
	if _model_thinking or not _view.allowed("planning_roll"): return "Waiting for the current decision to finish."
	if _view.rolls_used("blade") >= _view.max_rolls("blade"): return "All offensive rolls have been used. Choose an ability from your dice."
	return "Roll your combat dice, then choose an offensive ability. Playing a setup card does not end planning."

func _build_enemy_column(_parent: HBoxContainer) -> void:
	var column := _enemy_profile_dock
	var profile := ActorProfile.new(); column.add_child(profile); profile.display(_focused_enemy, _profile_actor(_focused_enemy), false); _actor_profiles[_focused_enemy] = profile
	if learned_battle_mode or _multiple_enemies():
		profile.title.text = _actor_display_name(_focused_enemy).trim_prefix("Learned ")
	if learned_battle_mode:
		profile.title.tooltip_text = "Keeps every 3 · one attack · one defense" if _is_single_ability_opponent() else "Learned Policy"
	var income := _income_actor_data(_focused_enemy)
	if not income.is_empty(): profile.prepare_income(income)
	else:
		var upcoming_income := _upcoming_income_actor_data(_focused_enemy)
		if not upcoming_income.is_empty(): profile.prepare_before_income(upcoming_income)
	if _director.peek().get("type") != "effects_resolved":
		var enemy_dice := _view.rolled_dice(_focused_enemy); var enemy_reveal := _view.offensive_reveal(_focused_enemy); var enemy_caption := "OPPONENT DICE" if learned_battle_mode else "ENEMY DICE"
		if not enemy_dice.is_empty(): enemy_caption += " · Rolls %d" % int(enemy_reveal.get("rolls_used", 0)) if learned_battle_mode else " · Simulated rolls %d" % int(enemy_reveal.get("simulated_rolls", 0))
		var dice := BattleDiceTray.new(); _enemy_dice_dock.add_child(dice); dice.display(enemy_dice, [], false, enemy_caption, "battle.die.goblin")
		# Keep the dice themselves on the player baseline; the caption sits below.
		dice.move_child(dice._caption, dice.get_child_count() - 1)
	_log = RichTextLabel.new(); _log.size_flags_vertical = Control.SIZE_EXPAND_FILL; _log.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _log.fit_content = false; _log.selection_enabled = true; _utility_contents.log.add_child(_log)
	_log.scroll_following = true
	_log.text = _view.combat_log.text()

func _build_center() -> void:
	if not _selection_morph.is_empty():
		var tile := BattleAbilityTile.new(); _ability_dock.add_child(tile)
		var id := str(_selection_morph.ability_id)
		tile.configure(id, false, true, false, _view.actor("blade")); tile.cinematic_compact(); tile.show_selected_attack(str(_selection_morph.get("text", "")))
		tile.set_meta("flow_key", "ability:blade:" + id)
		_build_hand()
		return
	if _quiet_offensive_reaction():
		_build_offensive(); _build_hand()
		return
	if _model_error:
		_build_model_error_recovery()
		return
	if _director.has_beats():
		if _history_review: _build_history_review_controls()
		_build_presentation_beat()
		return
	if _view.is_complete():
		if _history_review: _build_history_review_controls()
		_build_completion()
		return
	if _history_review: _build_history_review_controls()
	var prompt := Label.new(); prompt.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; prompt.add_theme_font_size_override("font_size", 21); prompt.visible = _view.segment not in ["offensive", "defensive", "damage_resolution"]; prompt.text = _board_stage().replace("_", " ").capitalize() if _inline_status_application() else _prompt_text(); _center.add_child(prompt)
	_build_reaction_card_feedback()
	if _inline_status_application():
		if _view.segment == "defensive": _build_defensive()
		elif _view.segment == "offensive": _build_offensive()
		else: _build_damage()
	elif _view.stage == "venom_status_reaction": _build_venom_status()
	elif _view.stage in ["status_roll", "status_roll_reaction"]: _build_effects()
	elif _view.stage == "status_damage_reaction": _build_damage()
	elif _view.segment == "offensive": _build_offensive()
	elif _view.segment == "defensive": _build_defensive()
	elif _view.segment == "damage_resolution" or "damage" in _view.stage: _build_damage()
	elif _view.segment == "ongoing_effects": _build_effects()
	elif _view.segment == "income": _build_income()
	_build_card_target_choices()
	_build_hand()
	if _selected_card.get("source_targeting", false): return
	var pass_row := _action_footer
	var planning_pass_label := ("Pass All Remaining" if _multiple_enemies() else "Pass Defense") if _view.segment == "defensive" else "Skip Offensive Ability"
	_add_action(pass_row, planning_pass_label, "planning_pass", _pass_planning)
	var pass_label := "Continue" if _defense_final_review() else "Continue Without Playing a Card" if _view.stage == "status_roll_reaction" else "Pass / Acknowledge"
	if (_inline_status_application() or _provoked_toxin_reaction()) and _view.legal_actions.size() == 1 and _view.legal_actions[0].get("type") == "pass": return
	_add_action(pass_row, pass_label, "pass", func(): _send(BattleCommandBuilder.pass_command(_view.battle_id, "blade", _pending())))

func _build_model_error_recovery() -> void:
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var title := Label.new(); title.text = "LEARNED OPPONENT NEEDS ATTENTION"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 26); title.add_theme_color_override("font_color", Color("ff8a78")); _center.add_child(title)
	var detail := Label.new(); detail.text = "No fallback action was submitted. Retry the same authoritative decision or return to the mode menu."; detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _center.add_child(detail)
	var actions := HBoxContainer.new(); actions.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(actions)
	var retry := Button.new(); retry.text = "Retry Learned Decision"; retry.pressed.connect(_retry_model_decision); actions.add_child(retry); _inspect(retry, "battle.learned_policy.retry", "Retry inference for the unchanged current authority decision")
	var leave := Button.new(); leave.text = "Return to Mode Menu"; leave.pressed.connect(_return_to_mode_menu); actions.add_child(leave); _inspect(leave, "battle.learned_policy.return_to_menu", "Leave this stopped learned battle without submitting a fallback")
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)

func _retry_model_decision() -> void:
	if _model_thinking or not learned_battle_mode:
		return
	_model_error = false
	_model_timeout_warning = false
	_error_message = ""
	_schedule_model_if_needed({"learned_policy": _view.learned_policy})
	_render()

func _build_offensive() -> void:
	_build_reaction_notice()
	_build_ability_row("ENEMY ABILITIES", _view.actor(_focused_enemy).get("offensive_abilities", []), _focused_enemy)
	var public_plan := _enemy_plan_text()
	if not public_plan.is_empty():
		var plan := Label.new(); plan.text = public_plan; plan.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; plan.add_theme_color_override("font_color", Color("ef9a55")); _center.add_child(plan)
	_build_ability_row(_actor_display_name("blade").to_upper() + " ABILITIES", _view.actor("blade").get("offensive_abilities", []), "blade")
	if _selected_card_selector() == "selected_die":
		if _view.stage == "offensive_reaction": _build_enemy_die_targets()
		elif _view.stage == "blind_reaction":
			var tip := Button.new(); tip.text = "Tip Blind die to face 5"; tip.disabled = _history_review; tip.pressed.connect(_play_blind_tip); _center.add_child(tip); _inspect(tip, "battle.tip_target.blind", "Use Tip It on the current blind-roll die")

func _build_defensive() -> void:
	if _view.stage == "defense_roll":
		_build_automatic_defense_roll()
		return
	if _board_stage() == "defense_reaction":
		_build_compact_defense_results()
		return
	_build_incoming_selection()
	if not _multiple_enemies() and not _selected_source.is_empty():
		var abilities: Array = _as_array(_view.actor("blade").get("defensive_abilities", []))
		_build_ability_row("DEFENSIVE ABILITIES", abilities, "blade")
	if _view.stage in ["defense_roll", "defense_reaction"]: _build_defense_mat()

func _defense_actions(ability_id: String) -> Array:
	var actions: Array = []
	if _selected_source.is_empty(): return actions
	for action in _view.legal_actions:
		var payload: Dictionary = action.get("payload", {})
		if action.get("type") == "planning_select_ability" and payload.get("ability_id") == ability_id and _selected_source in payload.get("target_ids", []): actions.append(action)
	return actions

func _build_incoming_selection() -> void:
	var sides := _combat_sides()
	for source in _view.damage_sources:
		if not _source_in_focus(source): continue
		var target := str(source.get("target_actor_id", ""))
		if not sides.has(target): continue
		var data := _compact_defense_data(target, _status_counts(), source)
		var handled := _source_handled(str(source.get("id", "")))
		var start := Time.get_ticks_msec()
		if handled:
			data["note"] = "Defense complete"; data["read_only"] = true
			start -= ceili((DEFENSE_TIMING.total_seconds() + 1.0) * 1000.0)
		else:
			var planned: Dictionary = _view.raw_snapshot.get("defense_plans", {}).get(str(source.get("id", "")), {})
			var prompt: String = "Queued: " + BattlePresentationCatalog.ability(str(planned.get("ability_id", ""))).name if not planned.is_empty() else "Needs defense" if _multiple_enemies() else "Select this attack to defend"
			data.merge({"selection_only": true, "dice": [], "gains": [], "prevented": 0, "after": data.before, "ability_name": "", "note": prompt if target == viewer_actor_id else "Opponent defending", "read_only": target != viewer_actor_id or _submitting or _history_review}, true)
		var panel = DEFENSE_RESULT.new(); sides[target].add_child(panel); panel.configure(data, start, true); _source_flow(panel, source)
		panel.source_selected.connect(func(id: String): _selected_source = id; _render())
		if _multiple_enemies() and target == viewer_actor_id and not _defense_source_chosen(str(source.id)):
			_build_source_defense_choices(panel, str(source.id))
		elif str(source.get("id", "")) == _selected_source: panel.self_modulate = Color("fff0bd")

func _build_compact_defense_results() -> void:
	var sides := _combat_sides()
	var together := _multiple_enemies() and _board_stage() == "defense_reaction"
	var round_key := "%s:%d" % [_view.battle_id, _view.round_number]
	if together and _defense_shared_round != round_key:
		_defense_shared_round = round_key
		_defense_shared_start = maxi(Time.get_ticks_msec(), _flow_until)
		_defense_shared_first_wave = _defense_review_key()
	var counts := {}
	for actor_id in _view.actors:
		counts[actor_id] = {}
		for status in _view.actor(actor_id).get("statuses", []):
			counts[actor_id][str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
	var sources := _view.damage_sources.duplicate()
	for source in sources:
		if not _source_in_focus(source): continue
		var actor_id := str(source.get("target_actor_id", ""))
		if not sides.has(actor_id): continue
		var data := _compact_defense_data(actor_id, counts, source)
		if data.is_empty(): continue
		data["awaiting_roll"] = _view.stage == "defense_roll" and not _source_handled(str(source.id)) and not _history_review and not _history_replay
		var queued: bool = _view.raw_snapshot.get("defense_plans", {}).has(str(source.id))
		data["effects_pending"] = queued
		if queued: data["note"] = "Effects follow the first defense"
		var key := "%s:%s" % [_defense_review_key(), str(source.get("id", ""))]
		if not _defense_animation_times.has(key):
			if _defense_animation_times.size() > 64: _defense_animation_times.clear()
			var start_effects := maxi(Time.get_ticks_msec(), _flow_until)
			if together:
				start_effects = _defense_shared_start if _defense_shared_first_wave == _defense_review_key() else start_effects - ceili(DEFENSE_TIMING.roll_seconds() * 1000.0)
			_defense_animation_times[key] = start_effects
		var start := int(_defense_animation_times[key])
		if together: data["roll_started_ms"] = _defense_shared_start
		if _history_review or _history_replay or _source_handled(str(source.get("id", ""))):
			start = Time.get_ticks_msec() - ceili((DEFENSE_TIMING.total_seconds() + 1.0) * 1000.0)
			data["roll_started_ms"] = start
		var panel = DEFENSE_RESULT.new(); sides[actor_id].add_child(panel)
		panel.damage_settled.connect(func(): _refresh_defense_summaries.call_deferred())
		panel.configure(data, start, true)
		_source_flow(panel, source)
		panel.source_selected.connect(func(id: String): _selected_source = id; _render())
		_inspect(panel, "battle.defense_result." + actor_id, "Live defense results; rules are available on the ability name")
		_defense_result_panels.append(panel)

func _defense_summary_ready(source_id: String) -> bool:
	if _history_review or _history_replay: return true
	var key := "%s:%s" % [_defense_review_key(), source_id]
	if not _defense_animation_times.has(key): return false
	return (Time.get_ticks_msec() - int(_defense_animation_times[key])) / 1000.0 >= DEFENSE_TIMING.roll_seconds() + DEFENSE_TIMING.effects_seconds() * 0.65

func _refresh_defense_summaries() -> void:
	for actor_id in _selected_attack_tiles:
		var tile: BattleAbilityTile = _selected_attack_tiles[actor_id]
		var attack := _selected_attack(actor_id)
		if is_instance_valid(tile) and not attack.is_empty():
			tile.update_selected_attack_summary(str(attack.text))

func _compact_defense_data(actor_id: String, counts: Dictionary, shown_source: Dictionary = {}) -> Dictionary:
	var selection := _as_dictionary(_view.defense_selections.get(actor_id, {}))
	var roll := _as_dictionary(_view.defense_rolls.get(actor_id, {}))
	if selection.is_empty(): selection = roll
	var source := {}
	for candidate in _view.damage_sources:
		if str(candidate.get("id", "")) == str(selection.get("source_id", "")) and str(candidate.get("target_actor_id", "")) == actor_id: source = candidate; break
	if source.is_empty(): source = _incoming_source_for_target(actor_id)
	if not shown_source.is_empty():
		source = shown_source
		if str(selection.get("source_id", "")) != str(source.get("id", "")): selection = {}; roll = {}
	if source.is_empty(): return {}
	var historical := _as_dictionary(_view.raw_snapshot.get("defense_history", {}).get(str(source.get("id", "")), {}))
	if not historical.is_empty(): selection = historical; roll = {}
	var queued: Dictionary = _view.raw_snapshot.get("defense_plans", {}).get(str(source.get("id", "")), {})
	if selection.is_empty() and not queued.is_empty(): selection = queued; roll = {}
	var ability_id := str(selection.get("ability_id", ""))
	var ability := BattlePresentationCatalog.ability(ability_id)
	var enemy := _focused_enemy if actor_id == "blade" else "blade"
	var attacker := str(source.get("source_actor_id", enemy))
	var before := maxi(0, int(source.get("base_amount", 0)) - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))
	var pending := before
	var finalized := bool(selection.get("finalized", false))
	# Completed gains are already in the authoritative status counts. Queued
	# rolls are only previews. Neither may inflate another source's live gains.
	if finalized or not queued.is_empty(): counts = counts.duplicate(true)
	if finalized: before = maxi(0, int(source.get("base_amount", 0)) - int(source.get("reaction_prevention", 0)) - (2 if selection.get("catalyst_paid", false) else 0)); pending = before
	var prevented := 0
	var faces: Array = selection.get("rolled_faces", roll.get("rolled_faces", [int(selection.get("rolled_face", roll.get("face", 0)))]))
	var operations := _as_array(_view.content_definition("abilities", ability_id).get("resolution", {}).get("operations", []))
	var dice: Array = []; var gains: Array = []
	var die_id := "standard_d6"
	for op in operations:
		if op.get("type") == "roll_dice": die_id = str(op.get("dice_id", die_id))
	# Fixed defenses resolve once without a rolled face.
	if faces.is_empty() and not operations.any(func(op): return op.get("type") == "roll_dice"):
		faces = [0]
	for face in faces:
		var benefits: Array[String] = []
		var die_prevented := 0
		var gain_start := gains.size()
		for op in _resolved_operations(operations, int(face)):
			match str(op.get("type", "")):
				"prevent_damage":
					var amount := _configured_amount(op.get("amount", 0), int(face)); prevented += amount; die_prevented += amount; pending = maxi(0, pending - amount); benefits.append("Prevent %d" % amount)
				"scale_damage":
					var after := floori(float(pending * int(op.get("numerator", 1))) / maxi(1, int(op.get("denominator", 1))))
					prevented += pending - after; die_prevented += pending - after; benefits.append("Prevent %d" % (pending - after)); pending = after
				"apply_status", "apply_incubation", "incubation_or_poison":
					var target := actor_id if op.get("target") == "self" else attacker
					var status_id := str(op.get("status_id", "incubation"))
					if op.get("type") == "incubation_or_poison" and (int(counts[target].get("poison", 0)) == 0 or int(counts[target].get("incubation", 0)) > 0): status_id = "poison"
					var amount := maxi(1, int(op.get("stack_count", 1)))
					var added := _defense_status_gain(gains, counts, target, status_id, amount)
					benefits.append(("+%d " % added if added > 0 else "At cap · ") + str(BattlePresentationCatalog.status(status_id).name))
		if int(face) > 0:
			for gain_index in range(gain_start, gains.size()): gains[gain_index]["die_index"] = dice.size()
			dice.append({"face": int(face), "benefit": "\n".join(benefits), "prevention": die_prevented})
	if int(source.get("scale_denominator", 0)) > 0:
		var denominator := int(source.scale_denominator)
		var numerator := int(source.get("scale_numerator", 1))
		before = floori(float(before * numerator) / denominator)
		pending = floori(float(pending * numerator) / denominator)
	# An attack's status applications are independent of its blocked damage.
	var reveal := _view.offensive_reveal(attacker)
	var attack_statuses := _pending_attack_status_text(_as_array(reveal.get("outcome", {}).get("status_applications", [])), actor_id)
	var note := ""
	if bool(selection.get("catalyst_paid", false)): note = "Catalyst spent · 2 already prevented"
	if int(source.get("reaction_prevention", 0)) > 0: note += (" · " if not note.is_empty() else "") + "%d prevented by cards" % int(source.reaction_prevention)
	if finalized or not queued.is_empty(): gains.clear()
	return {"source_id": str(source.get("id", "")), "read_only": finalized or _history_review or _submitting, "actor_id": actor_id, "actor_name": _actor_display_name(actor_id), "stacked": _multiple_enemies(), "attack_name": (_actor_display_name(attacker) + " · " if _multiple_enemies() and actor_id == viewer_actor_id else "") + BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))).name, "before": before, "after": pending, "prevented": prevented, "ability_name": ability.name if not ability_id.is_empty() else ("Queued — " + BattlePresentationCatalog.ability(str(_view.raw_snapshot.get("defense_plans", {}).get(str(source.get("id", "")), {}).get("ability_id", ""))).name if _view.raw_snapshot.get("defense_plans", {}).has(str(source.get("id", ""))) else "Passed" if _source_handled(str(source.get("id", ""))) else "Needs defense"), "rules": ability.text, "dice": dice, "die_id": die_id, "gains": gains, "attack_statuses": attack_statuses, "note": note}

func _pending_attack_status_text(applications: Array, target: String) -> String:
	# Ability and card modifiers can contribute the same status to one attack.
	# Summarize their total without changing the authority's individual effects.
	var totals := {}
	for value in applications:
		var application := _as_dictionary(value)
		if str(application.get("target_actor_id", target)) != target: continue
		var status_id := str(application.get("status_id", ""))
		if status_id.is_empty(): continue
		totals[status_id] = int(totals.get(status_id, 0)) + int(application.get("stacks", 1))
	var lines: Array[String] = []
	for status_id in totals:
		var status := BattlePresentationCatalog.status(str(status_id))
		lines.append("%s %s ×%d pending" % [status.glyph, status.name, int(totals[status_id])])
	return "\n".join(lines)

func _defense_status_gain(gains: Array, counts: Dictionary, target: String, status_id: String, amount: int) -> int:
	if not counts.has(target): counts[target] = {}
	var before := int(counts[target].get(status_id, 0))
	var cap := int(_view.content_definition("statuses", status_id).get("stacking", {}).get("stack_limit", 99))
	var added := mini(amount, maxi(0, cap - before))
	if added > 0:
		counts[target][status_id] = before + added
		gains.append({"target": target, "status_id": status_id, "amount": added, "after": before + added})
	if status_id == "poison" and added < amount and int(counts[target].get("incubation", 0)) == 0:
		_defense_status_gain(gains, counts, target, "incubation", 1)
	return added

func _start_defense_status_flights(generation: int) -> void:
	if generation != _income_animation_generation: return
	for panel in _defense_result_panels:
		for index in panel.data.gains.size():
			var gain: Dictionary = panel.data.gains[index]
			var profile: ActorProfile = _actor_profiles.get(str(gain.target))
			if profile == null: continue
			var flight = DEFENSE_STATUS_FLIGHT.new(); _root.add_child(flight)
			flight.configure(panel.gain_origins[index], profile, gain, panel.started_ms + index * 120)

func _defense_effects_finished_at() -> int:
	var finished := 0
	for panel in _defense_result_panels:
		if panel.data.get("effects_pending", false) or _source_handled(str(panel.data.get("source_id", ""))): continue
		var effects_ms := ceili(DEFENSE_TIMING.effects_seconds() * 1000.0)
		# Status flights are staggered by 120ms and land at 85% of the effect.
		var flights_ms := ceili(effects_ms * 0.85) + maxi(0, panel.data.gains.size() - 1) * 120
		finished = maxi(finished, panel.started_ms + ceili(DEFENSE_TIMING.roll_seconds() * 1000.0) + maxi(effects_ms, flights_ms))
	return finished

func _build_defense_mat() -> void:
	var revealed := _view.stage == "defense_reaction"
	var row := HBoxContainer.new(); row.size_flags_horizontal = Control.SIZE_EXPAND_FILL; row.add_theme_constant_override("separation", 30); _center.add_child(row)
	_inspect(row, "battle.defense_mat", "Fixed defense roll and reveal area")
	for actor_id in ["blade", _focused_enemy]:
		var panel := VBoxContainer.new(); panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; panel.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_child(panel)
		if actor_id != viewer_actor_id and not revealed:
			_inspect(panel, "battle.defense_hidden.goblin", "Reserved space for the hidden enemy defense")
			continue
		_build_defense_panel(panel, actor_id, revealed)

func _build_defense_panel(parent: VBoxContainer, actor_id: String, revealed: bool) -> void:
	var actor_name := _actor_display_name(actor_id).to_upper()
	var heading := Label.new(); heading.text = "%s DEFENSE" % actor_name; heading.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; heading.add_theme_font_size_override("font_size", 18); parent.add_child(heading)
	var source := _incoming_source_for_target(actor_id)
	if source.is_empty():
		var none_needed := Label.new(); none_needed.text = "No incoming attack\nNo defense needed"; none_needed.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; none_needed.add_theme_color_override("font_color", Color("9299a5")); parent.add_child(none_needed); return
	var attack := BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))); var base := int(source.get("base_amount", 0))
	var selection: Dictionary = _as_dictionary(_view.defense_selections.get(actor_id, {})); var roll: Dictionary = _as_dictionary(_view.defense_rolls.get(actor_id, {}))
	if selection.is_empty() and not roll.is_empty(): selection = roll
	if selection.is_empty():
		if not revealed: return
		var no_defense := Label.new(); no_defense.text = "NO DEFENSE USED"; no_defense.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; no_defense.add_theme_font_size_override("font_size", 20); no_defense.add_theme_color_override("font_color", Color("e5a07e")); parent.add_child(no_defense); _inspect(no_defense, "battle.defense_none.%s" % actor_id, "%s used no defense" % actor_name.capitalize())
		var unchanged := Label.new(); unchanged.text = "%s: %d → %d pending" % [attack.name, base, base]; unchanged.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; parent.add_child(unchanged); return
	var ability_id := str(selection.get("ability_id", roll.get("ability_id", "")))
	if ability_id.is_empty(): return
	var ability := BattlePresentationCatalog.ability(ability_id)
	var detail := Label.new(); detail.custom_minimum_size = Vector2(250, 96); detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	detail.text = "%s\n%s\n%s" % [ability.name, ability.recipe, ability.text]
	detail.text += "\nAgainst: %s" % attack.name
	detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; parent.add_child(detail); _inspect(detail, "battle.defense_selected.%s" % actor_id, detail.text)
	var face := int(roll.get("face", selection.get("rolled_face", 0)))
	var dice_row := HBoxContainer.new()
	dice_row.alignment = BoxContainer.ALIGNMENT_CENTER
	dice_row.add_theme_constant_override("separation", 12)
	parent.add_child(dice_row)
	var die := Button.new(); die.custom_minimum_size = Vector2(120, 105); die.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_style_defense_die(die)
	if face > 0:
		var die_data := _as_dictionary(roll.get("die", {})); var die_id := str(die_data.get("die_id", "standard_d6"))
		if ability_id in ["shedskin", "barbed_mantle"]: die_id = "venom_d6"
		die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(die_id, face), face]
		die.tooltip_text = "%s defensive die: face %d, %s." % [actor_name.capitalize(), face, BattlePresentationCatalog.symbol_name_for_die_face(die_id, face)]
		die.disabled = true; die.add_theme_font_size_override("font_size", 26); dice_row.add_child(die); _inspect(die, "battle.defense_die.%s" % actor_id, die.tooltip_text)
	elif revealed:
		var no_die := Label.new(); no_die.custom_minimum_size = Vector2(120, 105); no_die.text = "NO DIE ROLL"; no_die.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; no_die.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; no_die.add_theme_font_size_override("font_size", 18); no_die.add_theme_color_override("font_color", Color("b8bfca")); dice_row.add_child(no_die)
	else:
		die.tooltip_text = "Click this blank player defense die to roll it."
		die.disabled = _submitting or _director.has_beats() or _history_review
		die.pressed.connect(func(): _send(BattleCommandBuilder.roll_dice(_view.battle_id, "blade", _pending())))
		dice_row.add_child(die); _inspect(die, "battle.defense_die.blade.pending", die.tooltip_text)
		var instruction := Label.new(); instruction.text = "Click the blank player die to roll"; instruction.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; instruction.add_theme_color_override("font_color", Color("b8bfca")); parent.add_child(instruction)
		return
	if not revealed: return
	var faces: Array = selection.get("rolled_faces", roll.get("rolled_faces", [face]))
	for extra_index in range(1, faces.size()):
		var extra := Button.new()
		var extra_face := int(faces[extra_index])
		extra.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face("venom_d6", extra_face), extra_face]
		extra.disabled = true
		_style_defense_die(extra)
		extra.custom_minimum_size = Vector2(120, 105)
		extra.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
		extra.add_theme_font_size_override("font_size", 26)
		dice_row.add_child(extra)
		_inspect(extra, "battle.defense_die.%s.%d" % [actor_id, extra_index], "Second defense die")
	var preview := _defense_preview(ability_id, base, face)
	if ability_id == "shedskin":
		var prevention := 2 if selection.get("catalyst_paid", roll.get("catalyst_paid", false)) else 0
		for shown_face in faces:
			if int(shown_face) in [1, 2, 3]: prevention += 1
		preview = {"rolled": true, "prevented": mini(base, prevention), "pending": maxi(0, base - prevention)}
	var chosen := Label.new(); chosen.text = "%s · %d block" % [ability.name, int(preview.get("prevented", 0))] if bool(preview.get("rolled", false)) else ability.name; chosen.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; parent.add_child(chosen)
	var effect := Label.new(); effect.text = str(ability.get("text", "Defense selected")); effect.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; effect.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; effect.custom_minimum_size.x = 300; parent.add_child(effect)
	var pending := int(preview.get("pending", base)); var prevented := int(preview.get("prevented", 0))
	var outcome := Label.new(); outcome.text = "%s: %d − %d = %d pending" % [attack.name, base, prevented, pending] if bool(preview.get("rolled", false)) else "%s: %d → %d pending" % [attack.name, base, pending]; outcome.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; parent.add_child(outcome)
	_inspect(parent, "battle.defense_result.%s" % actor_id, "Revealed defense result for %s" % actor_id)

func _style_defense_die(die: Button) -> void:
	var normal_style := StyleBoxFlat.new(); normal_style.bg_color = Color("111722f2"); normal_style.border_color = Color("77879aff"); normal_style.set_border_width_all(2); normal_style.set_corner_radius_all(12); normal_style.shadow_color = Color(0, 0, 0, 0.75); normal_style.shadow_size = 8
	var hover_style := normal_style.duplicate(); hover_style.bg_color = Color("1b2431ff"); hover_style.border_color = Color("f0bc58ff")
	var pressed_style := hover_style.duplicate(); pressed_style.bg_color = Color("0b1018ff")
	die.add_theme_stylebox_override("normal", normal_style); die.add_theme_stylebox_override("hover", hover_style); die.add_theme_stylebox_override("pressed", pressed_style); die.add_theme_stylebox_override("disabled", normal_style.duplicate()); die.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND

func _build_damage() -> void:
	_build_damage_feedback()
	_build_damage_lanes(_view.settled_damage)

func _build_damage_lanes(batch: Dictionary, committed: bool = false) -> void:
	var sides := _combat_sides()
	var all_sources: Array = _as_array(batch.get("sources", _view.damage_sources))
	var sources := _display_damage_sources(all_sources)
	var removals: Array = _as_array(batch.get("removals", []))
	var cards_by_source := _damage_cards_by_source(all_sources, removals)
	var batch_key := _view.battle_id + ":" + str(batch.get("id", batch.get("batch_id", "")))
	if not _damage_card_times.has(batch_key): _damage_card_times[batch_key] = maxi(Time.get_ticks_msec(), _flow_until)
	var claimed := {}
	for source in sources:
		var target := str(source.get("target_actor_id", ""))
		if not sides.has(target): continue
		var amount := int(source.get("final_amount", maxi(0, int(source.get("base_amount", 0)) - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))))
		var applications := _as_array(source.get("status_applications", batch.get("status_applications", [])))
		var attack_statuses := _pending_attack_status_text(applications, target)
		var data := {"source_id": str(source.get("id", "")), "stacked": _multiple_enemies(), "actor_id": target, "actor_name": _actor_display_name(target), "attack_name": _actor_display_name(str(source.get("source_actor_id", _focused_enemy))) if target == viewer_actor_id and _multiple_enemies() else BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))).name, "before": amount, "after": amount, "prevented": 0, "dice": [], "gains": [], "die_id": "standard_d6", "ability_name": "", "rules": "", "attack_statuses": attack_statuses, "note": "", "read_only": committed or _submitting or _history_review, "selection_only": true}
		var panel = DEFENSE_RESULT.new(); panel.name = "DamageSource_" + str(source.get("id", "")); sides[target].add_child(panel); panel.configure(data, 0, true); _source_flow(panel, source)
		panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
		if _multiple_enemies() and not committed and target == viewer_actor_id and str(source.get("id", "")) == _selected_source: panel.self_modulate = Color("fff0bd")

		panel.source_selected.connect(func(id: String): _selected_source = id; _render())
		var grid := preload("res://presentation/battle/combat_card_reveal.gd").new(); grid.name = "DamageCards_" + str(source.get("id", "")); panel._body.add_child(grid); grid.set_meta("flow_part", "cards")
		grid.started_ms = int(_damage_card_times[batch_key]) if not _history_review else Time.get_ticks_msec() - 10000; _damage_grids.append(grid)
		for card_data in cards_by_source.get(str(source.get("id", "")), []):
			if card_data.get("target_actor_id") != target or not card_data.get("accepted", false) or card_data.get("released", false) or claimed.has(str(card_data.get("card_id", ""))): continue
			claimed[str(card_data.get("card_id", ""))] = true
			var card := BattleCard.new(); grid.add_child(card); card.configure(str(card_data.get("card_id", "")), str(card_data.get("card_definition_id", "unknown")), false, true)
			card.modulate.a = 0.0
			card.tooltip_text += " · From " + str(card_data.get("original_zone", "deck")); _inspect(card, "battle.damage_card." + str(card_data.get("card_id", "")), card.tooltip_text)
		var card_ids: Array[String] = []
		for card in grid.get_children(): card_ids.append(str(card.instance_id))
		grid.set_meta("flow_content", card_ids)
		if grid.get_child_count() == 0:
			grid.hide()
			panel._label("No cards lost", 16)
		else:
			grid.custom_minimum_size.y = 150 if _multiple_enemies() else 90
	if committed and not _history_review and not _history_replay:
		if not _damage_commit_started.has(batch_key):
			_damage_commit_started[batch_key] = maxi(Time.get_ticks_msec(), int(_damage_card_times[batch_key]) + ceili(COMBAT_TIMING.review_seconds() * 1000.0))
		for grid in _damage_grids: grid.removal_started_ms = int(_damage_commit_started[batch_key])
		_finish_damage_sequence.call_deferred(batch_key, _income_animation_generation)
	# Auto-pass/model handoffs can rebuild after processing for this frame.
	# Restore the existing reveal/removal progress before the first draw,
	# rather than showing reviewed cards as transparent for one frame.
	for grid in _damage_grids: grid.refresh_playback()

func _finish_damage_sequence(batch_key: String, generation: int) -> void:
	var last_tick := Time.get_ticks_msec()
	while generation == _income_animation_generation and is_inside_tree():
		var now := Time.get_ticks_msec()
		if _history_review or _snapshot_panel_open:
			_damage_commit_started[batch_key] = int(_damage_commit_started[batch_key]) + now - last_tick
			for grid in _damage_grids:
				grid.started_ms += now - last_tick
				grid.removal_started_ms = int(_damage_commit_started[batch_key])
		last_tick = now
		if not _history_review and not _snapshot_panel_open:
			var removal_end := int(_damage_commit_started[batch_key]) + ceili(COMBAT_TIMING.removal() * 1000.0)
			var counts_seconds := maxf(0.01, COMBAT_TIMING.seconds("damage_count_seconds", 0.35))
			if now >= removal_end: _show_damage_counts(clampf((now - removal_end) / (counts_seconds * 1000.0), 0.0, 1.0))
			var finish := removal_end + ceili(counts_seconds * 1000.0)
			if Time.get_ticks_msec() >= finish:
				if _director.peek().get("type") == "combat_damage": _advance_beat()
				return
		await get_tree().process_frame

func _show_damage_counts(progress: float) -> void:
	var batch: Dictionary = _director.peek().get("event", {}).get("data", {})
	for target in _actor_profiles:
		var before: Dictionary = batch.get("actors_before", {}).get(target, {}).duplicate(true)
		if before.is_empty(): continue
		before["health"] = before.get("current_health", 0); before["energy"] = before.get("energy_points", 0)
		var after := before.duplicate(true)
		for card in _as_array(batch.get("removals", [])):
			if card.get("target_actor_id") != target or not card.get("accepted", false) or card.get("released", false): continue
			after.health = maxi(0, int(after.health) - 1)
			after["removed_count"] = int(after.get("removed_count", 0)) + 1
			var zone := str(card.get("original_zone", "deck")).to_lower() + "_count"
			after[zone] = maxi(0, int(after.get(zone, 0)) - 1)
		_actor_profiles[target].show_effects_progress(before, after, "cards", progress)
	_show_pending_damage_statuses()

func _show_pending_damage_statuses() -> void:
	var batch := _view.settled_damage
	if _director.peek().get("type") == "combat_damage": batch = _director.peek().get("event", {}).get("data", {})
	else:
		if _director.has_beats() or _view.segment != "damage_resolution": return
		if _view.stage not in ["damage_reaction", "status_damage_reaction"] or bool(batch.get("committed", false)): return
	var grouped := {}
	# Use the actual pending batch, not the attack's old reveal. Statuses still
	# apply when damage is fully blocked, and reactions may change this batch.
	for application in _as_array(batch.get("status_applications", [])):
		var target := str(application.get("target_actor_id", ""))
		var id := str(application.get("status_id", ""))
		var stacks := int(application.get("stacks", 0))
		if not _actor_profiles.has(target) or id.is_empty() or stacks <= 0: continue
		if not grouped.has(target): grouped[target] = {}
		grouped[target][id] = int(grouped[target].get(id, 0)) + stacks
	for target in grouped:
		_actor_profiles[target].show_pending_applications(grouped[target])

func _build_pending_statuses() -> void:
	var heading := Label.new(); heading.text = "PENDING STATUS APPLICATIONS"; heading.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _center.add_child(heading)
	var grouped := {}
	for application in _as_array(_view.settled_damage.get("status_applications", [])):
		var status: Dictionary = _as_dictionary(application)
		var target_id := str(status.get("target_actor_id", "")); var status_id := str(status.get("status_id", ""))
		if target_id.is_empty() or status_id.is_empty(): continue
		var key := "%s|%s" % [target_id, status_id]
		grouped[key] = int(grouped.get(key, 0)) + int(status.get("stacks", 1))
	if grouped.is_empty():
		var empty := Label.new(); empty.text = "No statuses pending"; empty.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; empty.add_theme_color_override("font_color", Color("9299a5")); _center.add_child(empty); _inspect(empty, "battle.pending_status.none", empty.text)
		return
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 55); _center.add_child(row)
	for target_id in [_focused_enemy, "blade"]:
		for key in grouped:
			var parts := str(key).split("|", false, 1)
			if parts.size() != 2 or parts[0] != target_id: continue
			var status_id := str(parts[1]); var status_data := BattlePresentationCatalog.status(status_id); var target_name := _actor_display_name(target_id).to_upper()
			var pending := Label.new(); pending.custom_minimum_size = Vector2(260, 48); pending.text = "%s  %s %s ×%d" % [target_name, status_data.glyph, status_data.name, int(grouped[key])]; pending.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; pending.add_theme_font_size_override("font_size", 18); pending.add_theme_color_override("font_color", Color("e5a07e")); pending.tooltip_text = "%s will receive %s ×%d when this damage batch is acknowledged." % [target_name.capitalize(), status_data.name, int(grouped[key])]; row.add_child(pending); _inspect(pending, "battle.pending_status.%s.%s" % [target_id, status_id], pending.tooltip_text)

func _provoked_toxin_reaction() -> bool:
	return _view.segment != "ongoing_effects" and _view.stage == "status_roll_reaction" and not _view.effect_rolls.is_empty()

func _build_provoked_toxins() -> void:
	var batch_id := ""
	for emitted in _view.events:
		if emitted.get("type") == "interaction_window_opened" and emitted.get("data", {}).has("rolls"):
			batch_id = str(emitted.get("data", {}).get("batch_id", ""))
	if is_instance_valid(_provoked_panel) and (_provoked_panel.entries.size() != _view.effect_rolls.size() or (not batch_id.is_empty() and batch_id != str(_provoked_panel.get_meta("batch_id", "")))):
		_provoked_panel.queue_free(); _provoked_panel = null
	var outcomes: Array[String] = []
	for roll in _view.effect_rolls: outcomes.append(_effect_roll_outcome(roll))
	if not is_instance_valid(_provoked_panel):
		_provoked_panel = preload("res://presentation/battle/provoked_toxins.gd").new()
		_center.add_child(_provoked_panel)
		_provoked_panel.configure(_view.effect_rolls, _actor_names(), outcomes, _view.events)
		_provoked_panel.set_meta("batch_id", batch_id)
	else:
		_provoked_panel.reparent(_center)
		_provoked_panel.update_rolls(_view.effect_rolls, outcomes, _view.events)
	_provoked_panel.show()
	_provoked_panel.set_paused(_history_review or _history_replay or _snapshot_panel_open)

func _build_effects() -> void:
	if _provoked_toxin_reaction():
		_build_provoked_toxins()
		return
	var panel := VBoxContainer.new(); panel.alignment = BoxContainer.ALIGNMENT_CENTER; panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center.add_child(panel)
	var title := Label.new(); title.text = "☠ ONGOING EFFECTS" if _view.segment == "ongoing_effects" else "☠ STATUS CHECKS"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 28); panel.add_child(title)
	var details: Array[String] = []
	for id in [viewer_actor_id] + _enemy_ids():
		for status in _view.actor(id).get("statuses", []):
			var definition := str(status.get("definition_id", "")); details.append("%s: %s ×%d — %s" % [_actor_display_name(id), BattlePresentationCatalog.status(definition).name, int(status.get("stacks", 1)), BattlePresentationCatalog.status(definition).text])
	var body := Label.new(); body.text = "\n".join(details) if not details.is_empty() else "No active status work remains."; body.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; body.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; panel.add_child(body)
	if _view.stage in ["status_roll", "status_roll_reaction"]:
		_build_effects_mat(panel)
	if _view.stage == "status_roll_reaction":
		var response_hint := Label.new()
		response_hint.text = "These dice have already rolled. You may play a response card or continue.\nCatalyst rerolls are automatic; continuing cannot skip or undo them."
		response_hint.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		response_hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		panel.add_child(response_hint)
	if _selected_card_selector() == "selected_die" and _view.stage == "blind_reaction":
		var tip := Button.new(); tip.text = "Tip Blind die to face 5"; tip.pressed.connect(_play_blind_tip); panel.add_child(tip); _inspect(tip, "battle.tip_target.blind", "Use Tip It on the current blind-roll die")

func _build_effects_mat(parent: VBoxContainer) -> void:
	var revealed := _view.stage == "status_roll_reaction"
	var row := HBoxContainer.new(); row.size_flags_horizontal = Control.SIZE_EXPAND_FILL; row.add_theme_constant_override("separation", 30); parent.add_child(row)
	_inspect(row, "battle.effects_mat", "Fixed status-effect roll and reveal area")
	for actor_id in [viewer_actor_id] + _enemy_ids():
		var actor_panel := VBoxContainer.new(); actor_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; actor_panel.alignment = BoxContainer.ALIGNMENT_BEGIN; row.add_child(actor_panel)
		if actor_id != viewer_actor_id and not revealed:
			_inspect(actor_panel, "battle.effects_hidden.goblin", "Reserved space for hidden enemy effect dice")
			continue
		_build_effects_panel(actor_panel, actor_id, revealed)

func _build_effects_panel(parent: VBoxContainer, actor_id: String, revealed: bool) -> void:
	var actor_name := _actor_display_name(actor_id).to_upper()
	var heading := Label.new(); heading.text = "%s EFFECTS" % actor_name; heading.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; heading.add_theme_font_size_override("font_size", 18); parent.add_child(heading)
	var rolls: Array = []
	for value in _view.effect_rolls:
		var roll: Dictionary = _as_dictionary(value)
		if str(roll.get("actor_id", "")) == actor_id: rolls.append(roll)
	var detail := Label.new(); detail.custom_minimum_size = Vector2(300, 70); detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; parent.add_child(detail)
	if rolls.is_empty():
		detail.text = "No effect dice required"
		detail.add_theme_color_override("font_color", Color("9299a5"))
		_inspect(detail, "battle.effects_none.%s" % actor_id, detail.text)
		return
	var grouped := {}
	for value in rolls:
		var roll: Dictionary = _as_dictionary(value); var status_id := str(roll.get("source_content_id", roll.get("status_id", ""))); grouped[status_id] = int(grouped.get(status_id, 0)) + 1
	var descriptions: Array[String] = []
	for status_id in grouped:
		var status := BattlePresentationCatalog.status(str(status_id)); descriptions.append("%s ×%d\nRoll once per status stack" % [status.name, int(grouped[status_id])])
	detail.text = "\n".join(descriptions); _inspect(detail, "battle.effects_selected.%s" % actor_id, detail.text)
	var dice_row := HBoxContainer.new(); dice_row.custom_minimum_size.y = 105; dice_row.alignment = BoxContainer.ALIGNMENT_CENTER; dice_row.add_theme_constant_override("separation", 12); parent.add_child(dice_row)
	for index in rolls.size():
		var roll: Dictionary = _as_dictionary(rolls[index]); var die_data: Dictionary = _as_dictionary(roll.get("die", {})); var face := int(die_data.get("face", 0)); var secretly_rolled := bool(roll.get("resolved", false)) and face == 0
		var die_id := str(die_data.get("die_id", "standard_d6")); var status_id := str(roll.get("source_content_id", roll.get("status_id", "")))
		var die := Button.new(); die.custom_minimum_size = Vector2(96, 96); die.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; _style_defense_die(die)
		if face > 0:
			die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(die_id, face), face]; die.disabled = true; die.add_theme_font_size_override("font_size", 24)
			die.tooltip_text = "%s %s die: face %d, %s." % [actor_name.capitalize(), BattlePresentationCatalog.status(status_id).name, face, BattlePresentationCatalog.symbol_name_for_die_face(die_id, face)]
			dice_row.add_child(die); _inspect(die, "battle.effect_die.%s.%d" % [actor_id, index], die.tooltip_text)
		elif secretly_rolled:
			die.text = "✓\nHIDDEN"; die.disabled = true; die.add_theme_font_size_override("font_size", 16); die.tooltip_text = "This effect die was rolled in secret. Its face will appear during the reveal."
			dice_row.add_child(die); _inspect(die, "battle.effect_die.blade.hidden.%d" % index, die.tooltip_text)
		else:
			die.tooltip_text = "Click this blank player effect die to roll it in secret."
			die.disabled = _submitting or _director.has_beats() or _history_review
			die.pressed.connect(_roll_effect_die.bind(index))
			dice_row.add_child(die); _inspect(die, "battle.effect_die.blade.pending.%d" % index, die.tooltip_text)
	if not revealed:
		var instruction := Label.new(); instruction.text = "Click each blank player die to roll it in secret"; instruction.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; instruction.add_theme_color_override("font_color", Color("b8bfca")); parent.add_child(instruction)
		return
	for value in rolls:
		var outcome := Label.new(); outcome.text = _effect_roll_outcome(_as_dictionary(value)); outcome.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; outcome.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; outcome.custom_minimum_size.x = 300; parent.add_child(outcome)

func _effect_roll_outcome(roll: Dictionary) -> String:
	var status_id := str(roll.get("source_content_id", roll.get("status_id", ""))); var status := BattlePresentationCatalog.status(status_id); var face := int(_as_dictionary(roll.get("die", {})).get("face", 0))
	var definition := BattlePresentationCatalog.definition("statuses", status_id); var operation_id := str(roll.get("operation_id", "")); var trigger_id := str(roll.get("trigger_id", "")); var resolved: Array = []
	for trigger in _as_array(definition.get("triggers", [])):
		if not trigger is Dictionary or (not trigger_id.is_empty() and str(trigger.get("id", "")) != trigger_id): continue
		for operation in _as_array(trigger.get("operations", [])):
			if operation is Dictionary and str(operation.get("type", "")) == "roll_dice" and (operation_id.is_empty() or str(operation.get("id", "")) == operation_id):
				resolved.append_array(_resolved_operations([operation], face))
	var descriptions: Array[String] = []
	for operation in resolved:
		if not operation is Dictionary: continue
		match str(operation.get("type", "")):
			"deal_damage": descriptions.append("%d damage pending" % _configured_amount(operation.get("amount", 0), face))
			"remove_status", "remove_status_stack":
				var removed_id := str(operation.get("status_id", status_id)); var stacks := int(operation.get("stack_count", 0))
				descriptions.append("Remove %s" % BattlePresentationCatalog.status(removed_id).name if stacks == 0 else "Remove %d %s stack%s" % [stacks, BattlePresentationCatalog.status(removed_id).name, "s" if stacks != 1 else ""])
			"apply_status": descriptions.append("Apply %s ×%d" % [BattlePresentationCatalog.status(str(operation.get("status_id", ""))).name, maxi(1, int(operation.get("stack_count", 1)))])
			"gain_resource": descriptions.append("Gain %d %s" % [_configured_amount(operation.get("amount", 0), face), str(operation.get("resource", "resource"))])
			"draw_cards": descriptions.append("Draw %d card%s" % [_configured_amount(operation.get("amount", 0), face), "s" if _configured_amount(operation.get("amount", 0), face) != 1 else ""])
			"noop": descriptions.append("No effect")
	if descriptions.is_empty(): descriptions.append("Effect result revealed")
	return "%s · %d\n%s" % [status.name, face, ", ".join(descriptions)]

func _roll_effect_die(index: int) -> void:
	_send(BattleCommandBuilder.roll_dice(_view.battle_id, "blade", _pending(), [index]))

func _build_income() -> void:
	var label := Label.new(); label.text = "▣  DRAW  →  NEW CARD  →  HAND        ✦ +1 ENERGY"; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", 28); _center.add_child(label)

func _build_hand(income_drawn_ids: Array = []) -> void:
	var label := Label.new(); label.visible = false; label.text = "YOUR HAND"; label.add_theme_font_size_override("font_size", 15); label.add_theme_color_override("font_color", Color("bbac8e")); _hand_dock.add_child(label)
	var scroll := ScrollContainer.new(); scroll.name = "HandScroll"; scroll.custom_minimum_size.y = 250; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO; scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; scroll.follow_focus = true; _hand_dock.add_child(scroll)
	var row := HBoxContainer.new(); scroll.add_child(row)
	var hand_limit := _view.stage == "discard_to_hand_limit"
	for entry in _view.hand_cards():
		var card := BattleCard.new(); row.add_child(card)
		var legal := _card_legal(str(entry.definition_id)) or hand_limit
		card.configure(str(entry.instance_id), str(entry.definition_id), legal and not _submitting and not _director.has_beats() and _selection_morph.is_empty())
		_lock_button_until(card, _interaction_deadline(false))
		card.toggle_mode = hand_limit or _selected_card.get("source_targeting", false)
		card.button_pressed = str(entry.instance_id) in _hand_limit_selection or (_selected_card.get("source_targeting", false) and _selected_card.get("instance_id") == entry.instance_id)
		card.pressed.connect(_on_card_pressed.bind(card))
		if str(entry.instance_id) in income_drawn_ids:
			card.prepare_income_draw()
			_income_drawn_cards.append(card)
		_inspect(card, "battle.card.%s" % str(entry.instance_id), card.tooltip_text)
	if hand_limit:
		var need := maxi(0, _view.actor("blade").get("hand_count", 0) - _view.actor("blade").get("max_hand_size", 6))
		var commit := Button.new(); commit.text = "Discard selected cards (%d/%d)" % [_hand_limit_selection.size(), need]; commit.disabled = _hand_limit_selection.size() != need or _submitting or _history_review; commit.pressed.connect(func(): _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), _hand_limit_selection))); _center.add_child(commit); _inspect(commit, "battle.hand_limit.commit", "Commit the selected hand-limit discards")

func _build_card_target_choices() -> void:
	if _selected_card.get("source_targeting", false):
		var instruction := Label.new(); instruction.text = "Spined Rebuttal\nClick a highlighted incoming attack."; instruction.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; instruction.add_theme_color_override("font_color", Color("f0cf7c")); _ability_dock.add_child(instruction)
		var cancel := Button.new(); cancel.text = "Cancel targeting"; cancel.pressed.connect(func(): _selected_card.clear(); _render()); _ability_dock.add_child(cancel)
		_inspect(cancel, "battle.card_target.cancel", "Cancel without playing the card or spending energy")
		return
	if _selected_card_selector() != "one_negative_status_on_self": return
	var card_definition := _view.content_definition("cards", str(_selected_card.get("definition_id", "")))
	var card_name := str(card_definition.get("name", "Selected card"))
	var panel := VBoxContainer.new(); panel.alignment = BoxContainer.ALIGNMENT_CENTER; panel.add_theme_constant_override("separation", 6); _center.add_child(panel)
	var heading := Label.new(); heading.text = "CHOOSE A STATUS FOR %s" % card_name.to_upper(); heading.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; heading.add_theme_font_size_override("font_size", 19); heading.add_theme_color_override("font_color", Color("f0bc58")); panel.add_child(heading)
	var instruction := Label.new(); instruction.text = "Select one negative status to remove."; instruction.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; panel.add_child(instruction)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 10); panel.add_child(row)
	var found := false
	for status_value in _as_array(_view.actor("blade").get("statuses", [])):
		var status: Dictionary = _as_dictionary(status_value)
		var status_id := str(status.get("definition_id", status.get("id", "")))
		if status_id.is_empty() or str(_view.content_definition("statuses", status_id).get("polarity", "")) != "negative": continue
		found = true
		var status_data := BattlePresentationCatalog.status(status_id)
		var button := Button.new(); button.text = "Remove %s %s ×%d" % [status_data.glyph, status_data.name, int(status.get("stacks", 1))]
		button.disabled = _submitting or _history_review; button.tooltip_text = "Play %s to remove all %s stacks from %s." % [card_name, status_data.name, _actor_display_name("blade")]
		button.pressed.connect(_play_selected_status_card.bind(status_id)); row.add_child(button)
		_inspect(button, "battle.status_target.%s" % status_id, button.tooltip_text)
	if not found:
		var unavailable := Label.new(); unavailable.text = "No negative status is currently available."; unavailable.add_theme_color_override("font_color", Color("ff8a78")); row.add_child(unavailable)

func _build_ability_row(caption: String, abilities: Array, actor_id: String) -> void:
	var dock: VBoxContainer = _ability_dock if actor_id == "blade" else _utility_contents.enemy
	var label := Label.new(); label.visible = actor_id != "blade"; label.text = caption; label.add_theme_font_size_override("font_size", 14); label.add_theme_color_override("font_color", Color("bbac8e")); dock.add_child(label)
	var row := VBoxContainer.new(); row.add_theme_constant_override("separation", 7); dock.add_child(row)
	var actor := _view.actor(actor_id); var qualified: Array = _as_array(actor.get("qualified_abilities", [])); var selected := str(actor.get("selected_ability", ""))
	if _view.segment == "defensive" and actor_id == "blade": selected = str(_view.defense_selections.get(actor_id, {}).get("ability_id", ""))
	for ability_id in abilities:
		var selected_attack := _selected_attack(actor_id)
		if actor_id == "blade" and _view.segment == "offensive" and _offense_selected_for_display() and not selected_attack.is_empty() and str(ability_id) != str(selected_attack.ability_id): continue
		var tile := BattleAbilityTile.new(); row.add_child(tile)
		if actor_id == "blade": tile.set_meta("flow_key", "ability:%s:%s" % [actor_id, ability_id])
		var defense_ready := _view.segment == "defensive" and not _defense_actions(str(ability_id)).is_empty()
		var can_select := actor_id == "blade" and _view.allowed("planning_select_ability") and (defense_ready if _view.segment == "defensive" else str(ability_id) in qualified) and not _submitting and not _history_review
		if _selected_card_selector() == "one_owned_offensive_ability": can_select = actor_id == "blade" and _view.stage == "planning" and not _history_review
		tile.configure(str(ability_id), str(ability_id) in qualified, selected == str(ability_id), can_select, actor)
		tile.cinematic_compact()
		var attack := _selected_attack(actor_id)
		if str(ability_id) == str(attack.get("ability_id", "")) and _offense_selected_for_display() and actor_id == "blade":
			tile.show_selected_attack(str(attack.text))
			_selected_attack_tiles[actor_id] = tile
			_inspect(tile, "battle.outcome.blade", str(attack.text))
		elif str(ability_id) == "needlefang" and _selected_card_selector() != "one_owned_offensive_ability":
			var tiers := BattlePresentationCatalog.needlefang_tiers(actor)
			for tier in tiers:
				tier.enabled = can_select and not _director.has_beats() and not _model_thinking and not _needlefang_tier_action(str(tier.id)).is_empty()
				if not tier.enabled: tier.text += "\nNot available with the current dice or phase."
			tile.configure_tiers(tiers, str(actor.get("selected_tier", "")) if selected == "needlefang" else "")
			tile.tier_pressed.connect(_select_needlefang_tier)
			for button in tile.find_children("*", "Button", true, false):
				if not button.has_meta("tier_id"): continue
				_inspect(button, "battle.ability.%s.needlefang.%s" % [actor_id, str(button.get_meta("tier_id", ""))], button.tooltip_text)
		else:
			var choices := _ability_choice_options(str(ability_id), can_select)
			if actor_id == "blade" and _selected_card_selector() != "one_owned_offensive_ability" and (choices.size() > 1):
				tile.configure_choices(choices)
				tile.choice_pressed.connect(_select_ability_action)
				var choice_index := 0
				for button in tile.get_node("AbilityChoices").get_children():
					_inspect(button, "battle.ability.%s.%s.choice.%d" % [actor_id, ability_id, choice_index], button.tooltip_text)
					choice_index += 1
			else:
				tile.pressed.connect(_on_ability_pressed.bind(str(ability_id)))
		_inspect(tile, "battle.ability.%s.%s" % [actor_id, str(ability_id)], tile.tooltip_text)
		if ability_id == "needlefang" and not _history_review and not _history_replay:
			var feedback: Dictionary = _ability_upgrade_feedback.get(actor_id, {})
			if feedback.get("battle_id") == _view.battle_id:
				tile.show_upgrade(int(feedback.amount), (Time.get_ticks_msec() - int(feedback.started_ms)) / 1000.0)

func _capture_ability_upgrades(previous_actors: Dictionary) -> void:
	for actor_id in _view.actors:
		var before := int(previous_actors.get(actor_id, {}).get("needlefang_damage_bonus", 0))
		var after := int(_view.actor(actor_id).get("needlefang_damage_bonus", 0))
		if after > before:
			_ability_upgrade_feedback[actor_id] = {"battle_id": _view.battle_id, "amount": after - before, "started_ms": Time.get_ticks_msec()}

func _needlefang_tier_action(tier_id: String) -> Dictionary:
	for action in _view.legal_actions:
		var payload: Dictionary = action.get("payload", {})
		if action.get("type") == "planning_select_ability" and payload.get("ability_id") == "needlefang" and payload.get("tier_id") == tier_id and _action_in_focus(action):
			return action
	return {}

func _select_needlefang_tier(tier_id: String) -> void:
	if _submitting or _history_review or _model_thinking or _director.has_beats() or not _view.allowed("planning_select_ability"): return
	var action := _needlefang_tier_action(tier_id)
	if not action.is_empty(): _send(JSON.stringify(action))

func _build_sources(caption: String) -> void:
	var label := Label.new(); label.text = caption; _center.add_child(label)
	var sources: Array = _view.damage_sources
	var settled_sources: Array = _as_array(_view.settled_damage.get("sources", []))
	var using_settled_batch := _view.segment != "defensive" and not settled_sources.is_empty()
	if using_settled_batch:
		sources = settled_sources
		if _view.segment == "ongoing_effects": _build_damage_source_summary(sources)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(row)
	var selected_source := _selected_source
	if _view.segment == "defensive" and selected_source.is_empty(): selected_source = str(_view.defense_selections.get("blade", {}).get("source_id", ""))
	for source in sources:
		if not _source_in_focus(source): continue
		if _view.segment == "defensive" and str(source.get("target_actor_id", "")) != "blade": continue
		var button := Button.new(); var id := str(source.get("id", "")); var data := BattlePresentationCatalog.ability(str(source.get("source_content_id", "")))
		var base := int(source.get("base_amount", 0)); var final := int(source.get("final_amount", 0)); var prevented := maxi(0, base - final) if using_settled_batch else int(source.get("prevention", 0)) + int(source.get("reaction_prevention", 0))
		if using_settled_batch:
			button.text = "%s → %s\nBase %d · Prevented %d · Final %d" % [data.name, _actor_display_name(str(source.get("target_actor_id", ""))), base, prevented, final]
		else:
			button.text = "%s → %s\nBase %d · Prevented %d · Pending %d" % [data.name, _actor_display_name(str(source.get("target_actor_id", ""))), base, prevented, maxi(0, base - prevented)]
		button.tooltip_text = "Damage source %s" % id; button.disabled = _submitting or _history_review; button.button_pressed = selected_source == id; button.pressed.connect(func(): _selected_source = id; _render()); row.add_child(button)
		_inspect(button, "battle.source.%s" % id, button.tooltip_text)
		if _reaction_feedback_active() and id == _reaction_card_feedback.get("source_id") and not _history_review:
			var glow := StyleBoxFlat.new(); glow.bg_color = Color("37504c"); glow.border_color = Color("a4e79b"); glow.set_border_width_all(2); glow.set_corner_radius_all(5)
			button.add_theme_stylebox_override("normal", glow)
			var remaining := (int(_reaction_card_feedback.expires_ms) - Time.get_ticks_msec()) / 1000.0
			var tween := button.create_tween(); tween.tween_interval(maxf(0, remaining - 0.8)); tween.tween_property(glow, "bg_color:a", 0.0, minf(0.8, remaining)); tween.parallel().tween_property(glow, "border_color:a", 0.0, minf(0.8, remaining))

func _capture_reaction_card_feedback(sent: Dictionary, previous_actors: Dictionary, previous_sources: Array) -> void:
	var actor_id := str(sent.get("actor_id", ""))
	var payload: Dictionary = sent.get("payload", {})
	var commitment: Dictionary = payload.get("commitment", {})
	var card_ids: Array = payload.get("card_ids", commitment.get("card_ids", []))
	var targets: Array = payload.get("target_ids", commitment.get("proposal_ids", []))
	if card_ids.size() != 1 or targets.size() != 1: return
	var card: Dictionary = previous_actors.get(actor_id, {}).get("card_instances", {}).get(str(card_ids[0]), {})
	if card.get("definition_id") != "spined_rebuttal": return
	for source in previous_sources:
		if source.get("id") != targets[0]: continue
		for updated in _view.damage_sources:
			if updated.get("id") != targets[0]: continue
			var gained := int(updated.get("reaction_prevention", 0)) - int(source.get("reaction_prevention", 0))
			if gained <= 0: return
			var attacker := _actor_display_name(str(source.get("source_actor_id", "")))
			var ability := BattlePresentationCatalog.ability(str(source.get("source_content_id", "")))
			_reaction_card_feedback = {"battle_id": _view.battle_id, "source_id": targets[0], "expires_ms": Time.get_ticks_msec() + 2800, "text": "Spined Rebuttal · +%d prevention against %s\n1 Poison queued for %s" % [gained, ability.name, attacker]}

func _build_reaction_card_feedback() -> void:
	if not _reaction_feedback_active() or _history_review or _history_replay: return
	var notice := Label.new(); notice.text = str(_reaction_card_feedback.text); notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	notice.add_theme_font_size_override("font_size", 20); notice.add_theme_color_override("font_color", Color("c9f4aa")); notice.mouse_filter = Control.MOUSE_FILTER_IGNORE; _root.add_child(notice); _place_cinematic(notice, Rect2(460, 150, 1030, 60))
	_inspect(notice, "battle.reaction_card_feedback", notice.text)
	var remaining := (int(_reaction_card_feedback.expires_ms) - Time.get_ticks_msec()) / 1000.0
	var tween := notice.create_tween(); tween.tween_interval(maxf(0, remaining - 0.8)); tween.tween_property(notice, "modulate:a", 0.0, minf(0.8, remaining))

func _build_damage_source_summary(sources: Array) -> void:
	var grouped := {}
	var order: Array[String] = []
	for value in sources:
		var source: Dictionary = _as_dictionary(value); var content_id := str(source.get("source_content_id", "")); var target_id := str(source.get("target_actor_id", ""))
		var key := "%s|%s" % [content_id, target_id]
		if not grouped.has(key):
			grouped[key] = {"content_id": content_id, "target_id": target_id, "base": 0, "final": 0}; order.append(key)
		var accumulated: Dictionary = grouped[key]
		accumulated["base"] = int(accumulated.base) + int(source.get("base_amount", 0))
		accumulated["final"] = int(accumulated.final) + int(source.get("final_amount", 0))
		grouped[key] = accumulated
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 30); _center.add_child(row)
	for key in order:
		var summary: Dictionary = grouped[key]; var base := int(summary.base); var final := int(summary.final); var content := BattlePresentationCatalog.ability(str(summary.content_id)); var target_name := _actor_display_name(str(summary.target_id))
		var result := Label.new(); result.custom_minimum_size = Vector2(280, 58); result.text = "%s → %s\n%d incoming · %d prevented · %d pending" % [content.name, target_name, base, maxi(0, base - final), final]; result.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; result.add_theme_font_size_override("font_size", 17); row.add_child(result); _inspect(result, "battle.damage_summary.%s.%s" % [str(summary.content_id), str(summary.target_id)], result.text)

func _offense_selected_for_display() -> bool:
	return _view.stage != "planning" or (not str(_view.actor("blade").get("selected_ability", "")).is_empty() and not _view.allowed("planning_select_ability"))

func _selected_offensive_preview(actor_id: String, ability_id: String) -> Dictionary:
	# The human's locked selection is known before the joint reveal. Preview
	# its pinned tier operations; never infer an unrevealed opponent selection.
	if actor_id != viewer_actor_id: return {}
	var actor := _view.actor(actor_id)
	var targets: Array = actor.get("selected_targets", [])
	var attack_target := str(targets[0]) if not targets.is_empty() else _focused_enemy
	var definition := _view.content_definition("abilities", ability_id)
	var operations: Array = definition.get("resolution", {}).get("operations", [])
	for tier in definition.get("qualification", {}).get("activation_tiers", []):
		if str(tier.get("id", "")) == str(actor.get("selected_tier", "")): operations = tier.get("operations", []); break
	var outcome := {"base_damage": 0, "status_applications": [], "resource_gains": {}, "provoke": 0}
	for operation in operations:
		match str(operation.get("type", "")):
			"deal_damage": outcome.base_damage += int(operation.get("amount", 0)) + (int(actor.get("needlefang_damage_bonus", 0)) if ability_id == "needlefang" else 0)
			"apply_status": outcome.status_applications.append({"target_actor_id": actor_id if operation.get("target") == "self" else attack_target, "status_id": str(operation.get("status_id", "")), "stacks": int(operation.get("stack_count", 1))})
			"gain_resource": outcome.resource_gains[str(operation.get("resource", ""))] = int(operation.get("amount", 0))
			"provoke": outcome.provoke += int(operation.get("amount", 0))
	return outcome

func _selected_attack(actor_id: String) -> Dictionary:
	if _view.segment not in ["offensive", "defensive", "damage_resolution"] or (_view.stage == "planning" and str(_view.actor(actor_id).get("selected_ability", "")).is_empty()): return {}
	var reveal := _view.offensive_reveal(actor_id)
	var id := str(reveal.get("ability_id", _view.actor(actor_id).get("selected_ability", "")))
	if reveal.is_empty() and _view.segment != "offensive":
		for source in _view.damage_sources:
			var owner := str(source.get("source_actor_id", "blade" if source.get("target_actor_id") == _focused_enemy else _focused_enemy))
			if owner == actor_id:
				id = str(source.get("source_content_id", id)); break
	if id.is_empty(): return {}
	var outcome := _as_dictionary(reveal.get("outcome", {})).duplicate(true)
	if outcome.is_empty(): outcome = _selected_offensive_preview(actor_id, id)
	var sources := _view.damage_sources
	var settled := _as_array(_view.settled_damage.get("sources", []))
	if _view.segment == "damage_resolution" and not settled.is_empty(): sources = settled
	var matching: Array = []
	var damage := 0
	var prevented := 0
	for source in sources:
		if str(source.get("source_actor_id", "blade" if source.get("target_actor_id") == _focused_enemy else _focused_enemy)) != actor_id or str(source.get("source_content_id", "")) != id: continue
		matching.append(source)
		var base := int(source.get("base_amount", 0))
		var final := maxi(0, base - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))
		if _view.segment == "damage_resolution" and not settled.is_empty(): final = int(source.get("final_amount", final))
		if _board_stage() == "defense_reaction" and _defense_summary_ready(str(source.get("id", ""))):
			var counts := _status_counts()
			for owner in counts:
				for status in _view.actor(owner).get("statuses", []): counts[owner][str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
			var preview := _compact_defense_data(str(source.get("target_actor_id", "")), counts, source)
			if not preview.is_empty(): final = int(preview.after)
		damage += final; prevented += maxi(0, base - final)
	if not matching.is_empty(): outcome["base_damage"] = damage
	var description := _offensive_outcome_text(BattlePresentationCatalog.ability(id).name, outcome, [])
	var lines := description.split("\n")
	lines.remove_at(0)
	if damage == 0 and not matching.is_empty(): lines.insert(0, "0 damage")
	if prevented > 0: lines.append("%d prevented" % prevented)
	elif _view.segment == "defensive": lines.append("")
	for i in lines.size():
		if "damage" in lines[i]: lines[i] += " · final" if _view.segment == "damage_resolution" else " · pending"
	return {"ability_id": id, "text": "\n".join(lines).replace("Pending: ", "").replace("Applies: ", ""), "sources": matching}

func _build_side_attacks() -> void:
	if not _selection_morph.is_empty() or _central_combat() or _director.has_beats(): return
	for actor_id in ["blade", _focused_enemy]:
		var attack := _selected_attack(actor_id)
		if attack.is_empty(): continue
		if actor_id == "blade" and _view.segment == "offensive": continue
		var dock := _ability_dock if actor_id == "blade" else _enemy_attack_dock
		var tile := BattleAbilityTile.new(); dock.add_child(tile)
		tile.configure(str(attack.ability_id), false, true, false, _view.actor(actor_id))
		tile.cinematic_compact(); tile.show_selected_attack(str(attack.text))
		_selected_attack_tiles[actor_id] = tile
		tile.set_meta("flow_key", "ability:%s:%s" % [actor_id, attack.ability_id])
		_inspect(tile, "battle.outcome." + actor_id, str(attack.text))
		# Source targeting stays inside the selected attack card.
		var targets := VBoxContainer.new()
		tile.add_child(targets)
		targets.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE)
		targets.offset_left = 8; targets.offset_right = -8
		targets.offset_top = -32.0 * attack.sources.size() - 4; targets.offset_bottom = -4
		tile.custom_minimum_size.y += 32 * attack.sources.size()
		for source in attack.sources:
			var id := str(source.get("id", ""))
			var target := Button.new(); target.text = "→ " + _actor_display_name(str(source.get("target_actor_id", "")))
			target.tooltip_text = "Select this damage source"; target.toggle_mode = true; target.button_pressed = _selected_source == id
			target.disabled = _submitting or _history_review
			target.pressed.connect(func(): _selected_source = id; _render())
			target.custom_minimum_size.y = 28
			target.add_theme_font_size_override("font_size", 16)
			for key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color"]: target.add_theme_color_override(key, CINEMATIC.INK)
			for state in ["normal", "hover", "pressed", "disabled"]:
				target.add_theme_stylebox_override(state, CINEMATIC.panel(Color("b88d3840") if state in ["hover", "pressed"] else Color.TRANSPARENT, Color.TRANSPARENT, 0))
			targets.add_child(target); _inspect(target, "battle.source." + id, target.tooltip_text)

func _offensive_outcome_text(ability_name: String, outcome: Dictionary, targets: Array) -> String:
	var lines: Array[String] = [ability_name]
	var damage := int(outcome.get("base_damage", 0))
	if damage > 0: lines.append("Pending: %d damage" % damage)
	var statuses := {}
	for application in _as_array(outcome.get("status_applications", [])):
		var status: Dictionary = _as_dictionary(application)
		var status_id := str(status.get("status_id", ""))
		if not status_id.is_empty(): statuses[status_id] = int(statuses.get(status_id, 0)) + int(status.get("stacks", 1))
	for status_id in statuses:
		lines.append("Applies: %s ×%d" % [BattlePresentationCatalog.status(str(status_id)).name, int(statuses[status_id])])
	var resources: Dictionary = _as_dictionary(outcome.get("resource_gains", {}))
	for resource_id in resources:
		var amount := int(resources[resource_id])
		if amount > 0: lines.append("Gains: %d %s" % [amount, str(resource_id).replace("_", " ").capitalize()])
	if int(outcome.get("provoke", 0)) > 0: lines.append("Provoke %d" % int(outcome.provoke))
	if damage == 0 and statuses.is_empty() and resources.is_empty() and int(outcome.get("provoke", 0)) == 0: lines.append("No offensive effect pending")
	var target_names: Array[String] = []
	for target_id in targets: target_names.append(_actor_display_name(str(target_id)))
	if not target_names.is_empty(): lines.append("Target: %s" % ", ".join(target_names))
	return "\n".join(lines)

func _build_enemy_die_targets() -> void:
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(row)
	for i in _view.rolled_dice(_focused_enemy).size():
		var die: Dictionary = _view.rolled_dice(_focused_enemy)[i]
		if int(die.get("face", 0)) != 6: continue
		var button := Button.new(); button.text = "Tip enemy die %d: 6 → 5" % (i + 1); button.disabled = _history_review; button.pressed.connect(func(): _play_tip_it(i)); row.add_child(button); _inspect(button, "battle.tip_target.goblin.%d" % i, "Use Tip It on this revealed face-6 die")

func _build_presentation_beat() -> void:
	var beat := _director.peek()
	if beat.get("type") == "combat_damage":
		_build_damage_lanes(beat.get("event", {}).get("data", {}), true)
		_build_hand()
		return
	if beat.get("type") == "effects_resolved":
		_build_automatic_effects(beat)
		return
	if beat.get("type") == "poison_conversion":
		_build_poison_conversion(beat)
		return
	if beat.get("type") == "card_cleanse":
		_build_card_cleanse(beat)
		return
	if beat.get("type") == "income_summary":
		_build_income_presentation(beat)
		return
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var title := Label.new(); title.text = str(beat.get("title", "Battle Event")); title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 38); _center.add_child(title)
	var detail := Label.new(); detail.text = str(beat.get("detail", "")); detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.add_theme_font_size_override("font_size", 22); _center.add_child(detail)
	if beat.get("type") == "cards_drawn":
		var cards: Array = _as_array(beat.get("event", {}).get("cards", []))
		if not cards.is_empty():
			var instance_id := str(cards[0]); var definition_id := str(_view.actor("blade").get("card_instances", {}).get(instance_id, {}).get("definition_id", "unknown")); var card := BattleCard.new(); card.configure(instance_id, definition_id, false); card.custom_minimum_size = Vector2(250, 170); _center.add_child(card)
	var button := Button.new(); button.text = "Continue Presentation"; button.tooltip_text = "Continue the visual presentation only; this sends no gameplay command."; button.custom_minimum_size = Vector2(133, 72); button.add_theme_font_size_override("font_size", 18); button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; button.pressed.connect(_advance_beat); _action_footer.add_child(button); _inspect(button, "battle.presentation.continue", button.tooltip_text)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)
	if beat.get("type") == "segment_entered":
		_center.set_meta("flow_key", "empty-segment")
		if not _auto_pass_disabled and not _history_review and not _history_replay:
			button.hide()
			_finish_empty_segment.call_deferred(_income_animation_generation)

func _finish_empty_segment(generation: int) -> void:
	var until := Time.get_ticks_msec() + ceili((COMBAT_TIMING.transition() + COMBAT_TIMING.seconds("empty_segment_hold_seconds", 0.8)) * 1000.0)
	while is_inside_tree() and generation == _income_animation_generation:
		if _snapshot_panel_open or _history_review or _auto_pass_disabled: return
		if Time.get_ticks_msec() >= until:
			if _director.peek().get("type") == "segment_entered": _advance_beat()
			return
		await get_tree().process_frame

func _build_card_cleanse(beat: Dictionary) -> void:
	var event: Dictionary = beat.get("event", {})
	var data: Dictionary = event.get("data", {})
	var actor_id := str(event.get("actor_id", ""))
	var panel = preload("res://presentation/battle/card_cleanse.gd").new()
	_root.add_child(panel)
	panel.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	panel.configure(_actor_display_name(actor_id), str(data.get("card_definition_id", "")), str(data.get("choice_id", "")), int(data.get("stacks_before", 0)), int(data.get("stacks_after", 0)), actor_id != "blade")
	_inspect(panel, "battle.card_cleanse", panel.tooltip_text)
	panel.finished.connect(_finish_status_animation.bind(_income_animation_generation))
	_start_card_cleanse.call_deferred(panel, actor_id, event, _income_animation_generation)

func _build_poison_conversion(beat: Dictionary) -> void:
	var data: Dictionary = beat.get("event", {}).get("data", {}).get("poison_conversion", {})
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var panel = preload("res://presentation/battle/poison_conversion.gd").new(); _center.add_child(panel)
	panel.configure(_actor_display_name(str(data.get("target_actor_id", ""))), data)
	_inspect(panel, "battle.poison_conversion", panel.tooltip_text)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)
	if _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	panel.finished.connect(_finish_status_animation.bind(_income_animation_generation))
	panel.animate()

func _start_card_cleanse(panel: Control, actor_id: String, event: Dictionary, generation: int) -> void:
	if generation != _income_animation_generation or not is_instance_valid(panel) or not is_inside_tree(): return
	if _director.peek().get("type") != "card_cleanse": return
	var profile: ActorProfile = _actor_profiles.get(actor_id)
	if not is_instance_valid(profile): return
	panel.prepare(profile)
	if _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	var key := "%s:%s" % [_view.battle_id, event.get("sequence", 0)]
	if not _cleanse_animation_times.has(key): _cleanse_animation_times[key] = Time.get_ticks_msec()
	panel.animate(int(_cleanse_animation_times[key]))

func _finish_status_animation(generation: int) -> void:
	if generation != _income_animation_generation or not is_inside_tree() or _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	if _director.peek().get("type") in ["card_cleanse", "poison_conversion"]: _advance_beat()

func _build_income_presentation(_beat: Dictionary) -> void:
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var drawn_ids: Array = _as_array(_income_actor_data("blade").get("cards", []))
	_build_hand(drawn_ids)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)
	_start_income_animation.call_deferred(_income_animation_generation)

func _start_income_animation(generation: int) -> void:
	if generation != _income_animation_generation or not is_inside_tree(): return
	if not _director.has_beats() or _director.peek().get("type") != "income_summary": return
	var duration := maxf(0.05, float(ProjectSettings.get_setting(INCOME_DURATION_SETTING, 2.0)))
	for profile_value in _actor_profiles.values():
		var profile: ActorProfile = profile_value
		if is_instance_valid(profile): profile.animate_income(duration)
	for card in _income_drawn_cards:
		if is_instance_valid(card): card.animate_income_draw(duration)
	var timer := Timer.new(); timer.one_shot = true; timer.wait_time = duration; _root.add_child(timer)
	timer.timeout.connect(_finish_income_presentation.bind(generation)); timer.start()

func _finish_income_presentation(generation: int) -> void:
	if generation != _income_animation_generation or not is_inside_tree(): return
	if _director.has_beats() and _director.peek().get("type") == "income_summary": _advance_beat()

func _income_actor_data(actor_id: String) -> Dictionary:
	if not _director.has_beats() or _director.peek().get("type") != "income_summary": return {}
	var beat: Dictionary = _director.peek()
	var event_data: Dictionary = _as_dictionary(beat.get("event", {}).get("data", {}))
	return _as_dictionary(_as_dictionary(event_data.get("actors", {})).get(actor_id, {}))

func _upcoming_income_actor_data(actor_id: String) -> Dictionary:
	# An Effects baseline already excludes any later income.
	if not _director.pending_effects_actor_before(actor_id).is_empty(): return {}
	if not _director.has_beats(): return {}
	var beat: Dictionary = _director.peek()
	if str(beat.get("presentation_segment", "")) != "ongoing_effects": return {}
	return _director.pending_income_actor(actor_id)

func _build_completion() -> void:
	if not _history_review and not learned_battle_mode: active_store.clear()
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var title := Label.new(); title.text = ("VICTORY" if _view.battle_result == "victory" else _view.battle_result.to_upper()); title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 52); title.add_theme_color_override("font_color", Color("f5c963")); _center.add_child(title)
	var final := Label.new()
	if learned_battle_mode:
		final.text = "You %d/%d · %s %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0)), _actor_display_name(_focused_enemy), int(_view.actor(_focused_enemy).get("current_health", 0)), int(_view.actor(_focused_enemy).get("max_health", 0))]
	else:
		final.text = "Blade %d/%d · Goblin %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0)), int(_view.actor(_focused_enemy).get("current_health", 0)), int(_view.actor(_focused_enemy).get("max_health", 0))]
	if _multiple_enemies():
		var health: Array[String] = ["You %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0))]]
		for id in _enemy_ids(): health.append("%s %d/%d" % [_actor_display_name(id), int(_view.actor(id).get("current_health", 0)), int(_view.actor(id).get("max_health", 0))])
		final.text = " · ".join(health)
	final.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _center.add_child(final)
	var again := Button.new(); again.text = "Rematch · Same Seats" if learned_battle_mode else "Play Again"; again.disabled = _history_review; again.pressed.connect(_play_again); _center.add_child(again); _inspect(again, "battle.complete.play_again", "Reset this learned matchup without reloading the model" if learned_battle_mode else "Start a new real-random battle")
	if learned_battle_mode:
		var new_battle := Button.new(); new_battle.text = "New Battle · Change Seat or Mode"; new_battle.pressed.connect(_return_to_mode_menu); _center.add_child(new_battle); _inspect(new_battle, "battle.complete.new_battle", "Return to the graphical battle-mode menu")
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)

func _add_action(parent: Container, text: String, command: String, callback: Callable) -> void:
	if not _view.allowed(command): return
	var button := Button.new(); button.text = text; button.disabled = _submitting or _director.has_beats() or _history_review; button.pressed.connect(callback); parent.add_child(button)
	if parent == _action_footer: button.custom_minimum_size = Vector2(133, 72); button.add_theme_font_size_override("font_size", 18); button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	if parent == _action_footer and command == "planning_pass" and _view.segment == "offensive":
		# The hidden accessible label must not grow the visible one-line Skip button.
		button.autowrap_mode = TextServer.AUTOWRAP_OFF; button.clip_text = true
		for key in ["font_color", "font_hover_color", "font_pressed_color", "font_disabled_color", "font_focus_color"]: button.add_theme_color_override(key, Color.TRANSPARENT)
		var short_caption := Label.new(); short_caption.text = "Skip"; short_caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; short_caption.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; short_caption.mouse_filter = Control.MOUSE_FILTER_IGNORE
		button.add_child(short_caption); short_caption.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_inspect(button, "battle.command.%s" % command, "Submit the authority command %s" % command)
	_lock_button_until(button, _interaction_deadline(command == "pass"))
	if command == "pass":
		_auto_pass_button = button
		_style_auto_pass_button(_auto_pass_highlight_ms >= 0)

func _pass_planning() -> void:
	if _submitting or _history_review or not _view.allowed("planning_pass"): return
	var command_json := BattleCommandBuilder.planning_pass(_view.battle_id, "blade", _pending())
	if _view.segment != "offensive" or _view.rolls_used("blade") > 0:
		_send(command_json)
		return
	var dialog := ConfirmationDialog.new()
	dialog.name = "SkipUnrolledOffense"
	dialog.title = "Skip without rolling?"
	dialog.dialog_text = "You have not rolled your offensive dice.\nSkipping gives up your offensive ability for this round.\nYou can still defend, but cannot roll offensive dice until next round."
	dialog.get_ok_button().text = "Skip Offensive Ability"
	dialog.get_cancel_button().text = "Keep Planning"
	add_child(dialog)
	dialog.confirmed.connect(func():
		dialog.queue_free()
		_send(command_json)
	)
	dialog.canceled.connect(dialog.queue_free)
	dialog.popup_centered()

func _ability_actions(ability_id: String) -> Array:
	var actions: Array = []
	for action in _view.legal_actions:
		if action.get("type") != "planning_select_ability" or action.get("payload", {}).get("ability_id") != ability_id or not _action_in_focus(action): continue
		if _view.segment == "defensive" and _selected_source not in action.get("payload", {}).get("target_ids", []): continue
		actions.append(action)
	return actions

func _ability_choice_options(ability_id: String, enabled: bool) -> Array[Dictionary]:
	var actions := _ability_actions(ability_id)
	var options: Array[Dictionary] = []
	var ready := enabled and not _director.has_beats() and not _model_thinking
	if ability_id == "shedskin" and _view.segment == "defensive":
		for paid in [false, true]:
			var matching := {}
			for action in actions:
				if bool(action.get("payload", {}).get("spend_catalyst", false)) == paid: matching = action; break
			options.append({"label": "Pay 1 Catalyst\nPrevent +2" if paid else "No Catalyst\nRoll 2 dice", "tooltip": "Spend 1 Catalyst before rolling to prevent 2 additional damage." if paid else "Roll the defense without spending Catalyst.", "action": matching, "enabled": ready and not matching.is_empty()})
		return options
	for action in actions:
		var payload: Dictionary = action.get("payload", {})
		var label := "Select"
		var toxins: Array = payload.get("toxin_choices", [])
		if not toxins.is_empty():
			var counts := {}; var parts: Array[String] = []
			for toxin in toxins: counts[toxin] = int(counts.get(toxin, 0)) + 1
			for toxin in counts: parts.append("%d %s" % [counts[toxin], BattlePresentationCatalog.status(str(toxin)).glyph])
			label = "Provoke\n" + " + ".join(parts)
		elif payload.get("spend_catalyst", false): label = "Pay 1 Catalyst\nPrevent +2"
		elif not str(payload.get("tier_id", "")).is_empty():
			label = str(payload.tier_id).replace("_", " ").capitalize()
			for tier in BattlePresentationCatalog.offensive_tier_summaries(ability_id):
				if tier.id == payload.tier_id: label += "\n" + str(tier.summary)
		# Preserve and distinguish targets for any future multi-target ability.
		var targets: Array = payload.get("target_ids", [])
		var target_variants := {}
		for candidate in actions: target_variants[JSON.stringify(candidate.get("payload", {}).get("target_ids", []))] = true
		if target_variants.size() > 1:
			var names: Array[String] = []
			for target in targets: names.append(_actor_display_name(str(target)))
			label += "\n" + ", ".join(names)
		options.append({"label": label, "tooltip": _venom_choice_label(action), "action": action, "enabled": ready})
	return options

func _select_ability_action(action: Dictionary) -> void:
	if _history_review or _submitting or _model_thinking or _director.has_beats() or not _view.allowed("planning_select_ability"): return
	# Revalidate against the current source and priority, not a captured button.
	if action.is_empty() or action not in _ability_actions(str(action.get("payload", {}).get("ability_id", ""))): return
	_send(JSON.stringify(action))

func _on_ability_pressed(ability_id: String) -> void:
	if _history_review or _submitting or _model_thinking or _director.has_beats(): return
	if _selected_card_selector() == "one_owned_offensive_ability":
		_send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], ability_id)); return
	var choices := _ability_actions(ability_id)
	if choices.size() == 1: _select_ability_action(choices[0])
	# Multi-option abilities are selected exclusively through their inline row.

func _on_card_pressed(card: BattleCard) -> void:
	if _history_review or _submitting or _model_thinking or _director.has_beats(): return
	if card.definition_id == "spined_rebuttal" and _view.stage == "defense_reaction":
		if _selected_card.get("instance_id") == card.instance_id and _selected_card.get("source_targeting", false):
			_selected_card.clear()
		else:
			_selected_card = {"instance_id": card.instance_id, "definition_id": card.definition_id, "source_targeting": true}
			if _source_card_actions().is_empty(): _selected_card.clear()
		_render(); return
	if str(BattlePresentationCatalog.card(card.definition_id).targeting.get("selector", "")) == "venom_choice" and _view.stage != "discard_to_hand_limit":
		var was_targeting: bool = _selected_card.get("source_targeting", false)
		_selected_card.clear()
		if was_targeting: _render()
		var choices: Array = []
		for action in _view.legal_actions:
			if not _action_in_focus(action): continue
			var payload: Dictionary = action.get("payload", {})
			var cards: Array = payload.get("card_ids", payload.get("commitment", {}).get("card_ids", []))
			if card.instance_id in cards: choices.append(action)
		if choices.size() == 1:
			_send(JSON.stringify(choices[0]))
		else:
			_show_venom_choices(choices, BattlePresentationCatalog.card(card.definition_id).name)
		return

	if _history_review or _submitting: return
	if _view.stage == "discard_to_hand_limit":
		if card.instance_id in _hand_limit_selection: _hand_limit_selection.erase(card.instance_id)
		else: _hand_limit_selection.append(card.instance_id)
		_render(); return
	_selected_card = {"instance_id": card.instance_id, "definition_id": card.definition_id}
	var selector := _selected_card_selector()
	var planning := _view.allowed("planning_commit_cards")
	match selector:
		"self":
			if planning: _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], ["blade"]))
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [], ["blade"]))
		"one_enemy":
			if planning: _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], [_focused_enemy]))
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [], [_focused_enemy]))
		"one_owned_combat_die":
			if _selected_indices.size() == 1: _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], [], "", "", int(_selected_indices[0])))
			else: _show_error("Select exactly one rolled player die, then choose this card again.")
		"one_negative_status_on_self": _render()
		"one_incoming_damage_source":
			if _selected_source.is_empty(): _show_error("Select an incoming damage source, then choose this card again.")
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [_selected_source]))
		"selected_die", "one_owned_offensive_ability": _render()
		_: _show_error("This card uses an unsupported target selector: %s" % selector)

func _play_tip_it(index: int) -> void:
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": _focused_enemy, "die_index": index, "face": _selected_card_set_face()}]))

func _play_blind_tip() -> void:
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": "blade", "die_index": 0, "face": _selected_card_set_face()}]))

func _play_selected_status_card(status_id: String) -> void:
	if _selected_card.is_empty() or status_id.is_empty(): return
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], status_id))

func _source_card_actions(source_id: String = "") -> Array:
	var choices: Array = []
	if not _selected_card.get("source_targeting", false) or _history_review or _history_replay or _submitting or _model_thinking or _director.has_beats() or not _error_message.is_empty() or not _view.allowed("commit_interaction"): return choices
	for action in _view.legal_actions:
		if action.get("type") != "commit_interaction" or action.get("actor_id") != viewer_actor_id: continue
		var payload: Dictionary = action.get("payload", {})
		var commitment: Dictionary = payload.get("commitment", {})
		if payload.get("pending_input_id") != _pending().get("id"): continue
		if _selected_card.get("instance_id") not in payload.get("card_ids", commitment.get("card_ids", [])): continue
		var targets: Array = payload.get("target_ids", commitment.get("proposal_ids", []))
		if targets.size() != 1 or (not source_id.is_empty() and targets[0] != source_id): continue
		if _view.damage_sources.any(func(source): return source.get("id") == targets[0] and source.get("target_actor_id") == viewer_actor_id): choices.append(action)
	return choices

func _play_source_card(source_id: String) -> void:
	# Resolve against the current legal list, never a cached target command.
	var choices := _source_card_actions(source_id)
	if choices.size() != 1: return
	_selected_source = source_id
	_send(JSON.stringify(choices[0]))

func _start_player_roll(indices: Array) -> void:
	if _history_review or _history_replay or indices.is_empty(): return
	_player_roll_feedback = {"battle_id": _view.battle_id, "round": _view.round_number, "indices": indices.duplicate(), "started_ms": Time.get_ticks_msec(), "duration_ms": ceili(COMBAT_TIMING.seconds("player_roll_seconds", 0.45) * 1000.0)}

func _player_roll_active() -> bool:
	return not _player_roll_feedback.is_empty() and not _history_review and not _history_replay and _player_roll_feedback.battle_id == _view.battle_id and int(_player_roll_feedback.round) == _view.round_number and Time.get_ticks_msec() < int(_player_roll_feedback.started_ms) + int(_player_roll_feedback.duration_ms)

func _reroll_unkept(history_confirmed: bool = false) -> void:
	if _submitting or _history_review or _player_roll_active(): return
	var indices: Array = []
	for index in _view.rolled_dice("blade").size():
		if index not in _selected_indices: indices.append(index)
	if indices.is_empty():
		_show_error("Deselect at least one die before rerolling."); return
	var action := _history_reroll_action(indices)
	if _history_replay and not history_confirmed:
		_try_replay_history_action(action, "Reroll Unkept Dice", {"kind": "reroll"})
		return
	_record_history_point("Reroll Unkept Dice", "decision", action)
	var keep_command := BattleCommandBuilder.planning_keep(_view.battle_id, "blade", _pending(), _selected_indices)
	_submitting = true
	var keep_result: Dictionary = gateway.submit(keep_command)
	if keep_result.get("accepted") != true:
		_submitting = false; _show_error(str(keep_result.get("error", "Could not keep the selected dice.")), keep_result); _render(); return
	if not _view.apply_result(keep_result):
		_submitting = false; _show_error("The keep result was not a safe battle snapshot.", keep_result); _render(); return
	_save_active()
	var reroll_command := BattleCommandBuilder.planning_reroll(_view.battle_id, "blade", _pending(), indices)
	var reroll_result: Dictionary = gateway.submit(reroll_command)
	_submitting = false
	if reroll_result.get("accepted") != true:
		_show_error(str(reroll_result.get("error", "Could not reroll the unkept dice.")), reroll_result); _render(); return
	if not _view.apply_result(reroll_result):
		_show_error("The reroll result was not a safe battle snapshot.", reroll_result); _render(); return
	_error_message = ""
	_selected_card.clear(); _selected_source = ""; _hand_limit_selection.clear()
	_director.queue_result(reroll_result, _director.last_sequence())
	_history_action_completed()
	_save_active()
	_record_history_arrival("Reroll Unkept Dice")
	_start_player_roll(indices)
	_render()
	if learned_battle_mode: call_deferred("_schedule_model_if_needed", reroll_result)

func _send(command_json: String, history_confirmed: bool = false) -> void:
	if _submitting or _history_review or _player_roll_active() or command_json.is_empty(): return
	_reset_auto_pass_preview()
	var sent = JSON.parse_string(command_json)
	var sent_type := str(sent.get("type", "")) if sent is Dictionary else ""
	var morph_selection := sent_type == "planning_select_ability" and _view.segment == "offensive"
	var label := _history_label_for_command(sent)
	var action := _history_command_action(sent)
	if _history_replay and not history_confirmed:
		_try_replay_history_action(action, label, {"kind": "command", "command_json": command_json})
		return
	_record_history_point(label, "decision", action)
	# Submission is synchronous: a throwaway rebuild cannot be drawn and leaves
	# unlaid-out controls for the selection transition to capture.
	_submitting = true
	var result: Dictionary = gateway.submit(command_json)
	_submitting = false
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "Battle command rejected.")), result); _render(); return
	var previous_actors := _view.actors
	var previous_sources := _view.damage_sources
	var previous_damage := _view.settled_damage.duplicate(true)
	if not _view.apply_result(result):
		_show_error("Authority result was not a safe battle snapshot.", result); _render(); return
	_capture_ability_upgrades(previous_actors)
	_capture_reaction_card_feedback(sent, previous_actors, previous_sources)
	_capture_damage_feedback(result, previous_damage)
	_clear_reaction_notice_for_new_round()
	_error_message = ""
	_selected_card.clear(); _selected_source = ""; _hand_limit_selection.clear()
	if sent_type not in ["planning_keep", "planning_commit_cards"]: _selected_indices.clear()
	_director.queue_result(result, _director.last_sequence(), previous_actors)
	_history_action_completed()
	_save_active()
	_record_history_arrival(label)
	if sent_type in ["planning_roll", "planning_reroll"]:
		_start_player_roll(range(_view.rolled_dice("blade").size()) if sent_type == "planning_roll" else sent.get("payload", {}).get("reroll_indices", []))
	if morph_selection and not _history_review and not _history_replay:
		_selection_morph = _selected_attack("blade")
		if _selection_morph.is_empty(): _selection_morph = {"ability_id": str(sent.get("payload", {}).get("ability_id", "")), "text": "Selected"}
	_render()
	if not _selection_morph.is_empty(): _finish_selection_morph.call_deferred()
	elif learned_battle_mode: call_deferred("_schedule_model_if_needed", result)

func _finish_selection_morph() -> void:
	await get_tree().create_timer(COMBAT_TIMING.transition() + 0.35).timeout
	if not is_inside_tree(): return
	_selection_morph.clear()
	_render()

func _advance_beat(history_confirmed: bool = false) -> void:
	var completed_label := ""
	if _history_tools_enabled() and not _history_review and _director.has_beats():
		var beat: Dictionary = _director.peek()
		var label := "%s: %s" % ["Automatic" if beat.get("type") in ["income_summary", "card_cleanse", "poison_conversion"] else "Continue", _history_beat_label(beat)]
		completed_label = label
		var action := _history_presentation_action(beat)
		if _history_replay and not history_confirmed:
			_try_replay_history_action(action, label, {"kind": "presentation"})
			return
		_record_history_point(label, "presentation", action)
	_director.advance(); _history_action_completed(); _save_active(); _record_history_arrival(completed_label); _render()
	if learned_battle_mode: call_deferred("_schedule_model_if_needed", {"learned_policy": _view.learned_policy})

func _play_again() -> void:
	if _history_review: return
	if learned_battle_mode:
		_start_learned_rematch()
		return
	active_store.clear(); get_tree().change_scene_to_file("res://app/boot/battle_bootstrap.tscn")

func _start_learned_rematch() -> void:
	if _submitting or _model_thinking: return
	_submitting = true
	_model_error = false
	_error_message = ""
	_render()
	learned_seed = int(Time.get_unix_time_from_system() * 1000000.0) ^ Time.get_ticks_usec()
	var battle_id := "learned-%d-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]
	var result: Dictionary = gateway.start_battle(battle_id, learned_seed, true)
	_submitting = false
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "The learned rematch could not start.")), result)
		_render()
		return
	_view = BattleViewState.new()
	_director = BattlePresentationDirector.new()
	_director.configure_learned_battle(true)
	_selected_indices.clear(); _selected_card.clear(); _selected_source = ""; _hand_limit_selection.clear()
	_reaction_notice = ""
	_selection_roll_number = -1
	if not _view.apply_result(result):
		_show_error("The rematch did not return a safe human-viewer snapshot.", result)
		_render()
		return
	_director.queue_result(result)
	_render()
	call_deferred("_schedule_model_if_needed", result)

func _return_to_mode_menu() -> void:
	if _submitting or _model_thinking: return
	get_tree().change_scene_to_file("res://app/boot/battle_bootstrap.tscn")

func _snapshot_tools_enabled() -> bool:
	if learned_battle_mode: return false
	return (
		OS.is_debug_build()
		and OS.get_environment("DICE_AND_DESTINY_ENABLE_SNAPSHOTS") == "1"
		and bool(ProjectSettings.get_setting("dice_and_destiny/development/enable_snapshots", false))
	)

func _history_tools_enabled() -> bool:
	if learned_battle_mode: return false
	return (
		OS.is_debug_build()
		and OS.get_environment("DICE_AND_DESTINY_ENABLE_HISTORY") == "1"
		and bool(ProjectSettings.get_setting("dice_and_destiny/development/enable_history", false))
	)

func _apply_history_context(context: Dictionary) -> void:
	history_context = context.duplicate(true)
	_history_review = bool(context.get("review", false))
	_history_replay = bool(context.get("replay", false))
	_history_point_id = str(context.get("point_id", ""))
	_history_origin_battle_id = str(context.get("origin_battle_id", ""))
	if context.has("history_scroll_value"):
		_history_scroll_value = int(context.get("history_scroll_value", 0))
	if context.has("history_follow_latest"):
		_history_follow_latest = bool(context.get("history_follow_latest", true))
	elif _history_review or _history_replay:
		_history_follow_latest = false
	var client_state: Dictionary = _as_dictionary(context.get("client_state", {}))
	_selected_indices.clear()
	for value in _as_array(client_state.get("selected_indices", [])): _selected_indices.append(int(value))
	_selected_indices.sort()
	_selected_source = str(client_state.get("selected_source", ""))
	_focused_enemy = str(client_state.get("focused_enemy", _focused_enemy))
	_selected_card = _as_dictionary(client_state.get("selected_card", {})).duplicate(true)
	_hand_limit_selection = _as_array(client_state.get("hand_limit_selection", [])).duplicate()
	_selection_roll_number = _as_array(_view.actor("blade").get("roll_history", [])).size()

func _history_client_state() -> Dictionary:
	return {
		"selected_indices": _selected_indices.duplicate(),
		"selected_source": _selected_source,
		"focused_enemy": _focused_enemy,
		"selected_card": _selected_card.duplicate(true),
		"hand_limit_selection": _hand_limit_selection.duplicate(),
	}

func _refresh_history() -> bool:
	var result: Dictionary = gateway.list_dev_history(_view.battle_id, viewer_actor_id)
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not list history."))
		return false
	var timeline: Dictionary = _as_dictionary(result.get("data", {}).get("timeline", {}))
	_history_entries = _as_array(timeline.get("points", []))
	_history_branch = _as_dictionary(timeline.get("branch", {}))
	var status := str(_history_branch.get("status", "active"))
	_history_replay = status == "replay"
	if _history_replay:
		_history_point_id = str(_history_branch.get("cursor_point_id", _history_branch.get("base_point_id", _history_point_id)))
	elif not _history_review:
		_history_point_id = ""
	return true

func _build_history_bar(parent: VBoxContainer) -> void:
	var panel := PanelContainer.new()
	var style := StyleBoxFlat.new(); style.bg_color = Color("0b1119f2"); style.border_color = Color("536273ff"); style.set_border_width_all(1); style.set_corner_radius_all(6)
	panel.add_theme_stylebox_override("panel", style); parent.add_child(panel); _inspect(panel, "battle.history.bar", "Developer history timeline")
	var content := HBoxContainer.new(); content.add_theme_constant_override("separation", 8); panel.add_child(content)
	var heading := Label.new(); heading.text = "REVIEWING HISTORY" if _history_review else "REPLAYING HISTORY" if _history_replay else "HISTORY"; heading.add_theme_color_override("font_color", Color("f0bc58") if _history_review or _history_replay else Color("79d8ff")); content.add_child(heading)
	var scroll := ScrollContainer.new(); scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO; scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; content.add_child(scroll)
	_inspect(scroll, "battle.history.scroll", "Scrollable developer history points; follows the latest point until manually scrolled left")
	var points := HBoxContainer.new(); points.add_theme_constant_override("separation", 4); scroll.add_child(points)
	if _history_entries.is_empty():
		var empty := Label.new(); empty.text = "History points appear before player actions and presentation transitions."; empty.add_theme_color_override("font_color", Color("9299a5")); points.add_child(empty)
	else:
		for index in _history_entries.size():
			var entry: Dictionary = _as_dictionary(_history_entries[index])
			var point_id := str(entry.get("id", ""))
			var point := Button.new(); point.text = "%d · %s" % [index + 1, str(entry.get("label", "Point"))]; point.toggle_mode = true; point.button_pressed = point_id == _history_point_id; point.disabled = _submitting or point_id.is_empty(); point.tooltip_text = "Round %d · %s · %s · %s" % [int(entry.get("round", 0)), _segment_name(str(entry.get("segment", ""))), str(entry.get("stage", "")).replace("_", " "), str(entry.get("kind", ""))]; point.pressed.connect(func(): _jump_history(point_id)); points.add_child(point); _inspect(point, "battle.history.point.%s" % point_id, point.tooltip_text)
	var refresh := Button.new(); refresh.text = "↻"; refresh.tooltip_text = "Refresh history"; refresh.disabled = _submitting; refresh.pressed.connect(func(): _refresh_history(); _render()); content.add_child(refresh); _inspect(refresh, "battle.history.refresh", "Refresh the developer history timeline")
	var horizontal_bar := scroll.get_h_scroll_bar()
	_history_scroll_adjusting = true
	horizontal_bar.value_changed.connect(func(value): _history_scroll_changed(scroll, int(value)))
	_position_history_scroll.call_deferred(scroll)
	if not _history_message.is_empty():
		var message := Label.new(); message.text = _history_message; message.add_theme_color_override("font_color", Color("ff8a78") if "error" in _history_message.to_lower() else Color("9fd69f")); content.add_child(message)

func _position_history_scroll(scroll: ScrollContainer) -> void:
	if not is_instance_valid(scroll): return
	var horizontal_bar := scroll.get_h_scroll_bar()
	var latest := maxi(0, int(horizontal_bar.max_value - horizontal_bar.page))
	_history_scroll_adjusting = true
	scroll.scroll_horizontal = latest if _history_follow_latest else clampi(_history_scroll_value, 0, latest)
	if not _history_follow_latest and not _history_point_id.is_empty():
		var selected := _history_point_control(scroll, _history_point_id)
		if selected != null: scroll.ensure_control_visible(selected)
	_history_scroll_value = scroll.scroll_horizontal
	_history_scroll_adjusting = false

func _history_point_control(parent: Node, point_id: String) -> Control:
	var inspection_id := "battle.history.point.%s" % point_id
	for control in parent.find_children("*", "Control", true, false):
		if control.has_meta("inspection_id") and str(control.get_meta("inspection_id")) == inspection_id:
			return control
	return null

func _history_scroll_changed(scroll: ScrollContainer, value: int) -> void:
	if _history_scroll_adjusting or not is_instance_valid(scroll): return
	var horizontal_bar := scroll.get_h_scroll_bar()
	var latest := maxi(0, int(horizontal_bar.max_value - horizontal_bar.page))
	_history_scroll_value = value
	_history_follow_latest = value >= latest - 4

func _build_history_review_controls() -> void:
	var panel := PanelContainer.new()
	var style := StyleBoxFlat.new(); style.bg_color = Color("1a1210f5"); style.border_color = Color("f0bc58ff"); style.set_border_width_all(2); style.set_corner_radius_all(7)
	panel.add_theme_stylebox_override("panel", style); _center.add_child(panel); _inspect(panel, "battle.history.review", "Read-only history review controls")
	var content := VBoxContainer.new(); content.add_theme_constant_override("separation", 6); panel.add_child(content)
	var title := Label.new(); title.text = "HISTORY REVIEW · READ ONLY"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 22); title.add_theme_color_override("font_color", Color("f0bc58")); content.add_child(title)
	var detail := Label.new(); detail.text = "You are viewing an earlier authority checkpoint. Inspect it or capture a developer snapshot before choosing how to continue."; detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; content.add_child(detail)
	var choices := VBoxContainer.new(); choices.alignment = BoxContainer.ALIGNMENT_CENTER; content.add_child(choices)
	var preserve := Button.new(); preserve.text = "Resume Here · Keep Existing Future"; preserve.disabled = _submitting; preserve.pressed.connect(func(): _commit_history("preserve")); choices.add_child(preserve); _inspect(preserve, "battle.history.preserve", "Replay from this cursor while retaining every later history point")
	var replace := Button.new(); replace.text = "Resume Here · Replace Existing Future"; replace.disabled = _submitting; replace.pressed.connect(func(): _commit_history("replace")); choices.add_child(replace); _inspect(replace, "battle.history.replace", "Resume here and archive the previous future")
	var latest := Button.new(); latest.text = "Return to Latest"; latest.disabled = _submitting; latest.pressed.connect(_return_history_latest); choices.add_child(latest); _inspect(latest, "battle.history.latest", "Leave review and return to the prior latest battle state")

func _record_history_point(label: String, kind: String, action: Dictionary) -> bool:
	if not _history_tools_enabled() or _history_review or _history_replay: return true
	var result: Dictionary = gateway.mark_dev_history(_view.battle_id, viewer_actor_id, label, kind, _director.last_sequence(), _history_client_state(), action)
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not record history point."))
		return false
	var timeline: Dictionary = _as_dictionary(result.get("data", {}).get("timeline", {}))
	_history_entries = _as_array(timeline.get("points", [])); _history_branch = _as_dictionary(timeline.get("branch", {})); _history_message = ""
	return true

func _record_history_arrival(completed_action_label: String = "") -> bool:
	if not _history_tools_enabled() or _history_review or _history_replay: return true
	var kind := "presentation" if _director.has_beats() else "decision"
	var state_label := _history_current_state_label()
	var label := state_label if completed_action_label.is_empty() else "%s · %s" % [completed_action_label, state_label]
	return _record_history_point(label, kind, {})

func _history_action_completed() -> void:
	_history_follow_latest = true
	_history_scroll_value = 0
	if not _history_review and not _history_replay:
		history_context = {}

func _history_current_state_label() -> String:
	if _director.has_beats():
		return "Current: %s" % _history_beat_label(_director.peek())
	if _view.segment == "offensive" and _view.stage == "planning":
		return "Offensive Dice %d/%d" % [_view.rolls_used("blade"), _view.max_rolls("blade")]
	if _view.is_complete(): return "Battle Complete"
	return "Current: %s · %s" % [_segment_name(_view.segment), _view.stage.replace("_", " ").capitalize()]

func _jump_history(point_id: String) -> void:
	if point_id.is_empty() or _submitting: return
	_history_follow_latest = false
	_submitting = true
	var result: Dictionary = gateway.jump_dev_history(_view.battle_id, viewer_actor_id, point_id)
	_submitting = false
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not open history point.")); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	var point: Dictionary = _as_dictionary(data.get("point", {}))
	var context := {"review": true, "point_id": str(point.get("id", point_id)), "origin_battle_id": str(data.get("origin_battle_id", _view.battle_id)), "client_state": _as_dictionary(data.get("client_state", {})), "history_scroll_value": _history_scroll_value, "history_follow_latest": false}
	_handoff_battle_result(result, int(data.get("presented_sequence", 0)), loaded_snapshot_name, context)

func _commit_history(mode: String) -> void:
	if not _history_review or _submitting: return
	_submitting = true
	var result: Dictionary = gateway.commit_dev_history(_view.battle_id, viewer_actor_id, mode)
	_submitting = false
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not resume from history.")); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	_handoff_battle_result(result, int(data.get("presented_sequence", _director.last_sequence())), loaded_snapshot_name, _history_context_from_data(data))

func _return_history_latest() -> void:
	if not _history_review or _submitting: return
	_submitting = true
	var result: Dictionary = gateway.return_dev_history_latest(_view.battle_id, viewer_actor_id)
	_submitting = false
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not return to latest.")); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	_handoff_battle_result(result, int(data.get("presented_sequence", 0)), loaded_snapshot_name, {})

func _try_replay_history_action(action: Dictionary, label: String, pending: Dictionary) -> void:
	if _submitting: return
	_submitting = true
	var result: Dictionary = gateway.replay_dev_history_action(_view.battle_id, viewer_actor_id, action)
	_submitting = false
	if result.get("accepted") == true:
		var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
		_handoff_battle_result(result, int(data.get("presented_sequence", _director.last_sequence())), loaded_snapshot_name, _history_context_from_data(data))
		return
	var history_data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	if bool(history_data.get("divergence", false)):
		_history_pending_divergence = pending.duplicate(true)
		_history_pending_divergence["action"] = action.duplicate(true)
		_history_pending_divergence["attempted_label"] = label
		_history_pending_divergence["expected_label"] = str(history_data.get("expected_label", "the recorded action"))
		_history_pending_divergence["future_point_count"] = int(history_data.get("future_point_count", 1))
		_render()
		return
	_show_error(str(result.get("error", "Could not replay recorded history.")), result)
	_render()

func _build_history_divergence_panel() -> void:
	var scrim := ColorRect.new(); scrim.color = Color(0.01, 0.015, 0.025, 0.76); scrim.set_anchors_preset(Control.PRESET_FULL_RECT); scrim.mouse_filter = Control.MOUSE_FILTER_STOP; _root.add_child(scrim)
	var panel := PanelContainer.new(); panel.set_anchors_preset(Control.PRESET_CENTER)
	var style := StyleBoxFlat.new(); style.bg_color = Color("18110fff"); style.border_color = Color("f0bc58ff"); style.set_border_width_all(2); style.set_corner_radius_all(8); style.shadow_color = Color(0, 0, 0, 0.85); style.shadow_size = 16
	panel.add_theme_stylebox_override("panel", style); panel.position = Vector2(-390, -185); panel.size = Vector2(780, 370); _root.add_child(panel); _inspect(panel, "battle.history.divergence", "Confirmation required before replacing recorded future history")
	var margin := MarginContainer.new(); margin.add_theme_constant_override("margin_left", 24); margin.add_theme_constant_override("margin_right", 24); margin.add_theme_constant_override("margin_top", 22); margin.add_theme_constant_override("margin_bottom", 22); panel.add_child(margin)
	var content := VBoxContainer.new(); content.add_theme_constant_override("separation", 14); margin.add_child(content)
	var title := Label.new(); title.text = "THIS CHANGES THE RECORDED FUTURE"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 26); title.add_theme_color_override("font_color", Color("f0bc58")); content.add_child(title)
	var expected := str(_history_pending_divergence.get("expected_label", "the recorded action")); var attempted := str(_history_pending_divergence.get("attempted_label", "the new action")); var count := int(_history_pending_divergence.get("future_point_count", 1))
	var detail := Label.new(); detail.text = "Recorded next action: %s\nYour new action: %s\n\nContinuing will replace %d history point%s from this position forward. The preserved source branch remains available for diagnostics." % [expected, attempted, count, "s" if count != 1 else ""]; detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; content.add_child(detail)
	var replace := Button.new(); replace.text = "Replace Future and Continue"; replace.disabled = _submitting; replace.pressed.connect(_confirm_history_divergence); content.add_child(replace); _inspect(replace, "battle.history.divergence.confirm", "Confirm replacing future history with the new action")
	var cancel := Button.new(); cancel.text = "Cancel · Keep Existing Future"; cancel.disabled = _submitting; cancel.pressed.connect(func(): _history_pending_divergence.clear(); _render()); content.add_child(cancel); _inspect(cancel, "battle.history.divergence.cancel", "Cancel the changed action and keep all recorded future points")

func _confirm_history_divergence() -> void:
	if _history_pending_divergence.is_empty() or _submitting: return
	var pending := _history_pending_divergence.duplicate(true)
	_submitting = true
	var result: Dictionary = gateway.replace_dev_history_future(_view.battle_id, viewer_actor_id)
	_submitting = false
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "Could not replace future history.")), result); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {})); var timeline: Dictionary = _as_dictionary(data.get("timeline", {}))
	_history_entries = _as_array(timeline.get("points", [])); _history_branch = _as_dictionary(timeline.get("branch", {})); _history_replay = false; _history_point_id = ""; history_context = {}; _history_pending_divergence.clear()
	match str(pending.get("kind", "")):
		"command": _send(str(pending.get("command_json", "")), true)
		"reroll": _reroll_unkept(true)
		"presentation": _advance_beat(true)

func _history_context_from_data(data: Dictionary) -> Dictionary:
	var branch: Dictionary = _as_dictionary(data.get("branch", {}))
	var review := bool(data.get("review", str(branch.get("status", "")) == "review"))
	var replay := bool(data.get("replay", str(branch.get("status", "")) == "replay"))
	var point: Dictionary = _as_dictionary(data.get("point", {}))
	var point_id := str(branch.get("cursor_point_id", point.get("id", "")))
	if review:
		return {
			"review": true,
			"point_id": point_id,
			"origin_battle_id": str(data.get("origin_battle_id", branch.get("latest_battle_id", _history_origin_battle_id))),
			"client_state": _as_dictionary(data.get("client_state", {})),
			"history_scroll_value": _history_scroll_value,
			"history_follow_latest": false,
		}
	if not replay:
		return {"history_scroll_value": _history_scroll_value, "history_follow_latest": false}
	return {
		"review": false,
		"replay": true,
		"point_id": point_id,
		"origin_battle_id": str(branch.get("latest_battle_id", _history_origin_battle_id)),
		"client_state": _as_dictionary(data.get("client_state", {})),
		"history_scroll_value": _history_scroll_value,
		"history_follow_latest": false,
	}

func _history_command_action(command_value) -> Dictionary:
	var command: Dictionary = _as_dictionary(command_value)
	return {
		"type": "command",
		"actor_id": str(command.get("actor_id", viewer_actor_id)),
		"command_type": str(command.get("type", "")),
		"payload": _as_dictionary(command.get("payload", {})).duplicate(true),
	}

func _history_reroll_action(indices: Array) -> Dictionary:
	return {"type": "reroll_unkept", "kept_indices": _selected_indices.duplicate(), "reroll_indices": indices.duplicate()}

func _history_presentation_action(beat: Dictionary) -> Dictionary:
	return {"type": "presentation_continue", "beat_type": str(beat.get("type", "")), "watermark": int(beat.get("watermark", beat.get("sequence", 0)))}

func _handoff_battle_result(result: Dictionary, presented_sequence: int, snapshot_name: String, context: Dictionary) -> void:
	var battle_id := str(result.get("snapshot", {}).get("battle_id", ""))
	if battle_id.is_empty() or active_store.save_active(battle_id, viewer_actor_id, presented_sequence, snapshot_name, context) != OK:
		_show_error("The history battle pointer could not be saved.", result); _render(); return
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = result; screen.viewer_actor_id = viewer_actor_id; screen.gateway = gateway; screen.active_store = active_store; screen.last_presented_sequence = presented_sequence; screen.loaded_snapshot_name = snapshot_name; screen.history_context = context
	get_tree().root.add_child(screen); queue_free()

func _history_label_for_command(command_value) -> String:
	var command: Dictionary = _as_dictionary(command_value)
	var kind := str(command.get("type", "Action"))
	var payload: Dictionary = _as_dictionary(command.get("payload", {}))
	match kind:
		"planning_roll": return "Roll 5 Dice"
		"planning_reroll": return "Reroll Unkept Dice"
		"planning_select_ability": return "Choose %s" % str(BattlePresentationCatalog.ability(str(payload.get("ability_id", ""))).get("name", "Ability"))
		"planning_commit_cards", "commit_interaction":
			if _view.stage == "discard_to_hand_limit": return "Discard to Hand Limit"
			if not _selected_card.is_empty(): return "Play %s" % str(BattlePresentationCatalog.card(str(_selected_card.get("definition_id", ""))).get("name", "Card"))
			return "Commit Cards"
		"roll_dice": return "Roll Effect Dice" if _view.segment == "ongoing_effects" else "Roll Defense Die"
		"planning_pass": return "Pass Defense" if _view.segment == "defensive" else "Skip Offensive Ability"
		"pass": return "Pass / Acknowledge"
	return kind.replace("_", " ").capitalize()

func _history_beat_label(beat: Dictionary) -> String:
	var title := str(beat.get("title", ""))
	if not title.is_empty(): return title
	return str(beat.get("type", "Presentation")).replace("_", " ").capitalize()

func _toggle_snapshot_panel() -> void:
	_snapshot_panel_open = not _snapshot_panel_open
	if _snapshot_panel_open: _refresh_snapshot_entries()
	_render()

func _build_snapshot_panel() -> void:
	var scrim := ColorRect.new(); scrim.color = Color(0.01, 0.015, 0.025, 0.62); scrim.set_anchors_preset(Control.PRESET_FULL_RECT); scrim.mouse_filter = Control.MOUSE_FILTER_STOP; _root.add_child(scrim)
	var panel := PanelContainer.new(); panel.set_anchors_preset(Control.PRESET_CENTER_RIGHT)
	var panel_style := StyleBoxFlat.new(); panel_style.bg_color = Color("0d121aff"); panel_style.border_color = Color("536273ff"); panel_style.set_border_width_all(1); panel_style.set_corner_radius_all(8); panel_style.shadow_color = Color(0, 0, 0, 0.8); panel_style.shadow_size = 14
	panel.add_theme_stylebox_override("panel", panel_style)
	panel.position = Vector2(-470, -330); panel.size = Vector2(450, 660); _root.add_child(panel); _inspect(panel, "battle.dev_snapshots.panel", "Opaque developer snapshot dialog")
	var margin := MarginContainer.new(); margin.add_theme_constant_override("margin_left", 18); margin.add_theme_constant_override("margin_right", 18); margin.add_theme_constant_override("margin_top", 18); margin.add_theme_constant_override("margin_bottom", 18); panel.add_child(margin)
	var content := VBoxContainer.new(); content.add_theme_constant_override("separation", 10); margin.add_child(content)
	var heading := Label.new(); heading.text = "DEVELOPER SNAPSHOTS"; heading.add_theme_font_size_override("font_size", 22); content.add_child(heading)
	var warning := Label.new(); warning.text = "Authority checkpoints only · Loading always creates a new battle"; warning.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; warning.add_theme_color_override("font_color", Color("f0bc58")); content.add_child(warning)
	var name_label := Label.new(); name_label.text = "Snapshot name (letters, numbers, . _ -)"; content.add_child(name_label)
	var name_edit := LineEdit.new(); name_edit.text = _snapshot_name; name_edit.placeholder_text = "round-2-effects"; content.add_child(name_edit); _inspect(name_edit, "battle.dev_snapshots.name", "Name for the checkpoint snapshot")
	var overwrite := CheckBox.new(); overwrite.text = "Replace an existing snapshot with this name"; overwrite.button_pressed = _snapshot_overwrite; overwrite.toggled.connect(func(value): _snapshot_overwrite = value); content.add_child(overwrite); _inspect(overwrite, "battle.dev_snapshots.overwrite", overwrite.text)
	var capture := Button.new(); capture.text = "Capture Current Authority State"; capture.disabled = _submitting or not _valid_snapshot_name(_snapshot_name); capture.pressed.connect(_capture_dev_snapshot); content.add_child(capture); _inspect(capture, "battle.dev_snapshots.capture", "Capture the current authoritative battle checkpoint and exact presentation cursor")
	name_edit.text_changed.connect(func(value): _snapshot_name = value.strip_edges(); capture.disabled = _submitting or not _valid_snapshot_name(_snapshot_name))
	if _director.has_beats():
		var presentation_note := Label.new(); presentation_note.text = "The currently visible presentation will resume when this snapshot is loaded."; presentation_note.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; presentation_note.add_theme_color_override("font_color", Color("9fd69f")); content.add_child(presentation_note)
	var list_label := Label.new(); list_label.text = "SAVED SNAPSHOTS"; content.add_child(list_label)
	var list := ItemList.new(); list.custom_minimum_size.y = 210; content.add_child(list); _inspect(list, "battle.dev_snapshots.list", "Saved developer snapshots")
	for entry_value in _snapshot_entries:
		var entry: Dictionary = _as_dictionary(entry_value)
		var history_summary := " · %d history points" % int(entry.get("history_point_count", 0)) if bool(entry.get("history_included", false)) else " · legacy/no history"
		var title := "%s · R%d %s/%s · %d events%s" % [str(entry.get("name", "")), int(entry.get("round", 0)), str(entry.get("segment", "")).replace("_", " "), str(entry.get("stage", "")).replace("_", " "), int(entry.get("event_count", 0)), history_summary]
		list.add_item(title)
		if str(entry.get("name", "")) == _selected_snapshot_name: list.select(list.item_count - 1)
	var load := Button.new(); load.text = "Load Selected as New Battle"; load.disabled = _selected_snapshot_name.is_empty() or _submitting; load.pressed.connect(func(): _load_dev_snapshot(_selected_snapshot_name)); content.add_child(load); _inspect(load, "battle.dev_snapshots.load", "Clone the selected snapshot into a new active battle")
	list.item_selected.connect(func(index):
		if index >= 0 and index < _snapshot_entries.size():
			_selected_snapshot_name = str(_snapshot_entries[index].get("name", ""))
		load.disabled = _selected_snapshot_name.is_empty() or _submitting
	)
	var restart := Button.new(); restart.text = "Restart Loaded Snapshot" if not loaded_snapshot_name.is_empty() else "Restart Loaded Snapshot (none loaded)"; restart.disabled = loaded_snapshot_name.is_empty() or _submitting; restart.pressed.connect(func(): _load_dev_snapshot(loaded_snapshot_name)); content.add_child(restart); _inspect(restart, "battle.dev_snapshots.restart", "Create another fresh battle from the snapshot that launched this battle")
	if not loaded_snapshot_name.is_empty():
		var origin := Label.new(); origin.text = "Current battle came from: %s" % loaded_snapshot_name; content.add_child(origin)
	var status := Label.new(); status.text = _snapshot_message; status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; status.add_theme_color_override("font_color", Color("9fd69f") if not _snapshot_message.begins_with("Error:") else Color("ff8a78")); content.add_child(status)
	var close := Button.new(); close.text = "Close"; close.pressed.connect(func(): _snapshot_panel_open = false; _render()); content.add_child(close); _inspect(close, "battle.dev_snapshots.close", "Close developer snapshot controls")

func _refresh_snapshot_entries() -> bool:
	var result: Dictionary = gateway.list_dev_snapshots(_view.battle_id, viewer_actor_id)
	if result.get("accepted") != true:
		_snapshot_message = "Error: %s" % str(result.get("error", "Could not list snapshots.")); return false
	_snapshot_entries = _as_array(result.get("data", {}).get("snapshots", []))
	if not _selected_snapshot_name.is_empty():
		var found := false
		for entry in _snapshot_entries:
			if str(entry.get("name", "")) == _selected_snapshot_name: found = true; break
		if not found: _selected_snapshot_name = ""
	return true

func _capture_dev_snapshot() -> void:
	if not _valid_snapshot_name(_snapshot_name):
		_snapshot_message = "Error: Use 1–64 letters, numbers, dots, underscores, or hyphens."; _render(); return
	if not _checkpoint_snapshot_history():
		_render(); return
	var result: Dictionary = gateway.save_dev_snapshot(_view.battle_id, viewer_actor_id, _snapshot_name, _snapshot_overwrite)
	if result.get("accepted") != true:
		_snapshot_message = "Error: %s" % str(result.get("error", "Could not capture snapshot.")); _render(); return
	_selected_snapshot_name = _snapshot_name
	var metadata: Dictionary = _as_dictionary(result.get("data", {}).get("snapshot", {}))
	_snapshot_message = "Captured %s at Round %d · %s · %s with %d history point(s)." % [_snapshot_name, _view.round_number, _segment_name(_view.segment), _view.stage.replace("_", " "), int(metadata.get("history_point_count", 0))]
	_refresh_snapshot_entries(); _render()

func _checkpoint_snapshot_history() -> bool:
	# Review/replay branches already carry an explicit cursor. Active battles need
	# an actionless endpoint so a snapshot can restore the exact visible beat,
	# including a presentation the player has not acknowledged yet.
	if _history_review or _history_replay:
		return true
	var kind := "presentation" if _director.has_beats() else "decision"
	var result: Dictionary = gateway.mark_dev_history(_view.battle_id, viewer_actor_id, _history_current_state_label(), kind, _director.last_sequence(), _history_client_state(), {})
	if result.get("accepted") != true:
		_snapshot_message = "Error: Could not checkpoint the visible battle state: %s" % str(result.get("error", "history checkpoint failed"))
		return false
	if _history_tools_enabled():
		var timeline: Dictionary = _as_dictionary(result.get("data", {}).get("timeline", {}))
		_history_entries = _as_array(timeline.get("points", []))
		_history_branch = _as_dictionary(timeline.get("branch", {}))
	return true

func _load_dev_snapshot(snapshot_name: String) -> void:
	if snapshot_name.is_empty() or _submitting: return
	_submitting = true
	var result: Dictionary = gateway.load_dev_snapshot(_view.battle_id, viewer_actor_id, snapshot_name)
	_submitting = false
	if result.get("accepted") != true:
		_snapshot_message = "Error: %s" % str(result.get("error", "Could not load snapshot.")); _render(); return
	var snapshot: Dictionary = _as_dictionary(result.get("snapshot", {})); var battle_id := str(snapshot.get("battle_id", ""))
	if battle_id.is_empty():
		_snapshot_message = "Error: Loaded snapshot did not return a battle ID."; _render(); return
	var metadata: Dictionary = _as_dictionary(result.get("data", {}).get("loaded_snapshot", {})); var event_count := int(metadata.get("event_count", 0))
	var restored_history: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	var restored_sequence := int(restored_history.get("presented_sequence", event_count))
	var restored_context := _history_context_from_data(restored_history) if not restored_history.is_empty() else {}
	if not restored_history.is_empty() and not bool(restored_history.get("review", false)) and not bool(restored_history.get("replay", false)):
		restored_context = {}
	_handoff_battle_result(result, restored_sequence, snapshot_name, restored_context)

func _valid_snapshot_name(value: String) -> bool:
	var expression := RegEx.new()
	return expression.compile("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$") == OK and expression.search(value) != null

func _save_active() -> void:
	if learned_battle_mode: return
	active_store.save_active(_view.battle_id, viewer_actor_id, _director.last_sequence(), loaded_snapshot_name, history_context)

func _card_legal(definition: String) -> bool:
	if str(BattlePresentationCatalog.card(definition).targeting.get("selector", "")) == "venom_choice" or (_continuous_damage_response() and not _view.legal_actions.is_empty()):
		for action in _view.legal_actions:
			if not _action_in_focus(action): continue
			var payload: Dictionary = action.get("payload", {})
			for instance in payload.get("card_ids", payload.get("commitment", {}).get("card_ids", [])):
				for entry in _view.hand_cards():
					if str(entry.instance_id) == str(instance) and str(entry.definition_id) == definition: return true
		return false

	return not _submitting and not _history_review and _view.card_playable_now(definition)

func _defense_preview(ability_id: String, base: int, rolled_face: int) -> Dictionary:
	var ability := _view.content_definition("abilities", ability_id)
	var resolution := _as_dictionary(ability.get("resolution", {}))
	var pending := base
	var prevented := 0
	for operation in _resolved_operations(_as_array(resolution.get("operations", [])), rolled_face):
		if not operation is Dictionary: continue
		match str(operation.get("type", "")):
			"prevent_damage":
				var amount := _configured_amount(operation.get("amount", 0), rolled_face)
				prevented += amount; pending = maxi(0, pending - amount)
			"scale_damage":
				var denominator := maxi(1, int(operation.get("denominator", 1)))
				pending = floori(float(pending * int(operation.get("numerator", 1))) / float(denominator))
	return {"pending": pending, "prevented": prevented, "rolled": _contains_roll_operation(_as_array(resolution.get("operations", [])))}

func _contains_roll_operation(operations: Array) -> bool:
	for operation in operations:
		if operation is Dictionary and str(operation.get("type", "")) == "roll_dice": return true
	return false

func _resolved_operations(operations: Array, rolled_face: int) -> Array:
	var result: Array = []
	for operation in operations:
		if not operation is Dictionary: continue
		if str(operation.get("type", "")) != "roll_dice":
			result.append(operation); continue
		for outcome in _as_array(operation.get("outcomes", [])):
			if outcome is Dictionary and _outcome_contains_face(outcome, rolled_face):
				result.append_array(_resolved_operations(_as_array(outcome.get("operations", [])), rolled_face))
	return result

func _outcome_contains_face(outcome: Dictionary, rolled_face: int) -> bool:
	for configured_face in _as_array(outcome.get("faces", [])):
		if int(configured_face) == rolled_face: return true
	return false

func _configured_amount(value, rolled_face: int) -> int:
	return rolled_face if value is String and value == "rolled_face" else int(value)

func _selected_card_selector() -> String:
	if _selected_card.is_empty(): return ""
	var definition := _view.content_definition("cards", str(_selected_card.get("definition_id", "")))
	return str(_as_dictionary(definition.get("targeting", {})).get("selector", ""))

func _selected_card_set_face() -> int:
	if _selected_card.is_empty(): return 0
	var definition := _view.content_definition("cards", str(_selected_card.get("definition_id", "")))
	for operation in _as_array(definition.get("operations", [])):
		if operation is Dictionary and str(operation.get("type", "")) == "modify_die": return int(operation.get("face", 0))
	return 0

func _pending() -> Dictionary: return _view.viewer_pending()
func _segment_name(id: String) -> String:
	for pair in SEGMENTS:
		if pair[0] == id: return pair[1]
	return id.replace("_", " ").capitalize()

func _is_single_ability_opponent() -> bool:
	return _view.learned_policy.get("controller_kind", "") == "single_ability"

func _actor_display_name(actor_id: String) -> String:
	actor_id = _display_actor_id(actor_id)
	var actor := _view.actor(actor_id)
	var character := _as_dictionary(actor.get("character", {}))
	var display_name := str(character.get("name", "")).strip_edges()
	if display_name.is_empty():
		var definition_id := str(actor.get("definition_id", ""))
		if definition_id.is_empty():
			definition_id = "blade_warden" if actor_id == "blade" or (actor_id == _focused_enemy and learned_battle_mode) else "venom_goblin" if actor_id == _focused_enemy else actor_id
		display_name = definition_id.replace("_", " ").capitalize()
	if actor_id == _focused_enemy and learned_battle_mode and not _is_single_ability_opponent(): return "Learned " + display_name
	if _multiple_enemies() and actor_id in _enemy_ids():
		var siblings: Array = _enemy_ids().filter(func(id): return _view.actor(id).get("definition_id") == actor.get("definition_id"))
		if siblings.size() > 1: display_name += " %d" % (siblings.find(actor_id) + 1)
	return display_name

func _capture_offensive_reaction_notice(result: Dictionary) -> void:
	for event_value in _as_array(result.get("events", [])):
		var event: Dictionary = _as_dictionary(event_value)
		if str(event.get("type", "")) != "card_played" or str(event.get("segment", "")) != "offensive": continue
		var data: Dictionary = _as_dictionary(event.get("data", {}))
		var old_ability := str(data.get("old_ability", "")); var new_ability := str(data.get("new_ability", ""))
		if old_ability.is_empty() and new_ability.is_empty(): continue
		var card_name := _card_display_name(str(data.get("card_definition_id", "")))
		var actor_name := _actor_display_name(str(event.get("actor_id", "")))
		var die_number := int(data.get("die_index", -1)) + 1; var face := int(data.get("face", 0))
		var old_name := str(BattlePresentationCatalog.ability(old_ability).name) if not old_ability.is_empty() else "No attack"
		var new_name := str(BattlePresentationCatalog.ability(new_ability).name) if not new_ability.is_empty() else "No attack"
		var change := "%s → %s" % [old_name, new_name] if old_name != new_name else "the attack remains %s" % new_name
		_reaction_notice = "%s played %s: die %d changed to face %d; %s." % [actor_name, card_name, die_number, face, change]

func _build_reaction_notice() -> void:
	if _reaction_notice.is_empty(): return
	var notice := Label.new(); notice.text = _reaction_notice; notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; notice.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; notice.add_theme_font_size_override("font_size", 18); notice.add_theme_color_override("font_color", Color("ef9a55")); _center.add_child(notice)
	_inspect(notice, "battle.offensive_reaction.notice", _reaction_notice)

func _combat_event_text(event: Dictionary) -> String:
	var kind := str(event.get("type", ""))
	var data: Dictionary = _as_dictionary(event.get("data", {}))
	var application: Dictionary = data.get("status_application", {})
	if int(application.get("after", 0)) > int(application.get("before", 0)):
		return "%s gained %d %s" % [_actor_display_name(str(application.get("target_actor_id", ""))), int(application.after) - int(application.before), BattlePresentationCatalog.status(str(application.status_id)).name]
	var incubation: Dictionary = data.get("incubation_application", {})
	if int(incubation.get("after", 0)) > int(incubation.get("before", 0)):
		return "%s gained 1 Incubation" % _actor_display_name(str(incubation.get("target_actor_id", "")))
	var conversion: Dictionary = data.get("poison_conversion", {})
	if conversion.get("converted", false):
		return "%s: 1 Poison upgraded to 1 Volatile Poison" % _actor_display_name(str(conversion.get("target_actor_id", "")))
	if kind == "card_played":
		var actor_name := _actor_display_name(str(event.get("actor_id", "")))
		var card_name := _card_display_name(str(data.get("card_definition_id", "")))
		var status_id := str(data.get("choice_id", ""))
		var removed := int(data.get("stacks_removed", 0))
		if not status_id.is_empty() and removed > 0:
			var status_name := str(_view.content_definition("statuses", status_id).get("name", status_id.replace("_", " ").capitalize()))
			return "%s played %s; removed %s ×%d (%d → %d)" % [actor_name, card_name, status_name, removed, int(data.get("stacks_before", removed)), int(data.get("stacks_after", 0))]
		return "%s played %s" % [actor_name, card_name]
	if kind == "damage_prevented_or_modified" and not str(data.get("card_definition_id", "")).is_empty():
		return "%s played %s: incoming damage %d → %d" % [_actor_display_name(str(event.get("actor_id", ""))), _card_display_name(str(data.card_definition_id)), int(data.get("damage_before", 0)), int(data.get("damage_after", 0))]
	if kind == "defense_selected":
		var ability_id := str(data.get("ability_id", ""))
		var ability_name := str(_view.content_definition("abilities", ability_id).get("name", ability_id.replace("_", " ").capitalize()))
		return "%s revealed %s · face %d" % [_actor_display_name(str(event.get("actor_id", ""))), ability_name, int(data.get("rolled_face", 0))]
	if kind == "battle_completed": return "Battle completed: %s" % str(event.get("battle_result", "result")).replace("_", " ")
	if kind == "damage_committed": return "Damage and status results committed"
	if kind == "cards_permanently_removed": return "%s lost %d card(s) to damage" % [_actor_display_name(str(event.get("target_actor_id", ""))), _as_array(event.get("cards", [])).size()]
	return kind.replace("_", " ")

func _card_display_name(card_definition_id: String) -> String:
	if card_definition_id.is_empty(): return "a reaction card"
	var name := str(BattlePresentationCatalog.card(card_definition_id).get("name", ""))
	return name if not name.is_empty() else card_definition_id.replace("_", " ").capitalize()

func _clear_reaction_notice_for_new_round() -> void:
	if _view.segment == "offensive" and _view.stage == "planning" and not bool(_view.raw_snapshot.get("offensive_reselection", false)): _reaction_notice = ""

func _enemy_plan_text() -> String:
	for event in _view.events:
		var event_data: Dictionary = _as_dictionary(event.get("data", {}))
		var commitments: Dictionary = _as_dictionary(event_data.get("commitments", {}))
		var enemy: Dictionary = _as_dictionary(commitments.get(_focused_enemy, {}))
		if not enemy.is_empty() and int(enemy.get("ai_d100", 0)) > 0:
			return "Enemy D100 %d · simulated rolls %d" % [int(enemy.ai_d100), int(enemy.get("simulated_rolls", 0))]
	return ""

func _prompt_text() -> String:
	if _view.stage in ["defense_roll", "defense_reaction"]: return "Defense"
	if not _model_thinking and bool(_view.raw_snapshot.get("offensive_reselection", false)): return "Your dice changed · choose your attack"
	if _model_thinking: return ""
	var pending := _pending()
	if pending.is_empty(): return "The authority is resolving the battle…"
	return str(pending.get("stage", "Action")).replace("_", " ").capitalize()

func _build_error(parent: VBoxContainer) -> void:
	_error = Label.new(); _error.visible = not _error_message.is_empty(); _error.text = _error_message; _error.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _error.add_theme_color_override("font_color", Color("ff8a78")); parent.add_child(_error)

func _show_error(message: String, result: Dictionary = {}) -> void:
	_error_message = "Command error: %s\n%s" % [message, JSON.stringify(result)]
	if is_instance_valid(_error): _error.visible = true; _error.text = _error_message
	else: push_error(message)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.record_error(message, result)

func _build_error_only(message: String) -> void:
	var label := Label.new(); label.text = "BATTLE CLIENT ERROR\n%s" % message; label.set_anchors_preset(Control.PRESET_FULL_RECT); label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; add_child(label)

func _inspect(control: Control, control_id: String, description: String = "") -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, control, description)

func _as_array(value) -> Array:
	return value if value is Array else []

func _as_dictionary(value) -> Dictionary:
	return value if value is Dictionary else {}

func _incoming_source_for_target(actor_id: String) -> Dictionary:
	for source in _view.damage_sources:
		if str(source.get("target_actor_id", "")) == actor_id and _source_in_focus(source): return source
	for source in _as_array(_view.settled_damage.get("sources", [])):
		if str(source.get("target_actor_id", "")) == actor_id and _source_in_focus(source): return source
	return {}

func inspection_state() -> Dictionary:
	return {
		"ready": true,
		"battle_id": _view.battle_id,
		"status": _view.status,
		"battle_result": _view.battle_result,
		"learned_battle_mode": learned_battle_mode,
		"learned_human_seat": learned_human_seat,
		"learned_seed": learned_seed,
		"learned_policy": _view.learned_policy,
		"model_thinking": _model_thinking,
		"model_timeout_warning": _model_timeout_warning,
		"model_error": _model_error,
		"reaction_notice": _reaction_notice,
		"round": _view.round_number,
		"segment": _view.segment,
		"stage": _view.stage,
		"pending_input": _view.viewer_pending(),
		"snapshot": _view.raw_snapshot,
		"events": _view.events,
		"presentation_active": _director.has_beats(),
		"presentation_type": str(_director.peek().get("type", "")) if _director.has_beats() else "",
		"presentation_beat": _director.peek() if _director.has_beats() else {},
		"submitting": _submitting,
		"selected_dice": _selected_indices,
		"selected_source": _selected_source,
		"focused_enemy": _focused_enemy,
		"selected_card": _selected_card,
		"defense_rolls": _view.defense_rolls,
		"defense_selections": _view.defense_selections,
		"offensive_reveals": _view.offensive_reveals,
		"rolled_dice": {"blade": _view.rolled_dice("blade"), _focused_enemy: _view.rolled_dice(_focused_enemy)},
		"hand_limit_selection": _hand_limit_selection,
		"loaded_snapshot_name": loaded_snapshot_name,
		"snapshot_panel_open": _snapshot_panel_open,
		"selected_snapshot_name": _selected_snapshot_name,
		"history_enabled": _history_tools_enabled(),
		"history_review": _history_review,
		"history_replay": _history_replay,
		"history_point_id": _history_point_id,
		"history_origin_battle_id": _history_origin_battle_id,
		"history_branch": _history_branch,
		"history_points": _history_entries,
		"history_scroll_value": _history_scroll_value,
		"history_follow_latest": _history_follow_latest,
		"history_divergence_pending": _history_pending_divergence,
		"authority_transcript": _transcript_panel.inspection_state() if is_instance_valid(_transcript_panel) else {"enabled": false, "open": false},
		"error": _error_message,
	}

func _show_venom_choices(actions: Array, heading: String) -> void:
	if actions.is_empty() or _submitting or _history_review: return
	var dialog := AcceptDialog.new()
	add_child(dialog)
	dialog.title = heading
	dialog.get_ok_button().text = "Cancel"
	dialog.min_size = Vector2i(520, 260)
	var scroll := ScrollContainer.new()
	scroll.custom_minimum_size = Vector2(510, 250)
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	dialog.add_child(scroll)
	var list := VBoxContainer.new()
	list.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	list.add_theme_constant_override("separation", 8)
	scroll.add_child(list)
	var first_payload: Dictionary = actions[0].get("payload", {})
	var first_cards: Array = first_payload.get("card_ids", first_payload.get("commitment", {}).get("card_ids", []))
	var rules := ""
	for entry in _view.hand_cards():
		if str(entry.instance_id) in first_cards:
			rules = BattlePresentationCatalog.card(str(entry.definition_id)).text
	if rules.is_empty() and first_payload.has("ability_id"):
		rules = BattlePresentationCatalog.ability(str(first_payload.ability_id), _view.actor("blade")).text
	if not rules.is_empty():
		var description := Label.new()
		description.text = rules
		description.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		description.custom_minimum_size.x = 480
		list.add_child(description)
	for action in actions:
		var button := Button.new()
		button.text = _venom_choice_label(action)
		button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		button.custom_minimum_size.y = 48
		button.pressed.connect(func():
			dialog.hide()
			dialog.queue_free()
			_send(JSON.stringify(action))
		)
		list.add_child(button)
		_inspect(button, "battle.venom.choice.%d" % list.get_child_count(), button.text)
	dialog.confirmed.connect(dialog.queue_free)
	dialog.canceled.connect(dialog.queue_free)
	dialog.popup_centered(Vector2i(560, mini(560, 130 + actions.size() * 58)))

func _venom_choice_label(action: Dictionary) -> String:
	var payload: Dictionary = action.get("payload", {})
	var ability := str(payload.get("ability_id", ""))
	var commitment: Dictionary = payload.get("commitment", {})
	var targets: Array = payload.get("target_ids", commitment.get("proposal_ids", []))
	var key := str(payload.get("status_id", commitment.get("choice_id", "")))
	var text := "Confirm"
	var card_ids: Array = payload.get("card_ids", commitment.get("card_ids", []))
	for card in _view.hand_cards():
		if str(card.instance_id) in card_ids and str(card.definition_id) == "agitate":
			var count := 0
			for status in _view.actor(_focused_enemy).get("statuses", []):
				if str(status.get("definition_id", "")) == key: count += int(status.get("stacks", 0))
			return "Check all %d %s stack%s" % [count, BattlePresentationCatalog.status(key).name, "s" if count != 1 else ""]
	if not ability.is_empty():
		text = BattlePresentationCatalog.ability(ability).name
		var tier := str(payload.get("tier_id", ""))
		if tier.begins_with("fang_"):
			var n := int(tier.trim_prefix("fang_"))
			text = "%d Fang · %d damage + %d Poison" % [n, 7 - n + int(_view.actor("blade").get("needlefang_damage_bonus", 0)), n - 2]
		var toxins: Array = payload.get("toxin_choices", [])
		if not toxins.is_empty():
			var labels: Array[String] = []
			for status_id in toxins: labels.append(BattlePresentationCatalog.status(str(status_id)).name)
			text += " · Provoke " + " + ".join(labels)
		if payload.get("spend_catalyst", false): text += " · Pay 1 Catalyst for +2 prevention"
	elif key.begins_with("toxin:"):
		var index := int(key.split(":")[1])
		var roll: Dictionary = _view.effect_rolls[index] if index < _view.effect_rolls.size() else {}
		text = "%s die %d · showing %d" % [BattlePresentationCatalog.status(str(roll.get("source_content_id", "poison"))).name, index + 1, int(roll.get("die", {}).get("face", 0))]
	elif key.count(":") == 2:
		var parts := key.split(":")
		var owner := "Your" if parts[0] in ["blade", learned_human_seat] else _actor_display_name(parts[0])
		text = "%s die %d → face %s" % [owner, int(parts[1]) + 1, parts[2]]
	elif key.contains("poison") and key != "prevent":
		var labels: Array[String] = []
		for status_id in key.split(","): labels.append(BattlePresentationCatalog.status(status_id).name)
		text = " + ".join(labels)
		if targets.size() == 1 and str(targets[0]).begins_with("source-"): text = "Prevent 2 + pay 1 Catalyst to remove 1 " + text
	elif key == "incubation": text = "Prevent 2 + pay 1 Catalyst to remove 1 Incubation"
	elif key.begins_with("prevent|"): text = "Spend Poison from " + _actor_display_name(key.trim_prefix("prevent|")) + " · prevent damage"
	elif key == "prevent": text = "Prevent damage"
	for source in _view.damage_sources + _as_array(_view.settled_damage.get("sources", [])):
		if str(source.get("id", "")) in targets:
			text += " · " + _actor_display_name(str(source.get("source_actor_id", ""))) + " · " + BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))).name
			break
	return text

func _build_venom_status() -> void:
	var title := Label.new()
	title.text = "STATUS APPLICATION"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_font_size_override("font_size", 26)
	_center.add_child(title)
	var work: Dictionary = _view.raw_snapshot.get("venom_work", {})
	var detail := Label.new()
	detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	if work.get("Kind", "") == "conversion":
		detail.text = "Convert 1 surviving Poison into Volatile Poison on %s. The upgrade requires an open Volatile Poison slot." % _actor_display_name(str(work.get("TargetActorID", "")))
	else:
		detail.text = "Apply %d %s to %s." % [int(work.get("Stacks", 1)), BattlePresentationCatalog.status(str(work.get("StatusID", "incubation"))).name, _actor_display_name(str(work.get("TargetActorID", "")))]
	_center.add_child(detail)

func _build_automatic_effects(beat: Dictionary) -> void:
	var key := "%s:%s:%s" % [_view.battle_id, beat.get("event", {}).get("round", 0), beat.get("event", {}).get("sequence", 0)]
	if key != _effects_beat_key: _effects_elapsed = -COMBAT_TIMING.effects_gather(); _effects_beat_key = key
	var panel := preload("res://presentation/battle/automatic_effects.gd").new()
	if _multiple_enemies():
		var lane := preload("res://presentation/battle/combat_lane.gd").new(); _center.add_child(lane); lane.body.add_child(panel)
	else: _center.add_child(panel)
	panel.visible_actor_ids = [viewer_actor_id] + _enemy_ids()
	panel.configure(_as_dictionary(beat.get("event", {}).get("data", {})), _actor_names())
	_effects_panel = panel
	panel.set_meta("flow_key", "effects")
	panel.resume_at(_effects_elapsed)
	panel.finished.connect(_finish_automatic_effects.bind(_income_animation_generation))
	panel.phase_changed.connect(_show_effects_profiles.bind(panel.summary))
	_start_automatic_effects.call_deferred(panel, _income_animation_generation)

func _start_automatic_effects(panel: Control, generation: int) -> void:
	if generation != _income_animation_generation or not is_instance_valid(panel): return
	# The gameplay debug checkbox does not reintroduce Effects decisions.
	# Snapshot/history tools still freeze a presentation while inspecting it.
	panel.set_paused(_history_review or _snapshot_panel_open)
	var gather := preload("res://presentation/battle/effects_gather.gd").new(); _root.add_child(gather); gather.configure(panel, _actor_profiles)
	var catalyst := preload("res://presentation/battle/catalyst_trail.gd").new(); _root.add_child(catalyst); catalyst.configure(panel, _actor_profiles)

func _show_effects_profiles(phase: String, progress: float, summary: Dictionary) -> void:
	for actor_id in _actor_profiles:
		var before: Dictionary = summary.get("actors_before", {}).get(actor_id, {})
		var after: Dictionary = summary.get("actors_after", {}).get(actor_id, {})
		if before.is_empty() or after.is_empty(): continue
		_actor_profiles[actor_id].show_effects_progress(before, after, phase, progress)

func _finish_automatic_effects(generation: int) -> void:
	if generation != _income_animation_generation or _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	if _director.peek().get("type") == "effects_resolved": _advance_beat()

func _inline_status_application() -> bool:
	return _view.stage == "venom_status_reaction" and _view.raw_snapshot.get("venom_work", {}).get("Kind", "") == "application"

func _board_stage() -> String:
	return str(_view.raw_snapshot.get("presentation_stage", "defense_reaction" if _view.segment == "defensive" else "planning")) if _inline_status_application() else _view.stage

func _defense_roll_action() -> Dictionary:
	if _view.stage != "defense_roll" or _view.legal_actions.size() != 1: return {}
	if _submitting or _model_thinking or _model_error or not _error_message.is_empty(): return {}
	if _director.has_beats() or _history_review or _history_replay or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return {}
	if bool(_view.learned_policy.get("model_turn", false)): return {}
	var action: Dictionary = _view.legal_actions[0]
	if action.get("type") != "roll_dice" or action.get("actor_id") != viewer_actor_id or not _view.allowed("roll_dice"): return {}
	if str(action.get("payload", {}).get("pending_input_id", "")) != str(_pending().get("id", "")): return {}
	return action

func _auto_roll_defense_if_only_action() -> bool:
	var action := _defense_roll_action()
	if action.is_empty():
		_defense_auto_roll_key = ""
		return false
	_defense_auto_roll_key = _view.battle_id + ":" + str(_pending().get("id", ""))
	if _defense_auto_roll_key == _last_defense_auto_roll: return false
	_last_defense_auto_roll = _defense_auto_roll_key
	_send(JSON.stringify(action))
	return true

func _build_automatic_defense_roll() -> void:
	# Used only when opening a saved roll checkpoint. Never show a second mat.
	_build_compact_defense_results()

func _continuous_damage_response() -> bool:
	return _view.stage in ["damage_reaction", "status_damage_reaction"]

func _damage_batch_key() -> String:
	return _view.battle_id + ":" + str(_view.settled_damage.get("id", "%s:%d" % [_view.stage, _view.round_number]))

func _capture_damage_feedback(result: Dictionary, previous_damage: Dictionary) -> void:
	if _history_review or _history_replay: return
	for emitted in result.get("events", []):
		if emitted.get("type") != "damage_prevented_or_modified": continue
		var data: Dictionary = emitted.get("data", {})
		var card_id := str(data.get("card_definition_id", ""))
		if card_id.is_empty(): continue
		var before := int(data.get("damage_before", 0))
		var after := int(data.get("damage_after", 0))
		var actor_id := str(emitted.get("actor_id", ""))
		var card_name := str(BattlePresentationCatalog.card(card_id).name)
		var text := "%s played %s · damage %d → %d" % [_actor_display_name(actor_id), card_name, before, after]
		_damage_feedback = {"battle_id": _view.battle_id, "card_id": card_id, "instance_id": str(data.get("card_instance_id", "")), "started_ms": Time.get_ticks_msec(), "text": text, "before": before, "after": after, "saved": []}
		var still_pending := {}
		for removal in _as_array(_view.settled_damage.get("removals", [])):
			if removal.get("accepted", false) and not removal.get("released", false): still_pending[str(removal.get("card_id", ""))] = true
		for removal in _as_array(previous_damage.get("removals", [])):
			if removal.get("accepted", false) and not removal.get("released", false) and not still_pending.has(str(removal.get("card_id", ""))) and str(removal.get("card_id", "")) != str(data.get("card_instance_id", "")):
				# Only previously public damage cards are used; never inspect the opponent's hand.
				_damage_feedback.saved.append(removal.duplicate(true))
		_reaction_card_feedback = {"battle_id": _view.battle_id, "source_id": str(data.get("source_id", "")), "expires_ms": Time.get_ticks_msec() + 1800, "text": text}

func _build_damage_feedback() -> void:
	if _damage_feedback.get("battle_id") != _view.battle_id or not _reaction_feedback_active() or _history_review or _history_replay: return
	var panel := preload("res://presentation/battle/damage_response_feedback.gd").new()
	_enemy_attack_dock.add_child(panel)
	panel.configure(_damage_feedback)
	_fit_damage_feedback.call_deferred(panel)
	_inspect(panel, "battle.damage_response_feedback", str(_damage_feedback.text))

func _fit_damage_feedback(panel: Control) -> void:
	if not is_instance_valid(panel): return
	panel.scale = Vector2.ONE * minf(1.0, 380.0 / maxf(1.0, panel.get_combined_minimum_size().x))

func _pass_hands_off_priority() -> bool:
	return bool(_view.raw_snapshot.get("pass_hands_off_priority", false))

func _quiet_offensive_reaction() -> bool:
	if _view.stage != "offensive_reaction" or _history_review or _history_replay or _snapshot_panel_open or _model_error or not _error_message.is_empty() or _director.has_beats(): return false
	if _model_thinking or bool(_view.learned_policy.get("model_turn", false)): return true
	return _view.legal_actions.size() == 1 and _view.legal_actions[0].get("type") == "pass"

# Enemy focus changes presentation and future targeting, never actor identity.
func _enemy_ids() -> Array:
	var ids: Array = _view.actors.keys().filter(func(id): return str(id) != viewer_actor_id)
	ids.sort()
	return ids

func _multiple_enemies() -> bool:
	return _enemy_ids().size() > 1

func _actor_names() -> Dictionary:
	var names := {}
	for id in _view.actors: names[id] = _actor_display_name(str(id))
	return names

func _status_counts() -> Dictionary:
	var counts := {}
	for id in _view.actors:
		counts[id] = {}
		for status in _view.actor(id).get("statuses", []): counts[id][str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
	return counts

func _source_handled(id: String) -> bool:
	return _view.raw_snapshot.get("defense_history", {}).has(id)

func _incoming_from(enemy: String, unhandled_only: bool = false) -> Array:
	return _view.damage_sources.filter(func(source): return source.get("source_actor_id") == enemy and source.get("target_actor_id") == viewer_actor_id and (not unhandled_only or not _defense_source_chosen(str(source.get("id", "")))))

func _focused_defense_available() -> bool:
	return not _incoming_from(_focused_enemy, true).is_empty()

func _sync_enemy_focus() -> void:
	var enemies := _enemy_ids()
	if enemies.is_empty(): return
	if _focused_enemy not in enemies: _focused_enemy = str(enemies[0])
	if not _multiple_enemies(): return
	if _view.stage == "planning" and _view.actor(_focused_enemy).get("defeat_state") == "defeated":
		for id in enemies:
			if _view.actor(id).get("defeat_state") != "defeated": _focused_enemy = str(id); break
	if _view.stage == "defense_selection":
		var history: Dictionary = _view.raw_snapshot.get("defense_history", {})
		var key := "%s:%d:%d:%d" % [_view.battle_id, _view.round_number, history.size(), _view.raw_snapshot.get("defense_plans", {}).size()]
		if key != _defense_focus_key and not _history_review:
			_defense_focus_key = key
			if not _focused_defense_available():
				for id in enemies:
					if not _incoming_from(str(id), true).is_empty(): _focused_enemy = str(id); break
		var incoming := _incoming_from(_focused_enemy, true)
		if not incoming.any(func(source): return source.get("id") == _selected_source): _selected_source = str(incoming[0].id) if not incoming.is_empty() else ""
	elif _view.stage in ["defense_roll", "defense_reaction"]:
		var active := str(_view.defense_selections.get(viewer_actor_id, {}).get("source_id", ""))
		var key := "%s:%d:%s" % [_view.battle_id, _view.round_number, active]
		if not active.is_empty() and key != _defense_playback_source:
			_defense_playback_source = key
			for source in _view.damage_sources:
				if source.get("id") == active:
					_focused_enemy = str(source.get("source_actor_id")); _selected_source = active; break
	elif _selected_source.is_empty():
		var incoming := _incoming_from(_focused_enemy)
		if not incoming.is_empty(): _selected_source = str(incoming[0].id)

func _select_enemy(id: String) -> void:
	if id not in _enemy_ids() or id == _focused_enemy: return
	_focused_enemy = id
	_selected_source = ""
	_reset_auto_pass_preview()
	_render(true)

func _build_enemy_selector(scenery: Control) -> void:
	_enemy_buttons.clear()
	if not _multiple_enemies(): return
	var entries: Array = []
	for id in _enemy_ids():
		var actor := _profile_actor(str(id))
		var fighter: TextureRect
		for child in scenery.get_children():
			if child.get_meta("actor_id", "") == id: fighter = child; break
		if fighter == null: continue
		var incoming := _incoming_from(str(id))
		var defeated: bool = actor.get("defeat_state") == "defeated"
		var indicator := "Ready"
		if defeated: indicator = "Defeated"
		elif not incoming.is_empty(): indicator = "Attacking" if not _incoming_from(str(id), true).is_empty() else "Handled"
		elif _view.stage != "planning" and _view.segment in ["offensive", "defensive", "damage_resolution"]: indicator = "Miss"
		entries.append({"id": id, "name": _actor_display_name(str(id)), "health": int(actor.get("current_health", 0)), "maximum": int(actor.get("max_health", 1)), "rect": fighter.get_rect(), "state": indicator, "attacking": indicator == "Attacking", "defeated": defeated})
	var selector := preload("res://presentation/battle/enemy_selector.gd").new(); _root.add_child(selector)
	selector.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	selector.configure(entries, _focused_enemy); selector.enemy_selected.connect(_select_enemy)
	# The authority may already be in next-round planning while Damage plays.
	# Keep battlefield labels behind the same visible phase as the result panels.
	selector.visible = not _director.has_beats() and _view.segment == "offensive"
	_enemy_selector = selector; _enemy_buttons = selector.buttons
	for id in _enemy_buttons: _inspect(_enemy_buttons[id], "battle.enemy.select." + str(id), _enemy_buttons[id].tooltip_text)

func _source_in_focus(source: Dictionary) -> bool:
	if not _multiple_enemies() or _view.segment != "offensive" or _director.has_beats(): return true
	if source.get("target_actor_id") == viewer_actor_id: return source.get("source_actor_id") == _focused_enemy
	return source.get("target_actor_id") == _focused_enemy

func _action_in_focus(action: Dictionary) -> bool:
	if not _multiple_enemies() or _view.segment != "offensive": return true
	var payload: Dictionary = action.get("payload", {})
	var commitment: Dictionary = payload.get("commitment", {})
	var targets: Array = payload.get("target_ids", commitment.get("target_ids", commitment.get("proposal_ids", [])))
	for target in targets:
		if target in _enemy_ids() and target != _focused_enemy: return false
		for source in _view.damage_sources + _as_array(_view.settled_damage.get("sources", [])):
			if source.get("id") == target and not _source_in_focus(source): return false
	return true

func _display_actor_id(id: String) -> String:
	if learned_battle_mode:
		if id == learned_human_seat: return viewer_actor_id
		if id == str(_view.learned_policy.get("model_seat", "")): return "goblin"
		if id == "seat-c" and _view.actors.has("goblin-2"): return "goblin-2"
	return id

func _battlefield_visual() -> BattleEncounterVisual:
	var original: BattleEncounterVisual = encounter_visual if encounter_visual != null else preload("res://content/battle_visuals/library.tres").default_encounter
	if not _multiple_enemies(): return original
	var layout: BattleEncounterVisual = original.duplicate(true)
	var template: BattleFighterPlacement
	var kept: Array[BattleFighterPlacement] = []
	for placement in layout.fighters:
		if placement.actor_id in _enemy_ids():
			if template == null: template = placement
		else: kept.append(placement)
	if template == null:
		template = BattleFighterPlacement.new(); template.ground_position = Vector2(1380, 610)
	var enemies := _enemy_ids()
	for i in enemies.size():
		var placement: BattleFighterPlacement = template.duplicate(true)
		placement.actor_id = str(enemies[i]); placement.instance_id = "enemy" if i == 0 else "enemy_" + str(i + 1)
		# A compact staggered group stays inside the existing enemy battlefield.
		# Rows overlap in depth; adding actors never creates another UI column.
		placement.ground_position = template.ground_position + Vector2(-170 * (i % 3), -88 * (i / 3) + (36 if i % 3 == 0 else 0))
		placement.draw_order = 12 - i % 3 - (i / 3) * 3
		kept.append(placement)
	layout.fighters = kept
	return layout

func _display_damage_sources(sources: Array) -> Array:
	return sources

func _damage_cards_by_source(sources: Array, removals: Array) -> Dictionary:
	# The authority supplies a shared ordered removal batch for each defender.
	# Partition that batch by the individual final damage amounts, independent
	# of UI focus, so toggling never duplicates cards or shows another hit's loss.
	var cards := {}; var remaining := {}
	for source in sources:
		var id := str(source.get("id", ""))
		cards[id] = []
		remaining[id] = int(source.get("final_amount", maxi(0, int(source.get("base_amount", 0)) - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))))
	for removal in removals:
		if not removal.get("accepted", false) or removal.get("released", false): continue
		var ids: Array = removal.get("damage_proposal_ids", [])
		for source in sources:
			var id := str(source.get("id", ""))
			if source.get("target_actor_id") != removal.get("target_actor_id") or int(remaining[id]) <= 0: continue
			if not ids.is_empty() and id not in ids: continue
			cards[id].append(removal); remaining[id] -= 1; break
	return cards

func _defense_source_chosen(id: String) -> bool:
	return _source_handled(id) or _view.raw_snapshot.get("defense_plans", {}).has(id)

func _defense_final_review() -> bool:
	if not _multiple_enemies() or _view.stage != "defense_reaction": return false
	var incoming: Array = _view.damage_sources.filter(func(source): return source.get("target_actor_id") == viewer_actor_id)
	if incoming.size() < 2 or not _view.raw_snapshot.get("defense_plans", {}).is_empty(): return false
	var active := str(_view.defense_selections.get(viewer_actor_id, {}).get("source_id", ""))
	return incoming.all(func(source): return _source_handled(str(source.id)) or str(source.id) == active)

func _build_source_defense_choices(panel: Control, source_id: String) -> void:
	var row := GridContainer.new(); row.columns = 2; row.add_theme_constant_override("h_separation", 6); row.add_theme_constant_override("v_separation", 6)
	panel._body.add_child(row)
	for action in _view.legal_actions:
		var payload: Dictionary = action.get("payload", {})
		if action.get("type") != "planning_select_ability" or source_id not in payload.get("target_ids", []): continue
		var ability := BattlePresentationCatalog.ability(str(payload.get("ability_id", "")))
		var button := Button.new(); button.size_flags_horizontal = Control.SIZE_EXPAND_FILL; button.custom_minimum_size.y = 44
		button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; button.add_theme_font_size_override("font_size", 16)
		button.text = ability.name
		if payload.get("ability_id") == "shedskin": button.text += " · 1 Catalyst (+2 block)" if payload.get("spend_catalyst", false) else " · No Catalyst"
		else:
			var cost := int(_view.content_definition("abilities", str(payload.get("ability_id", ""))).get("cost", {}).get("energy", 0))
			if cost > 0: button.text += " · %d Energy" % cost
		button.tooltip_text = ability.text; button.disabled = _submitting or _history_review or _model_thinking
		button.pressed.connect(func(): _selected_source = source_id; _send(JSON.stringify(action)))
		row.add_child(button)
		_inspect(button, "battle.defense.choose.%s.%s.%s" % [source_id, payload.get("ability_id", ""), str(payload.get("spend_catalyst", false))], ability.text)
