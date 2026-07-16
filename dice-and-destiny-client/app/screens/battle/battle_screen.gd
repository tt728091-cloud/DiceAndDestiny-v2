extends Control

const SEGMENTS := [["ongoing_effects", "Effects"], ["income", "Income"], ["offensive", "Offensive"], ["defensive", "Defensive"], ["damage_resolution", "Damage"]]
const INCOME_DURATION_SETTING := "dice_and_destiny/presentation/income_animation_seconds"

var initial_result: Dictionary = {}
var viewer_actor_id := "blade"
var gateway: BattleGateway
var active_store: ActiveBattleStore
var last_presented_sequence := 0
var loaded_snapshot_name := ""
var history_context: Dictionary = {}

var _view := BattleViewState.new()
var _director := BattlePresentationDirector.new()
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

func _ready() -> void:
	add_to_group("inspectable_battle_screen")
	gateway = BattleGateway.new() if gateway == null else gateway
	active_store = ActiveBattleStore.new() if active_store == null else active_store
	if not _view.apply_result(initial_result):
		_build_error_only(str(initial_result.get("error", "Battle snapshot is missing or unsafe.")))
		return
	_apply_history_context(history_context)
	_director.queue_result(initial_result, last_presented_sequence)
	if _history_tools_enabled():
		_refresh_history()
		_record_history_arrival()
	_render()

func _render() -> void:
	_income_animation_generation += 1
	_actor_profiles.clear()
	_income_drawn_cards.clear()
	if is_instance_valid(_root): _root.queue_free()
	_root = Control.new(); _root.set_anchors_preset(Control.PRESET_FULL_RECT); add_child(_root)
	var background := TextureRect.new(); background.set_anchors_preset(Control.PRESET_FULL_RECT); background.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; background.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	if ResourceLoader.exists("res://assets/battle/board_background.png"): background.texture = load("res://assets/battle/board_background.png")
	else: background.modulate = Color("171b22")
	_root.add_child(background)
	var shade := ColorRect.new(); shade.color = Color(0.025, 0.035, 0.05, 0.66); shade.set_anchors_preset(Control.PRESET_FULL_RECT); _root.add_child(shade)
	var margin := MarginContainer.new(); margin.set_anchors_preset(Control.PRESET_FULL_RECT); margin.add_theme_constant_override("margin_left", 18); margin.add_theme_constant_override("margin_right", 18); margin.add_theme_constant_override("margin_top", 12); margin.add_theme_constant_override("margin_bottom", 12); _root.add_child(margin)
	var vertical := VBoxContainer.new(); vertical.add_theme_constant_override("separation", 10); margin.add_child(vertical)
	_build_header(vertical)
	var columns := HBoxContainer.new(); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; columns.add_theme_constant_override("separation", 12); vertical.add_child(columns)
	_build_player_column(columns)
	_center = VBoxContainer.new(); _center.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center.add_theme_constant_override("separation", 8); columns.add_child(_center)
	_build_center()
	_build_enemy_column(columns)
	_build_error(vertical)
	if _history_tools_enabled(): _build_history_bar(vertical)
	if _snapshot_panel_open: _build_snapshot_panel()
	if not _history_pending_divergence.is_empty(): _build_history_divergence_panel()

func _build_header(parent: VBoxContainer) -> void:
	var bar := HBoxContainer.new(); bar.alignment = BoxContainer.ALIGNMENT_CENTER; parent.add_child(bar)
	var display_segment := _view.segment
	var display_stage := _view.stage
	if _director.has_beats():
		var beat: Dictionary = _director.peek()
		var beat_event: Dictionary = _as_dictionary(beat.get("event", {}))
		var event_segment := str(beat.get("presentation_segment", ""))
		if event_segment.is_empty(): event_segment = str(beat_event.get("segment", beat_event.get("to", "")))
		if not event_segment.is_empty(): display_segment = event_segment
		if beat.get("type") == "defense_selected": display_stage = "defense_reveal"
		elif beat.get("type") == "income_summary": display_stage = "income_results"
		elif beat.get("type") == "segment_entered": display_stage = "presentation"
	for pair in SEGMENTS:
		var label := Label.new(); label.text = "  %s  " % pair[1]; label.add_theme_font_size_override("font_size", 18)
		label.add_theme_color_override("font_color", Color("f0bc58") if display_segment == pair[0] else Color("87909c")); bar.add_child(label)
	var round := Label.new(); round.text = "     ROUND %d · %s · %s" % [_view.round_number, _segment_name(display_segment).to_upper(), display_stage.replace("_", " ").to_upper()]; round.add_theme_color_override("font_color", Color("79d8ff")); bar.add_child(round)
	if _snapshot_tools_enabled():
		var snapshots := Button.new(); snapshots.text = "DEV SNAPSHOTS"; snapshots.pressed.connect(_toggle_snapshot_panel); bar.add_child(snapshots)
		_inspect(snapshots, "battle.dev_snapshots.toggle", "Open the developer snapshot controls")

