extends HBoxContainer
## One attack's pending card removals, totalled by the pile each card leaves.
## Readable even while the attack's card list is folded away.
const ICONS := preload("res://presentation/battle/battle_icons.gd")
const INK := preload("res://presentation/battle/cinematic_theme.gd")
const ZONES := [["hand", "hand"], ["deck", "draw pile"], ["discard", "discard pile"]]
var counts: Dictionary = {}

static func zone_counts(cards: Array) -> Dictionary:
	var result := {}
	for card in cards:
		var zone := str(card.get_meta("removal_origin_zone", ""))
		if zone.is_empty(): continue
		result[zone] = int(result.get(zone, 0)) + 1
	return result

func configure(value: Dictionary, font_size: int = 16) -> void:
	counts = value
	name = "RemovalTally"
	add_theme_constant_override("separation", 3)
	alignment = BoxContainer.ALIGNMENT_END
	var parts: Array[String] = []
	for zone in ZONES:
		var count := int(counts.get(zone[0], 0))
		if count <= 0: continue
		var icon := TextureRect.new(); icon.name = "Zone_" + str(zone[0]); icon.texture = ICONS.texture(str(zone[0]))
		icon.custom_minimum_size = Vector2.ONE * (font_size + 2); icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; icon.mouse_filter = Control.MOUSE_FILTER_IGNORE
		add_child(icon)
		var label := Label.new(); label.name = "Count_" + str(zone[0]); label.text = str(count)
		label.add_theme_font_size_override("font_size", font_size); label.mouse_filter = Control.MOUSE_FILTER_IGNORE
		INK.hud_lettering(label, true); add_child(label)
		parts.append("%d from %s" % [count, zone[1]])
	tooltip_text = "Cards being removed: " + ", ".join(parts)
	visible = not parts.is_empty()
