extends Control

# Played cards retain their hand pose; feedback survives board rebuilds. Multiple
# effects of a card share its reveal and trails, never a stack of floating text.
# Successive plays never hold player input.
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var screen: Control
var updates: Array[Dictionary] = []
var feedback: Dictionary
var _cards: Array[BattleCard] = []
var _label: Label
var _elapsed := 0.0
var _battle_id := ""
var _started := false
var duration := 2.4
var _destinations: Array[Vector2] = []
var _poses: Dictionary = {}
var _draw_cards: Dictionary = {}
var _hidden_hand_cards: Array[BattleCard] = []
var _held_hand: Control

func _exit_tree() -> void:
	for card in _hidden_hand_cards:
		if is_instance_valid(card): card.modulate.a = 1.0
	if is_instance_valid(_held_hand) and not hand_held_elsewhere(screen, _held_hand, self): _held_hand.gain_animation_active = false

# Concurrent card feedback may still be flying into the same hand.
static func hand_held_elsewhere(owner_screen: Node, hand: Control, leaving: Node) -> bool:
	if not is_instance_valid(owner_screen): return false
	for sibling in owner_screen.get_children():
		if sibling != leaving and sibling.get_meta("feedback_notice", false) and not sibling.is_queued_for_deletion() and sibling.get("_held_hand") == hand: return true
	return false

func configure(owner_screen: Control, changes: Array[Dictionary], poses: Dictionary = {}) -> void:
	screen = owner_screen; updates = changes
	process_priority = 20
	set_meta("feedback_notice", true)
	feedback = updates[0].card_feedback
	for played in feedback.cards:
		if poses.has(str(played.instance_id)): _poses[str(played.instance_id)] = poses[str(played.instance_id)].duplicate(true)
	_battle_id = screen._view.battle_id
	mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 30
	duration = maxf(0.5, TIMING.seconds("card_gain_seconds", 2.4)) * feedback.cards.size()
	for played in feedback.cards:
		var card := BattleCard.new(); card.name = "PlayedGainCard"
		card.configure(str(played.instance_id), str(played.definition_id), _poses.has(str(played.instance_id)), false, true)
		card.custom_minimum_size = Vector2(145, 195); card.size = card.custom_minimum_size
		card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.hide(); add_child(card); _cards.append(card)
	_label = Label.new(); _label.name = "CardGainSummary"; _label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.custom_minimum_size = Vector2(145, 0); _label.size.x = 145
	_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_label.add_theme_font_size_override("font_size", 14)
	_label.add_theme_stylebox_override("normal", preload("res://presentation/battle/cinematic_theme.gd").panel(Color("101b22f5"), Color("8ab789"), 5))
	_label.add_theme_color_override("font_color", Color("c8e9b4")); add_child(_label)
	var lines: Array[String] = []
	for update in updates:
		var data: Dictionary = update.data
		if update.kind == "resource": lines.append("+%d %s" % [int(data.amount), "card" if data.stat == "hand" and int(data.amount) == 1 else data.caption])
		elif update.kind == "attack_bonus": lines.append("%s\n%d + %d = %d damage" % [BattlePresentationCatalog.ability(str(data.ability_id)).name, int(data.before), int(data.amount), int(data.after)])
		elif update.kind == "conversion": lines.append("Poison → Volatile Poison")
		else: lines.append("+%d %s" % [int(data.after) - int(data.before), BattlePresentationCatalog.status(str(data.get("status_id", "incubation"))).name])
	_label.text = " · ".join(lines)
	for update in updates:
		if update.kind != "resource" or update.data.stat != "hand" or str(update.data.target_actor_id) != screen.viewer_actor_id: continue
		for id in update.data.get("drawn_ids", []):
			for entry in screen._view.hand_cards():
				if entry.instance_id != id or _draw_cards.has(id): continue
				var card := BattleCard.new(); add_child(card)
				card.configure(str(id), str(entry.definition_id), false, false, true)
				card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.size = BattleCard.STANDARD_SIZE; card.hide()
				_draw_cards[id] = card
	modulate.a = 0.0

func _waiting() -> bool:
	# Cards shown from their own hand pose overlap freely. Pose-less cards (an
	# opponent's burst) share one spot, so they still play one at a time.
	if _poses.is_empty():
		for sibling in screen.get_children():
			if sibling == self: break
			if sibling.get_script() == get_script() and not sibling.is_queued_for_deletion() and sibling._poses.is_empty(): return true
	for update in updates:
		if update.kind == "attack_bonus" and not is_instance_valid(screen.ability_intent(str(update.data.target_actor_id), str(update.data.ability_id))) and not is_instance_valid(screen._selected_attack_tiles.get(str(update.data.target_actor_id))): return true
	return screen._director.has_pending_status_animation()