func _build_player_column(parent: HBoxContainer) -> void:
	var column := VBoxContainer.new(); column.custom_minimum_size.x = 300; parent.add_child(column)
	var profile := ActorProfile.new(); column.add_child(profile); profile.display("blade", _view.actor("blade"), true); _actor_profiles["blade"] = profile
	var income := _income_actor_data("blade")
	if not income.is_empty(): profile.prepare_income(income)
	else:
		var upcoming_income := _upcoming_income_actor_data("blade")
		if not upcoming_income.is_empty(): profile.prepare_before_income(upcoming_income)
	var dice := BattleDiceTray.new(); column.add_child(dice)
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
	dice.display(rolled, kept, planning, "PLAYER OFFENSIVE DICE · Rolls %d/%d · %d left" % [_view.rolls_used("blade"), _view.max_rolls("blade"), maxi(0, _view.max_rolls("blade") - _view.rolls_used("blade"))], "battle.die.blade")
	dice.selection_changed.connect(func(indices): _selected_indices = indices)
	var controls := HBoxContainer.new(); controls.alignment = BoxContainer.ALIGNMENT_CENTER; column.add_child(controls)
	if _view.rolls_used("blade") == 0:
		_add_action(controls, "Roll 5 Dice", "planning_roll", func(): _send(BattleCommandBuilder.planning_roll(_view.battle_id, "blade", _pending())))
	elif _view.rolls_used("blade") < _view.max_rolls("blade"):
		_add_action(controls, "Reroll Unkept", "planning_reroll", func(): _reroll_unkept())

func _build_enemy_column(parent: HBoxContainer) -> void:
	var column := VBoxContainer.new(); column.custom_minimum_size.x = 300; parent.add_child(column)
	var profile := ActorProfile.new(); column.add_child(profile); profile.display("goblin", _view.actor("goblin"), false); _actor_profiles["goblin"] = profile
	var income := _income_actor_data("goblin")
	if not income.is_empty(): profile.prepare_income(income)
	else:
		var upcoming_income := _upcoming_income_actor_data("goblin")
		if not upcoming_income.is_empty(): profile.prepare_before_income(upcoming_income)
	var enemy_dice := _view.rolled_dice("goblin"); var enemy_reveal := _view.offensive_reveal("goblin"); var enemy_caption := "ENEMY OFFENSIVE DICE"
	if not enemy_dice.is_empty(): enemy_caption += " · Simulated rolls %d" % int(enemy_reveal.get("simulated_rolls", enemy_reveal.get("rolls_used", 0)))
	var dice := BattleDiceTray.new(); column.add_child(dice); dice.display(enemy_dice, [], false, enemy_caption, "battle.die.goblin")
	var log_title := Label.new(); log_title.text = "COMBAT LOG"; column.add_child(log_title)
	_log = RichTextLabel.new(); _log.custom_minimum_size = Vector2(280, 230); _log.fit_content = false; column.add_child(_log)
	var lines: Array[String] = []
	for event in _view.events.slice(maxi(0, _view.events.size() - 8)):
		lines.append("• %s" % str(event.get("type", "event")).replace("_", " "))
	_log.text = "\n".join(lines) if not lines.is_empty() else "Battle ready."

func _build_center() -> void:
	if _director.has_beats():
		if _history_review: _build_history_review_controls()
		_build_presentation_beat()
		return
	if _view.is_complete():
		if _history_review: _build_history_review_controls()
		_build_completion()
		return
	if _history_review: _build_history_review_controls()
	var prompt := Label.new(); prompt.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; prompt.add_theme_font_size_override("font_size", 21); prompt.text = _prompt_text(); _center.add_child(prompt)
	if _view.segment == "offensive": _build_offensive()
	elif _view.segment == "defensive": _build_defensive()
	elif _view.segment == "damage_resolution" or "damage" in _view.stage: _build_damage()
	elif _view.segment == "ongoing_effects": _build_effects()
	elif _view.segment == "income": _build_income()
	_build_hand()
	var pass_row := HBoxContainer.new(); pass_row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(pass_row)
	var planning_pass_label := "Pass Defense" if _view.segment == "defensive" else "Pass Planning"
	_add_action(pass_row, planning_pass_label, "planning_pass", func(): _send(BattleCommandBuilder.planning_pass(_view.battle_id, "blade", _pending())))
	_add_action(pass_row, "Pass / Acknowledge", "pass", func(): _send(BattleCommandBuilder.pass_command(_view.battle_id, "blade", _pending())))

