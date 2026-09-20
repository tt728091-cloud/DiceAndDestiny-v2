extends RefCounted

# Only consumes the same viewer-filtered results as BattleViewState. Never use
# the developer transcript here: it contains both players' private information.
var entries: Array[Dictionary] = []
var _battle_id := ""
var _seen := {}
var _actors := {}
var _viewer := "blade"
var _sequence := 0
var _context := ""
var _round := 0
var _step_events := {}
var _change_lines := {}

func receive(result: Dictionary) -> void:
	var snapshot: Dictionary = result.get("snapshot", {})
	var battle_id := str(snapshot.get("battle_id", ""))
	if battle_id != _battle_id:
		entries.clear(); _seen.clear(); _actors.clear(); _sequence = 0
		_battle_id = battle_id
	_viewer = str(snapshot.get("viewer_actor_id", "blade"))
	var after: Dictionary = snapshot.get("actors", {})
	var previous := _actors
	_actors = after.duplicate(true)
	_round = int(snapshot.get("round", 0))
	_step_events.clear(); _change_lines.clear()
	_context = "Round %d · %s" % [_round, _human(str(snapshot.get("segment", "")))]
	var events: Array = result.get("events", []).duplicate(true)
	events.sort_custom(func(a, b): return int(a.get("sequence", 0)) < int(b.get("sequence", 0)))
	var plays := {}; var draws := {}
	for event in events:
		var key := str(event.get("event_id", ""))
		if key.is_empty(): key = str(int(event.sequence)) if event.has("sequence") else JSON.stringify(event)
		if _seen.has(key): continue
		_seen[key] = true
		_sequence = maxi(_sequence, int(event.get("sequence", 0)))
		var owner := str(event.get("actor_id", ""))
		if not str(event.get("private_actor_id", "")).is_empty() and str(event.private_actor_id) != _viewer: continue
		_step_events[_step_key(event)] = true
		if event.get("type") == "card_played": plays[owner] = int(plays.get(owner, 0)) + 1
		if event.get("type") == "cards_drawn": draws[owner] = true
		_event(event)
	# Snapshot deltas cover effects that have no dedicated public event (card
	# resource costs, gains, upgrades, status caps and public pile counts).
	for actor in after:
		if not previous.has(actor):
			if draws.has(actor): _add("%s: hand size %d" % [_name(actor), _count(after[actor], "hand")])
			continue
		var before: Dictionary = previous[actor]
		var current: Dictionary = after[actor]
		var hand_before := _count(before, "hand"); var hand_after := _count(current, "hand")
		var inferred_draw := hand_after - hand_before + int(plays.get(actor, 0))
		if inferred_draw > 0 and not draws.has(actor):
			var known: Array = []
			if actor == _viewer:
				for card in current.get("hand", []):
					if card not in before.get("hand", []): known.append(card)
			_add("%s drew %d card(s)%s; hand size %d" % [_name(actor), inferred_draw, _draw_names(actor, known), hand_after])
		elif draws.has(actor):
			_add("%s: hand size %d" % [_name(actor), hand_after])
		_changes(actor, before, current)

func text() -> String:
	var lines: Array[String] = []
	var context := ""
	for entry in entries:
		if entry.context != context:
			context = entry.context
			lines.append(context)
		lines.append("• " + str(entry.text))
	return "\n".join(lines) if not lines.is_empty() else "Battle ready."

func _add(line: String) -> void:
	if not line.is_empty(): entries.append({"context": _context, "sequence": _sequence, "text": line})

