class_name BattlePresentationDirector
extends RefCounted

# Completion goes straight to the viewer's results screen after queued effects;
# it must not add a second result screen or a presentation acknowledgement.
const MEANINGFUL := {
	"segment_entered": true,
	"cards_drawn": true,
	"energy_points_gained": true,
	"status_changed": true,
}

var _queue: Array = []
var _status_updates: Array[Dictionary] = []
var _last_sequence := 0
# Received events must not replay while their presentation watermark is pending.
var _last_received_sequence := 0
var learned_battle_mode := false

func configure_learned_battle(enabled: bool) -> void:
	learned_battle_mode = enabled

func queue_result(result: Dictionary, already_presented_sequence: int = 0, previous_actors: Dictionary = {}) -> void:
	_last_sequence = maxi(_last_sequence, already_presented_sequence)
	var ordered: Array = result.get("events", []).duplicate(true)
	ordered.sort_custom(func(a, b): return int(a.get("sequence", 0)) < int(b.get("sequence", 0)))
	var viewer_pending_value = result.get("pending_input", {}).get("blade", {})
	var viewer_pending: Dictionary = viewer_pending_value if viewer_pending_value is Dictionary else {}
	var pending_segment := str(viewer_pending.get("segment", ""))
	var snapshot_value = result.get("snapshot", {})
	var snapshot: Dictionary = snapshot_value if snapshot_value is Dictionary else {}
	var active_segment := str(snapshot.get("segment", ""))
	# Segment placeholders describe segments that the authority traversed without
	# stopping. A segment that remains active is real work, even when its current
	# pending input belongs to the hidden learned opponent rather than the viewer.
	# Clear an older placeholder as soon as a later result stops in that segment.
	if not active_segment.is_empty(): _remove_segment_placeholder(active_segment)
	var has_effects_summary := false
	for emitted in ordered:
		if emitted.get("type") == "effects_resolved": has_effects_summary = true
	var automatic_segment := ""
	var fresh_events: Array = []
	var damage_actors := previous_actors.duplicate(true)
	for event in ordered:
		if event.get("type") == "segment_entered": automatic_segment = str(event.get("segment", event.get("to", "")))
		var sequence := int(event.get("sequence", 0))
		if sequence > 0 and sequence <= maxi(_last_sequence, _last_received_sequence): continue
		_last_received_sequence = maxi(_last_received_sequence, sequence)
		fresh_events.append(event)
		var kind := str(event.get("type", ""))
		var event_segment := automatic_segment
		if event_segment.is_empty(): event_segment = str(event.get("segment", ""))
		if has_effects_summary and event_segment == "ongoing_effects" and kind != "effects_resolved":
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if kind == "proposal_batch_committed":
			var data: Dictionary = event.get("data", {})
			var conversion: Dictionary = data.get("poison_conversion", {})
			var incubation: Dictionary = data.get("incubation_application", {})
			var application: Dictionary = data.get("status_application", {})
			if int(application.get("after", 0)) > int(application.get("before", 0)): _status_updates.append({"kind": "application", "data": application})
			if conversion.get("converted", false): _status_updates.append({"kind": "conversion", "data": conversion})
			if int(incubation.get("after", 0)) > int(incubation.get("before", 0)): _status_updates.append({"kind": "incubation", "data": incubation})
		if (kind == "dice_rolled" and str(event.get("segment", automatic_segment)) == "defensive") or kind == "defense_selected":
			_remove_segment_placeholder("defensive")
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if kind == "damage_committed" and event_segment == "damage_resolution":
			_remove_segment_placeholder(event_segment)
			var data: Dictionary = event.get("data", {}).duplicate(true)
			# Go's empty slices can arrive as JSON null (not a missing key).
			# Keep blocked sources/status applications as a real Damage beat,
			# and normalize its collections for every downstream presenter.
			for field in ["sources", "removals", "status_applications"]:
				data[field] = _array(data.get(field))
			if not data.get("sources", []).is_empty() or not data.get("removals", []).is_empty() or not data.get("status_applications", []).is_empty():
				data["actors_before"] = _damage_baseline(event, ordered, damage_actors, snapshot)
				damage_actors = _project_removals(data.actors_before, data.get("removals", []), -1)
				var damage_event: Dictionary = event.duplicate(true); damage_event["data"] = data
				_queue.append({"type": "combat_damage", "title": "Damage", "presentation_segment": "damage_resolution", "event": damage_event, "watermark": sequence})
				continue
		if kind in ["damage_cards_revealed", "damage_prevented_or_modified", "damage_committed", "cards_permanently_removed"]:
			_remove_segment_placeholder(event_segment)
			if _queue.is_empty(): _last_sequence = maxi(_last_sequence, sequence)
			else: _queue[-1]["watermark"] = sequence
			continue
		if event.get("type") == "segment_entered" and (automatic_segment == active_segment or (automatic_segment == pending_segment and not viewer_pending.is_empty())):
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

	_queue_card_gains(fresh_events, previous_actors, snapshot.get("actors", {}))