func _process(delta: float) -> void:
	if not is_instance_valid(screen) or screen._view.battle_id != _battle_id: queue_free(); return
	if _waiting() or screen._history_review or screen._snapshot_panel_open:
		modulate.a = 0.0; return
	_started = true; _elapsed += delta
	refresh()

func refresh() -> void:
	if not is_instance_valid(screen) or not is_instance_valid(screen._root): return
	# Work in the same 1920×1080 board coordinates at any window size.
	var board_transform: Transform2D = screen.get_global_transform_with_canvas().affine_inverse() * screen._root.get_global_transform_with_canvas()
	position = board_transform.origin; scale = board_transform.get_scale()
	_refresh_draws()
	# Hold reward values at their baseline from the very first render, including
	# while queued. Otherwise the final energy flashes before its animation.
	if not _started:
		for update in updates:
			if update.kind == "attack_bonus": _show_attack_bonus(update.data, 0.0)
			var profile: ActorProfile = screen._actor_profiles.get(str(update.data.target_actor_id))
			if is_instance_valid(profile) and update.kind == "resource":
				profile.show_resource_gain(update.data, 0.0)
				if update.data.stat == "hand": _show_draw_counts(profile, update.data)
		return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var card_index := mini(_cards.size() - 1, int(_elapsed / (duration / _cards.size())))
	var source := str(feedback.cards[card_index].actor_id)
	var origin: Vector2 = Vector2(1345, 25) if source != screen.viewer_actor_id else inverse * screen._hand_dock.get_global_rect().get_center() - _cards[card_index].size * 0.5
	for index in _cards.size():
		_cards[index].visible = index == card_index
		var pose: Dictionary = _poses.get(str(feedback.cards[index].instance_id), {})
		if not pose.is_empty():
			var transform: Transform2D = pose.transform
			_cards[index].size = pose.size
			_cards[index].position = transform.origin
			_cards[index].rotation = transform.get_rotation()
			_cards[index].scale = transform.get_scale()
			_cards[index].modulate.a = 1.0 - smoothstep(0.08, 0.55, _elapsed / duration)
		else: _cards[index].position = origin
	_label.visible = not _poses.has(str(feedback.cards[card_index].instance_id))
	_label.position = origin + Vector2(0, 200)
	var progress := clampf((_elapsed / duration - 0.38) / 0.25, 0.0, 1.0)
	_destinations.clear()
	for update in updates:
		if update.kind == "attack_bonus":
			_show_attack_bonus(update.data, progress)
			var tile: Control = screen._selected_attack_tiles.get(str(update.data.target_actor_id))
			var intent: Control = screen.ability_intent(str(update.data.target_actor_id), str(update.data.ability_id))
			if is_instance_valid(intent): _destinations.append(inverse * intent.attack_damage_rect().get_center())
			elif is_instance_valid(tile): _destinations.append(inverse * tile.get_global_rect().get_center())
			continue
		var profile: ActorProfile = screen._actor_profiles.get(str(update.data.target_actor_id))
		if not is_instance_valid(profile): continue
		if update.kind == "resource":
			if update.data.stat == "hand" and update.data.has("drawn_ids"):
				_show_draw_counts(profile, update.data)
			else: profile.show_resource_gain(update.data, progress)
			var value: Label = profile._stat_labels.get(str(update.data.stat))
			if is_instance_valid(value) and int(update.data.amount) > 0: _destinations.append(inverse * profile.anchor_rect(str(update.data.stat)).get_center())
		else:
			profile.show_status_transition(update, progress)
			_destinations.append(inverse * profile.status_anchor(str(update.data.get("status_id", "volatile_poison" if update.kind == "conversion" else "incubation"))))
	# Preparation cards affect both a visible status and one named ability.
	# Follow the authoritative target rather than the currently hovered tile.
	for played in feedback.cards:
		var ability_id := str(played.get("ability_id", ""))
		if ability_id.is_empty() or str(played.actor_id) != screen.viewer_actor_id: continue
		if BattlePresentationCatalog.temporary_ability_damage(ability_id, screen._view.actor(str(played.actor_id))) <= 0: continue
		for tile in screen._ability_dock.find_children("*", "Button", true, false):
			if not tile is BattleAbilityTile or tile.ability_id != ability_id: continue
			var badge := tile.find_child("TemporaryDamageBonus", true, false) as Control
			if badge != null: _destinations.append(inverse * badge.get_global_rect().get_center())
	modulate.a = (1.0 if not _poses.is_empty() else clampf(_elapsed / 0.2, 0, 1)) * clampf((duration - _elapsed) / 0.4, 0, 1)
	queue_redraw()
	if _elapsed >= duration: queue_free()

