extends Control
## The custom adventurer's Board Builder: pick a die, a board template, and a
## cost and rider for each slot. The authority prices every change
## (board_price) and the screen only shows its report; it never computes a
## price. Confirm writes the die and the priced board to the character sheet
## through the authority (board_save).
signal closed
const STYLE = preload("res://app/screens/character/character_style.gd")
const STONE_DIE := preload("res://presentation/dice/stone_die.gd")
const BAND_COLORS := {"easy": STYLE.GAIN, "medium": STYLE.AMBER, "hard": STYLE.LOSS}
const ROLE_NAMES := {"main": "Main", "side": "Side", "odd": "Odd", "odd_b": "Second odd", "apex": "Apex"}

## board_builder's result: menus, the chosen die's cost odds, template stats
## and the sheet.
var menus: Dictionary = {}
## The board being edited: die, template, slot picks and defenses.
var choice: Dictionary = {}
## The last board_price result for `choice`.
var priced: Dictionary = {}
var revision := 0
var saved_choice: Dictionary = {}
var _status: Label
var _dice_row: HFlowContainer
var _faces: HBoxContainer
var _die_title: Label
var _costs: VBoxContainer
var _templates: VBoxContainer
var _slots: VBoxContainer
var _defenses: VBoxContainer
var _stats: HFlowContainer
var _abilities: VBoxContainer
var _keep: VBoxContainer
var _keep_scroll: ScrollContainer
var _keep_toggle: Button
var _keep_roll := 1
var _confirm: Button
var die_buttons := {}
var template_buttons := {}
## Slot editors by slot id: {"cost": OptionButton, "rider": OptionButton,
## "next_rider": Button, "next_cost": Button}.
var slot_controls := {}

func _ready() -> void:
	name = "BoardBuilderWorkspace"
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	theme = STYLE.theme()
	_build()
	_load("")

func _runtime():
	return get_node_or_null("/root/LearnedBattleRuntime")

func _build() -> void:
	var bg := ColorRect.new(); bg.color = STYLE.BACKDROP; bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(bg)
	var margin := MarginContainer.new(); margin.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT); add_child(margin)
	for side in ["left", "right", "top", "bottom"]: margin.add_theme_constant_override("margin_" + side, 18)
	var body := VBoxContainer.new(); body.add_theme_constant_override("separation", 10); margin.add_child(body)
	var header := HBoxContainer.new(); header.add_theme_constant_override("separation", 12); body.add_child(header)
	var titles := VBoxContainer.new(); titles.size_flags_horizontal = Control.SIZE_EXPAND_FILL; titles.add_theme_constant_override("separation", 0); header.add_child(titles)
	STYLE.label(titles, "CHARACTER CREATION  /  BOARD BUILDER", 12, STYLE.GOLD)
	STYLE.label(titles, "Custom Adventurer · Board Builder", 28)
	STYLE.label(titles, "Choose a die, a board template and each slot's cost and rider. The authority prices every board so harder abilities pay more and every board averages the same per turn.", 14, STYLE.MUTED)
	_button(header, "Back to characters", func(): closed.emit(); queue_free(), "back").size_flags_vertical = Control.SIZE_SHRINK_END
	var columns := HBoxContainer.new(); columns.add_theme_constant_override("separation", 12); columns.size_flags_vertical = Control.SIZE_EXPAND_FILL; body.add_child(columns)
	# 1 · Die
	var left := _column(columns, 290)
	STYLE.heading(left, "1 · Die")
	_dice_row = HFlowContainer.new(); _dice_row.add_theme_constant_override("h_separation", 6); _dice_row.add_theme_constant_override("v_separation", 6); left.add_child(_dice_row)
	_die_title = STYLE.label(left, "", 16, STYLE.GOLD_BRIGHT)
	_faces = HBoxContainer.new(); _faces.add_theme_constant_override("separation", 4); left.add_child(_faces)
	STYLE.heading(left, "Chase odds of each cost")
	STYLE.label(left, "The chance to roll a cost with 5 dice and 3 rolls when every keep chases it alone.", 12, STYLE.DIM)
	_costs = VBoxContainer.new(); _costs.add_theme_constant_override("separation", 2); left.add_child(_costs)
	# 2 · Template and 3 · Slots
	var middle := _column(columns, 300, true)
	STYLE.heading(middle, "2 · Template")
	_templates = VBoxContainer.new(); _templates.add_theme_constant_override("separation", 4); middle.add_child(_templates)
	STYLE.heading(middle, "3 · Slots")
	_slots = VBoxContainer.new(); _slots.add_theme_constant_override("separation", 8); middle.add_child(_slots)
	STYLE.heading(middle, "Defenses")
	_defenses = VBoxContainer.new(); _defenses.add_theme_constant_override("separation", 6); middle.add_child(_defenses)
	# Preview
	var right := _column(columns, 380)
	STYLE.heading(right, "Preview · priced by the authority")
	_stats = HFlowContainer.new(); _stats.add_theme_constant_override("h_separation", 8); _stats.add_theme_constant_override("v_separation", 8); right.add_child(_stats)
	_keep_toggle = _button(right, "Show keep table", _toggle_keep, "keep_table"); _keep_toggle.toggle_mode = true
	_keep_toggle.tooltip_text = "The best keep after rolls 1 and 2 for every hand, from the authority's exact play."
	_keep_scroll = ScrollContainer.new(); _keep_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; _keep_scroll.custom_minimum_size.y = 260; right.add_child(_keep_scroll); _keep_scroll.hide()
	_keep = VBoxContainer.new(); _keep.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _keep.add_theme_constant_override("separation", 2); _keep_scroll.add_child(_keep)
	_abilities = VBoxContainer.new(); _abilities.add_theme_constant_override("separation", 10); right.add_child(_abilities)
	var footer_panel := PanelContainer.new(); footer_panel.add_theme_stylebox_override("panel", STYLE.box("0c1620", "1f2e3a", 14, 10)); body.add_child(footer_panel)
	var footer := HBoxContainer.new(); footer.add_theme_constant_override("separation", 12); footer_panel.add_child(footer)
	_status = STYLE.label(footer, "", 15, STYLE.MUTED); _status.size_flags_horizontal = Control.SIZE_EXPAND_FILL; _status.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_confirm = STYLE.accent(_button(footer, "Confirm board", _confirm_board, "confirm")); _confirm.custom_minimum_size.x = 170

