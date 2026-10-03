extends Control
## One stable hover region owns the fan. Card transforms never move that region.
## Slots own the fan motion, leaving card-local draw/glow animations independent.
const CARD_SIZE := BattleCard.STANDARD_SIZE
var cards: Array[BattleCard] = []
var slots: Array[Control] = []
var base_transforms: Array[Transform2D] = []
var reveal := 0.0
var held_open := false
var keep_visible := false
var hovered := -1
var _pressed := -1
var _keyboard := false
var _pointer := Vector2.INF

func _ready() -> void:
	clip_contents = true
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	get_window().mouse_exited.connect(func(): _pointer = Vector2(-10000, -10000); _keyboard = false)
	get_window().mouse_entered.connect(func(): _pointer = Vector2.INF)
	resized.connect(_layout)

func add_card(card: BattleCard) -> void:
	var slot := Control.new(); slot.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(slot); slot.size = CARD_SIZE; slot.pivot_offset = Vector2(CARD_SIZE.x * 0.5, CARD_SIZE.y)
	slot.add_child(card); card.size = CARD_SIZE
	# Route pointer selection through stable fan geometry, so raising a card
	# cannot steal a neighbouring card's exposed title or cause hover flicker.
	card.mouse_filter = Control.MOUSE_FILTER_IGNORE
	cards.append(card); slots.append(slot)
	card.focus_entered.connect(func():
		if _keyboard: hovered = cards.find(card))
	_layout()

func _input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed: _keyboard = true
	elif event is InputEventMouseMotion:
		_keyboard = false
		_pointer = event.position

func _process(delta: float) -> void:
	var pointer := get_local_mouse_position() if _pointer == Vector2.INF else get_global_transform_with_canvas().affine_inverse() * _pointer
	var focused := cards.any(func(card): return card.has_focus())
	var open := not cards.is_empty() and (Rect2(Vector2.ZERO, size).has_point(pointer) or held_open or keep_visible or (_keyboard and focused))
	reveal = move_toward(reveal, 1.0 if open else 0.0, delta / (0.24 if open else 0.30))
	if not _keyboard: hovered = card_at(pointer) if open and reveal > 0.85 else -1
	tooltip_text = cards[hovered].tooltip_text if hovered >= 0 else ""
	_layout()

func card_at(point: Vector2) -> int:
	if not Rect2(Vector2.ZERO, size).has_point(point): return -1
	for i in range(base_transforms.size() - 1, -1, -1):
		if Rect2(Vector2.ZERO, CARD_SIZE).has_point(base_transforms[i].affine_inverse() * point): return i
	if hovered >= 0 and hovered < slots.size() and Rect2(Vector2.ZERO, CARD_SIZE).has_point(slots[hovered].get_transform().affine_inverse() * point): return hovered
	return -1

func _layout() -> void:
	base_transforms.clear()
	var count := cards.size()
	mouse_filter = Control.MOUSE_FILTER_STOP if count > 0 else Control.MOUSE_FILTER_IGNORE
	if count == 0: return
	var step := minf(158, (size.x - CARD_SIZE.x - 66) / maxf(1, count - 1))
	var left := (size.x - CARD_SIZE.x - step * (count - 1)) * 0.5
	var eased := reveal * reveal * (3.0 - 2.0 * reveal)
	for i in count:
		var t := (float(i) / (count - 1) * 2 - 1) if count > 1 else 0.0
		var slot := slots[i]
		slot.rotation = deg_to_rad(t * 7.0)
		slot.position = Vector2(left + i * step, 17 + t * t * 12 + (1 - eased) * (size.y - 92))
		base_transforms.append(slot.get_transform())
		if i == hovered and reveal > 0.85:
			slot.rotation = 0
			slot.position.y -= 16
			slot.z_index = 2
		else: slot.z_index = 1 if cards[i].button_pressed else 0
		# Selection stays obvious while targeting something outside the hand.
		slots[i].modulate = Color("fff0bd") if cards[i].button_pressed else Color.WHITE

func _gui_input(event: InputEvent) -> void:
	if not event is InputEventMouseButton or event.button_index != MOUSE_BUTTON_LEFT: return
	var hit := card_at(event.position)
	if event.pressed:
		_pressed = hit if reveal > 0.85 else -1
	elif hit >= 0 and hit == _pressed:
		_pressed = -1
		var card := cards[hit]
		if not card.disabled:
			card.grab_focus()
			if card.toggle_mode: card.button_pressed = not card.button_pressed
			card.pressed.emit()
	accept_event()

func _make_custom_tooltip(for_text: String) -> Object:
	var card: Control = cards[hovered] if hovered >= 0 and hovered < cards.size() else self
	return preload("res://presentation/cards/card_rules_tooltip.gd").create(self, for_text, card)
