extends Control
## Tactics Sandbox battle screen (docs/game-design/tactics-sandbox-plan.md §9).
## It renders the authority's viewer-safe result and submits only the legal
## actions the authority lists: offense (roll, Sprint, abilities aimed at
## creatures or squares), the secret movement plan (intention, cards, map
## cards, energy; the authority computes the budget and the preview path),
## defense, the playback of each resolved round and the debug timeline.

const BOARD := preload("res://app/screens/tactics/tactics_board.gd")
const TOOLTIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
const TOOLTIP_LABEL := preload("res://presentation/battle/tooltip_label.gd")
const STATUS_STRIP := preload("res://presentation/battle/status_icon_strip.gd")
const VIEWER := "hero"
const PLAYBACK_SECONDS := 3.2
const NAME_INITIALS := {"Starter Movement": "SM", "Adventurer Movement": "AM", "Ribbon Eel": "RE", "Bell Diver": "BD", "Brine Mask": "BM", "The Drowned Regent": "DR"}

var gateway: TacticsGateway
var store: ActiveBattleStore
var initial_result: Dictionary = {}

var _result: Dictionary = {}
var _snap: Dictionary = {}
var _tv: Dictionary = {}
var _actions: Array = []
var _board
var _status: Label
var _error: Label
var _roster: VBoxContainer
var _side: VBoxContainer
var _dice: BattleDiceTray
var _hand: HBoxContainer
var _piles: Label
var _budget: Label
var _controls: HFlowContainer
var _popup: PanelContainer
var _timeline: PanelContainer
var _timeline_text: RichTextLabel
var _end: PanelContainer
var _speed_button: Button
var _aiming: Dictionary = {}
var _kept: Array = []
var _played_round := 0
var _playback: Dictionary = {}
var _speed := 1.0
var _busy := false
var _log: Array[String] = []

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_to_group("inspectable_battle_screen")
	_build()
	_apply(initial_result)

# --- Layout -----------------------------------------------------------------

func _register(id: String, control: Control, text: String = "") -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(id, control, text)

func _button(parent: Node, text: String, id: String, callback: Callable, tooltip: String = "") -> Button:
	var button := TOOLTIP_BUTTON.new(); button.text = text; button.tooltip_text = tooltip; button.custom_minimum_size.y = 34
	button.pressed.connect(callback); parent.add_child(button)
	if not id.is_empty(): _register(id, button, text)
	return button

func _build() -> void:
	var background := ColorRect.new(); background.color = Color("0b111a"); background.set_anchors_preset(Control.PRESET_FULL_RECT); add_child(background)
	var root := VBoxContainer.new(); root.set_anchors_preset(Control.PRESET_FULL_RECT); root.add_theme_constant_override("separation", 4)
	for side in ["offset_left", "offset_top"]: root.set(side, 6)
	for side in ["offset_right", "offset_bottom"]: root.set(side, -6)
	add_child(root)
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 8); root.add_child(top)
	_status = Label.new(); _status.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _status.add_theme_font_size_override("font_size", 16); _status.clip_text = true; top.add_child(_status)
	_error = Label.new(); _error.add_theme_color_override("font_color", Color("ff8a78")); _error.clip_text = true; _error.custom_minimum_size.x = 260; top.add_child(_error)
	_speed_button = _button(top, "Speed 1×", "tactics.playback.speed", _cycle_speed, "Playback speed")
	_button(top, "Replay round", "tactics.playback.replay", _replay, "Play the last resolved round again: movement, strikes, defenses and falls.")
	_button(top, "Timeline", "tactics.timeline.toggle", func(): _timeline.visible = not _timeline.visible, "The resolved movement step by step: exact times, waits, tiebreaks, contacts and area hits.")
	_button(top, "Menu", "tactics.menu", _to_menu, "Back to the main menu. The battle stays saved; resume it from the Tactics Sandbox tab.")
	var middle := HBoxContainer.new(); middle.size_flags_vertical = Control.SIZE_EXPAND_FILL; middle.add_theme_constant_override("separation", 6); root.add_child(middle)
	_board = BOARD.new(); _board.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _board.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_board.square_clicked.connect(_on_square); _board.creature_clicked.connect(_on_creature)
	middle.add_child(_board)
	var right := VBoxContainer.new(); right.custom_minimum_size.x = 360; right.add_theme_constant_override("separation", 4); middle.add_child(right)
	_roster = VBoxContainer.new(); _roster.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _roster.add_theme_constant_override("separation", 0); right.add_child(_roster)
	_dice = BattleDiceTray.new(); _dice.compact_row = true; _dice.hud_compact = true; right.add_child(_dice)
	var side_scroll := ScrollContainer.new(); side_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; side_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; right.add_child(side_scroll)
	_side = VBoxContainer.new(); _side.size_flags_horizontal = Control.SIZE_EXPAND_FILL; side_scroll.add_child(_side)
	_dice.selection_changed.connect(func(indices): _kept = indices.duplicate())
	var bottom := HBoxContainer.new(); bottom.custom_minimum_size.y = 168; bottom.add_theme_constant_override("separation", 8); root.add_child(bottom)
	var hand_column := VBoxContainer.new(); hand_column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; bottom.add_child(hand_column)
	_piles = Label.new(); _piles.add_theme_color_override("font_color", Color("9fb3c8")); hand_column.add_child(_piles)
	var hand_scroll := ScrollContainer.new(); hand_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; hand_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; hand_column.add_child(hand_scroll)
	_hand = HBoxContainer.new(); _hand.add_theme_constant_override("separation", 4); hand_scroll.add_child(_hand)
	var control_column := VBoxContainer.new(); control_column.custom_minimum_size.x = 420; bottom.add_child(control_column)
	_budget = TOOLTIP_LABEL.new(); _budget.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _budget.mouse_filter = Control.MOUSE_FILTER_PASS; control_column.add_child(_budget)
	_controls = HFlowContainer.new(); _controls.add_theme_constant_override("h_separation", 6); _controls.add_theme_constant_override("v_separation", 4); control_column.add_child(_controls)
	_popup = PanelContainer.new(); _popup.visible = false; _popup.add_theme_stylebox_override("panel", _panel_style()); add_child(_popup)
	_timeline = PanelContainer.new(); _timeline.visible = false; _timeline.add_theme_stylebox_override("panel", _panel_style()); _timeline.set_anchors_preset(Control.PRESET_LEFT_WIDE); _timeline.offset_top = 46; _timeline.offset_bottom = -180; _timeline.offset_right = 470; add_child(_timeline)
	var timeline_box := VBoxContainer.new(); _timeline.add_child(timeline_box)
	var timeline_title := Label.new(); timeline_title.text = "Movement timeline (debug)"; timeline_box.add_child(timeline_title)
	_timeline_text = RichTextLabel.new(); _timeline_text.bbcode_enabled = true; _timeline_text.size_flags_vertical = Control.SIZE_EXPAND_FILL; _timeline_text.scroll_active = true; _timeline_text.selection_enabled = true; timeline_box.add_child(_timeline_text)
	_register("tactics.timeline.text", _timeline_text, "timeline")

