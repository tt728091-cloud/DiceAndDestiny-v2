extends HBoxContainer

var _started := 0
var _before := 0
var _after := 0
var _amount: Label
var _saved: HBoxContainer

func configure(data: Dictionary) -> void:
	alignment = BoxContainer.ALIGNMENT_CENTER
	add_theme_constant_override("separation", 18)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_started = int(data.started_ms)
	_before = int(data.before); _after = int(data.after)
	var played := BattleCard.new(); add_child(played)
	played.configure(str(data.instance_id), str(data.card_id), false, false, true)
	var middle := VBoxContainer.new(); middle.alignment = BoxContainer.ALIGNMENT_CENTER; add_child(middle)
	var title := Label.new(); title.text = "INCOMING DAMAGE"; middle.add_child(title)
	_amount = Label.new(); _amount.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; _amount.add_theme_font_size_override("font_size", 36); _amount.add_theme_color_override("font_color", Color("a5edce")); middle.add_child(_amount)
	var benefit := Label.new(); benefit.text = "PREVENT %d" % maxi(0, _before - _after); middle.add_child(benefit)
	_saved = HBoxContainer.new(); add_child(_saved)
	for removal in data.get("saved", []):
		var column := VBoxContainer.new(); _saved.add_child(column)
		var caption := Label.new(); caption.text = "SAVED"; caption.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER; caption.add_theme_color_override("font_color", Color("a5edce")); column.add_child(caption)
		var card := BattleCard.new(); column.add_child(card); card.configure(str(removal.card_id), str(removal.card_definition_id), false, false, true)
	_process(0)

func _process(_delta: float) -> void:
	if not is_instance_valid(_amount): return
	var elapsed := (Time.get_ticks_msec() - _started) / 1000.0
	var progress := clampf((elapsed - 0.3) / 0.65, 0, 1)
	_amount.text = str(roundi(lerpf(_before, _after, progress)))
	_saved.modulate.a = 1.0 - clampf((elapsed - 0.85) / 0.65, 0, 1)
	modulate.a = 1.0 - clampf((elapsed - 1.4) / 0.4, 0, 1)
