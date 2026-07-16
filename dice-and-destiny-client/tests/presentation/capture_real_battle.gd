extends SceneTree

const OUTPUT := "res://tests/screenshots"
var gateway := BattleGateway.new()

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(OUTPUT))
	var battle_id := "capture-%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var started := gateway.start_battle(battle_id, "blade")
	if started.get("accepted") != true: _fail(JSON.stringify(started)); return
	await _capture(started, Vector2i(1920, 1080), "real-offensive-pre-1920x1080.png")
	var pending: Dictionary = started.get("pending_input", {}).get("blade", {})
	var rolled := gateway.submit(BattleCommandBuilder.planning_roll(battle_id, "blade", pending))
	if rolled.get("accepted") != true: _fail(JSON.stringify(rolled)); return
	await _capture(rolled, Vector2i(1920, 1080), "real-offensive-rolled-1920x1080.png")
	await _capture(rolled, Vector2i(1280, 720), "real-offensive-rolled-1280x720.png")
	print("SCREENSHOTS: captured real authority pre-roll and rolled states at both target sizes"); quit(0)

func _capture(result: Dictionary, size: Vector2i, filename: String) -> void:
	root.size = size
	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = result; screen.gateway = gateway; screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("capture_active.json")); screen.last_presented_sequence = 999999
	root.add_child(screen); await process_frame; await process_frame; await process_frame
	var image := root.get_texture().get_image()
	if image == null: _fail("rendering backend did not provide an image for %s" % filename); return
	var error := image.save_png(ProjectSettings.globalize_path("%s/%s" % [OUTPUT, filename]))
	if error != OK: _fail("could not save %s" % filename)
	screen.queue_free(); await process_frame

func _fail(message: String) -> void: push_error(message); quit(1)
