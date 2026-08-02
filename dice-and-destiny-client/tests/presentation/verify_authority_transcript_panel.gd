extends SceneTree

const PANEL := preload("res://devtools/developer_authority_transcript_panel.gd")

var _failed := false


func _initialize() -> void:
	call_deferred("_run")


func _run() -> void:
	if OS.get_environment("DICE_AND_DESTINY_ENABLE_AUTHORITY_TRANSCRIPT") != "1":
		_expect(not PANEL.is_enabled(), "transcript panel enabled without explicit environment opt-in")
		_finish("disabled-mode gate")
		return
	_expect(PANEL.is_enabled(), "opted-in debug transcript panel did not pass all gates")
	if _failed:
		_finish("enabled-mode gate")
		return

	var path := WorkspacePaths.persistent_file("debug/authority-transcript.jsonl")
	_expect(path == OS.get_environment("DICE_AND_DESTINY_AUTHORITY_TRANSCRIPT_PATH").simplify_path(), "panel path differs from launcher-owned authority path")
	DirAccess.make_dir_recursive_absolute(path.get_base_dir())
	var file := FileAccess.open(path, FileAccess.WRITE)
	if file == null:
		_fail("could not create transcript fixture: %s" % error_string(FileAccess.get_open_error()))
		_finish("fixture setup")
		return
	file.store_line(JSON.stringify(_record(1, "tail-a", "PUBLIC", "card_played", "human", "Human played a card")))
	file.flush()

	var panel := PANEL.new()
	panel.set_current_battle("tail-a")
	root.add_child(panel)
	await process_frame
	await create_timer(0.3).timeout
	panel.toggle_panel()
	await process_frame
	_expect(panel.inspection_state().get("open") == true, "panel toggle did not open the transcript")
	_expect(int(panel.inspection_state().get("records_buffered", 0)) == 1, "initial JSONL record was not tailed")
	_expect(panel._actor_label(panel._records[0]) == "Human Hero (seat-a)", "panel did not render the authority-provided actor name alongside its instance ID")
	var panel_bounds: Rect2 = panel.inspection_state().get("panel_bounds", Rect2())
	var viewport_size: Vector2 = panel._overlay.size
	_expect(panel.inspection_state().get("overlay_mouse_filter") == Control.MOUSE_FILTER_IGNORE, "empty transcript overlay still intercepts battle input")
	_expect(panel.find_children("*", "ColorRect", true, false).is_empty(), "transcript retained a full-screen dimming blocker")
	_expect(panel_bounds.position.x <= 24.0 and panel_bounds.position.y >= viewport_size.y * 0.5, "transcript was not docked in the lower-left play area: %s" % panel_bounds)
	_expect(panel_bounds.end.x <= viewport_size.x * 0.77 and panel_bounds.end.y <= viewport_size.y, "transcript overlaps the combat-log column or leaves the viewport: %s" % panel_bounds)

	_append(path, JSON.stringify(_record(2, "tail-a", "PRIVATE_OPPONENT", "die_rolled", "learned_policy", "Opponent secretly rolled")) + "\n")
	await create_timer(0.3).timeout
	_expect(int(panel.inspection_state().get("records_buffered", 0)) == 2, "live append did not reach the open panel")

	# A partial final write must not be parsed or skipped. Completing the same
	# line on a later poll proves live tailing never fabricates corrupt records.
	var third := JSON.stringify(_record(3, "tail-b", "DEBUG_SYSTEM", "command_rejected_stale", "authority", "Stale command rejected"))
	var split := floori(float(third.length()) / 2.0)
	_append(path, third.substr(0, split))
	await create_timer(0.3).timeout
	_expect(int(panel.inspection_state().get("records_buffered", 0)) == 2, "partial JSONL record was accepted")
	_append(path, third.substr(split) + "\n")
	await create_timer(0.3).timeout
	_expect(int(panel.inspection_state().get("records_buffered", 0)) == 3, "completed partial JSONL record was not retried")

	panel._filter = "Hidden"
	panel._search = "secretly"
	panel._render_records()
	_expect(int(panel.inspection_state().get("records_visible", 0)) == 1, "hidden/search filters did not isolate the opponent record")
	panel._pause_check.button_pressed = true
	panel._pause_check.toggled.emit(true)
	_expect(panel.inspection_state().get("paused") == true, "pause-view control did not pause rerendering")
	panel._follow_check.button_pressed = false
	panel._follow_check.toggled.emit(false)
	_expect(panel.inspection_state().get("follow_latest") == false, "follow-latest control did not pause scrolling")

	panel.set_current_battle("tail-b")
	var battle_b: Array = panel._read_battle_records()
	_expect(battle_b.size() == 1 and str(battle_b[0].get("battle_id", "")) == "tail-b", "rematch battle separation did not preserve both JSONL groups")
	panel._export_battle()
	var export_path := WorkspacePaths.persistent_file("debug/authority-transcript-tail-b.jsonl")
	_expect(FileAccess.file_exists(export_path), "current-battle export was not written beneath the workspace runtime")

	var command_controls := 0
	for button in panel.find_children("*", "Button", true, false):
		if str(button.get_meta("inspection_id", "")).begins_with("battle.command."):
			command_controls += 1
	_expect(command_controls == 0, "read-only transcript panel exposed a gameplay command control")
	_finish("live tail, partial writes, filters, pause/follow, rematches, and export")


func _record(sequence: int, battle_id: String, visibility: String, kind: String, controller: String, summary: String) -> Dictionary:
	return {
		"schema_version": 1,
		"recorder_version": "authority-transcript-v1",
		"recorded_at_utc": "2026-08-02T00:00:00Z",
		"sequence": sequence,
		"battle_id": battle_id,
		"round": 1,
		"segment": "offensive",
		"stage": "planning",
		"kind": kind,
		"actor_id": "seat-b" if controller == "learned_policy" else "seat-a",
		"human_actor_id": "seat-a",
		"model_actor_id": "seat-b",
		"controller": controller,
		"visibility": visibility,
		"summary": summary,
		"details": {"actors": {"seat-a": {"name": "Human Hero"}, "seat-b": {"name": "Learned Rival"}}},
	}


func _append(path: String, value: String) -> void:
	var file := FileAccess.open(path, FileAccess.READ_WRITE)
	if file == null:
		_fail("could not append transcript fixture: %s" % error_string(FileAccess.get_open_error()))
		return
	file.seek_end()
	file.store_string(value)
	file.flush()


func _expect(condition: bool, message: String) -> void:
	if not condition:
		_fail(message)


func _fail(message: String) -> void:
	_failed = true
	push_error("AUTHORITY TRANSCRIPT PANEL: %s" % message)


func _finish(scope: String) -> void:
	if not _failed:
		print("AUTHORITY TRANSCRIPT PANEL: %s passed" % scope)
	quit(1 if _failed else 0)
