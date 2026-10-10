extends Control
const STONE_DIE := preload("res://presentation/dice/stone_die.gd")

signal closed
const UpgradeComparison := preload("res://app/screens/character/upgrade_comparison.gd")
const STYLE = preload("res://app/screens/character/character_style.gd")
const CARD_TIMING = preload("res://presentation/cards/card_timing.gd")
var _comparison: PanelContainer
const GOLD := STYLE.GOLD
const MUTED := STYLE.MUTED
const ROSTER := ["adventurer", "venom", "curse", "blade_warden"]
var initial_character := "adventurer"
var loadout_mode := "sandbox"
var back_label := "Back to battle setup"
## The campaign spends earned XP only, so it opens these editors locked to
## Progression; Sandbox's free deck edits would bypass XP.
var lock_progression := false
var _catalog_mode := "sandbox"
var _mode_choice: OptionButton
var _mode_note: Label
var _purchase_overlay: Control
var _purchase_details: VBoxContainer
var _purchase_confirm: Button
var _purchase_cancel: Button
var _pending_purchase: Dictionary = {}
var _admin_button: Button
var _admin_overlay: Control
var _admin_content: VBoxContainer
var _admin_prices: Dictionary = {}
var _admin_budgets: Dictionary = {}
var _admin_draft: Dictionary = {}
var _admin_preview: Label
var _admin_error: Label
var _admin_save: Button
var _admin_types: Dictionary = {}
var _admin_tabs: TabBar
var _admin_search: LineEdit
var _admin_sort: OptionButton
var _admin_type_filter: OptionButton
var _admin_sort_by_type := false
var _admin_filter_type := ""
var _admin_items: VBoxContainer
var _admin_item_rows: Dictionary = {}
var _admin_group_labels: Dictionary = {}
var _admin_empty: Label
var _admin_item_details: VBoxContainer
var _admin_item_context: Label
var _admin_item_kind := ""
var _admin_item_id := ""
var _card_admin_token := ""
var _delete_card_dialog: ConfirmationDialog
var _skip_prompt: CheckBox
var _confirmation_options: HBoxContainer
var _confirm_buy: CheckBox
var _confirm_sell: CheckBox
var _transaction_preferences := ConfigFile.new()
var character_id := ""
var catalogs: Dictionary = {}
var character: Dictionary = {}
var selected_kind := ""
var selected_id := ""
var _previous_catalog: Dictionary
var _list: VBoxContainer
var _details: VBoxContainer
var _summary: VBoxContainer
var _portrait: TextureRect
var _tabs: TabBar
var _last_character_tab := 1
var _card_workspace: VBoxContainer
var _card_split: HSplitContainer
var _library_pane: VBoxContainer
var _deck_pane: VBoxContainer
var _library_search: LineEdit
var _deck_search: LineEdit
var _library_list: VBoxContainer
var _deck_list: VBoxContainer
var _library_heading: Label
var _deck_heading: Label
var _swap: Button
var _library_buttons: Array[Button] = []
var _deck_buttons: Array[Button] = []
var _roster_buttons: Dictionary = {}
var _entry_buttons: Array[Button] = []
var _error: Label
var _drafts: Dictionary = {}
var _saved: Dictionary = {}
var _apply: Button
var _revert: Button
var _reset: Button
var _save_status: Label
var _quantity: SpinBox
var _confirm: Control
var _cancel_changes: Button
var _discard_changes: Button
var _add_copy: Button
var _pending_action: Callable
var _tab_hint: Label
var _deck_curve: HBoxContainer
var _portrait_name: Label
var _portrait_note: Label
var _roster_icons: Dictionary = {}
## Card tree names by ID, for labelling shared cards' per-tree copies.
var _tree_names: Dictionary = {}
const TAB_HINTS := [
	"Your equipped ability board. Select an ability for its tiers and upgrades.",
	"",
	"Dice rolled for your abilities. Each face shows its number and symbol.",
]

func _ready() -> void:
	name = "CharacterCreation"
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_previous_catalog = BattlePresentationCatalog._catalog.duplicate(true)
	_transaction_preferences.load(WorkspacePaths.persistent_file("character_preferences.cfg"))
	theme = STYLE.theme()
	_build()
	reload_catalogs()

func _style(fill: Variant, border: Variant = "293a47", padding: int = 16) -> StyleBoxFlat:
	var s := STYLE.box(fill, border, padding, 10)
	s.content_margin_top = padding; s.content_margin_bottom = padding
	return s

func _label(parent: Node, text: String, font_size: int = 18, color: Color = Color("e9e6de")) -> Label:
	var l := Label.new(); l.text = text; l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	l.add_theme_font_size_override("font_size", font_size); l.add_theme_color_override("font_color", color)
	parent.add_child(l); return l

func _button(parent: Node, text: String, action: Callable, key: String) -> Button:
	var b := Button.new(); b.text = text; b.custom_minimum_size.y = 42
	b.add_theme_font_size_override("font_size", 16)
	b.pressed.connect(action); parent.add_child(b)
	b.set_meta("character_control", key)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control("character." + key, b, text)
	return b

func _panel(parent: Node, width: float = 0) -> VBoxContainer:
	var p := PanelContainer.new(); p.add_theme_stylebox_override("panel", _style("101b25", "1f2e3a"))
	p.custom_minimum_size.x = width; p.size_flags_vertical = Control.SIZE_EXPAND_FILL
	parent.add_child(p)
	var v := VBoxContainer.new(); v.add_theme_constant_override("separation", 12); p.add_child(v)
	return v

func _scroll(parent: Node) -> VBoxContainer:
	var s := ScrollContainer.new(); s.size_flags_vertical = Control.SIZE_EXPAND_FILL
	s.size_flags_horizontal = Control.SIZE_EXPAND_FILL; s.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; s.follow_focus = true
	parent.add_child(s)
	var v := VBoxContainer.new(); v.size_flags_horizontal = Control.SIZE_EXPAND_FILL; v.add_theme_constant_override("separation", 8); s.add_child(v)
	return v

func _build() -> void:
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var glow := TextureRect.new(); glow.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var gradient := Gradient.new(); gradient.set_color(0, Color(0.32, 0.22, 0.12, 0.22)); gradient.set_color(1, Color(0, 0, 0, 0))
	var glow_texture := GradientTexture2D.new(); glow_texture.gradient = gradient; glow_texture.fill = GradientTexture2D.FILL_RADIAL
	glow_texture.fill_from = Vector2(0.5, 0.0); glow_texture.fill_to = Vector2(0.5, 0.75)
	glow.texture = glow_texture; glow.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; add_child(glow)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 22)
	add_child(margin)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 14); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	_label(titles, "DICE & DESTINY  /  CHARACTERS", 12, GOLD)
	_label(titles, "Character Creation", 34)
	var mode_box := VBoxContainer.new(); mode_box.add_theme_constant_override("separation", 2); mode_box.size_flags_vertical = Control.SIZE_SHRINK_CENTER; header.add_child(mode_box)
	_label(mode_box, "MODE", 11, MUTED)
	_mode_choice = OptionButton.new(); _mode_choice.add_item("Sandbox · free editing"); _mode_choice.add_item("Progression · XP")
	_mode_choice.custom_minimum_size = Vector2(230, 40)
	_mode_choice.tooltip_text = "Sandbox edits decks freely. Progression buys and sells with XP."
	_mode_choice.select(1 if loadout_mode == "progression" else 0); mode_box.add_child(_mode_choice)
	_mode_choice.item_selected.connect(_change_mode)
	if lock_progression:
		_mode_choice.disabled = true
		_mode_choice.tooltip_text = "The campaign spends earned XP only."
	_button(header, "Reload definitions", func(): _guard_unsaved(reload_catalogs), "reload").size_flags_vertical = Control.SIZE_SHRINK_END
	_button(header, back_label, _close, "back").size_flags_vertical = Control.SIZE_SHRINK_END
	_error = _label(body, "", 16, Color("ffd2c8")); _error.hide()
	_error.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.LOSS, 0.12), Color(STYLE.LOSS, 0.6), 14, 8))
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 16)
	columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	var left := _panel(columns, 250)
	_label(left, "YOUR CHARACTERS", 12, GOLD)
	for id in ROSTER:
		var b := _button(left, id.replace("_", " ").capitalize(), func(): select_character(id), "select." + id)
		b.toggle_mode = true; b.alignment = HORIZONTAL_ALIGNMENT_LEFT; b.custom_minimum_size.y = 58
		b.add_theme_constant_override("icon_max_width", 40); b.add_theme_constant_override("h_separation", 12)
		b.add_theme_font_size_override("font_size", 17)
		b.add_theme_stylebox_override("pressed", STYLE.box("223442", STYLE.GOLD, 10, 10, 2))
		b.add_theme_stylebox_override("hover_pressed", STYLE.box("2a3e4d", STYLE.GOLD_BRIGHT, 10, 10, 2))
		b.icon = _roster_icon(id)
		_roster_buttons[id] = b
	# The selected character stands in a framed showcase below the roster.
	var showcase := PanelContainer.new(); showcase.size_flags_vertical = Control.SIZE_EXPAND_FILL
	showcase.add_theme_stylebox_override("panel", STYLE.box("0a121a", STYLE.BRONZE, 10, 10, 1)); left.add_child(showcase)
	var stage := VBoxContainer.new(); stage.add_theme_constant_override("separation", 4); showcase.add_child(stage)
	_portrait = TextureRect.new(); _portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; _portrait.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_portrait.custom_minimum_size.y = 60; stage.add_child(_portrait)
	_portrait_name = _label(stage, "", 18, GOLD); _portrait_name.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_portrait_note = _label(stage, "Every card is a point of health.", 13, MUTED); _portrait_note.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	var middle := _panel(columns); middle.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_summary = VBoxContainer.new(); _summary.add_theme_constant_override("separation", 8); middle.add_child(_summary)
	_tabs = TabBar.new(); _tabs.add_tab("Abilities"); _tabs.add_tab("Deck & Library"); _tabs.add_tab("Dice"); _tabs.add_tab("Card Creation"); _tabs.add_tab("Ability Creation"); _tabs.add_tab("Card Trees"); _tabs.current_tab = 1
	_tabs.clip_tabs = false
	var launch := STYLE.launch_icon()
	for index in [3, 4, 5]:
		_tabs.set_tab_icon(index, launch)
		_tabs.set_tab_tooltip(index, ["Opens the card workshop to design and publish cards.", "Opens the ability workshop to design abilities and boards.", "Opens card trees: upgrade and downgrade paths for cards."][index - 3])
	_tabs.set_tab_tooltip(0, "Your equipped offensive and defensive abilities.")
	_tabs.set_tab_tooltip(1, "Build your deck from the card library.")
	_tabs.set_tab_tooltip(2, "The dice you roll.")
	middle.add_child(_tabs)
	_tabs.tab_changed.connect(func(tab):
		if tab in [3, 4, 5]:
			_tabs.set_block_signals(true); _tabs.current_tab = _last_character_tab; _tabs.set_block_signals(false)
			_guard_unsaved(_open_card_workshop if tab == 3 else (_open_ability_workshop if tab == 4 else _open_card_trees))
		else: _last_character_tab = tab; _populate())
	_tab_hint = _label(middle, "", 14, MUTED)
	_list = _scroll(middle)
	_card_workspace = VBoxContainer.new(); _card_workspace.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_card_workspace.add_theme_constant_override("separation", 10); middle.add_child(_card_workspace)
	var card_toolbar := HBoxContainer.new(); _card_workspace.add_child(card_toolbar)
	_label(card_toolbar, "Use + and − on any row to change copies, or select a card for details and upgrades.", 14, MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_swap = _button(card_toolbar, "Swap sides  ⇄", _swap_card_sides, "swap_sides")
	_card_split = HSplitContainer.new(); _card_split.size_flags_vertical = Control.SIZE_EXPAND_FILL
	_card_split.add_theme_constant_override("separation", 16); _card_workspace.add_child(_card_split)
	_library_pane = VBoxContainer.new(); _library_pane.custom_minimum_size.x = 300
	_library_pane.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _library_pane.add_theme_constant_override("separation", 8); _card_split.add_child(_library_pane)
	var library_head := HBoxContainer.new(); library_head.custom_minimum_size.y = 58; _library_pane.add_child(library_head)
	_library_heading = _label(library_head, "CARD LIBRARY", 15, GOLD); _library_heading.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_library_heading.size_flags_vertical = Control.SIZE_SHRINK_END
	_library_search = LineEdit.new(); _library_search.placeholder_text = "Search library…"; _library_search.custom_minimum_size.y = 38
	_library_search.clear_button_enabled = true
	_library_pane.add_child(_library_search); _library_search.text_changed.connect(func(_text): _populate_entries())
	_library_list = _scroll(_library_pane)
	_deck_pane = VBoxContainer.new(); _deck_pane.custom_minimum_size.x = 300
	_deck_pane.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _deck_pane.add_theme_constant_override("separation", 8); _card_split.add_child(_deck_pane)
	var deck_head := HBoxContainer.new(); deck_head.add_theme_constant_override("separation", 10); deck_head.custom_minimum_size.y = 58; _deck_pane.add_child(deck_head)
	_deck_heading = _label(deck_head, "YOUR DECK", 15, GOLD); _deck_heading.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_deck_heading.size_flags_vertical = Control.SIZE_SHRINK_END
	_deck_curve = HBoxContainer.new(); _deck_curve.add_theme_constant_override("separation", 3); _deck_curve.tooltip_text = "Energy curve: copies in your deck by energy cost."
	deck_head.add_child(_deck_curve)
	_deck_search = LineEdit.new(); _deck_search.placeholder_text = "Search deck…"; _deck_search.custom_minimum_size.y = 38
	_deck_search.clear_button_enabled = true
	_deck_pane.add_child(_deck_search); _deck_search.text_changed.connect(func(_text): _populate_entries())
	_deck_list = _scroll(_deck_pane)
	var right := _panel(columns, 400)
	_label(right, "INSPECT & CONFIGURE", 12, GOLD)
	_details = _scroll(right)
	_details.add_theme_constant_override("separation", 10)
	var footer_panel := PanelContainer.new(); footer_panel.add_theme_stylebox_override("panel", STYLE.box("0c1620", "1f2e3a", 14, 10)); body.add_child(footer_panel)
	var footer := HBoxContainer.new(); footer.add_theme_constant_override("separation", 12); footer_panel.add_child(footer)
	_save_status = _label(footer, "", 15, MUTED); _save_status.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _save_status.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_confirmation_options = HBoxContainer.new(); _confirmation_options.add_theme_constant_override("separation", 10); footer.add_child(_confirmation_options)
	_confirm_buy = _confirmation_toggle(_confirmation_options, "Confirm buys", "buy_card")
	_confirm_sell = _confirmation_toggle(_confirmation_options, "Confirm sales", "sell_card")
	_admin_button = _button(footer, "Admin settings", _open_admin, "admin.open")
	_reset = _button(footer, "Reset to template", _reset_draft, "reset")
	_revert = _button(footer, "Revert", _revert_draft, "revert")
	_apply = STYLE.accent(_button(footer, "Apply deck", _apply_draft, "apply"))
	_apply.custom_minimum_size.x = 150
	_mode_note = _label(body, "", 13, MUTED)
	_confirm = Control.new(); _confirm.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_confirm)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.75); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _confirm.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _confirm.add_child(center)
	var dialog := _panel(center, 580)
	dialog.get_parent().add_theme_stylebox_override("panel", _style("101b25", STYLE.BRONZE, 24))
	_label(dialog, "Unsaved deck changes", 28, GOLD)
	_label(dialog, "Discard your unsaved changes? Return to apply the decks you want to keep. Saved decks will stay unchanged.", 18)
	var choices := HBoxContainer.new(); choices.add_theme_constant_override("separation", 12); dialog.add_child(choices)
	_cancel_changes = _button(choices, "Keep editing", func(): _confirm.hide(), "keep_editing")
	_discard_changes = STYLE.accent(_button(choices, "Discard changes", func(): _confirm.hide(); _pending_action.call(), "discard_changes"), STYLE.LOSS)
	for b in [_cancel_changes, _discard_changes]: b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_focus_pair(_cancel_changes, _discard_changes)
	_confirm.hide()
	_build_purchase_overlay()
	_build_admin_overlay()
	_comparison = UpgradeComparison.new(); add_child(_comparison)

