extends Control

# Animate saved cards to their authority-published live pile (or explicit
# effect destination), without applying a second mutation to pile counts.
const GREEN := Color("a5edce")
const CARD_SIZE := Vector2(132, 144)
var _started := 0
var _screen: Control
var _played: BattleCard
var _caption: Label
var _saved: Array[Dictionary] = []
var _piles: Dictionary = {}
var _pending: Array[Dictionary] = []
var _hidden_grids: Array[Control] = []
var _elapsed := 0.0
var _source_id := ""
var _ability_feedback := false
var _before := 0
var _after := 0
var _original_count := 0
var _held_groups: Array[Dictionary] = []
var _hand_play := false
var _held_hand: Control
var _defense_timing := false
const DEFENSE_TIMING := preload("res://presentation/battle/defense_timing.gd")

static func reduction_progress(elapsed: float) -> float:
	return clampf((elapsed - 0.1) / 0.8, 0, 1)

func configure(data: Dictionary, screen: Control) -> void:
	_screen = screen
	process_priority = 20 # Publish the shared amount after the intent's preview clock.
	_before = int(data.before); _after = int(data.after)
	_original_count = int(data.get("original_count", data.get("saved", []).size() + data.get("pending", []).size()))
	_source_id = str(data.get("source_id", ""))
	_ability_feedback = str(data.get("card_id", "")).is_empty()
	_started = int(data.started_ms)
	_defense_timing = bool(data.get("defense_timing", false))
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	z_index = 30
	_hand_play = not _ability_feedback and str(data.get("actor_id", "")) == screen.viewer_actor_id
	_played = BattleCard.new(); add_child(_played)
	_played.configure(str(data.instance_id), str(data.card_id), _hand_play, false, not _hand_play)
	_played.size = CARD_SIZE
	if str(data.card_id).is_empty(): _played.hide()
	_played.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_played.focus_mode = Control.FOCUS_NONE
	_played.position = Vector2(280, 490) if str(data.get("actor_id", "")) == screen.viewer_actor_id else Vector2(1510, 490)
	if _hand_play:
		var pose: Dictionary = data.get("hand_pose", {})
		_played.size = pose.get("size", BattleCard.STANDARD_SIZE)
		var transform: Transform2D = pose.get("transform", Transform2D(0, get_global_transform_with_canvas().affine_inverse() * screen._hand_dock.get_global_rect().get_center() - _played.size * 0.5))
		_played.position = transform.origin
		_played.rotation = transform.get_rotation()
		_played.scale = transform.get_scale()
		_held_hand = screen._hand_dock
	_caption = _label("%d damage prevented" % maxi(0, int(data.before) - int(data.after)), 18)
	_caption.position = _played.position + Vector2(-40, -52)
	_caption.size = Vector2(215, 48)
	# The incoming row already explains an ability's reduction. Do not float
	# a duplicate caption or launch saved-card trails from an invisible card.
	if _ability_feedback or _hand_play: _caption.hide()
	for removal in data.get("saved", []):
		var target := str(removal.get("target_actor_id", ""))
		if not screen._actor_profiles.has(target): continue
		var zone := str(removal.get("released_destination", "discard"))
		var key := target + ":" + zone
		if not _piles.has(key):
			var notice := _label("", 16)
			_piles[key] = {"target": target, "zone": zone, "count": 0, "label": notice, "rect": Rect2()}
		_piles[key].count += 1
		var origin: Rect2 = removal.get("origin_rect", Rect2(520 + _saved.size() * 140, 460, 132, 144))
		var card := BattleCard.new(); add_child(card)
		card.configure(str(removal.card_id), str(removal.card_definition_id), false, false, true)
		card.size = CARD_SIZE; card.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_prepare_header(card, origin.size.x)
		var badge := card.get_node("RemovalState") as Label
		badge.text = "✓ SAVED"; badge.hide(); badge.add_theme_color_override("font_color", GREEN)
		_saved.append({"card": card, "origin": origin, "pile": key, "progress": 0.0, "native_size": card.size})
	for removal in data.get("pending", []):
		if not removal.has("origin_rect"): continue
		var card := BattleCard.new(); add_child(card)
		card.configure(str(removal.card_id), str(removal.card_definition_id), false, true)
		card.size = CARD_SIZE; card.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_prepare_header(card, removal.origin_rect.size.x)
		_pending.append({"card": card, "origin": removal.origin_rect, "native_size": card.size})
	present_progress(maxf(0.0, (Time.get_ticks_msec() - _started) / 1000.0))

