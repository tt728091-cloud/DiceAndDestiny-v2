extends Control
## The campaign menu. Without a chosen save it lists the campaign saves (each
## the player's own copy of a starting character sheet) and starts new ones.
## With a save it shows that campaign's encounter path, the XP the last battle
## earned and the XP to spend; Prepare opens the save's deck and abilities and
## Fight starts the next encounter. The authority owns the encounter order,
## every reward and every save.

const STYLE := preload("res://app/screens/character/character_style.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const LEARNED_RUNTIME := preload("res://local_client/learned_battle/learned_battle_runtime.gd")
const CAMPAIGN_PREPARE := preload("res://app/screens/campaign/campaign_prepare.gd")

## The campaign save being played; empty shows the list of saves.
var save_id := ""
## The `campaign` block of the battle that just ended, if any.
var last_battle: Dictionary = {}
var status: Dictionary = {}

var _title: Label
var _subtitle: Label
var _all_saves: Button
var _saves_view: HBoxContainer
var _save_list: VBoxContainer
var _sheet_choice: VBoxContainer
var _new_name: LineEdit
var _new_note: Label
var _begin: Button
var _new_sheet := ""
var _play_view: VBoxContainer
var _summary: Label
var _banner: Label
var _path: HBoxContainer
var _fight: Button
var _deck: Button
var _message: Label
var _confirm_delete: ConfirmationDialog
var _pending_delete := ""

func _ready() -> void:
	name = "CampaignScreen"
	theme = STYLE.theme()
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_build()
	refresh()

func _build() -> void:
	var background := ColorRect.new(); background.color = STYLE.BACKDROP; background.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(background)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 28)
	add_child(margin)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 18); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	_subtitle = _label(titles, "DICE & DESTINY  /  CAMPAIGN", 12, STYLE.GOLD)
	_title = _label(titles, "Campaign", 34)
	_all_saves = _button(header, "All campaigns", _show_saves, "campaign.all_saves"); _all_saves.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var back := _button(header, "Back to menu", _back_to_menu, "campaign.back"); back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_build_saves(body)
	_build_play(body)
	_message = _label(body, "", 15, STYLE.MUTED); _message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_confirm_delete = ConfirmationDialog.new(); _confirm_delete.name = "ConfirmDelete"; _confirm_delete.title = "Delete campaign?"
	_confirm_delete.get_ok_button().text = "Delete campaign"; _confirm_delete.get_cancel_button().text = "Keep it"
	_confirm_delete.confirmed.connect(_delete_confirmed)
	add_child(_confirm_delete)

## The list of campaign saves, and a panel to start a new one.
func _build_saves(body: Node) -> void:
	_saves_view = HBoxContainer.new(); _saves_view.name = "Saves"; _saves_view.add_theme_constant_override("separation", 18); _saves_view.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(_saves_view)
	var list_panel := _panel(_saves_view); list_panel.get_parent().size_flags_horizontal = Control.SIZE_EXPAND_FILL
	STYLE.heading(list_panel, "Your campaigns")
	var scroll := ScrollContainer.new(); scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; list_panel.add_child(scroll)
	_save_list = VBoxContainer.new(); _save_list.name = "SaveList"; _save_list.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _save_list.add_theme_constant_override("separation", 10); scroll.add_child(_save_list)
	var new_panel := _panel(_saves_view); new_panel.get_parent().custom_minimum_size.x = 380
	STYLE.heading(new_panel, "New campaign")
	STYLE.label(new_panel, "Choose a starting character sheet. The campaign plays your own copy of it; the original never changes.", 14, STYLE.MUTED)
	_sheet_choice = VBoxContainer.new(); _sheet_choice.add_theme_constant_override("separation", 8); new_panel.add_child(_sheet_choice)
	STYLE.label(new_panel, "Name", 13, STYLE.MUTED)
	_new_name = LineEdit.new(); _new_name.name = "CampaignName"; _new_name.max_length = 40; _new_name.placeholder_text = "Name this campaign"; _new_name.custom_minimum_size.y = 42
	_new_name.text_changed.connect(func(_text): _update_begin())
	new_panel.add_child(_new_name)
	_new_note = STYLE.label(new_panel, "", 14, STYLE.GOLD)
	_begin = STYLE.accent(_button(new_panel, "Begin campaign", _begin_campaign, "campaign.begin"))
	_begin.custom_minimum_size.y = 52