func _panel_style() -> StyleBoxFlat:
	var style := StyleBoxFlat.new(); style.bg_color = Color("141b26f2"); style.border_color = Color("3d4a5c"); style.set_border_width_all(1); style.set_corner_radius_all(6)
	style.content_margin_left = 10; style.content_margin_right = 10; style.content_margin_top = 8; style.content_margin_bottom = 8
	return style

# --- Data helpers -----------------------------------------------------------

func _actor(id: String) -> Dictionary: return _snap.get("actors", {}).get(id, {})

func _name(id: String) -> String:
	var actor := _actor(id)
	var name := str(actor.get("character", {}).get("name", id))
	return "You" if id == VIEWER else name

func _label(id: String) -> String:
	var name := str(_actor(id).get("character", {}).get("name", id))
	var initials := str(NAME_INITIALS.get(name, name.substr(0, 2).to_upper()))
	if id.begins_with("enemy-"): initials += id.trim_prefix("enemy-")
	return initials

func _ability_name(id: String) -> String: return str(BattlePresentationCatalog.ability(id).get("name", id))

func _square(value) -> Vector2:
	if value is Dictionary: return Vector2(float(value.get("x", 0)), float(value.get("y", 0)))
	return Vector2.ZERO

func _frac(value) -> float:
	if value is Dictionary and float(value.get("den", 1)) != 0: return float(value.get("num", 0)) / float(value.get("den", 1))
	return 0.0

func _frac_text(value) -> String:
	if not value is Dictionary: return "0"
	var den := int(value.get("den", 1))
	return str(int(value.get("num", 0))) if den == 1 else "%d/%d" % [int(value.get("num", 0)), den]

func _payload(action: Dictionary) -> Dictionary: return action.get("payload", {})

func _of_type(type: String) -> Array:
	return _actions.filter(func(a): return str(a.get("type", "")) == type)

func _choice(action: Dictionary) -> Dictionary:
	var payload := _payload(action)
	var raw := str(payload.get("status_id", "")) if action.get("type") == "planning_commit_cards" else str(payload.get("commitment", {}).get("choice_id", ""))
	var parsed = JSON.parse_string(raw) if raw.begins_with("{") else null
	return parsed if parsed is Dictionary else {}

func _card_of(action: Dictionary) -> String:
	var payload := _payload(action)
	var ids: Array = payload.get("card_ids", []) if action.get("type") == "planning_commit_cards" else payload.get("commitment", {}).get("card_ids", [])
	return str(ids[0]) if ids.size() == 1 else ""

func _step() -> String:
	if str(_tv.get("step", "")) == "movement": return "movement"
	return str(_snap.get("simultaneous_step", ""))

# --- Apply a result -----------------------------------------------------------

func _apply(result: Dictionary) -> void:
	if result.get("accepted") != true:
		_error.text = str(result.get("error", "The authority refused the command."))
		return
	_error.text = ""
	_result = result
	_snap = result.get("snapshot", {})
	_tv = _snap.get("tactics", {})
	_actions = result.get("legal_actions", []) if result.get("legal_actions") is Array else []
	if _snap.get("content_catalog") is Dictionary: BattlePresentationCatalog.configure(_snap.content_catalog)
	_aiming = {}
	_hide_popup()
	for event in result.get("events", []): _log_event(event)
	_render()
	var movement: Dictionary = _tv.get("movement", {}) if _tv.get("movement") is Dictionary else {}
	var finished := not str(result.get("battle_result", "")).is_empty()
	if not movement.is_empty() and int(movement.round) != _played_round and (int(_snap.get("round", 0)) > int(movement.round) or finished):
		_played_round = int(movement.round)
		_start_playback(movement)
	if finished: _show_end(str(result.battle_result))
	_auto_pass_if_only_pass()

## As elsewhere: when the main Pass is the only thing left, it completes on
## its own after a short look.
func _auto_pass_if_only_pass() -> void:
	if _actions.size() != 1 or str(_actions[0].get("type", "")) != "planning_pass" or not str(_payload(_actions[0]).get("source_id", "")).is_empty(): return
	if _step() == "offense_before" or _step() == "offense_rolling": return
	var expected := str(_payload(_actions[0]).get("pending_input_id", ""))
	await get_tree().create_timer(0.9).timeout
	if not is_inside_tree() or _actions.size() != 1 or str(_payload(_actions[0]).get("pending_input_id", "")) != expected: return
	_submit(_actions[0])

func _submit(action: Dictionary) -> void:
	if _busy or action.is_empty(): return
	_busy = true
	var result := gateway.submit(action)
	_busy = false
	_apply(result)

func _log_event(event: Dictionary) -> void:
	var type := str(event.get("type", ""))
	if type in ["tactics_movement_resolved", "damage_committed", "battle_completed", "tactics_attacks_revealed"]:
		_log.append("R%s %s" % [str(event.get("round", "")), type.replace("_", " ")])

# --- Render -----------------------------------------------------------------

func _render() -> void:
	_render_status()
	_render_board()
	_render_roster()
	_render_side()
	_render_dice()
	_render_hand()
	_render_controls()
	_render_timeline()

func _render_status() -> void:
	var step := _step()
	var words := {"offense_before": "Offense: lock in — Roll, or Sprint (skip your attack)", "offense_rolling": "Offense: keep and reroll, then aim an attack", "movement": "Movement: choose a secret plan, then lock in", "defense_before": "Defense: choose a defense for each attack that connected", "defense_after": "Defense: after-defense cards, then pass"}
	var text := "Round %d · %s" % [int(_snap.get("round", 0)), words.get(step, str(_snap.get("segment", "")).replace("_", " "))]
	if _actions.is_empty() and str(_result.get("battle_result", "")).is_empty(): text += " · waiting"
	_status.text = "%s · %s" % [str(_tv.get("arena", {}).get("name", "")), text]

func _movement_this_round() -> Dictionary:
	var movement = _tv.get("movement")
	if movement is Dictionary and int(movement.get("round", 0)) == int(_snap.get("round", 0)) and str(_snap.get("segment", "")) == "defensive": return movement
	return {}

