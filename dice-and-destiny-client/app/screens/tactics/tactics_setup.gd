extends Control
## Tactics Sandbox setup (docs/game-design/tactics-sandbox-plan.md §9.1): the
## hero and optional map Move cards, 1–4 enemies, the arena, optional rule
## overrides and the seed. Everything offered comes from the authority's
## tactics_catalog; Start creates the battle through tactics_start.

const SCREEN := preload("res://app/screens/tactics/tactics_screen.gd")
const BOOTSTRAP := preload("res://app/boot/battle_bootstrap.tscn")
const NONE := "—"

var gateway: TacticsGateway
var store: ActiveBattleStore
var catalog: Dictionary = {}
var _hero: OptionButton
var _arena: OptionButton
var _enemies: Array[OptionButton] = []
var _map_cards: Dictionary = {}
var _overrides: Dictionary = {}
var _advanced: LineEdit
var _seed: LineEdit
var _message: Label
var _preview: Control
var _start: Button
var _resume: Button

func _ready() -> void:
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	gateway = TacticsGateway.new(get_node("/root/LearnedBattleRuntime")) if gateway == null else gateway
	store = TacticsGateway.active_store() if store == null else store
	var background := ColorRect.new(); background.color = Color("0b111a"); background.set_anchors_preset(Control.PRESET_FULL_RECT); add_child(background)
	var response := gateway.catalog()
	if response.get("ok") != true:
		var error := Label.new(); error.text = "The sandbox could not load: %s" % str(response.get("error", "")); error.position = Vector2(40, 40); add_child(error)
		_add_button(self, "Back", "tactics.setup.back", _back)
		return
	catalog = response.result
	_build()

func _register(id: String, control: Control, text: String) -> void:
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control(id, control, text)

func _add_button(parent: Node, text: String, id: String, callback: Callable) -> Button:
	var button := Button.new(); button.text = text; button.custom_minimum_size.y = 40
	button.pressed.connect(callback); parent.add_child(button); _register(id, button, text)
	return button

func _heading(parent: Node, text: String) -> void:
	var label := Label.new(); label.text = text; label.add_theme_color_override("font_color", Color("9fb3c8")); parent.add_child(label)

func _option(parent: Node, id: String, choices: Array) -> OptionButton:
	var select := OptionButton.new(); select.custom_minimum_size.y = 36
	select.fit_to_longest_item = false; select.clip_text = true; select.custom_minimum_size.x = 200
	for choice in choices:
		select.add_item(str(choice[0])); select.set_item_metadata(select.item_count - 1, choice[1])
	parent.add_child(select); _register(id, select, id)
	return select

