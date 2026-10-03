extends Control

const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const VIEWER := "blade"
const MODE_PANEL_MAXIMUM_SIZE := Vector2(760, 620)
const MODE_PANEL_VIEWPORT_INSET := Vector2(32, 32)

var gateway: BattleGateway
var store: ActiveBattleStore
var _message: Label
var _buttons: VBoxContainer
var _mode_panel: PanelContainer
var _empty_progression_decks: Dictionary = {}
var _loadout_hint: Label
var _loadout_choice: OptionButton
var _character_choice: OptionButton
var _model_choice: OptionButton
var _seat_choice: OptionButton
var _menu_actions: Array[Button] = []

func _ready() -> void:
	gateway = BattleGateway.new() if gateway == null else gateway
	store = ActiveBattleStore.new() if store == null else store
	_build_mode_menu()
	resized.connect(_fit_mode_panel)
	call_deferred("_fit_mode_panel")

func _build_mode_menu() -> void:
	var background := ColorRect.new()
	background.color = Color("0b111a")
	background.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(background)
	var center := CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(center)
	_mode_panel = PanelContainer.new()
	_mode_panel.name = "ModePanel"
	_mode_panel.custom_minimum_size = MODE_PANEL_MAXIMUM_SIZE
	center.add_child(_mode_panel)
	var margin := MarginContainer.new()
	for side in ["margin_left", "margin_top", "margin_right", "margin_bottom"]:
		margin.add_theme_constant_override(side, 34)
	_mode_panel.add_child(margin)
	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", 16)
	margin.add_child(content)
	var title := Label.new()
	title.text = "DICE & DESTINY"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_font_size_override("font_size", 40)
	title.add_theme_color_override("font_color", Color("f5c963"))
	content.add_child(title)
	var subtitle := Label.new()
	subtitle.text = "Choose your character and opponent"
	subtitle.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	subtitle.add_theme_font_size_override("font_size", 20)
	content.add_child(subtitle)
	var mode_scroll := ScrollContainer.new()
	mode_scroll.name = "ModeScroll"
	mode_scroll.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	mode_scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL
	mode_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	mode_scroll.vertical_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	mode_scroll.follow_focus = true
	content.add_child(mode_scroll)
	_buttons = VBoxContainer.new()
	_buttons.name = "ModeButtons"
	_buttons.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_buttons.add_theme_constant_override("separation", 12)
	mode_scroll.add_child(_buttons)
	_character_choice = _add_selection("YOUR CHARACTER", [
		["Blade Warden · sword, shields, and bleed", "blade_warden"],
		["Venom · Poison, Incubation, and Catalyst", "venom"],
		["Curse · cursed dice, Entombment, and misfortune", "curse"],
		["Adventurer · swords, shields, and coins", "adventurer"],
	], "battle.setup.character")
	_loadout_choice = _add_selection("LOADOUT", [["Sandbox · free deck editing", "sandbox"], ["Progression · XP purchases", "progression"]], "battle.setup.loadout")
	var selected_mode: String = get_node("/root/LearnedBattleRuntime").selected_loadout_mode
	_loadout_choice.select(1 if selected_mode == "progression" else 0)
	_loadout_choice.item_selected.connect(func(_index): get_node("/root/LearnedBattleRuntime").selected_loadout_mode = str(_loadout_choice.get_selected_metadata()); _refresh_character_loadouts())
	_character_choice.item_selected.connect(func(_index): _update_start_availability())
	_character_choice.select(3)
	_refresh_character_loadouts()
	_model_choice = _add_selection("OPPONENT", [
		["Brine Mask · minion · keeps every 3", "brine-mask"],
		["Blade Warden · Global Champion CP193", "global-champion"],
		["Prior Champion CP38 · winner-health", "prior-global-cp38"],
		["Prior Champion CP480 · 2.4M steps", "prior-global-cp480"],
		["Optimized v3 · 5M seed 22", "optimized-v3"],
		["Decision Quality v2 · seed 22", "decision-v2"],
		["Original v1 · seed 11", "accepted-v1"],
		["Two Brine Masks · allied minion encounter", "brine-mask-pair"],
	], "battle.setup.model")
	_seat_choice = _add_selection("YOUR SEAT", [["Seat A", "seat-a"], ["Seat B", "seat-b"]], "battle.setup.seat")
	_add_mode_button("Start Battle", _start_selected, "battle.setup.start")
	var start: Button = _buttons.get_child(_buttons.get_child_count() - 1)
	_add_mode_button("Character Creation", _open_character_creation, "battle.setup.characters")
	var creation: Button = _buttons.get_child(_buttons.get_child_count() - 1)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 12); content.add_child(actions)
	for action in [start, creation]:
		action.reparent(actions); action.custom_minimum_size.y = 54; action.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		_menu_actions.append(action)
	_message = Label.new()
	_model_choice.item_selected.connect(func(_index: int): _update_opponent_description())
	_update_opponent_description()
	_message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_message.add_theme_color_override("font_color", Color("9fb3c8"))
	content.add_child(_message)
	_loadout_hint = Label.new(); _loadout_hint.text = "Your deck is empty. Add cards in Character Creation before starting a battle."
	_loadout_hint.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _loadout_hint.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_loadout_hint.add_theme_color_override("font_color", Color("f5c963")); content.add_child(_loadout_hint)
	_update_start_availability()