func _event(event: Dictionary) -> void:
	var private_owner := str(event.get("private_actor_id", ""))
	if not private_owner.is_empty() and private_owner != _viewer: return
	var data: Dictionary = event.get("data", {})
	var actor := str(event.get("actor_id", ""))
	var kind := str(event.get("type", ""))
	var saved_context := _context
	if event.has("round") or event.has("segment"):
		_context = "Round %d · %s" % [int(event.get("round", _round)), _human(str(event.get("segment", event.get("to", ""))))]
	match kind:
		"segment_entered": _add("%s begins" % _human(str(event.get("to", event.get("segment", "")))))
		"card_played":
			_add("%s played %s" % [_name(actor), _content("card", str(data.get("card_definition_id", "")))])
			if data.has("die_index"):
				_add("%s's die %d changed to %d" % [_name(str(data.get("actor_id", actor))), int(data.die_index) + 1, int(data.get("face", 0))])
			if data.has("old_ability") and data.get("old_ability") != data.get("new_ability"):
				_add("Attack changed: %s → %s" % [_content("ability", str(data.get("old_ability", ""))), _content("ability", str(data.get("new_ability", "")))])
			if int(data.get("stacks_removed", 0)) > 0:
				_add_change("%s: %s %d → %d" % [_name(actor), _content("status", str(data.get("choice_id", ""))), int(data.get("stacks_before", 0)), int(data.get("stacks_after", 0))])
		"cards_drawn":
			# Even if a malformed event includes enemy card IDs, never print them.
			var count := int(event.get("count", event.get("cards", []).size()))
			_add("%s drew %d card(s)%s" % [_name(actor), count, _draw_names(actor, event.get("cards", []))])
		"energy_points_gained": _add("%s: Energy now %d" % [_name(actor), int(event.get("energy_points", 0))])
		"discard_reshuffled": _add("%s shuffled their discard into their deck" % _name(actor))
		"ability_selected": _add("%s selected %s" % [_name(actor), _content("ability", str(data.get("ability_id", event.get("source_id", ""))))])
		"defense_selected":
			_add("%s defended with %s; rolled %s" % [_name(actor), _content("ability", str(data.get("ability_id", ""))), str(data.get("rolled_faces", [data.get("rolled_face", 0)]))])
		"dice_rolled":
			if data.get("hidden", false):
				_context = saved_context
				return
			if data.get("source_type") == "catalyst":
				_add("%s spent 1 Catalyst to reroll %s's %s die: %d → %s" % [_name(str(data.get("holder", ""))), _name(actor), _content("status", str(data.get("status_id", "poison"))), int(data.get("face_before", 0)), _faces(event.get("dice", []))])
			elif not event.get("dice", []).is_empty(): _add("%s rolled %s" % [_name(actor), _faces(event.dice)])
			_rolls(data.get("rolls", []))
		"interaction_revealed":
			for owner in data.get("commitments", {}):
				var selection: Dictionary = data.commitments[owner]
				_add("%s revealed %s; dice %s" % [_name(owner), _content("ability", str(selection.get("ability_id", ""))), _faces(selection.get("dice", []))])
				var outcome: Dictionary = selection.get("outcome", {})
				if outcome.has("base_damage"): _add("%s: %d attack damage" % [_name(owner), int(outcome.base_damage)])
		"damage_prevented_or_modified":
			_add("%s: incoming damage %d → %d" % [_name(actor), int(data.get("damage_before", 0)), int(data.get("damage_after", 0))])
		"damage_proposed": _add("%s: %d damage pending from %s" % [_name(str(event.get("target_actor_id", ""))), int(event.get("amount", 0)), _content("ability", str(event.get("source_id", "")))])
		"damage_committed":
			for source in _array(data.get("sources")):
				_add("%s took %d damage from %s (%d prevented)" % [_name(str(source.get("target_actor_id", ""))), int(source.get("final_amount", 0)), _content("ability", str(source.get("source_content_id", ""))), int(source.get("prevention", 0)) + int(source.get("reaction_prevention", 0))])
			for card in _array(data.get("removals")):
				if card.get("accepted", false) and not card.get("released", false):
					_add("%s lost %s from %s" % [_name(str(card.get("target_actor_id", ""))), _content("card", str(card.get("card_definition_id", ""))), _human(str(card.get("original_zone", "deck")))])
			var overage: Dictionary = data.get("overage") if data.get("overage") is Dictionary else {}
			for owner in overage:
				if int(data.overage[owner]) > 0:
					_add("%s: %d excess damage; no cards left to remove" % [_name(owner), int(data.overage[owner])])
		"proposal_batch_committed":
			_rolls(data.get("rolls", []))
			for key in ["status_application", "incubation_application"]:
				var application: Dictionary = data.get(key, {})
				if not application.is_empty(): _add_change("%s: %s %d → %d" % [_name(str(application.get("target_actor_id", ""))), _content("status", str(application.get("status_id", "incubation"))), int(application.get("before", 0)), int(application.get("after", 0))])
			var conversion: Dictionary = data.get("poison_conversion", {})
			if conversion.get("converted", false): _add("%s: Poison → Volatile Poison" % _name(str(conversion.get("target_actor_id", ""))))
		"effects_resolved":
			# Summary steps also exist as top-level events in normal play. Only
			# expand them when loading a summary-only response/fixture.
			for step in (data.get("steps") if data.get("steps") is Array else []):
				if _step_events.has(_step_key(step)): continue
				_step_events[_step_key(step)] = true
				_event(step)
			for owner in data.get("actors_before", {}):
				_changes(owner, data.actors_before[owner], data.get("actors_after", {}).get(owner, {}))
		"battle_completed": _add("Battle ended: %s" % _human(str(event.get("battle_result", data.get("result", "complete")))) )
	_context = saved_context

