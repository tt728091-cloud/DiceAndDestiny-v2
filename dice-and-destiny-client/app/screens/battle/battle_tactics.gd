extends RefCounted
## The Tactics Sandbox mode of the battle screen (tactics-sandbox-plan.md §9).
## The battle screen keeps every part of the ordinary battle (HUD, dice,
## abilities, hand, stats, defense flow); this adds what the grid needs: the
## battlefield in the scenery's place, enemy stats pinned at the upper right,
## area aiming, the secret movement step, the playback of each resolved round
## and the debug timeline. Everything comes from the authority's viewer-safe
## view and legal actions; nothing here computes a rule.

const BATTLEFIELD := preload("res://presentation/battle/tactics_battlefield.gd")
const TOOLTIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const PLAYBACK_SECONDS := 3.2
const NAME_INITIALS := {"Starter Movement": "SM", "Adventurer Movement": "AM", "Ribbon Eel": "RE", "Bell Diver": "BD", "Brine Mask": "BM", "The Drowned Regent": "DR"}
const ENEMY_COLUMN_X := 1566.0
const ENEMY_COLUMN_TOP := 92.0

var screen
## The battlefield's view, kept across the screen's rebuilds.
var pan := Vector2.ZERO
var zoom := 1.0
var speed := 1.0
var playback: Dictionary = {}
var played_round := -1
## An area attack waiting for its square: {ability, base: payload without the square}.
var aim: Dictionary = {}
var board: Control
var _popup: PanelContainer
var _timeline_text: RichTextLabel
var _speed_button: Button

func _init(owner) -> void:
	screen = owner

# --- Data -------------------------------------------------------------------

func tv() -> Dictionary:
	var value = screen._view.raw_snapshot.get("tactics")
	return value if value is Dictionary else {}

func viewer() -> String: return screen.viewer_actor_id

func actor(id: String) -> Dictionary: return screen._view.actor(id)

func in_movement() -> bool: return str(tv().get("step", "")) == "movement"

func display_name(id: String) -> String:
	return "You" if id == viewer() else screen._actor_display_name(id)

func label(id: String) -> String:
	var name := str(actor(id).get("character", {}).get("name", id))
	var initials := str(NAME_INITIALS.get(name, name.substr(0, 2).to_upper()))
	if id.begins_with("enemy-"): initials += id.trim_prefix("enemy-")
	return initials

func ability_name(id: String) -> String: return str(BattlePresentationCatalog.ability(id).get("name", id))

static func square(value) -> Vector2:
	if value is Dictionary: return Vector2(float(value.get("x", 0)), float(value.get("y", 0)))
	return Vector2.ZERO

static func frac(value) -> float:
	if value is Dictionary and float(value.get("den", 1)) != 0: return float(value.get("num", 0)) / float(value.get("den", 1))
	return 0.0

static func frac_text(value) -> String:
	if not value is Dictionary: return "0"
	var den := int(value.get("den", 1))
	return str(int(value.get("num", 0))) if den == 1 else "%d/%d" % [int(value.get("num", 0)), den]

func _payload(action: Dictionary) -> Dictionary:
	var payload = action.get("payload", {})
	return payload if payload is Dictionary else {}

func _of_type(type: String) -> Array:
	return screen._view.legal_actions.filter(func(a): return str(a.get("type", "")) == type)

## The round's resolved movement while its defense is open (tokens stay on
## their starting squares then, with paths and ghosts, plan §9.4).
func movement_this_round() -> Dictionary:
	var movement = tv().get("movement")
	if movement is Dictionary and int(movement.get("round", 0)) == screen._view.round_number and screen._view.segment in ["defensive", "damage_resolution"]: return movement
	return {}

# --- Battlefield ------------------------------------------------------------

## Builds the grid in the scenery's place and returns it; its tokens are the
## fighters the enemy stats, attack intents and offensive targets use.
func build_battlefield(root: Control) -> Control:
	board = BATTLEFIELD.new(); board.name = "TacticsBattlefield"; board.view_state = self
	root.add_child(board)
	board.position = Vector2.ZERO; board.size = Vector2(1920, 1080)
	board.square_clicked.connect(on_square); board.creature_clicked.connect(on_creature)
	var rules: Dictionary = tv().get("rules", {})
	var colors := {}; var names := {}; var costs := {}
	for id in rules.get("terrain", {}):
		var terrain: Dictionary = rules.terrain[id]
		colors[id] = Color(str(terrain.get("color", "#333333"))); names[id] = str(terrain.get("name", id))
		costs[id] = "costs %d Move" % int(terrain.cost) if terrain.get("passable", false) else "impassable"
		if terrain.get("blocks_line_of_sight", false): costs[id] += ", blocks line of sight"
	board.set_arena({"terrain": tv().get("arena", {}).get("terrain", []), "colors": colors, "names": names, "costs": costs}, Callable(screen, "_inspect"))
	_maybe_start_playback()
	render_board()
	return board

