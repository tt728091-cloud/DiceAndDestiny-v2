extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
var canvas: SubViewport
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS
	root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("attack-ability-tooltips", 43)
	for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
		canvas.size = viewport
		for stage in ["offensive_reaction", "defense_selection"]:
			for count in [1, 4]:
				var fixture := base.duplicate(true)
				fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
				fixture.snapshot.stage = stage; fixture.snapshot.segment = "offensive" if stage == "offensive_reaction" else "defensive"
				fixture.snapshot.damage_sources = []
				for i in range(2, count + 1): fixture.snapshot.actors["goblin-" + str(i)] = fixture.snapshot.actors.goblin.duplicate(true)
				for actor_id in fixture.snapshot.actors:
					var player: bool = actor_id == "blade"
					var actor: Dictionary = fixture.snapshot.actors[actor_id]
					actor.selected_ability = "hexbrand" if player else "brine_lash"
					actor.selected_tier = "skull_4" if player else "brine_2"
					actor.dice = {"pool": "offensive", "dice": _dice([1, 2, 3, 3, 6] if player else [5, 2, 3, 1, 3], "curse_d6" if player else "brine_d6")}
					fixture.snapshot.damage_sources.append({"id": actor_id + "-attack", "source_actor_id": actor_id, "target_actor_id": "goblin" if player else "blade", "source_content_id": actor.selected_ability, "base_amount": 4 if player else 5})
				var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
				screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("attack-tooltips.json"))
				canvas.add_child(screen); screen.set_process(false)
				for frame in 6: await process_frame
				for actor_id in fixture.snapshot.actors:
					var panel = screen._attack_intents[actor_id + "-attack"]
					var hint: String = panel.intent.tooltip_text
					var id: String = panel.source.source_content_id
					_expect(hint.begins_with(BattlePresentationCatalog.ability(id).name), "ability name leads tooltip")
					_expect(BattlePresentationCatalog.ability(id).text in hint, "complete pinned rules included for every actor")
					_expect("Selected:" in hint and "Offensive dice:" in hint, "selected recipe and revealed faces included")
					if actor_id != "blade":
						_expect("2 Brine → 4 DMG" in hint and "5 · 2 · 3 · 1 · 3" in hint, "Brine Lash explains selected tier and dice")
						_expect("Current attack: 5 damage" in hint, "modified damage does not guess a different tier")
					else: _expect("4 Skull" in hint and "Then apply 2 Curse" in hint, "player tier includes precise status quantity")
					for icon in panel.intent_row.get_children():
						_expect(icon.mouse_filter == Control.MOUSE_FILTER_IGNORE, "symbols and numbers inherit the full ability hover")
					if is_instance_valid(panel.fighter_target): _expect(panel.fighter_target.tooltip_text == hint, "fighter target shares ability explanation")
					var popup: Control = panel.intent._make_custom_tooltip(hint)
					canvas.add_child(popup)
					for frame in 3: await process_frame
					_expect(popup.size.x < viewport.x and popup.size.y < viewport.y - 30, "full rules fit viewport without clipping")
					popup.queue_free(); await process_frame
				if stage == "defense_selection":
					await _hover(screen._attack_intents[("goblin" if count == 1 else "goblin-4") + "-attack"].intent, "enemy-%d-%d" % [count, viewport.x])
					await _hover(screen._attack_intents["blade-attack"].intent, "player-%d-%d" % [count, viewport.x])
				# Re-render after a legal dice edit: explain the new tier and faces.
				fixture.snapshot.actors.goblin.selected_tier = "brine_1"
				fixture.snapshot.actors.goblin.dice.dice = _dice([5, 2, 3, 1, 4], "brine_d6")
				for source in fixture.snapshot.damage_sources:
					if source.source_actor_id == "goblin": source.base_amount = 2
				screen._view.apply_result(fixture); screen._render()
				for frame in 4: await process_frame
				var updated: String = screen._attack_intents["goblin-attack"].intent.tooltip_text
				_expect("1 Brine → 2 DMG" in updated and "5 · 2 · 3 · 1 · 4" in updated, "live dice edit refreshes recipe and faces")
				screen.queue_free(); await process_frame
	# Every authored offensive ability/tier gets its own catalog-derived rules.
	for id in BattlePresentationCatalog._catalog.get("abilities", {}):
		var definition := BattlePresentationCatalog.definition("abilities", id)
		if definition.get("type") != "offensive": continue
		_expect(not BattlePresentationCatalog.ability(id).text.is_empty(), id + " authors full ability rules")
		for tier in definition.get("qualification", {}).get("activation_tiers", []):
			var text := BattlePresentationCatalog.attack_ability_tooltip(id, {}, {"ability_id": id, "tier_id": tier.id}, [])
			_expect("Selected:" in text and BattlePresentationCatalog.ability(id).text in text, id + " selected tier and full rules available")
	print("ATTACK ABILITY TOOLTIPS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _dice(faces: Array, die_id: String) -> Array:
	var dice := []
	for face in faces: dice.append({"face": face, "die_id": die_id})
	return dice
func _hover(button: Button, suffix: String) -> void:
	if DisplayServer.get_name() == "headless": return
	canvas.notify_mouse_entered()
	var motion := InputEventMouseMotion.new(); motion.position = button.get_global_rect().position + Vector2(15, 15); canvas.push_input(motion, true)
	await create_timer(1.5).timeout
	var popup := _find_popup(canvas)
	# The first popup may be cancelled by the initial damage-label animation.
	if popup == null:
		motion = InputEventMouseMotion.new(); motion.position = button.get_global_rect().get_center() + Vector2(2, 0); canvas.push_input(motion, true)
		await create_timer(1.0).timeout
		popup = _find_popup(canvas)
	_expect(popup != null, "actual mouse hover opens full tooltip " + suffix)
	if popup != null:
		var window := popup.get_window()
		_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(window.position), Vector2(window.size))), "hover popup stays within viewport " + suffix)
	var directory := OS.get_environment("DICE_AND_DESTINY_TOOLTIP_SCREENSHOTS")
	if not directory.is_empty():
		RenderingServer.force_draw(false)
		canvas.get_texture().get_image().save_png(directory.path_join("ability-hover-" + suffix + ".png"))
	motion = InputEventMouseMotion.new(); motion.position = Vector2(640, 700); canvas.push_input(motion, true)
	canvas.notify_mouse_exited()
	await process_frame
func _find_popup(node: Node) -> Control:
	if node.name == "AttackAbilityTooltip": return node
	for child in node.get_children(true):
		var found := _find_popup(child)
		if found != null: return found
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("ATTACK ABILITY TOOLTIPS: " + message)
