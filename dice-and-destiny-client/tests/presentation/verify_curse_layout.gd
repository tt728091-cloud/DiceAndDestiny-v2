extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var base: Dictionary
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "curse")
	base = native.start_battle("curse-layout", 1788900535520209)
	_expect(base.get("accepted", false), "native Curse catalog loads")
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for count in [0, 3, 4, 5]: await _planning(count, viewport)
		for defense in ["hexward_rebuttal", "misfortune_repaid"]:
			await _defense(defense, viewport)
		await _damage_and_effects(viewport)
		await _cards(viewport)
	print("CURSE SHARED LAYOUT: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _fixture(count: int = 3) -> Dictionary:
	var f := base.duplicate(true)
	f.events = []; f.learned_policy = {}; f.legal_actions = []
	f.snapshot.segment = "offensive"; f.snapshot.stage = "planning"
	f.pending_input = {"blade": {"id": "layout-input", "stage": "planning", "segment": "offensive", "allowed_commands": ["planning_select_ability", "planning_pass"]}}
	var actor: Dictionary = f.snapshot.actors.blade
	actor.selected_ability = ""; actor.selected_tier = ""; actor.qualified_abilities = ["hexbrand"] if count >= 3 else []
	actor.dice = []
	for i in 5: actor.dice.append({"index": i, "die_id": "curse_d6", "face": 1 if i < count else 4, "symbols": ["curse_skull" if i < count else "curse_shroud"]})
	actor.roll_history = [{"dice": actor.dice}] if count > 0 else []
	for n in range(3, count + 1):
		f.legal_actions.append({"battle_id": f.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "layout-input", "ability_id": "hexbrand", "tier_id": "skull_%d" % n, "target_ids": ["goblin"]}})
	return f
func _screen(f: Dictionary):
	var screen = SCREEN.instantiate(); screen.initial_result = f; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("curse-layout.json")); screen._auto_pass_disabled = true
	root.add_child(screen)
	return screen