func _damage_baseline(event: Dictionary, ordered: Array, previous: Dictionary, snapshot: Dictionary) -> Dictionary:
	# The last shown authority state is already before this damage batch. Never
	# replace it with a reconstruction from a later round's Effects summary.
	if not previous.is_empty(): return previous.duplicate(true)
	var sequence := int(event.get("sequence", 0))
	var baseline := previous.duplicate(true)
	var reverse_cards: Array = []
	# A future Effects baseline is authoritative immediately after combat. Walk
	# back only the combat removals still awaiting presentation, never poison.
	for later in ordered:
		if int(later.get("sequence", 0)) < sequence: continue
		if later.get("type") == "damage_committed" and later.get("segment") == "damage_resolution": reverse_cards.append_array(_array(later.get("data", {}).get("removals")))
		if later.get("type") == "effects_resolved":
			for actor_id in later.get("data", {}).get("actors_before", {}):
				var final: Dictionary = later.data.actors_before[actor_id]
				var actor: Dictionary = baseline.get(actor_id, {}).duplicate(true)
				actor["current_health"] = final.get("health", 0)
				for field in ["deck_count", "hand_count", "discard_count", "removed_count"]: actor[field] = final.get(field, 0)
				if not actor.has("statuses"): actor["statuses"] = final.get("statuses", [])
				baseline[actor_id] = actor
			return _project_removals(baseline, reverse_cards, 1)
	if not baseline.is_empty(): return baseline
	# Restoring a save can provide the final snapshot and unpresented events,
	# without the previous in-memory view. Reconstruct the card-count baseline.
	return _project_removals(snapshot.get("actors", {}), reverse_cards, 1)

func _array(value: Variant) -> Array:
	return value if value is Array else []

func _project_removals(actors: Dictionary, cards: Array, direction: int) -> Dictionary:
	var projected := actors.duplicate(true)
	var seen := {}
	for card in cards:
		var target := str(card.get("target_actor_id", ""))
		var id := str(card.get("card_id", ""))
		if not projected.has(target) or not card.get("accepted", false) or card.get("released", false) or seen.has(id): continue
		seen[id] = true
		var actor: Dictionary = projected[target]
		actor["current_health"] = maxi(0, int(actor.get("current_health", 0)) + direction)
		actor["removed_count"] = maxi(0, int(actor.get("removed_count", 0)) - direction)
		var zone := str(card.get("original_zone", "deck")).to_lower() + "_count"
		actor[zone] = maxi(0, int(actor.get(zone, 0)) + direction)
	return projected

func _queue_card_gains(events: Array, before: Dictionary, after: Dictionary) -> void:
	if before.is_empty(): return
	var played: Array = []
	var card_events: Array = []
	var paid_energy := {}
	var played_count := {}
	var seen_cards := {}
	for event in events:
		# These have their own coordinated animations; never infer card rewards
		# from a snapshot that also includes next-round income or rolled defenses.
		if event.get("type") in ["segment_entered", "effects_resolved", "defense_selected"]: return
		if event.get("type") in ["card_played", "damage_prevented_or_modified"]:
			var card_id := str(event.get("data", {}).get("card_definition_id", ""))
			var instance := str(event.get("data", {}).get("card_instance_id", ""))
			if not card_id.is_empty() and not seen_cards.has(instance):
				seen_cards[instance] = true
				played.append(card_id)
				card_events.append({"definition_id": card_id, "actor_id": str(event.get("actor_id", "")), "sequence": int(event.get("sequence", 0)), "instance_id": instance})
				var owner := str(event.get("actor_id", ""))
				paid_energy[owner] = int(paid_energy.get(owner, 0)) + int(event.get("energy_cost", BattlePresentationCatalog.card(card_id).cost))
				played_count[owner] = int(played_count.get(owner, 0)) + 1
	if played.is_empty(): return
	var names: Array[String] = []
	for card_id in played: names.append(str(BattlePresentationCatalog.card(card_id).name))
	var card_name := ", ".join(names)
	var feedback := {"key": JSON.stringify(card_events), "cards": card_events}
	var first_update := _status_updates.size()
	for actor_id in after:
		if not before.has(actor_id): continue
		var initial: Dictionary = before[actor_id]
		var final: Dictionary = after[actor_id]
		var counts := {}
		for status in initial.get("statuses", []): counts[str(status.get("definition_id", ""))] = int(status.get("stacks", 0))
		for status in final.get("statuses", []):
			var id := str(status.get("definition_id", ""))
			var start := int(counts.get(id, 0)); var finish := int(status.get("stacks", 0))
			if finish <= start: continue
			var already_animated := false
			for update in _status_updates:
				if str(update.data.get("target_actor_id", "")) != str(actor_id): continue
				if str(update.data.get("status_id", "incubation" if update.kind == "incubation" else "volatile_poison")) == id:
					already_animated = true
					update["card_name"] = card_name
					update["card_feedback"] = feedback
			if not already_animated:
				_status_updates.append({"kind": "application", "card_name": card_name, "data": {"target_actor_id": actor_id, "status_id": id, "before": start, "after": finish}})
		for pair in [["energy_points", "energy", "Energy"], ["hand_count", "hand", "cards"]]:
			var start := int(initial.get(pair[0], 0)); var finish := int(final.get(pair[0], 0))
			var gained := finish - start + int(paid_energy.get(actor_id, 0) if pair[1] == "energy" else played_count.get(actor_id, 0))
			# Playing a card can hide a draw in the net hand-size change. Public
			# draw events also work for opponents without exposing hidden card IDs.
			if pair[1] == "hand":
				var drawn := 0
				for event in events:
					if event.get("type") == "cards_drawn" and str(event.get("actor_id", "")) == str(actor_id):
						drawn += maxi(int(event.get("count", 0)), event.get("cards", []).size())
				gained = maxi(gained, drawn)
			if gained > 0:
				_status_updates.append({"kind": "resource", "card_name": card_name, "data": {"target_actor_id": actor_id, "stat": pair[1], "caption": pair[2], "amount": gained, "before": maxi(0, finish - gained), "after": finish}})

	for index in range(first_update, _status_updates.size()): _status_updates[index]["card_feedback"] = feedback

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

