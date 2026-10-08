extends Control
## A source's cards retain their identities/order. Only their headers overlap;
## the readable hover copy never captures the pointer away from those headers.
const TIMING := preload("res://presentation/battle/combat_timing.gd")
const STRIDE := 24.0
const ORIGIN_GUTTER := 26.0
const ORIGIN_ICON_SIZE := 18.0
const ICONS := preload("res://presentation/battle/battle_icons.gd")
var started_ms := 0
var removal_started_ms := 0
var screen: Control
var target_actor := ""
var source_id := ""
var fold_key := ""
var dock: ScrollContainer
var hovered_id := ""
var _hover_card: BattleCard
var _tear: Control
var _pointer := Vector2(-99999, -99999)

func _ready() -> void:
	clip_contents = true
	mouse_filter = Control.MOUSE_FILTER_PASS
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	child_order_changed.connect(_layout_cards, CONNECT_DEFERRED)
	resized.connect(_layout_cards)
	mouse_exited.connect(func(): _pointer = Vector2(-99999, -99999); _clear_hover())
func card_children() -> Array[BattleCard]:
	# Native tooltips can attach a PopupPanel beneath this control. Only actual
	# cards participate in ordering, reveal timing, hover, and damage feedback.
	var cards: Array[BattleCard] = []
	for child in get_children():
		if child is BattleCard: cards.append(child)
	return cards

func _layout_cards() -> void:
	var cards := card_children()
	custom_minimum_size.y = cards.size() * STRIDE
	for i in cards.size():
		var card := cards[i]
		card.size = Vector2(maxf(106, (dock.column_width if is_instance_valid(dock) else maxf(132, size.x)) - ORIGIN_GUTTER), 248)
		var title: Label = card.get_node("CardTitle")
		title.autowrap_mode = TextServer.AUTOWRAP_OFF; title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
		title.offset_left = 29; title.offset_top = 0; title.offset_bottom = STRIDE
		var energy: Label = card.get_node("EnergyCost")
		energy.offset_left = 4; energy.offset_right = 24; energy.offset_top = 2; energy.offset_bottom = STRIDE - 2
		energy.add_theme_font_size_override("font_size", 14)
		card.position = Vector2(ORIGIN_GUTTER, i * STRIDE)
		card.scale = Vector2.ONE
		card.mouse_filter = Control.MOUSE_FILTER_IGNORE
func _process(_delta: float) -> void:
	refresh_playback()
	_update_hover()
func visible_card_rect(card: Control) -> Rect2:
	var rect := card.get_global_rect()
	rect.size.y = STRIDE * get_global_transform_with_canvas().get_scale().y
	if is_instance_valid(dock): rect = rect.intersection(dock.get_global_rect())
	return rect
func feedback_card_rect(card: Control) -> Rect2:
	var rect := card.get_global_rect()
	rect.size.y = STRIDE * get_global_transform_with_canvas().get_scale().y
	# Animation copies always retain a whole header. Intersecting with the
	# scroll viewport can leave only a few pixels and squash the text when that
	# clipped rectangle is later used as the flight's scale.
	if is_instance_valid(dock):
		var bounds := dock.get_global_rect()
		rect.position.y = clampf(rect.position.y, bounds.position.y, maxf(bounds.position.y, bounds.end.y - rect.size.y))
	return rect

func _gui_input(event: InputEvent) -> void:
	if event is InputEventMouseMotion:
		_pointer = get_global_transform_with_canvas() * event.position
		_update_hover()

func _update_hover() -> void:
	tooltip_text = ""
	var wanted: BattleCard
	if removal_started_ms == 0 and is_instance_valid(screen) and not screen._snapshot_panel_open:
		var mouse := _pointer
		for card in card_children():
			var icon_rect := Rect2(get_global_transform_with_canvas() * origin_icon_rect(card).position, origin_icon_rect(card).size * get_global_transform_with_canvas().get_scale())
			if card.modulate.a > 0.95 and icon_rect.has_point(mouse):
				tooltip_text = "Pulled from " + {"deck": "draw pile", "discard": "discard pile", "hand": "hand"}.get(str(card.get_meta("removal_origin_zone", "")), "unknown pile")
			if card.modulate.a > 0.95 and visible_card_rect(card).has_point(mouse): wanted = card; break
	if wanted == null:
		_clear_hover(); return
	if hovered_id == wanted.instance_id:
		_position_hover(wanted); return
	_clear_hover()
	hovered_id = wanted.instance_id
	_hover_card = BattleCard.new(); screen._root.add_child(_hover_card)
	_hover_card.configure(wanted.instance_id, wanted.definition_id, false)
	_hover_card.mouse_filter = Control.MOUSE_FILTER_IGNORE; _hover_card.z_index = 70
	_hover_card.size = BattleCard.STANDARD_SIZE
	_position_hover(wanted)

