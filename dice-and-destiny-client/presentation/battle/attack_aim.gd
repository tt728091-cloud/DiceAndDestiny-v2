extends Control
## Choosing whom an attack hits: a red line runs from the selected attack to the
## pointer and snaps onto an enemy's name while the pointer is over that enemy.
## Eligible names glow; the aimed one turns red. Board coordinates (under _root).
const RED := Color("ff4636")
const GOLD := Color("ffd76a")
var screen: Control
var ability_id := ""
var targets: Array = []
var hovered := ""

func configure(owner_screen: Control, ability: String, ids: Array) -> void:
	screen = owner_screen; ability_id = ability; targets = ids
	name = "AttackAim"; mouse_filter = Control.MOUSE_FILTER_IGNORE; z_index = 40
	set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)

## Where the line leaves the selected attack, in global coordinates.
func origin() -> Vector2:
	for tile in screen._ability_dock.find_children("*", "Button", true, false):
		if tile is BattleAbilityTile and tile.ability_id == ability_id and tile.is_visible_in_tree():
			var rect: Rect2 = tile.get_global_rect()
			return Vector2(rect.end.x - 6, rect.get_center().y)
	return screen._ability_dock.get_global_rect().get_center()

func name_rect(id: String) -> Rect2:
	var profile: ActorProfile = screen._actor_profiles.get(id)
	return profile.title.get_global_rect() if is_instance_valid(profile) else Rect2()

## The enemy under the pointer: its artwork or its name and health.
func target_at(point: Vector2) -> String:
	for id in targets:
		var profile: ActorProfile = screen._actor_profiles.get(id)
		if not is_instance_valid(profile): continue
		if profile.get_global_rect().has_point(point): return id
		var fighter: Control = profile.get_parent().fighter
		if is_instance_valid(fighter) and fighter.get_global_rect().has_point(point): return id
	return ""

func _process(_delta: float) -> void:
	hovered = target_at(get_global_mouse_position())
	var pulse := (sin(Time.get_ticks_msec() / 1000.0 * TAU / 1.2) + 1.0) * 0.5
	for id in targets:
		var profile: ActorProfile = screen._actor_profiles.get(id)
		if is_instance_valid(profile): profile.title.add_theme_color_override("font_color", RED.lightened(0.2) if id == hovered else GOLD.lerp(Color("fff3cc"), pulse))
	queue_redraw()

func _draw() -> void:
	var inverse := get_global_transform().affine_inverse()
	var aimed := not hovered.is_empty()
	var finish: Vector2 = name_rect(hovered).get_center() if aimed else get_global_mouse_position()
	draw_aim(self, inverse * origin(), inverse * finish, aimed, Time.get_ticks_msec() / 1000.0)

## A shallow arc from the attack to the target.
static func arc(start: Vector2, finish: Vector2, steps: int = 28) -> PackedVector2Array:
	var lift := Vector2(0, -minf(180.0, start.distance_to(finish) * 0.28))
	var control := (start + finish) * 0.5 + lift
	var points := PackedVector2Array()
	for i in steps + 1:
		var t := float(i) / steps
		points.append(start.lerp(control, t).lerp(control.lerp(finish, t), t))
	return points

static func draw_aim(canvas: CanvasItem, start: Vector2, finish: Vector2, aimed: bool, time: float) -> void:
	var points := arc(start, finish)
	var heat := 1.0 if aimed else 0.7
	canvas.draw_polyline(points, Color(0.85, 0.05, 0.02, 0.16 * heat), 18, true)
	canvas.draw_polyline(points, Color(1.0, 0.16, 0.08, 0.42 * heat), 7, true)
	canvas.draw_polyline(points, Color(1.0, 0.62, 0.5, 0.95 * heat), 2.5, true)
	# Embers race along the line toward the target.
	for k in 7:
		var u := fmod(time * 1.5 + k / 7.0, 1.0)
		var point := points[mini(points.size() - 1, int(u * (points.size() - 1)))]
		canvas.draw_circle(point, 4.0 - 2.0 * u, Color(1.0, 0.78, 0.62, 0.9 * (1.0 - u * 0.6) * heat))
	if aimed:
		var radius := 20.0 + 3.0 * sin(time * 9.0)
		canvas.draw_arc(finish, radius, 0, TAU, 40, Color(1.0, 0.22, 0.14, 0.95), 3, true)
		for angle in [0.0, PI * 0.5, PI, PI * 1.5]:
			var dir := Vector2.RIGHT.rotated(angle)
			canvas.draw_line(finish + dir * (radius - 7), finish + dir * (radius + 9), Color(1.0, 0.4, 0.3, 0.95), 3, true)
	else:
		var dir := (points[-1] - points[-3]).normalized()
		var side := dir.orthogonal() * 9.0
		canvas.draw_colored_polygon(PackedVector2Array([finish + dir * 6, finish - dir * 14 + side, finish - dir * 14 - side]), Color(1.0, 0.36, 0.25, 0.9))
