@tool
class_name BattleFighterPlacement
extends Resource

## Unique per placement, so the same visual profile can appear repeatedly.
@export var instance_id := ""
## Optional authority actor binding. Its definition selects the visual profile.
@export var actor_id := ""
## Explicit profile for previews or unbound decorative combatants.
@export var profile: FighterVisualProfile
## Coordinates in the battle's existing 1920 x 1080 design space.
@export var ground_position := Vector2.ZERO
## Zero uses the profile's default height. Aspect ratio is always preserved.
@export_range(0.0, 1080.0) var height := 0.0
@export var face_left := true
## Higher values draw later, but all scenery remains behind the battle UI.
@export var draw_order := 0
