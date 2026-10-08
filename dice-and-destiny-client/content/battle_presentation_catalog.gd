class_name BattlePresentationCatalog
extends RefCounted

# The authority publishes the exact catalog pinned into the battle. Keeping it
# here lets presentation controls use the pinned definitions. Actor-specific
# upgrades are applied to a fresh presentation dictionary, never the catalog.
const ORDINARY_CURSE_RULES := "Each Curse: choose a random clean die and mark face 1. Once all five dice are cursed, roll an eligible owned die and mark its result. An already cursed result adds Count but no new mark."
const CARD_SUMMARIES := preload("res://content/card_effect_summaries.gd")
const CARD_KEYWORDS := preload("res://content/card_keywords.gd")
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
	var presentation := _dictionary(value.get("presentation", {})).duplicate()
	# Clarify the same extra-roll rule for older pinned battles as well.
	if id == "unquiet_hands" and not value.has("mechanic"):
		presentation["effect_summary"] = "Choose an enemy die: Curse check → +1 Count on a cursed face. Offensive result unchanged."
		presentation["rules_text"] = "Choose a cursed enemy die for a separate roll. A cursed face adds 1 Curse Count; a clean face adds none. No new faces are cursed. Keep its saved offensive result unchanged. Each named Curse card may be played once per player per round."
	return {
		"name": str(value.get("name", _title(id))),
		"cost": int(_dictionary(value.get("cost", {})).get("energy", 0)),
		"text": str(presentation.get("rules_text", "")),
		"art": "res://assets/battle/cards/%s.png" % id,
		"illustration_path": str(presentation.get("illustration_path", "")),
		# Reviewed face text first, then the authority's short summary, then rules.
		"effect_summary": str(CARD_SUMMARIES.TEXT.get(id, presentation.get("effect_summary", "") if not str(presentation.get("effect_summary", "")).is_empty() else presentation.get("rules_text", "No effect"))),
		# Structured timing for the face ribbon; battles pinned before it have none.
		"timing": _array(presentation.get("timing", [])),
		"play_limit": str(presentation.get("play_limit", "")),
		# Card face colours: a palette name or "#rrggbb"; empty uses the default.
		"frame_color": str(presentation.get("frame_color", "")),
		"border_color": str(presentation.get("border_color", "")),
		"targeting": _dictionary(value.get("targeting", {})),
		"play": _dictionary(value.get("play", {})),
		"operations": _array(value.get("operations", [])),
		"program": _dictionary(value.get("program", {})),
		"mechanic": _dictionary(value.get("mechanic", {})),
	}

# Card hover: name, the full rules, then a definition for each keyword and
# status the face uses, so the face can stay short.
static func card_tooltip(id: String) -> String:
	var info := card(id)
	var lines: Array[String] = [str(info.name)]
	if not str(info.text).is_empty(): lines.append(str(info.text))
	var glossary := card_glossary(str(info.effect_summary))
	if not glossary.is_empty(): lines.append("\n" + "\n".join(glossary))
	return "\n".join(lines)

static func card_glossary(face: String) -> Array[String]:
	var entries: Array[String] = []
	for keyword in CARD_KEYWORDS.TERMS:
		var pattern := RegEx.create_from_string(str(keyword.pattern))
		if pattern.search(face) != null: entries.append(str(keyword.text))
	var statuses = _catalog.get("statuses", {})
	if statuses is Dictionary:
		var ids: Array = statuses.keys(); ids.sort()
		for status_id in ids:
			var info := status(str(status_id))
			var rules := str(info.text).get_slice("\n", 0)
			if rules.is_empty() or str(info.name).is_empty(): continue
			if RegEx.create_from_string("\\b" + _regex_escape(str(info.name)) + "\\b").search(face) != null:
				entries.append("%s: %s" % [info.name, rules])
	return entries

static func _regex_escape(text: String) -> String:
	var escaped := ""
	for character in text:
		escaped += "\\" + character if character in ".^$*+?()[]{}|\\" else character
	return escaped