func _roster_icon(id: String) -> Texture2D:
	var tex := STYLE.texture("res://assets/battle/fighters/%s.png" % ("blade_warden" if id == "adventurer" else id))
	if tex == null: return null
	var image := tex.get_image()
	if image == null: return null
	image = image.duplicate() as Image
	if image.is_compressed(): image.decompress()
	# Crop the upper body so small icons show faces rather than whole figures.
	var w := image.get_width(); var h := image.get_height()
	var side := mini(w, int(h * 0.55))
	image = image.get_region(Rect2i((w - side) / 2, int(h * 0.04), side, side))
	image.resize(80, 80, Image.INTERPOLATE_BILINEAR)
	return ImageTexture.create_from_image(image)

func _change_mode(index: int) -> void:
	_mode_choice.select(1 if loadout_mode == "progression" else 0)
	_guard_unsaved(func():
		loadout_mode = "progression" if index == 1 else "sandbox"
		_mode_choice.select(index)
		get_node("/root/LearnedBattleRuntime").selected_loadout_mode = loadout_mode
		reload_catalogs()
	)

func _build_purchase_overlay() -> void:
	_purchase_overlay = Control.new(); _purchase_overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_purchase_overlay)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.8); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _purchase_overlay.add_child(dim)
	var center := CenterContainer.new(); center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _purchase_overlay.add_child(center)
	var panel := _panel(center, 900); panel.get_parent().custom_minimum_size.y = 650
	panel.get_parent().add_theme_stylebox_override("panel", _style("101b25", STYLE.BRONZE, 24))
	_label(panel, "REVIEW", 12, GOLD)
	_label(panel, "Review XP transaction", 30, STYLE.GOLD_BRIGHT)
	_purchase_details = _scroll(panel)
	_skip_prompt = CheckBox.new(); _skip_prompt.text = "Do not show again"; panel.add_child(_skip_prompt)
	_skip_prompt.add_theme_font_size_override("font_size", 18)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 14); panel.add_child(actions)
	_purchase_cancel = _button(actions, "Cancel", func(): _purchase_overlay.hide(), "purchase.cancel")
	_purchase_confirm = STYLE.accent(_button(actions, "Confirm purchase", _confirm_purchase, "purchase.confirm"))
	for b in [_purchase_cancel, _purchase_confirm]: b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_focus_pair(_purchase_cancel, _purchase_confirm)
	_purchase_overlay.hide()

func _confirmation_enabled(kind: String) -> bool:
	return bool(_transaction_preferences.get_value("confirmations", kind, true))

func _confirmation_toggle(parent: Node, caption: String, kind: String) -> CheckBox:
	var toggle := CheckBox.new(); toggle.text = caption
	toggle.button_pressed = _confirmation_enabled(kind)
	toggle.tooltip_text = "Show a review before each card purchase." if kind == "buy_card" else "Show a review before each card sale."
	toggle.toggled.connect(func(enabled): _set_confirmation(kind, enabled))
	parent.add_child(toggle)
	return toggle

func _set_confirmation(kind: String, enabled: bool) -> void:
	_transaction_preferences.set_value("confirmations", kind, enabled)
	_confirm_buy.set_pressed_no_signal(_confirmation_enabled("buy_card"))
	_confirm_sell.set_pressed_no_signal(_confirmation_enabled("sell_card"))
	if _transaction_preferences.save(WorkspacePaths.persistent_file("character_preferences.cfg")) != OK:
		_error.text = "Could not save confirmation preferences. This setting will last only while this screen is open."
		_error.show()

func _focus_pair(first: Button, second: Button) -> void:
	for button in [first, second]:
		var other: Button = second if button == first else first
		for property in ["focus_next", "focus_previous", "focus_neighbor_left", "focus_neighbor_right", "focus_neighbor_top", "focus_neighbor_bottom"]:
			button.set(property, button.get_path_to(other))

func reload_catalogs() -> void:
	var runtime = get_node_or_null("/root/LearnedBattleRuntime")
	var response: Dictionary = runtime.character_catalogs(loadout_mode) if runtime != null else {"ok": false, "error": "Character catalog unavailable."}
	if not response.get("ok", false):
		if not catalogs.is_empty():
			loadout_mode = _catalog_mode
			_mode_choice.select(1 if loadout_mode == "progression" else 0)
			runtime.selected_loadout_mode = loadout_mode
		_error.text = "Could not load characters: " + str(response.get("error", "Unknown error")); _error.show(); return
	_error.hide(); catalogs = response.get("result", {})
	_catalog_mode = loadout_mode
	var trees: Dictionary = runtime.card_trees("card_trees", {}, 0, "adventurer") if runtime.has_method("card_trees") else {}
	_tree_names = {}
	for tree_id in trees.get("result", {}).get("trees", {}): _tree_names[tree_id] = str(trees.result.trees[tree_id].get("name", tree_id))
	_drafts.clear(); _saved.clear()
	for id in catalogs:
		_saved[id] = catalogs[id].get("owned_decklist", catalogs[id].combatants[id].decklist).duplicate(true)
		_drafts[id] = _saved[id].duplicate(true)
	select_character(character_id if not character_id.is_empty() else initial_character)

func _clear(parent: Node) -> void:
	for child in parent.get_children(): parent.remove_child(child); child.queue_free()

func select_character(id: String) -> void:
	_comparison.dismiss()
	if not catalogs.has(id): return
	character_id = id; character = catalogs[id].combatants[id].duplicate(true)
	character.decklist = _drafts[id]
	BattlePresentationCatalog.configure(catalogs[id])
	for key in _roster_buttons:
		_roster_buttons[key].set_pressed_no_signal(key == id)
		_roster_buttons[key].text = catalogs[key].combatants[key].name
	_portrait.texture = load("res://assets/battle/fighters/%s.png" % ("blade_warden" if id == "adventurer" else id))
	_portrait_name.text = str(catalogs[id].combatants[id].name)
	_refresh_summary()
	_error.visible = catalogs[id].has("loadout_error")
	if _error.visible: _error.text = "Saved deck could not be loaded: " + str(catalogs[id].loadout_error) + ". Apply a valid deck to repair it."
	_deck_search.text = ""; _library_search.text = ""; selected_id = ""; selected_kind = ""
	_populate()
	_refresh_actions()

func _refresh_summary() -> void:
	_clear(_summary)
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 16); _summary.add_child(top)
	var identity := VBoxContainer.new(); identity.add_theme_constant_override("separation", 6); identity.size_flags_horizontal = Control.SIZE_EXPAND_FILL; top.add_child(identity)
	_label(identity, str(character.name), 30, STYLE.GOLD_BRIGHT)
	var tags := HFlowContainer.new(); tags.add_theme_constant_override("h_separation", 6); identity.add_child(tags)
	STYLE.chip(tags, "General pool", MUTED)
	if _character_type() != "general": STYLE.chip(tags, _type_name(_character_type()) + " cards", GOLD)
	STYLE.chip(tags, "Progression · XP" if loadout_mode == "progression" else "Sandbox", STYLE.AMBER if loadout_mode == "progression" else STYLE.DEFENSE)
	var health := 0
	for entry in character.decklist: health += int(entry.count)
	var stats := HBoxContainer.new(); stats.add_theme_constant_override("separation", 10); top.add_child(stats)
	STYLE.stat(stats, "%d  HEALTH" % health, "%d cards · %d unique" % [health, character.decklist.size()], STYLE.HEALTH)
	if loadout_mode == "progression":
		STYLE.stat(stats, "%d XP" % int(catalogs[character_id].progression.xp), "available to spend", GOLD)
	STYLE.stat(stats, "%d  ENERGY" % int(character.resources.starting_energy), "+%d each round" % int(character.income.energy), STYLE.ENERGY.lightened(0.2))
	STYLE.stat(stats, "%d  HAND" % int(character.resources.starting_hand_size), "limit %d · draw %d/round" % [int(character.resources.hand_limit), int(character.income.cards)], STYLE.IVORY)
	if loadout_mode == "progression":
		var xp := int(catalogs[character_id].progression.xp)
		var deck_value := 0
		for entry in character.decklist: deck_value += int(entry.count) * _copy_value(_entry_key(entry))
		var collection_value := int(catalogs[character_id].progression.get("collection_value", 0))
		var budget_text := "%d XP available + %d XP in deck" % [xp, deck_value]
		if collection_value > 0: budget_text += " + %d XP in collection" % collection_value
		_label(_summary, "Card budget: " + budget_text + " = %d XP" % (xp + deck_value + collection_value), 14, GOLD)
		var progress: Dictionary = catalogs[character_id].progression
		if int(progress.get("upgrade_spent", 0)) != 0:
			_label(_summary, "Total budget: %d XP · %d XP invested in upgrades" % [int(progress.total_budget), int(progress.upgrade_spent)], 13, MUTED)
	else:
		_label(_summary, "Deck size 1–%d cards · up to %d copies of each card" % [int(catalogs[character_id].deck_limits.max_cards), int(catalogs[character_id].deck_limits.max_copies)], 14, STYLE.DEFENSE.lightened(0.2))

func _populate() -> void:
	_card_workspace.visible = _tabs.current_tab == 1
	_list.get_parent().visible = _tabs.current_tab != 1
	_tab_hint.text = TAB_HINTS[_tabs.current_tab] if _tabs.current_tab < TAB_HINTS.size() else ""
	_tab_hint.visible = not _tab_hint.text.is_empty()
	_populate_entries()
	var choices := _deck_buttons if _tabs.current_tab == 1 and not _deck_buttons.is_empty() else _entry_buttons
	if not choices.is_empty(): choices[0].pressed.emit()
	else: _clear(_details); _label(_details, "Nothing in this category yet.", 16, MUTED)

func _swap_card_sides() -> void:
	_card_split.move_child(_card_split.get_child(0), 1)
	# Preserve the pane widths as well as each pane's own search and scroll.
	_card_split.split_offset = -_card_split.split_offset

