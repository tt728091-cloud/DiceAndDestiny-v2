class_name BattleAbilityTile
extends "res://presentation/battle/tooltip_button.gd"
const TOOLTIP_BUTTON := preload("res://presentation/battle/tooltip_button.gd")
const TOOLTIP_LABEL := preload("res://presentation/battle/tooltip_label.gd")

var ability_id := ""
var _upgrade_notice: Label
var _compact := false
var _minimal := false
var _selected_summary := ""
var _actor: Dictionary = {}
var _recipe_label: RichTextLabel
var _offensive_summary: VBoxContainer
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const UPGRADE_DURATION := 2.8
signal tier_pressed(tier_id: String)
signal choice_pressed(action: Dictionary)

# The tile remains the visual/tooltip container; only the individual tiers
# receive clicks and keyboard focus in this mode.
func configure_tiers(options: Array[Dictionary], selected_tier: String) -> void:
	text = ""
	if is_instance_valid(_recipe_label): _recipe_label.hide()
	if is_instance_valid(_offensive_summary): _offensive_summary.hide()
	disabled = true
	focus_mode = Control.FOCUS_NONE
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	modulate = Color.WHITE
	var any_available := false
	for option in options: any_available = any_available or bool(option.get("enabled", false))
	_style_availability(any_available)
	var content := HBoxContainer.new(); content.name = "TierControls"
	add_child(content)
	content.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	content.offset_left = 12 if _compact else 6; content.offset_right = -32
	content.offset_top = 3; content.offset_bottom = -3
	content.alignment = BoxContainer.ALIGNMENT_CENTER
	var title := TOOLTIP_LABEL.new()
	title.text = BattlePresentationCatalog.ability(ability_id).name
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title.add_theme_font_size_override("font_size", 18); title.add_theme_color_override("font_color", CINEMATIC.INK if any_available else Color("4e4b46")); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	title.tooltip_text = tooltip_text
	title.mouse_filter = Control.MOUSE_FILTER_PASS
	content.add_child(title)
	var row := HBoxContainer.new(); row.name = "MinimalTiers"
	row.alignment = BoxContainer.ALIGNMENT_BEGIN
	row.add_theme_constant_override("separation", 2)
	content.add_child(row)
	for option in options:
		if row.get_child_count() > 0:
			var slash := TOOLTIP_LABEL.new(); slash.text = "/"; slash.add_theme_color_override("font_color", CINEMATIC.INK); row.add_child(slash)
		var button := TOOLTIP_BUTTON.new()
		button.text = str(option.get("button_label", option.label)); button.expand_icon = true; button.custom_minimum_size = Vector2(64, 36); button.add_theme_constant_override("icon_max_width", 22)
		if option.has("icon"): button.icon = load(str(option.icon))
		button.add_theme_font_size_override("font_size", 20); button.add_theme_color_override("font_color", CINEMATIC.INK); button.add_theme_color_override("font_hover_color", Color("715216"))
		for style_name in ["normal", "hover", "pressed", "disabled"]:
			var style := CINEMATIC.panel(Color.TRANSPARENT if style_name == "disabled" else Color("ffefc980") if style_name == "normal" else Color("fff1c6"), Color.TRANSPARENT if style_name == "disabled" else Color("ad7b29"), 3)
			style.content_margin_bottom = 20 + str(option.get("summary", "")).count("\n") * 14
			button.add_theme_stylebox_override(style_name, style)
		button.tooltip_text = str(option.text)
		button.disabled = not bool(option.get("enabled", false))
		button.toggle_mode = true
		button.button_pressed = str(option.id) == selected_tier
		button.set_meta("tier_id", str(option.id))
		button.add_theme_color_override("font_disabled_color", Color("7d776c"))
		button.add_theme_color_override("font_pressed_color", Color("8a5520"))
		button.pressed.connect(func(): tier_pressed.emit(str(option.id)))
		row.add_child(button)
		var outcome := TOOLTIP_LABEL.new(); outcome.name = "TierOutcome"
		outcome.text = str(option.get("summary", "%d DMG +%d%s" % [int(option.get("damage", 0)), int(option.get("poison", 0)), BattlePresentationCatalog.status("poison").glyph]))
		outcome.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		outcome.add_theme_font_override("font", CINEMATIC.roll_control_font())
		outcome.add_theme_font_size_override("font_size", 11)
		outcome.add_theme_color_override("font_color", Color("7d776c") if button.disabled else CINEMATIC.INK)
		outcome.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		outcome.mouse_filter = Control.MOUSE_FILTER_IGNORE
		button.add_child(outcome)
		outcome.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE)
		outcome.offset_top = -19 - outcome.text.count("\n") * 14; outcome.offset_bottom = -3
	_upgrade_notice = TOOLTIP_LABEL.new()
	_upgrade_notice.name = "UpgradeNotice"
	_upgrade_notice.position = Vector2(5, -22)
	_upgrade_notice.size = Vector2(335, 22)
	_upgrade_notice.add_theme_font_size_override("font_size", 14)
	_upgrade_notice.add_theme_color_override("font_color", Color("d9ffa4"))
	_upgrade_notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_upgrade_notice.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_upgrade_notice)
	custom_minimum_size = Vector2(370, maxf(66, content.get_combined_minimum_size().y + 6)) if _compact else content.get_combined_minimum_size() + Vector2(12, 12)

