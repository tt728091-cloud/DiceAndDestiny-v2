extends Control

const SEGMENTS := [["ongoing_effects", "Effects"], ["income", "Income"], ["offensive", "Offensive"], ["defensive", "Defensive"], ["damage_resolution", "Damage"]]
const INCOME_DURATION_SETTING := "dice_and_destiny/presentation/income_animation_seconds"
const MODEL_TIMEOUT_MS := 2000
const MIN_THINKING_DISPLAY_MS := 180
const TRANSCRIPT_PANEL := preload("res://devtools/developer_authority_transcript_panel.gd")

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
const DAMAGE_AUTO_PASS_REVIEW_MS := 2500
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
	if elapsed < MIN_THINKING_DISPLAY_MS:
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
	if _provoked_toxin_reaction() and is_instance_valid(_provoked_panel) and not _provoked_panel.ready_to_continue(): return
	var action := _sole_pass_action()
	if action.is_empty():
		_reset_auto_pass_preview()
		return
	var key := _view.battle_id + ":" + str(_pending().get("id", ""))
	if key == _last_auto_pass_input: return
	var review_ms := 0
	if _view.stage == "defense_reaction":
		if not _defense_result_panels.is_empty() and _defense_reviewed_result != _defense_review_key(): review_ms = ceili(DEFENSE_TIMING.total_seconds() * 1000.0)
	elif _continuous_damage_response():
		if _damage_reviewed_batch != _damage_batch_key(): review_ms = DAMAGE_AUTO_PASS_REVIEW_MS
	# Even a no-choice handoff can be the last human checkpoint before the
	# opponent passes and the authority finishes defense/damage in one command.
	# Show the defense first, then allow subsequent handoffs without replaying it.
	if _pass_hands_off_priority() and _view.stage != "defense_reaction": review_ms = 0
	if review_ms > 0:
		var now := Time.get_ticks_msec()
		if key != _auto_pass_preview_input:
			_reset_auto_pass_preview()
			_auto_pass_preview_input = key
			_auto_pass_preview_started_ms = now
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

func _schedule_model_if_needed(result: Dictionary) -> void:
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
	_director.queue_result(result, _director.last_sequence())
	_render()
	call_deferred("_schedule_model_if_needed", result)

func _render() -> void:
	# Roll commands are authority work, not a separate visual beat. Keep the
	# selected board for this handoff; both rolls animate in the result panels.
	if _view.stage == "defense_roll" and is_instance_valid(_root) and not _history_review and not _history_replay and not _snapshot_panel_open and not _model_error and _error_message.is_empty():
		for control in _root.find_children("*", "BaseButton", true, false):
			if not control.get_meta("battle_utility", false): control.disabled = true
		return
	# Do not paint a decision screen for an acknowledgement-only offensive reveal.
	# Keep the selected attack board while authority/model handoffs run underneath.
	if _quiet_offensive_reaction() and is_instance_valid(_root):
		if _view.rolls_used("blade") == 0: _show_skipped_offense_hint()
		for control in _root.find_children("*", "BaseButton", true, false):
			if control != _auto_pass_toggle and not control.get_meta("battle_utility", false): control.disabled = true
		if is_instance_valid(_auto_pass_button): _auto_pass_button.hide()
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
	_selected_attack_tiles.clear()
	_actor_profiles.clear()
	_defense_result_panels.clear()
	_income_drawn_cards.clear()
	if is_instance_valid(_root): _root.queue_free()
	_root = Control.new(); _root.name = "CinematicBattle"; add_child(_root)
	_root.theme = theme
	_layout_cinematic_root()
	var background := TextureRect.new(); background.set_anchors_preset(Control.PRESET_FULL_RECT); background.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; background.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; background.mouse_filter = Control.MOUSE_FILTER_IGNORE
	background.texture = load("res://assets/battle/wasteland/duel.png" if _cinematic_venom_duel() else "res://assets/battle/wasteland/arena.png")
	_root.add_child(background)
	var shade := ColorRect.new(); shade.color = Color(0.025, 0.025, 0.025, 0.06); shade.set_anchors_preset(Control.PRESET_FULL_RECT); shade.mouse_filter = Control.MOUSE_FILTER_IGNORE; _root.add_child(shade)
	_build_cinematic_fighter("blade", Rect2(550, 310, 360, 390))
	_build_cinematic_fighter("goblin", Rect2(1240, 160, 360, 420))
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
	var full_height: bool = _director.peek().get("type") == "effects_resolved" or (_view.hand_cards().is_empty() and _view.stage != "planning")
	_center_scroll = ScrollContainer.new(); _center_scroll.name = "BattleContentScroll"; _root.add_child(_center_scroll)
	_place_cinematic(_center_scroll, Rect2(460, 215, 1030, 775) if full_height else Rect2(460, 215, 1030, 550))
	_center_scroll.follow_focus = true
	if _view.segment == "damage_resolution": _center_scroll.add_theme_stylebox_override("panel", CINEMATIC.panel(Color("10171dcc"), Color("89775280"), 12))
	_center = VBoxContainer.new(); _center.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_theme_constant_override("separation", 10); _center_scroll.add_child(_center)
	_build_cinematic_utilities()
	var header := _cinematic_box("PhaseRail", Rect2(570, 18, 780, 130))
	_build_header(header)
	_build_player_column(null)
	_build_side_attacks()
	_build_center()
	_build_enemy_column(null)
	_player_profile_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_enemy_profile_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_enemy_dice_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	_player_dice_dock.minimum_size_changed.connect(_layout_left_controls, CONNECT_DEFERRED)
	call_deferred("_layout_left_controls")
	# Keep the character's ability reference accessible during every segment.
	if _ability_dock.get_child_count() == 0:
		_build_ability_row("YOUR ABILITIES", _as_array(_view.actor("blade").get("offensive_abilities", [])), "blade")
	if _utility_contents.enemy.get_child_count() == 0:
		_build_ability_row("ENEMY ABILITIES", _as_array(_view.actor("goblin").get("offensive_abilities", [])), "goblin")
	_build_ability_row("ENEMY DEFENSES", _as_array(_view.actor("goblin").get("defensive_abilities", [])), "goblin")
	if _history_tools_enabled(): _build_history_bar(_utility_contents.history)
	for key in _utility_panels: _root.move_child(_utility_panels[key], _root.get_child_count() - 1)
	for update in _director.take_status_updates():
		if _history_review or _history_replay: continue
		var notice := preload("res://presentation/battle/status_change_notice.gd").new()
		notice.configure(self, update)
		add_child(notice)
	if not _defense_result_panels.is_empty(): call_deferred("_start_defense_status_flights", _income_animation_generation)
	_build_error(_center)
	_center_scroll.set_deferred("scroll_vertical", saved_scroll)
	if _snapshot_panel_open: _build_snapshot_panel()
	if not _history_pending_divergence.is_empty(): _build_history_divergence_panel()

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
	_enemy_dice_dock.position.y = top
	_enemy_dice_dock.size.y = 100.0
	_player_dice_dock.position.y = top
	_player_dice_dock.size.y = 100.0
	_enemy_attack_dock.get_parent().position.y = top + 113.0
	_enemy_attack_dock.get_parent().size.y = maxf(80.0, 985.0 - top - 113.0)
	if _utility_panels.has("log"):
		var log_top := _enemy_dice_dock.position.y + _enemy_dice_dock.size.y + 18.0
		_place_cinematic(_utility_panels.log, Rect2(_enemy_dice_dock.position.x, log_top, _enemy_dice_dock.size.x, maxf(80.0, 985.0 - log_top)))
	_roll_dock.position.y = top + 113.0
	_action_footer.get_parent().position.y = top + 113.0
	var rail := _ability_dock.get_parent() as ScrollContainer
	rail.position.y = top + 203.0
	rail.size.y = minf(370.0 if _view.segment == "offensive" else 600.0, maxf(80.0, 1055.0 - rail.position.y))

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

