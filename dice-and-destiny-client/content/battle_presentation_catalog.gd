class_name BattlePresentationCatalog
extends RefCounted

# The authority publishes the exact catalog pinned into the battle. Keeping it
# here lets presentation controls use the pinned definitions. Actor-specific
# upgrades are applied to a fresh presentation dictionary, never the catalog.
const CARD_SUMMARIES := preload("res://content/card_effect_summaries.gd")
static var _catalog: Dictionary = {}

static func configure(catalog: Dictionary) -> void:
	_catalog = catalog.duplicate(true)

static func definition(kind: String, id: String) -> Dictionary:
	var values = _catalog.get(kind, {})
	if values is Dictionary:
		var value = values.get(id, {})
		if value is Dictionary: return value
	return {}

static func card(id: String) -> Dictionary:
	var value := definition("cards", id)
	var presentation := _dictionary(value.get("presentation", {}))
	return {
		"name": str(value.get("name", _title(id))),
		"cost": int(_dictionary(value.get("cost", {})).get("energy", 0)),
		"text": str(presentation.get("rules_text", "")),
		"art": "res://assets/battle/cards/%s.png" % id,
		"effect_summary": str(presentation.get("effect_summary", CARD_SUMMARIES.TEXT.get(id, presentation.get("rules_text", "No effect")))),
		"targeting": _dictionary(value.get("targeting", {})),
		"play": _dictionary(value.get("play", {})),
		"operations": _array(value.get("operations", [])),
	}

static func ability(id: String, actor: Dictionary = {}) -> Dictionary:
	var value := definition("abilities", id)
	var presentation := _dictionary(value.get("presentation", {}))
	var rules := str(presentation.get("rules_text", ""))
	var bonus := int(actor.get("needlefang_damage_bonus", 0))
	if id == "needlefang" and bonus > 0:
		var lines: Array[String] = ["Choose a qualified tier (Venom Lens +%d damage included):" % bonus]
		for tier in needlefang_tiers(actor): lines.append(tier.text)
		lines.append("Poison overflow applies Incubation if none exists.")
		rules = "\n".join(lines)
	return {
		"name": str(value.get("name", _title(id))),
		"recipe": _ability_recipe(value),
		"text": rules,
		"targeting": _dictionary(value.get("targeting", {})),
	}

# Shared by the whole-ability hover and the inline tier buttons.
static func needlefang_tiers(actor: Dictionary = {}) -> Array[Dictionary]:
	var value := definition("abilities", "needlefang")
	var tiers := _array(_dictionary(value.get("qualification", {})).get("activation_tiers", [])).duplicate()
	tiers.sort_custom(func(a, b): return str(a.get("id", "")) < str(b.get("id", "")))
	var result: Array[Dictionary] = []
	for tier in tiers:
		var damage := 0
		var poison := 0
		for operation in _array(tier.get("operations", [])):
			if operation.get("type") == "deal_damage": damage += int(operation.get("amount", 0)) + int(actor.get("needlefang_damage_bonus", 0))
			if operation.get("type") == "apply_status" and operation.get("status_id") == "poison": poison += int(operation.get("stack_count", 0))
		var recipe := _ability_recipe({"qualification": {"activation_tiers": [tier]}})
		result.append({"id": str(tier.get("id", "")), "label": recipe, "damage": damage, "poison": poison, "text": "%s: %d damage + %d Poison" % [recipe, damage, poison]})
	return result

# Inline benefits come from the battle's pinned tier operations, like the
# requirements, so previews remain correct when a content definition changes.
static func offensive_tier_summaries(id: String) -> Array[Dictionary]:
	var value := definition("abilities", id)
	var result: Array[Dictionary] = []
	for tier in _array(_dictionary(value.get("qualification", {})).get("activation_tiers", [])):
		var benefits: Array[String] = []
		for operation in _array(tier.get("operations", [])):
			match str(operation.get("type", "")):
				"deal_damage": benefits.append("%d DMG" % int(operation.get("amount", 0)))
				"provoke": benefits.append("Provoke %d" % int(operation.get("amount", 0)))
				"apply_status":
					var status_id := str(operation.get("status_id", ""))
					var name := str(status(status_id).glyph) if status_id == "poison" else str(status(status_id).name)
					benefits.append("%s %d%s%s" % ["Gain" if operation.get("target") == "self" else "Apply", int(operation.get("stack_count", 0)), "" if status_id == "poison" else " ", name])
		result.append({"id": str(tier.get("id", "")), "recipe": _ability_recipe({"qualification": {"activation_tiers": [tier]}}), "summary": " · ".join(benefits)})
	return result

static func status(id: String) -> Dictionary:
	var value := definition("statuses", id)
	var presentation := _dictionary(value.get("presentation", {}))
	return {
		"name": str(value.get("name", _title(id))),
		"text": str(presentation.get("rules_text", "")),
		"glyph": str(presentation.get("glyph", "•")),
		"polarity": str(value.get("polarity", "")),
	}

static func symbol_for_die_face(die_id: String, face: int) -> String:
	var die := definition("dice", die_id)
	for entry in _array(die.get("faces", [])):
		if entry is Dictionary and int(entry.get("number", 0)) == face:
			var symbol := definition("symbols", str(entry.get("symbol", "")))
			return str(symbol.get("glyph", "•"))
	return "•"

static func symbol_name_for_die_face(die_id: String, face: int) -> String:
	var die := definition("dice", die_id)
	for entry in _array(die.get("faces", [])):
		if entry is Dictionary and int(entry.get("number", 0)) == face:
			var symbol := definition("symbols", str(entry.get("symbol", "")))
			return str(symbol.get("name", _title(str(entry.get("symbol", "symbol")))))
	return "Unknown symbol"

# Compatibility for presentation call sites that do not yet carry a die ID.
static func symbol_for_face(face: int) -> String:
	return symbol_for_die_face("standard_d6", face)

static func symbol_name(face: int) -> String:
	return symbol_name_for_die_face("standard_d6", face)

static func _ability_recipe(value: Dictionary) -> String:
	var qualification := _dictionary(value.get("qualification", {}))
	var tiers := _array(qualification.get("activation_tiers", []))
	if not tiers.is_empty():
		var labels: Array[String] = []
		for tier in tiers:
			if not tier is Dictionary: continue
			var requirements := _dictionary(tier.get("requirements", {}))
			var parts: Array[String] = []
			for requirement in _array(requirements.get("all", [])):
				if requirement is Dictionary: parts.append(_requirement_text(requirement))
			labels.append(" + ".join(parts))
		return " / ".join(labels)
	var selection := _dictionary(value.get("selection", {}))
	if not selection.is_empty():
		return "Select %d source%s" % [int(selection.get("target_count", 1)), "s" if int(selection.get("target_count", 1)) != 1 else ""]
	return ""

static func _requirement_text(requirement: Dictionary) -> String:
	match str(requirement.get("type", "")):
		"symbol_count":
			var count := int(requirement.get("exact", requirement.get("minimum", 0)))
			return "%d %s" % [count, definition("symbols", str(requirement.get("symbol_id", ""))).get("name", _title(str(requirement.get("symbol_id", ""))))]
		"number_pattern": return str(requirement.get("pattern", "")).replace("_", " ").capitalize()
		"exact_faces": return "Faces %s" % str(requirement.get("faces", []))
	return "Requirement"

static func _title(id: String) -> String:
	return id.replace("_", " ").capitalize()

static func _array(value) -> Array:
	return value if value is Array else []

static func _dictionary(value) -> Dictionary:
	return value if value is Dictionary else {}