static func configured_ability_damage(id: String, actor: Dictionary) -> int:
	return int(actor.get("configured_ability_damage", {}).get(id, actor.get("needlefang_damage_bonus", 0) if id == "needlefang" else 0))

static func ability(id: String, actor: Dictionary = {}) -> Dictionary:
	var value := definition("abilities", id)
	var presentation := _dictionary(value.get("presentation", {}))
	var rules := str(presentation.get("rules_text", ""))
	if id == "hexbrand" and int(value.get("configuration_version", 0)) == 0 and not rules.contains(ORDINARY_CURSE_RULES): rules += "\n" + ORDINARY_CURSE_RULES
	var bonus := configured_ability_damage(id, actor)
	if id == "needlefang" and bonus > 0 and int(value.get("configuration_version", 0)) == 0:
		var lines: Array[String] = ["Choose a qualified tier (+%d damage included):" % bonus]
		for tier in needlefang_tiers(actor): lines.append(tier.text)
		lines.append("Poison overflow applies Incubation if none exists.")
		rules = "\n".join(lines)
	if bonus > 0 and (id != "needlefang" or int(value.get("configuration_version", 0)) > 0): rules += "\n+%d damage from an active card preparation." % bonus
	var temporary_rules := temporary_ability_damage_rules(id, actor)
	if not temporary_rules.is_empty(): rules += "\n\n" + temporary_rules
	return {
		"name": str(value.get("name", _title(id))),
		"recipe": _ability_recipe(value),
		"text": rules,
		"targeting": _dictionary(value.get("targeting", {})),
		"type": str(value.get("type", "")),
	}

# The recipe may itself be the ability name (Small/Large Straight). Its full
# qualification is already explained in the rules, so don't repeat the heading.
static func ability_tooltip(id: String, actor: Dictionary = {}) -> String:
	var info := ability(id, actor)
	var lines: Array[String] = [str(info.name)]
	var recipe := str(info.recipe).strip_edges()
	if not recipe.is_empty() and recipe.nocasecmp_to(str(info.name).strip_edges()) != 0:
		lines.append("Requires: " + recipe)
	if not str(info.text).is_empty(): lines.append("\n" + str(info.text))
	return "\n".join(lines)

static func temporary_ability_damage_rules(id: String, actor: Dictionary = {}) -> String:
	var lines: Array[String] = []
	var legacy_damage := 0
	# Program bonuses are summed per playing card and duration, so stacked
	# copies read as one "Card: +N damage" line.
	var program_damage := {}
	for modifier in _array(actor.get("ability_modifiers", [])):
		if modifier.get("ability_id") != id: continue
		var status_id := str(modifier.get("status_id", ""))
		if not _array(actor.get("statuses", [])).any(func(status): return status.get("definition_id") == status_id and int(status.get("stacks", 0)) > 0): continue
		var program := _dictionary(modifier.get("program_bonus", {}))
		if not program.is_empty():
			var params := _dictionary(program.get("params", {}))
			var instance := _dictionary(_dictionary(actor.get("card_instances", {})).get(str(modifier.get("source_card_instance_id", "")), {}))
			var name := str(card(str(instance.get("definition_id", ""))).name) if not str(instance.get("definition_id", "")).is_empty() else str(definition("statuses", status_id).get("name", "Preparation"))
			var key := "%s|%s" % [name, str(params.get("duration", "offensive"))]
			program_damage[key] = int(program_damage.get(key, 0)) + int(params.get("damage", 0))
		else:
			for card in _dictionary(_catalog.get("cards", {})).values():
				for operation in _array(card.get("operations", [])):
					var bonus := _dictionary(_dictionary(operation.get("modifier", {})).get("add_conditional_bonus", {}))
					if bonus.get("id", "") != modifier.get("bonus_id", ""): continue
					for effect in _array(bonus.get("operations", [])):
						if effect.get("type") == "deal_damage": legacy_damage += int(effect.get("amount", 0))
	if legacy_damage > 0: lines.append("Strong Swing: +%d damage if this ability qualifies and is selected. Expires at the end of the offensive segment, even if unused." % legacy_damage)
	var expiry := {"offensive": "Expires at the end of the offensive segment, even if unused.", "round": "Expires at the end of the round, even if unused.", "next_use": "Used by the next qualifying attack.", "rounds": "Lasts for its authored number of rounds.", "battle": "Lasts for the rest of the battle."}
	for key in program_damage:
		if int(program_damage[key]) > 0: lines.append("%s: +%d damage if this ability qualifies and is selected. %s" % [str(key).get_slice("|", 0), int(program_damage[key]), expiry.get(str(key).get_slice("|", 1), "")])
	return "\n".join(lines)