func _entry(kind: String, id: String, title: String, subtitle: String, badge: String = "", target: VBoxContainer = null, source: String = "") -> void:
	var card_id := _card_of(id) if kind == "cards" else id
	var b := _button(_list if target == null else target, title, func(): inspect_entry(kind, id), "entry." + (source + "." if not source.is_empty() else "") + kind + "." + id)
	for state in ["font_color", "font_hover_color", "font_pressed_color", "font_focus_color", "font_hover_pressed_color"]: b.add_theme_color_override(state, Color.TRANSPARENT)
	b.custom_minimum_size.y = 78; b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.tooltip_text = title; b.clip_text = true
	b.add_theme_stylebox_override("normal", _row_style(selected_id == id))
	b.add_theme_stylebox_override("hover", STYLE.box("1d2e3b", "b99a60", 10, 10))
	var m := MarginContainer.new(); b.add_child(m); m.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	m.mouse_filter = Control.MOUSE_FILTER_IGNORE
	for side in ["left", "right", "top", "bottom"]: m.add_theme_constant_override("margin_" + side, 8)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 10); m.add_child(row); row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var definition: Dictionary = catalogs[character_id].get(kind, {}).get(card_id, {})
	match kind:
		"cards": STYLE.thumbnail(row, STYLE.card_texture(card_id), Vector2(44, 60), GOLD if _card_count(id) > 0 else STYLE.BRONZE)
		"abilities":
			var mark := STYLE.chip(row, badge, STYLE.OFFENSE if badge == "ATK" else STYLE.DEFENSE, 13)
			mark.custom_minimum_size = Vector2(46, 46); mark.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; mark.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		"dice":
			var die := STYLE.chip(row, "D%d" % int(definition.get("side_count", 6)), STYLE.IVORY, 16)
			die.custom_minimum_size = Vector2(52, 52); die.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; die.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	var v := VBoxContainer.new(); v.size_flags_horizontal = Control.SIZE_EXPAND_FILL; v.add_theme_constant_override("separation", 2); row.add_child(v); v.mouse_filter = Control.MOUSE_FILTER_IGNORE
	v.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var name_label := _label(v, title, 17); name_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	name_label.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; name_label.autowrap_mode = TextServer.AUTOWRAP_OFF
	if kind == "dice":
		var faces := HBoxContainer.new(); faces.add_theme_constant_override("separation", 4); faces.mouse_filter = Control.MOUSE_FILTER_IGNORE; v.add_child(faces)
		for face in definition.get("faces", []): _face_tile(faces, id, int(face.number), 40)
		b.custom_minimum_size.y = 96
	else:
		var sub := _label(v, (_type_caption(kind, card_id) + " · " if kind in ["cards", "abilities"] else "") + subtitle, 13, MUTED); sub.max_lines_visible = 1; sub.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; sub.mouse_filter = Control.MOUSE_FILTER_IGNORE
		if not _type_allowed(kind, card_id): sub.add_theme_color_override("font_color", STYLE.LOSS)
	if loadout_mode == "progression" and kind in ["cards", "abilities"]:
		var value := _label(v, _entry_xp_text(kind, id), 13, GOLD); value.mouse_filter = Control.MOUSE_FILTER_IGNORE
		b.custom_minimum_size.y = 92
		m.minimum_size_changed.connect(func(): b.custom_minimum_size.y = maxf(92, m.get_combined_minimum_size().y))
	if kind in ["cards", "abilities"] and not definition.is_empty(): STYLE.pip(row, int(definition.get("cost", {}).get("energy", 0)), 28)
	var count := _card_count(id) if kind == "cards" else -1
	if kind != "abilities":
		var tag := _label(row, badge, 16, GOLD if count != 0 else STYLE.DIM); tag.autowrap_mode = TextServer.AUTOWRAP_OFF; tag.mouse_filter = Control.MOUSE_FILTER_IGNORE
		tag.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		tag.add_theme_stylebox_override("normal", STYLE.box(Color(GOLD, 0.14) if count != 0 else Color("0b131a"), Color(GOLD, 0.5) if count != 0 else Color("22323f"), 8, 10))
	if kind == "cards" and source in ["library", "deck"]: _quick_controls(row, id, source)
	b.set_meta("entry_kind", kind); b.set_meta("entry_id", id); _entry_buttons.append(b)
	if source == "library": _library_buttons.append(b)
	elif source == "deck": _deck_buttons.append(b)

## How often each symbol appears across every equipped die face.
func _symbol_breakdown() -> void:
	var counts := {}; var glyphs := {}; var total := 0
	for entry in character.get("dice_loadout", []):
		var die: Dictionary = catalogs[character_id].dice.get(entry.dice_id, {})
		for face in die.get("faces", []):
			var name := BattlePresentationCatalog.symbol_name_for_die_face(str(entry.dice_id), int(face.number))
			counts[name] = int(counts.get(name, 0)) + int(entry.count); total += int(entry.count)
			glyphs[name] = BattlePresentationCatalog.symbol_for_die_face(str(entry.dice_id), int(face.number))
	if counts.is_empty(): return
	var box := STYLE.section(_list)
	STYLE.heading(box, "Symbols across your dice")
	_label(box, "Share of all faces you roll. Abilities qualify on these symbols and on face numbers.", 13, MUTED)
	var names: Array = counts.keys(); names.sort_custom(func(a, b): return counts[a] > counts[b] if counts[a] != counts[b] else str(a) < str(b))
	for name in names:
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 10); box.add_child(row)
		var caption := _label(row, "%s  %s" % [glyphs[name], name], 15); caption.custom_minimum_size.x = 180; caption.autowrap_mode = TextServer.AUTOWRAP_OFF
		var bar := ProgressBar.new(); bar.min_value = 0; bar.max_value = total; bar.value = counts[name]; bar.show_percentage = false
		bar.custom_minimum_size = Vector2(160, 14); bar.size_flags_horizontal = Control.SIZE_EXPAND_FILL; bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		bar.add_theme_stylebox_override("background", STYLE.box("0b131a", "22323f", 0, 6)); bar.add_theme_stylebox_override("fill", STYLE.box(STYLE.GOLD.darkened(0.2), STYLE.GOLD, 0, 6))
		row.add_child(bar)
		_label(row, "%d of %d faces" % [counts[name], total], 13, MUTED).autowrap_mode = TextServer.AUTOWRAP_OFF

func _row_style(selected: bool) -> StyleBoxFlat:
	return STYLE.box("223442", STYLE.GOLD, 10, 10, 2) if selected else STYLE.box("111d27", "1f2e3a", 10, 10)

## Add/remove (sandbox) or buy/sell (progression) one copy without leaving the list.
func _quick_controls(row: HBoxContainer, id: String, source: String) -> void:
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 4); box.size_flags_vertical = Control.SIZE_SHRINK_CENTER; row.add_child(box)
	var progression := loadout_mode == "progression"
	var add := _button(box, "+", func(): _quick_change(id, 1), "quick_add.%s.%s" % [source, id])
	var remove := _button(box, "−", func(): _quick_change(id, -1), "quick_remove.%s.%s" % [source, id])
	for button in [add, remove]:
		button.custom_minimum_size = Vector2(38, 32); button.add_theme_font_size_override("font_size", 18); button.focus_mode = Control.FOCUS_NONE
	add.disabled = not _can_add(id); remove.disabled = _card_count(id) < 1 or _copy_key(id).is_empty()
	add.tooltip_text = ("Buy a copy · %d XP" % _card_price(id)) if progression else "Add a copy" + (" via " + _tree_name(_tree_of(_copy_key(id, true))) if _is_shared(id) else "")
	remove.tooltip_text = ("Sell a copy · +%d XP" % _sale_value(_copy_key(id))) if progression else "Remove a copy"
	if _is_shared(id) and _copy_key(id).is_empty() and _card_count(id) > 0: remove.tooltip_text = "Copies come from several trees · use that tree's line in the deck"
	if progression and _membership(id).get("is_variant", false): add.tooltip_text = "Obtain this variant through its card tree."

## Copy limits cover every copy of a card, whichever tree it came through.
func _copy_limit(id: String) -> int:
	return int(_economy_map("card_copy_limits").get(_card_of(id), catalogs[character_id].deck_limits.max_copies))

func _can_add(id: String) -> bool:
	if not _type_allowed("cards", _card_of(id)) or _card_count(_card_of(id)) >= _copy_limit(id): return false
	if loadout_mode != "progression": return true
	if _membership(id).get("is_variant", false): return false
	return int(catalogs[character_id].progression.xp) >= _card_price(id) and _health() < int(catalogs[character_id].deck_limits.max_cards)

func _quick_change(id: String, delta: int) -> void:
	if loadout_mode == "progression":
		if delta > 0: _review_purchase("buy_card", id, _card_price(id))
		elif not _copy_key(id).is_empty(): _review_purchase("sell_card", _copy_key(id), _sale_value(_copy_key(id)))
		return
	var key := _copy_key(id, delta > 0)
	if key.is_empty(): return
	_set_card_count(key, _card_count(key) + delta)
	if selected_id == id and selected_kind == "cards": inspect_entry("cards", id)

func _face_tile(parent: Node, die_id: String, face: int, size: int) -> PanelContainer:
	var tile := PanelContainer.new(); tile.mouse_filter = Control.MOUSE_FILTER_IGNORE
	tile.custom_minimum_size = Vector2(size, size)
	tile.add_theme_stylebox_override("panel", STYLE.box("e9e1d1", "b6a679", 2, maxi(4, size / 5), 1))
	tile.tooltip_text = "%d · %s" % [face, BattlePresentationCatalog.symbol_name_for_die_face(die_id, face)]
	parent.add_child(tile)
	var text := Label.new(); text.text = ("%d %s" % [face, BattlePresentationCatalog.symbol_for_die_face(die_id, face)]) if size >= 40 else str(face)
	text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; text.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	text.add_theme_font_size_override("font_size", int(size * 0.5) if size < 40 else int(size * 0.3)); text.add_theme_color_override("font_color", Color("211e19"))
	text.mouse_filter = Control.MOUSE_FILTER_IGNORE; tile.add_child(text)
	# Full-size faces show the carved battle die; small chips stay plain numbers.
	if size >= 40:
		tile.add_theme_stylebox_override("panel", StyleBoxEmpty.new())
		STONE_DIE.dress(text, die_id, face)
	return tile

func _populate_entries() -> void:
	_clear(_list); _entry_buttons.clear(); _library_buttons.clear(); _deck_buttons.clear()
	match _tabs.current_tab:
		0:
			var board := HBoxContainer.new(); board.add_theme_constant_override("separation", 14); _list.add_child(board)
			for group in ["offensive", "defensive"]:
				var ids: Array = character.get("ability_board", {}).get(group, [])
				var column := VBoxContainer.new(); column.size_flags_horizontal = Control.SIZE_EXPAND_FILL; column.add_theme_constant_override("separation", 8); board.add_child(column)
				var head := HBoxContainer.new(); column.add_child(head)
				var heading := _label(head, ("OFFENSE" if group == "offensive" else "DEFENSE"), 13, STYLE.OFFENSE if group == "offensive" else STYLE.DEFENSE)
				heading.size_flags_horizontal = Control.SIZE_EXPAND_FILL; heading.autowrap_mode = TextServer.AUTOWRAP_OFF
				_label(head, "%d equipped" % ids.size(), 12, MUTED).autowrap_mode = TextServer.AUTOWRAP_OFF
				for id in ids:
					var info := BattlePresentationCatalog.ability(str(id))
					_entry("abilities", str(id), info.name, info.recipe, "ATK" if group == "offensive" else "DEF", column)
				if ids.is_empty(): _label(column, "No %s abilities equipped." % group, 14, MUTED)
		1:
			_populate_card_lists()
		2:
			for entry in character.get("dice_loadout", []):
				var die: Dictionary = catalogs[character_id].dice[entry.dice_id]
				_entry("dice", str(entry.dice_id), die.name, "%d faces" % int(die.side_count), "×%d" % int(entry.count))
			_symbol_breakdown()

