extends Node
## Payment feedback is nonblocking and survives battle HUD rebuilds.
const DURATION := 0.6
var screen: Control
var data: Dictionary
var elapsed := 0.0
var battle_id := ""
var _label: Label
func configure(owner_screen: Control, change: Dictionary) -> void:
	screen = owner_screen; data = change; battle_id = screen._view.battle_id
	process_priority = 30 # After resource-gain baselines have been presented.
func _process(delta: float) -> void:
	if screen._history_review or screen._snapshot_panel_open: return
	elapsed += delta
	refresh()
func refresh() -> void:
	if not is_instance_valid(screen) or screen._view.battle_id != battle_id:
		finish(); return
	var profile: ActorProfile = screen._actor_profiles.get(str(data.target_actor_id))
	if not is_instance_valid(profile): return
	if int(profile._display_values.energy) != int(data.expected):
		finish(); return
	_label = profile._stat_labels.energy
	_label.set_meta("energy_fade_owner", get_instance_id())
	profile.show_energy_fade(int(data.before), int(data.after), elapsed / DURATION)
	if elapsed >= DURATION: finish()
func finish() -> void:
	if is_instance_valid(_label) and _label.get_meta("energy_fade_owner", 0) == get_instance_id(): _label.self_modulate.a = 1.0
	queue_free()
func _exit_tree() -> void:
	if is_instance_valid(_label) and _label.get_meta("energy_fade_owner", 0) == get_instance_id(): _label.self_modulate.a = 1.0