func _build_offensive() -> void:
	_build_ability_row("ENEMY ABILITIES", _view.actor("goblin").get("offensive_abilities", []), "goblin")
	var public_plan := _enemy_plan_text()
	if not public_plan.is_empty():
		var plan := Label.new(); plan.text = public_plan; plan.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; plan.add_theme_color_override("font_color", Color("ef9a55")); _center.add_child(plan)
	var reveal := HBoxContainer.new(); reveal.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(reveal)
	_add_selected_detail(reveal, "blade"); _add_selected_detail(reveal, "goblin")
	_build_ability_row("BLADE WARDEN ABILITIES", _view.actor("blade").get("offensive_abilities", []), "blade")
	if not _selected_card.is_empty() and _selected_card.definition_id == "tip_it":
		if _view.stage == "offensive_reaction": _build_enemy_die_targets()
		elif _view.stage == "blind_reaction":
			var tip := Button.new(); tip.text = "Tip Blind die to face 5"; tip.disabled = _history_review; tip.pressed.connect(_play_blind_tip); _center.add_child(tip); _inspect(tip, "battle.tip_target.blind", "Use Tip It on the current blind-roll die")

func _build_defensive() -> void:
	_build_sources("INCOMING SOURCES")
	var abilities: Array = _as_array(_view.actor("blade").get("defensive_abilities", []))
	_build_ability_row("DEFENSIVE ABILITIES", abilities, "blade")
	if _view.stage in ["defense_roll", "defense_reaction"]: _build_defense_mat()

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
	var actor_name := "BLADE WARDEN" if actor_id == "blade" else "VENOM GOBLIN"
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
	var ability_id := str(selection.get("ability_id", ""))
	if ability_id.is_empty(): return
	var ability := BattlePresentationCatalog.ability(ability_id)
	var detail := Label.new(); detail.custom_minimum_size = Vector2(250, 96); detail.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	detail.text = "%s\n%s\n%s" % [ability.name, ability.recipe, ability.text]
	detail.text += "\nAgainst: %s" % attack.name
	detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; parent.add_child(detail); _inspect(detail, "battle.defense_selected.%s" % actor_id, detail.text)
	var face := int(roll.get("face", selection.get("rolled_face", 0)))
	var die := Button.new(); die.custom_minimum_size = Vector2(120, 105); die.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	_style_defense_die(die)
	if face > 0:
		die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_face(face), face]
		die.tooltip_text = "%s defensive die: face %d, %s." % [actor_name.capitalize(), face, BattlePresentationCatalog.symbol_name(face)]
		die.disabled = true; die.add_theme_font_size_override("font_size", 26); parent.add_child(die); _inspect(die, "battle.defense_die.%s" % actor_id, die.tooltip_text)
	elif revealed:
		var no_die := Label.new(); no_die.custom_minimum_size = Vector2(120, 105); no_die.text = "NO DIE ROLL"; no_die.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; no_die.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; no_die.add_theme_font_size_override("font_size", 18); no_die.add_theme_color_override("font_color", Color("b8bfca")); parent.add_child(no_die)
	else:
		die.tooltip_text = "Click this blank player defense die to roll it."
		die.disabled = _submitting or _director.has_beats() or _history_review
		die.pressed.connect(func(): _send(BattleCommandBuilder.roll_dice(_view.battle_id, "blade", _pending())))
		parent.add_child(die); _inspect(die, "battle.defense_die.blade.pending", die.tooltip_text)
		var instruction := Label.new(); instruction.text = "Click the blank player die to roll"; instruction.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; instruction.add_theme_color_override("font_color", Color("b8bfca")); parent.add_child(instruction)
		return
	if not revealed: return
	var chosen := Label.new(); chosen.text = "%s · %d block" % [ability.name, face] if ability_id == "basic_defense" else ability.name; chosen.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; parent.add_child(chosen)
	var effect := Label.new(); effect.text = str(ability.get("text", "Defense selected")); effect.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; effect.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; effect.custom_minimum_size.x = 300; parent.add_child(effect)
	var pending := base
	if ability_id == "basic_defense": pending = maxi(0, base - face)
	elif ability_id == "protect": pending = floori(float(base) / 2.0)
	var outcome := Label.new(); outcome.text = "%s: %d − %d = %d pending" % [attack.name, base, face, pending] if ability_id == "basic_defense" else "%s: %d → %d pending" % [attack.name, base, pending]; outcome.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; parent.add_child(outcome)
	_inspect(parent, "battle.defense_result.%s" % actor_id, "Revealed defense result for %s" % actor_id)

