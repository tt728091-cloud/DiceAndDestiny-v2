extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
 var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask-pair", "venom")
 var base: Dictionary = gateway.start_battle("completion-screen", 43)
 for outcome in ["defeat", "victory", "draw"]:
  var fixture := base.duplicate(true)
  fixture.learned_policy = {}; fixture.legal_actions = []; fixture.pending_input = {}
  fixture.battle_result = outcome
  fixture.snapshot.status = "draw" if outcome == "draw" else "victory"
  fixture.snapshot.stage = "complete"; fixture.snapshot.segment = "damage_resolution"
  fixture.snapshot.actors.blade.current_health = 24 if outcome == "victory" else 0
  # The raw authority event describes a team's victory; the viewer result is defeat.
  fixture.events = [{"sequence": 100, "type": "battle_completed", "segment": "damage_resolution", "battle_result": fixture.snapshot.status}]
  var fake := FakeBattleAuthority.new()
  var screen = SCREEN.instantiate(); screen.initial_result = fixture; screen.gateway = BattleGateway.new(fake); screen.learned_battle_mode = true
  screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("completion-test.json"))
  root.add_child(screen)
  await process_frame; await process_frame
  _expect(not screen._director.has_beats(), "completion has no extra presentation step")
  _expect(screen._director.last_sequence() == 100, "completion event watermark is preserved")
  var labels := screen.find_children("*", "Label", true, false)
  _expect(labels.any(func(label): return label.text == outcome.to_upper()), "results immediately show viewer's " + outcome)
  _expect(not labels.any(func(label): return label.text == "The battle is complete." or label.text == "Victory"), "old completion text never appears")
  var buttons := screen.find_children("*", "Button", true, false)
  _expect(buttons.any(func(button): return button.text == "Rematch · Same Seats" and not button.disabled), "rematch is available immediately")
  _expect(buttons.any(func(button): return button.text == "New Battle · Change Seat or Mode"), "mode menu remains available")
  _expect(not buttons.any(func(button): return button.text == "Continue Presentation"), "no redundant completion click")
  _expect(fake.commands.is_empty(), "showing results sends no gameplay command")
  screen.active_store.clear(); screen.queue_free(); await process_frame
 # The final damage animation still precedes results and owns the final watermark.
 var director := BattlePresentationDirector.new()
 director.queue_result({"events": [
  {"sequence": 1, "type": "damage_committed", "segment": "damage_resolution", "data": {"sources": [{"id": "lethal", "source_actor_id": "goblin", "target_actor_id": "blade", "base_amount": 1}]}},
  {"sequence": 2, "type": "battle_completed", "battle_result": "victory"}
 ]})
 _expect(director.peek().get("type") == "combat_damage", "final damage animation is retained")
 director.advance()
 _expect(not director.has_beats() and director.last_sequence() == 2, "damage leads directly to final results")
 print("COMPLETION SCREEN: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(ok: bool, message: String) -> void:
 if not ok: failed = true; push_error("COMPLETION SCREEN: " + message)