func _column(parent: Node, width: float, expand: bool = false) -> VBoxContainer:
	var panel := PanelContainer.new(); panel.add_theme_stylebox_override("panel", STYLE.box("101b25", "1f2e3a", 12, 10))
	panel.custom_minimum_size.x = width; panel.size_flags_vertical = Control.SIZE_EXPAND_FILL
	if expand: panel.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	parent.add_child(panel)
	var scroll := ScrollContainer.new(); scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED; panel.add_child(scroll)
	var v := VBoxContainer.new(); v.size_flags_horizontal = Control.SIZE_EXPAND_FILL; v.add_theme_constant_override("separation", 8); scroll.add_child(v)
	return v

func _button(parent: Node, text: String, action: Callable, key: String) -> Button:
	var b := Button.new(); b.text = text; b.custom_minimum_size.y = 38
	b.add_theme_font_size_override("font_size", 15)
	b.pressed.connect(action); parent.add_child(b)
	_register(b, key, text)
	return b

func _register(control: Control, key: String, description: String) -> void:
	control.set_meta("board_control", key)
	var inspector = get_node_or_null("/root/GameInspector")
	if inspector != null: inspector.register_control("board." + key, control, description)

func _clear(node: Node) -> void:
	for child in node.get_children(): node.remove_child(child); child.queue_free()

## Loads the menus for a die (empty: the sheet's die) and, on first load, the
## sheet's board.
func _load(die_id: String) -> void:
	var runtime = _runtime()
	var response: Dictionary = runtime.board_builder({"die": die_id} if not die_id.is_empty() else {}) if runtime != null else {"ok": false, "error": "Learned runtime unavailable."}
	if not response.get("ok", false):
		_error("Could not load the Board Builder: " + str(response.get("error", "unknown error"))); return
	menus = response.result
	var sheet: Dictionary = menus.get("sheet", {})
	if choice.is_empty():
		revision = int(sheet.get("revision", 0))
		choice = _copy(sheet.get("choice", {}))
		if choice.is_empty(): choice = {"die": menus.die, "template": _first_template(_family(str(menus.die)))}
		saved_choice = _copy(choice)
	_render_dice()
	_render_templates()
	_price()

