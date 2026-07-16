class_name BattleAbilityTile
extends Button

var ability_id := ""

func configure(id: String, qualified: bool, selected: bool, enabled: bool) -> void:
	ability_id = id
	var data := BattlePresentationCatalog.ability(id)
	text = "%s\n%s" % [data.name, data.recipe]
	tooltip_text = "%s — %s" % [data.name, data.text]
	custom_minimum_size = Vector2(130, 70)
	disabled = not enabled
	modulate = Color("ffffff") if qualified or selected else Color("87909c")
	if selected: add_theme_color_override("font_color", Color("6fd8ff"))
