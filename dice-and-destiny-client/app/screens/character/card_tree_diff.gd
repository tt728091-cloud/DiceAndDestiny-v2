extends RefCounted
## Describes how one tree card differs from its predecessor. Changes are read
## from the structured definition, never inferred from price or card names.
const LOWER_IS_BETTER := ["cost_stacks", "threshold", "minimum"]

static func changes(before: Dictionary, after: Dictionary, catalog: Dictionary) -> Array:
	var out: Array = []
	if before.is_empty() or after.is_empty(): return out
	var energy_a := int(before.get("cost", {}).get("energy", 0)); var energy_b := int(after.get("cost", {}).get("energy", 0))
	if energy_a != energy_b: out.append(_entry("Energy %d → %d" % [energy_a, energy_b], energy_b < energy_a))
	if before.get("program") is Dictionary and after.get("program") is Dictionary:
		_steps(out, before.program.get("steps", []), after.program.get("steps", []), catalog, "")
		_timing(out, before.program, after.program)
	elif before.get("mechanic") is Dictionary and after.get("mechanic") is Dictionary:
		var spec: Dictionary = catalog.get("mechanics", {}).get(str(after.mechanic.get("kind", "")), {})
		var params_a: Dictionary = before.mechanic.get("params", {}); var params_b: Dictionary = after.mechanic.get("params", {})
		var keys: Array = spec.get("parameters", {}).keys(); keys.sort()
		for key in keys:
			var a = params_a.get(key, spec.parameters[key].get("default")); var b = params_b.get(key, spec.parameters[key].get("default"))
			if str(a) != str(b): out.append(_value_change(_caption(key), a, b, key))
		_timing(out, before.mechanic, after.mechanic)
	var dest_a := str(before.get("play", {}).get("destination", "discard")); var dest_b := str(after.get("play", {}).get("destination", "discard"))
	if dest_a != dest_b: out.append(_entry("After play: %s → %s" % [_word(dest_a), _word(dest_b)], null if dest_b != "removed" else false))
	var saved_a := str(before.get("saved_card_destination", "")); var saved_b := str(after.get("saved_card_destination", ""))
	if saved_a != saved_b and not (saved_a + saved_b).is_empty(): out.append(_entry("Saved cards: %s → %s" % [_word(saved_a if not saved_a.is_empty() else "original"), _word(saved_b if not saved_b.is_empty() else "original")], null))
	if out.is_empty() and str(before.get("presentation", {}).get("rules_text", "")) != str(after.get("presentation", {}).get("rules_text", "")):
		out.append(_entry("Rules changed", null))
	return out

## The full change list for one connection: effect changes plus the XP step.
## Kept apart from changes() so suggested names describe effects, not price.
static func with_xp(before: Dictionary, after: Dictionary, catalog: Dictionary) -> Array:
	var out := changes(before, after, catalog)
	if before.is_empty() or after.is_empty(): return out
	var xp_a := int(before.get("economy", {}).get("buy", 0)); var xp_b := int(after.get("economy", {}).get("buy", 0))
	if xp_a != xp_b: out.append(_entry("XP value: %d → %d (%+d XP)" % [xp_a, xp_b, xp_b - xp_a], null))
	return out

static func summary(before: Dictionary, after: Dictionary, catalog: Dictionary, limit: int = 2) -> String:
	var parts: Array = changes(before, after, catalog).map(func(c): return c.text)
	if parts.size() > limit: parts = parts.slice(0, limit) + ["+%d more" % (parts.size() - limit)]
	return " · ".join(parts)

static func bbcode(entries: Array) -> String:
	var lines: Array = []
	for c in entries:
		var color := "#8fe2af" if c.tone == "gain" else "#ef9e91" if c.tone == "loss" else "#e6c17c"
		var mark := "▲" if c.tone == "gain" else "▼" if c.tone == "loss" else "◆"
		lines.append("[color=%s]%s[/color] %s" % [color, mark, str(c.text).replace("[", "[lb]")])
	return "\n".join(lines)

static func plain(entries: Array) -> String:
	return "\n".join(entries.map(func(c): return "• " + str(c.text)))

static func step_phrase(step: Dictionary, catalog: Dictionary) -> String:
	var p: Dictionary = step.get("params", {})
	var spec: Dictionary = catalog.get("effects", {}).get(str(step.get("effect", "")), {})
	var amount := int(p.get("amount", spec.get("parameters", {}).get("amount", {}).get("default", 1)))
	match str(step.get("effect", "")):
		"prevent": return "Prevent %d damage" % amount
		"draw": return "Draw %d card%s" % [amount, "" if amount == 1 else "s"]
		"energy": return "Gain %d energy" % amount
		"curse": return "Apply %d Curse" % amount
		"remove_status":
			var stacks := int(p.get("stacks", 0))
			return "Remove all status stacks" if stacks == 0 else "Remove %d status stack%s" % [stacks, "" if stacks == 1 else "s"]
		"apply_status": return "Apply %d %s" % [int(p.get("stacks", 1)), _word(str(p.get("status_id", "status")))]
	return str(spec.get("label", _word(str(step.get("effect", "effect")))))