func _style_defense_die(die: Button) -> void:
	var normal_style := StyleBoxFlat.new(); normal_style.bg_color = Color("111722f2"); normal_style.border_color = Color("77879aff"); normal_style.set_border_width_all(2); normal_style.set_corner_radius_all(12); normal_style.shadow_color = Color(0, 0, 0, 0.75); normal_style.shadow_size = 8
	var hover_style := normal_style.duplicate(); hover_style.bg_color = Color("1b2431ff"); hover_style.border_color = Color("f0bc58ff")
	var pressed_style := hover_style.duplicate(); pressed_style.bg_color = Color("0b1018ff")
	die.add_theme_stylebox_override("normal", normal_style); die.add_theme_stylebox_override("hover", hover_style); die.add_theme_stylebox_override("pressed", pressed_style); die.add_theme_stylebox_override("disabled", normal_style.duplicate()); die.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND

func _build_damage() -> void:
	if _view.segment == "ongoing_effects":
		var context := Label.new(); context.text = "STATUS DAMAGE · ONGOING EFFECTS"; context.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; context.add_theme_font_size_override("font_size", 24); context.add_theme_color_override("font_color", Color("f0bc58")); _center.add_child(context)
	_build_sources("DAMAGE SOURCES")
	_build_pending_statuses()
	var removals: Array = _as_array(_view.settled_damage.get("removals", []))
	for target in ["goblin", "blade"]:
		var label := Label.new(); label.text = "%s CARDS PENDING REMOVAL" % target.to_upper(); _center.add_child(label)
		var scroll := ScrollContainer.new(); scroll.custom_minimum_size.y = 120; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO; _center.add_child(scroll)
		var row := HBoxContainer.new(); scroll.add_child(row)
		for removal in removals:
			if str(removal.get("target_actor_id", "")) != target: continue
			var card := BattleCard.new(); row.add_child(card); card.configure(str(removal.get("card_id", "")), str(removal.get("card_definition_id", "unknown")), false, not bool(removal.get("released", false))); card.tooltip_text += " · Original zone: %s · Damage source: %s" % [str(removal.get("original_zone", "unknown")), ", ".join(removal.get("damage_proposal_ids", []))]; _inspect(card, "battle.damage_card.%s" % str(removal.get("card_id", "")), card.tooltip_text)
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
			var status_id := str(parts[1]); var status_data := BattlePresentationCatalog.status(status_id); var target_name := "BLADE WARDEN" if target_id == "blade" else "VENOM GOBLIN"
			var pending := Label.new(); pending.custom_minimum_size = Vector2(260, 48); pending.text = "%s  %s %s ×%d" % [target_name, status_data.glyph, status_data.name, int(grouped[key])]; pending.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; pending.add_theme_font_size_override("font_size", 18); pending.add_theme_color_override("font_color", Color("e5a07e")); pending.tooltip_text = "%s will receive %s ×%d when this damage batch is acknowledged." % [target_name.capitalize(), status_data.name, int(grouped[key])]; row.add_child(pending); _inspect(pending, "battle.pending_status.%s.%s" % [target_id, status_id], pending.tooltip_text)