func _populate_card_lists() -> void:
	var library_scroll: ScrollContainer = _library_list.get_parent()
	var deck_scroll: ScrollContainer = _deck_list.get_parent()
	var library_position := library_scroll.scroll_vertical
	var deck_position := deck_scroll.scroll_vertical
	_clear(_library_list); _clear(_deck_list)
	_library_heading.text = "CARD LIBRARY · %d" % _eligible_card_ids().size()
	_deck_heading.text = "YOUR DECK · %d cards" % _health()
	_refresh_curve()
	var ids: Array = _eligible_card_ids()
	ids.sort_custom(func(a, b): return str(catalogs[character_id].cards[a].name).naturalnocasecmp_to(str(catalogs[character_id].cards[b].name)) < 0)
	for id in ids:
		var info := BattlePresentationCatalog.card(str(id))
		if not _matches_card(info, _library_search.text): continue
		_entry("cards", str(id), info.name, "%d energy · %s" % [info.cost, info.effect_summary], "×%d" % _card_count(str(id)), _library_list, "library")
	# Sort a display copy so browsing never changes the configured deck order.
	# Copies of a shared card from different trees stay on separate lines.
	var deck: Array = character.get("decklist", []).duplicate()
	deck.sort_custom(func(a, b):
		var order := str(catalogs[character_id].cards[a.card_id].name).naturalnocasecmp_to(str(catalogs[character_id].cards[b.card_id].name))
		return order < 0 if order != 0 else _tree_name(str(a.get("tree", ""))) < _tree_name(str(b.get("tree", ""))))
	for entry in deck:
		var info := BattlePresentationCatalog.card(str(entry.card_id))
		var query := _deck_search.text.strip_edges().to_lower()
		var tree_match := not query.is_empty() and not str(entry.get("tree", "")).is_empty() and _tree_name(str(entry.tree)).to_lower().contains(query)
		if not _matches_card(info, _deck_search.text) and not tree_match: continue
		_entry("cards", _entry_key(entry), _display_name(_entry_key(entry)), "%d energy · %s" % [info.cost, info.effect_summary], "×%d" % int(entry.count), _deck_list, "deck")
	if _library_buttons.is_empty(): _label(_library_list, "No library cards match your search.", 16, MUTED)
	if _deck_buttons.is_empty(): _label(_deck_list, "Your deck is empty. Add cards from the library." if character.decklist.is_empty() else "No deck cards match your search.", 16, MUTED)
	library_scroll.set_deferred("scroll_vertical", library_position)
	deck_scroll.set_deferred("scroll_vertical", deck_position)

## Copies by energy cost, drawn as a small bar chart. Buckets follow the
## energy tiers: free, cheap, standard (5), strong and saved-up (10+).
const CURVE_LABELS := ["0", "1–4", "5", "6–9", "10+"]

func _curve_bucket(cost: int) -> int:
	if cost <= 0: return 0
	if cost < 5: return 1
	if cost == 5: return 2
	if cost < 10: return 3
	return 4

func _refresh_curve() -> void:
	_clear(_deck_curve)
	var buckets := [0, 0, 0, 0, 0]
	for entry in character.get("decklist", []):
		var cost := int(catalogs[character_id].cards.get(entry.card_id, {}).get("cost", {}).get("energy", 0))
		buckets[_curve_bucket(cost)] += int(entry.count)
	var most: int = maxi(1, buckets.max())
	for i in buckets.size():
		var column := VBoxContainer.new(); column.add_theme_constant_override("separation", 1); column.alignment = BoxContainer.ALIGNMENT_END
		column.tooltip_text = "%d %s at %s energy" % [buckets[i], "copy" if buckets[i] == 1 else "copies", CURVE_LABELS[i]]
		column.mouse_filter = Control.MOUSE_FILTER_PASS; _deck_curve.add_child(column)
		var count := _label(column, str(buckets[i]), 12, MUTED); count.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; count.autowrap_mode = TextServer.AUTOWRAP_OFF
		var bar := ColorRect.new(); bar.color = STYLE.ENERGY if buckets[i] > 0 else Color("22323f"); bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
		bar.custom_minimum_size = Vector2(24, maxf(2.0, 18.0 * buckets[i] / most)); column.add_child(bar)
		var caption := _label(column, CURVE_LABELS[i], 12, STYLE.ENERGY.lightened(0.3)); caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; caption.autowrap_mode = TextServer.AUTOWRAP_OFF

func _matches_card(info: Dictionary, query: String) -> bool:
	return query.strip_edges().is_empty() or (str(info.name) + " " + str(info.text)).to_lower().contains(query.strip_edges().to_lower())

func inspect_entry(kind: String, id: String) -> void:
	_comparison.dismiss()
	selected_kind = kind; selected_id = id
	for b in _entry_buttons:
		b.add_theme_stylebox_override("normal", _row_style(b.get_meta("entry_id") == id))
	_clear(_details)
	_quantity = null
	# Card rows may name one tree's copies of a shared card ("tree/card").
	var card_id := _card_of(id) if kind == "cards" else id
	var definition: Dictionary = catalogs[character_id][kind][card_id]
	var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 12); _details.add_child(head)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 6); head.add_child(titles)
	_label(titles, _display_name(id) if kind == "cards" else str(definition.name), 24, STYLE.GOLD_BRIGHT)
	var tags := HFlowContainer.new(); tags.add_theme_constant_override("h_separation", 6); tags.add_theme_constant_override("v_separation", 6); titles.add_child(tags)
	if kind == "cards" and _is_shared(id): STYLE.chip(tags, "Shared card", STYLE.DEFENSE)
	if kind in ["cards", "abilities"]:
		STYLE.chip(tags, "Type: " + _type_caption(kind, card_id), GOLD if _type_allowed(kind, card_id) else STYLE.LOSS)
		if kind == "abilities": STYLE.chip(tags, str(definition.type).capitalize(), STYLE.OFFENSE if str(definition.type) == "offensive" else STYLE.DEFENSE)
		else: STYLE.chip(tags, str(definition.get("type", "card")).replace("_", " ").capitalize(), MUTED)
		STYLE.pip(head, int(definition.get("cost", {}).get("energy", 0)), 40)
	else:
		STYLE.chip(tags, "%d faces" % int(definition.get("side_count", definition.get("faces", []).size())), MUTED)
	if loadout_mode == "progression" and kind in ["cards", "abilities"]: _label(_details, _entry_xp_text(kind, id), 15, GOLD)
	if kind in ["cards", "abilities"] and not _type_allowed(kind, card_id):
		var warning := _label(_details, "Unavailable for this character type. " + ("Sell/remove this card or change its type in Admin settings before battle." if kind == "cards" else "Choose an eligible downgrade or change the ability/character type in Admin settings before battle."), 15, Color("ffd2c8"))
		warning.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.LOSS, 0.12), Color(STYLE.LOSS, 0.55), 12, 8))
	if kind == "cards": _append_card_visual(_details, card_id)
	if kind == "cards" and _is_shared(id) and _tree_of(id).is_empty():
		_shared_copies(id)
	elif kind == "cards":
		if loadout_mode == "progression":
			_progression_actions(kind, id)
		else:
			var copies := STYLE.section(_details)
			STYLE.heading(copies, "Copies in deck")
			var quantity_row := HBoxContainer.new(); quantity_row.add_theme_constant_override("separation", 8); copies.add_child(quantity_row)
			var minus := _button(quantity_row, "−", func(): _set_card_count(id, _card_count(id) - 1), "remove." + id)
			minus.custom_minimum_size.x = 44; minus.disabled = _card_count(id) < 1
			if not _tree_of(id).is_empty(): _label(copies, "Copies obtained through %s · worth %d XP each in that tree." % [_tree_name(_tree_of(id)), _copy_value(id)], 14, MUTED)
			_quantity = SpinBox.new(); _quantity.min_value = 0; _quantity.max_value = _copy_limit(id) if _type_allowed("cards", card_id) else _card_count(id)
			_quantity.step = 1; _quantity.value = _card_count(id); _quantity.custom_minimum_size = Vector2(110, 42); quantity_row.add_child(_quantity)
			_quantity.alignment = HORIZONTAL_ALIGNMENT_CENTER
			_quantity.value_changed.connect(func(value): _set_card_count(id, int(value)); minus.disabled = int(value) < 1)
			_add_copy = STYLE.accent(_button(quantity_row, "+ Add a copy", func(): _set_card_count(id, _card_count(id) + 1); minus.disabled = _card_count(id) < 1, "add." + id))
			_add_copy.size_flags_horizontal = Control.SIZE_EXPAND_FILL
			_add_copy.disabled = not _type_allowed("cards", card_id) or _card_count(card_id) >= _copy_limit(id)
			_label(copies, "Set to 0 to remove · up to %d copies" % _copy_limit(id), 13, MUTED)
	elif kind == "abilities" and loadout_mode == "progression":
		_progression_actions(kind, id)
	_append_item_rules(_details, kind, card_id)

# Shared read-only rendering keeps Admin and the character inspector identical.
func _append_item_preview(parent: VBoxContainer, kind: String, id: String) -> void:
	if kind == "cards": _append_card_visual(parent, id)
	_append_item_rules(parent, kind, id)

func _append_card_visual(parent: VBoxContainer, id: String) -> void:
	var frame := CenterContainer.new(); parent.add_child(frame)
	var card := BattleCard.new(); frame.add_child(card); card.configure("preview", id, true)
	card.mouse_filter = Control.MOUSE_FILTER_IGNORE; card.tooltip_text = ""; card.focus_mode = Control.FOCUS_NONE

func _append_item_rules(parent: VBoxContainer, kind: String, id: String) -> void:
	var definition: Dictionary = catalogs[character_id][kind][id]
	var rules := STYLE.section(parent)
	if kind == "cards":
		STYLE.heading(rules, "Rules")
		_label(rules, "%d energy  ·  %s" % [int(definition.cost.energy), str(definition.type).replace("_", " ")], 14, MUTED)
		_label(rules, BattlePresentationCatalog.card(id).text, 17)
		STYLE.heading(rules, "Play window")
		var windows := HFlowContainer.new(); windows.add_theme_constant_override("h_separation", 6); windows.add_theme_constant_override("v_separation", 6); rules.add_child(windows)
		for timing in _card_play_windows(definition): STYLE.chip(windows, timing, STYLE.DEFENSE)
		if definition.play.get("before_first_roll", false): _label(rules, "Before your first offensive roll only.", 14, MUTED)
		_label(rules, "After play → " + str(definition.play.destination).capitalize(), 14, MUTED)
	elif kind == "abilities":
		STYLE.heading(rules, "Rules")
		_label(rules, "%s  ·  %d energy" % [str(definition.type).capitalize(), int(definition.cost.energy)], 14, MUTED)
		_label(rules, BattlePresentationCatalog.ability(id).text, 17)
		var tiers := BattlePresentationCatalog.offensive_tier_summaries(id)
		if not tiers.is_empty(): STYLE.heading(rules, "Tiers")
		for tier in tiers:
			var box := PanelContainer.new(); box.add_theme_stylebox_override("panel", STYLE.box("0c1620", Color(STYLE.OFFENSE, 0.45), 12, 8)); rules.add_child(box)
			var lines := VBoxContainer.new(); lines.add_theme_constant_override("separation", 2); box.add_child(lines)
			_label(lines, str(tier.recipe), 16, GOLD)
			_label(lines, str(tier.summary), 14)
		var uses := int(definition.get("usage", {}).get("maximum_per_segment", 0))
		if uses > 0: _label(rules, "Up to %d use%s per segment." % [uses, "" if uses == 1 else "s"], 14, MUTED)
	else:
		STYLE.heading(rules, "Faces")
		var grid := GridContainer.new(); grid.columns = 3; grid.add_theme_constant_override("h_separation", 10); grid.add_theme_constant_override("v_separation", 10); rules.add_child(grid)
		for face in definition.faces:
			var cell := VBoxContainer.new(); cell.add_theme_constant_override("separation", 2); grid.add_child(cell)
			_face_tile(cell, id, int(face.number), 56).size_flags_horizontal = Control.SIZE_SHRINK_CENTER
			var caption := _label(cell, BattlePresentationCatalog.symbol_name_for_die_face(id, int(face.number)), 13, MUTED)
			caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; caption.custom_minimum_size.x = 96
	if definition.has("saved_card_destination"):
		STYLE.heading(rules, "Saved cards")
		_label(rules, "Go to discard." if definition.saved_card_destination == "discard" else "Return to their piles. Played cards stay played.", 14)

func _unhandled_key_input(event: InputEvent) -> void:
	# Full-screen workspaces own Escape while they are open.
	for workspace in ["CardCreationWorkspace", "AbilityCreationWorkspace", "CardTreeWorkspace", "AdminPlayerSheet"]:
		if find_child(workspace, false, false) != null: return
	if event.is_action_pressed("ui_cancel") and _admin_overlay.visible:
		_close_admin(); get_viewport().set_input_as_handled(); return
	if event.is_action_pressed("ui_cancel"):
		if _purchase_overlay.visible: _purchase_overlay.hide()
		elif _confirm.visible: _confirm.hide()
		elif _comparison.visible: _comparison.dismiss()
		else: _close()
		get_viewport().set_input_as_handled()

func _close() -> void:
	_guard_unsaved(func(): closed.emit(); queue_free())

func _guard_unsaved(action: Callable) -> void:
	for id in _drafts:
		if _dirty(id):
			_pending_action = action; _confirm.show(); _cancel_changes.grab_focus(); return
	action.call()

func _deck_counts(deck: Array) -> Dictionary:
	var counts := {}
	for entry in deck: counts[_entry_key(entry)] = int(counts.get(_entry_key(entry), 0)) + int(entry.count)
	return counts

func _dirty(id: String) -> bool:
	return _deck_counts(_drafts[id]) != _deck_counts(_saved[id])

## Equipped copies. A "tree/card" key counts that tree's copies; a plain card
## ID counts every copy of the card, from any tree.
func _card_count(id: String) -> int:
	if id.contains("/"): return int(_deck_counts(_drafts[character_id]).get(id, 0))
	var total := 0
	for entry in _drafts[character_id]:
		if str(entry.card_id) == id: total += int(entry.count)
	return total

