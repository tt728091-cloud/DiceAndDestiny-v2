extends SceneTree

const SCENERY := preload("res://presentation/battle/battle_scenery.gd")
const LIBRARY := preload("res://content/battle_visuals/library.tres")
const SPORE := preload("res://content/battle_visuals/encounters/spore_cantor.tres")
const PAIR := preload("res://content/battle_visuals/encounters/spore_cantor_pair.tres")
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var scenery := SCENERY.new(); scenery.size = Vector2(1920, 1080); root.add_child(scenery)
	var actors := {"blade": {"definition_id": "venom"}, "goblin": {"definition_id": "blade_warden"}}
	var original := actors.duplicate(true)
	scenery.display(null, actors)
	_expect(scenery.get_node("Habitat").texture.resource_path.ends_with("arena.png"), "default background is environment-only")
	_expect(scenery.get_node("Fighter_player").texture == LIBRARY.fighter("venom").texture, "player has an independent visual")
	_expect(scenery.get_node("Fighter_enemy").texture == LIBRARY.fighter("blade_warden").texture, "enemy has an independent visual")
	for id in ["venom", "blade_warden", "spore_cantor"]:
		var art := LIBRARY.fighter(id).texture.get_image()
		_expect(art.detect_alpha() != Image.ALPHA_NONE, id + " has a real alpha channel")
		_expect(art.get_pixel(0, 0).a == 0.0, id + " has no opaque backdrop")
	# Change habitat only, keeping the same actors and their placement resources.
	var layout: BattleEncounterVisual = LIBRARY.default_encounter.duplicate(true)
	layout.habitat = SPORE.habitat
	scenery.display(layout, actors)
	_expect(scenery.get_node("Habitat").texture == SPORE.habitat.background, "habitat swaps independently")
	_expect(scenery.get_node("Fighter_enemy").get_meta("definition_id") == "blade_warden", "habitat swap does not select a new enemy")
	# Change one actor identity while retaining the habitat and placement.
	actors.goblin.definition_id = "spore_cantor"
	scenery.display(layout, actors)
	_expect(scenery.get_node("Fighter_enemy").texture == LIBRARY.fighter("spore_cantor").texture, "definition selects its registered visual")
	_expect(scenery.get_node("Habitat").texture == SPORE.habitat.background, "enemy swap preserves habitat")
	_expect(LIBRARY.default_encounter.habitat.id == "wasteland", "shared resource remains unchanged")
	# Independent placement, facing, size, and draw ordering for repeated profiles.
	scenery.display(PAIR)
	var first: TextureRect = scenery.get_node("Fighter_Cantor")
	var second: TextureRect = scenery.get_node("Fighter_CantorTwin")
	_expect(first != second and first.texture == second.texture, "duplicates share art but have distinct nodes")
	_expect(first.position != second.position and first.size != second.size, "duplicate positions and sizes are independent")
	_expect(first.get_index() < second.get_index(), "draw order is respected")
	var mirrored: BattleEncounterVisual = PAIR.duplicate(true)
	mirrored.fighters[1].face_left = false
	scenery.display(mirrored)
	first = scenery.get_node("Fighter_Cantor")
	var anchor := mirrored.fighters[1].profile.ground_anchor
	anchor.x = 1.0 - anchor.x
	_expect(first.flip_h and (first.position + first.size * anchor).is_equal_approx(mirrored.fighters[1].ground_position), "mirroring keeps the ground contact point")
	_expect(PAIR.fighters[1].face_left, "mirroring an instance does not mutate the shared profile")
	# A missing bound actor must never inherit an unrelated explicit preview skin.
	var missing: BattleEncounterVisual = SPORE.duplicate(true)
	missing.fighters[1].actor_id = "not-in-battle"
	scenery.display(missing, original)
	_expect(not scenery.has_node("Fighter_Cantor"), "unknown bound actor is not painted as another enemy")
	_expect(original.blade.definition_id == "venom" and original.goblin.definition_id == "blade_warden", "composition does not mutate actor state")
	for child in scenery.get_children():
		_expect(child.mouse_filter == Control.MOUSE_FILTER_IGNORE, "art cannot intercept battle controls")
	scenery.display(SPORE)
	await _capture("spore-cantor")
	scenery.display(PAIR)
	await _capture("spore-cantor-pair")
	scenery.queue_free(); await process_frame
	# The production screen uses the exact same compositor, including on redraw.
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var screen = SCREEN.instantiate()
	screen.initial_result = gateway.start_battle("visual-composition", 1789679833957118)
	screen.gateway = gateway
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("visual-composition.json"))
	screen._auto_pass_disabled = true
	root.add_child(screen)
	for frame in 5: await process_frame
	var live: Control = screen._root.get_node("BattleScenery")
	_expect(live.has_node("Fighter_player") and live.has_node("Fighter_enemy"), "live battle renders both independent fighters")
	await _capture("layered-live-battle")
	screen.set_encounter_visual(layout)
	screen._render()
	live = screen._root.get_node("BattleScenery")
	_expect(live.get_node("Habitat").texture == SPORE.habitat.background, "configured habitat survives a battle redraw")
	_expect(live.get_node("Fighter_enemy").get_meta("definition_id") == screen._view.actor("goblin").definition_id, "live enemy matches authoritative identity")
	root.size = Vector2i(1280, 800)
	for frame in 3: await process_frame
	_expect(screen._root.scale.x == screen._root.scale.y, "letterboxing scales habitat and fighters together")
	var live_enemy: TextureRect = live.get_node("Fighter_enemy")
	var enemy_visual := LIBRARY.fighter(str(screen._view.actor("goblin").definition_id))
	_expect((live_enemy.position + live_enemy.size * enemy_visual.ground_anchor).is_equal_approx(layout.fighters[1].ground_position), "resizing preserves the configured ground position")
	var profile := ActorProfile.new(); root.add_child(profile)
	profile.display("new-enemy", {"definition_id": "spore_cantor"}, false)
	_expect(profile.title.text == "Spore Cantor" and profile.portrait.texture == LIBRARY.fighter("spore_cantor").portrait, "HUD uses the same visual profile registry")
	profile.display("unknown", {"definition_id": "unknown_enemy"}, false)
	_expect(profile.portrait.texture == null, "unknown definition never inherits previous portrait")
	profile.queue_free(); screen.active_store.clear(); screen.queue_free(); await process_frame
	print("BATTLE SCENERY: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _capture(name: String) -> void:
	if DisplayServer.get_name() == "headless": return
	await process_frame
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png("res://.godot/" + name + ".png")

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("BATTLE SCENERY: " + message)