# Every attack hover uses the pinned ability, never a separate short rules copy.
# The selected tier comes from the reveal, not a guess from its damage total.
static func attack_ability_tooltip(id: String, actor: Dictionary, reveal: Dictionary, dice: Array) -> String:
	var info := ability(id, actor)
	var lines: Array[String] = [str(info.name)]
	var tier_id := str(reveal.get("tier_id", "")) if reveal.get("ability_id") == id else ""
	if tier_id.is_empty() and actor.get("selected_ability") == id: tier_id = str(actor.get("selected_tier", ""))
	var selected := false
	for tier in offensive_tier_summaries(id, actor):
		if tier.id != tier_id: continue
		lines.append("Selected: %s → %s" % [tier.recipe, tier.summary])
		selected = true
		break
	if not selected and not str(info.recipe).is_empty(): lines.append("Requires: " + str(info.recipe))
	var faces: Array[String] = []
	for die in dice:
		if int(die.get("face", 0)) > 0: faces.append(str(int(die.face)))
	if not faces.is_empty(): lines.append("Offensive dice: " + " · ".join(faces))
	if not str(info.text).is_empty(): lines.append("\n" + str(info.text))
	return "\n".join(lines)

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
			if operation.get("type") == "deal_damage": damage += int(operation.get("amount", 0)) + configured_ability_damage("needlefang", actor)
			if operation.get("type") == "apply_status" and operation.get("status_id") == "poison": poison += int(operation.get("stack_count", 0))
		var recipe := _ability_recipe({"qualification": {"activation_tiers": [tier]}})
		result.append({"id": str(tier.get("id", "")), "label": recipe, "damage": damage, "poison": poison, "text": "%s: %d damage + %d Poison" % [recipe, damage, poison]})
	return result

# Inline benefits come from the battle's pinned tier operations, like the
# requirements, so previews remain correct when a content definition changes.
static func offensive_tier_summaries(id: String, actor: Dictionary = {}) -> Array[Dictionary]:
	var value := definition("abilities", id)
	var result: Array[Dictionary] = []
	for tier in _array(_dictionary(value.get("qualification", {})).get("activation_tiers", [])):
		var benefits: Array[String] = []
		var needs_rules := false
		for operation in _array(tier.get("operations", [])):
			match str(operation.get("type", "")):
				"deal_damage": benefits.append("%d DMG" % (int(operation.get("amount", 0)) + configured_ability_damage(id, actor)))
				"gain_resource": benefits.append("Gain %d %s" % [int(operation.get("amount", 0)), _title(str(operation.get("resource", "energy")))])
				"curse_action":
					var followup := ability_followup(id, str(tier.get("id", "")))
					if followup.is_empty(): needs_rules = true
					else: benefits.append(followup)
				"provoke": benefits.append("Provoke %d" % int(operation.get("amount", 0)))
				"apply_status":
					var status_id := str(operation.get("status_id", ""))
					var name := str(status(status_id).glyph) if status_id == "poison" else str(status(status_id).name)
					benefits.append("%s %d%s%s" % ["Gain" if operation.get("target") == "self" else "Apply", int(operation.get("stack_count", 0)), "" if status_id == "poison" else " ", name])
				_: needs_rules = true
		# Configured abilities put their follow-up (e.g. Curse) in tier hooks.
		if int(value.get("configuration_version", 0)) > 0:
			var hooked := ability_followup(id, str(tier.get("id", "")))
			if not hooked.is_empty(): benefits.append("Then " + hooked.left(1).to_lower() + hooked.substr(1))
		if needs_rules or benefits.is_empty():
			benefits = [str(_dictionary(value.get("presentation", {})).get("effect_summary", ability(id, actor).text))]
		for modifier in _array(_dictionary(value.get("qualification", {})).get("conditional_bonuses", [])):
			for operation in _array(modifier.get("operations", [])):
				if operation.get("type") == "apply_status":
					benefits.append("%s: apply %d %s" % [_ability_recipe({"qualification": {"activation_tiers": [modifier]}}), int(operation.get("stack_count", 0)), status(str(operation.get("status_id", ""))).name])
		result.append({"id": str(tier.get("id", "")), "recipe": _ability_recipe({"qualification": {"activation_tiers": [tier]}}), "summary": " · ".join(benefits)})
	return result

