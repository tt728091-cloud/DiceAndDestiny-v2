extends SceneTree
## Loads every expedition client script so parse errors fail fast.
func _initialize() -> void:
	var failed := false
	for path in ["res://app/screens/map/map_style.gd", "res://app/screens/map/map_sheet.gd", "res://app/screens/map/map_line_layer.gd", "res://app/screens/map/map_marks_layer.gd", "res://app/screens/map/hex_button.gd", "res://app/screens/map/expedition_screen.gd", "res://app/screens/campaign/campaign_screen.gd", "res://app/screens/campaign/campaign_prepare.gd", "res://app/screens/battle/battle_screen.gd"]:
		var script = load(path)
		if script == null or not script.can_instantiate():
			push_error("cannot load " + path); failed = true
	print("EXPEDITION PARSE: " + ("FAILED" if failed else "PASSED")); quit(1 if failed else 0)
