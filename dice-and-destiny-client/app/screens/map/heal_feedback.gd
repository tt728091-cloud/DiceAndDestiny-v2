extends Control
## A short flourish for a camp healing roll, played over the camp sheet: a
## stone die tumbles in and lands on the rolled face (with the inn bonus), the
## healed amount pops, the healed cards flash green and fly to the draw pile,
## and a last line says whether the wound is healed or what remains. It never
## takes input and lasts under two seconds per roll; the camp sheet's heal log
## and the journal keep the full record. Several rolls ("Treat every wound")
## play one after another.

const CARD := preload("res://presentation/cards/battle_card.gd")
const DIE := preload("res://presentation/dice/stone_die.gd")
const CHART := preload("res://app/screens/map/map_style.gd")

const ROLL_SECONDS := 0.55
const HOLD_SECONDS := 0.5
const FLY_SECONDS := 0.45
const FADE_SECONDS := 0.25

## What the latest flourish showed: roll, die, bonus, healed, cards, closed,
## remaining (for tests and the inspector).
var last: Dictionary = {}
## Every flourish shown, oldest first.
var shown: Array = []
var _queue: Array = []
var _busy := false

func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	z_index = 60

func active() -> bool:
	return _busy or not _queue.is_empty()

## Queue one flourish per heal. `area` is the global rect to play over (the
## camp sheet), `target` the global point the healed cards fly to (the draw
## pile counter), `names` maps card ids to names.
func play(heals: Array, area: Rect2, target: Vector2, names: Dictionary) -> void:
	for heal in heals: _queue.append({"heal": heal, "area": area, "target": target, "names": names})
	if not _busy: _next()

func _next() -> void:
	if _queue.is_empty():
		_busy = false; return
	_busy = true
	var item: Dictionary = _queue.pop_front()
	await _show(item.heal, item.area, item.target, item.names)
	_next()

func _show(heal: Dictionary, area: Rect2, target: Vector2, names: Dictionary) -> void:
	var die_sides := int(heal.get("die", 3)); var roll := int(heal.get("roll", 1)); var bonus := int(heal.get("bonus", 0))
	var healed := int(heal.get("healed", 0)); var cards: Array = heal.get("cards", []); var closed: bool = heal.get("closed", false)
	last = {"roll": roll, "die": die_sides, "bonus": bonus, "healed": healed, "cards": cards.duplicate(), "closed": closed, "remaining": int(heal.get("remaining", 0)), "method": str(heal.get("method", ""))}
	shown.append(last)
	# The band: a dark ribbon across the middle of the sheet.
	var origin := get_global_transform_with_canvas().affine_inverse()
	var local_area: Rect2 = origin * area
	var band := Panel.new(); band.mouse_filter = Control.MOUSE_FILTER_IGNORE; band.name = "HealBand"
	var style := StyleBoxFlat.new(); style.bg_color = Color(0.05, 0.06, 0.07, 0.88); style.border_color = Color(CHART.GOLD_LEAF, 0.85)
	style.border_width_top = 2; style.border_width_bottom = 2; style.set_corner_radius_all(10)
	band.add_theme_stylebox_override("panel", style)
	var band_size := Vector2(minf(local_area.size.x - 24.0, 640.0), 236.0)
	band.size = band_size; band.position = local_area.get_center() - band_size * 0.5
	band.modulate.a = 0.0; add_child(band)
	var title := Label.new(); title.text = ("Basic aid" if str(heal.get("method", "")) == "basic_aid" else "Bandage") + " · the %s wound" % str(heal.get("source", ""))
	title.add_theme_font_override("font", CHART.serif_bold()); title.add_theme_font_size_override("font_size", 20); title.add_theme_color_override("font_color", Color("e9dcc0"))
	title.position = Vector2(20, 12); title.size = Vector2(band_size.x - 40, 28); title.clip_text = true; band.add_child(title)
	# The die tumbles in from the left and lands on the roll.
	var die := _DieFace.new(); die.sides = die_sides; die.face = 1; die.size = Vector2(110, 110); die.pivot_offset = die.size * 0.5
	die.position = Vector2(-130, 70); band.add_child(die)
	var result := Label.new(); result.add_theme_font_override("font", CHART.serif_bold()); result.add_theme_font_size_override("font_size", 40)
	result.add_theme_color_override("font_color", Color("ffe2a0")); result.position = Vector2(156, 62); result.size = Vector2(250, 52); result.modulate.a = 0.0; band.add_child(result)
	var detail := Label.new(); detail.add_theme_font_override("font", CHART.serif()); detail.add_theme_font_size_override("font_size", 21)
	detail.position = Vector2(156, 116); detail.size = Vector2(250, 84); detail.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART; detail.modulate.a = 0.0; band.add_child(detail)
	var tween := create_tween(); tween.tween_property(band, "modulate:a", 1.0, 0.12)
	tween.parallel().tween_property(die, "position:x", 28.0, ROLL_SECONDS).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	tween.parallel().tween_property(die, "rotation", TAU * 2.0, ROLL_SECONDS).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	# Faces flicker while it rolls.
	var flicker := Timer.new(); flicker.wait_time = 0.06; add_child(flicker)
	flicker.timeout.connect(func(): die.face = 1 + randi() % die_sides; die.queue_redraw())
	flicker.start()
	await tween.finished
	flicker.queue_free()
	die.rotation = 0.0; die.face = roll; die.landed = true; die.queue_redraw()
	result.text = "Rolled %d" % roll
	detail.text = "%sHealed %d\n%s" % ["+%d at the inn · " % bonus if bonus > 0 else "", healed, "The wound is healed!" if closed else "%d damage remains" % int(heal.get("remaining", 0))]
	detail.add_theme_color_override("font_color", Color("9fe0a8") if closed else Color("e9dcc0"))
	var pop := create_tween(); pop.tween_property(die, "scale", Vector2.ONE * 1.22, 0.08); pop.tween_property(die, "scale", Vector2.ONE, 0.1)
	pop.parallel().tween_property(result, "modulate:a", 1.0, 0.12); pop.parallel().tween_property(detail, "modulate:a", 1.0, 0.16)
	# The healed cards appear on the right, flash green and fly to the draw pile.
	var faces: Array = []
	for i in cards.size():
		var face := CARD.new(); face.mouse_filter = Control.MOUSE_FILTER_IGNORE; band.add_child(face)
		face.configure("heal-%d" % i, str(cards[i]), false, false, true)
		face.scale = Vector2.ONE * 0.86; face.position = Vector2(band_size.x - 20 - (cards.size() - i) * 104, 52)
		face.modulate = Color(0.6, 1.7, 0.8, 0.0); face.set_meta("card_id", str(cards[i])); face.tooltip_text = str(names.get(cards[i], cards[i]))
		faces.append(face)
		var flash := create_tween(); flash.tween_property(face, "modulate", Color(0.7, 1.5, 0.8, 1.0), 0.12); flash.tween_property(face, "modulate", Color.WHITE, 0.3)
	await get_tree().create_timer(HOLD_SECONDS).timeout
	if not faces.is_empty():
		var local_target: Vector2 = band.get_global_transform_with_canvas().affine_inverse() * target
		var fly := create_tween().set_parallel(true)
		for face in faces:
			fly.tween_property(face, "position", local_target - Vector2(20, 24), FLY_SECONDS).set_trans(Tween.TRANS_CUBIC).set_ease(Tween.EASE_IN)
			fly.tween_property(face, "scale", Vector2.ONE * 0.18, FLY_SECONDS)
			fly.tween_property(face, "modulate:a", 0.15, FLY_SECONDS)
		await fly.finished
	else:
		await get_tree().create_timer(0.2).timeout
	var out := create_tween(); out.tween_property(band, "modulate:a", 0.0, FADE_SECONDS)
	await out.finished
	band.queue_free()

