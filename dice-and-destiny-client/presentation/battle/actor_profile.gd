class_name ActorProfile
extends PanelContainer

var portrait: TextureRect
var title: Label
var health: ProgressBar
var stats: Label
var statuses: Label

func _ready() -> void:
	custom_minimum_size = Vector2(250, 205)
	var row := HBoxContainer.new()
	add_child(row)
	portrait = TextureRect.new()
	portrait.custom_minimum_size = Vector2(100, 170)
	portrait.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	portrait.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	row.add_child(portrait)
	var info := VBoxContainer.new()
	info.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	row.add_child(info)
	title = Label.new(); title.add_theme_font_size_override("font_size", 22); info.add_child(title)
	health = ProgressBar.new(); health.show_percentage = false; health.custom_minimum_size.y = 18; info.add_child(health)
	stats = Label.new(); info.add_child(stats)
	statuses = Label.new(); statuses.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; info.add_child(statuses)

func display(actor_id: String, actor: Dictionary, is_player: bool) -> void:
	var definition := str(actor.get("definition_id", actor_id))
	title.text = BattlePresentationCatalog.ability(definition).get("name", definition.replace("_", " ").capitalize())
	if definition == "blade_warden": title.text = "Blade Warden"
	if definition == "venom_goblin": title.text = "Venom Goblin"
	var current := int(actor.get("current_health", 0)); var maximum := maxi(1, int(actor.get("max_health", current)))
	health.max_value = maximum; health.value = current
	stats.text = "Health %d/%d    ✦ Energy %d\nDeck %d  Hand %d  Discard %d  Removed %d" % [current, maximum, int(actor.get("energy_points", 0)), int(actor.get("deck_count", 0)), int(actor.get("hand_count", 0)), int(actor.get("discard_count", 0)), int(actor.get("removed_count", 0))]
	var status_text: Array[String] = []
	for entry in actor.get("statuses", []):
		var id := str(entry.get("definition_id", entry.get("id", "status")))
		status_text.append("%s %s ×%d" % [BattlePresentationCatalog.status(id).glyph, BattlePresentationCatalog.status(id).name, int(entry.get("stacks", 1))])
	statuses.text = "No active statuses" if status_text.is_empty() else "\n".join(status_text)
	var path := "res://assets/battle/portraits/blade_warden.png" if is_player else "res://assets/battle/portraits/venom_goblin.png"
	if ResourceLoader.exists(path): portrait.texture = load(path)
	tooltip_text = "%s authoritative profile" % title.text
