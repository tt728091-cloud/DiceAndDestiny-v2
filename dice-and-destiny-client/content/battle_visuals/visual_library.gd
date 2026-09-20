@tool
class_name BattleVisualLibrary
extends Resource

@export var fighters: Array[FighterVisualProfile] = []
@export var habitats: Array[BattleHabitatProfile] = []
@export var default_encounter: BattleEncounterVisual

func fighter(definition_id: String) -> FighterVisualProfile:
	for profile in fighters:
		if profile != null and profile.definition_id == definition_id:
			return profile
	return null