# Bespoke authority hooks need authored explanations; ordinary operations are
# described above. Every tile and selected attack consumes this same wording.
static func ability_followup(id: String, tier_id: String = "") -> String:
	var value := definition("abilities", id)
	if int(value.get("configuration_version", 0)) > 0:
		var lines: Array[String] = []
		for hook in value.get("hooks", []):
			if not str(hook.get("tier_id", "")).is_empty() and str(hook.tier_id) != tier_id: continue
			for op in hook.operations:
				if op.type == "special_effect": lines.append(_special_words(op.special))
				elif op.type == "apply_status": lines.append("Apply %d %s" % [int(op.stack_count), status(str(op.status_id)).name])
		return " · ".join(lines)
	match id:
		"hexbrand": return "Then apply %d Curse" % maxi(1, int(tier_id.trim_prefix("skull_")) - 2)
		"grasp_of_the_sarcophagus": return "Apply 2 Curse · Entomb 1 cursed die\nIf rerolling, bound dice must join until Curse"
		"funeral_rattle": return "Then roll up to 3 cursed enemy dice for Count\nApply Blind if enemy reaches 6 Count"
		"eclipse_of_the_black_star": return "Curse one number across all enemy dice\nIf all 30 faces are cursed: roll all 5 instead"
	return ""

# Structured quantities for bespoke effects. Do not extract numbers from rules
# prose: a roll limit, threshold, and actual status amount mean different things.
static func ability_followup_intents(id: String, tier_id: String = "") -> Array[Dictionary]:
	var value := definition("abilities", id)
	if int(value.get("configuration_version", 0)) > 0:
		var result: Array[Dictionary] = []
		for hook in value.get("hooks", []):
			if not str(hook.get("tier_id", "")).is_empty() and str(hook.tier_id) != tier_id: continue
			for op in hook.operations:
				if op.type == "special_effect":
					var effect: Dictionary = op.special
					# JSON numbers arrive as floats; counts are whole numbers.
					var icon := "curse_count"; var count := str(int(effect.get("amount", 1)))
					if effect.kind == "roll_cursed": icon = "dice"; count = "≤" + count
					elif effect.kind == "status_threshold": icon = str(effect.result_status_id); count = str(int(effect.stacks)) + "?"
					elif effect.kind == "curse_face_choice": count = "1/die"
					result.append({"icon": icon, "count": count, "hint": _special_words(effect)})
					if effect.kind == "entomb_choice": result.append({"icon": "entomb", "count": "1", "hint": _special_words(effect)})
		return result
	match id:
		"hexbrand":
			var count: int = {"skull_3": 1, "skull_4": 2, "skull_5": 3}.get(tier_id, 1)
			return [{"icon": "curse_count", "count": str(count), "hint": ability_followup(id, tier_id)}]
		"grasp_of_the_sarcophagus":
			return [{"icon": "curse_count", "count": "2", "hint": "Apply 2 Curse"}, {"icon": "entomb", "count": "1", "hint": "Entomb 1 cursed die. If rerolling, bound dice must join until Curse."}]
		"funeral_rattle":
			return [{"icon": "dice", "count": "≤3", "hint": "Roll up to 3 cursed enemy dice for Count; the amount of Count depends on the rolls."}, {"icon": "blind", "count": "1?", "hint": "Apply 1 Blind if the enemy reaches 6 Count."}]
		"eclipse_of_the_black_star":
			return [{"icon": "curse_count", "count": "1/die", "hint": ability_followup(id, tier_id)}]
	return []

