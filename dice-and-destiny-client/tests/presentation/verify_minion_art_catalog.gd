extends SceneTree

const GALLERY := preload("res://devtools/battle_art_gallery.tscn")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var catalog: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://content/battle_visuals/minion_catalog.json"))
	_expect(catalog.factions.size() == 20, "twenty boss factions")
	var ids := {}
	var textures := {}
	for faction in catalog.factions:
		_expect(faction.minions.size() == 10, "ten minions for " + str(faction.boss_id))
		for entry in faction.minions:
			_expect(not ids.has(entry.id), "unique ID " + str(entry.id))
			_expect(not textures.has(entry.texture), "individual sprite " + str(entry.id))
			ids[entry.id] = true; textures[entry.texture] = true
			var profile: FighterVisualProfile = load(entry.profile)
			_expect(profile != null, "loadable profile " + str(entry.id))
			if profile == null: continue
			_expect(profile.definition_id == entry.id and profile.display_name == entry.name, "profile identity " + str(entry.id))
			_expect(profile.texture != null and profile.portrait == profile.texture, "sprite and portrait " + str(entry.id))
			_expect(profile.texture.get_width() == entry.width and profile.texture.get_height() == entry.height, "full-resolution sprite " + str(entry.id))
			_expect(profile.default_height <= 320.0 and profile.faces_left, "minion presentation defaults " + str(entry.id))
	_expect(ids.size() == 200, "two hundred distinct creatures")
	var gallery = GALLERY.instantiate(); root.add_child(gallery); await process_frame
	_expect(gallery._collection.item_count == 21, "boss collection plus twenty minion factions")
	for i in catalog.factions.size():
		gallery._collection.select(i + 1); gallery._populate_enemies()
		_expect(gallery._enemy.item_count == 10, "gallery contains whole faction")
		gallery._enemy.select(9); gallery._refresh()
		var habitat_texture: Texture2D = gallery._scenery.get_node("Habitat").texture
		gallery._enemy.select(0); gallery._refresh()
		_expect(gallery._scenery.get_node("Habitat").texture == habitat_texture, "minion selection preserves habitat")
		gallery._pair.button_pressed = true; gallery._refresh()
		var first: TextureRect = gallery._scenery.get_node("Fighter_enemy_0")
		var second: TextureRect = gallery._scenery.get_node("Fighter_enemy_1")
		_expect(first.texture == second.texture and first.position != second.position, "reusable sprite with independent placements")
		if i == 0 and DisplayServer.get_name() != "headless":
			await process_frame; await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png("res://.godot/minion-art-preview.png")
		gallery._pair.button_pressed = false
		await process_frame
	gallery.queue_free(); await process_frame
	print("MINION ART CATALOG: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("MINION ART CATALOG: " + message)