func _prepare_header(card: BattleCard, width: float) -> void:
	# The full card's invisible multiline Button text enforces a tall minimum.
	# Headers need an actual 24px layout, never a vertically squashed full card.
	card.text = ""
	for state in ["normal", "hover", "pressed", "disabled"]:
		var style := card.get_theme_stylebox(state).duplicate()
		for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]: style.set_content_margin(side, 0)
		card.add_theme_stylebox_override(state, style)
	card.custom_minimum_size = Vector2.ZERO; card.size = Vector2(maxf(132, width), 24); card.clip_contents = true
	card._effect_plaque.hide(); card.get_node("RemovalState").hide()
	var title: Label = card.get_node("CardTitle")
	title.autowrap_mode = TextServer.AUTOWRAP_OFF; title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	title.offset_left = 29; title.offset_top = 0; title.offset_bottom = 24
	var cost: Label = card.get_node("EnergyCost")
	cost.offset_left = 4; cost.offset_right = 24; cost.offset_top = 2; cost.offset_bottom = 22; cost.add_theme_font_size_override("font_size", 14)

func _exit_tree() -> void:
	if is_instance_valid(_held_hand): _held_hand.prevention_animation_active = false
	for entry in _held_groups:
		if is_instance_valid(entry.group): entry.group.custom_minimum_size = entry.minimum
	for grid in _hidden_grids:
		if is_instance_valid(grid): grid.modulate.a = 1

func _label(value: String, font_size: int) -> Label:
	var label := Label.new(); add_child(label)
	label.text = value; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.add_theme_font_size_override("font_size", font_size)
	label.add_theme_color_override("font_color", GREEN)
	label.add_theme_color_override("font_shadow_color", Color.BLACK)
	label.add_theme_constant_override("shadow_outline_size", 4)
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return label

func _process(_delta: float) -> void:
	if get_parent() != _screen._root: return
	present_progress(maxf(0.0, (Time.get_ticks_msec() - _started) / 1000.0))

func present_progress(elapsed: float) -> void:
	# Match the die-to-attack trail's 20–65% effects interval. Saved cards and
	# the attack number must use this same clock before the next phase appears.
	if _defense_timing:
		var scale := DEFENSE_TIMING.effects_seconds() * 0.45 / 0.8
		var delay := DEFENSE_TIMING.effects_seconds() * 0.2 - scale * 0.1
		elapsed = maxf(0.0, (elapsed - delay) / scale)
	_elapsed = elapsed
	var presenter: Control = _screen._attack_intents.get(_source_id)
	if is_instance_valid(presenter):
		var progress := reduction_progress(elapsed)
		presenter.damage.text = str(roundi(lerpf(_before, _after, progress)))
		presenter.damage.modulate.a = 1.0 - 0.35 * sin(progress * PI)
		presenter._refresh_target_heading()
		if is_instance_valid(presenter.incoming_row): presenter.incoming_row.refresh()
	modulate.a = 1.0 - clampf((elapsed - 2.25) / 0.35, 0, 1)
	_played.modulate.a = 1.0 - smoothstep(0.1, 1.4, elapsed) if _hand_play else 1.0 - clampf((elapsed - 1.35) / 0.35, 0, 1)
	if is_instance_valid(_held_hand): _held_hand.prevention_animation_active = elapsed < 2.25
	_caption.modulate.a = _played.modulate.a
	var inverse := get_global_transform_with_canvas().affine_inverse()
	# Hold the unsaved cards in their original slots until the saved cards leave.
	# Then gently close the gaps before handing back to the live damage grid.
	var held_ids := _pending.map(func(entry): return entry.card.instance_id)
	for entry in _pending: entry.card.hide()
	for grid in _screen._damage_grids:
		if not is_instance_valid(grid) or grid.source_id != _source_id: continue
		var cards: Array[BattleCard] = grid.card_children()
		var group: Control = grid.get_parent()
		if not _held_groups.any(func(entry): return entry.group == group):
			var natural_height := group.get_combined_minimum_size().y
			_held_groups.append({"group": group, "minimum": group.custom_minimum_size, "natural_height": natural_height, "held_height": natural_height + maxi(0, _original_count - cards.size()) * grid.STRIDE})
		for entry in _held_groups:
			var closing := smoothstep(0, 1, clampf((elapsed - 1.95) / 0.25, 0, 1))
			entry.group.custom_minimum_size.y = lerpf(entry.held_height, entry.natural_height, closing) if elapsed < 2.25 else entry.minimum.y
		if cards.is_empty() or not cards.all(func(card: BattleCard): return card.instance_id in held_ids): continue
		if grid not in _hidden_grids: _hidden_grids.append(grid)
		grid.modulate.a = 1.0 if elapsed >= 2.25 else 0.0
		for live in cards:
			for entry in _pending:
				var card: BattleCard = entry.card
				if live.instance_id != card.instance_id: continue
				var origin: Rect2 = entry.origin
				var destination: Rect2 = inverse * grid.visible_card_rect(live)
				if not destination.has_area(): continue
				var progress := smoothstep(0, 1, clampf((elapsed - 1.95) / 0.25, 0, 1))
				card.position = origin.position.lerp(destination.position, progress)
				card.size = origin.size.lerp(destination.size, progress)
				card.scale = Vector2.ONE
				card.visible = elapsed < 2.25
	for pile in _piles.values():
		var profile: ActorProfile = _screen._actor_profiles.get(pile.target)
		if is_instance_valid(profile) and profile._stat_labels.has(pile.zone):
			pile.rect = inverse * profile.anchor_rect(str(pile.zone))
		var label: Label = pile.label
		label.text = "%d saved\n%s" % [pile.count, str(pile.zone).capitalize()]
		label.size = Vector2(110, 42)
		if is_instance_valid(profile):
			var profile_rect: Rect2 = inverse * profile.get_global_rect()
			label.position = Vector2(pile.rect.get_center().x - 55, profile_rect.end.y + 6)
		label.modulate.a = clampf((elapsed - 1.65) / 0.25, 0, 1)
	for i in _saved.size():
		var entry := _saved[i]
		var card: BattleCard = entry.card
		var origin: Rect2 = entry.origin
		var end: Vector2 = _piles[entry.pile].rect.get_center()
		var delay := 0.15 * float(i) / maxi(1, _saved.size() - 1)
		var progress := smoothstep(0, 1, clampf((elapsed - 0.3 - delay) / 0.85, 0, 1))
		entry.progress = progress
		var center := origin.get_center().lerp(end, progress) + Vector2(60 * sin(PI * progress), -45 * sin(PI * progress))
		card.scale = (origin.size / entry.native_size).lerp(Vector2.ONE * 0.10, progress)
		card.position = center - entry.native_size * card.scale * 0.5
		card.rotation = -0.10 * sin(PI * progress)
		card.modulate = Color.WHITE.lerp(GREEN, clampf((elapsed - 0.15) / 0.35, 0, 0.45))
		card.modulate.a = 1.0 - clampf((progress - 0.8) / 0.2, 0, 1)
	queue_redraw()