## A stone die face with pips, in the player's stone colours.
class _DieFace extends Control:
	var sides := 3
	var face := 1
	var landed := false
	func _init() -> void: mouse_filter = Control.MOUSE_FILTER_IGNORE
	func _draw() -> void:
		var colors: Array = DIE.palette("standard_d6", false)
		var r := Rect2(Vector2.ZERO, size)
		var body := StyleBoxFlat.new(); body.bg_color = colors[0]; body.border_color = colors[3]; body.set_border_width_all(3); body.set_corner_radius_all(14)
		body.shadow_color = Color(0, 0, 0, 0.45); body.shadow_size = 6; body.shadow_offset = Vector2(2, 4)
		draw_style_box(body, r)
		if landed:
			var glow := StyleBoxFlat.new(); glow.draw_center = false; glow.border_color = Color(1.0, 0.85, 0.45, 0.8); glow.set_border_width_all(3); glow.set_corner_radius_all(18)
			draw_style_box(glow, r.grow(5))
		var pip := size.x * 0.085
		var c := size * 0.5; var o := size.x * 0.26
		var spots := {1: [c], 2: [c + Vector2(-o, -o), c + Vector2(o, o)], 3: [c + Vector2(-o, -o), c, c + Vector2(o, o)],
			4: [c + Vector2(-o, -o), c + Vector2(o, -o), c + Vector2(-o, o), c + Vector2(o, o)],
			5: [c + Vector2(-o, -o), c + Vector2(o, -o), c, c + Vector2(-o, o), c + Vector2(o, o)],
			6: [c + Vector2(-o, -o), c + Vector2(o, -o), c + Vector2(-o, 0), c + Vector2(o, 0), c + Vector2(-o, o), c + Vector2(o, o)]}
		for spot in spots.get(clampi(face, 1, 6), [c]): draw_circle(spot, pip, colors[4])
		# The die's size in the corner: d3 or d6.
		draw_string(CHART.serif_bold(), Vector2(0, size.y + 18), "d%d" % sides, HORIZONTAL_ALIGNMENT_CENTER, size.x, 15, Color("e9dcc0"))
