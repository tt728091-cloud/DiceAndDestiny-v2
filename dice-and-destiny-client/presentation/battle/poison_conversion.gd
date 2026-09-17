extends VBoxContainer

signal finished
const DURATION := 2.2
var _poison: Label
var _volatile: Label

func configure(actor_name: String, data: Dictionary) -> void:
	alignment = BoxContainer.ALIGNMENT_CENTER
	add_theme_constant_override("separation", 18)
	var title := Label.new(); title.text = "Poison upgraded"; title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.add_theme_font_size_override("font_size", 30); add_child(title)
	var owner := Label.new(); owner.text = actor_name; owner.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; add_child(owner)
	var transition := CenterContainer.new(); add_child(transition)
	var icons := Control.new(); icons.custom_minimum_size = Vector2(160, 110); transition.add_child(icons)
	_poison = Label.new(); _poison.text = BattlePresentationCatalog.status("poison").glyph; _poison.add_theme_font_size_override("font_size", 84); _poison.add_theme_color_override("font_color", Color("b6e17a")); _poison.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _poison.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); icons.add_child(_poison)
	_volatile = Label.new(); _volatile.text = BattlePresentationCatalog.status("volatile_poison").glyph; _volatile.add_theme_font_size_override("font_size", 84); _volatile.add_theme_color_override("font_color", Color("ce9aff")); _volatile.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _volatile.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); icons.add_child(_volatile)
	var change := Label.new(); change.text = "1 Poison  →  1 Volatile Poison"; change.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; change.add_theme_font_size_override("font_size", 24); change.add_theme_color_override("font_color", Color("dab9ff")); add_child(change)
	var counts := Label.new(); counts.text = "Poison %d → %d     ·     Volatile Poison %d → %d" % [int(data.poison_before), int(data.poison_after), int(data.volatile_before), int(data.volatile_after)]; counts.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; add_child(counts)
	tooltip_text = actor_name + ": " + counts.text

func animate() -> void:
	_volatile.modulate.a = 0.0
	var tween := create_tween()
	tween.tween_interval(0.35)
	tween.tween_property(_poison, "modulate:a", 0.0, 0.65)
	tween.parallel().tween_property(_volatile, "modulate:a", 1.0, 0.65)
	tween.tween_interval(DURATION - 1.0)
	tween.tween_callback(func(): finished.emit())