func _planning(count: int, viewport: Vector2i) -> void:
	var f := _fixture(count)
	var screen = _screen(f)
	await _frames()
	for tile in screen._ability_dock.find_children("*", "Button", true, false):
		if not tile is BattleAbilityTile: continue
		_check_tile(tile)
		if tile.ability_id == "hexbrand":
			var buttons := 0
			for button in tile.find_children("*", "Button", true, false):
				if not button.has_meta("tier_id"): continue
				buttons += 1
				var n := int(str(button.get_meta("tier_id")).trim_prefix("skull_"))
				_expect(button.disabled == (n > count), "Hexbrand qualification matches offered tier")
				_expect(button.get_node("TierOutcome").text == "%d DMG\n+%d Curse" % [n, n - 2], "every tier explains damage and Curse")
			_expect(buttons == 3, "all Hexbrand tiers visible")
		else:
			var outcomes: Array = tile._offensive_summary.find_children("AbilityOutcome", "Label", true, false)
			_expect(not outcomes.is_empty() and "Curse" in outcomes[0].text or tile.ability_id == "funeral_rattle", "abilities visibly explain outcomes")
	if count == 3: await _capture("curse-planning-%d" % viewport.x)
	if count >= 3:
		# Every selectable tier retains the authority's exact payload.
		for selected in range(3, count + 1):
			var action: Dictionary = screen._ability_tier_action("hexbrand", "skull_%d" % selected)
			_expect(action.payload.target_ids == ["goblin"], "tier preserves offered target")
		var fake: FakeBattleAuthority = screen.gateway._authority
		fake.enqueue(f.duplicate(true))
		var target: Button
		for button in screen._ability_dock.find_children("*", "Button", true, false):
			if button.get_meta("tier_id", "") == "skull_3": target = button
		var click := InputEventMouseButton.new(); click.position = target.get_global_rect().get_center(); click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true
		root.push_input(click, true)
		var release := click.duplicate(); release.pressed = false; root.push_input(release, true)
		await _frames()
		_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.tier_id == "skull_3", "real tier click submits exactly the chosen action")
		while Time.get_ticks_msec() <= screen._flow_until: await process_frame
		f.snapshot.actors.blade.selected_ability = "hexbrand"; f.snapshot.actors.blade.selected_tier = "skull_%d" % count
		f.legal_actions = []; screen._view.apply_result(f); screen._render(); await _frames()
		var selected: Dictionary = screen._selected_attack("blade")
		_expect("Then apply %d Curse" % (count - 2) in selected.text, "selected attack preserves chosen Curse tier")
		for tile in screen._ability_dock.find_children("*", "Button", true, false):
			if tile is BattleAbilityTile: _check_tile(tile)
		if count == 3: await _capture("curse-selected-%d" % viewport.x)
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _defense(id: String, viewport: Vector2i) -> void:
	var f := _fixture()
	f.snapshot.segment = "defensive"; f.snapshot.stage = "defense_selection"
	f.pending_input.blade.stage = "defense_selection"; f.pending_input.blade.segment = "defensive"
	f.snapshot.damage_sources = [{"id": "incoming", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 6}]
	f.legal_actions = [{"battle_id": f.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "layout-input", "ability_id": id, "target_ids": ["incoming"]}}]
	var screen = _screen(f); screen._selected_source = "incoming"; screen._render(); await _frames()
	for tile in screen._ability_dock.find_children("*", "Button", true, false):
		if tile is BattleAbilityTile:
			_check_tile(tile)
			_expect("Prevent" in tile._recipe_label.text and "Energy" in tile._recipe_label.text, "defenses explain rolls, costs and prevention")
	await _capture("curse-defense-selection-%s-%d" % [id, viewport.x])
	f.snapshot.stage = "defense_reaction"; f.pending_input.blade.stage = "defense_reaction"; f.pending_input.blade.allowed_commands = ["pass"]; f.legal_actions = []
	f.snapshot.defense_selections = {"blade": {"actor_id": "blade", "ability_id": id, "source_id": "incoming", "rolled_face": 1, "rolled_faces": [1] if id == "hexward_rebuttal" else [1, 6]}}
	screen._view.apply_result(f); screen._render(); await _frames()
	var counts := {"blade": {}, "goblin": {}}
	var result: Dictionary = screen._compact_defense_data("blade", counts, f.snapshot.damage_sources[0])
	_expect(result.prevented == (2 if id == "hexward_rebuttal" else 1), "visible prevention matches Curse symbols")
	_expect("Curse" in result.note if id == "hexward_rebuttal" else "Expand" in result.note, "follow-up outcome visible during defense")
	if id == "misfortune_repaid":
		_expect(result.gains.size() == 1 and result.gains[0].amount == 2 and result.gains[0].status_id == "curse_count", "Omen displays its Count reward")
		f.snapshot.defense_selections.blade.rolled_faces = [6, 6]
		screen._view.apply_result(f)
		result = screen._compact_defense_data("blade", {"blade": {}, "goblin": {}}, f.snapshot.damage_sources[0])
		_expect(result.gains.size() == 1 and result.gains[0].amount == 2, "double Omen reward shown once")
	for panel in screen._defense_result_panels: panel.started_ms = Time.get_ticks_msec() - 9000; panel._process(0)
	await _capture("curse-defense-result-%s-%d" % [id, viewport.x])
	screen.active_store.clear(); screen.queue_free(); await process_frame