func _copy(value) -> Dictionary:
	return value.duplicate(true) if value is Dictionary else {}

func _die(id: String) -> Dictionary:
	for die in menus.get("dice", []):
		if str(die.id) == id: return die
	return {}

func _family(die_id: String) -> String:
	return str(_die(die_id).get("family", ""))

func _first_template(family: String) -> String:
	for t in menus.get("templates", []):
		if str(t.family) == family: return str(t.id)
	return ""

func _template(id: String) -> Dictionary:
	for t in menus.get("templates", []):
		if str(t.id) == id: return t
	return {}

func _rider_name(id: String) -> String:
	for r in menus.get("riders", []):
		if str(r.id) == id: return str(r.name)
	return id

func _rider_value(id: String) -> float:
	for r in menus.get("riders", []):
		if str(r.id) == id: return float(r.value)
	return 0.0

func select_die(id: String) -> void:
	if id == str(choice.get("die", "")): return
	var same_family := _family(id) == _family(str(choice.get("die", "")))
	if same_family: choice.die = id
	else: choice = {"die": id, "template": _first_template(_family(id))}
	_load(id)

func select_template(id: String) -> void:
	if id == str(choice.get("template", "")): return
	choice = {"die": choice.die, "template": id}
	_render_templates()
	_price()

func _render_dice() -> void:
	_clear(_dice_row); die_buttons.clear()
	for die in menus.get("dice", []):
		var b := _button(_dice_row, "%s  %s" % [str(die.name).replace(" Die", ""), str(die.order)], select_die.bind(str(die.id)), "die." + str(die.id))
		b.toggle_mode = true; b.set_pressed_no_signal(str(die.id) == str(choice.die))
		b.tooltip_text = "%s · %s faces" % [die.name, die.order]
		die_buttons[str(die.id)] = b
	var die := _die(str(choice.die))
	_die_title.text = "%s · %s" % [die.get("name", ""), die.get("order", "")]
	_clear(_faces)
	for face in die.get("faces", []):
		var cell := VBoxContainer.new(); cell.add_theme_constant_override("separation", 2); _faces.add_child(cell)
		var stone := Button.new(); stone.custom_minimum_size = Vector2(40, 40); stone.disabled = true
		stone.text = "%s\n%d" % [str(face.glyph), int(face.number)]
		STONE_DIE.dress(stone, str(die.id), int(face.number), false)
		stone.tooltip_text = "Face %d: %s (%s)" % [int(face.number), face.name, ROLE_NAMES.get(str(face.role), str(face.role))]
		cell.add_child(stone)
		var role := STYLE.label(cell, ROLE_NAMES.get(str(face.role), str(face.role)), 11, STYLE.MUTED); role.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		var symbol := STYLE.label(cell, str(face.name), 11, STYLE.IVORY); symbol.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_clear(_costs)
	for cost in menus.get("costs", []):
		var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); _costs.add_child(row)
		var name := STYLE.label(row, str(cost.label), 13); name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var odds := STYLE.label(row, "%.1f%%" % (float(cost.chase) * 100.0), 13, STYLE.IVORY); odds.autowrap_mode = TextServer.AUTOWRAP_OFF
		odds.custom_minimum_size.x = 48; odds.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		STYLE.chip(row, str(menus.bands.get(str(cost.band), {}).get("name", cost.band)), BAND_COLORS.get(str(cost.band), STYLE.MUTED), 11)

func _render_templates() -> void:
	_clear(_templates); template_buttons.clear()
	var stats := {}
	for s in menus.get("template_stats", []): stats[str(s.template)] = s
	for t in menus.get("templates", []):
		if str(t.family) != _family(str(choice.die)): continue
		var s: Dictionary = stats.get(str(t.id), {})
		var text := "%s · %d abilities" % [t.name, t.slots.size()]
		if not s.is_empty(): text += " · avg %.2f · no hit %.1f%%" % [float(s.average), float(s.no_hit) * 100.0]
		var b := _button(_templates, text, select_template.bind(str(t.id)), "template." + str(t.id))
		b.toggle_mode = true; b.alignment = HORIZONTAL_ALIGNMENT_LEFT; b.set_pressed_no_signal(str(t.id) == str(choice.template))
		var slots: Array[String] = []
		for slot in t.slots: slots.append("%s %s" % [slot.job_name, ("(" + ", ".join(slot.bands) + ")") if slot.flex else slot.cost])
		b.tooltip_text = "Slots: " + "; ".join(slots)
		template_buttons[str(t.id)] = b