## One campaign: its encounter path and what to do next.
func _build_play(body: Node) -> void:
	_play_view = VBoxContainer.new(); _play_view.name = "Play"; _play_view.add_theme_constant_override("separation", 18); _play_view.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(_play_view)
	_summary = _label(_play_view, "", 20, STYLE.IVORY)
	_banner = _label(_play_view, "", 22, STYLE.GAIN); _banner.name = "LastBattle"; _banner.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_banner.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.GAIN, 0.08), Color(STYLE.GAIN, 0.5), 14, 8))
	_path = HBoxContainer.new(); _path.name = "EncounterPath"; _path.add_theme_constant_override("separation", 10); _path.alignment = BoxContainer.ALIGNMENT_CENTER
	_path.size_flags_vertical = Control.SIZE_EXPAND_FILL; _play_view.add_child(_path)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 14); _play_view.add_child(actions)
	_deck = _button(actions, "Prepare · Deck & Abilities", _open_deck, "campaign.deck")
	_deck.tooltip_text = "Spend XP: upgrade your cards through their card trees, buy new base cards, sell cards back, and upgrade abilities. Every card in your deck is a point of health."
	_fight = _button(actions, "Fight", _start_next, "campaign.fight")
	for button in [_deck, _fight]: button.custom_minimum_size.y = 64; button.size_flags_horizontal = Control.SIZE_EXPAND_FILL; button.add_theme_font_size_override("font_size", 20)
	_fight.add_theme_stylebox_override("normal", STYLE.box("2b3a26", STYLE.GOLD, 10, 8, 2))

func _panel(parent: Node) -> VBoxContainer:
	var panel := PanelContainer.new(); panel.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, "22323f", 16, 10)); panel.size_flags_vertical = Control.SIZE_EXPAND_FILL; parent.add_child(panel)
	var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 10); panel.add_child(box)
	return box

func refresh() -> void:
	var response: Dictionary = _runtime().campaign_status() if _runtime() != null else {"ok": false, "error": "The battle runtime is unavailable."}
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The campaign could not be loaded.")))
		return
	status = response.result
	if not save_id.is_empty() and _save().is_empty(): save_id = ""
	_saves_view.visible = save_id.is_empty()
	_play_view.visible = not save_id.is_empty()
	_all_saves.visible = not save_id.is_empty()
	_message.text = ""
	if save_id.is_empty(): _render_saves()
	else: _render_play()

func _save() -> Dictionary:
	for save in status.get("saves", []):
		if save.id == save_id: return save
	return {}