func _health() -> int:
	var total := 0
	for entry in _drafts[character_id]: total += int(entry.count)
	return total

func _refresh_actions() -> void:
	var progression := loadout_mode == "progression"
	_admin_button.visible = progression
	_confirmation_options.visible = progression
	_apply.visible = not progression; _revert.visible = not progression; _reset.visible = not progression
	_mode_note.text = "Buying and selling save immediately for your next battle. Cards sell for their current purchase price. Win campaign battles to earn more XP." if progression else "Sandbox: freely edit the same deck used in Progression. Apply changes to save; available XP stays unchanged."
	if progression:
		_save_status.text = "%d XP available · buy and sell cards at equal prices" % int(catalogs[character_id].progression.xp)
		_save_status.add_theme_color_override("font_color", GOLD)
		if _health() == 0: _save_status.text = "Deck empty · buy at least one card before starting a battle."; _save_status.add_theme_color_override("font_color", STYLE.LOSS)
		if _has_type_conflicts(): _save_status.text = "Type conflict · sell/remove incompatible cards or update Admin types before battle."; _save_status.add_theme_color_override("font_color", STYLE.LOSS)
		return
	var dirty := _dirty(character_id)
	var valid := not _has_type_conflicts() and _health() >= 1 and _health() <= int(catalogs[character_id].deck_limits.max_cards)
	_apply.disabled = (not dirty and not catalogs[character_id].has("loadout_error")) or not valid
	_revert.disabled = not dirty
	_reset.disabled = _deck_counts(_drafts[character_id]) == _deck_counts(catalogs[character_id].combatants[character_id].decklist)
	_save_status.text = "Unsaved changes · %d health" % _health() if dirty else "Saved deck · ready for a new battle"
	_save_status.add_theme_color_override("font_color", GOLD if dirty else STYLE.GAIN)
	if not valid:
		_save_status.text = "Deck needs 1–%d cards before applying." % int(catalogs[character_id].deck_limits.max_cards)
		_save_status.add_theme_color_override("font_color", STYLE.LOSS)
	if _has_type_conflicts():
		_save_status.text = "Type conflict · remove incompatible cards or update Admin types before battle."
		_save_status.add_theme_color_override("font_color", STYLE.LOSS)
	for id in _roster_buttons: _roster_buttons[id].text = str(catalogs[id].combatants[id].name) + (" *" if _dirty(id) else "")

func _set_card_count(id: String, count: int) -> void:
	# Free edits keep each shared copy's tree: a plain shared ID resolves to one tree's line.
	if _is_shared(id) and _tree_of(id).is_empty():
		id = _copy_key(id, count > _card_count(id))
		if id.is_empty(): return
	var others := _card_count(_card_of(id)) - _card_count(id)
	count = clampi(count, 0, maxi(0, _copy_limit(id) - others))
	if count > _card_count(id) and not _type_allowed("cards", _card_of(id)): return
	var found := false
	var deck: Array = _drafts[character_id]
	for index in range(deck.size() - 1, -1, -1):
		if _entry_key(deck[index]) != id: continue
		found = true
		if count == 0: deck.remove_at(index)
		else: deck[index].count = count
	if not found and count > 0:
		var line := {"card_id": _card_of(id), "count": count}
		if not _tree_of(id).is_empty(): line.tree = _tree_of(id)
		deck.append(line)
	character.decklist = deck
	# The inspector controls belong to the inspected card; quick row buttons may
	# change a different card.
	if id == selected_id and selected_kind == "cards":
		if is_instance_valid(_quantity): _quantity.set_value_no_signal(count)
		if is_instance_valid(_add_copy): _add_copy.disabled = not _type_allowed("cards", id) or count >= _copy_limit(id)
	_refresh_summary(); _populate_entries(); _refresh_actions()

func _refresh_draft() -> void:
	character.decklist = _drafts[character_id]
	_refresh_summary(); _populate_entries(); _refresh_actions()
	if selected_kind == "cards" and not selected_id.is_empty(): inspect_entry(selected_kind, selected_id)

func _revert_draft() -> void:
	_drafts[character_id] = _saved[character_id].duplicate(true); _refresh_draft()

func _reset_draft() -> void:
	_drafts[character_id] = catalogs[character_id].combatants[character_id].decklist.duplicate(true); _refresh_draft()

func _apply_draft() -> void:
	if is_instance_valid(_quantity): _quantity.apply()
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").save_character_deck(character_id, _drafts[character_id])
	if not response.get("ok", false):
		_error.text = "Could not save deck: " + str(response.get("error", "Unknown error")); _error.show(); return
	_error.hide(); catalogs[character_id].erase("loadout_error")
	_saved[character_id] = response.result.duplicate(true)
	_drafts[character_id] = _saved[character_id].duplicate(true)
	_refresh_draft()
	_save_status.text = "Deck saved · used in your next battle"


func _exit_tree() -> void:
	_end_card_admin()
	BattlePresentationCatalog.configure(_previous_catalog)

func _economy_map(key: String) -> Dictionary:
	var value = catalogs[character_id].get("economy", {}).get(key)
	return value if value is Dictionary else {}

func _card_price(id: String) -> int:
	return int(_economy_map("card_prices").get(_card_of(id), catalogs[character_id].economy.default_card_price))

# Shared cards ------------------------------------------------------------------
## Deck and collection rows are keyed by card ID, except copies of shared cards:
## each remembers the tree it came through and is keyed "tree/card".
static func _entry_key(entry: Dictionary) -> String:
	var tree := str(entry.get("tree", ""))
	return str(entry.card_id) if tree.is_empty() else "%s/%s" % [tree, entry.card_id]
static func _card_of(key: String) -> String: return key.get_slice("/", 1) if key.contains("/") else key
static func _tree_of(key: String) -> String: return key.get_slice("/", 0) if key.contains("/") else ""
func _membership(id: String) -> Dictionary:
	return _economy_map("card_tree_membership").get(_card_of(id), {})
func _is_shared(id: String) -> bool: return bool(_membership(id).get("shared", false))
func _tree_name(tree_id: String) -> String: return str(_tree_names.get(tree_id, tree_id))
func _display_name(key: String) -> String:
	var name := str(catalogs[character_id].cards.get(_card_of(key), {}).get("name", _card_of(key)))
	return name if _tree_of(key).is_empty() else "%s · via %s" % [name, _tree_name(_tree_of(key))]
## Each tree lending a shared card, with that tree's XP: [{tree_id, node_id, xp}].
func _shared_trees(id: String) -> Array:
	return _membership(id).get("trees", [])
## XP one copy is worth: its tree's value for shared copies, else the card price.
func _copy_value(key: String) -> int:
	for tree in _shared_trees(key):
		if str(tree.tree_id) == _tree_of(key): return int(tree.xp)
	return _card_price(key)
func _sale_value(key: String) -> int:
	if not _tree_of(key).is_empty(): return _copy_value(key)
	return int(_economy_map("card_sale_prices").get(_card_of(key), _card_price(key)))
## The one row a plain shared-card action means: the only tree whose copies are
## equipped, or (when adding) the first tree that lends it. Empty when ambiguous.
func _copy_key(id: String, adding: bool = false) -> String:
	if not _is_shared(id) or not _tree_of(id).is_empty(): return id
	var held: Array = []
	for entry in _drafts[character_id]:
		if str(entry.card_id) == id and not str(entry.get("tree", "")).is_empty() and _entry_key(entry) not in held: held.append(_entry_key(entry))
	held.sort()
	if held.size() == 1 or (adding and not held.is_empty()): return held[0]
	if adding and not _shared_trees(id).is_empty(): return "%s/%s" % [_shared_trees(id)[0].tree_id, id]
	return ""
## Stored copies for a row key (see _card_count for plain IDs).
func _stored_count(key: String) -> int:
	var total := 0
	for entry in catalogs[character_id].get("progression", {}).get("collection", []):
		if _entry_key(entry) == key or (not key.contains("/") and str(entry.card_id) == key): total += int(entry.count)
	return total
## A shared card's inspector lists each tree's copies; every action names one tree.
func _shared_copies(id: String) -> void:
	var box := STYLE.section(_details)
	STYLE.heading(box, "Copies by tree")
	_label(box, "A shared card keeps the tree it came through. Each tree's copies have that tree's XP value and move only along its paths. Battles count them together.", 14, MUTED)
	for tree in _shared_trees(id):
		var key := "%s/%s" % [tree.tree_id, id]
		var line := _button(box, "%s · %d XP · %d in deck · %d stored" % [_tree_name(str(tree.tree_id)), int(tree.xp), _card_count(key), _stored_count(key)], func(): inspect_entry("cards", key), "copies." + key)
		line.alignment = HORIZONTAL_ALIGNMENT_LEFT
	if _shared_trees(id).is_empty(): _label(box, "No published tree lends this card.", 14, MUTED)

func _progression_actions(kind: String, id: String) -> void:
	var progress: Dictionary = catalogs[character_id].progression
	var upgrades_box: VBoxContainer = null
	if kind == "cards":
		var copies := STYLE.section(_details)
		STYLE.heading(copies, "Copies")
		_label(copies, "%d %s equipped" % [_card_count(id), "copy" if _card_count(id) == 1 else "copies"], 16, MUTED)
		var card_id := _card_of(id)
		var price := _card_price(id)
		var trade := HBoxContainer.new(); trade.add_theme_constant_override("separation", 8); copies.add_child(trade)
		var buy := STYLE.accent(_button(trade, "Buy a copy · %d XP" % price, func(): _review_purchase("buy_card", id, price), "buy." + id))
		buy.disabled = not _type_allowed("cards", card_id) or int(progress.xp) < price or _card_count(card_id) >= _copy_limit(id) or _health() >= int(catalogs[character_id].deck_limits.max_cards)
		# A shared copy sells for the XP of the tree it came through.
		var sale_price := _sale_value(id)
		var sell := STYLE.accent(_button(trade, "Sell a copy · +%d XP" % sale_price, func(): _review_purchase("sell_card", id, sale_price), "sell." + id), STYLE.AMBER)
		sell.disabled = _card_count(id) < 1
		for b in [buy, sell]: b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		if int(progress.xp) < price and not _is_shared(id): _label(copies, "Need %d more XP to buy a copy." % (price - int(progress.xp)), 14, MUTED)
		var tree_membership := _membership(id)
		if not tree_membership.is_empty():
			var tree := STYLE.section(_details)
			STYLE.heading(tree, "Card tree")
			_button(tree, "Open Card Trees", _open_card_trees, "tree." + id)
			if _is_shared(id):
				buy.disabled = true
				var others: Array = _shared_trees(id).filter(func(t): return str(t.tree_id) != _tree_of(id)).map(func(t): return "%s (%d XP)" % [_tree_name(str(t.tree_id)), int(t.xp)])
				_label(tree, "Shared card · these copies came through %s and are worth %d XP each there. Obtain more through a card tree." % [_tree_name(_tree_of(id)), _copy_value(id)], 14, MUTED)
				if not others.is_empty(): _label(tree, "Also in: " + ", ".join(others) + ". Copies from different trees stay separate.", 14, MUTED)
			elif tree_membership.get("is_variant", false):
				buy.disabled = true
				_label(tree, "Obtain this variant through its connected card tree.", 14, MUTED)
		var stored := _stored_count(id)
		var collection := STYLE.section(_details)
		STYLE.heading(collection, "Collection")
		_label(collection, "%d in collection · not counted as health" % stored, 14, MUTED)
		var store := _button(collection, "Move deck copy to collection", func(): _move_collection_card("unequip_collection_card", id), "store." + id); store.disabled = _card_count(id) < 1
		if stored > 0:
			_button(collection, "Add collected copy to deck · no extra XP", func(): _move_collection_card("equip_collection_card", id), "equip." + id)
			_button(collection, "Sell collected copy · +%d XP" % sale_price, func(): _review_purchase("sell_collection_card", id, sale_price), "sell_collection." + id)
		for branch in _economy_map("card_upgrade_branches").get(id, []):
			if upgrades_box == null: upgrades_box = _upgrade_section()
			var target: Dictionary = catalogs[character_id].cards[branch.to]
			var button := STYLE.accent(_button(upgrades_box, "Upgrade → %s · %d XP" % [target.name, int(branch.xp)], func(): _review_purchase("upgrade_card", id, int(branch.xp), str(branch.to)), "upgrade." + id + "." + str(branch.to)))
			_bind_comparison(button, kind, id, str(branch.to), int(branch.xp))
			button.disabled = not _type_allowed("cards", str(branch.to)) or _card_count(id) < 1 or int(progress.xp) < int(branch.xp) or _card_count(str(branch.to)) >= _copy_limit(str(branch.to))
	var upgrades := _economy_map("card_upgrades" if kind == "cards" else "ability_upgrades")
	if upgrades.has(id):
		if upgrades_box == null: upgrades_box = _upgrade_section()
		var upgrade: Dictionary = upgrades[id]
		var target: Dictionary = catalogs[character_id][kind][upgrade.to]
		var action := "upgrade_card" if kind == "cards" else "upgrade_ability"
		var button := STYLE.accent(_button(upgrades_box, "Upgrade → %s · %d XP" % [target.name, int(upgrade.xp)], func(): _review_purchase(action, id, int(upgrade.xp)), "upgrade." + id))
		_bind_comparison(button, kind, id, str(upgrade.to), int(upgrade.xp))
		if not _type_allowed(kind, str(upgrade.to)): _label(upgrades_box, "Upgrade type: " + _type_caption(kind, str(upgrade.to)), 14, Color("ffae9f"))
		button.disabled = not _type_allowed(kind, str(upgrade.to)) or int(progress.xp) < int(upgrade.xp) or (kind == "cards" and (_card_count(id) < 1 or _card_count(str(upgrade.to)) >= int(catalogs[character_id].deck_limits.max_copies))) or (kind == "abilities" and str(upgrade.to) in (character.ability_board.offensive + character.ability_board.defensive))
	if kind == "abilities":
		var has_downgrade := false
		for previous_id in upgrades:
			var path: Dictionary = upgrades[previous_id]
			if str(path.to) != id: continue
			has_downgrade = true
			if upgrades_box == null: upgrades_box = _upgrade_section()
			var previous: Dictionary = catalogs[character_id].abilities[previous_id]
			var refund := int(path.xp)
			var button := STYLE.accent(_button(upgrades_box, "Downgrade → %s · +%d XP" % [previous.name, refund], func(): _review_purchase("downgrade_ability", id, refund, str(previous_id)), "downgrade." + id + "." + str(previous_id)), STYLE.AMBER)
			_bind_comparison(button, kind, id, str(previous_id), refund, true)
			button.disabled = not _type_allowed(kind, str(previous_id)) or int(progress.get("upgrade_spent", 0)) < refund or str(previous_id) in (character.ability_board.offensive + character.ability_board.defensive)
		if not upgrades.has(id) and not has_downgrade:
			if upgrades_box == null: upgrades_box = _upgrade_section()
			_label(upgrades_box, "No upgrade or downgrade configured.", 14, MUTED)