# All non-Needlefang action variants share an inline row. The ability's rules
# remain above it; the outer tile cannot accidentally choose a paid alternative.
func configure_choices(options: Array[Dictionary]) -> void:
	disabled = true; focus_mode = Control.FOCUS_NONE; mouse_filter = Control.MOUSE_FILTER_IGNORE
	var available := false
	for option in options: available = available or bool(option.enabled)
	_style_availability(available)
	var body_height := custom_minimum_size.y
	if ability_id == "shedskin":
		# Payment is explicit in the two buttons, so don't repeat that line.
		_recipe_label.text = _recipe_label.text.get_slice("\nOptional:", 0)
		body_height -= 23
	if is_instance_valid(_recipe_label):
		_recipe_label.anchor_bottom = 0; _recipe_label.offset_bottom = body_height - 5
	if is_instance_valid(_offensive_summary):
		_offensive_summary.anchor_bottom = 0; _offensive_summary.offset_bottom = body_height - 4
	var choices := GridContainer.new(); choices.name = "AbilityChoices"
	choices.columns = mini(3, options.size())
	choices.add_theme_constant_override("h_separation", 5); choices.add_theme_constant_override("v_separation", 5)
	add_child(choices); choices.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
	choices.offset_left = 10; choices.offset_right = -10; choices.offset_top = body_height
	for option in options:
		var button := TOOLTIP_BUTTON.new(); button.text = str(option.label); button.tooltip_text = str(option.get("tooltip", option.label))
		button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		button.custom_minimum_size = Vector2(0, 48); button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		button.add_theme_font_size_override("font_size", 14)
		for state in ["normal", "hover", "pressed", "disabled"]:
			var style := CINEMATIC.panel(Color("ffffff18") if state == "disabled" else Color("ffefc980") if state == "normal" else Color("fff1c6"), Color("8a867b") if state == "disabled" else Color("ad7b29"), 3)
			button.add_theme_stylebox_override(state, style)
			button.add_theme_color_override("font_" + state + "_color" if state != "normal" else "font_color", Color("7d776c") if state == "disabled" else CINEMATIC.INK)
		button.disabled = not bool(option.enabled)
		button.set_meta("ability_action", option.get("action", {}))
		button.pressed.connect(func():
			if not button.disabled: choice_pressed.emit(option.get("action", {}))
		)
		choices.add_child(button)
	custom_minimum_size.y = body_height + ceili(options.size() / 3.0) * 53 + 8