func _build() -> void:
	var margin := MarginContainer.new(); margin.set_anchors_preset(Control.PRESET_FULL_RECT)
	for side in ["margin_left", "margin_right"]: margin.add_theme_constant_override(side, 48)
	for side in ["margin_top", "margin_bottom"]: margin.add_theme_constant_override(side, 24)
	add_child(margin)
	var outer := VBoxContainer.new(); outer.add_theme_constant_override("separation", 10); margin.add_child(outer)
	var title := Label.new(); title.text = "TACTICS SANDBOX · prototype"; title.add_theme_font_size_override("font_size", 30); title.add_theme_color_override("font_color", Color("f5c963")); outer.add_child(title)
	var subtitle := Label.new(); subtitle.text = "A square grid where range, line of sight and secret simultaneous movement matter. Move is paid with your deck: each discarded card is +1 Move and still counts as health."
	subtitle.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; outer.add_child(subtitle)
	var scroll := ScrollContainer.new(); scroll.size_flags_vertical = Control.SIZE_EXPAND_FILL; scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; outer.add_child(scroll)
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 32); columns.size_flags_horizontal = Control.SIZE_EXPAND_FILL; scroll.add_child(columns)
	var left := VBoxContainer.new(); left.size_flags_horizontal = Control.SIZE_EXPAND_FILL; left.custom_minimum_size.x = 420; left.add_theme_constant_override("separation", 8); columns.add_child(left)
	var right := VBoxContainer.new(); right.size_flags_horizontal = Control.SIZE_EXPAND_FILL; right.custom_minimum_size.x = 420; right.add_theme_constant_override("separation", 8); columns.add_child(right)

	_heading(left, "HERO")
	var heroes: Array = []
	for hero in catalog.heroes: heroes.append([_hero_text(hero), str(hero.id)])
	_hero = _option(left, "tactics.setup.hero", heroes)
	var hero_detail := Label.new(); hero_detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; hero_detail.add_theme_color_override("font_color", Color("9fb3c8")); hero_detail.add_theme_font_size_override("font_size", 13); left.add_child(hero_detail)
	var show_hero := func(_i = 0):
		for hero in catalog.heroes:
			if str(hero.id) == str(_hero.get_selected_metadata()): hero_detail.text = _hero_detail(hero)
	_hero.item_selected.connect(show_hero); show_hero.call()
	_heading(left, "MAP MOVE CARDS (added to your deck; play one in movement for its Move)")
	var cards := VBoxContainer.new(); left.add_child(cards)
	if not catalog.get("map_cards") is Array or catalog.map_cards.is_empty():
		var none := Label.new(); none.text = "No map Move cards are published."; none.add_theme_color_override("font_color", Color("7d8a99")); cards.add_child(none)
	for card in (catalog.map_cards if catalog.get("map_cards") is Array else []):
		var box := CheckBox.new(); box.text = "%s · Move %d · %d energy%s" % [card.name, int(card.amount), int(card.energy), " · ignores terrain" if card.ignore_terrain else " · choose a terrain" if card.choose_terrain else ""]
		box.tooltip_text = str(card.rules); cards.add_child(box); _map_cards[str(card.id)] = box; _register("tactics.setup.map_card.%s" % card.id, box, box.text)
	_heading(left, "ENEMIES (1–4, placed on the arena's enemy starts in order)")
	var enemy_choices: Array = [[NONE, ""]]
	for enemy in catalog.enemies: enemy_choices.append([_enemy_text(enemy), str(enemy.id)])
	for slot in 4:
		var select := _option(left, "tactics.setup.enemy.%d" % (slot + 1), enemy_choices)
		select.select(1 if slot == 0 else 0)
		_enemies.append(select)

	_heading(right, "ARENA")
	var arenas: Array = []
	for arena in catalog.arenas: arenas.append([str(arena.name), str(arena.id)])
	_arena = _option(right, "tactics.setup.arena", arenas)
	_arena.item_selected.connect(func(_i): _preview.queue_redraw())
	_preview = Control.new(); _preview.custom_minimum_size = Vector2(360, 270); _preview.draw.connect(_draw_preview); right.add_child(_preview)
	_heading(right, "RULE OVERRIDES (this battle only; blank keeps the rules file)")
	var grid := GridContainer.new(); grid.columns = 2; right.add_child(grid)
	var rules: Dictionary = catalog.rules
	for key in ["base_speed", "card_move", "energy_per_move", "map_card_move_multiplier", "sprint_base_multiplier", "sprint_card_bonus"]:
		var label := Label.new(); label.text = key.replace("_", " "); grid.add_child(label)
		var spin := SpinBox.new(); spin.min_value = 0; spin.max_value = 100; spin.value = float(rules.get(key, 0)); grid.add_child(spin)
		_overrides[key] = spin; _register("tactics.setup.rule.%s" % key, spin, key)
	var ai_label := Label.new(); ai_label.text = "ai max cards per round"; grid.add_child(ai_label)
	var ai_spin := SpinBox.new(); ai_spin.max_value = 10; ai_spin.value = float(rules.get("ai", {}).get("max_cards_per_round", 1)); grid.add_child(ai_spin); _overrides["ai.max_cards_per_round"] = ai_spin
	var diagonal_label := Label.new(); diagonal_label.text = "diagonal rule"; grid.add_child(diagonal_label)
	var diagonal := OptionButton.new(); diagonal.add_item("chebyshev"); diagonal.add_item("alternating"); diagonal.select(1 if rules.get("diagonal_rule") == "alternating" else 0); grid.add_child(diagonal); _overrides["diagonal_rule"] = diagonal
	for key in ["corner_cutting", "los_blocked_by_creatures", "area_hits_allies", "charge_counts_as_running", "charge_allows_extra_move"]:
		var box := CheckBox.new(); box.text = key.replace("_", " "); box.button_pressed = bool(rules.get(key, false)); right.add_child(box)
		_overrides[key] = box; _register("tactics.setup.rule.%s" % key, box, key)
	var advanced_label := Label.new(); advanced_label.text = "More overrides (JSON object, e.g. {\"terrain\": {\"swamp\": {\"cost\": 3}}})"; advanced_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; right.add_child(advanced_label)
	_advanced = LineEdit.new(); _advanced.placeholder_text = "{}"; right.add_child(_advanced); _register("tactics.setup.advanced", _advanced, "advanced overrides")
	_heading(right, "SEED (blank: random)")
	_seed = LineEdit.new(); _seed.placeholder_text = "random"; right.add_child(_seed); _register("tactics.setup.seed", _seed, "seed")

	_message = Label.new(); _message.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; _message.add_theme_color_override("font_color", Color("ff8a78")); outer.add_child(_message)
	var buttons := HBoxContainer.new(); buttons.add_theme_constant_override("separation", 12); outer.add_child(buttons)
	_add_button(buttons, "Back", "tactics.setup.back", _back)
	var active := store.load_active()
	if not str(active.get("battle_id", "")).is_empty():
		_resume = _add_button(buttons, "Resume sandbox battle", "tactics.setup.resume", _resume_battle)
	_start = _add_button(buttons, "Start sandbox battle", "tactics.setup.start", _start_battle)
	_start.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_preview.queue_redraw()

func _hero_text(hero: Dictionary) -> String:
	return "%s · %d health · speed %d" % [hero.name, int(hero.health), int(hero.base_speed)]

