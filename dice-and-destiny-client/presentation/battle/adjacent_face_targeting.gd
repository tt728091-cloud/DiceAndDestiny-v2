extends Control

# The check result is informational; only the subsequent face choice adds a mark.
var screen: Control
var faces: Dictionary = {}
var panel: PanelContainer
var hint: Label
var actor := ""
var index := 0
var rolled := 0
var _ready_elapsed := 0.0
var _submitted := false

func configure(owner_screen: Control) -> void:
	screen = owner_screen
	name = "AdjacentFaceTargeting"; mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 25
	var work: Dictionary = screen._view.raw_snapshot.curse_choice
	actor = screen._display_actor_id(str(work.target)); index = int(work.die); rolled = int(work.face)
	for face in screen._number_face_actions():
		var button := Button.new(); button.name = "AdjacentFace%d" % face
		button.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		button.tooltip_text = "Curse face %d on die %d" % [face, index + 1]
		var style := StyleBoxFlat.new(); style.bg_color = Color("e2b2ff45"); style.border_color = Color("ffe9ac"); style.set_border_width_all(2); style.set_corner_radius_all(3)
		style.shadow_color = Color("e2b2ff90"); style.shadow_size = 4
		for state in ["normal", "hover", "pressed", "disabled"]: button.add_theme_stylebox_override(state, style)
		button.pressed.connect(_choose.bind(face)); add_child(button); faces[face] = button
	panel = PanelContainer.new(); add_child(panel)
	panel.add_theme_stylebox_override("panel", preload("res://presentation/battle/cinematic_theme.gd").panel(Color("201b22f5"), Color("b88bd1"), 8))
	hint = Label.new(); hint.custom_minimum_size.x = 220; hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; hint.add_theme_font_size_override("font_size", 17); panel.add_child(hint)
	hide()

func _process(delta: float) -> void:
	var choices: Dictionary = screen._number_face_actions()
	var ready: bool = not choices.is_empty() and not screen._card_gain_active() and not screen._submitting and not screen._model_thinking and not screen._history_review and not screen._history_replay and not screen._snapshot_panel_open
	visible = ready
	if not ready: _ready_elapsed = 0.0; return
	_ready_elapsed += delta
	refresh(choices)
	# Hold the sole eligible chip briefly so its automatic selection is visible.
	if choices.size() == 1 and _ready_elapsed >= 0.45 and not _submitted: _choose(int(choices.keys()[0]))

func refresh(choices: Dictionary) -> void:
	var dock: Control = screen.dice_dock(actor)
	if dock.get_child_count() == 0: return
	var tray: BattleDiceTray = dock.get_child(0)
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for face in faces:
		var chip: Control = tray._mark_faces[index][int(face) - 1]
		var bounds: Rect2 = inverse * chip.get_global_rect()
		faces[face].position = bounds.position; faces[face].size = bounds.size
		faces[face].disabled = not choices.has(face)
		faces[face].modulate.a = 0.9 + sin(_ready_elapsed * 6) * 0.1
	var bounds: Rect2 = inverse * dock.get_global_rect()
	panel.size = panel.get_combined_minimum_size()
	panel.position = Vector2(bounds.end.x + 18 if actor == "blade" else bounds.position.x - panel.size.x - 18, bounds.position.y)
	screen.fit_dice_popup(panel)
	hint.text = "Widen the Crack\nDie %d rolled %d\n%s" % [index + 1, rolled, "Applying the only eligible face…" if choices.size() == 1 else "Choose a highlighted face below this die."]
	queue_redraw()

func _choose(face: int) -> void:
	if _submitted or screen._card_gain_active() or not screen._number_face_actions().has(face): return
	_submitted = true
	screen._commit_number_face(face)

func _draw() -> void:
	var dock: Control = screen.dice_dock(actor)
	if dock.get_child_count() == 0: return
	var tray: BattleDiceTray = dock.get_child(0)
	var inverse := get_global_transform_with_canvas().affine_inverse()
	draw_rect((inverse * tray._buttons[index].get_global_rect()).grow(2), Color("e2b2ff"), false, 2, true)
