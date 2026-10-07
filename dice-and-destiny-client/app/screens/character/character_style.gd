extends RefCounted
## Shared palette and controls for Character Creation, its workshops and the
## card tree. Navy panels with gold accents, matching the battle presentation.
const GOLD := Color("e6c17c")
const GOLD_BRIGHT := Color("ffe2a0")
const BRONZE := Color("7a6440")
const IVORY := Color("e9e6de")
const MUTED := Color("9cabb7")
const DIM := Color("5d6b77")
const GAIN := Color("8fe2af")
const LOSS := Color("ef9e91")
const AMBER := Color("e0a96d")
const ENERGY := Color("5aa9e6")
const HEALTH := Color("f1ad9f")
const OFFENSE := Color("e0806d")
const DEFENSE := Color("6fb0e6")
const PANEL := Color("101b25")
const RAISED := Color("14222d")
const FIELD := Color("0d1720")
const NIGHT := Color("060a12")
const BACKDROP := Color("080d14")

static func box(fill: Variant, border: Variant = "22323f", padding: int = 10, radius: int = 8, width: int = 1) -> StyleBoxFlat:
	var s := StyleBoxFlat.new(); s.bg_color = Color(fill); s.border_color = Color(border)
	s.set_border_width_all(width); s.set_corner_radius_all(radius)
	s.content_margin_left = padding; s.content_margin_right = padding
	s.content_margin_top = maxi(4, padding - 4); s.content_margin_bottom = maxi(4, padding - 4)
	return s