func _upgrade_section() -> VBoxContainer:
	var box := STYLE.section(_details)
	STYLE.heading(box, "Upgrades")
	_label(box, "Hover an option to compare both versions.", 13, MUTED)
	return box

func _move_collection_card(kind: String, id: String) -> void:
	_pending_purchase = {"character": character_id, "kind": kind, "id": _card_of(id), "tree": _tree_of(id), "cost": 0, "revision": int(catalogs[character_id].progression.revision), "target_id": id}
	_confirm_purchase()

func _review_purchase(kind: String, id: String, cost: int, downgrade_target: String = "") -> void:
	_comparison.dismiss()
	var progress: Dictionary = catalogs[character_id].progression
	var collection := "abilities" if kind in ["upgrade_ability", "downgrade_ability"] else "cards"
	# Card rows may name one tree's copies of a shared card; the request names that tree.
	var key := id
	if collection == "cards": id = _card_of(key)
	_pending_purchase = {"character": character_id, "kind": kind, "id": id, "tree": _tree_of(key) if collection == "cards" else "", "cost": cost, "revision": int(progress.revision)}
	_clear(_purchase_details)
	var current: Dictionary = catalogs[character_id][collection][id]
	var upgrade := kind in ["upgrade_card", "upgrade_ability", "downgrade_ability"]
	var downgrade := kind == "downgrade_ability"
	var selling := kind in ["sell_card", "sell_collection_card"]
	var target_id := downgrade_target if kind == "upgrade_card" and not downgrade_target.is_empty() else str(_economy_map("ability_upgrades" if collection == "abilities" else "card_upgrades")[id].to) if upgrade and not downgrade else id
	if downgrade: target_id = downgrade_target
	if collection == "cards" and not upgrade: target_id = key
	_pending_purchase.target_id = target_id
	_skip_prompt.set_pressed_no_signal(false)
	_skip_prompt.visible = not upgrade and kind != "sell_collection_card"
	_focus_pair(_purchase_cancel, _purchase_confirm)
	if not upgrade:
		_skip_prompt.tooltip_text = "Skip future sale prompts. Turn Confirm sales back on below the deck lists." if selling else "Skip future buy prompts. Turn Confirm buys back on below the deck lists."
		var cycle: Array[Button] = [_skip_prompt, _purchase_cancel, _purchase_confirm]
		for index in cycle.size():
			cycle[index].focus_next = cycle[index].get_path_to(cycle[(index + 1) % cycle.size()])
			cycle[index].focus_previous = cycle[index].get_path_to(cycle[(index + cycle.size() - 1) % cycle.size()])
		if not _confirmation_enabled(kind):
			_confirm_purchase()
			return
	var target: Dictionary = catalogs[character_id][collection][_card_of(target_id) if collection == "cards" else target_id]
	var card_delta := -1 if kind == "sell_card" else 1 if kind == "buy_card" else 0
	_label(_purchase_details, "%s → %s" % [current.name, target.name] if upgrade else ("Sell one %s" if selling else "Add one %s") % (_display_name(key) if collection == "cards" else str(target.name)), 24, GOLD)
	var delta := _label(_purchase_details, "XP: %d → %d    ·    Health: %d → %d" % [int(progress.xp), int(progress.xp) + (cost if selling or downgrade else -cost), _health(), _health() + card_delta], 22)
	delta.add_theme_stylebox_override("normal", STYLE.box("0c1620", Color(GOLD, 0.4), 14, 8))
	if kind == "upgrade_card": _label(_purchase_details, "Replaces one owned copy. Other copies remain unchanged.", 16, MUTED)
	elif kind == "upgrade_ability": _label(_purchase_details, "Replaces the equipped ability in its slot.", 16, MUTED)
	elif downgrade: _label(_purchase_details, "Returns the previous ability tier to the same slot and refunds %d XP. You can buy this upgrade again." % cost, 16, MUTED)
	elif kind == "sell_collection_card": _label(_purchase_details, "Sells one stored copy. Your equipped deck and health are unchanged.", 16, MUTED)
	else: _label(_purchase_details, "Equipped copies: %d → %d" % [_card_count(key), _card_count(key) + card_delta], 16, MUTED)
	if selling:
		_label(_purchase_details, "Returns the current purchase price to your XP balance. You can buy this card again from the library.", 18, MUTED)
		if _health() == 1 and kind == "sell_card": _label(_purchase_details, "This empties your deck. Buy at least one card before starting a battle.", 18, GOLD)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 14); _purchase_details.add_child(columns)
	if upgrade:
		var before := STYLE.section(columns); before.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_label(before, "BEFORE", 14, GOLD)
		_label(before, str(current.presentation.rules_text), 18)
	var after := STYLE.section(columns); after.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_label(after, "AFTER" if upgrade else "CARD RULES", 14, STYLE.GAIN if upgrade else GOLD)
	_label(after, str(target.presentation.rules_text), 18)
	_purchase_confirm.text = ("Receive %d XP" if selling or downgrade else "Spend %d XP") % cost
	_purchase_confirm.disabled = false
	_purchase_overlay.show(); _purchase_cancel.grab_focus()

func _confirm_purchase() -> void:
	_purchase_confirm.disabled = true
	var p := _pending_purchase
	if loadout_mode != "progression" or character_id != str(p.character):
		_purchase_overlay.hide(); return
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").purchase_progression(str(p.character), str(p.kind), str(p.id), int(p.revision), int(p.cost), str(p.target_id) if p.kind in ["downgrade_ability", "upgrade_card"] else "", str(p.get("tree", "")))
	_purchase_overlay.hide()
	if not response.get("ok", false):
		_error.text = "Transaction not completed: " + str(response.get("error", "Unknown error")) + ". Reload definitions to refresh."; _error.show(); return
	_error.hide()
	var progress: Dictionary = response.result
	catalogs[character_id].progression = progress
	catalogs[character_id].owned_decklist = progress.decklist.duplicate(true)
	catalogs[character_id].combatants[character_id].ability_board = progress.ability_board.duplicate(true)
	character.ability_board = progress.ability_board.duplicate(true)
	_saved[character_id] = progress.decklist.duplicate(true); _drafts[character_id] = _saved[character_id].duplicate(true)
	selected_kind = "abilities" if p.kind in ["upgrade_ability", "downgrade_ability"] else "cards"; selected_id = str(p.target_id)
	_refresh_draft()
	inspect_entry(selected_kind, selected_id)
	if str(p.kind) in ["buy_card", "sell_card"] and _skip_prompt.button_pressed:
		_set_confirmation(str(p.kind), false)

func _build_admin_overlay() -> void:
	_admin_overlay = Control.new(); _admin_overlay.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(_admin_overlay)
	var dim := ColorRect.new(); dim.color = Color(0, 0, 0, 0.85); dim.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); _admin_overlay.add_child(dim)
	# Keep the modal's bounds independent of wrapped labels' temporary minimum
	# heights. A CenterContainer can retain an inflated height after a relayout
	# and center the whole dialog below the viewport, leaving only the dimmer.
	var viewport := ScrollContainer.new(); viewport.name = "AdminDialogViewport"
	viewport.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	viewport.follow_focus = true; _admin_overlay.add_child(viewport)
	var fit := func():
		viewport.size = Vector2(1450, 850).min((_admin_overlay.size - Vector2(48, 48)).max(Vector2.ONE))
		viewport.position = (_admin_overlay.size - viewport.size) / 2.0
	_admin_overlay.resized.connect(fit); fit.call()
	_admin_content = _panel(viewport)
	_admin_content.get_parent().add_theme_stylebox_override("panel", _style("0d1620", STYLE.BRONZE, 20))
	_admin_content.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_admin_content.get_parent().custom_minimum_size.y = 850
	_admin_overlay.hide()

