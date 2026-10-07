extends RefCounted
## Recursive form for an ordered effect program. It never writes authority state.
const STYLE = preload("res://app/screens/character/character_style.gd")

static func render(ui: Control, parent: VBoxContainer, sequence: Array, path: String, depth: int) -> void:
	for index in sequence.size():
		var step: Dictionary = sequence[index]
		var key := path + "." + str(index)
		var spec: Dictionary = ui.catalog.effects.get(step.effect, {})
		var panel := PanelContainer.new(); parent.add_child(panel)
		panel.add_theme_stylebox_override("panel", STYLE.box("101c27" if depth == 0 else "0b141c", Color(STYLE.GOLD, 0.4) if depth == 0 else Color("2b4150"), 14, 10))
		var box := VBoxContainer.new(); box.add_theme_constant_override("separation", 8); panel.add_child(box)
		var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 6); box.add_child(head)
		var number := STYLE.chip(head, str(index + 1), STYLE.GOLD, 14); number.custom_minimum_size.x = 30; number.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		var title: Label = ui._label(head, str(spec.get("label", step.effect)), 18); title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		title.add_theme_color_override("font_color", STYLE.GOLD_BRIGHT); title.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
		var up = ui._button(head, "↑", func(): var before = sequence[index - 1]; sequence[index - 1] = step; sequence[index] = before; ui._changed(true), key + ".up")
		up.disabled = index == 0 or step.effect == "sacrifice" or sequence[index - 1].effect == "sacrifice"; up.tooltip_text = "Move up"
		var down = ui._button(head, "↓", func(): var after = sequence[index + 1]; sequence[index + 1] = step; sequence[index] = after; ui._changed(true), key + ".down")
		down.disabled = index >= sequence.size() - 1 or step.effect == "sacrifice"; down.tooltip_text = "Move down"
		var remove = STYLE.accent(ui._button(head, "Remove", func(): sequence.remove_at(index); ui._reconcile_windows(); ui._changed(true), key + ".remove"), STYLE.LOSS)
		for b in [up, down]: b.custom_minimum_size = Vector2(36, 32)
		remove.custom_minimum_size.y = 32
		var rules: Dictionary = ui.catalog.target_rules[step.effect]
		if not str(rules.notes).is_empty(): ui._label(box, str(rules.notes), 13).add_theme_color_override("font_color", STYLE.MUTED)
		if spec.get("target") != "none":
			STYLE.heading(box, "Target")
			var targets := STYLE.form(box)
			for field in ["owner", "mode", "selection"]:
				var options: Array = rules["owners" if field == "owner" else "modes" if field == "mode" else "selections"]
				var select = ui._select(targets, "Target " + field, options, str(step.target[field]), func(v):
					step.target[field] = v
					if step.target.owner == "self" and step.target.mode == "exact" and int(rules.get("self_count_max", 0)) > 0: step.target.count = mini(int(step.target.count), int(rules.self_count_max))
					ui._reconcile_windows(); ui._changed(true), key + "." + field)
				select.disabled = options.size() == 1
				# An enemy die cannot be changed before it is revealed. Do not offer
				# that owner when other effects force this card into planning only.
				if field == "owner" and spec.target == "offensive_die":
					for n in select.item_count:
						if select.get_item_metadata(n) == "enemy" and "offensive_reaction" not in ui._compatible_windows(ui.draft.program.steps): select.set_item_disabled(n, true)
			if step.target.mode in ["exact", "up_to"]: ui._number(targets, "Number of targets", int(step.target.get("count", 1)), 1, int(rules.self_count_max) if step.target.owner == "self" and step.target.mode == "exact" and int(rules.get("self_count_max", 0)) > 0 else int(ui.catalog.limits.count), func(v): step.target.count = v; ui._changed(), key + ".count")
			_filters(ui, box, step, rules.filters, key)
		var values: GridContainer = null
		for field in spec.get("parameters", {}):
			if step.effect == "ability_bonus" and field == "rounds" and step.params.get("duration", "offensive") != "rounds": continue
			if step.effect == "ability_bonus" and field == "stacks" and str(step.params.get("status_id", "")).is_empty(): continue
			if values == null:
				STYLE.heading(box, "Values")
				values = STYLE.form(box)
			var param: Dictionary = spec.parameters[field]
			var value = step.params.get(field, param.get("default"))
			var caption := str(field).replace("_", " ").capitalize()
			if field == "stacks" and step.effect == "remove_status": caption += " · 0 removes all"
			match str(param.type):
				"integer":
					var low := int(param.get("minimum", 0)); var high := int(param.get("maximum", 100))
					if step.effect == "adjust_die" and field == "minimum": high = int(step.params.get("maximum", spec.parameters.maximum.default))
					if step.effect == "adjust_die" and field == "maximum": low = int(step.params.get("minimum", spec.parameters.minimum.default))
					if step.effect == "ability_bonus" and field == "damage" and str(step.params.get("status_id", "")).is_empty(): low = 1
					ui._number(values, caption, int(value), low, high, func(v): step.params[field] = v; ui._changed(field in ["minimum", "maximum"]), key + ".param." + field)
				"enum": ui._select(values, caption, param.options, str(value), func(v): step.params[field] = v; ui._changed(true), key + ".param." + field)
				"boolean":
					ui._label(values, "")
					ui._check(values, caption, bool(value), func(v): step.params[field] = v; ui._changed(), key + ".param." + field)
				"integers": _numbers(ui, values, caption + " · comma-separated", value, func(v): step.params[field] = v; ui._changed(), key + ".param." + field)
				"status", "status_optional":
					var ids: Array = ui.catalog.statuses.keys(); ids.sort()
					if param.type == "status_optional": ids.push_front("")
					ui._select(values, "Applied status", ids, str(value), func(v):
						step.params[field] = v
						if step.effect == "ability_bonus" and v.is_empty() and int(step.params.get("damage", 2)) == 0: step.params.damage = 1
						ui._changed(true), key + ".param." + field)
		if step.effect != "sacrifice": _conditions(ui, box, step, key)
		if step.effect == "choice":
			for option_index in step.choices.size():
				var option: Dictionary = step.choices[option_index]
				var option_key := key + ".option." + str(option_index)
				var option_box := VBoxContainer.new(); option_box.add_theme_constant_override("separation", 8)
				var option_panel := PanelContainer.new(); option_panel.add_theme_stylebox_override("panel", STYLE.box("0d1822", Color(STYLE.DEFENSE, 0.45), 12, 8)); box.add_child(option_panel); option_panel.add_child(option_box)
				STYLE.heading(option_box, "Option %d" % [option_index + 1])
				var option_form := STYLE.form(option_box)
				ui._line(option_form, "Option name", str(option.name), func(v): option.name = v; ui._changed(), option_key + ".name")
				ui._number(option_form, "Additional energy", int(option.energy), 0, 100, func(v): option.energy = v; ui._changed(), option_key + ".energy")
				var remove_option = ui._button(option_box, "Remove option", func(): step.choices.remove_at(option_index); ui._reconcile_windows(); ui._changed(true), option_key + ".remove")
				remove_option.disabled = step.choices.size() == 1
				render(ui, option_box, option.steps, option_key + ".steps", depth + 1)
			var add_option = ui._button(box, "+ Add option", func(): step.choices.append({"name": "Option " + str(step.choices.size() + 1), "energy": 0, "steps": [ui._new_step("energy")]}); ui._changed(true), key + ".option.add")
			add_option.disabled = step.choices.size() >= int(ui.catalog.limits.options)
	var toolbar := HBoxContainer.new(); toolbar.add_theme_constant_override("separation", 8); parent.add_child(toolbar)
	var add := OptionButton.new(); add.size_flags_horizontal = Control.SIZE_EXPAND_FILL; add.fit_to_longest_item = false; toolbar.add_child(add); add.set_meta("editor_key", path + ".effect")
	var effects: Array = ui.catalog.effects.keys(); effects.sort(); var first := -1
	for effect in effects:
		add.add_item(str(ui.catalog.effects[effect].label)); add.set_item_metadata(add.item_count - 1, effect)
		var available: bool = ui._effect_available(sequence, effect, depth)
		add.set_item_disabled(add.item_count - 1, not available)
		if available and first == -1: first = add.item_count - 1
	if first >= 0: add.select(first)
	var add_button = STYLE.accent(ui._button(toolbar, "+ Add effect", func():
		sequence.append(ui._new_step(str(add.get_selected_metadata()))); ui._reconcile_windows(); ui._changed(true), path + ".add"))
	add_button.disabled = first == -1
	ui._label(parent, "Unavailable effects conflict with timing, sequence order or nesting limits. Remove the conflicting effect to change direction.", 12).add_theme_color_override("font_color", STYLE.DIM)

