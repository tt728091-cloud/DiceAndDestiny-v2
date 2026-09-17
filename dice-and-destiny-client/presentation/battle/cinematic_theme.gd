extends RefCounted

const GOLD := Color("a6987e")
const INK := Color("211e19")
const IVORY := Color("e9e1d1")

static func panel(fill: Color = Color("171714ed"), edge: Color = Color("756446"), padding: int = 12) -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	style.bg_color = fill; style.border_color = edge
	style.set_border_width_all(1); style.set_corner_radius_all(3)
	style.content_margin_left = padding; style.content_margin_right = padding
	style.content_margin_top = padding; style.content_margin_bottom = padding
	return style

static func create() -> Theme:
	var result := Theme.new()
	result.default_font_size = 20
	var serif := SystemFont.new(); serif.font_names = PackedStringArray(["Georgia", "Noto Serif", "DejaVu Serif", "serif"])
	result.set_font("font", "Label", serif)
	result.set_font("font", "Button", serif)
	result.set_font("normal_font", "RichTextLabel", serif)
	result.set_font("bold_font", "RichTextLabel", serif)
	result.set_color("font_color", "Label", IVORY)
	result.set_color("font_shadow_color", "Label", Color("080b0f"))
	result.set_constant("shadow_offset_x", "Label", 1); result.set_constant("shadow_offset_y", "Label", 2)
	result.set_stylebox("panel", "PanelContainer", panel(Color("171714d9"), Color("75644650")))
	for kind in ["Button", "CheckBox", "OptionButton"]:
		result.set_color("font_color", kind, IVORY)
		result.set_color("font_hover_color", kind, Color("fff0bf"))
		result.set_color("font_pressed_color", kind, Color("fff0bf"))
		result.set_color("font_disabled_color", kind, Color("9c9d97"))
		result.set_stylebox("normal", kind, panel(Color("181816ef"), GOLD))
		result.set_stylebox("hover", kind, panel(Color("34372eea"), Color("e5c47b")))
		result.set_stylebox("pressed", kind, panel(Color("4a4230f2"), Color("f4d187")))
		result.set_stylebox("disabled", kind, panel(Color("191917d8"), Color("625d4c")))
		result.set_stylebox("focus", kind, panel(Color("00000000"), Color("ffe3a0"), 0))
	result.set_stylebox("background", "ProgressBar", panel(Color("0d1117"), GOLD, 0))
	result.set_stylebox("fill", "ProgressBar", panel(Color("742d32"), Color("a96658"), 0))
	var line := StyleBoxLine.new(); line.color = Color("9c855b90"); line.thickness = 1
	result.set_stylebox("separator", "HSeparator", line)
	result.set_stylebox("panel", "PopupPanel", panel())
	result.set_stylebox("panel", "TooltipPanel", panel(Color("0e151afa"), GOLD, 14))
	result.set_font_size("font_size", "TooltipLabel", 18)
	result.set_color("default_color", "RichTextLabel", IVORY)
	result.set_font_size("normal_font_size", "RichTextLabel", 18)
	return result

static func art(index: int) -> Texture2D:
	var path := "res://assets/battle/wasteland/card_illustrations.png"
	if not ResourceLoader.exists(path): return null
	var texture: Texture2D = load(path)
	var atlas := AtlasTexture.new(); atlas.atlas = texture
	var cell := texture.get_size() / Vector2(3, 2)
	atlas.region = Rect2(Vector2(index % 3, index / 3) * cell, cell)
	return atlas

static func card_art_index(id: String) -> int:
	if "hand" in id or "die" in id or "it" == id or "reserve" in id: return 2
	if "ward" in id or "coagulate" in id or "molt" in id or "rebuttal" in id: return 4
	if "antidote" in id or "draught" in id or "repurpose" in id: return 5
	if "fang" in id or "puncture" in id or "pinprick" in id or "blade" in id or "extract" in id: return 0
	if "shock" in id: return 3
	if "measured" in id: return 2
	if "tongue" in id or "incubate" in id or "cycle" in id: return 1
	return 1

# Nine-slice parchment keeps the weathered edge crisp at each control size.
static func paper(tint: Color = Color.WHITE) -> StyleBoxTexture:
	var style := StyleBoxTexture.new()
	style.texture = load("res://assets/battle/wasteland/parchment.png")
	style.modulate_color = tint
	style.region_rect = Rect2(Vector2(48, 48), style.texture.get_size() - Vector2(96, 96))
	for side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		style.set_texture_margin(side, 8)
		style.set_content_margin(side, 10)
	return style

static func paper_button(button: Button) -> void:
	for state in ["normal", "hover", "pressed", "disabled"]:
		button.add_theme_stylebox_override(state, paper(Color("fff0cc") if state == "hover" else Color("c9b89a") if state == "pressed" else Color("c0b9ab") if state == "disabled" else Color.WHITE))
	for key in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color"]:
		button.add_theme_color_override(key, INK)
	button.add_theme_color_override("font_disabled_color", Color("625a4c"))
	button.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)

static func roll_control_font() -> SystemFont:
	# Georgia uses descending old-style numerals and has no lining alternates
	# on macOS. Keep a serif face here with digits that share the text baseline.
	var font := SystemFont.new()
	font.font_names = PackedStringArray(["Times New Roman", "Noto Serif", "DejaVu Serif", "serif"])
	return font
