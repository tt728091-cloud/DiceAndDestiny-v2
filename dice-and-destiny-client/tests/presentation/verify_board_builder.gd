extends SceneTree
## Board Builder (Character Creation, Custom Adventurer): pick the 2-2-1-1 die
## and a template by pointer, change a rider and see the authority re-price,
## open the keep table, step the damage-level slider up the staircase and back
## to the standard level, confirm through the authority, and check that a
## battle pins the die and the generated abilities with their full rules on
## the shared attack hover.
const CREATION := preload("res://app/screens/character/character_creation.gd")
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var runtime = root.get_node("LearnedBattleRuntime")
	for width in [1280, 1920]:
		root.size = Vector2i(width, width * 9 / 16)
		await _builder_round(runtime, width)
	await _hover_round(runtime)
	print("BOARD BUILDER: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _builder_round(runtime, width: int) -> void:
	var creation = CREATION.new(); root.add_child(creation)
	for frame in 8: await process_frame
	await _click(creation._roster_buttons["adventurer"])
	_expect(creation._tabs.is_tab_disabled(creation.BOARD_BUILDER_TAB), "Board Builder is only for the Custom Adventurer")
	await _click(creation._roster_buttons["custom_adventurer"])
	_expect(creation.character_id == "custom_adventurer" and not creation._tabs.is_tab_disabled(creation.BOARD_BUILDER_TAB), "Custom Adventurer opens the Board Builder tab")
	await _click_tab(creation._tabs, creation.BOARD_BUILDER_TAB)
	var builder = creation.find_child("BoardBuilderWorkspace", false, false)
	_expect(builder != null, "the tab opens the Board Builder")
	if builder == null: creation.queue_free(); await process_frame; return
	for frame in 4: await process_frame
	_expect(builder.die_buttons.size() == 5, "five dice on the menu")
	_expect(builder._faces.get_child_count() == 6, "face strip shows six faces with roles")
	_expect(builder._costs.get_child_count() > 0, "each menu cost shows its chase odds")
	# 1 · the 2-2-1-1 die.
	await _click(builder.die_buttons["forge_d6"])
	_expect(builder.choice.die == "forge_d6", "pointer picks the Forge die")
	_expect(builder._family(str(builder.choice.die)) == "2211" and builder._template(str(builder.choice.template)).family == "2211", "die and template are 2-2-1-1")
	_expect(builder.template_buttons.size() == 4, "the 2-2-1-1 templates are offered: %d" % builder.template_buttons.size())
	var roles := []
	for face in builder._die("forge_d6").faces: roles.append(str(face.role))
	_expect(roles == ["main", "main", "side", "side", "odd", "apex"], "Forge die roles: %s" % str(roles))
	for button in builder.template_buttons.values():
		_expect("avg" in button.text and "no hit" in button.text, "each template shows its average and no-hit rate")
	# 2 · a template.
	await _click(builder.template_buttons["paired_standard"])
	_expect(builder.choice.template == "paired_standard", "pointer picks the template")
	var before := _slot_damage(builder, "flex_a")
	_expect(not before.is_empty(), "the preview prices the flex slot")
	var average_before := float(builder.priced.report.whole_average)
	_expect(absf(average_before - 6.2) <= 0.13, "the board averages the Starter's 6.2: %.2f" % average_before)
	# 3 · change a rider and watch the preview re-price.
	await _click(builder.slot_controls["flex_a"].next_rider)
	var rider := ""
	for pick in builder.choice.slots:
		if pick.slot == "flex_a": rider = str(pick.rider)
	_expect(rider != "none", "pointer gives the flex slot a rider")
	var tiers: Array = builder._tiers("flex_a")
	_expect(not tiers.is_empty() and float(tiers[0].rider_value) > 0, "the authority prices the rider")
	var after := _slot_damage(builder, "flex_a")
	_expect(after[0] < before[0], "the rider's worth comes off the damage: %s → %s" % [str(before), str(after)])
	_expect(builder.priced.abilities.any(func(a): return a.id == "board_odd2" and "Rivet" in a.presentation.rules_text), "generated rules name the die's symbols")
	# The keep table on demand.
	await _click(builder._keep_toggle)
	_expect(builder._keep_scroll.visible and builder._keep.get_child_count() > 252, "keep table lists every hand: %d rows" % builder._keep.get_child_count())
	await _click(builder._keep_toggle)
	# The damage-level slider: step up the staircase by pointer; no ability
	# loses damage, and the standard level comes back.
	var steps: Array = builder.staircase.get("steps", [])
	_expect(steps.size() > 100 and absf(builder.preview_target - 6.2) < 1e-6, "the staircase has %d steps and shows the standard 6.2" % steps.size())
	var step0 := int(builder.staircase.step)
	var damage0 := _damage(builder)
	var average0 := float(builder.priced.report.whole_average)
	await _click(builder._level_next)
	_expect(int(builder.staircase.step) == step0 + 1, "▶ moves one step up: %d → %d" % [step0, int(builder.staircase.step)])
	var damage1 := _damage(builder)
	var raised: Array = builder.staircase.steps[int(builder.staircase.step)].raised
	_expect(not raised.is_empty(), "the step raises an ability")
	for tier in damage0:
		_expect(int(damage1[tier]) >= int(damage0[tier]), "%s keeps its damage: %d → %d" % [tier, damage0[tier], damage1[tier]])
		if tier in raised: _expect(int(damage1[tier]) > int(damage0[tier]), "%s deals more on the step" % tier)
	_expect(float(builder.priced.report.whole_average) >= average0, "the average never falls")
	_expect("→" in _text(builder._level_raised), "the step names what it raises")
	await _click_at(builder._level_slider, 0.9)
	_expect(builder.preview_target > 15.0 and float(builder.priced.report.whole_average) > 14.0, "pointer high on the slider prices the board higher: target %.2f, average %.2f" % [builder.preview_target, float(builder.priced.report.whole_average)])
	await _click(builder._level_reset)
	_expect(absf(builder.preview_target - 6.2) < 1e-6 and int(builder.staircase.step) == step0, "the standard level returns: target %.2f, step %d" % [builder.preview_target, int(builder.staircase.step)])
	_expect(builder.size.x <= root.get_visible_rect().size.x + 1, "builder fits the viewport")
	await _capture("board-builder-%d" % width)
	# 4 · confirm through the authority.
	await _click(builder._confirm)
	_expect("Saved" in builder._status.text, "confirm saves: " + builder._status.text)
	var sheet: Dictionary = runtime.board_builder({}).result.sheet
	_expect(sheet.dice == "forge_d6" and sheet.choice.template == "paired_standard", "the sheet holds the die and the board")
	var back: Button
	for button in builder.find_children("*", "Button", true, false):
		if button.get_meta("board_control", "") == "back": back = button
	await _click(back)
	for frame in 4: await process_frame
	var character: Dictionary = creation.catalogs.custom_adventurer.combatants.custom_adventurer
	_expect(str(character.dice_loadout[0].dice_id) == "forge_d6", "Character Creation shows the chosen die")
	_expect("board_odd2" in character.ability_board.offensive, "Character Creation shows the generated board")
	creation._tabs.current_tab = 0; creation._tabs.tab_changed.emit(0)
	for frame in 3: await process_frame
	creation.inspect_entry("abilities", "board_odd2")
	var rules: String = creation.catalogs.custom_adventurer.abilities.board_odd2.presentation.rules_text
	_expect(rules in _text(creation._details), "full generated rules are inspectable")
	creation.queue_free(); await process_frame

## A battle as the Custom Adventurer pins the saved die and board; the shared
## attack hover shows a generated ability's full rules with the die's faces.
func _hover_round(runtime) -> void:
	var gateway = GATEWAY.new(runtime, "seat-a", "brine-mask", "custom_adventurer")
	var base: Dictionary = gateway.start_battle("board-builder-hover", 52)
	_expect(base.get("accepted") == true, "custom adventurer battle starts: %s" % str(base.get("error", "")))
	if base.get("accepted") != true: return
	var hero: Dictionary = base.snapshot.actors.blade
	_expect(hero.definition_id == "custom_adventurer", "the battle fields the Custom Adventurer")
	_expect(str(hero.dice_loadout[0].dice_id) == "forge_d6", "the battle pins the chosen die")
	_expect("board_main3_main4_main5" in hero.offensive_abilities and "holdfast" in hero.defensive_abilities, "the battle pins the generated board")
	var canvas := SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.size = Vector2i(1920, 1080); root.add_child(canvas)
	var fixture := base.duplicate(true)
	fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
	fixture.snapshot.stage = "defense_selection"; fixture.snapshot.segment = "defensive"
	fixture.snapshot.actors.blade.selected_ability = "board_main3_main4_main5"
	fixture.snapshot.actors.blade.selected_tier = "main3"
	fixture.snapshot.actors.blade.dice = {"pool": "offensive", "dice": _dice([1, 2, 1, 4, 6], "forge_d6")}
	fixture.snapshot.damage_sources = [{"id": "blade-attack", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "board_main3_main4_main5", "base_amount": 5}]
	var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("board-builder-hover.json"))
	canvas.add_child(screen); screen.set_process(false)
	for frame in 6: await process_frame
	var panel = screen._attack_intents.get("blade-attack")
	_expect(panel != null, "the generated attack has an intent")
	if panel != null:
		var hint: String = panel.intent.tooltip_text
		var rules: String = BattlePresentationCatalog.ability("board_main3_main4_main5").text
		_expect(hint.begins_with(BattlePresentationCatalog.ability("board_main3_main4_main5").name), "ability name leads the hover")
		_expect(not rules.is_empty() and rules in hint, "the hover shows the generated ability's full rules")
		_expect("Anvil" in hint and "Selected:" in hint and "Offensive dice:" in hint, "the hover names the tier, the die's symbols and the revealed dice")
	screen.queue_free(); canvas.queue_free(); await process_frame

