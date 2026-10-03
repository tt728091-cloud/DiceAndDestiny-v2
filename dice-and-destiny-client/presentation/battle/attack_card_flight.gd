extends Control
## Card prevention resolves against live source IDs, including after redraws.
var screen: Control
var feedback: Dictionary
var card: BattleCard
var caption: Label
var progress := 0.0
var endpoints: Array[Vector2] = []
var trail_origin := Vector2.ZERO
func configure(owner_screen: Control, value: Dictionary) -> void:
	screen = owner_screen; feedback = value.duplicate(true)
	mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 25
	process_priority = 10 # Apply the card counter after the underlying defense clock.
	card = BattleCard.new(); add_child(card)
	card.configure(str(value.get("instance_id", "")), str(value.get("card_id", "")), false, false, true)
	card.size = Vector2(132, 178); card.mouse_filter = Control.MOUSE_FILTER_IGNORE
	card.position = Vector2(560, 430)
	caption = Label.new(); add_child(caption); caption.text = str(value.text)
	caption.position = Vector2(560, 615); caption.size = Vector2(310, 60); caption.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	caption.add_theme_font_size_override("font_size", 16)
	preload("res://presentation/battle/cinematic_theme.gd").hud_lettering(caption)
	caption.set_meta("inspection_id", "battle.reaction_card_feedback")
func _process(_delta: float) -> void:
	if Time.get_ticks_msec() >= int(feedback.get("expires_ms", 0)): queue_free(); return
	var elapsed := maxf(0, (Time.get_ticks_msec() - int(feedback.get("started_ms", int(feedback.expires_ms) - 2800))) / 1000.0)
	progress = clampf((elapsed - 0.25) / 0.8, 0, 1)
	modulate.a = 1.0 - clampf((elapsed - 2.0) / 0.5, 0, 1)
	trail_origin = card.position + card.size * 0.5
	# Damage-response feedback already owns a played card and saved-card flights.
	# Reuse that visible card as the origin instead of displaying a duplicate.
	for child in screen._root.get_children():
		if child.get_script() == preload("res://presentation/battle/damage_response_feedback.gd"):
			card.hide(); caption.hide()
			trail_origin = get_global_transform_with_canvas().affine_inverse() * child._played.get_global_rect().get_center()
	for notice in screen.get_children():
		if notice.get_script() != preload("res://presentation/battle/card_gain_notice.gd"): continue
		for played in notice._cards:
			if played.instance_id == str(feedback.get("instance_id", "")) and played.is_visible_in_tree():
				card.hide(); caption.hide()
				trail_origin = get_global_transform_with_canvas().affine_inverse() * played.get_global_rect().get_center()
	endpoints.clear()
	var inverse := get_global_transform_with_canvas().affine_inverse()
	for change in feedback.get("changes", []):
		var panel: Control = screen._attack_intents.get(str(change.source_id))
		if not is_instance_valid(panel): continue
		endpoints.append(inverse * screen.attack_anchor_rect(str(change.source_id)).get_center())
		# Presentation only; the authority has already resolved the card.
		if progress < 1: panel.damage.text = str(change.before)
		else: panel.damage.text = str(change.after)
	queue_redraw()
func _draw() -> void:
	if progress <= 0 or progress >= 1: return
	var start := trail_origin
	for endpoint in endpoints:
		var points := PackedVector2Array()
		for step in 24:
			var t := lerpf(maxf(0, progress - 0.35), progress, step / 23.0)
			points.append(start.lerp(endpoint, t) - Vector2(0, sin(t * PI) * 60))
		draw_polyline(points, Color("b4f28f"), 3, true)
		draw_circle(points[-1], 5, Color("deffbf"))
