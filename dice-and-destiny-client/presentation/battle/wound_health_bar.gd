extends Control
## One muted, hoverable segment per committed hit. Oldest wounds stay at the
## right edge; later wounds grow leftward as the displayed health falls.
const COLORS := [Color("50565d"), Color("666268"), Color("535f61"), Color("68665c")]
const TIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
var health: ProgressBar
var segments: Array[Button] = []
var _unrecorded := 0

func attach(bar: ProgressBar) -> void:
	name = "WoundSegments"
	health = bar; mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	health.resized.connect(_layout)
	health.value_changed.connect(func(_value): _layout())

func display(wounds: Array, missing_health: int) -> void:
	for segment in segments: remove_child(segment); segment.queue_free()
	segments.clear()
	var recorded := 0
	for wound in wounds:
		var cards: Array = wound.get("cards", [])
		if cards.is_empty(): continue
		var button := TIP_BUTTON.new(); button.name = "Wound_%d" % segments.size()
		button.set_meta("wound_id", wound.get("id", "")); button.set_meta("damage", cards.size())
		button.focus_mode = Control.FOCUS_NONE; button.mouse_default_cursor_shape = Control.CURSOR_ARROW
		var style := StyleBoxFlat.new(); style.bg_color = COLORS[segments.size() % COLORS.size()]
		style.border_color = Color("303337"); style.border_width_left = 1
		for state in ["normal", "hover", "pressed", "focus"]: button.add_theme_stylebox_override(state, style)
		button.tooltip_text = _tooltip(wound, segments.size() + 1)
		add_child(button); segments.append(button); recorded += cards.size()
	_unrecorded = maxi(0, missing_health - recorded)
	_layout()

func _layout() -> void:
	if not is_instance_valid(health): return
	var maximum := maxf(1, health.max_value)
	var inner := Rect2(5, 4, maxf(0, health.size.x - 10), maxf(0, health.size.y - 8))
	var used := _unrecorded
	var missing := maximum - health.value
	for segment in segments:
		var damage := int(segment.get_meta("damage"))
		var right := inner.end.x - inner.size.x * used / maximum
		used += damage
		var left := maxf(inner.position.x, inner.end.x - inner.size.x * used / maximum)
		segment.position = Vector2(left, inner.position.y)
		segment.size = Vector2(maxf(0, right - left), inner.size.y)
		# A future queued damage/effects beat may already exist in the snapshot.
		# Reveal its wound only when the displayed health has taken that hit.
		segment.visible = used <= missing and segment.size.x > 0

func _tooltip(wound: Dictionary, number: int) -> String:
	var cards: Array = wound.get("cards", [])
	var source_id := str(wound.get("source_content_id", ""))
	var source_name := "Damage"
	for kind in ["abilities", "cards", "statuses"]:
		var definition := BattlePresentationCatalog.definition(kind, source_id)
		if not definition.is_empty(): source_name = str(definition.get("name", source_name)); break
	var lines: Array[String] = ["Wound %d · %d damage" % [number, cards.size()], "Round %d · %s" % [int(wound.get("round", 1)), source_name], "Cards lost:"]
	var counts := {}
	for card in cards:
		var id := str(card.get("card_definition_id", ""))
		var label := str(BattlePresentationCatalog.card(id).name) if not id.is_empty() else "Card details unavailable"
		var zone := str(card.get("original_zone", ""))
		label += " (" + str({"deck": "draw pile", "hand": "hand", "discard": "discard pile"}.get(zone, zone)) + ")" if not zone.is_empty() else ""
		counts[label] = int(counts.get(label, 0)) + 1
	for label in counts: lines.append("%s%s" % [str(counts[label]) + " × " if int(counts[label]) > 1 else "", label])
	return "\n".join(lines)
