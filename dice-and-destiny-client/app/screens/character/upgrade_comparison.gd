extends PanelContainer

# A read-only comparison beside the action, with scrollable full rules on both sides.
const ADDED := Color("8fe2af")
const REPLACED := Color("e6c17c")
var anchor: Button
var _title: Label
var _before_name: Label
var _after_name: Label
var _before_rules: RichTextLabel
var _after_rules: RichTextLabel
var before_text := ""
var after_text := ""
var changed_before: Array[String] = []
var changed_after: Array[String] = []
var _away := 0.0
var _pointer := Vector2(-1, -1)

func _ready() -> void:
	z_index = 30
	mouse_filter = Control.MOUSE_FILTER_STOP
	var style := StyleBoxFlat.new(); style.bg_color = Color("101b25"); style.border_color = REPLACED
	style.set_border_width_all(1); style.set_corner_radius_all(8)
	style.content_margin_left = 20; style.content_margin_right = 20; style.content_margin_top = 18; style.content_margin_bottom = 18
	add_theme_stylebox_override("panel", style)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 12); add_child(body)
	_title = _label(body, "", 22, REPLACED)
	_label(body, "Compare versions · highlighted words change · scroll either column for full details", 14, Color("aebdc9"))
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 22); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	var left := _column(columns, "CURRENT", REPLACED); _before_name = left[0]; _before_rules = left[1]
	var right := _column(columns, "AFTER UPGRADE", ADDED); _after_name = right[0]; _after_rules = right[1]
	hide()

func _label(parent: Node, value: String, font_size: int, color: Color) -> Label:
	var label := Label.new(); label.text = value; label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", color); parent.add_child(label); return label

func _column(parent: Node, caption: String, color: Color) -> Array:
	var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.add_theme_constant_override("separation", 10); parent.add_child(column)
	_label(column, caption, 14, color)
	var heading := _label(column, "", 22, color)
	var rules := RichTextLabel.new(); rules.size_flags_vertical = Control.SIZE_EXPAND_FILL
	rules.size_flags_horizontal = Control.SIZE_EXPAND_FILL; rules.add_theme_font_size_override("normal_font_size", 18); rules.add_theme_font_size_override("bold_font_size", 18)
	rules.add_theme_color_override("default_color", Color("e9e6de")); rules.scroll_active = true; rules.selection_enabled = true
	column.add_child(rules); return [heading, rules]

func present(button: Button, current_name: String, target_name: String, current_rules: String, target_rules: String, cost: int, downgrade: bool = false) -> void:
	anchor = button; _away = 0
	_title.text = ("Downgrade comparison · +%d XP" if downgrade else "Upgrade comparison · %d XP") % cost
	_before_name.text = current_name; _after_name.text = target_name
	_after_name.get_parent().get_child(0).text = "AFTER DOWNGRADE" if downgrade else "AFTER UPGRADE"
	before_text = current_rules; after_text = target_rules
	var a := _tokens(current_rules); var b := _tokens(target_rules)
	var unchanged := _matching_tokens(a, b)
	changed_before = _write_diff(_before_rules, a, unchanged[0], REPLACED)
	changed_after = _write_diff(_after_rules, b, unchanged[1], ADDED)
	_before_rules.scroll_to_line(0); _after_rules.scroll_to_line(0)
	_place(); show()

func dismiss() -> void:
	anchor = null; hide()

func _place() -> void:
	var bounds := get_viewport_rect().size
	size = Vector2(minf(900, bounds.x - 48), minf(600, bounds.y - 48))
	var rect := anchor.get_global_rect()
	var x := rect.position.x - size.x - 16
	if x < 24: x = rect.end.x + 16
	position = Vector2(clampf(x, 24, maxf(24, bounds.x - size.x - 24)), clampf(rect.position.y, 24, maxf(24, bounds.y - size.y - 24)))

func _input(event: InputEvent) -> void:
	if event is InputEventMouse: _pointer = event.position

func _process(delta: float) -> void:
	if not visible: return
	if not is_instance_valid(anchor) or not anchor.is_visible_in_tree(): dismiss(); return
	_place()
	var pointer := _pointer if _pointer.x >= 0 else get_global_mouse_position()
	if anchor.get_global_rect().has_point(pointer) or get_global_rect().has_point(pointer) or anchor.has_focus(): _away = 0
	else: _away += delta
	if _away > 0.15: dismiss()

func _tokens(value: String) -> Array[String]:
	var expression := RegEx.new(); expression.compile("\\S+\\s*")
	var result: Array[String] = []
	for match in expression.search_all(value): result.append(match.get_string())
	return result

# Longest common subsequence keeps unchanged wording neutral, even when phrases move.
func _matching_tokens(a: Array[String], b: Array[String]) -> Array:
	var lengths: Array[PackedInt32Array] = []
	for i in range(a.size() + 1):
		var row := PackedInt32Array(); row.resize(b.size() + 1); lengths.append(row)
	for i in range(a.size() - 1, -1, -1):
		for j in range(b.size() - 1, -1, -1):
			lengths[i][j] = 1 + lengths[i + 1][j + 1] if a[i].strip_edges() == b[j].strip_edges() else maxi(lengths[i + 1][j], lengths[i][j + 1])
	var left := {}; var right := {}; var i := 0; var j := 0
	while i < a.size() and j < b.size():
		if a[i].strip_edges() == b[j].strip_edges(): left[i] = true; right[j] = true; i += 1; j += 1
		elif lengths[i + 1][j] >= lengths[i][j + 1]: i += 1
		else: j += 1
	return [left, right]

func _write_diff(label: RichTextLabel, tokens: Array[String], unchanged: Dictionary, color: Color) -> Array[String]:
	label.clear()
	var changed: Array[String] = []
	for i in tokens.size():
		if not unchanged.has(i):
			changed.append(tokens[i].strip_edges()); label.push_color(color); label.push_bold()
		label.add_text(tokens[i])
		if not unchanged.has(i): label.pop(); label.pop()
	return changed
