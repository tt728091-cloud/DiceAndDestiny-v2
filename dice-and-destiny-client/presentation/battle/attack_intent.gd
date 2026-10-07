extends "res://presentation/battle/defense_result.gd"
## One source identity, two presentations: an intent on the attacker and a
## physical defense roll on the battlefield. Existing effect clocks are shared.
const INTENT_BUTTON := preload("res://presentation/battle/attack_intent_button.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const INK := preload("res://presentation/battle/cinematic_theme.gd")
var screen: Control
var source: Dictionary
var intent: Button
var intent_row: HBoxContainer
var roll_area: Control
var roll_cells: Array[Control] = []
var slot := 0
var actor_slot := 0
var attacker_id := ""
var fighter_target: Button
var _tooltip_damage := ""
var _card_target_hint := ""
var target_heading: Button
var incoming_row: Button

func attach_to_battlefield(owner_screen: Control, attack_source: Dictionary) -> void:
	screen = owner_screen; source = attack_source; attacker_id = str(source.get("source_actor_id", ""))
	slot = screen._attack_intents.size()
	for other in screen._attack_intents.values():
		if other.attacker_id == attacker_id: actor_slot += 1
	screen._attack_intents[str(data.source_id)] = self
	if not bool(data.get("reveal_cards", false)): reparent(screen._root, false)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_theme_stylebox_override("panel", StyleBoxEmpty.new())
	for child in _body.get_children(): child.hide()
	intent = INTENT_BUTTON.new(); intent.name = "AttackIntent_" + str(data.source_id)
	# Enemy profiles are attached later in the board rebuild. Never expose the
	# default origin while waiting for that anchor, even for a single frame.
	intent.hide()
	screen._root.add_child(intent); intent.z_index = 8
	intent.custom_minimum_size = Vector2(70, 44)
	intent.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	var can_protect: bool = not screen._source_protection_action(str(data.source_id)).is_empty() and not screen._submitting and not screen._history_review and not screen._model_thinking
	intent.disabled = (bool(data.read_only) and not can_protect) or str(data.actor_id) != screen.viewer_actor_id
	intent.tooltip_text = _ability_tooltip()
	intent.set_meta("inspection_id", "battle.source." + str(data.source_id))
	intent.pressed.connect(func():
		if not screen._selected_card.get("source_targeting", false): source_selected.emit(str(data.source_id))
	)
	for state in ["normal", "hover", "pressed", "disabled", "focus"]:
		var style := INK.bone_panel(Color("ffe1a6") if state in ["hover", "pressed", "focus"] or screen._selected_source == str(data.source_id) else Color.WHITE, 8)
		intent.add_theme_stylebox_override(state, style)
	intent_row = HBoxContainer.new(); intent_row.add_theme_constant_override("separation", 5)
	intent.add_child(intent_row); intent_row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	if not source.get("preview", false) or int(data.before) > 0:
		_add_icon("attack", "%s: %s damage to %s" % [data.attack_name, data.before, data.actor_name])
	damage.reparent(intent_row, false); damage.show(); damage.custom_minimum_size = Vector2(28, 36)
	damage.autowrap_mode = TextServer.AUTOWRAP_OFF; INK.hud_lettering(damage, true)
	damage.mouse_filter = Control.MOUSE_FILTER_IGNORE
	if source.get("preview", false) and int(data.before) == 0: damage.hide()
	var raw_applications = source.get("status_applications")
	if not raw_applications is Array: raw_applications = screen._view.offensive_reveal(attacker_id).get("outcome", {}).get("status_applications", [])
	var applications: Array = screen._as_array(raw_applications)
	var totals := {}
	for application in applications:
		if bool(application.get("applied", false)): continue
		var target := str(application.get("target_actor_id", data.actor_id))
		if not source.get("preview", false) and target != str(data.actor_id): continue
		var id := str(application.get("status_id", ""))
		var key := target + ":" + id
		if id.is_empty(): continue
		if not totals.has(key): totals[key] = {"id": id, "target": target, "count": 0}
		totals[key].count += int(application.get("stacks", 1))
	for value in totals.values():
		_add_icon(str(value.id), "Apply %d %s to %s" % [value.count, BattlePresentationCatalog.status(str(value.id)).name, screen._actor_display_name(str(value.target))])
		_add_count(str(value.count))
	for resource in source.get("resource_gains", {}):
		var amount := int(source.resource_gains[resource])
		if amount <= 0: continue
		_add_icon(str(resource), "Gain %d %s" % [amount, str(resource).capitalize()]); _add_count(str(amount))
	var followups := BattlePresentationCatalog.ability_followup_intents(str(source.get("source_content_id", "")), str(screen._view.actor(attacker_id).get("selected_tier", "")))
	for effect in followups:
		_add_icon(str(effect.icon), str(effect.hint))
		_add_count(str(effect.count), str(effect.hint))
	if intent_row.get_child_count() == 1 and not damage.visible: _add_icon("effect", str(data.attack_statuses))
	if int(data.prevented) > 0: _add_icon("block", "%s: prevents %d damage" % [data.ability_name, data.prevented])
	attack_origin.remove_meta("inspection_id")
	attack_origin = intent
	if not intent.disabled and (screen._view.stage == "defense_selection" or screen._early_defense_available() or can_protect) and not screen._attack_intents.values().any(func(other): return other.attacker_id == attacker_id and is_instance_valid(other.fighter_target)):
		fighter_target = INTENT_BUTTON.new(); fighter_target.name = "SelectAttacker_" + attacker_id
		screen._root.add_child(fighter_target); fighter_target.flat = true
		fighter_target.tooltip_text = intent.tooltip_text
		fighter_target.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		fighter_target.set_meta("inspection_id", "battle.attacker." + attacker_id)
		fighter_target.pressed.connect(func():
			if not intent.disabled: intent.pressed.emit()
		)
		screen._root.move_child(intent, -1)
		for state in ["normal", "hover", "pressed", "focus"]: fighter_target.add_theme_stylebox_override(state, StyleBoxEmpty.new())
	roll_area = Control.new(); roll_area.name = "DefenseRoll_" + str(data.source_id); roll_area.mouse_filter = Control.MOUSE_FILTER_IGNORE
	screen._root.add_child(roll_area); roll_area.z_index = 12
	for i in dice_controls.size():
		var cell: Control = dice_controls[i].get_parent()
		cell.reparent(roll_area, false); cell.show(); roll_cells.append(cell)
		var player_die: bool = str(data.actor_id) == screen.viewer_actor_id
		cell.custom_minimum_size = Vector2(56, 94) if player_die else Vector2(104, 104)
		dice_controls[i].custom_minimum_size = BattleDiceTray.HUD_DIE_SIZE if player_die else Vector2(64, 64)
		if player_die: dice_controls[i].add_theme_font_size_override("font_size", 18)
		dice_controls[i].size_flags_horizontal = Control.SIZE_SHRINK_BEGIN if player_die else Control.SIZE_SHRINK_CENTER
		benefit_labels[i].custom_minimum_size.x = 56 if player_die else 104
		if player_die:
			benefit_labels[i].add_theme_font_size_override("font_size", 12)
			benefit_labels[i].max_lines_visible = 2
			benefit_labels[i].text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
			benefit_labels[i].tooltip_text = benefit_labels[i].text
		benefit_labels[i].autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		INK.hud_lettering(benefit_labels[i])
		# Configure wrapping before sizing: an unwrapped benefit can otherwise
		# impose its full sentence width, which Containers retain after it wraps.
		cell.size = cell.custom_minimum_size
	# Fixed benefits and Curse follow-ups originate at the same landing area.
	effect_origin.reparent(roll_area, false); effect_origin.show()
	effect_origin.text = str(data.ability_name) if not bool(data.get("selection_only", false)) else ""
	effect_origin.size = Vector2(220, 28); INK.hud_lettering(effect_origin)
	for origin in gain_origins:
		if origin in benefit_labels: continue
		if origin == effect_origin: continue
		origin.reparent(roll_area, false); origin.show(); INK.hud_lettering(origin)
	_update()

## The recipient's pending-loss heading replaces the player's duplicate badge.
## Keep this presenter alive for its defense clocks, rules and effect anchors.
func use_target_heading(heading: Button) -> void:
	target_heading = heading
	intent.hide()
	attack_origin = heading
	_refresh_target_heading()

func _refresh_target_heading() -> void:
	if not is_instance_valid(target_heading): return
	target_heading.text = "%s · %s" % [damage.text, data.attack_name]
	target_heading.tooltip_text = _ability_tooltip()

func attack_damage_rect() -> Rect2:
	if is_instance_valid(incoming_row): return incoming_row.damage_rect()
	return target_heading.get_global_rect() if is_instance_valid(target_heading) else super.attack_damage_rect()

func _add_icon(id: String, hint: String) -> void:
	var icon := TextureRect.new(); icon.texture = ICONS.texture(id)
	icon.custom_minimum_size = Vector2(30, 30); icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	icon.set_meta("intent_effect", id)
	icon.tooltip_text = hint; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
	intent_row.add_child(icon)

func _add_count(value: String, hint: String = "") -> void:
	var label := Label.new(); label.text = value if value.begins_with("≤") else "×" + value; label.set_meta("intent_count", value); label.tooltip_text = hint; label.add_theme_font_size_override("font_size", 20); INK.hud_lettering(label, true)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE; intent_row.add_child(label)

func highlight_card_target(description: String) -> Button:
	intent.disabled = false; _card_target_hint = description; intent.tooltip_text = _ability_tooltip()
	intent.set_meta("inspection_id", "battle.card_target." + str(data.source_id))
	intent.add_theme_stylebox_override("normal", INK.bone_panel(Color("ffe49c"), 8))
	# A separate signal connection is used by the screen for the card action.
	return intent

func _ability_tooltip() -> String:
	var id := str(source.get("source_content_id", ""))
	var actor: Dictionary = screen._view.actor(attacker_id)
	var reveal: Dictionary = screen._view.offensive_reveal(attacker_id)
	var description := BattlePresentationCatalog.attack_ability_tooltip(id, actor, reveal, screen._view.rolled_dice(attacker_id))
	description += "\n\n%s → %s" % [screen._actor_display_name(attacker_id), data.actor_name]
	description += "\nCurrent attack: %s damage" % damage.text
	for detail in [data.get("attack_statuses", ""), data.get("note", ""), _card_target_hint]:
		if not str(detail).is_empty(): description += "\n" + str(detail)
	var defense := str(data.get("ability_name", ""))
	if not defense.is_empty(): description += "\nDefense: " + defense
	for bonus in screen._as_array(reveal.get("damage_bonuses")):
		if str(bonus.get("ability_id", reveal.get("ability_id", ""))) != id: continue
		description += "\n\n+%d damage to this attack from playing %s." % [int(bonus.get("amount", 0)), BattlePresentationCatalog.card(str(bonus.get("card_definition_id", ""))).name]
	return description

func _update() -> void:
	super._update()
	if not is_instance_valid(screen) or not is_instance_valid(intent): return
	if _tooltip_damage != damage.text:
		_tooltip_damage = damage.text
		intent.tooltip_text = _ability_tooltip()
		if is_instance_valid(fighter_target): fighter_target.tooltip_text = intent.tooltip_text
	_refresh_target_heading()
	if is_instance_valid(incoming_row): incoming_row.refresh()
	var profile: ActorProfile = screen._actor_profiles.get(attacker_id)
	if profile == null:
		intent.hide()
		return
	var fighter: Control = profile.get_parent().fighter if is_instance_valid(profile.get_parent().fighter) else profile
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * fighter.get_global_rect()
	var minimum := intent_row.get_combined_minimum_size() + Vector2(24, 16)
	intent.size = Vector2(maxf(70, minimum.x), maxf(44, minimum.y))
	intent.position = Vector2(clampf(rect.get_center().x - intent.size.x * 0.5, 16, 1904 - intent.size.x), maxf(screen.TOP_HUD_BOTTOM, rect.position.y - intent.size.y - 12) + actor_slot * 58)
	# Tall silhouettes use the space beside their head, keeping the compact
	# top bar and the enemy artwork clear. Each source retains its own row.
	if attacker_id != screen.viewer_actor_id and intent.position.y + intent.size.y > rect.position.y:
		intent.position.x = maxf(16, rect.position.x - intent.size.x - 8)
	intent_row.position = Vector2(12, 8); intent_row.size = intent.size - Vector2(24, 16)
	if str(data.source_id) == screen._selected_source: screen._layout_defense_choices()
	if is_instance_valid(fighter_target):
		fighter_target.position = rect.position; fighter_target.size = rect.size
	# Only the latest defense for each actor occupies its dice station.
	var selected: String = screen.defense_station_source(str(data.actor_id))
	roll_area.visible = not bool(data.get("selection_only", false)) and selected == str(data.source_id)
	var landing: Vector2 = screen.DEFENSE_DICE_ORIGIN
	if str(data.actor_id) != screen.viewer_actor_id:
		var defender: Control = screen._actor_profiles.get(str(data.actor_id))
		if is_instance_valid(defender):
			var defender_rect: Rect2 = inverse * defender.get_global_rect()
			var dock: Control = screen.dice_dock(str(data.actor_id))
			# Revealed defense rolls are always visible. The optional offensive
			# tray gets its own row without moving the actor's name or health.
			var bottom := (inverse * dock.get_global_rect()).position.y if is_instance_valid(dock) and dock.visible else defender_rect.position.y
			var width := maxf(104, roll_cells.size() * 108 - 4)
			landing = Vector2(defender_rect.get_center().x - width * 0.5, bottom - 112)
	# Outgoing attacks stay beside the player's stats, without a player sprite.
	if attacker_id == screen.viewer_actor_id:
		intent.position = Vector2(478 + actor_slot * 230, 710)
	# Only reveal an anchored enemy badge. Outgoing damage belongs exclusively
	# to the recipient's pending-card heading throughout every phase handoff.
	intent.visible = attacker_id != screen.viewer_actor_id and not is_instance_valid(target_heading)

	# Card reveal containers stay in the open battlefield, never on the intent.
	if not bool(data.get("reveal_cards", false)):
		position = Vector2(570 + (slot % 2) * 460, 500 + (slot / 2) * 145)
		size = Vector2(440, maxf(1, _body.get_combined_minimum_size().y))
	var roll_elapsed := maxf(0, (Time.get_ticks_msec() - int(data.get("roll_started_ms", started_ms))) / 1000.0)
	var t := clampf(roll_elapsed / TIMING.roll_seconds(), 0, 1)
	if bool(data.get("awaiting_roll", false)): t = fmod(t, 0.9)
	for i in roll_cells.size():
		var cell := roll_cells[i]
		dice_controls[i].show()
		cell.custom_minimum_size.y = 94 if str(data.actor_id) == screen.viewer_actor_id else 104
		cell.size = cell.custom_minimum_size
		var finish := landing + Vector2(i * (62 if str(data.actor_id) == screen.viewer_actor_id else 108), 0)
		cell.position = finish
		# The authoritative faces cycle in place; no travel, spin, or bounce.
		dice_controls[i].rotation = 0
		dice_controls[i].scale = Vector2.ONE

	# Leave a gap below a two-line benefit, while keeping the name above the
	# incoming-attack list. The old 94px offset overlapped wrapped captions.
	effect_origin.size = Vector2(220, 22 if str(data.actor_id) == screen.viewer_actor_id else 28)
	effect_origin.position = landing + Vector2(0, 101 if str(data.actor_id) == screen.viewer_actor_id else -32)
	if str(data.actor_id) != screen.viewer_actor_id:
		# Identify the defense while its faces cycle; per-die prevention stays
		# hidden until landing on the shared roll/effects clock.
		effect_origin.position.x += (maxf(104, roll_cells.size() * 108 - 4) - effect_origin.size.x) * 0.5
		effect_origin.modulate.a = 1.0
	else:
		effect_origin.modulate.a = 0.0 if t < 1.0 else 1.0
	for i in gain_origins.size():
		var origin: Label = gain_origins[i]
		if origin not in benefit_labels and origin != effect_origin: origin.position = landing + Vector2(0, (112 if str(data.actor_id) == screen.viewer_actor_id else -60) + i * 25)

	if is_instance_valid(incoming_row): screen._layout_incoming_attack_list()

func _prevention_origin(index: int) -> Control:
	return dice_controls[index] if dice_controls[index].is_visible_in_tree() else benefit_labels[index]

func _draw() -> void:
	if is_instance_valid(roll_area) and not roll_area.visible: return
	super._draw()
	if not is_instance_valid(intent) or not dice_controls.is_empty() or int(data.get("prevented", 0)) <= 0 or data.get("effects_pending", false) or data.get("damage_pending", false): return
	var elapsed := (Time.get_ticks_msec() - started_ms) / 1000.0
	var effects := maxf(0, elapsed - TIMING.roll_seconds()) / TIMING.effects_seconds()
	var t := clampf((effects - 0.2) / 0.45, 0, 1)
	if t <= 0 or t >= 1: return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var start := inverse * effect_origin.get_global_rect().get_center()
	var end := inverse * attack_damage_rect().get_center()
	var points := PackedVector2Array()
	for step in 20:
		var p := lerpf(maxf(0, t - 0.4), t, step / 19.0)
		points.append(start.lerp(end, p) - Vector2(0, sin(p * PI) * 40))
	draw_polyline(points, Color("b4f28f"), 3, true)
	draw_circle(points[-1], 4, Color("deffbf"))
