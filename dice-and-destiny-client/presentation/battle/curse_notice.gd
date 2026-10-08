extends Control

# Public card/die events retain the played hand pose and send a traveling effect
# to the target. Extra Curse checks have a separate preview; forced offensive
# rerolls animate the actual tray while preserving its authoritative data.
const TIMING := preload("res://presentation/battle/combat_timing.gd")
const THEME := preload("res://presentation/battle/cinematic_theme.gd")
var screen: Control
var feedback: Dictionary
var _label: Label
var _card: BattleCard
var _card_pose: Dictionary = {}
var _held_hand: Control
var _roll: Label
var _ability_label: Label
var _defense_start := Vector2.ZERO
var _defense_origins: Dictionary = {}
var _attack_masks: Array = []
var _pending_tray: BattleDiceTray
var _reroll_tray: BattleDiceTray
var _pending_index := -1
var _pending_face := 0
var _elapsed := 0.0
var _step := 0
var _battle_id := ""
var _knell_origin := Rect2()
var _refusal_expiry_profile: ActorProfile
var _status_symbol: TextureRect
var _count_target := Rect2()
var _target := Rect2()
var _die_rect := Rect2()
var _origin := Vector2.ZERO
var duration := 2.1
var _batch_targets: Array[Rect2] = []
var _batch_dice: Array[Rect2] = []
var _batch_starts: Array[Vector2] = []
var _batch_rolls: Array[Label] = []

func configure(owner_screen: Control, data: Dictionary, poses: Dictionary = {}) -> void:
	screen = owner_screen; feedback = data
	_card_pose = poses.get(str(data.get("card_instance_id", "")), {}).duplicate(true)
	_defense_origins = screen._curse_defense_origins.duplicate()
	_battle_id = screen._view.battle_id
	name = "CurseFeedback"; mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 31
	set_meta("feedback_notice", true)
	duration = maxf(1.0, TIMING.seconds("curse_feedback_seconds", 2.1))
	if data.changes.any(func(change): return change.get("kind") == "offensive_reroll"): duration = 2.8
	if BattlePresentationCatalog.card_mechanic(str(data.card_id)) == "unquiet_hands": duration = 2.6
	if BattlePresentationCatalog.card_mechanic(str(data.card_id)) == "curse_bloom": duration = 2.8
	if data.changes[0].get("kind") == "second_knell_trigger": duration = 3.6 + 0.55 * maxi(0, data.changes[0].get("retry_dice", []).size() - 1)
	if data.changes[0].get("kind") in ["second_knell_expired", "refusal_expired"]: duration = 1.5
	if data.changes[0].get("kind") == "refusal_trigger": duration = 2.8
	if data.changes[0].get("kind") == "refusal_expired": duration = 2.2
	if data.changes.any(func(change): return change.get("kind") == "offensive_face_set"): duration = 1.15
	if _face_batch_end() > 1: duration = maxf(duration, 2.8)
	if _inline_effect(): duration += _launch_end()
	# A played card can only be shown at its captured hand pose. Missing poses
	# (hidden enemy hands, replays, delayed status triggers) never spawn a copy
	# at a fixed screen coordinate.
	if not str(data.card_id).is_empty() and not _card_pose.is_empty():
		_card = BattleCard.new()
		_card.configure(str(data.get("card_instance_id", "")), str(data.card_id), true, false, false)
		_card.mouse_filter = Control.MOUSE_FILTER_IGNORE; add_child(_card)
		_card.draw.connect(_card.draw_targeting_outline.bind(_card))
	_label = Label.new(); _label.name = "CurseOutcome"
	_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.custom_minimum_size = Vector2(260, 0); _label.size.x = 260
	_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_label.add_theme_font_size_override("font_size", 18)
	_label.add_theme_color_override("font_color", Color("efdbff"))
	_label.add_theme_stylebox_override("normal", THEME.panel(Color("201526f5"), Color("c38ce5"), 6))
	add_child(_label)
	_roll = Label.new(); _roll.name = "CurseRollPreview"; _roll.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_roll.size = Vector2(70, 76); _roll.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_roll.vertical_alignment = VERTICAL_ALIGNMENT_CENTER; _roll.add_theme_font_size_override("font_size", 24)
	_roll.add_theme_stylebox_override("normal", THEME.panel(Color("242421"), Color("b6ad92"), 5))
	add_child(_roll); modulate.a = 0
	_ability_label = Label.new(); _ability_label.name = "CurseDefenseOrigin"
	_ability_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_ability_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_ability_label.add_theme_font_size_override("font_size", 18)
	_ability_label.add_theme_color_override("font_color", Color("efdbff"))
	_ability_label.add_theme_stylebox_override("normal", THEME.panel(Color("201526f5"), Color("c38ce5"), 6))
	add_child(_ability_label); _ability_label.hide()

func _inline_defense() -> bool:
	return feedback.get("defense_inline", false)

func _inline_effect() -> bool:
	return _inline_defense() or feedback.get("attack_inline", false)

func _launch_delay() -> float:
	return 0.0 if feedback.get("attack_inline", false) else 0.48

func _launch_end() -> float:
	return _launch_delay() + 0.52

func _curse_progress() -> float:
	return clampf((_elapsed - _launch_end()) / (duration - _launch_end()), 0, 1) if _inline_effect() else _elapsed / duration

func _waiting() -> bool:
	if feedback.get("damage_inline", false):
		return not screen._damage_followups_ready.get(str(feedback.beat_key), false) or not screen._director.peek().get("curse_followups", []).any(func(item): return item.feedback_key == feedback.beat_key)
	if _inline_defense() and screen._held_defense_view != null:
		return Time.get_ticks_msec() < screen._defense_outcome_start + ceili(preload("res://presentation/battle/defense_timing.gd").roll_seconds() * 1000.0)
	if feedback.has("beat_key"):
		return screen._director.peek().get("feedback_key", "") != feedback.beat_key
	for sibling in screen.get_children():
		if sibling == self: break
		if sibling.get_meta("feedback_notice", false) and not sibling.is_queued_for_deletion(): return true
	return screen._director.has_beats()

func _process(delta: float) -> void:
	if not is_instance_valid(screen) or screen._view.battle_id != _battle_id: queue_free(); return
	if _waiting() or screen._history_review or screen._history_replay or screen._snapshot_panel_open:
		modulate.a = 0; return
	_elapsed += delta
	if _elapsed >= duration:
		_release_pending_face()
		_release_forced_roll()
		_elapsed -= duration; _step = _face_batch_end()
		if _step >= feedback.changes.size():
			queue_free()
			if feedback.has("beat_key") and screen._director.peek().get("feedback_key", "") == feedback.beat_key: screen.call_deferred("_advance_beat")
			return
	refresh()

