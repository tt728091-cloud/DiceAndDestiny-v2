extends Control

# Keep every pending card visible. Wrap at the normal compact card size first,
# then uniformly scale the whole card only when the available height requires it.
const CARD_SIZE := Vector2(132, 144)
const GAP := 8.0

func _ready() -> void:
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	size_flags_vertical = Control.SIZE_EXPAND_FILL
	resized.connect(_layout_cards)
	child_order_changed.connect(_layout_cards, CONNECT_DEFERRED)

func _layout_cards() -> void:
	var count := get_child_count()
	if count == 0 or size.x <= 0 or size.y <= 0: return
	var columns := 1
	var best_scale := 0.0
	var best_rows := count
	for candidate in range(1, count + 1):
		var rows := ceili(float(count) / candidate)
		var fit := minf(1.0, minf(size.x / (candidate * CARD_SIZE.x + (candidate - 1) * GAP), size.y / (rows * CARD_SIZE.y + (rows - 1) * GAP)))
		if fit > best_scale or (is_equal_approx(fit, best_scale) and rows < best_rows):
			best_scale = fit
			columns = candidate
			best_rows = rows
	for index in count:
		var card: Control = get_child(index)
		card.size = CARD_SIZE
		card.scale = Vector2.ONE * best_scale
		card.position = Vector2(index % columns, index / columns) * (CARD_SIZE + Vector2.ONE * GAP) * best_scale
