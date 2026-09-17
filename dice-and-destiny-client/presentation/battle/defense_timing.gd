extends RefCounted

# Seconds, configurable in Project Settings > Dice And Destiny > Presentation.
static func roll_seconds() -> float:
	return maxf(0.1, float(ProjectSettings.get_setting("dice_and_destiny/presentation/defense_roll_seconds", 1.2)))

static func effects_seconds() -> float:
	return maxf(0.1, float(ProjectSettings.get_setting("dice_and_destiny/presentation/defense_effects_seconds", 2.4)))

static func hold_seconds() -> float:
	return maxf(0.0, float(ProjectSettings.get_setting("dice_and_destiny/presentation/defense_hold_seconds", 1.5)))

static func total_seconds() -> float:
	return roll_seconds() + effects_seconds() + hold_seconds()
