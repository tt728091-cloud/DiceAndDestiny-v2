extends Control

signal finished

const TIMING := preload("res://presentation/battle/combat_timing.gd")
var duration := 3.2
var _card: BattleCard
var _profile: ActorProfile
var _target: Label
var _status_id := ""
var _before := 0
var _started_ms := 0
var _running := false
var _progress := 0.0
var _settled := false
var _enemy := true

func configure(actor_name: String, card_id: String, status_id: String, before: int, after: int, enemy: bool) -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	z_index = 20
	set_process(false)
	duration = maxf(0.5, TIMING.seconds("card_cleanse_seconds", 3.2))
	_status_id = status_id; _before = before; _enemy = enemy
	var data := BattlePresentationCatalog.card(card_id)
	var status := BattlePresentationCatalog.status(status_id)
	tooltip_text = "%s played %s: %s  %d → %d" % [actor_name, data.name, status.name, before, after]
	_card = BattleCard.new(); _card.name = "PlayedCleanseCard"
	_card.configure("cleanse-presentation", card_id, false, false, true)
	_card.custom_minimum_size = Vector2(145, 195); _card.size = _card.custom_minimum_size
	_card.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_card)
	# The enemy card sits immediately left of its profile; mirror for the player.
	_card.position = Vector2(1345, 25) if enemy else Vector2(448, 25)

func prepare(profile: ActorProfile) -> void:
	_profile = profile
	_target = profile.prepare_card_cleanse(_status_id, _before)

func animate(started_ms: int) -> void:
	_started_ms = started_ms
	_running = true
	set_process(true)
	_process(0.0)

func _process(_delta: float) -> void:
	if not _running or not is_instance_valid(_target): return
	_progress = clampf((Time.get_ticks_msec() - _started_ms) / (duration * 1000.0), 0.0, 1.0)
	_card.modulate.a = clampf(_progress / 0.10, 0.0, 1.0) * clampf((1.0 - _progress) / 0.16, 0.0, 1.0)
	# Read the card, follow the trail, then remove just the affected stacks.
	_target.modulate.a = 1.0 - clampf((_progress - 0.43) / 0.27, 0.0, 1.0)
	if _progress >= 0.70 and not _settled:
		_settled = true
		_profile.finish_card_cleanse(_target)
	queue_redraw()
	if _progress >= 1.0:
		_running = false; set_process(false); finished.emit()

func _draw() -> void:
	if not is_instance_valid(_target) or _progress < 0.23 or _progress >= 0.70: return
	var inverse := get_global_transform_with_canvas().affine_inverse()
	var start := _card.position + Vector2(_card.size.x if _enemy else 0.0, _card.size.y * 0.55)
	var bounds := _target.get_global_rect()
	var target := inverse * (bounds.position + Vector2(8, bounds.size.y * 0.5))
	var tip := start.lerp(target, clampf((_progress - 0.23) / 0.20, 0.0, 1.0))
	var alpha := clampf((0.70 - _progress) / 0.14, 0.0, 1.0)
	draw_line(start, tip, Color(0.55, 0.94, 0.48, alpha * 0.18), 9.0, true)
	draw_line(start, tip, Color(0.69, 1.0, 0.58, alpha), 2.0, true)
	draw_circle(tip, 4.0, Color(0.80, 1.0, 0.68, alpha))
