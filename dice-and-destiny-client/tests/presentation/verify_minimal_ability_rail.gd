extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("minimal-rail", 9)
	base.events = []; base.learned_policy = {}; base.legal_actions = []
	base.snapshot.segment = "offensive"; base.snapshot.stage = "planning"
	base.pending_input = {"blade": {"id": "minimal", "segment": "offensive", "stage": "planning", "allowed_commands": ["planning_select_ability", "planning_reroll", "planning_pass"]}}
	base.snapshot.actors.blade.qualified_abilities = ["hexbrand", "grasp_of_the_sarcophagus", "funeral_rattle", "eclipse_of_the_black_star"]
	for pair in [["hexbrand", "skull_3"], ["hexbrand", "skull_4"], ["grasp_of_the_sarcophagus", "grasp"], ["funeral_rattle", "rattle"], ["eclipse_of_the_black_star", "eclipse"]]:
		base.legal_actions.append({"battle_id": base.snapshot.battle_id, "actor_id": "blade", "type": "planning_select_ability", "payload": {"pending_input_id": "minimal", "ability_id": pair[0], "tier_id": pair[1]}})
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for action in base.legal_actions:
			var fake := FakeBattleAuthority.new(); fake.enqueue(base)
			var screen = SCREEN.instantiate(); screen.initial_result = base; screen.gateway = BattleGateway.new(fake)
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("minimal-rail.json"))
			root.add_child(screen); screen.set_process(false)
			for frame in 8: await process_frame
			var tiles: Array = screen._ability_dock.find_children("*", "BattleAbilityTile", true, false)
			_expect(tiles.size() == 4, "four minimal Curse rows")
			var first_title: Label = tiles[0].find_child("MinimalTitle", true, false)
			var first_icon: TextureRect = tiles[0].get_node("TierControls").get_child(0)
			var tier_right: float = tiles[0].get_node("TierControls").get_global_rect().end.x
			var click: Button
			for tile in tiles:
				var title: Label = tile.find_child("MinimalTitle", true, false)
				_expect(is_equal_approx(title.global_position.x, first_title.global_position.x), "ability names share a left edge")
				_expect(tile.size.y == 48 and tile._minimal, "no expanded effect prose")
				_expect(screen._ability_dock.get_parent().get_global_rect().encloses(tile.get_global_rect()), "rows stay inside rail")
				if tile.ability_id == "hexbrand":
					var tiers: Array = tile.find_children("*", "Button", true, false).filter(func(button): return button.has_meta("tier_id"))
					_expect(tiers.size() == 3, "all three Hexbrand tier buttons remain")
					_expect(is_equal_approx(tiers.back().get_global_rect().end.x, tier_right), "last tier reaches shared right edge")
					_expect(title.get_global_rect().end.x <= tiers.front().global_position.x, "expanded title does not overlap tier hit targets")
					for button in tiers:
						var id: String = button.get_meta("tier_id")
						_expect(button.disabled == (id == "skull_5"), "qualification gates each tier")
						if tile.ability_id == action.payload.ability_id and id == action.payload.tier_id: click = button
				else:
					var row: HBoxContainer = tile.get_node("MinimalAbility")
					_expect(is_equal_approx(row.get_child(0).global_position.x, first_icon.global_position.x), "ability icons share a left edge")
					_expect(is_equal_approx(row.get_node("MinimalRequirement").get_global_rect().end.x, tier_right), "requirements and tier controls share a right edge")
					_expect(tile.get_node_or_null("AbilityChoices") == null and not tile.disabled, "one enabled button for single-outcome ability")
					if tile.ability_id == action.payload.ability_id: click = tile
			_expect(screen._roll_dock.get_global_rect().end.y < screen._player_dice_dock.get_global_rect().position.y and screen._player_dice_dock.get_global_rect().end.y < screen._ability_dock.get_global_rect().position.y, "Roll/Skip above dice above abilities")
			_expect(is_equal_approx(screen._player_dice_dock.position.x, screen._roll_dock.position.x), "dice and Roll share the left edge")
			_expect(screen._roll_dock.get_global_rect().end.x <= screen._action_footer.get_global_rect().position.x, "Roll leaves room for Skip")
			if not OS.get_environment("DICE_AND_DESTINY_MINIMAL_SCREENSHOTS").is_empty() and action == base.legal_actions[0] and DisplayServer.get_name() != "headless":
				RenderingServer.force_draw(false); root.get_texture().get_image().save_png(OS.get_environment("DICE_AND_DESTINY_MINIMAL_SCREENSHOTS").path_join("enabled-%d.png" % viewport.x))
			if click != null:
				var point := click.get_global_rect().get_center()
				var motion := InputEventMouseMotion.new(); motion.position = point; root.push_input(motion, true)
				for pressed in [true, false]:
					var event := InputEventMouseButton.new(); event.position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true)
				await process_frame
				_expect(fake.commands.size() == 1, "button submits one selection")
				if fake.commands.size() == 1: _expect(JSON.parse_string(fake.commands[0]).payload == action.payload, "correct ability and tier submitted")
			else: _expect(false, "click target exists")
			screen.queue_free(); await process_frame
	print("MINIMAL ABILITY RAIL: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("MINIMAL RAIL: " + message)