func _build_cinematic_fighter(actor_id: String, rect: Rect2) -> void:
	if _cinematic_venom_duel(): return
	# Other character matchups retain their own portrait rather than inheriting
	# the Venom/Blade Warden artwork from the reference scene.
	var definition := str(_view.actor(actor_id).get("definition_id", ""))
	var path := "res://assets/battle/portraits/%s.png" % definition
	if definition == "venom": path = "res://assets/battle/portraits/venom.svg"
	if not ResourceLoader.exists(path): return
	var fighter := TextureRect.new(); fighter.name = "Fighter_" + actor_id
	fighter.mouse_filter = Control.MOUSE_FILTER_IGNORE; fighter.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; fighter.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	fighter.texture = load(path)
	_root.add_child(fighter); _place_cinematic(fighter, rect)

func _cinematic_venom_duel() -> bool:
	return _view.actor("blade").get("definition_id") == "venom" and _view.actor("goblin").get("definition_id") == "blade_warden"

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
	var display_stage := "attacks_selected" if _quiet_offensive_reaction() else _board_stage()
	if display_stage in ["defense_roll", "defense_reaction"]: display_stage = "defense"
	if _provoked_toxin_reaction(): display_stage = "provoked_toxins"
	if _director.has_beats():
		var beat: Dictionary = _director.peek()
		var beat_event: Dictionary = _as_dictionary(beat.get("event", {}))
		var event_segment := str(beat.get("presentation_segment", ""))
		if event_segment.is_empty(): event_segment = str(beat_event.get("segment", beat_event.get("to", "")))
		if not event_segment.is_empty(): display_segment = event_segment
		if beat.get("type") == "defense_selected": display_stage = "defense_reveal"
		elif beat.get("type") == "effects_resolved": display_stage = "effects"
		elif beat.get("type") == "income_summary": display_stage = "income_results"
		elif beat.get("type") == "card_cleanse": display_stage = "card_played"
		elif beat.get("type") == "poison_conversion": display_stage = "poison_upgraded"
		elif beat.get("type") == "segment_entered": display_stage = "presentation"
	for pair in SEGMENTS:
		var label := Label.new(); label.text = "●\n%s" % pair[1]; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", 18)
		label.add_theme_color_override("font_color", Color("ffe3a0") if display_segment == pair[0] else Color("b4ab9b")); bar.add_child(label)
	var rule := HSeparator.new(); parent.add_child(rule)
	var round := Label.new(); round.text = "Round %d · %s" % [_view.round_number, display_stage.replace("_", " ").capitalize()]; round.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; round.add_theme_color_override("font_color", CINEMATIC.INK); round.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); round.add_theme_font_size_override("font_size", 27); round.add_theme_stylebox_override("normal", CINEMATIC.paper()); round.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; parent.add_child(round); parent.move_child(round, 0)
	if learned_battle_mode:
		var policy_badge := Label.new()
		var policy_schema := str(_view.learned_policy.get("observation_schema", ""))
		var policy_label := "NEW V2" if policy_schema == "dice-and-destiny-observation-v2" else "OLD V1"
		policy_badge.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		policy_badge.text = "LEARNED BATTLE · %s · HUMAN %s" % [policy_label, learned_human_seat.to_upper()]
		policy_badge.add_theme_color_override("font_color", Color("9de0ff"))
		policy_badge.tooltip_text = "Frozen policy %s · no training or fallback" % str(_view.learned_policy.get("model_id", "unknown"))
		_utility_contents.inspect.add_child(policy_badge)
		_inspect(policy_badge, "battle.learned_policy.badge", policy_badge.tooltip_text)
	if _snapshot_tools_enabled():
		var snapshots := Button.new(); snapshots.text = "DEV SNAPSHOTS"; snapshots.pressed.connect(_toggle_snapshot_panel); _utility_contents.inspect.add_child(snapshots)
		_inspect(snapshots, "battle.dev_snapshots.toggle", "Open the developer snapshot controls")
	if is_instance_valid(_transcript_panel):
		var transcript := Button.new(); transcript.text = "DEV TRANSCRIPT"; transcript.pressed.connect(_transcript_panel.toggle_panel); _utility_contents.inspect.add_child(transcript)
		_inspect(transcript, "battle.dev_transcript.toggle", "Open the read-only authority transcript panel")