## Asks the authority to price the current choice and redraws the preview.
func _price() -> void:
	var runtime = _runtime()
	if runtime == null: return
	var response: Dictionary = runtime.board_price(choice, _keep_toggle.button_pressed)
	if not response.get("ok", false):
		_error("This board cannot be priced: " + str(response.get("error", "unknown error")))
		_confirm.disabled = true
		return
	priced = response.result
	choice = _copy(priced.choice)
	_confirm.disabled = false
	_render_slots()
	_render_preview()
	if _keep_toggle.button_pressed: _render_keep()
	_status.add_theme_color_override("font_color", STYLE.MUTED)
	_status.text = "Priced by the authority." + (" Unsaved changes." if not _same(choice, saved_choice) else " This is the board on your sheet.")

func _same(a: Dictionary, b: Dictionary) -> bool:
	return JSON.stringify(a) == JSON.stringify(b)

func _error(message: String) -> void:
	_status.add_theme_color_override("font_color", STYLE.LOSS)
	_status.text = message

func _tiers(slot_id: String) -> Array:
	var out: Array = []
	for tier in priced.get("report", {}).get("tiers", []):
		if str(tier.slot) == slot_id: out.append(tier)
	return out

func _render_slots() -> void:
	_clear(_slots); slot_controls.clear()
	for i in priced.get("slots", []).size():
		var slot: Dictionary = priced.slots[i]
		var id := str(slot.slot)
		var box := STYLE.section(_slots, 10)
		var head := HBoxContainer.new(); head.add_theme_constant_override("separation", 6); box.add_child(head)
		STYLE.chip(head, str(slot.job_name), STYLE.GOLD, 12)
		var title := STYLE.label(head, str(slot.label), 15, STYLE.IVORY); title.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		for band in slot.get("bands", []): STYLE.chip(head, str(menus.bands.get(str(band), {}).get("name", band)), BAND_COLORS.get(str(band), STYLE.MUTED), 11)
		var controls := {}
		var cost_row := HBoxContainer.new(); cost_row.add_theme_constant_override("separation", 6); box.add_child(cost_row)
		STYLE.label(cost_row, "Cost", 13, STYLE.MUTED).custom_minimum_size.x = 48
		if slot.get("flex", false):
			var options := OptionButton.new(); options.size_flags_horizontal = Control.SIZE_EXPAND_FILL; options.fit_to_longest_item = false; cost_row.add_child(options)
			for option in slot.get("options", []):
				options.add_item("%s · %.0f%% %s" % [option.label, float(option.chase) * 100.0, option.band])
				options.set_item_metadata(options.item_count - 1, str(option.cost))
				options.set_item_disabled(options.item_count - 1, bool(option.taken) and str(option.cost) != str(slot.cost))
				if str(option.cost) == str(slot.cost): options.select(options.item_count - 1)
			options.item_selected.connect(func(index): _set_pick(i, "cost", str(options.get_item_metadata(index))))
			_register(options, "slot.%s.cost" % id, "Cost of " + id)
			controls.cost = options
			controls.next_cost = _button(cost_row, "▶", func(): _cycle(options, i, "cost"), "slot.%s.next_cost" % id)
		else:
			STYLE.label(cost_row, "Core: %s" % slot.label, 13, STYLE.IVORY).size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var rider_row := HBoxContainer.new(); rider_row.add_theme_constant_override("separation", 6); box.add_child(rider_row)
		STYLE.label(rider_row, "Rider", 13, STYLE.MUTED).custom_minimum_size.x = 48
		var riders := OptionButton.new(); riders.size_flags_horizontal = Control.SIZE_EXPAND_FILL; riders.fit_to_longest_item = false; rider_row.add_child(riders)
		for rider_id in slot.get("riders", []):
			var value := _rider_value(str(rider_id))
			riders.add_item(_rider_name(str(rider_id)) + (" · worth %.1f damage" % value if value > 0 else ""))
			riders.set_item_metadata(riders.item_count - 1, str(rider_id))
			if str(rider_id) == str(slot.rider): riders.select(riders.item_count - 1)
		riders.disabled = slot.get("riders", []).size() <= 1
		riders.tooltip_text = "This slot takes no rider." if riders.disabled else "A rider's worth comes off the slot's damage."
		riders.item_selected.connect(func(index): _set_pick(i, "rider", str(riders.get_item_metadata(index))))
		_register(riders, "slot.%s.rider" % id, "Rider of " + id)
		controls.rider = riders
		controls.next_rider = _button(rider_row, "▶", func(): _cycle(riders, i, "rider"), "slot.%s.next_rider" % id)
		controls.next_rider.disabled = riders.disabled
		var damage: Array[String] = []
		for tier in _tiers(id): damage.append("%s → %d" % [_tier_label(slot, str(tier.tier)), int(tier.whole_damage)])
		STYLE.label(box, "Damage: " + " · ".join(damage), 13, STYLE.OFFENSE.lightened(0.2))
		slot_controls[id] = controls
	_render_defenses()

