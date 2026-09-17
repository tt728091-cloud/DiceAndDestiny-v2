extends SceneTree

const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void:
	call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var result: Dictionary = gateway.start_battle("lens-preview", 1788900535520209)
	_expect(result.get("accepted") == true, "native battle starts")
	var screen = SCREEN.instantiate()
	screen.gateway = gateway
	screen.initial_result = result
	screen.learned_battle_mode = true
	screen.learned_human_seat = "seat-a"
	root.add_child(screen)
	await _settle(screen)
	_expect(_needlefang_tooltip(screen).contains("3 Fang: 4 damage + 1 Poison"), "base hover before Lens")
	var lens = null
	for node in screen.find_children("*", "", true, false):
		if node is BattleCard and node.definition_id == "venom_lens": lens = node
	_expect(lens != null, "incident seed has Venom Lens")
	if lens != null:
		screen._on_card_pressed(lens)
		await process_frame
		var dialogs := screen.find_children("*", "AcceptDialog", false, false)
		_expect(dialogs.is_empty(), "Lens plays immediately without a one-option popup")
		await _settle(screen)
		_expect(int(screen._view.actor("blade").get("needlefang_damage_bonus", 0)) == 1, "native Lens upgrade applied")
		var preview := _needlefang_tooltip(screen)
		for expected in ["3 Fang: 5 damage + 1 Poison", "4 Fang: 4 damage + 2 Poison", "5 Fang: 3 damage + 3 Poison"]:
			_expect(preview.contains(expected), "upgraded hover: " + expected)
		_expect(BattlePresentationCatalog.ability("needlefang").text.contains("3 Fang: 4 damage + 1 Poison"), "upgrade does not mutate the shared base catalog")
		_expect(BattlePresentationCatalog.ability("needlefang", screen._view.actor("goblin")).text.contains("3 Fang: 4 damage + 1 Poison"), "upgrade belongs only to its actor")
		var tile := _needlefang(screen)
		var notice := tile.get_node_or_null("UpgradeGlow")
		_expect(notice != null and tile._upgrade_notice.text == "+1 DAMAGE · ALL TIERS", "upgrade glow and explanation appear on Needlefang")
		_expect(screen._view.allowed("planning_roll") and not screen._director.has_beats(), "upgrade feedback does not interrupt planning")
		var path := OS.get_environment("DICE_AND_DESTINY_LENS_SCREENSHOT")
		if not path.is_empty() and DisplayServer.get_name() != "headless":
			await RenderingServer.frame_post_draw
			root.get_texture().get_image().save_png(path)
		var started: int = screen._ability_upgrade_feedback.blade.started_ms
		for action in screen._view.legal_actions:
			if action.get("type") == "planning_roll":
				screen._send(JSON.stringify(action))
				break
		await _settle(screen)
		_expect(screen._view.rolls_used("blade") == 1, "can roll while upgrade effect is visible")
		_expect(screen._ability_upgrade_feedback.blade.started_ms == started, "refreshing the board does not restart the animation")
		await create_timer(3.0).timeout
		_expect(_needlefang(screen)._upgrade_notice.modulate.a < 0.01, "upgrade message fades away")
		screen._render()
		await process_frame
		_expect(_needlefang(screen).get_node_or_null("UpgradeGlow") == null, "expired effect does not return after a refresh")
	_expect(screen._error_message.is_empty(), "no command errors")
	screen.active_store.clear()
	screen.queue_free()
	await process_frame
	print("VENOM LENS PREVIEW: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _needlefang_tooltip(screen) -> String:
	for node in screen.find_children("*", "Button", true, false):
		if node is BattleAbilityTile and node.ability_id == "needlefang": return node.tooltip_text
	return ""

func _needlefang(screen) -> BattleAbilityTile:
	for node in screen.find_children("*", "Button", true, false):
		if node is BattleAbilityTile and node.ability_id == "needlefang": return node
	return null

func _expect(condition: bool, message: String) -> void:
	if not condition:
		failed = true
		push_error("VENOM LENS PREVIEW: " + message)

func _settle(screen) -> void:
	# Let real native model decisions finish, then dismiss presentation only.
	for step in range(2000):
		await process_frame
		screen._director.clear()
		if not screen._model_thinking and not bool(screen._view.learned_policy.get("model_turn", false)):
			screen._render()
			await process_frame
			return
		if screen._model_error: break
		await create_timer(0.01).timeout
	_expect(false, "model failed to return control: " + str(screen._error_message))