func show_upgrade(amount: int, elapsed: float) -> void:
	if amount <= 0 or elapsed >= UPGRADE_DURATION or not is_instance_valid(_upgrade_notice): return
	_upgrade_notice.text = "+%d DAMAGE · ALL TIERS" % amount
	var glow := Panel.new()
	glow.name = "UpgradeGlow"
	glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	glow.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	var style := StyleBoxFlat.new()
	style.bg_color = Color("91c84a18")
	style.border_color = Color("cbf78e")
	style.set_border_width_all(2)
	style.set_corner_radius_all(6)
	style.shadow_color = Color("a7e15d66")
	style.shadow_size = 8
	glow.add_theme_stylebox_override("panel", style)
	add_child(glow)
	move_child(glow, 0)
	# Resume the fade after a board refresh; rolling must not restart it.
	var alpha := clampf((UPGRADE_DURATION - elapsed) / 1.1, 0.0, 1.0)
	glow.modulate.a = alpha
	_upgrade_notice.modulate.a = alpha
	var tween := create_tween()
	tween.tween_interval(maxf(0.0, 1.7 - elapsed))
	tween.tween_property(glow, "modulate:a", 0.0, minf(1.1, UPGRADE_DURATION - elapsed))
	tween.parallel().tween_property(_upgrade_notice, "modulate:a", 0.0, minf(1.1, UPGRADE_DURATION - elapsed))

func configure(id: String, qualified: bool, selected: bool, enabled: bool, actor: Dictionary = {}) -> void:
	ability_id = id
	_actor = actor
	var data := BattlePresentationCatalog.ability(id, actor)
	text = "%s\n%s" % [data.name, data.recipe]
	tooltip_text = BattlePresentationCatalog.ability_tooltip(id, actor)
	custom_minimum_size = Vector2(130, 70)
	disabled = not enabled
	modulate = Color.WHITE
	if selected: add_theme_color_override("font_color", Color("8a5520"))

func cinematic_compact() -> void:
	_compact = true
	custom_minimum_size = Vector2(370, 66)
	var data := BattlePresentationCatalog.ability(ability_id, _actor)
	var recipe := str(data.recipe).replace("Fang", "✧").replace("Gland", "⚗").replace("Coil", "◉").replace(" / ", "\n")
	text = "%s   %s" % [data.name, recipe]
	add_theme_font_size_override("font_size", 17)
	autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	alignment = HORIZONTAL_ALIGNMENT_LEFT
	CINEMATIC.paper_button(self)
	_style_availability(not disabled)
	for key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]: add_theme_color_override(key, Color.TRANSPARENT)
	_recipe_label = RichTextLabel.new(); _recipe_label.bbcode_enabled = true; _recipe_label.fit_content = false; _recipe_label.scroll_active = false; _recipe_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var illustrated := str(data.recipe).replace(" / ", "\n")
	for token in ["Fang", "Gland", "Coil"]: illustrated = illustrated.replace(token, "[img=23]res://assets/battle/wasteland/%s.svg[/img]" % token.to_lower())
	_recipe_label.text = "[b]%s[/b]   %s" % [data.name, illustrated]
	_recipe_label.add_theme_color_override("default_color", CINEMATIC.INK if not disabled else Color("4e4b46")); _recipe_label.add_theme_font_size_override("normal_font_size", 18); _recipe_label.add_theme_font_size_override("bold_font_size", 18)
	add_child(_recipe_label); _recipe_label.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _recipe_label.offset_left = 12; _recipe_label.offset_right = -32; _recipe_label.offset_top = 15; _recipe_label.offset_bottom = -5
	if data.type == "defensive": _show_defense_recipe(data)
	else: _show_offensive_recipe(data)
	var info := TOOLTIP_BUTTON.new(); info.set_meta("battle_utility", true); info.text = "ⓘ"; info.tooltip_text = tooltip_text; info.flat = true; info.add_theme_font_size_override("font_size", 22); info.add_theme_color_override("font_color", CINEMATIC.INK); info.add_theme_color_override("font_hover_color", Color("715216"))
	for style_name in ["normal", "disabled", "hover", "pressed"]: info.add_theme_stylebox_override(style_name, CINEMATIC.panel(Color("00000000"), Color("00000000"), 0))
	add_child(info); info.set_anchors_and_offsets_preset(Control.PRESET_TOP_RIGHT); info.offset_left = -29; info.offset_right = -4; info.offset_top = 4; info.offset_bottom = 30
	info.pressed.connect(func():
		var dialog := AcceptDialog.new(); dialog.title = BattlePresentationCatalog.ability(ability_id).name; dialog.dialog_text = tooltip_text; dialog.dialog_autowrap = true
		add_child(dialog); dialog.popup_centered(Vector2i(520, 240)); dialog.confirmed.connect(dialog.queue_free); dialog.canceled.connect(dialog.queue_free)
	)