func _render_board() -> void:
	var rules: Dictionary = _tv.get("rules", {})
	var colors := {}; var names := {}; var costs := {}
	for id in rules.get("terrain", {}):
		var terrain: Dictionary = rules.terrain[id]
		colors[id] = Color(str(terrain.get("color", "#333333"))); names[id] = str(terrain.get("name", id))
		costs[id] = "costs %d Move" % int(terrain.cost) if terrain.get("passable", false) else "impassable"
		if terrain.get("blocks_line_of_sight", false): costs[id] += ", blocks line of sight"
	if _board.arena.get("terrain") != _tv.get("arena", {}).get("terrain"):
		_board.set_arena({"terrain": _tv.get("arena", {}).get("terrain", []), "colors": colors, "names": names, "costs": costs})
	var movement := _movement_this_round()
	var creatures := {}
	for id in _tv.get("creatures", {}):
		var c: Dictionary = _tv.creatures[id]
		var actor := _actor(id)
		var square := _square(c.position)
		if not movement.is_empty() and movement.start.has(id): square = _square(movement.start[id])
		var statuses := {}
		for status in actor.get("statuses", []): statuses[str(status.definition_id)] = int(status.stacks)
		creatures[id] = {"square": square, "team": str(actor.get("team_id", "")), "label": _label(id), "name": _name(id), "viewer": id == VIEWER,
			"defeated": str(actor.get("defeat_state", "")) == "defeated", "running": statuses.has("running"), "hp": int(actor.get("current_health", 0)), "max_hp": int(actor.get("max_health", 1)), "tooltip": _creature_tooltip(id)}
	_board.creatures = creatures
	_board.areas = []; _board.paths = []; _board.ghosts = {}; _board.badges = []; _board.highlights = {}; _board.clickable_creatures = {}
	var attacks: Array = _tv.get("attacks", [])
	if not movement.is_empty(): attacks = movement.get("attacks", attacks)
	for attack in attacks:
		var connected := str(attack.get("outcome", "")) == "hit"
		var missed := bool(attack.get("resolved", false)) and not connected
		if str(attack.delivery) == "area":
			_board.areas.append({"squares": attack.area.map(func(s): return Vector2i(int(s.x), int(s.y))), "color": Color(0.9, 0.35, 0.15, 0.18 if missed else 0.32)})
		_board.badges.append(_attack_badge(attack))
	if not movement.is_empty():
		for id in movement.get("paths", {}):
			var points: Array = movement.paths[id].map(func(w): return _square(w.square))
			if points.size() > 1:
				var color: Color = BOARD.TEAM_COLORS.get(str(_actor(id).get("team_id", "")), Color.GRAY)
				_board.paths.append({"points": points, "color": color, "width": 3.0})
				_board.ghosts[id] = points[-1]
	var plan = _tv.get("plan")
	if _step() == "movement" and plan is Dictionary and plan.get("preview") is Dictionary:
		var points: Array = plan.preview.path.map(func(w): return _square(w.square))
		if points.size() > 1:
			_board.paths.append({"points": points, "color": Color("ffd75e"), "width": 3.0, "dashed": true})
			_board.ghosts[VIEWER] = points[-1]
	_apply_aiming_highlights()
	_board.refresh()

func _attack_badge(attack: Dictionary) -> Dictionary:
	var attacker := str(attack.attacker)
	var ability := str(attack.ability_id)
	var damage := 0
	for source in _snap.get("damage_sources", []):
		if str(source.source_actor_id) == attacker and str(source.source_content_id) == ability: damage += int(source.get("final_amount", source.get("base_amount", 0)))
	var outcome := str(attack.get("outcome", ""))
	var text := "%s %s" % [_label(attacker), _ability_name(ability)]
	var color := Color("7a1f17")
	if outcome == "hit" and (damage > 0 or attack.get("resolved", false)): text += " %d" % damage
	elif outcome == "dodged": text += " · dodged"; color = Color("4b5563")
	elif outcome == "out_of_reach": text += " · out of reach"; color = Color("4b5563")
	var reveal := {"ability_id": ability, "tier_id": str(attack.get("tier_id", ""))}
	var dice: Array = _actor(attacker).get("dice", {}).get("dice", []) if _actor(attacker).get("dice") is Dictionary else []
	var tooltip := BattlePresentationCatalog.attack_ability_tooltip(ability, _actor(attacker), reveal, dice)
	var delivery: Dictionary = _tv.get("creatures", {}).get(attacker, {}).get("deliveries", {}).get(ability, {})
	tooltip = "%s\nAttacker: %s · %s\n%s" % [tooltip, _name(attacker), str(delivery.get("text", "")), _outcome_text(attack)]
	var badge := {"id": "%s.%s" % [attacker, ability], "text": text, "tooltip": tooltip, "color": color}
	if str(attack.delivery) == "area": badge["square"] = Vector2i(int(attack.square.x), int(attack.square.y))
	else:
		badge["target"] = str(attack.get("target", ""))
		badge["below"] = badge.target == VIEWER
	return badge

func _outcome_text(attack: Dictionary) -> String:
	match str(attack.get("outcome", "")):
		"hit":
			var hits: Array = attack.get("hits", [])
			return "Connected: hits %s." % ", ".join(hits.map(func(id): return _name(str(id))))
		"dodged": return "Dodged: nobody was in the area when movement ended."
		"out_of_reach": return "Out of reach: the charger never got adjacent."
	if str(attack.delivery) == "area": return "Hits every enemy standing in the area when movement ends."
	if str(attack.delivery) == "charge": return "Connects the moment the charger is adjacent to its target."
	return "Direct: lands wherever the target runs."

func _creature_tooltip(id: String) -> String:
	var c: Dictionary = _tv.get("creatures", {}).get(id, {})
	var actor := _actor(id)
	var lines: Array[String] = ["%s · %d/%d health · %d energy" % [_name(id), int(actor.get("current_health", 0)), int(actor.get("max_health", 0)), int(actor.get("energy_points", 0))]]
	var speed := "Base speed %d" % int(c.get("base_speed", 1))
	if not str(c.get("label", "")).is_empty(): speed += " · " + str(c.label)
	var movement = _tv.get("movement")
	if movement is Dictionary and movement.get("budgets", {}).has(id): speed += " · last Move %d%s" % [int(movement.budgets[id].total), " (ran)" if movement.budgets[id].get("running", false) else ""]
	if id == VIEWER and _tv.get("plan") is Dictionary: speed += " · planned Move %d" % int(_tv.plan.get("budget", {}).get("total", 0))
	lines.append(speed)
	for ability in c.get("deliveries", {}):
		lines.append("%s: %s" % [_ability_name(str(ability)), str(c.deliveries[ability].get("text", ""))])
	for status in actor.get("statuses", []):
		var data := BattlePresentationCatalog.status(str(status.definition_id))
		lines.append("%s ×%d: %s" % [str(data.get("name", status.definition_id)), int(status.stacks), str(data.get("text", ""))])
	return "\n".join(lines)