static func _filters(ui: Control, box: VBoxContainer, step: Dictionary, filters: Array, key: String) -> void:
	var target: Dictionary = step.target
	if "zones" in filters:
		if not target.has("zones"): target.zones = ["hand"]
		ui._multi(box, "Select cards from", ui.catalog.source_zones, target.zones, func(): ui._changed(), key + ".zones", true)
		ui._check(box, "Exclude cards that recover other cards", bool(target.get("exclude_recovery", false)), func(v): target.exclude_recovery = v; ui._changed(), key + ".exclude_recovery")
		var drawn = ui._check(box, "Only cards drawn earlier in this play", bool(target.get("drawn_this_play", false)), func(v): target.drawn_this_play = v; ui._changed(), key + ".drawn_this_play")
		drawn.disabled = not ui._draw_before(ui.draft.program.steps, key) and not bool(target.get("drawn_this_play", false))
		drawn.tooltip_text = "Add a draw for yourself before this effect to enable this filter." if drawn.disabled else ""
		_id_filter(ui, box, target, "exclude_cards", ui.catalog.get("card_names", ui.catalog.templates).keys(), "Exclude specific cards", key)
	if "faces" in filters: _numbers(ui, box, "Only dice showing these faces · empty = any", target.get("faces", []), func(v): target.faces = v; ui._changed(), key + ".faces", true)
	if "polarity" in filters:
		ui._select(box, "Status polarity", ["any", "positive", "negative"], str(target.get("polarity", "any")), func(v):
			target.polarity = v
			# Drop chosen statuses the new polarity could never target.
			if target.has("status_ids"): target.status_ids = target.status_ids.filter(func(id): return _status_matches(ui, str(id), v))
			ui._changed(true), key + ".polarity")
		var polarity := str(target.get("polarity", "any"))
		var statuses: Array = ui.catalog.statuses.keys().filter(func(id): return _status_matches(ui, str(id), polarity))
		var names := {}
		for id in ui.catalog.statuses: names[id] = str(ui.catalog.statuses[id].get("name", id))
		for id in names:
			if names.values().count(names[id]) > 1: names[id] = "%s (%s)" % [names[id], id]
		_id_filter(ui, box, target, "status_ids", statuses, "Limit to specific statuses · empty = any", key, names)
	if "qualified" in filters: ui._check(box, "Only abilities that currently qualify", bool(target.get("qualified", false)), func(v): target.qualified = v; ui._reconcile_windows(); ui._changed(true), key + ".qualified")
