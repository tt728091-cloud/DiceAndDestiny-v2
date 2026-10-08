extends SceneTree
const SCREEN := preload("res://app/screens/battle/battle_screen.tscn")
const GATEWAY := preload("res://local_client/learned_battle/learned_battle_gateway.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = GATEWAY.new(root.get_node("LearnedBattleRuntime"), "seat-a", "global-champion", "venom")
	var initial: Dictionary = gateway.start_battle("combined-attack-statuses", 1789679833957118)
	initial.events = []; initial.learned_policy = {}; initial.pending_input = {}; initial.legal_actions = []
	var applications := [
		{"source_actor_id": "goblin", "target_actor_id": "blade", "status_id": "bleed", "stacks": 1},
		{"source_actor_id": "goblin", "target_actor_id": "blade", "status_id": "bleed", "stacks": 1},
		{"source_actor_id": "goblin", "target_actor_id": "blade", "status_id": "poison", "stacks": 3},
		{"source_actor_id": "goblin", "target_actor_id": "goblin", "status_id": "bleed", "stacks": 4}]
	initial.snapshot.damage_sources = [{"id": "sword-cut", "source_actor_id": "goblin", "target_actor_id": "blade", "source_content_id": "sword_cut", "base_amount": 7}]
	initial.snapshot.actors.goblin.offensive_outcome = {"base_damage": 7, "status_applications": applications.duplicate(true)}
	initial.snapshot.settled_damage = {"id": "combined-status-batch", "sources": initial.snapshot.damage_sources, "status_applications": applications.duplicate(true), "removals": []}
	initial.snapshot.segment = "defensive"; initial.snapshot.stage = "defense_selection"
	var screen = SCREEN.instantiate(); screen.initial_result = initial; screen._auto_pass_disabled = true
	screen.active_store = ActiveBattleStore.new(WorkspacePaths.persistent_file("combined-attack-statuses.json"))
	root.add_child(screen); screen.set_process(false)
	for phase in ["defense_selection", "defense_reaction", "damage_reaction"]:
		screen._view.stage = phase
		screen._view.segment = "damage_resolution" if phase == "damage_reaction" else "defensive"
		screen._render(); await process_frame
		var found := false
		# Defense lists each incoming source in the lower-left station; Damage keeps its target panel.
		var labels: Array = screen._center.find_children("*", "Label", true, false).filter(func(l): return l.get_meta("flow_part", "") == "statuses")
		if phase != "damage_reaction":
			var row: Node = screen._incoming_attack_list.find_child("IncomingAttack_sword-cut", true, false)
			labels = [] if row == null else row.find_children("*", "Label", true, false).filter(func(l): return "pending" in l.text and l.is_visible_in_tree())
			var intent = screen._attack_intents.get("sword-cut")
			_expect(intent != null and _count(intent, "bleed") == "×2" and _count(intent, "poison") == "×3", phase + ": intent symbols sum per status for this recipient only")
		for label in labels:
			found = true
			_expect(label.text.count("Bleed ×2 pending") == 1, phase + ": one combined Bleed ×2 line")
			_expect(label.text.count("Poison ×3 pending") == 1, phase + ": different statuses stay separate")
			_expect(label.text.split("\n").size() == 2, phase + ": other recipient excluded and no duplicate lines")
		_expect(found, phase + ": attack status label rendered")
	_expect(screen._view.actor("goblin").offensive_outcome.status_applications == applications, "display does not change individual authority applications")
	screen.active_store.clear(); screen.queue_free(); await process_frame
	print("COMBINED ATTACK STATUSES: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)

func _expect(condition: bool, message: String) -> void:
	if not condition: failed = true; push_error("COMBINED ATTACK STATUSES: " + message)

func _count(panel: Control, effect: String) -> String:
	var children: Array = panel.intent_row.get_children()
	for i in range(children.size() - 1):
		if children[i].get_meta("intent_effect", "") == effect and children[i + 1] is Label: return children[i + 1].text
	return ""