func _render_roster() -> void:
	for child in _roster.get_children(): child.queue_free()
	var ids: Array = _tv.get("creatures", {}).keys(); ids.sort()
	ids.erase(VIEWER); ids.push_front(VIEWER)
	for id in ids:
		var actor := _actor(id)
		var row := VBoxContainer.new(); _roster.add_child(row)
		var label := TOOLTIP_LABEL.new(); label.mouse_filter = Control.MOUSE_FILTER_PASS; label.tooltip_text = _creature_tooltip(id)
		var defeated := str(actor.get("defeat_state", "")) == "defeated"
		label.text = "%s %s · %d/%d HP · %d⚡%s" % [_label(id), _name(id) if id == VIEWER else str(actor.get("character", {}).get("name", id)), int(actor.get("current_health", 0)), int(actor.get("max_health", 0)), int(actor.get("energy_points", 0)), " · defeated" if defeated else ""]
		label.add_theme_color_override("font_color", BOARD.TEAM_COLORS.get(str(actor.get("team_id", "")), Color.WHITE).lightened(0.3))
		row.add_child(label); _register("tactics.roster.%s" % id, label, label.text)
		var counts := {}
		for status in actor.get("statuses", []): counts[str(status.definition_id)] = int(status.stacks)
		if not counts.is_empty():
			var strip = STATUS_STRIP.new(); strip.custom_minimum_size = Vector2(340, 0); row.add_child(strip)
			strip.set_counts(counts)

func _render_side() -> void:
	for child in _side.get_children(): child.queue_free()
	var step := _step()
	if step == "defense_before" or step == "defense_after":
		_render_incoming()
		return
	var me := _actor(VIEWER)
	var qualified: Array = me.get("qualified_abilities", [])
	var heading := Label.new(); heading.text = "Attacks (hover for range and rules)"; heading.add_theme_color_override("font_color", Color("9fb3c8")); _side.add_child(heading)
	for ability in me.get("offensive_abilities", []):
		var legal := _ability_actions(str(ability))
		var tile := BattleAbilityTile.new(); _side.add_child(tile)
		tile.configure(str(ability), str(ability) in qualified, str(_aiming.get("ability", "")) == str(ability), not legal.is_empty() and not _busy, me)
		var delivery: Dictionary = _tv.get("creatures", {}).get(VIEWER, {}).get("deliveries", {}).get(ability, {})
		var reason := ""
		if legal.is_empty() and str(ability) in qualified and _step() == "offense_rolling": reason = "\nNo target in range and line of sight from your square this round."
		tile.tooltip_text = "%s%s\n%s" % [str(delivery.get("text", "")), reason, tile.tooltip_text]
		tile.custom_minimum_size = Vector2(340, 46)
		tile.pressed.connect(_aim.bind(str(ability)))
		_register("tactics.ability.%s" % ability, tile, _ability_name(str(ability)))
	if not me.get("defensive_abilities", []).is_empty():
		var defenses := Label.new(); defenses.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		defenses.text = "Defenses: " + ", ".join(me.defensive_abilities.map(func(id): return _ability_name(str(id)) + (" (on the move)" if BattlePresentationCatalog.definition("abilities", str(id)).get("on_the_move", false) else "")))
		_side.add_child(defenses)

func _render_incoming() -> void:
	var heading := Label.new(); heading.text = "Incoming attacks"; heading.add_theme_color_override("font_color", Color("9fb3c8")); _side.add_child(heading)
	var any := false
	var disabled: Dictionary = _tv.get("disabled_defenses", {}) if _tv.get("disabled_defenses") is Dictionary else {}
	for source in _snap.get("damage_sources", []):
		if str(source.target_actor_id) != VIEWER: continue
		any = true
		var box := VBoxContainer.new(); _side.add_child(box)
		var title := TOOLTIP_LABEL.new(); title.mouse_filter = Control.MOUSE_FILTER_PASS; title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		title.text = "%s · %s · %d damage" % [_name(str(source.source_actor_id)), _ability_name(str(source.source_content_id)), int(source.get("final_amount", 0))]
		title.tooltip_text = BattlePresentationCatalog.attack_ability_tooltip(str(source.source_content_id), _actor(str(source.source_actor_id)), {"ability_id": source.source_content_id}, [])
		box.add_child(title)
		var threatened: Array[String] = []
		for removal in _snap.get("settled_damage", {}).get("removals", []):
			if str(source.id) in removal.get("damage_proposal_ids", []) and not removal.get("released", false):
				threatened.append(str(BattlePresentationCatalog.card(str(removal.get("card_definition_id", ""))).name))
		if not threatened.is_empty():
			var cards := Label.new(); cards.text = "Threatens: " + ", ".join(threatened); cards.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; cards.add_theme_color_override("font_color", Color("ffb9a0")); box.add_child(cards)
		var defense = _snap.get("defense_history", {}).get(str(source.id))
		if defense is Dictionary:
			var done := Label.new(); done.text = "Defended with %s: prevented %d" % [_ability_name(str(defense.ability_id)), int(defense.get("prevented", 0))] if not str(defense.get("ability_id", "")).is_empty() else "No defense"; box.add_child(done)
			continue
		var row := HFlowContainer.new(); box.add_child(row)
		for action in _of_type("planning_select_ability"):
			var payload := _payload(action)
			if payload.get("target_ids", []) == [source.id]:
				_button(row, _ability_name(str(payload.ability_id)), "tactics.defense.%s.%s" % [source.id, payload.ability_id], _submit.bind(action), BattlePresentationCatalog.ability_tooltip(str(payload.ability_id)))
		for ability in disabled:
			var off := _button(row, _ability_name(str(ability)), "tactics.defense.%s.%s" % [source.id, ability], func(): pass, "%s\n%s" % [str(disabled[ability]), BattlePresentationCatalog.ability_tooltip(str(ability))])
			off.disabled = true
		for action in _of_type("planning_pass"):
			if str(_payload(action).get("source_id", "")) == str(source.id):
				_button(row, "No defense", "tactics.defense.%s.none" % source.id, _submit.bind(action))
	if not any:
		var none := Label.new(); none.text = "No attack reached you this round."; _side.add_child(none)