static func _needlefang_summary_fits(value: Dictionary) -> bool:
	var qualification := _dictionary(value.get("qualification", {}))
	if not _array(value.get("hooks", [])).is_empty() or not _array(qualification.get("conditional_bonuses", [])).is_empty(): return false
	for tier in _array(qualification.get("activation_tiers", [])):
		var requirements := _array(_dictionary(tier.get("requirements", {})).get("all", []))
		if requirements.size() != 1 or requirements[0].get("symbol_id") != "fang": return false
		for op in _array(tier.get("operations", [])):
			if op.get("target") != "selected_targets": return false
			if op.get("type") != "deal_damage" and not (op.get("type") == "apply_status" and op.get("status_id") == "poison"): return false
	return true

static func inline_tiers(id: String, actor: Dictionary = {}) -> Array[Dictionary]:
	if id == "needlefang" and _needlefang_summary_fits(definition("abilities", id)):
		var options := needlefang_tiers(actor)
		for option in options:
			option.summary = "%d DMG +%d%s" % [option.damage, option.poison, status("poison").glyph]
			option.icon = "res://assets/battle/wasteland/fang.svg"
			option.button_label = str(option.label).get_slice(" ", 0)
		return options
	# Minimum-count tiers let the player choose a lower qualified outcome.
	var tiers := _array(_dictionary(definition("abilities", id).get("qualification", {})).get("activation_tiers", []))
	if tiers.size() < 2: return []
	var symbol_id := ""
	for tier in tiers:
		var requirements := _array(_dictionary(tier.get("requirements", {})).get("all", []))
		if requirements.size() != 1 or requirements[0].get("type") != "symbol_count" or not requirements[0].has("minimum"): return []
		if not symbol_id.is_empty() and symbol_id != str(requirements[0].get("symbol_id")): return []
		symbol_id = str(requirements[0].get("symbol_id"))
	var options: Array[Dictionary] = []
	for tier in offensive_tier_summaries(id, actor):
		var summary := str(tier.summary).replace(" · Then apply ", "\n+")
		options.append({"id": tier.id, "label": tier.recipe, "button_label": str(tier.recipe).get_slice(" ", 0) + " " + str(definition("symbols", symbol_id).get("glyph", "")), "summary": summary, "text": "%s: %s" % [tier.recipe, tier.summary]})
	options.sort_custom(func(a, b): return str(a.label).naturalnocasecmp_to(str(b.label)) < 0)
	return options

static func defense_lines(id: String) -> Array[String]:
	var configured := definition("abilities", id)
	if int(configured.get("configuration_version", 0)) > 0: return [str(ability(id).text)]
	match id:
		"shedskin": return ["2 owned dice · 0 Energy", "Each Fang  Prevent 1", "Each Gland  Gain 1 Catalyst", "Any Coil  Apply 1 Incubation", "Optional: pay 1 Catalyst → prevent 2 more"]
		"barbed_mantle": return ["1 owned die · 1 Energy", "Fang  Prevent 2 · Apply 1 Poison", "Gland  Prevent 3 · Gain 1 Catalyst", "Coil  Prevent 1 · Apply 1 Incubation", "  if poisoned, no Incubation;", "  otherwise apply 1 Poison"]
		"hexward_rebuttal": return ["1 owned die · 0 Energy", "Skull  Prevent 2 · Shroud  Prevent 3", "Omen  Prevent 1", "Then apply 1 Curse to the attacker"]
		"misfortune_repaid": return ["2 owned dice · 1 Energy", "Each Skull  Prevent 1 · Shroud  Prevent 2", "Any Omen  Apply 2 Count once", "If damage prevented: choose a die", "Clean: seed 1 · Partial: Expand · Full: Surge"]
	var value := definition("abilities", id)
	var cost := "%d Energy" % int(_dictionary(value.get("cost", {})).get("energy", 0))
	for op in _array(_dictionary(value.get("resolution", {})).get("operations", [])):
		if op.get("type") == "roll_dice": cost = "%d owned dice · %s" % [int(op.get("dice_count", 1)), cost]
	return [cost, str(ability(id).text)]

