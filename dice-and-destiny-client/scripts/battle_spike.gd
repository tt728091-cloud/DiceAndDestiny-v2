extends Control

const ROLL_DICE_COMMAND := {
	"battle_id": "battle-1",
	"actor_id": "player",
	"type": "roll_dice",
	"payload": {
		"pool": "offensive"
	}
}

var _authority: BattleAuthority = GoGDExtensionBattleAuthority.new()
var _result_display: TextEdit

func _ready() -> void:
	_build_ui()

func _build_ui() -> void:
	var root := VBoxContainer.new()
	root.set_anchors_preset(Control.PRESET_FULL_RECT)
	root.offset_left = 32.0
	root.offset_top = 32.0
	root.offset_right = -32.0
	root.offset_bottom = -32.0
	root.add_theme_constant_override("separation", 16)
	add_child(root)

	var title := Label.new()
	title.text = "Dice and Destiny battle authority spike"
	title.add_theme_font_size_override("font_size", 24)
	root.add_child(title)

	var button := Button.new()
	button.text = "Roll Dice"
	button.custom_minimum_size = Vector2(160, 48)
	button.pressed.connect(_on_roll_dice_pressed)
	root.add_child(button)

	_result_display = TextEdit.new()
	_result_display.editable = false
	_result_display.wrap_mode = TextEdit.LINE_WRAPPING_BOUNDARY
	_result_display.custom_minimum_size = Vector2(0, 280)
	_result_display.text = "Press Roll Dice."
	root.add_child(_result_display)

func _on_roll_dice_pressed() -> void:
	var command_json := JSON.stringify(ROLL_DICE_COMMAND)
	var result_json := _authority.submit_command(command_json)
	_result_display.text = result_json