func _style_availability(available: bool) -> void:
	# Needlefang's outer button is disabled intentionally; its tier legality
	# determines whether the whole ability should read as available.
	for state in ["normal", "hover", "pressed", "disabled"]:
		var tint := Color("b5b8ba")
		if available: tint = Color("fff4d8") if state in ["hover", "pressed"] else Color.WHITE
		add_theme_stylebox_override(state, CINEMATIC.paper(tint))
	var previous := get_node_or_null("AvailableOutline")
	if previous != null: remove_child(previous); previous.queue_free()
	if not available: return
	var outline := Panel.new(); outline.name = "AvailableOutline"
	outline.mouse_filter = Control.MOUSE_FILTER_IGNORE
	outline.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	var style := CINEMATIC.panel(Color("fff2b510"), Color("f4cf7b"), 0)
	style.set_border_width_all(2); style.shadow_color = Color("f4cf7b45"); style.shadow_size = 4
	outline.add_theme_stylebox_override("panel", style)
	add_child(outline)

func show_selected_attack(summary: String) -> void:
	# Replace the requirements with the selected attack's live outcome.
	if is_instance_valid(_offensive_summary): _offensive_summary.hide()
	_recipe_label.show()
	disabled = true
	_style_availability(true)
	custom_minimum_size.y = maxf(90, 48 + summary.count("\n") * 22 + 22)
	update_selected_attack_summary(summary)
	_recipe_label.add_theme_color_override("default_color", CINEMATIC.INK)
	_recipe_label.offset_top = 10
	if not _recipe_label.resized.is_connected(_fit_rules): _recipe_label.resized.connect(_fit_rules, CONNECT_DEFERRED)
	_fit_rules.call_deferred()
	text = BattlePresentationCatalog.ability(ability_id).name + "\n" + summary