# Saves ---------------------------------------------------------------------
func _render_saves() -> void:
	_title.text = "Campaigns"
	_subtitle.text = "DICE & DESTINY  /  CAMPAIGN"
	for child in _save_list.get_children(): _save_list.remove_child(child); child.queue_free()
	var saves: Array = status.get("saves", [])
	if saves.is_empty():
		STYLE.label(_save_list, "No campaigns yet. Start one on the right.", 16, STYLE.MUTED)
	var encounters: Array = status.get("encounters", [])
	for save in saves:
		var card := PanelContainer.new(); card.name = "Save_" + str(save.id)
		card.add_theme_stylebox_override("panel", STYLE.box(STYLE.RAISED, STYLE.BRONZE, 14, 10))
		_save_list.add_child(card)
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 14); card.add_child(row)
		var text := VBoxContainer.new(); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; text.add_theme_constant_override("separation", 2); row.add_child(text)
		_label(text, str(save.name), 22, STYLE.GOLD_BRIGHT)
		var progress: Dictionary = save.get("campaign", {})
		var next: Dictionary = save.get("next_encounter", {})
		_label(text, "%s · %d health · %d XP to spend" % [save.character_name, int(save.health), int(save.xp)], 16)
		_label(text, "Run %d · %d victories · next: %s (%d of %d)" % [int(progress.get("runs_completed", 0)) + 1, int(progress.get("victories", 0)), next.get("name", ""), int(progress.get("next_encounter", 0)) + 1, encounters.size()], 14, STYLE.MUTED)
		var id := str(save.id)
		var play := STYLE.accent(_button(row, "Continue", func(): _open_save(id), "campaign.continue." + id))
		play.custom_minimum_size = Vector2(130, 48); play.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		play.disabled = not save.get("playable", true)
		var remove := STYLE.accent(_button(row, "Delete", func(): _ask_delete(id), "campaign.delete." + id), STYLE.LOSS)
		remove.custom_minimum_size = Vector2(90, 48); remove.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	for problem in status.get("problems", []):
		STYLE.label(_save_list, str(problem), 13, STYLE.LOSS)
	_render_sheets()

func _render_sheets() -> void:
	for child in _sheet_choice.get_children(): _sheet_choice.remove_child(child); child.queue_free()
	var sheets: Array = status.get("sheets", [])
	if _new_sheet.is_empty() or not sheets.any(func(s): return s.id == _new_sheet):
		_new_sheet = str(sheets[0].id) if not sheets.is_empty() else ""
	for sheet in sheets:
		var id := str(sheet.id)
		var button := _button(_sheet_choice, "%s · %d health" % [sheet.name, int(sheet.health)], func(): _pick_sheet(id), "campaign.sheet." + id)
		button.toggle_mode = true; button.button_pressed = id == _new_sheet; button.alignment = HORIZONTAL_ALIGNMENT_LEFT
		button.custom_minimum_size.y = 48; button.add_theme_font_size_override("font_size", 18)
		button.add_theme_stylebox_override("pressed", STYLE.box("223442", STYLE.GOLD, 10, 8, 2))
		button.add_theme_stylebox_override("hover_pressed", STYLE.box("2a3e4d", STYLE.GOLD_BRIGHT, 10, 8, 2))
	if _new_name.text.strip_edges().is_empty(): _new_name.text = _default_name()
	_update_begin()

func _pick_sheet(id: String) -> void:
	var was_default := _new_name.text == _default_name()
	_new_sheet = id
	if was_default or _new_name.text.strip_edges().is_empty(): _new_name.text = ""
	_render_sheets()

func _sheet_name(id: String) -> String:
	for sheet in status.get("sheets", []):
		if sheet.id == id: return str(sheet.name)
	return id

## "Starter campaign", then "Starter campaign 2", … for unused names.
func _default_name() -> String:
	var base := "%s campaign" % _sheet_name(_new_sheet)
	var taken: Array = status.get("saves", []).map(func(s): return str(s.name))
	var name_text := base
	var n := 2
	while name_text in taken:
		name_text = "%s %d" % [base, n]; n += 1
	return name_text

func _update_begin() -> void:
	_new_note.text = "Starts with the %s's deck and abilities, plus %d bonus XP." % [_sheet_name(_new_sheet), int(status.get("bonus_xp", 0))]
	_begin.disabled = _new_sheet.is_empty() or _new_name.text.strip_edges().is_empty()

func _begin_campaign() -> void:
	if _begin.disabled: return
	var response: Dictionary = _runtime().campaign_new(_new_sheet, _new_name.text.strip_edges())
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The campaign could not start.")))
		return
	_new_name.text = ""
	_open_save(str(response.result.id))

func _open_save(id: String) -> void:
	save_id = id; last_battle = {}
	refresh()

func _show_saves() -> void:
	save_id = ""; last_battle = {}
	refresh()

