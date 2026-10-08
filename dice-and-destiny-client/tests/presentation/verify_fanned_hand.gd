extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const FAN := preload("res://presentation/cards/fanned_hand.gd")
var failed := false
var clicked := -1
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var capture := not OS.get_environment("DICE_AND_DESTINY_FAN_SCREENSHOTS").is_empty()
	for character in ["blade_warden", "venom", "curse"]:
		var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", character)
		var fixture: Dictionary = gateway.start_battle("fan-" + character, 3)
		_expect(fixture.get("accepted", false), "fixture for " + character)
		fixture.events = []; fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
		fixture.snapshot.segment = "offensive"; fixture.snapshot.stage = "planning"
		for viewport in [Vector2i(1920, 1080), Vector2i(1280, 720)]:
			root.size = viewport
			var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(FakeBattleAuthority.new())
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("fan-test.json"))
			root.add_child(screen); screen.set_process(false)
			var fan = screen._hand_dock; fan.set_process(false)
			await _frames()
			_expect(fan.cards.size() == fixture.snapshot.actors.blade.hand.size(), "shared hand retains every " + character + " card")
			fan.reveal = 0; fan.hovered = -1; fan._layout()
			for card in fan.cards:
				_expect(card.get_node("EnergyCost").position.x < 12 and card.get_node("CardTitle").position.x > card.get_node("EnergyCost").get_rect().end.x, "cost left, title inset for " + character)
				var top: Vector2 = fan.get_global_transform().affine_inverse() * (card.get_global_transform() * Vector2(0, 0))
				_expect(top.y > fan.size.y - 105 and top.y < fan.size.y - 25, "collapsed tops visible")
			if capture and viewport.x == 1920: await _capture(character + "-collapsed")
			fan.reveal = 1; fan._layout(); await _frames()
			for slot in fan.slots:
				for corner in [Vector2.ZERO, Vector2(185, 0), Vector2(0, 248), Vector2(185, 248)]:
					_expect(Rect2(Vector2.ZERO, fan.size).encloses(Rect2(slot.get_transform() * corner, Vector2.ONE)), "expanded card fits hover region")
			_expect(fan.slots[0].rotation < 0 and fan.slots[-1].rotation > 0, "cards fan in both directions")
			if capture and viewport.x == 1920:
				await _capture(character + "-expanded")
				fan.hovered = 2; fan._layout(); await _frames()
				await _capture(character + "-hovered")
			# Opening state survives a battle redraw, such as card targeting.
			screen._render(); screen._hand_dock.set_process(false); await _frames()
			_expect(screen._hand_dock.reveal == 1, "redraw does not snap hand shut")
			screen.queue_free(); await process_frame
	await _interaction()
	print("FANNED HAND: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _interaction() -> void:
	root.size = Vector2i(1280, 720)
	var fan = FAN.new(); root.add_child(fan); fan.position = Vector2(140, 430); fan.size = Vector2(965, 290); fan.set_process(false)
	for count in [0, 1, 6, 10]:
		for slot in fan.slots: slot.queue_free()
		fan.cards.clear(); fan.slots.clear(); fan.base_transforms.clear()
		for i in count:
			var card := BattleCard.new(); card.configure("fan-" + str(i), "unquiet_hands", true); fan.add_card(card)
			card.pressed.connect(func(): clicked = i)
		await _frames()
		fan.reveal = 0; fan._layout()
		# The empty upper part of the fully expanded box must open the fan.
		await _move(fan.position + Vector2(480, 15)); fan._process(0.3)
		_expect(fan.reveal == (1.0 if count > 0 else 0.0), "whole region opens hand")
		for i in count:
			var point: Vector2 = fan.base_transforms[i] * Vector2(22, 22)
			_expect(fan.card_at(point) == i, "exposed cost selects correct overlapping card %d/%d" % [i, count])
			await _move(fan.position + point); fan._process(0)
			clicked = -1
			await _click(fan.position + point)
			_expect(clicked == i, "real pointer activates correct card")
			fan.cards[i].disabled = true; clicked = -1; await _click(fan.position + point)
			_expect(clicked == -1, "disabled card cannot be played")
			fan.cards[i].disabled = false
		if count > 0:
			# The lifted card's newly exposed top remains a valid pointer target.
			fan.hovered = 0; fan._layout()
			var lifted: Vector2 = fan.slots[0].get_transform() * Vector2(22, 3)
			_expect(fan.card_at(lifted) == 0, "lifted title remains clickable")
			fan.cards[0].toggle_mode = true; clicked = -1
			await _click(fan.position + lifted)
			_expect(clicked == 0 and fan.cards[0].button_pressed, "selection toggle survives custom pointer routing")
			fan.cards[0].grab_focus(); clicked = -1
			for pressed in [true, false]:
				var key := InputEventKey.new(); key.keycode = KEY_SPACE; key.pressed = pressed; root.push_input(key, true)
				await process_frame
			_expect(clicked == 0, "keyboard activates focused card")
			fan.cards[0].prepare_income_draw(); fan.cards[0].animate_income_draw(0.1)
			await create_timer(0.15).timeout
			_expect(fan.cards[0].modulate.a > 0.99 and fan.cards[0].scale.is_equal_approx(Vector2.ONE), "draw animation completes within fan slot")
		await _move(Vector2(20, 20)); fan._process(0.4)
		_expect(fan.reveal == 0, "leaving region collapses despite mouse focus")
		fan.held_open = count > 0; fan._process(0.4)
		_expect(fan.reveal == (1.0 if count > 0 else 0.0), "targeted/discard hand remains available")
		fan.held_open = false
	fan.queue_free(); await process_frame
func _move(point: Vector2) -> void:
	var event := InputEventMouseMotion.new(); event.position = point; event.global_position = point; root.push_input(event, true); await process_frame
func _click(point: Vector2) -> void:
	for pressed in [true, false]:
		var event := InputEventMouseButton.new(); event.position = point; event.global_position = point; event.button_index = MOUSE_BUTTON_LEFT; event.pressed = pressed; root.push_input(event, true); await process_frame
func _frames() -> void:
	for i in 8: await process_frame
func _capture(id: String) -> void:
	if DisplayServer.get_name() == "headless": return
	RenderingServer.force_draw(false)
	root.get_texture().get_image().save_png(OS.get_environment("DICE_AND_DESTINY_FAN_SCREENSHOTS").path_join(id + ".png"))
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("FAN: " + message)
