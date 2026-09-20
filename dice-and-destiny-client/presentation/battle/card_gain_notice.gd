extends Control

# One card at a time beside the profile; survives board rebuilds. Multiple
# effects of a card share its reveal and trails, never a stack of floating text.
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

func configure(owner_screen: Control, changes: Array[Dictionary]) -> void:
	screen = owner_screen; updates = changes
	feedback = updates[0].card_feedback
	_battle_id = screen._view.battle_id
	mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 30
	duration = maxf(0.5, TIMING.seconds("card_gain_seconds", 2.4)) * feedback.cards.size()
	for played in feedback.cards:
		var card := BattleCard.new(); card.name = "PlayedGainCard"
		card.configure(str(played.instance_id), str(played.definition_id), false, false, true)
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
		elif update.kind == "conversion": lines.append("Poison → Volatile Poison")
		else: lines.append("+%d %s" % [int(data.after) - int(data.before), BattlePresentationCatalog.status(str(data.get("status_id", "incubation"))).name])
	_label.text = " · ".join(lines)
	modulate.a = 0.0

func _waiting() -> bool:
	# Serialize quick successive plays instead of piling them over the HUD.
	for sibling in screen.get_children():
		if sibling == self: break
		if sibling.get_script() == get_script() and not sibling.is_queued_for_deletion(): return true
	return screen._director.has_pending_status_animation()

func _process(delta: float) -> void:
	if not is_instance_valid(screen) or screen._view.battle_id != _battle_id: queue_free(); return
	if _waiting() or screen._history_review or screen._snapshot_panel_open:
		modulate.a = 0.0; return
	_started = true; _elapsed += delta
	refresh()

func refresh() -> void:
	if not is_instance_valid(screen) or not is_instance_valid(screen._root): return
	# Hold reward values at their baseline from the very first render, including
	# while queued. Otherwise the final energy flashes before its animation.
	if not _started:
		for update in updates:
			var profile: ActorProfile = screen._actor_profiles.get(str(update.data.target_actor_id))
			if is_instance_valid(profile) and update.kind == "resource": profile.show_resource_gain(update.data, 0.0)
		return
	# Work in the same 1920×1080 board coordinates at any window size.
	var board_transform: Transform2D = screen.get_global_transform_with_canvas().affine_inverse() * screen._root.get_global_transform_with_canvas()
	position = board_transform.origin; scale = board_transform.get_scale()
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var card_index := mini(_cards.size() - 1, int(_elapsed / (duration / _cards.size())))
	var source := str(feedback.cards[card_index].actor_id)
	var origin := Vector2(1345, 25) if source != "blade" else Vector2(448, 25)
	for index in _cards.size():
		_cards[index].visible = index == card_index
		_cards[index].position = origin
	_label.position = origin + Vector2(0, 200)
	var progress := clampf((_elapsed / duration - 0.38) / 0.25, 0.0, 1.0)
	_destinations.clear()
	for update in updates:
		var profile: ActorProfile = screen._actor_profiles.get(str(update.data.target_actor_id))
		if not is_instance_valid(profile): continue
		if update.kind == "resource":
			profile.show_resource_gain(update.data, progress)
			var value: Label = profile._stat_labels.get(str(update.data.stat))
			if is_instance_valid(value): _destinations.append(inverse * value.get_global_rect().get_center())
		else:
			profile.show_status_transition(update, progress)
			_destinations.append(inverse * profile.status_anchor(str(update.data.get("status_id", "volatile_poison" if update.kind == "conversion" else "incubation"))))
	modulate.a = clampf(_elapsed / 0.2, 0, 1) * clampf((duration - _elapsed) / 0.4, 0, 1)
	queue_redraw()
	if _elapsed >= duration: queue_free()

func _draw() -> void:
	if not _started or _cards.is_empty(): return
	var progress := _elapsed / duration
	if progress < 0.20 or progress > 0.70: return
	var card_index := mini(_cards.size() - 1, int(_elapsed / (duration / _cards.size())))
	var card := _cards[card_index]
	var start := card.position + card.size * 0.5
	var travel := clampf((progress - 0.20) / 0.18, 0, 1)
	var alpha := clampf((0.70 - progress) / 0.12, 0, 1)
	for target in _destinations:
		var tip := start.lerp(target, travel)
		draw_line(start, tip, Color(0.55, 0.94, 0.48, alpha * 0.14), 7, true)
		draw_line(start, tip, Color(0.69, 1.0, 0.58, alpha), 2, true)
		draw_circle(tip, 4, Color(0.8, 1, 0.68, alpha))