func _update_opponent_description() -> void:
	if _model_choice.get_selected_metadata() == "brine-mask-pair":
		_message.text = "Two Brine Masks, 16 health each. Choose your attack target and defend separately against each incoming attack. Both keep every 3 and roll Salt Veil to block half their die, rounded up."
		return
	_message.text = "Brine Mask: 16 health, one attack, one defense. Keeps every 3 across up to three rolls. Brine Surge adds 1 attack damage for 1 energy." if _model_choice.get_selected_metadata() == "brine-mask" else "Play against the selected trained Blade Warden. Rematch keeps your character and opponent."

func _add_selection(caption: String, choices: Array, control_id: String) -> OptionButton:
	var heading := Label.new()
	heading.text = caption
	heading.add_theme_color_override("font_color", Color("9fb3c8"))
	_buttons.add_child(heading)
	var select := OptionButton.new()
	select.custom_minimum_size.y = 48
	select.add_theme_font_size_override("font_size", 19)
	for choice in choices:
		select.add_item(str(choice[0]))
		select.set_item_metadata(select.item_count - 1, choice[1])
	_buttons.add_child(select)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null:
		inspector.register_control(control_id, select, caption)
	return select

func _start_selected() -> void:
	if _progression_deck_empty(): return
	_start_learned(str(_seat_choice.get_selected_metadata()), str(_model_choice.get_selected_metadata()))

func _add_mode_button(text: String, callback: Callable, control_id: String) -> void:
	var button := Button.new()
	button.text = text
	button.custom_minimum_size.y = 82
	button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	button.add_theme_font_size_override("font_size", 18)
	button.pressed.connect(callback)
	_buttons.add_child(button)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null:
		inspector.register_control(control_id, button, text.replace("\n", " — "))

func _fit_mode_panel() -> void:
	if _mode_panel == null:
		return
	var available := Vector2(
		maxf(1.0, size.x - MODE_PANEL_VIEWPORT_INSET.x),
		maxf(1.0, size.y - MODE_PANEL_VIEWPORT_INSET.y)
	)
	_mode_panel.custom_minimum_size = Vector2(
		minf(MODE_PANEL_MAXIMUM_SIZE.x, available.x),
		minf(MODE_PANEL_MAXIMUM_SIZE.y, available.y)
	)

func _start_classic() -> void:
	_set_buttons_disabled(true)
	_message.text = "Opening the classic battle authority…"
	var active := store.load_active()
	if str(active.get("actor_id", VIEWER)) == VIEWER and not str(active.get("battle_id", "")).is_empty():
		var reopened := gateway.open_battle(str(active.battle_id), VIEWER)
		if reopened.get("accepted") == true:
			_handoff_classic(reopened, int(active.get("last_sequence", 0)), str(active.get("snapshot_name", "")), active.get("history_context", {}))
			return
		store.clear()
	var battle_id := _new_battle_id("classic")
	var started := gateway.start_battle(battle_id, VIEWER, true)
	if started.get("accepted") != true:
		_show_error(str(started.get("error", "The battle could not start.")), started)
		return
	_handoff_classic(started, 0)