static func theme() -> Theme:
	var t := Theme.new(); t.default_font_size = 17
	for kind in ["Label", "Button", "CheckBox", "CheckButton", "OptionButton", "LineEdit", "MenuButton", "PopupMenu", "TabBar"]:
		t.set_color("font_color", kind, IVORY)
	for kind in ["Button", "OptionButton", "MenuButton"]:
		t.set_stylebox("normal", kind, box(RAISED, "2b4150"))
		t.set_stylebox("hover", kind, box("253a46", "b99a60"))
		t.set_stylebox("pressed", kind, box("30434a", GOLD))
		t.set_stylebox("hover_pressed", kind, box("30434a", GOLD_BRIGHT))
		t.set_stylebox("disabled", kind, box("0f1921", "2a3945"))
		t.set_stylebox("focus", kind, box("00000000", GOLD, 10, 8, 1))
		t.set_color("font_hover_color", kind, Color("fff0bf"))
		t.set_color("font_pressed_color", kind, Color("fff0bf"))
		t.set_color("font_hover_pressed_color", kind, Color("fff0bf"))
		t.set_color("font_focus_color", kind, IVORY)
		t.set_color("font_disabled_color", kind, Color("7d8b97"))
	for kind in ["CheckBox", "CheckButton"]:
		for state in ["normal", "hover", "pressed", "hover_pressed", "disabled"]: t.set_stylebox(state, kind, box("00000000", "00000000", 4, 6, 0))
		t.set_stylebox("focus", kind, box("00000000", Color(GOLD, 0.6), 4, 6, 1))
		t.set_color("font_hover_color", kind, GOLD_BRIGHT)
		t.set_color("font_pressed_color", kind, IVORY)
		t.set_color("font_hover_pressed_color", kind, GOLD_BRIGHT)
		t.set_color("font_disabled_color", kind, DIM)
	t.set_icon("checked", "CheckBox", check_icon(true, GOLD))
	t.set_icon("unchecked", "CheckBox", check_icon(false, MUTED))
	t.set_icon("checked_disabled", "CheckBox", check_icon(true, DIM))
	t.set_icon("unchecked_disabled", "CheckBox", check_icon(false, Color("34424e")))
	t.set_constant("h_separation", "CheckBox", 8)
	# Spin boxes: themed stepper buttons beside the field.
	for part in ["up", "down"]:
		t.set_stylebox(part + "_background", "SpinBox", box(RAISED, "2b4150", 2, 4))
		t.set_stylebox(part + "_background_hovered", "SpinBox", box("253a46", "b99a60", 2, 4))
		t.set_stylebox(part + "_background_pressed", "SpinBox", box("30434a", GOLD, 2, 4))
		t.set_stylebox(part + "_background_disabled", "SpinBox", box("0e1820", "1d2a35", 2, 4))
		t.set_color(part + "_icon_modulate", "SpinBox", Color.WHITE)
		t.set_color(part + "_hover_icon_modulate", "SpinBox", Color(1.2, 1.2, 1.1))
		t.set_color(part + "_pressed_icon_modulate", "SpinBox", Color(1.2, 1.2, 1.1))
		t.set_color(part + "_disabled_icon_modulate", "SpinBox", DIM)
	t.set_icon("updown", "SpinBox", stepper_icon(true, true))
	t.set_icon("up", "SpinBox", stepper_icon(true, false))
	t.set_icon("down", "SpinBox", stepper_icon(false, true))
	t.set_constant("buttons_width", "SpinBox", 22)
	t.set_constant("field_and_buttons_separation", "SpinBox", 4)
	t.set_constant("buttons_vertical_separation", "SpinBox", 2)
	t.set_stylebox("normal", "LineEdit", box(FIELD, "2b4150"))
	t.set_stylebox("focus", "LineEdit", box(FIELD, GOLD))
	t.set_stylebox("read_only", "LineEdit", box("0b131a", "1d2a35"))
	t.set_color("font_placeholder_color", "LineEdit", DIM)
	t.set_color("font_uneditable_color", "LineEdit", MUTED)
	t.set_color("caret_color", "LineEdit", GOLD_BRIGHT)
	t.set_color("selection_color", "LineEdit", Color(GOLD, 0.35))
	t.set_stylebox("normal", "TextEdit", box(FIELD, "2b4150"))
	t.set_stylebox("focus", "TextEdit", box(FIELD, GOLD))
	t.set_color("font_color", "TextEdit", IVORY)
	t.set_stylebox("normal", "CodeEdit", box(FIELD, "2b4150"))
	t.set_stylebox("focus", "CodeEdit", box(FIELD, GOLD))
	t.set_color("font_color", "CodeEdit", IVORY)
	t.set_color("background_color", "CodeEdit", FIELD)
	t.set_stylebox("panel", "PanelContainer", box(PANEL, "22323f", 14, 10))
	t.set_stylebox("panel", "PopupMenu", box("0d1720", BRONZE, 8, 8))
	t.set_stylebox("hover", "PopupMenu", box("253a46", "253a46", 6, 6))
	t.set_color("font_hover_color", "PopupMenu", GOLD_BRIGHT)
	t.set_color("font_disabled_color", "PopupMenu", DIM)
	t.set_stylebox("panel", "TooltipPanel", box("0b131a", BRONZE, 12, 8))
	t.set_color("font_color", "TooltipLabel", IVORY)
	t.set_font_size("font_size", "TooltipLabel", 16)
	# Scroll bars need explicit margins for width; keep the grabber visible.
	for bar in ["VScrollBar", "HScrollBar"]:
		var vertical: bool = bar == "VScrollBar"
		for item in [["grabber", "4a6577"], ["grabber_highlight", "6a8597"], ["grabber_pressed", "b08d57"], ["scroll", "121e28"], ["scroll_focus", "121e28"]]:
			var style: StyleBoxFlat = box(item[1], item[1] if item[0] != "scroll" and item[0] != "scroll_focus" else "22323f", 0, 5)
			if vertical: style.content_margin_left = 5; style.content_margin_right = 5; style.content_margin_top = 0; style.content_margin_bottom = 0
			else: style.content_margin_top = 5; style.content_margin_bottom = 5; style.content_margin_left = 0; style.content_margin_right = 0
			t.set_stylebox(item[0], bar, style)
	# Tabs read as a segmented strip: the selected tab carries a gold underline.
	var selected := box("1b2b38", "1b2b38", 16, 8, 0); selected.border_color = GOLD; selected.border_width_bottom = 3
	selected.corner_radius_bottom_left = 0; selected.corner_radius_bottom_right = 0
	var unselected := box("0e1820", "0e1820", 16, 8, 0); unselected.corner_radius_bottom_left = 0; unselected.corner_radius_bottom_right = 0
	var hovered := unselected.duplicate(); hovered.bg_color = Color("182632"); hovered.border_color = Color(GOLD, 0.45); hovered.border_width_bottom = 2
	t.set_stylebox("tab_selected", "TabBar", selected)
	t.set_stylebox("tab_unselected", "TabBar", unselected)
	t.set_stylebox("tab_hovered", "TabBar", hovered)
	t.set_stylebox("tab_disabled", "TabBar", unselected)
	t.set_stylebox("tab_focus", "TabBar", box("00000000", Color(GOLD, 0.6), 16, 8, 1))
	t.set_color("font_selected_color", "TabBar", GOLD_BRIGHT)
	t.set_color("font_unselected_color", "TabBar", MUTED)
	t.set_color("font_hovered_color", "TabBar", IVORY)
	t.set_color("font_disabled_color", "TabBar", DIM)
	t.set_constant("h_separation", "TabBar", 8)
	t.set_font_size("font_size", "TabBar", 17)
	# Tables (Tree): navy rows, gold selection and header buttons.
	t.set_stylebox("panel", "Tree", box(FIELD, "22323f", 6, 8))
	t.set_stylebox("focus", "Tree", box("00000000", Color(GOLD, 0.4), 6, 8))
	t.set_stylebox("selected", "Tree", box(Color(GOLD, 0.18), Color(GOLD, 0.7), 4, 4))
	t.set_stylebox("selected_focus", "Tree", box(Color(GOLD, 0.24), GOLD, 4, 4))
	t.set_stylebox("hovered", "Tree", box("182632", "182632", 4, 4))
	t.set_stylebox("cursor", "Tree", box("00000000", Color(GOLD, 0.5), 4, 4))
	t.set_stylebox("cursor_unfocused", "Tree", box("00000000", Color(GOLD, 0.3), 4, 4))
	t.set_stylebox("title_button_normal", "Tree", box("14222d", "22323f", 8, 0))
	t.set_stylebox("title_button_hover", "Tree", box("1b2b38", "2b4150", 8, 0))
	t.set_stylebox("title_button_pressed", "Tree", box("223442", GOLD, 8, 0))
	t.set_color("title_button_color", "Tree", GOLD)
	t.set_color("font_color", "Tree", IVORY)
	t.set_color("font_selected_color", "Tree", GOLD_BRIGHT)
	t.set_color("guide_color", "Tree", Color("1b2833"))
	t.set_color("relationship_line_color", "Tree", Color("1b2833"))
	t.set_constant("v_separation", "Tree", 8)
	t.set_constant("h_separation", "Tree", 8)
	t.set_constant("item_margin", "Tree", 6)
	t.set_constant("draw_guides", "Tree", 1)
	t.set_font_size("font_size", "Tree", 16)
	t.set_font_size("title_button_font_size", "Tree", 14)
	var line := StyleBoxLine.new(); line.color = Color("22323f"); line.thickness = 1
	t.set_stylebox("separator", "HSeparator", line)
	t.set_stylebox("split_bar_background", "HSplitContainer", box("0b131a", "0b131a", 0, 0))
	t.set_stylebox("panel", "Window", box(PANEL, BRONZE, 14, 10))
	t.set_stylebox("embedded_border", "Window", box(PANEL, BRONZE, 14, 10))
	t.set_stylebox("panel", "AcceptDialog", box(PANEL, BRONZE, 14, 10))
	return t

