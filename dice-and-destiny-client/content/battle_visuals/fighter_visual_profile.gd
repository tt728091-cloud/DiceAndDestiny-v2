@tool
class_name FighterVisualProfile
extends Resource

## Reusable art, independent of an actor instance, habitat, or combat rules.
@export var definition_id := ""
@export var display_name := ""
@export var texture: Texture2D
@export var portrait: Texture2D
@export var faces_left := true
@export_range(1.0, 1080.0) var default_height := 520.0
## Normalized contact point in the texture, usually between the feet.
@export var ground_anchor := Vector2(0.5, 0.98)
