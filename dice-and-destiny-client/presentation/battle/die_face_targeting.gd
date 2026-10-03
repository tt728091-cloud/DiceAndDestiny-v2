extends Control

# Board-local selection: only the dice and the unfolded face net intercept input.
var screen: Control
var targets: Dictionary = {}
var panel: PanelContainer
var faces: Array[Button] = []
var _elapsed := 0.0
var card_name := ""
var extra_check := false
var mandatory := false
const FOLDOUT := preload("res://presentation/battle/number_face_foldout.gd")

func configure(owner_screen: Control) -> void:
	screen = owner_screen
	mandatory = not screen._board_curse_actions().is_empty()
	var work: Dictionary = screen._view.raw_snapshot.get("curse_choice", {}) if mandatory else {}
	var id := str(work.card_id) if mandatory else str(screen._selected_card.definition_id)
	card_name = str(BattlePresentationCatalog.card(id).name) if not BattlePresentationCatalog.definition("cards", id).is_empty() else str(BattlePresentationCatalog.ability(id).name)
	extra_check = id == "unquiet_hands"
	mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 25
	for action in screen._mark_actions():
		var parts: PackedStringArray = screen._mark_choice(action).split(":")
		var key := "%s:%s" % [parts[0], parts[1]]
		if targets.has(key): continue
		var button := Button.new(); button.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		button.tooltip_text = "%s · choose %s die %d" % [card_name, screen._actor_display_name(parts[0]), int(parts[1]) + 1]
		var style := StyleBoxFlat.new(); style.bg_color = Color("ad71e518"); style.border_color = Color("dfadff"); style.set_border_width_all(3); style.set_corner_radius_all(8)
		for state in ["normal", "hover", "pressed", "focus"]: button.add_theme_stylebox_override(state, style)
		button.pressed.connect(screen._select_mark_die.bind(parts[0], int(parts[1])))
		add_child(button); targets[key] = button
	panel = PanelContainer.new(); panel.name = "CallTheMarkFaces"; add_child(panel)
	panel.add_theme_stylebox_override("panel", preload("res://presentation/battle/cinematic_theme.gd").panel(Color("201526f5"), Color("b88bd1"), 8))
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 8); panel.add_child(body)
	var title := Label.new(); title.text = card_name; title.add_theme_font_size_override("font_size", 19); body.add_child(title)
	var hint := Label.new(); hint.add_theme_font_size_override("font_size", 16); hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; hint.custom_minimum_size.x = 220; body.add_child(hint)
	var selected := str(screen._selected_card.get("mark_die", ""))
	hint.text = "Choose a highlighted die" if selected.is_empty() else "Die %d · choose a cursed face" % (int(selected.get_slice(":", 1)) + 1)
	if id in ["steady_hand", "forked_tongue", "nudge"] and not selected.is_empty(): hint.text = "Die %d · choose a face" % (int(selected.get_slice(":", 1)) + 1)
	if extra_check: hint.text = "Choose a highlighted die.\nSeparate Curse check: cursed face → +1 Count.\nOffensive result stays unchanged."
	if id == "widen_the_crack": hint.text = "Choose a highlighted die. Roll it, then curse an available adjacent number."
	if id == "chosen_instrument": hint.text = "Choose a highlighted die for Chosen Instrument."
	if id == "try_again": hint.text = "Choose one of your highlighted dice to reroll. No normal roll attempt is consumed."
	if id == "no_safe_keep": hint.text = "Choose a highlighted kept die to reroll. Its offensive result will change."
	if mandatory: hint.text = "Choose a highlighted die to entomb." if work.kind == "grasp" else "Choose a highlighted die to apply Curse."
	if not selected.is_empty() and not extra_check:
		var net := Control.new(); net.custom_minimum_size = Vector2(212, 284); body.add_child(net)
		var legal := {}
		for action in screen._mark_actions():
			var parts: PackedStringArray = screen._mark_choice(action).split(":")
			if "%s:%s" % [parts[0], parts[1]] == selected: legal[int(parts[2])] = action
		for face in FOLDOUT.CELLS:
			var button := FOLDOUT.FaceButton.new(); button.face = face
			button.name = "Face%d" % face; button.position = FOLDOUT.CELLS[face] * 72; button.size = Vector2(68, 68)
			button.disabled = not legal.has(face)
			button.tooltip_text = "Set die %d to face %d · no roll" % [int(selected.get_slice(":", 1)) + 1, face] if not button.disabled else "Face %d is unavailable" % face
			for state in ["normal", "hover", "pressed"]:
				button.add_theme_stylebox_override(state, FOLDOUT.THEME.panel(Color("242421") if state == "normal" else Color("44334e"), Color("a6987e") if state == "normal" else Color("dfadff"), 0))
			if not button.disabled: button.pressed.connect(screen._commit_mark_face.bind(legal[face])); faces.append(button)
			net.add_child(button)
	var cancel := Button.new(); cancel.text = "Cancel"; cancel.pressed.connect(screen._cancel_mark_targeting); body.add_child(cancel); cancel.visible = not mandatory
	panel.modulate.a = 0
	refresh()

func _process(delta: float) -> void:
	_elapsed += delta; refresh()

func refresh() -> void:
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for key in targets:
		var actor: String = key.get_slice(":", 0); var index := int(key.get_slice(":", 1))
		var dock: Control = screen.dice_dock(actor)
		var tray: BattleDiceTray = dock.get_child(0)
		var bounds: Rect2 = inverse * tray._buttons[index].get_global_rect()
		targets[key].position = bounds.position; targets[key].size = bounds.size
		targets[key].modulate.a = 0.7 + 0.3 * sin(_elapsed * 4.0)
		if key == screen._selected_card.get("mark_die", ""): targets[key].modulate.a = 1.0
	var selected := str(screen._selected_card.get("mark_die", ""))
	var actor: String = selected.get_slice(":", 0) if not selected.is_empty() else (screen._focused_enemy if targets.keys().any(func(key): return str(key).begins_with(screen._focused_enemy + ":")) else "blade")
	var dock: Control = screen.dice_dock(actor)
	var bounds: Rect2 = inverse * dock.get_global_rect()
	panel.size = Vector2(244, panel.get_combined_minimum_size().y)
	var unfold := 1.0 - pow(1.0 - clampf(_elapsed / 0.18, 0, 1), 3)
	panel.position = Vector2(bounds.end.x + 18 if actor == "blade" else bounds.position.x - panel.size.x - 18, bounds.position.y)
	panel.position.x += (1.0 - unfold) * (16 if actor != "blade" else -16)
	screen.fit_dice_popup(panel)
	panel.modulate.a = unfold
	queue_redraw()

func _unhandled_key_input(event: InputEvent) -> void:
	if not mandatory and event.is_action_pressed("ui_cancel"):
		screen._cancel_mark_targeting(); get_viewport().set_input_as_handled()

func _draw() -> void:
	for card in screen._hand_dock.find_children("*", "Button", true, false):
		if card is BattleCard and card.instance_id == screen._selected_card.get("instance_id"):
			card.draw_targeting_outline(self)