static func label(parent: Node, value: String, font_size: int = 17, color: Color = IVORY) -> Label:
	var l := Label.new(); l.text = value; l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.add_theme_font_size_override("font_size", font_size); l.add_theme_color_override("font_color", color)
	l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	parent.add_child(l); return l

static func heading(parent: Node, value: String) -> Label:
	var l := label(parent, value.to_upper(), 13, GOLD)
	l.add_theme_constant_override("outline_size", 0)
	return l

static func section(parent: Node, padding: int = 14) -> VBoxContainer:
	var p := PanelContainer.new(); p.add_theme_stylebox_override("panel", box(PANEL, "22323f", padding, 10)); parent.add_child(p)
	var v := VBoxContainer.new(); v.add_theme_constant_override("separation", 8); p.add_child(v)
	return v

## A two-column form: captions on the left, controls stretched on the right.
static func form(parent: Node) -> GridContainer:
	var g := GridContainer.new(); g.columns = 2
	g.add_theme_constant_override("h_separation", 14); g.add_theme_constant_override("v_separation", 8)
	g.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	parent.add_child(g); return g

static func accent(button: Button, color: Color = GOLD) -> Button:
	button.add_theme_stylebox_override("normal", box(Color(color, 0.16), color))
	button.add_theme_stylebox_override("hover", box(Color(color, 0.3), color.lightened(0.2)))
	button.add_theme_stylebox_override("pressed", box(Color(color, 0.42), color.lightened(0.3)))
	button.add_theme_stylebox_override("disabled", box(Color(color, 0.05), Color(color, 0.25)))
	button.add_theme_color_override("font_color", color.lightened(0.35))
	button.add_theme_color_override("font_disabled_color", Color(color, 0.45))
	return button

