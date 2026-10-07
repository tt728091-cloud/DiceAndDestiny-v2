extends Label
## Keep Godot's hover lifetime, but anchor its popup outside the displayed cards.
var anchor: Control
var subject: Control
const GAP := 12.0

static func create(owner: Control, value: String, card: Control = null) -> Label:
	var label := preload("res://presentation/battle/wrapped_tooltip.gd").create(owner, value)
	if label == null: return null
	label.set_script(load("res://presentation/cards/card_rules_tooltip.gd"))
	label.anchor = owner
	label.subject = card if card != null else owner
	return label

func _process(_delta: float) -> void:
	if not is_instance_valid(anchor) or not is_instance_valid(subject): return
	var popup := get_window()
	if not popup is PopupPanel: return
	var avoid := anchor.get_global_transform_with_canvas() * Rect2(Vector2.ZERO, anchor.size)
	var focus := subject.get_global_transform_with_canvas() * Rect2(Vector2.ZERO, subject.size)
	var bounds := anchor.get_viewport_rect()
	if not popup.is_embedded():
		var screen_transform := anchor.get_viewport().get_screen_transform()
		avoid = screen_transform * avoid
		focus = screen_transform * focus
		bounds = screen_transform * bounds
	popup.position = Vector2i(place(avoid, focus, Vector2(popup.size), bounds))

static func place(avoid: Rect2, focus: Rect2, extent: Vector2, viewport: Rect2) -> Vector2:
	var bounds := viewport.grow(-GAP)
	var x := clampf(focus.get_center().x - extent.x * 0.5, bounds.position.x, maxf(bounds.position.x, bounds.end.x - extent.x))
	var y := clampf(focus.get_center().y - extent.y * 0.5, bounds.position.y, maxf(bounds.position.y, bounds.end.y - extent.y))
	# Prefer above the whole hand, then either side, then below for top-edge cards.
	var candidates := [Vector2(x, avoid.position.y - extent.y - GAP), Vector2(avoid.end.x + GAP, y), Vector2(avoid.position.x - extent.x - GAP, y), Vector2(x, avoid.end.y + GAP)]
	for point in candidates:
		if bounds.encloses(Rect2(point, extent)): return point
	# Extremely small windows retain readable text within the viewport.
	return Vector2(x, clampf(avoid.position.y - extent.y - GAP, bounds.position.y, maxf(bounds.position.y, bounds.end.y - extent.y)))