func _hero_detail(hero: Dictionary) -> String:
	var attacks: Array[String] = []
	for attack in hero.attacks: attacks.append("%s (%s)" % [attack.name, attack.delivery])
	var defenses: Array[String] = []
	for defense in hero.defenses: defenses.append(str(defense.name) + (" (on the move)" if defense.on_the_move else ""))
	return "Attacks: %s.\nDefenses: %s." % [", ".join(attacks), ", ".join(defenses)]

func _enemy_text(enemy: Dictionary) -> String:
	var attacks: Array[String] = []
	for attack in enemy.attacks: attacks.append("%s (%s)" % [attack.name, attack.delivery])
	return "%s · %s · speed %d · %d health · %s" % [enemy.name, str(enemy.get("label", enemy.get("role", ""))), int(enemy.base_speed), int(enemy.health), ", ".join(attacks)]

func _selected_arena() -> Dictionary:
	for arena in catalog.arenas:
		if str(arena.id) == str(_arena.get_selected_metadata()): return arena
	return {}

func _draw_preview() -> void:
	var arena := _selected_arena()
	if arena.is_empty(): return
	var rules: Dictionary = catalog.rules
	var cell := minf(_preview.size.x / float(arena.width), _preview.size.y / float(arena.height))
	for y in int(arena.height):
		for x in int(arena.width):
			var terrain := str(arena.terrain[y][x])
			_preview.draw_rect(Rect2(Vector2(x, y) * cell, Vector2(cell, cell) - Vector2.ONE), Color(str(rules.terrain.get(terrain, {}).get("color", "#333333"))))
	var hero: Dictionary = arena.player_start
	_preview.draw_circle((Vector2(hero.x, hero.y) + Vector2(0.5, 0.5)) * cell, cell * 0.35, Color("4fa3ff"))
	for i in arena.enemy_starts.size():
		var start: Dictionary = arena.enemy_starts[i]
		var used: bool = i < _chosen_enemies().size()
		_preview.draw_circle((Vector2(start.x, start.y) + Vector2(0.5, 0.5)) * cell, cell * 0.3, Color("e0533f") if used else Color("e0533f55"))

func _chosen_enemies() -> Array:
	var chosen: Array = []
	for select in _enemies:
		var id := str(select.get_selected_metadata())
		if not id.is_empty(): chosen.append(id)
	return chosen

func _rule_overrides() -> Dictionary:
	var rules: Dictionary = catalog.rules
	var out := {}
	for key in _overrides:
		var control = _overrides[key]
		var value
		if control is SpinBox: value = int(control.value)
		elif control is CheckBox: value = control.button_pressed
		elif control is OptionButton: value = control.get_item_text(control.selected)
		if key == "ai.max_cards_per_round":
			if value != int(rules.get("ai", {}).get("max_cards_per_round", 1)): out["ai"] = {"max_cards_per_round": value}
			continue
		var current = rules.get(key)
		if typeof(current) == TYPE_FLOAT or typeof(current) == TYPE_INT: current = int(current)
		if value != current: out[key] = value
	var advanced := _advanced.text.strip_edges()
	if not advanced.is_empty():
		var parsed = JSON.parse_string(advanced)
		if parsed is Dictionary: out.merge(parsed, true)
		else: return {"__invalid": true}
	return out

func _start_battle() -> void:
	var enemies := _chosen_enemies()
	if enemies.is_empty(): _message.text = "Choose at least one enemy."; return
	var overrides := _rule_overrides()
	if overrides.has("__invalid"): _message.text = "The advanced overrides are not a JSON object."; return
	var map_cards: Array = []
	for id in _map_cards:
		if _map_cards[id].button_pressed: map_cards.append(id)
	var seed_text := _seed.text.strip_edges()
	var seed := int(seed_text) if seed_text.is_valid_int() and int(seed_text) > 0 else (int(Time.get_unix_time_from_system() * 1000.0) ^ Time.get_ticks_usec()) & 0x3fffffffffff
	var setup := {"hero": str(_hero.get_selected_metadata()), "enemies": enemies, "arena": str(_arena.get_selected_metadata()), "map_cards": map_cards, "seed": seed}
	if not overrides.is_empty(): setup["overrides"] = overrides
	var id := "tactics-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	_start.disabled = true; _message.text = "Starting…"
	var result := gateway.start(id, setup)
	if result.get("accepted") != true:
		_start.disabled = false
		_message.text = "The sandbox battle could not start: %s" % str(result.get("error", ""))
		return
	store.save_active(id, "hero")
	_open_screen(result)

func _resume_battle() -> void:
	var active := store.load_active()
	var result := gateway.open(str(active.get("battle_id", "")))
	if result.get("accepted") != true:
		_message.text = "The saved sandbox battle could not open: %s" % str(result.get("error", ""))
		store.clear()
		if _resume != null: _resume.disabled = true
		return
	_open_screen(result)

func _open_screen(result: Dictionary) -> void:
	var screen = SCREEN.new()
	screen.gateway = gateway; screen.store = store; screen.initial_result = result
	get_tree().root.add_child(screen)
	queue_free()

func _back() -> void:
	get_tree().root.add_child(BOOTSTRAP.instantiate())
	queue_free()
