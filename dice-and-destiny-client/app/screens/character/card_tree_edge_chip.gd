extends Button
## The cost and deck rules of one connection, centred on its line.
const STYLE = preload("res://app/screens/character/character_style.gd")
var edge_id := ""
var cost := 0
## Each rule: {art: Texture2D, forbidden: bool}
var rules: Array = []
var selected := false
var dim := false
var hovered := false

func setup(id: String) -> void:
	edge_id = id; flat = true; focus_mode = Control.FOCUS_NONE
	for s in ["normal", "hover", "pressed", "disabled", "focus", "hover_pressed"]: add_theme_stylebox_override(s, StyleBoxEmpty.new())
	mouse_entered.connect(func(): hovered = true; queue_redraw())
	mouse_exited.connect(func(): hovered = false; queue_redraw())
	set_meta("tree_edge", id)

func _label() -> String:
	return "±0 XP" if cost == 0 else ("+%d XP" % cost if cost > 0 else "−%d XP" % absi(cost))

func refresh() -> void:
	var w := get_theme_default_font().get_string_size(_label(), HORIZONTAL_ALIGNMENT_LEFT, -1, 20).x
	var shown := mini(3, rules.size())
	size = Vector2(w + 30 + shown * 34 + (30 if rules.size() > 3 else 0), 40)
	custom_minimum_size = size
	queue_redraw()

func _get_tooltip(_at: Vector2) -> String:
	return preload("res://presentation/battle/wrapped_tooltip.gd").content(tooltip_text)

func _make_custom_tooltip(for_text: String) -> Object:
	return preload("res://presentation/battle/wrapped_tooltip.gd").create(self, for_text)

func _draw() -> void:
	var font := get_theme_default_font()
	var color := STYLE.GOLD if cost > 0 else STYLE.AMBER if cost < 0 else STYLE.MUTED
	if dim: color = color.darkened(0.45)
	var border := STYLE.GOLD_BRIGHT if selected else Color(color, 0.85 if hovered else 0.55)
	draw_style_box(STYLE.box(Color(0.03, 0.05, 0.08, 0.92), border, 0, 20, 2 if selected or hovered else 1), Rect2(Vector2.ZERO, size))
	draw_string(font, Vector2(15, 20 + font.get_ascent(20) * 0.5 - 3), _label(), HORIZONTAL_ALIGNMENT_LEFT, -1, 20, color)
	var x := size.x - 15 - mini(3, rules.size()) * 34 - (30 if rules.size() > 3 else 0) + 17
	for i in mini(3, rules.size()):
		var rule: Dictionary = rules[i]
		var at := Vector2(x + i * 34, 20)
		var ring := STYLE.LOSS if rule.forbidden else STYLE.GAIN
		draw_circle(at, 14, Color("12161d"))
		if rule.art != null:
			var points := PackedVector2Array(); var uvs := PackedVector2Array(); var colors := PackedColorArray()
			for k in 24:
				var d := Vector2(cos(TAU * k / 24.0), sin(TAU * k / 24.0))
				points.append(at + d * 13); uvs.append(Vector2(0.5, 0.4) + d * Vector2(0.5, 0.33)); colors.append(Color.WHITE if not rule.forbidden else Color(0.7, 0.6, 0.6))
			draw_polygon(points, colors, uvs, rule.art)
		draw_arc(at, 14, 0, TAU, 24, ring, 2, true)
		if rule.forbidden: draw_line(at + Vector2(-9, 9), at + Vector2(9, -9), STYLE.LOSS, 3, true)
	if rules.size() > 3:
		draw_string(font, Vector2(size.x - 38, 20 + font.get_ascent(16) * 0.5 - 2), "+%d" % (rules.size() - 3), HORIZONTAL_ALIGNMENT_LEFT, -1, 16, STYLE.MUTED)