func _render_dice() -> void:
	var me := _actor(VIEWER)
	var dice: Dictionary = me.get("dice", {}) if me.get("dice") is Dictionary else {}
	var rolled: Array = dice.get("dice", [])
	var interactive := not _of_type("planning_reroll").is_empty()
	if not interactive: _kept = dice.get("kept_indices", []).duplicate()
	_dice.display(rolled, _kept, interactive, "", "tactics.die")
	_dice.visible = not rolled.is_empty() and str(_snap.get("segment", "")) == "offensive"

func _render_hand() -> void:
	for child in _hand.get_children(): child.queue_free()
	var me := _actor(VIEWER)
	var plan = _tv.get("plan")
	var committed: Array = plan.get("cards", []) if plan is Dictionary else []
	var played: Array = (plan.get("map_cards", []) if plan is Dictionary else []).map(func(m): return str(m.card_id))
	for id in me.get("hand", []):
		var definition := str(me.get("card_instances", {}).get(id, {}).get("definition_id", ""))
		var card := BattleCard.new(); _hand.add_child(card)
		var usable := _card_usable(str(id))
		card.configure(str(id), definition, usable, false, true)
		card.custom_minimum_size = Vector2(118, 140)
		if str(id) in committed or str(id) in played:
			card.modulate = Color("ffe08a")
			card.tooltip_text = ("Played for its map Move.\n" if str(id) in played else "Discarded for +Move.\n") + card.tooltip_text
		card.pressed.connect(func(): _on_card(str(id), Input.is_key_pressed(KEY_SHIFT)))
		_register("tactics.card.%s" % id, card, BattlePresentationCatalog.card(definition).name)
	_piles.text = "Hand %d · Deck %d · Discard %d · Removed %d · Energy %d" % [int(me.get("hand_count", 0)), int(me.get("deck_count", 0)), int(me.get("discard_count", 0)), int(me.get("removed_count", 0)), int(me.get("energy_points", 0))]

func _card_usable(id: String) -> bool:
	if _step() == "movement": return not _plans_toggling(id, false).is_empty() or not _plans_toggling(id, true).is_empty()
	for action in _actions:
		if _card_of(action) == id: return true
	return false

func _render_controls() -> void:
	for child in _controls.get_children(): child.queue_free()
	_budget.text = ""
	if not str(_result.get("battle_result", "")).is_empty(): return
	if _program_choices():
		return
	if _snap.get("stage", "") == "discard_to_hand_limit":
		_budget.text = "Discard down to your hand limit: choose the cards."
		for action in _of_type("commit_interaction"):
			var names: Array = _payload(action).get("commitment", {}).get("card_ids", []).map(func(id): return str(BattlePresentationCatalog.card(str(_actor(VIEWER).card_instances[id].definition_id)).name))
			_button(_controls, "Discard " + ", ".join(names), "", _submit.bind(action))
		return
	var step := _step()
	if step == "movement":
		_render_movement_controls()
		return
	for action in _of_type("planning_roll"):
		_button(_controls, "Roll (lock in)", "tactics.command.roll", _submit.bind(action), "Lock in your offense: your dice roll at the reveal.")
	if not _of_type("planning_reroll").is_empty():
		_button(_controls, "Reroll unkept", "tactics.command.reroll", _reroll, "Keep the dice you selected and reroll the others.")
	for action in _of_type("planning_pass"):
		var payload := _payload(action)
		if not str(payload.get("source_id", "")).is_empty(): continue
		if step == "offense_before":
			_button(_controls, "Sprint (skip attack)", "tactics.command.sprint", _submit.bind(action), "Skip your attack this round and sprint: base speed ×%d and +%d extra Move per committed card. Sprinting always runs." % [int(_tv.rules.sprint_base_multiplier), int(_tv.rules.sprint_card_bonus)])
		elif step == "offense_rolling":
			_button(_controls, "No attack", "tactics.command.pass", _submit.bind(action))
		else:
			_button(_controls, "Pass (lock in)", "tactics.command.pass", _submit.bind(action), "End your defenses and card plays for this step.")
	if not _aiming.is_empty():
		_budget.text = "Aiming %s: click a highlighted %s." % [_ability_name(str(_aiming.ability)), "square" if _aiming.get("area", false) else "creature"]
		_button(_controls, "Cancel aim", "tactics.command.cancel_aim", func(): _aiming = {}; _render())

# --- Offense ----------------------------------------------------------------

func _ability_actions(ability: String, tier: String = "") -> Array:
	return _of_type("planning_select_ability").filter(func(a): return str(_payload(a).get("ability_id", "")) == ability and (tier.is_empty() or str(_payload(a).get("tier_id", "")) == tier) and _step() != "defense_before")

func _aim(ability: String) -> void:
	var legal := _ability_actions(ability)
	if legal.is_empty(): return
	var tiers: Array = []
	for action in legal:
		var tier := str(_payload(action).get("tier_id", ""))
		if not tier in tiers: tiers.append(tier)
	if tiers.size() > 1:
		var options: Array = []
		for tier in tiers:
			var summary: String = str(tier)
			for t in BattlePresentationCatalog.offensive_tier_summaries(ability, _actor(VIEWER)):
				if str(t.id) == tier: summary = "%s → %s" % [t.recipe, t.summary]
			options.append({"text": summary, "id": "tactics.tier.%s" % tier, "call": _aim_tier.bind(ability, tier)})
		_show_popup("Choose a tier", options)
		return
	_aim_tier(ability, tiers[0])

func _aim_tier(ability: String, tier: String) -> void:
	_hide_popup()
	_aiming = {"ability": ability, "tier": tier, "area": _payload(_ability_actions(ability, tier)[0]).get("target_square") != null}
	_render()

func _apply_aiming_highlights() -> void:
	if _aiming.is_empty(): return
	for action in _ability_actions(str(_aiming.ability), str(_aiming.tier)):
		var payload := _payload(action)
		if payload.get("target_square") is Dictionary:
			_board.highlights[Vector2i(int(payload.target_square.x), int(payload.target_square.y))] = Color("ffd75e")
		for id in payload.get("target_ids", []): _board.clickable_creatures[str(id)] = true

func _reroll() -> void:
	var me := _actor(VIEWER)
	var dice: Dictionary = me.get("dice", {})
	var kept: Array = _kept.duplicate(); kept.sort()
	var server_kept: Array = dice.get("kept_indices", []).duplicate(); server_kept.sort()
	if kept != server_kept:
		for action in _of_type("planning_keep"):
			var indices: Array = _payload(action).get("kept_indices", []).map(func(i): return int(i)); indices.sort()
			if indices == kept: _submit(action); break
	var reroll: Array = []
	for i in dice.get("dice", []).size():
		if not i in kept: reroll.append(i)
	for action in _of_type("planning_reroll"):
		var indices: Array = _payload(action).get("reroll_indices", []).map(func(i): return int(i)); indices.sort()
		if indices == reroll: _submit(action); return