func _position_hover(wanted: Control) -> void:
	var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
	var origin: Rect2 = inverse * visible_card_rect(wanted)
	var list_rect: Rect2 = inverse * (dock.get_global_rect() if is_instance_valid(dock) else get_global_rect())
	var bounds := Rect2(Vector2.ZERO, screen._root.size).grow(-16)
	var preview_size := _hover_card.size
	# Prefer the right of the entire list, never the hovered row's origin.
	# At the right screen edge, the left side keeps the full preview readable.
	var x := list_rect.end.x + 12
	if x + preview_size.x > bounds.end.x: x = list_rect.position.x - 12 - preview_size.x
	x = clampf(x, bounds.position.x, bounds.end.x - preview_size.x)
	var rect := Rect2(Vector2(x, clampf(origin.position.y, bounds.position.y, bounds.end.y - preview_size.y)), preview_size)
	# In crowded encounters the adjacent lane can contain another card list.
	# Lift the preview above those rows instead of obscuring another list.
	for _pass in screen._damage_grids.size():
		for grid in screen._damage_grids:
			if not grid.is_visible_in_tree(): continue # Folded lists occupy nothing.
			var occupied: Rect2 = grid.get_global_rect()
			if is_instance_valid(grid.dock): occupied = occupied.intersection(grid.dock.get_global_rect())
			occupied = inverse * occupied
			if occupied.has_area() and rect.intersects(occupied):
				rect.position.y = maxf(bounds.position.y, occupied.position.y - 12 - preview_size.y)
	_hover_card.position = rect.position
func _clear_hover() -> void:
	hovered_id = ""
	if is_instance_valid(_hover_card): _hover_card.queue_free()
	_hover_card = null
func _exit_tree() -> void:
	_clear_hover()
	if is_instance_valid(_tear): _tear.queue_free()
func refresh_playback() -> void:
	var elapsed := maxf(0.0, (Time.get_ticks_msec() - started_ms) / 1000.0)
	var tearing := removal_started_ms > 0 and Time.get_ticks_msec() >= removal_started_ms
	# A folded list has no card headers to tear; its counters still settle.
	if tearing and is_instance_valid(screen) and not is_instance_valid(_tear) and is_visible_in_tree():
		_clear_hover()
		_tear = preload("res://presentation/battle/damage_stack_tear.gd").new()
		screen._root.add_child(_tear); _tear.configure(self, screen, target_actor)
	var cards := card_children()
	for i in cards.size():
		var card := cards[i]
		var delay := 0.25 * float(i) / maxi(1, cards.size() - 1)
		var progress := clampf((elapsed - delay) / maxf(0.01, TIMING.reveal()), 0.0, 1.0)
		card.modulate.a = 0.0 if tearing else progress
	queue_redraw()
	if is_instance_valid(_tear): _tear.visible = is_visible_in_tree()
	if is_instance_valid(_tear): _tear.present_progress(clampf((Time.get_ticks_msec() - removal_started_ms) / (maxf(0.01, TIMING.removal()) * 1000.0), 0, 1))

func origin_icon_rect(card: Control) -> Rect2:
	return Rect2(Vector2(3, card.position.y + (STRIDE - ORIGIN_ICON_SIZE) * 0.5), Vector2.ONE * ORIGIN_ICON_SIZE)

func _draw() -> void:
	for card in card_children():
		var zone := str(card.get_meta("removal_origin_zone", ""))
		if zone not in ["deck", "discard", "hand"]: continue
		draw_texture_rect(ICONS.texture(zone), origin_icon_rect(card), false, Color(1, 1, 1, card.modulate.a))
