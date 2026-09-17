extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = native.start_battle("cinematic-layout", 1788900535520209)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	fixture.events = []; fixture.learned_policy = {}
	fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
	fixture.pending_input = {"blade": {"id": "cinematic", "segment": "offensive", "stage": "planning", "allowed_commands": ["planning_keep", "planning_reroll", "planning_select_ability", "planning_commit_cards", "planning_pass"]}}
	var actor: Dictionary = fixture.snapshot.actors.blade
	actor.energy_points = 3; actor.qualified_abilities = ["needlefang"]
	actor.statuses = [{"definition_id": "catalyst", "stacks": 2}, {"definition_id": "bleed", "stacks": 1}]
	actor.hand = []; actor.card_instances = {}; actor.hand_count = 5
	for id in ["deep_puncture", "steady_hand", "shock_dose", "spined_rebuttal", "venom_lens"]:
		actor.hand.append(id); actor.card_instances[id] = {"instance_id": id, "definition_id": id}
	var dice: Array = []
	for i in 5: dice.append({"index": i, "pool": "offensive", "die_id": "venom_d6", "face": [1, 2, 3, 1, 5][i]})
	actor.dice = dice; actor.roll_history = [{"number": 1, "dice": dice, "kept_indices": [0, 1]}]
	fixture.legal_actions = []
	# Catalog is configured when the screen receives the fixture.
	for id in ["fang_3", "fang_4"]:
		fixture.legal_actions.append({"battle_id": fixture.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "cinematic", "ability_id": "needlefang", "tier_id": id}})
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720), Vector2i(1600, 1000)]:
		root.size = viewport
		var fake := FakeBattleAuthority.new(); fake.enqueue(fixture)
		var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen.set_process(false)
		screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("cinematic-layout.json"))
		root.add_child(screen); screen.set_process(false)
		await process_frame; await process_frame
		for id in ["PlayerProfile", "EnemyProfile", "PlayerDice", "EnemyDice", "AbilityRail", "HandDock", "BattleActionFooter", "BattleUtilities", "BattleContentScroll"]:
			var control: Control = screen._root.find_child(id, true, false)
			_expect(control != null and root.get_visible_rect().encloses(control.get_global_rect()), "%s fits %s" % [id, viewport])
		var dice_rect: Rect2 = screen._player_dice_dock.get_global_rect()
		var roll_rect: Rect2 = screen._roll_dock.get_global_rect()
		_expect(is_equal_approx(dice_rect.position.y, screen._enemy_dice_dock.get_global_rect().position.y), "player and opponent dice share one horizontal baseline")
		var ability_rect: Rect2 = screen._ability_dock.get_parent().get_global_rect()
		_expect(dice_rect.end.y <= roll_rect.position.y and roll_rect.end.y <= ability_rect.position.y, "left dice, roll controls and abilities never overlap: %s / %s / %s" % [dice_rect, roll_rect, ability_rect])
		_expect(not screen._roll_dock.get_global_rect().intersects(screen._action_footer.get_global_rect()), "roll and skip remain separate click targets")
		var roll_button := _control(screen, "battle.command.planning_reroll")
		var skip_button := _control(screen, "battle.command.planning_pass")
		_expect(roll_button != null and skip_button != null and is_equal_approx(roll_button.get_global_rect().position.y, skip_button.get_global_rect().position.y) and is_equal_approx(roll_button.size.y, skip_button.size.y), "Roll and Skip have identical top and height")
		for die in screen._player_dice_dock.find_children("*", "Button", true, false):
			for state in ["normal", "hover", "pressed", "disabled"]:
				_expect(die.get_theme_stylebox(state).bg_color == Color("242423"), "dice retain uniform black faces in every interaction state")
		var hand_scroll: ScrollContainer = screen._hand_dock.find_child("HandScroll", true, false)
		_expect(hand_scroll != null and hand_scroll.get_child(0).get_child_count() == 5, "all hand cards retained")
		_expect(screen._ability_dock.find_children("*", "BattleAbilityTile", true, false).size() == 4, "four compact ability tiles retained")
		_expect(screen.find_children("*", "BattleDiceTray", true, false).size() == 2, "both live dice trays retained")
		var log_button := _control(screen, "battle.utility.log")
		await _click(log_button)
		await process_frame; await process_frame
		var log_panel: Control = screen._utility_panels.log
		var log_rect := log_panel.get_global_rect()
		var enemy_rect: Rect2 = screen._enemy_dice_dock.get_global_rect()
		var utilities: Control = screen._root.find_child("BattleUtilities", true, false)
		_expect(log_panel.visible and is_equal_approx(log_rect.position.x, enemy_rect.position.x) and is_equal_approx(log_rect.size.x, enemy_rect.size.x), "Log opens aligned under enemy dice")
		_expect(log_rect.position.y > enemy_rect.end.y and log_rect.end.y < utilities.get_global_rect().position.y, "Log leaves gaps below dice and above utility buttons")
		_expect(screen._log.size.y > log_panel.size.y * 0.75, "combat log fills the tall side panel")
		_expect(not log_rect.intersects(screen._hand_dock.get_global_rect()), "Log leaves hand and center unobstructed")
		await _click(log_button)
		_expect(not log_panel.visible, "Log button closes side panel")
		var settings := _control(screen, "battle.utility.settings")
		await _click(settings)
		_expect(screen._utility_panels.settings.is_visible_in_tree(), "Settings opens via real pointer input")
		await _click(screen._auto_pass_toggle)
		_expect(screen._auto_pass_disabled, "debug auto-pass setting remains functional")
		await _click(settings)
		_expect(not screen._utility_panels.settings.is_visible_in_tree(), "Settings closes without a gameplay command")
		for tile in screen._ability_dock.find_children("*", "BattleAbilityTile", true, false):
			_expect(screen._ability_dock.get_parent().get_global_rect().encloses(tile.get_global_rect()), "all four ability tiles fit without scrolling")
		var expected_benefits := {"venom_gland": ["Apply 1" + str(BattlePresentationCatalog.status("poison").glyph) + " · Gain 2 Catalyst"], "fever_spike": ["4 DMG · Provoke 2", "3 DMG · Provoke 1"], "terminal_bite": ["Gain 1 Catalyst · 4 DMG · Provoke 2"]}
		for tile in screen._ability_dock.find_children("*", "BattleAbilityTile", true, false):
			if not expected_benefits.has(tile.ability_id): continue
			var benefits: Array[String] = []
			for label in tile.find_children("*", "Label", true, false):
				if label.name != "AbilityOutcome": continue
				benefits.append(label.text)
				_expect(label.is_visible_in_tree() and tile.get_global_rect().encloses(label.get_global_rect()), "always-visible outcome stays inside " + tile.ability_id)
				_expect(label.get_minimum_size().x <= label.size.x + 1, "effect text fits without clipping")
				_expect(label.mouse_filter == Control.MOUSE_FILTER_IGNORE, "summary leaves ability click target intact")
			_expect(benefits == expected_benefits[tile.ability_id], "all tier outcomes match current rules for " + tile.ability_id)
			_expect(tile.size.y == 66, "inline outcomes preserve compact tile height")
			for recipe in tile._offensive_summary.find_children("*", "RichTextLabel", true, false):
				_expect(tile.get_global_rect().encloses(recipe.get_global_rect()) and recipe.get_content_height() <= recipe.size.y + 1, "requirements remain readable inside tile")
		var three := _control(screen, "battle.ability.blade.needlefang.fang_3")
		var four := _control(screen, "battle.ability.blade.needlefang.fang_4")
		var five := _control(screen, "battle.ability.blade.needlefang.fang_5")
		_expect(three != null and not three.disabled and four != null and not four.disabled and five != null and five.disabled, "only qualified Needlefang tiers are selectable")
		var kept_die := _control(screen, "battle.die.blade.0")
		if not kept_die.button_pressed: await _click(kept_die)
		await _click(kept_die)
		_expect(not kept_die.button_pressed and 0 not in screen._selected_indices, "click releases a kept die")
		await _click(kept_die)
		_expect(kept_die.button_pressed and 0 in screen._selected_indices, "click keeps the die again")
		var hover := InputEventMouseMotion.new(); hover.position = kept_die.get_global_rect().get_center(); root.push_input(hover, true)
		_expect(kept_die.get_draw_mode() == BaseButton.DRAW_HOVER_PRESSED, "pointer exercises the kept-and-hovered rendering state")
		for key in ["font_color", "font_hover_color", "font_pressed_color", "font_hover_pressed_color", "font_disabled_color", "font_focus_color"]:
			_expect(kept_die.get_theme_color(key).a == 0.0, "illustrated dice never draw duplicate native text: " + key)
		await _capture("planning-%dx%d" % [viewport.x, viewport.y])
		# Status growth must push the block as a unit without touching the HUD.
		var profile = screen._actor_profiles.blade
		var original_statuses: String = profile.statuses.text
		profile.statuses.text = "Poison ×3\nVolatile Poison ×3\nCatalyst ×3\nBleed ×3\nIncubation ×1\nEntangle ×1\nAdditional status\nAdditional status"
		await process_frame; await process_frame; await process_frame
		_expect(screen._player_dice_dock.position.y >= screen._player_profile_dock.position.y + screen._player_profile_dock.get_combined_minimum_size().y + 48, "expanded status HUD retains breathing room above dice")
		_expect(is_equal_approx(screen._roll_dock.position.y - screen._player_dice_dock.position.y, 113), "status growth moves the controls together")
		_expect(is_equal_approx(screen._player_dice_dock.position.y, screen._enemy_dice_dock.position.y), "status growth preserves dice symmetry")
		_expect(screen._utility_panels.log.position.y == screen._enemy_dice_dock.position.y + screen._enemy_dice_dock.size.y + 18.0, "log follows dice when statuses grow")
		profile.statuses.text = original_statuses
		await process_frame; await process_frame; await process_frame
		# Long content must never move the docked action controls or hand.
		var footer_rect: Rect2 = screen._action_footer.get_global_rect()
		for i in 60:
			var line := Label.new(); line.text = "Long combat result %d" % i; screen._center.add_child(line)
		await process_frame; await process_frame
		_expect(screen._center_scroll.get_v_scroll_bar().max_value > screen._center_scroll.size.y, "long results remain scrollable")
		_expect(screen._action_footer.get_global_rect() == footer_rect, "long results cannot push actions off screen")
		await _click(three)
		_expect(fake.commands.size() == 1 and JSON.parse_string(fake.commands[0]).payload.tier_id == "fang_3", "inline tier pointer click submits exactly the chosen tier")
		# Chosen attacks stay with their owner through reactions, defense and damage.
		screen._view.actors.blade.selected_ability = "needlefang"
		screen._view.actors.goblin.selected_ability = "sword_cut"
		screen._view.offensive_reveals = {
			"blade": {"ability_id": "needlefang", "outcome": {"base_damage": 3, "status_applications": [{"status_id": "poison", "stacks": 2}]}},
			"goblin": {"ability_id": "sword_cut", "outcome": {"base_damage": 7, "status_applications": [{"status_id": "bleed", "stacks": 1}]}}}
		screen._view.damage_sources = [
			{"id": "enemy-attack", "source_actor_id": "goblin", "source_content_id": "sword_cut", "target_actor_id": "blade", "base_amount": 7, "reaction_prevention": 1},
			{"id": "player-attack", "source_actor_id": "blade", "source_content_id": "needlefang", "target_actor_id": "goblin", "base_amount": 3}]
		for stage in ["offensive_reaction", "defense_selection", "defense_reaction", "damage_reaction"]:
			screen._view.stage = stage
			screen._view.segment = "offensive" if stage == "offensive_reaction" else "damage_resolution" if stage == "damage_reaction" else "defensive"
			if stage == "defense_reaction":
				screen._view.defense_selections = {"blade": {"ability_id": "shedskin", "source_id": "enemy-attack", "rolled_faces": [1, 4]}, "goblin": {"ability_id": "basic_defense", "source_id": "player-attack", "rolled_faces": [6]}}
			screen._render()
			for frame in 5: await process_frame
			var player_attack: BattleAbilityTile
			for tile in screen._ability_dock.find_children("*", "BattleAbilityTile", true, false):
				if tile.ability_id == "needlefang": player_attack = tile
			var enemy_attack := _control(screen, "battle.outcome.goblin")
			_expect(player_attack != null and "3 damage" in player_attack.text and "Poison ×2" in player_attack.text, "player attack replaces requirements with outcome during " + stage)
			_expect(enemy_attack != null and "6 damage" in enemy_attack.text and "Bleed ×1" in enemy_attack.text, "enemy attack shows live prevention during " + stage)
			_expect(screen._enemy_attack_dock.get_global_rect().position.x >= screen._enemy_dice_dock.get_global_rect().position.x, "enemy attack stays under enemy dice")
			_expect(screen._center.find_children("*", "BattleAbilityTile", true, false).is_empty(), "attack summaries stay out of center")
			for label in screen._center.find_children("*", "Label", true, false):
				_expect(not label.visible or label.text not in ["INCOMING SOURCES", "Defense Selection", "Offensive Reaction"], "duplicate center headings removed")
			if stage == "defense_selection":
				await _click(_control(screen, "battle.source.enemy-attack"))
				_expect(screen._selected_source == "enemy-attack", "side attack source remains selectable")
			if stage == "defense_reaction":
				for panel in screen._defense_result_panels:
					_expect(root.get_visible_rect().encloses(panel.get_global_rect()), "side defense results fit viewport")
			await _capture("side-attacks-%s-%dx%d" % [stage, viewport.x, viewport.y])
		screen._view.actors.blade.hand = []; screen._view.segment = "damage_resolution"; screen._view.stage = "damage_reaction"
		screen._render(); await process_frame; await process_frame
		var actions: Control = screen._root.find_child("BattleActionFooter", true, false)
		_expect(not screen._center_scroll.get_global_rect().intersects(actions.get_global_rect()), "expanded empty-hand results cannot cover the action dock")
		screen.active_store.clear(); screen.queue_free(); await process_frame
	print("CINEMATIC LAYOUT: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _control(node: Node, id: String) -> Control:
	for child in node.find_children("*", "Control", true, false):
		if child.get_meta("inspection_id", "") == id: return child
	return null

func _click(control: Control) -> void:
	if control == null: _expect(false, "pointer target exists"); return
	var point := control.get_global_rect().get_center()
	var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true)
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true)
	await process_frame

func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_CINEMATIC_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("CINEMATIC LAYOUT: " + message)