func _build_effects() -> void:
	var panel := VBoxContainer.new(); panel.alignment = BoxContainer.ALIGNMENT_CENTER; panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _center.add_child(panel)
	var title := Label.new(); title.text = "☠ ONGOING EFFECTS"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 28); panel.add_child(title)
	var details: Array[String] = []
	for id in ["blade", "goblin"]:
		for status in _view.actor(id).get("statuses", []):
			var definition := str(status.get("definition_id", "")); details.append("%s: %s ×%d — %s" % [id.capitalize(), BattlePresentationCatalog.status(definition).name, int(status.get("stacks", 1)), BattlePresentationCatalog.status(definition).text])
	var body := Label.new(); body.text = "\n".join(details) if not details.is_empty() else "No active status work remains."; body.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; body.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; panel.add_child(body)
	if _view.stage in ["status_roll", "status_roll_reaction"]:
		_build_effects_mat(panel)
	if not _selected_card.is_empty() and _selected_card.definition_id == "tip_it" and _view.stage == "blind_reaction":
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
	var actor_name := "BLADE WARDEN" if actor_id == "blade" else "VENOM GOBLIN"
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
		var roll: Dictionary = _as_dictionary(value); var status_id := str(roll.get("status_id", "poison")); grouped[status_id] = int(grouped.get(status_id, 0)) + 1
	var descriptions: Array[String] = []
	for status_id in grouped:
		var status := BattlePresentationCatalog.status(str(status_id)); descriptions.append("%s ×%d\nRoll once per status stack" % [status.name, int(grouped[status_id])])
	detail.text = "\n".join(descriptions); _inspect(detail, "battle.effects_selected.%s" % actor_id, detail.text)
	var dice_row := HBoxContainer.new(); dice_row.custom_minimum_size.y = 105; dice_row.alignment = BoxContainer.ALIGNMENT_CENTER; dice_row.add_theme_constant_override("separation", 12); parent.add_child(dice_row)
	for index in rolls.size():
		var roll: Dictionary = _as_dictionary(rolls[index]); var die_data: Dictionary = _as_dictionary(roll.get("die", {})); var face := int(die_data.get("face", 0)); var secretly_rolled := bool(roll.get("resolved", false)) and face == 0
		var die := Button.new(); die.custom_minimum_size = Vector2(96, 96); die.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; _style_defense_die(die)
		if face > 0:
			die.text = "%s\n%d" % [BattlePresentationCatalog.symbol_for_face(face), face]; die.disabled = true; die.add_theme_font_size_override("font_size", 24)
			die.tooltip_text = "%s %s die: face %d, %s." % [actor_name.capitalize(), BattlePresentationCatalog.status(str(roll.get("status_id", "poison"))).name, face, BattlePresentationCatalog.symbol_name(face)]
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
	var status_id := str(roll.get("status_id", "poison")); var status := BattlePresentationCatalog.status(status_id); var face := int(_as_dictionary(roll.get("die", {})).get("face", 0))
	if status_id == "poison":
		if face <= 4: return "%s · %d\n1 damage pending" % [status.name, face]
		return "%s · %d\nRemove 1 Poison stack" % [status.name, face]
	return "%s · %d\nEffect result revealed" % [status.name, face]

func _roll_effect_die(index: int) -> void:
	_send(BattleCommandBuilder.roll_dice(_view.battle_id, "blade", _pending(), [index]))

func _build_income() -> void:
	var label := Label.new(); label.text = "▣  DRAW  →  NEW CARD  →  HAND        ✦ +1 ENERGY"; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.add_theme_font_size_override("font_size", 28); _center.add_child(label)

func _build_hand(income_drawn_ids: Array = []) -> void:
	var label := Label.new(); label.text = "YOUR HAND"; _center.add_child(label)
	var scroll := ScrollContainer.new(); scroll.custom_minimum_size.y = 132; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO; _center.add_child(scroll)
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

func _build_ability_row(caption: String, abilities: Array, actor_id: String) -> void:
	var label := Label.new(); label.text = caption; _center.add_child(label)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(row)
	var actor := _view.actor(actor_id); var qualified: Array = _as_array(actor.get("qualified_abilities", [])); var selected := str(actor.get("selected_ability", ""))
	if _view.segment == "defensive" and actor_id == "blade": selected = str(_view.defense_selections.get(actor_id, {}).get("ability_id", ""))
	for ability_id in abilities:
		var tile := BattleAbilityTile.new(); row.add_child(tile)
		var defense_ready := _view.segment == "defensive" and not _selected_source.is_empty()
		var can_select := actor_id == "blade" and _view.allowed("planning_select_ability") and (str(ability_id) in qualified or defense_ready) and not _submitting and not _history_review
		if not _selected_card.is_empty() and _selected_card.definition_id == "sharpen_blade": can_select = actor_id == "blade" and _view.stage == "planning" and not _history_review
		tile.configure(str(ability_id), str(ability_id) in qualified, selected == str(ability_id), can_select)
		tile.pressed.connect(_on_ability_pressed.bind(str(ability_id)))
		_inspect(tile, "battle.ability.%s.%s" % [actor_id, str(ability_id)], tile.tooltip_text)

