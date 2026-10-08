extends SceneTree

## Card face colours come from presentation.frame_color / border_color: a
## palette name, a "#rrggbb" value, or empty for the defaults.

const COLORS := preload("res://presentation/cards/card_colors.gd")
var failed := false

func _initialize() -> void: call_deferred("_run")

func _run() -> void:
	var gateway = preload("res://local_client/learned_battle/learned_battle_gateway.gd").new(root.get_node("LearnedBattleRuntime"), "seat-a", "brine-mask", "adventurer")
	var fixture: Dictionary = gateway.start_battle("card-colors", 43)
	_expect(fixture.get("accepted") == true, "native catalog loads")
	BattleViewState.new().apply_result(fixture)
	var cards: Dictionary = BattlePresentationCatalog._catalog.get("cards", {})
	_expect(cards.has("brace"), "Brace is in the catalog")
	var board := Control.new(); root.add_child(board)
	var cases := [
		[{}, COLORS.FRAMES.colorless, COLORS.BORDERS.black],
		[{"frame_color": "red", "border_color": "white"}, COLORS.FRAMES.red, COLORS.BORDERS.white],
		[{"frame_color": "#336699", "border_color": "#102030"}, Color("336699"), Color("102030")],
		[{"frame_color": "purple", "border_color": "#12"}, COLORS.FRAMES.colorless, COLORS.BORDERS.black],
	]
	for entry in cases:
		var presentation: Dictionary = cards.brace.get("presentation", {})
		presentation.erase("frame_color"); presentation.erase("border_color")
		presentation.merge(entry[0], true)
		cards.brace.presentation = presentation
		var card := BattleCard.new(); board.add_child(card)
		card.configure("brace-1", "brace", true)
		card.size = card.custom_minimum_size
		_expect(card._frame_color.is_equal_approx(entry[1]), "frame colour for %s" % [entry[0]])
		_expect(card._border_color.is_equal_approx(entry[2]), "border colour for %s" % [entry[0]])
		_expect(card.get_node("CardFrame") != null and card.get_node("EnergyCost") != null, "frame overlay and cost exist")
	for frame in 3: await process_frame
	print("CARD COLORS: " + ("FAILED" if failed else "PASSED"))
	quit(1 if failed else 0)

func _expect(ok: bool, message: String) -> void:
	if not ok: failed = true; push_error(message)