func _open_admin() -> void:
	_end_card_admin()
	_comparison.dismiss()
	# Read every character before presenting a coherent economy snapshot.
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").character_catalogs("progression")
	if not response.get("ok", false):
		_error.text = str(response.get("error", "Could not load admin settings")); _error.show(); return
	catalogs = response.result
	var admin_session: Dictionary = get_node("/root/LearnedBattleRuntime").card_admin("open_card_admin")
	if admin_session.get("ok", false): _card_admin_token = str(admin_session.result.admin_token)
	BattlePresentationCatalog.configure(catalogs[character_id])
	_admin_draft = catalogs[character_id].admin_settings.duplicate(true)
	_admin_draft.revision = int(_admin_draft.revision)
	if not _admin_draft.get("card_prices") is Dictionary: _admin_draft.card_prices = {}
	for id in _admin_draft.card_prices: _admin_draft.card_prices[id] = int(_admin_draft.card_prices[id])
	_admin_draft.budgets = {}
	for key in ["card_types", "ability_types", "character_types"]:
		if not _admin_draft.get(key) is Dictionary: _admin_draft[key] = {}
	# Retire the old controls before registering the replacement dialog.
	remove_child(_admin_overlay); _admin_overlay.queue_free(); _build_admin_overlay()
	_admin_prices.clear(); _admin_budgets.clear(); _admin_types = {"cards": {}, "abilities": {}, "characters": {}}
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 10); _admin_content.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	_label(titles, "CHARACTER CREATION  /  ADMIN SETTINGS", 12, GOLD)
	_label(titles, "Admin · Cards, XP & types", 30, STYLE.GOLD_BRIGHT)
	_button(header, "Admin player sheet · All cards", func():
		var sheet := preload("res://app/screens/character/admin_player_sheet.gd").new()
		sheet.character_id = character_id; sheet.name = "AdminPlayerSheet"; add_child(sheet), "admin.player_sheet").size_flags_vertical = Control.SIZE_SHRINK_END
	_button(header, "Manage card trees", func():
		var workshop := preload("res://app/screens/character/card_tree_workshop.gd").new()
		workshop.admin_token = _card_admin_token; workshop.character_id = character_id; workshop.name = "CardTreeWorkspace"; add_child(workshop)
		workshop.closed.connect(func(): _close_admin(); reload_catalogs()), "admin.trees").size_flags_vertical = Control.SIZE_SHRINK_END
	_label(_admin_content, "General items are available to everyone; other items require a matching character type. Type changes keep owned items and XP, but incompatible loadouts cannot enter battle. Prices and budgets save together.", 15, MUTED)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 14); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; _admin_content.add_child(columns)
	var left := STYLE.section(columns, 14); left.get_parent().custom_minimum_size.x = 340; left.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL; left.get_parent().size_flags_stretch_ratio = 1.3
	left.add_theme_constant_override("separation", 8)
	STYLE.heading(left, "Catalog")
	_admin_tabs = TabBar.new(); _admin_tabs.add_tab("Cards · price & type"); _admin_tabs.add_tab("Abilities · type"); left.add_child(_admin_tabs)
	_admin_search = LineEdit.new(); _admin_search.placeholder_text = "Search cards or abilities…"; _admin_search.clear_button_enabled = true; left.add_child(_admin_search)
	var browse := HBoxContainer.new(); browse.add_theme_constant_override("separation", 8); left.add_child(browse)
	_admin_sort = OptionButton.new(); _admin_sort.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _admin_sort.fit_to_longest_item = false
	_admin_sort.add_item("Name · A–Z"); _admin_sort.add_item("Type → Name · A–Z"); _admin_sort.select(1 if _admin_sort_by_type else 0)
	_admin_sort.tooltip_text = "Sort alphabetically, or group by type with alphabetical names within each group."; browse.add_child(_admin_sort)
	_admin_type_filter = OptionButton.new(); _admin_type_filter.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _admin_type_filter.fit_to_longest_item = false
	_admin_type_filter.add_item("All types"); _admin_type_filter.set_item_metadata(0, ""); browse.add_child(_admin_type_filter)
	var type_ids: Array = catalogs[character_id].access.types.keys()
	type_ids.sort_custom(func(a, b): return _type_name(str(a)).naturalnocasecmp_to(_type_name(str(b))) < 0)
	for type_id in type_ids:
		_admin_type_filter.add_item(_type_name(str(type_id))); _admin_type_filter.set_item_metadata(_admin_type_filter.item_count - 1, type_id)
		if str(type_id) == _admin_filter_type: _admin_type_filter.select(_admin_type_filter.item_count - 1)
	_admin_filter_type = str(_admin_type_filter.get_selected_metadata())
	_admin_type_filter.tooltip_text = "Filter cards and abilities by their current draft type. Works with search and either sort order."
	var prices := _scroll(left); _admin_items = prices
	prices.get_parent().vertical_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_ALWAYS
	prices.add_theme_constant_override("separation", 6)
	_admin_sort.item_selected.connect(func(index): _admin_sort_by_type = index == 1; _arrange_admin_items(true))
	_admin_type_filter.item_selected.connect(func(_index): _admin_filter_type = str(_admin_type_filter.get_selected_metadata()); _arrange_admin_items(true))
	_admin_tabs.tab_changed.connect(func(tab):
		_admin_search.text = ""
		_populate_admin_items(prices, "cards" if tab == 0 else "abilities")
		_clear_admin_item_preview()
	)
	_admin_search.text_changed.connect(func(_query): _arrange_admin_items(true))
	var preview_panel := _panel(columns, 300)
	preview_panel.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	preview_panel.get_parent().add_theme_stylebox_override("panel", _style("0a121a", STYLE.BRONZE, 14))
	_label(preview_panel, "ITEM PREVIEW", 13, GOLD)
	_admin_item_details = _scroll(preview_panel)
	_admin_item_details.get_parent().vertical_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_ALWAYS
	_admin_item_details.add_theme_constant_override("separation", 10)
	_clear_admin_item_preview()
	_populate_admin_items(prices, "cards")
	var right := _scroll(columns); right.get_parent().custom_minimum_size.x = 320
	right.get_parent().vertical_scroll_mode = ScrollContainer.SCROLL_MODE_SHOW_ALWAYS
	right.add_theme_constant_override("separation", 10)
	_label(right, "CHARACTER BUDGETS & TYPES", 13, GOLD)
	for id in ROSTER:
		var card := PanelContainer.new(); card.add_theme_stylebox_override("panel", STYLE.box("111d27", "1f2e3a", 10, 10)); right.add_child(card)
		var line := HBoxContainer.new(); line.add_theme_constant_override("separation", 10); card.add_child(line)
		var portrait := TextureRect.new(); portrait.texture = _roster_icon(id); portrait.custom_minimum_size = Vector2(48, 48)
		portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED; line.add_child(portrait)
		var group := VBoxContainer.new(); group.size_flags_horizontal = Control.SIZE_EXPAND_FILL; group.add_theme_constant_override("separation", 4); line.add_child(group)
		_label(group, str(catalogs[id].combatants[id].name), 17, STYLE.GOLD_BRIGHT)
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); group.add_child(row)
		var budget := int(catalogs[id].progression.total_budget); _admin_draft.budgets[id] = budget
		var spin := _admin_spin(row, budget, 0); _admin_budgets[id] = spin
		spin.tooltip_text = "Total XP budget: available XP plus the value of owned cards and upgrades."
		spin.value_changed.connect(func(amount): _admin_draft.budgets[id] = int(amount); _refresh_admin_preview())
		_admin_type_choice(row, "characters", id)
	var after := STYLE.section(right, 12)
	_label(after, "AFTER APPLYING", 13, GOLD)
	_admin_preview = _label(after, "", 15); _admin_preview.custom_minimum_size.x = 260
	_admin_error = _label(_admin_content, "", 15, Color("ffd2c8")); _admin_error.custom_minimum_size.x = 0; _admin_error.hide()
	_admin_error.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.LOSS, 0.12), Color(STYLE.LOSS, 0.55), 12, 8))
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 10); _admin_content.add_child(actions)
	_label(actions, "Changes apply to future battles and every character's ledger.", 13, MUTED).size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_button(actions, "Close · discard edits", _close_admin, "admin.close")
	_admin_save = STYLE.accent(_button(actions, "Apply economy changes", _save_admin, "admin.save"))
	_admin_save.custom_minimum_size.x = 230
	_refresh_admin_preview(); _admin_overlay.show(); _admin_search.grab_focus()
	if selected_kind == "cards" and _admin_item_rows.has(selected_id) and _admin_item_rows[selected_id].visible: _show_admin_item_preview("cards", selected_id)

func _admin_base_price(id: String) -> int:
	var catalog: Dictionary = catalogs[character_id]
	if not catalog.cards.has(id):
		for candidate in catalogs.values():
			if candidate.cards.has(id): catalog = candidate; break
	var prices = catalog.economy.get("card_prices")
	return int(prices.get(id, catalog.economy.default_card_price)) if prices is Dictionary else int(catalog.economy.default_card_price)

## A shared copy's value in the tree it came through (prices never apply).
func _tree_value(catalog: Dictionary, entry: Dictionary) -> int:
	for tree in catalog.economy.get("card_tree_membership", {}).get(str(entry.card_id), {}).get("trees", []):
		if str(tree.tree_id) == str(entry.tree): return int(tree.xp)
	return 0

func _admin_spin(parent: Node, amount: int, minimum: int) -> SpinBox:
	var spin := SpinBox.new(); spin.min_value = minimum; spin.max_value = 1000000; spin.step = 1; spin.value = amount
	spin.custom_minimum_size.x = 120; spin.suffix = "XP"; spin.alignment = HORIZONTAL_ALIGNMENT_RIGHT; parent.add_child(spin); return spin

func _refresh_admin_preview() -> void:
	var lines: PackedStringArray = []; var valid := true
	for id in ROSTER:
		var catalog: Dictionary = catalogs[id]; var progress: Dictionary = catalog.progression
		var value := 0
		var prices = catalog.economy.get("card_prices")
		for entry in progress.decklist:
			var base := int(prices.get(entry.card_id, catalog.economy.default_card_price)) if prices is Dictionary else int(catalog.economy.default_card_price)
			value += int(entry.count) * (_tree_value(catalog, entry) if not str(entry.get("tree", "")).is_empty() else int(_admin_draft.card_prices.get(entry.card_id, base)))
		var collection_value := 0
		for entry in progress.get("collection", []):
			var base := int(prices.get(entry.card_id, catalog.economy.default_card_price)) if prices is Dictionary else int(catalog.economy.default_card_price)
			collection_value += int(entry.count) * (_tree_value(catalog, entry) if not str(entry.get("tree", "")).is_empty() else int(_admin_draft.card_prices.get(entry.card_id, base)))
		var spent := int(progress.get("upgrade_spent", 0)); var budget := int(_admin_draft.budgets[id]); var available := budget - value - collection_value - spent
		var conflicts := 0
		for entry in progress.decklist:
			if not _draft_type_allowed(id, "cards", str(entry.card_id)): conflicts += int(entry.count)
		for ability in progress.ability_board.offensive + progress.ability_board.defensive:
			if not _draft_type_allowed(id, "abilities", str(ability)): conflicts += 1
		var values := "%d in deck" % value
		if collection_value > 0: values += " + %d in collection" % collection_value
		lines.append("%s · %d XP total\n%s + %d in upgrades · %d XP available%s" % [catalog.combatants[id].name, budget, values, spent, available, "\n%d incompatible equipped items · battle blocked" % conflicts if conflicts > 0 else ""])
		if available < 0: valid = false
	_admin_preview.text = "\n\n".join(lines)
	_admin_error.text = "A character is over budget. Increase its budget here, or close and sell cards before changing prices."
	_admin_error.visible = not valid; _admin_save.disabled = not valid
	_refresh_admin_item_context()

func _save_admin() -> void:
	_admin_save.disabled = true
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").save_economy_admin(_admin_draft)
	if not response.get("ok", false):
		_admin_error.text = str(response.get("error", "Could not save economy")); _admin_error.show(); _admin_save.disabled = false; return
	_close_admin()
	var kind := selected_kind; var id := selected_id
	reload_catalogs()
	if not id.is_empty(): inspect_entry(kind, id)

func _bind_comparison(button: Button, kind: String, id: String, target_id: String, cost: int, downgrade: bool = false) -> void:
	var show_preview := func():
		if _purchase_overlay.visible or _admin_overlay.visible or _confirm.visible: return
		var definitions: Dictionary = catalogs[character_id][kind]
		_comparison.present(button, str(definitions[id].name), str(definitions[target_id].name), _comparison_rules(kind, id), _comparison_rules(kind, target_id), cost, downgrade)
	button.mouse_entered.connect(show_preview)
	button.focus_entered.connect(show_preview)

func _comparison_rules(kind: String, id: String) -> String:
	var definition: Dictionary = catalogs[character_id][kind][id]
	var parts: PackedStringArray = []
	parts.append("Type: " + _type_caption(kind, id))
	parts.append("%d energy · %s" % [int(definition.cost.energy), str(definition.type).replace("_", " ").capitalize()])
	parts.append(str(definition.presentation.rules_text))
	if kind == "cards":
		var timing := _card_play_windows(definition)
		parts.append("Play window: " + ", ".join(timing))
		if definition.play.get("before_first_roll", false): parts.append("Before your first offensive roll only.")
		parts.append("After play: " + str(definition.play.destination).capitalize())
	else:
		for tier in BattlePresentationCatalog.offensive_tier_summaries(id): parts.append(str(tier.recipe) + "\n" + str(tier.summary))
		var uses := int(definition.get("usage", {}).get("maximum_per_segment", 0))
		if uses > 0: parts.append("Up to %d use(s) per segment." % uses)
	if definition.has("saved_card_destination"):
		parts.append("Saved cards: " + ("return to their original piles." if definition.saved_card_destination == "original" else "go to discard."))
	return "\n\n".join(parts)

func _entry_xp_text(kind: String, id: String) -> String:
	if kind == "cards":
		if _is_shared(id) and _tree_of(id).is_empty():
			var total := 0
			for entry in _drafts[character_id]:
				if str(entry.card_id) == id: total += int(entry.count) * _copy_value(_entry_key(entry))
			return "Shared card · XP set per tree · %d XP total" % total
		var unit := _copy_value(id); var count := _card_count(id)
		return "%d XP × %d = %d XP total" % [unit, count, unit * count]
	var value := _ability_xp_value(id)
	return "%d XP total · %s" % [value, "included starter ability" if value == 0 else "configured upgrade investment"]

func _ability_xp_value(id: String, visited: Array[String] = []) -> int:
	# Abilities currently have no base purchase cost. Sum the configured tier path;
	# use the cheapest authored path if future upgrades converge on the same tier.
	if id in visited: return -1
	var path: Array[String] = visited.duplicate(); path.append(id)
	var best := -1; var has_parent := false
	var upgrades := _economy_map("ability_upgrades")
	for previous in upgrades:
		if str(upgrades[previous].to) != id: continue
		has_parent = true
		var parent := _ability_xp_value(str(previous), path)
		if parent < 0: continue
		var total := parent + int(upgrades[previous].xp)
		if best < 0 or total < best: best = total
	return best if has_parent else 0

func _type_key(kind: String) -> String:
	return {"cards": "card_types", "abilities": "ability_types", "characters": "character_types"}[kind]

func _character_type() -> String:
	return str(catalogs[character_id].access.character_types.get(character_id, "general"))

func _type_name(id: String) -> String:
	return str(catalogs[character_id].access.types.get(id, id))

func _type_caption(kind: String, id: String) -> String:
	return _type_name(str(catalogs[character_id].access[_type_key(kind)].get(id, "general"))) + (" · incompatible" if not _type_allowed(kind, id) else "")

func _type_allowed(kind: String, id: String) -> bool:
	var required := str(catalogs[character_id].access[_type_key(kind)].get(id, "general"))
	return required == "general" or required == _character_type()

func _eligible_card_ids() -> Array:
	return catalogs[character_id].cards.keys().filter(func(id): return _type_allowed("cards", str(id)))

func _has_type_conflicts() -> bool:
	for entry in character.decklist:
		if not _type_allowed("cards", str(entry.card_id)): return true
	for id in character.ability_board.offensive + character.ability_board.defensive:
		if not _type_allowed("abilities", str(id)): return true
	return false

func _draft_type(kind: String, id: String) -> String:
	var key := _type_key(kind)
	return str(_admin_draft[key].get(id, catalogs[character_id].access[key].get(id, "general")))

