extends SceneTree

const LIBRARY := preload("res://content/battle_visuals/library.tres")
const SCENERY := preload("res://presentation/battle/battle_scenery.gd")
const GALLERY := preload("res://devtools/battle_art_gallery.tscn")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var ids := {}
	for profile in LIBRARY.fighters:
		_expect(not ids.has(profile.definition_id), "unique fighter ID: " + profile.definition_id)
		ids[profile.definition_id] = true
		_expect(profile.texture != null, "texture exists: " + profile.definition_id)
	var habitats := {}
	for habitat in LIBRARY.habitats:
		_expect(not habitats.has(habitat.id) and habitat.background != null, "unique loadable habitat: " + habitat.id)
		habitats[habitat.id] = habitat
		var ratio := float(habitat.background.get_width()) / habitat.background.get_height()
		_expect(absf(ratio - 16.0 / 9.0) < 0.05, "wide battle background: " + habitat.id)
	var manifest: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://../docs/battle-art-extraction-manifest.json"))
	_expect(manifest.entries.size() == 20 and habitats.size() == 11, "all 20 concepts and 11 distinct habitats available")
	var scenery := SCENERY.new(); scenery.size = Vector2(1920, 1080); root.add_child(scenery)
	var grid := GridContainer.new(); grid.columns = 4; grid.position = Vector2(0, 0)
	grid.add_theme_constant_override("h_separation", 0); grid.add_theme_constant_override("v_separation", 0)
	var cards: Array[Control] = []
	for entry in manifest.entries:
		var id: String = entry.id
		var profile := LIBRARY.fighter(id)
		_expect(profile != null, "source concept registered: " + id)
		if profile == null: continue
		var pixels := profile.texture.get_image()
		_expect(pixels.detect_alpha() != Image.ALPHA_NONE, "true transparent alpha: " + id)
		var transparent_samples := 0
		for y in range(0, pixels.get_height(), 16):
			for x in range(0, pixels.get_width(), 16):
				if pixels.get_pixel(x, y).a == 0: transparent_samples += 1
		_expect(transparent_samples > pixels.get_width() * pixels.get_height() / 256 * 0.12, "substantial empty alpha outside silhouette: " + id)
		for corner in [Vector2i(0, 0), Vector2i(pixels.get_width() - 1, 0), Vector2i(0, pixels.get_height() - 1), pixels.get_size() - Vector2i.ONE]:
			_expect(pixels.get_pixelv(corner).a < 0.005, "transparent corner: " + id)
		var layout: BattleEncounterVisual = load("res://content/battle_visuals/encounters/" + id + ".tres")
		_expect(layout.habitat == habitats[entry.habitat], "shared environment reference: " + id)
		scenery.display(layout)
		var enemy: TextureRect = scenery.get_node("Fighter_" + layout.fighters[1].instance_id)
		_expect(enemy.texture == profile.texture, "correct cutout in composition: " + id)
		_expect(Rect2(Vector2.ZERO, Vector2(1920, 1080)).encloses(enemy.get_rect()), "complete creature fits viewport: " + id)
		if DisplayServer.get_name() != "headless":
			await process_frame; await RenderingServer.frame_post_draw
			var screenshot := root.get_texture().get_image()
			var tile := VBoxContainer.new(); tile.custom_minimum_size = Vector2(480, 296)
			var label := Label.new(); label.text = entry.name + " · " + str(entry.habitat).replace("_", " "); label.add_theme_font_size_override("font_size", 16); tile.add_child(label)
			var picture := TextureRect.new(); picture.texture = ImageTexture.create_from_image(screenshot)
			picture.expand_mode = TextureRect.EXPAND_IGNORE_SIZE; picture.custom_minimum_size = Vector2(480, 270)
			tile.add_child(picture); cards.append(tile)
	scenery.queue_free(); await process_frame
	if DisplayServer.get_name() != "headless":
		var catalog := SubViewport.new()
		catalog.size = Vector2i(1920, 1500)
		catalog.render_target_update_mode = SubViewport.UPDATE_ALWAYS
		root.add_child(catalog); catalog.add_child(grid)
		for tile in cards: grid.add_child(tile)
		await process_frame; await RenderingServer.frame_post_draw
		catalog.get_texture().get_image().save_png("res://.godot/enemy-art-catalog.png")
		catalog.queue_free(); await process_frame
	else: grid.free()
	root.size = Vector2i(1920, 1080)
	var gallery = GALLERY.instantiate(); root.add_child(gallery); await process_frame
	_expect(gallery._enemy.item_count == LIBRARY.fighters.size() and gallery._habitat.item_count == 11, "gallery lists all independently selectable resources")
	var habitat_texture: Texture2D = gallery._scenery.get_node("Habitat").texture
	gallery._enemy.select(0); gallery._refresh()
	_expect(gallery._scenery.get_node("Habitat").texture == habitat_texture, "enemy selector does not change habitat")
	gallery._pair.button_pressed = true; gallery._refresh()
	_expect(gallery._scenery.has_node("Fighter_enemy_0") and gallery._scenery.has_node("Fighter_enemy_1"), "gallery supports duplicate profile instances")
	gallery.queue_free(); await process_frame
	print("ENEMY ART CATALOG: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("ENEMY ART CATALOG: " + message)