func _show_offensive_recipe(data: Dictionary) -> void:
	var tiers := BattlePresentationCatalog.offensive_tier_summaries(ability_id, _actor)
	if tiers.is_empty():
		_show_rules(data)
		return
	if tiers.size() > 1 and tiers.all(func(tier): return tier.summary == tiers[0].summary):
		var recipes: Array[String] = []
		for tier in tiers: recipes.append(str(tier.recipe))
		tiers = [{"id": tiers[0].id, "recipe": " or ".join(recipes), "summary": tiers[0].summary}]
	_recipe_label.hide()
	text = str(data.name)
	_offensive_summary = VBoxContainer.new(); _offensive_summary.name = "OffensiveSummary"
	_offensive_summary.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_offensive_summary.add_theme_constant_override("separation", 1)
	add_child(_offensive_summary); _offensive_summary.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_offensive_summary.offset_left = 12; _offensive_summary.offset_right = -32
	_offensive_summary.offset_top = 4 if tiers.size() > 1 else 10; _offensive_summary.offset_bottom = -4
	var split_title := tiers.size() > 1 or str(data.name).length() + str(tiers[0].recipe).length() > 30
	if split_title:
		var title := TOOLTIP_LABEL.new(); title.text = str(data.name); title.add_theme_font_size_override("font_size", 18); title.add_theme_color_override("font_color", CINEMATIC.INK if not disabled else Color("4e4b46")); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); title.mouse_filter = Control.MOUSE_FILTER_IGNORE; _offensive_summary.add_child(title)
	var row := HBoxContainer.new(); row.mouse_filter = Control.MOUSE_FILTER_IGNORE; row.add_theme_constant_override("separation", 10); _offensive_summary.add_child(row)
	for tier in tiers:
		var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.mouse_filter = Control.MOUSE_FILTER_IGNORE; column.add_theme_constant_override("separation", 1); row.add_child(column)
		var recipe := RichTextLabel.new(); recipe.bbcode_enabled = true; recipe.fit_content = true; recipe.scroll_active = false; recipe.mouse_filter = Control.MOUSE_FILTER_IGNORE
		var illustrated := str(tier.recipe)
		for token in ["Fang", "Gland", "Coil"]: illustrated = illustrated.replace(token, "[img=%d]res://assets/battle/wasteland/%s.svg[/img]" % [16 if tiers.size() > 1 else 23, token.to_lower()])
		recipe.text = illustrated if split_title else "[b]%s[/b]   %s" % [data.name, illustrated]
		recipe.add_theme_font_size_override("normal_font_size", 14 if tiers.size() > 1 else 18); recipe.add_theme_font_size_override("bold_font_size", 18)
		recipe.add_theme_color_override("default_color", CINEMATIC.INK if not disabled else Color("4e4b46")); column.add_child(recipe)
		var outcome := TOOLTIP_LABEL.new(); outcome.name = "AbilityOutcome"; outcome.text = str(tier.summary); outcome.set_meta("tier_id", str(tier.id)); outcome.mouse_filter = Control.MOUSE_FILTER_IGNORE
		outcome.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		outcome.add_theme_font_override("font", CINEMATIC.roll_control_font()); outcome.add_theme_font_size_override("font_size", 13 if str(tier.summary).length() > 55 else 11)
		outcome.add_theme_color_override("font_color", CINEMATIC.INK if not disabled else Color("4e4b46")); outcome.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); column.add_child(outcome)

	_offensive_summary.minimum_size_changed.connect(_fit_offensive_summary, CONNECT_DEFERRED)
	_fit_offensive_summary.call_deferred()

func _show_defense_recipe(data: Dictionary) -> void:
	var lines: Array[String] = ["[b]%s[/b]" % data.name]
	lines.append_array(BattlePresentationCatalog.defense_lines(ability_id))
	_show_rules(data, "\n".join(lines))

func _show_rules(data: Dictionary, summary: String = "") -> void:
	if summary.is_empty(): summary = "[b]%s[/b]\n%s\n%s" % [data.name, data.recipe, data.text]
	text = summary.replace("[b]", "").replace("[/b]", "")
	for token in ["Fang", "Gland", "Coil"]:
		summary = summary.replace(token, "[img=19]res://assets/battle/wasteland/%s.svg[/img]" % token.to_lower())
	_recipe_label.text = summary
	_recipe_label.add_theme_font_size_override("normal_font_size", 17)
	_recipe_label.add_theme_font_size_override("bold_font_size", 18)
	_recipe_label.offset_top = 9
	custom_minimum_size.y = 18 + (summary.count("\n") + 1) * 23
	_recipe_label.resized.connect(_fit_rules, CONNECT_DEFERRED)
	_fit_rules.call_deferred()

func _set_body_height(height: float) -> void:
	var choices := get_node_or_null("AbilityChoices") as GridContainer
	if choices != null:
		_recipe_label.offset_bottom = height - 5
		if is_instance_valid(_offensive_summary): _offensive_summary.offset_bottom = height - 4
		choices.offset_top = height
		custom_minimum_size.y = height + ceili(choices.get_child_count() / 3.0) * 53 + 8
	else: custom_minimum_size.y = height

