class_name BattleAbilityTile
extends Button

var ability_id := ""
var _upgrade_notice: Label
var _compact := false
var _recipe_label: RichTextLabel
var _offensive_summary: VBoxContainer
const CINEMATIC := preload("res://presentation/battle/cinematic_theme.gd")
const UPGRADE_DURATION := 2.8
signal tier_pressed(tier_id: String)

# The tile remains the visual/tooltip container; only the individual tiers
# receive clicks and keyboard focus in this mode.
func configure_tiers(options: Array[Dictionary], selected_tier: String) -> void:
	text = ""
	if is_instance_valid(_recipe_label): _recipe_label.hide()
	disabled = true
	focus_mode = Control.FOCUS_NONE
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	modulate = Color.WHITE
	var any_available := false
	for option in options: any_available = any_available or bool(option.get("enabled", false))
	_style_availability(any_available)
	var content := HBoxContainer.new()
	add_child(content)
	content.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	content.offset_left = 12 if _compact else 6; content.offset_right = -32
	content.offset_top = 3; content.offset_bottom = -3
	content.alignment = BoxContainer.ALIGNMENT_CENTER
	var title := Label.new()
	title.text = BattlePresentationCatalog.ability(ability_id).name
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_LEFT
	title.add_theme_font_size_override("font_size", 18); title.add_theme_color_override("font_color", CINEMATIC.INK if any_available else Color("4e4b46")); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
	title.tooltip_text = tooltip_text
	title.mouse_filter = Control.MOUSE_FILTER_PASS
	content.add_child(title)
	var row := HBoxContainer.new()
	row.alignment = BoxContainer.ALIGNMENT_BEGIN
	row.add_theme_constant_override("separation", 2)
	content.add_child(row)
	for option in options:
		if row.get_child_count() > 0:
			var slash := Label.new(); slash.text = "/"; slash.add_theme_color_override("font_color", CINEMATIC.INK); row.add_child(slash)
		var button := Button.new()
		button.text = str(option.label).get_slice(" ", 0); button.icon = preload("res://assets/battle/wasteland/fang.svg"); button.expand_icon = true; button.custom_minimum_size = Vector2(64, 36); button.add_theme_constant_override("icon_max_width", 22)
		button.add_theme_font_size_override("font_size", 20); button.add_theme_color_override("font_color", CINEMATIC.INK); button.add_theme_color_override("font_hover_color", Color("715216"))
		for style_name in ["normal", "hover", "pressed", "disabled"]:
			var style := CINEMATIC.panel(Color.TRANSPARENT if style_name == "disabled" else Color("ffefc980") if style_name == "normal" else Color("fff1c6"), Color.TRANSPARENT if style_name == "disabled" else Color("ad7b29"), 3)
			style.content_margin_bottom = 20
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
		var outcome := Label.new(); outcome.name = "TierOutcome"
		outcome.text = "%d DMG +%d%s" % [int(option.get("damage", 0)), int(option.get("poison", 0)), BattlePresentationCatalog.status("poison").glyph]
		outcome.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		outcome.add_theme_font_override("font", CINEMATIC.roll_control_font())
		outcome.add_theme_font_size_override("font_size", 11)
		outcome.add_theme_color_override("font_color", Color("7d776c") if button.disabled else CINEMATIC.INK)
		outcome.add_theme_color_override("font_shadow_color", Color.TRANSPARENT)
		outcome.mouse_filter = Control.MOUSE_FILTER_IGNORE
		button.add_child(outcome)
		outcome.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_WIDE)
		outcome.offset_top = -19; outcome.offset_bottom = -3
	_upgrade_notice = Label.new()
	_upgrade_notice.name = "UpgradeNotice"
	_upgrade_notice.position = Vector2(5, -22)
	_upgrade_notice.size = Vector2(335, 22)
	_upgrade_notice.add_theme_font_size_override("font_size", 14)
	_upgrade_notice.add_theme_color_override("font_color", Color("d9ffa4"))
	_upgrade_notice.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_upgrade_notice.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_upgrade_notice)
	custom_minimum_size = Vector2(370, 66) if _compact else content.get_combined_minimum_size() + Vector2(12, 12)

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
	var data := BattlePresentationCatalog.ability(id, actor)
	text = "%s\n%s" % [data.name, data.recipe]
	tooltip_text = "%s — %s" % [data.name, data.text]
	custom_minimum_size = Vector2(130, 70)
	disabled = not enabled
	modulate = Color.WHITE
	if selected: add_theme_color_override("font_color", Color("8a5520"))

func cinematic_compact() -> void:
	_compact = true
	custom_minimum_size = Vector2(370, 66)
	var data := BattlePresentationCatalog.ability(ability_id)
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
	if ability_id in ["shedskin", "barbed_mantle"]: _show_defense_recipe(data)
	elif ability_id in ["venom_gland", "fever_spike", "terminal_bite"]: _show_offensive_recipe(data)
	var info := Button.new(); info.set_meta("battle_utility", true); info.text = "ⓘ"; info.tooltip_text = tooltip_text; info.flat = true; info.add_theme_font_size_override("font_size", 22); info.add_theme_color_override("font_color", CINEMATIC.INK); info.add_theme_color_override("font_hover_color", Color("715216"))
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
	text = BattlePresentationCatalog.ability(ability_id).name + "\n" + summary