func _build_sources(caption: String) -> void:
	var label := Label.new(); label.text = caption; _center.add_child(label)
	var sources: Array = _view.damage_sources
	var settled_sources: Array = _as_array(_view.settled_damage.get("sources", []))
	var using_settled_batch := _view.segment != "defensive" and not settled_sources.is_empty()
	if using_settled_batch:
		sources = settled_sources
		_build_damage_source_summary(sources)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _center.add_child(row)
	var selected_source := _selected_source
	if _view.segment == "defensive" and selected_source.is_empty(): selected_source = str(_view.defense_selections.get("blade", {}).get("source_id", ""))
	for source in sources:
		if _view.segment == "defensive" and str(source.get("target_actor_id", "")) != "blade": continue
		var button := Button.new(); var id := str(source.get("id", "")); var data := BattlePresentationCatalog.ability(str(source.get("source_content_id", "")))
		var base := int(source.get("base_amount", 0)); var final := int(source.get("final_amount", 0)); var prevented := maxi(0, base - final) if using_settled_batch else int(source.get("prevention", 0)) + int(source.get("reaction_prevention", 0))
		button.text = "%s → %s\nBase %d · Prevented %d · Final %d" % [data.name, str(source.get("target_actor_id", "")), base, prevented, final]
		button.tooltip_text = "Damage source %s" % id; button.disabled = _submitting or _history_review; button.button_pressed = selected_source == id; button.pressed.connect(func(): _selected_source = id; _render()); row.add_child(button)
		_inspect(button, "battle.source.%s" % id, button.tooltip_text)

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
		var summary: Dictionary = grouped[key]; var base := int(summary.base); var final := int(summary.final); var content := BattlePresentationCatalog.ability(str(summary.content_id)); var target_name := "Blade Warden" if summary.target_id == "blade" else "Venom Goblin" if summary.target_id == "goblin" else str(summary.target_id).capitalize()
		var result := Label.new(); result.custom_minimum_size = Vector2(280, 58); result.text = "%s → %s\n%d incoming · %d prevented · %d pending" % [content.name, target_name, base, maxi(0, base - final), final]; result.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; result.add_theme_font_size_override("font_size", 17); row.add_child(result); _inspect(result, "battle.damage_summary.%s.%s" % [str(summary.content_id), str(summary.target_id)], result.text)

func _add_selected_detail(parent: Container, actor_id: String) -> void:
	var revealed: Dictionary = _view.offensive_reveal(actor_id)
	var id := str(revealed.get("ability_id", _view.actor(actor_id).get("selected_ability", "")))
	if id.is_empty(): return
	var data := BattlePresentationCatalog.ability(id); var panel := Label.new(); panel.custom_minimum_size = Vector2(250, 110)
	var targets: Array = _as_array(revealed.get("targets", _view.actor(actor_id).get("selected_targets", [])))
	var outcome: Dictionary = _as_dictionary(revealed.get("outcome", {}))
	if _view.stage == "offensive_reaction" and not outcome.is_empty(): panel.text = _offensive_outcome_text(data.name, outcome, targets)
	else: panel.text = "%s\n%s\n%s\nTarget: %s" % [data.name, data.recipe, data.text, ", ".join(targets)]
	panel.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; parent.add_child(panel); _inspect(panel, "battle.outcome.%s" % actor_id, panel.text)

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
	for target_id in targets: target_names.append("Blade Warden" if str(target_id) == "blade" else "Venom Goblin" if str(target_id) == "goblin" else str(target_id).capitalize())
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
	var button := Button.new(); button.text = "Continue Presentation"; button.tooltip_text = "Continue the visual presentation only; this sends no gameplay command."; button.custom_minimum_size = Vector2(240, 54); button.pressed.connect(_advance_beat); _center.add_child(button); _inspect(button, "battle.presentation.continue", button.tooltip_text)
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)

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
	if not _history_review: active_store.clear()
	var spacer := Control.new(); spacer.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer)
	var title := Label.new(); title.text = ("VICTORY" if _view.battle_result == "victory" else _view.battle_result.to_upper()); title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 52); title.add_theme_color_override("font_color", Color("f5c963")); _center.add_child(title)
	var final := Label.new(); final.text = "Blade %d/%d · Goblin %d/%d" % [int(_view.actor("blade").get("current_health", 0)), int(_view.actor("blade").get("max_health", 0)), int(_view.actor("goblin").get("current_health", 0)), int(_view.actor("goblin").get("max_health", 0))]; final.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _center.add_child(final)
	var again := Button.new(); again.text = "Play Again"; again.disabled = _history_review; again.pressed.connect(_play_again); _center.add_child(again); _inspect(again, "battle.complete.play_again", "Start a new real-random battle")
	var spacer2 := Control.new(); spacer2.size_flags_vertical = Control.SIZE_EXPAND_FILL; _center.add_child(spacer2)