func _ask_delete(id: String) -> void:
	_pending_delete = id
	var save_name := id
	for save in status.get("saves", []):
		if save.id == id: save_name = str(save.name)
	_confirm_delete.dialog_text = "Delete \"%s\"?\nIts deck, XP and progress are lost for good. Other campaigns and the original character sheet are not affected." % save_name
	_confirm_delete.popup_centered()

func _delete_confirmed() -> void:
	if _pending_delete.is_empty(): return
	var response: Dictionary = _runtime().campaign_delete(_pending_delete)
	_pending_delete = ""
	refresh()
	if response.get("ok") != true: _show_error(str(response.get("error", "The campaign could not be deleted.")))

# One campaign ----------------------------------------------------------------
func _render_play() -> void:
	var save := _save()
	var progress: Dictionary = save.get("campaign", {})
	_title.text = str(save.name)
	_subtitle.text = "DICE & DESTINY  /  CAMPAIGN  /  %s" % str(save.character_name).to_upper()
	_summary.text = "%s · %d health · %d XP to spend · Run %d · %d victories" % [save.character_name, int(save.health), int(save.xp), int(progress.get("runs_completed", 0)) + 1, int(progress.get("victories", 0))]
	_render_banner()
	_render_path(int(progress.get("next_encounter", 0)))
	var next: Dictionary = save.get("next_encounter", {})
	_fight.text = "Fight · %s" % str(next.get("name", ""))
	_fight.tooltip_text = "%s Victory earns %d XP." % [str(next.get("description", "")), int(next.get("xp", 0))]
	var blocked := ""
	if not save.get("playable", true): blocked = "%s is no longer a campaign character." % save.character_name
	elif int(save.get("health", 0)) == 0: blocked = "Your deck is empty. Open Prepare and buy at least one card."
	elif not save.get("type_conflicts", []).is_empty(): blocked = "Your deck has a type conflict. Open Prepare to sell or store the card."
	elif not save.get("tree_conflicts", []).is_empty(): blocked = "The campaign uses only card-tree cards. In Prepare, sell or store: %s." % ", ".join(PackedStringArray(save.tree_conflicts))
	_fight.disabled = not blocked.is_empty(); _deck.disabled = false
	_message.text = blocked if not blocked.is_empty() else "Win to earn XP. A defeat earns nothing, and the encounter waits for you to try again. Change your deck as often as you like between battles."
	_message.add_theme_color_override("font_color", STYLE.LOSS if not blocked.is_empty() else STYLE.MUTED)

func _render_banner() -> void:
	var outcome: Dictionary = last_battle.get("outcome", {})
	_banner.visible = not last_battle.is_empty()
	if last_battle.is_empty(): return
	var encounter: Dictionary = last_battle.get("encounter", {})
	var color := STYLE.GAIN
	if outcome.is_empty():
		_banner.text = "The last battle's result was not recorded: %s" % str(last_battle.get("error", "unknown error"))
		color = STYLE.LOSS
	elif int(outcome.get("xp_awarded", 0)) > 0:
		_banner.text = "Victory at %s · +%d XP" % [encounter.get("name", ""), int(outcome.xp_awarded)]
		if outcome.get("run_completed", false): _banner.text += " · Campaign cleared! A new run begins."
	else:
		_banner.text = "%s at %s · no XP earned" % [str(outcome.get("result", "defeat")).capitalize(), encounter.get("name", "")]
		color = STYLE.LOSS
	_banner.add_theme_color_override("font_color", color)
	_banner.add_theme_stylebox_override("normal", STYLE.box(Color(color, 0.08), Color(color, 0.5), 14, 8))

