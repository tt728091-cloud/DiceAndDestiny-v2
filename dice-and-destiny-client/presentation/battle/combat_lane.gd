extends Control
var body: VBoxContainer
var _measured_size := Vector2.ZERO

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	size_flags_vertical = Control.SIZE_EXPAND_FILL
	body = VBoxContainer.new(); body.mouse_filter = Control.MOUSE_FILTER_IGNORE; body.add_theme_constant_override("separation", 12); add_child(body)
	resized.connect(_layout_body)
	_layout_body()

func _layout_body() -> void:
	# A handoff can rebuild this lane after processing has already run. Set
	# its body's width during Container layout, before that same frame draws;
	# waiting for _process exposes the children's narrow minimum widths.
	if is_instance_valid(body): body.size = size / body.scale

func _process(_delta: float) -> void:
	if not is_instance_valid(body) or size.x <= 0 or size.y <= 0: return
	# Wrapped labels initially report their height at a provisional width.
	# Give the containers a layout pass at the actual lane width before using
	# that height to fit the content. Otherwise each rebuild briefly shrinks
	# the entire lane (including its text), then expands it again next frame.
	if not size.is_equal_approx(_measured_size):
		_measured_size = size
		_layout_body()
		return
	var fit := minf(1.0, size.y / maxf(1.0, body.get_combined_minimum_size().y))
	body.scale = Vector2.ONE * fit
	body.size = size / fit

func restore_layout(previous: Dictionary) -> void:
	# A same-phase redraw retains the settled fit before its first draw. This
	# matters when a compact results area requires the lane to scale down.
	size = previous.size; body.scale = previous.scale; _measured_size = size
	_layout_body()