func _build_player_column(_parent: HBoxContainer) -> void:
	var column := _player_profile_dock
	var profile := ActorProfile.new(); column.add_child(profile); profile.display("blade", _view.actor("blade"), true); _actor_profiles["blade"] = profile
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
	var caption := "YOUR DICE · %d/%d rolls · %d left" % [_view.rolls_used("blade"), _view.max_rolls("blade"), maxi(0, _view.max_rolls("blade") - _view.rolls_used("blade"))]
	if _view.segment == "offensive" and _view.stage == "offensive_reaction":
		caption = "YOUR DICE · Planning finished"
		if _view.rolls_used("blade") == 0: caption = "YOUR DICE · Skipped without rolling"
	dice.display(rolled, kept, planning, caption, "battle.die.blade")
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
	var profile := ActorProfile.new(); column.add_child(profile); profile.display("goblin", _view.actor("goblin"), false); _actor_profiles["goblin"] = profile
	if learned_battle_mode:
		profile.title.text = "Blade Warden"
		profile.title.tooltip_text = "Learned Policy"
	var income := _income_actor_data("goblin")
	if not income.is_empty(): profile.prepare_income(income)
	else:
		var upcoming_income := _upcoming_income_actor_data("goblin")
		if not upcoming_income.is_empty(): profile.prepare_before_income(upcoming_income)
	if _director.peek().get("type") != "effects_resolved":
		var enemy_dice := _view.rolled_dice("goblin"); var enemy_reveal := _view.offensive_reveal("goblin"); var enemy_caption := "OPPONENT DICE" if learned_battle_mode else "ENEMY DICE"
		if not enemy_dice.is_empty(): enemy_caption += " · Simulated rolls %d" % int(enemy_reveal.get("simulated_rolls", enemy_reveal.get("rolls_used", 0)))
		var dice := BattleDiceTray.new(); _enemy_dice_dock.add_child(dice); dice.display(enemy_dice, [], false, enemy_caption, "battle.die.goblin")
	_log = RichTextLabel.new(); _log.size_flags_vertical = Control.SIZE_EXPAND_FILL; _log.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _log.fit_content = false; _log.selection_enabled = true; _utility_contents.log.add_child(_log)
	var lines: Array[String] = []
	if not _reaction_notice.is_empty(): lines.append("• %s" % _reaction_notice)
	for event in _view.events.slice(maxi(0, _view.events.size() - 8)):
		var event_text := _combat_event_text(event)
		if not event_text.is_empty() and event_text != _reaction_notice: lines.append("• %s" % event_text)
	# A response with no events must not erase the roll still awaiting resolution.
	if _view.stage == "status_roll_reaction":
		for roll in _view.effect_rolls:
			lines.append("• %s: %s" % [_actor_display_name(str(roll.get("actor_id", ""))), _effect_roll_outcome(roll).replace("\n", " · ")])
	_log.text = "\n".join(lines) if not lines.is_empty() else "Battle ready."
	if _director.peek().get("type") == "effects_resolved": _log.text = "Effects are resolving automatically."

func _build_center() -> void:
	if _quiet_offensive_reaction():
		var notice := Label.new(); notice.text = "Resolving attacks…"; notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _center.add_child(notice)
		return
	if _model_thinking and not _inline_status_application() and not _continuous_damage_response() and _view.stage != "defense_reaction" and not _provoked_toxin_reaction():
		_build_model_thinking()
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
	var pass_row := _action_footer
	var planning_pass_label := "Pass Defense" if _view.segment == "defensive" else "Skip Offensive Ability"
	_add_action(pass_row, planning_pass_label, "planning_pass", _pass_planning)
	var pass_label := "Continue Without Playing a Card" if _view.stage == "status_roll_reaction" else "Pass / Acknowledge"
	if (_inline_status_application() or _provoked_toxin_reaction()) and _view.legal_actions.size() == 1 and _view.legal_actions[0].get("type") == "pass": return
	_add_action(pass_row, pass_label, "pass", func(): _send(BattleCommandBuilder.pass_command(_view.battle_id, "blade", _pending())))