# --- Board clicks -------------------------------------------------------------

func _on_square(square: Vector2i) -> void:
	if not _aiming.is_empty():
		for action in _ability_actions(str(_aiming.ability), str(_aiming.tier)):
			var target = _payload(action).get("target_square")
			if target is Dictionary and Vector2i(int(target.x), int(target.y)) == square: _submit(action); return
		return
	if _step() == "movement":
		var plan := _plan_with({"kind": "move_to", "square": {"x": square.x, "y": square.y}})
		if not plan.is_empty(): _submit(plan)

func _on_creature(id: String) -> void:
	if not _aiming.is_empty():
		for action in _ability_actions(str(_aiming.ability), str(_aiming.tier)):
			if _payload(action).get("target_ids", []) == [id]: _submit(action); return
		return
	if _step() == "movement" and id != VIEWER:
		var options: Array = []
		for kind in ["pursue", "flee"]:
			var plan := _plan_with({"kind": kind, "target": id})
			if not plan.is_empty(): options.append({"text": ("Pursue %s" if kind == "pursue" else "Flee from %s") % _name(id), "id": "tactics.intent.%s" % kind, "call": _submit.bind(plan)})
		if not options.is_empty(): _show_popup("Move relative to %s" % _name(id), options)

# --- Movement -----------------------------------------------------------------

func _plans() -> Array: return _of_type("tactics_movement_plan")

func _current_plan() -> Dictionary:
	for action in _plans():
		if _payload(action).get("lock_in", false): return action
	return {}

func _same_cards(a: Array, b: Array) -> bool:
	var x := a.map(func(v): return str(v)); var y := b.map(func(v): return str(v)); x.sort(); y.sort()
	return x == y

func _map_key(list: Array) -> Array:
	var out := list.map(func(m): return "%s|%s" % [str(m.card_id), str(m.get("terrain", ""))]); out.sort()
	return out

func _plan_matches(action: Dictionary, intention: Dictionary, cards: Array, maps: Array, energy: int) -> bool:
	var p := _payload(action)
	if p.get("lock_in", false): return false
	var i: Dictionary = p.get("intention", {})
	if str(i.get("kind", "")) != str(intention.get("kind", "")) or str(i.get("target", "")) != str(intention.get("target", "")): return false
	if intention.get("square") is Dictionary and not (i.get("square") is Dictionary and int(i.square.x) == int(intention.square.x) and int(i.square.y) == int(intention.square.y)): return false
	return _same_cards(p.get("card_ids", []), cards) and _map_key(p.get("map_cards", [])) == _map_key(maps) and int(p.get("energy", 0)) == energy

func _find_plan(intention: Dictionary, cards: Array, maps: Array, energy: int) -> Dictionary:
	for action in _plans():
		if _plan_matches(action, intention, cards, maps, energy): return action
	return {}

## A plan one change away: a new intention keeping the commitments (or none,
## for Hold).
func _plan_with(intention: Dictionary) -> Dictionary:
	var current := _payload(_current_plan())
	if current.is_empty(): return {}
	if intention.kind == "hold": return _find_plan(intention, [], [], 0)
	var found := _find_plan(intention, current.get("card_ids", []), current.get("map_cards", []), int(current.get("energy", 0)))
	return found if not found.is_empty() else _find_plan(intention, [], [], 0)

func _plans_toggling(card: String, as_map: bool) -> Array:
	var current := _payload(_current_plan())
	if current.is_empty(): return []
	var out: Array = []
	for action in _plans():
		var p := _payload(action)
		if p.get("lock_in", false) or p.get("intention") != current.get("intention") or int(p.get("energy", 0)) != int(current.get("energy", 0)): continue
		var cards: Array = p.get("card_ids", []).map(func(v): return str(v))
		var maps: Array = p.get("map_cards", []).map(func(m): return str(m.card_id))
		var had_card: bool = card in current.get("card_ids", []).map(func(v): return str(v))
		var had_map: bool = card in current.get("map_cards", []).map(func(m): return str(m.card_id))
		if as_map and (card in maps) != had_map and not card in cards: out.append(action)
		if not as_map and (card in cards) != had_card and not card in maps: out.append(action)
	return out

func _on_card(id: String, discard_only: bool = false) -> void:
	if _busy: return
	if _step() == "movement":
		var as_map := _plans_toggling(id, true)
		var discard := _plans_toggling(id, false)
		if not discard_only and not as_map.is_empty():
			if as_map.size() == 1: _submit(as_map[0]); return
			var options: Array = []
			for action in as_map:
				var terrain := ""
				for m in _payload(action).get("map_cards", []):
					if str(m.card_id) == id: terrain = str(m.get("terrain", ""))
				options.append({"text": "Make %s cost 1" % str(_tv.rules.terrain.get(terrain, {}).get("name", terrain)), "id": "tactics.terrain.%s" % terrain, "call": _submit.bind(action)})
			_show_popup("Choose a terrain", options)
			return
		if not discard.is_empty(): _submit(discard[0])
		return
	var starts: Array = []
	for action in _actions:
		if _card_of(action) != id: continue
		var choice := _choice(action)
		if choice.is_empty() or str(choice.get("verb", "")) == "start": starts.append(action)
	if starts.size() == 1: _submit(starts[0]); return
	if starts.size() > 1:
		var options: Array = []
		for action in starts: options.append({"text": _action_label(action), "id": "", "call": _submit.bind(action)})
		_show_popup(str(BattlePresentationCatalog.card(str(_actor(VIEWER).card_instances[id].definition_id)).name), options)

func _action_label(action: Dictionary) -> String:
	var choice := _choice(action)
	if not str(choice.get("label", "")).is_empty(): return str(choice.label)
	var payload := _payload(action)
	if not str(payload.get("ability_id", "")).is_empty(): return "On " + _ability_name(str(payload.ability_id))
	var proposals: Array = payload.get("commitment", {}).get("proposal_ids", [])
	if not proposals.is_empty(): return "On attack " + str(proposals[0])
	return "Play"

## While a card's choices are open, its options are the only legal actions.
func _program_choices() -> bool:
	if _actions.is_empty(): return false
	for action in _actions:
		var choice := _choice(action)
		if choice.is_empty() or str(choice.get("verb", "")) == "start": return false
	var options: Array = []
	for action in _actions:
		var choice := _choice(action)
		options.append({"text": "%s %s" % [str(choice.get("verb", "")).capitalize(), str(choice.get("label", ""))], "id": "tactics.choice.%d" % options.size(), "call": _submit.bind(action)})
	_budget.text = "Finish the card: choose an option."
	for option in options: _button(_controls, option.text, option.id, option.call)
	return true