func _render_path(next_index: int) -> void:
	for child in _path.get_children(): _path.remove_child(child); child.queue_free()
	var encounters: Array = status.get("encounters", [])
	for index in encounters.size():
		if index > 0:
			var arrow := _label(_path, "→", 30, STYLE.BRONZE); arrow.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		var encounter: Dictionary = encounters[index]
		var state := "CLEARED" if index < next_index else "NEXT" if index == next_index else "AHEAD"
		var accent: Color = STYLE.GAIN if state == "CLEARED" else STYLE.GOLD if state == "NEXT" else STYLE.DIM
		var card := PanelContainer.new(); card.name = "Encounter%d" % (index + 1)
		card.custom_minimum_size = Vector2(250, 220); card.size_flags_horizontal = Control.SIZE_EXPAND_FILL; card.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		card.add_theme_stylebox_override("panel", STYLE.box(STYLE.PANEL, accent, 16, 10, 3 if state == "NEXT" else 1))
		_path.add_child(card)
		var column := VBoxContainer.new(); column.add_theme_constant_override("separation", 6); card.add_child(column)
		_label(column, "ENCOUNTER %d  ·  %s" % [index + 1, state], 12, accent)
		_label(column, str(encounter.get("name", "")), 24, STYLE.IVORY if state != "AHEAD" else STYLE.MUTED)
		var opponents := int(encounter.get("opponent_count", 1))
		_label(column, ("%d × %s" % [opponents, encounter.get("opponent_name", "")]) if opponents > 1 else str(encounter.get("opponent_name", "")), 16, STYLE.OFFENSE)
		var description := _label(column, str(encounter.get("description", "")), 15, STYLE.MUTED); description.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		_label(column, "Victory · +%d XP" % int(encounter.get("xp", 0)), 17, STYLE.GOLD)

func _start_next() -> void:
	var save := _save()
	var encounter: Dictionary = save.get("next_encounter", {})
	if encounter.is_empty() or _fight.disabled: return
	_fight.disabled = true; _deck.disabled = true
	_message.text = "Preparing %s…" % str(encounter.get("name", "the encounter"))
	await get_tree().process_frame
	var gateway: RefCounted = LEARNED_GATEWAY.new(_runtime(), "seat-a", LEARNED_RUNTIME.encounter_model_key(encounter), str(save.character))
	gateway.unified_defense = true
	gateway.loadout_mode = "campaign"
	gateway.encounter_id = str(encounter.id)
	gateway.campaign_save = save_id
	var seed := int(Time.get_unix_time_from_system() * 1000000.0) ^ Time.get_ticks_usec()
	var battle_id := "campaign-%d-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]
	var result: Dictionary = gateway.start_battle(battle_id, seed)
	if result.get("accepted") != true:
		refresh()
		_show_error(str(result.get("error", "The campaign battle could not start.")))
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = "blade"
	screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new()
	screen.learned_battle_mode = true
	screen.learned_human_seat = "seat-a"
	screen.learned_seed = seed
	get_tree().root.add_child(screen)
	queue_free()

func _open_deck() -> void:
	var screen = CAMPAIGN_PREPARE.new()
	screen.save_id = save_id
	screen.closed.connect(func(): show(); last_battle = {}; refresh(); _fight.grab_focus())
	get_tree().root.add_child(screen)
	hide()

func _back_to_menu() -> void:
	get_tree().change_scene_to_file("res://app/boot/battle_bootstrap.tscn")
	queue_free()

func _show_error(text: String) -> void:
	_message.text = text
	_message.add_theme_color_override("font_color", STYLE.LOSS)

func _runtime() -> Node:
	return get_node_or_null("/root/LearnedBattleRuntime")

func _label(parent: Node, text: String, font_size: int, color: Color = STYLE.IVORY) -> Label:
	var label := Label.new(); label.text = text
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", color)
	parent.add_child(label)
	return label

func _button(parent: Node, text: String, callback: Callable, control_id: String) -> Button:
	var button := Button.new(); button.text = text; button.pressed.connect(callback)
	button.set_meta("campaign_control", control_id)
	parent.add_child(button)
	_register(control_id, button, text)
	return button

func _register(control_id: String, control: Control, caption: String) -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, control, caption)