func _build_model_thinking() -> void:
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var title := Label.new(); title.text = "LEARNED BLADE WARDEN IS THINKING…"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 26); title.add_theme_color_override("font_color", Color("9de0ff")); _center.add_child(title)
	var detail := Label.new(); detail.text = "Frozen seed-11 final policy · authority actions are locked while the opponent decides"; detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _center.add_child(detail)
	_inspect(title, "battle.learned_policy.thinking", detail.text)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)

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
	_build_ability_row("ENEMY ABILITIES", _view.actor("goblin").get("offensive_abilities", []), "goblin")
	var public_plan := _enemy_plan_text()
	if not public_plan.is_empty():
		var plan := Label.new(); plan.text = public_plan; plan.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; plan.add_theme_color_override("font_color", Color("ef9a55")); _center.add_child(plan)
	_build_ability_row(_actor_display_name("blade").to_upper() + " ABILITIES", _view.actor("blade").get("offensive_abilities", []), "blade")
	if _selected_card_selector() == "selected_die":
		if _view.stage == "offensive_reaction": _build_enemy_die_targets()
		elif _view.stage == "blind_reaction":
			var tip := Button.new(); tip.text = "Tip Blind die to face 5"; tip.disabled = _history_review; tip.pressed.connect(_play_blind_tip); _center.add_child(tip); _inspect(tip, "battle.tip_target.blind", "Use Tip It on the current blind-roll die")

func _build_defensive() -> void:
	_build_reaction_notice()
	if _view.stage == "defense_roll":
		_build_automatic_defense_roll()
		return
	if _board_stage() == "defense_reaction":
		_build_compact_defense_results()
		return
	var abilities: Array = _as_array(_view.actor("blade").get("defensive_abilities", []))
	_build_ability_row("DEFENSIVE ABILITIES", abilities, "blade")
	if _view.stage in ["defense_roll", "defense_reaction"]: _build_defense_mat()

func _build_compact_defense_results() -> void:
	var sides := {"blade": _ability_dock, "goblin": _enemy_attack_dock}
	var counts := {}
	for actor_id in ["blade", "goblin"]:
		counts[actor_id] = {}
		for status in _view.actor(actor_id).get("statuses", []):
			counts[actor_id][str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
	var sources := _view.damage_sources.duplicate()
	sources.sort_custom(func(a: Dictionary, b: Dictionary): return str(a.get("target_actor_id", "")) < str(b.get("target_actor_id", "")))
	for source in sources:
		var actor_id := str(source.get("target_actor_id", ""))
		if actor_id not in ["blade", "goblin"]: continue
		var data := _compact_defense_data(actor_id, counts, source)
		if data.is_empty(): continue
		data["awaiting_roll"] = _view.stage == "defense_roll" and not _history_review and not _history_replay
		var key := "%s:%s" % [_defense_review_key(), str(source.get("id", ""))]
		if not _defense_animation_times.has(key):
			if _defense_animation_times.size() > 64: _defense_animation_times.clear()
			_defense_animation_times[key] = Time.get_ticks_msec()
		var start := int(_defense_animation_times[key])
		if _history_review or _history_replay: start = Time.get_ticks_msec() - ceili((DEFENSE_TIMING.total_seconds() + 1.0) * 1000.0)
		var panel = DEFENSE_RESULT.new(); sides[actor_id].add_child(panel)
		panel.damage_settled.connect(func(): _refresh_defense_summaries.call_deferred())
		panel.configure(data, start, true)
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
	var ability_id := str(selection.get("ability_id", ""))
	var ability := BattlePresentationCatalog.ability(ability_id)
	var enemy := "goblin" if actor_id == "blade" else "blade"
	var attacker := str(source.get("source_actor_id", enemy))
	var before := maxi(0, int(source.get("base_amount", 0)) - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))
	var pending := before
	var prevented := 0
	var faces: Array = selection.get("rolled_faces", roll.get("rolled_faces", [int(selection.get("rolled_face", roll.get("face", 0)))]))
	var operations := _as_array(_view.content_definition("abilities", ability_id).get("resolution", {}).get("operations", []))
	var dice: Array = []; var gains: Array = []
	var die_id := "standard_d6"
	for op in operations:
		if op.get("type") == "roll_dice": die_id = str(op.get("dice_id", die_id))
	for face in faces:
		var benefits: Array[String] = []
		for op in _resolved_operations(operations, int(face)):
			match str(op.get("type", "")):
				"prevent_damage":
					var amount := _configured_amount(op.get("amount", 0), int(face)); prevented += amount; pending = maxi(0, pending - amount); benefits.append("Prevent %d" % amount)
				"scale_damage":
					var after := floori(float(pending * int(op.get("numerator", 1))) / maxi(1, int(op.get("denominator", 1))))
					prevented += pending - after; pending = after
				"apply_status", "apply_incubation", "incubation_or_poison":
					var target := actor_id if op.get("target") == "self" else attacker
					var status_id := str(op.get("status_id", "incubation"))
					if op.get("type") == "incubation_or_poison" and (int(counts[target].get("poison", 0)) == 0 or int(counts[target].get("incubation", 0)) > 0): status_id = "poison"
					var amount := maxi(1, int(op.get("stack_count", 1)))
					var added := _defense_status_gain(gains, counts, target, status_id, amount)
					benefits.append(("+%d " % added if added > 0 else "At cap · ") + str(BattlePresentationCatalog.status(status_id).name))
		if int(face) > 0: dice.append({"face": int(face), "benefit": "\n".join(benefits)})
	if int(source.get("scale_denominator", 0)) > 0:
		var denominator := int(source.scale_denominator)
		var numerator := int(source.get("scale_numerator", 1))
		before = floori(float(before * numerator) / denominator)
		pending = floori(float(pending * numerator) / denominator)
	var status_lines: Array[String] = []
	# An attack's status applications are independent of its blocked damage.
	var reveal := _view.offensive_reveal(attacker)
	for application in reveal.get("outcome", {}).get("status_applications", []):
		var target := str(application.get("target_actor_id", actor_id))
		if target != actor_id: continue
		var status := BattlePresentationCatalog.status(str(application.get("status_id", "")))
		status_lines.append("%s %s ×%d pending" % [status.glyph, status.name, int(application.get("stacks", 1))])
	var note := ""
	if bool(selection.get("catalyst_paid", false)): note = "Catalyst spent · 2 already prevented"
	if int(source.get("reaction_prevention", 0)) > 0: note += (" · " if not note.is_empty() else "") + "%d prevented by cards" % int(source.reaction_prevention)
	return {"source_id": str(source.get("id", "")), "read_only": _history_review or _submitting, "actor_id": actor_id, "actor_name": _actor_display_name(actor_id), "attack_name": BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))).name, "before": before, "after": pending, "prevented": prevented, "ability_name": ability.name if not ability_id.is_empty() else "No defense used", "rules": ability.text, "dice": dice, "die_id": die_id, "gains": gains, "attack_statuses": "\n".join(status_lines), "note": note}