func _fit_rules() -> void:
	if _minimal: return
	if not is_instance_valid(_recipe_label) or not _recipe_label.visible: return
	_set_body_height(maxf(66, _recipe_label.get_content_height() + _recipe_label.offset_top + 8))

func _fit_offensive_summary() -> void:
	if _minimal: return
	if not is_instance_valid(_offensive_summary) or not _offensive_summary.visible: return
	_set_body_height(maxf(66, _offensive_summary.get_combined_minimum_size().y + _offensive_summary.offset_top + 8))

func update_selected_attack_summary(summary: String) -> void:
	_selected_summary = summary
	if _minimal:
		tooltip_text = BattlePresentationCatalog.ability_tooltip(ability_id, _actor)
		if not summary.is_empty(): tooltip_text += "\n\nSelected outcome\n" + summary
		return
	_recipe_label.text = "[b]%s · SELECTED[/b]\n%s" % [BattlePresentationCatalog.ability(ability_id).name, summary]
	text = BattlePresentationCatalog.ability(ability_id).name + "\n" + summary

## Minimal live action rail; detailed inspection and combat results retain their
## full presentation. Legality and action signals remain on the original buttons.
func minimal_rail() -> void:
	_minimal = true
	var data := BattlePresentationCatalog.ability(ability_id, _actor)
	tooltip_text = BattlePresentationCatalog.ability_tooltip(ability_id, _actor)
	if not _selected_summary.is_empty(): tooltip_text += "\n\nSelected outcome\n" + _selected_summary
	if is_instance_valid(_recipe_label): _recipe_label.hide()
	if is_instance_valid(_offensive_summary): _offensive_summary.hide()
	text = ""
	var outline := get_node_or_null("AvailableOutline")
	var available := outline != null
	if outline != null: remove_child(outline); outline.queue_free()
	_minimal_styles(self, available)
	var tier_controls := get_node_or_null("TierControls") as HBoxContainer
	if tier_controls != null:
		tier_controls.offset_left = 8; tier_controls.offset_right = -31
		tier_controls.anchor_bottom = 0; tier_controls.offset_bottom = 34
		tier_controls.alignment = BoxContainer.ALIGNMENT_BEGIN
		tier_controls.add_theme_constant_override("separation", 8)
		var title := tier_controls.get_child(0) as Label
		title.text = _short_name(str(data.name)); CINEMATIC.hud_lettering(title, true)
		title.name = "MinimalTitle"; title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var icon := _rail_icon(); tier_controls.add_child(icon); tier_controls.move_child(icon, 0)
		for button in tier_controls.find_children("*", "Button", true, false):
			_minimal_styles(button, not button.disabled)
			button.custom_minimum_size = Vector2(57, 32)
			if ability_id == "hexbrand":
				button.text = str(button.get_meta("tier_id")).trim_prefix("skull_")
				button.icon = preload("res://presentation/battle/battle_icons.gd").texture("curse_count")
				button.icon_alignment = HORIZONTAL_ALIGNMENT_RIGHT
				button.add_theme_constant_override("icon_max_width", 20)
				button.add_theme_color_override("icon_disabled_color", Color("c4bbd0"))
			var outcome := button.get_node_or_null("TierOutcome")
			if outcome != null: outcome.hide()
		for label in tier_controls.find_children("*", "Label", true, false):
			if label.text == "/": label.hide()
	else:
		var row := HBoxContainer.new(); row.name = "MinimalAbility"; row.mouse_filter = Control.MOUSE_FILTER_IGNORE
		add_child(row); row.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
		row.offset_left = 8; row.offset_right = -31; row.offset_top = 0; row.offset_bottom = 34
		row.add_theme_constant_override("separation", 8); row.add_child(_rail_icon())
		var title := TOOLTIP_LABEL.new(); title.text = _short_name(str(data.name)); title.add_theme_font_size_override("font_size", 18)
		title.name = "MinimalTitle"; CINEMATIC.hud_lettering(title, true)
		title.size_flags_horizontal = Control.SIZE_EXPAND_FILL; title.autowrap_mode = TextServer.AUTOWRAP_OFF; title.mouse_filter = Control.MOUSE_FILTER_IGNORE; row.add_child(title)
		var recipe := TOOLTIP_LABEL.new(); recipe.name = "MinimalRequirement"; recipe.text = _symbol_recipe(str(data.recipe)).replace("\n", " / ")
		var tiers := BattlePresentationCatalog.inline_tiers(ability_id, _actor)
		if not tiers.is_empty():
			var requirements: Array[String] = []
			for tier in tiers: requirements.append(str(tier.get("button_label", tier.label)))
			recipe.text = "   ".join(requirements)
		recipe.add_theme_font_size_override("font_size", 18); recipe.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		recipe.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		CINEMATIC.hud_lettering(recipe, true)
		recipe.mouse_filter = Control.MOUSE_FILTER_IGNORE; row.add_child(recipe)
	var choices := get_node_or_null("AbilityChoices") as GridContainer
	custom_minimum_size = Vector2(370, 36)
	if choices != null:
		choices.offset_top = 36
		custom_minimum_size.y += ceili(choices.get_child_count() / 3.0) * 53 + 8
		for button in choices.get_children(): _minimal_styles(button, not button.disabled)
	var temporary := BattlePresentationCatalog.temporary_ability_damage(ability_id, _actor)
	if temporary > 0:
		var badge := TOOLTIP_LABEL.new(); badge.name = "TemporaryDamageBonus"
		badge.text = "+%d DMG" % temporary
		badge.add_theme_font_size_override("font_size", 14); CINEMATIC.hud_lettering(badge, true)
		badge.add_theme_color_override("font_color", Color("a5edce")); badge.mouse_filter = Control.MOUSE_FILTER_IGNORE
		var title := find_child("MinimalTitle", true, false) as Label
		var row := title.get_parent() as HBoxContainer
		var title_index := title.get_index()
		row.remove_child(title)
		var heading := HBoxContainer.new(); heading.name = "AbilityHeading"
		heading.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		heading.mouse_filter = Control.MOUSE_FILTER_IGNORE
		heading.add_theme_constant_override("separation", 6)
		row.add_child(heading); row.move_child(heading, title_index)
		title.size_flags_horizontal = Control.SIZE_SHRINK_BEGIN
		heading.add_child(title); heading.add_child(badge)
		var bonus_rules := BattlePresentationCatalog.temporary_ability_damage_rules(ability_id, _actor)
		for button in find_children("*", "Button", true, false):
			if not button.tooltip_text.contains(bonus_rules): button.tooltip_text += "\n\n" + bonus_rules
	for child in get_children():
		if child is Button and child.get_meta("battle_utility", false):
			CINEMATIC.hud_lettering(child)
			child.add_theme_color_override("font_color", Color("e5d5ae")); child.add_theme_color_override("font_hover_color", Color("ffe7a1"))
			child.tooltip_text = tooltip_text

	if not resized.is_connected(_fit_minimal_layout): resized.connect(_fit_minimal_layout)
	_fit_minimal_layout()
	_fit_minimal_layout.call_deferred()

