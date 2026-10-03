extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
var canvas: SubViewport

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	canvas = SubViewport.new(); canvas.gui_embed_subwindows = true
	canvas.render_target_update_mode = SubViewport.UPDATE_ALWAYS; root.add_child(canvas)
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var before := {}; var revealed := {}
	# Exercise the real automatic card policy and public joint reveal.
	for seed_value in range(1, 60):
		var result: Dictionary = gateway.start_battle("surge-feedback-%d" % seed_value, seed_value, seed_value > 1)
		for step in 80:
			for event in result.get("events", []):
				if event.get("type") != "interaction_revealed" or event.get("segment") != "offensive": continue
				var bonuses = event.get("data", {}).get("commitments", {}).get("goblin", {}).get("damage_bonuses")
				if bonuses is Array and not bonuses.is_empty() and int(bonuses[0].before) == 4:
					revealed = result.duplicate(true); break
			if not revealed.is_empty() or result.get("snapshot", {}).get("stage") == "offensive_reaction": break
			before = result.duplicate(true)
			if result.get("learned_policy", {}).get("model_turn", false): result = gateway.advance_model()
			else:
				var action := {}
				for candidate in result.get("legal_actions", []):
					if candidate.get("type") in ["planning_pass", "pass"]: action = candidate; break
				if action.is_empty(): break
				result = gateway.submit(JSON.stringify(action))
			_expect(result.get("accepted", false), "native action accepted")
		if not revealed.is_empty(): break
	_expect(not revealed.is_empty(), "native Brine Mask reveals a 4 + 1 attack")
	if revealed.is_empty(): quit(1); return
	for width in [1920, 1280]:
		canvas.size = Vector2i(width, int(width * 9 / 16.0))
		await _check_screen(before, revealed)
	print("BRINE SURGE FEEDBACK: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _check_screen(before: Dictionary, revealed: Dictionary) -> void:
	var initial := before.duplicate(true); initial.events = []; initial.learned_policy = {}; initial.legal_actions = []; initial.pending_input = {}
	var result := revealed.duplicate(true); result.learned_policy = {}; result.legal_actions = []; result.pending_input = {}
	var screen = SCREEN.instantiate(); screen.initial_result = initial
	screen.gateway = BattleGateway.new(FakeBattleAuthority.new()); screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("surge-feedback.json"))
	canvas.add_child(screen)
	await create_timer(0.7).timeout
	_expect(_notices(screen).is_empty(), "no premature card reveal during hidden planning")
	screen._apply_model_result(result)
	_expect(_notices(screen).is_empty() and not screen._card_gain_active(), "joint reveal has no Brine Surge animation or input hold")
	await process_frame
	var panel = screen.ability_intent("goblin", "brine_lash")
	_expect(is_instance_valid(panel), "revealed enemy attack is visible immediately")
	if not is_instance_valid(panel): screen.queue_free(); return
	_expect(panel.damage.text == "5", "first visible attack already includes Brine Surge")
	var hint: String = panel.intent.tooltip_text
	_expect("Brine Lash" in hint and BattlePresentationCatalog.ability("brine_lash").text in hint, "full selected ability rules remain on hover")
	_expect("Current attack: 5 damage" in hint and "+1 damage to this attack from playing Brine Surge." in hint, "hover explains the authoritative card contribution")
	_expect(hint.ends_with("+1 damage to this attack from playing Brine Surge."), "card explanation is subtext at the bottom")
	_expect(hint.count("Brine Surge") == 1, "card explanation appears only once")
	await _capture_hover(panel, "reveal")
	screen._render(); await process_frame
	_expect(_notices(screen).is_empty() and screen.ability_intent("goblin", "brine_lash").damage.text == "5", "redraw retains final damage without an animation")
	# A later public re-reveal must update outcomes without replaying the card.
	var repeat := result.duplicate(true)
	for event in repeat.events: event.sequence = int(event.sequence) + 100000
	screen._director.queue_result(repeat, 0, before.snapshot.actors)
	_expect(screen._director.take_status_updates().is_empty(), "re-revealing the same card cannot animate it")
	# Defense snapshots can omit old reveal events: preserve the public cause.
	var defense := result.duplicate(true); defense.events = []
	defense.snapshot.segment = "defensive"; defense.snapshot.stage = "defense_selection"
	screen._view.apply_result(defense); screen._render(); await process_frame
	panel = screen.ability_intent("goblin", "brine_lash")
	_expect(panel.damage.text == "5" and "+1 damage to this attack from playing Brine Surge." in panel.intent.tooltip_text, "defense keeps the total and its card explanation")
	_expect(_notices(screen).is_empty(), "defense does not replay Brine Surge")
	await _capture_hover(panel, "defense")
	# Reusing the same actor in a later round must not carry the old bonus.
	defense.snapshot.round = int(defense.snapshot.round) + 1
	screen._view.apply_result(defense); screen._render(); await process_frame
	panel = screen.ability_intent("goblin", "brine_lash")
	_expect(not "from playing Brine Surge" in panel.intent.tooltip_text, "card explanation expires with its attack round")

	screen.active_store.clear(); screen.queue_free(); await process_frame

func _notices(screen: Node) -> Array:
	return screen.get_children().filter(func(child): return child.get_script() == NOTICE and not child.is_queued_for_deletion())

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("BRINE SURGE: " + message)

func _capture_hover(panel, stage: String) -> void:
	var path := OS.get_environment("DICE_AND_DESTINY_SURGE_SCREENSHOT")
	if path.is_empty() or DisplayServer.get_name() == "headless": return
	for frame in 8: await process_frame
	canvas.notify_mouse_entered()
	var motion := InputEventMouseMotion.new(); motion.position = panel.intent.get_global_rect().get_center(); canvas.push_input(motion, true)
	await create_timer(1.2).timeout
	var popup := _find_tooltip(canvas)
	if popup == null:
		motion.position += Vector2(2, 0); canvas.push_input(motion, true)
		await create_timer(1.0).timeout
		popup = _find_tooltip(canvas)
	_expect(popup != null, "actual hover opens tooltip")
	if popup != null:
		var window := popup.get_window()
		_expect(Rect2(Vector2.ZERO, Vector2(canvas.size)).encloses(Rect2(Vector2(window.position), Vector2(window.size))), "hover explanation fits viewport")
	RenderingServer.force_draw()
	canvas.get_texture().get_image().save_png(path.replace(".png", "-%s-%d.png" % [stage, canvas.size.x]))
	canvas.notify_mouse_exited(); motion.position = Vector2(640, 700); canvas.push_input(motion, true); await process_frame
func _find_tooltip(node: Node) -> Control:
	if node.name == "AttackAbilityTooltip": return node
	for child in node.get_children(true):
		var found := _find_tooltip(child)
		if found != null: return found
	return null