func _start_learned(human_seat: String, model_key: String) -> void:
	_set_buttons_disabled(true)
	_message.text = "Preparing Brine Masks…" if model_key in ["brine-mask", "brine-mask-pair"] else "Loading the selected frozen learned policy…"
	await get_tree().process_frame
	var runtime := get_node_or_null("/root/LearnedBattleRuntime")
	if runtime == null:
		_show_error("The learned battle runtime autoload is unavailable.", {})
		return
	var learned_gateway: RefCounted = LEARNED_GATEWAY.new(runtime, human_seat, model_key, str(_character_choice.get_selected_metadata()) if _character_choice != null else "blade_warden")
	var seed := int(Time.get_unix_time_from_system() * 1000000.0) ^ Time.get_ticks_usec()
	learned_gateway.unified_defense = true
	learned_gateway.loadout_mode = str(_loadout_choice.get_selected_metadata())
	var result: Dictionary = learned_gateway.start_battle(_new_battle_id("learned"), seed)
	if result.get("accepted") != true:
		_show_error(str(result.get("error", "The learned battle could not start.")), result)
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = VIEWER
	screen.gateway = learned_gateway
	screen.active_store = store
	screen.learned_battle_mode = true
	screen.learned_human_seat = human_seat
	screen.learned_seed = seed
	get_tree().root.add_child(screen)
	queue_free()

func _new_battle_id(prefix: String = "battle") -> String:
	return "%s-%d-%d-%d" % [prefix, Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]

func _handoff_classic(result: Dictionary, last_sequence: int, snapshot_name: String = "", history_context: Dictionary = {}) -> void:
	var battle_id := str(result.get("snapshot", {}).get("battle_id", ""))
	if battle_id.is_empty() or store.save_active(battle_id, VIEWER, last_sequence, snapshot_name, history_context) != OK:
		_show_error("The active battle record could not be saved.", result)
		return
	var screen = BATTLE_SCREEN.instantiate()
	screen.initial_result = result
	screen.viewer_actor_id = VIEWER
	screen.gateway = gateway
	screen.active_store = store
	screen.last_presented_sequence = last_sequence
	screen.loaded_snapshot_name = snapshot_name
	screen.history_context = history_context
	get_tree().root.add_child(screen)
	queue_free()

func _set_buttons_disabled(disabled: bool) -> void:
	for button in _menu_actions: button.disabled = disabled
	for child in _buttons.get_children():
		if child is Button:
			child.disabled = disabled

	if not disabled: _update_start_availability()

func _show_error(message: String, result: Dictionary) -> void:
	_message.text = "BATTLE START ERROR\n%s\n\n%s" % [message, JSON.stringify(result)]
	_message.add_theme_color_override("font_color", Color("ff8a78"))
	_set_buttons_disabled(false)

func _open_character_creation() -> void:
	var screen = preload("res://app/screens/character/character_creation.gd").new()
	screen.initial_character = str(_character_choice.get_selected_metadata())
	screen.loadout_mode = str(_loadout_choice.get_selected_metadata())
	screen.closed.connect(func(): show(); _loadout_choice.select(1 if get_node("/root/LearnedBattleRuntime").selected_loadout_mode == "progression" else 0); _refresh_character_loadouts(); _character_choice.grab_focus())
	get_tree().root.add_child(screen)
	hide()

func _refresh_character_loadouts() -> void:
	var response: Dictionary = get_node("/root/LearnedBattleRuntime").character_catalogs(str(_loadout_choice.get_selected_metadata()))
	if not response.get("ok", false): return
	_empty_progression_decks.clear()
	for index in _character_choice.item_count:
		var id := str(_character_choice.get_item_metadata(index))
		var catalog: Dictionary = response.result.get(id, {})
		if catalog.is_empty(): continue
		var health := 0
		for entry in catalog.get("owned_decklist", catalog.combatants[id].decklist): health += int(entry.count)
		_empty_progression_decks[id] = catalog.has("progression") and health == 0
		_character_choice.set_item_text(index, "%s · %d health · %s" % [catalog.combatants[id].name, health, "Progression · %d XP" % int(catalog.progression.xp) if catalog.has("progression") else "Saved deck" if catalog.has("owned_decklist") else "Starter deck"])

	_update_start_availability()

func _progression_deck_empty() -> bool:
	return _loadout_choice != null and str(_loadout_choice.get_selected_metadata()) == "progression" and bool(_empty_progression_decks.get(str(_character_choice.get_selected_metadata()), false))

func _update_start_availability() -> void:
	var empty := _progression_deck_empty()
	if not _menu_actions.is_empty():
		_menu_actions[0].disabled = empty
		_menu_actions[0].tooltip_text = "Buy at least one card in Character Creation." if empty else ""
	if _loadout_hint != null: _loadout_hint.visible = empty
