extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
const NOTICE := preload("res://presentation/battle/card_gain_notice.gd")
var failed := false
func _initialize() -> void: call_deferred("_run")
func _run() -> void:
 root.size = Vector2i(1920, 1080)
 var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
 var initial: Dictionary = gateway.start_battle("focus-rewards", 1789759004637659)
 for action in initial.get("legal_actions", []):
  if action.type == "planning_pass":
   initial = gateway.submit(JSON.stringify(action)); break
 _expect(initial.get("learned_policy", {}).get("model_turn", false), "native opponent owns planning")
 initial.events = []
 var screen = SCREEN.instantiate(); screen.initial_result = initial; screen.gateway = gateway
 screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("focus-rewards.json")); screen._auto_pass_disabled = true
 root.add_child(screen); screen.set_process(false)
 for frame in 5: await process_frame
 for play in 3:
  var before: Dictionary = screen._view.actors.goblin.duplicate(true)
  var result: Dictionary = gateway.advance_model()
  _expect(result.get("accepted", false), "native card play accepted")
  var definition := str(result.get("events", [{}])[0].get("data", {}).get("card_definition_id", ""))
  _expect(definition == ("battle_focus" if play < 2 else "sharpen_blade"), "recorded seed exercises two draws then a non-drawing card")
  screen._apply_model_result(result)
  var after: Dictionary = screen._view.actors.goblin
  _expect(after.get("hand", []).is_empty() and after.get("card_instances", {}).is_empty() and after.get("dice", {}) == {}, "enemy hand and dice stay hidden")
  var notices := []
  for child in screen.get_children():
   if child.get_script() == NOTICE and not child.is_queued_for_deletion(): notices.append(child)
  if play < 2:
   _expect(int(after.energy_points) == int(before.energy_points) + 1, "native snapshot grants one Energy")
   _expect(after.hand_count == before.hand_count and after.deck_count == before.deck_count - 1 and after.discard_count == before.discard_count + 1, "public pile counts reflect play plus draw")
   _expect(notices.size() == 1, "one reward animation")
   if notices.is_empty(): break
   var notice = notices[0]; notice.set_process(false)
   _expect(notice._label.text == "+1 Energy · +1 card", "both rewards announced")
   _expect(screen._actor_profiles.goblin._stat_labels.energy.text.ends_with(str(int(before.energy_points))), "no first-frame flash of final energy")
   notice._started = true; notice._elapsed = notice.duration * 0.3; notice.refresh()
   screen._render()
   for frame in 5: await process_frame
   _expect(screen._actor_profiles.goblin._stat_labels.energy.text.ends_with(str(int(before.energy_points))), "redraw preserves pre-reward energy")
   await _capture("before")
   notice._elapsed = notice.duration * 0.64; notice.refresh()
   _expect(screen._actor_profiles.goblin._stat_labels.energy.text.ends_with(str(int(after.energy_points))), "energy visibly increments when trail arrives")
   _expect(notice._destinations.size() == 2, "trails reach Energy and Hand")
   await _capture("after")
   notice._elapsed = notice.duration + 0.1; notice.refresh(); await process_frame
  else:
   _expect(notices.is_empty(), "non-drawing upgrade never invents a draw")
   _expect(after.hand_count == before.hand_count - 1, "public hand drops for upgrade card")
 screen.active_store.clear(); screen.queue_free(); await process_frame
 print("BATTLE FOCUS REWARDS: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
func _capture(phase: String) -> void:
 var directory := OS.get_environment("DICE_AND_DESTINY_FOCUS_SCREENSHOTS")
 if directory.is_empty() or DisplayServer.get_name() == "headless": return
 await RenderingServer.frame_post_draw
 root.get_texture().get_image().save_png(directory.path_join("battle-focus-" + phase + ".png"))
func _expect(condition: bool, message: String) -> void:
 if not condition: failed = true; push_error("BATTLE FOCUS: " + message)