func _fit_minimal_layout() -> void:
	var row := get_node_or_null("TierControls") as HBoxContainer
	if row == null: row = get_node_or_null("MinimalAbility") as HBoxContainer
	if row == null: return
	var heading := find_child("AbilityHeading", true, false) as HBoxContainer
	var badge := find_child("TemporaryDamageBonus", true, false) as Label
	var requirement := find_child("MinimalRequirement", true, false) as Control
	if requirement == null: requirement = find_child("MinimalTiers", true, false) as Control
	var available_width := maxf(custom_minimum_size.x, size.x) - 39
	var body_height := 36.0
	# Use natural, unwrapped widths. Move whole pieces to another line rather
	# than allowing a HBox to crush an ability name into individual letters.
	if requirement != null:
		var base_width := row.get_combined_minimum_size().x
		if badge != null and badge.get_parent() == heading: base_width -= badge.get_combined_minimum_size().x + 6
		if requirement.get_parent() != row: base_width += requirement.get_combined_minimum_size().x + 8
		if base_width > available_width:
			if requirement.get_parent() != self: requirement.reparent(self)
			requirement.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
			requirement.offset_left = 8 if requirement is HBoxContainer else 41; requirement.offset_right = -31
			var line_height := maxf(24, requirement.get_combined_minimum_size().y)
			requirement.offset_top = 34; requirement.offset_bottom = 34 + line_height
			body_height = 36 + line_height
		elif requirement.get_parent() != row:
			requirement.reparent(row)
	if badge != null and heading != null:
		var natural_width := row.get_combined_minimum_size().x
		if badge.get_parent() != heading: natural_width += badge.get_combined_minimum_size().x + 6
		if natural_width > available_width:
			if badge.get_parent() != self: badge.reparent(self)
			badge.set_anchors_and_offsets_preset(Control.PRESET_TOP_LEFT)
			badge.position = Vector2(41, body_height - 2)
			badge.size = badge.get_combined_minimum_size()
			body_height += 20
		elif badge.get_parent() != heading:
			badge.reparent(heading)
	# Clear stale container bounds after moving a child to a separate line.
	row.set_deferred("size", Vector2(available_width, 34 - row.offset_top))
	var choices := get_node_or_null("AbilityChoices") as GridContainer
	if choices != null:
		choices.offset_top = body_height
		body_height += ceili(choices.get_child_count() / 3.0) * 53 + 8
	custom_minimum_size.y = body_height

