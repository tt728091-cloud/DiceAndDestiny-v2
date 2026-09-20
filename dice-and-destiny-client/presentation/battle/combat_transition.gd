extends Control

# Keep outgoing controls alive above the rebuilt board until their replacements
# have arrived. Coordinates use canvas transforms so window scaling is respected.
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var ghosts: Dictionary = {}
var motions: Array[Tween] = []
var destinations: Array[Dictionary] = []
var persistent_panels: Dictionary = {}
var retained_parts: Array[Dictionary] = []

func finish() -> void:
	for motion in motions:
		if motion.is_valid(): motion.kill()
	motions.clear()
	for item in destinations:
		if is_instance_valid(item.node):
			item.node.modulate.a = 1.0
	destinations.clear()
	for item in retained_parts:
		if is_instance_valid(item.node): item.node.set_indexed(item.property, item.alpha)
	retained_parts.clear()
	for item in persistent_panels.values():
		if is_instance_valid(item.destination): item.destination.self_modulate.a = 1.0
		if is_instance_valid(item.background): item.background.queue_free()
		for part in item.parts:
			if is_instance_valid(part): part.queue_free()
	persistent_panels.clear()
	for item in ghosts.values():
		if is_instance_valid(item.node): item.node.queue_free()
	ghosts.clear()

func capture(board: Control) -> void:
	finish()
	if not is_instance_valid(board): return
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	for node in board.find_children("*", "Control", true, false):
		if not node.has_meta("flow_key") or not node.is_visible_in_tree(): continue
		var rect: Rect2 = node.get_global_rect()
		var original_size: Vector2 = node.size
		var key := str(node.get_meta("flow_key"))
		node.reparent(self, false)
		node.set_anchors_preset(Control.PRESET_TOP_LEFT)
		node.position = get_global_transform_with_canvas().affine_inverse() * rect.position
		node.size = original_size
		node.scale = rect.size / original_size.max(Vector2.ONE)
		node.mouse_filter = Control.MOUSE_FILTER_IGNORE
		node.process_mode = Node.PROCESS_MODE_DISABLED
		for control in node.find_children("*", "Control", true, false): control.mouse_filter = Control.MOUSE_FILTER_IGNORE
		ghosts[key] = {"node": node, "rect": rect}

func _steady_control(key: String) -> bool:
	return key in ["HandDock", "PlayerDice", "EnemyDice", "RollControls", "BattleActionFooter"]

func prepare(board: Control) -> void:
	# Hide destinations before their first draw. Layout is measured next frame;
	# revealing them first would produce exactly the one-frame flash we avoid.
	for node in board.find_children("*", "Control", true, false):
		if not node.has_meta("flow_key"): continue
		var key := str(node.get_meta("flow_key"))
		if _steady_control(key):
			node.modulate.a = 1.0
			if ghosts.has(key): ghosts[key].node.hide()
		else: node.modulate.a = 0.0

func present(board: Control) -> void:
	if not is_instance_valid(board): return
	var duration := TIMING.transition()
	var continued := {}
	for node in board.find_children("*", "Control", true, false):
		if not node.has_meta("flow_key"): continue
		var key := str(node.get_meta("flow_key"))
		var prior: Dictionary = ghosts.get(key, ghosts.get(str(node.get_meta("flow_origin", "")), {}))
		if _steady_control(key):
			node.modulate.a = 1.0
			if ghosts.has(key): ghosts[key].node.hide()
			continued[key] = true
			continue
		if key.begins_with("ability:") and ghosts.has(key):
			_continue_ability(ghosts[key].node, node, duration)
			continued[key] = true
			continue
		if key.begins_with("source:") and ghosts.has(key) and node is PanelContainer:
			_continue_panel(key, ghosts[key].node, node, duration)
			continued[key] = true
			continue
		node.modulate.a = 0.0
		destinations.append({"node": node})
		var tween := create_tween(); motions.append(tween)
		tween.tween_property(node, "modulate:a", 1.0, duration * 0.55).set_delay(duration * 0.45)
		if not prior.is_empty():
			var ghost: Control = prior.node
			var target: Rect2 = node.get_global_rect()
			var motion := create_tween().set_parallel(true); motions.append(motion)
			motion.tween_property(ghost, "position", get_global_transform_with_canvas().affine_inverse() * target.position, duration).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
	for key in ghosts:
		if continued.has(key): continue
		var item: Dictionary = ghosts[key]
		var tween := create_tween(); motions.append(tween)
		tween.tween_property(item.node, "modulate:a", 0.0, duration * 0.65)
		tween.tween_callback(item.node.queue_free)