func render_board() -> void:
	if not is_instance_valid(board): return
	var movement := movement_this_round()
	var creatures := {}
	for id in tv().get("creatures", {}):
		var c: Dictionary = tv().creatures[id]
		var a := actor(id)
		var at := square(c.position)
		if not movement.is_empty() and movement.get("start", {}).has(id): at = square(movement.start[id])
		var statuses := {}
		for status in a.get("statuses", []): statuses[str(status.definition_id)] = int(status.stacks)
		creatures[id] = {"square": at, "team": str(a.get("team_id", "")), "label": label(id), "name": display_name(id), "viewer": id == viewer(),
			"focused": id == screen._focused_enemy and screen._multiple_enemies(), "defeated": str(a.get("defeat_state", "")) == "defeated",
			"running": statuses.has("running"), "hp": int(a.get("current_health", 0)), "max_hp": int(a.get("max_health", 1)), "tooltip": creature_tooltip(id)}
	board.creatures = creatures
	board.areas = []; board.paths = []; board.ghosts = {}; board.badges = []; board.highlights = {}; board.clickable_creatures = {}
	var attacks: Array = tv().get("attacks", [])
	if not movement.is_empty(): attacks = movement.get("attacks", attacks)
	for attack in attacks:
		var outcome := str(attack.get("outcome", ""))
		var missed := bool(attack.get("resolved", false)) and outcome != "hit"
		if str(attack.delivery) == "area":
			board.areas.append({"squares": attack.area.map(func(s): return Vector2i(int(s.x), int(s.y))), "color": Color(0.9, 0.35, 0.15, 0.18 if missed else 0.34)})
			board.badges.append(_badge(attack, outcome))
		elif missed:
			# Connected attacks show as the battle's own intents at the attacker.
			board.badges.append(_badge(attack, outcome))
	if not movement.is_empty():
		for id in movement.get("paths", {}):
			var points: Array = movement.paths[id].map(func(w): return square(w.square))
			if points.size() > 1:
				board.paths.append({"points": points, "color": BATTLEFIELD.TEAM_COLORS.get(str(actor(id).get("team_id", "")), Color.GRAY), "width": 3.0})
				board.ghosts[id] = points[-1]
	var plan = tv().get("plan")
	if in_movement() and plan is Dictionary and plan.get("preview") is Dictionary:
		var points: Array = plan.preview.path.map(func(w): return square(w.square))
		if points.size() > 1:
			board.paths.append({"points": points, "color": Color("ffd75e"), "width": 3.0, "dashed": true})
			board.ghosts[viewer()] = points[-1]
	for action in aim_actions():
		var target = _payload(action).get("target_square")
		board.highlights[Vector2i(int(target.x), int(target.y))] = Color("ffd75e")
	for id in screen._offensive_target_actions(): board.clickable_creatures[str(id)] = true
	board.refresh(Callable(screen, "_inspect"))
	process()

func _badge(attack: Dictionary, outcome: String) -> Dictionary:
	var attacker := str(attack.attacker)
	var text := "%s %s" % [label(attacker), ability_name(str(attack.ability_id))]
	var color := Color("7a1f17")
	if outcome == "dodged": text += " · dodged"; color = Color("4b5563")
	elif outcome == "out_of_reach": text += " · out of reach"; color = Color("4b5563")
	var delivery: Dictionary = tv().get("creatures", {}).get(attacker, {}).get("deliveries", {}).get(str(attack.ability_id), {})
	var badge := {"id": "%s.%s" % [attacker, attack.ability_id], "text": text, "color": color,
		"tooltip": "%s · %s\n%s\n%s" % [display_name(attacker), ability_name(str(attack.ability_id)), str(delivery.get("text", "")), outcome_text(attack)]}
	if str(attack.delivery) == "area":
		badge["square"] = Vector2i(int(attack.square.x), int(attack.square.y))
		badge["below"] = true
	else:
		badge["target"] = str(attack.get("target", ""))
	return badge

func outcome_text(attack: Dictionary) -> String:
	match str(attack.get("outcome", "")):
		"hit":
			var hits: Array = attack.get("hits", [])
			return "Connected: hits %s." % ", ".join(hits.map(func(id): return display_name(str(id))))
		"dodged": return "Dodged: nobody was in the area when movement ended."
		"out_of_reach": return "Out of reach: the charger never got adjacent."
	if str(attack.delivery) == "area": return "Hits every enemy standing in the area when movement ends."
	if str(attack.delivery) == "charge": return "Connects the moment the charger is adjacent to its target."
	return "Direct: lands wherever the target runs."

