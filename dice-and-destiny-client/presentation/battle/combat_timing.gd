extends RefCounted

# Project Settings > Dice And Destiny > Presentation. All values are seconds.
static func seconds(key: String, fallback: float) -> float:
	return maxf(0.0, float(ProjectSettings.get_setting("dice_and_destiny/presentation/" + key, fallback)))

static func transition() -> float: return seconds("combat_transition_seconds", 0.65)
static func reveal() -> float: return seconds("damage_reveal_seconds", 0.65)
static func hold() -> float: return seconds("damage_hold_seconds", 2.4)
static func review_seconds() -> float: return reveal() + hold() + 0.25
static func removal() -> float: return seconds("damage_removal_seconds", 0.7)
static func effects_gather() -> float: return seconds("effects_gather_seconds", 0.65)