static func _numbers(ui: Control, box: Control, title: String, value: Variant, changed: Callable, key: String, allow_empty: bool = false) -> void:
	var text := str(value) if not value is Array else ", ".join(value.map(func(v): return str(v)))
	ui._line(box, title, text, func(v):
		var numbers: Array = []
		if v.strip_edges().is_empty() and allow_empty: changed.call(numbers); return
		for part in v.split(","):
			if not part.strip_edges().is_valid_int(): changed.call(v); return
			numbers.append(int(part.strip_edges()))
		changed.call(numbers), key)
static func _status_matches(ui: Control, id: String, polarity: String) -> bool:
	return polarity not in ["positive", "negative"] or str(ui.catalog.statuses.get(id, {}).get("polarity", "")) == polarity
## labels maps IDs to display names; unlabeled IDs are shown humanized.
static func _id_filter(ui: Control, box: VBoxContainer, target: Dictionary, field: String, values: Array, title: String, key: String, labels: Dictionary = {}) -> void:
	var selected: Array = target.get(field, [])
	ui._label(box, title, 14)
	for id in selected:
		ui._button(box, "Remove filter: " + str(labels.get(id, id)), func(): selected.erase(id); target[field] = selected; ui._changed(true), key + "." + field + ".remove." + str(id))
	var options: Array = values.filter(func(id): return id not in selected)
	options.sort_custom(func(a, b): return str(labels.get(a, a)).naturalnocasecmp_to(str(labels.get(b, b))) < 0)
	if options.is_empty(): return
	var pick = ui._select(box, "Add filter", options, str(options[0]), func(_v): pass, key + "." + field + ".pick")
	for i in pick.item_count:
		if labels.has(pick.get_item_metadata(i)): pick.set_item_text(i, str(labels[pick.get_item_metadata(i)]))
	ui._button(box, "Add selected filter", func(): selected.append(pick.get_selected_metadata()); target[field] = selected; ui._changed(true), key + "." + field + ".add")
