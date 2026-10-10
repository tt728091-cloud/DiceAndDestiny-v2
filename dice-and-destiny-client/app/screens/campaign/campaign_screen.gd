extends Control
## Between campaign battles: shows the encounter path, the XP the last battle
## earned and the character's XP to spend. Prepare opens the focused deck and
## ability screen; Fight starts the next encounter. The authority owns the
## encounter order and every reward.

const STYLE := preload("res://app/screens/character/character_style.gd")
const BATTLE_SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const LEARNED_GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const LEARNED_RUNTIME := preload("res://local_client/learned_battle/learned_battle_runtime.gd")
const CAMPAIGN_PREPARE := preload("res://app/screens/campaign/campaign_prepare.gd")

## Empty picks the first campaign character.
var character_id := ""
## The `campaign` block of the battle that just ended, if any.
var last_battle: Dictionary = {}
var status: Dictionary = {}

var _body: VBoxContainer
var _summary: Label
var _character_choice: OptionButton
var _banner: Label
var _path: HBoxContainer
var _fight: Button
var _deck: Button
var _message: Label

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
	_body = VBoxContainer.new(); _body.add_theme_constant_override("separation", 18); margin.add_child(_body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); _body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	_label(titles, "DICE & DESTINY  /  CAMPAIGN", 12, STYLE.GOLD)
	_label(titles, "Campaign", 34)
	_character_choice = OptionButton.new(); _character_choice.custom_minimum_size = Vector2(220, 40); _character_choice.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_character_choice.item_selected.connect(func(index: int): character_id = str(_character_choice.get_item_metadata(index)); refresh())
	header.add_child(_character_choice); _register("campaign.character", _character_choice, "Campaign character")
	var back := _button(header, "Back to menu", _back_to_menu, "campaign.back"); back.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_summary = _label(_body, "", 20, STYLE.IVORY)
	_banner = _label(_body, "", 22, STYLE.GAIN); _banner.name = "LastBattle"; _banner.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_banner.add_theme_stylebox_override("normal", STYLE.box(Color(STYLE.GAIN, 0.08), Color(STYLE.GAIN, 0.5), 14, 8))
	_path = HBoxContainer.new(); _path.name = "EncounterPath"; _path.add_theme_constant_override("separation", 10); _path.alignment = BoxContainer.ALIGNMENT_CENTER
	_path.size_flags_vertical = Control.SIZE_EXPAND_FILL; _body.add_child(_path)
	var actions := HBoxContainer.new(); actions.add_theme_constant_override("separation", 14); _body.add_child(actions)
	_deck = _button(actions, "Prepare · Deck & Abilities", _open_deck, "campaign.deck")
	_deck.tooltip_text = "Spend XP: upgrade your cards through their card trees, buy new base cards, sell cards back, and upgrade abilities. Every card in your deck is a point of health."
	_fight = _button(actions, "Fight", _start_next, "campaign.fight")
	for button in [_deck, _fight]: button.custom_minimum_size.y = 64; button.size_flags_horizontal = Control.SIZE_EXPAND_FILL; button.add_theme_font_size_override("font_size", 20)
	_fight.add_theme_stylebox_override("normal", STYLE.box("2b3a26", STYLE.GOLD, 10, 8, 2))
	_message = _label(_body, "", 15, STYLE.MUTED); _message.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER

func refresh() -> void:
	var response: Dictionary = _runtime().campaign_status() if _runtime() != null else {"ok": false, "error": "The battle runtime is unavailable."}
	if response.get("ok") != true:
		_show_error(str(response.get("error", "The campaign could not be loaded.")))
		return
	status = response.result
	var order: Array = status.get("character_order", [])
	if character_id not in order and not order.is_empty(): character_id = str(order[0])
	_character_choice.clear()
	for id in order:
		_character_choice.add_item(str(status.characters[id].get("name", id)))
		_character_choice.set_item_metadata(_character_choice.item_count - 1, id)
		if id == character_id: _character_choice.select(_character_choice.item_count - 1)
	_character_choice.visible = order.size() > 1
	var character: Dictionary = status.characters.get(character_id, {})
	var progress: Dictionary = character.get("campaign", {})
	_summary.text = "%s · %d health · %d XP to spend · Run %d · %d victories" % [character.get("name", character_id), int(character.get("health", 0)), int(character.get("xp", 0)), int(progress.get("runs_completed", 0)) + 1, int(progress.get("victories", 0))]
	_render_banner()
	_render_path(int(progress.get("next_encounter", 0)))
	var next: Dictionary = character.get("next_encounter", {})
	_fight.text = "Fight · %s" % str(next.get("name", ""))
	_fight.tooltip_text = "%s Victory earns %d XP." % [str(next.get("description", "")), int(next.get("xp", 0))]
	var blocked := ""
	if int(character.get("health", 0)) == 0: blocked = "Your deck is empty. Open Prepare and buy at least one card."
	elif not character.get("type_conflicts", []).is_empty(): blocked = "Your deck has a type conflict. Open Prepare to sell or store the card."
	elif not character.get("tree_conflicts", []).is_empty(): blocked = "The campaign uses only card-tree cards. In Prepare, sell or store: %s." % ", ".join(PackedStringArray(character.tree_conflicts))
	_fight.disabled = not blocked.is_empty()
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
	var character: Dictionary = status.characters.get(character_id, {})
	var encounter: Dictionary = character.get("next_encounter", {})
	if encounter.is_empty() or _fight.disabled: return
	_fight.disabled = true; _deck.disabled = true
	_message.text = "Preparing %s…" % str(encounter.get("name", "the encounter"))
	await get_tree().process_frame
	var gateway: RefCounted = LEARNED_GATEWAY.new(_runtime(), "seat-a", LEARNED_RUNTIME.encounter_model_key(encounter), character_id)
	gateway.unified_defense = true
	gateway.loadout_mode = "progression"
	gateway.encounter_id = str(encounter.id)
	var seed := int(Time.get_unix_time_from_system() * 1000000.0) ^ Time.get_ticks_usec()
	var battle_id := "campaign-%d-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec(), OS.get_process_id()]
	var result: Dictionary = gateway.start_battle(battle_id, seed)
	if result.get("accepted") != true:
		_deck.disabled = false
		_show_error(str(result.get("error", "The campaign battle could not start.")))
		refresh()
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
	screen.character_id = character_id
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
	parent.add_child(button)
	_register(control_id, button, text)
	return button

func _register(control_id: String, control: Control, caption: String) -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(control_id, control, caption)
