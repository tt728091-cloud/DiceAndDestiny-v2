extends VBoxContainer
## Quick card adjustments for tree variants. Offers only valid one-step changes
## to the predecessor's own fields (energy, effect values, saved-card
## destinations, compatible extra effects); the full editor covers the rest.
signal committed(card: Dictionary)
signal cancelled
signal full_editor_requested(card: Dictionary)

const STYLE = preload("res://app/screens/character/character_style.gd")
const DIFF = preload("res://app/screens/character/card_tree_diff.gd")
## Extra effects offered as one-click additions when their timing fits.
const QUICK_EFFECTS := ["draw", "energy", "prevent", "remove_status", "curse"]

var catalog: Dictionary = {}
var runtime: Node
## upgrade, downgrade or edit.
var mode := "upgrade"
var compare_card: Dictionary = {}
var compare_name := ""
## Every card this one is compared with. A converging card in Quick edit has
## one per incoming route; compare_card stays the first for field highlights.
var compare_cards: Array = []
var card: Dictionary = {}
var used_names: Dictionary = {}
## Owned by the workshop so adjustments survive re-rendering: {card, name_touched}.
var session: Dictionary = {}
var _error := ""
var _name_field: LineEdit
var _commit: Button
var _error_label: Label

func start(kind: String, price: int, fresh: bool) -> void:
	mode = kind; card = session.card
	if fresh and mode != "edit":
		card.economy.buy = maxi(1, price); card.economy.sell = card.economy.buy
		card.name = _suggest_name()
	_validate(); rebuild()

func rebuild() -> void:
	for child in get_children(): remove_child(child); child.queue_free()
	add_theme_constant_override("separation", 10)
	STYLE.heading(self, {"upgrade": "Forge an upgrade", "downgrade": "Forge a cheaper variant", "edit": "Quick edit"}[mode])
	STYLE.label(self, ("Adjusting a copy of %s. Pick one or more changes; everything else stays the same." % compare_name) if mode != "edit" else "Adjust this card in place. Changes compare with %s." % " and ".join(_compares().map(func(c): return str(c.name))), 14, STYLE.MUTED)
	_preview()
	var identity := STYLE.section(self)
	STYLE.heading(identity, "Name & value")
	_name_field = LineEdit.new(); _name_field.text = str(card.name); _name_field.set_meta("tree_field", "forge.name"); identity.add_child(_name_field)
	_name_field.text_changed.connect(func(v): session.name_touched = true; card.name = v; _validate(false); _show_error())
	var deltas: Array = _compares().map(func(c): return "%+d from %s" % [int(card.economy.buy) - int(c.get("economy", {}).get("buy", card.economy.buy)), c.name])
	_stepper(identity, "XP value", int(card.economy.buy), " · ".join(deltas), 1, 100000, func(v): card.economy.buy = v; card.economy.sell = v, "forge.xp", int(compare_card.get("economy", {}).get("buy", -1)))
	var adjust := STYLE.section(self)
	STYLE.heading(adjust, "Adjust")
	_stepper(adjust, "Energy cost", int(card.cost.energy), "", 0, 100, func(v): card.cost.energy = v, "forge.energy", int(compare_card.get("cost", {}).get("energy", -1)))
	if card.get("program") is Dictionary: _program_rows(adjust)
	elif card.get("mechanic") is Dictionary: _mechanic_rows(adjust)
	var timing: Dictionary = card.program if card.get("program") is Dictionary else card.get("mechanic", {})
	if not timing.is_empty():
		_stepper(adjust, "Uses per round", int(timing.get("uses_per_round", 0)), "0 = unlimited", 0, 100, func(v): timing.uses_per_round = v, "forge.uses_per_round", int(_compare_timing().get("uses_per_round", -1)))
	_enum_row(adjust, "After play, card goes to", ["discard", "removed", "hand", "deck"], str(card.play.destination), func(v): card.play.destination = v, "forge.play_destination", str(compare_card.get("play", {}).get("destination", "")))
	if card.get("program") is Dictionary:
		var additions := STYLE.section(self)
		STYLE.heading(additions, "Add an effect")
		var flow := HFlowContainer.new(); flow.add_theme_constant_override("h_separation", 8); flow.add_theme_constant_override("v_separation", 8); additions.add_child(flow)
		var any := false
		for effect in QUICK_EFFECTS:
			if not catalog.get("effects", {}).has(effect) or not _fits(effect): continue
			var step := _new_step(effect)
			var b := _button(flow, "+ " + DIFF.step_phrase(step, catalog), func(): _add_step(effect), "forge.add." + effect)
			b.tooltip_text = "Adds this effect after the existing ones. Play timing narrows to windows every effect supports."
			any = true
		if not any: STYLE.label(additions, "No quick additions fit this card's play timing. Use the full editor for other effects.", 14, STYLE.MUTED)
	var full := _button(self, "Open full card editor…", func(): full_editor_requested.emit(card.duplicate(true)), "forge.full")
	full.tooltip_text = "Edit every setting, including targets, conditions and choices. Returns here afterwards."
	_error_label = STYLE.label(self, "", 15, STYLE.LOSS)
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 8); add_child(row)
	var commit := STYLE.accent(_button(row, {"upgrade": "Create upgrade ↑", "downgrade": "Create cheaper variant ↓", "edit": "Apply changes"}[mode], func(): committed.emit(card.duplicate(true)), "forge.commit"))
	commit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_commit = commit
	_button(row, "Cancel", func(): cancelled.emit(), "forge.cancel")
	_show_error()