func _defense_status_gain(gains: Array, counts: Dictionary, target: String, status_id: String, amount: int) -> int:
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

func _build_defense_mat() -> void:
	var revealed := _view.stage == "defense_reaction"
	var row := HBoxContainer.new(); row.size_flags_horizontal = Control.SIZE_EXPAND_FILL; row.add_theme_constant_override("separation", 30); _center.add_child(row)
	_inspect(row, "battle.defense_mat", "Fixed defense roll and reveal area")
	for actor_id in ["blade", "goblin"]:
		var panel := VBoxContainer.new(); panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; panel.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_child(panel)
		if actor_id == "goblin" and not revealed:
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
	if _view.segment == "ongoing_effects":
		var context := Label.new(); context.text = "STATUS DAMAGE · ONGOING EFFECTS"; context.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; context.add_theme_font_size_override("font_size", 24); context.add_theme_color_override("font_color", Color("f0bc58")); _center.add_child(context)
	if _view.segment == "ongoing_effects": _build_sources("DAMAGE SOURCES")
	_build_damage_feedback()
	if _view.segment == "ongoing_effects": _build_pending_statuses()
	var removals: Array = _as_array(_view.settled_damage.get("removals", []))
	var losses := HBoxContainer.new(); losses.name = "DamageCardPiles"; losses.size_flags_vertical = Control.SIZE_EXPAND_FILL; losses.add_theme_constant_override("separation", 18); _center.add_child(losses)
	for target in ["goblin", "blade"]:
		var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; losses.add_child(column)
		var label := Label.new(); label.text = "%s CARDS PENDING REMOVAL" % _actor_display_name(target).to_upper(); label.add_theme_font_size_override("font_size", 16); label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; column.add_child(label)
		var row := preload("res://presentation/cards/damage_card_grid.gd").new(); row.name = "DamageCards_" + target; column.add_child(row)
		for removal in removals:
			if str(removal.get("target_actor_id", "")) != target or not bool(removal.get("accepted", false)) or bool(removal.get("released", false)): continue
			var card := BattleCard.new(); row.add_child(card); card.configure(str(removal.get("card_id", "")), str(removal.get("card_definition_id", "unknown")), false, true); card.tooltip_text += " · Original zone: %s · Damage source: %s" % [str(removal.get("original_zone", "unknown")), ", ".join(removal.get("damage_proposal_ids", []))]; _inspect(card, "battle.damage_card.%s" % str(removal.get("card_id", "")), card.tooltip_text)
		if row.get_child_count() == 0:
			row.queue_free()
			var empty := Label.new(); empty.text = "No cards lost"; empty.add_theme_font_size_override("font_size", 16); column.add_child(empty)
	var overage: Dictionary = _as_dictionary(_view.settled_damage.get("overage", {}))
	if not overage.is_empty():
		var label := Label.new(); label.text = "Overage: Player %d · Enemy %d" % [int(overage.get("blade", 0)), int(overage.get("goblin", 0))]; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _center.add_child(label)

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
	for target_id in ["goblin", "blade"]:
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
		_provoked_panel.configure(_view.effect_rolls, {"blade": _actor_display_name("blade"), "goblin": _actor_display_name("goblin")}, outcomes, _view.events)
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
	for id in ["blade", "goblin"]:
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
	for actor_id in ["blade", "goblin"]:
		var actor_panel := VBoxContainer.new(); actor_panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; actor_panel.alignment = BoxContainer.ALIGNMENT_BEGIN; row.add_child(actor_panel)
		if actor_id == "goblin" and not revealed:
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
		card.configure(str(entry.instance_id), str(entry.definition_id), legal and not _submitting)
		card.toggle_mode = hand_limit
		card.button_pressed = str(entry.instance_id) in _hand_limit_selection
		card.pressed.connect(_on_card_pressed.bind(card))
		if str(entry.instance_id) in income_drawn_ids:
			card.prepare_income_draw()
			_income_drawn_cards.append(card)
		_inspect(card, "battle.card.%s" % str(entry.instance_id), card.tooltip_text)
	if hand_limit:
		var need := maxi(0, _view.actor("blade").get("hand_count", 0) - _view.actor("blade").get("max_hand_size", 6))
		var commit := Button.new(); commit.text = "Discard selected cards (%d/%d)" % [_hand_limit_selection.size(), need]; commit.disabled = _hand_limit_selection.size() != need or _submitting or _history_review; commit.pressed.connect(func(): _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), _hand_limit_selection))); _center.add_child(commit); _inspect(commit, "battle.hand_limit.commit", "Commit the selected hand-limit discards")

