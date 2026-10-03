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
	screen._root.add_child(intent); intent.z_index = 8
	intent.custom_minimum_size = Vector2(70, 44)
	intent.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	intent.disabled = bool(data.read_only) or str(data.actor_id) != screen.viewer_actor_id
	intent.tooltip_text = _ability_tooltip()
	intent.set_meta("inspection_id", "battle.source." + str(data.source_id))
	intent.pressed.connect(func():
		if not screen._selected_card.get("source_targeting", false): source_selected.emit(str(data.source_id))
	)
	for state in ["normal", "hover", "pressed", "disabled", "focus"]:
		var style := INK.panel(Color("17131d55") if state == "normal" else Color("67562b99") if state in ["hover", "pressed"] else Color.TRANSPARENT, Color("efcd80") if state == "focus" or screen._selected_source == str(data.source_id) else Color.TRANSPARENT, 4)
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
	if not intent.disabled and (screen._view.stage == "defense_selection" or screen._early_defense_available()) and not screen._attack_intents.values().any(func(other): return other.attacker_id == attacker_id and is_instance_valid(other.fighter_target)):
		fighter_target = INTENT_BUTTON.new(); fighter_target.name = "SelectAttacker_" + attacker_id
		screen._root.add_child(fighter_target); fighter_target.flat = true
		fighter_target.tooltip_text = intent.tooltip_text
		fighter_target.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		fighter_target.set_meta("inspection_id", "battle.attacker." + attacker_id)
		fighter_target.pressed.connect(func(): source_selected.emit(str(data.source_id)))
		for state in ["normal", "hover", "pressed", "focus"]: fighter_target.add_theme_stylebox_override(state, StyleBoxEmpty.new())
	roll_area = Control.new(); roll_area.name = "DefenseRoll_" + str(data.source_id); roll_area.mouse_filter = Control.MOUSE_FILTER_IGNORE
	screen._root.add_child(roll_area); roll_area.z_index = 12
	for i in dice_controls.size():
		var cell: Control = dice_controls[i].get_parent()
		cell.reparent(roll_area, false); cell.show(); roll_cells.append(cell)
		cell.custom_minimum_size = Vector2(104, 104); cell.size = Vector2(104, 104)
		dice_controls[i].custom_minimum_size = Vector2(64, 64)
		dice_controls[i].size_flags_horizontal = Control.SIZE_SHRINK_CENTER
		benefit_labels[i].custom_minimum_size.x = 104
		benefit_labels[i].autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		INK.hud_lettering(benefit_labels[i])
	# Fixed benefits and Curse follow-ups originate at the same landing area.
	effect_origin.reparent(roll_area, false); effect_origin.show()
	effect_origin.text = str(data.ability_name) if not bool(data.get("selection_only", false)) else ""
	effect_origin.size = Vector2(220, 28); INK.hud_lettering(effect_origin)
	for origin in gain_origins:
		if origin in benefit_labels: continue
		if origin == effect_origin: continue
		origin.reparent(roll_area, false); origin.show(); INK.hud_lettering(origin)
	_update()

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
	intent.add_theme_stylebox_override("normal", INK.panel(Color("9a713366"), Color("ffe49c"), 4))
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
	var profile: ActorProfile = screen._actor_profiles.get(attacker_id)
	if profile == null or not is_instance_valid(profile.get_parent().fighter): return
	var fighter: Control = profile.get_parent().fighter
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var rect: Rect2 = inverse * fighter.get_global_rect()
	var minimum := intent_row.get_combined_minimum_size() + Vector2(12, 8)
	intent.size = Vector2(maxf(70, minimum.x), maxf(44, minimum.y))
	intent.position = Vector2(clampf(rect.get_center().x - intent.size.x * 0.5, 16, 1904 - intent.size.x), maxf(16, rect.position.y - intent.size.y - 12) + actor_slot * 48)
	intent_row.position = Vector2(6, 4); intent_row.size = intent.size - Vector2(12, 8)
	if str(data.source_id) == screen._selected_source: screen._layout_defense_choices()
	if is_instance_valid(fighter_target):
		fighter_target.position = rect.position; fighter_target.size = rect.size
	var landing := Vector2(570 + (slot % 4) * 225, (800 if screen._enemy_ids().size() >= 3 else 570) + (slot / 4) * 125)
	# Card reveal containers stay in the open battlefield, never on the intent.
	if not bool(data.get("reveal_cards", false)):
		position = Vector2(570 + (slot % 2) * 460, 500 + (slot / 2) * 145)
		size = Vector2(440, maxf(1, _body.get_combined_minimum_size().y))
	var roll_elapsed := maxf(0, (Time.get_ticks_msec() - int(data.get("roll_started_ms", started_ms))) / 1000.0)
	var t := clampf(roll_elapsed / TIMING.roll_seconds(), 0, 1)
	if bool(data.get("awaiting_roll", false)): t = fmod(t, 0.9)
	for i in roll_cells.size():
		var cell := roll_cells[i]
		var finish := landing + Vector2(i * 108, 0)
		var launch := Vector2(450 if str(data.actor_id) == screen.viewer_actor_id else 1420, 410 + (slot % 3) * 28)
		cell.position = launch.lerp(finish, 1.0 - pow(1.0 - t, 2)) - Vector2(0, absf(sin(t * PI * 3)) * 65 * (1.0 - t))
		dice_controls[i].rotation = (1.0 - t) * TAU * (1 if i % 2 == 0 else -1)
		dice_controls[i].scale = Vector2.ONE * (1.0 + sin(t * PI) * 0.15)
	effect_origin.position = landing + Vector2(0, -32)
	effect_origin.modulate.a = 0.0 if t < 1.0 else 1.0
	for i in gain_origins.size():
		var origin: Label = gain_origins[i]
		if origin not in benefit_labels and origin != effect_origin: origin.position = landing + Vector2(0, 80 + i * 25)

func _draw() -> void:
	super._draw()
	if not is_instance_valid(intent) or not dice_controls.is_empty() or int(data.get("prevented", 0)) <= 0 or data.get("effects_pending", false): return
	var elapsed := (Time.get_ticks_msec() - started_ms) / 1000.0
	var effects := maxf(0, elapsed - TIMING.roll_seconds()) / TIMING.effects_seconds()
	var t := clampf((effects - 0.2) / 0.45, 0, 1)
	if t <= 0 or t >= 1: return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var start := inverse * effect_origin.get_global_rect().get_center()
	var end := inverse * damage.get_global_rect().get_center()
	var points := PackedVector2Array()
	for step in 20:
		var p := lerpf(maxf(0, t - 0.4), t, step / 19.0)
		points.append(start.lerp(end, p) - Vector2(0, sin(p * PI) * 40))
	draw_polyline(points, Color("b4f28f"), 3, true)
	draw_circle(points[-1], 4, Color("deffbf"))
