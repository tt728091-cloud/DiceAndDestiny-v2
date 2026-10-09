extends SceneTree

# Status-removal faces ("Remove 1 debuff stack") don't say whose statuses, so
# the timing ribbon ends with the authority's target badge ("Self"). The badge
# must fit beside the widest timing and play limit on a hand-sized card.
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	root.size = Vector2i(900, 400)
	var both_any := [{"segment": "offense", "when": "any"}, {"segment": "defense", "when": "any"}]
	var split := [{"segment": "offense", "when": "before"}, {"segment": "defense", "when": "after", "reaction": true}]
	BattlePresentationCatalog.configure({"cards": {
		"salve": _card("Salve", "Remove 1 debuff stack", both_any, "", "Self"),
		"dispel": _card("Dispel", "Remove 1 enemy buff stack", both_any, "", "Enemy"),
		"crowded": _card("Crowded", "Remove 1 status stack", split, "", "Self/Anyone"),
		"steady_guard": _card("Steady Guard", "Prevent 1", [{"segment": "defense", "when": "any"}], "", ""),
		"legacy": _card("Legacy", "Remove 1 debuff stack", [], "", "Self"),
	}})
	var board := Control.new(); board.theme = preload("res://presentation/battle/cinematic_theme.gd").create(); root.add_child(board)
	var shown := {}
	var index := 0
	for id in ["salve", "dispel", "crowded", "steady_guard", "legacy"]:
		var card := BattleCard.new(); board.add_child(card); card.configure(id, id, true)
		card.size = card.custom_minimum_size; card.position = Vector2(15 + index * 175, 20)
		shown[id] = card; index += 1
	for frame in 5: await process_frame
	for id in shown:
		var card: BattleCard = shown[id]
		var badge := card.find_child("TargetBadge", true, false) as Label
		var target := str(BattlePresentationCatalog.card(id).target)
		if target.is_empty():
			_expect(badge == null, id + " removes no statuses and shows no target")
			continue
		_expect(badge != null and badge.text.ends_with(target), id + " ribbon names its target: " + (badge.text if badge else "missing"))
		var ribbon := card.find_child("TimingRibbon", true, false) as Control
		var plaque := card.get_node("EffectPlaque") as Control
		_expect(plaque.get_combined_minimum_size().x <= card.size.x - 2.0 * card._plaque_inset() + 0.5, "%s ribbon fits the plaque: %.1f" % [id, ribbon.get_combined_minimum_size().x])
		_expect((badge.get_parent() is HBoxContainer) == (id != "crowded"), id + " target shares the timing line only when it fits")
		_expect(card.get_global_rect().encloses(badge.get_global_rect()), id + " target stays on the card")
	_expect((shown.salve.find_child("TargetBadge", true, false) as Label).text == "· Self", "Salve reads Any time · Self")
	_expect((shown.legacy.find_child("TargetBadge", true, false) as Label).text == "Self", "a target without timing has no leading separator")
	await _capture("card-status-target")
	board.queue_free(); await process_frame
	await _check_editor_preview()
	print("CARD STATUS TARGET: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

# The editor preview takes the authority's validated presentation, so a
# status-removal card names its target as soon as its owner is chosen.
func _check_editor_preview() -> void:
	root.size = Vector2i(1280, 720)
	var characters = preload("res://app/screens/character/character_creation.gd").new(); root.add_child(characters)
	for frame in 6: await process_frame
	characters._tabs.current_tab = 3
	for frame in 6: await process_frame
	var ui = characters.get_node("CardCreationWorkspace")
	for index in ui._template.item_count:
		if ui._template.get_item_metadata(index) == "dispel": ui._template.select(index)
	ui._load_template(); ui._clone_template()
	ui.draft.id = "status_target_preview"; ui.draft.name = "Status Target Preview"
	ui._validate(); await process_frame
	_expect(str(ui.draft.presentation.get("target", "")) == "Enemy", "authority names Dispel's target: " + str(ui.draft.presentation))
	ui.draft.program.steps[0].target.owner = "self"; ui.draft.program.steps[0].target.polarity = "negative"
	ui._validate(); for frame in 3: await process_frame
	var badge := ui._card_frame.find_child("TargetBadge", true, false) as Label
	_expect(badge != null and badge.text == "· Self", "editor preview reads Self: " + (badge.text if badge else ui._error.text))
	characters.queue_free(); await process_frame

func _card(name: String, face: String, timing: Array, limit: String, target: String) -> Dictionary:
	return {"name": name, "cost": {"energy": 5}, "presentation": {"effect_summary": face, "rules_text": face, "timing": timing, "play_limit": limit, "target": target}}

func _capture(id: String) -> void:
	var directory := OS.get_environment("DICE_AND_DESTINY_CINEMATIC_SCREENSHOTS")
	if directory.is_empty() or DisplayServer.get_name() == "headless": return
	await RenderingServer.frame_post_draw
	root.get_texture().get_image().save_png(directory.path_join(id + ".png"))

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