func _add_action(parent: Container, text: String, command: String, callback: Callable) -> void:
	if not _view.allowed(command): return
	var button := Button.new(); button.text = text; button.disabled = _submitting or _director.has_beats() or _history_review; button.pressed.connect(callback); parent.add_child(button)
	_inspect(button, "battle.command.%s" % command, "Submit the authority command %s" % command)

func _on_ability_pressed(ability_id: String) -> void:
	if _history_review: return
	if not _selected_card.is_empty() and _selected_card.definition_id == "sharpen_blade":
		_send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], ability_id)); return
	var targets := [_selected_source] if _view.segment == "defensive" and not _selected_source.is_empty() else ["goblin"] if _view.segment == "offensive" else []
	if targets.is_empty(): _show_error("Select an incoming source first."); return
	_send(BattleCommandBuilder.planning_select_ability(_view.battle_id, "blade", _pending(), ability_id, targets))

func _on_card_pressed(card: BattleCard) -> void:
	if _history_review: return
	if _view.stage == "discard_to_hand_limit":
		if card.instance_id in _hand_limit_selection: _hand_limit_selection.erase(card.instance_id)
		else: _hand_limit_selection.append(card.instance_id)
		_render(); return
	_selected_card = {"instance_id": card.instance_id, "definition_id": card.definition_id}
	match card.definition_id:
		"battle_focus": _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], ["blade"]))
		"loaded_die":
			if _selected_indices.size() == 1: _send(BattleCommandBuilder.planning_commit_cards(_view.battle_id, "blade", _pending(), [card.instance_id], [], "", "", int(_selected_indices[0])))
			else: _show_error("Select exactly one rolled player die, then choose Loaded Die again.")
		"antidote":
			var statuses: Array = _as_array(_view.actor("blade").get("statuses", []))
			if statuses.is_empty(): _show_error("No negative self status is available.")
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [], [], str(statuses[0].get("definition_id", ""))))
		"emergency_ward":
			if _selected_source.is_empty(): _show_error("Select an incoming damage source, then choose Emergency Ward again.")
			else: _send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [card.instance_id], [_selected_source]))
		"tip_it": _render()
		"sharpen_blade": _render()

func _play_tip_it(index: int) -> void:
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": "goblin", "die_index": index, "face": 5}]))

func _play_blind_tip() -> void:
	_send(BattleCommandBuilder.commit_interaction(_view.battle_id, "blade", _pending(), [_selected_card.instance_id], [], [], "", [{"type": "set_die_face", "actor_id": "blade", "die_index": 0, "face": 5}]))

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
	var keep_result := gateway.submit(keep_command)
	if keep_result.get("accepted") != true:
		_submitting = false; _show_error(str(keep_result.get("error", "Could not keep the selected dice.")), keep_result); _render(); return
	if not _view.apply_result(keep_result):
		_submitting = false; _show_error("The keep result was not a safe battle snapshot.", keep_result); _render(); return
	_save_active()
	var reroll_command := BattleCommandBuilder.planning_reroll(_view.battle_id, "blade", _pending(), indices)
	var reroll_result := gateway.submit(reroll_command)
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

func _send(command_json: String, history_confirmed: bool = false) -> void:
	if _submitting or _history_review or command_json.is_empty(): return
	var sent = JSON.parse_string(command_json)
	var sent_type := str(sent.get("type", "")) if sent is Dictionary else ""
	var label := _history_label_for_command(sent)
	var action := _history_command_action(sent)
	if _history_replay and not history_confirmed:
		_try_replay_history_action(action, label, {"kind": "command", "command_json": command_json})
		return
	_record_history_point(label, "decision", action)
	_submitting = true; _render()
	var result := gateway.submit(command_json)
	_submitting = false
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "Battle command rejected.")), result); _render(); return
	if not _view.apply_result(result):
		_show_error("Authority result was not a safe battle snapshot.", result); _render(); return
	_error_message = ""
	_selected_card.clear(); _selected_source = ""; _hand_limit_selection.clear()
	if sent_type not in ["planning_keep", "planning_commit_cards"]: _selected_indices.clear()
	_director.queue_result(result, _director.last_sequence())
	_history_action_completed()
	_save_active()
	_record_history_arrival(label)
	_render()

func _advance_beat(history_confirmed: bool = false) -> void:
	var completed_label := ""
	if _history_tools_enabled() and not _history_review and _director.has_beats():
		var beat: Dictionary = _director.peek()
		var label := "%s: %s" % ["Automatic" if beat.get("type") == "income_summary" else "Continue", _history_beat_label(beat)]
		completed_label = label
		var action := _history_presentation_action(beat)
		if _history_replay and not history_confirmed:
			_try_replay_history_action(action, label, {"kind": "presentation"})
			return
		_record_history_point(label, "presentation", action)
	_director.advance(); _history_action_completed(); _save_active(); _record_history_arrival(completed_label); _render()