func creature_tooltip(id: String) -> String:
	var c: Dictionary = tv().get("creatures", {}).get(id, {})
	var a := actor(id)
	var lines: Array[String] = ["%s · %d/%d health · %d energy" % [display_name(id), int(a.get("current_health", 0)), int(a.get("max_health", 0)), int(a.get("energy_points", 0))]]
	var speed_text := "Base speed %d" % int(c.get("base_speed", 1))
	if not str(c.get("label", "")).is_empty(): speed_text += " · " + str(c.label)
	var movement = tv().get("movement")
	if movement is Dictionary and movement.get("budgets", {}).has(id): speed_text += " · last Move %d%s" % [int(movement.budgets[id].total), " (ran)" if movement.budgets[id].get("running", false) else ""]
	if id == viewer() and tv().get("plan") is Dictionary: speed_text += " · planned Move %d" % int(tv().plan.get("budget", {}).get("total", 0))
	lines.append(speed_text)
	for ability in c.get("deliveries", {}):
		lines.append("%s: %s" % [ability_name(str(ability)), str(c.deliveries[ability].get("text", ""))])
	for status in a.get("statuses", []):
		var data := BattlePresentationCatalog.status(str(status.definition_id))
		lines.append("%s ×%d: %s" % [str(data.get("name", status.definition_id)), int(status.stacks), str(data.get("text", ""))])
	return "\n".join(lines)

## Enemy stats stand at the upper right, one under another, in seat order;
## clicking a name shows that enemy's dice and focuses it, as in battle.
## The cards your attack threatens hang beneath the target's stats, as in
## battle; the next enemy's stats start below them.
func layout_enemy_docks() -> void:
	var y := ENEMY_COLUMN_TOP
	for id in screen._enemy_ids():
		var profile = screen._actor_profiles.get(id)
		if not is_instance_valid(profile): continue
		var dock: Control = profile.get_parent()
		dock.position = Vector2(ENEMY_COLUMN_X, y)
		y += dock.get_combined_minimum_size().y + 10
		var stack = screen._damage_stack_docks.get(id)
		if is_instance_valid(stack) and stack.visible and stack.size.y > 0: y += stack.size.y + 6
		var dice = screen._enemy_dice_docks.get(id)
		if is_instance_valid(dice): dice.place()

## An attack the movement dodged or left out of reach has no effect at all;
## the battle screen shows no intent or incoming attack for it.
func attack_missed(actor_id: String) -> bool:
	var attacks: Array = tv().get("attacks", [])
	var movement := movement_this_round()
	if not movement.is_empty(): attacks = movement.get("attacks", attacks)
	for attack in attacks:
		if str(attack.attacker) == actor_id and bool(attack.get("resolved", false)) and str(attack.get("outcome", "")) != "hit": return true
	return false

# --- Clicks -----------------------------------------------------------------

func on_square(at: Vector2i) -> void:
	if screen._submitting or screen._director.has_beats(): return
	if not aim.is_empty():
		for action in aim_actions():
			var target = _payload(action).get("target_square")
			if Vector2i(int(target.x), int(target.y)) == at:
				aim = {}
				screen._send(JSON.stringify(action))
				return
		return
	if in_movement():
		var plan := plan_with({"kind": "move_to", "square": {"x": at.x, "y": at.y}})
		if not plan.is_empty(): screen._send(JSON.stringify(plan))

func on_creature(id: String) -> void:
	if screen._submitting or screen._director.has_beats(): return
	if in_movement() and id != viewer():
		var options: Array = []
		for kind in ["pursue", "flee"]:
			var plan := plan_with({"kind": kind, "target": id})
			if not plan.is_empty(): options.append({"text": ("Pursue %s" if kind == "pursue" else "Flee from %s") % display_name(id), "id": "battle.tactics.intent.%s" % kind, "call": func(): screen._send(JSON.stringify(plan))})
		if not options.is_empty(): show_popup("Move relative to %s" % display_name(id), options, board.tokens[id].get_global_rect() if board.tokens.has(id) else Rect2())
		return
	if not screen._pending_attack.is_empty():
		screen._choose_offensive_target(id)
		return
	# As on the battle screen: clicking an enemy focuses it and shows its dice.
	var button = screen._enemy_buttons.get(id)
	if is_instance_valid(button): button.pressed.emit()

# --- Offense: area aiming ---------------------------------------------------

## The legal actions that differ only in their target are one choice on the
## ability rail; the target comes from the grid.
func group_actions(actions: Array) -> Array:
	var seen := {}
	var out: Array = []
	for action in actions:
		var key: Dictionary = _payload(action).duplicate(true)
		key.erase("target_ids"); key.erase("target_square")
		var text := JSON.stringify(key)
		if seen.has(text): continue
		seen[text] = true
		out.append(action)
	return out

## An area ability waits for its square: legal squares light up on the grid.
func begin_square_aim(action: Dictionary) -> bool:
	if not _payload(action).get("target_square") is Dictionary: return false
	var base: Dictionary = _payload(action).duplicate(true); base.erase("target_square")
	aim = {} if aim.get("base") == base else {"ability": str(base.get("ability_id", "")), "base": base}
	screen._selected_card.clear(); screen._pending_attack.clear()
	screen._render(true)
	return true

func aim_actions() -> Array:
	if aim.is_empty(): return []
	return _of_type("planning_select_ability").filter(func(a):
		var payload: Dictionary = _payload(a).duplicate(true)
		if not payload.get("target_square") is Dictionary: return false
		payload.erase("target_square")
		return payload == aim.base)

