@tool
extends Control

## The same compositor serves the live battle and standalone encounter previews.
## It only reads resources and actor identities; it never changes game state.
@export var encounter: BattleEncounterVisual:
	set(value):
		encounter = value
		if is_inside_tree(): display(value, _actors)
@export var library: BattleVisualLibrary = preload("res://content/battle_visuals/library.tres")
var _actors: Dictionary = {}

func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	display(encounter, _actors)

func display(layout: BattleEncounterVisual, actors: Dictionary = {}) -> void:
	_actors = actors.duplicate(true)
	for child in get_children():
		remove_child(child)
		child.queue_free()
	if layout == null: layout = library.default_encounter
	if layout == null: return
	if layout.habitat != null:
		var background := _texture_rect("Habitat", layout.habitat.background)
		background.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
		background.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_COVERED
	var placements := layout.fighters.filter(func(value): return value != null)
	placements.sort_custom(func(a, b): return a.draw_order < b.draw_order)
	var instance_ids := {}
	for placement in placements:
		if placement.instance_id.is_empty() or instance_ids.has(placement.instance_id):
			push_warning("Battle scenery requires a unique, non-empty instance_id for each fighter.")
			continue
		instance_ids[placement.instance_id] = true
		var profile: FighterVisualProfile = placement.profile
		if not placement.actor_id.is_empty():
			# Bound actors always use their actual identity, never a preview skin.
			var actor: Dictionary = actors.get(placement.actor_id, {})
			profile = library.fighter(str(actor.get("definition_id", "")))
		if profile == null or profile.texture == null: continue
		var fighter := _texture_rect("Fighter_" + placement.instance_id, profile.texture)
		fighter.set_meta("actor_id", placement.actor_id)
		fighter.set_meta("definition_id", profile.definition_id)
		fighter.set_meta("instance_id", placement.instance_id)
		fighter.flip_h = profile.faces_left != placement.face_left
		var actor_height: float = placement.height if placement.height > 0 else profile.default_height
		fighter.size = Vector2(actor_height * profile.texture.get_width() / profile.texture.get_height(), actor_height)
		var anchor := profile.ground_anchor
		if fighter.flip_h: anchor.x = 1.0 - anchor.x
		fighter.position = placement.ground_position - fighter.size * anchor
	if layout.habitat != null:
		var shade := ColorRect.new()
		shade.name = "HabitatShade"
		shade.mouse_filter = Control.MOUSE_FILTER_IGNORE
		shade.color = layout.habitat.shade
		add_child(shade)
		shade.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)

func _texture_rect(id: String, texture: Texture2D) -> TextureRect:
	var result := TextureRect.new()
	result.name = id
	result.texture = texture
	result.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	result.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(result)
	return result
