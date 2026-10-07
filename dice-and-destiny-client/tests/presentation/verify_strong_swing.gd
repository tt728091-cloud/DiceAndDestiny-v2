extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true; canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	for width in [1024, 1280, 1920]:
		canvas.size = Vector2i(width, width * 9 / 16)
		await _scenario(width, "adventurer_strike")
		await _scenario(width, "adventurer_small_straight")
	print("STRONG SWING PREPARATION: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _scenario(width: int, target_id: String) -> void:
	canvas.notify_mouse_entered()
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var result := {}; var swing_id := ""
	for seed_value in range(1, 45):
		result = gateway.start_battle("strong-swing-%d-%d" % [width, seed_value], seed_value)
		for action in result.get("legal_actions", []):
			var ids: Array = action.get("payload", {}).get("card_ids", [])
			if ids.size() == 1 and result.snapshot.actors.blade.card_instances.get(ids[0], {}).get("definition_id") == "strong_swing": swing_id = ids[0]; break
		if not swing_id.is_empty(): break
	_expect(not swing_id.is_empty(), "native Strong Swing offered before rolling")
	if swing_id.is_empty(): return
	var screen = SCREEN.instantiate(); screen.gateway = gateway; screen.initial_result = result; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("strong-swing.json")); canvas.add_child(screen); screen.set_process(false)
	for frame in 6: await process_frame
	screen._director.clear()
	screen._flow_until = 0
	for entry in screen._timed_buttons: entry.until = 0
	screen._process(0)
	var energy_before: int = screen._view.actor("blade").energy_points
	await _click_hand(screen, swing_id)
	for frame in 6: await process_frame
	var prompt = screen.find_child("AbilityCardTargetPrompt", true, false)
	_expect(prompt != null and "Strong Swing" in prompt.text and "+2 damage" in prompt.text, "clear card and bonus targeting prompt")
	_expect(screen._view.actor("blade").energy_points == energy_before, "selection does not spend energy")
	await _click(screen.find_child("CancelAbilityCardTarget", true, false))
	_expect(screen._selected_card.is_empty() and screen._view.actor("blade").energy_points == energy_before, "cancel preserves energy and card")
	await _click_hand(screen, swing_id)
	for frame in 6: await process_frame
	var target = _tile(screen, target_id)
	_expect(target != null and not target.disabled, "unrolled Strike is a selectable card target")
	if target == null: screen.queue_free(); return
	var motion := InputEventMouseMotion.new(); motion.position = target.get_global_rect().get_center(); canvas.push_input(motion, true)
	for frame in 6: await process_frame
	var selected: BattleCard
	for entry in screen._hand_dock.cards:
		if entry.instance_id == swing_id: selected = entry
	_expect(selected != null and selected.button_pressed, "card stays highlighted after pointer leaves hand")
	_expect(is_zero_approx(selected.get_parent().rotation), "selected card stays raised and straightened")
	var requirement: Label = target.find_child("MinimalRequirement", true, false)
	_expect(requirement != null and not "\n" in requirement.text and requirement.get_line_count() == 1, "Strike requirements stay horizontal")
	_expect(target.get_global_rect().encloses(requirement.get_global_rect()), "target requirements stay inside Strike row")
	var pose: Transform2D = screen._root.get_global_transform_with_canvas().affine_inverse() * selected.get_global_transform_with_canvas()
	await _capture("strong-swing-targeting-%d" % width)
	await _click(target)
	var notices := screen.get_children().filter(func(node): return node.get_script() == NOTICE)
	_expect(notices.size() == 1, "confirmed play starts one feedback animation")
	if not notices.is_empty():
		var notice = notices[0]; notice.set_process(false)
		for frame in 4: await process_frame
		notice._started = true; notice._elapsed = notice.duration * 0.4; notice.refresh()
		_expect(notice._cards[0].position.distance_to(pose.origin) < 1, "played card stays at its selected hand position")
		_expect(not notice._label.visible, "no upper-left summary")
		var badge: Control = _tile(screen, target_id).find_child("TemporaryDamageBonus", true, false)
		var inverse: Transform2D = notice.get_global_transform_with_canvas().affine_inverse()
		var ability_point: Vector2 = inverse * badge.get_global_rect().get_center()
		var status_point: Vector2 = inverse * screen._actor_profiles.blade.status_anchor("strong_swing_ready")
		_expect(notice._destinations.any(func(point): return point.distance_to(ability_point) < 1), "arc targets selected ability bonus")
		_expect(notice._destinations.any(func(point): return point.distance_to(status_point) < 1), "arc targets authoritative player status")
		await _capture("strong-swing-arcs-%d" % width)
		notice._elapsed = notice.duration * 0.6; notice.refresh()
		_expect(notice._cards[0].modulate.a < 0.05, "played card fades away")
		notice.queue_free()
	_expect(screen._view.actor("blade").energy_points == energy_before - 1, "confirmed play spends energy once")
	_expect(not swing_id in screen._view.actor("blade").hand, "played card leaves hand")
	await create_timer(2.0).timeout
	for frame in 6: await process_frame
	_expect(screen._error_message.is_empty(), "pointer play accepted: " + screen._error_message)
	screen._director.clear(); screen._render()
	for frame in 6: await process_frame
	_check_marker(screen, target_id)
	await _capture("strong-swing-before-roll-%s-%d" % [target_id, width])
	if target_id == "adventurer_strike":
		await _check_all_bonus_rows(screen)
		await _check_catalog_bonus_rows(screen)
	var roll := {}
	for action in screen._view.legal_actions:
		if action.get("type") == "planning_roll": roll = action; break
	_expect(not roll.is_empty(), "normal first roll still available")
	if not roll.is_empty(): result = gateway.submit(JSON.stringify(roll))
	screen._view.apply_result(result); screen._selected_card.clear(); screen._render()
	for frame in 6: await process_frame
	_check_marker(screen, target_id)
	for action in result.get("legal_actions", []):
		for id in action.get("payload", {}).get("card_ids", []):
			_expect(result.snapshot.actors.blade.card_instances.get(id, {}).get("definition_id") != "strong_swing", "no Strong Swing actions after first roll")
	await _capture("strong-swing-after-roll-%d" % width)
	# Complete the real offensive segment, including AI turns and reaction passes.
	for step in 40:
		if result.snapshot.segment != "offensive": break
		if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model(); continue
		var chosen := {}
		for kind in ["planning_select_ability", "planning_pass", "pass"]:
			for action in result.get("legal_actions", []):
				if action.get("type") == kind: chosen = action; break
			if not chosen.is_empty(): break
		if chosen.is_empty(): break
		result = gateway.submit(JSON.stringify(chosen))
	_expect(result.snapshot.segment != "offensive", "native offensive segment completes")
	screen._view.apply_result(result); screen._render(true)
	await create_timer(2.0).timeout
	for frame in 4: await process_frame
	_expect(int(screen._actor_profiles.blade.statuses.counts.get("strong_swing_ready", 0)) == 0 and (not screen._actor_profiles.blade.statuses.cells.has("strong_swing_ready") or screen._actor_profiles.blade.statuses.cells.strong_swing_ready.modulate.a == 0.0), "status expires on offensive exit")
	_expect(screen.find_children("TemporaryDamageBonus", "Label", true, false).is_empty(), "temporary marker expires with status")
	canvas.notify_mouse_exited(); screen.queue_free(); await process_frame
