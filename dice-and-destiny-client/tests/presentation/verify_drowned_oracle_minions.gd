extends SceneTree

## Bell Diver and Ribbon Eel: single-ability minions with blank health decks.
## Each starts from the opponent menu and plays a full battle through the screen.

const MENU := preload("res://app/boot/battle_bootstrap.gd")
const STONE_DIE := preload("res://presentation/dice/stone_die.gd")
const MINIONS := [
	{"key": "bell-diver", "definition": "drowned_oracle_bell_diver", "name": "Bell Diver", "health": 18, "art": "bell_diver", "card": "ballast_stone",
		"die": "bell_d6", "kept": [5, 6], "symbol": "toll", "defense": "Brass Helm", "prevent": [2, 2, 2, 3, 3, 3], "badge": "BELL DIVER"},
	{"key": "ribbon-eel", "definition": "drowned_oracle_ribbon_eel", "name": "Ribbon Eel", "health": 15, "art": "ribbon_eel", "card": "shed_ribbon",
		"die": "eel_d6", "kept": [2, 4, 6], "symbol": "snare", "defense": "Slip the Current", "prevent": [1, 1, 1, 2, 2, 4], "badge": "RIBBON EEL"},
]
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	for key in ["combat_transition_seconds", "damage_reveal_seconds", "damage_hold_seconds", "damage_removal_seconds", "effects_gather_seconds", "defense_roll_seconds", "defense_effects_seconds", "defense_hold_seconds"]:
		ProjectSettings.set_setting("dice_and_destiny/presentation/" + key, 0.1)
	for minion in MINIONS: await _play(minion)
	print("DROWNED ORACLE MINIONS UI: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _play(minion: Dictionary) -> void:
	var menu := MENU.new(); root.add_child(menu)
	await process_frame
	menu._character_choice.select(1)
	var found := -1
	for i in menu._model_choice.item_count:
		if menu._model_choice.get_item_metadata(i) == minion.key: found = i
	_expect(found >= 0, "%s is in the opponent dropdown" % minion.name)
	menu._model_choice.select(found); menu._update_opponent_description()
	_expect(menu._message.text.begins_with(minion.name) and "blank" in menu._message.text, "description explains %s" % minion.name)
	menu._start_selected()
	await process_frame; await process_frame
	var screen: Control
	for child in root.get_children():
		if child.get_script() != null and child.get_script().resource_path == "res://app/screens/battle/battle_screen.gd": screen = child
	_expect(screen != null, "Start Battle opens the battle screen for %s" % minion.name)
	if screen == null: return
	await process_frame
	var enemy: Dictionary = screen._view.actor("goblin")
	_expect(enemy.get("definition_id") == minion.definition, "authority selected %s" % minion.name)
	_expect(enemy.get("max_health") == minion.health, "%s has %d blank health cards" % [minion.name, minion.health])
	_expect(screen._actor_display_name("goblin") == minion.name, "enemy is named %s" % minion.name)
	var fighter := screen._root.get_node("BattleScenery/Fighter_enemy") as TextureRect
	_expect(fighter.texture.resource_path == "res://assets/battle/minions/drowned_oracle/%s.png" % minion.art, "%s art is in battle" % minion.name)
	var card: Dictionary = screen._view.content_definition("cards", minion.card)
	_expect(not card.is_empty() and card.get("operations", []).is_empty() and card.get("play", {}).get("playable_during", []).is_empty(), "%s cards are blank" % minion.name)
	for face in range(1, 7):
		var symbol := BattlePresentationCatalog.symbol_id_for_die_face(minion.die, face)
		_expect(symbol == (minion.symbol if face in minion.kept else "murk"), "%s face %d shows %s" % [minion.die, face, symbol])
		_expect(not STONE_DIE.glyph(symbol).is_empty(), "%s is engraved, not a text fallback" % symbol)
	_expect(STONE_DIE.palette(minion.die, true) != STONE_DIE.PALETTES.enemy, "%s has its own stone colour" % minion.die)
	_expect(minion.badge in screen._minion_summary().get("badge", ""), "battle badge names %s" % minion.name)
	var deadline := Time.get_ticks_msec() + 150000
	var saw_defense := false
	while Time.get_ticks_msec() < deadline and not screen._view.is_complete():
		if screen._model_error or not screen._error_message.is_empty(): break
		for panel in screen._defense_result_panels:
			if saw_defense or panel.data.get("actor_id") != "goblin" or panel.data.get("ability_name") != minion.defense or panel.data.get("awaiting_roll", false) or panel.data.dice.is_empty(): continue
			saw_defense = true
			_expect(panel.data.dice.size() == 1 and panel.dice_controls[0].is_visible_in_tree(), "%s shows one visible defense die" % minion.defense)
			var face := int(panel.data.dice[0].face)
			var prevented := int(panel.data.prevented)
			_expect(face >= 1 and face <= 6 and prevented >= 1 and prevented <= int(minion.prevent[face - 1]), "%s rolled %d and prevented %d: %s" % [minion.defense, face, prevented, JSON.stringify(panel.data)])
		if not screen._submitting and not screen._model_thinking and not screen._director.has_beats() and not screen._player_roll_active() and Time.get_ticks_msec() >= screen._interaction_deadline(true) and not bool(screen._view.learned_policy.get("model_turn", false)):
			var action := _choose(screen._view.legal_actions)
			if not action.is_empty(): screen._send(JSON.stringify(action))
		await process_frame
	_expect(screen._error_message.is_empty() and not screen._model_error, "%s battle has no authority or controller errors: %s" % [minion.name, screen._error_message])
	_expect(screen._view.is_complete(), "%s battle reaches victory, defeat, or draw" % minion.name)
	_expect(saw_defense, "%s defense roll exercised" % minion.name)
	screen._play_again()
	await process_frame; await process_frame
	_expect(screen._view.actor("goblin").get("current_health") == minion.health, "%s rematch restores all health cards" % minion.name)
	screen.queue_free(); await process_frame

func _choose(actions: Array) -> Dictionary:
	for kind in ["planning_commit_cards", "commit_interaction", "planning_roll", "planning_select_ability", "roll_dice", "pass", "planning_pass"]:
		for action in actions:
			if action.get("type") == kind: return action
	return {}

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DROWNED ORACLE MINIONS: " + message)