func _draft_type_allowed(owner: String, kind: String, id: String) -> bool:
	var required := _draft_type(kind, id)
	return required == "general" or required == _draft_type("characters", owner)

func _admin_type_choice(parent: Node, kind: String, id: String) -> void:
	var choice := OptionButton.new(); choice.custom_minimum_size.x = 140; choice.fit_to_longest_item = false
	choice.tooltip_text = "Pool type" if kind != "characters" else "Character type"
	parent.add_child(choice); _admin_types[kind][id] = choice
	for type_id in catalogs[character_id].access.types:
		choice.add_item(_type_name(str(type_id))); choice.set_item_metadata(choice.item_count - 1, type_id)
		if str(type_id) == _draft_type(kind, id): choice.select(choice.item_count - 1)
	choice.item_selected.connect(func(index):
		_admin_draft[_type_key(kind)][id] = str(choice.get_item_metadata(index))
		_refresh_admin_preview()
		if kind != "characters": _arrange_admin_items.call_deferred(false)
	)

func _populate_admin_items(list: VBoxContainer, kind: String) -> void:
	_clear(list); _admin_types[kind].clear(); _admin_item_rows.clear(); _admin_group_labels.clear()
	if kind == "cards": _admin_prices.clear()
	for type_id in catalogs[character_id].access.types:
		var heading := _label(list, "", 13, GOLD); heading.add_theme_constant_override("line_spacing", 0)
		_admin_group_labels[str(type_id)] = heading
	_admin_empty = _label(list, "No cards or abilities match this type and search.", 15, MUTED)
	var definitions: Dictionary = catalogs[character_id][kind]
	var ids: Array = definitions.keys(); ids.sort_custom(func(a, b): return str(definitions[a].name) < str(definitions[b].name))
	for id in ids:
		# Row: panel → line → [info (hover target), controls]. Tests and hover
		# wiring rely on controls → line → info being the first child.
		var group := PanelContainer.new(); group.add_theme_stylebox_override("panel", STYLE.box("111d27", "1f2e3a", 8, 8)); list.add_child(group)
		group.set_meta("search_name", str(definitions[id].name).to_lower()); group.set_meta("entry_id", id)
		_admin_item_rows[str(id)] = group
		var line := HBoxContainer.new(); line.add_theme_constant_override("separation", 8); group.add_child(line)
		var info := HBoxContainer.new(); info.add_theme_constant_override("separation", 8); info.size_flags_horizontal = Control.SIZE_EXPAND_FILL; line.add_child(info)
		if kind == "cards": STYLE.thumbnail(info, STYLE.card_texture(str(id)), Vector2(30, 40))
		else:
			var offensive := str(definitions[id].get("type", "")) == "offensive"
			var mark := STYLE.chip(info, "ATK" if offensive else "DEF", STYLE.OFFENSE if offensive else STYLE.DEFENSE, 11)
			mark.custom_minimum_size = Vector2(36, 30); mark.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; mark.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		var text := VBoxContainer.new(); text.add_theme_constant_override("separation", 0); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; text.size_flags_vertical = Control.SIZE_SHRINK_CENTER; info.add_child(text)
		var caption := _label(text, str(definitions[id].name), 16); caption.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS; caption.autowrap_mode = TextServer.AUTOWRAP_OFF
		var detail := "%d energy" % int(definitions[id].get("cost", {}).get("energy", 0))
		if _economy_map("card_tree_membership").has(id): detail += " · card tree"
		var sub := _label(text, detail, 12, MUTED); sub.autowrap_mode = TextServer.AUTOWRAP_OFF
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); row.size_flags_vertical = Control.SIZE_SHRINK_CENTER; line.add_child(row)
		if kind == "cards":
			var value := int(_admin_draft.card_prices.get(id, _admin_base_price(id)))
			var spin := _admin_spin(row, value, 1); _admin_prices[id] = spin
			if _economy_map("card_tree_membership").has(id):
				spin.value = _card_price(id); spin.editable = false
				spin.tooltip_text = "Shared card · each tree sets its own XP in Card Trees" if _is_shared(id) else "Edit XP in Card Trees"
			spin.value_changed.connect(func(amount): _admin_draft.card_prices[id] = int(amount); _refresh_admin_preview())
		_admin_type_choice(row, kind, str(id))
		_connect_admin_item_hover(group, kind, str(id))
	_arrange_admin_items(true)

# Move existing rows instead of recreating edit controls: sorting must not lose
# pending prices/types, focus, or the selected preview.
func _arrange_admin_items(reset_scroll: bool) -> void:
	if not is_instance_valid(_admin_items): return
	var kind := "cards" if _admin_tabs.current_tab == 0 else "abilities"
	var definitions: Dictionary = catalogs[character_id][kind]
	var ids: Array = _admin_item_rows.keys()
	ids.sort_custom(func(a, b):
		if _admin_sort_by_type:
			var comparison := _type_name(_draft_type(kind, a)).naturalnocasecmp_to(_type_name(_draft_type(kind, b)))
			if comparison != 0: return comparison < 0
		var comparison := str(definitions[a].name).naturalnocasecmp_to(str(definitions[b].name))
		return comparison < 0 if comparison != 0 else str(a) < str(b)
	)
	for label in _admin_group_labels.values(): label.hide()
	var counts := {}; var position := 0; var visible_count := 0
	var query := _admin_search.text.strip_edges().to_lower()
	for id in ids:
		var type_id := _draft_type(kind, id)
		var row: Control = _admin_item_rows[id]
		row.visible = (query.is_empty() or query in str(row.get_meta("search_name"))) and (_admin_filter_type.is_empty() or _admin_filter_type == type_id)
		if not row.visible:
			if _admin_item_id == id: _clear_admin_item_preview()
			continue
		if _admin_sort_by_type and not counts.has(type_id):
			var heading: Label = _admin_group_labels[type_id]
			heading.show(); _admin_items.move_child(heading, position); position += 1
		counts[type_id] = int(counts.get(type_id, 0)) + 1
		_admin_items.move_child(row, position); position += 1; visible_count += 1
	for type_id in counts: _admin_group_labels[type_id].text = "%s · %d" % [_type_name(type_id).to_upper(), counts[type_id]]
	_admin_empty.visible = visible_count == 0
	if reset_scroll: _admin_items.get_parent().set_deferred("scroll_vertical", 0)

func _connect_admin_item_hover(control: Control, kind: String, id: String) -> void:
	if control is Label or control is Container: control.mouse_filter = Control.MOUSE_FILTER_PASS
	control.mouse_entered.connect(_show_admin_item_preview.bind(kind, id))
	control.focus_entered.connect(_show_admin_item_preview.bind(kind, id))
	# SpinBox keeps its editable field as an internal child.
	if control is SpinBox: _connect_admin_item_hover(control.get_line_edit(), kind, id)
	for child in control.get_children():
		if child is Control: _connect_admin_item_hover(child, kind, id)

func _clear_admin_item_preview() -> void:
	_admin_item_kind = ""; _admin_item_id = ""; _admin_item_context = null
	_clear(_admin_item_details)
	var empty := STYLE.section(_admin_item_details, 18)
	_label(empty, "Nothing selected", 20, STYLE.GOLD_BRIGHT)
	_label(empty, "Hover a card or ability to inspect its artwork and full rules. You can also focus an edit control with the keyboard.", 15, MUTED)
	_label(empty, "Card rows edit the XP price and pool type. Ability rows edit the pool type. Card tree members take their XP from Card Trees.", 13, STYLE.DIM)

func _show_admin_item_preview(kind: String, id: String) -> void:
	if _admin_item_kind == kind and _admin_item_id == id: return
	_admin_item_kind = kind; _admin_item_id = id
	_clear(_admin_item_details)
	_label(_admin_item_details, str(catalogs[character_id][kind][id].name), 24, STYLE.GOLD_BRIGHT)
	_admin_item_context = _label(_admin_item_details, "", 14, MUTED)
	_refresh_admin_item_context()
	if kind == "cards": _append_admin_card_delete(id)
	_append_item_preview(_admin_item_details, kind, id)
	var scroll: ScrollContainer = _admin_item_details.get_parent()
	scroll.scroll_vertical = 0

func _end_card_admin() -> void:
	var runtime = get_node_or_null("/root/LearnedBattleRuntime")
	if runtime != null and not _card_admin_token.is_empty(): runtime.card_admin("close_card_admin", _card_admin_token)
	_card_admin_token = ""
	if is_instance_valid(_delete_card_dialog): _delete_card_dialog.queue_free()
	_delete_card_dialog = null

func _close_admin() -> void:
	_end_card_admin()
	_admin_overlay.hide()

func _append_admin_card_delete(id: String) -> void:
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").card_admin("preview_delete_card", _card_admin_token, id)
	var info: Dictionary = response.get("result", {})
	var button := STYLE.accent(_button(_admin_item_details, "Delete card…", func(): _confirm_card_delete(info), "admin.delete_card"), STYLE.LOSS)
	button.size_flags_horizontal = Control.SIZE_SHRINK_END; button.custom_minimum_size.x = 160
	button.disabled = not response.get("ok", false) or not info.get("can_delete", false)
	if button.disabled:
		_label(_admin_item_details, str(info.get("reason", response.get("error", "Deletion unavailable."))), 14, MUTED)
	else:
		_label(_admin_item_details, "Removes this card from the library and Card Creation. Existing battles keep their saved definition.", 14, MUTED)

func _confirm_card_delete(info: Dictionary) -> void:
	if not _admin_overlay.visible or _card_admin_token.is_empty(): return
	if is_instance_valid(_delete_card_dialog): _delete_card_dialog.queue_free()
	var id := str(info.card_id)
	_delete_card_dialog = ConfirmationDialog.new(); _delete_card_dialog.title = "Delete card"
	_delete_card_dialog.dialog_text = "Delete %s (%s)?\n\nThis removes its definition for future battles. It cannot be undone here.\nExisting battles are unchanged. Unsaved economy edits will be discarded." % [str(info.name), id]
	_delete_card_dialog.ok_button_text = "Delete card"
	add_child(_delete_card_dialog)
	_delete_card_dialog.confirmed.connect(func():
		var response: Dictionary = get_node("/root/LearnedBattleRuntime").card_admin("admin_delete_card", _card_admin_token, id, int(info.revision))
		if not response.get("ok", false):
			_admin_error.text = str(response.get("error", "Could not delete card")); _admin_error.show(); return
		_close_admin(); reload_catalogs(); _open_admin()
		_admin_error.text = "Deleted %s." % str(info.name); _admin_error.show()
	)
	_delete_card_dialog.popup_centered(Vector2i(660, 240))

func _refresh_admin_item_context() -> void:
	if not is_instance_valid(_admin_item_context) or _admin_item_id.is_empty(): return
	var pool := _type_name(_draft_type(_admin_item_kind, _admin_item_id))
	var context := "Type: " + pool
	if _admin_item_kind == "cards" and _is_shared(_admin_item_id): context += " · shared card · XP per tree: " + ", ".join(_shared_trees(_admin_item_id).map(func(t): return "%s %d" % [_tree_name(str(t.tree_id)), int(t.xp)]))
	elif _admin_item_kind == "cards": context += " · %d XP" % int(_admin_draft.card_prices.get(_admin_item_id, _admin_base_price(_admin_item_id)))
	context += "\n" + str(character.name) + (" · Available" if _draft_type_allowed(character_id, _admin_item_kind, _admin_item_id) else " · Requires matching type")
	_admin_item_context.text = context

func _open_card_workshop() -> void:
	var workshop := preload("res://app/screens/character/card_authoring.gd").new()
	workshop.name = "CardCreationWorkspace"; add_child(workshop)
	workshop.closed.connect(reload_catalogs)
	workshop.deck_requested.connect(func(id):
		_tabs.current_tab = 1; _deck_search.text = ""; _library_search.text = ""
		inspect_entry("cards", id))

func _card_play_windows(definition: Dictionary) -> Array[String]:
	var timing: Array[String] = []
	if definition.get("mechanic") is Dictionary:
		timing.append_array(CARD_TIMING.chips(definition.mechanic.get("windows", [])))
	elif definition.get("program") is Dictionary:
		var program: Dictionary = definition.program
		timing.append_array(CARD_TIMING.chips(program.windows, str(program.get("roll_requirement", "any"))))
		for key in ["uses_per_round", "uses_per_battle"]:
			if int(program.get(key, 0)) > 0: timing.append("%d %s" % [int(program[key]), key.replace("_", " ")])
	else:
		for window in definition.play.playable_during:
			var caption := "Defense / damage response" if str(window.segment) == "damage_resolution" else str(window.segment).replace("_", " ").capitalize()
			if caption not in timing: timing.append(caption)
	return timing

func _open_ability_workshop() -> void:
	var workshop := preload("res://app/screens/character/ability_authoring.gd").new()
	workshop.name = "AbilityCreationWorkspace"; add_child(workshop); workshop.closed.connect(reload_catalogs)

func _open_card_trees() -> void:
	var workshop := preload("res://app/screens/character/card_tree_workshop.gd").new()
	workshop.character_id = character_id; workshop.name = "CardTreeWorkspace"; add_child(workshop); workshop.closed.connect(reload_catalogs)