func refresh() -> void:
	if not is_instance_valid(screen._root) or _step >= feedback.changes.size(): return
	var board_transform: Transform2D = screen.get_global_transform_with_canvas().affine_inverse() * screen._root.get_global_transform_with_canvas()
	position = board_transform.origin; scale = board_transform.get_scale()
	var inverse := get_global_transform_with_canvas().affine_inverse()
	if is_instance_valid(_status_symbol): _status_symbol.hide()
	_batch_targets.clear(); _batch_dice.clear(); _batch_starts.clear()
	for preview in _batch_rolls: preview.hide()
	_sync_attack_marks()
	_sync_counts()
	_count_target = Rect2()
	var change: Dictionary = feedback.changes[_step]
	var player: bool = str(change.actor_id) == screen.viewer_actor_id
	# Keep the notice in the open lane beside the recipient's profile/tray.
	_origin = Vector2(448, 25) if player else Vector2(1240, 25)
	if is_instance_valid(_card):
		if is_instance_valid(screen._hand_dock):
			_held_hand = screen._hand_dock
			_held_hand.gain_animation_active = true
		var pose: Transform2D = _card_pose.transform
		_card.size = _card_pose.size
		_card.position = pose.origin
		_card.rotation = pose.get_rotation()
		_card.scale = pose.get_scale()
	_label.position = _origin + Vector2(0, 200 if is_instance_valid(_card) else 125)
	var heading := str(feedback.get("title", "Curse"))
	_ability_label.hide()
	if str(change.get("kind", "")).begins_with("bloom_"):
		_refresh_bloom(change, inverse)
		_finish_refresh()
		return
	if str(change.get("kind", "")).begins_with("dividend_"):
		_refresh_dividend(change, inverse)
		_finish_refresh()
		return
	if str(change.get("kind", "")).begins_with("refusal_"):
		_refresh_refusal(change, inverse)
		_finish_refresh()
		return
	if change.get("kind") in ["second_knell_trigger", "second_knell_expired"]:
		_refresh_knell(change, inverse)
		_finish_refresh()
		return
	if change.get("kind") == "offensive_face_set":
		_refresh_face_set(change, inverse)
		_finish_refresh()
		return
	if change.get("kind") == "offensive_reroll":
		_refresh_offensive_reroll(change, inverse)
		_finish_refresh()
		return
	if BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "unquiet_hands":
		_refresh_extra_check(change, inverse)
		_finish_refresh()
		return
	if change.get("kind") == "preparation":
		_roll.hide(); _die_rect = Rect2(); _target = Rect2()
		_label.text = "%s\n%s · Guard prepared\nNext Count cleanse: roll 1 die.\nCurse cancels the cleanse.\nExpires next Income." % [heading, screen._actor_display_name(str(change.actor_id))]
		var prepared_label: Label = screen._curse_preparation_labels.get(str(change.actor_id) + ":" + str(change.card_id))
		if is_instance_valid(prepared_label): _target = inverse * prepared_label.get_global_rect()
		_finish_refresh()
		return
	var defense_id := str(change.get("source_ability_id", ""))
	_ability_label.visible = not defense_id.is_empty()
	if _ability_label.visible:
		heading = str(BattlePresentationCatalog.ability(defense_id).name)
		_ability_label.text = heading + ("\n⌁ Curse applications" if change.has("attack_source_id") else "\n⌁ Curse to attacker")
		_ability_label.size = _ability_label.get_minimum_size()
		var key := _battle_id + ":" + str(change.get("defense_source_id", ""))
		_defense_start = _defense_origins.get(key, Vector2(710, 475) if str(change.get("source_actor_id", "")) == screen.viewer_actor_id else Vector2(1210, 475))
		if change.has("attack_source_id"):
			var origin: Control = screen._curse_attack_origins.get(str(change.attack_source_id))
			if is_instance_valid(origin): _defense_start = inverse * origin.get_global_rect().get_center()
		_ability_label.position = _defense_start - _ability_label.size * 0.5
	if feedback.get("attack_inline", false):
		var origin: Control = screen._curse_attack_origins.get(str(feedback.get("attack_source_id", "")))
		if is_instance_valid(origin): _defense_start = inverse * origin.get_global_rect().get_center()
	if _inline_defense():
		for panel in screen._defense_result_panels:
			if str(panel.data.get("source_id", "")) != str(feedback.get("defense_source_id", "")): continue
			var origin: Control = panel._prevention_origin(0) if not panel.dice_controls.is_empty() else panel.effect_origin
			if is_instance_valid(origin): _defense_start = inverse * origin.get_global_rect().get_center()
	var number := int(change.index) + 1
	var die_name := "D%d" % number if number > 0 else str(change.die_id)
	var rolling := bool(change.rolled) and _curse_progress() < 0.3
	var outcome := "Rolling %s…" % die_name if rolling else "Face %d · %s" % [int(change.face), "Curse applied" if change.marked else "already cursed · +1 Count" if change.get("cursed", false) else "clean · no Count"]
	var reason := ("Rolled to expand Curse" if change.marked else "Rolled to resolve Curse") if change.rolled else "Applied directly · no roll"
	if change.get("ordinary_curse", false):
		if change.rolled: reason = "All five dice are cursed.\nRoll to expand; mark the result."
		else: reason = "Random clean die → face 1.\nApplied directly · no roll"
	elif BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) in ["mark_the_number", "maledictions_refusal"] and not change.rolled: reason += "\nRandomly selected clean die"
	if change.get("released", false) and not rolling: reason += "\nEntombment released"
	_label.text = "%s\n%s · %s\n%s\n%s" % [heading, screen._actor_display_name(str(change.actor_id)), die_name, outcome, reason]
	if change.has("count_after") and not rolling:
		_label.text += "\nCurse Count %d → %d" % [int(change.count_before), int(change.count_after)]
		var profile: ActorProfile = screen._actor_profiles.get(str(change.actor_id))
		if is_instance_valid(profile): _count_target = inverse * profile.anchor_rect("status", "curse_count")
	if change.has("application_index"): _label.text += "\nCurse %d of %d" % [int(change.application_index), int(change.application_count)]
	elif feedback.changes.size() > 1: _label.text += "\n%d / %d" % [_step + 1, feedback.changes.size()]
	_roll.visible = bool(change.rolled)
	_roll.position = _origin + (Vector2(178, 85) if is_instance_valid(_card) else Vector2(95, 35))
	if change.has("attack_source_id") and not is_instance_valid(_card):
		_label.position = _origin
		_roll.position = _origin + Vector2(270 if player else -85, 35)
	var face := (int(_elapsed * 22) % 6 + 1) if rolling else int(change.face)
	var marked: bool = not rolling and (change.get("cursed", false) or (change.marked and _curse_progress() >= 0.52))
	_roll.text = "%s%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(change.get("definition_id", "standard_d6")), face), " ⌁" if marked else "", face]
	_stone(_roll, str(change.get("definition_id", "standard_d6")), face, change, marked)
	_target = Rect2(); _die_rect = Rect2()
	# Resolve the actual actor, including visible off-focus enemy trays.
	if screen.dice_dock(str(change.actor_id)) != null:
		var dock: Control = screen.dice_dock(str(change.actor_id))
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() > 0 and dock.get_child(0) is BattleDiceTray:
			var tray: BattleDiceTray = dock.get_child(0)
			var index := int(change.index)
			if index >= 0 and index < tray._buttons.size():
				if not defense_id.is_empty() and change.marked and not change.has("attack_source_id"):
					if is_instance_valid(_pending_tray) and _pending_tray != tray: _release_pending_face()
					_pending_tray = tray; _pending_index = index; _pending_face = int(change.face)
					tray.set_curse_face_pending(index, int(change.face), _curse_progress() < 0.52 and not screen._history_review and not screen._snapshot_panel_open)
				_die_rect = inverse * tray._buttons[index].get_global_rect()
				if int(change.face) >= 1 and int(change.face) <= 6 and tray._mark_faces[index][int(change.face) - 1].is_visible_in_tree():
					_target = inverse * tray._mark_faces[index][int(change.face) - 1].get_global_rect()
	if _uses_tray_curse(): _refresh_face_batch(inverse, heading)
	_finish_refresh()

