extends Control

const LIBRARY := preload("res://content/battle_visuals/library.tres")
const SCENERY := preload("res://presentation/battle/battle_scenery.gd")
var _scenery: Control
var _enemy: OptionButton
var _habitat: OptionButton
var _pair: CheckButton
var _collection: OptionButton
var _minion_factions: Array = []

func _ready() -> void:
	_scenery = SCENERY.new(); add_child(_scenery)
	_scenery.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	var panel := PanelContainer.new(); add_child(panel)
	panel.set_anchors_and_offsets_preset(Control.PRESET_TOP_WIDE)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 20); panel.add_child(row)
	var title := Label.new(); title.text = "Battle art preview"; row.add_child(title)
	_collection = OptionButton.new(); _collection.name = "CollectionPicker"; row.add_child(_collection)
	_collection.add_item("Bosses / characters")
	if FileAccess.file_exists("res://content/battle_visuals/minion_catalog.json"):
		var catalog: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://content/battle_visuals/minion_catalog.json"))
		for faction in catalog.get("factions", []):
			if faction.get("minions", []).is_empty(): continue
			_minion_factions.append(faction)
			_collection.add_item(str(faction.boss_name) + " minions")
	_enemy = OptionButton.new(); _enemy.name = "EnemyPicker"; row.add_child(_enemy)
	_populate_enemies()
	_habitat = OptionButton.new(); _habitat.name = "HabitatPicker"; row.add_child(_habitat)
	for habitat in LIBRARY.habitats:
		_habitat.add_item(habitat.id.replace("_", " ").capitalize())
		if habitat.id == "mushroom_cavern": _habitat.select(_habitat.item_count - 1)
	_pair = CheckButton.new(); _pair.text = "Two copies"; row.add_child(_pair)
	_collection.item_selected.connect(func(_index): _populate_enemies(); _refresh())
	_enemy.item_selected.connect(func(_index): _refresh())
	_habitat.item_selected.connect(func(_index): _refresh())
	_pair.toggled.connect(func(_pressed): _refresh())
	_refresh()

func _populate_enemies() -> void:
	_enemy.clear()
	if _collection.selected == 0:
		for profile in LIBRARY.fighters:
			_enemy.add_item(profile.display_name)
			_enemy.set_item_metadata(_enemy.item_count - 1, profile.definition_id)
			if profile.definition_id == "spore_cantor": _enemy.select(_enemy.item_count - 1)
	else:
		for minion in _minion_factions[_collection.selected - 1].minions:
			_enemy.add_item(str(minion.name))
			_enemy.set_item_metadata(_enemy.item_count - 1, minion.profile)
		_enemy.select(0)

func _refresh() -> void:
	if _enemy.item_count == 0: return
	var layout := BattleEncounterVisual.new()
	var selected_habitat = LIBRARY.habitats[_habitat.selected]
	layout.habitat = selected_habitat
	var player := BattleFighterPlacement.new()
	player.instance_id = "player"; player.profile = LIBRARY.fighter("venom")
	player.ground_position = Vector2(720, 700); player.face_left = false; player.draw_order = 2
	layout.fighters.append(player)
	var selected := str(_enemy.get_selected_metadata())
	# The 200 minion textures are loaded on selection, not when a battle opens.
	var profile: FighterVisualProfile = load(selected) if selected.begins_with("res://") else LIBRARY.fighter(selected)
	if profile == null: return
	for i in (2 if _pair.button_pressed else 1):
		var enemy := BattleFighterPlacement.new()
		enemy.instance_id = "enemy_%d" % i; enemy.profile = profile
		enemy.ground_position = Vector2(1435, 585) if not _pair.button_pressed else Vector2(1310 + i * 240, 550 + i * 150)
		enemy.height = profile.default_height * (0.8 if _pair.button_pressed else 1.0)
		enemy.draw_order = i
		layout.fighters.append(enemy)
	_scenery.display(layout)