func _damage(builder) -> Dictionary:
	var out := {}
	for tier in builder.priced.report.tiers: out[str(tier.tier)] = int(tier.whole_damage)
	return out

func _click_at(control: Control, fraction: float) -> void:
	var rect := control.get_global_rect()
	var point := rect.position + Vector2(rect.size.x * fraction, rect.size.y / 2.0)
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	for frame in 3: await process_frame

func _slot_damage(builder, slot: String) -> Array:
	var out := []
	for tier in builder._tiers(slot): out.append(int(tier.whole_damage))
	return out

func _dice(faces: Array, die_id: String) -> Array:
	var dice := []
	for face in faces: dice.append({"face": face, "die_id": die_id})
	return dice

func _click(button: Control) -> void:
	if button == null: _expect(false, "control exists"); return
	var point := button.get_global_rect().get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	for frame in 3: await process_frame

func _click_tab(tabs: TabBar, index: int) -> void:
	var point := tabs.get_global_rect().position + tabs.get_tab_rect(index).get_center()
	var move := InputEventMouseMotion.new(); move.position = point; root.push_input(move, true); await process_frame
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
	for frame in 3: await process_frame

func _text(node: Node) -> String:
	var result := str(node.text) if node is Label else ""
	for child in node.get_children(): result += "\n" + _text(child)
	return result

func _capture(filename: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_BOARD_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	for frame in 4: await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(filename + ".png"))

func _expect(ok: bool, message: String) -> void:
	if ok: return
	failed = true; push_error(message); print("FAIL: " + message)