func _render_movement_controls() -> void:
	var plan = _tv.get("plan")
	if not plan is Dictionary: return
	var budget: Dictionary = plan.get("budget", {})
	var parts: Array[String] = []
	for key in ["base", "charge", "cards", "energy", "map", "sprint_bonus"]:
		if int(budget.get(key, 0)) > 0: parts.append("%s %d" % [key.replace("_", " "), int(budget[key])])
	var intention: Dictionary = plan.get("intention", {})
	var what := "Hold"
	match str(intention.get("kind", "")):
		"move_to": what = "Move to (%d, %d)" % [int(intention.square.x), int(intention.square.y)]
		"pursue": what = "Pursue " + _name(str(intention.target))
		"flee": what = "Flee from " + _name(str(intention.target))
	var text := "Plan: %s · Move %d (%s)" % [what, int(budget.get("total", 0)), ", ".join(parts) if not parts.is_empty() else "none"]
	if not str(plan.get("charge", "")).is_empty(): text += " · Charge"
	if plan.get("sprint", false): text += " · Sprint"
	if budget.get("running", false): text += "\n⚠ RUNNING: more Move than your base speed — only on-the-move defenses this round."
	var preview = plan.get("preview")
	if preview is Dictionary:
		text += "\nPreview (if everyone else holds): ends at (%d, %d)" % [int(preview.final.x), int(preview.final.y)]
		if preview.get("arrive") is Dictionary: text += " at t = %s" % _frac_text(preview.arrive)
		if not str(preview.get("stop", "")).is_empty(): text += " · %s" % str(preview.stop).replace("_", " ")
	_budget.text = text
	_budget.tooltip_text = "Move: base speed free, +%d per discarded card, +1 per %d energy, a map Move card's amount when played. Click a square to move there, a creature to pursue or flee, a hand card to commit it (shift-click a map card to discard it for +1 instead)." % [int(_tv.rules.card_move), int(_tv.rules.energy_per_move)]
	var hold := _plan_with({"kind": "hold"})
	var hold_button := _button(_controls, "Hold", "tactics.intent.hold", _submit.bind(hold), "Stay put. Commits nothing.")
	hold_button.disabled = hold.is_empty() or str(intention.get("kind", "")) == "hold"
	var current := _payload(_current_plan())
	var energy := int(current.get("energy", 0))
	var step := int(_tv.rules.energy_per_move)
	var less := _find_plan(current.get("intention", {}), current.get("card_ids", []), current.get("map_cards", []), energy - step) if energy >= step else {}
	var more := _find_plan(current.get("intention", {}), current.get("card_ids", []), current.get("map_cards", []), energy + step)
	var minus := _button(_controls, "−%d⚡" % step, "tactics.energy.less", _submit.bind(less), "Spend less energy on Move."); minus.disabled = less.is_empty()
	var plus := _button(_controls, "+%d⚡ (+1 Move)" % step, "tactics.energy.more", _submit.bind(more), "Spend %d more energy for +1 Move." % step); plus.disabled = more.is_empty()
	var lock := _current_plan()
	var lock_button := _button(_controls, "Lock in plan", "tactics.command.lock_in", _submit.bind(lock), "Lock in your movement. It stays secret until everyone has locked in.")
	lock_button.disabled = lock.is_empty()

# --- Popup ----------------------------------------------------------------------

func _show_popup(title: String, options: Array) -> void:
	for child in _popup.get_children(): child.queue_free()
	var box := VBoxContainer.new(); _popup.add_child(box)
	var heading := Label.new(); heading.text = title; box.add_child(heading)
	for option in options: _button(box, str(option.text), str(option.get("id", "")), option.call)
	_button(box, "Cancel", "tactics.popup.cancel", _hide_popup)
	_popup.visible = true
	_popup.reset_size()
	var at := get_local_mouse_position() + Vector2(12, 12)
	_popup.position = at.clamp(Vector2.ZERO, size - _popup.get_combined_minimum_size() - Vector2(8, 8))

func _hide_popup() -> void:
	if _popup != null: _popup.visible = false

# --- Playback ---------------------------------------------------------------------

func _cycle_speed() -> void:
	_speed = {1.0: 2.0, 2.0: 0.5, 0.5: 1.0}.get(_speed, 1.0)
	_speed_button.text = "Speed %s×" % ("½" if _speed == 0.5 else str(int(_speed)))

func _replay() -> void:
	var movement = _tv.get("movement")
	if movement is Dictionary: _start_playback(movement)

func _start_playback(movement: Dictionary) -> void:
	_playback = {"movement": movement, "started": Time.get_ticks_msec(), "struck": {}}
	_board.floaters = []; _board.falling = {}

func _process(_delta: float) -> void:
	if _playback.is_empty(): return
	var movement: Dictionary = _playback.movement
	var elapsed := (Time.get_ticks_msec() - int(_playback.started)) / 1000.0 * _speed
	var t := clampf(elapsed / PLAYBACK_SECONDS, 0.0, 1.0)
	for id in movement.get("paths", {}):
		if _board.creatures.has(id): _board.creatures[id]["square"] = _position_at(movement.paths[id], t)
	_board.ghosts = {}
	for hit in movement.get("hits", []): _strike(movement, hit, t)
	if t >= 1.0:
		for hit in movement.get("hits", []):
			if hit.get("lethal", false): _board.falling[str(hit.target)] = clampf((elapsed - PLAYBACK_SECONDS) / 0.8, 0, 1)
	_board._place_tokens(); _board.queue_redraw()
	for id in _board._token_buttons: _board._token_buttons[id].queue_redraw()
	if elapsed > PLAYBACK_SECONDS + 2.0:
		_playback = {}
		_board.floaters = []
		_board.falling = {}
		_render_board()

func _position_at(path: Array, t: float) -> Vector2:
	var at := _square(path[0].square)
	for i in range(1, path.size()):
		var t0 := _frac(path[i - 1].t); var t1 := _frac(path[i].t)
		if t >= t1: at = _square(path[i].square); continue
		if t > t0: return _square(path[i - 1].square).lerp(_square(path[i].square), clampf((t - t0) / maxf(0.001, t1 - t0), 0.0, 1.0))
		break
	return at

