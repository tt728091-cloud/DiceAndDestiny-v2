extends PanelContainer

const THEME := preload("res://presentation/battle/cinematic_theme.gd")
var screen: Control
var data: Dictionary
var mode := "pending"
var key := ""
var elapsed := 0.0
var duration := 1.7
var finished := false
var die: Label
var outcome: Label
var attack: Label
var status: Label
var _body: VBoxContainer

func configure(owner_screen: Control, check: Dictionary, kind: String, sequence: String) -> void:
	screen = owner_screen; data = check; mode = kind
	key = screen._view.battle_id + ":" + sequence + ":" + kind
	elapsed = float(screen._blind_progress.get(key, 0.0))
	duration = 2.0 if mode == "blind_result" else 1.7
	name = "BlindCheck"; mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(470, 0); size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	add_theme_stylebox_override("panel", THEME.panel(Color("18151ef5"), Color("aa94c7"), 8))
	_body = VBoxContainer.new(); _body.add_theme_constant_override("separation", 12); add_child(_body)
	status = _label("◉ BLIND · " + screen._actor_display_name(str(data.get("actor_id", ""))), 24)
	_label("1–2: cancel selected ability  ·  3–6: attack remains", 16)
	var row := HBoxContainer.new(); row.alignment = BoxContainer.ALIGNMENT_CENTER; _body.add_child(row)
	die = Label.new(); die.custom_minimum_size = Vector2(86, 86); die.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; die.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	die.add_theme_font_size_override("font_size", 34); die.add_theme_stylebox_override("normal", THEME.panel(Color("232321"), Color("b6ad92"), 7)); row.add_child(die)
	outcome = _label("", 24); outcome.custom_minimum_size.y = 64
	attack = _label("", 22); attack.custom_minimum_size.y = 65; attack.add_theme_stylebox_override("normal", THEME.panel(Color("2a2634"), Color("b6ad92"), 5))
	refresh()

func _label(text: String, font_size: int) -> Label:
	var label := Label.new(); label.text = text; label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.add_theme_font_size_override("font_size", font_size); label.add_theme_color_override("font_color", Color("efe5fa")); _body.add_child(label); return label

func _process(delta: float) -> void:
	if mode == "pending" or finished: return
	if screen._history_review or screen._history_replay or screen._snapshot_panel_open: return
	elapsed += delta; screen._blind_progress[key] = elapsed
	refresh()
	if elapsed >= duration:
		finished = true
		if screen._director.peek().get("type") == mode: screen.call_deferred("_advance_beat")

func refresh() -> void:
	var rolling := mode == "blind_roll" and elapsed < 0.9
	var face := 1 + int(elapsed * 21) % 6 if rolling else int(data.get("face", 0))
	die.text = str(face) if face > 0 else "—"
	die.pivot_offset = die.size * 0.5; die.rotation = sin(elapsed * 28) * 0.09 if rolling else 0.0
	var name := str(BattlePresentationCatalog.ability(str(data.get("ability_id", ""))).name)
	attack.text = name + " · SELECTED"
	status.modulate.a = 1.0
	if mode != "blind_result":
		outcome.text = "Rolling Blind check…" if rolling else "Face %d · %s\nReactions may change this result" % [face, "would cancel the ability" if face <= 2 else "would leave the attack active"]
	else:
		var cancelled: bool = data.get("cancelled", false)
		var resolved := elapsed >= 0.65
		outcome.text = "Face %d · resolving Blind…" % face if not resolved else "BLINDED · ATTACK CANCELLED" if cancelled else "BLIND MISSED · ATTACK REMAINS"
		outcome.add_theme_color_override("font_color", Color("dbabff") if cancelled else Color("aae1d4"))
		status.modulate.a = 1.0 - 0.65 * clampf((elapsed - 0.65) / 0.5, 0, 1)
		if resolved:
			attack.text = name + " → No offensive ability" if cancelled else name + " · Still active"
			outcome.text += "\nBlind consumed"
		attack.modulate = Color.WHITE.lerp(Color("aa7cbf") if cancelled else Color("a6ffda"), sin(clampf((elapsed - 0.5) / 1.2, 0, 1) * PI) * 0.65)
	queue_redraw()

func _draw() -> void:
	if not is_instance_valid(die): return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var from: Vector2 = inverse * status.get_global_rect().get_center()
	var target: Vector2 = inverse * die.get_global_rect().get_center()
	var profile: ActorProfile = screen._actor_profiles.get(str(data.get("actor_id", "")))
	if mode == "blind_roll" and is_instance_valid(profile): from = inverse * profile.status_anchor("blind")
	var progress := elapsed / 0.35
	if mode == "blind_result":
		from = target; target = inverse * attack.get_global_rect().get_center(); progress = elapsed / 0.65
	if mode == "blind_result" and elapsed >= 0.65:
		var center: Vector2 = inverse * die.get_global_rect().get_center()
		var pulse := clampf((elapsed - 0.65) / 0.65, 0, 1)
		var tint := Color("d5abff") if data.get("cancelled", false) else Color("a6ffda")
		draw_arc(center, 44 + pulse * 28, 0, TAU, 48, Color(tint, 1.0 - pulse), 3, true)
		if data.get("cancelled", false) and pulse < 1:
			draw_line(center + Vector2(-29, 29), center + Vector2(29, -29), Color(tint, 1.0 - pulse), 5, true)
	if mode == "pending" or progress > 1.2: return
	var tip := from.lerp(target, clampf(progress, 0, 1))
	var color := Color("d5abff")
	draw_line(from, tip, Color(color, 0.2), 8, true); draw_line(from, tip, color, 2, true); draw_circle(tip, 4, color)
