extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas); canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "adventurer")
	var base: Dictionary = gateway.start_battle("wound-health-bar", 43)
	for viewport in [Vector2i(1280,720), Vector2i(1920,1080), Vector2i(1024,768)]:
		canvas.size = viewport
		for count in [1, 4]:
			var fixture := base.duplicate(true); fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
			fixture.snapshot.actors.erase("goblin-2")
			for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
			fixture.snapshot.wounds = []
			for actor_id in fixture.snapshot.actors:
				var actor: Dictionary = fixture.snapshot.actors[actor_id]
				actor.current_health = actor.max_health - 6; actor.removed_count = 6
				for index in 2:
					fixture.snapshot.wounds.append({"id": "%s-wound-%d" % [actor_id,index], "target_actor_id": actor_id, "round": index + 1, "segment": "defensive", "source_content_id": "brine_lash" if actor_id == "blade" else "adventurer_strike", "cards": [
						{"card_id": "%s-%d-a" % [actor_id,index], "card_definition_id": "brace" if actor_id == "blade" else "brine_surge", "original_zone": "hand"},
						{"card_id": "%s-%d-b" % [actor_id,index], "card_definition_id": "nudge" if actor_id == "blade" else "brine_surge", "original_zone": "deck"},
						{"card_id": "%s-%d-c" % [actor_id,index], "card_definition_id": "take_stock" if actor_id == "blade" else "brine_surge", "original_zone": "discard"}]})
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("wound-bars.json"))
			canvas.add_child(screen); screen.set_process(false)
			await _settle()
			for actor_id in screen._actor_profiles:
				var profile = screen._actor_profiles[actor_id]
				var segments: Array = profile.wound_bar.segments
				_expect(segments.size() == 2, "two distinct hits for " + actor_id)
				_expect(segments[0].get_theme_stylebox("normal").bg_color != segments[1].get_theme_stylebox("normal").bg_color, "adjacent wounds have different muted colors")
				for segment in segments:
					_expect(segment.visible and profile.health.get_global_rect().encloses(segment.get_global_rect()), "wound stays inside health bar")
					_expect(absf(segment.size.x - (profile.health.size.x - 10) * 3 / profile.health.max_value) < 0.1, "each wound occupies exactly three health units")
					await _hover(segment)
					_expect(canvas.gui_get_hovered_control() == segment, "pointer reaches " + actor_id + " wound")
					_expect("3 damage" in segment.tooltip_text and "Round" in segment.tooltip_text and ("Brace" if actor_id == "blade" else "Brine Surge") in segment.tooltip_text, "hover describes wound and exact lost cards")
					var tooltip: Control = segment._make_custom_tooltip(segment.tooltip_text); screen._root.add_child(tooltip)
					await _settle(); _expect(tooltip.size.x <= 440 and tooltip.size.y < viewport.y, "tooltip wraps within viewport"); tooltip.queue_free()
				# The final snapshot can arrive before damage playback. No future
				# wound or tooltip may leak into the still-healthy part of the bar.
				profile.health.value = profile.health.max_value
				_expect(segments.all(func(segment): return not segment.visible), "full health conceals queued wounds")
				profile.health.value = profile.health.max_value - 3
				_expect(segments[0].visible and not segments[1].visible, "only already-presented hit is visible")
				profile.health.value = profile.health.max_value - 6
				_expect(segments.all(func(segment): return segment.visible), "both wounds appear after both hits")
			await _settle()
			if DisplayServer.get_name() != "headless":
				RenderingServer.force_draw(false); canvas.get_texture().get_image().save_png("res://.godot/layout-review/wound-bars-%d-%d.png" % [viewport.x,count])
			# Rebuilds/resume use the ledger without duplicating wounds.
			screen._render(); await _settle()
			_expect(screen._actor_profiles.blade.wound_bar.segments.size() == 2, "redraw retains separate wound identities")
			screen.queue_free(); await process_frame
	# Older saves must keep unrecorded loss dark rather than inventing a hit.
	var profile := preload("res://presentation/battle/actor_profile.gd").new(); canvas.add_child(profile)
	profile.display("blade", {"current_health": 6, "max_health": 12}, true); await _settle()
	_expect(profile.wound_bar.segments.is_empty(), "missing historical records do not fabricate wounds")
	profile.queue_free(); await process_frame
	print("WOUND HEALTH BAR: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _settle() -> void:
	for frame in 6: await process_frame
func _hover(control: Control) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = control.get_global_rect().get_center(); canvas.push_input(motion,true)
	await _settle()
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("WOUND HEALTH BAR: " + message)