func add_aim_controls() -> void:
	if aim.is_empty(): return
	if aim_actions().is_empty(): aim = {}; return
	var cancel := TOOLTIP_BUTTON.new()
	cancel.text = "Choose a square · Cancel"
	cancel.tooltip_text = "%s: click a highlighted square on the grid, or cancel." % ability_name(str(aim.ability))
	cancel.pressed.connect(func(): aim = {}; screen._render(true))
	screen._ability_dock.add_child(cancel)
	screen._inspect(cancel, "battle.tactics.cancel_aim", cancel.tooltip_text)

## Ability tiles name their reach, and why an attack has no target.
func annotate_ability_tile(tile: Control, ability_id: String, defending: bool, qualified: bool, can_select: bool) -> void:
	if defending:
		var reason := str(tv().get("disabled_defenses", {}).get(ability_id, "")) if tv().get("disabled_defenses") is Dictionary else ""
		if not reason.is_empty(): tile.tooltip_text = "%s\n\n%s" % [reason, tile.tooltip_text]
		return
	var delivery: Dictionary = tv().get("creatures", {}).get(viewer(), {}).get("deliveries", {}).get(ability_id, {})
	var reach := str(delivery.get("text", ""))
	if qualified and not can_select and screen._simultaneous_step() == "offense_rolling": reach += "\nNo target in range and line of sight from your square this round."
	if not reach.is_empty(): tile.tooltip_text = "%s\n\n%s" % [reach, tile.tooltip_text]

func sprint_tooltip() -> String:
	var rules: Dictionary = tv().get("rules", {})
	return "Sprint: skip your attack this round and move faster — base speed ×%d and +%d extra Move per committed card. Sprinting always runs (only on-the-move defenses)." % [int(rules.get("sprint_base_multiplier", 2)), int(rules.get("sprint_card_bonus", 1))]

# --- Movement step ----------------------------------------------------------

func plans() -> Array: return _of_type("tactics_movement_plan")

func current_plan() -> Dictionary:
	for action in plans():
		if _payload(action).get("lock_in", false): return action
	return {}

func _same(a: Array, b: Array) -> bool:
	var x := a.map(func(v): return str(v)); var y := b.map(func(v): return str(v)); x.sort(); y.sort()
	return x == y

func _map_key(list: Array) -> Array:
	var out := list.map(func(m): return "%s|%s" % [str(m.card_id), str(m.get("terrain", ""))]); out.sort()
	return out

func _find_plan(intention: Dictionary, cards: Array, maps: Array, energy: int) -> Dictionary:
	for action in plans():
		var p := _payload(action)
		if p.get("lock_in", false): continue
		var i: Dictionary = p.get("intention", {})
		if str(i.get("kind", "")) != str(intention.get("kind", "")) or str(i.get("target", "")) != str(intention.get("target", "")): continue
		if intention.get("square") is Dictionary and not (i.get("square") is Dictionary and int(i.square.x) == int(intention.square.x) and int(i.square.y) == int(intention.square.y)): continue
		if _same(p.get("card_ids", []), cards) and _map_key(p.get("map_cards", [])) == _map_key(maps) and int(p.get("energy", 0)) == energy: return action
	return {}

## A plan one change away: a new intention keeping the commitments (none for Hold).
func plan_with(intention: Dictionary) -> Dictionary:
	var current := _payload(current_plan())
	if current.is_empty(): return {}
	if intention.kind == "hold": return _find_plan(intention, [], [], 0)
	var found := _find_plan(intention, current.get("card_ids", []), current.get("map_cards", []), int(current.get("energy", 0)))
	return found if not found.is_empty() else _find_plan(intention, [], [], 0)

func plans_toggling(card: String, as_map: bool) -> Array:
	var current := _payload(current_plan())
	if current.is_empty(): return []
	var out: Array = []
	var had_card: bool = card in current.get("card_ids", []).map(func(v): return str(v))
	var had_map: bool = card in current.get("map_cards", []).map(func(m): return str(m.card_id))
	for action in plans():
		var p := _payload(action)
		if p.get("lock_in", false) or p.get("intention") != current.get("intention") or int(p.get("energy", 0)) != int(current.get("energy", 0)): continue
		var cards: Array = p.get("card_ids", []).map(func(v): return str(v))
		var maps: Array = p.get("map_cards", []).map(func(m): return str(m.card_id))
		if as_map and (card in maps) != had_map and not card in cards: out.append(action)
		if not as_map and (card in cards) != had_card and not card in maps: out.append(action)
	return out

func card_usable(id: String) -> bool:
	return not plans_toggling(id, false).is_empty() or not plans_toggling(id, true).is_empty()

