extends SceneTree

func _init() -> void:
	call_deferred("_run")

func _run() -> void:
	OS.set_environment("DICE_AND_DESTINY_ENABLE_SNAPSHOTS", "1")
	OS.set_environment("DICE_AND_DESTINY_ENABLE_HISTORY", "1")
	root.size = Vector2i(1280, 720)
	var gateway := BattleGateway.new()
	var store := ActiveBattleStore.new(WorkspacePaths.persistent_file("verify_snapshot_during_presentation_active.json"))
	store.clear()
	var suffix := "%d-%d" % [Time.get_unix_time_from_system(), Time.get_ticks_usec()]
	var battle_id := "snapshot-presentation-%s" % suffix
	var snapshot_name := "presentation-%s" % suffix
	var started := gateway.start_battle(battle_id, "blade")
	if not _accepted(started, "start presentation snapshot battle"): return

	var screen = load("res://app/screens/battle/battle_screen.tscn").instantiate()
	screen.initial_result = started
	screen.gateway = gateway
	screen.active_store = store
	root.add_child(screen)
	await process_frame; await process_frame
	var before: Dictionary = screen.inspection_state()
	if not before.get("presentation_active", false):
		_fail("new battle did not begin on a capturable presentation beat: %s" % before); return
	var presentation_type := str(before.get("presentation_type", ""))

	var toggle := _button(screen, "DEV SNAPSHOTS")
	if toggle == null: _fail("developer snapshot controls were unavailable"); return
	toggle.pressed.emit(); await process_frame; await process_frame
	var name_edit := _inspection_control(screen, "battle.dev_snapshots.name") as LineEdit
	var capture := _button(screen, "Capture Current Authority State")
	if name_edit == null or capture == null: _fail("snapshot capture controls were incomplete"); return
	name_edit.text = snapshot_name
	name_edit.text_changed.emit(snapshot_name)
	await process_frame
	if capture.disabled: _fail("presentation incorrectly disabled snapshot capture"); return
	capture.pressed.emit(); await process_frame; await process_frame
	if _error_label(screen) != "": _fail("presentation snapshot capture failed: %s" % _error_label(screen)); return

	var load_snapshot := _button(screen, "Load Selected as New Battle")
	if load_snapshot == null or load_snapshot.disabled: _fail("captured presentation snapshot was not loadable"); return
	load_snapshot.pressed.emit(); await process_frame; await process_frame; await process_frame
	var screens: Array[Node] = get_nodes_in_group("inspectable_battle_screen")
	var restored: Node = screens[-1]
	var after: Dictionary = restored.inspection_state()
	if after.get("loaded_snapshot_name") != snapshot_name or not after.get("presentation_active", false) or str(after.get("presentation_type", "")) != presentation_type:
		_fail("loaded snapshot did not resume the exact visible presentation: before=%s after=%s" % [before, after]); return
	var active := store.load_active()
	if active.get("battle_id") == battle_id or active.get("snapshot_name") != snapshot_name or int(active.get("last_sequence", -1)) != 0:
		_fail("loaded presentation cursor was not persisted as the active battle: %s" % active); return

	store.clear()
	restored.queue_free(); await process_frame
	print("SNAPSHOT DURING PRESENTATION: capture enabled and exact visible beat resumed after load")
	quit(0)

func _inspection_control(node: Node, inspection_id: String) -> Control:
	for child in node.find_children("*", "Control", true, false):
		if child.has_meta("inspection_id") and str(child.get_meta("inspection_id")) == inspection_id: return child
	return null

func _button(node: Node, text: String) -> Button:
	for child in node.find_children("*", "Button", true, false):
		if child.text == text: return child
	return null

func _error_label(node: Node) -> String:
	for child in node.find_children("*", "Label", true, false):
		if child.text.begins_with("Error:"): return child.text
	return ""

func _accepted(result: Dictionary, step: String) -> bool:
	if result.get("accepted") == true: return true
	_fail("%s rejected: %s" % [step, JSON.stringify(result)])
	return false

func _fail(message: String) -> void:
	push_error(message)
	quit(1)
