extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false
class DelayedGateway:
 extends RefCounted
 var gate := Semaphore.new()
 var result: Dictionary
 var commands := 0
 func advance_model() -> Dictionary:
  gate.wait()
  return result
 func submit(_command: String) -> Dictionary:
  commands += 1
  return result
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
 root.size = Vector2i(1920, 1080)
 var native = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
 var initial: Dictionary = native.start_battle("quiet-model-wait", 1789679833957118)
 initial.events = []; initial.learned_policy = {}
 var delayed := DelayedGateway.new(); delayed.result = initial.duplicate(true)
 var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = delayed
 screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("quiet-model-wait.json")); screen._auto_pass_disabled = true
 root.add_child(screen)
 for frame in 5: await process_frame
 var board: Control = screen._root
 var hand: Control = screen._hand_dock
 var dice: Control = screen._player_dice_dock
 var hand_count := hand.get_child_count()
 var hand_rect := hand.get_global_rect()
 screen.learned_battle_mode = true
 screen._schedule_model_if_needed({"learned_policy": {"model_turn": true}})
 _expect(screen._model_thinking, "worker starts")
 for frame in 20:
  await process_frame
  _expect(screen._root == board and screen._hand_dock == hand and screen._player_dice_dock == dice, "waiting preserves existing board nodes")
  _expect(hand.get_child_count() == hand_count and hand.modulate.a == 1.0 and hand.get_global_rect() == hand_rect, "hand stays present and stationary")
  for label in screen._root.find_children("*", "Label", true, false):
   _expect(not "thinking" in label.text.to_lower() and not "Frozen seed" in label.text, "no thinking interstitial")
 screen._send("{}")
 _expect(delayed.commands == 0, "gameplay remains locked during inference")
 for button in board.find_children("*", "BaseButton", true, false):
  if not button.get_meta("battle_utility", false): _expect(button.disabled, "gameplay controls locked in place")
 # A utility redraw while the worker waits must also keep the hand and board.
 screen._render()
 _expect(screen._hand_dock.get_child_count() == hand_count, "redraw during wait does not replace hand with an interstitial")
 _expect(screen._prompt_text().is_empty(), "no fallback thinking prompt")
 delayed.gate.post()
 for frame in 100:
  if not screen._model_thinking: break
  await process_frame
 _expect(not screen._model_thinking and not screen._submitting and not screen._model_error, "result resumes normal play")
 screen.active_store.clear(); screen.queue_free(); await process_frame
 print("QUIET MODEL WAIT: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _expect(condition: bool, message: String) -> void:
 if not condition: failed = true; push_error("QUIET MODEL WAIT: " + message)