func _short_name(full_name: String) -> String:
	return {"grasp_of_the_sarcophagus": "Grasp", "funeral_rattle": "Rattle", "eclipse_of_the_black_star": "Eclipse"}.get(ability_id, full_name)

func _symbol_recipe(recipe: String) -> String:
	var known := {"grasp_of_the_sarcophagus": "2 ▧ + 1 ✦", "funeral_rattle": "2 ☠ + ▧ + ✦", "eclipse_of_the_black_star": "1–5 / 2–6"}
	if known.has(ability_id): return known[ability_id]
	for id in BattlePresentationCatalog._catalog.get("symbols", {}):
		var symbol := BattlePresentationCatalog.definition("symbols", str(id))
		var name := str(symbol.get("name", "")); var glyph := str(symbol.get("glyph", ""))
		if not name.is_empty() and not glyph.is_empty(): recipe = recipe.replace(name, glyph)
	return recipe.replace(" / ", "\n")

func _rail_icon() -> TextureRect:
	var icon := TextureRect.new(); icon.custom_minimum_size = Vector2(25, 28)
	icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var key: String = {"hexbrand": "curse_count", "grasp_of_the_sarcophagus": "grave_debt", "funeral_rattle": "second_knell", "eclipse_of_the_black_star": "blind", "needlefang": "bleed", "venom_gland": "poison", "fever_spike": "catalyst", "terminal_bite": "incubation"}.get(ability_id, "energy")
	icon.texture = preload("res://presentation/battle/battle_icons.gd").texture(key)
	return icon

func _minimal_styles(button: Button, available: bool) -> void:
	CINEMATIC.hud_lettering(button, true)
	for state in ["normal", "hover", "pressed", "disabled"]:
		var lit: bool = available and state != "disabled"
		var fill := Color("ddbd6925") if lit and state in ["hover", "pressed"] else Color("ddbd6910") if lit else Color.TRANSPARENT
		button.add_theme_stylebox_override(state, CINEMATIC.panel(fill, Color("bfa16c") if lit else Color.TRANSPARENT, 3))
		button.add_theme_color_override("font_" + state + "_color" if state != "normal" else "font_color", Color("ffe4a0") if lit else Color("c9c0b0"))
	button.add_theme_color_override("font_hover_pressed_color", Color("ffe4a0"))
	button.add_theme_color_override("font_focus_color", Color("ffe4a0"))