func _finish_refresh() -> void:
	if _inline_effect() or is_instance_valid(_card):
		_label.hide(); _ability_label.hide()
	modulate.a = (1.0 if is_instance_valid(_card) else clampf(_elapsed / 0.12, 0, 1)) * clampf((duration - _elapsed) / 0.3, 0, 1)
	if _waiting() or screen._history_review or screen._history_replay or screen._snapshot_panel_open: modulate.a = 0
	queue_redraw()

func _draw() -> void:
	if _step >= feedback.changes.size() or is_queued_for_deletion(): return
	if feedback.changes[_step].get("kind") == "refusal_expired":
		_draw_refusal_expiry(); return
	var progress := _curse_progress()
	if str(feedback.changes[_step].get("kind", "")).begins_with("bloom_"):
		if _target.size != Vector2.ZERO: draw_rect(_target.grow(3), Color(0.88, 0.69, 1.0, sin(progress * PI)), false, 2, true)
		return
	if str(feedback.changes[_step].get("kind", "")).begins_with("dividend_"):
		_draw_dividend(progress)
		return
	if _step < feedback.changes.size() and feedback.changes[_step].get("kind") == "offensive_face_set":
		if _target.size != Vector2.ZERO:
			draw_rect(_target.grow(3), Color("e2b2ff"), false, 2, true)
		return
	var color := Color("e2b2ff")
	if _uses_tray_curse():
		_draw_face_batch(progress, color)
		return
	if feedback.changes[_step].get("kind") in ["second_knell_trigger", "second_knell_expired"]:
		_draw_knell(progress, color)
		return
	if BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "unquiet_hands":
		_draw_extra_check(progress, color)
		return
	if _count_target.size != Vector2.ZERO and progress >= 0.52:
		var start := _die_rect.get_center()
		var end := _count_target.get_center()
		var tip := start.lerp(end, clampf((progress - 0.52) / 0.18, 0, 1))
		draw_line(start, tip, Color(color, 0.2), 8, true)
		draw_line(start, tip, color, 2, true)
		draw_rect(_count_target.grow(3), Color(color, 0.7), false, 2, true)
	# Identify the physical die immediately, then land on its numbered face chip.
	if _die_rect.size != Vector2.ZERO: draw_rect(_die_rect.grow(3), Color(color, 0.6), false, 2, true)
	if _target.size == Vector2.ZERO or progress < 0.3: return
	var start := _defense_start if _ability_label.visible else _card.get_transform() * (_card.size * 0.5) if is_instance_valid(_card) else _roll.position + _roll.size * 0.5
	var destination := _target.get_center()
	var travel := clampf((progress - 0.3) / 0.22, 0, 1)
	var tip := start.lerp(destination, travel)
	if progress < 0.65:
		if is_instance_valid(_card):
			var points := _card_arc(destination, travel)
			draw_polyline(points, Color(color, 0.16), 8, true)
			draw_polyline(points, color, 2, true); draw_circle(points[-1], 5, color)
		else:
			draw_line(start, tip, Color(color, 0.16), 8, true)
			draw_line(start, tip, color, 2, true); draw_circle(tip, 5, color)
	if travel >= 1:
		var pulse := (sin((progress - 0.52) * 24) + 1) * 0.5
		draw_rect(_target.grow(3 + pulse * 3), Color(color, 0.7), false, 2, true)
		draw_circle(destination, 22 + pulse * 8, Color(color, 0.12))

func _card_arc(destination: Vector2, travel: float) -> PackedVector2Array:
	var start := _card.get_transform() * (_card.size * 0.5)
	var points := PackedVector2Array()
	for step in 25:
		var t := travel * step / 24.0
		points.append(start.lerp(destination, t) + Vector2(0, -95 * sin(PI * t)))
	return points

func _release_pending_face() -> void:
	if is_instance_valid(_pending_tray): _pending_tray.set_curse_face_pending(_pending_index, _pending_face, false)
	_pending_tray = null

func _exit_tree() -> void:
	if is_instance_valid(_held_hand): _held_hand.gain_animation_active = false
	if is_instance_valid(_refusal_expiry_profile):
		var counts: Dictionary = _refusal_expiry_profile.statuses.counts.duplicate()
		counts.erase(_feedback_status("maledictions_refusal"))
		_refusal_expiry_profile.statuses.set_counts(counts)
	_release_pending_face()
	_release_forced_roll()
	for mask in _attack_masks:
		if is_instance_valid(mask.tray): mask.tray.set_curse_face_pending(mask.index, mask.face, false)

func _release_forced_roll() -> void:
	if is_instance_valid(_reroll_tray) and _step < feedback.changes.size():
		var change: Dictionary = feedback.changes[_step]
		if change.get("kind") == "offensive_reroll": _reroll_tray.present_forced_roll(int(change.index), int(change.face_before), 1.0, 0.0)
		elif change.get("kind") == "offensive_face_set": _reroll_tray.present_face_change(int(change.index), int(change.face_before), 1.0)
	_reroll_tray = null

func _refresh_offensive_reroll(change: Dictionary, inverse: Transform2D) -> void:
	_roll.hide(); _target = Rect2(); _die_rect = Rect2()
	var actor := str(change.actor_id)
	var index := int(change.index)
	var roll_start := duration * 0.52
	var roll_seconds := duration * 0.28
	var elapsed := _elapsed - roll_start
	var inspecting: bool = screen._history_review or screen._history_replay or screen._snapshot_panel_open
	if screen.dice_dock(actor) != null:
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() > 0 and dock.get_child(0) is BattleDiceTray:
			_reroll_tray = dock.get_child(0)
			_reroll_tray.present_forced_roll(index, int(change.face_before), roll_seconds if inspecting else elapsed, roll_seconds)
			if index >= 0 and index < _reroll_tray._buttons.size():
				_die_rect = inverse * _reroll_tray._buttons[index].get_global_rect()
				_target = _die_rect
	var outcome := ("Reroll without spending a roll attempt" if change.get("starter_card", false) else "Forces this kept die to reroll") if elapsed < 0 else "Rolling D%d…" % (index + 1)
	if elapsed >= roll_seconds:
		outcome = "D%d: %d → %d\n%s" % [index + 1, int(change.face_before), int(change.face), "No normal roll attempt consumed" if change.get("starter_card", false) else "+1 Curse Count" if change.get("cursed", false) else "Clean face · no Curse Count"]
		if change.get("released", false): outcome += "\nEntombment released"
		var reveal: Dictionary = screen._view.offensive_reveal(actor)
		var ability := str(reveal.get("ability_id", ""))
		outcome += "\nChoose a qualified ability" if ability.is_empty() else "\n%s: %d damage" % [BattlePresentationCatalog.ability(ability).name, int(reveal.get("outcome", {}).get("base_damage", 0))]
	_label.text = "%s\n%s · D%d\n%s" % [feedback.title, screen._actor_display_name(actor), index + 1, outcome]

