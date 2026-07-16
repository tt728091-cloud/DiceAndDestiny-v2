class_name BattleCard
extends Button

var instance_id := ""
var definition_id := ""

func configure(instance: String, definition: String, enabled: bool, pending_removal: bool = false) -> void:
	instance_id = instance; definition_id = definition
	var data := BattlePresentationCatalog.card(definition)
	text = "%s  %d✦%s" % [data.name, int(data.cost), "\n⚔ PENDING" if pending_removal else ""]
	clip_text = true
	tooltip_text = "%s (%s) — %s" % [data.name, instance, data.text]
	custom_minimum_size = Vector2(150, 105)
	add_theme_font_size_override("font_size", 12)
	var art_path := str(data.get("art", ""))
	if not art_path.is_empty() and ResourceLoader.exists(art_path):
		icon = load(art_path)
		expand_icon = true
		add_theme_constant_override("icon_max_width", 42)
	disabled = not enabled
	if pending_removal: modulate = Color("ff8a70")