## A hit lands at its moment: direct at t = 0, a charge at contact, an area
## attack when movement ends. Its defense result shows with it.
func _strike(movement: Dictionary, hit: Dictionary, t: float) -> void:
	var key := str(hit.source_id)
	if _playback.struck.has(key): return
	var at := 0.0
	for attack in movement.get("attacks", []):
		if str(attack.attacker) != str(hit.attacker) or str(attack.ability_id) != str(hit.ability_id): continue
		if str(attack.delivery) == "area": at = 1.0
		elif str(attack.delivery) == "charge" and attack.get("contact_at") is Dictionary: at = _frac(attack.contact_at)
	if t < at: return
	_playback.struck[key] = true
	var square: Vector2 = _board.creatures.get(str(hit.target), {}).get("square", Vector2.ZERO)
	var text := "−%d" % int(hit.final)
	if int(hit.base) > int(hit.final): text += " (%s %d)" % [_ability_name(str(hit.get("defense", ""))) if not str(hit.get("defense", "")).is_empty() else "saved", int(hit.base) - int(hit.final)]
	_board.floaters.append({"position": square, "text": "%s %s" % [_ability_name(str(hit.ability_id)), text], "color": Color("ffb36b"), "born_ms": Time.get_ticks_msec()})

# --- Timeline ---------------------------------------------------------------------

func _render_timeline() -> void:
	var movement = _tv.get("movement")
	if not movement is Dictionary:
		_timeline_text.text = "No movement resolved yet."
		return
	var lines: Array[String] = ["[b]Round %d[/b]" % int(movement.round)]
	for id in movement.get("budgets", {}):
		var b: Dictionary = movement.budgets[id]
		lines.append("%s: Move %d (paid %d)%s" % [_label(id), int(b.total), int(b.paid), " · running" if b.get("running", false) else ""])
	for entry in movement.get("timeline", []):
		var t := "%s (%d%%)" % [_frac_text(entry.t), int(round(_frac(entry.t) * 100))]
		var who := _label(str(entry.get("creature", "")))
		match str(entry.kind):
			"step": lines.append("%s %s steps (%d,%d)→(%d,%d) cost %d" % [t, who, int(entry.from.x), int(entry.from.y), int(entry.to.x), int(entry.to.y), int(entry.get("cost", 0))])
			"wait": lines.append("%s %s waits at (%d,%d): %s" % [t, who, int(entry.square.x), int(entry.square.y), str(entry.reason).replace("_", " ")])
			"tiebreak": lines.append("%s tiebreak at (%d,%d): %s wins over %s by %s" % [t, int(entry.square.x), int(entry.square.y), _label(str(entry.winner)), ", ".join(entry.get("losers", []).map(func(l): return _label(str(l)))), str(entry.rule).replace("_", " ")])
			"contact": lines.append("%s %s touches %s (charge contact)" % [t, who, _label(str(entry.target))])
			"stop": lines.append("%s %s stops at (%d,%d): %s" % [t, who, int(entry.square.x), int(entry.square.y), str(entry.reason).replace("_", " ")])
	for attack in movement.get("attacks", []):
		lines.append("%s %s: %s %s" % [_label(str(attack.attacker)), _ability_name(str(attack.ability_id)), str(attack.get("outcome", "")), ", ".join(attack.get("hits", []).map(func(h): return _label(str(h))))])
	for hit in movement.get("hits", []):
		lines.append("hit %s → %s: %d of %d%s" % [_label(str(hit.attacker)), _label(str(hit.target)), int(hit.final), int(hit.base), " (lethal)" if hit.get("lethal", false) else ""])
	_timeline_text.text = "\n".join(lines)

# --- End ---------------------------------------------------------------------

func _show_end(outcome: String) -> void:
	store.clear()
	if _end != null: return
	_end = PanelContainer.new(); _end.add_theme_stylebox_override("panel", _panel_style()); _end.custom_minimum_size = Vector2(420, 0); add_child(_end)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 8); _end.add_child(box)
	var title := Label.new(); title.text = {"victory": "VICTORY", "defeat": "DEFEAT", "draw": "DRAW"}.get(outcome, outcome.to_upper()); title.add_theme_font_size_override("font_size", 48); title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_color_override("font_color", {"victory": Color("f5c963"), "defeat": Color("ff8a78")}.get(outcome, Color.WHITE)); box.add_child(title)
	var summary := Label.new(); summary.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; summary.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	summary.text = "After %d rounds on %s." % [int(_snap.get("round", 0)), str(_tv.get("arena", {}).get("name", ""))]; box.add_child(summary)
	_register("tactics.end.result", title, title.text)
	_button(box, "Replay last round", "tactics.end.replay", _replay)
	_button(box, "New sandbox battle", "tactics.end.new", _to_setup)
	_button(box, "Main menu", "tactics.end.menu", _to_menu)
	_end.reset_size()
	_end.position = (size - _end.get_combined_minimum_size()) * 0.5

func _to_setup() -> void:
	get_tree().root.add_child(preload("res://app/screens/tactics/tactics_setup.gd").new())
	queue_free()

func _to_menu() -> void:
	get_tree().root.add_child(preload("res://app/boot/battle_bootstrap.tscn").instantiate())
	queue_free()

func inspection_state() -> Dictionary:
	var creatures := {}
	for id in _tv.get("creatures", {}):
		var actor := _actor(id)
		creatures[id] = {"at": "%d,%d" % [int(_tv.creatures[id].position.x), int(_tv.creatures[id].position.y)], "hp": "%d/%d" % [int(actor.get("current_health", 0)), int(actor.get("max_health", 0))], "energy": int(actor.get("energy_points", 0)), "hand": int(actor.get("hand_count", 0)), "statuses": actor.get("statuses", []).map(func(st): return "%s×%d" % [st.definition_id, int(st.stacks)])}
	var dice: Array = []
	if _actor(VIEWER).get("dice") is Dictionary:
		for die in _actor(VIEWER).dice.get("dice", []): dice.append(int(die.get("face", 0)))
	var attacks: Array = []
	for attack in _tv.get("attacks", []): attacks.append("%s %s → %s %s" % [attack.attacker, attack.ability_id, str(attack.get("target", attack.get("square", ""))), str(attack.get("outcome", ""))])
	var plan = _tv.get("plan")
	return {"ready": true, "battle_id": str(_snap.get("battle_id", "")), "round": int(_snap.get("round", 0)), "segment": str(_snap.get("segment", "")), "stage": str(_snap.get("stage", "")), "step": _step(), "legal_actions": _actions.size(), "battle_result": str(_result.get("battle_result", "")), "playing_back": not _playback.is_empty(), "aiming": _aiming, "error": _error.text,
		"creatures": creatures, "dice": dice, "attacks": attacks, "plan": _budget.text if plan is Dictionary else "", "hand": _actor(VIEWER).get("hand", []).map(func(id): return "%s=%s" % [id, str(_actor(VIEWER).card_instances[id].definition_id)])}