func _check_all_bonus_rows(screen) -> void:
	var actor: Dictionary = screen._view.actor("blade")
	var original: Array = actor.ability_modifiers.duplicate(true)
	var ids: Array[String] = []
	for tile in screen._ability_dock.find_children("*", "Button", true, false):
		if tile is BattleAbilityTile: ids.append(tile.ability_id)
	_expect(ids.size() == actor.offensive_abilities.size(), "exercise every offensive ability, including nested rows")
	for id in ids:
		actor.ability_modifiers[0].ability_id = id
		screen._render()
		for frame in 5: await process_frame
		var tile = _tile(screen, id)
		var badge: Control = tile.find_child("TemporaryDamageBonus", true, false)
		var title: Label = tile.find_child("MinimalTitle", true, false)
		_expect(title.get_line_count() == 1, "ability name never wraps vertically: " + id)
		_expect(tile.get_global_rect().encloses(badge.get_global_rect()), "bonus fits " + id)
		_expect(tile.get_global_rect().encloses(title.get_global_rect()), "title fits " + id)
		_expect(tile.custom_minimum_size.y <= 56, "bonus uses at most one extra line for " + id)
		_expect(not title.get_global_rect().intersects(badge.get_global_rect()), "bonus does not overlap name for " + id)
		await _capture("strong-swing-all-%s-%d" % [id, canvas.size.x])
		var row: Control = tile.get_node_or_null("TierControls")
		if row == null: row = tile.get_node("MinimalAbility")
		_expect(tile.get_global_rect().encloses(row.get_global_rect()), "whole content fits " + id)
	actor.ability_modifiers = original
	screen._render()
	for frame in 5: await process_frame

