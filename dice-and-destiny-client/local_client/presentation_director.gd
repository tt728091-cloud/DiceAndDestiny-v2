class_name BattlePresentationDirector
extends RefCounted

const MEANINGFUL := {
	"segment_entered": true,
	"cards_drawn": true,
	"energy_points_gained": true,
	"status_changed": true,
	"battle_completed": true,
}

var _queue: Array = []
var _last_sequence := 0

func queue_result(result: Dictionary, already_presented_sequence: int = 0) -> void:
	_last_sequence = maxi(_last_sequence, already_presented_sequence)
	var ordered: Array = result.get("events", []).duplicate(true)
	ordered.sort_custom(func(a, b): return int(a.get("sequence", 0)) < int(b.get("sequence", 0)))
	var viewer_pending_value = result.get("pending_input", {}).get("blade", {})
	var viewer_pending: Dictionary = viewer_pending_value if viewer_pending_value is Dictionary else {}
	var pending_segment := str(viewer_pending.get("segment", ""))
	var automatic_segment := ""
	for event in ordered:
		if event.get("type") == "segment_entered": automatic_segment = str(event.get("segment", event.get("to", "")))
		var sequence := int(event.get("sequence", 0))
		if sequence > 0 and sequence <= _last_sequence: continue
		var kind := str(event.get("type", ""))
		var event_segment := automatic_segment
		if event_segment.is_empty(): event_segment = str(event.get("segment", ""))
		if (kind == "dice_rolled" and str(event.get("segment", automatic_segment)) == "defensive") or kind == "defense_selected":
			_remove_segment_placeholder("defensive")
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if kind in ["damage_cards_revealed", "damage_prevented_or_modified", "damage_committed", "cards_permanently_removed"]:
			_remove_segment_placeholder(event_segment)
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if event.get("type") == "segment_entered" and automatic_segment == pending_segment and not viewer_pending.is_empty():
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if str(event.get("type", "")) in ["cards_drawn", "energy_points_gained"] and automatic_segment == "income":
			_queue_income_event(event, sequence)
			continue
		if not _is_meaningful(event, automatic_segment):
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if event.get("type") != "segment_entered": _remove_segment_placeholder(event_segment)
		var beat := _beat(event)
		beat["watermark"] = sequence
		beat["presentation_segment"] = event_segment
		beat["segment_placeholder"] = event.get("type") == "segment_entered"
		_queue.append(beat)

func _queue_income_event(event: Dictionary, sequence: int) -> void:
	_remove_segment_placeholder("income")
	var round_number := int(event.get("round", 0))
	var beat: Dictionary
	if not _queue.is_empty() and _queue[-1].get("type") == "income_summary" and int(_queue[-1].get("round", 0)) == round_number:
		beat = _queue[-1]
	else:
		var summary_event := {"sequence": sequence, "type": "income_summary", "segment": "income", "round": round_number, "data": {"actors": {}}}
		beat = _beat(summary_event)
		beat["round"] = round_number
		beat["presentation_segment"] = "income"
		_queue.append(beat)
	var summary: Dictionary = beat.get("event", {})
	var data: Dictionary = summary.get("data", {})
	var actors: Dictionary = data.get("actors", {})
	var actor_id := str(event.get("actor_id", ""))
	var actor: Dictionary = actors.get(actor_id, {"cards": [], "card_count": 0})
	if event.get("type") == "cards_drawn":
		var cards_value = event.get("cards", [])
		var cards: Array = cards_value if cards_value is Array else []
		actor["cards"].append_array(cards)
		actor["card_count"] = int(actor.get("card_count", 0)) + maxi(int(event.get("count", 0)), cards.size())
	else:
		actor["energy_points"] = int(event.get("energy_points", 0))
		# Income currently grants one energy. Keeping the delta in the summary lets
		# the presentation count from the prior value to the authoritative total.
		actor["energy_gain"] = int(event.get("amount", 1))
	actors[actor_id] = actor
	data["actors"] = actors
	summary["data"] = data
	beat["event"] = summary
	beat["watermark"] = sequence

func _remove_segment_placeholder(segment_id: String) -> void:
	if segment_id.is_empty(): return
	for index in range(_queue.size() - 1, -1, -1):
		if bool(_queue[index].get("segment_placeholder", false)) and str(_queue[index].get("presentation_segment", "")) == segment_id:
			_queue.remove_at(index)
			return

func has_beats() -> bool:
	return not _queue.is_empty()

func peek() -> Dictionary:
	return _queue[0] if not _queue.is_empty() else {}

func pending_income_actor(actor_id: String) -> Dictionary:
	for beat_value in _queue:
		var beat: Dictionary = beat_value
		if beat.get("type") != "income_summary": continue
		var event: Dictionary = beat.get("event", {})
		var data: Dictionary = event.get("data", {})
		var actors: Dictionary = data.get("actors", {})
		var actor_value = actors.get(actor_id, {})
		return actor_value if actor_value is Dictionary else {}
	return {}

func advance() -> Dictionary:
	if _queue.is_empty(): return {}
	var beat: Dictionary = _queue.pop_front()
	_last_sequence = maxi(_last_sequence, int(beat.get("watermark", beat.get("sequence", 0))))
	return peek()

func clear() -> void:
	_queue.clear()

func last_sequence() -> int:
	return _last_sequence

func _is_meaningful(event: Dictionary, automatic_segment: String = "") -> bool:
	var kind := str(event.get("type", ""))
	if kind in ["cards_drawn", "energy_points_gained"] and automatic_segment != "income": return false
	if kind == "segment_entered":
		return str(event.get("to", event.get("segment", ""))) in ["ongoing_effects", "income", "defensive", "damage_resolution"]
	return MEANINGFUL.has(kind)

func _beat(event: Dictionary) -> Dictionary:
	var kind := str(event.get("type", ""))
	var title := kind.replace("_", " ").capitalize()
	var detail := ""
	match kind:
		"segment_entered":
			var segment := str(event.get("to", event.get("segment", "")))
			if segment == "ongoing_effects":
				title = "Ongoing Effects"
				detail = "No ongoing effects to resolve"
			elif segment == "income":
				title = "Income"
				detail = "No income changes this round"
			elif segment == "defensive":
				title = "Defensive Segment"
				detail = "No attacks require a defense"
			else:
				title = "Damage Resolution"
				detail = "No damage to resolve"
		"cards_drawn":
			title = "Card Drawn"
			if str(event.get("actor_id", "")) == "blade" and not event.get("cards", []).is_empty(): detail = "Blade Warden drew %s" % str(event.cards[0])
			else: detail = "Venom Goblin drew %d hidden card%s" % [maxi(1, int(event.get("count", 1))), "s" if int(event.get("count", 1)) != 1 else ""]
		"energy_points_gained":
			title = "Energy Gained"
			var actor_name := "Blade Warden" if str(event.get("actor_id", "")) == "blade" else "Venom Goblin"
			detail = "%s energy is now %d" % [actor_name, int(event.get("energy_points", 0))]
		"income_summary":
			title = "Income Results"
			detail = "Both combatants receive their round income"
		"status_changed": detail = JSON.stringify(event.get("data", {}))
		"battle_completed":
			title = str(event.get("battle_result", "Battle Complete")).capitalize()
			detail = "The battle is complete."
	return {"sequence": int(event.get("sequence", 0)), "type": kind, "title": title, "detail": detail, "event": event}
