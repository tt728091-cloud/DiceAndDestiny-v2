extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
const GATHER := preload("res://presentation/battle/effects_gather.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	root.size = Vector2i(1280, 720)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary
	var action := {}
	for seed_value in range(1, 31):
		base = gateway.start_battle("grave-status-%d" % seed_value, seed_value)
		for candidate in base.get("legal_actions", []):
			if candidate.get("type") != "planning_commit_cards": continue
			var ids: Array = candidate.payload.get("card_ids", [])
			if not ids.is_empty() and base.snapshot.actors.blade.card_instances[ids[0]].definition_id == "grave_interest": action = candidate; break
		if not action.is_empty(): break
	_expect(not action.is_empty(), "native Grave Interest card is playable")
	if action.is_empty(): quit(1); return
	base.events = []; base.learned_policy = {}
	var screen = _screen(base, gateway)
	await process_frame
	screen._send(JSON.stringify(action))
	var notices: Array = screen.get_children().filter(func(child): return child.get_script() == NOTICE and not child.is_queued_for_deletion())
	_expect(notices.size() == 1, "real card play produces normal status gain feedback")
	_expect("Grave Interest ×1" in screen._actor_profiles.goblin.statuses.text, "status lives in normal enemy status list")
	_expect(not screen._curse_preparation_labels.has("goblin:grave_interest"), "no floating Prepared label")
	if not notices.is_empty():
		var notice = notices[0]; notice.set_process(false); notice._started = true; notice._elapsed = 0.95; notice.refresh()
		_expect("+1 Grave Interest" in notice._label.text and notice._cards[0].definition_id == "grave_interest", "card is connected to named status gain")
		for frame in 8: await process_frame
		notice._elapsed = 1.4; notice.refresh()
		await _capture("application")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	for width in [1280, 1920]:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		for count in [2, 3, 4, 7]:
			var before := {"health": 11, "energy": 0, "deck_count": 8, "hand_count": 1, "discard_count": 2, "removed_count": 5, "statuses": [{"definition_id": "curse_count", "stacks": count}, {"definition_id": "grave_interest", "stacks": 1}]}
			var after := before.duplicate(true); after.statuses = []
			if count % 3 > 0: after.statuses.append({"definition_id": "curse_count", "stacks": count % 3})
			if count >= 3: after.statuses.append({"definition_id": "grave_debt", "stacks": 5})
			var summary := {"actors_before": {"goblin": before}, "actors_after": {"goblin": after}, "steps": [{"type": "curse_resolved", "actor_id": "goblin", "data": {"kind": "grave_interest_trigger", "triggered": count >= 3, "count_spent": 3 if count >= 3 else 0, "debt": 5 if count >= 3 else 0}}]}
			if count == 7:
				after.health -= 1; after.deck_count -= 1; after.removed_count += 1
				summary.steps.append({"type": "damage_committed", "data": {"sources": [{"id": "loss", "target_actor_id": "goblin", "source_content_id": "curse_count", "final_amount": 1}], "removals": [{"card_id": "lost", "card_definition_id": "brine_surge", "original_zone": "deck", "target_actor_id": "goblin", "accepted": true, "damage_proposal_ids": ["loss"]}]}})
			var fixture := base.duplicate(true); fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
			fixture.events = [{"sequence": 1000, "type": "effects_resolved", "segment": "ongoing_effects", "round": 3, "data": summary}]
			fixture.snapshot.segment = "income"; fixture.snapshot.round = 3
			fixture.snapshot.actors.goblin.statuses = after.statuses
			screen = _screen(fixture, BattleGateway.new(FakeBattleAuthority.new())); await process_frame
			var panel = screen._effects_panel; panel.set_process(false)
			_expect(panel._grave_labels.size() == 1, "one trigger visual")
			var gathers: Array = screen._root.get_children().filter(func(child): return child.get_script() == GATHER)
			_expect(not gathers.is_empty() and gathers[0].badges.has("goblin:grave_interest"), "status animates from profile into Effects")
			panel.resume_at(1.5); panel.present_progress()
			var cue: Label = panel._grave_labels[0].label
			_expect(("3 Curse Count consumed" in cue.text and "Grave Debt · −5 Energy next Income" in cue.text) if count >= 3 else "Expired · no Energy penalty" in cue.text, "trigger or expiry is explicit")
			for frame in 6: await process_frame
			_expect(root.get_visible_rect().encloses(cue.get_global_rect()), "trigger fits viewport")
			await _capture("effects-%d-%d" % [count, width])
			panel.resume_at(panel.duration); panel.present_progress()
			_expect(not "Grave Interest" in screen._actor_profiles.goblin.statuses.text, "consumed status leaves profile")
			_expect(("Grave Debt ×5" in screen._actor_profiles.goblin.statuses.text) == (count >= 3), "debt becomes normal status only when triggered")
			screen.active_store.clear(); screen.queue_free(); await process_frame
	# Public income events carry actual zero gain, so the UI cannot invent a gain.
	var income := base.duplicate(true); income.learned_policy = {}; income.legal_actions = []; income.pending_input = {}
	income.snapshot.segment = "income"; income.snapshot.round = 4; income.snapshot.actors.goblin.energy_points = 0
	income.events = [{"sequence": 2000, "type": "segment_entered", "segment": "income", "round": 4}, {"sequence": 2001, "type": "energy_points_gained", "segment": "income", "round": 4, "actor_id": "goblin", "energy_points": 0, "data": {"energy_gain": 0, "status_id": "grave_debt", "energy_prevented": 5}}]
	screen = _screen(income, BattleGateway.new(FakeBattleAuthority.new())); await process_frame
	var profile = screen._actor_profiles.goblin
	_expect(profile._income_start_values.energy == 0, "income starts at real zero rather than minus one")
	_expect("Grave Debt −5" in profile._income_markers.energy.text and "+0 Energy · consumed" in profile._income_markers.energy.text, "income explains penalty and consumption")
	await _capture("income")
	await create_timer(2.5).timeout
	_expect(not is_instance_valid(profile) or not profile._income_markers.energy.visible, "income feedback cleans up")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("GRAVE INTEREST STATUS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _screen(result: Dictionary, gateway):
	var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = gateway; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("grave-interest-status.json")); root.add_child(screen); return screen
func _capture(label: String) -> void:
	var dir := OS.get_environment("DICE_AND_DESTINY_GRAVE_SCREENSHOTS")
	if not dir.is_empty() and DisplayServer.get_name() != "headless":
		for frame in 6: await process_frame
		await RenderingServer.frame_post_draw; root.get_texture().get_image().save_png(dir.path_join(label + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("GRAVE INTEREST STATUS: " + message)