## Selects the next enabled item of an option list (pointer-friendly).
func _cycle(options: OptionButton, index: int, field: String) -> void:
	if options.item_count < 2: return
	var next := options.selected
	for step in options.item_count:
		next = (next + 1) % options.item_count
		if not options.is_item_disabled(next): break
	options.select(next)
	_set_pick(index, field, str(options.get_item_metadata(next)))

func _set_pick(index: int, field: String, value: String) -> void:
	var slots: Array = choice.get("slots", [])
	if index >= slots.size(): return
	slots[index][field] = value
	choice.slots = slots
	_price()

func _render_defenses() -> void:
	_clear(_defenses)
	var chosen: Array = choice.get("defenses", [])
	for i in chosen.size():
		var options := OptionButton.new(); options.fit_to_longest_item = false; _defenses.add_child(options)
		for d in menus.get("defenses", []):
			options.add_item(str(d.name)); options.set_item_metadata(options.item_count - 1, str(d.ability))
			options.set_item_tooltip(options.item_count - 1, str(d.rules_text))
			if str(d.ability) == str(chosen[i]): options.select(options.item_count - 1)
		options.item_selected.connect(func(index):
			choice.defenses[i] = str(options.get_item_metadata(index)); _price())
		_register(options, "defense.%d" % i, "Defense %d" % (i + 1))
		for d in menus.get("defenses", []):
			if str(d.ability) == str(chosen[i]): STYLE.label(_defenses, str(d.rules_text), 12, STYLE.MUTED)