## A rounded pill such as a type tag or a status note.
static func chip(parent: Node, value: String, color: Color = GOLD, font_size: int = 13) -> Label:
	var l := label(parent, value, font_size, color.lightened(0.25))
	l.autowrap_mode = TextServer.AUTOWRAP_OFF
	l.add_theme_stylebox_override("normal", box(Color(color, 0.14), Color(color, 0.55), 8, 10, 1))
	l.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN; l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return l

## Round energy cost marker, as on battle cards.
static func pip(parent: Node, value: int, diameter: int = 30, color: Color = ENERGY) -> Label:
	var l := Label.new(); l.text = str(value); l.mouse_filter = Control.MOUSE_FILTER_IGNORE
	l.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; l.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	l.custom_minimum_size = Vector2(diameter, diameter)
	l.size_flags_horizontal = Control.SIZE_SHRINK_CENTER; l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	l.add_theme_font_size_override("font_size", int(diameter * 0.55)); l.add_theme_color_override("font_color", Color.WHITE)
	var s := box(color.darkened(0.3), Color("a9d6f5") if color == ENERGY else color.lightened(0.4), 0, diameter, 2)
	l.add_theme_stylebox_override("normal", s)
	parent.add_child(l); return l

## A headline number with a small caption, e.g. "12  HEALTH".
static func stat(parent: Node, value: String, caption: String, color: Color = IVORY) -> VBoxContainer:
	var p := PanelContainer.new(); p.add_theme_stylebox_override("panel", box(Color("0c1620"), Color(color, 0.35), 14, 8)); parent.add_child(p)
	var v := VBoxContainer.new(); v.add_theme_constant_override("separation", 0); p.add_child(v)
	var big := label(v, value, 26, color); big.autowrap_mode = TextServer.AUTOWRAP_OFF
	if not caption.is_empty(): label(v, caption.to_upper(), 12, MUTED).autowrap_mode = TextServer.AUTOWRAP_OFF
	return v

static func texture(path: String) -> Texture2D:
	return load(path) if path.begins_with("res://") and ResourceLoader.exists(path) else null

## The same art a battle card shows for this definition.
static func card_texture(id: String) -> Texture2D:
	var data := BattlePresentationCatalog.card(id)
	var tex := texture(str(data.get("illustration_path", "")))
	if tex == null: tex = texture(str(data.get("art", "")))
	if tex == null:
		var cinematic := preload("res://presentation/battle/cinematic_theme.gd")
		tex = cinematic.art(cinematic.card_art_index(id))
	return tex

## Framed art thumbnail; the art is cropped to fill.
static func thumbnail(parent: Node, tex: Texture2D, size: Vector2 = Vector2(46, 62), border: Color = BRONZE) -> PanelContainer:
	var frame := PanelContainer.new(); frame.mouse_filter = Control.MOUSE_FILTER_IGNORE
	frame.add_theme_stylebox_override("panel", box("0b131a", border, 2, 6, 1))
	frame.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	parent.add_child(frame)
	var art := TextureRect.new(); art.texture = tex; art.mouse_filter = Control.MOUSE_FILTER_IGNORE
	art.custom_minimum_size = size; art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	frame.add_child(art)
	return frame

## A small resized copy of a texture, for icons in buttons and dropdowns.
static func icon(tex: Texture2D, width: int, height: int) -> Texture2D:
	if tex == null: return null
	var image := tex.get_image()
	if image == null: return null
	image = image.duplicate() as Image
	if image.is_compressed(): image.decompress()
	image.resize(width, height, Image.INTERPOLATE_BILINEAR)
	return ImageTexture.create_from_image(image)