func _show_error() -> void:
	if not is_instance_valid(_commit): return
	_error_label.text = _error; _error_label.visible = not _error.is_empty()
	_commit.disabled = not _error.is_empty()

func _preview() -> void:
	var box := STYLE.section(self)
	var top := HBoxContainer.new(); top.add_theme_constant_override("separation", 12); box.add_child(top)
	var art := TextureRect.new(); art.texture = STYLE.texture(str(card.get("presentation", {}).get("illustration_path", "")))
	art.custom_minimum_size = Vector2(84, 118); art.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; art.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED; top.add_child(art)
	var text := VBoxContainer.new(); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; top.add_child(text)
	STYLE.label(text, str(card.name), 20, STYLE.GOLD_BRIGHT)
	STYLE.label(text, "%d energy · %d XP" % [int(card.cost.energy), int(card.economy.buy)], 15, STYLE.MUTED)
	STYLE.label(text, str(card.get("presentation", {}).get("rules_text", "")), 15)
	var compares := _compares()
	var blocks: Array = []
	for before in compares:
		var lines := ""
		if compares.size() > 1: lines = "[color=#e6c17c]From %s[/color]\n" % str(before.name).replace("[", "[lb]")
		var entries := DIFF.with_xp(before, card, catalog)
		lines += DIFF.bbcode(entries)
		if DIFF.changes(before, card, catalog).is_empty(): lines += ("\n" if not entries.is_empty() else "") + "[color=#9cabb7]No effect changes yet · choose an adjustment below.[/color]"
		blocks.append(lines)
	var diff := RichTextLabel.new(); diff.bbcode_enabled = true; diff.fit_content = true; diff.scroll_active = false
	diff.add_theme_font_size_override("normal_font_size", 15); diff.mouse_filter = Control.MOUSE_FILTER_IGNORE
	diff.text = "\n\n".join(blocks)
	diff.set_meta("tree_field", "forge.changes")
	box.add_child(diff)

func _compares() -> Array:
	return compare_cards if not compare_cards.is_empty() else [compare_card]

func _compare_timing() -> Dictionary:
	if compare_card.get("program") is Dictionary: return compare_card.program
	return compare_card.get("mechanic", {})

func _program_rows(parent: VBoxContainer) -> void:
	var steps: Array = card.program.steps
	var before: Array = compare_card.get("program", {}).get("steps", []) if compare_card.get("program") is Dictionary else []
	for i in steps.size():
		_step_rows(parent, steps[i], before[i] if i < before.size() and before[i].effect == steps[i].effect else {}, str(i), "")
		if steps.size() > 1:
			var index := i
			_button(parent, "Remove “%s”" % DIFF.step_phrase(steps[i], catalog), func(): steps.remove_at(index); _changed(), "forge.step.%d.remove" % i)

