extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TIMING := preload("res://presentation/battle/combat_timing.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "curse")
	var base: Dictionary = gateway.start_battle("damage-card-layout", 43)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		root.size = viewport
		for enemies in [1, 2, 3, 4]:
			for large in [false, true]:
				var fixture := base.duplicate(true)
				fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
				fixture.snapshot.actors.erase("goblin-2")
				for i in range(2, enemies + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				if enemies == 3: fixture.snapshot.actors.goblin.definition_id = "drowned_oracle"
				fixture.snapshot.unified_defense = OS.get_environment("DICE_AND_DESTINY_UNIFIED_LAYOUT") == "1"
				fixture.snapshot.segment = "defensive" if fixture.snapshot.unified_defense else "damage_resolution"; fixture.snapshot.stage = "defense_selection" if fixture.snapshot.unified_defense else "damage_reaction"
				var sources: Array = []; var removals: Array = []
				for actor in fixture.snapshot.actors:
					var amounts: Array = [5, 2, 3] if actor == "blade" else [5]
					if large: amounts = [24]
					for group in amounts.size():
						var id: String = actor + "-" + str(group)
						sources.append({"id": id, "target_actor_id": actor, "source_actor_id": "blade" if actor != "blade" else "goblin", "source_content_id": "hexbrand" if actor != "blade" else "brine_lash", "base_amount": amounts[group], "final_amount": amounts[group]})
						for index in amounts[group]:
							removals.append({"card_id": id + "-" + str(index), "card_definition_id": ["black_dividend", "curse_bloom", "three_knocks", "grave_interest"][index % 4], "target_actor_id": actor, "original_zone": ["discard", "deck", "hand"][index % 3], "accepted": true, "released": false, "damage_proposal_ids": [id]})
				removals.append({"card_id": "saved", "card_definition_id": "curse_bloom", "target_actor_id": "blade", "accepted": true, "released": true})
				fixture.snapshot.settled_damage = {"id": "stacks", "sources": sources, "removals": removals}
				if fixture.snapshot.unified_defense: fixture.snapshot.damage_sources = sources.duplicate(true)
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("damage-stacks.json"))
				root.add_child(screen); screen.set_process(false)
				# Let the opening presentation settle before exercising real hover input.
				await create_timer(0.75).timeout
				for frame in 12: await process_frame
				var total := 0
				_expect(screen._damage_stack_docks.size() == enemies + 1, "one area under every actor")
				_expect(screen._damage_grids.size() == sources.size(), "separate source stacks")
				for source in sources:
					var panel: Control = screen._attack_intents[str(source.id)]
					if str(source.source_actor_id) == screen.viewer_actor_id:
						_expect(not panel.intent.visible, "outgoing damage has no duplicate floating badge")
						_expect(is_instance_valid(panel.target_heading) and panel.target_heading.is_visible_in_tree(), "outgoing attack remains beneath its recipient")
						_expect(panel.target_heading.text.contains(str(panel.data.attack_name)), "recipient heading retains attack name")
						_expect(panel.target_heading.tooltip_text == panel._ability_tooltip(), "recipient heading retains full live attack rules")
						_expect(screen.attack_anchor_rect(str(source.id)) == panel.target_heading.get_global_rect(), "outgoing effect targets follow recipient heading")
						_expect(panel.attack_origin == panel.target_heading, "attack origins follow visible outgoing heading")
					else:
						_expect(panel.intent.visible, "incoming enemy attack badges remain available for defense")
				for grid in screen._damage_grids:
					grid.started_ms = Time.get_ticks_msec() - 5000; grid.refresh_playback()
					total += grid.get_child_count()
					for i in grid.get_child_count():
						var card: BattleCard = grid.get_child(i)
						_expect(card.scale == Vector2.ONE and card.position.y == i * grid.STRIDE, "headers remain full size and evenly stacked")
						_expect(card.instance_id.begins_with(grid.source_id), "card remains with its source")
						_expect(card.instance_id != "saved", "saved cards excluded")
						_expect(card.get_meta("removal_origin_zone") == ["discard", "deck", "hand"][i % 3], "origin icon uses the revealed source zone")
						var icon: Rect2 = grid.origin_icon_rect(card)
						_expect(icon.position.x >= 0 and icon.end.x < card.position.x and is_equal_approx(icon.get_center().y, card.position.y + grid.STRIDE / 2), "source symbol fits immediately left of each card header")
						_expect(card.position.x + card.size.x <= grid.dock.column_width + 1, "source gutter preserves the card column width")
				_expect(total == removals.size() - 1, "all accepted cards retained")
				for actor in screen._damage_stack_docks:
					var dock: Control = screen._damage_stack_docks[actor]
					_expect(root.get_visible_rect().grow(1).encloses(dock.get_global_rect()), "stack area fits viewport")
					_expect(dock.get_global_rect().end.y < screen._actor_profiles[actor].get_global_rect().position.y if actor == "blade" else dock.get_global_rect().position.y >= screen._actor_profiles[actor].get_global_rect().end.y, "stack stays beside its owner: above player stats, below enemy stats")
					for other in screen._damage_stack_docks:
						if other != actor: _expect(not dock.get_global_rect().intersects(screen._damage_stack_docks[other].get_global_rect()), "actor areas do not overlap")

				# At rest, the first five complete card headers fit on both sides.
				for actor in screen._damage_stack_docks:
					var grids: Array = screen._damage_grids.filter(func(g): return g.target_actor == actor)
					var first: Control = grids[0]
					for index in 5:
						var card: Control = first.get_child(index)
						var visible_row: Rect2 = first.visible_card_rect(card)
						_expect(visible_row.size.y >= first.STRIDE * screen._root.scale.y - 1, "five full pending card rows visible for %s at %s / %d enemies: row %d, %s" % [actor, viewport, enemies, index, visible_row])

				if large:
					var dock: ScrollContainer = screen._damage_stack_docks.blade
					dock.scroll_vertical = 99999
					await process_frame
					var grid: Control = screen._damage_grids[0]
					_expect(grid.visible_card_rect(grid.get_child(-1)).has_area(), "scroll reaches last card in a full-deck stack")
					await _check_preview(screen, grid, grid.get_child(-1))
				if not large:
					await _capture("damage-stacks-" + str(enemies))
					var grid: Control = screen._damage_grids[0]
					var header: Rect2 = grid.visible_card_rect(grid.get_child(0))
					var icon_event := InputEventMouseMotion.new(); icon_event.position = grid.get_global_transform_with_canvas() * grid.origin_icon_rect(grid.get_child(0)).get_center(); root.push_input(icon_event, true)
					# The first native window-focus event can replace a synthetic motion.
					await process_frame; root.push_input(icon_event, true); grid._update_hover()
					_expect(grid.tooltip_text == "Pulled from discard pile", "source icon hover identifies its recorded pile %s/%d: %s" % [viewport, enemies, grid.tooltip_text])
					var event := InputEventMouseMotion.new(); event.position = header.get_center(); root.push_input(event, true)
					await process_frame; grid._update_hover()
					_expect(grid.hovered_id == grid.get_child(0).instance_id, "hover exposes full readable card")
					await _check_preview(screen, grid, grid.get_child(0))
					await _capture("damage-stacks-hover-%dx%d-%d" % [viewport.x, viewport.y, enemies])
					# Every row, both sides and the rightmost enemy, retains a clear list.
					for other_grid in screen._damage_grids:
						for card in other_grid.get_children():
							await _check_preview(screen, other_grid, card)
					if enemies == 4: await _capture("damage-stacks-hover-edge-%dx%d" % [viewport.x, viewport.y])
					event = InputEventMouseMotion.new(); event.position = header.end + Vector2(40, 0); root.push_input(event, true)
					await process_frame; grid._update_hover()
					_expect(grid.hovered_id.is_empty(), "moving off header resets stack")
					for item in screen._damage_grids:
						item.removal_started_ms = Time.get_ticks_msec() - int(TIMING.removal() * 500)
						item.refresh_playback(); item.set_process(false)
						_expect(item._tear != null and item.get_child(0).modulate.a == 0, "committed stack tears rather than fading intact")
						var profile: Control = screen._actor_profiles[item.target_actor]
						var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
						_expect(item._tear.endpoint.is_equal_approx(inverse * profile.anchor_rect("removed").get_center()), "trail targets exact actor Remove counter")
					if enemies == 4: await _capture("damage-stacks-tearing")
					var item: Control = screen._damage_grids[0]
					var profile: Control = screen._actor_profiles[item.target_actor]
					profile.position += Vector2(25, -12)
					item._tear.present_progress(0.5)
					var inverse: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse()
					_expect(item._tear.endpoint.is_equal_approx(inverse * profile.anchor_rect("removed").get_center()), "trail follows a moved profile without cached coordinates")
				screen.active_store.clear(); screen.queue_free(); await process_frame
	print("DAMAGE CARD LAYOUT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _check_preview(screen: Control, grid: Control, card: Control) -> void:
	# The compact actor lanes intentionally scroll; every reserved card must
	# still be reachable and previewable, including later source groups.
	var local_y: float = grid.position.y + grid.get_parent().position.y + card.position.y
	grid.dock.scroll_vertical = int(local_y)
	await process_frame; await process_frame
	var header: Rect2 = grid.visible_card_rect(card)
	var event := InputEventMouseMotion.new(); event.position = header.get_center(); root.push_input(event, true)
	await process_frame; grid._update_hover()
	_expect(grid.hovered_id == card.instance_id and is_instance_valid(grid._hover_card), "each row opens its own preview: %s -> %s at %s; hit %s" % [card.instance_id, grid.hovered_id, root.size, root.gui_get_hovered_control()])
	if not is_instance_valid(grid._hover_card): return
	var preview: Rect2 = grid._hover_card.get_global_rect()
	_expect(root.get_visible_rect().grow(1).encloses(preview), "full preview remains inside viewport")
	var list_rect: Rect2 = grid.dock.get_global_rect()
	if list_rect.end.x + preview.size.x + 12 * screen._root.scale.x <= screen._root.get_global_rect().end.x - 16 * screen._root.scale.x:
		_expect(preview.position.x > list_rect.end.x, "preview appears to the right of the list when space permits")
	else:
		_expect(preview.end.x < list_rect.position.x, "right-edge list uses a clear left-side preview")
	for other in screen._damage_grids:
		var occupied: Rect2 = other.get_global_rect().intersection(other.dock.get_global_rect())
		_expect(not preview.intersects(occupied), "preview never covers this or another damage list")

func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_DAMAGE_LAYOUT_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await create_timer(0.1).timeout; RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(directory.path_join(name + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("DAMAGE CARD LAYOUT: " + message)
