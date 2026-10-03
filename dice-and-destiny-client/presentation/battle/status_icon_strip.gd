extends Container
const ICONS := preload("res://presentation/battle/battle_icons.gd")
var cells: Dictionary = {}
var counts: Dictionary = {}
var persistent_counts: Dictionary = {}
var pending := false
# Diagnostic/accessibility text, never painted. Presentation uses semantic IDs.
var text := "":
	set(value):
		text = value
		if not _writing: _read_legacy_text(value)
var _writing := false
const CELL := Vector2(58, 34)
func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	resized.connect(_layout)
func set_counts(values: Dictionary, is_pending: bool = false, preview_ids: Dictionary = {}) -> void:
	counts = values.duplicate(); counts.merge(persistent_counts, true); pending = is_pending
	var lines: Array[String] = []
	for id in counts:
		ensure_slot(str(id))
		var data := BattlePresentationCatalog.status(str(id))
		if int(counts[id]) > 0: lines.append("%s %s ×%d%s" % [data.glyph, data.name, int(counts[id]), " · pending" if pending or preview_ids.has(id) else ""])
	for id in cells:
		var amount := int(counts.get(id, 0))
		cells[id].get_node("Count").text = ("+" if pending else "") + str(amount)
		cells[id].modulate.a = 1.0 if amount > 0 else 0.0
		cells[id].mouse_filter = Control.MOUSE_FILTER_STOP if amount > 0 else Control.MOUSE_FILTER_IGNORE
	_writing = true; text = "No active statuses" if lines.is_empty() else "\n".join(lines); _writing = false
	_layout()
func ensure_slot(id: String) -> Control:
	if cells.has(id): return cells[id]
	var cell := StatusCell.new(); cell.name = "Status_" + id; cell.add_theme_constant_override("separation", 4)
	cell.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var data := BattlePresentationCatalog.status(id)
	cell.tooltip_text = "%s\n%s" % [data.name, data.text]
	var icon := TextureRect.new(); icon.texture = ICONS.texture(id); icon.custom_minimum_size = Vector2(28, 28); icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE; cell.add_child(icon)
	var count := Label.new(); count.name = "Count"; count.text = "0"; count.add_theme_font_size_override("font_size", 20); count.mouse_filter = Control.MOUSE_FILTER_IGNORE; cell.add_child(count)
	preload("res://presentation/battle/cinematic_theme.gd").hud_lettering(count, true)
	add_child(cell); cells[id] = cell; cell.modulate.a = 0
	_layout(); return cell
func _get_minimum_size() -> Vector2:
	var columns := maxi(1, int(size.x / CELL.x))
	return Vector2(0, maxf(CELL.y, ceilf(float(cells.size()) / columns) * CELL.y))
func _layout() -> void:
	var columns := maxi(1, int(size.x / CELL.x))
	var keys := cells.keys(); keys.sort()
	for i in keys.size():
		var row_count := mini(columns, keys.size() - (i / columns) * columns)
		var offset := maxf(0, (size.x - row_count * CELL.x) / 2)
		fit_child_in_rect(cells[keys[i]], Rect2(Vector2(offset + (i % columns) * CELL.x, (i / columns) * CELL.y), CELL))
	update_minimum_size()
func bounds(id: String) -> Rect2:
	return ensure_slot(id).get_global_rect()
func _read_legacy_text(value: String) -> void:
	# Older saved presentation/tests may supply label text. Resolve it once at
	# this boundary; all new animation callers address IDs and counts directly.
	var values := {}
	for line in value.split("\n"):
		if "×" not in line: continue
		var name := line.get_slice("×", 0).strip_edges()
		var id := name.to_snake_case()
		for known in ICONS.SHAPES:
			var data := BattlePresentationCatalog.status(known)
			if name == str(data.name) or name == "%s %s" % [data.glyph, data.name]: id = known; break
		values[id] = int(line.get_slice("×", 1))
	set_counts(values, "pending" in value)

class StatusCell extends HBoxContainer:
	func _make_custom_tooltip(for_text: String) -> Object:
		return preload("res://presentation/battle/wrapped_tooltip.gd").create(self, for_text)