func _show_offensive_recipe(data: Dictionary) -> void:
	var tiers := BattlePresentationCatalog.offensive_tier_summaries(ability_id)
	if tiers.is_empty(): return
	_recipe_label.hide()
	text = str(data.name)
	_offensive_summary = VBoxContainer.new(); _offensive_summary.name = "OffensiveSummary"
	_offensive_summary.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_offensive_summary.add_theme_constant_override("separation", 1)
	add_child(_offensive_summary); _offensive_summary.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_offensive_summary.offset_left = 12; _offensive_summary.offset_right = -32
	_offensive_summary.offset_top = 4 if tiers.size() > 1 else 10; _offensive_summary.offset_bottom = -4
	if tiers.size() > 1:
		var title := Label.new(); title.text = str(data.name); title.add_theme_font_size_override("font_size", 18); title.add_theme_color_override("font_color", CINEMATIC.INK if not disabled else Color("4e4b46")); title.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); title.mouse_filter = Control.MOUSE_FILTER_IGNORE; _offensive_summary.add_child(title)
	var row := HBoxContainer.new(); row.mouse_filter = Control.MOUSE_FILTER_IGNORE; row.add_theme_constant_override("separation", 10); _offensive_summary.add_child(row)
	for tier in tiers:
		var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.mouse_filter = Control.MOUSE_FILTER_IGNORE; column.add_theme_constant_override("separation", 1); row.add_child(column)
		var recipe := RichTextLabel.new(); recipe.bbcode_enabled = true; recipe.fit_content = true; recipe.scroll_active = false; recipe.mouse_filter = Control.MOUSE_FILTER_IGNORE
		var illustrated := str(tier.recipe)
		for token in ["Fang", "Gland", "Coil"]: illustrated = illustrated.replace(token, "[img=%d]res://assets/battle/wasteland/%s.svg[/img]" % [16 if tiers.size() > 1 else 23, token.to_lower()])
		recipe.text = illustrated if tiers.size() > 1 else "[b]%s[/b]   %s" % [data.name, illustrated]
		recipe.add_theme_font_size_override("normal_font_size", 14 if tiers.size() > 1 else 18); recipe.add_theme_font_size_override("bold_font_size", 18)
		recipe.add_theme_color_override("default_color", CINEMATIC.INK if not disabled else Color("4e4b46")); column.add_child(recipe)
		var outcome := Label.new(); outcome.name = "AbilityOutcome"; outcome.text = str(tier.summary); outcome.set_meta("tier_id", str(tier.id)); outcome.mouse_filter = Control.MOUSE_FILTER_IGNORE
		outcome.add_theme_font_override("font", CINEMATIC.roll_control_font()); outcome.add_theme_font_size_override("font_size", 11)
		outcome.add_theme_color_override("font_color", CINEMATIC.INK if not disabled else Color("686257")); outcome.add_theme_color_override("font_shadow_color", Color.TRANSPARENT); column.add_child(outcome)

func _show_defense_recipe(data: Dictionary) -> void:
	var lines: Array[String]
	if ability_id == "shedskin":
		lines = ["[b]%s[/b] · 2d6" % data.name,
			"Each Fang  Prevent 1",
			"Each Gland  Gain 1 Catalyst",
			"Any Coil  Apply 1 Incubation",
			"Optional: pay 1 Catalyst → prevent 2 more"]
	else:
		lines = ["[b]%s[/b] · 1d6 · 1 Energy" % data.name,
			"Fang  Prevent 2 · Apply 1 Poison",
			"Gland  Prevent 3 · Gain 1 Catalyst",
			"Coil  Prevent 1 · Apply 1 Incubation",
			"  if poisoned, no Incubation;",
			"  otherwise apply 1 Poison"]
	var summary := "\n".join(lines)
	text = summary.replace("[b]", "").replace("[/b]", "")
	for token in ["Fang", "Gland", "Coil"]:
		summary = summary.replace(token, "[img=19]res://assets/battle/wasteland/%s.svg[/img]" % token.to_lower())
	_recipe_label.text = summary
	_recipe_label.add_theme_font_size_override("normal_font_size", 17)
	_recipe_label.add_theme_font_size_override("bold_font_size", 18)
	_recipe_label.offset_top = 9
	_recipe_label.offset_right = -12
	custom_minimum_size.y = 18 + lines.size() * 25

func update_selected_attack_summary(summary: String) -> void:
	_recipe_label.text = "[b]%s · SELECTED[/b]\n%s" % [BattlePresentationCatalog.ability(ability_id).name, summary]
	text = BattlePresentationCatalog.ability(ability_id).name + "\n" + summary