func _step_rows(parent: VBoxContainer, step: Dictionary, before: Dictionary, path: String, prefix: String) -> void:
	var spec: Dictionary = catalog.get("effects", {}).get(str(step.effect), {})
	var label := prefix + str(spec.get("label", step.effect))
	var keys: Array = spec.get("parameters", {}).keys(); keys.sort()
	for key in keys:
		var param: Dictionary = spec.parameters[key]
		if step.effect == "ability_bonus" and key == "rounds" and step.params.get("duration", "offensive") != "rounds": continue
		var value = step.params.get(key, param.get("default"))
		var old = before.get("params", {}).get(key, param.get("default")) if not before.is_empty() else null
		var caption := label if key == "amount" else "%s · %s" % [label, str(key).trim_suffix("_id").replace("_", " ")]
		if key == "destination" and step.effect in ["prevent", "save_cards"]: caption = "Saved cards go to"
		var field_key := "forge.step.%s.%s" % [path, key]
		match str(param.type):
			"integer": _stepper(parent, caption, int(value), "", int(param.get("minimum", 0)), int(param.get("maximum", 100)), func(v): step.params[key] = v, field_key, int(old) if old != null else -1)
			"enum": _enum_row(parent, caption, param.options, str(value), func(v): step.params[key] = v, field_key, str(old) if old != null else "")
			"boolean":
				var check := CheckBox.new(); check.text = caption; check.button_pressed = bool(value); check.set_meta("tree_field", field_key); parent.add_child(check)
				check.toggled.connect(func(on): step.params[key] = on; _changed())
			"status", "status_optional":
				var ids: Array = catalog.get("statuses", {}).keys(); ids.sort()
				if param.type == "status_optional": ids.push_front("")
				_enum_row(parent, caption, ids, str(value), func(v): step.params[key] = v, field_key, str(old) if old != null else "")
	var target: Dictionary = step.get("target", {})
	if str(target.get("mode", "one")) in ["exact", "up_to"]:
		_stepper(parent, label + " · targets", int(target.get("count", 1)), "", 1, int(catalog.get("limits", {}).get("count", 100)), func(v): target.count = v, "forge.step.%s.count" % path, int(before.get("target", {}).get("count", -1)) if not before.is_empty() else -1)
	var options: Array = step.get("choices", [])
	for k in options.size():
		var option: Dictionary = options[k]
		var old_option: Dictionary = before.get("choices", [])[k] if k < before.get("choices", []).size() else {}
		_stepper(parent, "%s · extra energy" % option.name, int(option.get("energy", 0)), "", 0, 100, func(v): option.energy = v, "forge.step.%s.option.%d.energy" % [path, k], int(old_option.get("energy", -1)))
		for j in option.get("steps", []).size():
			var old_steps: Array = old_option.get("steps", [])
			var nested: Dictionary = option.steps[j]
			_step_rows(parent, nested, old_steps[j] if j < old_steps.size() and old_steps[j].effect == nested.effect else {}, "%s.option.%d.%d" % [path, k, j], str(option.name) + ": ")

func _mechanic_rows(parent: VBoxContainer) -> void:
	var spec: Dictionary = catalog.get("mechanics", {}).get(str(card.mechanic.get("kind", "")), {})
	var keys: Array = spec.get("parameters", {}).keys(); keys.sort()
	var old_params: Dictionary = compare_card.get("mechanic", {}).get("params", {}) if compare_card.get("mechanic") is Dictionary else {}
	for key in keys:
		var param: Dictionary = spec.parameters[key]
		var value = card.mechanic.params.get(key, param.get("default"))
		var caption := str(key).trim_suffix("_id").replace("_", " ").capitalize()
		match str(param.type):
			"integer": _stepper(parent, caption, int(value), "", int(param.get("minimum", 0)), int(param.get("maximum", 100)), func(v): card.mechanic.params[key] = v, "forge.mechanic." + key, int(old_params.get(key, -1)))
			"boolean":
				var check := CheckBox.new(); check.text = caption; check.button_pressed = bool(value); parent.add_child(check)
				check.toggled.connect(func(on): card.mechanic.params[key] = on; _changed())
			"status":
				var ids: Array = catalog.get("statuses", {}).keys(); ids.sort()
				_enum_row(parent, caption, ids, str(value), func(v): card.mechanic.params[key] = v, "forge.mechanic." + key, str(old_params.get(key, "")))

func _stepper(parent: Node, caption: String, value: int, hint: String, low: int, high: int, apply: Callable, key: String, original: int = -1) -> void:
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); parent.add_child(row)
	var text := VBoxContainer.new(); text.size_flags_horizontal = Control.SIZE_EXPAND_FILL; row.add_child(text)
	STYLE.label(text, caption, 15)
	if not hint.is_empty(): STYLE.label(text, hint, 13, STYLE.MUTED)
	var dec := _button(row, "−", func(): apply.call(value - 1); _changed(), key + ".dec"); dec.custom_minimum_size = Vector2(40, 36); dec.disabled = value <= low
	var shown := STYLE.label(row, str(value), 18, STYLE.GOLD_BRIGHT if original >= 0 and original != value else STYLE.IVORY)
	shown.custom_minimum_size.x = 44; shown.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; shown.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	shown.set_meta("tree_field", key + ".value")
	var inc := _button(row, "+", func(): apply.call(value + 1); _changed(), key + ".inc"); inc.custom_minimum_size = Vector2(40, 36); inc.disabled = value >= high

