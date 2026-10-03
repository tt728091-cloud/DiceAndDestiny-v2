extends Control

const THEME := preload("res://presentation/battle/cinematic_theme.gd")
# An unfolded cube: all six choices are visible, with no list or scrolling.
const CELLS := {1: Vector2(1, 0), 2: Vector2(0, 1), 3: Vector2(1, 1), 5: Vector2(2, 1), 6: Vector2(1, 2), 4: Vector2(1, 3)}
class FaceButton extends Button:
	var face := 1
	func _draw() -> void:
		var spots: Array[Vector2] = []
		if face % 2 == 1: spots.append(Vector2(0.5, 0.5))
		if face >= 2: spots.append_array([Vector2(0.28, 0.28), Vector2(0.72, 0.72)])
		if face >= 4: spots.append_array([Vector2(0.72, 0.28), Vector2(0.28, 0.72)])
		if face == 6: spots.append_array([Vector2(0.28, 0.5), Vector2(0.72, 0.5)])
		for spot in spots: draw_circle(size * spot, 4, Color("6b665e") if disabled else Color("efe1bf"))

var screen: Control
var panel: PanelContainer
var faces: Dictionary = {}
var _elapsed := 0.0
var mandatory := false
var target_actor := ""

func configure(owner_screen: Control) -> void:
	screen = owner_screen
	mandatory = not screen._board_curse_actions().is_empty()
	var work: Dictionary = screen._view.raw_snapshot.get("curse_choice", {}) if mandatory else {}
	target_actor = screen._display_actor_id(str(work.target)) if mandatory else screen._focused_enemy
	var id := str(work.card_id) if mandatory else str(screen._selected_card.definition_id)
	var definition: Dictionary = BattlePresentationCatalog.card(id) if not BattlePresentationCatalog.definition("cards", id).is_empty() else BattlePresentationCatalog.ability(id)
	name = "NumberFaceFoldout"; mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 25
	panel = PanelContainer.new(); add_child(panel)
	panel.add_theme_stylebox_override("panel", THEME.panel(Color("201b22f5"), Color("b88bd1"), 10))
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 8); panel.add_child(body)
	var title := Label.new(); title.text = str(definition.name)
	title.add_theme_font_size_override("font_size", 19); body.add_child(title)
	var hint := Label.new(); hint.text = "%s\nChoose the number to curse" % screen._actor_display_name(target_actor)
	hint.add_theme_font_size_override("font_size", 16); body.add_child(hint)
	var net := Control.new(); net.custom_minimum_size = Vector2(212, 284); body.add_child(net)
	var actions: Dictionary = screen._number_face_actions()
	for face in CELLS:
		var button := FaceButton.new(); button.face = face
		button.name = "Face%d" % face; button.position = CELLS[face] * 72; button.size = Vector2(68, 68)
		button.disabled = not actions.has(face)
		button.mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
		button.tooltip_text = "Curse face %d" % face if not button.disabled else "Face %d is unavailable" % face
		button.add_theme_stylebox_override("normal", THEME.panel(Color("242421"), Color("a6987e"), 0))
		button.add_theme_stylebox_override("hover", THEME.panel(Color("44334e"), Color("dfadff"), 0))
		button.add_theme_stylebox_override("pressed", THEME.panel(Color("594261"), Color("efcbff"), 0))
		button.pressed.connect(screen._commit_number_face.bind(face)); net.add_child(button); faces[face] = button
	var rule := Label.new(); rule.custom_minimum_size.x = 212; rule.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	rule.add_theme_font_size_override("font_size", 15)
	rule.text = "Marks 2 random clean dice. Missing clean dice trigger expansion rolls." if id == "shared_misfortune" else "Marks this number on up to 3 random dice that lack it."
	if mandatory:
		rule.text = "Curse this number on every enemy die." if work.kind == "eclipse" else "Die %d rolled %d. Choose an available adjacent number to curse." % [int(work.die) + 1, int(work.face)]
	body.add_child(rule)
	var cancel := Button.new(); cancel.text = "Cancel"; cancel.pressed.connect(screen._cancel_mark_targeting); body.add_child(cancel); cancel.visible = not mandatory
	panel.modulate.a = 0; refresh()

func _process(delta: float) -> void:
	_elapsed += delta; refresh()

func refresh() -> void:
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var bounds: Rect2 = inverse * screen.dice_dock(target_actor).get_global_rect()
	panel.size = panel.get_combined_minimum_size()
	var unfold := 1.0 - pow(1.0 - clampf(_elapsed / 0.2, 0, 1), 3)
	panel.position = Vector2((bounds.end.x + 18 if target_actor == "blade" else bounds.position.x - panel.size.x - 18) + (1.0 - unfold) * 16, bounds.position.y - 35)
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