## During movement each hand card shows the Move the authority says it adds,
## and committed cards stay marked.
func decorate_card(card: Control, id: String) -> void:
	if not in_movement(): return
	var plan = tv().get("plan")
	var committed: Array = plan.get("cards", []) if plan is Dictionary else []
	var played: Array = (plan.get("map_cards", []) if plan is Dictionary else []).map(func(m): return str(m.card_id))
	var hand_move: Dictionary = tv().get("hand_move", {}) if tv().get("hand_move") is Dictionary else {}
	if hand_move.has(id):
		var move: Dictionary = hand_move[id]
		var tag := Label.new(); tag.name = "MoveTag"; tag.mouse_filter = Control.MOUSE_FILTER_IGNORE
		tag.text = "+%d Move · discard +%d" % [int(move.map), int(move.discard)] if int(move.get("map", 0)) > 0 else "+%d Move" % int(move.get("discard", 0))
		tag.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; tag.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		tag.add_theme_font_size_override("font_size", 16); tag.add_theme_color_override("font_color", Color("ffe08a"))
		tag.add_theme_constant_override("outline_size", 5); tag.add_theme_color_override("font_outline_color", Color.BLACK)
		var back := StyleBoxFlat.new(); back.bg_color = Color(0, 0, 0, 0.72); back.set_corner_radius_all(5)
		tag.add_theme_stylebox_override("normal", back)
		card.add_child(tag)
		tag.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE); tag.offset_top = 40; tag.offset_bottom = 84; tag.offset_left = 8; tag.offset_right = -8
	if id in committed or id in played:
		var ring := Panel.new(); ring.name = "MoveCommitted"; ring.mouse_filter = Control.MOUSE_FILTER_IGNORE
		var border := StyleBoxFlat.new(); border.bg_color = Color(1, 0.86, 0.4, 0.12); border.border_color = Color("f4d48b"); border.set_border_width_all(4)
		ring.add_theme_stylebox_override("panel", border)
		card.add_child(ring); ring.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
		card.tooltip_text = ("Played for its map Move.\n\n" if id in played else "Discarded for +Move.\n\n") + card.tooltip_text

## A hand card during movement toggles its commitment; a map Move card plays
## for its amount (shift-click discards it for +1). Returns false otherwise.
func on_card(card) -> bool:
	if not in_movement(): return false
	var id := str(card.instance_id)
	var as_map := plans_toggling(id, true)
	var discard := plans_toggling(id, false)
	if not Input.is_key_pressed(KEY_SHIFT) and not as_map.is_empty():
		if as_map.size() == 1: screen._send(JSON.stringify(as_map[0])); return true
		var options: Array = []
		var terrain_rules: Dictionary = tv().get("rules", {}).get("terrain", {})
		for action in as_map:
			var terrain := ""
			for m in _payload(action).get("map_cards", []):
				if str(m.card_id) == id: terrain = str(m.get("terrain", ""))
			options.append({"text": "Make %s cost 1" % str(terrain_rules.get(terrain, {}).get("name", terrain)), "id": "battle.tactics.terrain.%s" % terrain, "call": func(): screen._send(JSON.stringify(action))})
		show_popup("Choose a terrain", options, card.get_global_rect())
		return true
	if not discard.is_empty(): screen._send(JSON.stringify(discard[0]))
	return true

func plan_text() -> String:
	var plan = tv().get("plan")
	if not plan is Dictionary: return ""
	var budget: Dictionary = plan.get("budget", {})
	var parts: Array[String] = []
	for key in ["base", "charge", "cards", "energy", "map", "sprint_bonus"]:
		if int(budget.get(key, 0)) > 0: parts.append("%s %d" % [key.replace("_", " "), int(budget[key])])
	var intention: Dictionary = plan.get("intention", {})
	var what := "Hold"
	match str(intention.get("kind", "")):
		"move_to": what = "Move to (%d, %d)" % [int(intention.square.x), int(intention.square.y)]
		"pursue": what = "Pursue " + display_name(str(intention.target))
		"flee": what = "Flee from " + display_name(str(intention.target))
	var text := "%s · Move %d (%s)" % [what, int(budget.get("total", 0)), ", ".join(parts) if not parts.is_empty() else "none"]
	if not str(plan.get("charge", "")).is_empty(): text += " · Charge"
	if plan.get("sprint", false): text += " · Sprint"
	if budget.get("running", false): text += "\n⚠ RUNNING: more Move than your base speed — only on-the-move defenses this round."
	var preview = plan.get("preview")
	if preview is Dictionary:
		text += "\nPreview (if everyone else holds): ends at (%d, %d)" % [int(preview.final.x), int(preview.final.y)]
		if preview.get("arrive") is Dictionary: text += " at t = %s" % frac_text(preview.arrive)
		if not str(preview.get("stop", "")).is_empty(): text += " · %s" % str(preview.stop).replace("_", " ")
	if plan.get("locked", false): text += "\nLocked in. Waiting for the others."
	return text