func _build_card_target_choices() -> void:
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
		var tile := BattleAbilityTile.new(); row.add_child(tile)
		var defense_ready := _view.segment == "defensive" and not _selected_source.is_empty()
		var can_select := actor_id == "blade" and _view.allowed("planning_select_ability") and (str(ability_id) in qualified or defense_ready) and not _submitting and not _history_review
		if _selected_card_selector() == "one_owned_offensive_ability": can_select = actor_id == "blade" and _view.stage == "planning" and not _history_review
		tile.configure(str(ability_id), str(ability_id) in qualified, selected == str(ability_id), can_select, actor)
		tile.cinematic_compact()
		var attack := _selected_attack(actor_id)
		if str(ability_id) == str(attack.get("ability_id", "")) and _view.stage != "planning" and actor_id == "blade":
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
		if action.get("type") == "planning_select_ability" and payload.get("ability_id") == "needlefang" and payload.get("tier_id") == tier_id:
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
	notice.add_theme_font_size_override("font_size", 20); notice.add_theme_color_override("font_color", Color("c9f4aa")); notice.mouse_filter = Control.MOUSE_FILTER_IGNORE; _center.add_child(notice)
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

func _selected_attack(actor_id: String) -> Dictionary:
	if _view.segment not in ["offensive", "defensive", "damage_resolution"] or _view.stage == "planning": return {}
	var reveal := _view.offensive_reveal(actor_id)
	var id := str(reveal.get("ability_id", _view.actor(actor_id).get("selected_ability", "")))
	if reveal.is_empty() and _view.segment != "offensive":
		for source in _view.damage_sources:
			var owner := str(source.get("source_actor_id", "blade" if source.get("target_actor_id") == "goblin" else "goblin"))
			if owner == actor_id:
				id = str(source.get("source_content_id", id)); break
	if id.is_empty(): return {}
	var outcome := _as_dictionary(reveal.get("outcome", {})).duplicate(true)
	var sources := _view.damage_sources
	var settled := _as_array(_view.settled_damage.get("sources", []))
	if _view.segment == "damage_resolution" and not settled.is_empty(): sources = settled
	var matching: Array = []
	var damage := 0
	var prevented := 0
	for source in sources:
		if str(source.get("source_actor_id", "blade" if source.get("target_actor_id") == "goblin" else "goblin")) != actor_id or str(source.get("source_content_id", "")) != id: continue
		matching.append(source)
		var base := int(source.get("base_amount", 0))
		var final := maxi(0, base - int(source.get("prevention", 0)) - int(source.get("reaction_prevention", 0)))
		if _view.segment == "damage_resolution" and not settled.is_empty(): final = int(source.get("final_amount", final))
		if _board_stage() == "defense_reaction" and _defense_summary_ready(str(source.get("id", ""))):
			var counts := {"blade": {}, "goblin": {}}
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
	for actor_id in ["blade", "goblin"]:
		var attack := _selected_attack(actor_id)
		if attack.is_empty(): continue
		if actor_id == "blade" and _view.segment == "offensive": continue
		var dock := _ability_dock if actor_id == "blade" else _enemy_attack_dock
		var tile := BattleAbilityTile.new(); dock.add_child(tile)
		tile.configure(str(attack.ability_id), false, true, false, _view.actor(actor_id))
		tile.cinematic_compact(); tile.show_selected_attack(str(attack.text))
		_selected_attack_tiles[actor_id] = tile
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
	if damage == 0 and statuses.is_empty() and resources.is_empty(): lines.append("No offensive effect pending")
	var target_names: Array[String] = []
	for target_id in targets: target_names.append(_actor_display_name(str(target_id)))
	if not target_names.is_empty(): lines.append("Target: %s" % ", ".join(target_names))
	return "\n".join(lines)

func _build_enemy_die_targets() -> void:
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(row)
	for i in _view.rolled_dice("goblin").size():
		var die: Dictionary = _view.rolled_dice("goblin")[i]
		if int(die.get("face", 0)) != 6: continue
		var button := Button.new(); button.text = "Tip enemy die %d: 6 → 5" % (i + 1); button.disabled = _history_review; button.pressed.connect(func(): _play_tip_it(i)); row.add_child(button); _inspect(button, "battle.tip_target.goblin.%d" % i, "Use Tip It on this revealed face-6 die")

func _build_presentation_beat() -> void:
	var beat := _director.peek()
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