func _sync_attack_marks() -> void:
	for i in feedback.changes.size():
		var change: Dictionary = feedback.changes[i]
		if not change.has("attack_source_id") or not change.get("marked", false): continue
		var actor := str(change.actor_id)
		if screen.dice_dock(actor) == null: continue
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() == 0 or not dock.get_child(0) is BattleDiceTray: continue
		var tray: BattleDiceTray = dock.get_child(0)
		var index := int(change.index); var face := int(change.face)
		if not _attack_masks.any(func(mask): return is_instance_valid(mask.tray) and mask.tray == tray and mask.index == index and mask.face == face): _attack_masks.append({"tray": tray, "index": index, "face": face})
		var pending: bool = i >= _face_batch_end() or (i >= _step and _curse_progress() < 0.52)
		tray.set_curse_face_pending(index, face, pending and not screen._history_review and not screen._snapshot_panel_open)

func _refresh_face_set(change: Dictionary, inverse: Transform2D) -> void:
	_roll.hide(); _target = Rect2(); _die_rect = Rect2()
	var actor := str(change.actor_id)
	var index := int(change.index)
	_label.text = "%s · D%d\n%d → %d · Face set" % [BattlePresentationCatalog.card(str(feedback.card_id)).name, index + 1, int(change.face_before), int(change.face)]
	if screen.dice_dock(actor) == null: return
	var dock: Control = screen.dice_dock(actor)
	screen.reveal_dice_for_effect(dock)
	if dock.get_child_count() == 0 or not dock.get_child(0) is BattleDiceTray: return
	_reroll_tray = dock.get_child(0)
	var progress := clampf((_elapsed - 0.12) / 0.58, 0.0, 1.0)
	_reroll_tray.present_face_change(index, int(change.face_before), progress)
	if index < 0 or index >= _reroll_tray._buttons.size(): return
	_target = inverse * _reroll_tray._buttons[index].get_global_rect()
	var tray_bounds: Rect2 = inverse * dock.get_global_rect()
	_label.position = Vector2(tray_bounds.end.x + 18 if actor == screen.viewer_actor_id else tray_bounds.position.x - 278, tray_bounds.position.y)