func _show_attack_bonus(data: Dictionary, progress: float) -> void:
	var actor_id := str(data.target_actor_id)
	var intent: Control = screen.ability_intent(actor_id, str(data.ability_id))
	if is_instance_valid(intent): intent.damage.text = str(int(data.before) if progress <= 0 else int(data.after))
	var tile: BattleAbilityTile = screen._selected_attack_tiles.get(actor_id)
	if not is_instance_valid(tile) or tile.ability_id != str(data.ability_id): return
	var attack: Dictionary = screen._selected_attack(actor_id)
	var summary := str(attack.get("text", ""))
	if progress <= 0.0:
		summary = summary.replace("%d damage" % int(data.after), "%d damage" % int(data.before))
		summary = summary.replace("Includes +%d from %s" % [int(data.amount), BattlePresentationCatalog.card(str(data.card_definition_id)).name], "%s · +%d incoming" % [BattlePresentationCatalog.card(str(data.card_definition_id)).name, int(data.amount)])
	tile.update_selected_attack_summary(summary)
	tile.modulate = Color.WHITE.lerp(Color("bfffea"), sin(progress * PI) * 0.65)

func _draw() -> void:
	if not _started or _cards.is_empty(): return
	var progress := _elapsed / duration
	if progress < 0.20 or progress > 0.70: return
	var card_index := mini(_cards.size() - 1, int(_elapsed / (duration / _cards.size())))
	var card := _cards[card_index]
	var start := card.get_transform() * (card.size * 0.5)
	var travel := clampf((progress - 0.20) / 0.18, 0, 1)
	var alpha := clampf((0.70 - progress) / 0.12, 0, 1)
	for target in _destinations:
		var points := PackedVector2Array()
		for step in 25:
			var t := travel * step / 24.0
			points.append(start.lerp(target, t) + Vector2(0, -95 * sin(PI * t)))
		draw_polyline(points, Color(0.55, 0.94, 0.48, alpha * 0.14), 7, true)
		draw_polyline(points, Color(0.69, 1.0, 0.58, alpha), 2, true)
		draw_circle(points[-1], 4, Color(0.8, 1, 0.68, alpha))

func _draw_launch(index: int, count: int) -> float:
	return 0.40 + 0.18 * float(index) / maxi(1, count - 1)

func _show_draw_counts(profile: ActorProfile, data: Dictionary) -> void:
	var count := maxi(0, int(data.amount))
	var launched := 0
	for i in count:
		if _started and _elapsed / duration >= _draw_launch(i, count): launched += 1
	var progress := float(launched) / maxi(1, count)
	profile.show_resource_gain(data, progress)
	if data.has("deck_after"):
		profile.show_resource_gain({"stat": "deck", "before": data.deck_before, "after": data.deck_after}, progress)

func _refresh_draws() -> void:
	if (_draw_cards.is_empty() and _poses.is_empty()) or not is_instance_valid(screen._hand_dock): return
	var hand: Control = screen._hand_dock
	_held_hand = hand; hand.gain_animation_active = true
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var profile: ActorProfile = screen._actor_profiles.get(screen.viewer_actor_id)
	if not is_instance_valid(profile): return
	var origin := inverse * profile.anchor_rect("deck").get_center()
	var index := 0
	for id in _draw_cards:
		var flight: BattleCard = _draw_cards[id]
		flight.hide()
		for card in hand.cards:
			if card.instance_id != id: continue
			if card not in _hidden_hand_cards: _hidden_hand_cards.append(card)
			var t := clampf((_elapsed / duration - _draw_launch(index, _draw_cards.size())) / 0.25, 0, 1) if _started else 0.0
			card.modulate.a = 1.0 if t >= 1.0 else 0.0
			flight.visible = t > 0 and t < 1
			var target: Transform2D = inverse * card.get_global_transform_with_canvas()
			var eased := smoothstep(0, 1, t)
			flight.size = card.size
			flight.scale = (Vector2.ONE * 0.18).lerp(target.get_scale(), eased)
			flight.rotation = lerpf(-0.2, target.get_rotation(), eased)
			var center := origin.lerp(target * (card.size * 0.5), eased) + Vector2(0, -85 * sin(PI * eased))
			flight.position = center - Transform2D(flight.rotation, flight.scale, 0, Vector2.ZERO) * (flight.size * 0.5)
			flight.modulate.a = clampf(t / 0.12, 0, 1)
		index += 1