static func _steps(out: Array, a: Array, b: Array, catalog: Dictionary, prefix: String) -> void:
	var i := 0; var j := 0
	while i < a.size() or j < b.size():
		if i < a.size() and j < b.size() and a[i].effect == b[j].effect:
			_step(out, a[i], b[j], catalog, prefix); i += 1; j += 1
		elif j < b.size() and (i >= a.size() or not a.slice(i).any(func(s): return s.effect == b[j].effect)):
			out.append(_entry(prefix + "+ " + step_phrase(b[j], catalog), true)); j += 1
		else:
			out.append(_entry(prefix + "− " + step_phrase(a[i], catalog), false)); i += 1

static func _step(out: Array, a: Dictionary, b: Dictionary, catalog: Dictionary, prefix: String) -> void:
	var spec: Dictionary = catalog.get("effects", {}).get(str(b.effect), {})
	var label := prefix + _effect_name(str(b.effect), spec)
	var keys: Array = spec.get("parameters", {}).keys(); keys.sort()
	for key in keys:
		var default = spec.parameters[key].get("default")
		var va = a.get("params", {}).get(key, default); var vb = b.get("params", {}).get(key, default)
		if JSON.stringify(va) == JSON.stringify(vb): continue
		if key == "destination" and str(b.effect) in ["prevent", "save_cards"]:
			out.append(_entry(prefix + "Saved cards: %s → %s" % [_word(str(va)), _word(str(vb))], null))
		elif key == "amount": out.append(_value_change(label, va, vb, key))
		else: out.append(_value_change(label + " " + _caption(key).to_lower(), va, vb, key))
	var ta: Dictionary = a.get("target", {}); var tb: Dictionary = b.get("target", {})
	if int(ta.get("count", 1)) != int(tb.get("count", 1)) and str(tb.get("mode", "one")) in ["exact", "up_to"]:
		out.append(_value_change(label + " targets", ta.get("count", 1), tb.get("count", 1), "count"))
	for key in ["owner", "mode", "selection"]:
		if str(ta.get(key, "")) != str(tb.get(key, "")): out.append(_entry("%s target %s: %s → %s" % [label, key, _word(str(ta.get(key, ""))), _word(str(tb.get(key, "")))], null))
	var oa: Array = a.get("choices", []); var ob: Array = b.get("choices", [])
	for k in maxi(oa.size(), ob.size()):
		if k >= oa.size(): out.append(_entry(prefix + "+ Option: " + str(ob[k].name), true))
		elif k >= ob.size(): out.append(_entry(prefix + "− Option: " + str(oa[k].name), false))
		else:
			if int(oa[k].get("energy", 0)) != int(ob[k].get("energy", 0)): out.append(_entry("%s%s energy %d → %d" % [prefix, ob[k].name, int(oa[k].energy), int(ob[k].energy)], int(ob[k].energy) < int(oa[k].energy)))
			_steps(out, oa[k].get("steps", []), ob[k].get("steps", []), catalog, prefix + str(ob[k].name) + ": ")
	if JSON.stringify(a.get("condition", {})) != JSON.stringify(b.get("condition", {})): out.append(_entry(label + " condition changed", null))

static func _timing(out: Array, a: Dictionary, b: Dictionary) -> void:
	for key in ["uses_per_round", "uses_per_battle"]:
		var va := int(a.get(key, 0)); var vb := int(b.get(key, 0))
		if va == vb: continue
		var text := "%s: %s → %s" % ["Uses per round" if key == "uses_per_round" else "Uses per battle", "unlimited" if va == 0 else str(va), "unlimited" if vb == 0 else str(vb)]
		out.append(_entry(text, vb == 0 or (va != 0 and vb > va)))

static func _value_change(label: String, a, b, key: String) -> Dictionary:
	if (a is float or a is int) and (b is float or b is int):
		var better: bool = (float(b) < float(a)) if key in LOWER_IS_BETTER else (float(b) > float(a))
		return _entry("%s: %d → %d" % [label, int(a), int(b)], better)
	if a is bool or b is bool: return _entry("%s: %s" % [label, "on" if bool(b) else "off"], null)
	return _entry("%s: %s → %s" % [label, _word(_text(a)), _word(_text(b))], null)

static func _text(value) -> String:
	if value is Array: return ", ".join(value.map(func(v): return str(int(v)) if v is float else str(v)))
	return str(value)

static func _effect_name(effect: String, spec: Dictionary) -> String:
	return str(spec.get("label", _word(effect)))

static func _caption(key: String) -> String:
	return key.trim_suffix("_id").replace("_", " ").capitalize()

static func _word(value: String) -> String:
	return value.replace("_", " ") if not value.is_empty() else "none"

static func _entry(text: String, better) -> Dictionary:
	return {"text": text, "tone": "neutral" if better == null else ("gain" if better else "loss")}