func has_pending_card_cleanse() -> bool:
	for beat in _queue:
		if beat.get("type") == "card_cleanse": return true
	return false

func has_pending_status_animation() -> bool:
	for beat in _queue:
		if beat.get("type") in ["card_cleanse", "poison_conversion", "effects_resolved"]: return true
	return false

func peek() -> Dictionary:
	return _queue[0] if not _queue.is_empty() else {}

func pending_damage_actor_before(actor_id: String) -> Dictionary:
	for beat in _queue:
		if beat.get("type") == "combat_damage": return beat.get("event", {}).get("data", {}).get("actors_before", {}).get(actor_id, {})
	return {}

func pending_effects_actor_before(actor_id: String) -> Dictionary:
	# The authority can finish the next round while earlier segment beats are
	# still queued. Keep profiles before that Effects batch until it is shown.
	for beat in _queue:
		if beat.get("type") != "effects_resolved": continue
		var before = beat.get("event", {}).get("data", {}).get("actors_before", {}).get(actor_id, {})
		return before if before is Dictionary else {}
	return {}

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
	if kind == "effects_resolved": return true
	if kind == "proposal_batch_committed": return false
	if kind == "card_played":
		var data: Dictionary = event.get("data", {})
		return data.get("operation") == "remove_status" and int(data.get("stacks_removed", 0)) > 0 and not str(data.get("choice_id", "")).is_empty()
	if kind in ["cards_drawn", "energy_points_gained"] and automatic_segment != "income": return false
	if kind == "segment_entered":
		return str(event.get("to", event.get("segment", ""))) in ["ongoing_effects", "income", "defensive", "damage_resolution"]
	return MEANINGFUL.has(kind)

func _beat(event: Dictionary) -> Dictionary:
	var kind := str(event.get("type", ""))
	if kind == "card_played": kind = "card_cleanse"
	if kind == "proposal_batch_committed": kind = "poison_conversion"
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
			else: detail = "%s drew %d hidden card%s" % ["Learned Blade Warden" if learned_battle_mode else "Venom Goblin", maxi(1, int(event.get("count", 1))), "s" if int(event.get("count", 1)) != 1 else ""]
		"energy_points_gained":
			title = "Energy Gained"
			var actor_name := "Blade Warden" if str(event.get("actor_id", "")) == "blade" else "Learned Blade Warden" if learned_battle_mode else "Venom Goblin"
			detail = "%s energy is now %d" % [actor_name, int(event.get("energy_points", 0))]
		"income_summary":
			title = "Income Results"
			detail = "Both combatants receive their round income"
		"status_changed": detail = JSON.stringify(event.get("data", {}))
	return {"sequence": int(event.get("sequence", 0)), "type": kind, "title": title, "detail": detail, "event": event}

func take_status_updates() -> Array[Dictionary]:
	var updates := _status_updates
	_status_updates = []
	return updates