## Card art available to authors.
static func card_art() -> Array:
	var result: Array = []
	var dir := DirAccess.open("res://assets/battle/cards")
	if dir == null: return result
	for file in dir.get_files():
		var path := "res://assets/battle/cards/" + file.trim_suffix(".import")
		if path.get_extension() in ["png", "jpg", "webp"] and path != "res://assets/battle/cards/card_back.png" and path not in result and ResourceLoader.exists(path): result.append(path)
	result.sort()
	return result

## Gold chevrons for spin box steppers (up, down, or both stacked).
static func stepper_icon(up: bool, down: bool) -> Texture2D:
	var w := 12; var h := 16 if up and down else 8
	var image := Image.create(w, h, false, Image.FORMAT_RGBA8); image.fill(Color(0, 0, 0, 0))
	var white := GOLD
	for k in 5:
		if up:
			image.set_pixel(1 + k, 5 - k, white); image.set_pixel(10 - k, 5 - k, white)
			image.set_pixel(1 + k, 6 - k, white); image.set_pixel(10 - k, 6 - k, white)
		if down:
			var top := h - 8
			image.set_pixel(1 + k, top + 1 + k, white); image.set_pixel(10 - k, top + 1 + k, white)
			image.set_pixel(1 + k, top + 2 + k, white); image.set_pixel(10 - k, top + 2 + k, white)
	return ImageTexture.create_from_image(image)

## Each card pool type gets its own colour.
static func type_color(type_id: String) -> Color:
	match type_id:
		"general": return MUTED
		"venom": return Color("8fd18a")
		"curse": return Color("c49ae6")
		"blade_warden": return Color("e0906d")
	return GOLD

## Square check box glyphs drawn to match the theme.
static func check_icon(checked: bool, color: Color) -> Texture2D:
	var size := 18
	var image := Image.create(size, size, false, Image.FORMAT_RGBA8); image.fill(Color(0, 0, 0, 0))
	for i in size:
		for edge in [Vector2i(i, 0), Vector2i(i, size - 1), Vector2i(0, i), Vector2i(size - 1, i), Vector2i(i, 1), Vector2i(i, size - 2), Vector2i(1, i), Vector2i(size - 2, i)]: image.set_pixelv(edge, color)
	if checked:
		for y in range(4, size - 4):
			for x in range(4, size - 4): image.set_pixel(x, y, Color(color, 0.95))
		var ink := Color("10161d")
		for k in 4: image.set_pixel(5 + k, 8 + k, ink); image.set_pixel(5 + k, 9 + k, ink)
		for k in 6: image.set_pixel(8 + k, 11 - k, ink); image.set_pixel(8 + k, 12 - k, ink)
	return ImageTexture.create_from_image(image)

## A small "opens a workspace" glyph for tabs that launch full-screen editors.
static func launch_icon(color: Color = GOLD) -> Texture2D:
	var image := Image.create(16, 16, false, Image.FORMAT_RGBA8)
	image.fill(Color(0, 0, 0, 0))
	for i in range(2, 11):
		for edge in [Vector2i(i, 4), Vector2i(i, 13), Vector2i(2, i + 2), Vector2i(10, i + 2)]:
			if not (edge.x >= 7 and edge.y <= 7): image.set_pixelv(edge, Color(color, 0.8))
	for i in range(6, 14):
		image.set_pixel(i, 15 - i, color); image.set_pixel(i, 16 - i if 16 - i < 16 else 15, color)
	for i in range(9, 14):
		image.set_pixel(i, 2, color); image.set_pixel(13, i - 7, color)
	return ImageTexture.create_from_image(image)

static func slug(value: String) -> String:
	var out := ""
	for ch in value.to_lower():
		if (ch >= "a" and ch <= "z") or (ch >= "0" and ch <= "9"): out += ch
		elif not out.ends_with("_") and not out.is_empty(): out += "_"
	out = out.trim_suffix("_")
	if out.is_empty() or (out[0] >= "0" and out[0] <= "9"): out = "card_" + out
	return out.trim_suffix("_")