## The movement plan takes the ability rail's place; Lock in sits where Roll
## and Skip sit during offense.
func build_movement_panel() -> void:
	var dock: VBoxContainer = screen._ability_dock
	screen._roll_dock.visible = false
	var heading := Label.new(); heading.text = "MOVEMENT · secret plan"; heading.add_theme_font_size_override("font_size", 14); heading.add_theme_color_override("font_color", Color("bbac8e")); dock.add_child(heading)
	var text := preload("res://presentation/battle/tooltip_label.gd").new()
	text.text = plan_text(); text.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; text.custom_minimum_size.x = 380
	text.add_theme_font_size_override("font_size", 17); text.add_theme_color_override("font_color", Color("e2d8c5")); text.mouse_filter = Control.MOUSE_FILTER_PASS
	var rules: Dictionary = tv().get("rules", {})
	text.tooltip_text = "Move: base speed free, +%d per discarded card, +1 per %d energy, a map Move card's amount when played. Click a square to move there, an enemy to pursue or flee, a hand card to commit it (shift-click a map card to discard it for +1)." % [int(rules.get("card_move", 1)), int(rules.get("energy_per_move", 5))]
	dock.add_child(text); screen._inspect(text, "battle.tactics.plan", text.text)
	var plan = tv().get("plan")
	var locked: bool = plan is Dictionary and plan.get("locked", false)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); dock.add_child(row)
	var intention: Dictionary = plan.get("intention", {}) if plan is Dictionary else {}
	var hold := plan_with({"kind": "hold"})
	_rail_button(row, "Hold", "battle.tactics.hold", "Stay put. Commits nothing.", hold, str(intention.get("kind", "")) == "hold" or locked)
	var current := _payload(current_plan())
	var energy := int(current.get("energy", 0))
	var step := int(rules.get("energy_per_move", 5))
	var less := _find_plan(current.get("intention", {}), current.get("card_ids", []), current.get("map_cards", []), energy - step) if energy >= step else {}
	var more := _find_plan(current.get("intention", {}), current.get("card_ids", []), current.get("map_cards", []), energy + step)
	_rail_button(row, "−%d✦" % step, "battle.tactics.energy.less", "Spend less energy on Move.", less, locked)
	_rail_button(row, "+%d✦" % step, "battle.tactics.energy.more", "Spend %d more energy for +1 Move." % step, more, locked)
	var lock := current_plan()
	if not lock.is_empty():
		var button := TOOLTIP_BUTTON.new(); button.text = "Lock In"
		button.disabled = screen._submitting or screen._director.has_beats()
		button.pressed.connect(func(): screen._send(JSON.stringify(lock)))
		screen._action_footer.add_child(button); screen._compact_action_button(button)
		button.tooltip_text = "Lock in your secret movement plan. Movement resolves once everyone has locked in."
		screen._inspect(button, "battle.command.tactics_movement_plan", button.tooltip_text)

func _rail_button(parent: Control, text: String, id: String, tooltip: String, action: Dictionary, off: bool) -> void:
	var button := TOOLTIP_BUTTON.new(); button.text = text; button.tooltip_text = tooltip; button.custom_minimum_size = Vector2(78, 30)
	button.add_theme_font_override("font", CINEMATIC.roll_control_font()); button.add_theme_font_size_override("font_size", 15)
	CINEMATIC.paper_button(button)
	button.disabled = off or action.is_empty() or screen._submitting or screen._director.has_beats()
	button.pressed.connect(func(): screen._send(JSON.stringify(action)))
	parent.add_child(button); screen._inspect(button, id, tooltip)

# --- Popup ------------------------------------------------------------------

func show_popup(title: String, options: Array, near: Rect2) -> void:
	hide_popup()
	_popup = PanelContainer.new(); _popup.name = "TacticsPopup"; _popup.add_theme_stylebox_override("panel", CINEMATIC.panel(CINEMATIC.DARK_SURFACE))
	_popup.z_index = 60
	screen._root.add_child(_popup)
	var box := VBoxContainer.new(); _popup.add_child(box)
	var heading := Label.new(); heading.text = title; box.add_child(heading)
	for option in options:
		var button := TOOLTIP_BUTTON.new(); button.text = str(option.text)
		var call: Callable = option.call
		button.pressed.connect(func(): hide_popup(); call.call())
		box.add_child(button); screen._inspect(button, str(option.id), button.text)
	var cancel := TOOLTIP_BUTTON.new(); cancel.text = "Cancel"; cancel.pressed.connect(hide_popup); box.add_child(cancel)
	screen._inspect(cancel, "battle.tactics.popup.cancel", "Cancel")
	_popup.reset_size()
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var anchor: Rect2 = inverse * near if near.size != Vector2.ZERO else Rect2(Vector2(960, 540), Vector2.ZERO)
	var size := _popup.get_combined_minimum_size()
	var at := Vector2(anchor.end.x + 8, anchor.position.y)
	if at.x + size.x > 1912: at.x = anchor.position.x - size.x - 8
	_popup.position = at.clamp(Vector2(8, 8), Vector2(1912, 1072) - size)

func hide_popup() -> void:
	if is_instance_valid(_popup): _popup.queue_free()
	_popup = null

# --- Playback ---------------------------------------------------------------

func _maybe_start_playback() -> void:
	var movement = tv().get("movement")
	if not movement is Dictionary: return
	var finished: bool = screen._view.is_complete()
	if int(movement.round) != played_round and (screen._view.round_number > int(movement.round) or finished):
		played_round = int(movement.round)
		start_playback(movement)