func _build_card_cleanse(beat: Dictionary) -> void:
	var event: Dictionary = beat.get("event", {})
	var data: Dictionary = event.get("data", {})
	var actor_id := str(event.get("actor_id", ""))
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var panel = preload("res://presentation/battle/card_cleanse.gd").new()
	_center.add_child(panel)
	panel.configure(_actor_display_name(actor_id), BattlePresentationCatalog.card(str(data.get("card_definition_id", ""))), BattlePresentationCatalog.status(str(data.get("choice_id", ""))), int(data.get("stacks_before", 0)), int(data.get("stacks_after", 0)))
	_inspect(panel, "battle.card_cleanse", panel.tooltip_text)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)
	if _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	panel.finished.connect(_finish_status_animation.bind(_income_animation_generation))
	_start_card_cleanse.call_deferred(panel, actor_id, data, _income_animation_generation)

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

func _start_card_cleanse(panel: Control, actor_id: String, data: Dictionary, generation: int) -> void:
	if generation != _income_animation_generation or not is_instance_valid(panel) or not is_inside_tree(): return
	if _director.peek().get("type") != "card_cleanse" or _history_review or _snapshot_panel_open or not _history_pending_divergence.is_empty(): return
	panel.animate()
	var profile: ActorProfile = _actor_profiles.get(actor_id)
	if is_instance_valid(profile): profile.animate_card_cleanse(str(data.get("choice_id", "")), int(data.get("stacks_before", 0)), panel.DURATION)

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
		final.text = "You %d/%d · Learned Policy %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0)), int(_view.actor("goblin").get("current_health", 0)), int(_view.actor("goblin").get("max_health", 0))]
	else:
		final.text = "Blade %d/%d · Goblin %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0)), int(_view.actor("goblin").get("current_health", 0)), int(_view.actor("goblin").get("max_health", 0))]
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

func _on_ability_pressed(ability_id: String) -> void:
	if _history_review or _submitting or _model_thinking or _director.has_beats(): return
	if ability_id in ["needlefang", "fever_spike", "terminal_bite", "shedskin", "barbed_mantle", "venom_gland"]:
		var choices: Array = []
		for action in _view.legal_actions:
			if str(action.get("type", "")) == "planning_select_ability" and str(action.get("payload", {}).get("ability_id", "")) == ability_id:
				choices.append(action)
		if _view.segment == "defensive" and choices.size() == 1:
			_send(JSON.stringify(choices[0]))
		else:
			_show_venom_choices(choices, BattlePresentationCatalog.ability(ability_id).name)
		return

	if _history_review or _submitting: return
	if _selected_card_selector() == "one_owned_offensive_ability":
		_send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], ability_id)); return
	var targets := [_selected_source] if _view.segment == "defensive" and not _selected_source.is_empty() else ["goblin"] if _view.segment == "offensive" else []
	if targets.is_empty(): _show_error("Select an incoming source first."); return
	_send(BattleCommandBuilder.planning_select_ability(_view.battle_id, "blade", _pending(), ability_id, targets))

func _on_card_pressed(card: BattleCard) -> void:
	if _history_review or _submitting or _model_thinking or _director.has_beats(): return
	if str(BattlePresentationCatalog.card(card.definition_id).targeting.get("selector", "")) == "venom_choice" and _view.stage != "discard_to_hand_limit":
		var choices: Array = []
		for action in _view.legal_actions:
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
			if planning: _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], ["goblin"]))
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [], ["goblin"]))
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
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": "goblin", "die_index": index, "face": _selected_card_set_face()}]))

func _play_blind_tip() -> void:
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": "blade", "die_index": 0, "face": _selected_card_set_face()}]))

func _play_selected_status_card(status_id: String) -> void:
	if _selected_card.is_empty() or status_id.is_empty(): return
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], status_id))

func _reroll_unkept(history_confirmed: bool = false) -> void:
	if _submitting or _history_review: return
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
	_submitting = true; _render()
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
	_render()
	if learned_battle_mode: call_deferred("_schedule_model_if_needed", reroll_result)

func _send(command_json: String, history_confirmed: bool = false) -> void:
	if _submitting or _history_review or command_json.is_empty(): return
	_reset_auto_pass_preview()
	var sent = JSON.parse_string(command_json)
	var sent_type := str(sent.get("type", "")) if sent is Dictionary else ""
	var label := _history_label_for_command(sent)
	var action := _history_command_action(sent)
	if _history_replay and not history_confirmed:
		_try_replay_history_action(action, label, {"kind": "command", "command_json": command_json})
		return
	_record_history_point(label, "decision", action)
	_submitting = true; _render()
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
	_director.queue_result(result, _director.last_sequence())
	_history_action_completed()
	_save_active()
	_record_history_arrival(label)
	_render()
	if learned_battle_mode: call_deferred("_schedule_model_if_needed", result)

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
	_selected_card = _as_dictionary(client_state.get("selected_card", {})).duplicate(true)
	_hand_limit_selection = _as_array(client_state.get("hand_limit_selection", [])).duplicate()
	_selection_roll_number = _as_array(_view.actor("blade").get("roll_history", [])).size()