# Keep the chosen tile opaque while it moves to the top of the rail. Swap its
# recipe for the selected result once, without overlapping two translucent tiles.
func _continue_ability(old: Control, current: Control, duration: float) -> void:
	current.modulate.a = 0.0
	destinations.append({"node": current})
	var motion := create_tween(); motions.append(motion)
	motion.tween_property(old, "position", get_global_transform_with_canvas().affine_inverse() * current.get_global_rect().position, duration).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
	motion.tween_callback(func():
		if is_instance_valid(current): current.modulate.a = 1.0
		if is_instance_valid(old): old.hide()
	)

# A source that survives a stage change keeps an opaque frame and uninterrupted
# common text. Only outgoing/incoming contents fade. The layout can still be
# rebuilt safely by the screen; this visual shell bridges those two layouts.
func _continue_panel(key: String, old: PanelContainer, current: PanelContainer, duration: float) -> void:
	var shell := Panel.new()
	shell.mouse_filter = Control.MOUSE_FILTER_IGNORE
	shell.add_theme_stylebox_override("panel", old.get_theme_stylebox("panel").duplicate())
	# A plain holder keeps Container sizing away from the animated frame. Unlike
	# a top-level CanvasItem, it also draws *behind* the live dice and old labels.
	var background := Control.new(); background.mouse_filter = Control.MOUSE_FILTER_IGNORE
	current.add_child(background); current.move_child(background, 0); background.add_child(shell)
	var from_rect := old.get_global_rect()
	var to_rect := current.get_global_rect()
	var frame_scale := current.get_global_transform_with_canvas().get_scale()
	shell.global_position = from_rect.position; shell.size = from_rect.size / frame_scale
	current.modulate.a = 1.0; current.self_modulate.a = 0.0
	var record := {"shell": shell, "background": background, "destination": current, "parts": []}
	persistent_panels[key] = record
	var geometry := create_tween().set_parallel(true); motions.append(geometry)
	geometry.tween_property(shell, "global_position", to_rect.position, duration).from(from_rect.position).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
	geometry.tween_property(shell, "size", to_rect.size / frame_scale, duration).from(from_rect.size / frame_scale).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
	var previous := {}
	for part in old._body.get_children():
		if not part is Control or not part.visible: continue
		var rect := _local_rect(part)
		var original_size: Vector2 = part.size
		part.reparent(self, false)
		part.set_anchors_preset(Control.PRESET_TOP_LEFT)
		part.position = rect.position; part.size = original_size
		part.scale = rect.size / original_size.max(Vector2.ONE)
		part.process_mode = Node.PROCESS_MODE_DISABLED
		record.parts.append(part)
		previous[str(part.get_meta("flow_part", str(part.get_instance_id())))] = part
	old.hide()
	for part in current._body.get_children():
		if not part is Control or not part.visible: continue
		var part_key := str(part.get_meta("flow_part", ""))
		var prior: Control = previous.get(part_key)
		# Reviewed cards already exist on screen. Hand them directly to their
		# live removal clock, rather than fading them out and revealing again.
		if part_key == "cards" and is_instance_valid(prior) and part.get_meta("flow_content", []) == prior.get_meta("flow_content", []):
			prior.hide(); previous.erase(part_key)
			continue
		# Leaf self-modulation stays independent of the live number animation.
		var property := "self_modulate:a" if part is Label or part is Button else "modulate:a"
		retained_parts.append({"node": part, "property": property, "alpha": part.get_indexed(property)})
		part.set_indexed(property, 0.0)
		var same_text: bool = is_instance_valid(prior) and (part is Label or part is Button) and part.text == prior.text
		if same_text:
			previous.erase(part_key)
			var target := _local_rect(part)
			var motion := create_tween().set_parallel(true); motions.append(motion)
			motion.tween_property(prior, "position", target.position, duration).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
			motion.tween_property(prior, "scale", (get_global_transform_with_canvas().affine_inverse() * part.get_global_transform_with_canvas()).get_scale(), duration).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN_OUT)
		else:
			var reveal := create_tween(); motions.append(reveal)
			reveal.tween_property(part, property, 1.0, duration * 0.2).set_delay(duration * 0.8)
	for part in previous.values():
		var fade := create_tween(); motions.append(fade)
		fade.tween_property(part, "modulate:a", 0.0, duration * 0.5)
	var completion := create_tween(); motions.append(completion)
	completion.tween_interval(duration)
	completion.tween_callback(func():
		if is_instance_valid(current): current.self_modulate.a = 1.0
		for part in current._body.get_children() if is_instance_valid(current) else []:
			if part is Label or part is Button: part.self_modulate.a = 1.0
		if is_instance_valid(background): background.queue_free()
		for part in record.parts:
			if is_instance_valid(part): part.queue_free()
	)

func _local_rect(control: Control) -> Rect2:
	var transform := get_global_transform_with_canvas().affine_inverse() * control.get_global_transform_with_canvas()
	return Rect2(transform * Vector2.ZERO, control.size * transform.get_scale())
