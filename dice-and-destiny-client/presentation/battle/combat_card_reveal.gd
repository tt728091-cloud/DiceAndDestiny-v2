extends "res://presentation/cards/damage_card_grid.gd"
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var started_ms := 0
var removal_started_ms := 0

func _process(_delta: float) -> void:
	refresh_playback()

func refresh_playback() -> void:
	var elapsed := maxf(0.0, (Time.get_ticks_msec() - started_ms) / 1000.0)
	for i in get_child_count():
		var card: Control = get_child(i)
		# A bounded stagger keeps even a large damage batch readable on time.
		var delay := 0.25 * float(i) / maxi(1, get_child_count() - 1)
		var progress := clampf((elapsed - delay) / maxf(0.01, TIMING.reveal()), 0.0, 1.0)
		if removal_started_ms > 0:
			progress *= 1.0 - clampf((Time.get_ticks_msec() - removal_started_ms) / (maxf(0.01, TIMING.removal()) * 1000.0), 0.0, 1.0)
		card.modulate.a = progress