func _history_client_state() -> Dictionary:
	return {
		"selected_indices": _selected_indices.duplicate(),
		"selected_source": _selected_source,
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

func _actor_display_name(actor_id: String) -> String:
	var actor := _view.actor(actor_id)
	var character := _as_dictionary(actor.get("character", {}))
	var display_name := str(character.get("name", "")).strip_edges()
	if display_name.is_empty():
		var definition_id := str(actor.get("definition_id", ""))
		if definition_id.is_empty():
			definition_id = "blade_warden" if actor_id == "blade" or (actor_id == "goblin" and learned_battle_mode) else "venom_goblin" if actor_id == "goblin" else actor_id
		display_name = definition_id.replace("_", " ").capitalize()
	if actor_id == "goblin" and learned_battle_mode: return "Learned " + display_name
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
		var enemy: Dictionary = _as_dictionary(commitments.get("goblin", {}))
		if not enemy.is_empty() and int(enemy.get("ai_d100", 0)) > 0:
			return "Enemy D100 %d · simulated rolls %d" % [int(enemy.ai_d100), int(enemy.get("simulated_rolls", 0))]
	return ""

func _prompt_text() -> String:
	if _view.stage in ["defense_roll", "defense_reaction"]: return "Defense"
	if not _model_thinking and bool(_view.raw_snapshot.get("offensive_reselection", false)): return "Your dice changed · choose your attack"
	if _model_thinking: return "Learned Blade Warden is thinking…"
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
		if str(source.get("target_actor_id", "")) == actor_id: return source
	for source in _as_array(_view.settled_damage.get("sources", [])):
		if str(source.get("target_actor_id", "")) == actor_id: return source
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
		"selected_card": _selected_card,
		"defense_rolls": _view.defense_rolls,
		"defense_selections": _view.defense_selections,
		"offensive_reveals": _view.offensive_reveals,
		"rolled_dice": {"blade": _view.rolled_dice("blade"), "goblin": _view.rolled_dice("goblin")},
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
			for status in _view.actor("goblin").get("statuses", []):
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
		var owner := "Your" if parts[0] in ["blade", learned_human_seat] else "Enemy"
		text = "%s die %d → face %s" % [owner, int(parts[1]) + 1, parts[2]]
	elif key.contains("poison") and key != "prevent":
		var labels: Array[String] = []
		for status_id in key.split(","): labels.append(BattlePresentationCatalog.status(status_id).name)
		text = " + ".join(labels)
		if targets.size() == 1 and str(targets[0]).begins_with("source-"): text = "Prevent 2 + pay 1 Catalyst to remove 1 " + text
	elif key == "incubation": text = "Prevent 2 + pay 1 Catalyst to remove 1 Incubation"
	elif key == "prevent": text = "Prevent damage"
	for source in _view.damage_sources + _as_array(_view.settled_damage.get("sources", [])):
		if str(source.get("id", "")) in targets:
			text += " · " + BattlePresentationCatalog.ability(str(source.get("source_content_id", ""))).name
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
	if key != _effects_beat_key: _effects_elapsed = 0.0; _effects_beat_key = key
	var panel := preload("res://presentation/battle/automatic_effects.gd").new()
	_center.add_child(panel)
	panel.configure(_as_dictionary(beat.get("event", {}).get("data", {})), {"blade": _actor_display_name("blade"), "goblin": _actor_display_name("goblin")})
	_effects_panel = panel
	panel.resume_at(_effects_elapsed)
	panel.finished.connect(_finish_automatic_effects.bind(_income_animation_generation))
	panel.phase_changed.connect(_show_effects_profiles.bind(panel.summary))
	_start_automatic_effects.call_deferred(panel, _income_animation_generation)

func _start_automatic_effects(panel: Control, generation: int) -> void:
	if generation != _income_animation_generation or not is_instance_valid(panel): return
	_show_effects_profiles("cards", 0, panel.summary)
	# The gameplay debug checkbox does not reintroduce Effects decisions.
	# Snapshot/history tools still freeze a presentation while inspecting it.
	panel.set_paused(_history_review or _snapshot_panel_open)

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
		for removal in _view.settled_damage.get("removals", []):
			if removal.get("accepted", false) and not removal.get("released", false): still_pending[str(removal.get("card_id", ""))] = true
		for removal in previous_damage.get("removals", []):
			if removal.get("accepted", false) and not removal.get("released", false) and not still_pending.has(str(removal.get("card_id", ""))) and str(removal.get("card_id", "")) != str(data.get("card_instance_id", "")):
				# Only previously public damage cards are used; never inspect the opponent's hand.
				_damage_feedback.saved.append(removal.duplicate(true))
		_reaction_card_feedback = {"battle_id": _view.battle_id, "source_id": str(data.get("source_id", "")), "expires_ms": Time.get_ticks_msec() + 1800, "text": text}

func _build_damage_feedback() -> void:
	if _damage_feedback.get("battle_id") != _view.battle_id or not _reaction_feedback_active() or _history_review or _history_replay: return
	var panel := preload("res://presentation/battle/damage_response_feedback.gd").new()
	_center.add_child(panel)
	panel.configure(_damage_feedback)
	_inspect(panel, "battle.damage_response_feedback", str(_damage_feedback.text))

func _pass_hands_off_priority() -> bool:
	return bool(_view.raw_snapshot.get("pass_hands_off_priority", false))

func _quiet_offensive_reaction() -> bool:
	if _view.stage != "offensive_reaction" or _history_review or _history_replay or _snapshot_panel_open or _model_error or not _error_message.is_empty() or _director.has_beats(): return false
	if _model_thinking or bool(_view.learned_policy.get("model_turn", false)): return true
	return _view.legal_actions.size() == 1 and _view.legal_actions[0].get("type") == "pass"