func _enum_row(parent: Node, caption: String, options: Array, value: String, apply: Callable, key: String, original: String = "") -> void:
	var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); parent.add_child(row)
	var text := STYLE.label(row, caption, 15, STYLE.GOLD_BRIGHT if not original.is_empty() and original != value else STYLE.IVORY)
	text.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	var pick := OptionButton.new(); pick.set_meta("tree_field", key); pick.fit_to_longest_item = false; pick.custom_minimum_size.x = 150; row.add_child(pick)
	for option in options:
		pick.add_item(str(option).replace("_", " ") if not str(option).is_empty() else "none"); pick.set_item_metadata(pick.item_count - 1, option)
		if str(option) == value: pick.select(pick.item_count - 1)
	pick.item_selected.connect(func(i): apply.call(str(pick.get_item_metadata(i))); _changed())

func _button(parent: Node, text: String, action: Callable, key: String) -> Button:
	var b := Button.new(); b.text = text; b.set_meta("tree_action", key); b.pressed.connect(action); parent.add_child(b); return b

func _fits(effect: String) -> bool:
	if card.program.steps.size() >= int(catalog.get("limits", {}).get("steps", 32)): return false
	var windows: Array = catalog.effects[effect].get("windows", [])
	return card.program.get("windows", []).any(func(w): return w in windows)

func _new_step(effect: String) -> Dictionary:
	return new_step(catalog, effect)

static func new_step(catalog: Dictionary, effect: String) -> Dictionary:
	var rules: Dictionary = catalog.get("target_rules", {}).get(effect, {"modes": [], "filters": []})
	var step := {"effect": effect, "params": {}, "target": {"owner": "self", "mode": "one", "count": 1, "selection": "choose"}}
	if not rules.get("modes", []).is_empty(): step.target.mode = rules.modes[0]
	if not rules.get("owners", []).is_empty() and "self" not in rules.owners: step.target.owner = rules.owners[0]
	if "zones" in rules.get("filters", []): step.target.zones = ["hand"]
	if catalog.effects[effect].get("parameters", {}).has("amount"): step.params.amount = 1
	if effect == "remove_status": step.params.stacks = 1
	return step

func _add_step(effect: String) -> void:
	card.program.steps.append(_new_step(effect))
	var windows: Array = catalog.effects[effect].get("windows", [])
	card.program.windows = card.program.windows.filter(func(w): return w in windows)
	_changed()

func _changed() -> void:
	if not session.get("name_touched", false) and mode != "edit": card.name = _suggest_name()
	_validate(); rebuild()

func _validate(refresh_rules: bool = true) -> void:
	_error = ""
	var name := str(card.name).strip_edges()
	if name.is_empty(): _error = "Enter a name."
	elif used_names.has(name.to_lower()): _error = "Another card already uses the name %s." % name
	if not refresh_rules or runtime == null: return
	var response: Dictionary = runtime.card_authoring("validate_card", card)
	if response.get("ok", false): card.presentation = response.result.card.presentation
	elif _error.is_empty(): _error = str(response.get("error", "This combination is not valid."))

func _suggest_name() -> String:
	var root := str(compare_name).split(" · ")[0]
	var entries := DIFF.changes(compare_card, card, catalog)
	var word := "Variant" if entries.is_empty() else _adjective(entries[0])
	var candidate := "%s · %s" % [root, word]
	var n := 2
	while used_names.has(candidate.to_lower()):
		candidate = "%s · %s %d" % [root, word, n]; n += 1
	return candidate

func _adjective(change: Dictionary) -> String:
	var text := str(change.text); var gain: bool = change.tone == "gain"; var loss: bool = change.tone == "loss"
	if text.begins_with("Energy"): return "Swift" if gain else "Heavy"
	if text.begins_with("Saved cards"): return "Steadfast"
	if text.begins_with("+ Draw"): return "Insightful"
	if text.begins_with("+ Gain"): return "Energizing"
	if text.begins_with("+ Prevent"): return "Warded"
	if text.begins_with("+ "): return "Empowered"
	if text.begins_with("− "): return "Stripped"
	if text.begins_with("After play") and text.ends_with("removed"): return "Fleeting"
	if text.begins_with("Uses per"): return "Tireless" if gain else "Limited"
	if text.begins_with("Prevent"): return "Fortified" if gain else "Brittle"
	if text.begins_with("Draw"): return "Keen" if gain else "Dull"
	return "Greater" if gain else "Lesser" if loss else "Altered"