## Replays the last resolved round on the open grid (the Timeline closes).
func replay() -> void:
	screen._open_utility = ""
	for key in screen._utility_panels: screen._utility_panels[key].visible = false
	var bar = screen._root.get_node_or_null("BattleUtilities") if is_instance_valid(screen._root) else null
	if bar != null:
		for button in bar.get_children(): button.set_pressed_no_signal(false)
	var movement = tv().get("movement")
	if movement is Dictionary: start_playback(movement)

func start_playback(movement: Dictionary) -> void:
	playback = {"movement": movement, "started": Time.get_ticks_msec(), "struck": {}}
	if is_instance_valid(board): board.floaters = []; board.falling = {}

func cycle_speed() -> void:
	speed = {1.0: 2.0, 2.0: 0.5, 0.5: 1.0}.get(speed, 1.0)
	if is_instance_valid(_speed_button): _speed_button.text = "Speed %s×" % ("½" if speed == 0.5 else str(int(speed)))

## Called every frame: one playback shows the round's movement, strikes,
## defense results and falls together (plan §9.4).
func process() -> void:
	if is_instance_valid(screen._root): layout_enemy_docks()
	if playback.is_empty() or not is_instance_valid(board): return
	var movement: Dictionary = playback.movement
	var elapsed := (Time.get_ticks_msec() - int(playback.started)) / 1000.0 * speed
	var t := clampf(elapsed / PLAYBACK_SECONDS, 0.0, 1.0)
	for id in movement.get("paths", {}):
		if board.creatures.has(id): board.creatures[id]["square"] = _position_at(movement.paths[id], t)
	board.ghosts = {}
	for hit in movement.get("hits", []): _strike(movement, hit, t)
	if t >= 1.0:
		for hit in movement.get("hits", []):
			if hit.get("lethal", false): board.falling[str(hit.target)] = clampf((elapsed - PLAYBACK_SECONDS) / 0.8, 0, 1)
	board.place_tokens(); board.queue_redraw()
	if elapsed > PLAYBACK_SECONDS + 2.0:
		playback = {}
		board.floaters = []; board.falling = {}
		render_board()

func _position_at(path: Array, t: float) -> Vector2:
	var at := square(path[0].square)
	for i in range(1, path.size()):
		var t0 := frac(path[i - 1].t); var t1 := frac(path[i].t)
		if t >= t1: at = square(path[i].square); continue
		if t > t0: return square(path[i - 1].square).lerp(square(path[i].square), clampf((t - t0) / maxf(0.001, t1 - t0), 0.0, 1.0))
		break
	return at

## A hit lands at its moment: direct at t = 0, a charge at contact, an area
## attack when movement ends. Its defense result shows with it.
func _strike(movement: Dictionary, hit: Dictionary, t: float) -> void:
	var key := str(hit.source_id)
	if playback.struck.has(key): return
	var at := 0.0
	for attack in movement.get("attacks", []):
		if str(attack.attacker) != str(hit.attacker) or str(attack.ability_id) != str(hit.ability_id): continue
		if str(attack.delivery) == "area": at = 1.0
		elif str(attack.delivery) == "charge" and attack.get("contact_at") is Dictionary: at = frac(attack.contact_at)
	if t < at: return
	playback.struck[key] = true
	var where: Vector2 = board.creatures.get(str(hit.target), {}).get("square", Vector2.ZERO)
	var text := "−%d" % int(hit.final)
	if int(hit.base) > int(hit.final): text += " (%s %d)" % [ability_name(str(hit.get("defense", ""))) if not str(hit.get("defense", "")).is_empty() else "saved", int(hit.base) - int(hit.final)]
	board.floaters.append({"position": where, "text": "%s %s" % [ability_name(str(hit.ability_id)), text], "color": Color("ffb36b"), "born_ms": Time.get_ticks_msec()})

# --- Utilities --------------------------------------------------------------

## The Timeline utility: playback speed, Replay round and the debug timeline
## of the last resolved movement (plan §9.5).
func build_timeline_panel(content: VBoxContainer) -> void:
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); content.add_child(row)
	_speed_button = TOOLTIP_BUTTON.new(); _speed_button.text = "Speed %s×" % ("½" if speed == 0.5 else str(int(speed))); _speed_button.pressed.connect(cycle_speed); row.add_child(_speed_button)
	_speed_button.set_meta("battle_utility", true); screen._inspect(_speed_button, "battle.tactics.speed", "Playback speed")
	var again := TOOLTIP_BUTTON.new(); again.text = "Replay round"; again.tooltip_text = "Play the last resolved round again: movement, strikes, defenses and falls."
	again.pressed.connect(replay); again.set_meta("battle_utility", true); row.add_child(again); screen._inspect(again, "battle.tactics.replay", again.tooltip_text)
	_timeline_text = RichTextLabel.new(); _timeline_text.fit_content = true; _timeline_text.selection_enabled = true; _timeline_text.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_timeline_text.text = timeline_text(); content.add_child(_timeline_text)
	screen._inspect(_timeline_text, "battle.tactics.timeline", "Movement timeline")