func _damage_and_effects(viewport: Vector2i) -> void:
	var f := _fixture(4)
	f.pending_input = {}; f.legal_actions = []
	f.snapshot.segment = "damage_resolution"; f.snapshot.stage = "damage_reaction"
	f.snapshot.actors.blade.selected_ability = "hexbrand"; f.snapshot.actors.blade.selected_tier = "skull_4"
	var source := {"id": "curse-damage", "source_actor_id": "blade", "target_actor_id": "goblin", "source_content_id": "hexbrand", "base_amount": 4, "final_amount": 2, "prevention": 2}
	var removal := {"card_id": "lost", "card_definition_id": "blind_omen", "target_actor_id": "goblin", "accepted": true, "released": false, "original_zone": "deck", "damage_proposal_ids": ["curse-damage"]}
	f.snapshot.damage_sources = [source]
	f.snapshot.settled_damage = {"id": "curse-batch", "committed": false, "sources": [source], "removals": [removal]}
	var screen = _screen(f); await _frames()
	var selected: Dictionary = screen._selected_attack("blade")
	_expect("2 damage" in selected.text and "2 prevented" in selected.text and "2 Curse" in selected.text, "damage stage retains actual damage, prevention and Curse effect")
	await _capture("curse-damage-%d" % viewport.x)
	screen.active_store.clear(); screen.queue_free(); await process_frame
	f = _fixture(); f.pending_input = {}; f.legal_actions = []
	var before: Dictionary = f.snapshot.actors.duplicate(true)
	before.goblin.statuses = [{"definition_id": "curse_count", "stacks": 8}, {"definition_id": "three_knocks_status", "stacks": 1}]
	var after := before.duplicate(true); after.goblin.statuses = [{"definition_id": "curse_count", "stacks": 2}]
	source.source_content_id = "curse_count"; source.base_amount = 4; source.final_amount = 4
	var summary := {"actors_before": before, "actors_after": after, "steps": [
		{"type": "curse_resolved", "actor_id": "goblin", "data": {"kind": "conversion", "count_before": 8, "damage": 4, "count_after": 2}},
		{"type": "damage_committed", "data": {"sources": [source], "removals": [removal], "overage": {"goblin": 3}}}]}
	f.events = [{"type": "effects_resolved", "segment": "ongoing_effects", "round": 1, "sequence": 100, "data": summary}]
	screen = _screen(f); await _frames()
	var panel = screen._effects_panel
	_expect(is_instance_valid(panel), "Curse uses shared automatic Effects layout")
	if is_instance_valid(panel):
		panel.set_paused(true); panel.resume_at(3.9); panel.present_progress(); await _frames()
		_expect(panel.entries.size() == 1 and panel.entries[0].damage == 4, "Count conversion displays actual Three Knocks damage")
		var detail := panel.find_child("CurseConversion", true, false) as Label
		_expect(detail != null and detail.text == "8 Count · 4 damage · 2 retained", "conversion explains Count, damage and remainder")
		_expect(panel.entries[0].cards_ui.size() == 4, "effects show lost card and excess damage using common grid")
		await _capture("curse-effects-%d" % viewport.x)
		await create_timer(1.0).timeout
	screen.active_store.clear(); screen.queue_free(); await process_frame

func _cards(_viewport: Vector2i) -> void:
	BattlePresentationCatalog.configure(base.snapshot.content_catalog)
	var stage := Control.new(); root.add_child(stage)
	stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create()
	var cards: Dictionary = base.snapshot.content_catalog.cards
	var checked := 0
	var displayed: Array[BattleCard] = []
	for id in cards:
		if cards[id].get("targeting", {}).get("selector") != "curse_choice": continue
		for compact in [false, true]:
			var card := BattleCard.new(); stage.add_child(card); card.configure("layout", id, false, compact, compact)
			card.size = card.custom_minimum_size
			displayed.append(card)
		checked += 1
	await _frames()
	for card in displayed:
		var label := card.find_child("EffectSummary", true, false) as Label
		_expect(label != null and not label.text.is_empty(), "every Curse card has a visible effect plaque")
		if card.custom_minimum_size.x == 185:
			_expect(label.get_theme_font_size("font_size") == 14, "normal hand cards retain readable summary text")
		_expect(card.get_global_rect().encloses(label.get_global_rect()), "hand and removed-card summaries fit")
		_expect(card.get_node("EffectPlaque").position.y >= 43, "card summaries stay below the title: %s, font %d, plaque %s, label %s" % [card.definition_id, label.get_theme_font_size("font_size"), card.get_node("EffectPlaque").get_rect(), label.size])
	_expect(checked == 24, "all 24 Curse cards checked")
	stage.queue_free(); await process_frame
func _check_tile(tile: BattleAbilityTile) -> void:
	for label in tile.find_children("*", "Label", true, false):
		if not label.is_visible_in_tree() or label.name == "UpgradeNotice": continue
		_expect(tile.get_global_rect().grow(1).encloses(label.get_global_rect()), "%s label fits tile: %s" % [tile.ability_id, label.text])
	for label in tile.find_children("*", "RichTextLabel", true, false):
		if label.is_visible_in_tree(): _expect(label.get_content_height() <= label.size.y + 1, "%s wrapped rules fit" % tile.ability_id)
	_expect(root.get_visible_rect().encloses(tile.get_global_rect()), "%s fits viewport" % tile.ability_id)
func _frames() -> void:
	for frame in 8: await process_frame
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_CURSE_UI_SCREENSHOTS")
	if dir.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CURSE LAYOUT: " + message)
