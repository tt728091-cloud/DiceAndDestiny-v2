extends "res://presentation/battle/attack_intent_button.gd"
## A second entry point for one authoritative incoming source. The presenter
## supplies the same tooltip and animated amount used by the overhead intent.
const INK := preload("res://presentation/battle/cinematic_theme.gd")
const ICONS := preload("res://presentation/battle/battle_icons.gd")
var presenter: Control
var amount: Label
var state_label: Label
var body: VBoxContainer
var _last_rank := -1

func configure(value: Control) -> void:
	presenter = value
	name = "IncomingAttack_" + str(presenter.data.source_id)
	set_meta("inspection_id", "battle.incoming." + str(presenter.data.source_id))
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	body = VBoxContainer.new(); body.add_theme_constant_override("separation", 2)
	body.mouse_filter = Control.MOUSE_FILTER_IGNORE; add_child(body)
	var header := HBoxContainer.new(); header.mouse_filter = Control.MOUSE_FILTER_IGNORE; body.add_child(header)
	var icon := TextureRect.new(); icon.texture = ICONS.texture("attack")
	icon.custom_minimum_size = Vector2(22, 22); icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE; header.add_child(icon)
	amount = _label(header, "", 22)
	var damage_caption := _label(header, "damage", 16); damage_caption.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	state_label = _label(header, "", 16); state_label.add_theme_color_override("font_color", Color("ffe1a6"))
	var title := _label(body, BattlePresentationCatalog.ability(str(presenter.source.get("source_content_id", ""))).name, 19)
	title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_label(body, "From " + presenter.screen._actor_display_name(presenter.attacker_id), 16).autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	var effects := str(presenter.data.get("attack_statuses", ""))
	if not effects.is_empty():
		var detail := _label(body, effects, 15); detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	for style_name in ["normal", "hover", "pressed", "disabled", "focus"]:
		var selected: bool = presenter.screen._selected_source == str(presenter.data.source_id) or not presenter.screen._source_card_actions(str(presenter.data.source_id)).is_empty()
		add_theme_stylebox_override(style_name, INK.panel(INK.DARK_SURFACE, Color("e7c378") if selected or style_name in ["hover", "focus"] else Color("726951"), 4))
	resized.connect(_layout_body); body.minimum_size_changed.connect(_layout_body)
	pressed.connect(func():
		var screen: Control = presenter.screen
		var id := str(presenter.data.source_id)
		if screen._selected_card.get("source_targeting", false):
			if not screen._source_card_actions(id).is_empty(): screen._play_source_card(id)
		else: screen._select_attack_intent(id, true)
	)
	_layout_body(); refresh()

func _label(parent: Node, value: String, font_size: int) -> Label:
	var label := Label.new(); label.text = value; label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", Color("f4ecda")); parent.add_child(label)
	return label

func _layout_body() -> void:
	body.position = Vector2(9, 6); body.size.x = maxf(1, size.x - 18)
	custom_minimum_size.y = body.get_combined_minimum_size().y + 12

func refresh() -> void:
	amount.text = presenter.damage.text
	amount.modulate = presenter.damage.modulate
	amount.add_theme_color_override("font_color", presenter.damage.get_theme_color("font_color"))
	tooltip_text = presenter._ability_tooltip()
	var screen: Control = presenter.screen
	var id := str(presenter.data.source_id)
	var history: Dictionary = screen._view.raw_snapshot.get("defense_history", {}).get(id, {})
	var plan: Dictionary = screen._view.raw_snapshot.get("defense_plans", {}).get(id, {})
	var active: Dictionary = screen._view.defense_selections.get(screen.viewer_actor_id, {})
	state_label.text = "Undefended"
	if not history.is_empty():
		state_label.text = "" if not str(history.get("ability_id", "")).is_empty() else "Passed"
	elif not plan.is_empty(): state_label.text = "Queued"
	elif str(active.get("source_id", "")) == id and not str(active.get("ability_id", "")).is_empty():
		state_label.text = "" if presenter._damage_settled else "Defending…"
	var rank := defense_rank()
	if _last_rank >= 0 and rank != _last_rank: screen._sort_incoming_attacks.call_deferred(rank == 2)
	_last_rank = rank
	# Legal source actions, not remaining damage alone, govern this entry point.
	# This also keeps card-first targeting available in the lower-left list.
	disabled = presenter.intent.disabled
	if not screen._selected_card.get("source_targeting", false):
		disabled = disabled or (screen._source_protection_action(id).is_empty() and not screen._view.legal_actions.any(func(action): return action.get("actor_id") == screen.viewer_actor_id and action.get("type") == "planning_select_ability" and id in action.get("payload", {}).get("target_ids", [])))

func defense_rank() -> int:
	if state_label.text in ["", "Passed"]: return 2
	return 1 if state_label.text == "Queued" else 0

func damage_rect() -> Rect2:
	# A source can scroll out of view. Keep its effect endpoint inside the list,
	# never redirect a prevention flight back up to the enemy's overhead badge.
	var rect := amount.get_global_rect()
	var viewport: Rect2 = presenter.screen._incoming_attack_list.get_global_rect()
	rect.position.y = clampf(rect.position.y, viewport.position.y, maxf(viewport.position.y, viewport.end.y - rect.size.y))
	return rect
