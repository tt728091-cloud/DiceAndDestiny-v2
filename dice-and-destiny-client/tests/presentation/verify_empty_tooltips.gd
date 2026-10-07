extends SceneTree
const WRAPPED := preload("res://presentation/battle/wrapped_tooltip.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.size = Vector2i(1280, 720); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var stage := Control.new(); stage.size = Vector2(canvas.size)
	stage.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); canvas.add_child(stage)
	var controls: Array[Control] = [
		preload("res://presentation/battle/tooltip_button.gd").new(),
		preload("res://presentation/battle/tooltip_check_box.gd").new(),
		preload("res://presentation/battle/tooltip_label.gd").new(),
		preload("res://presentation/battle/status_tooltip_label.gd").new(),
		preload("res://presentation/battle/status_icon_strip.gd").StatusCell.new(),
		preload("res://presentation/battle/attack_intent_button.gd").new(),
		BattleCard.new(),
		preload("res://presentation/cards/fanned_hand.gd").new(),
		ActorProfile.new(),
		preload("res://presentation/battle/card_cleanse.gd").new(),
		preload("res://presentation/battle/poison_conversion.gd").new(),
	]
	for control in controls:
		var kind: String = control.get_script().resource_path.get_file()
		if kind.is_empty(): kind = "StatusCell"
		for blank in ["", " ", "\t\n  \r"]:
			control.tooltip_text = blank
			_expect(control.get_tooltip(Vector2.ZERO).is_empty(), kind + " suppresses blank text before popup creation")
			var tooltip = control._make_custom_tooltip(blank)
			_expect(tooltip == null, kind + " does not build empty content")
			if tooltip != null: tooltip.free()
		stage.add_child(control); control.set_process(false)
		_ignore_children(control)
		control.position = Vector2(200, 200); control.custom_minimum_size = Vector2(240, 180); control.size = Vector2(240, 180)
		for frame in 3: await process_frame
		control.mouse_filter = Control.MOUSE_FILTER_STOP
		# Roll uses this same shared button with an empty tooltip.
		if control is Button: control.text = "Roll 2/3"
		for blank in ["", " \t\n "]:
			control.tooltip_text = blank
			await _hover(control)
			_expect(not _has_popup(canvas), kind + " opens no popup on a real blank hover")
			canvas.notify_mouse_exited(); await process_frame
		control.tooltip_text = "Useful information\nComplete rules remain available."
		await _hover(control)
		if DisplayServer.get_name() != "headless":
			_expect(_has_popup(canvas), kind + " still shows populated tooltips")
		canvas.notify_mouse_exited(); control.queue_free(); await process_frame
	print("EMPTY TOOLTIPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _ignore_children(node: Node) -> void:
	for child in node.get_children():
		if child is Control: child.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_ignore_children(child)
func _hover(control: Control) -> void:
	if DisplayServer.get_name() == "headless": return
	canvas.notify_mouse_entered()
	var motion := InputEventMouseMotion.new(); motion.position = Vector2(900, 30); canvas.push_input(motion, true)
	await process_frame
	motion = InputEventMouseMotion.new(); motion.position = control.get_global_rect().get_center(); canvas.push_input(motion, true)
	await create_timer(1.1).timeout
func _has_popup(node: Node) -> bool:
	if node is Popup and node.visible: return true
	for child in node.get_children(true):
		if _has_popup(child): return true
	return false
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("EMPTY TOOLTIPS: " + message)
