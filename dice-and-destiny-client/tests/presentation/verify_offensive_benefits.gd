extends SceneTree
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var fixture: Dictionary = native.start_battle("offensive-benefits", 1788900535520209)
	BattlePresentationCatalog.configure(fixture.snapshot.content_catalog)
	var stage := Control.new(); stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); root.add_child(stage)
	stage.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	var tiles: Array[BattleAbilityTile] = []
	for available in [true, false]:
		var column := VBoxContainer.new(); column.position = Vector2(40 if available else 460, 50); column.size.x = 390; stage.add_child(column)
		for id in ["venom_gland", "fever_spike", "terminal_bite"]:
			var tile := BattleAbilityTile.new(); column.add_child(tile); tile.configure(id, available, false, available); tile.cinematic_compact(); tiles.append(tile)
	for frame in 5: await process_frame
	for tile in tiles:
		_expect(tile.size.y == 66, "compact height preserved")
		_expect(tile._offensive_summary.is_visible_in_tree(), "summaries visible for enabled and disabled abilities")
		for label in tile._offensive_summary.find_children("*", "Label", true, false):
			_expect(tile.get_global_rect().encloses(label.get_global_rect()), "all text fits")
		for recipe in tile._offensive_summary.find_children("*", "RichTextLabel", true, false):
			_expect(recipe.get_content_height() <= recipe.size.y + 1, "requirements fit")
	var path := OS.get_environment("DICE_AND_DESTINY_ABILITY_BENEFITS_SCREENSHOT")
	if not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(path)
	for tile in tiles:
		tile.show_selected_attack("4 damage · pending")
		_expect(not tile._offensive_summary.visible and tile._recipe_label.visible and "4 damage" in tile._recipe_label.text, "selected attack replaces recipe and subtext cleanly")
	stage.queue_free(); await process_frame
	print("OFFENSIVE BENEFITS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
