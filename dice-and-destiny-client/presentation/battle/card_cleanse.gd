extends PanelContainer

signal finished

const DURATION := 2.8
var _tokens: Array[Label] = []
var _card: PanelContainer

func configure(actor_name: String, card_data: Dictionary, status_data: Dictionary, before: int, after: int) -> void:
	custom_minimum_size = Vector2(560, 330)
	var frame := StyleBoxFlat.new()
	frame.bg_color = Color("101e25f5")
	frame.border_color = Color("68c8b8")
	frame.set_border_width_all(2)
	frame.set_corner_radius_all(14)
	frame.content_margin_left = 24; frame.content_margin_right = 24
	frame.content_margin_top = 20; frame.content_margin_bottom = 20
	add_theme_stylebox_override("panel", frame)
	var layout := VBoxContainer.new(); layout.add_theme_constant_override("separation", 18); add_child(layout)
	var title := Label.new(); title.text = "%s played %s" % [actor_name, card_data.name]
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	title.add_theme_font_size_override("font_size", 24); layout.add_child(title)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; row.add_theme_constant_override("separation", 28); layout.add_child(row)
	_card = PanelContainer.new(); _card.custom_minimum_size = Vector2(180, 225); row.add_child(_card)
	var card_style := frame.duplicate() as StyleBoxFlat; card_style.bg_color = Color("203b3f"); card_style.set_border_width_all(1); _card.add_theme_stylebox_override("panel", card_style)
	var card_layout := VBoxContainer.new(); card_layout.alignment = BoxContainer.ALIGNMENT_CENTER; _card.add_child(card_layout)
	var played := Label.new(); played.text = "PLAYED"; played.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; played.add_theme_color_override("font_color", Color("8ce5c6")); card_layout.add_child(played)
	var art_path := str(card_data.get("art", ""))
	if not art_path.is_empty() and ResourceLoader.exists(art_path):
		var art := TextureRect.new(); art.texture = load(art_path); art.custom_minimum_size = Vector2(110, 120); art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; card_layout.add_child(art)
	var name_label := Label.new(); name_label.text = card_data.name; name_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; name_label.add_theme_font_size_override("font_size", 21); card_layout.add_child(name_label)
	var effect := VBoxContainer.new(); effect.alignment = BoxContainer.ALIGNMENT_CENTER; effect.add_theme_constant_override("separation", 12); row.add_child(effect)
	var cleared := Label.new(); cleared.text = "STATUS CLEARED" if after == 0 else "STATUS REDUCED"; cleared.add_theme_color_override("font_color", Color("8ce5c6")); effect.add_child(cleared)
	var counts := Label.new(); counts.text = "%s  %d → %d" % [status_data.name, before, after]; counts.add_theme_font_size_override("font_size", 28); effect.add_child(counts)
	var tokens := HBoxContainer.new(); tokens.add_theme_constant_override("separation", 12); effect.add_child(tokens)
	for index in mini(before - after, 8):
		var token := Label.new(); token.text = str(status_data.glyph); token.add_theme_font_size_override("font_size", 40); token.add_theme_color_override("font_color", Color("afe079")); tokens.add_child(token); _tokens.append(token)
	var cause := Label.new(); cause.text = "%s removed %d stack%s" % [card_data.name, before - after, "s" if before - after != 1 else ""]; effect.add_child(cause)
	tooltip_text = title.text + ": " + counts.text

func animate() -> void:
	_card.modulate.a = 0.0
	var reveal := create_tween()
	reveal.tween_property(_card, "modulate:a", 1.0, 0.25)
	for index in _tokens.size():
		var token := _tokens[index]
		var tween := token.create_tween()
		tween.tween_interval(0.85 + index * 0.12)
		tween.tween_property(token, "modulate:a", 0.0, 0.75)
	var timer := create_tween()
	timer.tween_interval(DURATION)
	timer.tween_callback(func(): finished.emit())
