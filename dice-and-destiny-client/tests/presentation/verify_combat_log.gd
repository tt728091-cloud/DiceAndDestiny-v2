extends SceneTree
const VIEW := preload("res://local_client/view_state/battle_view_state.gd")
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1920, 1080)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = gateway.start_battle("combat-log", 1789679833957118)
	initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	var view := VIEW.new(); _expect(view.apply_result(initial), "viewer-safe baseline accepted")
	var update := initial.duplicate(true)
	update.snapshot.actors.goblin.hand_count = 6
	update.snapshot.actors.goblin.deck_count = 14
	update.snapshot.actors.blade.energy_points = 2
	update.snapshot.actors.blade.statuses = [{"definition_id": "catalyst", "stacks": 2}]
	update.snapshot.actors.blade.selected_ability = "needlefang"
	update.events = [
		{"sequence": 1, "type": "cards_drawn", "actor_id": "goblin", "count": 1, "cards": ["ENEMY_SECRET_CARD"], "data": {"card_definition_id": "ENEMY_SECRET_DEFINITION"}},
		{"sequence": 2, "type": "card_played", "actor_id": "blade", "data": {"card_definition_id": "culture_flask"}},
		{"sequence": 3, "type": "dice_rolled", "actor_id": "blade", "dice": [{"face": 1}, {"face": 3}, {"face": 6}]},
		{"sequence": 4, "type": "ability_selected", "actor_id": "goblin", "private_actor_id": "goblin", "data": {"ability_id": "ENEMY_SECRET_ATTACK"}}]
	view.apply_result(update)
	var text := view.combat_log.text()
	for expected in ["drew 1 card", "hand size 6", "Culture Flask", "Catalyst 0 → 2", "Energy 3 → 2", "Needlefang", "rolled 1, 3, 6"]:
		_expect(expected in text, "records visible fact: " + expected)
	_expect(not "SECRET" in text, "enemy identities never enter the log")
	var first_response := VIEW.new(); first_response.apply_result(update)
	_expect("hand size 6" in first_response.combat_log.text(), "initial response draws include final hand size")
	var count: int = view.combat_log.entries.size()
	view.apply_result(update)
	_expect(view.combat_log.entries.size() == count, "repeated cumulative response does not duplicate entries")
	var empty := update.duplicate(true); empty.events = []
	view.apply_result(empty)
	_expect(view.combat_log.entries.size() == count, "empty response preserves prior history")
	# Batched enemy draw cards can change energy while leaving hand size unchanged.
	var focus := empty.duplicate(true)
	focus.snapshot.actors.goblin.energy_points += 1
	focus.events = [{"sequence": 5, "type": "card_played", "actor_id": "goblin", "data": {"card_definition_id": "battle_focus"}}]
	view.apply_result(focus)
	_expect("drew 1 card(s); hand size 6" in view.combat_log.text(), "public hand arithmetic records a draw even when play and draw cancel")
	# Effects contain authoritative before/after state and public lost cards.
	var effect: Dictionary = JSON.parse_string(FileAccess.get_file_as_string("res://tests/fixtures/automatic_effects.json").replace('"player"', '"blade"').replace('"enemy"', '"goblin"'))
	effect.sequence = 100
	var effects := focus.duplicate(true); effects.events = [effect]
	view.apply_result(effects)
	text = view.combat_log.text()
	for expected in ["Poison → Volatile Poison", "Bleed 3 → 2", "Volatile Poison 1 → 2", "spent 1 Catalyst", "took 2 damage", "lost Tip It"]:
		_expect(expected in text, "Effects detail logged: " + expected)
	# A real response includes the same steps before the summary: do not double
	# print its rolls/removals merely because the summary embeds them again.
	var normal := effects.duplicate(true); normal.snapshot.battle_id = "normal-effects"
	normal.events = effect.data.steps.duplicate(true)
	for index in normal.events.size(): normal.events[index].sequence = index + 1
	normal.events.append(effect)
	var other := VIEW.new(); other.apply_result(normal)
	_expect(other.combat_log.text().count("lost Tip It") == text.count("lost Tip It"), "embedded Effects steps do not repeat public losses")
	# An empty ongoing-effects phase serializes its step slice as null.
	var no_effects := effects.duplicate(true)
	no_effects.events = [{"sequence": 101, "type": "effects_resolved", "data": {"steps": null}}]
	var empty_effect_view := VIEW.new()
	_expect(empty_effect_view.apply_result(no_effects), "empty Effects summary is accepted")
	# More than eight events survive; both actors' revealed selections are named.
	var reveal := effects.duplicate(true); reveal.events = []
	for index in 12:
		reveal.events.append({"sequence": 101 + index, "type": "card_played", "actor_id": "goblin", "data": {"card_definition_id": "tip_it"}})
	reveal.events.append({"sequence": 113, "type": "interaction_revealed", "segment": "offensive", "round": 2, "data": {"commitments": {"goblin": {"ability_id": "sword_cut", "dice": [{"face": 3}], "outcome": {"base_damage": 5}}}}})
	view.apply_result(reveal)
	_expect("Culture Flask" in view.combat_log.text() and "revealed Sword Cut" in view.combat_log.text(), "full history and revealed enemy attack remain readable")
	var screen = SCREEN.instantiate(); screen.initial_result = effects; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("combat-log-test.json")); screen._auto_pass_disabled = true; screen._open_utility = "log"
	root.add_child(screen); screen.set_process(false)
	for frame in 5: await process_frame
	_expect("Bleed 3 → 2" in screen._log.text, "regular log retains Effects details instead of a placeholder")
	var panel = screen._effects_panel
	panel.resume_at(6.3 + panel.catalyst_extra_seconds()); panel._process(0); panel.set_paused(true)
	for label in panel.find_children("*", "Label", true, false):
		_expect(" → " not in label.text, "no state-change summary in center")
	var capture := OS.get_environment("DICE_AND_DESTINY_COMBAT_LOG_SCREENSHOT")
	if not capture.is_empty() and DisplayServer.get_name() != "headless":
		await RenderingServer.frame_post_draw
		root.get_texture().get_image().save_png(capture)
	screen.active_store.clear(); screen.queue_free(); await process_frame
	var fresh := initial.duplicate(true); fresh.snapshot.battle_id = "fresh-battle"
	view.apply_result(fresh)
	_expect(view.combat_log.entries.is_empty(), "new battle clears the previous battle log")
	print("COMBAT LOG: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("COMBAT LOG: " + message)
