extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const TARGETING := preload("res://presentation/battle/die_face_targeting.gd")
const ADJACENT := preload("res://presentation/battle/adjacent_face_targeting.gd")
const FOLDOUT := preload("res://presentation/battle/number_face_foldout.gd")
var failed := false
class RecordingGateway extends RefCounted:
	var commands: Array = []
	func submit(command: String) -> Dictionary:
		commands.append(JSON.parse_string(command)); return {"accepted": false, "error": "Test captured submission"}
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "curse")
	var base: Dictionary = gateway.start_battle("remaining-dice-choices", 9)
	base.events = []; base.learned_policy = {}; base.pending_input = {}
	var widths := [1280, 1920]
	if not OS.get_environment("DICE_AND_DESTINY_CHOICE_TEST_WIDTH").is_empty(): widths = [int(OS.get_environment("DICE_AND_DESTINY_CHOICE_TEST_WIDTH"))]
	for width in widths:
		root.size = Vector2i(width, int(width * 9 / 16.0))
		for id in ["widen_the_crack", "chosen_instrument", "no_safe_keep", "steady_hand", "forked_tongue", "eclipse", "adjacent", "grasp", "repaid"]:
			var target := "blade" if id == "steady_hand" else "goblin"
			var face_card: bool = id in ["steady_hand", "forked_tongue"]
			var mandatory: bool = id in ["eclipse", "adjacent", "grasp", "repaid"]
			var face_choice: bool = id in ["eclipse", "adjacent"]
			var result := base.duplicate(true)
			var stage := "curse_choice" if mandatory else ("offensive_reaction" if id in ["no_safe_keep", "forked_tongue"] else "planning")
			result.snapshot.stage = stage; result.snapshot.flow.stage = stage
			if mandatory: result.snapshot.curse_choice = {"kind": id, "card_id": "eclipse_of_the_black_star" if id == "eclipse" else "widen_the_crack", "source": "blade", "target": "goblin", "die": 1, "face": 3}
			result.legal_actions = []
			for key in (["2", "4"] if face_choice else ([target + ":0:1", target + ":0:4"] if face_card else ["0", "2"])):
				var data := {"choice_id": "goblin:%s:0" % key if id == "no_safe_keep" else key, "target_ids": [target], "card_ids": [] if mandatory else ["choice-card"]}
				if stage == "planning":
					data["status_id"] = data.choice_id
					result.legal_actions.append({"type": "planning_commit_cards", "actor_id": "blade", "payload": data})
				else: result.legal_actions.append({"type": "commit_interaction", "actor_id": "blade", "payload": {"commitment": data}})
			var recorder := RecordingGateway.new()
			var screen = SCREEN.instantiate(); screen.initial_result = result; screen.gateway = recorder; screen._auto_pass_disabled = true
			screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("remaining-dice.json")); root.add_child(screen); await process_frame
			if not mandatory:
				if id == "no_safe_keep" or face_card:
					# Revealed dice are supplied by the live authority; this fixture
					# isolates routing and exact command forwarding from gameplay RNG.
					screen._view.actors[target]["dice"] = {"pool": "offensive", "dice": [{"die_id": "brine_d6", "face": 3}, {"die_id": "brine_d6", "face": 2}, {"die_id": "brine_d6", "face": 1}]}
				var card := BattleCard.new(); card.instance_id = "choice-card"; card.definition_id = id
				screen._on_card_pressed(card); card.free(); await process_frame
			var selector = _selector(screen, ADJACENT if id == "adjacent" else FOLDOUT if face_choice else TARGETING)
			_expect(selector != null, id + " uses board selection")
			_expect(screen.get_children().all(func(child): return not child is AcceptDialog), id + " creates no modal")
			if selector == null: screen.queue_free(); await process_frame; continue
			for frame in 8: await process_frame
			if id == "adjacent": selector.refresh(screen._number_face_actions())
			else: selector._elapsed = 1; selector.refresh()
			await process_frame
			_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), id + " fits viewport")
			_expect(recorder.commands.is_empty(), id + " does not submit while opening")
			var capture_dir := OS.get_environment("DICE_AND_DESTINY_FOLDOUT_SCREENSHOTS")
			if not capture_dir.is_empty() and DisplayServer.get_name() != "headless":
				RenderingServer.force_draw(false); root.get_texture().get_image().save_png(capture_dir.path_join("%s-%d.png" % [id, width]))
			if face_choice:
				if id == "adjacent": _expect(selector.faces.size() == 2 and selector.faces.has(4), "adjacent choices highlight exact face chips")
				else: _expect(selector.faces.size() == 6 and selector.faces[3].disabled and not selector.faces[4].disabled, id + " enables only legal faces in six-face net")
				# Removed legal choices cannot be submitted through an old button.
				screen._view.legal_actions = [result.legal_actions[0]]
				selector.faces[4].pressed.emit()
				_expect(recorder.commands.is_empty(), id + " rejects stale choice")
				selector.faces[2].pressed.emit()
			elif face_card:
				selector.targets[target + ":0"].pressed.emit(); await process_frame
				selector = _selector(screen, TARGETING)
				_expect(selector.faces.size() == 2 and recorder.commands.is_empty(), id + " unfolds faces before sending")
				for frame in 8: await process_frame
				_expect(root.get_visible_rect().encloses(selector.panel.get_global_rect()), id + " face net fits viewport")
				if not capture_dir.is_empty() and DisplayServer.get_name() != "headless":
					RenderingServer.force_draw(false); root.get_texture().get_image().save_png(capture_dir.path_join("%s-faces-%d.png" % [id, width]))
				selector.faces[0].pressed.emit()
			else:
				_expect(selector.targets.size() == 2 and selector.targets.has("goblin:2"), id + " highlights only eligible physical dice")
				selector.targets["goblin:0"].pressed.emit()
			await process_frame
			_expect(recorder.commands.size() == 1 and recorder.commands[0] == result.legal_actions[0], id + " forwards exact legal command")
			screen.active_store.clear(); screen.queue_free(); await process_frame
	print("REMAINING DICE CHOICES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _selector(screen, script):
	for child in screen._root.get_children():
		if child.get_script() == script: return child
	return null
func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error("REMAINING DICE CHOICES: " + message)