func _check_catalog_bonus_rows(screen) -> void:
	# Exercise the shared presenter for every pinned definition, including
	# abilities outside the Adventurer deck and inline tier controls.
	var actor: Dictionary = screen._view.actor("blade").duplicate(true)
	var rail := VBoxContainer.new(); screen._root.add_child(rail)
	rail.position = Vector2(700, 150); rail.size.x = 402
	for id in BattlePresentationCatalog._catalog.abilities:
		actor.ability_modifiers[0].ability_id = id
		var tile := BattleAbilityTile.new(); rail.add_child(tile)
		tile.configure(id, true, false, true, actor); tile.cinematic_compact()
		var tiers := BattlePresentationCatalog.inline_tiers(id, actor)
		if not tiers.is_empty():
			for tier in tiers: tier.enabled = true
			tile.configure_tiers(tiers, "")
		tile.minimal_rail()
		for frame in 6: await process_frame
		_check_tooltip(tile)
		var title: Label = tile.find_child("MinimalTitle", true, false)
		var badge: Label = tile.find_child("TemporaryDamageBonus", true, false)
		_expect(title.get_line_count() == 1 and title.size.x >= title.get_minimum_size().x, "catalog name remains horizontal: " + id)
		_expect(tile.get_global_rect().grow(0.1).encloses(title.get_global_rect()), "catalog name stays inside row: " + id)
		_expect(tile.get_global_rect().grow(0.1).encloses(badge.get_global_rect()), "catalog bonus stays inside row: " + id)
		_expect(not title.get_global_rect().intersects(badge.get_global_rect()), "catalog bonus never overlaps name: " + id)
		var main: Control = tile.get_node_or_null("TierControls")
		if main == null: main = tile.get_node("MinimalAbility")
		_expect(tile.get_global_rect().grow(0.1).encloses(main.get_global_rect()), "catalog main row fits: " + id)
		var requirement: Control = tile.find_child("MinimalRequirement", true, false)
		if requirement == null: requirement = tile.find_child("MinimalTiers", true, false)
		if requirement != null:
			_expect(tile.get_global_rect().grow(0.1).encloses(requirement.get_global_rect()), "catalog requirements fit: " + id)
			_expect(not requirement.get_global_rect().intersects(title.get_global_rect()) and not requirement.get_global_rect().intersects(badge.get_global_rect()), "catalog requirements never overlap name or bonus: " + id)
		rail.remove_child(tile); tile.queue_free(); await process_frame
	rail.queue_free(); await process_frame

func _click_hand(screen, id: String) -> void:
	var hand = screen._hand_dock
	hand.keep_visible = true; hand.reveal = 1; hand._layout()
	var index := -1
	for i in hand.cards.size():
		if hand.cards[i].instance_id == id: index = i
	_expect(index >= 0, "selected card exists in live hand")
	if index < 0: return
	var point: Vector2 = hand.get_global_transform_with_canvas() * (hand.base_transforms[index] * Vector2(45, 40))
	var motion := InputEventMouseMotion.new(); motion.position = point; canvas.push_input(motion, true); await process_frame
	for pressed in [true, false]:
		var click := InputEventMouseButton.new(); click.position = point; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = pressed; canvas.push_input(click, true)
	await process_frame
	_expect(screen._selected_card.get("instance_id") == id, "real hand click starts targeting")

func _tile(screen, id: String):
	for tile in screen.find_children("*", "Button", true, false):
		if tile is BattleAbilityTile and tile.ability_id == id: return tile
	return null