func _sync_counts() -> void:
	var shown := {}
	for i in feedback.changes.size():
		var change: Dictionary = feedback.changes[i]
		if not change.has("count_after"): continue
		var actor := str(change.actor_id)
		if not shown.has(actor): shown[actor] = int(change.get("count_rolled", change.count_before)) if change.get("already_rolled", false) else int(change.count_before)
		var arrival := 0.75 if BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "unquiet_hands" else 0.8 if change.get("kind") == "offensive_reroll" else 0.52
		if change.get("kind") == "second_knell_trigger":
			if not _waiting():
				if _curse_progress() >= 0.3: shown[actor] = int(change.count_before) + 1
				if _curse_progress() >= 0.86: shown[actor] = int(change.count_after)
		elif not _waiting() and (i < _step or (i >= _step and i < _face_batch_end() and _curse_progress() >= arrival)): shown[actor] = int(change.count_after)
	for actor in shown:
		var earlier := false
		for sibling in screen.get_children():
			if sibling == self: break
			if sibling.get_script() != get_script() or sibling.is_queued_for_deletion(): continue
			if sibling.feedback.changes.any(func(change): return str(change.actor_id) == actor and change.has("count_after")): earlier = true; break
		if earlier: continue
		var profile: ActorProfile = screen._actor_profiles.get(actor)
		if not is_instance_valid(profile): continue
		var counts := {}
		for status in profile._status_entries:
			if status.get("definition_id") == "curse_count": continue
			if status.get("definition_id") == _feedback_status("second_knell") and feedback.changes[_step].get("kind") == "second_knell_trigger": continue
			if status.get("definition_id") == _feedback_status("maledictions_refusal") and feedback.changes[_step].get("kind") == "refusal_trigger" and not _waiting(): continue
			counts[str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
		counts.curse_count = int(shown[actor])
		profile.statuses.set_counts(counts)

# A separate check uses a preview next to the tray. Never replace or reveal the
# saved offensive face, including when the opponent has not rolled yet.
func _refresh_extra_check(change: Dictionary, inverse: Transform2D) -> void:
	var progress := _curse_progress()
	var actor := str(change.actor_id)
	var index := int(change.index)
	_die_rect = Rect2(); _target = Rect2()
	if screen.dice_dock(actor) != null:
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() > 0 and dock.get_child(0) is BattleDiceTray:
			var tray: BattleDiceTray = dock.get_child(0)
			if index >= 0 and index < tray._buttons.size(): _die_rect = inverse * tray._buttons[index].get_global_rect()
	_roll.visible = progress >= 0.2
	_roll.position = _origin + Vector2(178, 85)
	_roll.pivot_offset = _roll.size * 0.5
	var rolling := progress >= 0.2 and progress < 0.55
	_roll.rotation = sin(_elapsed * 32) * 0.12 if rolling else 0.0
	var face := int(_elapsed * 24) % 6 + 1 if rolling else int(change.face)
	_roll.text = "%s%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(change.get("definition_id", "standard_d6")), face), " ⌁" if not rolling and change.get("cursed", false) else "", face]
	_stone(_roll, str(change.get("definition_id", "standard_d6")), face, change, not rolling and change.get("cursed", false))
	var outcome := "Checking this die for Curse" if progress < 0.2 else "Rolling separate check…" if rolling else "Face %d · %s" % [int(change.face), "Curse hit · +1 Count" if change.get("cursed", false) else "Clean face · no Count"]
	_label.text = "%s · %s · D%d\n%s\nOffensive result unchanged" % [_feedback_title("Unquiet Hands"), screen._actor_display_name(actor), index + 1, outcome]
	if progress >= 0.55 and change.has("count_after"):
		_label.text += "\nCurse Count %d → %d" % [int(change.count_before), int(change.count_after)]
		var profile: ActorProfile = screen._actor_profiles.get(actor)
		if is_instance_valid(profile): _count_target = inverse * profile.anchor_rect("status", "curse_count")
	if progress >= 0.55 and change.get("released", false): _label.text += "\nEntombment released"
	if feedback.changes.size() > 1: _label.text += "\nCheck %d of %d" % [_step + 1, feedback.changes.size()]

func _draw_extra_check(progress: float, color: Color) -> void:
	if _die_rect.size != Vector2.ZERO:
		draw_rect(_die_rect.grow(3), Color(color, 0.7), false, 2, true)
		if progress < 0.3 and is_instance_valid(_card):
			var start := _card.get_transform() * (_card.size * 0.5)
			var tip := start.lerp(_die_rect.get_center(), clampf(progress / 0.2, 0, 1))
			draw_line(start, tip, Color(color, 0.2), 8, true); draw_line(start, tip, color, 2, true)
		if progress >= 0.2 and progress < 0.55:
			draw_line(_die_rect.get_center(), _roll.position + _roll.size * 0.5, Color(color, 0.4), 2, true)
	if _count_target.size != Vector2.ZERO and progress >= 0.55:
		var start := _roll.position + _roll.size * 0.5
		var edge := Vector2(_count_target.position.x if start.x < _count_target.position.x else _count_target.end.x, _count_target.get_center().y)
		var tip := start.lerp(edge, clampf((progress - 0.55) / 0.2, 0, 1))
		draw_line(start, tip, Color(color, 0.2), 8, true); draw_line(start, tip, color, 2, true); draw_circle(tip, 4, color)
		if progress >= 0.75: draw_rect(_count_target.grow(3 + sin(_elapsed * 20) * 2), Color(color, 0.8), false, 2, true)

func _refresh_knell(change: Dictionary, inverse: Transform2D) -> void:
	var progress := _curse_progress()
	var expired: bool = change.kind == "second_knell_expired"
	var actor := str(change.actor_id)
	var profile: ActorProfile = screen._actor_profiles.get(actor)
	_knell_origin = Rect2(); _die_rect = Rect2(); _target = Rect2()
	_ability_label.show(); _show_status_symbol(_feedback_status("second_knell"))
	_ability_label.modulate.a = 1.0 - clampf((progress - (0.15 if expired else 0.36)) / 0.14, 0, 1)
	if is_instance_valid(profile):
		var bounds: Rect2 = inverse * profile.anchor_rect("status", _feedback_status("second_knell"))
		_ability_label.position = bounds.position
		_ability_label.size = Vector2(bounds.size.x, 28)
		_knell_origin = Rect2(_ability_label.position, _ability_label.size)
		_count_target = inverse * profile.anchor_rect("status", "curse_count")
	_label.position = _origin + Vector2(0, 200 if is_instance_valid(_card) else 165)
	_roll.position = _origin + (Vector2(178, 85) if is_instance_valid(_card) else Vector2(95, 65))
	_roll.visible = not expired
	if expired:
		_label.text = "%s expired\nUnused effect expired" % _feedback_title("Second Knell")
		return
	var die: Dictionary = change.die
	var retries: Array = change.get("retry_dice", [change.retry_die])
	var retry_flags: Array = change.get("retry_cursed_faces", [change.retry_cursed])
	var retry_progress := clampf((progress - 0.5) / 0.22, 0, 0.99999) * retries.size()
	var retry_index := mini(int(retry_progress), retries.size() - 1)
	var retry: Dictionary = retries[retry_index]
	var index := int(die.index)
	if screen.dice_dock(actor) != null:
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() > 0 and dock.get_child(0) is BattleDiceTray:
			var tray: BattleDiceTray = dock.get_child(0)
			if index >= 0 and index < tray._buttons.size(): _die_rect = inverse * tray._buttons[index].get_global_rect()
	var rolling := progress < 0.2 or (progress >= 0.5 and progress < 0.72 and fmod(retry_progress, 1.0) < 0.65)
	var face := int(_elapsed * 26) % 6 + 1 if rolling else int(die.face) if progress < 0.5 else int(retry.face)
	var cursed: bool = not rolling and (progress < 0.5 or retry_flags[retry_index])
	_roll.pivot_offset = _roll.size * 0.5; _roll.rotation = sin(_elapsed * 30) * 0.12 if rolling else 0.0
	_roll.text = "%s%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(die.die_id), face), " ⌁" if cursed else "", face]
	_stone(_roll, str(die.die_id), face, change, cursed)
	var outcome := "Original roll…"
	if progress >= 0.2: outcome = "Face %d · Curse hit · +1 Count" % int(die.face)
	if progress >= 0.36: outcome = "%s consumed\nReroll this same die %d time(s)" % [_feedback_title("Second Knell"), retries.size()]
	if progress >= 0.5: outcome = "Reroll %d / %d · %s" % [retry_index + 1, retries.size(), "Rolling…" if rolling else "Face %d" % int(retry.face)]
	if progress >= 0.72: outcome = "Final face %d · %s\nCurse Count %d → %d" % [int(retry.face), "+1 more Count" if change.retry_cursed else "clean · no extra Count", int(change.count_before), int(change.count_after)]
	_label.text = "%s · %s · D%d\n%s" % [_feedback_title("Second Knell"), screen._actor_display_name(actor), index + 1, outcome]
	if change.get("roll_context") == "curse_dice": _label.text += "\nSeparate check · offense unchanged"
	else: _label.text += "\nUse the final result"

func _draw_knell(progress: float, color: Color) -> void:
	if feedback.changes[_step].kind == "second_knell_expired": return
	var preview := _roll.position + _roll.size * 0.5
	if is_instance_valid(_card) and progress < 0.2 and _die_rect.size != Vector2.ZERO:
		var start := _card.get_transform() * (_card.size * 0.5)
		var tip := start.lerp(_die_rect.get_center(), clampf(progress / 0.15, 0, 1))
		draw_line(start, tip, color, 2, true)
	if _die_rect.size != Vector2.ZERO:
		draw_rect(_die_rect.grow(3), Color(color, 0.7), false, 2, true)
		draw_line(_die_rect.get_center(), preview, Color(color, 0.25), 2, true)
	if progress >= 0.36 and progress < 0.56 and _knell_origin.size != Vector2.ZERO:
		var start := _knell_origin.get_center()
		var tip := start.lerp(preview, clampf((progress - 0.36) / 0.14, 0, 1))
		draw_line(start, tip, Color(color, 0.2), 8, true); draw_line(start, tip, color, 2, true); draw_circle(tip, 5, color)
	var gain := (progress - 0.2) / 0.1 if progress < 0.36 else (progress - 0.72) / 0.14
	var show_gain: bool = (progress >= 0.2 and progress < 0.36) or (progress >= 0.72 and feedback.changes[_step].retry_cursed)
	if show_gain and _count_target.size != Vector2.ZERO:
		var edge := Vector2(_count_target.position.x if preview.x < _count_target.position.x else _count_target.end.x, _count_target.get_center().y)
		var tip := preview.lerp(edge, clampf(gain, 0, 1))
		draw_line(preview, tip, Color(color, 0.2), 8, true); draw_line(preview, tip, color, 2, true)
		if gain >= 1: draw_rect(_count_target.grow(3), color, false, 2, true)

# The status owns this separate cleanse check, never the saved offensive dice.
func _refresh_refusal(change: Dictionary, inverse: Transform2D) -> void:
	var progress := _curse_progress()
	var expired: bool = change.kind == "refusal_expired"
	var actor := str(change.actor_id)
	var profile: ActorProfile = screen._actor_profiles.get(actor)
	_target = Rect2(); _die_rect = Rect2()
	# Queued feedback may be refreshed on the Damage/Effects board. It must
	# not consume or fade an active status until its own beat is presented.
	if _waiting():
		_ability_label.hide(); _roll.hide(); return
	if expired:
		_refresh_refusal_expiry(profile, actor, inverse, progress)
		return
	_ability_label.show(); _show_status_symbol(_feedback_status("maledictions_refusal"))
	_ability_label.modulate.a = 1.0 - clampf((progress - 0.15) / 0.2, 0, 1)
	if is_instance_valid(profile):
		var bounds: Rect2 = inverse * profile.anchor_rect("status", _feedback_status("maledictions_refusal"))
		_ability_label.position = bounds.position
		_ability_label.size = Vector2(bounds.size.x, 28)
		_count_target = inverse * profile.anchor_rect("status", "curse_count")
	_roll.visible = not expired
	_roll.position = _origin + Vector2(95, 65)
	_label.position = _origin + Vector2(0, 165)
	var die: Dictionary = change.die
	var rolling: bool = progress < 0.45 and not change.get("already_rolled", false)
	var face := int(_elapsed * 26) % 6 + 1 if rolling else int(die.face)
	_roll.text = "%s%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(die.die_id), face), " ⌁" if not rolling and change.blocked else "", face]
	_stone(_roll, str(die.die_id), face, change, not rolling and change.blocked)
	_roll.pivot_offset = _roll.size * 0.5
	_roll.rotation = sin(_elapsed * 30) * 0.12 if rolling else 0.0
	_target = Rect2(_roll.position, _roll.size)
	_die_rect = _target
	_defense_start = _ability_label.position + _ability_label.size * 0.5
	_label.text = "%s consumed\nChecking D%d against Count cleanse…" % [_feedback_title("Malediction’s Refusal"), int(die.index) + 1]
	if not rolling:
		_label.text = "%s consumed\nD%d · Face %d\n%s\nCurse Count %d → %d\nOffensive result unchanged" % [_feedback_title("Malediction’s Refusal"), int(die.index) + 1, face, "CURSED · CLEANSE BLOCKED" if change.blocked else "CLEAN · CLEANSE SUCCEEDS", int(change.count_before), int(change.count_after)]


func _refresh_refusal_expiry(profile: ActorProfile, actor: String, inverse: Transform2D, progress: float) -> void:
	_ability_label.hide(); _roll.hide()
	if not is_instance_valid(profile): return
	_refusal_expiry_profile = profile
	# The authoritative snapshot may already be in Planning. Hold just this
	# status in its real HUD slot for its local expiration presentation.
	var counts: Dictionary = profile.statuses.counts.duplicate()
	counts[_feedback_status("maledictions_refusal")] = 1
	profile.statuses.set_counts(counts)
	var slot: Control = profile.statuses.ensure_slot(_feedback_status("maledictions_refusal"))
	slot.modulate.a = 1.0 - smoothstep(0.05, 0.55, progress)
	slot.mouse_filter = Control.MOUSE_FILTER_IGNORE if progress >= 0.55 else Control.MOUSE_FILTER_STOP
	var bounds: Rect2 = inverse * slot.get_global_rect()
	_defense_start = bounds.get_center()
	_label.text = "%s expired\nNo cleanse attempted" % _feedback_title("Malediction’s Refusal")
	_label.add_theme_stylebox_override("normal", THEME.panel(Color("10191ff2"), Color("b6ad92"), 6))
	_label.add_theme_color_override("font_color", Color("f3ead4"))
	_label.custom_minimum_size = Vector2(270, 0); _label.size = Vector2(270, 0)
	_label.size.y = _label.get_minimum_size().y
	var left := bounds.get_center().x > 960
	var hud: Rect2 = inverse * profile.get_global_rect()
	var caption := Rect2(Vector2(hud.position.x - _label.size.x - 18 if left else hud.end.x + 18, bounds.get_center().y - _label.size.y * 0.5), _label.size)
	var crowded := caption.position.x < 16 or caption.end.x > 1904
	for other in screen._actor_profiles.values():
		if caption.intersects((inverse * other.get_global_rect()).grow(8)): crowded = true
	if crowded:
		# Dense encounters have no lateral room. Use the space just beneath
		# this actor's dice rather than covering anyone's health or statuses.
		var bottom := hud.end.y
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if is_instance_valid(dock) and dock.is_visible_in_tree(): bottom = maxf(bottom, (inverse * dock.get_global_rect()).end.y)
		caption.position = Vector2(hud.get_center().x - caption.size.x * 0.5, bottom + 16)
	_label.position = Vector2(clampf(caption.position.x, 16, 1904 - _label.size.x), clampf(caption.position.y, 16, 1064 - _label.size.y))
	_label.modulate.a = smoothstep(0.05, 0.35, progress)
	_target = Rect2(_label.position, _label.size)

func _draw_refusal_expiry() -> void:
	if _waiting() or not _target.has_area(): return
	var progress := _curse_progress()
	var travel := clampf((progress - 0.05) / 0.35, 0, 1)
	var fade := 1.0 - smoothstep(0.5, 0.8, progress)
	if travel <= 0 or fade <= 0: return
	var end := Vector2(clampf(_defense_start.x, _target.position.x, _target.end.x), clampf(_defense_start.y, _target.position.y, _target.end.y))
	var tip := _defense_start.lerp(end, travel)
	draw_line(_defense_start, tip, Color(0.67, 0.94, 0.43, fade * 0.2), 8, true)
	draw_line(_defense_start, tip, Color(0.67, 0.94, 0.43, fade), 2.5, true)
	draw_circle(tip, 4, Color("d6ffb0") * Color(1, 1, 1, fade))


# One visual beat per source effect, regardless of how many marks it produces.
# Keep status triggers, forced rerolls and intentional face changes distinct.
func _face_batch_end() -> int:
	if _step >= feedback.changes.size(): return _step + 1
	var first: Dictionary = feedback.changes[_step]
	if not _is_face_application(first): return _step + 1
	var end := _step + 1
	while end < feedback.changes.size():
		var next: Dictionary = feedback.changes[end]
		if not _is_face_application(next): break
		var same_source := true
		for key in ["actor_id", "source_actor_id", "source_ability_id", "attack_source_id", "defense_source_id", "source_card_id"]:
			if next.get(key, "") != first.get(key, ""): same_source = false
		if not same_source: break
		end += 1
	return end

func _is_face_application(change: Dictionary) -> bool:
	return str(change.get("kind", "")).is_empty() and change.has("face") and change.has("index") and (change.get("marked", false) or change.get("rolled", false))

func _refresh_face_batch(inverse: Transform2D, heading: String) -> void:
	_roll.hide(); _release_pending_face()
	var progress := _curse_progress()
	var descriptions: Array[String] = []
	var marks := 0
	var rolls := 0
	var unique := {}
	var count_start := -1
	var count_end := -1
	var ordinary := false
	for i in range(_step, _face_batch_end()):
		var change: Dictionary = feedback.changes[i]
		ordinary = ordinary or change.get("ordinary_curse", false)
		var actor := str(change.actor_id)
		var index := int(change.index)
		var face := int(change.face)
		if change.has("count_after"):
			if count_start < 0: count_start = int(change.count_before)
			count_end = int(change.count_after)
		if change.get("marked", false):
			var key := "%s:%d:%d" % [actor, index, face]
			if not unique.has(key):
				unique[key] = true; marks += 1
				descriptions.append("D%d → %d" % [index + 1, face])
		if change.get("rolled", false):
			rolls += 1
			if not change.get("marked", false): descriptions.append("D%d · face %d · %s" % [index + 1, face, "already cursed · +1 Count" if change.get("cursed", false) else "clean · no Count"])
		if screen.dice_dock(actor) == null: continue
		var dock: Control = screen.dice_dock(actor)
		screen.reveal_dice_for_effect(dock)
		if dock.get_child_count() == 0 or not dock.get_child(0) is BattleDiceTray: continue
		var tray: BattleDiceTray = dock.get_child(0)
		if index < 0 or index >= tray._buttons.size() or face < 1 or face > 6: continue
		if change.get("marked", false):
			if not _attack_masks.any(func(mask): return is_instance_valid(mask.tray) and mask.tray == tray and mask.index == index and mask.face == face):
				_attack_masks.append({"tray": tray, "index": index, "face": face})
			tray.set_curse_face_pending(index, face, progress < 0.52 and not screen._history_review and not screen._snapshot_panel_open)
		if _inline_defense() and change.get("released", false) and progress >= 0.3:
			tray._bindings[index].hide(); tray._bound_labels[index].hide()
		var die_bounds: Rect2 = inverse * tray._buttons[index].get_global_rect()
		if die_bounds not in _batch_dice: _batch_dice.append(die_bounds)
		var line_start := Vector2(die_bounds.get_center().x, die_bounds.end.y)
		if change.get("rolled", false):
			# Paint a temporary check directly on the physical slot. Saved
			# offensive dice never change. Multiple checks of the same die
			# share that slot, so every actual result remains visible together.
			var same_die := 0
			var ordinal := 0
			for j in range(_step, _face_batch_end()):
				var other: Dictionary = feedback.changes[j]
				if other.get("rolled", false) and other.actor_id == actor and int(other.index) == index:
					if j < i: ordinal += 1
					same_die += 1
			while _batch_rolls.size() < rolls:
				var preview := Label.new()
				preview.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
				preview.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
				preview.add_theme_stylebox_override("normal", THEME.panel(Color("24202b"), Color("d5a2ef"), 0))
				preview.mouse_filter = Control.MOUSE_FILTER_IGNORE; add_child(preview); _batch_rolls.append(preview)
			var preview := _batch_rolls[rolls - 1]
			var width := die_bounds.size.x / same_die
			preview.position = die_bounds.position + Vector2(ordinal * width, 0)
			preview.size = Vector2(width, die_bounds.size.y)
			preview.add_theme_font_size_override("font_size", 23 if same_die == 1 else 16)
			preview.pivot_offset = preview.size * 0.5
			var rolling := progress < 0.3
			preview.rotation = sin(_elapsed * 28 + i) * 0.10 if rolling else 0.0
			var shown := (int(_elapsed * 22) + i * 2) % 6 + 1 if rolling else face
			preview.text = "%s%s\n%d" % [BattlePresentationCatalog.symbol_for_die_face(str(change.get("definition_id", "standard_d6")), shown), " ⌁" if not rolling and change.get("cursed", false) else "", shown]
			_stone(preview, str(change.get("definition_id", "standard_d6")), shown, change, not rolling and change.get("cursed", false))
			preview.visible = not _inline_effect() or _elapsed >= _launch_end()
			line_start.x = preview.position.x + preview.size.x * 0.5
		var chip: Control = tray._mark_faces[index][face - 1]
		if change.get("marked", false) and chip.is_visible_in_tree():
			var bounds: Rect2 = inverse * chip.get_global_rect()
			if bounds not in _batch_targets:
				_batch_targets.append(bounds)
				_batch_starts.append(line_start)
	_label.position = _origin + Vector2(0, 200 if is_instance_valid(_card) else 0)
	var detail := "%d faces cursed together" % marks if marks > 0 else "%d Curse checks resolved" % rolls
	if rolls > 0 and progress < 0.3: detail = "Rolling %d Curse checks…" % rolls
	elif not descriptions.is_empty(): detail += "\n" + " · ".join(descriptions)
	if rolls == 0: detail += "\nApplied directly · no roll"
	elif ordinary: detail += "\nClean dice: face 1.\nAlready cursed dice: roll to expand."
	else: detail += "\n%d rolled checks resolved" % rolls
	if rolls > 0: detail += "\nCurse checks · offense unchanged"
	if count_start >= 0 and progress >= 0.3: detail += "\nCurse Count %d → %d" % [count_start, count_end]
	if BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "widen_the_crack" and marks > 0:
		var change: Dictionary = feedback.changes[_step]
		detail = "Die %d · face %d cursed\nAdjacent face marked" % [int(change.index) + 1, int(change.face)]
	if BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "widen_the_crack" and marks == 0:
		var change: Dictionary = feedback.changes[_step]
		detail = "Die %d · rolling…" % (int(change.index) + 1) if progress < 0.3 else "Die %d rolled %d · %s" % [int(change.index) + 1, int(change.face), "cursed result" if change.get("cursed", false) else "clean result · no Count"]
		if progress >= 0.3:
			if count_start >= 0: detail += "\nCurse Count %d → %d" % [count_start, count_end]
			var work: Dictionary = screen._view.raw_snapshot.get("curse_choice", {})
			detail += "\nChoose an adjacent highlighted face next." if work.get("kind") == "adjacent" else "\nNo adjacent face available."
		detail += "\nOffensive result unchanged"
	_label.text = "%s\n%s\n%s" % [heading, screen._actor_display_name(str(feedback.changes[_step].actor_id)), detail]
	if count_start >= 0:
		var profile: ActorProfile = screen._actor_profiles.get(str(feedback.changes[_step].actor_id))
		if is_instance_valid(profile): _count_target = inverse * profile.anchor_rect("status", "curse_count")

func _draw_face_batch(progress: float, color: Color) -> void:
	var source := _defense_start if _inline_effect() or _ability_label.visible else _card.get_transform() * (_card.size * 0.5) if is_instance_valid(_card) else _label.position + Vector2(_label.size.x * 0.5, 0)
	if _inline_effect():
		# Launch beside the prevention trail, then resolve the existing Curse
		# check on the receiving die. Never use a detached text pop-up as origin.
		var flight := clampf((_elapsed - _launch_delay()) / 0.52, 0, 1)
		if flight > 0 and _elapsed < _launch_end() + 0.3:
			for bounds in _batch_dice:
				var tip := source.lerp(bounds.get_center(), flight)
				draw_line(source, tip, Color(0.67, 0.94, 0.43, 0.85), 3, true)
				draw_circle(tip, 5, Color("d6ffb0"))
	for bounds in _batch_dice: draw_rect(bounds.grow(3), Color(color, 0.6), false, 2, true)
	if progress < 0.3: return
	var travel := clampf((progress - 0.3) / 0.22, 0, 1)
	for i in _batch_targets.size():
		var target := _batch_targets[i]
		var start := _batch_starts[i]
		var end := target.get_center()
		var tip := start.lerp(end, travel)
		# Keep every connection visible through the shared result hold.
		draw_line(start, tip, Color(color, 0.16), 7, true)
		draw_line(start, tip, Color(color, 0.85), 2, true)
		if travel < 1: draw_circle(tip, 4, color)
		else:
			var pulse := (sin((progress - 0.52) * 24) + 1) * 0.5
			draw_rect(target.grow(3 + pulse * 2), Color(color, 0.8), false, 2, true)
	if _count_target.size != Vector2.ZERO and progress >= 0.52:
		var tip := source.lerp(_count_target.get_center(), clampf((progress - 0.52) / 0.18, 0, 1))
		draw_line(source, tip, color, 2, true)

func _uses_tray_curse() -> bool:
	if _step >= feedback.changes.size() or BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "unquiet_hands": return false
	return _is_face_application(feedback.changes[_step]) and (_inline_effect() or _face_batch_end() > _step + 1 or feedback.changes[_step].get("rolled", false) or BattlePresentationCatalog.card_mechanic(str(feedback.card_id)) == "widen_the_crack")


func _refresh_dividend(change: Dictionary, inverse: Transform2D) -> void:
	var progress := _curse_progress()
	var expired: bool = change.kind == "dividend_expired"
	_roll.hide(); _target = Rect2(); _die_rect = Rect2()
	if is_instance_valid(_card): _card.hide()
	_label.position = _origin
	var profile: ActorProfile = screen._actor_profiles.get(str(change.actor_id))
	if is_instance_valid(profile):
		_target = inverse * profile.anchor_rect("status", _feedback_status("black_dividend"))
		_ability_label.show(); _show_status_symbol(_feedback_status("black_dividend"))
		_ability_label.position = _target.position
		_ability_label.size = Vector2(_target.size.x, 26)
		_ability_label.modulate.a = 1.0 - progress if expired or change.get("consumed", false) else sin(progress * PI)
	if expired:
		_label.text = "%s expired\n%d / %d rewards awarded" % [_feedback_title("Black Dividend"), int(change.get("rewards", 0)), int(change.get("limit", 2))]
		return
	var dock: Control = screen.dice_dock(str(change.actor_id))
	screen.reveal_dice_for_effect(dock)
	if is_instance_valid(dock) and dock.get_child_count() > 0:
		var tray: BattleDiceTray = dock.get_child(0)
		var index := int(change.die.index)
		if index >= 0 and index < tray._buttons.size(): _die_rect = inverse * tray._buttons[index].get_global_rect()
	var recipient: ActorProfile = screen._actor_profiles.get(str(change.source_actor_id))
	if is_instance_valid(recipient):
		_count_target = inverse * recipient.anchor_rect("energy")
		recipient.show_resource_gain({"stat": "energy", "before": int(change.energy_before), "after": int(change.energy_after)}, clampf((progress - 0.55) / 0.2, 0, 1))
	_label.text = "%s activated\nD%d · cursed face %d\n%s gains %d Energy\nEnergy %d → %d\n%d / %d rewards · %s" % [_feedback_title("Black Dividend"), int(change.die.index) + 1, int(change.die.face), screen._actor_display_name(str(change.source_actor_id)), int(change.energy_after) - int(change.energy_before), int(change.energy_before), int(change.energy_after), int(change.rewards), int(change.get("limit", 2)), "status consumed" if change.get("consumed", false) else "waits for next round"]

func _draw_dividend(progress: float) -> void:
	var color := Color("e2b2ff")
	if _target.size != Vector2.ZERO: draw_rect(_target.grow(3), Color(color, sin(progress * PI)), false, 2, true)
	if feedback.changes[_step].kind == "dividend_expired": return
	if _die_rect.size != Vector2.ZERO:
		draw_rect(_die_rect.grow(3), Color(color, 0.8), false, 2, true)
		if _target.size != Vector2.ZERO:
			draw_line(_die_rect.get_center(), _die_rect.get_center().lerp(_target.get_center(), clampf(progress / 0.3, 0, 1)), color, 2, true)
	if _target.size != Vector2.ZERO and _count_target.size != Vector2.ZERO and progress > 0.3:
		var tip := _target.get_center().lerp(_count_target.get_center(), clampf((progress - 0.3) / 0.35, 0, 1))
		draw_line(_target.get_center(), tip, Color(color, 0.25), 7, true)
		draw_line(_target.get_center(), tip, color, 2, true); draw_circle(tip, 4, color)
		if progress > 0.65: draw_rect(_count_target.grow(3), Color("ffe9ac"), false, 2, true)


func _refresh_bloom(change: Dictionary, inverse: Transform2D) -> void:
	_roll.hide(); _target = Rect2(); _die_rect = Rect2()
	_label.position = _origin
	var profile: ActorProfile = screen._actor_profiles.get(str(change.actor_id))
	if is_instance_valid(profile):
		_target = inverse * profile.anchor_rect("status", _feedback_status("curse_bloom"))
		_ability_label.show(); _show_status_symbol(_feedback_status("curse_bloom"))
		_ability_label.position = _target.position
		_ability_label.size = Vector2(_target.size.x, 26)
		_ability_label.modulate.a = 1.0 - _curse_progress()
	if change.kind == "bloom_expired":
		_label.text = "%s expired\nNo damaging Curse conversion" % _feedback_title("Curse Bloom")
	elif int(change.damage) == 0:
		_label.text = "%s consumed\nCurse damage removed no health\nNo expansion" % _feedback_title("Curse Bloom")
	elif int(change.attempts) == 0:
		_label.text = "%s consumed\nCurse removed %d health\nNo cursed dice to expand" % [_feedback_title("Curse Bloom"), int(change.damage)]
	else:
		_label.text = "%s activated · consumed\nCurse removed %d health\n%d expansion attempts\nNew Count waits until next Effects" % [_feedback_title("Curse Bloom"), int(change.damage), int(change.attempts)]

func _show_status_symbol(status_id: String) -> void:
	# Consumed statuses fade at their own slot using the same symbol as the HUD.
	_ability_label.text = "    1"
	if not is_instance_valid(_status_symbol):
		_status_symbol = TextureRect.new(); _status_symbol.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_status_symbol.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		_status_symbol.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
		_ability_label.add_child(_status_symbol); _status_symbol.size = Vector2(28, 28)
	_status_symbol.texture = preload("res://presentation/battle/battle_icons.gd").texture(status_id)
	_status_symbol.show()

func _feedback_title(fallback: String) -> String:
	var id := str(feedback.changes[mini(_step, feedback.changes.size() - 1)].get("status_card_id", feedback.get("card_id", "")))
	return str(BattlePresentationCatalog.card(id).name) if not id.is_empty() else str(feedback.get("title", fallback))

func _feedback_status(fallback: String) -> String:
	var id := str(feedback.changes[mini(_step, feedback.changes.size() - 1)].get("status_card_id", feedback.get("card_id", "")))
	var kind := BattlePresentationCatalog.card_mechanic(id)
	return id + "_card_effect" if kind == fallback and id != kind else fallback

## Roll previews use the same carved stone as the owner's tray dice.
func _stone(preview: Label, die_id: String, face: int, change: Dictionary, cursed: bool) -> void:
	var owner := str(change.get("actor_id", ""))
	StoneDie.dress(preview, die_id, face, not owner.is_empty() and owner != screen.viewer_actor_id, "⌁" if cursed else "")