func timeline_text() -> String:
	var movement = tv().get("movement")
	if not movement is Dictionary: return "No movement has resolved yet."
	var lines: Array[String] = ["Round %d · movement timeline" % int(movement.round)]
	for id in movement.get("budgets", {}):
		var b: Dictionary = movement.budgets[id]
		lines.append("%s: Move %d (paid %d)%s" % [label(id), int(b.total), int(b.paid), " · running" if b.get("running", false) else ""])
	for entry in movement.get("timeline", []):
		var t := frac(entry.get("t", {}))
		var at := "%s (%d%%)" % [frac_text(entry.get("t", {})), roundi(t * 100.0)]
		var who := label(str(entry.get("creature", "")))
		match str(entry.get("kind", "")):
			"step": lines.append("%s %s steps (%d,%d)→(%d,%d) cost %d" % [at, who, int(entry.from.x), int(entry.from.y), int(entry.to.x), int(entry.to.y), int(entry.get("cost", 0))])
			"wait": lines.append("%s %s waits at (%d,%d): %s" % [at, who, int(entry.square.x), int(entry.square.y), str(entry.get("reason", "")).replace("_", " ")])
			"tiebreak": lines.append("%s tiebreak at (%d,%d): %s wins over %s by %s" % [at, int(entry.square.x), int(entry.square.y), label(str(entry.winner)), ", ".join(entry.get("losers", []).map(func(l): return label(str(l)))), str(entry.get("rule", "")).replace("_", " ")])
			"contact": lines.append("%s %s touches %s (charge contact)" % [at, who, label(str(entry.get("target", "")))])
			"stop": lines.append("%s %s stops at (%d,%d): %s" % [at, who, int(entry.square.x), int(entry.square.y), str(entry.get("reason", "")).replace("_", " ")])
			_: lines.append("%s %s %s" % [at, who, str(entry.get("kind", ""))])
	for attack in movement.get("attacks", []):
		lines.append("%s %s: %s%s" % [label(str(attack.attacker)), ability_name(str(attack.ability_id)), str(attack.get("outcome", "")).replace("_", " "), "" if attack.get("hits", []).is_empty() else " " + ", ".join(attack.hits.map(func(id): return label(str(id))))])
	for hit in movement.get("hits", []):
		lines.append("hit %s → %s: %d of %d" % [label(str(hit.attacker)), label(str(hit.target)), int(hit.final), int(hit.base)])
	return "\n".join(lines)

## A finished sandbox battle: Replay the last round, a new battle or the menu.
func add_completion_buttons(parent: Container) -> void:
	for entry in [["Replay last round", "battle.complete.tactics_replay", replay], ["New sandbox battle", "battle.complete.tactics_new", _to_setup], ["Main menu", "battle.complete.tactics_menu", _to_menu]]:
		var button := TOOLTIP_BUTTON.new(); button.text = str(entry[0]); button.pressed.connect(entry[2]); parent.add_child(button)
		screen._inspect(button, str(entry[1]), button.text)

func add_settings(parent: Container) -> void:
	var leave := TOOLTIP_BUTTON.new(); leave.text = "Leave to the main menu"; leave.tooltip_text = "The battle stays saved; resume it from the Tactics Sandbox tab."
	leave.set_meta("battle_utility", true); leave.pressed.connect(_to_menu); parent.add_child(leave)
	screen._inspect(leave, "battle.tactics.menu", leave.tooltip_text)

# Loaded when used: both screens preload the battle screen, which preloads this.
func _to_setup() -> void:
	screen.get_tree().root.add_child(load("res://app/screens/tactics/tactics_setup.gd").new())
	screen.queue_free()

func _to_menu() -> void:
	screen.get_tree().root.add_child(load("res://app/boot/battle_bootstrap.tscn").instantiate())
	screen.queue_free()

func inspection() -> Dictionary:
	var creatures := {}
	for id in tv().get("creatures", {}):
		var a := actor(id)
		creatures[id] = {"at": "%d,%d" % [int(tv().creatures[id].position.x), int(tv().creatures[id].position.y)], "hp": "%d/%d" % [int(a.get("current_health", 0)), int(a.get("max_health", 0))], "energy": int(a.get("energy_points", 0)), "hand": int(a.get("hand_count", 0)), "statuses": a.get("statuses", []).map(func(st): return "%s×%d" % [st.definition_id, int(st.stacks)])}
	return {"arena": str(tv().get("arena", {}).get("id", "")), "step": "movement" if in_movement() else screen._simultaneous_step(), "creatures": creatures, "plan": plan_text(),
		"aim": aim, "aim_squares": aim_actions().map(func(a): return [int(_payload(a).target_square.x), int(_payload(a).target_square.y)]),
		"offensive_targets": screen._offensive_target_actions().keys(), "playing_back": not playback.is_empty(), "pan": [pan.x, pan.y], "zoom": zoom, "view": tv()}