func _render_preview() -> void:
	_clear(_stats)
	var report: Dictionary = priced.get("report", {})
	var target := float(menus.get("pricing", {}).get("target", 0))
	STYLE.stat(_stats, "%.2f" % float(report.get("whole_average", 0)), "avg damage / turn · target %.1f" % target, STYLE.GOLD_BRIGHT)
	STYLE.stat(_stats, "%.1f%%" % (float(report.get("whole_no_hit", 0)) * 100.0), "no hit", STYLE.LOSS)
	STYLE.stat(_stats, "×%.2f" % float(report.get("multiplier", 0)), "a sure hit is worth", STYLE.IVORY)
	_clear(_abilities)
	var abilities: Array = priced.get("abilities", [])
	for i in abilities.size():
		var ability: Dictionary = abilities[i]
		var slot: Dictionary = priced.slots[i] if i < priced.get("slots", []).size() else {}
		var box := STYLE.section(_abilities, 10)
		box.set_meta("ability_id", str(ability.id))
		var head := HBoxContainer.new(); box.add_child(head)
		var name := STYLE.label(head, str(ability.name), 17, STYLE.GOLD_BRIGHT); name.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		if str(slot.get("rider", "none")) != "none": STYLE.chip(head, _rider_name(str(slot.rider)), STYLE.DEFENSE, 11)
		for tier in _tiers(str(slot.get("slot", ""))):
			var row := HBoxContainer.new(); row.add_theme_constant_override("separation", 6); box.add_child(row)
			var what := STYLE.label(row, "%s: %d dmg" % [_tier_label(slot, str(tier.tier)), int(tier.whole_damage)], 13)
			what.autowrap_mode = TextServer.AUTOWRAP_OFF; what.custom_minimum_size.x = 140
			var bar := ProgressBar.new(); bar.max_value = 0.3; bar.value = minf(0.3, float(tier.use_whole)); bar.show_percentage = false
			bar.custom_minimum_size = Vector2(90, 12); bar.size_flags_horizontal = Control.SIZE_EXPAND_FILL; bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
			bar.tooltip_text = "Used on %.1f%% of turns (exact price: %.1f%%); chase %.1f%%" % [float(tier.use_whole) * 100.0, float(tier.use_exact) * 100.0, float(tier.chase) * 100.0]
			row.add_child(bar)
			var share := STYLE.label(row, "%.1f%%" % (float(tier.use_whole) * 100.0), 12, STYLE.MUTED)
			share.autowrap_mode = TextServer.AUTOWRAP_OFF; share.custom_minimum_size.x = 42; share.horizontal_alignment = HORIZONTAL_ALIGNMENT_RIGHT
		var rules := STYLE.label(box, str(ability.get("presentation", {}).get("rules_text", "")), 12, STYLE.MUTED)
		rules.size_flags_horizontal = Control.SIZE_EXPAND_FILL

## A tier in the die's symbol names, as the authority labels it.
func _tier_label(slot: Dictionary, tier: String) -> String:
	for entry in slot.get("tiers", []):
		if str(entry.tier) == tier: return str(entry.label)
	return tier

func _toggle_keep() -> void:
	_keep_scroll.visible = _keep_toggle.button_pressed
	_keep_toggle.text = "Hide keep table" if _keep_toggle.button_pressed else "Show keep table"
	if _keep_toggle.button_pressed: _price()

func _render_keep() -> void:
	_clear(_keep)
	var policy: Dictionary = priced.get("report", {}).get("policy", {})
	var tabs := HBoxContainer.new(); tabs.add_theme_constant_override("separation", 6); _keep.add_child(tabs)
	for roll in [1, 2]:
		var b := _button(tabs, "After roll %d" % roll, func(): _keep_roll = roll; _render_keep(), "keep_roll.%d" % roll)
		b.toggle_mode = true; b.set_pressed_no_signal(_keep_roll == roll)
	STYLE.label(_keep, "Best keep for each hand (faces sorted); the value is what the turn is then worth on average.", 12, STYLE.DIM)
	var die := _die(str(choice.die))
	var glyphs := {}
	for face in die.get("faces", []): glyphs[int(face.number)] = str(face.glyph)
	var rows: Array = policy.get("after_roll_%d" % _keep_roll, [])
	for row in rows:
		var hand: Array[String] = []
		for f in row.hand: hand.append("%s%d" % [glyphs.get(int(f), ""), int(f)])
		var kept: Array[String] = []
		for f in row.keep: kept.append(str(int(f)))
		var text := "%s  →  %s  ·  %.2f" % [" ".join(hand), "keep " + " ".join(kept) if not kept.is_empty() else "reroll all", float(row.expected)]
		if kept.size() == row.hand.size(): text = "%s  →  stop  ·  %.2f" % [" ".join(hand), float(row.expected)]
		STYLE.label(_keep, text, 12, STYLE.IVORY)

func _confirm_board() -> void:
	var runtime = _runtime()
	if runtime == null: return
	var response: Dictionary = runtime.board_save(choice, revision)
	if not response.get("ok", false):
		_error("The board was not saved: " + str(response.get("error", "unknown error"))); return
	revision = int(response.result.revision)
	saved_choice = _copy(response.result.board_choice)
	choice = _copy(saved_choice)
	_status.add_theme_color_override("font_color", STYLE.GAIN)
	_status.text = "Saved to the Custom Adventurer's sheet: the %s and %d abilities." % [_die(str(choice.die)).get("name", "die"), response.result.ability_board.offensive.size()]
