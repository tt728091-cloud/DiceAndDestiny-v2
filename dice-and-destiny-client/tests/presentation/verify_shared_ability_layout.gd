extends SceneTree
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1280, 1000)
	var stage := Control.new(); stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); root.add_child(stage)
	var tiles: Array[BattleAbilityTile] = []
	var column_index := 0
	for character in ["venom", "curse", "blade_warden"]:
		var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", character)
		var fixture: Dictionary = native.start_battle("shared-ability-" + character, 1788900535520209)
		BattlePresentationCatalog.configure(fixture.snapshot.content_catalog)
		var actor: Dictionary = fixture.snapshot.actors.blade
		var column := VBoxContainer.new(); column.position = Vector2(30 + column_index * 415, 30); column.size.x = 390; stage.add_child(column)
		var title := Label.new(); title.text = character.capitalize(); column.add_child(title)
		var ids: Array = actor.offensive_abilities.duplicate(); ids.append_array(actor.defensive_abilities)
		for id in ids:
			var tile := BattleAbilityTile.new(); column.add_child(tile); tile.configure(id, true, false, true, actor); tile.cinematic_compact()
			var tiers := BattlePresentationCatalog.inline_tiers(id, actor)
			if not tiers.is_empty():
				for tier in tiers: tier.enabled = true
				tile.configure_tiers(tiers, "")
			tiles.append(tile)
		# A new character's custom operation still gets its authored explanation.
		if character == "blade_warden":
			var catalog: Dictionary = fixture.snapshot.content_catalog.duplicate(true)
			catalog.abilities.future_ability = {"name": "Future ability", "type": "offensive", "presentation": {"rules_text": "Deal damage, then perform the new custom effect."}, "qualification": {"activation_tiers": [{"id": "base", "requirements": {"all": [{"type": "symbol_count", "symbol_id": "sword", "minimum": 3}]}, "operations": [{"type": "deal_damage", "amount": 3}, {"type": "future_operation"}]}]}}
			BattlePresentationCatalog.configure(catalog)
			var tile := BattleAbilityTile.new(); column.add_child(tile); tile.configure("future_ability", true, false, true); tile.cinematic_compact(); tiles.append(tile)
		column_index += 1
	for frame in 10: await process_frame
	for tile in tiles:
		_expect(not tile.tooltip_text.is_empty(), "full rules remain available")
		for label in tile.find_children("*", "Label", true, false):
			if not label.is_visible_in_tree() or label.name == "UpgradeNotice": continue
			_expect(tile.get_global_rect().grow(1).encloses(label.get_global_rect()), "%s: readable labels fit" % tile.ability_id)
		for label in tile.find_children("*", "RichTextLabel", true, false):
			if label.is_visible_in_tree(): _expect(label.get_content_height() <= label.size.y + 1, "%s: wrapped content fits" % tile.ability_id)
		if tile.ability_id == "future_ability":
			_expect("new custom effect" in tile.find_child("AbilityOutcome", true, false).text, "unknown operations fall back to authored rules")
	var path := OS.get_environment("DICE_AND_DESTINY_SHARED_ABILITY_SCREENSHOT")
	if not path.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(path)
	stage.queue_free(); await process_frame
	print("SHARED ABILITY LAYOUT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