func _changes(actor: String, before: Dictionary, after: Dictionary) -> void:
	if after.is_empty(): return
	var selected := str(after.get("selected_ability", ""))
	if selected != str(before.get("selected_ability", "")) and not selected.is_empty():
		_add_change("%s selected %s" % [_name(actor), _content("ability", selected)])
	var kept := _kept_indices(after)
	if actor == _viewer and kept != _kept_indices(before):
		var numbers: Array = []
		for index in kept: numbers.append(int(index) + 1)
		_add_change("%s kept dice %s" % [_name(actor), str(numbers)])
	for modifier in after.get("ability_modifiers", []):
		if modifier not in before.get("ability_modifiers", []):
			_add_change("%s upgraded %s" % [_name(actor), _content("ability", str(modifier.get("ability_id", "")))])
	for field in ["health", "energy", "hand", "deck", "discard", "removed"]:
		var old := _value(before, field); var current := _value(after, field)
		if old != current: _add_change("%s: %s %d → %d" % [_name(actor), _human(field) + (" size" if field in ["hand", "deck", "discard", "removed"] else ""), old, current])
	var old_statuses := _statuses(before); var new_statuses := _statuses(after)
	var keys := old_statuses.keys()
	for id in new_statuses:
		if id not in keys: keys.append(id)
	for id in keys:
		var old := int(old_statuses.get(id, 0)); var current := int(new_statuses.get(id, 0))
		if old != current: _add_change("%s: %s %d → %d" % [_name(actor), _content("status", id), old, current])
	for id in after.get("ability_levels", {}):
		var old := int(before.get("ability_levels", {}).get(id, 0)); var current := int(after.ability_levels[id])
		if old != current: _add_change("%s: %s level %d → %d" % [_name(actor), _content("ability", id), old, current])

func _kept_indices(actor: Dictionary) -> Array:
	var dice = actor.get("dice", {})
	return dice.get("kept_indices", []) if dice is Dictionary else actor.get("kept_indices", [])

func _array(value: Variant) -> Array:
	return value if value is Array else []

func _rolls(rolls: Array) -> void:
	for roll in rolls:
		_add("%s: %s rolled %d" % [_name(str(roll.get("actor_id", ""))), _content("status", str(roll.get("source_content_id", ""))), int(roll.get("die", {}).get("face", 0))])

func _value(actor: Dictionary, field: String) -> int:
	if field == "health": return int(actor.get("current_health", actor.get("health", 0)))
	if field == "energy": return int(actor.get("energy_points", actor.get("resources", {}).get("energy_points", actor.get("energy", 0))))
	return _count(actor, field)

func _count(actor: Dictionary, pile: String) -> int:
	return int(actor.get(pile + "_count", actor.get(pile, []).size()))

func _statuses(actor: Dictionary) -> Dictionary:
	var result := {}
	for status in actor.get("statuses", []):
		var id := str(status.get("definition_id", ""))
		result[id] = int(result.get(id, 0)) + int(status.get("stacks", 0))
	return result

func _name(actor: String) -> String:
	if _actors.has("goblin-2") and actor in ["goblin", "goblin-2"]: return "Brine Mask 1" if actor == "goblin" else "Brine Mask 2"
	var data: Dictionary = _actors.get(actor, {})
	return str(data.get("character", {}).get("name", _human(str(data.get("definition_id", actor)))))

func _content(kind: String, id: String) -> String:
	if id.is_empty(): return "no attack" if kind == "ability" else "card"
	if kind == "card": return str(BattlePresentationCatalog.card(id).get("name", _human(id)))
	if kind == "status": return str(BattlePresentationCatalog.status(id).get("name", _human(id)))
	return str(BattlePresentationCatalog.ability(id).get("name", _human(id)))

func _faces(dice: Array) -> String:
	var faces: Array[String] = []
	for die in dice: faces.append(str(die.get("face", 0)))
	return ", ".join(faces)

func _human(value: String) -> String: return value.replace("_", " ").capitalize()

func _add_change(line: String) -> void:
	if _change_lines.has(line): return
	_change_lines[line] = true
	_add(line)

func _step_key(event: Dictionary) -> String:
	return JSON.stringify([event.get("type"), event.get("actor_id"), event.get("data", {}), event.get("dice", [])])

func _draw_names(actor: String, cards: Array) -> String:
	if actor != _viewer: return ""
	var names: Array[String] = []
	for card in cards:
		var definition := str(_actors.get(actor, {}).get("card_instances", {}).get(str(card), {}).get("definition_id", ""))
		if not definition.is_empty(): names.append(_content("card", definition))
	return " (" + ", ".join(names) + ")" if not names.is_empty() else ""