func prevention_origin() -> Vector2:
	if _ability_feedback:
		var anchor: Rect2 = _screen.attack_anchor_rect(_source_id)
		if anchor.has_area(): return get_global_transform_with_canvas().affine_inverse() * anchor.get_center()
	return _played.get_transform() * (_played.size * 0.5)

func _draw() -> void:
	if not is_instance_valid(_played): return
	var start := prevention_origin()
	if not _ability_feedback and _elapsed < 1.1:
		var anchor: Rect2 = _screen.attack_anchor_rect(_source_id)
		if anchor.has_area():
			_trail(start, get_global_transform_with_canvas().affine_inverse() * anchor.get_center(), reduction_progress(_elapsed), 1.0 - clampf((_elapsed - 0.8) / 0.3, 0, 1))
	for entry in _saved:
		var origin: Rect2 = entry.origin
		if _elapsed < 1.1:
			_trail(start, origin.get_center(), reduction_progress(_elapsed), 1.0 - clampf((_elapsed - 0.8) / 0.3, 0, 1))
		if _elapsed > 0.25 and _elapsed < 0.95:
			draw_rect(origin.grow(4), Color(GREEN, 0.75), false, 3, true)
		if _elapsed >= 0.3:
			var endpoint: Vector2 = _piles[entry.pile].rect.get_center()
			_trail(origin.get_center(), endpoint, float(entry.progress), (1.0 - clampf((_elapsed - 1.8) / 0.35, 0, 1)) * 0.65)
	for pile in _piles.values():
		if _elapsed < 1.6: continue
		var pulse := 0.55 + 0.3 * sin((_elapsed - 1.6) * PI * 3)
		draw_rect(pile.rect.grow(5), Color(GREEN, pulse), false, 2, true)

func _trail(start: Vector2, end: Vector2, progress: float, opacity: float) -> void:
	if progress <= 0 or opacity <= 0: return
	var points := PackedVector2Array()
	for step in 25:
		var t := progress * step / 24.0
		points.append(start.lerp(end, t) + Vector2(0, -40 * sin(PI * t)))
	draw_polyline(points, Color(GREEN, opacity * 0.18), 9, true)
	draw_polyline(points, Color(GREEN, opacity), 2, true)
	draw_circle(points[-1], 5, Color(GREEN, opacity))
