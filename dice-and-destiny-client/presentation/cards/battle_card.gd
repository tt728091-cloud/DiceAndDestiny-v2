class_name BattleCard
extends Button

var instance_id := ""
var definition_id := ""
var _income_glow: Panel

func configure(instance: String, definition: String, enabled: bool, pending_removal: bool = false) -> void:
	instance_id = instance; definition_id = definition
	var data := BattlePresentationCatalog.card(definition)
	text = "%s  %d✦%s" % [data.name, int(data.cost), "\n⚔ PENDING" if pending_removal else ""]
	clip_text = true
	tooltip_text = "%s (%s) — %s" % [data.name, instance, data.text]
	custom_minimum_size = Vector2(150, 105)
	add_theme_font_size_override("font_size", 12)
	var art_path := str(data.get("art", ""))
	if not art_path.is_empty() and ResourceLoader.exists(art_path):
		icon = load(art_path)
		expand_icon = true
		add_theme_constant_override("icon_max_width", 42)
	disabled = not enabled
	if pending_removal: modulate = Color("ff8a70")

func prepare_income_draw() -> void:
	modulate = Color(1, 1, 1, 0)
	scale = Vector2(0.94, 0.94)
	_income_glow = Panel.new()
	_income_glow.set_anchors_preset(Control.PRESET_FULL_RECT)
	_income_glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var glow_style := StyleBoxFlat.new()
	glow_style.bg_color = Color("ffe28a20")
	glow_style.border_color = Color("fff0a8ff")
	glow_style.set_border_width_all(3)
	glow_style.set_corner_radius_all(7)
	glow_style.shadow_color = Color("ffd36a99")
	glow_style.shadow_size = 12
	_income_glow.add_theme_stylebox_override("panel", glow_style)
	add_child(_income_glow)

func animate_income_draw(duration_seconds: float) -> void:
	var duration := maxf(0.05, duration_seconds)
	pivot_offset = size * 0.5
	var settled_position := position
	position.y -= 18.0
	var tween := create_tween().bind_node(self).set_parallel(true)
	tween.tween_property(self, "modulate", Color.WHITE, duration * 0.22).set_delay(duration * 0.18).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "position", settled_position, duration * 0.28).set_delay(duration * 0.18).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", Vector2.ONE, duration * 0.28).set_delay(duration * 0.18).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	if is_instance_valid(_income_glow):
		_income_glow.modulate = Color.WHITE
		tween.tween_property(_income_glow, "modulate:a", 0.0, duration * 0.42).set_delay(duration * 0.48).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_IN)