static func status(id: String) -> Dictionary:
	var value := definition("statuses", id)
	var presentation := _dictionary(value.get("presentation", {}))
	return {
		"name": "Malediction’s Refusal" if id == "maledictions_refusal" else _title(id) if id in ["grave_interest", "second_knell", "black_dividend", "curse_bloom"] else str(value.get("name", _title(id))),
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
		"exact_faces":
			var faces: Array[String] = []
			for face in requirement.get("faces", []): faces.append(str(int(face)))
			return "–".join(faces)
	return "Requirement"

static func _title(id: String) -> String:
	return id.replace("_", " ").capitalize()

static func _array(value) -> Array:
	return value if value is Array else []

static func _dictionary(value) -> Dictionary:
	return value if value is Dictionary else {}

static func temporary_ability_damage(id: String, actor: Dictionary) -> int:
	var total := 0
	for modifier in _array(actor.get("ability_modifiers", [])):
		if modifier.get("ability_id") != id: continue
		var status_id := str(modifier.get("status_id", ""))
		if not _array(actor.get("statuses", [])).any(func(status): return status.get("definition_id") == status_id and int(status.get("stacks", 0)) > 0): continue
		var program := _dictionary(modifier.get("program_bonus", {}))
		if not program.is_empty():
			total += int(_dictionary(program.get("params", {})).get("damage", 2)); continue
		for value in _dictionary(_catalog.get("cards", {})).values():
			for operation in _array(value.get("operations", [])):
				var bonus := _dictionary(_dictionary(operation.get("modifier", {})).get("add_conditional_bonus", {}))
				if bonus.get("id", "") != modifier.get("bonus_id", ""): continue
				for effect in _array(bonus.get("operations", [])):
					if effect.get("type") == "deal_damage": total += int(effect.get("amount", 0))
	return total

static func card_mechanic(id: String) -> String:
	var value := definition("cards", id)
	return str(value.get("mechanic", {}).get("kind", id))

static func _special_words(effect: Dictionary) -> String:
	match str(effect.get("kind", "")):
		"curse": return "Apply %d Curse" % int(effect.amount)
		"roll_cursed": return "Roll up to %d cursed dice for Count" % int(effect.amount)
		"entomb_choice": return "Apply %d Curse; Entomb one die (limit %d)" % [int(effect.amount), int(effect.limit)]
		"curse_face_choice": return "Curse one number across enemy dice; if full, roll up to %d dice" % int(effect.amount)
		"curse_die_choice": return "Choose a die: seed face %d, expand, or Surge if full" % int(effect.face)
		"conditional_status": return "If %s is at least %d and %s is below %d, apply %d %s; otherwise apply %d %s" % [status(str(effect.status_id)).name, int(effect.threshold), status(str(effect.result_status_id)).name, int(effect.limit), int(effect.stacks), status(str(effect.result_status_id)).name, int(effect.fallback_stacks), status(str(effect.fallback_status_id)).name]
		"status_threshold": return "At %d %s, apply %d %s" % [int(effect.threshold), status(str(effect.status_id)).name, int(effect.stacks), status(str(effect.result_status_id)).name]
	return ""