func _play_again() -> void:
	if _history_review: return
	active_store.clear(); get_tree().change_scene_to_file("res://app/boot/battle_bootstrap.tscn")

func _snapshot_tools_enabled() -> bool:
	return (
		OS.is_debug_build()
		and OS.get_environment("DICE_AND_DESTINY_ENABLE_SNAPSHOTS") == "1"
		and bool(ProjectSettings.get_setting("dice_and_destiny/development/enable_snapshots", false))
	)

func _history_tools_enabled() -> bool:
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
	var result := gateway.list_dev_history(_view.battle_id, viewer_actor_id)
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
	var result := gateway.mark_dev_history(_view.battle_id, viewer_actor_id, label, kind, _director.last_sequence(), _history_client_state(), action)
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
	var result := gateway.jump_dev_history(_view.battle_id, viewer_actor_id, point_id)
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
	var result := gateway.commit_dev_history(_view.battle_id, viewer_actor_id, mode)
	_submitting = false
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not resume from history.")); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	_handoff_battle_result(result, int(data.get("presented_sequence", _director.last_sequence())), loaded_snapshot_name, _history_context_from_data(data))

func _return_history_latest() -> void:
	if not _history_review or _submitting: return
	_submitting = true
	var result := gateway.return_dev_history_latest(_view.battle_id, viewer_actor_id)
	_submitting = false
	if result.get("accepted") != true:
		_history_message = "History error: %s" % str(result.get("error", "Could not return to latest.")); _render(); return
	var data: Dictionary = _as_dictionary(result.get("data", {}).get("history", {}))
	_handoff_battle_result(result, int(data.get("presented_sequence", 0)), loaded_snapshot_name, {})

func _try_replay_history_action(action: Dictionary, label: String, pending: Dictionary) -> void:
	if _submitting: return
	_submitting = true
	var result := gateway.replay_dev_history_action(_view.battle_id, viewer_actor_id, action)
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
	var result := gateway.replace_dev_history_future(_view.battle_id, viewer_actor_id)
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
		"planning_pass": return "Pass Defense" if _view.segment == "defensive" else "Pass Planning"
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
	var result := gateway.list_dev_snapshots(_view.battle_id, viewer_actor_id)
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
	var result := gateway.save_dev_snapshot(_view.battle_id, viewer_actor_id, _snapshot_name, _snapshot_overwrite)
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
	var result := gateway.mark_dev_history(_view.battle_id, viewer_actor_id, _history_current_state_label(), kind, _director.last_sequence(), _history_client_state(), {})
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
	var result := gateway.load_dev_snapshot(_view.battle_id, viewer_actor_id, snapshot_name)
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
	active_store.save_active(_view.battle_id, viewer_actor_id, _director.last_sequence(), loaded_snapshot_name, history_context)

func _card_legal(definition: String) -> bool:
	if _submitting or _history_review: return false
	if _view.allowed("planning_commit_cards"):
		if definition == "loaded_die": return not _view.rolled_dice("blade").is_empty()
		return definition in ["battle_focus", "sharpen_blade"]
	if not _view.allowed("commit_interaction"): return false
	if _view.stage == "offensive_reaction": return definition == "tip_it"
	if _view.stage == "status_roll_reaction": return definition == "antidote"
	if _view.stage == "blind_reaction": return definition == "tip_it"
	if _view.segment == "damage_resolution" and "damage" in _view.stage: return definition == "emergency_ward"
	return false

func _pending() -> Dictionary: return _view.viewer_pending()
func _segment_name(id: String) -> String:
	for pair in SEGMENTS:
		if pair[0] == id: return pair[1]
	return id.replace("_", " ").capitalize()

func _enemy_plan_text() -> String:
	for event in _view.events:
		var event_data: Dictionary = _as_dictionary(event.get("data", {}))
		var commitments: Dictionary = _as_dictionary(event_data.get("commitments", {}))
		var enemy: Dictionary = _as_dictionary(commitments.get("goblin", {}))
		if not enemy.is_empty() and int(enemy.get("ai_d100", 0)) > 0:
			return "Enemy D100 %d · simulated rolls %d" % [int(enemy.ai_d100), int(enemy.get("simulated_rolls", 0))]
	return ""

func _prompt_text() -> String:
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
		"error": _error_message,
	}