func _check_marker(screen, target_id: String) -> void:
	var tile = _tile(screen, target_id)
	_expect(tile != null, "target ability stays on board")
	if tile == null: return
	var badge = tile.find_child("TemporaryDamageBonus", true, false)
	_expect(badge != null and "+2 DMG" in badge.text and not "THIS OFFENSE" in badge.text, "persistent temporary +2 label")
	if badge != null: _expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(badge.get_global_rect()), "bonus marker fits viewport")
	if badge != null:
		var title: Label = tile.find_child("MinimalTitle", true, false)
		_expect(title.get_line_count() == 1, "name remains horizontal after live play and roll")
		_expect(not title.get_global_rect().intersects(badge.get_global_rect()), "bonus does not overlap ability name")
		if target_id == "adventurer_strike":
			_expect(absf(title.get_global_rect().get_center().y - badge.get_global_rect().get_center().y) < 2, "short ability keeps bonus inline")
			_expect(tile.custom_minimum_size.y == 36, "short ability adds no row height")
	var rail: ScrollContainer = screen._ability_dock.get_parent()
	_expect(not rail.get_v_scroll_bar().visible, "six abilities and bonus fit without scrollbar")
	var previous: BattleAbilityTile
	for entry in screen._ability_dock.find_children("*", "Button", true, false):
		if not entry is BattleAbilityTile: continue
		_expect(rail.get_global_rect().encloses(entry.get_global_rect()), "every ability fully visible")
		if previous != null: _expect(previous.get_global_rect().end.y <= entry.get_global_rect().position.y + 0.1, "ability rows do not overlap")
		previous = entry
	var dice = screen._player_dice_dock.get_child(0)
	for button in dice._buttons: _expect(button.size.is_equal_approx(BattleDiceTray.HUD_DIE_SIZE), "offensive dice retain full size")
	var roll: Button = screen._roll_dock.get_child(0).get_child(0)
	var skip: Button = screen._action_footer.get_child(0)
	_expect(roll.get_global_rect().position.x > dice.get_global_rect().end.x, "roll sits to right of dice")
	_expect(skip.get_global_rect().position.x > dice.get_global_rect().end.x, "skip sits to right of dice")
	_expect(not roll.get_global_rect().intersects(skip.get_global_rect()), "roll and skip do not overlap")
	_check_tooltip(tile)
	_expect("even if unused" in tile.tooltip_text, "ability hover explains expiry")
	var strip = screen._actor_profiles.blade.statuses
	_expect(strip.cells.has("strong_swing_ready") and strip.counts.get("strong_swing_ready") == 1, "positive status appears on character board")
	if strip.cells.has("strong_swing_ready"): _expect("end of the offensive segment" in strip.cells.strong_swing_ready.tooltip_text, "status hover explains exact expiry")
func _check_tooltip(tile: BattleAbilityTile) -> void:
	var info := BattlePresentationCatalog.ability(tile.ability_id, tile._actor)
	var tip := tile.tooltip_text
	_expect(tip.begins_with(str(info.name) + "\n"), "hover starts with ability name")
	_expect(not tip.contains(str(info.name) + "\n" + str(info.name)), "no duplicate ability heading: " + tile.ability_id)
	_expect(tip.contains(str(info.text)), "full pinned rules remain in hover: " + tile.ability_id)
	_expect(tip.count("Strong Swing:") == 1 and tip.count("even if unused") == 1, "bonus and expiration appear once: " + tile.ability_id)
	_expect(not tip.contains("Offensive Exit"), "hover uses player-facing phase wording")
	for button in tile.find_children("*", "Button", true, false):
		_expect(button.tooltip_text.count("Strong Swing:") == 1, "child button explains bonus once: " + tile.ability_id)
		if button.get_meta("battle_utility", false): _expect(button.tooltip_text == tip, "info button shares full ability tooltip")
	if tile.ability_id == "adventurer_small_straight":
		_expect(tip.count("Small Straight") == 1 and "Deal 6 damage" in tip and "Misses" in tip and "fifth die" in tip, "Small Straight retains requirements, damage and miss condition without repeating its name")

func _click(button: Button) -> void:
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center(); canvas.push_input(motion, true); await process_frame
	_expect(canvas.gui_get_hovered_control() == button, "pointer reaches the unqualified ability target")
	var click := InputEventMouseButton.new(); click.position = motion.position; click.button_index = MOUSE_BUTTON_LEFT; click.pressed = true; canvas.push_input(click, true)
	click = click.duplicate(); click.pressed = false; canvas.push_input(click, true); await process_frame
func _capture(name: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_SWING_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await process_frame
	RenderingServer.force_draw(false)
	canvas.get_texture().get_image().save_png(directory.path_join(name + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("STRONG SWING: " + message)