static func _conditions(ui: Control, box: VBoxContainer, step: Dictionary, key: String) -> void:
	var condition: Dictionary = step.get("condition", {})
	var requirements: Array = condition.get("all", [])
	STYLE.heading(box, "Conditions · all must match" if not requirements.is_empty() else "Conditions · always applies")
	if step.effect == "ability_bonus": ui._label(box, "Checked when the chosen attack resolves. Other effect conditions check your offensive dice when the effect is reached.", 13)
	for i in requirements.size():
		var req: Dictionary = requirements[i]; var req_key := key + ".condition." + str(i)
		ui._select(box, "Requirement", ui.catalog.condition_types, str(req.type), func(v):
			var replacement := {"type": v}
			match v:
				"symbol_count": replacement.symbol_id = ui.catalog.symbols.keys()[0]; replacement.minimum = 1
				"number_pattern": replacement.pattern = "small_straight"
				"exact_faces": replacement.faces = [1, 2, 3, 4, 5]
			requirements[i] = replacement; ui._changed(true), req_key + ".type")
		match str(req.type):
			"symbol_count":
				ui._select(box, "Symbol", ui.catalog.symbols.keys(), str(req.symbol_id), func(v): req.symbol_id = v; ui._changed(), req_key + ".symbol")
				ui._check(box, "Exact count", req.has("exact"), func(on):
					if on: req.erase("minimum"); req.erase("maximum"); req.exact = 1
					else: req.erase("exact"); req.minimum = 1
					ui._changed(true), req_key + ".exact_mode")
				if req.has("exact"): ui._number(box, "Exactly", int(req.exact), 0, 100, func(v): req.exact = v; ui._changed(), req_key + ".exact")
				else:
					ui._number(box, "Minimum", int(req.get("minimum", 0)), 0 if int(req.get("maximum", 0)) > 0 else 1, int(req.maximum) if int(req.get("maximum", 0)) > 0 else 100, func(v): req.minimum = v; ui._changed(true), req_key + ".minimum")
					ui._check(box, "Limit the maximum", int(req.get("maximum", 0)) > 0, func(on):
						if on: req.maximum = maxi(1, int(req.get("minimum", 1)))
						else: req.erase("maximum"); req.minimum = maxi(1, int(req.get("minimum", 1)))
						ui._changed(true), req_key + ".maximum_enabled")
					if int(req.get("maximum", 0)) > 0: ui._number(box, "Maximum", int(req.maximum), maxi(1, int(req.get("minimum", 0))), 100, func(v): req.maximum = v; ui._changed(true), req_key + ".maximum")
			"number_pattern": ui._select(box, "Pattern", ui.catalog.patterns, str(req.pattern), func(v): req.pattern = v; ui._changed(), req_key + ".pattern")
			"exact_faces": _numbers(ui, box, "Required faces · comma-separated", req.faces, func(v): req.faces = v; ui._changed(), req_key + ".faces")
		ui._button(box, "Remove condition", func(): requirements.remove_at(i); step.condition = {"all": requirements}; ui._changed(true), req_key + ".remove")
	var add = ui._button(box, "Add condition", func():
		requirements.append({"type": "number_pattern", "pattern": "small_straight"}); step.condition = {"all": requirements}; ui._changed(true), key + ".condition.add")
	add.disabled = requirements.size() >= int(ui.catalog.limits.conditions)
